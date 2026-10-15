// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package chatlog parses the plain-text chat log the meeting provider saves
// next to a cloud recording (the CHAT recording file).
//
// The parser has no dependency on the HTTP layer or on any model of this
// service so it can be exercised with fixtures alone. It interprets nothing:
// message text is data, returned as the provider wrote it apart from the
// layout's own framing (the leading tab of a text line, the line terminator
// and trailing whitespace).
package chatlog

import (
	"bytes"
	"regexp"
	"strings"
)

// Message is one chat message as parsed from the log.
type Message struct {
	// Offset is the elapsed time since the recording started, as written in
	// the log ("HH:MM:SS"). It is not a clock time. Empty for text that
	// precedes the first recognised header line.
	Offset string
	// Sender is the display name of the sender, trimmed, as written.
	Sender string
	// Recipient is the recipient as written and trimmed: "Everyone", a
	// display name, or a form such as "Name(Direct Message)".
	Recipient string
	// Text is the message text. Multi-line messages keep their line breaks
	// as "\n".
	Text string
}

// headerPattern matches the two known header layouts:
//
//	HH:MM:SS<TAB>From <sender> to <recipient>:<TAB><text>   (older, text on the same line)
//	HH:MM:SS From <sender> to <recipient>:                  (newer, text on following tab-indented lines)
//
// The sender is the shortest run before " to " so that a recipient containing
// no " to " (the common case) is matched exactly; the recipient is the shortest
// run before the ':' that ends the header. Any trailing text after ':' must be
// introduced by a tab; the group is empty for the newer layout.
var headerPattern = regexp.MustCompile(`^(\d{2}:\d{2}:\d{2})[\t ]+From (.+?) to (.+?):(?:\t(.*))?$`)

// Parse parses a saved-chat log. data is the whole file, or its first part when
// the caller cut it at a byte cap; truncated says which. When truncated, the
// last line is dropped unless it is terminated by a newline, so a message is
// never returned with part of its text missing.
//
// Lines are split on "\n"; a trailing "\r" and trailing spaces or tabs are
// removed from every line, a UTF-8 byte-order mark is removed from the first
// line, and blank lines are ignored. A line matching neither header layout is
// appended to the previous message's text (after removing one leading tab, the
// indentation of the newer layout); when there is no previous message it
// becomes a message with an empty offset, sender and recipient.
func Parse(data []byte, truncated bool) []Message {
	if truncated {
		if i := bytes.LastIndexByte(data, '\n'); i >= 0 {
			data = data[:i+1]
		} else {
			data = nil
		}
	}
	data = bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF"))

	var messages []Message
	for _, raw := range bytes.Split(data, []byte("\n")) {
		line := strings.TrimRight(string(raw), " \t\r")
		if line == "" {
			continue
		}

		if m := headerPattern.FindStringSubmatch(line); m != nil {
			messages = append(messages, Message{
				Offset:    m[1],
				Sender:    strings.TrimSpace(m[2]),
				Recipient: strings.TrimSpace(m[3]),
				Text:      m[4],
			})
			continue
		}

		text := strings.TrimPrefix(line, "\t")
		if len(messages) == 0 {
			messages = append(messages, Message{Text: text})
			continue
		}
		last := &messages[len(messages)-1]
		if last.Text == "" {
			last.Text = text
		} else {
			last.Text += "\n" + text
		}
	}
	return messages
}
