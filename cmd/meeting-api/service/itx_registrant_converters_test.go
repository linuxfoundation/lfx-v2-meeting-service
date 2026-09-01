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

// ── ConvertCreateITXRegistrantPayloadToITX ──────────────────────────────────

func TestConvertCreateITXRegistrantPayloadToITX(t *testing.T) {
	t.Run("maps all scalar fields", func(t *testing.T) {
		p := &meetingservice.CreateItxRegistrantPayload{
			CommitteeUID:   utils.StringPtrOmitEmpty("cmte-1"),
			Email:          utils.StringPtrOmitEmpty("alice@example.com"),
			Username:       utils.StringPtrOmitEmpty("alice"),
			FirstName:      utils.StringPtrOmitEmpty("Alice"),
			LastName:       utils.StringPtrOmitEmpty("Example"),
			Org:            utils.StringPtrOmitEmpty("Example Foundation"),
			JobTitle:       utils.StringPtrOmitEmpty("Engineer"),
			ProfilePicture: utils.StringPtrOmitEmpty("https://example.com/alice.png"),
			Host:           utils.BoolPtr(true),
			Occurrence:     utils.StringPtrOmitEmpty("occ-123"),
		}

		req := ConvertCreateITXRegistrantPayloadToITX(p)

		assert.Equal(t, "cmte-1", req.CommitteeID, "committee_uid must map to committee_id")
		assert.Equal(t, "alice@example.com", req.Email)
		assert.Equal(t, "alice", req.Username)
		assert.Equal(t, "Alice", req.FirstName)
		assert.Equal(t, "Example", req.LastName)
		assert.Equal(t, "Example Foundation", req.Org)
		assert.Equal(t, "Engineer", req.JobTitle)
		assert.Equal(t, "https://example.com/alice.png", req.ProfilePicture)
		assert.True(t, req.Host)
		assert.Equal(t, "occ-123", req.Occurrence)
	})

	t.Run("nil pointer fields become empty strings and false", func(t *testing.T) {
		p := &meetingservice.CreateItxRegistrantPayload{}

		req := ConvertCreateITXRegistrantPayloadToITX(p)

		assert.Empty(t, req.CommitteeID)
		assert.Empty(t, req.Email)
		assert.Empty(t, req.Username)
		assert.False(t, req.Host)
		assert.Empty(t, req.Occurrence)
	})

	t.Run("host false and omitted host both send false to ITX", func(t *testing.T) {
		// The destination is a plain bool (not *bool), so utils.BoolValue(nil) == false.
		// Both inputs collapse to the same wire value — this test pins that equivalence.
		explicit := ConvertCreateITXRegistrantPayloadToITX(&meetingservice.CreateItxRegistrantPayload{Host: utils.BoolPtr(false)})
		omitted := ConvertCreateITXRegistrantPayloadToITX(&meetingservice.CreateItxRegistrantPayload{})
		assert.False(t, explicit.Host)
		assert.False(t, omitted.Host)
	})
}

// ── ConvertUpdateITXRegistrantPayloadToITX ──────────────────────────────────

