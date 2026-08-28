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

// ── ConvertCreateITXMeetingPayloadToDomain — committees & recurrence ─────────

func TestConvertCreateITXMeetingPayloadToDomain_Committees(t *testing.T) {
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

	t.Run("maps committees with allowed_voting_statuses", func(t *testing.T) {
		p := basePayload()
		uid := "cmte-1"
		p.Committees = []*meetingservice.Committee{
			{
				UID:                   &uid,
				AllowedVotingStatuses: []meetingservice.AllowedVotingStatus{"voting_rep", "observer"},
			},
		}

		req := ConvertCreateITXMeetingPayloadToDomain(p)

		require.Len(t, req.Committees, 1)
		assert.Equal(t, "cmte-1", req.Committees[0].UID)
		assert.Equal(t, []itx.CommitteeFilter{"voting_rep", "observer"}, req.Committees[0].AllowedVotingStatuses)
	})

	t.Run("nil committees produce empty slice", func(t *testing.T) {
		req := ConvertCreateITXMeetingPayloadToDomain(basePayload())
		assert.Empty(t, req.Committees)
	})
}

func TestConvertCreateITXMeetingPayloadToDomain_Recurrence(t *testing.T) {
	basePayload := func() *meetingservice.CreateItxMeetingPayload {
		return &meetingservice.CreateItxMeetingPayload{
			ProjectUID: "proj-1",
			Title:      "Recurring",
			StartTime:  "2026-01-01T00:00:00Z",
			Duration:   60,
			Timezone:   "UTC",
			Visibility: "public",
		}
	}

	t.Run("maps all recurrence fields when present", func(t *testing.T) {
		p := basePayload()
		p.Recurrence = &meetingservice.Recurrence{
			Type:           utils.IntPtrOmitZero(3),
			RepeatInterval: utils.IntPtrOmitZero(1),
			WeeklyDays:     utils.StringPtrOmitEmpty("1,2"),
			MonthlyDay:     utils.IntPtrOmitZero(15),
			MonthlyWeek:    utils.IntPtrOmitZero(2),
			MonthlyWeekDay: utils.IntPtrOmitZero(4),
			EndTimes:       utils.IntPtrOmitZero(10),
			EndDateTime:    utils.StringPtrOmitEmpty("2026-12-31T00:00:00Z"),
		}

		req := ConvertCreateITXMeetingPayloadToDomain(p)

		require.NotNil(t, req.Recurrence)
		assert.Equal(t, itx.RecurrenceType(3), req.Recurrence.Type)
		assert.Equal(t, 1, req.Recurrence.RepeatInterval)
		assert.Equal(t, "1,2", req.Recurrence.WeeklyDays)
		assert.Equal(t, 15, req.Recurrence.MonthlyDay)
		assert.Equal(t, 2, req.Recurrence.MonthlyWeek)
		assert.Equal(t, 4, req.Recurrence.MonthlyWeekDay)
		assert.Equal(t, 10, req.Recurrence.EndTimes)
		assert.Equal(t, "2026-12-31T00:00:00Z", req.Recurrence.EndDateTime)
	})

	t.Run("nil recurrence leaves field nil", func(t *testing.T) {
		req := ConvertCreateITXMeetingPayloadToDomain(basePayload())
		assert.Nil(t, req.Recurrence)
	})
}

