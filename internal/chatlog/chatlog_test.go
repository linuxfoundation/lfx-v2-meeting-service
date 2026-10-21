// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package chatlog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return data
}

func TestParse(t *testing.T) {
	t.Run("older single-line layout", func(t *testing.T) {
		got := Parse(fixture(t, "older_single_line.chat"), false)
		assert.Equal(t, []Message{
			{Offset: "00:00:12", Sender: "Alice Example", Recipient: "Everyone", Text: "Hello everyone"},
			{Offset: "00:01:03", Sender: "Bob Fixture", Recipient: "Everyone", Text: "Agenda: item one, item two"},
			{Offset: "00:02:45", Sender: "Alice Example", Recipient: "Bob Fixture(Direct Message)", Text: "can you share the doc?"},
			{Offset: "00:03:10", Sender: "Bob Fixture", Recipient: "Alice Example(Privately)", Text: "sure, one moment"},
		}, got)
	})

	t.Run("newer multi-line layout", func(t *testing.T) {
		got := Parse(fixture(t, "newer_multi_line.chat"), false)
		assert.Equal(t, []Message{
			{Offset: "00:00:12", Sender: "Alice Example", Recipient: "Everyone", Text: "Hello everyone"},
			{Offset: "00:01:03", Sender: "Bob Fixture", Recipient: "Everyone", Text: "Two points today:\n1. the release\n2. the roadmap"},
			{Offset: "00:02:45", Sender: "Alice Example", Recipient: "Bob Fixture(Direct Message)", Text: "can you share the doc?"},
		}, got)
	})

	t.Run("only tab-indented lines continue the previous message", func(t *testing.T) {
		// The fixture has, after a public header: a line without a tab, then a
		// tab-indented line. The first is unattributed (its own message, dropped by
		// PublicOnly); the second continues the message before it, which is that
		// unattributed one, never the public one.
		got := Parse(fixture(t, "continuation.chat"), false)
		assert.Equal(t, []Message{
			{Offset: "00:00:12", Sender: "Alice Example", Recipient: "Everyone", Text: "first line"},
			{Text: "second line without a tab\nthird line with a tab"},
			{Offset: "00:00:30", Sender: "Bob Fixture", Recipient: "Everyone", Text: "next message"},
		}, got)
		public := PublicOnly(got)
		require.Len(t, public, 2)
		assert.Equal(t, "first line", public[0].Text)
	})

	t.Run("a tab-indented line after a public message is a continuation", func(t *testing.T) {
		got := Parse([]byte("00:00:12\tFrom Alice Example to Everyone:\tfirst\n\tmore\n"), false)
		require.Len(t, got, 1)
		assert.Equal(t, "first\nmore", got[0].Text)
	})

	t.Run("a non-indented, non-timestamped line after a public message is a new unattributed message", func(t *testing.T) {
		got := Parse([]byte("00:00:12\tFrom Alice Example to Everyone:\tfirst\nloose line\n"), false)
		require.Len(t, got, 2)
		assert.Equal(t, "first", got[0].Text)
		assert.Equal(t, Message{Text: "loose line"}, got[1])
		assert.Len(t, PublicOnly(got), 1)
	})

	t.Run("a line with an unrecognised timestamp form never joins the previous message", func(t *testing.T) {
		got := Parse([]byte(""+
			"00:00:12\tFrom Alice Example to Everyone:\tpublic text\n"+
			"00:00:40.123\tFrom Bob Fixture to Alice Example(Direct Message):\tprivate-secret\n"+
			"0:00:50 \tBob Fixture -> Everyone: odd-layout-text\n"), false)
		require.Len(t, got, 3)
		assert.Equal(t, "public text", got[0].Text, "the public message's text is unchanged")
		// Fractional seconds are a recognised header form: the private message is
		// attributed and dropped by PublicOnly as any direct message.
		assert.Equal(t, Message{Offset: "00:00:40.123", Sender: "Bob Fixture", Recipient: "Alice Example(Direct Message)", Text: "private-secret"}, got[1])
		assert.Equal(t, Message{Offset: "0:00:50", Text: "Bob Fixture -> Everyone: odd-layout-text"}, got[2])

		public := PublicOnly(got)
		require.Len(t, public, 1)
		assert.Equal(t, "public text", public[0].Text)
	})

	t.Run("a fractional-seconds public header parses", func(t *testing.T) {
		got := Parse([]byte("01:02:03.456 From Alice Example to Everyone:\n\thello\n"), false)
		assert.Equal(t, []Message{{Offset: "01:02:03.456", Sender: "Alice Example", Recipient: "Everyone", Text: "hello"}}, got)
		assert.Len(t, PublicOnly(got), 1)
	})

	t.Run("unrecognised first line becomes a message without sender", func(t *testing.T) {
		got := Parse(fixture(t, "unrecognised_first_line.chat"), false)
		assert.Equal(t, []Message{
			{Text: "Saved chat for Test Org weekly"},
			{Offset: "00:00:12", Sender: "Alice Example", Recipient: "Everyone", Text: "Hello"},
		}, got)
	})

	t.Run("a timestamped line of an unknown layout starts its own message", func(t *testing.T) {
		// The drifted line has a timestamp and a tab but no ":<TAB>" header
		// terminator, so no header pattern matches it.
		got := Parse(fixture(t, "drifted_timestamped_line.chat"), false)
		require.Len(t, got, 3, "the drifted line must not be glued to its neighbours")
		assert.Equal(t, []Message{
			{Offset: "00:00:12", Sender: "Alice Example", Recipient: "Everyone", Text: "first"},
			{Offset: "00:00:40", Text: "Alice Example -> Everyone: drifted layout line"},
			{Offset: "00:01:03", Sender: "Bob Fixture", Recipient: "Everyone", Text: "third"},
		}, got)
	})

	t.Run("a bare timestamp line followed by continuation lines", func(t *testing.T) {
		got := Parse([]byte("00:00:40\n\tbody line\n"), false)
		assert.Equal(t, []Message{{Offset: "00:00:40", Text: "body line"}}, got)
	})

	t.Run("BOM, CRLF and trailing whitespace are tolerated", func(t *testing.T) {
		got := Parse(fixture(t, "bom_crlf.chat"), false)
		assert.Equal(t, []Message{
			{Offset: "00:00:12", Sender: "Alice Example", Recipient: "Everyone", Text: "Hello"},
			{Offset: "00:01:03", Sender: "Bob Fixture", Recipient: "Everyone", Text: "line one\nline two"},
		}, got)
	})

	t.Run("empty file yields no messages", func(t *testing.T) {
		assert.Empty(t, Parse(fixture(t, "empty.chat"), false))
		assert.Empty(t, Parse(nil, false))
	})

	t.Run("text is returned verbatim and never interpreted", func(t *testing.T) {
		// Markup, template syntax and a link-looking token are data.
		line := "00:00:01\tFrom Alice Example to Everyone:\t<b>{{ .Secret }}</b> see example.test/path?x=1 ${HOME}"
		got := Parse([]byte(line), false)
		require.Len(t, got, 1)
		assert.Equal(t, "<b>{{ .Secret }}</b> see example.test/path?x=1 ${HOME}", got[0].Text)
	})

	t.Run("header with empty text then continuation", func(t *testing.T) {
		got := Parse([]byte("00:00:01\tFrom Alice Example to Everyone:\t\n\tlater line\n"), false)
		assert.Equal(t, []Message{
			{Offset: "00:00:01", Sender: "Alice Example", Recipient: "Everyone", Text: "later line"},
		}, got)
	})
}

