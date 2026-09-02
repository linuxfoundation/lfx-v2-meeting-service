// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	meetingservice "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	itxservice "github.com/linuxfoundation/lfx-v2-meeting-service/internal/service/itx"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/utils"
)

// ── ConvertCreateParticipantPayload ──────────────────────────────────────────

func TestConvertCreateParticipantPayload_IsInvitedOnly(t *testing.T) {
	t.Run("is_invited true produces invitee request with all fields", func(t *testing.T) {
		p := &meetingservice.CreateItxPastMeetingParticipantPayload{
			IsInvited:             utils.BoolPtr(true),
			FirstName:             utils.StringPtrOmitEmpty("Alice"),
			LastName:              utils.StringPtrOmitEmpty("Example"),
			Email:                 utils.StringPtrOmitEmpty("alice@example.com"),
			Username:              utils.StringPtrOmitEmpty("alice"),
			LfUserID:              utils.StringPtrOmitEmpty("sf-001"),
			OrgName:               utils.StringPtrOmitEmpty("Example Foundation"),
			JobTitle:              utils.StringPtrOmitEmpty("Engineer"),
			AvatarURL:             utils.StringPtrOmitEmpty("https://example.com/alice.png"),
			CommitteeID:           utils.StringPtrOmitEmpty("cmte-1"),
			CommitteeRole:         utils.StringPtrOmitEmpty("Chair"),
			CommitteeVotingStatus: utils.StringPtrOmitEmpty("voting"),
			OrgIsMember:           utils.BoolPtr(true),
			OrgIsProjectMember:    utils.BoolPtr(true),
		}

		invitee, attendee := ConvertCreateParticipantPayload(p)

		require.NotNil(t, invitee)
		assert.Nil(t, attendee)
		assert.Equal(t, "Alice", invitee.FirstName)
		assert.Equal(t, "Example", invitee.LastName)
		assert.Equal(t, "alice@example.com", invitee.PrimaryEmail)
		assert.Equal(t, "alice", invitee.LFSSO)
		assert.Equal(t, "sf-001", invitee.LFUserID)
		assert.Equal(t, "Example Foundation", invitee.Org)
		assert.Equal(t, "Engineer", invitee.JobTitle)
		assert.Equal(t, "https://example.com/alice.png", invitee.ProfilePicture)
		assert.Equal(t, "cmte-1", invitee.CommitteeID)
		assert.Equal(t, "Chair", invitee.CommitteeRole)
		assert.Equal(t, "voting", invitee.CommitteeVotingStatus)
		assert.True(t, invitee.OrgIsMember)
		assert.True(t, invitee.OrgIsProjectMember)
	})

	t.Run("is_invited false produces no invitee request", func(t *testing.T) {
		p := &meetingservice.CreateItxPastMeetingParticipantPayload{
			IsInvited: utils.BoolPtr(false),
		}
		invitee, _ := ConvertCreateParticipantPayload(p)
		assert.Nil(t, invitee)
	})
}