func TestConvertCreateITXMeetingPayloadToDomain_ScalarFields(t *testing.T) {
	t.Run("maps all scalar fields", func(t *testing.T) {
		p := &meetingservice.CreateItxMeetingPayload{
			ProjectUID:               "proj-1",
			Title:                    "Board Meeting",
			StartTime:                "2026-06-01T10:00:00Z",
			Duration:                 90,
			Timezone:                 "America/New_York",
			Visibility:               "private",
			Description:              utils.StringPtrOmitEmpty("Quarterly review"),
			Restricted:               utils.BoolPtr(true),
			MeetingType:              utils.StringPtrOmitEmpty(string(itx.MeetingTypeBoard)),
			EarlyJoinTimeMinutes:     utils.IntPtrOmitZero(10),
			RecordingEnabled:         utils.BoolPtr(true),
			TranscriptEnabled:        utils.BoolPtr(false),
			YoutubeUploadEnabled:     utils.BoolPtr(true),
			AiSummaryEnabled:         utils.BoolPtr(false),
			RequireAiSummaryApproval: utils.BoolPtr(true),
			ArtifactVisibility:       utils.StringPtrOmitEmpty(string(itx.ArtifactAccessHosts)),
		}

		req := ConvertCreateITXMeetingPayloadToDomain(p)

		assert.Equal(t, "proj-1", req.ProjectUID)
		assert.Equal(t, "Board Meeting", req.Title)
		assert.Equal(t, "2026-06-01T10:00:00Z", req.StartTime)
		assert.Equal(t, 90, req.Duration)
		assert.Equal(t, "America/New_York", req.Timezone)
		assert.Equal(t, itx.MeetingVisibility("private"), req.Visibility)
		assert.Equal(t, "Quarterly review", req.Description)
		assert.True(t, req.Restricted)
		assert.Equal(t, itx.MeetingTypeBoard, req.MeetingType)
		assert.Equal(t, 10, req.EarlyJoinTimeMinutes)
		assert.True(t, req.RecordingEnabled)
		assert.False(t, req.TranscriptEnabled)
		assert.True(t, req.YoutubeUploadEnabled)
		assert.False(t, req.AISummaryEnabled)
		assert.True(t, req.RequireAISummaryApproval)
		assert.Equal(t, itx.ArtifactAccessHosts, req.ArtifactVisibility)
	})
}

// Three-case boolean cross-wiring guard. Column assignments are chosen so that
// every pair of fields has distinct values in at least one case, making any
// source-field swap detectable:
//
//	Restricted     T T F
//	RecordingEnabled  T F T
//	TranscriptEnabled F T T
//	YoutubeUploadEnabled  T F F
//	AISummaryEnabled  F T F
//	RequireAISummaryApproval  F F T
func TestConvertCreateITXMeetingPayloadToDomain_BooleanFields(t *testing.T) {
	base := func() *meetingservice.CreateItxMeetingPayload {
		return &meetingservice.CreateItxMeetingPayload{
			ProjectUID: "proj-1", Title: "T", StartTime: "2026-01-01T00:00:00Z",
			Duration: 30, Timezone: "UTC", Visibility: "public",
		}
	}

	t.Run("Rst=T Rec=T Trans=F YT=T AI=F Req=F", func(t *testing.T) {
		p := base()
		p.Restricted = utils.BoolPtr(true)
		p.RecordingEnabled = utils.BoolPtr(true)
		p.TranscriptEnabled = utils.BoolPtr(false)
		p.YoutubeUploadEnabled = utils.BoolPtr(true)
		p.AiSummaryEnabled = utils.BoolPtr(false)
		p.RequireAiSummaryApproval = utils.BoolPtr(false)
		req := ConvertCreateITXMeetingPayloadToDomain(p)
		assert.True(t, req.Restricted)
		assert.True(t, req.RecordingEnabled)
		assert.False(t, req.TranscriptEnabled)
		assert.True(t, req.YoutubeUploadEnabled)
		assert.False(t, req.AISummaryEnabled)
		assert.False(t, req.RequireAISummaryApproval)
	})

	t.Run("Rst=T Rec=F Trans=T YT=F AI=T Req=F", func(t *testing.T) {
		p := base()
		p.Restricted = utils.BoolPtr(true)
		p.RecordingEnabled = utils.BoolPtr(false)
		p.TranscriptEnabled = utils.BoolPtr(true)
		p.YoutubeUploadEnabled = utils.BoolPtr(false)
		p.AiSummaryEnabled = utils.BoolPtr(true)
		p.RequireAiSummaryApproval = utils.BoolPtr(false)
		req := ConvertCreateITXMeetingPayloadToDomain(p)
		assert.True(t, req.Restricted)
		assert.False(t, req.RecordingEnabled)
		assert.True(t, req.TranscriptEnabled)
		assert.False(t, req.YoutubeUploadEnabled)
		assert.True(t, req.AISummaryEnabled)
		assert.False(t, req.RequireAISummaryApproval)
	})

	t.Run("Rst=F Rec=T Trans=T YT=F AI=F Req=T", func(t *testing.T) {
		p := base()
		p.Restricted = utils.BoolPtr(false)
		p.RecordingEnabled = utils.BoolPtr(true)
		p.TranscriptEnabled = utils.BoolPtr(true)
		p.YoutubeUploadEnabled = utils.BoolPtr(false)
		p.AiSummaryEnabled = utils.BoolPtr(false)
		p.RequireAiSummaryApproval = utils.BoolPtr(true)
		req := ConvertCreateITXMeetingPayloadToDomain(p)
		assert.False(t, req.Restricted)
		assert.True(t, req.RecordingEnabled)
		assert.True(t, req.TranscriptEnabled)
		assert.False(t, req.YoutubeUploadEnabled)
		assert.False(t, req.AISummaryEnabled)
		assert.True(t, req.RequireAISummaryApproval)
	})
}