// TestParse_NameHeaderLayout covers the third layout: "HH:MM:SS<TAB><name>:<TAB><text>"
// with unindented continuation lines and no recipient written.
func TestParse_NameHeaderLayout(t *testing.T) {
	t.Run("production-shaped file: CRLF, no BOM, trailing empty line", func(t *testing.T) {
		got := Parse(fixture(t, "name_header_crlf.chat"), false)
		assert.Equal(t, []Message{
			{Offset: "00:00:12", Sender: "Alice Example", Recipient: "Everyone", Text: "Hello everyone"},
			// A parenthesised affiliation stays part of the sender.
			{Offset: "00:00:30", Sender: "Bob Fixture (Example Foundation, Inc.)", Recipient: "Everyone", Text: "Two points today:\n1. the release\n2. the roadmap"},
			// The provider's reply marker is text, kept verbatim.
			{Offset: "00:01:03", Sender: "Carol Example", Recipient: "Everyone", Text: "Replying to \"Two points today:\"\nagreed on both"},
			// " to " in the header: sender to recipient; its continuation belongs to it.
			{Offset: "00:01:20", Sender: "Alice Example", Recipient: "Bob Fixture", Text: "private-to-text\nprivate-to-continuation-text"},
			{Offset: "00:01:45", Sender: "Bob Fixture (Direct Message)", Recipient: "(Direct Message)", Text: "private-dm-text"},
			{Offset: "00:02:00", Sender: "Carol Example (Privately)", Recipient: "(Privately)", Text: "private-pm-text"},
			// Empty text on the header line, then a continuation line; the trailing
			// empty line adds no message.
			{Offset: "00:02:30", Sender: "Dana Example", Recipient: "Everyone", Text: "text after an empty header line"},
		}, got)

		public := PublicOnly(got)
		require.Len(t, public, 4)
		assert.Equal(t, []string{"Alice Example", "Bob Fixture (Example Foundation, Inc.)", "Carol Example", "Dana Example"},
			[]string{public[0].Sender, public[1].Sender, public[2].Sender, public[3].Sender})
		for _, m := range public {
			assert.Equal(t, "Everyone", m.Recipient)
			assert.NotContains(t, m.Text, "private-")
		}
	})

	t.Run("single-line message", func(t *testing.T) {
		got := Parse([]byte("00:00:12\tAlice Example:\tHello everyone\n"), false)
		assert.Equal(t, []Message{{Offset: "00:00:12", Sender: "Alice Example", Recipient: "Everyone", Text: "Hello everyone"}}, got)
		assert.Len(t, PublicOnly(got), 1)
	})

	t.Run("two unindented continuation lines join with newlines", func(t *testing.T) {
		got := Parse([]byte("00:00:12\tAlice Example:\tone\ntwo\nthree\n00:00:20\tBob Fixture:\tnext\n"), false)
		require.Len(t, got, 2)
		assert.Equal(t, "one\ntwo\nthree", got[0].Text)
		assert.Equal(t, "next", got[1].Text)
	})

	t.Run("a tab-indented line continues a name-layout message too", func(t *testing.T) {
		got := Parse([]byte("00:00:12\tAlice Example:\tone\n\ttwo\n"), false)
		require.Len(t, got, 1)
		assert.Equal(t, "one\ntwo", got[0].Text)
	})

	t.Run("a continuation after a private message is dropped with it", func(t *testing.T) {
		got := Parse([]byte(""+
			"00:00:12\tAlice Example:\tpublic one\n"+
			"00:00:20\tAlice Example to Bob Fixture:\tprivate-text\n"+
			"private-continuation-text\n"+
			"00:00:30\tBob Fixture:\tpublic two\n"), false)
		require.Len(t, got, 3)
		assert.Equal(t, "private-text\nprivate-continuation-text", got[1].Text)
		public := PublicOnly(got)
		require.Len(t, public, 2)
		assert.Equal(t, "public one", public[0].Text)
		assert.Equal(t, "public two", public[1].Text)
	})

	t.Run("private markers in the header drop the message", func(t *testing.T) {
		for _, header := range []string{
			"Alice Example to Bob Fixture",
			"Alice Example to Bob Fixture (Direct Message)",
			"Bob Fixture (Direct Message)",
			"Bob Fixture (direct message)",
			"Bob Fixture (Privately)",
			"Bob Fixture (PRIVATELY)",
		} {
			got := Parse([]byte("00:00:12\t"+header+":\tprivate-text\n"), false)
			require.Len(t, got, 1, header)
			assert.Equal(t, "00:00:12", got[0].Offset, header)
			assert.Empty(t, PublicOnly(got), "header %q must be dropped", header)
		}
	})

	t.Run("a header that names Everyone after \" to \" is public, as in the older layouts", func(t *testing.T) {
		got := Parse([]byte("00:00:12\tAlice Example to Everyone:\thello\n"), false)
		assert.Equal(t, []Message{{Offset: "00:00:12", Sender: "Alice Example", Recipient: "Everyone", Text: "hello"}}, got)
	})

	t.Run("\"to\" inside a word is not a recipient separator", func(t *testing.T) {
		got := Parse([]byte("00:00:12\tAntonio Example:\thello\n00:00:13\tAlice Tomas:\thi\n"), false)
		require.Len(t, got, 2)
		assert.Equal(t, Message{Offset: "00:00:12", Sender: "Antonio Example", Recipient: "Everyone", Text: "hello"}, got[0])
		assert.Equal(t, Message{Offset: "00:00:13", Sender: "Alice Tomas", Recipient: "Everyone", Text: "hi"}, got[1])
	})

	t.Run("a colon inside the header is kept; the header ends at the last colon before the tab", func(t *testing.T) {
		got := Parse([]byte("00:00:12\tAlice Example (Team: Core):\tnote: see agenda\n"), false)
		assert.Equal(t, []Message{{Offset: "00:00:12", Sender: "Alice Example (Team: Core)", Recipient: "Everyone", Text: "note: see agenda"}}, got)
	})

	t.Run("non-ASCII and punctuation in the name are kept as written", func(t *testing.T) {
		got := Parse([]byte("00:00:12\tZo\u00eb O'Example-Fixture, Jr.:\thello\n"), false)
		assert.Equal(t, []Message{{Offset: "00:00:12", Sender: "Zo\u00eb O'Example-Fixture, Jr.", Recipient: "Everyone", Text: "hello"}}, got)
	})

	t.Run("H:MM:SS and fractional seconds", func(t *testing.T) {
		got := Parse([]byte("1:02:03\tAlice Example:\tone\n01:02:04.567\tBob Fixture:\ttwo\n"), false)
		require.Len(t, got, 2)
		assert.Equal(t, "1:02:03", got[0].Offset)
		assert.Equal(t, "01:02:04.567", got[1].Offset)
		assert.Len(t, PublicOnly(got), 2)
	})

	t.Run("empty text on the header line followed by continuation lines", func(t *testing.T) {
		got := Parse([]byte("00:00:12\tAlice Example:\t\nfirst\nsecond\n"), false)
		assert.Equal(t, []Message{{Offset: "00:00:12", Sender: "Alice Example", Recipient: "Everyone", Text: "first\nsecond"}}, got)
	})

	t.Run("text before the first header is unattributed and dropped", func(t *testing.T) {
		got := Parse([]byte("Saved chat preamble\nsecond preamble line\n00:00:12\tAlice Example:\thello\n"), false)
		assert.Equal(t, []Message{
			{Text: "Saved chat preamble"},
			{Text: "second preamble line"},
			{Offset: "00:00:12", Sender: "Alice Example", Recipient: "Everyone", Text: "hello"},
		}, got)
		public := PublicOnly(got)
		require.Len(t, public, 1)
		assert.Equal(t, "hello", public[0].Text)
	})

	t.Run("an unindented line after an older-layout message is still unattributed", func(t *testing.T) {
		// The name layout's continuation rule applies only to messages it opened.
		got := Parse([]byte("00:00:12\tFrom Alice Example to Everyone:\tfirst\nloose line\n00:00:20\tBob Fixture:\tsecond\nits continuation\n"), false)
		assert.Equal(t, []Message{
			{Offset: "00:00:12", Sender: "Alice Example", Recipient: "Everyone", Text: "first"},
			{Text: "loose line"},
			{Offset: "00:00:20", Sender: "Bob Fixture", Recipient: "Everyone", Text: "second\nits continuation"},
		}, got)
	})

	t.Run("an unindented line after a timestamped line of an unknown layout is still unattributed", func(t *testing.T) {
		got := Parse([]byte("00:00:12\tAlice Example:\tfirst\n00:00:20\tBob -> Alice: drifted\nloose line\n"), false)
		assert.Equal(t, []Message{
			{Offset: "00:00:12", Sender: "Alice Example", Recipient: "Everyone", Text: "first"},
			{Offset: "00:00:20", Text: "Bob -> Alice: drifted"},
			{Text: "loose line"},
		}, got)
	})

	t.Run("a continuation line that starts with a timestamp is read as an unknown timestamped line", func(t *testing.T) {
		// Fail closed: such a line could be a private message in a layout the
		// parser does not know, so it is never glued onto the public message.
		got := Parse([]byte("00:00:12\tAlice Example:\tmeet at\n10:30:00 tomorrow\n"), false)
		assert.Equal(t, []Message{
			{Offset: "00:00:12", Sender: "Alice Example", Recipient: "Everyone", Text: "meet at"},
			{Offset: "10:30:00", Text: "tomorrow"},
		}, got)
		assert.Len(t, PublicOnly(got), 1)
	})

	t.Run("a \"From … to …\" header is never read as a name header", func(t *testing.T) {
		got := Parse([]byte("00:00:12\tFrom Alice Example to Bob Fixture:\tprivate-text\n"), false)
		assert.Equal(t, []Message{{Offset: "00:00:12", Sender: "Alice Example", Recipient: "Bob Fixture", Text: "private-text"}}, got)
	})

	t.Run("a header line whose text is introduced by a space, not a tab, is unknown", func(t *testing.T) {
		got := Parse([]byte("00:00:12\tAlice Example: hello\n"), false)
		assert.Equal(t, []Message{{Offset: "00:00:12", Text: "Alice Example: hello"}}, got)
		assert.Empty(t, PublicOnly(got))
	})

	t.Run("a file mixing the older and the name layouts parses both", func(t *testing.T) {
		got := Parse(fixture(t, "mixed_layouts.chat"), false)
		assert.Equal(t, []Message{
			{Offset: "00:00:12", Sender: "Alice Example", Recipient: "Everyone", Text: "older layout line"},
			{Offset: "00:00:20", Sender: "Bob Fixture", Recipient: "Everyone", Text: "newer layout line"},
			{Offset: "00:00:31", Sender: "Carol Example", Recipient: "Everyone", Text: "name layout line\nand its unindented continuation"},
			{Offset: "00:00:40", Sender: "Alice Example", Recipient: "Bob Fixture(Direct Message)", Text: "private-older-text"},
			{Text: "loose line after an older-layout message"},
			{Offset: "00:00:50", Sender: "Dana Example", Recipient: "Everyone", Text: "last"},
		}, got)
		assert.Len(t, PublicOnly(got), 4)
	})

	t.Run("truncated file drops the unterminated last line", func(t *testing.T) {
		full := "00:00:01\tAlice Example:\tone\n00:00:02\tBob Fixture:\ttwo\ncontinuation\n00:00:03\tAlice Example:\tthr"
		got := Parse([]byte(full), true)
		require.Len(t, got, 2)
		assert.Equal(t, "two\ncontinuation", got[1].Text)

		got = Parse([]byte(full), false)
		require.Len(t, got, 3)
		assert.Equal(t, "thr", got[2].Text)
	})

	t.Run("many unindented continuation lines stay linear", func(t *testing.T) {
		const n = 200000
		var b strings.Builder
		b.WriteString("00:00:01\tAlice Example:\tstart\n")
		for i := 0; i < n; i++ {
			b.WriteString("continuation line\n")
		}
		got := Parse([]byte(b.String()), false)
		require.Len(t, got, 1)
		assert.Equal(t, n, strings.Count(got[0].Text, "\n"))
	})
}

