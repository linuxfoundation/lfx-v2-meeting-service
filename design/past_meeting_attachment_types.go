// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package design

import (
	. "goa.design/goa/v3/dsl" //nolint:staticcheck // ST1001: the recommended way of using the goa DSL package is with the . import
)

// PastMeetingAttachment represents a file attachment for a past meeting
var PastMeetingAttachment = Type("PastMeetingAttachment", func() {
	Description("Past meeting attachment metadata")
	PastMeetingAttachmentUIDAttribute()
	PastMeetingAttachmentPastMeetingUIDAttribute()
	PastMeetingAttachmentFileNameAttribute()
	PastMeetingAttachmentFileSizeAttribute()
	PastMeetingAttachmentContentTypeAttribute()
	PastMeetingAttachmentUploadedByAttribute()
	PastMeetingAttachmentUploadedAtAttribute()
	PastMeetingAttachmentDescriptionAttribute()
	PastMeetingAttachmentSourceObjectUIDAttribute()
	Required("uid", "past_meeting_uid", "file_name", "file_size", "content_type", "uploaded_by", "source_object_uid")
})

//
// Past Meeting Attachment attribute helper functions
//

// PastMeetingAttachmentUIDAttribute is the DSL attribute for past meeting attachment UID
func PastMeetingAttachmentUIDAttribute() {
	Attribute("uid", String, "The UID of the attachment", func() {
		Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
		Format(FormatUUID)
	})
}

// PastMeetingAttachmentPastMeetingUIDAttribute is the DSL attribute for past meeting UID
func PastMeetingAttachmentPastMeetingUIDAttribute() {
	Attribute("past_meeting_uid", String, "The UID of the past meeting this attachment belongs to", func() {
		Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
		Format(FormatUUID)
	})
}

// PastMeetingAttachmentFileNameAttribute is the DSL attribute for file name
func PastMeetingAttachmentFileNameAttribute() {
	Attribute("file_name", String, "The name of the file", func() {
		Example("meeting-recording.mp4")
		MinLength(1)
		MaxLength(255)
	})
}

// PastMeetingAttachmentFileSizeAttribute is the DSL attribute for file size
func PastMeetingAttachmentFileSizeAttribute() {
	Attribute("file_size", Int64, "The size of the file in bytes", func() {
		Example(1024000)
		Minimum(0)
	})
}

// PastMeetingAttachmentContentTypeAttribute is the DSL attribute for content type
func PastMeetingAttachmentContentTypeAttribute() {
	Attribute("content_type", String, "The MIME type of the file", func() {
		Example("video/mp4")
		MinLength(1)
	})
}

// PastMeetingAttachmentUploadedByAttribute is the DSL attribute for uploader
func PastMeetingAttachmentUploadedByAttribute() {
	Attribute("uploaded_by", String, "The username of the user who uploaded the file", func() {
		Example("john.doe")
		MinLength(1)
	})
}

// PastMeetingAttachmentUploadedAtAttribute is the DSL attribute for upload timestamp
func PastMeetingAttachmentUploadedAtAttribute() {
	Attribute("uploaded_at", String, "RFC3339 timestamp when the file was uploaded", func() {
		Format(FormatDateTime)
		Example("2024-01-15T10:00:00Z")
	})
}

// PastMeetingAttachmentDescriptionAttribute is the DSL attribute for description
func PastMeetingAttachmentDescriptionAttribute() {
	Attribute("description", String, "Optional description of the attachment", func() {
		Example("Meeting recording for Q1 2024")
		MaxLength(500)
	})
}

// PastMeetingAttachmentSourceObjectUIDAttribute is the DSL attribute for source object UID
func PastMeetingAttachmentSourceObjectUIDAttribute() {
	Attribute("source_object_uid", String, "The UID of the file in the shared Object Store", func() {
		Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
		Format(FormatUUID)
	})
}
