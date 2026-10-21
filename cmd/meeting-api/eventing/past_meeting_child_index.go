// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/google/uuid"
	fgaconstants "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/eventing"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	pastMeetingAttachmentIndexPrefix = "v1-mappings.past-meeting-attachments."
	pastMeetingInviteeIndexPrefix    = "v1-mappings.past-meeting-invitees."
	pastMeetingAttendeeIndexPrefix   = "v1-mappings.past-meeting-attendees."
	pastMeetingCleanupKeyPrefix      = "v1-mappings.past-meeting-artifacts-cleaned."
	pastMeetingRestorePendingPrefix  = "v1-mappings.past-meeting-restore-pending."
	pastMeetingChildRepairPrefix     = "v1-mappings.past-meeting-child-repair."
)

type pastMeetingReplayContextKey struct{}
type pastMeetingChildAttemptContextKey struct{}
type pastMeetingChildRepairOwnerContextKey struct{}

type pastMeetingReplayState struct {
	parentID        string
	ready           map[string]uint64
	pendingRevision uint64
	cleanupRevision uint64
}

func parentReplay(ctx context.Context, parentID string) *pastMeetingReplayState {
	replay, _ := ctx.Value(pastMeetingReplayContextKey{}).(*pastMeetingReplayState)
	if replay != nil && replay.parentID == parentID {
		return replay
	}
	return nil
}

const (
	childRepairWriting = "writing"
	childRepairReady   = "repairable"
	childRepairRunning = "repairing"
)

// childRepairIntent is a durable, non-expiring ownership record. A timeout is
// never permission to steal a writer's slot: the old writer may still publish.
// worker identifies the pod/host for safe operator recovery after termination.
type childRepairIntent struct {
	ParentID               string   `json:"parent_id"`
	Owner                  string   `json:"owner"`
	Worker                 string   `json:"worker"`
	Phase                  string   `json:"phase"`
	ParentLive             bool     `json:"parent_live,omitempty"`
	ParentRevision         uint64   `json:"parent_revision,omitempty"`
	FullReplay             bool     `json:"full_replay,omitempty"`
	ParentSynchronized     bool     `json:"parent_synchronized,omitempty"`
	ParentMappingRevision  uint64   `json:"parent_mapping_revision,omitempty"`
	ParticipantIDs         []string `json:"participant_ids,omitempty"`
	ParticipantCompensated bool     `json:"participant_compensated,omitempty"`
	ParticipantDelete      bool     `json:"participant_delete,omitempty"`
	AttachmentSourceDelete bool     `json:"attachment_source_delete,omitempty"`
}

type pastMeetingChildAttempt struct {
	key      string
	intent   childRepairIntent
	revision uint64
}

func newChildRepairIntent(parentID, phase string) childRepairIntent {
	worker, _ := os.Hostname()
	return childRepairIntent{ParentID: parentID, Owner: uuid.NewString(), Worker: worker, Phase: phase, FullReplay: true}
}

func claimChildRepairIntent(previous childRepairIntent, fullReplay bool) childRepairIntent {
	claim := previous
	owner := newChildRepairIntent(previous.ParentID, childRepairRunning)
	claim.Owner, claim.Worker, claim.Phase = owner.Owner, owner.Worker, owner.Phase
	claim.FullReplay = fullReplay
	return claim
}

func (h *EventHandlers) updateChildRepairIntent(ctx context.Context, key string, intent childRepairIntent, revision uint64) (uint64, error) {
	value, err := json.Marshal(intent)
	if err != nil {
		return 0, err
	}
	return h.v1MappingsKV.Update(ctx, pastMeetingChildRepairPrefix+key, value, revision)
}

