// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	fgaconstants "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/constants"
	indexerConstants "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/eventing"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	itx "github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
	"github.com/nats-io/nats.go/jetstream"
)

// =============================================================================
// Past Meeting Event Handler
// =============================================================================

// pastMeetingDBRaw represents raw past meeting data from v1 DynamoDB/NATS KV bucket
type pastMeetingDBRaw struct {
	// MeetingAndOccurrenceID is the partition key of the past meeting table
	MeetingAndOccurrenceID string `json:"meeting_and_occurrence_id"`

	// ProjectID is the ID of the salesforce (v1) project associated with the past meeting
	ProjectID string `json:"proj_id"`

	// ProjectSlug is the slug of the project associated with the past meeting
	ProjectSlug string `json:"project_slug"`

	// Committee is the ID of the committee associated with the past meeting
	Committee string `json:"committee"`

	// CommitteeFilters is the list of filters associated with the committee
	CommitteeFilters []string `json:"committee_filters"`

	// Topic is the title/topic of the past meeting (v1 field name)
	Topic string `json:"topic"`

	// Agenda is the description/agenda of the past meeting (v1 field name)
	Agenda string `json:"agenda"`

	// Duration is the duration of the past meeting
	Duration int `json:"duration"`

	// MeetingID is the ID of the meeting associated with the past meeting
	MeetingID string `json:"meeting_id"`

	// OccurrenceID is the ID of the occurrence associated with the past meeting
	OccurrenceID string `json:"occurrence_id"`

	// RecordingAccess is the access type of the recording of the past meeting
	RecordingAccess string `json:"recording_access"`

	// RecordingEnabled is whether the recording of the past meeting is enabled
	RecordingEnabled bool `json:"recording_enabled"`

	// ScheduledStartTime is the scheduled start time of the past meeting.
	// This differs from the actual start time of the meeting because the [Sessions] stores
	// the actual start and end times of the meeting from Zoom of when it officially started.
	ScheduledStartTime string `json:"scheduled_start_time"`

	// ScheduledEndTime is the scheduled end time of the past meeting
	// This differs from the actual end time of the meeting because the [Sessions] stores
	// the actual start and end times of the meeting from Zoom of when it officially ended.
	ScheduledEndTime string `json:"scheduled_end_time"`

	// Sessions is the list of sessions associated with the past meeting
	Sessions []ZoomPastMeetingSession `json:"sessions"`

	// Timezone is the timezone of the past meeting
	Timezone string `json:"timezone"`

	// MeetingType is the type of the past meeting
	MeetingType string `json:"meeting_type"`

	// TranscriptAccess is the access type of the transcript of the past meeting
	TranscriptAccess string `json:"transcript_access"`

	// TranscriptEnabled is whether the transcript of the past meeting is enabled
	TranscriptEnabled bool `json:"transcript_enabled"`

	// Type is the type of the past meeting
	Type int `json:"type"`

	// Visibility is the visibility of the past meeting
	Visibility string `json:"visibility"`

	// Recurrence is the recurrence of the past meeting
	Recurrence *recurrenceDBRaw `json:"recurrence"`

	// Restricted is whether the past meeting is restricted to only invited participants
	Restricted bool `json:"restricted"`

	// IsManuallyCreated indicates whether the past meeting was created manually
	IsManuallyCreated bool `json:"is_manually_created"`

	// MeetingPassword is the ITX-generated UUID password from the active meeting record,
	// used to secure the past meeting join page URL.
	MeetingPassword string `json:"meeting_password"`

	// ZoomAIEnabled is whether the meeting was hosted on a zoom user with AI-companion enabled
	ZoomAIEnabled *bool `json:"zoom_ai_enabled,omitempty"`

	// AISummaryAccess is the access level of the meeting AI summary within the LFX platform.
	AISummaryAccess string `json:"ai_summary_access,omitempty"`

	// RequireAISummaryApproval is whether the meeting requires approval of the AI summary
	RequireAISummaryApproval *bool `json:"require_ai_summary_approval,omitempty"`

	// EarlyJoinTime is the number of minutes before the scheduled start time that participants can join the meeting
	EarlyJoinTime int `json:"early_join_time"`

	// Artifacts is the list of artifacts for the past meeting
	Artifacts []ZoomPastMeetingArtifact `json:"artifacts"`

	// YoutubeLink is the link to the YouTube video of the past meeting
	YoutubeLink string `json:"youtube_link,omitempty"`

	// CreatedAt is the creation time of the past meeting
	CreatedAt string `json:"created_at"`

	// ModifiedAt is the last modification time in RFC3339 format of the past meeting
	ModifiedAt string `json:"modified_at"`

	// CreatedBy is the user who created the past meeting
	CreatedBy models.CreatedBy `json:"created_by"`

	// UpdatedBy is the user who last updated the past meeting
	UpdatedBy models.UpdatedBy `json:"updated_by"`

	// UpdatedByList is the list of users who have updated the past meeting
	UpdatedByList []models.UpdatedBy `json:"updated_by_list"`
}

// UnmarshalJSON implements custom unmarshaling to handle both string and int inputs for numeric fields.
func (p *pastMeetingDBRaw) UnmarshalJSON(data []byte) error {
	type Alias pastMeetingDBRaw
	tmp := struct {
		Duration      interface{} `json:"duration"`
		EarlyJoinTime interface{} `json:"early_join_time"`
		Type          interface{} `json:"type"`
		*Alias
	}{
		Alias: (*Alias)(p),
	}

	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	if err := coerceInt(&p.Duration, tmp.Duration, "duration"); err != nil {
		return err
	}
	if err := coerceInt(&p.EarlyJoinTime, tmp.EarlyJoinTime, "early_join_time"); err != nil {
		return err
	}
	if err := coerceInt(&p.Type, tmp.Type, "type"); err != nil {
		return err
	}

	return nil
}

// ZoomPastMeetingSession represents a single meeting instance/session
// A meeting being started then ended is one session, then restarting it is a second session.
type ZoomPastMeetingSession struct {
	// UUID is the UUID of the session.
	// This comes from Zoom when the meeting is started and ended. It is unique to each time
	// that the meeting is run, so if the same meeting is restarted then it will have a different UUID.
	UUID string `json:"uuid"`

	// StartTime is the start time of the session in RFC3339 format
	StartTime string `json:"start_time"`

	// EndTime is the end time of the session in RFC3339 format
	EndTime string `json:"end_time"`
}

