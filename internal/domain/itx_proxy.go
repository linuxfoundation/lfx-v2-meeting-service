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
}
