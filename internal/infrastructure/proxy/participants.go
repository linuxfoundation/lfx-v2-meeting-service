// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package proxy

import (
	"context"
	"net/http"

	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// CreateInvitee creates an invitee for a past meeting via the ITX proxy.
func (c *Client) CreateInvitee(ctx context.Context, pastMeetingID string, req *itx.CreateInviteeRequest) (*itx.InviteeResponse, error) {
	return doJSONTyped[itx.InviteeResponse](c, ctx, apiRequest{
		method:     http.MethodPost,
		path:       "/v2/zoom/past_meetings/%s/invitees",
		pathArgs:   []any{pastMeetingID},
		body:       req,
		parseError: "failed to unmarshal response",
	})
}

// UpdateInvitee updates an invitee for a past meeting via the ITX proxy.
func (c *Client) UpdateInvitee(ctx context.Context, pastMeetingID, inviteeID string, req *itx.UpdateInviteeRequest) (*itx.InviteeResponse, error) {
	return doJSONTypedOptional[itx.InviteeResponse](c, ctx, apiRequest{
		method:   http.MethodPut,
		path:     "/v2/zoom/past_meetings/%s/invitees/%s",
		pathArgs: []any{pastMeetingID, inviteeID},
		body:     req,
	})
}

// DeleteInvitee deletes an invitee from a past meeting via the ITX proxy.
func (c *Client) DeleteInvitee(ctx context.Context, pastMeetingID, inviteeID string) error {
	return c.doNoContent(ctx, apiRequest{
		method:   http.MethodDelete,
		path:     "/v2/zoom/past_meetings/%s/invitees/%s",
		pathArgs: []any{pastMeetingID, inviteeID},
	})
}

// CreateAttendee creates an attendee for a past meeting via the ITX proxy.
func (c *Client) CreateAttendee(ctx context.Context, pastMeetingID string, req *itx.CreateAttendeeRequest) (*itx.AttendeeResponse, error) {
	return doJSONTyped[itx.AttendeeResponse](c, ctx, apiRequest{
		method:     http.MethodPost,
		path:       "/v2/zoom/past_meetings/%s/attendees",
		pathArgs:   []any{pastMeetingID},
		body:       req,
		parseError: "failed to unmarshal response",
	})
}

// UpdateAttendee updates an attendee for a past meeting via the ITX proxy.
func (c *Client) UpdateAttendee(ctx context.Context, pastMeetingID, attendeeID string, req *itx.UpdateAttendeeRequest) (*itx.AttendeeResponse, error) {
	return doJSONTypedOptional[itx.AttendeeResponse](c, ctx, apiRequest{
		method:   http.MethodPut,
		path:     "/v2/zoom/past_meetings/%s/attendees/%s",
		pathArgs: []any{pastMeetingID, attendeeID},
		body:     req,
	})
}

// DeleteAttendee deletes an attendee from a past meeting via the ITX proxy.
func (c *Client) DeleteAttendee(ctx context.Context, pastMeetingID, attendeeID string) error {
	return c.doNoContent(ctx, apiRequest{
		method:   http.MethodDelete,
		path:     "/v2/zoom/past_meetings/%s/attendees/%s",
		pathArgs: []any{pastMeetingID, attendeeID},
	})
}
