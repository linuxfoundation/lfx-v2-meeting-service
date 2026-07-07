// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package userservice provides an HTTP client for the v1 user-service preferences
// API, used to read and write a user's preferred meeting-invite email (Phase 1
// storage for LFXV2-2599). It calls the v1 API gateway AS the user, using the bearer
// token forwarded from self-serve.
package userservice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

const (
	// apiPath is the user-service prefix behind the LFX API gateway.
	apiPath = "/user-service/v1"

	// meetingPreferenceType is the user-service email-preference type for meeting invites.
	meetingPreferenceType = "Meeting"

	// meetingFilter selects the Type=Meeting preference record (case-insensitive match).
	meetingFilter = "type eq meeting"
)

// Config holds v1 user-service (API-gateway) client configuration.
type Config struct {
	// BaseURL is the API-gateway root, e.g. https://api-gw.dev.platform.linuxfoundation.org
	BaseURL string
	// Timeout bounds each HTTP request.
	Timeout time.Duration
}

// Client implements domain.UserServiceClient against the v1 user-service API, calling
// it as the user via their bearer token.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient creates a new user-service client.
func NewClient(config Config) (*Client, error) {
	if config.BaseURL == "" {
		return nil, fmt.Errorf("user-service base URL is required")
	}

	httpClient := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
		Timeout:   config.Timeout,
	}
	return newClient(httpClient, config.BaseURL), nil
}

// newClient builds a Client from a ready HTTP client and base URL. It is the shared
// constructor used by NewClient and tests.
func newClient(httpClient *http.Client, baseURL string) *Client {
	return &Client{
		httpClient: httpClient,
		baseURL:    strings.TrimRight(baseURL, "/"),
	}
}

// --- user-service DTOs (subset of the swagger schema) ---

// meResponse is the subset of GET /v1/me (me-user) we need: the SFID and email records.
type meResponse struct {
	ID     string      `json:"ID"`
	Emails []userEmail `json:"Emails"`
}

type userEmail struct {
	ID           string `json:"ID"`
	EmailAddress string `json:"EmailAddress"`
	Active       bool   `json:"Active"`
	IsVerified   bool   `json:"IsVerified"`
}

type emailPreference struct {
	ID      string `json:"ID"`
	EmailID string `json:"EmailID"`
	Email   string `json:"Email"`
	Type    string `json:"Type"`
}

type emailPreferenceListResponse struct {
	Data []emailPreference `json:"Data"`
}

type createEmailPreferenceRequest struct {
	EmailID   string `json:"EmailID"`
	Type      string `json:"Type"`
	IsDefault bool   `json:"IsDefault"`
}

// updateEmailPreferenceRequest is the PATCH body for an existing preference. It omits
// Type on purpose: the record's Type is already set, and sending Type on the update path
// makes the upstream user-service return an empty-body 502 (the write still lands). This
// mirrors the myprofile client, which sends Type only on create.
type updateEmailPreferenceRequest struct {
	EmailID   string `json:"EmailID"`
	IsDefault bool   `json:"IsDefault"`
}

// GetSelf resolves the calling user's SFID and email records from their bearer token.
func (c *Client) GetSelf(ctx context.Context, token string) (*domain.Self, error) {
	reqURL := fmt.Sprintf("%s%s/me", c.baseURL, apiPath)

	var me meResponse
	if err := c.doJSON(ctx, token, http.MethodGet, reqURL, nil, &me); err != nil {
		return nil, err
	}
	if me.ID == "" {
		return nil, domain.NewInternalError("user-service /v1/me returned no user ID")
	}

	self := &domain.Self{SFID: me.ID, Emails: make([]domain.SelfEmail, 0, len(me.Emails))}
	for _, e := range me.Emails {
		self.Emails = append(self.Emails, domain.SelfEmail{
			ID:       e.ID,
			Address:  e.EmailAddress,
			Active:   e.Active,
			Verified: e.IsVerified,
		})
	}
	return self, nil
}

