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

// ── Meeting attachment — Goa→ITX ─────────────────────────────────────────────

func TestConvertGoaToITXCreateMeetingAttachment(t *testing.T) {
	t.Run("maps required fields", func(t *testing.T) {
		p := &meetingservice.CreateItxMeetingAttachmentPayload{
			Type:     "file",
			Category: "Meeting Minutes",
			Name:     "agenda.pdf",
		}

		req := ConvertGoaToITXCreateMeetingAttachment(p)

		assert.Equal(t, "file", req.Type)
		assert.Equal(t, "Meeting Minutes", req.Category)
		assert.Equal(t, "agenda.pdf", req.Name)
		assert.Empty(t, req.Link)
		assert.Empty(t, req.Description)
		assert.Nil(t, req.CreatedBy, "created_by is stamped by the service layer, not the converter")
	})

	t.Run("maps optional link and description when provided", func(t *testing.T) {
		p := &meetingservice.CreateItxMeetingAttachmentPayload{
			Type:        "link",
			Category:    "Notes",
			Name:        "meeting notes",
			Link:        utils.StringPtrOmitEmpty("https://example.com/notes"),
			Description: utils.StringPtrOmitEmpty("Draft notes"),
		}

		req := ConvertGoaToITXCreateMeetingAttachment(p)

		assert.Equal(t, "https://example.com/notes", req.Link)
		assert.Equal(t, "Draft notes", req.Description)
	})
}

func TestConvertGoaToITXUpdateMeetingAttachment(t *testing.T) {
	t.Run("maps required fields", func(t *testing.T) {
		p := &meetingservice.UpdateItxMeetingAttachmentPayload{
			Type:     "file",
			Category: "Presentation",
			Name:     "slides.pptx",
		}

		req := ConvertGoaToITXUpdateMeetingAttachment(p)

		assert.Equal(t, "file", req.Type)
		assert.Equal(t, "Presentation", req.Category)
		assert.Equal(t, "slides.pptx", req.Name)
		assert.Empty(t, req.Link)
		assert.Nil(t, req.UpdatedBy, "updated_by is stamped by the service layer, not the converter")
	})

	t.Run("maps optional link and description", func(t *testing.T) {
		p := &meetingservice.UpdateItxMeetingAttachmentPayload{
			Type:        "link",
			Category:    "Other",
			Name:        "ref",
			Link:        utils.StringPtrOmitEmpty("https://example.com/ref"),
			Description: utils.StringPtrOmitEmpty("Reference doc"),
		}

		req := ConvertGoaToITXUpdateMeetingAttachment(p)

		assert.Equal(t, "https://example.com/ref", req.Link)
		assert.Equal(t, "Reference doc", req.Description)
	})
}

func TestConvertGoaToITXCreateMeetingAttachmentPresign(t *testing.T) {
	t.Run("maps all fields", func(t *testing.T) {
		p := &meetingservice.CreateItxMeetingAttachmentPresignPayload{
			Name:        "report.pdf",
			FileSize:    1024,
			FileType:    "application/pdf",
			Description: utils.StringPtrOmitEmpty("Annual report"),
			Category:    utils.StringPtrOmitEmpty("Notes"),
		}

		req := ConvertGoaToITXCreateMeetingAttachmentPresign(p)

		assert.Equal(t, "report.pdf", req.Name)
		assert.Equal(t, int64(1024), req.FileSize)
		assert.Equal(t, "application/pdf", req.FileType)
		assert.Equal(t, "Annual report", req.Description)
		assert.Equal(t, "Notes", req.Category)
	})

	t.Run("nil description and category become empty strings", func(t *testing.T) {
		p := &meetingservice.CreateItxMeetingAttachmentPresignPayload{
			Name:     "file.pdf",
			FileSize: 512,
			FileType: "application/pdf",
		}

		req := ConvertGoaToITXCreateMeetingAttachmentPresign(p)

		assert.Empty(t, req.Description)
		assert.Empty(t, req.Category)
	})
}

// ── Meeting attachment — ITX→Goa ─────────────────────────────────────────────

