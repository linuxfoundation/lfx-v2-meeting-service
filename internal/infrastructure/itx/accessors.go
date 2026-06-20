// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import "github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"

// Meetings returns the meeting operations client.
func (c *Client) Meetings() domain.ITXMeetingClient {
	return c
}

// Registrants returns the registrant operations client.
func (c *Client) Registrants() domain.ITXRegistrantClient {
	return c
}

// PastMeetings returns the past meeting operations client.
func (c *Client) PastMeetings() domain.ITXPastMeetingClient {
	return c
}

// PastMeetingSummaries returns the past meeting summary operations client.
func (c *Client) PastMeetingSummaries() domain.ITXPastMeetingSummaryClient {
	return c
}

// Participants returns invitee and attendee operations for past meetings.
func (c *Client) Participants() domain.ITXPastMeetingParticipantClient {
	return c
}

// MeetingAttachments returns active meeting attachment operations.
func (c *Client) MeetingAttachments() domain.ITXMeetingAttachmentClient {
	return c
}

// PastMeetingAttachments returns past meeting attachment operations.
func (c *Client) PastMeetingAttachments() domain.ITXPastMeetingAttachmentClient {
	return c
}
