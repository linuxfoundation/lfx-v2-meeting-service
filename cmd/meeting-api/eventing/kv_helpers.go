// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
	"github.com/nats-io/nats.go/jetstream"
)

// lookupProjectFromMeeting fetches the proj_id and primary committee SFID of the parent active
// meeting from the v1-objects KV bucket. Returns ("","",nil) when the meeting record is not found
// in KV yet. When the meeting exists but has no proj_id, projSFID is empty but primaryCommitteeSFID
// may still be non-empty if the committee field is set. Callers that need to distinguish a missing
// meeting from a meeting with no project should perform a follow-up KV lookup. Returns a non-nil
// error for transient KV/decode failures (caller should retry).
func lookupProjectFromMeeting(
	ctx context.Context,
	meetingID string,
	v1ObjectsKV jetstream.KeyValue,
	logger *slog.Logger,
) (projSFID, primaryCommitteeSFID string, err error) {
	if meetingID == "" {
		return "", "", nil
	}
	meetingKey := fmt.Sprintf("itx-zoom-meetings-v2.%s", meetingID)
	entry, kvErr := v1ObjectsKV.Get(ctx, meetingKey)
	if kvErr != nil {
		if errors.Is(kvErr, jetstream.ErrKeyNotFound) {
			logger.WarnContext(ctx, "parent meeting not found in KV for project lookup", "key", meetingKey)
			return "", "", nil
		}
		return "", "", domain.NewUnavailableError("transient error fetching parent meeting", kvErr)
	}
	meetingData, decErr := decodeData(entry.Value())
	if decErr != nil {
		return "", "", domain.NewUnavailableError("transient error decoding parent meeting", decErr)
	}
	return utils.GetString(meetingData["proj_id"]), utils.GetString(meetingData["committee"]), nil
}

// lookupProjectFromPastMeeting fetches the proj_id, project_slug, and primary committee SFID of
// the parent past meeting from the v1-objects KV bucket. Returns empty strings (no error) when the
// record is not found — that is a permanent miss and the caller should not retry. Returns a non-nil
// error for transient KV fetch or decode failures (caller should retry).
func lookupProjectFromPastMeeting(
	ctx context.Context,
	meetingAndOccurrenceID string,
	v1ObjectsKV jetstream.KeyValue,
	logger *slog.Logger,
) (projSFID, projectSlug, primaryCommitteeSFID string, err error) {
	if meetingAndOccurrenceID == "" {
		return "", "", "", nil
	}
	pastMeetingKey := fmt.Sprintf("itx-zoom-past-meetings.%s", meetingAndOccurrenceID)
	entry, kvErr := v1ObjectsKV.Get(ctx, pastMeetingKey)
	if kvErr != nil {
		if errors.Is(kvErr, jetstream.ErrKeyNotFound) {
			logger.WarnContext(ctx, "parent past_meeting not found for project lookup", "key", pastMeetingKey)
			return "", "", "", nil
		}
		return "", "", "", domain.NewUnavailableError("transient error fetching parent past_meeting", kvErr)
	}
	pastMeetingData, decErr := decodeData(entry.Value())
	if decErr != nil {
		return "", "", "", domain.NewUnavailableError("transient error decoding parent past_meeting", decErr)
	}
	return utils.GetString(pastMeetingData["proj_id"]),
		utils.GetString(pastMeetingData["project_slug"]),
		utils.GetString(pastMeetingData["committee"]),
		nil
}

// hasRecordingForPastMeeting reports whether a playable recording exists for the given occurrence.
// Parity with the self-serve card: the card only ever surfaces the largest session (by total_size)
// share_url, so we check that same session — counting a smaller-session-only URL would over-count
// recordings the UI never exposes. A missing recording object is a permanent miss and returns
// (false, nil); transient KV fetch or decode failures return a non-nil error (caller should retry).
func hasRecordingForPastMeeting(
	ctx context.Context,
	meetingAndOccurrenceID string,
	v1ObjectsKV jetstream.KeyValue,
	logger *slog.Logger,
) (bool, error) {
	if meetingAndOccurrenceID == "" {
		return false, nil
	}
	recordingKey := fmt.Sprintf("itx-zoom-past-meetings-recordings.%s", meetingAndOccurrenceID)
	entry, kvErr := v1ObjectsKV.Get(ctx, recordingKey)
	if kvErr != nil {
		if errors.Is(kvErr, jetstream.ErrKeyNotFound) {
			logger.DebugContext(ctx, "no recording object for past meeting", "key", recordingKey)
			return false, nil
		}
		return false, domain.NewUnavailableError("transient error fetching recording", kvErr)
	}
	data, decErr := decodeData(entry.Value())
	if decErr != nil {
		return false, domain.NewUnavailableError("transient error decoding recording", decErr)
	}
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return false, domain.NewUnavailableError("transient error re-encoding recording", err)
	}
	var rec RecordingDBRaw
	if err := json.Unmarshal(jsonBytes, &rec); err != nil {
		return false, domain.NewUnavailableError("transient error decoding recording", err)
	}
	if len(rec.Sessions) == 0 {
		return false, nil
	}
	// Mirror the frontend's getLargestSessionShareUrl reduce: seed with the first session, keep the
	// strictly-larger one on ties (first wins), then report whether that session exposes a share_url.
	largest := rec.Sessions[0]
	for _, s := range rec.Sessions[1:] {
		if s.TotalSize > largest.TotalSize {
			largest = s
		}
	}
	return largest.ShareURL != "", nil
}