func TestConvertUpdateITXRegistrantPayloadToITX(t *testing.T) {
	t.Run("maps all scalar fields the same way as create", func(t *testing.T) {
		p := &meetingservice.UpdateItxRegistrantPayload{
			CommitteeUID:   utils.StringPtrOmitEmpty("cmte-2"),
			Email:          utils.StringPtrOmitEmpty("bob@example.com"),
			Username:       utils.StringPtrOmitEmpty("bob"),
			FirstName:      utils.StringPtrOmitEmpty("Bob"),
			LastName:       utils.StringPtrOmitEmpty("Fixture"),
			Org:            utils.StringPtrOmitEmpty("Test Org"),
			JobTitle:       utils.StringPtrOmitEmpty("Manager"),
			ProfilePicture: utils.StringPtrOmitEmpty("https://example.com/bob.png"),
			Host:           utils.BoolPtr(true),
			Occurrence:     utils.StringPtrOmitEmpty("occ-456"),
		}

		req := ConvertUpdateITXRegistrantPayloadToITX(p)

		assert.Equal(t, "cmte-2", req.CommitteeID, "committee_uid must map to committee_id")
		assert.Equal(t, "bob@example.com", req.Email)
		assert.Equal(t, "bob", req.Username)
		assert.Equal(t, "Bob", req.FirstName)
		assert.Equal(t, "Fixture", req.LastName)
		assert.Equal(t, "Test Org", req.Org)
		assert.Equal(t, "Manager", req.JobTitle)
		assert.True(t, req.Host)
		assert.Equal(t, "occ-456", req.Occurrence)
	})

	t.Run("nil pointer fields become empty strings", func(t *testing.T) {
		req := ConvertUpdateITXRegistrantPayloadToITX(&meetingservice.UpdateItxRegistrantPayload{})

		assert.Empty(t, req.CommitteeID)
		assert.Empty(t, req.Email)
		assert.False(t, req.Host)
	})
}

// ── ConvertSelfRegisterPayloadToITX ─────────────────────────────────────────

func TestConvertSelfRegisterPayloadToITX(t *testing.T) {
	t.Run("maps name, org, job title, and occurrence", func(t *testing.T) {
		p := &meetingservice.SelfRegisterItxMeetingPayload{
			FirstName:  "Carol",
			LastName:   "Example",
			Org:        utils.StringPtrOmitEmpty("Example Foundation"),
			JobTitle:   utils.StringPtrOmitEmpty("Architect"),
			Occurrence: utils.StringPtrOmitEmpty("occ-789"),
		}

		req := ConvertSelfRegisterPayloadToITX(p)

		assert.Equal(t, "Carol", req.FirstName)
		assert.Equal(t, "Example", req.LastName)
		assert.Equal(t, "Example Foundation", req.Org)
		assert.Equal(t, "Architect", req.JobTitle)
		assert.Equal(t, "occ-789", req.Occurrence)
	})

	t.Run("email and username are absent — caller cannot set them", func(t *testing.T) {
		p := &meetingservice.SelfRegisterItxMeetingPayload{
			FirstName: "Carol",
			LastName:  "Example",
		}

		req := ConvertSelfRegisterPayloadToITX(p)

		assert.Empty(t, req.Email, "email must not be set by self-register converter")
		assert.Empty(t, req.Username, "username must not be set by self-register converter")
	})

	t.Run("nil optional fields become empty strings", func(t *testing.T) {
		p := &meetingservice.SelfRegisterItxMeetingPayload{
			FirstName: "Dave",
			LastName:  "Test",
		}

		req := ConvertSelfRegisterPayloadToITX(p)

		assert.Empty(t, req.Org)
		assert.Empty(t, req.JobTitle)
		assert.Empty(t, req.Occurrence)
	})
}

// ── ConvertITXRegistrantToGoa ────────────────────────────────────────────────

