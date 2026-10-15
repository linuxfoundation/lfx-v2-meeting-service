// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

// fakeKV serves one value per key or an error.
type fakeKV struct {
	values map[string][]byte
	err    error
	keys   []string
}

func (f *fakeKV) Get(_ context.Context, key string) (jetstream.KeyValueEntry, error) {
	f.keys = append(f.keys, key)
	if f.err != nil {
		return nil, f.err
	}
	v, ok := f.values[key]
	if !ok {
		return nil, jetstream.ErrKeyNotFound
	}
	return fakeKVEntry{key: key, value: v}, nil
}

type fakeKVEntry struct {
	key   string
	value []byte
}

func (e fakeKVEntry) Bucket() string                  { return "v1-objects" }
func (e fakeKVEntry) Key() string                     { return e.key }
func (e fakeKVEntry) Value() []byte                   { return e.value }
func (e fakeKVEntry) Revision() uint64                { return 1 }
func (e fakeKVEntry) Created() time.Time              { return time.Time{} }
func (e fakeKVEntry) Delta() uint64                   { return 0 }
func (e fakeKVEntry) Operation() jetstream.KeyValueOp { return jetstream.KeyValuePut }

const recordingJSON = `{
  "meeting_and_occurrence_id": "96365872728-1630560600000",
  "proj_id": "a0A000000000001",
  "recording_access": "meeting_participants",
  "recording_count": "3",
  "total_size": "12345",
  "recording_files": [
    {"file_type": "MP4", "download_url": "https://recordings.example.com/v?token=v", "file_size": "12000",
     "recording_start": "2026-10-01T15:00:00Z", "recording_end": "2026-10-01T16:02:11Z", "id": "f1"},
    {"file_type": "CHAT", "download_url": "https://recordings.example.com/c?token=c", "file_size": 345,
     "recording_start": "2026-10-01 15:00:00", "recording_end": "not a time", "id": "f2"}
  ],
  "sessions": [
    {"uuid": "s1", "share_url": "https://recordings.example.com/share/s1", "password": "hunter2",
     "total_size": "12345", "start_time": "2026-10-01T15:00:00Z"}
  ]
}`

func TestRecordingRecordReader_GetRecordingRecord(t *testing.T) {
	const id = "96365872728-1630560600000"
	logger := slog.New(slog.DiscardHandler)

	t.Run("decodes a JSON record under the recording key prefix", func(t *testing.T) {
		kv := &fakeKV{values: map[string][]byte{"itx-zoom-past-meetings-recordings." + id: []byte(recordingJSON)}}
		r := NewRecordingRecordReader(kv, logger)

		rec, err := r.GetRecordingRecord(context.Background(), id)
		require.NoError(t, err)
		assert.Equal(t, []string{"itx-zoom-past-meetings-recordings." + id}, kv.keys)
		assert.Equal(t, id, rec.MeetingAndOccurrenceID)
		assert.Equal(t, "meeting_participants", rec.RecordingAccess)
		require.Len(t, rec.RecordingFiles, 2)

		chat := rec.RecordingFiles[1]
		assert.Equal(t, "CHAT", chat.FileType)
		assert.Equal(t, "https://recordings.example.com/c?token=c", chat.DownloadURL)
		assert.Equal(t, int64(345), chat.FileSize)
		// Space-separated timestamps parse (as on the indexer path); garbage is zero.
		assert.Equal(t, time.Date(2026, 10, 1, 15, 0, 0, 0, time.UTC), chat.RecordingStart)
		assert.True(t, chat.RecordingEnd.IsZero())
		// String-encoded numbers are coerced.
		assert.Equal(t, int64(12000), rec.RecordingFiles[0].FileSize)

		require.Len(t, rec.Sessions, 1)
		assert.Equal(t, "https://recordings.example.com/share/s1", rec.Sessions[0].ShareURL)
		assert.Equal(t, int64(12345), rec.Sessions[0].TotalSize)
	})

	t.Run("decodes a msgpack record the same way", func(t *testing.T) {
		var asMap map[string]any
		require.NoError(t, json.Unmarshal([]byte(recordingJSON), &asMap))
		packed, err := msgpack.Marshal(asMap)
		require.NoError(t, err)
		kv := &fakeKV{values: map[string][]byte{"itx-zoom-past-meetings-recordings." + id: packed}}
		r := NewRecordingRecordReader(kv, logger)

		rec, err := r.GetRecordingRecord(context.Background(), id)
		require.NoError(t, err)
		require.Len(t, rec.RecordingFiles, 2)
		assert.Equal(t, "CHAT", rec.RecordingFiles[1].FileType)
		assert.Equal(t, int64(345), rec.RecordingFiles[1].FileSize)
	})

	t.Run("missing key is ErrRecordingRecordNotFound", func(t *testing.T) {
		r := NewRecordingRecordReader(&fakeKV{values: map[string][]byte{}}, logger)
		_, err := r.GetRecordingRecord(context.Background(), id)
		assert.ErrorIs(t, err, domain.ErrRecordingRecordNotFound)
	})

	t.Run("an ID that cannot be a KV key is not found", func(t *testing.T) {
		r := NewRecordingRecordReader(&fakeKV{err: jetstream.ErrInvalidKey}, logger)
		_, err := r.GetRecordingRecord(context.Background(), "93699735000:1700000000")
		assert.ErrorIs(t, err, domain.ErrRecordingRecordNotFound)
	})

	t.Run("any other KV error is unavailable, never not found", func(t *testing.T) {
		r := NewRecordingRecordReader(&fakeKV{err: errors.New("nats: timeout")}, logger)
		_, err := r.GetRecordingRecord(context.Background(), id)
		require.Error(t, err)
		assert.NotErrorIs(t, err, domain.ErrRecordingRecordNotFound)
		assert.Equal(t, domain.ErrorTypeUnavailable, domain.GetErrorType(err))
	})

	t.Run("a record that does not decode is internal with a fixed message", func(t *testing.T) {
		kv := &fakeKV{values: map[string][]byte{"itx-zoom-past-meetings-recordings." + id: []byte("\xc1 not json nor msgpack")}}
		r := NewRecordingRecordReader(kv, logger)
		_, err := r.GetRecordingRecord(context.Background(), id)
		require.Error(t, err)
		assert.Equal(t, domain.ErrorTypeInternal, domain.GetErrorType(err))
		assert.Equal(t, "saved chat record could not be decoded", err.Error())
	})

	t.Run("a record with a non-numeric file_size is internal", func(t *testing.T) {
		kv := &fakeKV{values: map[string][]byte{"itx-zoom-past-meetings-recordings." + id: []byte(`{"recording_files":[{"file_size":"many"}]}`)}}
		r := NewRecordingRecordReader(kv, logger)
		_, err := r.GetRecordingRecord(context.Background(), id)
		assert.Equal(t, domain.ErrorTypeInternal, domain.GetErrorType(err))
	})

	t.Run("null recording_files decodes to no files", func(t *testing.T) {
		kv := &fakeKV{values: map[string][]byte{"itx-zoom-past-meetings-recordings." + id: []byte(`{"meeting_and_occurrence_id":"` + id + `","recording_files":null}`)}}
		r := NewRecordingRecordReader(kv, logger)
		rec, err := r.GetRecordingRecord(context.Background(), id)
		require.NoError(t, err)
		assert.Empty(t, rec.RecordingFiles)
	})
}