func TestConvertCreateParticipantPayload_IsAttendedOnly(t *testing.T) {
	t.Run("is_attended true produces attendee request with all identity and committee fields", func(t *testing.T) {
		p := &meetingservice.CreateItxPastMeetingParticipantPayload{
			IsAttended:            utils.BoolPtr(true),
			FirstName:             utils.StringPtrOmitEmpty("Bob"),
			LastName:              utils.StringPtrOmitEmpty("Fixture"),
			Email:                 utils.StringPtrOmitEmpty("bob@example.com"),
			Username:              utils.StringPtrOmitEmpty("bob"),
			LfUserID:              utils.StringPtrOmitEmpty("sf-002"),
			OrgName:               utils.StringPtrOmitEmpty("Test Org"),
			JobTitle:              utils.StringPtrOmitEmpty("Director"),
			AvatarURL:             utils.StringPtrOmitEmpty("https://example.com/bob.png"),
			CommitteeID:           utils.StringPtrOmitEmpty("cmte-2"),
			CommitteeRole:         utils.StringPtrOmitEmpty("Member"),
			CommitteeVotingStatus: utils.StringPtrOmitEmpty("observer"),
			OrgIsMember:           utils.BoolPtr(true),
			OrgIsProjectMember:    utils.BoolPtr(false),
			IsVerified:            utils.BoolPtr(true),
			IsUnknown:             utils.BoolPtr(false),
			Sessions: []*meetingservice.ParticipantSession{
				{
					ParticipantUUID: utils.StringPtrOmitEmpty("uuid-1"),
					JoinTime:        utils.StringPtrOmitEmpty("2026-01-01T10:00:00Z"),
					LeaveTime:       utils.StringPtrOmitEmpty("2026-01-01T11:00:00Z"),
					LeaveReason:     utils.StringPtrOmitEmpty("left meeting"),
				},
			},
		}

		_, attendee := ConvertCreateParticipantPayload(p)

		require.NotNil(t, attendee)
		assert.Equal(t, "Bob Fixture", attendee.Name)
		assert.Equal(t, "bob@example.com", attendee.Email)
		assert.Equal(t, "bob", attendee.LFSSO)
		assert.Equal(t, "sf-002", attendee.LFUserID)
		assert.Equal(t, "Test Org", attendee.Org)
		assert.Equal(t, "Director", attendee.JobTitle)
		assert.Equal(t, "https://example.com/bob.png", attendee.ProfilePicture)
		assert.Equal(t, "cmte-2", attendee.CommitteeID)
		assert.Equal(t, "Member", attendee.CommitteeRole)
		assert.Equal(t, "observer", attendee.CommitteeVotingStatus)
		assert.True(t, attendee.OrgIsMember)
		assert.False(t, attendee.OrgIsProjectMember)
		assert.True(t, attendee.IsVerified)
		assert.False(t, attendee.IsUnknown)
		require.Len(t, attendee.Sessions, 1)
		assert.Equal(t, "uuid-1", attendee.Sessions[0].ParticipantUUID)
		assert.Equal(t, "2026-01-01T10:00:00Z", attendee.Sessions[0].JoinTime)
		assert.Equal(t, "2026-01-01T11:00:00Z", attendee.Sessions[0].LeaveTime)
		assert.Equal(t, "left meeting", attendee.Sessions[0].LeaveReason)
	})

	t.Run("inverted boolean flags are each observed true", func(t *testing.T) {
		// The first subtest has OrgIsProjectMember=false and IsUnknown=false. Because
		// both are plain bool fields (zero value = false), assert.False can't catch a
		// dropped assignment. This subtest flips those two (and complements the others)
		// so every plain-bool assignment is observed true in at least one row.
		p := &meetingservice.CreateItxPastMeetingParticipantPayload{
			IsAttended:         utils.BoolPtr(true),
			OrgIsMember:        utils.BoolPtr(false),
			OrgIsProjectMember: utils.BoolPtr(true),
			IsVerified:         utils.BoolPtr(false),
			IsUnknown:          utils.BoolPtr(true),
		}

		_, attendee := ConvertCreateParticipantPayload(p)

		require.NotNil(t, attendee)
		assert.False(t, attendee.OrgIsMember)
		assert.True(t, attendee.OrgIsProjectMember)
		assert.False(t, attendee.IsVerified)
		assert.True(t, attendee.IsUnknown)
	})

	t.Run("is_attended false produces no attendee request", func(t *testing.T) {
		p := &meetingservice.CreateItxPastMeetingParticipantPayload{
			IsAttended: utils.BoolPtr(false),
		}
		_, attendee := ConvertCreateParticipantPayload(p)
		assert.Nil(t, attendee)
	})
}

