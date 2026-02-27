// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// ITXMeetingClient defines the interface for ITX meeting operations
type ITXMeetingClient interface {
	CreateZoomMeeting(ctx context.Context, req *itx.CreateZoomMeetingRequest) (*itx.ZoomMeetingResponse, error)
	GetZoomMeeting(ctx context.Context, meetingID string) (*itx.ZoomMeetingResponse, error)
	UpdateZoomMeeting(ctx context.Context, meetingID string, req *itx.CreateZoomMeetingRequest) error
	DeleteZoomMeeting(ctx context.Context, meetingID string) error
	GetMeetingCount(ctx context.Context, projectID string) (*itx.MeetingCountResponse, error)
	GetMeetingJoinLink(ctx context.Context, req *itx.GetJoinLinkRequest) (*itx.ZoomMeetingJoinLink, error)
	ResendMeetingInvitations(ctx context.Context, meetingID string, req *itx.ResendMeetingInvitationsRequest) error
	RegisterCommitteeMembers(ctx context.Context, meetingID string) error
	UpdateOccurrence(ctx context.Context, meetingID, occurrenceID string, req *itx.UpdateOccurrenceRequest) error
	DeleteOccurrence(ctx context.Context, meetingID, occurrenceID string) error
}

// ITXRegistrantClient defines the interface for ITX registrant operations
type ITXRegistrantClient interface {
	CreateRegistrant(ctx context.Context, meetingID string, req *itx.ZoomMeetingRegistrant) (*itx.ZoomMeetingRegistrant, error)
	GetRegistrant(ctx context.Context, meetingID, registrantID string) (*itx.ZoomMeetingRegistrant, error)
	UpdateRegistrant(ctx context.Context, meetingID, registrantID string, req *itx.ZoomMeetingRegistrant) error
	DeleteRegistrant(ctx context.Context, meetingID, registrantID string) error
	GetRegistrantICS(ctx context.Context, meetingID, registrantID string) (*itx.RegistrantICS, error)
	ResendRegistrantInvitation(ctx context.Context, meetingID, registrantID string) error
}

// ITXPastMeetingClient defines the interface for ITX past meeting operations
type ITXPastMeetingClient interface {
	CreatePastMeeting(ctx context.Context, req *itx.CreatePastMeetingRequest) (*itx.PastMeetingResponse, error)
	GetPastMeeting(ctx context.Context, pastMeetingID string) (*itx.PastMeetingResponse, error)
	UpdatePastMeeting(ctx context.Context, pastMeetingID string, req *itx.CreatePastMeetingRequest) (*itx.PastMeetingResponse, error)
	DeletePastMeeting(ctx context.Context, pastMeetingID string) error
}

// ITXPastMeetingSummaryClient defines the interface for ITX past meeting summary operations
type ITXPastMeetingSummaryClient interface {
	GetPastMeetingSummary(ctx context.Context, pastMeetingID, summaryID string) (*itx.PastMeetingSummaryResponse, error)
	UpdatePastMeetingSummary(ctx context.Context, pastMeetingID, summaryID string, req *itx.UpdatePastMeetingSummaryRequest) (*itx.PastMeetingSummaryResponse, error)
}

// ITXInviteeClient defines the interface for ITX invitee operations
type ITXInviteeClient interface {
	CreateInvitee(ctx context.Context, pastMeetingID string, req *itx.CreateInviteeRequest) (*itx.InviteeResponse, error)
	UpdateInvitee(ctx context.Context, pastMeetingID, inviteeID string, req *itx.UpdateInviteeRequest) (*itx.InviteeResponse, error)
	DeleteInvitee(ctx context.Context, pastMeetingID, inviteeID string) error
}

// ITXAttendeeClient defines the interface for ITX attendee operations
type ITXAttendeeClient interface {
	CreateAttendee(ctx context.Context, pastMeetingID string, req *itx.CreateAttendeeRequest) (*itx.AttendeeResponse, error)
	UpdateAttendee(ctx context.Context, pastMeetingID, attendeeID string, req *itx.UpdateAttendeeRequest) (*itx.AttendeeResponse, error)
	DeleteAttendee(ctx context.Context, pastMeetingID, attendeeID string) error
}

// ITXPastMeetingParticipantClient combines invitee and attendee operations
type ITXPastMeetingParticipantClient interface {
	ITXInviteeClient
	ITXAttendeeClient
}

// ITXMeetingAttachmentClient defines the interface for ITX meeting attachment operations
type ITXMeetingAttachmentClient interface {
	CreateMeetingAttachment(ctx context.Context, meetingID string, req *itx.CreateMeetingAttachmentRequest) (*itx.MeetingAttachment, error)
	GetMeetingAttachment(ctx context.Context, meetingID, attachmentID string) (*itx.MeetingAttachment, error)
	UpdateMeetingAttachment(ctx context.Context, meetingID, attachmentID string, req *itx.UpdateMeetingAttachmentRequest) error
	DeleteMeetingAttachment(ctx context.Context, meetingID, attachmentID string) error
	CreateMeetingAttachmentPresignURL(ctx context.Context, meetingID string, req *itx.CreateAttachmentPresignRequest) (*itx.MeetingAttachmentPresignResponse, error)
	GetMeetingAttachmentDownloadURL(ctx context.Context, meetingID, attachmentID string) (*itx.AttachmentDownloadResponse, error)
}

// ITXPastMeetingAttachmentClient defines the interface for ITX past meeting attachment operations
type ITXPastMeetingAttachmentClient interface {
	CreatePastMeetingAttachment(ctx context.Context, meetingAndOccurrenceID string, req *itx.CreatePastMeetingAttachmentRequest) (*itx.PastMeetingAttachment, error)
	GetPastMeetingAttachment(ctx context.Context, meetingAndOccurrenceID, attachmentID string) (*itx.PastMeetingAttachment, error)
	UpdatePastMeetingAttachment(ctx context.Context, meetingAndOccurrenceID, attachmentID string, req *itx.UpdatePastMeetingAttachmentRequest) error
	DeletePastMeetingAttachment(ctx context.Context, meetingAndOccurrenceID, attachmentID string) error
	CreatePastMeetingAttachmentPresignURL(ctx context.Context, meetingAndOccurrenceID string, req *itx.CreateAttachmentPresignRequest) (*itx.PastMeetingAttachmentPresignResponse, error)
	GetPastMeetingAttachmentDownloadURL(ctx context.Context, meetingAndOccurrenceID, attachmentID string) (*itx.AttachmentDownloadResponse, error)
}

// ITXProxyClient combines meeting, registrant, past meeting, past meeting summary, participant, and attachment operations
type ITXProxyClient interface {
	ITXMeetingClient
	ITXRegistrantClient
	ITXPastMeetingClient
	ITXPastMeetingSummaryClient
	ITXPastMeetingParticipantClient
	ITXMeetingAttachmentClient
	ITXPastMeetingAttachmentClient
}
