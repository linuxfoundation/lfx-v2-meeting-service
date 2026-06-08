// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

const itxScope = "manage:zoom"

const (
	acceptJSON     = "application/json"
	acceptCalendar = "text/calendar"
)

type apiRequest struct {
	method      string
	path        string
	pathArgs    []any
	query       url.Values
	body        any
	accept      string
	debugOp     string
	debugFields []any
	parseError  string
}

func (c *Client) apiURL(path string, args ...any) string {
	if len(args) > 0 {
		return c.config.BaseURL + fmt.Sprintf(path, args...)
	}
	return c.config.BaseURL + path
}

func withQuery(rawURL string, query url.Values) string {
	if len(query) == 0 {
		return rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL + "?" + query.Encode()
	}
	u.RawQuery = query.Encode()
	return u.String()
}

func (c *Client) execute(ctx context.Context, req apiRequest) ([]byte, error) {
	targetURL := withQuery(c.apiURL(req.path, req.pathArgs...), req.query)

	var bodyReader io.Reader
	var bodyBytes []byte
	if req.body != nil {
		var err error
		bodyBytes, err = json.Marshal(req.body)
		if err != nil {
			return nil, domain.NewInternalError("failed to marshal request", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	if req.debugOp != "" {
		fields := append([]any{"method", req.method, "url", targetURL}, req.debugFields...)
		if len(bodyBytes) > 0 {
			fields = append(fields, "request", string(bodyBytes))
		}
		slog.DebugContext(ctx, "ITX "+req.debugOp+" request", fields...)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.method, targetURL, bodyReader)
	if err != nil {
		return nil, domain.NewInternalError("failed to create request", err)
	}

	if bodyReader != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	if req.accept != "" {
		httpReq.Header.Set("Accept", req.accept)
	}
	httpReq.Header.Set("x-scope", itxScope)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		if req.debugOp != "" {
			slog.DebugContext(ctx, "ITX "+req.debugOp+" request failed", logging.ErrKey, err)
		}
		return nil, domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, domain.NewInternalError("failed to read response", err)
	}

	if req.debugOp != "" {
		slog.DebugContext(ctx, "ITX "+req.debugOp+" response",
			"statusCode", resp.StatusCode,
			"response", string(respBody))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, mapHTTPError(resp.StatusCode, respBody)
	}

	return respBody, nil
}

func (c *Client) doJSON(ctx context.Context, req apiRequest, dest any) error {
	respBody, err := c.execute(ctx, req)
	if err != nil {
		return err
	}
	if len(respBody) == 0 {
		if dest == nil {
			return nil
		}
		return domain.NewInternalError("empty response body", nil)
	}

	parseError := req.parseError
	if parseError == "" {
		parseError = "failed to parse response"
	}
	if err := json.Unmarshal(respBody, dest); err != nil {
		return domain.NewInternalError(parseError, err)
	}
	return nil
}

func doJSONTyped[T any](c *Client, ctx context.Context, req apiRequest) (*T, error) {
	var result T
	if err := c.doJSON(ctx, req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func doJSONTypedOptional[T any](c *Client, ctx context.Context, req apiRequest) (*T, error) {
	respBody, err := c.execute(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(respBody) == 0 {
		return nil, nil
	}

	parseError := req.parseError
	if parseError == "" {
		parseError = "failed to unmarshal response"
	}
	var result T
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, domain.NewInternalError(parseError, err)
	}
	return &result, nil
}

func (c *Client) doNoContent(ctx context.Context, req apiRequest) error {
	_, err := c.execute(ctx, req)
	return err
}

func (c *Client) doRaw(ctx context.Context, req apiRequest) ([]byte, error) {
	return c.execute(ctx, req)
}

func mapHTTPError(statusCode int, body []byte) error {
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
