// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	meetingservice "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting_service"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// ============================================================================
// Meeting Attachment Converters
// ============================================================================

// ConvertGoaToITXCreateMeetingAttachment converts Goa payload to ITX request
func ConvertGoaToITXCreateMeetingAttachment(payload *meetingservice.CreateItxMeetingAttachmentPayload, username string) *itx.CreateMeetingAttachmentRequest {
	req := &itx.CreateMeetingAttachmentRequest{
		Type:     payload.Type,
		Category: payload.Category,
		Name:     payload.Name,
		CreatedBy: &itx.CreatedUpdatedBy{
			Username: username,
		},
	}

	if payload.Link != nil {
		req.Link = *payload.Link
	}

	if payload.Description != nil {
		req.Description = *payload.Description
	}

	return req
}

// ConvertGoaToITXUpdateMeetingAttachment converts Goa payload to ITX request
func ConvertGoaToITXUpdateMeetingAttachment(payload *meetingservice.UpdateItxMeetingAttachmentPayload, username string) *itx.UpdateMeetingAttachmentRequest {
	req := &itx.UpdateMeetingAttachmentRequest{
		Type:     payload.Type,
		Category: payload.Category,
		Name:     payload.Name,
		UpdatedBy: &itx.CreatedUpdatedBy{
			Username: username,
		},
	}

	if payload.Link != nil {
		req.Link = *payload.Link
	}

	if payload.Description != nil {
		req.Description = *payload.Description
	}

	return req
}

// ConvertGoaToITXCreateMeetingAttachmentPresign converts Goa payload to ITX request
func ConvertGoaToITXCreateMeetingAttachmentPresign(payload *meetingservice.CreateItxMeetingAttachmentPresignPayload, username string) *itx.CreateAttachmentPresignRequest {
	req := &itx.CreateAttachmentPresignRequest{
		Name:     payload.Name,
		FileSize: payload.FileSize,
		FileType: payload.FileType,
		CreatedBy: &itx.CreatedUpdatedBy{
			Username: username,
		},
	}

	if payload.Description != nil {
		req.Description = *payload.Description
	}

	if payload.Category != nil {
		req.Category = *payload.Category
	}

	return req
}

// ConvertITXMeetingAttachmentToGoa converts ITX response to Goa type
func ConvertITXMeetingAttachmentToGoa(resp *itx.MeetingAttachment) *meetingservice.ITXMeetingAttachment {
	result := &meetingservice.ITXMeetingAttachment{
		UID:          resp.ID,
		MeetingID:    resp.MeetingID,
		Type:         resp.Type,
		Category:     resp.Category,
		Name:         resp.Name,
		FileUploaded: &resp.FileUploaded,
	}

	if resp.Link != "" {
		result.Link = &resp.Link
	}

	if resp.Description != "" {
		result.Description = &resp.Description
	}

	if resp.FileName != "" {
		result.FileName = &resp.FileName
	}

	if resp.FileSize > 0 {
		result.FileSize = &resp.FileSize
	}

	if resp.FileURL != "" {
		result.FileURL = &resp.FileURL
	}

	if resp.FileUploadStatus != "" {
		result.FileUploadStatus = &resp.FileUploadStatus
	}

	if resp.FileContentType != "" {
		result.FileContentType = &resp.FileContentType
	}

	if resp.CreatedAt != "" {
		result.CreatedAt = &resp.CreatedAt
	}

	if resp.CreatedBy != nil {
		result.CreatedBy = convertITXUserToGoa(resp.CreatedBy)
	}

	if resp.UpdatedAt != "" {
		result.UpdatedAt = &resp.UpdatedAt
	}

	if resp.UpdatedBy != nil {
		result.UpdatedBy = convertITXUserToGoa(resp.UpdatedBy)
	}

	if resp.FileUploadedBy != nil {
		result.FileUploadedBy = convertITXUserToGoa(resp.FileUploadedBy)
	}

	if resp.FileUploadedAt != "" {
		result.FileUploadedAt = &resp.FileUploadedAt
	}

	return result
}