// startPastMeetingChildAttempt resumes only repairable work, then gives this
// invocation its own ownership scope. Completion is called on error paths too:
// no further writes may follow the flush and transition to repairable.
func (h *EventHandlers) startPastMeetingChildAttempt(ctx context.Context, key string) (context.Context, func(bool) bool, bool) {
	if owner := childRepairOwners(ctx)[key]; owner != nil {
		// This is the claimed repair replaying its own unfinished source.
		// Borrow the in-process capability, never reacquire its repairing key.
		ctx = context.WithValue(ctx, pastMeetingChildAttemptContextKey{}, owner)
		return ctx, func(retry bool) bool { return retry }, false
	}
	if h.resumePastMeetingChildRepair(ctx, key) {
		return ctx, nil, true
	}
	attempt := &pastMeetingChildAttempt{key: key}
	ctx = context.WithValue(ctx, pastMeetingChildAttemptContextKey{}, attempt)
	finish := func(retry bool) bool {
		if attempt.revision == 0 {
			return retry
		}
		if err := h.publisher.Flush(ctx); err != nil {
			h.logger.With(logging.ErrKey, err).ErrorContext(ctx, "child compensation flush failed; ownership retained for operator recovery", "repair_key", pastMeetingChildRepairPrefix+key)
			return true
		}
		attempt.intent.Phase = childRepairReady
		revision, err := h.updateChildRepairIntent(ctx, key, attempt.intent, attempt.revision)
		if err != nil {
			h.logger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to hand off child compensation; ownership retained", "repair_key", pastMeetingChildRepairPrefix+key)
			return true
		}
		if replay := parentReplay(ctx, attempt.intent.ParentID); replay != nil {
			replay.ready[key] = revision
			return retry
		}
		if retry {
			return true
		}
		return h.resumePastMeetingChildRepair(ctx, key)
	}
	return ctx, finish, false
}

func childRepairOwners(ctx context.Context) map[string]*pastMeetingChildAttempt {
	owners, _ := ctx.Value(pastMeetingChildRepairOwnerContextKey{}).(map[string]*pastMeetingChildAttempt)
	return owners
}

// parentReplaySnapshot distinguishes a live source from a fully published
// parent. Both marker state and mapping revision must remain stable for the
// ordinary-removal fast path; source revision alone cannot fence restoration.
func (h *EventHandlers) parentReplaySnapshot(ctx context.Context, parentID string) (bool, uint64, error) {
	entry, err := h.v1MappingsKV.Get(ctx, "v1_past_meetings."+parentID)
	if isKVAbsenceError(err) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, err
	}
	if string(entry.Value()) != "1" {
		return false, entry.Revision(), nil
	}
	for _, prefix := range []string{pastMeetingRestorePendingPrefix, pastMeetingCleanupKeyPrefix} {
		pending, err := h.liveMarkerSet(ctx, prefix+parentID)
		if err != nil {
			return false, 0, err
		}
		if pending {
			return false, entry.Revision(), nil
		}
	}
	return true, entry.Revision(), nil
}

// beginPastMeetingChildRepair records the event responsible for compensation
// before its first destructive write. Parent restoration must not clear this
// intent: that restoration can finish before the in-flight deletes do.
func (h *EventHandlers) beginPastMeetingChildRepair(ctx context.Context, key, parentID string) (retry bool) {
	return h.claimPastMeetingChildRepair(ctx, key, parentID, true)
}

