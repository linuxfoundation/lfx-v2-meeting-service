// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

// CreatePastMeetingAttachmentRequest represents a request to create a past meeting attachment
type CreatePastMeetingAttachmentRequest struct {
	PastMeetingUID  string
	SourceObjectUID string // Optional: UID of existing file in Object Store
	FileData        []byte // Optional: file data for new upload
	FileName        string
	ContentType     string
	Username        string
	Description     string
}