// ConvertITXMeetingAttachmentPresignToGoa converts ITX presign response to Goa type
func ConvertITXMeetingAttachmentPresignToGoa(resp *itx.MeetingAttachmentPresignResponse) *meetingservice.ITXMeetingAttachmentPresignResponse {
	result := &meetingservice.ITXMeetingAttachmentPresignResponse{
		UID:       resp.ID,
		MeetingID: resp.MeetingID,
		FileURL:   resp.FileURL, // Required field
	}

	if resp.Type != "" {
		result.Type = &resp.Type
	}

	if resp.Category != "" {
		result.Category = &resp.Category
	}

	if resp.Name != "" {
		result.Name = &resp.Name
	}

	if resp.Description != "" {
		result.Description = &resp.Description
	}

	if resp.FileName != "" {
		result.FileName = &resp.FileName
	}

	if resp.FileSize > 0 {
		result.FileSize = &resp.FileSize
	}

	if resp.FileUploadStatus != "" {
		result.FileUploadStatus = &resp.FileUploadStatus
	}

	if resp.FileContentType != "" {
		result.FileContentType = &resp.FileContentType
	}

	if resp.CreatedAt != "" {
		result.CreatedAt = &resp.CreatedAt
	}

	if resp.CreatedBy != nil {
		result.CreatedBy = convertITXUserToGoa(resp.CreatedBy)
	}

	if resp.UpdatedAt != "" {
		result.UpdatedAt = &resp.UpdatedAt
	}

	if resp.UpdatedBy != nil {
		result.UpdatedBy = convertITXUserToGoa(resp.UpdatedBy)
	}

	return result
}

// ============================================================================
// Past Meeting Attachment Converters
// ============================================================================

// ConvertGoaToITXCreatePastMeetingAttachment converts Goa payload to ITX request
func ConvertGoaToITXCreatePastMeetingAttachment(payload *meetingservice.CreateItxPastMeetingAttachmentPayload, username string) *itx.CreatePastMeetingAttachmentRequest {
	req := &itx.CreatePastMeetingAttachmentRequest{
		Type:     payload.Type,
		Category: payload.Category,
		Name:     payload.Name,
		CreatedBy: &itx.CreatedUpdatedBy{
			Username: username,
		},
	}

	if payload.Link != nil {
		req.Link = *payload.Link
	}

	if payload.Description != nil {
		req.Description = *payload.Description
	}

	return req
}

// ConvertGoaToITXUpdatePastMeetingAttachment converts Goa payload to ITX request
func ConvertGoaToITXUpdatePastMeetingAttachment(payload *meetingservice.UpdateItxPastMeetingAttachmentPayload, username string) *itx.UpdatePastMeetingAttachmentRequest {
	req := &itx.UpdatePastMeetingAttachmentRequest{
		Type:     payload.Type,
		Category: payload.Category,
		Name:     payload.Name,
		UpdatedBy: &itx.CreatedUpdatedBy{
			Username: username,
		},
	}

	if payload.Link != nil {
		req.Link = *payload.Link
	}

	if payload.Description != nil {
		req.Description = *payload.Description
	}

	return req
}

// ConvertGoaToITXCreatePastMeetingAttachmentPresign converts Goa payload to ITX request
func ConvertGoaToITXCreatePastMeetingAttachmentPresign(payload *meetingservice.CreateItxPastMeetingAttachmentPresignPayload, username string) *itx.CreateAttachmentPresignRequest {
	req := &itx.CreateAttachmentPresignRequest{
		Name:     payload.Name,
		FileSize: payload.FileSize,
		FileType: payload.FileType,
		CreatedBy: &itx.CreatedUpdatedBy{
			Username: username,
		},
	}

	if payload.Description != nil {
		req.Description = *payload.Description
	}

	if payload.Category != nil {
		req.Category = *payload.Category
	}

	return req
}

