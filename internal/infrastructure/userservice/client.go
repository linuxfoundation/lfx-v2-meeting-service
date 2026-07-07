// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package userservice provides an HTTP client for the v1 user-service preferences
// API, used to read and write a user's preferred meeting-invite email (Phase 1
// storage for LFXV2-2599). It mirrors the ITX proxy client's Auth0 M2M setup but
// targets the v1 API-gateway audience.
package userservice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/auth0/go-auth0/authentication"
	"github.com/auth0/go-auth0/authentication/oauth"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/oauth2"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/redaction"
)

const (
	tokenExpiryLeeway = 60 * time.Second

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
	// ClientID is the Auth0 M2M client ID.
	ClientID string
	// PrivateKey is an RSA private key in PEM format for client-assertion (RS256) auth.
	// Takes precedence over ClientSecret when both are set.
	PrivateKey string
	// ClientSecret is the Auth0 M2M client secret, used when PrivateKey is empty.
	ClientSecret string
	// Auth0Domain is the Auth0 tenant domain.
	Auth0Domain string
	// Audience is the OAuth2 audience for the API gateway.
	Audience string
	// Timeout bounds each HTTP request.
	Timeout time.Duration
}

// Client implements domain.UserServiceClient against the v1 user-service API.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// auth0TokenSource implements oauth2.TokenSource using the Auth0 SDK.
type auth0TokenSource struct {
	ctx        context.Context
	authConfig *authentication.Authentication
	audience   string
}

// Token implements the oauth2.TokenSource interface.
func (a *auth0TokenSource) Token() (*oauth2.Token, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.TODO()
	}

	tokenSet, err := a.authConfig.OAuth.LoginWithClientCredentials(ctx, oauth.LoginWithClientCredentialsRequest{
		Audience: a.audience,
	}, oauth.IDTokenValidationOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get token from Auth0: %w", err)
	}

	return &oauth2.Token{
		AccessToken:  tokenSet.AccessToken,
		TokenType:    tokenSet.TokenType,
		RefreshToken: tokenSet.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(tokenSet.ExpiresIn)*time.Second - tokenExpiryLeeway),
	}, nil
}

// NewClient creates a new user-service client with OAuth2 M2M authentication.
// It uses private-key client-assertion when PrivateKey is set, otherwise a client secret.
func NewClient(config Config) (*Client, error) {
	ctx := context.Background()

	if config.BaseURL == "" {
		return nil, fmt.Errorf("user-service base URL is required")
	}
	if config.Auth0Domain == "" {
		return nil, fmt.Errorf("user-service Auth0 domain is required")
	}
	if config.Audience == "" {
		return nil, fmt.Errorf("user-service audience is required")
	}
	if config.ClientID == "" {
		return nil, fmt.Errorf("user-service client ID is required")
	}
	if config.PrivateKey == "" && config.ClientSecret == "" {
		return nil, fmt.Errorf("user-service requires either a private key or a client secret")
	}

	// otel-instrumented HTTP client for the Auth0 token requests.
	otelClient := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
		Timeout:   config.Timeout,
	}

	authOpts := []authentication.Option{
		authentication.WithClientID(config.ClientID),
		authentication.WithClient(otelClient),
	}
	if config.PrivateKey != "" {
		authOpts = append(authOpts, authentication.WithClientAssertion(config.PrivateKey, "RS256"))
	} else {
		authOpts = append(authOpts, authentication.WithClientSecret(config.ClientSecret))
	}

	authConfig, err := authentication.New(ctx, config.Auth0Domain, authOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Auth0 client: %w", err)
	}

	tokenSource := &auth0TokenSource{ctx: ctx, authConfig: authConfig, audience: config.Audience}
	reuseTokenSource := oauth2.ReuseTokenSource(nil, tokenSource)

	// HTTP client that auto-manages tokens; wrap the transport with otelhttp so API
	// calls appear in traces.
	httpClient := oauth2.NewClient(ctx, reuseTokenSource)
	httpClient.Transport = otelhttp.NewTransport(httpClient.Transport)
	httpClient.Timeout = config.Timeout

	return newClient(httpClient, config.BaseURL), nil
}

