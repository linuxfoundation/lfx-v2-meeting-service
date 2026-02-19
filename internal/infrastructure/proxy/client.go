// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package proxy

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
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
	"golang.org/x/oauth2"
)

const tokenExpiryLeeway = 60 * time.Second

// Config holds ITX proxy configuration
type Config struct {
	BaseURL     string
	ClientID    string
	PrivateKey  string // RSA private key in PEM format
	Auth0Domain string
	Audience    string
	Timeout     time.Duration
}

// Client implements domain.ITXProxyClient
type Client struct {
	httpClient *http.Client
	config     Config
}

// auth0TokenSource implements oauth2.TokenSource using Auth0 SDK with private key
type auth0TokenSource struct {
	ctx        context.Context
	authConfig *authentication.Authentication
	audience   string
}

// Token implements the oauth2.TokenSource interface
func (a *auth0TokenSource) Token() (*oauth2.Token, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.TODO()
	}

	// Build and issue a request using Auth0 SDK
	body := oauth.LoginWithClientCredentialsRequest{
		Audience: a.audience,
	}

	tokenSet, err := a.authConfig.OAuth.LoginWithClientCredentials(ctx, body, oauth.IDTokenValidationOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get token from Auth0: %w", err)
	}

	// Convert Auth0 response to oauth2.Token with leeway for expiration
	token := &oauth2.Token{
		AccessToken:  tokenSet.AccessToken,
		TokenType:    tokenSet.TokenType,
		RefreshToken: tokenSet.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(tokenSet.ExpiresIn)*time.Second - tokenExpiryLeeway),
	}

	// Add extra fields
	token = token.WithExtra(map[string]any{
		"scope": tokenSet.Scope,
	})

	return token, nil
}

// NewClient creates a new ITX proxy client with OAuth2 M2M authentication using private key
func NewClient(config Config) *Client {
	ctx := context.Background()

	if config.PrivateKey == "" {
		panic("ITX_CLIENT_PRIVATE_KEY is required but not set")
	}

	// Strip trailing slash from base URL to prevent double slashes in URL construction
	config.BaseURL = strings.TrimRight(config.BaseURL, "/")

	// Create Auth0 authentication client with private key assertion (JWT)
	// The private key should be in PEM format (raw, not base64-encoded)
	authConfig, err := authentication.New(
		ctx,
		config.Auth0Domain,
		authentication.WithClientID(config.ClientID),
		authentication.WithClientAssertion(config.PrivateKey, "RS256"),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create Auth0 client: %v (ensure ITX_CLIENT_PRIVATE_KEY contains a valid RSA private key in PEM format)", err))
	}

	// Create token source
	tokenSource := &auth0TokenSource{
		ctx:        ctx,
		authConfig: authConfig,
		audience:   config.Audience,
	}

	// Wrap with oauth2.ReuseTokenSource for automatic caching and renewal
	reuseTokenSource := oauth2.ReuseTokenSource(nil, tokenSource)

	// Create HTTP client that automatically handles token management
	httpClient := oauth2.NewClient(ctx, reuseTokenSource)
	httpClient.Timeout = config.Timeout

	return &Client{
		httpClient: httpClient,
		config:     config,
	}
}

// logRequest logs the outgoing HTTP request for debugging
func (c *Client) logRequest(ctx context.Context, method, url string, body []byte) {
	slog.DebugContext(ctx, "ITX API Request",
		"method", method,
		"url", url,
		"body", string(body),
	)
}

// logResponse logs the incoming HTTP response for debugging
func (c *Client) logResponse(ctx context.Context, statusCode int, body []byte) {
	if statusCode < 200 || statusCode >= 300 {
		slog.ErrorContext(ctx, "ITX API Response Error",
			"status_code", statusCode,
			"body", string(body),
		)
	} else {
		slog.DebugContext(ctx, "ITX API Response",
			"status_code", statusCode,
			"body", string(body),
		)
	}
}

