// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
)

type pastMeetingCleanupCompletion struct {
	Complete       bool   `json:"complete"`
	SourceRevision uint64 `json:"source_revision"`
}

// cleanupCompletedForRevision accepts proof only for a known current deletion
// revision. Legacy boolean markers and unavailable source history cannot prove
// that artifacts published during a later unprocessed restore were cleaned.
func (h *EventHandlers) cleanupCompletedForRevision(ctx context.Context, parentID string, revision uint64) (bool, error) {
	entry, err := h.v1MappingsKV.Get(ctx, pastMeetingCleanupKeyPrefix+parentID)
	if isKVAbsenceError(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if string(entry.Value()) == "1" || revision == 0 {
		return false, nil
	}
	var completion pastMeetingCleanupCompletion
	if err := json.Unmarshal(entry.Value(), &completion); err != nil {
		return false, err
	}
	return completion.Complete && completion.SourceRevision == revision, nil
}
