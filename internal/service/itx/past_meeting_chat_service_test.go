// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// fakeRecordingRecordReader returns a canned record or error.
type fakeRecordingRecordReader struct {
	record *models.RecordingRecord
	err    error
	calls  []string
}

func (f *fakeRecordingRecordReader) GetRecordingRecord(_ context.Context, id string) (*models.RecordingRecord, error) {
	f.calls = append(f.calls, id)
	if f.err != nil {
		return nil, f.err
	}
	return f.record, nil
}

// fakeRecordingFileFetcher serves bodies keyed by link; a link with no body fails.
type fakeRecordingFileFetcher struct {
	bodies map[string]string
	err    error
	calls  []string
}

func (f *fakeRecordingFileFetcher) FetchText(_ context.Context, link string, maxBytes int64) ([]byte, bool, error) {
	f.calls = append(f.calls, link)
	if f.err != nil {
		return nil, false, f.err
	}
	body, ok := f.bodies[link]
	if !ok {
		return nil, false, errors.New("no body for link")
	}
	if int64(len(body)) > maxBytes {
		return []byte(body[:maxBytes]), true, nil
	}
	return []byte(body), false, nil
}

const (
	chatID    = "96365872728-1630560600000"
	chatLinkA = "https://recordings.example.com/a/chat_file?token=secret-a"
	chatLinkB = "https://recordings.example.com/b/chat_file?token=secret-b"
)

var (
	sessionAStart = time.Date(2026, 10, 1, 15, 0, 0, 0, time.UTC)
	sessionAEnd   = time.Date(2026, 10, 1, 16, 2, 11, 0, time.UTC)
	sessionBStart = time.Date(2026, 10, 1, 16, 10, 0, 0, time.UTC)
	sessionBEnd   = time.Date(2026, 10, 1, 16, 45, 0, 0, time.UTC)
)

func chatLines(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString("00:00:0")
		b.WriteByte(byte('0' + i%10))
		b.WriteString("\tFrom Alice Example to Everyone:\tmessage ")
		b.WriteString(strings.Repeat("x", i%3))
		b.WriteString("\n")
	}
	return b.String()
}

// twoSessionRecord returns a record whose CHAT files are listed newest first, with a
// video file and a transcript file that the route must ignore.
func twoSessionRecord() *models.RecordingRecord {
	return &models.RecordingRecord{
		MeetingAndOccurrenceID: chatID,
		RecordingAccess:        "meeting_participants",
		RecordingFiles: []models.RecordingFile{
			{FileType: "MP4", DownloadURL: "https://recordings.example.com/a/video?token=v", PlayURL: "https://recordings.example.com/play/a", RecordingStart: sessionAStart, RecordingEnd: sessionAEnd, FileSize: 12345, ID: "file-a"},
			{FileType: "CHAT", DownloadURL: chatLinkB, PlayURL: "https://recordings.example.com/play/b-chat", RecordingStart: sessionBStart, RecordingEnd: sessionBEnd, FileSize: 200, ID: "file-b-chat"},
			{FileType: "TRANSCRIPT", DownloadURL: "https://recordings.example.com/a/transcript?token=t", RecordingStart: sessionAStart, RecordingEnd: sessionAEnd},
			{FileType: "CHAT", DownloadURL: chatLinkA, PlayURL: "https://recordings.example.com/play/a-chat", RecordingStart: sessionAStart, RecordingEnd: sessionAEnd, FileSize: 100, ID: "file-a-chat"},
		},
		Sessions: []models.RecordingSession{
			{UUID: "session-a", ShareURL: "https://recordings.example.com/share/a?pwd=hunter2", StartTime: sessionAStart},
		},
	}
}

func defaultWindow() PastMeetingChatWindow {
	return PastMeetingChatWindow{Limit: 500, Offset: 0}
}