// CreateZoomMeeting creates a new Zoom meeting in ITX
func (c *Client) CreateZoomMeeting(ctx context.Context, req *itx.CreateZoomMeetingRequest) (*itx.ZoomMeetingResponse, error) {
	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return nil, domain.NewInternalError("failed to marshal request", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/v2/zoom/meetings", c.config.BaseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization automatically added by OAuth2 transport)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("x-scope", "manage:zoom")

	// Log request
	c.logRequest(ctx, http.MethodPost, url, body)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, domain.NewInternalError("failed to read response", err)
	}

	// Log response
	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.mapHTTPError(resp.StatusCode, respBody)
	}

	// Parse response
	var result itx.ZoomMeetingResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}

	return &result, nil
}

// GetZoomMeeting retrieves a Zoom meeting from ITX
func (c *Client) GetZoomMeeting(ctx context.Context, meetingID string) (*itx.ZoomMeetingResponse, error) {
	// Create HTTP request
	url := fmt.Sprintf("%s/v2/zoom/meetings/%s", c.config.BaseURL, meetingID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization automatically added by OAuth2 transport)
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("x-scope", "manage:zoom")

	// Log request
	c.logRequest(ctx, http.MethodGet, url, nil)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, domain.NewInternalError("failed to read response", err)
	}

	// Log response
	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.mapHTTPError(resp.StatusCode, respBody)
	}

	// Parse response
	var result itx.ZoomMeetingResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}

	return &result, nil
}

// DeleteZoomMeeting deletes a Zoom meeting from ITX
func (c *Client) DeleteZoomMeeting(ctx context.Context, meetingID string) error {
	// Create HTTP request
	url := fmt.Sprintf("%s/v2/zoom/meetings/%s", c.config.BaseURL, meetingID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization automatically added by OAuth2 transport)
	httpReq.Header.Set("x-scope", "manage:zoom")

	// Log request
	c.logRequest(ctx, http.MethodDelete, url, nil)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body (for error messages)
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.NewInternalError("failed to read response", err)
	}

	// Log response
	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.mapHTTPError(resp.StatusCode, respBody)
	}

	return nil
}

// UpdateZoomMeeting updates a Zoom meeting in ITX
func (c *Client) UpdateZoomMeeting(ctx context.Context, meetingID string, req *itx.CreateZoomMeetingRequest) error {
	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return domain.NewInternalError("failed to marshal request", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/v2/zoom/meetings/%s", c.config.BaseURL, meetingID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization automatically added by OAuth2 transport)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-scope", "manage:zoom")

	// Log request
	c.logRequest(ctx, http.MethodPut, url, body)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body (for error messages)
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.NewInternalError("failed to read response", err)
	}

	// Log response
	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.mapHTTPError(resp.StatusCode, respBody)
	}

	return nil
}

// GetMeetingCount retrieves the count of meetings for a project from ITX
func (c *Client) GetMeetingCount(ctx context.Context, projectID string) (*itx.MeetingCountResponse, error) {
	// Create HTTP request with query parameter
	url := fmt.Sprintf("%s/v2/zoom/meeting_count?project=%s", c.config.BaseURL, projectID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization automatically added by OAuth2 transport)
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("x-scope", "manage:zoom")

	// Log request
	c.logRequest(ctx, http.MethodGet, url, nil)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, domain.NewInternalError("failed to read response", err)
	}

	// Log response
	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.mapHTTPError(resp.StatusCode, respBody)
	}

	// Parse response
	var result itx.MeetingCountResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}

	return &result, nil
}