func (h *EventHandlers) claimPastMeetingChildRepair(ctx context.Context, key, parentID string, fullReplay bool) (retry bool) {
	attempt, ok := ctx.Value(pastMeetingChildAttemptContextKey{}).(*pastMeetingChildAttempt)
	if !ok {
		h.logger.ErrorContext(ctx, "child compensation called without ownership scope", "repair_key", pastMeetingChildRepairPrefix+key)
		return true
	}
	if attempt.revision != 0 {
		if fullReplay && !attempt.intent.FullReplay {
			attempt.intent.FullReplay = true
			revision, err := h.updateChildRepairIntent(ctx, key, attempt.intent, attempt.revision)
			if err != nil {
				return true
			}
			attempt.revision = revision
		}
		return false
	}
	attempt.intent = newChildRepairIntent(parentID, childRepairWriting)
	attempt.intent.FullReplay = fullReplay
	if !fullReplay {
		entry, err := h.v1ObjectsKV.Get(ctx, pastMeetingObjectKeyPrefix+parentID)
		if err != nil && !isKVAbsenceError(err) {
			return true
		}
		if err == nil {
			data, err := decodeData(entry.Value())
			if err != nil {
				return true
			}
			_, deleted := softDeletedAt(data)
			attempt.intent.ParentLive = !deleted
			attempt.intent.ParentRevision = entry.Revision()
		}
		attempt.intent.ParentSynchronized, attempt.intent.ParentMappingRevision, err = h.parentReplaySnapshot(ctx, parentID)
		if err != nil {
			return true
		}
	}
	value, err := json.Marshal(attempt.intent)
	if err != nil {
		return true
	}
	revision, err := h.v1MappingsKV.Create(ctx, pastMeetingChildRepairPrefix+key, value)
	if errors.Is(err, jetstream.ErrKeyExists) && parentReplay(ctx, parentID) != nil {
		// Parent redelivery can own the continuation of a previously handed-
		// off child. Never take over writing/repairing ownership.
		entry, getErr := h.v1MappingsKV.Get(ctx, pastMeetingChildRepairPrefix+key)
		if getErr != nil {
			return true
		}
		var previous childRepairIntent
		if json.Unmarshal(entry.Value(), &previous) != nil || previous.ParentID != parentID || previous.Phase != childRepairReady {
			return true
		}
		// Continue the existing obligation, not a fresh snapshot which could
		// discard pending sibling IDs or a source deletion after restoration.
		previous.Owner, previous.Worker, previous.Phase = attempt.intent.Owner, attempt.intent.Worker, childRepairWriting
		previous.FullReplay = previous.FullReplay || attempt.intent.FullReplay
		attempt.intent = previous
		value, err = json.Marshal(attempt.intent)
		if err != nil {
			return true
		}
		revision, err = h.v1MappingsKV.Update(ctx, pastMeetingChildRepairPrefix+key, value, entry.Revision())
	}
	if err != nil {
		h.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to persist past meeting child repair intent, will retry")
		return true
	}
	attempt.revision = revision
	return false
}

// childRepairNeedsFullReplay avoids parent-wide work only when no parent-wide
// compensation occurred and the same authoritative live revision still exists.
// The comparison happens after the writer flushed all destructive publications.
func (h *EventHandlers) childRepairNeedsFullReplay(ctx context.Context, intent childRepairIntent) (bool, error) {
	if intent.FullReplay || !intent.ParentLive || intent.ParentRevision == 0 || !intent.ParentSynchronized {
		return true, nil
	}
	synchronized, mappingRevision, err := h.parentReplaySnapshot(ctx, intent.ParentID)
	if err != nil {
		return true, err
	}
	if !synchronized || mappingRevision != intent.ParentMappingRevision {
		return true, nil
	}
	entry, err := h.v1ObjectsKV.Get(ctx, pastMeetingObjectKeyPrefix+intent.ParentID)
	if isKVAbsenceError(err) {
		return true, nil
	}
	if err != nil {
		return true, err
	}
	if entry.Revision() != intent.ParentRevision {
		return true, nil
	}
	data, err := decodeData(entry.Value())
	if err != nil {
		return true, err
	}
	_, deleted := softDeletedAt(data)
	return deleted, nil
}

