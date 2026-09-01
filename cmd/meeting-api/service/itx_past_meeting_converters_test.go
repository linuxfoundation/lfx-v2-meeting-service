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

// ── ConvertCreatePastMeetingPayload ─────────────────────────────────────────

func TestConvertCreatePastMeetingPayload(t *testing.T) {
	basePayload := func() *meetingservice.CreateItxPastMeetingPayload {
		return &meetingservice.CreateItxPastMeetingPayload{
			MeetingID:    "zoom-100",
			OccurrenceID: "occ-200",
			ProjectUID:   "proj-1",
			StartTime:    "2026-06-01T10:00:00Z",
			Duration:     60,
			Timezone:     "America/New_York",
		}
	}

	t.Run("maps required fields", func(t *testing.T) {
		req := ConvertCreatePastMeetingPayload(basePayload())

		assert.Equal(t, "zoom-100", req.MeetingID)
		assert.Equal(t, "occ-200", req.OccurrenceID)
		assert.Equal(t, "proj-1", req.ProjectID)
		assert.Equal(t, "2026-06-01T10:00:00Z", req.StartTime)
		assert.Equal(t, 60, req.Duration)
		assert.Equal(t, "America/New_York", req.Timezone)
	})

	t.Run("maps optional fields when provided", func(t *testing.T) {
		p := basePayload()
		p.Title = utils.StringPtrOmitEmpty("Board Meeting")
		p.Description = utils.StringPtrOmitEmpty("Quarterly review")
		p.Restricted = utils.BoolPtr(true)
		p.MeetingType = utils.StringPtrOmitEmpty(string(itx.MeetingTypeBoard))
		p.Visibility = utils.StringPtrOmitEmpty("public")
		p.RecordingEnabled = utils.BoolPtr(true)
		p.TranscriptEnabled = utils.BoolPtr(true)
		p.ArtifactVisibility = utils.StringPtrOmitEmpty(string(itx.ArtifactAccessHosts))

		req := ConvertCreatePastMeetingPayload(p)

		assert.Equal(t, "Board Meeting", req.Topic)
		assert.Equal(t, "Quarterly review", req.Agenda)
		assert.True(t, req.Restricted)
		assert.Equal(t, itx.MeetingTypeBoard, req.MeetingType)
		assert.Equal(t, itx.MeetingVisibility("public"), req.Visibility)
		assert.True(t, req.RecordingEnabled)
		assert.True(t, req.TranscriptEnabled)
	})

	t.Run("artifact_visibility fans out to both recording_access and transcript_access", func(t *testing.T) {
		p := basePayload()
		p.ArtifactVisibility = utils.StringPtrOmitEmpty(string(itx.ArtifactAccessHosts))

		req := ConvertCreatePastMeetingPayload(p)

		assert.Equal(t, itx.ArtifactAccessHosts, req.RecordingAccess)
		assert.Equal(t, itx.ArtifactAccessHosts, req.TranscriptAccess)
	})

	t.Run("nil artifact_visibility leaves both access fields empty", func(t *testing.T) {
		req := ConvertCreatePastMeetingPayload(basePayload())

		assert.Empty(t, req.RecordingAccess)
		assert.Empty(t, req.TranscriptAccess)
	})

	t.Run("maps committees with allowed_voting_statuses", func(t *testing.T) {
		p := basePayload()
		uid := "cmte-10"
		p.Committees = []*meetingservice.Committee{
			{
				UID:                   &uid,
				AllowedVotingStatuses: []meetingservice.AllowedVotingStatus{"voting_rep", "observer"},
			},
		}

		req := ConvertCreatePastMeetingPayload(p)

		require.Len(t, req.Committees, 1)
		assert.Equal(t, "cmte-10", req.Committees[0].ID)
		assert.Equal(t, []itx.CommitteeFilter{"voting_rep", "observer"}, req.Committees[0].Filters)
	})

	t.Run("nil committees result in no committees on request", func(t *testing.T) {
		req := ConvertCreatePastMeetingPayload(basePayload())
		assert.Empty(t, req.Committees)
	})

	t.Run("nil and nil-UID committee entries are skipped; filters are forwarded", func(t *testing.T) {
		p := basePayload()
		uid := "cmte-10"
		p.Committees = []*meetingservice.Committee{
			nil,
			{UID: nil},
			{UID: &uid, AllowedVotingStatuses: []meetingservice.AllowedVotingStatus{"voting_rep"}},
		}

		req := ConvertCreatePastMeetingPayload(p)

		require.Len(t, req.Committees, 1, "nil and nil-UID entries must be skipped")
		assert.Equal(t, "cmte-10", req.Committees[0].ID)
		assert.Equal(t, []itx.CommitteeFilter{"voting_rep"}, req.Committees[0].Filters)
	})
}