func TestClassifyNameHeader(t *testing.T) {
	for _, tc := range []struct {
		header, sender, recipient string
		public                    bool
	}{
		{"Alice Example", "Alice Example", "Everyone", true},
		{"Bob Fixture (Example Foundation, Inc.)", "Bob Fixture (Example Foundation, Inc.)", "Everyone", true},
		{"Alice Example (Team: Core)", "Alice Example (Team: Core)", "Everyone", true},
		{"Antonio Example", "Antonio Example", "Everyone", true},
		{"Alice Tomas", "Alice Tomas", "Everyone", true},
		{"Alice Example to Bob Fixture", "Alice Example", "Bob Fixture", false},
		{"Alice Example to Bob Fixture (Direct Message)", "Alice Example", "Bob Fixture (Direct Message)", false},
		{"Alice Example to Everyone", "Alice Example", "Everyone", true},
		// The first " to " separates sender and recipient.
		{"Alice Example to Bob to Carol", "Alice Example", "Bob to Carol", false},
		{"Bob Fixture (Direct Message)", "Bob Fixture (Direct Message)", "(Direct Message)", false},
		{"Bob Fixture (direct message)", "Bob Fixture (direct message)", "(Direct Message)", false},
		{"Bob Fixture (Privately)", "Bob Fixture (Privately)", "(Privately)", false},
		{"Bob Fixture (PRIVATELY)", "Bob Fixture (PRIVATELY)", "(Privately)", false},
	} {
		sender, recipient := classifyNameHeader(tc.header)
		assert.Equal(t, tc.sender, sender, "sender of %q", tc.header)
		assert.Equal(t, tc.recipient, recipient, "recipient of %q", tc.header)
		assert.Equal(t, tc.public, IsPublicRecipient(recipient), "publicness of %q", tc.header)
	}

	// The plain-name rule is the one behind recipientlessNameHeaderIsPublic: the
	// expectations above for plain names flip to an empty, non-public recipient
	// when that constant is false.
	assert.True(t, recipientlessNameHeaderIsPublic, "the plain-name expectations in this test assume the public-by-default rule")
}

