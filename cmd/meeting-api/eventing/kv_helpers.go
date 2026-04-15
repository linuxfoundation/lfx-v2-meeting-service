// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
	"github.com/nats-io/nats.go/jetstream"
)

// lookupProjectFromPastMeeting fetches the proj_id and project_slug of the parent past meeting
// from the v1-objects KV bucket. Returns empty strings (no error) when the record is not found —
// that is a permanent miss and the caller should not retry. Returns a non-nil error for transient
// KV fetch failures or decode failures (caller should retry).
func lookupProjectFromPastMeeting(
	ctx context.Context,
	meetingAndOccurrenceID string,
	v1ObjectsKV jetstream.KeyValue,
	logger *slog.Logger,
) (projSFID, projectSlug string, err error) {
	if meetingAndOccurrenceID == "" {
		return "", "", nil
	}
	pastMeetingKey := fmt.Sprintf("itx-zoom-past-meetings.%s", meetingAndOccurrenceID)
	entry, kvErr := v1ObjectsKV.Get(ctx, pastMeetingKey)
	if kvErr != nil {
		if errors.Is(kvErr, jetstream.ErrKeyNotFound) {
			logger.WarnContext(ctx, "parent past_meeting not found for project lookup", "key", pastMeetingKey)
			return "", "", nil
		}
		return "", "", fmt.Errorf("transient error fetching parent past_meeting: %w", kvErr)
	}
	pastMeetingData, decErr := decodeData(entry.Value())
	if decErr != nil {
		return "", "", fmt.Errorf("transient error decoding parent past_meeting: %w", decErr)
	}
	return utils.GetString(pastMeetingData["proj_id"]), utils.GetString(pastMeetingData["project_slug"]), nil
}
