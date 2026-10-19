// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/nats-io/nats.go/jetstream"
)

const pastMeetingOperationPrefix = "v1-mappings.past-meeting-operation."

type parentOperationContextKey struct{}

type parentOperationRecord struct {
	ParentID       string `json:"parent_id"`
	Owner          string `json:"owner"`
	Worker         string `json:"worker"`
	Phase          string `json:"phase"`
	SourceRevision uint64 `json:"source_revision"`
	Requests       uint64 `json:"requests"`
}

type parentOperationOwner struct {
	record   parentOperationRecord
	revision uint64
}

func parentOperation(ctx context.Context, parentID string) *parentOperationOwner {
	owners, _ := ctx.Value(parentOperationContextKey{}).(map[string]*parentOperationOwner)
	return owners[parentID]
}

func newParentOperation(parentID, phase string) parentOperationRecord {
	owner := newChildRepairIntent(parentID, phase)
	return parentOperationRecord{ParentID: parentID, Owner: owner.Owner, Worker: owner.Worker, Phase: phase}
}

// withParentOperation owns the complete parent mutation, including nested
// reconciliation and child-claim completion. Nested calls borrow the capability
// and cannot finalize it. No worker waits while holding another worker's slot.
func (h *EventHandlers) withParentOperation(ctx context.Context, parentID string, run func(context.Context) bool) bool {
	if parentOperation(ctx, parentID) != nil {
		return run(ctx)
	}
	key := pastMeetingOperationPrefix + parentID
	entry, err := h.v1MappingsKV.Get(ctx, key)
	recovered := false
	record := newParentOperation(parentID, childRepairWriting)
	var revision uint64
	if isKVAbsenceError(err) {
		value, _ := json.Marshal(record)
		revision, err = h.v1MappingsKV.Create(ctx, key, value)
	} else if err == nil {
		var previous parentOperationRecord
		if json.Unmarshal(entry.Value(), &previous) != nil || previous.ParentID != parentID {
			return true
		}
		if previous.Phase != childRepairReady {
			h.requestParentOperation(ctx, parentID)
			return true
		}
		record = newParentOperation(parentID, childRepairRunning)
		record.Requests = previous.Requests
		value, _ := json.Marshal(record)
		revision, err = h.v1MappingsKV.Update(ctx, key, value, entry.Revision())
		recovered = true
	}
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyExists) || errors.Is(err, jetstream.ErrKeyRevisionMismatch) {
			h.requestParentOperation(ctx, parentID)
		}
		return true
	}
	owner := &parentOperationOwner{record: record, revision: revision}
	owners := make(map[string]*parentOperationOwner)
	if inherited, _ := ctx.Value(parentOperationContextKey{}).(map[string]*parentOperationOwner); inherited != nil {
		for id, existing := range inherited {
			owners[id] = existing
		}
	}
	owners[parentID] = owner
	ctx = context.WithValue(ctx, parentOperationContextKey{}, owners)
	sourceRevision, err := h.parentSourceRevision(ctx, parentID)
	if err != nil {
		return h.finishParentOperation(ctx, owner, true)
	}
	// Snapshot requests before any work. A later request invalidates completion
	// even if its source revision is unchanged (e.g. a new ready child intent).
	owner.record.SourceRevision = sourceRevision
	value, _ := json.Marshal(owner.record)
	owner.revision, err = h.v1MappingsKV.Update(ctx, key, value, owner.revision)
	if err != nil {
		return h.finishParentOperation(ctx, owner, true)
	}
	if recovered && h.reconcileParentOperation(ctx, parentID) {
		return h.finishParentOperation(ctx, owner, true)
	}
	return h.finishParentOperation(ctx, owner, run(ctx))
}

// requestParentOperation leaves durable wake-up evidence without stealing or
// replacing the owner. Its revision change prevents stale finalization; a
// request racing slot deletion creates repairable work instead of disappearing.
func (h *EventHandlers) requestParentOperation(ctx context.Context, parentID string) {
	key := pastMeetingOperationPrefix + parentID
	for range 3 {
		entry, err := h.v1MappingsKV.Get(ctx, key)
		if isKVAbsenceError(err) {
			record := newParentOperation(parentID, childRepairReady)
			record.Requests = 1
			value, _ := json.Marshal(record)
			if _, err := h.v1MappingsKV.Create(ctx, key, value); err == nil {
				return
			}
			continue
		}
		if err != nil {
			return
		}
		var record parentOperationRecord
		if json.Unmarshal(entry.Value(), &record) != nil || record.ParentID != parentID {
			return
		}
		record.Requests++
		value, _ := json.Marshal(record)
		if _, err := h.v1MappingsKV.Update(ctx, key, value, entry.Revision()); err == nil {
			return
		}
	}
	h.logger.WarnContext(ctx, "failed to persist parent-operation request; delivery must retry", "operation_key", key)
}

func (h *EventHandlers) reconcileParentOperation(ctx context.Context, parentID string) bool {
	key := pastMeetingObjectKeyPrefix + parentID
	state, data, err := h.loadV1Parent(ctx, key, "v1_past_meetings."+parentID)
	if err != nil {
		return true
	}
	switch state {
	case parentLive:
		// A retained owned operation, not a legacy boolean marker, proves that
		// full reconciliation is still owed after a partial mutation failure.
		if _, err := h.v1MappingsKV.Put(ctx, pastMeetingRestorePendingPrefix+parentID, []byte("1")); err != nil {
			return true
		}
		return h.handlePastMeetingUpdate(ctx, key, data)
	case parentGone, parentAbsent:
		return h.handlePastMeetingDelete(ctx, key, nil)
	default:
		return true
	}
}

func (h *EventHandlers) finishParentOperation(ctx context.Context, owner *parentOperationOwner, retry bool) bool {
	key := pastMeetingOperationPrefix + owner.record.ParentID
	if err := h.publisher.Flush(ctx); err != nil {
		h.logger.With(logging.ErrKey, err).ErrorContext(ctx, "parent operation flush failed; ownership retained for operator recovery", "operation_key", key)
		return true
	}
	after, err := h.parentSourceRevision(ctx, owner.record.ParentID)
	if err != nil || after != owner.record.SourceRevision {
		retry = true
	}
	for range 3 {
		entry, err := h.v1MappingsKV.Get(ctx, key)
		if err != nil {
			return true
		}
		var current parentOperationRecord
		if json.Unmarshal(entry.Value(), &current) != nil || current.Owner != owner.record.Owner || current.ParentID != owner.record.ParentID {
			return true
		}
		if current.Requests != owner.record.Requests {
			retry = true
		}
		if !retry {
			if err := h.v1MappingsKV.Delete(ctx, key, jetstream.LastRevision(entry.Revision())); err == nil || errors.Is(err, jetstream.ErrKeyNotFound) {
				return false
			}
			retry = true
			continue
		}
		current.Phase = childRepairReady
		value, _ := json.Marshal(current)
		if _, err := h.v1MappingsKV.Update(ctx, key, value, entry.Revision()); err == nil {
			return true
		}
	}
	h.logger.ErrorContext(ctx, "parent operation handoff uncertain; ownership retained for operator recovery", "operation_key", key)
	return true
}