func TestPastMeetingChatService_GetPastMeetingChat(t *testing.T) {
	t.Run("returns sessions ordered by recording start with parsed messages", func(t *testing.T) {
		reader := &fakeRecordingRecordReader{record: twoSessionRecord()}
		fetcher := &fakeRecordingFileFetcher{bodies: map[string]string{
			chatLinkA: "00:00:12\tFrom Alice Example to Everyone:\tfirst session\n",
			chatLinkB: "00:00:05 From Bob Fixture to Alice Example(Direct Message):\n\tsecond session\n\tline two\n",
		}}
		svc := NewPastMeetingChatService(reader, fetcher, 1024)

		resp, err := svc.GetPastMeetingChat(context.Background(), chatID, defaultWindow())
		require.NoError(t, err)
		assert.Equal(t, []string{chatID}, reader.calls)
		// Only the CHAT files are fetched, in session order.
		assert.Equal(t, []string{chatLinkA, chatLinkB}, fetcher.calls)

		assert.Equal(t, chatID, resp.MeetingAndOccurrenceID)
		require.Len(t, resp.Sessions, 2)

		a := resp.Sessions[0]
		assert.Equal(t, sessionAStart, a.RecordingStart)
		assert.Equal(t, sessionAEnd, a.RecordingEnd)
		assert.Equal(t, 1, a.MessageCount)
		assert.False(t, a.Truncated)
		require.Len(t, a.Messages, 1)
		assert.Equal(t, "00:00:12", a.Messages[0].Offset)
		assert.Equal(t, "Alice Example", a.Messages[0].Sender)
		assert.Equal(t, "Everyone", a.Messages[0].Recipient)
		assert.Equal(t, "first session", a.Messages[0].Text)

		b := resp.Sessions[1]
		assert.Equal(t, sessionBStart, b.RecordingStart)
		require.Len(t, b.Messages, 1)
		assert.Equal(t, "Bob Fixture", b.Messages[0].Sender)
		assert.Equal(t, "Alice Example(Direct Message)", b.Messages[0].Recipient)
		assert.Equal(t, "second session\nline two", b.Messages[0].Text)
	})

	t.Run("empty chat file yields an empty session", func(t *testing.T) {
		record := twoSessionRecord()
		record.RecordingFiles = record.RecordingFiles[3:] // session A's CHAT only
		reader := &fakeRecordingRecordReader{record: record}
		fetcher := &fakeRecordingFileFetcher{bodies: map[string]string{chatLinkA: ""}}
		svc := NewPastMeetingChatService(reader, fetcher, 1024)

		resp, err := svc.GetPastMeetingChat(context.Background(), chatID, defaultWindow())
		require.NoError(t, err)
		require.Len(t, resp.Sessions, 1)
		assert.Equal(t, 0, resp.Sessions[0].MessageCount)
		assert.False(t, resp.Sessions[0].Truncated)
		assert.NotNil(t, resp.Sessions[0].Messages)
		assert.Empty(t, resp.Sessions[0].Messages)
	})

	t.Run("no record is 404", func(t *testing.T) {
		reader := &fakeRecordingRecordReader{err: domain.ErrRecordingRecordNotFound}
		fetcher := &fakeRecordingFileFetcher{}
		svc := NewPastMeetingChatService(reader, fetcher, 1024)

		_, err := svc.GetPastMeetingChat(context.Background(), chatID, defaultWindow())
		assert.Equal(t, domain.ErrorTypeNotFound, domain.GetErrorType(err))
		assert.Equal(t, "no saved chat for this past meeting", err.Error())
		assert.Empty(t, fetcher.calls)
	})

	t.Run("record without CHAT file is 404", func(t *testing.T) {
		record := twoSessionRecord()
		record.RecordingFiles = []models.RecordingFile{record.RecordingFiles[0], record.RecordingFiles[2]}
		reader := &fakeRecordingRecordReader{record: record}
		fetcher := &fakeRecordingFileFetcher{}
		svc := NewPastMeetingChatService(reader, fetcher, 1024)

		_, err := svc.GetPastMeetingChat(context.Background(), chatID, defaultWindow())
		assert.Equal(t, domain.ErrorTypeNotFound, domain.GetErrorType(err))
		assert.Equal(t, "no saved chat for this past meeting", err.Error())
		assert.Empty(t, fetcher.calls)
	})

	t.Run("record with null recording_files is 404", func(t *testing.T) {
		reader := &fakeRecordingRecordReader{record: &models.RecordingRecord{MeetingAndOccurrenceID: chatID}}
		svc := NewPastMeetingChatService(reader, &fakeRecordingFileFetcher{}, 1024)

		_, err := svc.GetPastMeetingChat(context.Background(), chatID, defaultWindow())
		assert.Equal(t, domain.ErrorTypeNotFound, domain.GetErrorType(err))
	})

	t.Run("nil reader is 503 without reading anything", func(t *testing.T) {
		fetcher := &fakeRecordingFileFetcher{}
		svc := NewPastMeetingChatService(nil, fetcher, 1024)

		_, err := svc.GetPastMeetingChat(context.Background(), chatID, defaultWindow())
		assert.Equal(t, domain.ErrorTypeUnavailable, domain.GetErrorType(err))
		assert.Equal(t, "chat retrieval unavailable", err.Error())
		assert.Empty(t, fetcher.calls)
	})

	t.Run("reader failure is passed through as classified", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			err  error
			want domain.ErrorType
		}{
			{"unavailable", domain.NewUnavailableError("recording record could not be read"), domain.ErrorTypeUnavailable},
			{"internal", domain.NewInternalError("saved chat record could not be decoded"), domain.ErrorTypeInternal},
		} {
			t.Run(tc.name, func(t *testing.T) {
				reader := &fakeRecordingRecordReader{err: tc.err}
				svc := NewPastMeetingChatService(reader, &fakeRecordingFileFetcher{}, 1024)
				_, err := svc.GetPastMeetingChat(context.Background(), chatID, defaultWindow())
				assert.Equal(t, tc.want, domain.GetErrorType(err))
			})
		}
	})

	t.Run("one failing file fails the whole request with 503 and no partial response", func(t *testing.T) {
		reader := &fakeRecordingRecordReader{record: twoSessionRecord()}
		fetcher := &fakeRecordingFileFetcher{bodies: map[string]string{
			chatLinkA: "00:00:12\tFrom Alice Example to Everyone:\tfirst session\n",
			// chatLinkB has no body: the fetch fails.
		}}
		svc := NewPastMeetingChatService(reader, fetcher, 1024)

		resp, err := svc.GetPastMeetingChat(context.Background(), chatID, defaultWindow())
		assert.Nil(t, resp)
		assert.Equal(t, domain.ErrorTypeUnavailable, domain.GetErrorType(err))
		assert.Equal(t, "saved chat could not be retrieved; please retry", err.Error())
		assert.NotContains(t, err.Error(), "example.com")
	})

	t.Run("fetch error detail never reaches the client", func(t *testing.T) {
		reader := &fakeRecordingRecordReader{record: twoSessionRecord()}
		fetcher := &fakeRecordingFileFetcher{err: errors.New("Get \"" + chatLinkA + "\": dial tcp: lookup failed")}
		svc := NewPastMeetingChatService(reader, fetcher, 1024)

		_, err := svc.GetPastMeetingChat(context.Background(), chatID, defaultWindow())
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "secret-a")
		assert.NotContains(t, err.Error(), "http")
	})

	t.Run("byte cap marks the session truncated and drops the partial line", func(t *testing.T) {
		record := twoSessionRecord()
		record.RecordingFiles = record.RecordingFiles[3:]
		body := chatLines(3) // three complete lines
		cut := int64(len(body) - 5)
		reader := &fakeRecordingRecordReader{record: record}
		fetcher := &fakeRecordingFileFetcher{bodies: map[string]string{chatLinkA: body}}
		svc := NewPastMeetingChatService(reader, fetcher, cut)

		resp, err := svc.GetPastMeetingChat(context.Background(), chatID, defaultWindow())
		require.NoError(t, err)
		require.Len(t, resp.Sessions, 1)
		s := resp.Sessions[0]
		assert.True(t, s.Truncated)
		assert.Equal(t, 2, s.MessageCount)
		require.Len(t, s.Messages, 2)
		for _, m := range s.Messages {
			assert.True(t, strings.HasPrefix(m.Text, "message"), m.Text)
		}
	})

	t.Run("limit and offset window the messages of each session", func(t *testing.T) {
		record := twoSessionRecord()
		record.RecordingFiles = record.RecordingFiles[3:]
		reader := &fakeRecordingRecordReader{record: record}
		fetcher := &fakeRecordingFileFetcher{bodies: map[string]string{chatLinkA: chatLines(10)}}
		svc := NewPastMeetingChatService(reader, fetcher, 1<<20)

		t.Run("first page is truncated", func(t *testing.T) {
			resp, err := svc.GetPastMeetingChat(context.Background(), chatID, PastMeetingChatWindow{Limit: 4, Offset: 0})
			require.NoError(t, err)
			s := resp.Sessions[0]
			assert.Equal(t, 10, s.MessageCount)
			assert.True(t, s.Truncated)
			require.Len(t, s.Messages, 4)
			assert.Equal(t, "00:00:00", s.Messages[0].Offset)
			assert.Equal(t, "00:00:03", s.Messages[3].Offset)
		})

		t.Run("last page is not truncated", func(t *testing.T) {
			resp, err := svc.GetPastMeetingChat(context.Background(), chatID, PastMeetingChatWindow{Limit: 4, Offset: 8})
			require.NoError(t, err)
			s := resp.Sessions[0]
			assert.Equal(t, 10, s.MessageCount)
			assert.False(t, s.Truncated)
			require.Len(t, s.Messages, 2)
			assert.Equal(t, "00:00:08", s.Messages[0].Offset)
		})

		t.Run("offset beyond the count is an empty window", func(t *testing.T) {
			resp, err := svc.GetPastMeetingChat(context.Background(), chatID, PastMeetingChatWindow{Limit: 4, Offset: 50})
			require.NoError(t, err)
			s := resp.Sessions[0]
			assert.Equal(t, 10, s.MessageCount)
			assert.False(t, s.Truncated)
			assert.NotNil(t, s.Messages)
			assert.Empty(t, s.Messages)
		})
	})

	t.Run("window out of range is 400 before any read", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			window PastMeetingChatWindow
		}{
			{"limit 0", PastMeetingChatWindow{Limit: 0}},
			{"limit negative", PastMeetingChatWindow{Limit: -1}},
			{"limit above max", PastMeetingChatWindow{Limit: 2001}},
			{"offset negative", PastMeetingChatWindow{Limit: 10, Offset: -1}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				reader := &fakeRecordingRecordReader{record: twoSessionRecord()}
				svc := NewPastMeetingChatService(reader, &fakeRecordingFileFetcher{}, 1024)
				_, err := svc.GetPastMeetingChat(context.Background(), chatID, tc.window)
				assert.Equal(t, domain.ErrorTypeValidation, domain.GetErrorType(err))
				assert.Empty(t, reader.calls)
			})
		}
	})

	t.Run("limit at the maximum is accepted", func(t *testing.T) {
		record := twoSessionRecord()
		record.RecordingFiles = record.RecordingFiles[3:]
		reader := &fakeRecordingRecordReader{record: record}
		fetcher := &fakeRecordingFileFetcher{bodies: map[string]string{chatLinkA: chatLines(2)}}
		svc := NewPastMeetingChatService(reader, fetcher, 1024)
		_, err := svc.GetPastMeetingChat(context.Background(), chatID, PastMeetingChatWindow{Limit: 2000})
		require.NoError(t, err)
	})

	t.Run("malformed past meeting ID is 400 before any read", func(t *testing.T) {
		for _, id := range []string{"", "../etc", "123 456", "a#b", "x@y", "id\n", strings.Repeat("1", 129)} {
			reader := &fakeRecordingRecordReader{record: twoSessionRecord()}
			svc := NewPastMeetingChatService(reader, &fakeRecordingFileFetcher{}, 1024)
			_, err := svc.GetPastMeetingChat(context.Background(), id, defaultWindow())
			assert.Equal(t, domain.ErrorTypeValidation, domain.GetErrorType(err), "id %q", id)
			assert.Empty(t, reader.calls, "id %q", id)
		}
	})

	t.Run("both ID delimiters are accepted", func(t *testing.T) {
		for _, id := range []string{"93699735000:1700000000", "12343245463-1630560600000", "abc_DEF-123"} {
			record := twoSessionRecord()
			record.RecordingFiles = record.RecordingFiles[3:]
			reader := &fakeRecordingRecordReader{record: record}
			fetcher := &fakeRecordingFileFetcher{bodies: map[string]string{chatLinkA: ""}}
			svc := NewPastMeetingChatService(reader, fetcher, 1024)
			_, err := svc.GetPastMeetingChat(context.Background(), id, defaultWindow())
			require.NoError(t, err, id)
			assert.Equal(t, []string{id}, reader.calls)
		}
	})
}