func TestParse_Truncated(t *testing.T) {
	full := "00:00:01\tFrom Alice Example to Everyone:\tone\n" +
		"00:00:02\tFrom Bob Fixture to Everyone:\ttwo\n" +
		"00:00:03\tFrom Alice Example to Everyone:\tthree"

	t.Run("drops the unterminated last line when truncated", func(t *testing.T) {
		got := Parse([]byte(full), true)
		require.Len(t, got, 2)
		assert.Equal(t, "two", got[1].Text)
	})

	t.Run("keeps a terminated last line when truncated", func(t *testing.T) {
		got := Parse([]byte(full+"\n"), true)
		require.Len(t, got, 3)
		assert.Equal(t, "three", got[2].Text)
	})

	t.Run("keeps an unterminated last line when not truncated", func(t *testing.T) {
		got := Parse([]byte(full), false)
		require.Len(t, got, 3)
	})

	t.Run("truncated data without any newline yields nothing", func(t *testing.T) {
		assert.Empty(t, Parse([]byte("00:00:01\tFrom Alice Example to Everyone:\tcut"), true))
	})
}

// manyContinuations builds one header followed by n continuation lines.
func manyContinuations(n int) []byte {
	var b strings.Builder
	b.WriteString("00:00:01\tFrom Alice Example to Everyone:\tstart\n")
	for i := 0; i < n; i++ {
		b.WriteString("\tcontinuation line\n")
	}
	return []byte(b.String())
}