// GetMeetingJoinLink retrieves a join link for a meeting from ITX
func (c *Client) GetMeetingJoinLink(ctx context.Context, req *itx.GetJoinLinkRequest) (*itx.ZoomMeetingJoinLink, error) {
	// Build URL with query parameters
	queryURL := fmt.Sprintf("%s/v2/zoom/meetings/%s/join_link", c.config.BaseURL, req.MeetingID)

	// Build query parameters
	params := url.Values{}
	if req.UseEmail {
		params.Add("use_email", "true")
	}
	if req.UserID != "" {
		params.Add("user_id", req.UserID)
	}
	if req.Name != "" {
		params.Add("name", req.Name)
	}
	if req.Email != "" {
		params.Add("email", req.Email)
	}
	if req.Register {
		params.Add("register", "true")
	}

	if len(params) > 0 {
		queryURL = fmt.Sprintf("%s?%s", queryURL, params.Encode())
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL, nil)
	if err != nil {
		return nil, domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization automatically added by OAuth2 transport)
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("x-scope", "manage:zoom")

	// Log request
	c.logRequest(ctx, http.MethodGet, queryURL, nil)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, domain.NewInternalError("failed to read response", err)
	}

	// Log response
	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.mapHTTPError(resp.StatusCode, respBody)
	}

	// Parse response
	var result itx.ZoomMeetingJoinLink
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}

	return &result, nil
}

// CreateRegistrant creates a meeting registrant via ITX proxy
func (c *Client) CreateRegistrant(ctx context.Context, meetingID string, req *itx.ZoomMeetingRegistrant) (*itx.ZoomMeetingRegistrant, error) {
	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return nil, domain.NewInternalError("failed to marshal request", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/v2/zoom/meetings/%s/registrants", c.config.BaseURL, meetingID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization automatically added by OAuth2 transport)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("x-scope", "manage:zoom")

	// Log request
	c.logRequest(ctx, http.MethodPost, url, body)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, domain.NewInternalError("failed to read response", err)
	}

	// Log response
	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.mapHTTPError(resp.StatusCode, respBody)
	}

	// Parse response
	var result itx.ZoomMeetingRegistrant
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}

	return &result, nil
}

// GetRegistrant retrieves a meeting registrant via ITX proxy
func (c *Client) GetRegistrant(ctx context.Context, meetingID, registrantID string) (*itx.ZoomMeetingRegistrant, error) {
	// Create HTTP request
	url := fmt.Sprintf("%s/v2/zoom/meetings/%s/registrants/%s", c.config.BaseURL, meetingID, registrantID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization automatically added by OAuth2 transport)
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("x-scope", "manage:zoom")

	// Log request
	c.logRequest(ctx, http.MethodGet, url, nil)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, domain.NewInternalError("failed to read response", err)
	}

	// Log response
	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.mapHTTPError(resp.StatusCode, respBody)
	}

	// Parse response
	var result itx.ZoomMeetingRegistrant
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, domain.NewInternalError("failed to parse response", err)
	}

	return &result, nil
}

// UpdateRegistrant updates a meeting registrant via ITX proxy
func (c *Client) UpdateRegistrant(ctx context.Context, meetingID, registrantID string, req *itx.ZoomMeetingRegistrant) error {
	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return domain.NewInternalError("failed to marshal request", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/v2/zoom/meetings/%s/registrants/%s", c.config.BaseURL, meetingID, registrantID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization automatically added by OAuth2 transport)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-scope", "manage:zoom")

	// Log request
	c.logRequest(ctx, http.MethodPut, url, body)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body (for error messages)
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.NewInternalError("failed to read response", err)
	}

	// Log response
	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.mapHTTPError(resp.StatusCode, respBody)
	}

	return nil
}

// DeleteRegistrant deletes a meeting registrant via ITX proxy
func (c *Client) DeleteRegistrant(ctx context.Context, meetingID, registrantID string) error {
	// Create HTTP request
	url := fmt.Sprintf("%s/v2/zoom/meetings/%s/registrants/%s", c.config.BaseURL, meetingID, registrantID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization automatically added by OAuth2 transport)
	httpReq.Header.Set("x-scope", "manage:zoom")

	// Log request
	c.logRequest(ctx, http.MethodDelete, url, nil)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body (for error messages)
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.NewInternalError("failed to read response", err)
	}

	// Log response
	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.mapHTTPError(resp.StatusCode, respBody)
	}

	return nil
}

