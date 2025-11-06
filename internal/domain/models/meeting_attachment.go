// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"fmt"
	"time"
)

// Attachment type constants
const (
	AttachmentTypeFile = "file"
	AttachmentTypeLink = "link"
)

// CreateMeetingAttachmentRequest represents a request to create a file or link attachment for a meeting
type CreateMeetingAttachmentRequest struct {
	MeetingUID  string // Meeting this attachment is for
	Type        string // "file" or "link"
	Link        string // If type is "link", the URL for the link-type attachment
	Name        string // Name for the attachment
	Description string // Description for the attachment
	Username    string // Username of the user creating the attachment
	FileName    string // If type is "file", the name of the file being uploaded
	ContentType string // If type is "file", the MIME type of the file
	FileData    []byte // If type is "file", the file data
}

// MeetingAttachment represents a file or link attachment that can be referenced by meetings or past meetings.
// Type can be "file" or "link":
// - For "file" type: metadata records in KV store associate a meeting with a file in Object Store
// - For "link" type: metadata contains a URL reference without any file storage
// Multiple metadata records can reference the same file, allowing file reuse across meetings.
// Metadata is stored in NATS KV store, while actual files are in NATS Object Store.
type MeetingAttachment struct {
	UID         string     `json:"uid"`
	MeetingUID  string     `json:"meeting_uid"`
	Type        string     `json:"type"`                   // The type of attachment
	Link        string     `json:"link,omitempty"`         // URL for link-type attachments
	Name        string     `json:"name"`                   // Name for the attachment
	Description string     `json:"description,omitempty"`  // Description for the attachment
	FileName    string     `json:"file_name,omitempty"`    // File name (for file-type only)
	FileSize    int64      `json:"file_size,omitempty"`    // File size in bytes (for file-type only)
	ContentType string     `json:"content_type,omitempty"` // MIME type (for file-type only)
	UploadedBy  string     `json:"uploaded_by"`
	UploadedAt  *time.Time `json:"uploaded_at,omitempty"`
}

// Tags generates a consistent set of tags for the meeting attachment.
// TODO: Actually document all the tags in the README.md - audit all other resources.
// IMPORTANT: If you modify this method, please update the Tags documentation in the README.md
// to ensure consumers understand how to use these tags for searching.
func (a *MeetingAttachment) Tags() []string {
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

	if a.MeetingUID != "" {
		tag := fmt.Sprintf("meeting_uid:%s", a.MeetingUID)
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

	if a.Link != "" {
		tag := fmt.Sprintf("link:%s", a.Link)
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

	return tags
}
