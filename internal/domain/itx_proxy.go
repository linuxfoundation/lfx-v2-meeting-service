// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// ITXProxyClient defines the interface for ITX Zoom API proxy operations
type ITXProxyClient interface {
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

	// Registrant operations
	CreateRegistrant(ctx context.Context, meetingID string, req *itx.ZoomMeetingRegistrant) (*itx.ZoomMeetingRegistrant, error)
	GetRegistrant(ctx context.Context, meetingID, registrantID string) (*itx.ZoomMeetingRegistrant, error)
	UpdateRegistrant(ctx context.Context, meetingID, registrantID string, req *itx.ZoomMeetingRegistrant) error
	DeleteRegistrant(ctx context.Context, meetingID, registrantID string) error
	GetRegistrantICS(ctx context.Context, meetingID, registrantID string) (*itx.RegistrantICS, error)
	ResendRegistrantInvitation(ctx context.Context, meetingID, registrantID string) error
}