// ZoomPastMeetingArtifact represents a a meeting artifact.
// An artifact is a link to a url where some information about the meeting can be found.
// For example a spreadsheet for meeting minutes or a link to an agenda can be represented
// by this artifact model.
type ZoomPastMeetingArtifact struct {
	// ID is the UUID of the artifact record.
	ID string `json:"id"`

	// Category is the category of the artifact.
	Category string `json:"category"`

	// Link is the link to the artifact.
	Link string `json:"link"`

	// Name is the name of the artifact.
	Name string `json:"name"`

	// CreatedAt is the creation time of the artifact in RFC3339 format.
	CreatedAt string `json:"created_at"`

	// CreatedBy is the user who created the artifact.
	CreatedBy models.CreatedBy `json:"created_by"`

	// UpdatedAt is the last modification time of the artifact in RFC3339 format.
	UpdatedAt string `json:"updated_at"`

	// UpdatedBy is the user who last updated the artifact.
	UpdatedBy models.UpdatedBy `json:"updated_by"`
}

// handlePastMeetingUpdate processes updates under one outer parent owner.
func (h *EventHandlers) handlePastMeetingUpdate(
	ctx context.Context, key string, v1Data map[string]interface{},
) bool {
	if _, deleted := softDeletedAt(v1Data); deleted {
		return false // Soft-delete routing owns the actual mutation.
	}
	return h.withParentOperation(ctx, extractIDFromKey(key, pastMeetingObjectKeyPrefix), func(owned context.Context) bool {
		return h.handlePastMeetingUpdateOwned(owned, key, v1Data)
	})
}

func (h *EventHandlers) handlePastMeetingUpdateOwned(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) (retry bool) {
	funcLogger := h.logger.With("key", key, "handler", "past_meeting")
	funcLogger.DebugContext(ctx, "processing past meeting update")

	// Only a live record may be published, mark the past meeting mapping live, or
	// republish its artifacts. handleKVPut already routes soft deletes elsewhere; this
	// also covers the retrigger path.
	if _, deleted := softDeletedAt(v1Data); deleted {
		funcLogger.InfoContext(ctx, "skipping past meeting update for soft-deleted record")
		return false
	}
	parentID := extractIDFromKey(key, pastMeetingObjectKeyPrefix)
	state, authoritative, err := h.loadV1Parent(ctx, key, "v1_past_meetings."+parentID)
	if err != nil {
		return true
	}
	if state == parentGone || state == parentAbsent {
		return h.handlePastMeetingDelete(ctx, key, nil)
	}
	if state != parentLive {
		return false
	}
	v1Data = authoritative
	if err := h.invalidatePastMeetingParticipantDiscovery(ctx, parentID); err != nil {
		return true
	}

	// Convert v1Data to past meeting event data
	pastMeetingData, err := convertMapToPastMeetingData(ctx, v1Data, h.idMapper, h.v1ObjectsKV, h.v1MappingsKV, funcLogger)
	if err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to convert v1Data to past meeting")
		return isTransientError(err)
	}

	// Validate required fields
	if pastMeetingData.ID == "" {
		funcLogger.ErrorContext(ctx, "missing required fields in past meeting data")
		return false
	}
	if pastMeetingData.ProjectUID == "" {
		funcLogger.InfoContext(ctx, "skipping past meeting sync - parent project not found in mappings", "project_sfid", pastMeetingData.ProjectSFID)
		// The record is live, so a tombstone from an earlier delete is stale; clear it so a
		// later mapping retrigger can index the past meeting once its project is mapped.
		if err := h.clearTombstone(ctx, fmt.Sprintf("v1_past_meetings.%s", pastMeetingData.ID)); err != nil {
			funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to clear stale past meeting tombstone, will retry")
			return true
		}
		return false
	}
	funcLogger = funcLogger.With("past_meeting_id", pastMeetingData.ID)
	funcLogger.InfoContext(ctx, "processing past meeting update")
	baseCtx := ctx
	replay := &pastMeetingReplayState{parentID: pastMeetingData.ID, ready: make(map[string]uint64)}
	defer func() {
		retry = h.finishPastMeetingReplay(baseCtx, key, replay, retry)
	}()

	// Determine action (created vs updated)
	mappingKey := fmt.Sprintf("v1_past_meetings.%s", pastMeetingData.ID)
	indexerAction := indexerConstants.ActionCreated
	restored := false
	if entry, err := h.v1MappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
		restored = entryIsTombstoned(entry)
	} else if !isKVAbsenceError(err) {
		funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to load past meeting mapping, will retry")
		return true
	}
	if !restored {
		// A failed restore may already have written the live parent mapping.
		// Keep retrying its children until both restore markers are cleared.
		pending, err := h.liveMarkerSet(ctx, pastMeetingRestorePendingPrefix+pastMeetingData.ID)
		if err != nil {
			funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to check past meeting restore-pending marker, will retry")
			return true
		}
		cleaned, err := h.liveMarkerSet(ctx, pastMeetingCleanupKeyPrefix+pastMeetingData.ID)
		if err != nil {
			funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to check past meeting restore marker, will retry")
			return true
		}
		restored = pending || cleaned
	}
	needsChildReplay := restored || indexerAction == indexerConstants.ActionCreated
	if restored {
		entry, err := h.v1MappingsKV.Get(ctx, pastMeetingCleanupKeyPrefix+pastMeetingData.ID)
		if err == nil {
			replay.cleanupRevision = entry.Revision()
		} else if !isKVAbsenceError(err) {
			return true
		}
	}
	if needsChildReplay {
		// Persist first-create or restore intent before writing the live parent
		// mapping. A child failure must still be discoverable on redelivery.
		revision, err := h.v1MappingsKV.Put(ctx, pastMeetingRestorePendingPrefix+pastMeetingData.ID, []byte("1"))
		if err != nil {
			funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to mark past meeting child replay pending, will retry")
			return true
		}
		replay.pendingRevision = revision
	}

	// Publish to indexer and FGA-sync
	if err := h.publisher.PublishPastMeetingEvent(ctx, string(indexerAction), pastMeetingData); err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to publish past meeting event")
		return true
	}

	// Store mapping. On a restore this replaces the delete tombstone; if the write fails
	// the tombstone would keep later mapping retriggers from re-indexing the past meeting, so
	// retry. An invalid key can never be written, so it is not retried.
	retry = false
	if _, err := h.v1MappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		if !errors.Is(err, jetstream.ErrInvalidKey) {
			funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to store past meeting mapping, will retry")
			retry = true
		} else {
			funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to store past meeting mapping")
		}
	}

	// The recording, transcript and summary documents take their indexer public flag from this
	// record's recording_access, transcript_access and ai_summary_access, but their own KV
	// records do not change when those settings do. Republish them so that tightening a
	// setting revokes public visibility in the index, not only in FGA. This runs even when the
	// marker write failed, so a retry never delays revoking public visibility.
	ctx = context.WithValue(ctx, pastMeetingReplayContextKey{}, replay)
	if h.republishPastMeetingArtifacts(ctx, pastMeetingData.MeetingAndOccurrenceID, funcLogger) {
		retry = true
	}
	if retry {
		return true
	}
	if needsChildReplay {
		if h.republishPastMeetingChildren(ctx, pastMeetingData.ID, funcLogger) {
			return true
		}
	}
	funcLogger.InfoContext(ctx, "successfully processed past meeting", "action", string(indexerAction))
	return false // Success, ACK
}

