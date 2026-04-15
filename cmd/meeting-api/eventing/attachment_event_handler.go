// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	indexerConstants "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

// =============================================================================
// Meeting Attachment Event Handler
// =============================================================================

// AttachmentDBRaw represents raw meeting attachment data from v1 DynamoDB/NATS KV bucket.
type AttachmentDBRaw struct {
	ID               string                `json:"id"`
	MeetingID        string                `json:"meeting_id"`
	Type             string                `json:"type"`
	Category         string                `json:"category"`
	Link             string                `json:"link"`
	Name             string                `json:"name"`
	Description      string                `json:"description"`
	Source           string                `json:"source"`
	FileName         string                `json:"file_name"`
	FileSize         int                   `json:"file_size"`
	FileURL          string                `json:"file_url"`
	FileUploaded     *bool                 `json:"file_uploaded"`
	FileUploadStatus string                `json:"file_upload_status"`
	FileContentType  string                `json:"file_content_type"`
	FileUploadedBy   *attachmentActorDBRaw `json:"file_uploaded_by"`
	FileUploadedAt   string                `json:"file_uploaded_at"`
	CreatedAt        string                `json:"created_at"`
	UpdatedAt        string                `json:"updated_at"`
	CreatedBy        attachmentActorDBRaw  `json:"created_by"`
	UpdatedBy        attachmentActorDBRaw  `json:"updated_by"`
}

// UnmarshalJSON implements custom unmarshaling to handle both string and number inputs for numeric fields.
func (a *AttachmentDBRaw) UnmarshalJSON(data []byte) error {
	type Alias AttachmentDBRaw
	tmp := struct {
		FileSize interface{} `json:"file_size"`
		*Alias
	}{
		Alias: (*Alias)(a),
	}
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	switch v := tmp.FileSize.(type) {
	case string:
		if v != "" {
			val, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid value for file_size: %w", err)
			}
			a.FileSize = val
		}
	case float64:
		a.FileSize = int(v)
	case nil:
		// leave as zero value
	default:
		return fmt.Errorf("invalid type for file_size: %T", v)
	}
	return nil
}

// attachmentActorDBRaw represents the created_by/updated_by actor in raw attachment data.
type attachmentActorDBRaw struct {
	ID             string `json:"id"`
	Email          string `json:"email"`
	Name           string `json:"name"`
	Username       string `json:"username"`
	ProfilePicture string `json:"profile_picture"`
}

// UnmarshalJSON implements custom unmarshaling for attachmentActorDBRaw.
func (a *attachmentActorDBRaw) UnmarshalJSON(data []byte) error {
	type Alias attachmentActorDBRaw
	tmp := struct{ *Alias }{Alias: (*Alias)(a)}
	return json.Unmarshal(data, &tmp)
}

// handleMeetingAttachmentUpdate processes updates to meeting attachments
func (h *EventHandlers) handleMeetingAttachmentUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) (retry bool) {
	funcLogger := h.logger.With("key", key, "handler", "meeting_attachment")
	funcLogger.DebugContext(ctx, "processing meeting attachment update")

	attachmentData, err := convertMapToMeetingAttachmentData(v1Data)
	if err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to convert v1Data to meeting attachment")
		return false
	}

	if attachmentData.UID == "" || attachmentData.MeetingID == "" {
		funcLogger.ErrorContext(ctx, "missing required fields in meeting attachment data")
		return false
	}
	funcLogger = funcLogger.With("attachment_uid", attachmentData.UID, "meeting_id", attachmentData.MeetingID)

	// Look up project UID from parent meeting. lookupProjectFromMeeting returns ("", nil) for
	// ErrKeyNotFound (permanent miss) and a non-nil error for transient KV/decode failures.
	projSFID, err := lookupProjectFromMeeting(ctx, attachmentData.MeetingID, h.v1ObjectsKV, funcLogger)
	if err != nil {
		funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "transient error looking up parent meeting, will retry")
		return true
	}
	if projSFID == "" {
		funcLogger.WarnContext(ctx, "skipping attachment: parent meeting not found or has no project")
		return false
	}
	projectUID, mapErr := h.idMapper.MapProjectV1ToV2(ctx, projSFID)
	if mapErr != nil {
		funcLogger.With(logging.ErrKey, mapErr).WarnContext(ctx, "error mapping project v1 to v2 for attachment")
		return isTransientError(mapErr)
	}
	if projectUID == "" {
		funcLogger.WarnContext(ctx, "skipping attachment: project not yet in v2")
		return false
	}
	attachmentData.ProjectUID = projectUID

	// Look up project slug from the projects API via NATS.
	// An empty slug (no error) means the project was found but has no slug — proceed without it.
	projectSlug, slugErr := h.projectSlugLookup.GetProjectSlug(ctx, projectUID)
	if slugErr != nil {
		funcLogger.With(logging.ErrKey, slugErr).WarnContext(ctx, "transient error looking up project slug, will retry")
		return true
	}
	attachmentData.ProjectSlug = projectSlug

	// Determine action (created vs updated)
	mappingKey := fmt.Sprintf("v1_meeting_attachments.%s", attachmentData.UID)
	indexerAction := indexerConstants.ActionCreated
	if _, err := h.v1MappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
	}

	if err := h.publisher.PublishMeetingAttachmentEvent(ctx, string(indexerAction), attachmentData); err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to publish meeting attachment event")
		return isTransientError(err)
	}

	if _, err := h.v1MappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to store meeting attachment mapping")
	}

	funcLogger.InfoContext(ctx, "successfully processed meeting attachment")
	return false
}

