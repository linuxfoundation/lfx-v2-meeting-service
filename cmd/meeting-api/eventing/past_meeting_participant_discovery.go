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

const pastMeetingParticipantDiscoveryPrefix = "v1-mappings.past-meeting-participants-discovered."

type participantDiscovery struct {
	Owner          childRepairIntent `json:"owner"`
	SourceRevision uint64            `json:"source_revision"`
}

// parentSourceRevision includes retained DEL/PURGE history, whose revision must
// distinguish a new hard-deletion generation from an earlier completed scan.
func (h *EventHandlers) parentSourceRevision(ctx context.Context, parentID string) (uint64, error) {
	key := pastMeetingObjectKeyPrefix + parentID
	entry, err := h.v1ObjectsKV.Get(ctx, key)
	if err == nil {
		return entry.Revision(), nil
	}
	if !isKVAbsenceError(err) {
		return 0, err
	}
	history, err := h.v1ObjectsKV.History(ctx, key)
	if isKVAbsenceError(err) || errors.Is(err, jetstream.ErrNoKeysFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var revision uint64
	for _, entry := range history {
		if entry.Revision() > revision {
			revision = entry.Revision()
		}
	}
	return revision, nil
}

// ensurePastMeetingParticipantsDiscovered coalesces global legacy discovery for
// a deletion generation. The owner performs metadata-only tracking; no access
// revocation is allowed before completion is durable. Restoration invalidates
// the marker, so an old scanner cannot complete over a newer generation.
func (h *EventHandlers) ensurePastMeetingParticipantsDiscovered(ctx context.Context, parentID string) (retry bool) {
	sourceRevision, err := h.parentSourceRevision(ctx, parentID)
	if err != nil {
		return true
	}
	key := pastMeetingParticipantDiscoveryPrefix + parentID
	entry, err := h.v1MappingsKV.Get(ctx, key)
	var previous participantDiscovery
	if err == nil {
		if json.Unmarshal(entry.Value(), &previous) != nil {
			return true
		}
		if previous.Owner.Phase == "discovered" && previous.SourceRevision == sourceRevision {
			return false
		}
		if previous.Owner.Phase != "discovered" {
			h.logger.WarnContext(ctx, "participant discovery is owned; retry without takeover", "discovery_key", key, "worker", previous.Owner.Worker)
			return true
		}
	} else if !isKVAbsenceError(err) {
		return true
	}
	state := participantDiscovery{Owner: newChildRepairIntent(parentID, "discovering"), SourceRevision: sourceRevision}
	value, _ := json.Marshal(state)
	var revision uint64
	if entry == nil {
		revision, err = h.v1MappingsKV.Create(ctx, key, value)
	} else {
		revision, err = h.v1MappingsKV.Update(ctx, key, value, entry.Revision())
	}
	if err != nil {
		return true
	}
	complete := false
	defer func() {
		if !complete {
			// A failed scan has made no destructive publications. Release only
			// its revision; an uncertain release remains fail-closed for ops.
			if err := h.v1MappingsKV.Delete(ctx, key, jetstream.LastRevision(revision)); err != nil && !isKVAbsenceError(err) {
				h.logger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to release participant discovery owner", "discovery_key", key)
			}
		}
	}()
	for _, child := range pastMeetingChildKinds {
		if !child.participant() {
			continue
		}
		ids, err := listIndexedPastMeetingChildIDs(ctx, h.v1MappingsKV, child, parentID)
		if err != nil {
			return true
		}
		if _, err := h.discoverPastMeetingChildren(ctx, parentID, child, ids); err != nil {
			return true
		}
	}
	state.Owner.Phase = "discovered"
	value, _ = json.Marshal(state)
	if _, err := h.v1MappingsKV.Update(ctx, key, value, revision); err != nil {
		return true
	}
	complete = true
	return false
}

func (h *EventHandlers) invalidatePastMeetingParticipantDiscovery(ctx context.Context, parentID string) error {
	key := pastMeetingParticipantDiscoveryPrefix + parentID
	entry, err := h.v1MappingsKV.Get(ctx, key)
	if isKVAbsenceError(err) {
		return nil
	}
	if err != nil {
		return err
	}
	// A scanner only tracks metadata until its CAS completion. Invalidating
	// it on a live parent cancels that scan before it authorizes revocation.
	return h.v1MappingsKV.Delete(ctx, key, jetstream.LastRevision(entry.Revision()))
}
