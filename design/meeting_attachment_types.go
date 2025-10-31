// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package design

import (
	. "goa.design/goa/v3/dsl" //nolint:staticcheck // ST1001: the recommended way of using the goa DSL package is with the . import
)

// MeetingAttachment represents a file attachment associated with a meeting
var MeetingAttachment = Type("MeetingAttachment", func() {
	Description("Meeting attachment metadata")
	AttachmentUIDAttribute()
	AttachmentMeetingUIDAttribute()
	AttachmentFileNameAttribute()
	AttachmentFileSizeAttribute()
	AttachmentContentTypeAttribute()
	AttachmentUploadedByAttribute()
	AttachmentUploadedAtAttribute()
	AttachmentDescriptionAttribute()
	Required("uid", "meeting_uid", "file_name", "file_size", "content_type", "uploaded_by")
})

//
// Attachment attribute helper functions
//

// AttachmentUIDAttribute is the DSL attribute for attachment UID.
func AttachmentUIDAttribute() {
	Attribute("uid", String, "The UID of the attachment", func() {
		Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
		Format(FormatUUID)
	})
}

// AttachmentMeetingUIDAttribute is the DSL attribute for attachment meeting UID.
func AttachmentMeetingUIDAttribute() {
	Attribute("meeting_uid", String, "The UID of the meeting this attachment belongs to", func() {
		Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
		Format(FormatUUID)
	})
}

// AttachmentFileNameAttribute is the DSL attribute for attachment file name.
func AttachmentFileNameAttribute() {
	Attribute("file_name", String, "The name of the uploaded file", func() {
		Example("meeting-agenda.pdf")
		MinLength(1)
		MaxLength(255)
	})
}

// AttachmentFileSizeAttribute is the DSL attribute for attachment file size.
func AttachmentFileSizeAttribute() {
	Attribute("file_size", Int64, "The size of the file in bytes", func() {
		Example(1024000)
		Minimum(0)
	})
}

// AttachmentContentTypeAttribute is the DSL attribute for attachment content type.
func AttachmentContentTypeAttribute() {
	Attribute("content_type", String, "The MIME type of the file", func() {
		Example("application/pdf")
		MinLength(1)
	})
}

// AttachmentUploadedByAttribute is the DSL attribute for attachment uploader.
func AttachmentUploadedByAttribute() {
	Attribute("uploaded_by", String, "The username of the user who uploaded the file", func() {
		Example("john.doe")
		MinLength(1)
	})
}

// AttachmentUploadedAtAttribute is the DSL attribute for attachment upload timestamp.
func AttachmentUploadedAtAttribute() {
	Attribute("uploaded_at", String, "RFC3339 timestamp when the file was uploaded", func() {
		Format(FormatDateTime)
		Example("2024-01-15T10:00:00Z")
	})
}

// AttachmentDescriptionAttribute is the DSL attribute for attachment description.
func AttachmentDescriptionAttribute() {
	Attribute("description", String, "Optional description of the attachment", func() {
		Example("Meeting agenda for Q1 2024")
		MaxLength(500)
	})
}