// finishPastMeetingReplay runs outside child recursion suppression, even after
// a replay failure. A newer authoritative deletion supersedes stale live data.
// Only child revisions actually reconciled by this replay can be completed.
func (h *EventHandlers) finishPastMeetingReplay(ctx context.Context, key string, replay *pastMeetingReplayState, retry bool) bool {
	if replay.pendingRevision == 0 && (retry || len(replay.ready) != 0) {
		// Ordinary updates can also create child work. Persist that obligation
		// before draining, so a drain read/claim failure survives redelivery.
		revision, err := h.v1MappingsKV.Put(ctx, pastMeetingRestorePendingPrefix+replay.parentID, []byte("1"))
		if err != nil {
			return true
		}
		replay.pendingRevision = revision
	}
	state, _, err := h.loadV1Parent(ctx, key, "v1_past_meetings."+replay.parentID)
	if err != nil {
		return true
	}
	wasDeleted := state == parentGone || state == parentAbsent
	if wasDeleted {
		retry = h.publishPastMeetingDelete(ctx, key, replay.parentID)
	}
	if retry {
		return true
	}
	if h.drainPastMeetingChildRepairs(ctx, replay) {
		return true
	}
	after, _, err := h.loadV1Parent(ctx, key, "v1_past_meetings."+replay.parentID)
	if err != nil {
		return true
	}
	if after == parentGone || after == parentAbsent {
		if wasDeleted {
			return false // Preserve deletion/restore markers for a future restore.
		}
		return h.publishPastMeetingDelete(ctx, key, replay.parentID)
	}
	if after != parentLive {
		return true
	}
	return h.finalizePastMeetingReplayMarkers(ctx, replay)
}

// finalizePastMeetingReplayMarkers clears only the revisions covered by this
// successful replay and drain. Never let an older invocation erase newer work.
func (h *EventHandlers) finalizePastMeetingReplayMarkers(ctx context.Context, replay *pastMeetingReplayState) bool {
	for _, marker := range []struct {
		key      string
		revision uint64
	}{
		{pastMeetingCleanupKeyPrefix + replay.parentID, replay.cleanupRevision},
		{pastMeetingRestorePendingPrefix + replay.parentID, replay.pendingRevision},
	} {
		if marker.revision == 0 {
			continue
		}
		entry, err := h.v1MappingsKV.Get(ctx, marker.key)
		if isKVAbsenceError(err) {
			continue // A nested successful repair already finished this work.
		}
		if err != nil || entry.Revision() != marker.revision {
			return true
		}
		if err := h.v1MappingsKV.Delete(ctx, marker.key, jetstream.LastRevision(marker.revision)); err != nil && !isKVAbsenceError(err) {
			return true
		}
	}
	return false
}

func (h *EventHandlers) drainPastMeetingChildRepairs(ctx context.Context, replay *pastMeetingReplayState) bool {
	if len(replay.ready) != 0 && parentOperation(ctx, replay.parentID) == nil {
		return h.withParentOperation(ctx, replay.parentID, func(owned context.Context) bool {
			return h.drainPastMeetingChildRepairs(owned, replay)
		})
	}
	owners := make(map[string]*pastMeetingChildAttempt)
	for key, owner := range childRepairOwners(ctx) {
		owners[key] = owner
	}
	var claimed []*pastMeetingChildAttempt
	claimFailed := false
	fullReplay := false
	for childKey, expected := range replay.ready {
		entry, err := h.v1MappingsKV.Get(ctx, pastMeetingChildRepairPrefix+childKey)
		if isKVAbsenceError(err) {
			continue // Another worker already completed this obligation.
		}
		if err != nil || entry.Revision() != expected {
			claimFailed = true
			break
		}
		var intent childRepairIntent
		if json.Unmarshal(entry.Value(), &intent) != nil || intent.ParentID != replay.parentID || intent.Phase != childRepairReady {
			claimFailed = true
			break // Never consume an active writer or a different repair.
		}
		needsReplay, err := h.childRepairNeedsFullReplay(ctx, intent)
		if err != nil {
			claimFailed = true
			break
		}
		intent = claimChildRepairIntent(intent, needsReplay)
		fullReplay = fullReplay || intent.FullReplay
		revision, err := h.updateChildRepairIntent(ctx, childKey, intent, expected)
		if err != nil {
			claimFailed = true
			break
		}
		owner := &pastMeetingChildAttempt{key: childKey, intent: intent, revision: revision}
		owners[childKey] = owner
		claimed = append(claimed, owner)
	}
	if len(claimed) == 0 {
		return claimFailed
	}
	retry := claimFailed
	ctx = context.WithValue(ctx, pastMeetingChildRepairOwnerContextKey{}, owners)
	if !claimFailed {
		for _, owner := range claimed {
			if h.compensateClaimedChildRepair(ctx, owner) {
				retry = true
				break
			}
			fullReplay = fullReplay || owner.intent.FullReplay
		}
	}
	if !retry && fullReplay {
		// All destructive child writes have finished. Reconcile the actual
		// live parent/children AFTER them, once for this batch, rather than
		// treating a live parent read as evidence that a late delete is fixed.
		handled, failed := h.repairPastMeetingIfLive(ctx, replay.parentID)
		retry = failed
		if !retry && !handled {
			retry = h.retriggerPastMeetingIndexing(ctx, replay.parentID)
		}
	}
	for _, owner := range claimed {
		if h.completePastMeetingChildRepair(ctx, owner.key, owner.intent, owner.revision, retry) {
			retry = true
		}
	}
	return retry
}

// republishPastMeetingArtifacts re-runs the recording and summary handlers on the past
// meeting's current artifact records, so their index documents pick up the access settings
// now on the past meeting. It returns true when a transient failure means the past meeting
// event must be retried.
func (h *EventHandlers) republishPastMeetingArtifacts(
	ctx context.Context,
	meetingAndOccurrenceID string,
	logger *slog.Logger,
) (retry bool) {
	// The recording record, the source of both the recording and the transcript documents,
	// is keyed by meeting_and_occurrence_id.
	retry = h.republishArtifactRecord(ctx, "itx-zoom-past-meetings-recordings."+meetingAndOccurrenceID, logger)

	summaryIDs, err := listPastMeetingSummaryIDs(ctx, meetingAndOccurrenceID, h.v1MappingsKV)
	if err != nil {
		logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to list summaries of past meeting, will retry")
		return true
	}
	for _, summaryID := range summaryIDs {
		if h.republishArtifactRecord(ctx, "itx-zoom-past-meetings-summaries."+summaryID, logger) {
			retry = true
		}
	}
	return retry
}

