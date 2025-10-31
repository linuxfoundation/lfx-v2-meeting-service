// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"fmt"
	"time"
)

// UploadAttachmentRequest represents a request to upload a file attachment for a meeting
type UploadAttachmentRequest struct {
	MeetingUID  string // Meeting this attachment is for
	Username    string // Username of the user uploading the file
	FileName    string // Name of the file being uploaded
	ContentType string // MIME type of the file
	FileData    []byte // The file data
	Description string // Optional description of the attachment
}

// MeetingAttachment represents a file attachment associated with a meeting.
// Files are stored in NATS JetStream Object Store with metadata.
type MeetingAttachment struct {
	UID         string     `json:"uid"`
	MeetingUID  string     `json:"meeting_uid"`
	FileName    string     `json:"file_name"`
	FileSize    int64      `json:"file_size"`
	ContentType string     `json:"content_type"`
	UploadedBy  string     `json:"uploaded_by"`
	UploadedAt  *time.Time `json:"uploaded_at,omitempty"`
	Description string     `json:"description,omitempty"`
}

// Tags generates a consistent set of tags for the meeting attachment.
// IMPORTANT: If you modify this method, please update the Meeting Tags documentation in the README.md
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