// ── ConvertITXMeetingResponseToGoa — committees, recurrence & occurrences ────

func TestConvertITXMeetingResponseToGoa_Committees(t *testing.T) {
	t.Run("maps committees and passes known filters through", func(t *testing.T) {
		resp := &itx.ZoomMeetingResponse{
			ID: "mtg-1",
			Committees: []itx.Committee{
				{ID: "cmte-1", Filters: []itx.CommitteeFilter{itx.CommitteeFilterVotingRep, itx.CommitteeFilterObserver}},
			},
		}

		g := ConvertITXMeetingResponseToGoa(resp)

		require.Len(t, g.Committees, 1)
		require.NotNil(t, g.Committees[0].UID)
		assert.Equal(t, "cmte-1", *g.Committees[0].UID)
		assert.Equal(t, []meetingservice.AllowedVotingStatus{"voting_rep", "observer"}, g.Committees[0].AllowedVotingStatuses)
	})

	t.Run("nil committees produce nil slice in response", func(t *testing.T) {
		g := ConvertITXMeetingResponseToGoa(&itx.ZoomMeetingResponse{})
		assert.Nil(t, g.Committees)
	})
}

func TestConvertITXMeetingResponseToGoa_Recurrence(t *testing.T) {
	t.Run("maps all recurrence fields when present", func(t *testing.T) {
		resp := &itx.ZoomMeetingResponse{
			Recurrence: &itx.Recurrence{
				Type:           itx.RecurrenceType(3),
				RepeatInterval: 1,
				WeeklyDays:     "1,3",
				MonthlyDay:     15,
				MonthlyWeek:    2,
				MonthlyWeekDay: 4,
				EndTimes:       5,
				EndDateTime:    "2026-12-31T00:00:00Z",
			},
		}

		g := ConvertITXMeetingResponseToGoa(resp)

		require.NotNil(t, g.Recurrence)
		require.NotNil(t, g.Recurrence.Type)
		assert.Equal(t, 3, *g.Recurrence.Type)
		require.NotNil(t, g.Recurrence.RepeatInterval)
		assert.Equal(t, 1, *g.Recurrence.RepeatInterval)
		assert.Equal(t, "1,3", utils.StringValue(g.Recurrence.WeeklyDays))
		assert.Equal(t, 15, utils.IntValue(g.Recurrence.MonthlyDay))
		assert.Equal(t, 2, utils.IntValue(g.Recurrence.MonthlyWeek))
		assert.Equal(t, 4, utils.IntValue(g.Recurrence.MonthlyWeekDay))
		assert.Equal(t, 5, utils.IntValue(g.Recurrence.EndTimes))
		assert.Equal(t, "2026-12-31T00:00:00Z", utils.StringValue(g.Recurrence.EndDateTime))
	})

	t.Run("nil recurrence leaves field nil", func(t *testing.T) {
		g := ConvertITXMeetingResponseToGoa(&itx.ZoomMeetingResponse{})
		assert.Nil(t, g.Recurrence)
	})
}