// republishArtifactRecord dispatches the current v1-objects record at key through handleKVPut,
// as if the record had just been written. A record that no longer exists has nothing to
// republish.
func (h *EventHandlers) republishArtifactRecord(ctx context.Context, key string, logger *slog.Logger) (retry bool) {
	if _, replaying := ctx.Value(pastMeetingReplayContextKey{}).(*pastMeetingReplayState); replaying {
		if entry, err := h.v1MappingsKV.Get(ctx, pastMeetingChildRepairPrefix+key); err == nil {
			var intent childRepairIntent
			if json.Unmarshal(entry.Value(), &intent) != nil {
				return true
			}
			if intent.Phase != childRepairReady && childRepairOwners(ctx)[key] == nil {
				// Do not race or steal the source, but do not clear the parent's
				// replay obligation either: a fast-path owner might not repair
				// restoration that starts during its final flush.
				return true
			}
		} else if !isKVAbsenceError(err) {
			return true
		}
		// Record handed-off work even if the child record is now absent. The
		// outer replay drains it after authoritative parent reconciliation.
		if h.resumePastMeetingChildRepair(ctx, key) {
			return true
		}
	}
	entry, err := h.v1ObjectsKV.Get(ctx, key)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return false
		}
		if errors.Is(err, jetstream.ErrInvalidKey) {
			logger.With(logging.ErrKey, err).ErrorContext(ctx, "invalid past meeting artifact key, skipping republish", "artifact_key", key)
			return false
		}
		logger.With(logging.ErrKey, err).WarnContext(ctx, "transient error fetching past meeting artifact for republish, will retry", "artifact_key", key)
		return true
	}
	data, err := decodeData(entry.Value())
	if err != nil {
		logger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to decode past meeting artifact for republish", "artifact_key", key)
		return false
	}
	ctx = context.WithValue(ctx, replayedSourceContextKey{}, &replayedSource{key: key, revision: entry.Revision()})
	return handleKVPut(ctx, key, data, h)
}

// handlePastMeetingDelete processes past meeting deletions
func (h *EventHandlers) handlePastMeetingDelete(ctx context.Context, key string, _ map[string]interface{}) (retry bool) {
	return h.withParentOperation(ctx, extractIDFromKey(key, pastMeetingObjectKeyPrefix), func(owned context.Context) bool {
		return h.handlePastMeetingDeleteOwned(owned, key)
	})
}

func (h *EventHandlers) handlePastMeetingDeleteOwned(ctx context.Context, key string) (retry bool) {
	pastMeetingID := extractIDFromKey(key, pastMeetingObjectKeyPrefix)
	if handled, retry := h.repairPastMeetingIfLive(ctx, pastMeetingID); handled {
		return retry // A stale delete must not undo a newer live restore.
	}
	// Older tombstones predate child cleanup. Only a separate durable marker
	// proves that the full scan finished; later deliveries can then skip it.
	sourceRevision, err := h.parentSourceRevision(ctx, pastMeetingID)
	if err != nil {
		return true
	}
	cleaned, err := h.cleanupCompletedForRevision(ctx, pastMeetingID, sourceRevision)
	if err != nil {
		return true
	}
	tombstoned, err := h.tombstoneState(ctx, "v1_past_meetings."+pastMeetingID)
	if err != nil {
		return true
	}
	if !cleaned || !tombstoned {
		h.logger.InfoContext(ctx, "processing past meeting delete", "past_meeting_id", pastMeetingID)
		if h.publishPastMeetingDelete(ctx, key, pastMeetingID) {
			return true
		}
	}
	return h.drainReadyPastMeetingChildren(ctx, pastMeetingID)
}

// drainReadyPastMeetingChildren also runs on an already-clean actual delete
// delivery: parent completion cannot discharge a later failed child retraction.
func (h *EventHandlers) drainReadyPastMeetingChildren(ctx context.Context, parentID string) bool {
	replay := &pastMeetingReplayState{parentID: parentID, ready: make(map[string]uint64)}
	replayCtx := context.WithValue(ctx, pastMeetingReplayContextKey{}, replay)
	if h.resumePastMeetingChildRepair(replayCtx, "itx-zoom-past-meetings-recordings."+parentID) {
		return true
	}
	for _, child := range pastMeetingChildKinds {
		ids, err := listIndexedPastMeetingChildIDs(ctx, h.v1MappingsKV, child, parentID)
		if err != nil {
			return true
		}
		for _, id := range ids {
			if h.resumePastMeetingChildRepair(replayCtx, child.objectPrefix+id) {
				return true
			}
		}
	}
	return h.drainPastMeetingChildRepairs(ctx, replay)
}

// publishPastMeetingDelete removes the past meeting and its children from the indexer,
// revokes access, and tombstones its mappings. Keep the tombstone last so a failed
// child cleanup is retried. The retrigger can replay the idempotent deletes after
// a republish that raced a delete.
func (h *EventHandlers) publishPastMeetingDelete(ctx context.Context, key, pastMeetingID string) (retry bool) {
	return h.withParentOperation(ctx, pastMeetingID, func(owned context.Context) bool {
		return h.publishPastMeetingDeleteOwned(owned, key, pastMeetingID)
	})
}

func (h *EventHandlers) publishPastMeetingDeleteOwned(ctx context.Context, key, pastMeetingID string) (retry bool) {
	if handled, retry := h.repairPastMeetingIfLive(ctx, pastMeetingID); handled {
		return retry
	}
	sourceRevision, err := h.parentSourceRevision(ctx, pastMeetingID)
	if err != nil {
		return true
	}
	// Persist deletion/restore intent before the first child is removed. A
	// partial cleanup may fail before the parent tombstone or completion marker
	// exists; restoration must still know to republish those children.
	if _, err := h.v1MappingsKV.Put(ctx, pastMeetingRestorePendingPrefix+pastMeetingID, []byte("1")); err != nil {
		h.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to mark past meeting deletion in progress, will retry")
		return true
	}
	if h.deletePastMeetingArtifacts(ctx, pastMeetingID) {
		return true
	}
	deleteAccessPayload, err := buildGenericDeleteAccessPayload("v1_past_meeting", pastMeetingID)
	if err != nil {
		h.logger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to build delete access payload", "past_meeting_id", pastMeetingID)
		return false
	}
	if h.handleMeetingTypeDelete(ctx, key, pastMeetingID, deleteAccessPayload, meetingDeleteConfig{
		indexerSubject:      "lfx.index.v1_past_meeting",
		deleteAccessSubject: fgaconstants.GenericDeleteAccessSubject,
		tombstoneKeyFmts:    []string{"v1_past_meetings.%s", "v1-mappings.past-meeting-mappings.%s"},
	}) {
		return true
	}
	tombstoned, err := h.tombstoneState(ctx, "v1_past_meetings."+pastMeetingID)
	if err != nil || !tombstoned {
		return true
	}
	completion, err := json.Marshal(pastMeetingCleanupCompletion{Complete: true, SourceRevision: sourceRevision})
	if err != nil {
		return true
	}
	if _, err := h.v1MappingsKV.Put(ctx, pastMeetingCleanupKeyPrefix+pastMeetingID, completion); err != nil {
		h.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to mark past meeting child cleanup complete, will retry")
		return true
	}
	// A restore may finish while cleanup is publishing its final deletes. If
	// so, republish the parent access and every tracked child so delete cannot
	// be the last writer.
	if handled, retry := h.repairPastMeetingIfLive(ctx, pastMeetingID); handled {
		return retry
	}
	afterRevision, err := h.parentSourceRevision(ctx, pastMeetingID)
	return err != nil || afterRevision != sourceRevision
}