// GetMeetingEmailPreference returns the user's Type=Meeting email preference, or nil.
func (c *Client) GetMeetingEmailPreference(ctx context.Context, token, sfid string) (*domain.PreferredEmail, error) {
	pref, err := c.getMeetingPreference(ctx, token, sfid)
	if err != nil {
		return nil, err
	}
	if pref == nil {
		return nil, nil
	}
	return &domain.PreferredEmail{PreferenceID: pref.ID, EmailID: pref.EmailID, Email: pref.Email}, nil
}

// SetMeetingEmailPreference upserts the user's Type=Meeting email preference.
func (c *Client) SetMeetingEmailPreference(ctx context.Context, token, sfid, emailID string) (*domain.PreferredEmail, error) {
	emailID = strings.TrimSpace(emailID)
	if emailID == "" {
		return nil, domain.NewValidationError("email_id is required to set a preference")
	}

	existing, err := c.getMeetingPreference(ctx, token, sfid)
	if err != nil {
		return nil, err
	}

	var result emailPreference
	var writeErr error
	if existing != nil {
		reqURL := fmt.Sprintf("%s%s/users/%s/preferences/emails/%s", c.baseURL, apiPath, url.PathEscape(sfid), url.PathEscape(existing.ID))
		body := updateEmailPreferenceRequest{EmailID: emailID, IsDefault: true}
		writeErr = c.doJSON(ctx, token, http.MethodPatch, reqURL, body, &result)
	} else {
		reqURL := fmt.Sprintf("%s%s/users/%s/preferences/emails", c.baseURL, apiPath, url.PathEscape(sfid))
		body := createEmailPreferenceRequest{EmailID: emailID, Type: meetingPreferenceType, IsDefault: true}
		writeErr = c.doJSON(ctx, token, http.MethodPost, reqURL, body, &result)
	}

	if writeErr != nil {
		// The user-service preferences write path commits the change but can return a
		// bodyless upstream 5xx (observed on PATCH). Verify by re-reading: if the stored
		// preference now matches the requested EmailID, treat the write as successful.
		if pref, verifyErr := c.getMeetingPreference(ctx, token, sfid); verifyErr == nil && pref != nil && pref.EmailID == emailID {
			slog.DebugContext(ctx, "user-service write returned an error but the change was persisted; treating as success",
				logging.ErrKey, writeErr)
			return &domain.PreferredEmail{PreferenceID: pref.ID, EmailID: pref.EmailID, Email: pref.Email}, nil
		}
		return nil, writeErr
	}

	return &domain.PreferredEmail{PreferenceID: result.ID, EmailID: result.EmailID, Email: result.Email}, nil
}

// ClearMeetingEmailPreference removes the user's Type=Meeting preference (no-op if none).
func (c *Client) ClearMeetingEmailPreference(ctx context.Context, token, sfid string) error {
	existing, err := c.getMeetingPreference(ctx, token, sfid)
	if err != nil {
		return err
	}
	if existing == nil {
		return nil
	}

	reqURL := fmt.Sprintf("%s%s/users/%s/preferences/emails/%s", c.baseURL, apiPath, url.PathEscape(sfid), url.PathEscape(existing.ID))
	if delErr := c.doJSON(ctx, token, http.MethodDelete, reqURL, nil, nil); delErr != nil {
		// As with the upsert path, verify the delete actually landed despite an upstream error.
		if pref, verifyErr := c.getMeetingPreference(ctx, token, sfid); verifyErr == nil && pref == nil {
			slog.DebugContext(ctx, "user-service delete returned an error but the record is gone; treating as success",
				logging.ErrKey, delErr)
			return nil
		}
		return delErr
	}
	return nil
}