func TestConvertITXMeetingResponseToGoa_Occurrences(t *testing.T) {
	t.Run("maps occurrences slice", func(t *testing.T) {
		resp := &itx.ZoomMeetingResponse{
			Occurrences: []itx.Occurrence{
				{
					OccurrenceID:    "occ-1",
					StartTime:       "2026-06-01T10:00:00Z",
					Duration:        60,
					Status:          itx.OccurrenceStatusAvailable,
					RegistrantCount: 5,
				},
			},
		}

		g := ConvertITXMeetingResponseToGoa(resp)

		require.Len(t, g.Occurrences, 1)
		occ := g.Occurrences[0]
		assert.Equal(t, "occ-1", utils.StringValue(occ.OccurrenceID))
		assert.Equal(t, "2026-06-01T10:00:00Z", utils.StringValue(occ.StartTime))
		require.NotNil(t, occ.Duration)
		assert.Equal(t, 60, *occ.Duration)
		assert.Equal(t, string(itx.OccurrenceStatusAvailable), utils.StringValue(occ.Status))
		require.NotNil(t, occ.RegistrantCount)
		assert.Equal(t, 5, *occ.RegistrantCount)
	})

	t.Run("nil occurrences produce nil slice", func(t *testing.T) {
		g := ConvertITXMeetingResponseToGoa(&itx.ZoomMeetingResponse{})
		assert.Nil(t, g.Occurrences)
	})
}

func TestConvertITXMeetingResponseToGoa_ArtifactVisibilityCoalesce(t *testing.T) {
	t.Run("uses first non-empty of recording, transcript, ai_summary access", func(t *testing.T) {
		resp := &itx.ZoomMeetingResponse{
			RecordingAccess:  itx.ArtifactAccessHosts,
			TranscriptAccess: itx.ArtifactAccessPublic,
		}
		g := ConvertITXMeetingResponseToGoa(resp)
		assert.Equal(t, string(itx.ArtifactAccessHosts), utils.StringValue(g.ArtifactVisibility))
	})

	t.Run("transcript_access wins over ai_summary_access when recording is empty", func(t *testing.T) {
		resp := &itx.ZoomMeetingResponse{
			TranscriptAccess: itx.ArtifactAccessPublic,
			AISummaryAccess:  itx.ArtifactAccessParticipants, // lower-priority; must not win
		}
		g := ConvertITXMeetingResponseToGoa(resp)
		assert.Equal(t, string(itx.ArtifactAccessPublic), utils.StringValue(g.ArtifactVisibility))
	})

	t.Run("falls back to ai_summary_access when recording and transcript are both empty", func(t *testing.T) {
		resp := &itx.ZoomMeetingResponse{
			AISummaryAccess: itx.ArtifactAccessParticipants,
		}
		g := ConvertITXMeetingResponseToGoa(resp)
		assert.Equal(t, string(itx.ArtifactAccessParticipants), utils.StringValue(g.ArtifactVisibility))
	})

	t.Run("nil when all access fields are empty", func(t *testing.T) {
		g := ConvertITXMeetingResponseToGoa(&itx.ZoomMeetingResponse{})
		assert.Nil(t, g.ArtifactVisibility)
	})
}

// ── ConvertGetJoinLinkPayloadToITX ───────────────────────────────────────────

