// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"fmt"
	"time"
)

// CreatePastMeetingAttachmentRequest represents a request to create a past meeting attachment
type CreatePastMeetingAttachmentRequest struct {
	PastMeetingUID  string
	Type            string // "file" or "link"
	Link            string // Optional: URL for link-type attachments
	Name            string // Required: Custom name for the attachment
	SourceObjectUID string // Optional: UID of existing file in Object Store (for file-type)
	FileData        []byte // Optional: file data for new upload (for file-type)
	FileName        string
	ContentType     string
	Username        string
	Description     string
}

// PastMeetingAttachment represents a file or link attachment for a past meeting.
// Type can be "file" or "link":
// - For "file" type: metadata records in KV store associate a past meeting with a file in Object Store
// - For "link" type: metadata contains a URL reference without any file storage
// Files are stored in the same Object Store as meeting attachments, allowing file reuse.
// Metadata is stored in NATS KV store, while actual files are in NATS Object Store.
type PastMeetingAttachment struct {
	UID             string     `json:"uid"`
	PastMeetingUID  string     `json:"past_meeting_uid"`
	Type            string     `json:"type"`                          // "file" or "link"
	Link            string     `json:"link,omitempty"`                // URL for link-type attachments
	Name            string     `json:"name"`                          // Custom name for the attachment
	FileName        string     `json:"file_name,omitempty"`           // File name (for file-type only)
	FileSize        int64      `json:"file_size,omitempty"`           // File size in bytes (for file-type only)
	ContentType     string     `json:"content_type,omitempty"`        // MIME type (for file-type only)
	UploadedBy      string     `json:"uploaded_by"`
	UploadedAt      *time.Time `json:"uploaded_at,omitempty"`
	Description     string     `json:"description,omitempty"`
	SourceObjectUID string     `json:"source_object_uid,omitempty"` // UID of the file in Object Store (for file-type only)
}

// Tags generates a consistent set of tags for the past meeting attachment.
func (a *PastMeetingAttachment) Tags() []string {
	tags := []string{}

	if a == nil {
		return nil
	}

	if a.UID != "" {
		// without prefix
		tags = append(tags, a.UID)
		// with prefix
		tag := fmt.Sprintf("attachment_uid:%s", a.UID)
		tags = append(tags, tag)
	}

	if a.PastMeetingUID != "" {
		tag := fmt.Sprintf("past_meeting_uid:%s", a.PastMeetingUID)
		tags = append(tags, tag)
	}

	if a.Type != "" {
		tag := fmt.Sprintf("type:%s", a.Type)
		tags = append(tags, tag)
	}

	if a.Name != "" {
		tag := fmt.Sprintf("name:%s", a.Name)
		tags = append(tags, tag)
	}

	if a.FileName != "" {
		tag := fmt.Sprintf("file_name:%s", a.FileName)
		tags = append(tags, tag)
	}

	if a.UploadedBy != "" {
		tag := fmt.Sprintf("uploaded_by:%s", a.UploadedBy)
		tags = append(tags, tag)
	}

	if a.SourceObjectUID != "" {
		tag := fmt.Sprintf("source_object_uid:%s", a.SourceObjectUID)
		tags = append(tags, tag)
	}

	return tags
}