// repairPastMeetingIfLive fences a stale delete or compensation against the
// current v1 record. Mark replay pending before republishing, so any failure
// remains retryable after the parent mapping has already become live.
func (h *EventHandlers) repairPastMeetingIfLive(ctx context.Context, pastMeetingID string) (handled, retry bool) {
	if parentOperation(ctx, pastMeetingID) != nil {
		return h.repairPastMeetingIfLiveOwned(ctx, pastMeetingID)
	}
	state, _, err := h.loadV1Parent(ctx, pastMeetingObjectKeyPrefix+pastMeetingID, "v1_past_meetings."+pastMeetingID)
	if err != nil {
		return true, true
	}
	if state != parentLive {
		return false, false
	}
	retry = h.withParentOperation(ctx, pastMeetingID, func(owned context.Context) bool {
		var failed bool
		handled, failed = h.repairPastMeetingIfLiveOwned(owned, pastMeetingID)
		return failed
	})
	return handled || retry, retry
}

func (h *EventHandlers) repairPastMeetingIfLiveOwned(ctx context.Context, pastMeetingID string) (handled, retry bool) {
	key := pastMeetingObjectKeyPrefix + pastMeetingID
	state, data, err := h.loadV1Parent(ctx, key, "v1_past_meetings."+pastMeetingID)
	if err != nil {
		h.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to check past meeting after delete, will retry")
		return true, true
	}
	if state != parentLive {
		return false, false
	}
	if _, err := h.v1MappingsKV.Put(ctx, pastMeetingRestorePendingPrefix+pastMeetingID, []byte("1")); err != nil {
		h.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to mark restored past meeting for repair, will retry")
		return true, true
	}
	if h.handlePastMeetingUpdate(ctx, key, data) {
		return true, true
	}
	// The snapshot used above can become stale while its writes are in flight.
	// As on the mapping-retrigger path, deletion must win even with no children
	// available to compensate the stale parent publish.
	after, _, err := h.loadV1Parent(ctx, key, "v1_past_meetings."+pastMeetingID)
	if err != nil {
		return true, true
	}
	if after == parentGone || after == parentAbsent {
		return true, h.publishPastMeetingDelete(ctx, key, pastMeetingID)
	}
	return true, false
}

// deletePastMeetingArtifacts retracts every indexed child before the parent tombstone
// makes future delete deliveries no-ops. The recording and transcript share the parent
// ID; summaries are tracked by parent in v1-mappings, but older indexed summaries
// may predate that index. Scan child tables as well to find legacy summaries,
// attachments and participants with independent IDs.
func (h *EventHandlers) deletePastMeetingArtifacts(ctx context.Context, pastMeetingID string) (retry bool) {
	if h.ensurePastMeetingParticipantsDiscovered(ctx, pastMeetingID) {
		return true
	}
	for _, subject := range []string{eventing.IndexV1PastMeetingRecordingSubject, eventing.IndexV1PastMeetingTranscriptSubject} {
		if err := h.publisher.PublishIndexerDelete(ctx, subject, pastMeetingID); err != nil {
			h.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to delete past meeting artifact, will retry", "subject", subject, "past_meeting_id", pastMeetingID)
			return true
		}
	}

	for _, child := range pastMeetingChildKinds {
		ids, err := listIndexedPastMeetingChildIDs(ctx, h.v1MappingsKV, child, pastMeetingID)
		if err != nil {
			h.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to list indexed past meeting children, will retry", "subject", child.subject)
			return true
		}
		for _, id := range ids {
			if err := h.publisher.PublishIndexerDelete(ctx, child.subject, id); err != nil {
				h.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to delete indexed past meeting child, will retry", "subject", child.subject)
				return true
			}
		}
		// Backfill legacy children that predate the per-parent lists. The scan
		// only runs until the cleanup-complete marker is durably written.
		if !child.participant() && h.deletePastMeetingChildren(ctx, pastMeetingID, child, ids) {
			return true
		}
	}
	return false
}

// deletePastMeetingChildren scans a child table for documents indexed under the
// deleted parent. Do not tombstone child mappings: their own delete events still
// need to clean up cross-references and can safely repeat these indexer deletes.
func (h *EventHandlers) deletePastMeetingChildren(ctx context.Context, pastMeetingID string, child pastMeetingChildKind, indexedIDs []string) (retry bool) {
	ids, err := h.discoverPastMeetingChildren(ctx, pastMeetingID, child, indexedIDs)
	if err != nil {
		h.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to discover legacy past meeting children, will retry", "subject", child.subject)
		return true
	}
	for _, id := range ids {
		if err := h.publisher.PublishIndexerDelete(ctx, child.subject, id); err != nil {
			h.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to delete past meeting child, will retry", "subject", child.subject)
			return true
		}
	}
	return false
}

// discoverPastMeetingChildren durably tracks legacy children before deleting
// documents or revoking parent-wide access. Returns only IDs not already indexed.
func (h *EventHandlers) discoverPastMeetingChildren(ctx context.Context, pastMeetingID string, child pastMeetingChildKind, indexedIDs []string) ([]string, error) {
	prefix := child.objectPrefix
	lister, err := h.v1ObjectsKV.ListKeysFiltered(ctx, prefix+">")
	if errors.Is(err, jetstream.ErrNoKeysFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = lister.Stop() }() // The keys channel is consumed or the handler retries.
	var discovered []string
	for key := range lister.Keys() {
		entry, err := h.v1ObjectsKV.Get(ctx, key)
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			continue // A concurrent child delete already removed this record.
		}
		if err != nil {
			h.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to read past meeting child for delete, will retry", "key", key)
			return nil, err
		}
		data, err := decodeData(entry.Value())
		if err != nil {
			// decodeData only reads these fixed bytes; retrying cannot fix them.
			// We cannot identify an indexed document's parent from corrupt data,
			// so log for operator follow-up without blocking other deletions.
			h.logger.With(logging.ErrKey, err).ErrorContext(ctx, "permanent decode failure for past meeting child; operator cleanup may be required", "key", key)
			continue
		}
		if utils.GetString(data["meeting_and_occurrence_id"]) != pastMeetingID {
			continue
		}
		childID := strings.TrimPrefix(key, prefix)
		if slices.Contains(indexedIDs, childID) {
			continue // Already deleted through the parent-keyed list.
		}
		var trackErr error
		if child.participant() {
			trackErr = recordPastMeetingParticipantID(ctx, h.v1MappingsKV, child.indexPrefix, pastMeetingID, childID)
		} else {
			trackErr = recordPastMeetingChildID(ctx, h.v1MappingsKV, child.indexPrefix, pastMeetingID, childID)
		}
		if trackErr != nil {
			h.logger.With(logging.ErrKey, trackErr).WarnContext(ctx, "failed to track past meeting child for restore, will retry", "key", key)
			return nil, trackErr
		}
		discovered = append(discovered, childID)
	}
	if err := ctx.Err(); err != nil {
		h.logger.With(logging.ErrKey, err).WarnContext(ctx, "past meeting child listing interrupted, will retry")
		return nil, err
	}
	return discovered, nil
}