// ConvertITXPastMeetingAttachmentToGoa converts ITX response to Goa type
func ConvertITXPastMeetingAttachmentToGoa(resp *itx.PastMeetingAttachment) *meetingservice.ITXPastMeetingAttachment {
	result := &meetingservice.ITXPastMeetingAttachment{
		UID:                    resp.ID,
		MeetingAndOccurrenceID: resp.MeetingAndOccurrenceID,
		MeetingID:              resp.MeetingID,
		Type:                   resp.Type,
		Category:               resp.Category,
		Name:                   resp.Name,
		FileUploaded:           &resp.FileUploaded,
	}

	if resp.Link != "" {
		result.Link = &resp.Link
	}

	if resp.Description != "" {
		result.Description = &resp.Description
	}

	if resp.FileName != "" {
		result.FileName = &resp.FileName
	}

	if resp.FileSize > 0 {
		result.FileSize = &resp.FileSize
	}

	if resp.FileURL != "" {
		result.FileURL = &resp.FileURL
	}

	if resp.FileUploadStatus != "" {
		result.FileUploadStatus = &resp.FileUploadStatus
	}

	if resp.FileContentType != "" {
		result.FileContentType = &resp.FileContentType
	}

	if resp.CreatedAt != "" {
		result.CreatedAt = &resp.CreatedAt
	}

	if resp.CreatedBy != nil {
		result.CreatedBy = convertITXUserToGoa(resp.CreatedBy)
	}

	if resp.UpdatedAt != "" {
		result.UpdatedAt = &resp.UpdatedAt
	}

	if resp.UpdatedBy != nil {
		result.UpdatedBy = convertITXUserToGoa(resp.UpdatedBy)
	}

	if resp.FileUploadedBy != nil {
		result.FileUploadedBy = convertITXUserToGoa(resp.FileUploadedBy)
	}

	if resp.FileUploadedAt != "" {
		result.FileUploadedAt = &resp.FileUploadedAt
	}

	return result
}

// ConvertITXPastMeetingAttachmentPresignToGoa converts ITX presign response to Goa type
func ConvertITXPastMeetingAttachmentPresignToGoa(resp *itx.PastMeetingAttachmentPresignResponse) *meetingservice.ITXPastMeetingAttachmentPresignResponse {
	result := &meetingservice.ITXPastMeetingAttachmentPresignResponse{
		UID:                    resp.ID,
		MeetingAndOccurrenceID: resp.MeetingAndOccurrenceID,
		FileURL:                resp.FileURL, // Required field
	}

	if resp.MeetingID != "" {
		result.MeetingID = &resp.MeetingID
	}

	if resp.Type != "" {
		result.Type = &resp.Type
	}

	if resp.Category != "" {
		result.Category = &resp.Category
	}

	if resp.Name != "" {
		result.Name = &resp.Name
	}

	if resp.Description != "" {
		result.Description = &resp.Description
	}

	if resp.FileName != "" {
		result.FileName = &resp.FileName
	}

	if resp.FileSize > 0 {
		result.FileSize = &resp.FileSize
	}

	if resp.FileUploadStatus != "" {
		result.FileUploadStatus = &resp.FileUploadStatus
	}

	if resp.FileContentType != "" {
		result.FileContentType = &resp.FileContentType
	}

	if resp.CreatedAt != "" {
		result.CreatedAt = &resp.CreatedAt
	}

	if resp.CreatedBy != nil {
		result.CreatedBy = convertITXUserToGoa(resp.CreatedBy)
	}

	if resp.UpdatedAt != "" {
		result.UpdatedAt = &resp.UpdatedAt
	}

	if resp.UpdatedBy != nil {
		result.UpdatedBy = convertITXUserToGoa(resp.UpdatedBy)
	}

	return result
}

// ConvertITXAttachmentDownloadToGoa converts ITX download response to Goa type
func ConvertITXAttachmentDownloadToGoa(resp *itx.AttachmentDownloadResponse) *meetingservice.ITXAttachmentDownloadResponse {
	return &meetingservice.ITXAttachmentDownloadResponse{
		DownloadURL: resp.DownloadURL,
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

// convertITXUserToGoa converts ITX user info to Goa type
func convertITXUserToGoa(user *itx.CreatedUpdatedBy) *meetingservice.ITXUser {
	if user == nil {
		return nil
	}
	result := &meetingservice.ITXUser{}

	if user.Username != "" {
		username := user.Username
		result.Username = &username
	}

	if user.Email != "" {
		email := user.Email
		result.Email = &email
	}

	if user.Name != "" {
		name := user.Name
		result.Name = &name
	}

	return result
}