// newClient builds a Client from a ready HTTP client and base URL. It is the shared
// constructor used by NewClient and tests (which inject an unauthenticated client).
func newClient(httpClient *http.Client, baseURL string) *Client {
	return &Client{
		httpClient: httpClient,
		baseURL:    strings.TrimRight(baseURL, "/"),
	}
}

// --- user-service DTOs (subset of the swagger schema) ---

type userListResponse struct {
	Data []struct {
		ID string `json:"ID"`
	} `json:"Data"`
}

// userResponse is the subset of GET /v1/users/{sfid} we need: the user's email records.
type userResponse struct {
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

// ResolveSFIDByUsername returns the Salesforce ID for the given LFID/username.
func (c *Client) ResolveSFIDByUsername(ctx context.Context, username string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return "", domain.NewValidationError("username is required")
	}

	q := url.Values{}
	q.Set("username", username)
	reqURL := fmt.Sprintf("%s%s/users/search?%s", c.baseURL, apiPath, q.Encode())

	var result userListResponse
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &result); err != nil {
		return "", err
	}

	for _, u := range result.Data {
		if u.ID != "" {
			return u.ID, nil
		}
	}
	return "", domain.ErrUserNotFound
}

// ResolveEmailID returns the Salesforce ID of the user's email record matching the given
// address (case-insensitive). The email must be an active, VERIFIED record on the account —
// meeting invites must only ever be routed to a verified address. A matching-but-unverified
// address is a validation error; a completely unknown address yields a retryable
// UnavailableError (verified emails sync into SFDC from auth0 asynchronously). This method
// never creates records.
func (c *Client) ResolveEmailID(ctx context.Context, sfid, email string) (string, error) {
	sfid = strings.TrimSpace(sfid)
	if sfid == "" {
		return "", domain.NewValidationError("salesforce ID is required")
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return "", domain.NewValidationError("email is required")
	}

	reqURL := fmt.Sprintf("%s%s/users/%s", c.baseURL, apiPath, url.PathEscape(sfid))
	var user userResponse
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &user); err != nil {
		return "", err
	}

	matchedUnverified := false
	for i := range user.Emails {
		e := user.Emails[i]
		if e.ID == "" || !strings.EqualFold(strings.TrimSpace(e.EmailAddress), email) {
			continue
		}
		if e.Active && e.IsVerified {
			return e.ID, nil
		}
		// Address belongs to the user but is not usable as an invite target.
		matchedUnverified = true
	}

	// Redact the address in the returned error — it propagates into logs via ErrKey.
	redactedEmail := redaction.RedactEmail(email)
	if matchedUnverified {
		return "", domain.NewValidationError(
			fmt.Sprintf("email %q is not a verified address on this account", redactedEmail))
	}
	return "", domain.NewUnavailableError(
		fmt.Sprintf("email %q not yet available in user-service; retry", redactedEmail))
}

// GetMeetingEmailPreference returns the user's Type=Meeting email preference, or nil.
func (c *Client) GetMeetingEmailPreference(ctx context.Context, sfid string) (*domain.PreferredEmail, error) {
	pref, err := c.getMeetingPreference(ctx, sfid)
	if err != nil {
		return nil, err
	}
	if pref == nil {
		return nil, nil
	}
	return &domain.PreferredEmail{PreferenceID: pref.ID, EmailID: pref.EmailID, Email: pref.Email}, nil
}

