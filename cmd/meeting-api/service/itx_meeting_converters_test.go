// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	meetingservice "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

func TestConvertCreateITXMeetingPayloadToDomain_AutoEmailReminder(t *testing.T) {
	basePayload := func() *meetingservice.CreateItxMeetingPayload {
		return &meetingservice.CreateItxMeetingPayload{
			ProjectUID: "proj-1",
			Title:      "Test Meeting",
			StartTime:  "2026-01-01T00:00:00Z",
			Duration:   30,
			Timezone:   "UTC",
			Visibility: "public",
		}
	}

	t.Run("maps enabled reminder with time", func(t *testing.T) {
		p := basePayload()
		p.AutoEmailReminderEnabled = utils.BoolPtr(true)
		p.AutoEmailReminderTime = utils.IntPtrOmitZero(150)

		req := ConvertCreateITXMeetingPayloadToDomain(p)
		require.NotNil(t, req.AutoEmailReminderEnabled)
		assert.True(t, *req.AutoEmailReminderEnabled)
		assert.Equal(t, 150, req.AutoEmailReminderTime)
	})

	t.Run("maps explicit false distinctly from omission", func(t *testing.T) {
		p := basePayload()
		p.AutoEmailReminderEnabled = utils.BoolPtr(false)

		req := ConvertCreateITXMeetingPayloadToDomain(p)
		require.NotNil(t, req.AutoEmailReminderEnabled)
		assert.False(t, *req.AutoEmailReminderEnabled)
	})

	t.Run("preserves omission as nil when fields are absent", func(t *testing.T) {
		req := ConvertCreateITXMeetingPayloadToDomain(basePayload())
		assert.Nil(t, req.AutoEmailReminderEnabled)
		assert.Equal(t, 0, req.AutoEmailReminderTime)
	})
}
