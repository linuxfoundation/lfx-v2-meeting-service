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

// defaultArtifactAccess is the fail-closed default for recording_access and transcript_access
// when the parent past meeting record does not carry a value. "meeting_hosts" is the most
// restrictive option and prevents inadvertent public exposure.
const defaultArtifactAccess = "meeting_hosts"

// lookupProjectFromMeeting fetches the proj_id and primary committee SFID of the parent active
// meeting from the v1-objects KV bucket. Returns ("","",nil) when the meeting record is not found
// in KV yet. When the meeting exists but has no proj_id, projSFID is empty but primaryCommitteeSFID
// may still be non-empty if the committee field is set. Callers that need to distinguish a missing
// meeting from a meeting with no project should perform a follow-up KV lookup.
//
// Error semantics: returns a non-nil error for KV or decode failures. Callers MUST check
// errors.Is(err, errMsgpackStructural) first: structural decode errors are permanent (ACK, skip)
// and must not be retried. All other non-nil errors are transient (NAK for retry).
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
		if errors.Is(decErr, errMsgpackStructural) {
			return "", "", fmt.Errorf("permanent decode failure for parent meeting %s: %w", meetingID, decErr)
		}
		return "", "", domain.NewUnavailableError("transient error decoding parent meeting", decErr)
	}
	return utils.GetString(meetingData["proj_id"]), utils.GetString(meetingData["committee"]), nil
}

// parentPastMeetingInfo holds fields fetched from the parent past meeting record for use by
// child-record handlers (recording, attachment, participant).
type parentPastMeetingInfo struct {
	ProjectSFID          string
	ProjectSlug          string
	PrimaryCommitteeSFID string
	RecordingAccess      string
	TranscriptAccess     string
}

// lookupParentPastMeeting fetches all key fields from the parent past meeting record in the
// v1-objects KV bucket, including the authoritative recording_access and transcript_access values.
// Use this when the caller needs access settings (e.g. the recording handler). Use
// lookupProjectFromPastMeeting when only project/committee fields are needed.
//
// Returns a zero-value struct (no error) when the record is not found — that is a permanent miss
// and the caller should not retry.
//
// Error semantics: returns a non-nil error for KV or decode failures. Callers MUST check
// errors.Is(err, errMsgpackStructural) first: structural decode errors are permanent (ACK, skip)
// and must not be retried. All other non-nil errors are transient (NAK for retry).
func lookupParentPastMeeting(
	ctx context.Context,
	meetingAndOccurrenceID string,
	v1ObjectsKV jetstream.KeyValue,
	logger *slog.Logger,
) (parentPastMeetingInfo, error) {
	if meetingAndOccurrenceID == "" {
		return parentPastMeetingInfo{}, nil
	}
	pastMeetingKey := fmt.Sprintf("itx-zoom-past-meetings.%s", meetingAndOccurrenceID)
	entry, kvErr := v1ObjectsKV.Get(ctx, pastMeetingKey)
	if kvErr != nil {
		if errors.Is(kvErr, jetstream.ErrKeyNotFound) {
			logger.WarnContext(ctx, "parent past_meeting not found", "key", pastMeetingKey)
			return parentPastMeetingInfo{}, nil
		}
		return parentPastMeetingInfo{}, domain.NewUnavailableError("transient error fetching parent past_meeting", kvErr)
	}
	pastMeetingData, decErr := decodeData(entry.Value())
	if decErr != nil {
		if errors.Is(decErr, errMsgpackStructural) {
			return parentPastMeetingInfo{}, fmt.Errorf("permanent decode failure for parent past_meeting %s: %w", meetingAndOccurrenceID, decErr)
		}
		return parentPastMeetingInfo{}, domain.NewUnavailableError("transient error decoding parent past_meeting", decErr)
	}
	return parentPastMeetingInfo{
		ProjectSFID:          utils.GetString(pastMeetingData["proj_id"]),
		ProjectSlug:          utils.GetString(pastMeetingData["project_slug"]),
		PrimaryCommitteeSFID: utils.GetString(pastMeetingData["committee"]),
		RecordingAccess:      utils.GetString(pastMeetingData["recording_access"]),
		TranscriptAccess:     utils.GetString(pastMeetingData["transcript_access"]),
	}, nil
}

// lookupProjectFromPastMeeting fetches the project and primary committee SFID from the parent past
// meeting record. Use this when access settings are not needed; use lookupParentPastMeeting when
// they are.
//
// Returns empty strings (no error) when the record is not found — that is a permanent miss and the
// caller should not retry.
//
// Error semantics: same as lookupParentPastMeeting.
func lookupProjectFromPastMeeting(
	ctx context.Context,
	meetingAndOccurrenceID string,
	v1ObjectsKV jetstream.KeyValue,
	logger *slog.Logger,
) (projSFID, projectSlug, primaryCommitteeSFID string, err error) {
	info, err := lookupParentPastMeeting(ctx, meetingAndOccurrenceID, v1ObjectsKV, logger)
	return info.ProjectSFID, info.ProjectSlug, info.PrimaryCommitteeSFID, err
}