func TestConvertCreateParticipantPayload_AttendeeNameCombination(t *testing.T) {
	t.Run("first and last joined with space", func(t *testing.T) {
		p := &meetingservice.CreateItxPastMeetingParticipantPayload{
			IsAttended: utils.BoolPtr(true),
			FirstName:  utils.StringPtrOmitEmpty("Alice"),
			LastName:   utils.StringPtrOmitEmpty("Example"),
		}
		_, attendee := ConvertCreateParticipantPayload(p)
		require.NotNil(t, attendee)
		assert.Equal(t, "Alice Example", attendee.Name)
	})

	t.Run("first name only when last is absent", func(t *testing.T) {
		p := &meetingservice.CreateItxPastMeetingParticipantPayload{
			IsAttended: utils.BoolPtr(true),
			FirstName:  utils.StringPtrOmitEmpty("Alice"),
		}
		_, attendee := ConvertCreateParticipantPayload(p)
		require.NotNil(t, attendee)
		assert.Equal(t, "Alice", attendee.Name)
	})

	t.Run("last name only when first is absent", func(t *testing.T) {
		p := &meetingservice.CreateItxPastMeetingParticipantPayload{
			IsAttended: utils.BoolPtr(true),
			LastName:   utils.StringPtrOmitEmpty("Example"),
		}
		_, attendee := ConvertCreateParticipantPayload(p)
		require.NotNil(t, attendee)
		assert.Equal(t, "Example", attendee.Name)
	})

	t.Run("neither name set leaves name empty", func(t *testing.T) {
		p := &meetingservice.CreateItxPastMeetingParticipantPayload{
			IsAttended: utils.BoolPtr(true),
		}
		_, attendee := ConvertCreateParticipantPayload(p)
		require.NotNil(t, attendee)
		assert.Empty(t, attendee.Name)
	})
}

// ── ConvertUpdateParticipantPayload ──────────────────────────────────────────

func TestConvertUpdateParticipantPayload(t *testing.T) {
	t.Run("invitee-only fields produce invitee request, no attendee", func(t *testing.T) {
		p := &meetingservice.UpdateItxPastMeetingParticipantPayload{
			FirstName: utils.StringPtrOmitEmpty("Alice"),
			LastName:  utils.StringPtrOmitEmpty("Example"),
		}

		invitee, attendee := ConvertUpdateParticipantPayload(p)

		require.NotNil(t, invitee)
		assert.Nil(t, attendee)
		assert.Equal(t, "Alice", invitee.FirstName)
		assert.Equal(t, "Example", invitee.LastName)
	})

	t.Run("attendee-only field (is_verified) produces attendee request, no invitee", func(t *testing.T) {
		p := &meetingservice.UpdateItxPastMeetingParticipantPayload{
			IsVerified: utils.BoolPtr(true),
		}

		invitee, attendee := ConvertUpdateParticipantPayload(p)

		assert.Nil(t, invitee)
		require.NotNil(t, attendee)
		assert.True(t, attendee.IsVerified)
	})

	t.Run("shared fields (org_name) produce both invitee and attendee requests", func(t *testing.T) {
		p := &meetingservice.UpdateItxPastMeetingParticipantPayload{
			OrgName: utils.StringPtrOmitEmpty("Test Org"),
		}

		invitee, attendee := ConvertUpdateParticipantPayload(p)

		require.NotNil(t, invitee)
		require.NotNil(t, attendee)
		assert.Equal(t, "Test Org", invitee.Org)
		assert.Equal(t, "Test Org", attendee.Org)
	})

	t.Run("job_title, committee_role, and committee_voting_status are forwarded to both invitee and attendee", func(t *testing.T) {
		p := &meetingservice.UpdateItxPastMeetingParticipantPayload{
			JobTitle:              utils.StringPtrOmitEmpty("Director"),
			CommitteeRole:         utils.StringPtrOmitEmpty("Chair"),
			CommitteeVotingStatus: utils.StringPtrOmitEmpty("voting_rep"),
		}

		invitee, attendee := ConvertUpdateParticipantPayload(p)

		require.NotNil(t, invitee)
		assert.Equal(t, "Director", invitee.JobTitle)
		assert.Equal(t, "Chair", invitee.CommitteeRole)
		assert.Equal(t, "voting_rep", invitee.CommitteeVotingStatus)
		require.NotNil(t, attendee)
		assert.Equal(t, "Director", attendee.JobTitle)
		assert.Equal(t, "Chair", attendee.CommitteeRole)
		assert.Equal(t, "voting_rep", attendee.CommitteeVotingStatus)
	})

	t.Run("empty payload produces neither request", func(t *testing.T) {
		invitee, attendee := ConvertUpdateParticipantPayload(&meetingservice.UpdateItxPastMeetingParticipantPayload{})
		assert.Nil(t, invitee)
		assert.Nil(t, attendee)
	})

	t.Run("identity fields are copied to invitee for upsert lookup", func(t *testing.T) {
		p := &meetingservice.UpdateItxPastMeetingParticipantPayload{
			Email:    utils.StringPtrOmitEmpty("alice@example.com"),
			Username: utils.StringPtrOmitEmpty("alice"),
			LfUserID: utils.StringPtrOmitEmpty("sf-001"),
		}

		invitee, _ := ConvertUpdateParticipantPayload(p)

		require.NotNil(t, invitee)
		assert.Equal(t, "alice@example.com", invitee.PrimaryEmail)
		assert.Equal(t, "alice", invitee.LFSSO)
		assert.Equal(t, "sf-001", invitee.LFUserID)
	})
}