// SetMeetingEmailPreference upserts the user's Type=Meeting email preference.
func (c *Client) SetMeetingEmailPreference(ctx context.Context, sfid, emailID string) (*domain.PreferredEmail, error) {
	emailID = strings.TrimSpace(emailID)
	if emailID == "" {
		return nil, domain.NewValidationError("email_id is required to set a preference")
	}

	existing, err := c.getMeetingPreference(ctx, sfid)
	if err != nil {
		return nil, err
	}

	var result emailPreference
	var writeErr error
	if existing != nil {
		reqURL := fmt.Sprintf("%s%s/users/%s/preferences/emails/%s", c.baseURL, apiPath, url.PathEscape(sfid), url.PathEscape(existing.ID))
		body := updateEmailPreferenceRequest{EmailID: emailID, IsDefault: true}
		writeErr = c.doJSON(ctx, http.MethodPatch, reqURL, body, &result)
	} else {
		reqURL := fmt.Sprintf("%s%s/users/%s/preferences/emails", c.baseURL, apiPath, url.PathEscape(sfid))
		body := createEmailPreferenceRequest{EmailID: emailID, Type: meetingPreferenceType, IsDefault: true}
		writeErr = c.doJSON(ctx, http.MethodPost, reqURL, body, &result)
	}

	if writeErr != nil {
		// The user-service preferences write path commits the change but can return a
		// bodyless upstream 5xx (observed on PATCH). Verify by re-reading: if the stored
		// preference now matches the requested EmailID, treat the write as successful.
		if pref, verifyErr := c.getMeetingPreference(ctx, sfid); verifyErr == nil && pref != nil && pref.EmailID == emailID {
			slog.DebugContext(ctx, "user-service write returned an error but the change was persisted; treating as success",
				logging.ErrKey, writeErr)
			return &domain.PreferredEmail{PreferenceID: pref.ID, EmailID: pref.EmailID, Email: pref.Email}, nil
		}
		return nil, writeErr
	}

	return &domain.PreferredEmail{PreferenceID: result.ID, EmailID: result.EmailID, Email: result.Email}, nil
}

// ClearMeetingEmailPreference removes the user's Type=Meeting preference (no-op if none).
func (c *Client) ClearMeetingEmailPreference(ctx context.Context, sfid string) error {
	existing, err := c.getMeetingPreference(ctx, sfid)
	if err != nil {
		return err
	}
	if existing == nil {
		return nil
	}

	reqURL := fmt.Sprintf("%s%s/users/%s/preferences/emails/%s", c.baseURL, apiPath, url.PathEscape(sfid), url.PathEscape(existing.ID))
	if delErr := c.doJSON(ctx, http.MethodDelete, reqURL, nil, nil); delErr != nil {
		// As with the upsert path, verify the delete actually landed despite an upstream error.
		if pref, verifyErr := c.getMeetingPreference(ctx, sfid); verifyErr == nil && pref == nil {
			slog.DebugContext(ctx, "user-service delete returned an error but the record is gone; treating as success",
				logging.ErrKey, delErr)
			return nil
		}
		return delErr
	}
	return nil
}

// getMeetingPreference fetches the single Type=Meeting preference record, or nil if absent.
func (c *Client) getMeetingPreference(ctx context.Context, sfid string) (*emailPreference, error) {
	sfid = strings.TrimSpace(sfid)
	if sfid == "" {
		return nil, domain.NewValidationError("salesforce ID is required")
	}

	q := url.Values{}
	q.Set("$filter", meetingFilter)
	reqURL := fmt.Sprintf("%s%s/users/%s/preferences/emails?%s", c.baseURL, apiPath, url.PathEscape(sfid), q.Encode())

	var result emailPreferenceListResponse
	if err := c.doJSON(ctx, http.MethodGet, reqURL, nil, &result); err != nil {
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

// doJSON performs an HTTP request with an optional JSON body and decodes a JSON
// response into out (out may be nil for no-content responses).
func (c *Client) doJSON(ctx context.Context, method, reqURL string, body, out any) error {
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
	if body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	httpReq.Header.Set("Accept", "application/json")

	// Log only the URL path (not query) and body length — the query embeds usernames/SFIDs
	// and response bodies contain email addresses, which are PII we must keep out of logs.
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		slog.DebugContext(ctx, "user-service request errored", "method", method, "path", httpReq.URL.Path, logging.ErrKey, err)
		return domain.NewUnavailableError("user-service request failed", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.NewInternalError("failed to read response", err)
	}

	slog.DebugContext(ctx, "user-service response",
		"method", method, "path", httpReq.URL.Path, "status", resp.StatusCode, "body_len", len(respBody))

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
		// The M2M principal (not the RPC caller) failed to auth — a server-side/config
		// problem, so Internal rather than Validation (which callers read as bad input).
		return domain.NewInternalError(fmt.Sprintf("user-service authentication/authorization failed: %s", message))
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