func TestConvertGetJoinLinkPayloadToITX(t *testing.T) {
	t.Run("maps meeting_id and all optional fields", func(t *testing.T) {
		p := &meetingservice.GetItxJoinLinkPayload{
			MeetingID: "zoom-100",
			UseEmail:  utils.BoolPtr(true),
			UserID:    utils.StringPtrOmitEmpty("sf-001"),
			Name:      utils.StringPtrOmitEmpty("Alice Example"),
			Email:     utils.StringPtrOmitEmpty("alice@example.com"),
			Register:  utils.BoolPtr(true),
		}

		req := ConvertGetJoinLinkPayloadToITX(p)

		assert.Equal(t, "zoom-100", req.MeetingID)
		assert.True(t, req.UseEmail)
		assert.Equal(t, "sf-001", req.UserID)
		assert.Equal(t, "Alice Example", req.Name)
		assert.Equal(t, "alice@example.com", req.Email)
		assert.True(t, req.Register)
	})

	t.Run("use_email and register are independent — cross-wiring is detectable", func(t *testing.T) {
		p := &meetingservice.GetItxJoinLinkPayload{
			MeetingID: "zoom-100",
			UseEmail:  utils.BoolPtr(true),
			Register:  utils.BoolPtr(false),
		}

		req := ConvertGetJoinLinkPayloadToITX(p)

		assert.True(t, req.UseEmail)
		assert.False(t, req.Register)
	})

	t.Run("nil optional fields leave ITX fields as zero values", func(t *testing.T) {
		req := ConvertGetJoinLinkPayloadToITX(&meetingservice.GetItxJoinLinkPayload{MeetingID: "zoom-100"})

		assert.Equal(t, "zoom-100", req.MeetingID)
		assert.False(t, req.UseEmail)
		assert.Empty(t, req.UserID)
		assert.Empty(t, req.Name)
		assert.Empty(t, req.Email)
		assert.False(t, req.Register)
	})
}

// ── ConvertITXJoinLinkResponseToGoa ─────────────────────────────────────────

func TestConvertITXJoinLinkResponseToGoa(t *testing.T) {
	t.Run("passes link through unchanged", func(t *testing.T) {
		resp := &itx.ZoomMeetingJoinLink{Link: "https://zoom.us/j/123?pwd=abc"}
		g := ConvertITXJoinLinkResponseToGoa(resp)
		assert.Equal(t, "https://zoom.us/j/123?pwd=abc", g.Link)
	})
}

// ── ConvertUpdateOccurrencePayloadToITX ──────────────────────────────────────

func TestConvertUpdateOccurrencePayloadToITX(t *testing.T) {
	t.Run("empty payload produces zero-value request", func(t *testing.T) {
		req := ConvertUpdateOccurrencePayloadToITX(&meetingservice.UpdateItxOccurrencePayload{})
		assert.Empty(t, req.StartTime)
		assert.Zero(t, req.Duration)
		assert.Empty(t, req.Topic)
		assert.Empty(t, req.Agenda)
		assert.Nil(t, req.Recurrence)
	})

	t.Run("maps all optional scalar fields", func(t *testing.T) {
		p := &meetingservice.UpdateItxOccurrencePayload{
			StartTime: utils.StringPtrOmitEmpty("2026-06-15T09:00:00Z"),
			Duration:  utils.IntPtrOmitZero(45),
			Topic:     utils.StringPtrOmitEmpty("Rescheduled"),
			Agenda:    utils.StringPtrOmitEmpty("New agenda"),
		}

		req := ConvertUpdateOccurrencePayloadToITX(p)

		assert.Equal(t, "2026-06-15T09:00:00Z", req.StartTime)
		assert.Equal(t, 45, req.Duration)
		assert.Equal(t, "Rescheduled", req.Topic)
		assert.Equal(t, "New agenda", req.Agenda)
		assert.Nil(t, req.Recurrence)
	})

	t.Run("maps all recurrence fields when present", func(t *testing.T) {
		p := &meetingservice.UpdateItxOccurrencePayload{
			Recurrence: &meetingservice.Recurrence{
				Type:           utils.IntPtrOmitZero(2),
				RepeatInterval: utils.IntPtrOmitZero(1),
				WeeklyDays:     utils.StringPtrOmitEmpty("1,3"),
				MonthlyDay:     utils.IntPtrOmitZero(15),
				MonthlyWeek:    utils.IntPtrOmitZero(3),
				MonthlyWeekDay: utils.IntPtrOmitZero(4),
				EndTimes:       utils.IntPtrOmitZero(5),
				EndDateTime:    utils.StringPtrOmitEmpty("2026-12-31T00:00:00Z"),
			},
		}

		req := ConvertUpdateOccurrencePayloadToITX(p)

		require.NotNil(t, req.Recurrence)
		assert.Equal(t, itx.RecurrenceType(2), req.Recurrence.Type)
		assert.Equal(t, 1, req.Recurrence.RepeatInterval)
		assert.Equal(t, "1,3", req.Recurrence.WeeklyDays)
		assert.Equal(t, 15, req.Recurrence.MonthlyDay)
		assert.Equal(t, 3, req.Recurrence.MonthlyWeek)
		assert.Equal(t, 4, req.Recurrence.MonthlyWeekDay)
		assert.Equal(t, 5, req.Recurrence.EndTimes)
		assert.Equal(t, "2026-12-31T00:00:00Z", req.Recurrence.EndDateTime)
	})
}