// ── ConvertParticipantResponseToGoa ─────────────────────────────────────────

func TestConvertParticipantResponseToGoa(t *testing.T) {
	t.Run("maps all fields", func(t *testing.T) {
		resp := &itxservice.ParticipantResponse{
			InviteeID:             "inv-001",
			AttendeeID:            "att-001",
			PastMeetingID:         "zoom-100-occ-200",
			MeetingID:             "zoom-100",
			IsInvited:             false,
			IsAttended:            true,
			FirstName:             "Alice",
			LastName:              "Example",
			Email:                 "alice@example.com",
			Username:              "alice",
			LFUserID:              "sf-001",
			OrgName:               "Example Foundation",
			JobTitle:              "Engineer",
			AvatarURL:             "https://example.com/alice.png",
			OrgIsMember:           false,
			OrgIsProjectMember:    true,
			CommitteeID:           "cmte-1",
			CommitteeRole:         "Chair",
			IsCommitteeMember:     false,
			CommitteeVotingStatus: "voting_rep",
			IsVerified:            true,
			IsUnknown:             false,
			AverageAttendance:     3,
			CreatedAt:             "2026-01-01T00:00:00Z",
			ModifiedAt:            "2026-01-02T00:00:00Z",
		}

		g := ConvertParticipantResponseToGoa(resp)

		// invitee_id takes priority as the canonical ID
		assert.Equal(t, "inv-001", utils.StringValue(g.ID))
		assert.Equal(t, "inv-001", utils.StringValue(g.InviteeID))
		assert.Equal(t, "att-001", utils.StringValue(g.AttendeeID))
		assert.Equal(t, "zoom-100-occ-200", utils.StringValue(g.PastMeetingID))
		assert.Equal(t, "zoom-100", utils.StringValue(g.MeetingID))
		// Adjacent same-type flags are given opposite values so invited/attended,
		// org-member/project-member, and verified/unknown mix-ups are caught.
		// The complementary subtest below flips all seven so every field is
		// observed true in at least one row.
		require.NotNil(t, g.IsInvited)
		assert.False(t, *g.IsInvited)
		require.NotNil(t, g.IsAttended)
		assert.True(t, *g.IsAttended)
		assert.Equal(t, "Alice", utils.StringValue(g.FirstName))
		assert.Equal(t, "Example", utils.StringValue(g.LastName))
		assert.Equal(t, "alice@example.com", utils.StringValue(g.Email))
		assert.Equal(t, "alice", utils.StringValue(g.Username))
		assert.Equal(t, "sf-001", utils.StringValue(g.LfUserID))
		assert.Equal(t, "Example Foundation", utils.StringValue(g.OrgName))
		assert.Equal(t, "Engineer", utils.StringValue(g.JobTitle))
		assert.Equal(t, "https://example.com/alice.png", utils.StringValue(g.AvatarURL))
		assert.Equal(t, "cmte-1", utils.StringValue(g.CommitteeID))
		assert.Equal(t, "Chair", utils.StringValue(g.CommitteeRole))
		assert.Equal(t, "voting_rep", utils.StringValue(g.CommitteeVotingStatus))
		require.NotNil(t, g.OrgIsMember)
		assert.False(t, *g.OrgIsMember)
		require.NotNil(t, g.OrgIsProjectMember)
		assert.True(t, *g.OrgIsProjectMember)
		require.NotNil(t, g.IsCommitteeMember)
		assert.False(t, *g.IsCommitteeMember)
		require.NotNil(t, g.IsVerified)
		assert.True(t, *g.IsVerified)
		require.NotNil(t, g.IsUnknown)
		assert.False(t, *g.IsUnknown)
		require.NotNil(t, g.AverageAttendance)
		assert.Equal(t, 3, *g.AverageAttendance)
		assert.Equal(t, "2026-01-01T00:00:00Z", utils.StringValue(g.CreatedAt))
		assert.Equal(t, "2026-01-02T00:00:00Z", utils.StringValue(g.ModifiedAt))
	})

	t.Run("maps all boolean fields — complementary pattern T/F/T/F/T/F/T", func(t *testing.T) {
		resp := &itxservice.ParticipantResponse{
			IsInvited:          true,
			IsAttended:         false,
			OrgIsMember:        true,
			OrgIsProjectMember: false,
			IsCommitteeMember:  true,
			IsVerified:         false,
			IsUnknown:          true,
		}

		g := ConvertParticipantResponseToGoa(resp)

		require.NotNil(t, g.IsInvited)
		assert.True(t, *g.IsInvited)
		require.NotNil(t, g.IsAttended)
		assert.False(t, *g.IsAttended)
		require.NotNil(t, g.OrgIsMember)
		assert.True(t, *g.OrgIsMember)
		require.NotNil(t, g.OrgIsProjectMember)
		assert.False(t, *g.OrgIsProjectMember)
		require.NotNil(t, g.IsCommitteeMember)
		assert.True(t, *g.IsCommitteeMember)
		require.NotNil(t, g.IsVerified)
		assert.False(t, *g.IsVerified)
		require.NotNil(t, g.IsUnknown)
		assert.True(t, *g.IsUnknown)
	})

	t.Run("attendee_id used as ID when invitee_id is empty", func(t *testing.T) {
		resp := &itxservice.ParticipantResponse{AttendeeID: "att-002"}
		g := ConvertParticipantResponseToGoa(resp)
		assert.Equal(t, "att-002", utils.StringValue(g.ID))
	})

	t.Run("ID is nil when both invitee_id and attendee_id are empty", func(t *testing.T) {
		g := ConvertParticipantResponseToGoa(&itxservice.ParticipantResponse{})
		assert.Nil(t, g.ID)
	})

	t.Run("zero average_attendance is omitted (nil)", func(t *testing.T) {
		g := ConvertParticipantResponseToGoa(&itxservice.ParticipantResponse{AverageAttendance: 0})
		assert.Nil(t, g.AverageAttendance)
	})

	t.Run("sessions are converted", func(t *testing.T) {
		resp := &itxservice.ParticipantResponse{
			Sessions: []itx.AttendeeSession{
				{
					ParticipantUUID: "uuid-1",
					JoinTime:        "2026-01-01T10:00:00Z",
					LeaveTime:       "2026-01-01T11:00:00Z",
					LeaveReason:     "left",
				},
			},
		}

		g := ConvertParticipantResponseToGoa(resp)

		require.Len(t, g.Sessions, 1)
		assert.Equal(t, "uuid-1", utils.StringValue(g.Sessions[0].ParticipantUUID))
		assert.Equal(t, "2026-01-01T10:00:00Z", utils.StringValue(g.Sessions[0].JoinTime))
		assert.Equal(t, "2026-01-01T11:00:00Z", utils.StringValue(g.Sessions[0].LeaveTime))
		assert.Equal(t, "left", utils.StringValue(g.Sessions[0].LeaveReason))
	})

	t.Run("audit users are mapped when present", func(t *testing.T) {
		resp := &itxservice.ParticipantResponse{
			CreatedBy:  &itx.User{Username: "alice", Name: "Alice Example", Email: "alice@example.com"},
			ModifiedBy: &itx.User{Username: "bob"},
		}

		g := ConvertParticipantResponseToGoa(resp)

		require.NotNil(t, g.CreatedBy)
		assert.Equal(t, "alice", utils.StringValue(g.CreatedBy.Username))
		assert.Equal(t, "Alice Example", utils.StringValue(g.CreatedBy.Name))
		assert.Equal(t, "alice@example.com", utils.StringValue(g.CreatedBy.Email))
		require.NotNil(t, g.ModifiedBy)
		assert.Equal(t, "bob", utils.StringValue(g.ModifiedBy.Username))
	})

	t.Run("audit users nil when absent", func(t *testing.T) {
		g := ConvertParticipantResponseToGoa(&itxservice.ParticipantResponse{})
		assert.Nil(t, g.CreatedBy)
		assert.Nil(t, g.ModifiedBy)
	})
}