// handleMeetingAttachmentDelete processes meeting attachment deletions
func (h *EventHandlers) handleMeetingAttachmentDelete(
	ctx context.Context,
	key string,
	_ map[string]interface{},
) (retry bool) {
	attachmentUID := extractIDFromKey(key, "itx-zoom-meetings-attachments-v2.")
	mappingKey := fmt.Sprintf("v1_meeting_attachments.%s", attachmentUID)
	if h.isTombstoned(ctx, mappingKey) {
		h.logger.DebugContext(ctx, "meeting attachment delete already processed, skipping", "attachment_uid", attachmentUID)
		return false
	}
	return h.handleMeetingTypeDelete(ctx, key, attachmentUID, []byte(attachmentUID), meetingDeleteConfig{
		indexerSubject:   "lfx.index.v1_meeting_attachment",
		tombstoneKeyFmts: []string{"v1_meeting_attachments.%s"},
	})
}

// convertMapToMeetingAttachmentData converts a raw v1 map to MeetingAttachmentEventData
func convertMapToMeetingAttachmentData(v1Data map[string]interface{}) (*models.MeetingAttachmentEventData, error) {
	raw, err := json.Marshal(v1Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal v1Data: %w", err)
	}

	var tmp AttachmentDBRaw
	if err := json.Unmarshal(raw, &tmp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal meeting attachment data: %w", err)
	}

	createdAt, _ := parseTime(tmp.CreatedAt)
	modifiedAt, _ := parseTime(tmp.UpdatedAt)
	var fileUploadedAt *time.Time
	if t, err := parseTime(tmp.FileUploadedAt); err == nil {
		fileUploadedAt = &t
	}

	var fileUploadedBy *models.CreatedBy
	if tmp.FileUploadedBy != nil {
		fileUploadedBy = &models.CreatedBy{
			UserID:         tmp.FileUploadedBy.ID,
			Email:          tmp.FileUploadedBy.Email,
			Username:       tmp.FileUploadedBy.Username,
			Name:           tmp.FileUploadedBy.Name,
			ProfilePicture: tmp.FileUploadedBy.ProfilePicture,
		}
	}

	return &models.MeetingAttachmentEventData{
		UID:              tmp.ID,
		MeetingID:        tmp.MeetingID,
		Type:             tmp.Type,
		Category:         tmp.Category,
		Link:             tmp.Link,
		Name:             tmp.Name,
		Description:      tmp.Description,
		Source:           tmp.Source,
		FileName:         tmp.FileName,
		FileSize:         tmp.FileSize,
		FileURL:          tmp.FileURL,
		FileUploaded:     tmp.FileUploaded,
		FileUploadStatus: tmp.FileUploadStatus,
		FileContentType:  tmp.FileContentType,
		FileUploadedBy:   fileUploadedBy,
		FileUploadedAt:   fileUploadedAt,
		CreatedAt:        createdAt,
		ModifiedAt:       modifiedAt,
		CreatedBy: models.CreatedBy{
			UserID:         tmp.CreatedBy.ID,
			Email:          tmp.CreatedBy.Email,
			Username:       tmp.CreatedBy.Username,
			Name:           tmp.CreatedBy.Name,
			ProfilePicture: tmp.CreatedBy.ProfilePicture,
		},
		UpdatedBy: models.UpdatedBy{
			UserID:         tmp.UpdatedBy.ID,
			Email:          tmp.UpdatedBy.Email,
			Username:       tmp.UpdatedBy.Username,
			Name:           tmp.UpdatedBy.Name,
			ProfilePicture: tmp.UpdatedBy.ProfilePicture,
		},
	}, nil
}

