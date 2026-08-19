// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	meetingservice "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
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

func TestConvertCreateITXMeetingPayloadToDomain_Owner(t *testing.T) {
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

	t.Run("maps a provided owner to the domain request", func(t *testing.T) {
		p := basePayload()
		p.Owner = &meetingservice.ITXUser{
			Username:       utils.StringPtrOmitEmpty("oowner"),
			Name:           utils.StringPtrOmitEmpty("Olive Owner"),
			Email:          utils.StringPtrOmitEmpty("olive@example.com"),
			ProfilePicture: utils.StringPtrOmitEmpty("https://example.com/olive.png"),
		}

		req := ConvertCreateITXMeetingPayloadToDomain(p)
		require.NotNil(t, req.Owner)
		assert.Equal(t, "oowner", req.Owner.Username)
		assert.Equal(t, "Olive Owner", req.Owner.Name)
		assert.Equal(t, "olive@example.com", req.Owner.Email)
		assert.Equal(t, "https://example.com/olive.png", req.Owner.ProfilePicture)
	})

	t.Run("preserves omission as nil so ITX keeps the stored owner", func(t *testing.T) {
		req := ConvertCreateITXMeetingPayloadToDomain(basePayload())
		assert.Nil(t, req.Owner)
	})
}

func TestConvertITXMeetingResponseToGoa_Owner(t *testing.T) {
	baseResponse := func() *itx.ZoomMeetingResponse {
		return &itx.ZoomMeetingResponse{ID: "1234567890"}
	}

	t.Run("maps a set owner to the Goa response", func(t *testing.T) {
		resp := baseResponse()
		resp.Owner = &itx.User{
			Username: "oowner",
			Name:     "Olive Owner",
			Email:    "olive@example.com",
		}

		goaResp := ConvertITXMeetingResponseToGoa(resp)
		require.NotNil(t, goaResp.Owner)
		assert.Equal(t, "oowner", utils.StringValue(goaResp.Owner.Username))
		assert.Equal(t, "Olive Owner", utils.StringValue(goaResp.Owner.Name))
		assert.Equal(t, "olive@example.com", utils.StringValue(goaResp.Owner.Email))
		assert.Nil(t, goaResp.Owner.ProfilePicture)
	})

	t.Run("leaves owner nil when ITX does not return one", func(t *testing.T) {
		goaResp := ConvertITXMeetingResponseToGoa(baseResponse())
		assert.Nil(t, goaResp.Owner)
	})
}