func TestParse_ManyContinuationLines(t *testing.T) {
	const n = 200000
	got := Parse(manyContinuations(n), false)
	require.Len(t, got, 1)
	assert.Equal(t, n, strings.Count(got[0].Text, "\n"))
	assert.True(t, strings.HasPrefix(got[0].Text, "start\ncontinuation line\n"))
	assert.True(t, strings.HasSuffix(got[0].Text, "\ncontinuation line"))
}

// BenchmarkParse_ManyContinuationLines guards the linear bound on continuation
// handling: bytes allocated per operation must stay within a small multiple of
// the input size as n grows (a quadratic implementation blows up by n).
func BenchmarkParse_ManyContinuationLines(b *testing.B) {
	for _, n := range []int{1000, 10000, 100000} {
		data := manyContinuations(n)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = Parse(data, false)
			}
		})
	}
}

func TestIsPublicRecipient(t *testing.T) {
	for _, tc := range []struct {
		recipient string
		public    bool
	}{
		{"Everyone", true},
		{"everyone", true},
		{"  EVERYONE ", true},
		{"", false},
		{"   ", false},
		{"Bob Fixture(Direct Message)", false},
		{"Bob Fixture(Privately)", false},
		{"Bob Fixture", false},
		{"Everyone(Direct Message)", false},
		{"Everyone else", false},
	} {
		assert.Equal(t, tc.public, IsPublicRecipient(tc.recipient), "recipient %q", tc.recipient)
	}
}