// =============================================================================
// Past Meeting Attachment Event Handler
// =============================================================================

// PastMeetingAttachmentDBRaw represents raw past meeting attachment data from v1 DynamoDB/NATS KV bucket.
type PastMeetingAttachmentDBRaw struct {
	ID                     string                `json:"id"`
	MeetingAndOccurrenceID string                `json:"meeting_and_occurrence_id"`
	MeetingID              string                `json:"meeting_id"`
	Type                   string                `json:"type"`
	Category               string                `json:"category"`
	Link                   string                `json:"link"`
	Name                   string                `json:"name"`
	Description            string                `json:"description"`
	Source                 string                `json:"source"`
	FileName               string                `json:"file_name"`
	FileSize               int                   `json:"file_size"`
	FileURL                string                `json:"file_url"`
	FileUploaded           *bool                 `json:"file_uploaded"`
	FileUploadStatus       string                `json:"file_upload_status"`
	FileContentType        string                `json:"file_content_type"`
	FileUploadedBy         *attachmentActorDBRaw `json:"file_uploaded_by"`
	FileUploadedAt         string                `json:"file_uploaded_at"`
	CreatedAt              string                `json:"created_at"`
	UpdatedAt              string                `json:"updated_at"`
	CreatedBy              attachmentActorDBRaw  `json:"created_by"`
	UpdatedBy              attachmentActorDBRaw  `json:"updated_by"`
}

// UnmarshalJSON implements custom unmarshaling to handle both string and number inputs for numeric fields.
func (a *PastMeetingAttachmentDBRaw) UnmarshalJSON(data []byte) error {
	type Alias PastMeetingAttachmentDBRaw
	tmp := struct {
		FileSize interface{} `json:"file_size"`
		*Alias
	}{
		Alias: (*Alias)(a),
	}
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	switch v := tmp.FileSize.(type) {
	case string:
		if v != "" {
			val, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid value for file_size: %w", err)
			}
			a.FileSize = val
		}
	case float64:
		a.FileSize = int(v)
	case nil:
		// leave as zero value
	default:
		return fmt.Errorf("invalid type for file_size: %T", v)
	}
	return nil
}

// handlePastMeetingAttachmentUpdate processes updates to past meeting attachments
func (h *EventHandlers) handlePastMeetingAttachmentUpdate(
	ctx context.Context,
	key string,
	v1Data map[string]interface{},
) (retry bool) {
	funcLogger := h.logger.With("key", key, "handler", "past_meeting_attachment")
	funcLogger.DebugContext(ctx, "processing past meeting attachment update")

	attachmentData, err := convertMapToPastMeetingAttachmentData(v1Data)
	if err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to convert v1Data to past meeting attachment")
		return false
	}

	if attachmentData.UID == "" || attachmentData.MeetingAndOccurrenceID == "" {
		funcLogger.ErrorContext(ctx, "missing required fields in past meeting attachment data")
		return false
	}
	funcLogger = funcLogger.With("attachment_uid", attachmentData.UID, "meeting_and_occurrence_id", attachmentData.MeetingAndOccurrenceID)

	// Look up project info from the parent past meeting record.
	// lookupProjectFromPastMeeting returns ("","",nil) for ErrKeyNotFound (permanent miss)
	// and a non-nil error for transient KV/decode failures.
	projSFID, projectSlug, err := lookupProjectFromPastMeeting(ctx, attachmentData.MeetingAndOccurrenceID, h.v1ObjectsKV, funcLogger)
	if err != nil {
		funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "transient error looking up parent past meeting, will retry")
		return true
	}
	if projSFID == "" {
		funcLogger.WarnContext(ctx, "skipping attachment: parent past meeting not found or has no project")
		return false
	}
	attachmentData.ProjectSlug = projectSlug
	projectUID, mapErr := h.idMapper.MapProjectV1ToV2(ctx, projSFID)
	if mapErr != nil {
		funcLogger.With(logging.ErrKey, mapErr).WarnContext(ctx, "error mapping project v1 to v2 for attachment")
		return isTransientError(mapErr)
	}
	if projectUID == "" {
		funcLogger.WarnContext(ctx, "skipping attachment: project not yet in v2")
		return false
	}
	attachmentData.ProjectUID = projectUID

	// Determine action (created vs updated)
	mappingKey := fmt.Sprintf("v1_past_meeting_attachments.%s", attachmentData.UID)
	indexerAction := indexerConstants.ActionCreated
	if _, err := h.v1MappingsKV.Get(ctx, mappingKey); err == nil {
		indexerAction = indexerConstants.ActionUpdated
	}

	if err := h.publisher.PublishPastMeetingAttachmentEvent(ctx, string(indexerAction), attachmentData); err != nil {
		funcLogger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to publish past meeting attachment event")
		return isTransientError(err)
	}

	if _, err := h.v1MappingsKV.Put(ctx, mappingKey, []byte("1")); err != nil {
		funcLogger.With(logging.ErrKey, err).WarnContext(ctx, "failed to store past meeting attachment mapping")
	}

	funcLogger.InfoContext(ctx, "successfully processed past meeting attachment")
	return false
}