// ── ConvertUpdatePastMeetingPayload ─────────────────────────────────────────

func TestConvertUpdatePastMeetingPayload(t *testing.T) {
	t.Run("empty payload produces empty request", func(t *testing.T) {
		req := ConvertUpdatePastMeetingPayload(&meetingservice.UpdateItxPastMeetingPayload{})

		assert.Empty(t, req.MeetingID)
		assert.Empty(t, req.OccurrenceID)
		assert.Empty(t, req.ProjectID)
		assert.Empty(t, req.Topic)
		assert.Empty(t, req.Agenda)
		assert.False(t, req.Restricted)
		assert.Empty(t, req.Committees)
	})

	t.Run("maps all optional fields when provided", func(t *testing.T) {
		p := &meetingservice.UpdateItxPastMeetingPayload{
			MeetingID:          utils.StringPtrOmitEmpty("zoom-101"),
			OccurrenceID:       utils.StringPtrOmitEmpty("occ-201"),
			ProjectUID:         utils.StringPtrOmitEmpty("proj-2"),
			StartTime:          utils.StringPtrOmitEmpty("2026-07-01T09:00:00Z"),
			Duration:           utils.IntPtrOmitZero(90),
			Timezone:           utils.StringPtrOmitEmpty("UTC"),
			Title:              utils.StringPtrOmitEmpty("Updated Meeting"),
			Description:        utils.StringPtrOmitEmpty("Updated agenda"),
			Restricted:         utils.BoolPtr(true),
			MeetingType:        utils.StringPtrOmitEmpty("webinar"),
			Visibility:         utils.StringPtrOmitEmpty("private"),
			RecordingEnabled:   utils.BoolPtr(false),
			TranscriptEnabled:  utils.BoolPtr(true),
			ArtifactVisibility: utils.StringPtrOmitEmpty(string(itx.ArtifactAccessPublic)),
		}

		req := ConvertUpdatePastMeetingPayload(p)

		assert.Equal(t, "zoom-101", req.MeetingID)
		assert.Equal(t, "occ-201", req.OccurrenceID)
		assert.Equal(t, "proj-2", req.ProjectID)
		assert.Equal(t, "2026-07-01T09:00:00Z", req.StartTime)
		assert.Equal(t, 90, req.Duration)
		assert.Equal(t, "UTC", req.Timezone)
		assert.Equal(t, "Updated Meeting", req.Topic)
		assert.Equal(t, "Updated agenda", req.Agenda)
		assert.True(t, req.Restricted)
		assert.Equal(t, itx.MeetingType("webinar"), req.MeetingType)
		assert.Equal(t, itx.MeetingVisibility("private"), req.Visibility)
		assert.False(t, req.RecordingEnabled)
		assert.True(t, req.TranscriptEnabled)
		assert.Equal(t, itx.ArtifactAccessPublic, req.RecordingAccess)
		assert.Equal(t, itx.ArtifactAccessPublic, req.TranscriptAccess)
	})

	t.Run("committees with nil entries are skipped", func(t *testing.T) {
		uid := "cmte-20"
		p := &meetingservice.UpdateItxPastMeetingPayload{
			Committees: []*meetingservice.Committee{
				nil,
				{UID: nil},
				{UID: &uid},
			},
		}

		req := ConvertUpdatePastMeetingPayload(p)

		require.Len(t, req.Committees, 1, "nil entry and entry with nil UID must be skipped")
		assert.Equal(t, "cmte-20", req.Committees[0].ID)
	})
}