// getMeetingPreference fetches the single Type=Meeting preference record, or nil if absent.
func (c *Client) getMeetingPreference(ctx context.Context, token, sfid string) (*emailPreference, error) {
	sfid = strings.TrimSpace(sfid)
	if sfid == "" {
		return nil, domain.NewValidationError("salesforce ID is required")
	}

	q := url.Values{}
	q.Set("$filter", meetingFilter)
	reqURL := fmt.Sprintf("%s%s/users/%s/preferences/emails?%s", c.baseURL, apiPath, url.PathEscape(sfid), q.Encode())

	var result emailPreferenceListResponse
	if err := c.doJSON(ctx, token, http.MethodGet, reqURL, nil, &result); err != nil {
		return nil, err
	}

	// The gateway filter is best-effort; guard by Type in case it returns extra rows.
	for i := range result.Data {
		if strings.EqualFold(result.Data[i].Type, meetingPreferenceType) {
			return &result.Data[i], nil
		}
	}
	return nil, nil
}

// doJSON performs an HTTP request as the user (bearer token) with an optional JSON body
// and decodes a JSON response into out (out may be nil for no-content responses).
func (c *Client) doJSON(ctx context.Context, token, method, reqURL string, body, out any) error {
	token = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(token), "Bearer "))
	if token == "" {
		return domain.NewValidationError("user token is required")
	}

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return domain.NewInternalError("failed to marshal request", err)
		}
		reader = bytes.NewReader(encoded)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, reqURL, reader)
	if err != nil {
		return domain.NewInternalError("failed to create request", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	httpReq.Header.Set("Accept", "application/json")

	// Log only method/status/length — the URL path embeds the SFID and response bodies
	// contain email addresses, both of which are identifiers we keep out of logs.
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		// *url.Error embeds the full request URL (SFID in the path); unwrap it so the
		// SFID isn't logged here or carried into the returned/replied error.
		cause := err
		var urlErr *url.Error
		if errors.As(err, &urlErr) && urlErr.Err != nil {
			cause = urlErr.Err
		}
		slog.DebugContext(ctx, "user-service request errored", "method", method, logging.ErrKey, cause)
		return domain.NewUnavailableError("user-service request failed", cause)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.NewInternalError("failed to read response", err)
	}

	slog.DebugContext(ctx, "user-service response",
		"method", method, "status", resp.StatusCode, "body_len", len(respBody))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return mapHTTPError(resp.StatusCode, respBody)
	}

	if out == nil || len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return domain.NewInternalError("failed to parse response", err)
	}
	return nil
}

// mapHTTPError maps HTTP status codes to domain errors.
func mapHTTPError(statusCode int, body []byte) error {
	var errMsg struct {
		Message string `json:"Message"`
		Error   string `json:"error"`
	}
	_ = json.Unmarshal(body, &errMsg)

	message := errMsg.Message
	if message == "" {
		message = errMsg.Error
	}
	if message == "" {
		// No structured error (e.g. a gateway 5xx with an HTML/empty body); include a
		// truncated raw body so the failure is diagnosable from the error alone.
		message = fmt.Sprintf("HTTP %d error", statusCode)
		if snippet := truncate(strings.TrimSpace(string(body)), 256); snippet != "" {
			message = fmt.Sprintf("%s: %s", message, snippet)
		}
	}

	switch statusCode {
	case http.StatusBadRequest:
		return domain.NewValidationError(message)
	case http.StatusUnauthorized, http.StatusForbidden:
		// The user's forwarded token is missing/expired/unauthorized — a caller-supplied
		// credential problem, so surface it as a validation error.
		return domain.NewValidationError(fmt.Sprintf("user token rejected by user-service: %s", message))
	case http.StatusNotFound:
		return domain.NewNotFoundError(message)
	case http.StatusConflict:
		return domain.NewConflictError(message)
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		// Transient upstream failures — keep them retryable.
		return domain.NewUnavailableError(message)
	default:
		return domain.NewInternalError(message)
	}
}

// truncate shortens s to at most n runes, appending an ellipsis when truncated.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// Ensure Client implements domain.UserServiceClient.
var _ domain.UserServiceClient = (*Client)(nil)
