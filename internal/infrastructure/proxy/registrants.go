// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package proxy

import (
	"context"
	"net/http"

	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// CreateRegistrant creates a meeting registrant via ITX proxy.
func (c *Client) CreateRegistrant(ctx context.Context, meetingID string, req *itx.ZoomMeetingRegistrant) (*itx.ZoomMeetingRegistrant, error) {
	return doJSONTyped[itx.ZoomMeetingRegistrant](c, ctx, apiRequest{
		method:   http.MethodPost,
		path:     "/v2/zoom/meetings/%s/registrants",
		pathArgs: []any{meetingID},
		body:     req,
		accept:   acceptJSON,
	})
}

// GetRegistrant retrieves a meeting registrant via ITX proxy.
func (c *Client) GetRegistrant(ctx context.Context, meetingID, registrantID string) (*itx.ZoomMeetingRegistrant, error) {
	return doJSONTyped[itx.ZoomMeetingRegistrant](c, ctx, apiRequest{
		method:   http.MethodGet,
		path:     "/v2/zoom/meetings/%s/registrants/%s",
		pathArgs: []any{meetingID, registrantID},
		accept:   acceptJSON,
	})
}

// UpdateRegistrant updates a meeting registrant via ITX proxy.
func (c *Client) UpdateRegistrant(ctx context.Context, meetingID, registrantID string, req *itx.ZoomMeetingRegistrant) error {
	return c.doNoContent(ctx, apiRequest{
		method:   http.MethodPut,
		path:     "/v2/zoom/meetings/%s/registrants/%s",
		pathArgs: []any{meetingID, registrantID},
		body:     req,
	})
}

// DeleteRegistrant deletes a meeting registrant via ITX proxy.
func (c *Client) DeleteRegistrant(ctx context.Context, meetingID, registrantID string) error {
	return c.doNoContent(ctx, apiRequest{
		method:   http.MethodDelete,
		path:     "/v2/zoom/meetings/%s/registrants/%s",
		pathArgs: []any{meetingID, registrantID},
	})
}

// GetRegistrantICS retrieves an ICS calendar file for a meeting registrant via ITX proxy.
func (c *Client) GetRegistrantICS(ctx context.Context, meetingID, registrantID string) (*itx.RegistrantICS, error) {
	content, err := c.doRaw(ctx, apiRequest{
		method:   http.MethodGet,
		path:     "/v2/zoom/meetings/%s/registrants/%s/ics",
		pathArgs: []any{meetingID, registrantID},
		accept:   acceptCalendar,
	})
	if err != nil {
		return nil, err
	}
	return &itx.RegistrantICS{Content: content}, nil
}

// ResendRegistrantInvitation resends a meeting invitation to a registrant via ITX proxy.
func (c *Client) ResendRegistrantInvitation(ctx context.Context, meetingID, registrantID string) error {
	return c.doNoContent(ctx, apiRequest{
		method:   http.MethodPost,
		path:     "/v2/zoom/meetings/%s/registrants/%s/resend",
		pathArgs: []any{meetingID, registrantID},
	})
}
