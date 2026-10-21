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
	// display name, or a form such as "Name(Direct Message)". In the name
	// header layout, which writes no recipient, it is what classifyNameHeader
	// derives from the header: everyoneRecipient for a plain display name, the
	// private marker when the header carries one, else the part after " to ".
	Recipient string
	// Text is the message text. Multi-line messages keep their line breaks
	// as "\n".
	Text string
}

// headerPattern matches the two "From … to …" header layouts:
//
//	HH:MM:SS<TAB>From <sender> to <recipient>:<TAB><text>   (older, text on the same line)
//	HH:MM:SS From <sender> to <recipient>:                  (newer, text on following tab-indented lines)
//
// The timestamp is H:MM:SS or HH:MM:SS with optional fractional seconds. The
// sender is the shortest run before " to " so that a recipient containing no
// " to " (the common case) is matched exactly; the recipient is the shortest run
// before the ':' that ends the header. Any trailing text after ':' must be
// introduced by a tab; the group is empty for the newer layout.
var headerPattern = regexp.MustCompile(`^(\d{1,2}:\d{2}:\d{2}(?:\.\d+)?)[\t ]+From (.+?) to (.+?):(?:\t(.*))?$`)

// nameHeaderPattern matches the third header layout, the one the provider
// writes today, in which the header is the sender's display name alone:
//
//	HH:MM:SS<TAB><name>:<TAB><text>   (text may be empty; more text on following unindented lines)
//
// The name is everything between the tab and the last ':' that is followed by
// a tab or ends the line, so a name containing ':' or parentheses (an
// affiliation such as "Name (Example Foundation, Inc.)") is kept whole. The
// layout writes no recipient: classifyNameHeader derives one from the header.
// It is tried after headerPattern, so a "From … to …" header is never read as
// a name, and before timestampedPattern.
var nameHeaderPattern = regexp.MustCompile(`^(\d{1,2}:\d{2}:\d{2}(?:\.\d+)?)\t([^\t]+):(?:\t(.*))?$`)

// timestampedPattern matches any line that starts with a timestamp but is not a
// recognised header (a drifted or unknown layout), whatever follows the
// timestamp. Such a line always starts a new message of its own: the offset is
// kept, sender and recipient are empty (so PublicOnly drops it), and the text is
// the remainder. It is never appended to the message before it, which could be
// public while this line is a private message in a layout the parser does not
// know.
var timestampedPattern = regexp.MustCompile(`^(\d{1,2}:\d{2}:\d{2}(?:\.\d+)?)(.*)$`)

// layout says which header pattern opened the current message, because the
// continuation rule depends on it.
type layout int

const (
	// layoutNone: no message is open, or the open one was not opened by a
	// header (text before the first header, a timestamped line of an unknown
	// layout).
	layoutNone layout = iota
	// layoutFromTo: opened by headerPattern (the older or the newer layout).
	layoutFromTo
	// layoutName: opened by nameHeaderPattern (the name header layout).
	layoutName
)

// Parse parses a saved-chat log. data is the whole file, or its first part when
// the caller cut it at a byte cap; truncated says which. When truncated, the
// last line is dropped unless it is terminated by a newline, so a message is
// never returned with part of its text missing.
//
// Lines are split on "\n"; a trailing "\r" and trailing spaces or tabs are
// removed from every line, a UTF-8 byte-order mark is removed from the first
// line, and blank lines are ignored. A line that starts with a timestamp but
// matches no header layout still starts a new message, with an empty sender
// and recipient. A tab-indented line (the newer layout's indentation)
// continues the message before it, whatever opened that message; the tab is
// removed. A line that is neither continues the message before it only when
// that message was opened by the name header layout, whose continuation lines
// carry no indentation; after any other message it starts a new message with
// an empty offset, sender and recipient. The rule fails closed: text the
// parser cannot attribute to a recipient is never glued onto a message that
// has one, and PublicOnly drops it. A continuation line of the name layout that
// itself starts with a timestamp is read as a timestamped line of an unknown
// layout and dropped the same way.
func Parse(data []byte, truncated bool) []Message {
	if truncated {
		if i := bytes.LastIndexByte(data, '\n'); i >= 0 {
			data = data[:i+1]
		} else {
			data = nil
		}
	}
	data = bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF"))

	// Text lines of the open message are collected and joined once when the
	// message closes, so a long run of continuation lines costs linear work.
	var (
		messages  []Message
		fragments []string
		open      layout
	)
	closeMessage := func() {
		open = layoutNone
		if len(messages) == 0 {
			return
		}
		messages[len(messages)-1].Text = strings.Join(fragments, "\n")
		fragments = fragments[:0]
	}

	for _, raw := range bytes.Split(data, []byte("\n")) {
		line := strings.TrimRight(string(raw), " \t\r")
		if line == "" {
			continue
		}

		if m := headerPattern.FindStringSubmatch(line); m != nil {
			closeMessage()
			messages = append(messages, Message{
				Offset:    m[1],
				Sender:    strings.TrimSpace(m[2]),
				Recipient: strings.TrimSpace(m[3]),
			})
			open = layoutFromTo
			if m[4] != "" {
				fragments = append(fragments, m[4])
			}
			continue
		}

		if m := nameHeaderPattern.FindStringSubmatch(line); m != nil {
			closeMessage()
			sender, recipient := classifyNameHeader(strings.TrimSpace(m[2]))
			messages = append(messages, Message{
				Offset:    m[1],
				Sender:    sender,
				Recipient: recipient,
			})
			open = layoutName
			if m[3] != "" {
				fragments = append(fragments, m[3])
			}
			continue
		}

		if m := timestampedPattern.FindStringSubmatch(line); m != nil {
			closeMessage()
			messages = append(messages, Message{Offset: m[1]})
			if rest := strings.TrimLeft(m[2], "\t "); rest != "" {
				fragments = append(fragments, rest)
			}
			continue
		}

		if strings.HasPrefix(line, "\t") && len(messages) > 0 {
			fragments = append(fragments, strings.TrimPrefix(line, "\t"))
			continue
		}

		// The name layout's continuation lines are not indented. They continue
		// only a message that layout opened, so stray text is never glued onto a
		// message of the older layouts; a private name-layout message takes its
		// continuation lines down with it.
		if open == layoutName {
			fragments = append(fragments, line)
			continue
		}

		// Neither a header nor a continuation: an unattributed message.
		closeMessage()
		messages = append(messages, Message{})
		fragments = append(fragments, strings.TrimPrefix(line, "\t"))
	}
	closeMessage()
	return messages
}