func TestConvertITXMeetingAttachmentToGoa(t *testing.T) {
	t.Run("maps all scalar fields", func(t *testing.T) {
		resp := &itx.MeetingAttachment{
			ID:               "att-001",
			MeetingID:        "zoom-100",
			Type:             "file",
			Source:           "api",
			Category:         "Meeting Minutes",
			Name:             "agenda.pdf",
			FileUploaded:     true,
			Link:             "https://example.com/agenda.pdf",
			Description:      "Agenda document",
			FileName:         "agenda.pdf",
			FileSize:         2048,
			FileURL:          "s3://bucket/agenda.pdf",
			FileUploadStatus: "completed",
			FileContentType:  "application/pdf",
			CreatedAt:        "2026-01-01T00:00:00Z",
			UpdatedAt:        "2026-01-02T00:00:00Z",
			FileUploadedAt:   "2026-01-01T01:00:00Z",
		}

		g := ConvertITXMeetingAttachmentToGoa(resp)

		assert.Equal(t, "att-001", g.UID)
		assert.Equal(t, "zoom-100", g.MeetingID)
		assert.Equal(t, "file", g.Type)
		assert.Equal(t, "api", utils.StringValue(g.Source))
		assert.Equal(t, "Meeting Minutes", g.Category)
		assert.Equal(t, "agenda.pdf", g.Name)
		require.NotNil(t, g.FileUploaded)
		assert.True(t, *g.FileUploaded)
		assert.Equal(t, "https://example.com/agenda.pdf", utils.StringValue(g.Link))
		assert.Equal(t, "Agenda document", utils.StringValue(g.Description))
		assert.Equal(t, "agenda.pdf", utils.StringValue(g.FileName))
		require.NotNil(t, g.FileSize)
		assert.Equal(t, int64(2048), *g.FileSize)
		assert.Equal(t, "s3://bucket/agenda.pdf", utils.StringValue(g.FileURL))
		assert.Equal(t, "completed", utils.StringValue(g.FileUploadStatus))
		assert.Equal(t, "application/pdf", utils.StringValue(g.FileContentType))
		assert.Equal(t, "2026-01-01T00:00:00Z", utils.StringValue(g.CreatedAt))
		assert.Equal(t, "2026-01-02T00:00:00Z", utils.StringValue(g.UpdatedAt))
		assert.Equal(t, "2026-01-01T01:00:00Z", utils.StringValue(g.FileUploadedAt))
	})

	t.Run("file_uploaded false is omitted (nil) in Goa response", func(t *testing.T) {
		g := ConvertITXMeetingAttachmentToGoa(&itx.MeetingAttachment{FileUploaded: false})
		assert.Nil(t, g.FileUploaded)
	})

	t.Run("maps created_by, updated_by, and file_uploaded_by when present", func(t *testing.T) {
		resp := &itx.MeetingAttachment{
			CreatedBy:      &itx.CreatedUpdatedBy{Username: "alice", Email: "alice@example.com", Name: "Alice Example"},
			UpdatedBy:      &itx.CreatedUpdatedBy{Username: "bob", Email: "bob@example.com", Name: "Bob Fixture"},
			FileUploadedBy: &itx.CreatedUpdatedBy{Username: "carol", Name: "Carol Test"},
		}

		g := ConvertITXMeetingAttachmentToGoa(resp)

		require.NotNil(t, g.CreatedBy)
		assert.Equal(t, "alice", utils.StringValue(g.CreatedBy.Username))
		assert.Equal(t, "alice@example.com", utils.StringValue(g.CreatedBy.Email))
		assert.Equal(t, "Alice Example", utils.StringValue(g.CreatedBy.Name))
		require.NotNil(t, g.UpdatedBy)
		assert.Equal(t, "bob", utils.StringValue(g.UpdatedBy.Username))
		require.NotNil(t, g.FileUploadedBy)
		assert.Equal(t, "carol", utils.StringValue(g.FileUploadedBy.Username))
	})

	t.Run("leaves audit user fields nil when absent", func(t *testing.T) {
		g := ConvertITXMeetingAttachmentToGoa(&itx.MeetingAttachment{})
		assert.Nil(t, g.CreatedBy)
		assert.Nil(t, g.UpdatedBy)
		assert.Nil(t, g.FileUploadedBy)
	})
}

// ── Past meeting attachment — Goa→ITX ────────────────────────────────────────

func TestConvertGoaToITXCreatePastMeetingAttachment(t *testing.T) {
	t.Run("maps required fields and leaves created_by to service layer", func(t *testing.T) {
		p := &meetingservice.CreateItxPastMeetingAttachmentPayload{
			Type:     "link",
			Category: "Notes",
			Name:     "ref link",
			Link:     utils.StringPtrOmitEmpty("https://example.com"),
		}

		req := ConvertGoaToITXCreatePastMeetingAttachment(p)

		assert.Equal(t, "link", req.Type)
		assert.Equal(t, "Notes", req.Category)
		assert.Equal(t, "ref link", req.Name)
		assert.Equal(t, "https://example.com", req.Link)
		assert.Nil(t, req.CreatedBy)
	})
}

