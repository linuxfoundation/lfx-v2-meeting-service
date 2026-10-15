// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/vmihailenco/msgpack/v5"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/v1objects"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/msgpackutil"
)

// recordingRecordKeyPrefix is the v1-objects key prefix of past meeting recording
// records; the event pipeline watches the same prefix.
const recordingRecordKeyPrefix = "itx-zoom-past-meetings-recordings."

// KeyValueGetter is the subset of jetstream.KeyValue the recording record reader uses.
type KeyValueGetter interface {
	Get(ctx context.Context, key string) (jetstream.KeyValueEntry, error)
}

// RecordingRecordReader implements domain.RecordingRecordReader over the v1-objects
// JetStream KV bucket. It decodes the record exactly as the event pipeline does
// (JSON first, then msgpack behind the nesting-depth guard, into the shared
// v1objects.Recording shape) and reduces it to models.RecordingRecord.
type RecordingRecordReader struct {
	kv     KeyValueGetter
	logger *slog.Logger
}

// Verify interface compliance at compile time.
var _ domain.RecordingRecordReader = (*RecordingRecordReader)(nil)

// NewRecordingRecordReader creates a RecordingRecordReader over kv.
func NewRecordingRecordReader(kv KeyValueGetter, logger *slog.Logger) *RecordingRecordReader {
	return &RecordingRecordReader{kv: kv, logger: logger}
}

// GetRecordingRecord reads itx-zoom-past-meetings-recordings.{meetingAndOccurrenceID}.
// Absence (domain.ErrRecordingRecordNotFound) is jetstream.ErrKeyNotFound, or
// jetstream.ErrInvalidKey for an ID that cannot be a KV key at all (NATS keys allow
// only [-/_=.A-Za-z0-9], so such a record cannot exist; the event pipeline's
// isKVAbsenceError makes the same call). Every other read failure is reported as
// unavailable, and a record that does not decode as internal, so that callers never
// mistake a failure for "no recording".
func (r *RecordingRecordReader) GetRecordingRecord(ctx context.Context, meetingAndOccurrenceID string) (*models.RecordingRecord, error) {
	entry, err := r.kv.Get(ctx, recordingRecordKeyPrefix+meetingAndOccurrenceID)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) || errors.Is(err, jetstream.ErrInvalidKey) {
			return nil, domain.ErrRecordingRecordNotFound
		}
		r.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to read recording record from KV",
			"meeting_and_occurrence_id", meetingAndOccurrenceID)
		return nil, domain.NewUnavailableError("recording record could not be read")
	}

	raw, deleted, err := decodeRecordingRecord(entry.Value())
	if err != nil {
		// The decoder error is deliberately not logged, at any level: strconv and
		// json errors embed the rejected value, that is, record contents.
		r.logger.ErrorContext(ctx, "failed to decode recording record",
			"meeting_and_occurrence_id", meetingAndOccurrenceID)
		return nil, domain.NewInternalError("saved chat record could not be decoded")
	}
	if deleted {
		// A soft-deleted record (_sdc_deleted_at set, the event pipeline's rule) is
		// gone for every reader of it.
		return nil, domain.ErrRecordingRecordNotFound
	}

	record := &models.RecordingRecord{
		MeetingAndOccurrenceID: raw.MeetingAndOccurrenceID,
		RecordingAccess:        raw.RecordingAccess,
	}
	for _, f := range raw.RecordingFiles {
		// A timestamp that does not parse is left at its zero value, as on the indexer path.
		start, _ := v1objects.ParseTime(f.RecordingStart)
		end, _ := v1objects.ParseTime(f.RecordingEnd)
		record.RecordingFiles = append(record.RecordingFiles, models.RecordingFile{
			DownloadURL:    f.DownloadURL,
			FileExtension:  f.FileExtension,
			FileSize:       f.FileSize,
			FileType:       f.FileType,
			ID:             f.ID,
			MeetingID:      f.MeetingID,
			PlayURL:        f.PlayURL,
			RecordingStart: start,
			RecordingEnd:   end,
			RecordingType:  f.RecordingType,
			Status:         f.Status,
		})
	}
	for _, s := range raw.Sessions {
		start, _ := v1objects.ParseTime(s.StartTime)
		record.Sessions = append(record.Sessions, models.RecordingSession{
			UUID:      s.UUID,
			ShareURL:  s.ShareURL,
			TotalSize: s.TotalSize,
			StartTime: start,
		})
	}
	return record, nil
}

// decodeRecordingRecord decodes a KV value as JSON or, failing that, as msgpack behind
// the depth guard, reports whether the record is soft-deleted, and otherwise decodes it
// into the shared raw recording shape by way of JSON so that the numeric coercions of
// v1objects.Recording apply to both encodings, as in the event pipeline's decodeV1.
// The returned error may embed record contents and must not be logged.
func decodeRecordingRecord(value []byte) (raw *v1objects.Recording, deleted bool, err error) {
	var data map[string]any
	if jsonErr := json.Unmarshal(value, &data); jsonErr != nil {
		if depthErr := msgpackutil.CheckNestingDepth(value); depthErr != nil {
			return nil, false, depthErr
		}
		if err := msgpack.Unmarshal(value, &data); err != nil {
			return nil, false, jsonErr
		}
	}
	if _, deleted := v1objects.SoftDeletedAt(data); deleted {
		return nil, true, nil
	}
	normalized, err := json.Marshal(data)
	if err != nil {
		return nil, false, err
	}
	var rec v1objects.Recording
	if err := json.Unmarshal(normalized, &rec); err != nil {
		return nil, false, err
	}
	return &rec, false, nil
}