// everyoneRecipient is the recipient the provider writes, in its casing, for a
// message sent to the whole meeting in the "From … to …" layouts, and the one
// the parser writes for a recipient-less public header of the name layout.
const everyoneRecipient = "Everyone"

// recipientlessNameHeaderIsPublic is the one-line switch of the name layout's
// classification rule. true: a header that is a plain display name (no " to ",
// no private marker) is a message to the whole meeting and gets
// everyoneRecipient. That rests on the provider's documented behaviour that a
// chat saved to the cloud holds only the messages sent to everyone (see the
// contract, docs/api-contracts/itx-past-meeting-chat-api.md). false: fail
// closed; such a header gets an empty recipient, PublicOnly drops it, and the
// route returns no messages on this layout until the provider marks private
// messages.
const recipientlessNameHeaderIsPublic = true

// privateMarkers are the provider's markers of a private message, matched
// case-insensitively anywhere in a name header.
var privateMarkers = []string{"(Direct Message)", "(Privately)"}

// classifyNameHeader derives sender and recipient from a name layout header,
// already trimmed. A header containing a private marker, case-insensitively and
// wherever it sits, is private whatever else the header holds: the recipient is
// the marker, which IsPublicRecipient refuses, so "Name (Privately) to Everyone"
// is dropped; the sender is the text before " to " when there is one, else the
// whole header. Otherwise a header containing " to " (spaces on both sides, so
// "Antonio" is not one) is read as sender to recipient, as in the older
// layouts, and the recipient is judged by IsPublicRecipient like any other. Any
// other header is a plain display name, classified by
// recipientlessNameHeaderIsPublic.
func classifyNameHeader(header string) (sender, recipient string) {
	before, after, hasTo := strings.Cut(header, " to ")
	sender = strings.TrimSpace(before)
	lower := strings.ToLower(header)
	for _, marker := range privateMarkers {
		if strings.Contains(lower, strings.ToLower(marker)) {
			return sender, marker
		}
	}
	if hasTo {
		return sender, strings.TrimSpace(after)
	}
	if recipientlessNameHeaderIsPublic {
		return header, everyoneRecipient
	}
	return header, ""
}

// IsPublicRecipient reports whether a message with this recipient was explicitly
// sent to the whole meeting: the recipient is "Everyone" (case-insensitively,
// after trimming). Nothing else is public. An empty recipient is not: it means the
// parser could not attribute the line to a recipient (text before the first
// header, or a timestamped line in a layout no header pattern knows), and such
// a line may well be a direct message in a drifted layout. Failing closed on
// layout drift is what keeps a private message from reaching every recording
// viewer; "Name(Direct Message)", "Name(Privately)" and bare names are private
// for the same reason. In the name header layout the recipient is not written:
// classifyNameHeader supplies "Everyone" for a plain display name and a
// non-public value for a header that carries " to " or a private marker, so
// this predicate stays the one rule for every layout.
func IsPublicRecipient(recipient string) bool {
	return strings.EqualFold(strings.TrimSpace(recipient), everyoneRecipient)
}

// PublicOnly returns the messages of msgs whose recipient IsPublicRecipient, in
// order. Messages the parser could not attribute to a recipient are dropped, never
// returned. It never returns nil.
func PublicOnly(msgs []Message) []Message {
	public := make([]Message, 0, len(msgs))
	for _, m := range msgs {
		if IsPublicRecipient(m.Recipient) {
			public = append(public, m)
		}
	}
	return public
}
