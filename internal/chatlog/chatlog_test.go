// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package chatlog

import (
	"os"
	"path/filepath"
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
		got := Parse(fixture(t, "older_single_line.log"), false)
		assert.Equal(t, []Message{
			{Offset: "00:00:12", Sender: "Alice Example", Recipient: "Everyone", Text: "Hello everyone"},
			{Offset: "00:01:03", Sender: "Bob Fixture", Recipient: "Everyone", Text: "Agenda: item one, item two"},
			{Offset: "00:02:45", Sender: "Alice Example", Recipient: "Bob Fixture(Direct Message)", Text: "can you share the doc?"},
			{Offset: "00:03:10", Sender: "Bob Fixture", Recipient: "Alice Example(Privately)", Text: "sure, one moment"},
		}, got)
	})

	t.Run("newer multi-line layout", func(t *testing.T) {
		got := Parse(fixture(t, "newer_multi_line.log"), false)
		assert.Equal(t, []Message{
			{Offset: "00:00:12", Sender: "Alice Example", Recipient: "Everyone", Text: "Hello everyone"},
			{Offset: "00:01:03", Sender: "Bob Fixture", Recipient: "Everyone", Text: "Two points today:\n1. the release\n2. the roadmap"},
			{Offset: "00:02:45", Sender: "Alice Example", Recipient: "Bob Fixture(Direct Message)", Text: "can you share the doc?"},
		}, got)
	})

	t.Run("continuation lines join the previous message", func(t *testing.T) {
		got := Parse(fixture(t, "continuation.log"), false)
		assert.Equal(t, []Message{
			{Offset: "00:00:12", Sender: "Alice Example", Recipient: "Everyone", Text: "first line\nsecond line without a tab\nthird line with a tab"},
			{Offset: "00:00:30", Sender: "Bob Fixture", Recipient: "Everyone", Text: "next message"},
		}, got)
	})

	t.Run("unrecognised first line becomes a message without sender", func(t *testing.T) {
		got := Parse(fixture(t, "unrecognised_first_line.log"), false)
		assert.Equal(t, []Message{
			{Text: "Saved chat for Test Org weekly"},
			{Offset: "00:00:12", Sender: "Alice Example", Recipient: "Everyone", Text: "Hello"},
		}, got)
	})

	t.Run("BOM, CRLF and trailing whitespace are tolerated", func(t *testing.T) {
		got := Parse(fixture(t, "bom_crlf.log"), false)
		assert.Equal(t, []Message{
			{Offset: "00:00:12", Sender: "Alice Example", Recipient: "Everyone", Text: "Hello"},
			{Offset: "00:01:03", Sender: "Bob Fixture", Recipient: "Everyone", Text: "line one\nline two"},
		}, got)
	})

	t.Run("empty file yields no messages", func(t *testing.T) {
		assert.Empty(t, Parse(fixture(t, "empty.log"), false))
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
