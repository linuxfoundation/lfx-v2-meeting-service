// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"fmt"
	"time"
)

// PastMeetingAttachment represents a file attachment for a past meeting.
// Similar to MeetingAttachment, metadata records in KV store associate a past meeting with a file in Object Store.
// The actual files are stored in the same Object Store as meeting attachments, allowing file reuse.
// Metadata is stored in NATS KV store, while actual files are in NATS Object Store.
type PastMeetingAttachment struct {
	UID             string     `json:"uid"`
	PastMeetingUID  string     `json:"past_meeting_uid"`
	FileName        string     `json:"file_name"`
	FileSize        int64      `json:"file_size"`
	ContentType     string     `json:"content_type"`
	UploadedBy      string     `json:"uploaded_by"`
	UploadedAt      *time.Time `json:"uploaded_at,omitempty"`
	Description     string     `json:"description,omitempty"`
	SourceObjectUID string     `json:"source_object_uid"` // UID of the file in Object Store
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