// ── ConvertITXMeetingResponseToGoa — boolean fields ──────────────────────────

// Three-case boolean cross-wiring guard for the response converter. The eight
// booleans split into two groups by null semantics:
//
//	Always-pointer (&resp.X): RecordingEnabled, TranscriptEnabled,
//	  YoutubeUploadEnabled, AiSummaryEnabled (ZoomAIEnabled source)
//	OmitFalse (*bool, nil when false): Restricted, RequireAiSummaryApproval,
//	  AutoEmailReminderEnabled, IsInviteResponsesEnabled
//
// Column assignments give every field a unique 4-bit pattern so any
// source-field swap is detectable across all 28 pairs:
//
//	Restricted            T T F F
//	RecordingEnabled      T F T T
//	TranscriptEnabled     F T T F
//	YoutubeUploadEnabled  T F F F
//	AiSummaryEnabled      F T F F
//	RequireAiSummaryAppr  F F T T
//	AutoEmailReminder     T F F T
//	IsInviteResponses     F T F T
func TestConvertITXMeetingResponseToGoa_BooleanFields(t *testing.T) {
	t.Run("Rst=T Rec=T Trans=F YT=T AI=F Req=F AER=T IR=F", func(t *testing.T) {
		resp := &itx.ZoomMeetingResponse{
			Restricted:               true,
			RecordingEnabled:         true,
			TranscriptEnabled:        false,
			YoutubeUploadEnabled:     true,
			ZoomAIEnabled:            false,
			RequireAISummaryApproval: false,
			AutoEmailReminderEnabled: true,
			IsInviteResponsesEnabled: false,
		}
		g := ConvertITXMeetingResponseToGoa(resp)
		require.NotNil(t, g.Restricted)
		assert.True(t, *g.Restricted)
		require.NotNil(t, g.RecordingEnabled)
		assert.True(t, *g.RecordingEnabled)
		require.NotNil(t, g.TranscriptEnabled)
		assert.False(t, *g.TranscriptEnabled)
		require.NotNil(t, g.YoutubeUploadEnabled)
		assert.True(t, *g.YoutubeUploadEnabled)
		require.NotNil(t, g.AiSummaryEnabled)
		assert.False(t, *g.AiSummaryEnabled)
		assert.Nil(t, g.RequireAiSummaryApproval)
		require.NotNil(t, g.AutoEmailReminderEnabled)
		assert.True(t, *g.AutoEmailReminderEnabled)
		assert.Nil(t, g.IsInviteResponsesEnabled)
	})

	t.Run("Rst=T Rec=F Trans=T YT=F AI=T Req=F AER=F IR=T", func(t *testing.T) {
		resp := &itx.ZoomMeetingResponse{
			Restricted:               true,
			RecordingEnabled:         false,
			TranscriptEnabled:        true,
			YoutubeUploadEnabled:     false,
			ZoomAIEnabled:            true,
			RequireAISummaryApproval: false,
			AutoEmailReminderEnabled: false,
			IsInviteResponsesEnabled: true,
		}
		g := ConvertITXMeetingResponseToGoa(resp)
		require.NotNil(t, g.Restricted)
		assert.True(t, *g.Restricted)
		require.NotNil(t, g.RecordingEnabled)
		assert.False(t, *g.RecordingEnabled)
		require.NotNil(t, g.TranscriptEnabled)
		assert.True(t, *g.TranscriptEnabled)
		require.NotNil(t, g.YoutubeUploadEnabled)
		assert.False(t, *g.YoutubeUploadEnabled)
		require.NotNil(t, g.AiSummaryEnabled)
		assert.True(t, *g.AiSummaryEnabled)
		assert.Nil(t, g.RequireAiSummaryApproval)
		assert.Nil(t, g.AutoEmailReminderEnabled)
		require.NotNil(t, g.IsInviteResponsesEnabled)
		assert.True(t, *g.IsInviteResponsesEnabled)
	})

	t.Run("Rst=F Rec=T Trans=T YT=F AI=F Req=T AER=F IR=F", func(t *testing.T) {
		resp := &itx.ZoomMeetingResponse{
			Restricted:               false,
			RecordingEnabled:         true,
			TranscriptEnabled:        true,
			YoutubeUploadEnabled:     false,
			ZoomAIEnabled:            false,
			RequireAISummaryApproval: true,
			AutoEmailReminderEnabled: false,
			IsInviteResponsesEnabled: false,
		}
		g := ConvertITXMeetingResponseToGoa(resp)
		assert.Nil(t, g.Restricted)
		require.NotNil(t, g.RecordingEnabled)
		assert.True(t, *g.RecordingEnabled)
		require.NotNil(t, g.TranscriptEnabled)
		assert.True(t, *g.TranscriptEnabled)
		require.NotNil(t, g.YoutubeUploadEnabled)
		assert.False(t, *g.YoutubeUploadEnabled)
		require.NotNil(t, g.AiSummaryEnabled)
		assert.False(t, *g.AiSummaryEnabled)
		require.NotNil(t, g.RequireAiSummaryApproval)
		assert.True(t, *g.RequireAiSummaryApproval)
		assert.Nil(t, g.AutoEmailReminderEnabled)
		assert.Nil(t, g.IsInviteResponsesEnabled)
	})

	// Case 4 breaks the two duplicate columns from the 3-case matrix:
	// YT and AER were both T F F; AI and IR were both F T F.
	t.Run("Rst=F Rec=T Trans=F YT=F AI=F Req=T AER=T IR=T", func(t *testing.T) {
		resp := &itx.ZoomMeetingResponse{
			Restricted:               false,
			RecordingEnabled:         true,
			TranscriptEnabled:        false,
			YoutubeUploadEnabled:     false,
			ZoomAIEnabled:            false,
			RequireAISummaryApproval: true,
			AutoEmailReminderEnabled: true,
			IsInviteResponsesEnabled: true,
		}
		g := ConvertITXMeetingResponseToGoa(resp)
		assert.Nil(t, g.Restricted)
		require.NotNil(t, g.RecordingEnabled)
		assert.True(t, *g.RecordingEnabled)
		require.NotNil(t, g.TranscriptEnabled)
		assert.False(t, *g.TranscriptEnabled)
		require.NotNil(t, g.YoutubeUploadEnabled)
		assert.False(t, *g.YoutubeUploadEnabled)
		require.NotNil(t, g.AiSummaryEnabled)
		assert.False(t, *g.AiSummaryEnabled)
		require.NotNil(t, g.RequireAiSummaryApproval)
		assert.True(t, *g.RequireAiSummaryApproval)
		require.NotNil(t, g.AutoEmailReminderEnabled)
		assert.True(t, *g.AutoEmailReminderEnabled)
		require.NotNil(t, g.IsInviteResponsesEnabled)
		assert.True(t, *g.IsInviteResponsesEnabled)
	})
}