// resumePastMeetingChildRepair runs both before redelivery and after the whole
// child handler succeeds (including a participant delete's final member_remove).
// A tombstoned child must still finish this work before its early ACK. The
// parent replay context prevents recursively repairing the same parent while
// its repair is dispatching children through their normal handlers.
func (h *EventHandlers) resumePastMeetingChildRepair(ctx context.Context, key string) (retry bool) {
	markerKey := pastMeetingChildRepairPrefix + key
	entry, err := h.v1MappingsKV.Get(ctx, markerKey)
	if isKVAbsenceError(err) {
		return false
	}
	if err != nil {
		h.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to read past meeting child repair intent, will retry")
		return true
	}
	var intent childRepairIntent
	if err := json.Unmarshal(entry.Value(), &intent); err != nil {
		// The preceding PR revision stored a plain parent ID. That format had
		// no ownership phase; do not silently treat an old writer as finished.
		h.logger.With(logging.ErrKey, err).ErrorContext(ctx, "invalid or legacy child repair intent requires operator recovery", "repair_key", markerKey)
		return true
	}
	parentID := intent.ParentID
	if replay := parentReplay(ctx, parentID); replay != nil {
		if intent.Phase == childRepairReady {
			replay.ready[key] = entry.Revision()
		}
		return false
	}
	if parentID == "" || intent.Phase != childRepairReady {
		h.logger.WarnContext(ctx, "past meeting child repair is owned; will retry without stealing ownership", "repair_key", markerKey, "worker", intent.Worker, "phase", intent.Phase)
		return true
	}
	fullReplay, err := h.childRepairNeedsFullReplay(ctx, intent)
	if err != nil {
		return true
	}
	if fullReplay && parentOperation(ctx, parentID) == nil {
		// The outer parent capability must outlive this child claim's final
		// completion, not end when nested parent reconciliation returns.
		return h.withParentOperation(ctx, parentID, func(owned context.Context) bool {
			return h.resumePastMeetingChildRepair(owned, key)
		})
	}
	intent = claimChildRepairIntent(intent, fullReplay)
	revision, err := h.updateChildRepairIntent(ctx, key, intent, entry.Revision())
	if err != nil {
		return true
	}
	owner := &pastMeetingChildAttempt{key: key, intent: intent, revision: revision}
	ctx = context.WithValue(ctx, pastMeetingChildRepairOwnerContextKey{}, map[string]*pastMeetingChildAttempt{key: owner})
	retry = h.compensateClaimedChildRepair(ctx, owner)
	handled := false
	fullReplay = fullReplay || owner.intent.FullReplay
	if fullReplay && parentOperation(ctx, parentID) == nil {
		// A source continuation upgraded an ordinary claim. Hand it off
		// before acquiring parent ownership; never invert a blocking lock.
		return h.completePastMeetingChildRepair(ctx, key, owner.intent, owner.revision, true)
	}
	if !retry && fullReplay {
		handled, retry = h.repairPastMeetingIfLive(ctx, parentID)
	}
	if !retry && fullReplay && !handled && h.retriggerPastMeetingIndexing(ctx, parentID) {
		// A previous repair may have republished the parent, then failed its
		// compensating delete. Resume that cleanup as well as live repairs.
		retry = true
	}
	return h.completePastMeetingChildRepair(ctx, key, owner.intent, owner.revision, retry)
}

// compensateClaimedChildRepair is shared by individual and batch repair:
// owning a marker does not prove its affected-document retractions finished.
// Artifact IDs are recoverable directly from their stable source keys, even
// when a matching parent cleanup marker makes parent-wide cleanup a no-op.
func (h *EventHandlers) compensateClaimedChildRepair(ctx context.Context, owner *pastMeetingChildAttempt) bool {
	if owner.intent.AttachmentSourceDelete && h.reconcileClaimedAttachmentSource(ctx, owner) {
		return true
	}
	if owner.intent.ParticipantDelete && h.resumeClaimedParticipantDelete(ctx, owner) {
		return true
	}
	parentID := owner.intent.ParentID
	state, _, err := h.loadV1Parent(ctx, pastMeetingObjectKeyPrefix+parentID, "v1_past_meetings."+parentID)
	if err != nil {
		return true
	}
	if state != parentGone && state != parentAbsent {
		return false // Live parent repair republishes the current children.
	}
	ctx = context.WithValue(ctx, pastMeetingChildAttemptContextKey{}, owner)
	switch {
	case strings.HasPrefix(owner.key, "itx-zoom-past-meetings-recordings."):
		return h.deletePastMeetingRecordingDocuments(ctx, owner.key, strings.TrimPrefix(owner.key, "itx-zoom-past-meetings-recordings."))
	case strings.HasPrefix(owner.key, "itx-zoom-past-meetings-summaries."):
		return h.deletePastMeetingSummaryDocument(ctx, owner.key, parentID, strings.TrimPrefix(owner.key, "itx-zoom-past-meetings-summaries."))
	case strings.HasPrefix(owner.key, "itx-zoom-past-meetings-attachments."):
		return h.deletePastMeetingAttachmentDocument(ctx, owner.key, parentID, strings.TrimPrefix(owner.key, "itx-zoom-past-meetings-attachments."))
	case len(owner.intent.ParticipantIDs) != 0 && !owner.intent.ParticipantCompensated:
		return h.retractDeletedPastMeetingParticipants(ctx, owner.key, parentID)
	}
	return false
}