// ── ConvertPastMeetingToGoa ──────────────────────────────────────────────────

func TestConvertPastMeetingToGoa(t *testing.T) {
	t.Run("maps all scalar fields", func(t *testing.T) {
		resp := &itx.PastMeetingResponse{
			PastMeetingID:     "zoom-100-occ-200",
			MeetingID:         "zoom-100",
			OccurrenceID:      "occ-200",
			ProjectID:         "proj-1",
			Topic:             "Board Meeting",
			Agenda:            "Quarterly review",
			StartTime:         "2026-06-01T10:00:00Z",
			Duration:          60,
			Timezone:          "America/New_York",
			Visibility:        itx.MeetingVisibility("public"),
			Restricted:        true,
			MeetingType:       itx.MeetingTypeBoard,
			RecordingEnabled:  true,
			TranscriptEnabled: true,
			RecordingAccess:   itx.ArtifactAccessHosts,
			IsManuallyCreated: true,
			MeetingPassword:   "secret-uuid",
		}

		g := ConvertPastMeetingToGoa(resp)

		assert.Equal(t, "zoom-100-occ-200", utils.StringValue(g.ID))
		assert.Equal(t, "zoom-100", utils.StringValue(g.MeetingID))
		assert.Equal(t, "occ-200", utils.StringValue(g.OccurrenceID))
		assert.Equal(t, "proj-1", utils.StringValue(g.ProjectUID))
		assert.Equal(t, "Board Meeting", utils.StringValue(g.Title))
		assert.Equal(t, "Quarterly review", utils.StringValue(g.Description))
		assert.Equal(t, "2026-06-01T10:00:00Z", utils.StringValue(g.StartTime))
		require.NotNil(t, g.Duration)
		assert.Equal(t, 60, *g.Duration)
		assert.Equal(t, "America/New_York", utils.StringValue(g.Timezone))
		assert.Equal(t, "public", utils.StringValue(g.Visibility))
		require.NotNil(t, g.Restricted)
		assert.True(t, *g.Restricted)
		assert.Equal(t, string(itx.MeetingTypeBoard), utils.StringValue(g.MeetingType))
		require.NotNil(t, g.RecordingEnabled)
		assert.True(t, *g.RecordingEnabled)
		require.NotNil(t, g.TranscriptEnabled)
		assert.True(t, *g.TranscriptEnabled)
		assert.Equal(t, string(itx.ArtifactAccessHosts), utils.StringValue(g.ArtifactVisibility))
		require.NotNil(t, g.IsManuallyCreated)
		assert.True(t, *g.IsManuallyCreated)
		assert.Equal(t, "secret-uuid", utils.StringValue(g.MeetingPassword))
	})

	t.Run("maps committees", func(t *testing.T) {
		resp := &itx.PastMeetingResponse{
			Committees: []itx.Committee{
				{ID: "cmte-10", Filters: []itx.CommitteeFilter{"voting_rep"}},
				{ID: "cmte-11"},
			},
		}

		g := ConvertPastMeetingToGoa(resp)

		require.Len(t, g.Committees, 2)
		require.NotNil(t, g.Committees[0].UID)
		assert.Equal(t, "cmte-10", *g.Committees[0].UID)
		require.NotNil(t, g.Committees[1].UID)
		assert.Equal(t, "cmte-11", *g.Committees[1].UID)
	})

	t.Run("nil committees produce nil slice in Goa response", func(t *testing.T) {
		g := ConvertPastMeetingToGoa(&itx.PastMeetingResponse{})
		assert.Nil(t, g.Committees)
	})

	t.Run("empty string fields become nil pointers via StringPtrOmitEmpty", func(t *testing.T) {
		g := ConvertPastMeetingToGoa(&itx.PastMeetingResponse{})

		assert.Nil(t, g.ID)
		assert.Nil(t, g.MeetingID)
		assert.Nil(t, g.Title)
		assert.Nil(t, g.Description)
		assert.Nil(t, g.ArtifactVisibility)
		assert.Nil(t, g.MeetingPassword)
	})
}
