// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

// CreatedUpdatedBy represents user information for created_by and updated_by fields
type CreatedUpdatedBy struct {
	Username string `json:"username"`        // Username of the user
	Email    string `json:"email,omitempty"` // Email of the user
	Name     string `json:"name,omitempty"`  // Name of the user
}

// CreateAttachmentPresignRequest represents the request to generate a presigned URL for attachment upload
type CreateAttachmentPresignRequest struct {
	Name        string            `json:"name"`                  // File name (required)
	Description string            `json:"description,omitempty"` // Optional description
	Category    string            `json:"category,omitempty"`    // Category: "Meeting Minutes", "Notes", "Presentation", "Other"
	FileSize    int64             `json:"file_size"`             // File size in bytes (required)
	FileType    string            `json:"file_type"`             // MIME type (required)
	CreatedBy   *CreatedUpdatedBy `json:"created_by"`            // User who created the attachment (required)
}

// MeetingAttachmentPresignResponse represents the response from presigned URL generation for meeting attachments
type MeetingAttachmentPresignResponse struct {
	ID               string            `json:"id"`                          // Attachment ID (UUID)
	MeetingID        string            `json:"meeting_id"`                  // Zoom meeting ID
	Type             string            `json:"type"`                        // Attachment type ("file" or "link")
	Category         string            `json:"category"`                    // Attachment category
	Name             string            `json:"name"`                        // File name
	Description      string            `json:"description,omitempty"`       // Description
	FileName         string            `json:"file_name,omitempty"`         // File name
	FileSize         int64             `json:"file_size,omitempty"`         // File size in bytes
	FileURL          string            `json:"file_url,omitempty"`          // Presigned S3 PUT URL (valid for 60 minutes)
	FileUploadStatus string            `json:"file_upload_status"`          // Upload status: "ongoing", "completed", "failed"
	FileContentType  string            `json:"file_content_type,omitempty"` // MIME type
	CreatedAt        string            `json:"created_at,omitempty"`        // ISO 8601 timestamp
	CreatedBy        *CreatedUpdatedBy `json:"created_by"`                  // User who created the attachment
	UpdatedAt        string            `json:"updated_at,omitempty"`        // ISO 8601 timestamp
	UpdatedBy        *CreatedUpdatedBy `json:"updated_by,omitempty"`        // User who last updated the attachment
}

// PastMeetingAttachmentPresignResponse represents the response from presigned URL generation for past meeting attachments
type PastMeetingAttachmentPresignResponse struct {
	ID                     string            `json:"id"`                          // Attachment ID (UUID)
	MeetingAndOccurrenceID string            `json:"meeting_and_occurrence_id"`   // Meeting ID and occurrence timestamp
	MeetingID              string            `json:"meeting_id,omitempty"`        // Meeting ID
	Type                   string            `json:"type"`                        // Attachment type ("file" or "link")
	Category               string            `json:"category"`                    // Attachment category
	Name                   string            `json:"name"`                        // File name
	Description            string            `json:"description,omitempty"`       // Description
	FileName               string            `json:"file_name,omitempty"`         // File name
	FileSize               int64             `json:"file_size,omitempty"`         // File size in bytes
	FileURL                string            `json:"file_url,omitempty"`          // Presigned S3 PUT URL (valid for 60 minutes)
	FileUploadStatus       string            `json:"file_upload_status"`          // Upload status: "ongoing", "completed", "failed"
	FileContentType        string            `json:"file_content_type,omitempty"` // MIME type
	CreatedAt              string            `json:"created_at,omitempty"`        // ISO 8601 timestamp
	CreatedBy              *CreatedUpdatedBy `json:"created_by"`                  // User who created the attachment
	UpdatedAt              string            `json:"updated_at,omitempty"`        // ISO 8601 timestamp
	UpdatedBy              *CreatedUpdatedBy `json:"updated_by,omitempty"`        // User who last updated the attachment
}

// AttachmentDownloadResponse represents the presigned URL response for downloading attachments
type AttachmentDownloadResponse struct {
	DownloadURL string `json:"download_url"` // Presigned S3 URL for file download (valid for 60 minutes)
}

// MeetingAttachment represents a meeting attachment
type MeetingAttachment struct {
	ID               string            `json:"id"`                           // Attachment ID (UUID)
	MeetingID        string            `json:"meeting_id"`                   // Meeting ID
	Type             string            `json:"type"`                         // "file" or "link"
	Source           string            `json:"source,omitempty"`             // "api" or "description"
	Category         string            `json:"category"`                     // "Meeting Minutes", "Notes", "Presentation", "Other"
	Link             string            `json:"link,omitempty"`               // External link URL (only for link-type attachments)
	Name             string            `json:"name"`                         // Attachment name or file name
	Description      string            `json:"description,omitempty"`        // Optional description
	FileName         string            `json:"file_name,omitempty"`          // File name (only for file-type attachments)
	FileSize         int64             `json:"file_size,omitempty"`          // File size in bytes (only for file-type attachments)
	FileURL          string            `json:"file_url,omitempty"`           // S3 key path (only for file-type attachments)
	FileUploaded     bool              `json:"file_uploaded,omitempty"`      // Whether the file has been uploaded to S3 (omitted if false)
	FileUploadStatus string            `json:"file_upload_status,omitempty"` // "ongoing", "completed", "failed"
	FileContentType  string            `json:"file_content_type,omitempty"`  // MIME type of the file
	CreatedAt        string            `json:"created_at,omitempty"`         // ISO 8601 timestamp
	CreatedBy        *CreatedUpdatedBy `json:"created_by,omitempty"`         // User who created the attachment
	UpdatedAt        string            `json:"updated_at,omitempty"`         // ISO 8601 timestamp
	UpdatedBy        *CreatedUpdatedBy `json:"updated_by,omitempty"`         // User who last updated the attachment
	FileUploadedBy   *CreatedUpdatedBy `json:"file_uploaded_by,omitempty"`   // User who uploaded the file
	FileUploadedAt   string            `json:"file_uploaded_at,omitempty"`   // ISO 8601 timestamp when file was uploaded
}

