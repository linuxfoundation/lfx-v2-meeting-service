// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import "context"

type replayedSourceContextKey struct{}

type replayedSource struct {
	key      string
	revision uint64
}

// attachmentReplaySource checks the child, not just its parent. A missing or
// deleted source requires unconditional, durably owned retraction: an older
// mapping tombstone cannot prove a later stale publication was removed.
func (h *EventHandlers) attachmentReplaySource(ctx context.Context, key, parentID string, beforePublish bool) (handled, retry bool) {
	snapshot, _ := ctx.Value(replayedSourceContextKey{}).(*replayedSource)
	if snapshot == nil || snapshot.key != key {
		return false, false
	}
	entry, err := h.v1ObjectsKV.Get(ctx, key)
	deleted := isKVAbsenceError(err)
	if err != nil && !deleted {
		return true, true
	}
	if !deleted {
		data, err := decodeData(entry.Value())
		if err != nil {
			return true, true
		}
		_, deleted = softDeletedAt(data)
	}
	if deleted {
		if h.beginPastMeetingChildRepair(ctx, key, parentID) {
			return true, true
		}
		owner := ctx.Value(pastMeetingChildAttemptContextKey{}).(*pastMeetingChildAttempt)
		if !owner.intent.AttachmentSourceDelete {
			owner.intent.AttachmentSourceDelete = true
			revision, err := h.updateChildRepairIntent(ctx, key, owner.intent, owner.revision)
			if err != nil {
				return true, true
			}
			owner.revision = revision
		}
		return true, h.reconcileClaimedAttachmentSource(ctx, owner)
	}
	if entry.Revision() != snapshot.revision {
		if !beforePublish && h.beginPastMeetingChildRepair(ctx, key, parentID) {
			return true, true
		}
		return true, true // Retry the newer live value; never acknowledge saved stale bytes.
	}
	return false, false
}

func (h *EventHandlers) reconcileClaimedAttachmentSource(ctx context.Context, owner *pastMeetingChildAttempt) bool {
	entry, err := h.v1ObjectsKV.Get(ctx, owner.key)
	deleted := isKVAbsenceError(err)
	if err != nil && !deleted {
		return true
	}
	if !deleted {
		data, err := decodeData(entry.Value())
		if err != nil {
			return true
		}
		_, deleted = softDeletedAt(data)
	}
	if !deleted {
		// A recreated source supersedes this deletion. Keep full parent/child
		// reconciliation required so its current value is republished.
		owner.intent.AttachmentSourceDelete = false
		revision, err := h.updateChildRepairIntent(ctx, owner.key, owner.intent, owner.revision)
		if err != nil {
			return true
		}
		owner.revision = revision
		return false
	}
	id := extractIDFromKey(owner.key, "itx-zoom-past-meetings-attachments.")
	if h.handleMeetingTypeDelete(ctx, owner.key, id, nil, meetingDeleteConfig{
		indexerSubject:   "lfx.index.v1_past_meeting_attachment",
		tombstoneKeyFmts: []string{"v1_past_meeting_attachments.%s"},
	}) {
		return true
	}
	// Do not complete an old delete over a source recreated during retraction.
	current, err := h.v1ObjectsKV.Get(ctx, owner.key)
	if isKVAbsenceError(err) {
		return false
	}
	if err != nil {
		return true
	}
	data, err := decodeData(current.Value())
	if err != nil {
		return true
	}
	_, stillDeleted := softDeletedAt(data)
	return !stillDeleted
}
