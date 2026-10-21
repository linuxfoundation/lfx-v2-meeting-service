// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertMapToMeetingAttachmentData_OccurrenceScope(t *testing.T) {
	base := func() map[string]interface{} {
		return map[string]interface{}{
			"id":         "att-1",
			"meeting_id": "mtg-1",
			"type":       "link",
			"name":       "notes",
			"created_at": "2026-10-01T10:00:00Z",
			"updated_at": "2026-10-01T10:00:00Z",
		}
	}

	t.Run("carries the occurrence an attachment is scoped to", func(t *testing.T) {
		raw := base()
		raw["occurrence_id"] = "1788000000"

		data, err := convertMapToMeetingAttachmentData(raw)
		require.NoError(t, err)
		assert.Equal(t, "1788000000", data.OccurrenceID)
	})

	t.Run("a record without occurrence_id is series-wide", func(t *testing.T) {
		data, err := convertMapToMeetingAttachmentData(base())
		require.NoError(t, err)
		assert.Empty(t, data.OccurrenceID)
	})
}