// protectParticipantDeletion retains the source operation separately from its
// affected publications, including a hard delete whose KV value is now absent.
func (h *EventHandlers) protectParticipantDeletion(ctx context.Context, key, parentID string) bool {
	if h.claimPastMeetingChildRepair(ctx, key, parentID, false) {
		return true
	}
	owner := ctx.Value(pastMeetingChildAttemptContextKey{}).(*pastMeetingChildAttempt)
	prefix, indexPrefix := "itx-zoom-past-meetings-invitees.", pastMeetingInviteeIndexPrefix
	if strings.HasPrefix(key, "itx-zoom-past-meetings-attendees.") {
		prefix, indexPrefix = "itx-zoom-past-meetings-attendees.", pastMeetingAttendeeIndexPrefix
	}
	if !strings.HasPrefix(key, prefix) {
		return true
	}
	// A legacy rich mapping can predate parent-keyed tracking. Make its
	// missing source discoverable to parent-only repair before any delete.
	if err := recordPastMeetingParticipantID(ctx, h.v1MappingsKV, indexPrefix, parentID, strings.TrimPrefix(key, prefix)); err != nil {
		return true
	}
	if owner.intent.ParticipantDelete {
		return false
	}
	owner.intent.ParticipantDelete = true
	revision, err := h.updateChildRepairIntent(ctx, key, owner.intent, owner.revision)
	if err != nil {
		return true
	}
	owner.revision = revision
	return false
}

// resumeClaimedParticipantDelete finishes a retained soft delete or a missing
// hard-delete source under this exact owner. Missing rich mapping evidence is
// not permission to discard a potentially outstanding member_remove.
func (h *EventHandlers) resumeClaimedParticipantDelete(ctx context.Context, owner *pastMeetingChildAttempt) bool {
	prefix := "itx-zoom-past-meetings-invitees."
	mappingPrefix := "v1_past_meeting_invitees."
	if strings.HasPrefix(owner.key, "itx-zoom-past-meetings-attendees.") {
		prefix, mappingPrefix = "itx-zoom-past-meetings-attendees.", "v1_past_meeting_attendees."
	}
	if !strings.HasPrefix(owner.key, prefix) {
		return true
	}
	id := strings.TrimPrefix(owner.key, prefix)
	mapping, mappingErr := h.v1MappingsKV.Get(ctx, mappingPrefix+id)
	if mappingErr != nil && !isKVAbsenceError(mappingErr) {
		return true
	}
	if mappingErr == nil && entryIsTombstoned(mapping) {
		return false // This source's publications already completed.
	}
	entry, err := h.v1ObjectsKV.Get(ctx, owner.key)
	ctx = context.WithValue(ctx, pastMeetingChildAttemptContextKey{}, owner)
	if isKVAbsenceError(err) {
		if mappingErr != nil {
			return true
		}
		_, _, parentID := parseRegistrantMappingValue(string(mapping.Value()))
		if parentID == "" || parentID != owner.intent.ParentID {
			h.logger.ErrorContext(ctx, "pending hard participant delete lacks matching rich mapping; reconciliation required", "repair_key", pastMeetingChildRepairPrefix+owner.key)
			return true
		}
		return routeDelete(ctx, owner.key, nil, h)
	}
	if err != nil {
		return true
	}
	data, err := decodeData(entry.Value())
	if err != nil {
		return true
	}
	if _, deleted := softDeletedAt(data); deleted {
		return routeDelete(ctx, owner.key, data, h)
	}
	return false // Never apply an old deletion over a recreated live source.
}

