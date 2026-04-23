// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
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