// handlePastMeetingAttachmentDelete processes past meeting attachment deletions
func (h *EventHandlers) handlePastMeetingAttachmentDelete(
	ctx context.Context,
	key string,
	_ map[string]interface{},
) (retry bool) {
	attachmentUID := extractIDFromKey(key, "itx-zoom-past-meetings-attachments.")
	mappingKey := fmt.Sprintf("v1_past_meeting_attachments.%s", attachmentUID)
	if h.isTombstoned(ctx, mappingKey) {
		h.logger.DebugContext(ctx, "past meeting attachment delete already processed, skipping", "attachment_uid", attachmentUID)
		return false
	}
	return h.handleMeetingTypeDelete(ctx, key, attachmentUID, []byte(attachmentUID), meetingDeleteConfig{
		indexerSubject:   "lfx.index.v1_past_meeting_attachment",
		tombstoneKeyFmts: []string{"v1_past_meeting_attachments.%s"},
	})
}

// convertMapToPastMeetingAttachmentData converts a raw v1 map to PastMeetingAttachmentEventData
func convertMapToPastMeetingAttachmentData(v1Data map[string]interface{}) (*models.PastMeetingAttachmentEventData, error) {
	raw, err := json.Marshal(v1Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal v1Data: %w", err)
	}

	var tmp PastMeetingAttachmentDBRaw
	if err := json.Unmarshal(raw, &tmp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal past meeting attachment data: %w", err)
	}

	createdAt, _ := parseTime(tmp.CreatedAt)
	modifiedAt, _ := parseTime(tmp.UpdatedAt)
	var fileUploadedAt *time.Time
	if t, err := parseTime(tmp.FileUploadedAt); err == nil {
		fileUploadedAt = &t
	}

	var fileUploadedBy *models.CreatedBy
	if tmp.FileUploadedBy != nil {
		fileUploadedBy = &models.CreatedBy{
			UserID:         tmp.FileUploadedBy.ID,
			Email:          tmp.FileUploadedBy.Email,
			Username:       tmp.FileUploadedBy.Username,
			Name:           tmp.FileUploadedBy.Name,
			ProfilePicture: tmp.FileUploadedBy.ProfilePicture,
		}
	}

	return &models.PastMeetingAttachmentEventData{
		UID:                    tmp.ID,
		MeetingAndOccurrenceID: tmp.MeetingAndOccurrenceID,
		MeetingID:              tmp.MeetingID,
		Type:                   tmp.Type,
		Category:               tmp.Category,
		Link:                   tmp.Link,
		Name:                   tmp.Name,
		Description:            tmp.Description,
		Source:                 tmp.Source,
		FileName:               tmp.FileName,
		FileSize:               tmp.FileSize,
		FileURL:                tmp.FileURL,
		FileUploaded:           tmp.FileUploaded,
		FileUploadStatus:       tmp.FileUploadStatus,
		FileContentType:        tmp.FileContentType,
		FileUploadedBy:         fileUploadedBy,
		FileUploadedAt:         fileUploadedAt,
		CreatedAt:              createdAt,
		ModifiedAt:             modifiedAt,
		CreatedBy: models.CreatedBy{
			UserID:         tmp.CreatedBy.ID,
			Email:          tmp.CreatedBy.Email,
			Username:       tmp.CreatedBy.Username,
			Name:           tmp.CreatedBy.Name,
			ProfilePicture: tmp.CreatedBy.ProfilePicture,
		},
		UpdatedBy: models.UpdatedBy{
			UserID:         tmp.UpdatedBy.ID,
			Email:          tmp.UpdatedBy.Email,
			Username:       tmp.UpdatedBy.Username,
			Name:           tmp.UpdatedBy.Name,
			ProfilePicture: tmp.UpdatedBy.ProfilePicture,
		},
	}, nil
}
