// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package proxy

import (
	"context"
	"net/http"
	"net/url"

	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// CreateZoomMeeting creates a new Zoom meeting in ITX.
func (c *Client) CreateZoomMeeting(ctx context.Context, req *itx.CreateZoomMeetingRequest) (*itx.ZoomMeetingResponse, error) {
	return doJSONTyped[itx.ZoomMeetingResponse](c, ctx, apiRequest{
		method: http.MethodPost,
		path:   "/v2/zoom/meetings",
		body:   req,
		accept: acceptJSON,
	})
}

// GetZoomMeeting retrieves a Zoom meeting from ITX.
func (c *Client) GetZoomMeeting(ctx context.Context, meetingID string) (*itx.ZoomMeetingResponse, error) {
	return doJSONTyped[itx.ZoomMeetingResponse](c, ctx, apiRequest{
		method:   http.MethodGet,
		path:     "/v2/zoom/meetings/%s",
		pathArgs: []any{meetingID},
		accept:   acceptJSON,
	})
}

// DeleteZoomMeeting deletes a Zoom meeting from ITX.
func (c *Client) DeleteZoomMeeting(ctx context.Context, meetingID string) error {
	return c.doNoContent(ctx, apiRequest{
		method:   http.MethodDelete,
		path:     "/v2/zoom/meetings/%s",
		pathArgs: []any{meetingID},
	})
}

// UpdateZoomMeeting updates a Zoom meeting in ITX.
func (c *Client) UpdateZoomMeeting(ctx context.Context, meetingID string, req *itx.CreateZoomMeetingRequest) error {
	return c.doNoContent(ctx, apiRequest{
		method:   http.MethodPut,
		path:     "/v2/zoom/meetings/%s",
		pathArgs: []any{meetingID},
		body:     req,
	})
}

// GetMeetingCount retrieves the count of meetings for a project from ITX.
func (c *Client) GetMeetingCount(ctx context.Context, projectID string) (*itx.MeetingCountResponse, error) {
	return doJSONTyped[itx.MeetingCountResponse](c, ctx, apiRequest{
		method: http.MethodGet,
		path:   "/v2/zoom/meeting_count",
		query:  url.Values{"project": {projectID}},
		accept: acceptJSON,
	})
}

// GetMeetingJoinLink retrieves a join link for a meeting from ITX.
func (c *Client) GetMeetingJoinLink(ctx context.Context, req *itx.GetJoinLinkRequest) (*itx.ZoomMeetingJoinLink, error) {
	query := url.Values{}
	if req.UseEmail {
		query.Add("use_email", "true")
	}
	if req.UserID != "" {
		query.Add("user_id", req.UserID)
	}
	if req.Name != "" {
		query.Add("name", req.Name)
	}
	if req.Email != "" {
		query.Add("email", req.Email)
	}
	if req.Register {
		query.Add("register", "true")
	}

	return doJSONTyped[itx.ZoomMeetingJoinLink](c, ctx, apiRequest{
		method:   http.MethodGet,
		path:     "/v2/zoom/meetings/%s/join_link",
		pathArgs: []any{req.MeetingID},
		query:    query,
		accept:   acceptJSON,
	})
}

// ResendMeetingInvitations resends meeting invitations to all registrants via ITX proxy.
func (c *Client) ResendMeetingInvitations(ctx context.Context, meetingID string, req *itx.ResendMeetingInvitationsRequest) error {
	if req == nil {
		req = &itx.ResendMeetingInvitationsRequest{}
	}
	return c.doNoContent(ctx, apiRequest{
		method:   http.MethodPost,
		path:     "/v2/zoom/meetings/%s/resend",
		pathArgs: []any{meetingID},
		body:     req,
	})
}

// RegisterCommitteeMembers registers committee members to a meeting asynchronously via ITX proxy.
func (c *Client) RegisterCommitteeMembers(ctx context.Context, meetingID string) error {
	return c.doNoContent(ctx, apiRequest{
		method:   http.MethodPost,
		path:     "/v2/zoom/meetings/%s/register_committee_members",
		pathArgs: []any{meetingID},
	})
}

// UpdateOccurrence updates a specific occurrence of a recurring meeting via ITX proxy.
func (c *Client) UpdateOccurrence(ctx context.Context, meetingID, occurrenceID string, req *itx.UpdateOccurrenceRequest) error {
	return c.doNoContent(ctx, apiRequest{
		method:   http.MethodPut,
		path:     "/v2/zoom/meetings/%s/occurrences/%s",
		pathArgs: []any{meetingID, occurrenceID},
		body:     req,
	})
}

// DeleteOccurrence deletes a specific occurrence of a recurring meeting via ITX proxy.
func (c *Client) DeleteOccurrence(ctx context.Context, meetingID, occurrenceID string) error {
	return c.doNoContent(ctx, apiRequest{
		method:   http.MethodDelete,
		path:     "/v2/zoom/meetings/%s/occurrences/%s",
		pathArgs: []any{meetingID, occurrenceID},
	})
}

// SubmitMeetingResponse submits a meeting response for a meeting or occurrence via ITX proxy.
func (c *Client) SubmitMeetingResponse(ctx context.Context, meetingAndOccurrenceID string, req *itx.MeetingResponseRequest) (*itx.MeetingResponseResult, error) {
	return doJSONTyped[itx.MeetingResponseResult](c, ctx, apiRequest{
		method:     http.MethodPost,
		path:       "/v2/zoom/meetings/%s/responses",
		pathArgs:   []any{url.PathEscape(meetingAndOccurrenceID)},
		body:       req,
		accept:     acceptJSON,
		parseError: "failed to unmarshal response",
	})
}