// convertMapToPastMeetingData converts v1 past meeting data to v2 format
func convertMapToPastMeetingData(
	ctx context.Context,
	v1Data map[string]interface{},
	idMapper domain.IDMapper,
	v1ObjectsKV jetstream.KeyValue,
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) (*models.PastMeetingEventData, error) {
	rawPastMeeting, err := decodeV1[pastMeetingDBRaw](v1Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode past meeting data: %w", err)
	}

	// Validate required fields
	if rawPastMeeting.MeetingAndOccurrenceID == "" || rawPastMeeting.ProjectID == "" {
		return nil, fmt.Errorf("missing required fields: meeting_and_occurrence_id or proj_id")
	}

	// Map project ID from v1 SFID to v2 UID. A missing mapping means the project isn't in v2 yet —
	// the caller checks ProjectUID == "" and skips. Any other error is transient and propagated for retry.
	projectUID, err := idMapper.MapProjectV1ToV2(ctx, rawPastMeeting.ProjectID)
	if err != nil && domain.GetErrorType(err) != domain.ErrorTypeValidation {
		return nil, fmt.Errorf("failed to map project ID (transient): %w", err)
	}

	// Parse times
	startTime, _ := parseTime(rawPastMeeting.ScheduledStartTime)
	endTime, _ := parseTime(rawPastMeeting.ScheduledEndTime)
	createdAt, _ := parseTime(rawPastMeeting.CreatedAt)
	modifiedAt, _ := parseTime(rawPastMeeting.ModifiedAt)

	// Convert sessions
	var sessions []models.PastMeetingSession
	for _, rawSession := range rawPastMeeting.Sessions {
		sessionStart, _ := parseTime(rawSession.StartTime)
		sessionEnd, _ := parseTime(rawSession.EndTime)
		sessions = append(sessions, models.PastMeetingSession{
			UUID:      rawSession.UUID,
			StartTime: sessionStart,
			EndTime:   sessionEnd,
		})
	}

	// Get committees from mapping index (same logic as active meetings)
	committees, err := getCommitteesForPastMeeting(ctx, rawPastMeeting.MeetingAndOccurrenceID, idMapper, mappingsKV, logger)
	if err != nil {
		return nil, err
	}

	// Resolve the primary committee UID from the v1 SFID (mirrors MeetingEventData.CommitteeUID).
	var primaryCommitteeUID string
	if rawPastMeeting.Committee != "" {
		mapped, err := idMapper.MapCommitteeV1ToV2(ctx, rawPastMeeting.Committee)
		if err != nil {
			logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to map primary committee ID", "v1_id", rawPastMeeting.Committee)
		} else {
			primaryCommitteeUID = mapped
		}
	}

	// Fallback: if no committees from the mapping index and the primary committee was resolved,
	// use it as the sole entry. This mirrors v1-sync-helper's fallback path.
	if len(committees) == 0 && primaryCommitteeUID != "" {
		committees = []models.Committee{{
			UID:                   primaryCommitteeUID,
			AllowedVotingStatuses: utils.CastSlice[itx.CommitteeFilter](rawPastMeeting.CommitteeFilters),
		}}
	}

	// Compute artifact visibility from access fields (fallback chain)
	artifactVisibility := rawPastMeeting.RecordingAccess
	if artifactVisibility == "" {
		artifactVisibility = rawPastMeeting.TranscriptAccess
	}
	if artifactVisibility == "" {
		artifactVisibility = rawPastMeeting.AISummaryAccess
	}
	if artifactVisibility == "" {
		artifactVisibility = "meeting_hosts"
	}

	// Build ZoomConfig from flat v1 fields
	zoomConfig := buildPastMeetingZoomConfig(&rawPastMeeting)

	// Build event data
	return &models.PastMeetingEventData{
		ID:                       rawPastMeeting.MeetingAndOccurrenceID,
		MeetingID:                rawPastMeeting.MeetingID,
		MeetingAndOccurrenceID:   rawPastMeeting.MeetingAndOccurrenceID,
		OccurrenceID:             rawPastMeeting.OccurrenceID,
		ProjectSFID:              rawPastMeeting.ProjectID,
		ProjectUID:               projectUID,
		ProjectSlug:              rawPastMeeting.ProjectSlug,
		Committee:                rawPastMeeting.Committee,
		CommitteeUID:             primaryCommitteeUID,
		CommitteeFilters:         rawPastMeeting.CommitteeFilters,
		Title:                    rawPastMeeting.Topic,
		Description:              rawPastMeeting.Agenda,
		StartTime:                startTime,
		EndTime:                  endTime,
		Duration:                 rawPastMeeting.Duration,
		Timezone:                 rawPastMeeting.Timezone,
		MeetingType:              rawPastMeeting.MeetingType,
		Committees:               committees,
		Visibility:               rawPastMeeting.Visibility,
		ArtifactVisibility:       artifactVisibility,
		Restricted:               rawPastMeeting.Restricted,
		RecordingEnabled:         rawPastMeeting.RecordingEnabled,
		RecordingAccess:          rawPastMeeting.RecordingAccess,
		TranscriptEnabled:        rawPastMeeting.TranscriptEnabled,
		TranscriptAccess:         rawPastMeeting.TranscriptAccess,
		ZoomAIEnabled:            rawPastMeeting.ZoomAIEnabled,
		AISummaryAccess:          rawPastMeeting.AISummaryAccess,
		RequireAISummaryApproval: rawPastMeeting.RequireAISummaryApproval,
		EarlyJoinTimeMinutes:     rawPastMeeting.EarlyJoinTime,
		YoutubeLink:              rawPastMeeting.YoutubeLink,
		MeetingPassword:          rawPastMeeting.MeetingPassword,
		ZoomConfig:               zoomConfig,
		IsManuallyCreated:        rawPastMeeting.IsManuallyCreated,
		Sessions:                 sessions,
		CreatedAt:                createdAt,
		UpdatedAt:                modifiedAt,
		CreatedBy:                models.CreatedBy(rawPastMeeting.CreatedBy),
		UpdatedBy:                models.UpdatedBy(rawPastMeeting.UpdatedBy),
		UpdatedByList:            rawPastMeeting.UpdatedByList,
	}, nil
}