func TestPublicOnly(t *testing.T) {
	t.Run("drops direct, private and named recipients, keeps order", func(t *testing.T) {
		all := Parse(fixture(t, "mixed_private_public.chat"), false)
		require.Len(t, all, 6)

		public := PublicOnly(all)
		require.Len(t, public, 3)
		assert.Equal(t, "public one", public[0].Text)
		assert.Equal(t, "public two", public[1].Text)
		assert.Equal(t, "public three\nsecond line", public[2].Text)
		for _, m := range public {
			assert.NotContains(t, m.Text, "private-")
		}
	})

	t.Run("drops messages the parser could not attribute to a recipient", func(t *testing.T) {
		// Pre-header text and a drifted timestamped line still parse (so they never
		// glue onto a neighbour), but they are not returned: fail closed.
		public := PublicOnly([]Message{{Text: "preamble"}, {Offset: "00:00:01", Text: "drifted"}})
		assert.Empty(t, public)
	})

	t.Run("drops a \" to \" line of the name layout that is a direct message", func(t *testing.T) {
		all := Parse([]byte(""+
			"00:00:12\tFrom Alice Example to Everyone:\tpublic one\n"+
			"00:00:40\tAlice Example to Bob Fixture (Direct Message):\tdm-private-text\n"+
			"00:01:03\tFrom Bob Fixture to Everyone:\tpublic two\n"), false)
		require.Len(t, all, 3, "the line is its own message, not glued to a neighbour")
		assert.Equal(t, Message{Offset: "00:00:40", Sender: "Alice Example", Recipient: "Bob Fixture (Direct Message)", Text: "dm-private-text"}, all[1])

		public := PublicOnly(all)
		require.Len(t, public, 2)
		assert.Equal(t, "public one", public[0].Text)
		assert.Equal(t, "public two", public[1].Text)
		for _, m := range public {
			assert.NotContains(t, m.Text, "dm-private-text")
		}
	})

	t.Run("never returns nil", func(t *testing.T) {
		assert.NotNil(t, PublicOnly(nil))
		assert.Empty(t, PublicOnly([]Message{{Recipient: "Bob Fixture(Privately)"}}))
	})
}