// GetRegistrantICS retrieves an ICS calendar file for a meeting registrant via ITX proxy
func (c *Client) GetRegistrantICS(ctx context.Context, meetingID, registrantID string) (*itx.RegistrantICS, error) {
	// Create HTTP request
	url := fmt.Sprintf("%s/v2/zoom/meetings/%s/registrants/%s/ics", c.config.BaseURL, meetingID, registrantID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization automatically added by OAuth2 transport)
	httpReq.Header.Set("Accept", "text/calendar")
	httpReq.Header.Set("x-scope", "manage:zoom")

	// Log request
	c.logRequest(ctx, http.MethodGet, url, nil)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, domain.NewInternalError("failed to read response", err)
	}

	// Log response
	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.mapHTTPError(resp.StatusCode, respBody)
	}

	// Return ICS content as-is (binary data)
	return &itx.RegistrantICS{
		Content: respBody,
	}, nil
}

// ResendRegistrantInvitation resends a meeting invitation to a registrant via ITX proxy
func (c *Client) ResendRegistrantInvitation(ctx context.Context, meetingID, registrantID string) error {
	// Create HTTP request
	url := fmt.Sprintf("%s/v2/zoom/meetings/%s/registrants/%s/resend", c.config.BaseURL, meetingID, registrantID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization automatically added by OAuth2 transport)
	httpReq.Header.Set("x-scope", "manage:zoom")

	// Log request
	c.logRequest(ctx, http.MethodPost, url, nil)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body for error handling
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.NewInternalError("failed to read response", err)
	}

	// Log response
	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.mapHTTPError(resp.StatusCode, respBody)
	}

	return nil
}

// ResendMeetingInvitations resends meeting invitations to all registrants via ITX proxy
func (c *Client) ResendMeetingInvitations(ctx context.Context, meetingID string, req *itx.ResendMeetingInvitationsRequest) error {
	// Always marshal the request body, even if empty
	// ITX API expects a JSON body (empty object {} is fine)
	if req == nil {
		req = &itx.ResendMeetingInvitationsRequest{}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return domain.NewInternalError("failed to marshal request", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/v2/zoom/meetings/%s/resend", c.config.BaseURL, meetingID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization automatically added by OAuth2 transport)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-scope", "manage:zoom")

	// Log request
	c.logRequest(ctx, http.MethodPost, url, body)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body for error handling
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.NewInternalError("failed to read response", err)
	}

	// Log response
	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.mapHTTPError(resp.StatusCode, respBody)
	}

	return nil
}

// RegisterCommitteeMembers registers committee members to a meeting asynchronously via ITX proxy
func (c *Client) RegisterCommitteeMembers(ctx context.Context, meetingID string) error {
	// Create HTTP request
	url := fmt.Sprintf("%s/v2/zoom/meetings/%s/register_committee_members", c.config.BaseURL, meetingID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization automatically added by OAuth2 transport)
	httpReq.Header.Set("x-scope", "manage:zoom")

	// Log request
	c.logRequest(ctx, http.MethodPost, url, nil)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body for error handling
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.NewInternalError("failed to read response", err)
	}

	// Log response
	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.mapHTTPError(resp.StatusCode, respBody)
	}

	return nil
}

// UpdateOccurrence updates a specific occurrence of a recurring meeting via ITX proxy
func (c *Client) UpdateOccurrence(ctx context.Context, meetingID, occurrenceID string, req *itx.UpdateOccurrenceRequest) error {
	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return domain.NewInternalError("failed to marshal request", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/v2/zoom/meetings/%s/occurrences/%s", c.config.BaseURL, meetingID, occurrenceID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization automatically added by OAuth2 transport)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-scope", "manage:zoom")

	// Log request
	c.logRequest(ctx, http.MethodPut, url, body)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body for error handling
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.NewInternalError("failed to read response", err)
	}

	// Log response
	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.mapHTTPError(resp.StatusCode, respBody)
	}

	return nil
}