// TestPastMeetingChatService_ResponseCarriesNoLinkOrSecret builds a response from a
// record whose file entries carry download_url and play_url, whose session carries
// share_url and a password-bearing link, and whose chat text mentions none of them,
// then marshals it and asserts that no link and no credential appears anywhere.
func TestPastMeetingChatService_ResponseCarriesNoLinkOrSecret(t *testing.T) {
	record := twoSessionRecord()
	reader := &fakeRecordingRecordReader{record: record}
	fetcher := &fakeRecordingFileFetcher{bodies: map[string]string{
		chatLinkA: "00:00:12\tFrom Alice Example to Everyone:\tagenda is in the invite\n",
		chatLinkB: "00:00:05\tFrom Bob Fixture to Everyone:\tthanks\n",
	}}
	svc := NewPastMeetingChatService(reader, fetcher, 1024)

	resp, err := svc.GetPastMeetingChat(context.Background(), chatID, defaultWindow())
	require.NoError(t, err)

	out, err := json.Marshal(resp)
	require.NoError(t, err)
	body := strings.ToLower(string(out))

	for _, forbidden := range []string{
		"http", "example.com", "token=", "secret-a", "secret-b", "hunter2",
		"download_url", "play_url", "share_url", "password", "passcode", "pwd",
		"file_size", "file-a-chat", "session-a", "host_",
	} {
		assert.NotContains(t, body, forbidden)
	}
	// Chat text itself is present, so the response is not trivially empty.
	assert.Contains(t, body, "agenda is in the invite")
}
