// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package models

import (
	"time"
)

func MergeUpdateMeetingRequest(reqMeeting *MeetingBase, existingMeeting *MeetingBase) *MeetingBase {
	if reqMeeting == nil && existingMeeting == nil {
		return nil
	}

	// If there's no existing meeting but there's a payload, create new meeting
	if existingMeeting == nil {
		return reqMeeting
	}

	// If there's existing meeting but no payload, preserve existing meeting
	if reqMeeting == nil {
		return existingMeeting
	}

	now := time.Now().UTC()
	meeting := &MeetingBase{
		UID:                             existingMeeting.UID,
		ProjectUID:                      reqMeeting.ProjectUID,
		StartTime:                       reqMeeting.StartTime,
		Duration:                        reqMeeting.Duration,
		Timezone:                        reqMeeting.Timezone,
		Recurrence:                      reqMeeting.Recurrence,
		Title:                           reqMeeting.Title,
		Description:                     reqMeeting.Description,
		Committees:                      reqMeeting.Committees,
		Platform:                        reqMeeting.Platform,
		EarlyJoinTimeMinutes:            reqMeeting.EarlyJoinTimeMinutes,
		MeetingType:                     reqMeeting.MeetingType,
		Visibility:                      reqMeeting.Visibility,
		Restricted:                      reqMeeting.Restricted,
		ArtifactVisibility:              reqMeeting.ArtifactVisibility,
		PublicLink:                      existingMeeting.PublicLink, // Preserve platform-generated URL
		JoinURL:                         existingMeeting.JoinURL,    // Preserve platform-generated URL
		RecordingEnabled:                reqMeeting.RecordingEnabled,
		TranscriptEnabled:               reqMeeting.TranscriptEnabled,
		YoutubeUploadEnabled:            reqMeeting.YoutubeUploadEnabled,
		Occurrences:                     existingMeeting.Occurrences,
		EmailDeliveryErrorCount:         existingMeeting.EmailDeliveryErrorCount,
		RegistrantCount:                 existingMeeting.RegistrantCount,
		RegistrantResponseAcceptedCount: existingMeeting.RegistrantResponseAcceptedCount,
		RegistrantResponseDeclinedCount: existingMeeting.RegistrantResponseDeclinedCount,
		ZoomConfig:                      mergeUpdateMeetingRequestZoomConfig(reqMeeting.ZoomConfig, existingMeeting.ZoomConfig),
		CreatedAt:                       existingMeeting.CreatedAt,
		UpdatedAt:                       &now,
	}

	return meeting
}

// mergeUpdateMeetingRequestZoomConfig merges the existing ZoomConfig with updates from the payload,
// preserving the MeetingID from the existing config
func mergeUpdateMeetingRequestZoomConfig(reqZoomConfig *ZoomConfig, existingZoomConfig *ZoomConfig) *ZoomConfig {
	// If there's no existing config and no payload, return nil
	if existingZoomConfig == nil && reqZoomConfig == nil {
		return nil
	}

	// If there's no existing config but there's a payload, create new config
	if existingZoomConfig == nil {
		return reqZoomConfig
	}

	// If there's existing config but no payload, preserve existing config
	if reqZoomConfig == nil {
		return existingZoomConfig
	}

	// Merge existing config with payload updates, preserving MeetingID
	return &ZoomConfig{
		MeetingID:                existingZoomConfig.MeetingID, // Preserve existing MeetingID
		Passcode:                 existingZoomConfig.Passcode,  // Preserve existing Passcode
		AICompanionEnabled:       reqZoomConfig.AICompanionEnabled,
		AISummaryRequireApproval: reqZoomConfig.AISummaryRequireApproval,
	}
}

func MergeUpdateMeetingSettingsRequest(reqSettings *MeetingSettings, existingSettings *MeetingSettings) *MeetingSettings {
	if reqSettings == nil && existingSettings == nil {
		return nil
	}

	// If there's no existing settings but there's a payload, create new settings
	if existingSettings == nil {
		return reqSettings
	}

	// If there's existing settings but no payload, preserve existing settings
	if reqSettings == nil {
		return existingSettings
	}

	// Merge existing settings with payload updates, preserving UID
	now := time.Now().UTC()
	settings := &MeetingSettings{
		UID:        existingSettings.UID,
		Organizers: reqSettings.Organizers,
		CreatedAt:  existingSettings.CreatedAt,
		UpdatedAt:  &now,
	}

	return settings
}