// buildPastMeetingZoomConfig constructs a ZoomConfig from flat v1 fields on the raw past meeting.
// Returns nil if no source fields are present so ZoomConfig is omitted from the event payload.
func buildPastMeetingZoomConfig(m *pastMeetingDBRaw) *models.ZoomConfig {
	if m.ZoomAIEnabled == nil && m.RequireAISummaryApproval == nil {
		return nil
	}
	cfg := &models.ZoomConfig{}
	if m.ZoomAIEnabled != nil {
		cfg.AICompanionEnabled = *m.ZoomAIEnabled
	}
	if m.RequireAISummaryApproval != nil {
		cfg.AISummaryRequireApproval = *m.RequireAISummaryApproval
	}
	return cfg
}

// =============================================================================
// Past Meeting Mapping Event Handler
// =============================================================================

// handlePastMeetingMappingUpdate processes updates to past meeting-committee mappings
func (h *EventHandlers) handlePastMeetingMappingUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) (retry bool) {
	funcLogger := h.logger.With("key", key, "handler", "past_meeting_mapping")
	funcLogger.InfoContext(ctx, "processing past meeting mapping update")

	// Extract past meeting ID and mapping data
	pastMeetingUUID := utils.GetString(v1Data["past_meeting_uuid"])
	mappingID := utils.GetString(v1Data["id"])
	committeeID := utils.GetString(v1Data["committee_id"])

	if pastMeetingUUID == "" || mappingID == "" {
		funcLogger.WarnContext(ctx, "missing required fields in past meeting mapping")
		return false
	}

	// The parent past meeting record decides: a deleted past meeting keeps its tombstones,
	// so do not rewrite its committee mappings or re-index it. A live record being restored
	// may still carry a stale tombstone; its mappings are stored for the restore to pick up.
	parent, _, err := h.loadV1Parent(ctx, fmt.Sprintf("itx-zoom-past-meetings.%s", pastMeetingUUID), fmt.Sprintf("v1_past_meetings.%s", pastMeetingUUID))
	if err != nil {
		funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to load parent past meeting, will retry", "past_meeting_uuid", pastMeetingUUID)
		return true
	}
	if parent == parentGone || parent == parentUnusable {
		funcLogger.InfoContext(ctx, "skipping past meeting mapping update for deleted past meeting", "past_meeting_uuid", pastMeetingUUID)
		return false
	}

	// Update committee mappings in KV bucket
	if err := updatePastMeetingCommitteeMappings(ctx, pastMeetingUUID, mappingID, committeeID, v1Data, h.v1MappingsKV, funcLogger); err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to update past meeting committee mappings")
		return isTransientError(err)
	}

	// Re-trigger past meeting indexing. This runs even when the parent was absent above: it
	// may have been indexed since, and the retrigger re-reads it and skips missing or deleted
	// past meetings.
	if h.retriggerPastMeetingIndexing(ctx, pastMeetingUUID) {
		return true
	}

	funcLogger.InfoContext(ctx, "successfully processed past meeting mapping update")
	return false
}

// handlePastMeetingMappingDelete processes past meeting-committee mapping deletions
func (h *EventHandlers) handlePastMeetingMappingDelete(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) (retry bool) {
	if v1Data == nil {
		h.logger.WarnContext(ctx, "no v1Data available for past meeting mapping delete, skipping", "key", key)
		return false
	}

	pastMeetingUUID := utils.GetString(v1Data["past_meeting_uuid"])
	mappingID := extractIDFromKey(key, "itx-zoom-past-meetings-mappings.")

	if pastMeetingUUID == "" || mappingID == "" {
		h.logger.WarnContext(ctx, "missing required fields in past meeting mapping deletion")
		return false
	}

	funcLogger := h.logger.With("past_meeting_uuid", pastMeetingUUID, "mapping_id", mappingID, "handler", "past_meeting_mapping_delete")
	funcLogger.InfoContext(ctx, "processing past meeting mapping deletion")

	// Remove the mapping
	if err := removePastMeetingCommitteeMapping(ctx, pastMeetingUUID, mappingID, h.v1MappingsKV, funcLogger); err != nil {
		if isTransientError(err) {
			return true
		}
	}

	// Re-trigger past meeting indexing
	shouldRetry := h.retriggerPastMeetingIndexing(ctx, pastMeetingUUID)
	if shouldRetry {
		return true
	}

	funcLogger.InfoContext(ctx, "successfully processed past meeting mapping deletion")
	return false
}

// =============================================================================
// Helper Functions for Past Meetings
// =============================================================================

// Data models for v1 past meeting raw data

func getCommitteesForPastMeeting(
	ctx context.Context,
	pastMeetingUUID string,
	idMapper domain.IDMapper,
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) ([]models.Committee, error) {
	key := fmt.Sprintf("v1-mappings.past-meeting-mappings.%s", pastMeetingUUID)
	entry, err := mappingsKV.Get(ctx, key)
	if err != nil {
		if !errors.Is(err, jetstream.ErrKeyNotFound) {
			logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to load past meeting committee mappings", "key", key)
			return nil, fmt.Errorf("failed to load past meeting committee mappings: %w", err)
		}
		return nil, nil
	}
	if string(entry.Value()) == tombstoneMarker {
		return nil, nil
	}

	var mappings map[string]map[string]interface{}
	if err := json.Unmarshal(entry.Value(), &mappings); err != nil {
		logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to unmarshal past meeting committee mappings")
		return nil, nil
	}

	committees := make([]models.Committee, 0, len(mappings))
	for _, mapping := range mappings {
		committeeID := utils.GetStringFromMap(mapping, "committee_id")
		if committeeID != "" {
			committeeUID, err := idMapper.MapCommitteeV1ToV2(ctx, committeeID)
			if err != nil {
				logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to map committee ID", "v1_id", committeeID)
				continue
			}

			filters := utils.GetStringSliceFromMap(mapping, "committee_filters")
			committees = append(committees, models.Committee{
				UID:                   committeeUID,
				AllowedVotingStatuses: utils.CastSlice[itx.CommitteeFilter](filters),
			})
		}
	}

	return committees, nil
}