func TestConvertGoaToITXUpdatePastMeetingAttachment(t *testing.T) {
	t.Run("maps required fields and leaves updated_by to service layer", func(t *testing.T) {
		p := &meetingservice.UpdateItxPastMeetingAttachmentPayload{
			Type:        "file",
			Category:    "Presentation",
			Name:        "deck.pdf",
			Description: utils.StringPtrOmitEmpty("Updated deck"),
		}

		req := ConvertGoaToITXUpdatePastMeetingAttachment(p)

		assert.Equal(t, "file", req.Type)
		assert.Equal(t, "deck.pdf", req.Name)
		assert.Equal(t, "Updated deck", req.Description)
		assert.Nil(t, req.UpdatedBy)
	})
}

// ── Past meeting attachment — ITX→Goa ────────────────────────────────────────

func TestConvertITXPastMeetingAttachmentToGoa(t *testing.T) {
	t.Run("maps meeting_and_occurrence_id and meeting_id", func(t *testing.T) {
		resp := &itx.PastMeetingAttachment{
			ID:                     "att-002",
			MeetingAndOccurrenceID: "zoom-100-occ-200",
			MeetingID:              "zoom-100",
			Type:                   "file",
			Category:               "Meeting Minutes",
			Name:                   "minutes.pdf",
		}

		g := ConvertITXPastMeetingAttachmentToGoa(resp)

		assert.Equal(t, "att-002", g.UID)
		assert.Equal(t, "zoom-100-occ-200", g.MeetingAndOccurrenceID)
		assert.Equal(t, "zoom-100", g.MeetingID)
		assert.Equal(t, "file", g.Type)
	})

	t.Run("file_uploaded false is omitted (nil)", func(t *testing.T) {
		g := ConvertITXPastMeetingAttachmentToGoa(&itx.PastMeetingAttachment{FileUploaded: false})
		assert.Nil(t, g.FileUploaded)
	})

	t.Run("audit users nil when absent", func(t *testing.T) {
		g := ConvertITXPastMeetingAttachmentToGoa(&itx.PastMeetingAttachment{})
		assert.Nil(t, g.CreatedBy)
		assert.Nil(t, g.UpdatedBy)
		assert.Nil(t, g.FileUploadedBy)
	})
}

// ── Presign and download ──────────────────────────────────────────────────────

func TestConvertITXMeetingAttachmentPresignToGoa(t *testing.T) {
	t.Run("maps required and optional fields", func(t *testing.T) {
		resp := &itx.MeetingAttachmentPresignResponse{
			ID:               "att-003",
			MeetingID:        "zoom-100",
			FileURL:          "https://s3.example.com/presigned",
			Type:             "file",
			Category:         "Notes",
			Name:             "upload.pdf",
			FileSize:         4096,
			FileUploadStatus: "ongoing",
			CreatedBy:        &itx.CreatedUpdatedBy{Username: "alice"},
		}

		g := ConvertITXMeetingAttachmentPresignToGoa(resp)

		assert.Equal(t, "att-003", g.UID)
		assert.Equal(t, "zoom-100", g.MeetingID)
		assert.Equal(t, "https://s3.example.com/presigned", g.FileURL)
		assert.Equal(t, "file", utils.StringValue(g.Type))
		require.NotNil(t, g.FileSize)
		assert.Equal(t, int64(4096), *g.FileSize)
		require.NotNil(t, g.CreatedBy)
		assert.Equal(t, "alice", utils.StringValue(g.CreatedBy.Username))
		assert.Nil(t, g.UpdatedBy)
	})
}

func TestConvertITXPastMeetingAttachmentPresignToGoa(t *testing.T) {
	t.Run("maps meeting_and_occurrence_id", func(t *testing.T) {
		resp := &itx.PastMeetingAttachmentPresignResponse{
			ID:                     "att-004",
			MeetingAndOccurrenceID: "zoom-100-occ-200",
			FileURL:                "https://s3.example.com/presigned",
		}

		g := ConvertITXPastMeetingAttachmentPresignToGoa(resp)

		assert.Equal(t, "att-004", g.UID)
		assert.Equal(t, "zoom-100-occ-200", g.MeetingAndOccurrenceID)
		assert.Equal(t, "https://s3.example.com/presigned", g.FileURL)
	})
}

func TestConvertITXAttachmentDownloadToGoa(t *testing.T) {
	t.Run("passes download_url through unchanged", func(t *testing.T) {
		resp := &itx.AttachmentDownloadResponse{
			DownloadURL: "https://s3.example.com/download?sig=abc",
		}

		g := ConvertITXAttachmentDownloadToGoa(resp)

		assert.Equal(t, "https://s3.example.com/download?sig=abc", g.DownloadURL)
	})
}