func TestConvertITXRegistrantToGoa(t *testing.T) {
	t.Run("maps all scalar fields", func(t *testing.T) {
		resp := &itx.ZoomMeetingRegistrant{
			ID:                            "reg-001",
			Type:                          itx.RegistrantTypeDirect,
			CommitteeID:                   "cmte-3",
			Email:                         "alice@example.com",
			Username:                      "alice",
			FirstName:                     "Alice",
			LastName:                      "Example",
			Org:                           "Example Foundation",
			JobTitle:                      "Engineer",
			ProfilePicture:                "https://example.com/alice.png",
			Host:                          true,
			Occurrence:                    "occ-123",
			AttendedOccurrenceCount:       3,
			TotalOccurrenceCount:          5,
			LastInviteReceivedTime:        "2026-01-01T00:00:00Z",
			LastInviteReceivedMessageID:   "msg-abc",
			LastInviteDeliveryStatus:      "delivered",
			LastInviteDeliveryDescription: "ok",
			CreatedAt:                     "2026-01-01T00:00:00Z",
			ModifiedAt:                    "2026-01-02T00:00:00Z",
		}

		g := ConvertITXRegistrantToGoa(resp)

		require.NotNil(t, g.UID)
		assert.Equal(t, "reg-001", *g.UID)
		assert.Equal(t, string(itx.RegistrantTypeDirect), utils.StringValue(g.Type))
		assert.Equal(t, "cmte-3", utils.StringValue(g.CommitteeUID), "committee_id must map back to committee_uid")
		assert.Equal(t, "alice@example.com", utils.StringValue(g.Email))
		assert.Equal(t, "alice", utils.StringValue(g.Username))
		assert.Equal(t, "Alice", utils.StringValue(g.FirstName))
		assert.Equal(t, "Example", utils.StringValue(g.LastName))
		assert.Equal(t, "Example Foundation", utils.StringValue(g.Org))
		assert.Equal(t, "Engineer", utils.StringValue(g.JobTitle))
		assert.Equal(t, "https://example.com/alice.png", utils.StringValue(g.ProfilePicture))
		require.NotNil(t, g.Host)
		assert.True(t, *g.Host)
		assert.Equal(t, "occ-123", utils.StringValue(g.Occurrence))
		require.NotNil(t, g.AttendedOccurrenceCount)
		assert.Equal(t, 3, *g.AttendedOccurrenceCount)
		require.NotNil(t, g.TotalOccurrenceCount)
		assert.Equal(t, 5, *g.TotalOccurrenceCount)
		assert.Equal(t, "2026-01-01T00:00:00Z", utils.StringValue(g.LastInviteReceivedTime))
		assert.Equal(t, "msg-abc", utils.StringValue(g.LastInviteReceivedMessageID))
		assert.Equal(t, "delivered", utils.StringValue(g.LastInviteDeliveryStatus))
		assert.Equal(t, "ok", utils.StringValue(g.LastInviteDeliveryDescription))
		assert.Equal(t, "2026-01-01T00:00:00Z", utils.StringValue(g.CreatedAt))
		assert.Equal(t, "2026-01-02T00:00:00Z", utils.StringValue(g.ModifiedAt))
	})

	t.Run("maps created_by and updated_by when present", func(t *testing.T) {
		resp := &itx.ZoomMeetingRegistrant{
			CreatedBy: &itx.User{
				Username:       "alice",
				Name:           "Alice Example",
				Email:          "alice@example.com",
				ProfilePicture: "https://example.com/alice.png",
			},
			UpdatedBy: &itx.User{
				Username: "bob",
				Name:     "Bob Fixture",
				Email:    "bob@example.com",
			},
		}

		g := ConvertITXRegistrantToGoa(resp)

		require.NotNil(t, g.CreatedBy)
		assert.Equal(t, "alice", utils.StringValue(g.CreatedBy.Username))
		assert.Equal(t, "Alice Example", utils.StringValue(g.CreatedBy.Name))
		assert.Equal(t, "alice@example.com", utils.StringValue(g.CreatedBy.Email))
		assert.Equal(t, "https://example.com/alice.png", utils.StringValue(g.CreatedBy.ProfilePicture))

		require.NotNil(t, g.UpdatedBy)
		assert.Equal(t, "bob", utils.StringValue(g.UpdatedBy.Username))
	})

	t.Run("leaves created_by and updated_by nil when absent", func(t *testing.T) {
		g := ConvertITXRegistrantToGoa(&itx.ZoomMeetingRegistrant{})

		assert.Nil(t, g.CreatedBy)
		assert.Nil(t, g.UpdatedBy)
	})

	t.Run("zero-value attendance counts are omitted (nil) in Goa response", func(t *testing.T) {
		g := ConvertITXRegistrantToGoa(&itx.ZoomMeetingRegistrant{})

		assert.Nil(t, g.AttendedOccurrenceCount)
		assert.Nil(t, g.TotalOccurrenceCount)
	})
}