// resolveParentPastMeetingCommittees returns the fully-resolved committee list for a past meeting,
// matching the shape PastMeetingEventData.Committees is populated with. Combines the v1-mappings KV
// lookup with the primary-committee fallback from the past-meeting row itself.
func resolveParentPastMeetingCommittees(
	ctx context.Context,
	meetingAndOccurrenceID string,
	primaryCommitteeSFID string,
	idMapper domain.IDMapper,
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) ([]models.Committee, error) {
	committees, err := getCommitteesForPastMeeting(ctx, meetingAndOccurrenceID, idMapper, mappingsKV, logger)
	if err != nil {
		return nil, err
	}

	if len(committees) == 0 && primaryCommitteeSFID != "" {
		uid, mapErr := idMapper.MapCommitteeV1ToV2(ctx, primaryCommitteeSFID)
		if mapErr != nil {
			if domain.GetErrorType(mapErr) != domain.ErrorTypeValidation {
				return nil, fmt.Errorf("failed to map primary committee ID for parent past meeting (transient): %w", mapErr)
			}
			logger.With(logging.ErrKey, mapErr).WarnContext(ctx, "primary committee mapping not found for parent past meeting", "v1_id", primaryCommitteeSFID)
		} else if uid != "" {
			committees = []models.Committee{{UID: uid}}
		}
	}

	return committees, nil
}

func updatePastMeetingCommitteeMappings(
	ctx context.Context,
	pastMeetingUUID string,
	mappingID string,
	committeeID string,
	v1Data map[string]interface{},
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) error {
	mappingsKey := fmt.Sprintf("v1-mappings.past-meeting-mappings.%s", pastMeetingUUID)
	var mappings map[string]map[string]interface{}

	entry, err := mappingsKV.Get(ctx, mappingsKey)
	if err != nil {
		if !errors.Is(err, jetstream.ErrKeyNotFound) {
			return fmt.Errorf("failed to load past meeting mappings: %w", err)
		}
		mappings = make(map[string]map[string]interface{})
	} else {
		if string(entry.Value()) == tombstoneMarker {
			mappings = make(map[string]map[string]interface{})
		} else if err := json.Unmarshal(entry.Value(), &mappings); err != nil {
			return fmt.Errorf("failed to unmarshal existing past meeting mappings: %w", err)
		}
	}

	// Update or add mapping
	mappingData := map[string]interface{}{"committee_id": committeeID}
	if filters := utils.GetStringSliceFromMap(v1Data, "committee_filters"); len(filters) > 0 {
		mappingData["committee_filters"] = filters
	}
	mappings[mappingID] = mappingData

	// Store updated mappings
	mappingsJSON, err := json.Marshal(mappings)
	if err != nil {
		return fmt.Errorf("failed to marshal past meeting mappings: %w", err)
	}

	if _, err := mappingsKV.Put(ctx, mappingsKey, mappingsJSON); err != nil {
		return fmt.Errorf("failed to store past meeting mappings: %w", err)
	}

	return nil
}

func removePastMeetingCommitteeMapping(
	ctx context.Context,
	pastMeetingUUID string,
	mappingID string,
	mappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) error {
	mappingsKey := fmt.Sprintf("v1-mappings.past-meeting-mappings.%s", pastMeetingUUID)
	entry, err := mappingsKV.Get(ctx, mappingsKey)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return nil // No mappings found
		}
		return fmt.Errorf("failed to load past meeting mappings: %w", err)
	}
	if string(entry.Value()) == tombstoneMarker {
		return nil // Already tombstoned, nothing to remove
	}

	var mappings map[string]map[string]interface{}
	if err := json.Unmarshal(entry.Value(), &mappings); err != nil {
		return fmt.Errorf("failed to unmarshal past meeting mappings: %w", err)
	}

	// Remove the mapping
	delete(mappings, mappingID)

	// If no mappings left, delete the key
	if len(mappings) == 0 {
		return mappingsKV.Delete(ctx, mappingsKey)
	}

	// Store updated mappings
	mappingsJSON, err := json.Marshal(mappings)
	if err != nil {
		return fmt.Errorf("failed to marshal updated past meeting mappings: %w", err)
	}

	_, err = mappingsKV.Put(ctx, mappingsKey, mappingsJSON)
	return err
}

func (h *EventHandlers) retriggerPastMeetingIndexing(
	ctx context.Context,
	pastMeetingUUID string,
) (retry bool) {
	pastMeetingKey := fmt.Sprintf("itx-zoom-past-meetings.%s", pastMeetingUUID)
	mappingKey := fmt.Sprintf("v1_past_meetings.%s", pastMeetingUUID)

	// A soft-deleted or already-deleted past meeting must not be republished as live.
	parent, pastMeetingData, err := h.loadV1Parent(ctx, pastMeetingKey, mappingKey)
	if err != nil {
		h.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to load past meeting during retrigger, will retry", "past_meeting_uuid", pastMeetingUUID)
		return true
	}
	switch parent {
	case parentLive:
	case parentGone, parentAbsent:
		// A deleted past meeting whose mapping still carries the live marker was indexed
		// after its delete cleanup (or before it ran): replay the cleanup.
		indexed, err := h.liveMarkerSet(ctx, mappingKey)
		if err != nil {
			h.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to check past meeting marker during retrigger, will retry", "past_meeting_uuid", pastMeetingUUID)
			return true
		}
		if indexed {
			h.logger.InfoContext(ctx, "deleted past meeting is still marked live, replaying delete", "past_meeting_uuid", pastMeetingUUID)
			return h.publishPastMeetingDelete(ctx, pastMeetingKey, pastMeetingUUID)
		}
		h.logger.InfoContext(ctx, "past meeting is not live, skipping retrigger", "past_meeting_uuid", pastMeetingUUID)
		return false
	default:
		h.logger.InfoContext(ctx, "past meeting record is unusable, skipping retrigger", "past_meeting_uuid", pastMeetingUUID)
		return false
	}
	// A live record with a tombstone is a restore whose own put has not been processed
	// yet; that put re-indexes the past meeting.
	deleted, err := h.tombstoneState(ctx, mappingKey)
	if err != nil {
		h.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to check past meeting tombstone during retrigger, will retry", "past_meeting_uuid", pastMeetingUUID)
		return true
	}
	if deleted {
		h.logger.InfoContext(ctx, "past meeting is tombstoned, skipping retrigger", "past_meeting_uuid", pastMeetingUUID)
		return false
	}

	// Re-process the past meeting
	if h.handlePastMeetingUpdate(ctx, pastMeetingKey, pastMeetingData) {
		return true
	}

	// v1 rewrites (soft delete) or removes (hard delete) the past meeting record before its
	// delete event is processed. If the record is no longer live, a delete ran concurrently
	// and this republish may have landed after its cleanup; replay the cleanup so the delete
	// wins.
	after, _, err := h.loadV1Parent(ctx, pastMeetingKey, mappingKey)
	if err != nil {
		h.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to recheck past meeting after retrigger, will retry", "past_meeting_uuid", pastMeetingUUID)
		return true
	}
	if after == parentGone || after == parentAbsent {
		h.logger.InfoContext(ctx, "past meeting was deleted during retrigger, replaying delete", "past_meeting_uuid", pastMeetingUUID)
		return h.publishPastMeetingDelete(ctx, pastMeetingKey, pastMeetingUUID)
	}
	return false
}