// completePastMeetingChildRepair relinquishes only this claimed revision after
// its writes have ended. Parent replay can use it after reconciling the whole
// parent without recursively invoking another parent repair.
func (h *EventHandlers) completePastMeetingChildRepair(ctx context.Context, key string, intent childRepairIntent, revision uint64, retry bool) bool {
	markerKey := pastMeetingChildRepairPrefix + key
	// Even failed repairs must finish their queued writes before handoff. If
	// flushing fails, retain repairing ownership for fail-closed recovery.
	if err := h.publisher.Flush(ctx); err != nil {
		h.logger.With(logging.ErrKey, err).ErrorContext(ctx, "child repair flush failed; ownership retained for operator recovery", "repair_key", markerKey)
		return true
	}
	if !retry {
		err := h.v1MappingsKV.Delete(ctx, markerKey, jetstream.LastRevision(revision))
		if err == nil || errors.Is(err, jetstream.ErrKeyNotFound) {
			return false
		}
		h.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to clear past meeting child repair intent, will retry")
	}
	intent.Phase = childRepairReady
	if _, err := h.updateChildRepairIntent(ctx, key, intent, revision); err != nil {
		h.logger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to release child repair ownership", "repair_key", markerKey)
	}
	return true
}

type pastMeetingChildKind struct {
	objectPrefix string
	indexPrefix  string
	subject      string
}

var pastMeetingChildKinds = [...]pastMeetingChildKind{
	{"itx-zoom-past-meetings-summaries.", pastMeetingSummaryIndexKeyPrefix, eventing.IndexV1PastMeetingSummarySubject},
	{"itx-zoom-past-meetings-attachments.", pastMeetingAttachmentIndexPrefix, eventing.IndexV1PastMeetingAttachmentSubject},
	{"itx-zoom-past-meetings-invitees.", pastMeetingInviteeIndexPrefix, eventing.IndexV1PastMeetingParticipantSubject},
	{"itx-zoom-past-meetings-attendees.", pastMeetingAttendeeIndexPrefix, eventing.IndexV1PastMeetingParticipantSubject},
}

func (c pastMeetingChildKind) participant() bool {
	return c.indexPrefix == pastMeetingInviteeIndexPrefix || c.indexPrefix == pastMeetingAttendeeIndexPrefix
}

// Participants use one mapping key each: a single growing JSON array would
// contend on CAS and require O(n) rewrites for every participant event.
func recordPastMeetingParticipantID(ctx context.Context, mappingsKV jetstream.KeyValue, prefix, parentID, id string) error {
	_, err := mappingsKV.Put(ctx, prefix+parentID+"."+id, []byte("1"))
	return err
}

func listIndexedPastMeetingChildIDs(ctx context.Context, mappingsKV jetstream.KeyValue, child pastMeetingChildKind, parentID string) ([]string, error) {
	if !child.participant() {
		return listPastMeetingChildIDs(ctx, mappingsKV, child.indexPrefix, parentID)
	}
	prefix := child.indexPrefix + parentID + "."
	lister, err := mappingsKV.ListKeysFiltered(ctx, prefix+">")
	if errors.Is(err, jetstream.ErrNoKeysFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = lister.Stop() }()
	var ids []string
	for key := range lister.Keys() {
		ids = append(ids, strings.TrimPrefix(key, prefix))
	}
	return ids, ctx.Err()
}