// ── ConvertSubmitITXMeetingResponsePayloadToITX ──────────────────────────────

func TestConvertSubmitITXMeetingResponsePayloadToITX(t *testing.T) {
	t.Run("maps response, scope, and registrant_id", func(t *testing.T) {
		p := &meetingservice.SubmitItxMeetingResponsePayload{
			Response:     "accepted",
			Scope:        "all",
			RegistrantID: "reg-001",
		}

		req := ConvertSubmitITXMeetingResponsePayloadToITX(p)

		assert.Equal(t, "accepted", req.Response)
		assert.Equal(t, "all", req.Scope)
		assert.Equal(t, "reg-001", req.RegistrantID)
	})
}

// ── ConvertITXMeetingResponseResultToGoa ─────────────────────────────────────

func TestConvertITXMeetingResponseResultToGoa(t *testing.T) {
	t.Run("maps all fields", func(t *testing.T) {
		r := &itx.MeetingResponseResult{
			ID:           "resp-001",
			MeetingID:    "zoom-100",
			RegistrantID: "reg-001",
			Username:     "alice",
			Email:        "alice@example.com",
			Response:     "accepted",
			Scope:        "all",
			OccurrenceID: "occ-1",
			CreatedAt:    "2026-01-01T00:00:00Z",
			UpdatedAt:    "2026-01-02T00:00:00Z",
		}

		g := ConvertITXMeetingResponseResultToGoa(r)

		assert.Equal(t, "resp-001", g.ID)
		assert.Equal(t, "zoom-100", g.MeetingID)
		assert.Equal(t, "reg-001", g.RegistrantID)
		assert.Equal(t, "alice", utils.StringValue(g.Username))
		assert.Equal(t, "alice@example.com", utils.StringValue(g.Email))
		assert.Equal(t, "accepted", g.Response)
		assert.Equal(t, "all", g.Scope)
		assert.Equal(t, "occ-1", utils.StringValue(g.OccurrenceID))
		assert.Equal(t, "2026-01-01T00:00:00Z", utils.StringValue(g.CreatedAt))
		assert.Equal(t, "2026-01-02T00:00:00Z", utils.StringValue(g.UpdatedAt))
	})

	t.Run("empty optional string fields become nil via StringPtrOmitEmpty", func(t *testing.T) {
		g := ConvertITXMeetingResponseResultToGoa(&itx.MeetingResponseResult{})
		assert.Nil(t, g.Username)
		assert.Nil(t, g.Email)
		assert.Nil(t, g.OccurrenceID)
		assert.Nil(t, g.CreatedAt)
		assert.Nil(t, g.UpdatedAt)
	})
}