// CreateMeetingAttachmentRequest represents the request to create a meeting attachment
type CreateMeetingAttachmentRequest struct {
	Type        string            `json:"type"`                  // Required: "file" or "link"
	Category    string            `json:"category"`              // Required: "Meeting Minutes", "Notes", "Presentation", "Other"
	Link        string            `json:"link,omitempty"`        // Required if type is "link"
	Name        string            `json:"name"`                  // Required: Attachment name
	Description string            `json:"description,omitempty"` // Optional description
	CreatedBy   *CreatedUpdatedBy `json:"created_by"`            // Required: User who created the attachment
}

// UpdateMeetingAttachmentRequest represents the request to update a meeting attachment
type UpdateMeetingAttachmentRequest struct {
	Type        string            `json:"type"`                  // Required: "file" or "link"
	Category    string            `json:"category"`              // Required: "Meeting Minutes", "Notes", "Presentation", "Other"
	Link        string            `json:"link,omitempty"`        // Required if type is "link"
	Name        string            `json:"name"`                  // Required: Attachment name
	Description string            `json:"description,omitempty"` // Optional description
	UpdatedBy   *CreatedUpdatedBy `json:"updated_by"`            // Required: User who updated the attachment
}

// PastMeetingAttachment represents a past meeting attachment
type PastMeetingAttachment struct {
	ID                     string            `json:"id"`                           // Attachment ID (UUID)
	MeetingAndOccurrenceID string            `json:"meeting_and_occurrence_id"`    // Past meeting and occurrence ID
	MeetingID              string            `json:"meeting_id"`                   // Meeting ID
	Type                   string            `json:"type"`                         // "file" or "link"
	Source                 string            `json:"source,omitempty"`             // "api", "scheduled_meeting_api", or "scheduled_meeting_description"
	Category               string            `json:"category"`                     // "Meeting Minutes", "Notes", "Presentation", "Other"
	Link                   string            `json:"link,omitempty"`               // External link URL (only for link-type attachments)
	Name                   string            `json:"name"`                         // Attachment name or file name
	Description            string            `json:"description,omitempty"`        // Optional description
	FileName               string            `json:"file_name,omitempty"`          // File name (only for file-type attachments)
	FileSize               int64             `json:"file_size,omitempty"`          // File size in bytes (only for file-type attachments)
	FileURL                string            `json:"file_url,omitempty"`           // S3 key path (only for file-type attachments)
	FileUploaded           bool              `json:"file_uploaded,omitempty"`      // Whether the file has been uploaded to S3 (omitted if false)
	FileUploadStatus       string            `json:"file_upload_status,omitempty"` // "ongoing", "completed", "failed"
	FileContentType        string            `json:"file_content_type,omitempty"`  // MIME type of the file
	CreatedAt              string            `json:"created_at,omitempty"`         // ISO 8601 timestamp
	CreatedBy              *CreatedUpdatedBy `json:"created_by,omitempty"`         // User who created the attachment
	UpdatedAt              string            `json:"updated_at,omitempty"`         // ISO 8601 timestamp
	UpdatedBy              *CreatedUpdatedBy `json:"updated_by,omitempty"`         // User who last updated the attachment
	FileUploadedBy         *CreatedUpdatedBy `json:"file_uploaded_by,omitempty"`   // User who uploaded the file
	FileUploadedAt         string            `json:"file_uploaded_at,omitempty"`   // ISO 8601 timestamp when file was uploaded
}

// CreatePastMeetingAttachmentRequest represents the request to create a past meeting attachment
type CreatePastMeetingAttachmentRequest struct {
	Type        string            `json:"type"`                  // Required: "file" or "link"
	Category    string            `json:"category"`              // Required: "Meeting Minutes", "Notes", "Presentation", "Other"
	Link        string            `json:"link,omitempty"`        // Required if type is "link"
	Name        string            `json:"name"`                  // Required: Attachment name
	Description string            `json:"description,omitempty"` // Optional description
	CreatedBy   *CreatedUpdatedBy `json:"created_by"`            // Required: User who created the attachment
}

// UpdatePastMeetingAttachmentRequest represents the request to update a past meeting attachment
type UpdatePastMeetingAttachmentRequest struct {
	Type        string            `json:"type"`                  // Required: "file" or "link"
	Category    string            `json:"category"`              // Required: "Meeting Minutes", "Notes", "Presentation", "Other"
	Link        string            `json:"link,omitempty"`        // Required if type is "link"
	Name        string            `json:"name"`                  // Required: Attachment name
	Description string            `json:"description,omitempty"` // Optional description
	UpdatedBy   *CreatedUpdatedBy `json:"updated_by"`            // Required: User who updated the attachment
}