// DeleteOccurrence deletes a specific occurrence of a recurring meeting via ITX proxy
func (c *Client) DeleteOccurrence(ctx context.Context, meetingID, occurrenceID string) error {
	// Create HTTP request
	url := fmt.Sprintf("%s/v2/zoom/meetings/%s/occurrences/%s", c.config.BaseURL, meetingID, occurrenceID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization automatically added by OAuth2 transport)
	httpReq.Header.Set("x-scope", "manage:zoom")

	// Log request
	c.logRequest(ctx, http.MethodDelete, url, nil)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body for error handling
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.NewInternalError("failed to read response", err)
	}

	// Log response
	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.mapHTTPError(resp.StatusCode, respBody)
	}

	return nil
}

// CreatePastMeeting creates a past meeting record via ITX proxy
func (c *Client) CreatePastMeeting(ctx context.Context, req *itx.CreatePastMeetingRequest) (*itx.PastMeetingResponse, error) {
	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return nil, domain.NewInternalError("failed to marshal request", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/v2/zoom/past_meetings", c.config.BaseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization automatically added by OAuth2 transport)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("x-scope", "manage:zoom")

	// Log request
	c.logRequest(ctx, http.MethodPost, url, body)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, domain.NewInternalError("failed to read response", err)
	}

	// Log response
	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.mapHTTPError(resp.StatusCode, respBody)
	}

	// Unmarshal response
	var result itx.PastMeetingResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, domain.NewInternalError("failed to unmarshal response", err)
	}

	return &result, nil
}

// GetPastMeeting retrieves a past meeting record via ITX proxy
func (c *Client) GetPastMeeting(ctx context.Context, pastMeetingID string) (*itx.PastMeetingResponse, error) {
	// Create HTTP request
	url := fmt.Sprintf("%s/v2/zoom/past_meetings/%s", c.config.BaseURL, pastMeetingID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization automatically added by OAuth2 transport)
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("x-scope", "manage:zoom")

	// Log request
	c.logRequest(ctx, http.MethodGet, url, nil)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, domain.NewInternalError("failed to read response", err)
	}

	// Log response
	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.mapHTTPError(resp.StatusCode, respBody)
	}

	// Unmarshal response
	var result itx.PastMeetingResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, domain.NewInternalError("failed to unmarshal response", err)
	}

	return &result, nil
}

// UpdatePastMeeting updates a past meeting record via ITX proxy
// Returns nil on success (ITX API returns 204 No Content)
func (c *Client) UpdatePastMeeting(ctx context.Context, pastMeetingID string, req *itx.CreatePastMeetingRequest) (*itx.PastMeetingResponse, error) {
	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return nil, domain.NewInternalError("failed to marshal request", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/v2/zoom/past_meetings/%s", c.config.BaseURL, pastMeetingID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return nil, domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization automatically added by OAuth2 transport)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("x-scope", "manage:zoom")

	// Log request
	c.logRequest(ctx, http.MethodPut, url, body)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body for error handling
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, domain.NewInternalError("failed to read response", err)
	}

	// Log response
	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.mapHTTPError(resp.StatusCode, respBody)
	}

	// Success - ITX returns 204 No Content, return nil
	return nil, nil
}

// DeletePastMeeting deletes a past meeting record via ITX proxy

func (c *Client) DeletePastMeeting(ctx context.Context, pastMeetingID string) error {
	// Create HTTP request
	url := fmt.Sprintf("%s/v2/zoom/past_meetings/%s", c.config.BaseURL, pastMeetingID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization automatically added by OAuth2 transport)
	httpReq.Header.Set("x-scope", "manage:zoom")

	c.logRequest(ctx, http.MethodDelete, url, nil)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body for error handling
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.NewInternalError("failed to read response", err)
	}

	// Log response
	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.mapHTTPError(resp.StatusCode, respBody)
	}

	return nil
}