// protectParticipantDocuments persists the bounded set of documents this source
// may publish BEFORE publication. Redelivery can compensate its own effects
// without scanning or deleting every other participant of the parent.
func (h *EventHandlers) protectParticipantDocuments(ctx context.Context, key, parentID string, ids ...string) bool {
	if h.claimPastMeetingChildRepair(ctx, key, parentID, false) {
		return true
	}
	attempt := ctx.Value(pastMeetingChildAttemptContextKey{}).(*pastMeetingChildAttempt)
	changed := false
	if attempt.intent.ParticipantCompensated {
		attempt.intent.ParticipantCompensated = false
		changed = true
	}
	for _, id := range ids {
		if id != "" && !slices.Contains(attempt.intent.ParticipantIDs, id) {
			attempt.intent.ParticipantIDs = append(attempt.intent.ParticipantIDs, id)
			changed = true
		}
	}
	if !changed {
		return false
	}
	revision, err := h.updateChildRepairIntent(ctx, key, attempt.intent, attempt.revision)
	if err != nil {
		return true
	}
	attempt.revision = revision
	return false
}

// retractDeletedPastMeetingParticipants compensates only documents this source
// could have published and parent access. Legacy discovery is shared durably by
// deletion generation, not repeated for each late child.
func (h *EventHandlers) retractDeletedPastMeetingParticipants(ctx context.Context, key, parentID string) (retry bool) {
	if h.beginPastMeetingChildRepair(ctx, key, parentID) {
		return true
	}
	if h.ensurePastMeetingParticipantsDiscovered(ctx, parentID) {
		return true
	}
	id := strings.TrimPrefix(key, "itx-zoom-past-meetings-invitees.")
	if id == key {
		id = strings.TrimPrefix(key, "itx-zoom-past-meetings-attendees.")
	}
	attempt := ctx.Value(pastMeetingChildAttemptContextKey{}).(*pastMeetingChildAttempt)
	if !slices.Contains(attempt.intent.ParticipantIDs, id) {
		attempt.intent.ParticipantIDs = append(attempt.intent.ParticipantIDs, id)
		revision, err := h.updateChildRepairIntent(ctx, key, attempt.intent, attempt.revision)
		if err != nil {
			return true
		}
		attempt.revision = revision
	}
	for _, id := range attempt.intent.ParticipantIDs {
		if err := h.publisher.PublishIndexerDelete(ctx, eventing.IndexV1PastMeetingParticipantSubject, id); err != nil {
			h.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to retract affected participant of deleted past meeting, will retry")
			return true
		}
	}
	payload, err := buildGenericDeleteAccessPayload("v1_past_meeting", parentID)
	if err != nil {
		h.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to build deleted past meeting access revocation, will retry")
		return true
	}
	if err := h.publisher.PublishAccessDelete(ctx, fgaconstants.GenericDeleteAccessSubject, payload); err != nil {
		h.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to revoke deleted past meeting access, will retry")
		return true
	}
	if err := h.publisher.Flush(ctx); err != nil {
		return true
	}
	attempt.intent.ParticipantCompensated = true
	revision, err := h.updateChildRepairIntent(ctx, key, attempt.intent, attempt.revision)
	if err != nil {
		return true
	}
	attempt.revision = revision
	// The caller repairs only after its remaining writes have finished, so a
	// participant delete cannot revoke a sibling immediately after repair.
	return false
}

// republishPastMeetingChildren restores attachment and participant documents
// after the parent mapping changes from a delete tombstone back to live.
func (h *EventHandlers) republishPastMeetingChildren(ctx context.Context, parentID string, logger *slog.Logger) (retry bool) {
	for _, child := range pastMeetingChildKinds[1:] { // summaries and recordings are republished separately
		ids, err := listIndexedPastMeetingChildIDs(ctx, h.v1MappingsKV, child, parentID)
		if err != nil {
			logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to list past meeting children for restore, will retry", "subject", child.subject)
			return true
		}
		for _, id := range ids {
			if h.republishArtifactRecord(ctx, fmt.Sprintf("%s%s", child.objectPrefix, id), logger) {
				retry = true
			}
		}
	}
	return retry
}