// ── filterVotingStatuses ─────────────────────────────────────────────────────

func TestFilterVotingStatuses(t *testing.T) {
	t.Run("all known values pass through", func(t *testing.T) {
		filters := []itx.CommitteeFilter{
			itx.CommitteeFilterVotingRep,
			itx.CommitteeFilterAltVotingRep,
			itx.CommitteeFilterObserver,
			itx.CommitteeFilterEmeritus,
			itx.CommitteeFilterNone,
		}

		result := filterVotingStatuses(filters)

		require.Len(t, result, 5)
		assert.Equal(t, meetingservice.AllowedVotingStatus("voting_rep"), result[0])
		assert.Equal(t, meetingservice.AllowedVotingStatus("alt_voting_rep"), result[1])
		assert.Equal(t, meetingservice.AllowedVotingStatus("observer"), result[2])
		assert.Equal(t, meetingservice.AllowedVotingStatus("emeritus"), result[3])
		assert.Equal(t, meetingservice.AllowedVotingStatus("none"), result[4])
	})

	t.Run("unknown values are dropped to avoid violating the OpenAPI contract", func(t *testing.T) {
		filters := []itx.CommitteeFilter{
			itx.CommitteeFilterVotingRep,
			"unrecognized_future_value",
			itx.CommitteeFilterObserver,
		}

		result := filterVotingStatuses(filters)

		require.Len(t, result, 2)
		assert.Equal(t, meetingservice.AllowedVotingStatus("voting_rep"), result[0])
		assert.Equal(t, meetingservice.AllowedVotingStatus("observer"), result[1])
	})

	t.Run("nil input returns empty (not nil) slice", func(t *testing.T) {
		result := filterVotingStatuses(nil)
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})
}