// GetPastMeetingSummary retrieves a past meeting summary from ITX
func (c *Client) GetPastMeetingSummary(ctx context.Context, pastMeetingID, summaryID string) (*itx.PastMeetingSummaryResponse, error) {
	// Create HTTP request
	url := fmt.Sprintf("%s/v2/zoom/past_meetings/%s/summaries/%s", c.config.BaseURL, pastMeetingID, summaryID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization automatically added by OAuth2 transport)
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("x-scope", "manage:zoom")

	// Log request
	c.logRequest(ctx, http.MethodGet, url, nil)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, domain.NewInternalError("failed to read response", err)
	}

	// Log response
	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.mapHTTPError(resp.StatusCode, respBody)
	}

	// Unmarshal response
	var summaryResp itx.PastMeetingSummaryResponse
	if err := json.Unmarshal(respBody, &summaryResp); err != nil {
		return nil, domain.NewInternalError("failed to unmarshal response", err)
	}

	return &summaryResp, nil
}

// UpdatePastMeetingSummary updates a past meeting summary in ITX
func (c *Client) UpdatePastMeetingSummary(ctx context.Context, pastMeetingID, summaryID string, req *itx.UpdatePastMeetingSummaryRequest) (*itx.PastMeetingSummaryResponse, error) {
	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return nil, domain.NewInternalError("failed to marshal request", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/v2/zoom/past_meetings/%s/summaries/%s", c.config.BaseURL, pastMeetingID, summaryID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return nil, domain.NewInternalError("failed to create request", err)
	}

	// Set headers (Authorization automatically added by OAuth2 transport)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("x-scope", "manage:zoom")

	// Log request
	c.logRequest(ctx, http.MethodPut, url, body)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, domain.NewInternalError("failed to read response", err)
	}

	// Log response
	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.mapHTTPError(resp.StatusCode, respBody)
	}

	// Unmarshal response
	var summaryResp itx.PastMeetingSummaryResponse
	if err := json.Unmarshal(respBody, &summaryResp); err != nil {
		return nil, domain.NewInternalError("failed to unmarshal response", err)
	}

	return &summaryResp, nil
}

// mapHTTPError maps HTTP status codes to domain errors
func (c *Client) mapHTTPError(statusCode int, body []byte) error {
	var errMsg itx.ErrorResponse
	_ = json.Unmarshal(body, &errMsg)

	message := errMsg.Message
	if message == "" {
		message = errMsg.Error
	}
	if message == "" {
		message = fmt.Sprintf("HTTP %d error", statusCode)
	}

	switch statusCode {
	case http.StatusBadRequest:
		return domain.NewValidationError(message)
	case http.StatusUnauthorized, http.StatusForbidden:
		return domain.NewValidationError(fmt.Sprintf("authentication/authorization failed: %s", message))
	case http.StatusNotFound:
		return domain.NewNotFoundError(message)
	case http.StatusConflict:
		return domain.NewConflictError(message)
	case http.StatusServiceUnavailable:
		return domain.NewUnavailableError(message)
	default:
		return domain.NewInternalError(message)
	}
}

// CreateInvitee creates an invitee for a past meeting via the ITX proxy
func (c *Client) CreateInvitee(ctx context.Context, pastMeetingID string, req *itx.CreateInviteeRequest) (*itx.InviteeResponse, error) {
	url := fmt.Sprintf("%s/v2/zoom/past_meetings/%s/invitees", c.config.BaseURL, pastMeetingID)

	// Marshal request body
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, domain.NewInternalError("failed to marshal request", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, domain.NewInternalError("failed to create request", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-scope", "manage:zoom")

	// Log request
	c.logRequest(ctx, http.MethodPost, url, bodyBytes)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, domain.NewInternalError("failed to read response", err)
	}

	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.mapHTTPError(resp.StatusCode, respBody)
	}

	// Unmarshal response
	var inviteeResp itx.InviteeResponse
	if err := json.Unmarshal(respBody, &inviteeResp); err != nil {
		return nil, domain.NewInternalError("failed to unmarshal response", err)
	}

	return &inviteeResp, nil
}

