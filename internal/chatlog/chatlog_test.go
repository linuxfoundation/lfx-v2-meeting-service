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

	t.Run("continuation lines join the previous message", func(t *testing.T) {
		got := Parse(fixture(t, "continuation.chat"), false)
		assert.Equal(t, []Message{
			{Offset: "00:00:12", Sender: "Alice Example", Recipient: "Everyone", Text: "first line\nsecond line without a tab\nthird line with a tab"},
			{Offset: "00:00:30", Sender: "Bob Fixture", Recipient: "Everyone", Text: "next message"},
		}, got)
	})

	t.Run("unrecognised first line becomes a message without sender", func(t *testing.T) {
		got := Parse(fixture(t, "unrecognised_first_line.chat"), false)
		assert.Equal(t, []Message{
			{Text: "Saved chat for Test Org weekly"},
			{Offset: "00:00:12", Sender: "Alice Example", Recipient: "Everyone", Text: "Hello"},
		}, got)
	})

	t.Run("a timestamped line of an unknown layout starts its own message", func(t *testing.T) {
		got := Parse(fixture(t, "drifted_timestamped_line.chat"), false)
		require.Len(t, got, 3, "the drifted line must not be glued to its neighbours")
		assert.Equal(t, []Message{
			{Offset: "00:00:12", Sender: "Alice Example", Recipient: "Everyone", Text: "first"},
			{Offset: "00:00:40", Text: "Alice Example:\tdrifted layout line"},
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
		{"", true},
		{"   ", true},
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

	t.Run("keeps messages without a recipient", func(t *testing.T) {
		public := PublicOnly([]Message{{Text: "preamble"}, {Offset: "00:00:01", Text: "drifted"}})
		assert.Len(t, public, 2)
	})

	t.Run("never returns nil", func(t *testing.T) {
		assert.NotNil(t, PublicOnly(nil))
		assert.Empty(t, PublicOnly([]Message{{Recipient: "Bob Fixture(Privately)"}}))
	})
}
