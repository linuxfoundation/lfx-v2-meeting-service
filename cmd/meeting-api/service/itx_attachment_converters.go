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
		UID:              resp.ID,
		MeetingID:        resp.MeetingID,
		Type:             resp.Type,
		Category:         resp.Category,
		Name:             resp.Name,
		FileUploaded:     ptrIfTrue(resp.FileUploaded),
		Link:             ptrIfNotEmpty(resp.Link),
		Description:      ptrIfNotEmpty(resp.Description),
		FileName:         ptrIfNotEmpty(resp.FileName),
		FileSize:         ptrIfNotZeroInt64(resp.FileSize),
		FileURL:          ptrIfNotEmpty(resp.FileURL),
		FileUploadStatus: ptrIfNotEmpty(resp.FileUploadStatus),
		FileContentType:  ptrIfNotEmpty(resp.FileContentType),
		CreatedAt:        ptrIfNotEmpty(resp.CreatedAt),
		UpdatedAt:        ptrIfNotEmpty(resp.UpdatedAt),
		FileUploadedAt:   ptrIfNotEmpty(resp.FileUploadedAt),
	}

	if resp.CreatedBy != nil {
		result.CreatedBy = convertITXUserToGoa(resp.CreatedBy)
	}

	if resp.UpdatedBy != nil {
		result.UpdatedBy = convertITXUserToGoa(resp.UpdatedBy)
	}

	if resp.FileUploadedBy != nil {
		result.FileUploadedBy = convertITXUserToGoa(resp.FileUploadedBy)
	}

	return result
}

// ConvertITXMeetingAttachmentPresignToGoa converts ITX presign response to Goa type
func ConvertITXMeetingAttachmentPresignToGoa(resp *itx.MeetingAttachmentPresignResponse) *meetingservice.ITXMeetingAttachmentPresignResponse {
	result := &meetingservice.ITXMeetingAttachmentPresignResponse{
		UID:              resp.ID,
		MeetingID:        resp.MeetingID,
		FileURL:          resp.FileURL, // Required field
		Type:             ptrIfNotEmpty(resp.Type),
		Category:         ptrIfNotEmpty(resp.Category),
		Name:             ptrIfNotEmpty(resp.Name),
		Description:      ptrIfNotEmpty(resp.Description),
		FileName:         ptrIfNotEmpty(resp.FileName),
		FileSize:         ptrIfNotZeroInt64(resp.FileSize),
		FileUploadStatus: ptrIfNotEmpty(resp.FileUploadStatus),
		FileContentType:  ptrIfNotEmpty(resp.FileContentType),
		CreatedAt:        ptrIfNotEmpty(resp.CreatedAt),
		UpdatedAt:        ptrIfNotEmpty(resp.UpdatedAt),
	}

	if resp.CreatedBy != nil {
		result.CreatedBy = convertITXUserToGoa(resp.CreatedBy)
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
		FileUploaded:           ptrIfTrue(resp.FileUploaded),
		Link:                   ptrIfNotEmpty(resp.Link),
		Description:            ptrIfNotEmpty(resp.Description),
		FileName:               ptrIfNotEmpty(resp.FileName),
		FileSize:               ptrIfNotZeroInt64(resp.FileSize),
		FileURL:                ptrIfNotEmpty(resp.FileURL),
		FileUploadStatus:       ptrIfNotEmpty(resp.FileUploadStatus),
		FileContentType:        ptrIfNotEmpty(resp.FileContentType),
		CreatedAt:              ptrIfNotEmpty(resp.CreatedAt),
		UpdatedAt:              ptrIfNotEmpty(resp.UpdatedAt),
		FileUploadedAt:         ptrIfNotEmpty(resp.FileUploadedAt),
	}

	if resp.CreatedBy != nil {
		result.CreatedBy = convertITXUserToGoa(resp.CreatedBy)
	}

	if resp.UpdatedBy != nil {
		result.UpdatedBy = convertITXUserToGoa(resp.UpdatedBy)
	}

	if resp.FileUploadedBy != nil {
		result.FileUploadedBy = convertITXUserToGoa(resp.FileUploadedBy)
	}

	return result
}

// ConvertITXPastMeetingAttachmentPresignToGoa converts ITX presign response to Goa type
func ConvertITXPastMeetingAttachmentPresignToGoa(resp *itx.PastMeetingAttachmentPresignResponse) *meetingservice.ITXPastMeetingAttachmentPresignResponse {
	result := &meetingservice.ITXPastMeetingAttachmentPresignResponse{
		UID:                    resp.ID,
		MeetingAndOccurrenceID: resp.MeetingAndOccurrenceID,
		FileURL:                resp.FileURL, // Required field
		MeetingID:              ptrIfNotEmpty(resp.MeetingID),
		Type:                   ptrIfNotEmpty(resp.Type),
		Category:               ptrIfNotEmpty(resp.Category),
		Name:                   ptrIfNotEmpty(resp.Name),
		Description:            ptrIfNotEmpty(resp.Description),
		FileName:               ptrIfNotEmpty(resp.FileName),
		FileSize:               ptrIfNotZeroInt64(resp.FileSize),
		FileUploadStatus:       ptrIfNotEmpty(resp.FileUploadStatus),
		FileContentType:        ptrIfNotEmpty(resp.FileContentType),
		CreatedAt:              ptrIfNotEmpty(resp.CreatedAt),
		UpdatedAt:              ptrIfNotEmpty(resp.UpdatedAt),
	}

	if resp.CreatedBy != nil {
		result.CreatedBy = convertITXUserToGoa(resp.CreatedBy)
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

// ptrIfNotZeroInt64 returns a pointer to the int64 value if it's not zero, otherwise nil
func ptrIfNotZeroInt64(i int64) *int64 {
	if i == 0 {
		return nil
	}
	return &i
}