// UpdateInvitee updates an invitee for a past meeting via the ITX proxy
func (c *Client) UpdateInvitee(ctx context.Context, pastMeetingID, inviteeID string, req *itx.UpdateInviteeRequest) (*itx.InviteeResponse, error) {
	url := fmt.Sprintf("%s/v2/zoom/past_meetings/%s/invitees/%s", c.config.BaseURL, pastMeetingID, inviteeID)

	// Marshal request body
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, domain.NewInternalError("failed to marshal request", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, domain.NewInternalError("failed to create request", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-scope", "manage:zoom")

	// Log request
	c.logRequest(ctx, http.MethodPut, url, bodyBytes)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, domain.NewInternalError("failed to read response", err)
	}

	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.mapHTTPError(resp.StatusCode, respBody)
	}

	// Success - ITX returns 204 No Content, return nil
	return nil, nil
}

// DeleteInvitee deletes an invitee from a past meeting via the ITX proxy
func (c *Client) DeleteInvitee(ctx context.Context, pastMeetingID, inviteeID string) error {
	url := fmt.Sprintf("%s/v2/zoom/past_meetings/%s/invitees/%s", c.config.BaseURL, pastMeetingID, inviteeID)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return domain.NewInternalError("failed to create request", err)
	}

	// Set headers
	httpReq.Header.Set("x-scope", "manage:zoom")

	// Log request
	c.logRequest(ctx, http.MethodDelete, url, nil)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.NewInternalError("failed to read response", err)
	}

	// Log response
	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.mapHTTPError(resp.StatusCode, respBody)
	}

	return nil
}

// CreateAttendee creates an attendee for a past meeting via the ITX proxy
func (c *Client) CreateAttendee(ctx context.Context, pastMeetingID string, req *itx.CreateAttendeeRequest) (*itx.AttendeeResponse, error) {
	url := fmt.Sprintf("%s/v2/zoom/past_meetings/%s/attendees", c.config.BaseURL, pastMeetingID)

	// Marshal request body
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, domain.NewInternalError("failed to marshal request", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, domain.NewInternalError("failed to create request", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-scope", "manage:zoom")

	// Log request
	c.logRequest(ctx, http.MethodPost, url, bodyBytes)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, domain.NewInternalError("failed to read response", err)
	}

	// Log response
	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.mapHTTPError(resp.StatusCode, respBody)
	}

	// Unmarshal response
	var attendeeResp itx.AttendeeResponse
	if err := json.Unmarshal(respBody, &attendeeResp); err != nil {
		return nil, domain.NewInternalError("failed to unmarshal response", err)
	}

	return &attendeeResp, nil
}

// UpdateAttendee updates an attendee for a past meeting via the ITX proxy
func (c *Client) UpdateAttendee(ctx context.Context, pastMeetingID, attendeeID string, req *itx.UpdateAttendeeRequest) (*itx.AttendeeResponse, error) {
	url := fmt.Sprintf("%s/v2/zoom/past_meetings/%s/attendees/%s", c.config.BaseURL, pastMeetingID, attendeeID)

	// Marshal request body
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, domain.NewInternalError("failed to marshal request", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, domain.NewInternalError("failed to create request", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-scope", "manage:zoom")

	// Log request
	c.logRequest(ctx, http.MethodPut, url, bodyBytes)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, domain.NewInternalError("failed to read response", err)
	}

	// Log response
	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.mapHTTPError(resp.StatusCode, respBody)
	}

	// Success - ITX returns 204 No Content, return nil
	return nil, nil
}

// DeleteAttendee deletes an attendee from a past meeting via the ITX proxy
func (c *Client) DeleteAttendee(ctx context.Context, pastMeetingID, attendeeID string) error {
	url := fmt.Sprintf("%s/v2/zoom/past_meetings/%s/attendees/%s", c.config.BaseURL, pastMeetingID, attendeeID)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return domain.NewInternalError("failed to create request", err)
	}

	// Set headers
	httpReq.Header.Set("x-scope", "manage:zoom")

	// Log request
	c.logRequest(ctx, http.MethodDelete, url, nil)

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.NewInternalError("failed to read response", err)
	}

	// Log response
	c.logResponse(ctx, resp.StatusCode, respBody)

	// Handle non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.mapHTTPError(resp.StatusCode, respBody)
	}

	return nil
}
