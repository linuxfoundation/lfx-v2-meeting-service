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
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/proxy"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

const itxScope = "manage:zoom"

const (
	acceptJSON     = "application/json"
	acceptCalendar = "text/calendar"
)

type apiRequest struct {
	method   string
	path     string
	pathArgs []any
	query    url.Values
	body     any
	accept   string
	// debugOp names the operation for logging purposes. It drives the
	// Debug-level request/response lines below and, unconditionally, the
	// operation name in the Error-level failure log. Every call site should
	// set it.
	debugOp     string
	debugFields []any
	// redactQuery lists query parameter keys whose values must never appear
	// in logs (e.g. email/name/user_id on the join-link request). The real
	// request still sends the unredacted values to ITX; only the URL used
	// in slog output is masked.
	redactQuery []string
	// skipBodyLog suppresses the "request" body field in the Debug request
	// log. Use it when body is a type proxy.RequestJSONForLog's audit-field
	// redaction doesn't know about and that carries PII (e.g. AcceptInvite's
	// email/username) — the op name/method/url/statusCode still log.
	skipBodyLog bool
	// skipResponseLog suppresses the "response" body field in the Debug
	// success log and the Error failure log. Use it when the response type
	// carries fields proxy.ResponseJSONForLog's generic created_by/updated_by/
	// modified_by/file_uploaded_by redaction doesn't reach — e.g. a meeting's
	// host_key/passcode/owner, a join link's URL, or a registrant's direct
	// email/name fields — statusCode and the operation name still log.
	skipResponseLog bool
	parseError      string
}

// redactQueryParams returns rawURL with the values of the given query
// parameter keys replaced with "REDACTED", for safe use in log output. The
// original URL (used for the actual outbound request) is left untouched.
func redactQueryParams(rawURL string, keys []string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		// Fail closed: an unparseable URL can't be checked for the sensitive
		// keys we're trying to redact, and rawURL may still carry them (e.g.
		// a malformed BaseURL config with a join-link query attached).
		return "REDACTED (unparseable URL)"
	}
	q := u.Query()
	redacted := false
	for _, k := range keys {
		if q.Has(k) {
			q.Set(k, "REDACTED")
			redacted = true
		}
	}
	if !redacted {
		return rawURL
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// maxLoggedResponseBytes bounds the size of an ITX response body written to
// logs, so a large or malformed upstream response can't flood log storage.
const maxLoggedResponseBytes = 512

// truncateForLog caps s at max bytes, appending a marker if truncated.
func truncateForLog(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
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
	logURL := targetURL
	if len(req.redactQuery) > 0 {
		logURL = redactQueryParams(targetURL, req.redactQuery)
	}

	op := req.debugOp
	if op == "" {
		// Should not happen once every call site sets debugOp; keeps the
		// failure logging below meaningful even if one is missed.
		op = "unnamed ITX request"
	}

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
		fields := append([]any{"method", req.method, "url", logURL}, req.debugFields...)
		if req.body != nil && !req.skipBodyLog {
			fields = append(fields, "request", proxy.RequestJSONForLog(req.body))
		}
		slog.DebugContext(ctx, "ITX "+op+" request", fields...)
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
		// Error, not Debug: a transport failure (DNS, timeout, connection
		// refused) is exactly the kind of issue that must be visible without
		// flipping LOG_LEVEL, so this is unconditional regardless of debugOp.
		// http.Client.Do commonly returns a *url.Error whose Error() string
		// embeds the full request URL (including query), so the same
		// redaction applied to logURL must also be applied to the error text.
		errMsg := err.Error()
		if len(req.redactQuery) > 0 {
			errMsg = strings.ReplaceAll(errMsg, targetURL, logURL)
		}
		slog.ErrorContext(ctx, "ITX "+op+" request failed",
			"method", req.method, "url", logURL, logging.ErrKey, errMsg)
		return nil, domain.NewUnavailableError("ITX service request failed", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.ErrorContext(ctx, "ITX "+op+" failed to read response",
			"method", req.method, "url", logURL, "statusCode", resp.StatusCode, logging.ErrKey, err)
		return nil, domain.NewInternalError("failed to read response", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Unconditional Error log with the ITX status code — this is the
		// visibility gap #1897 exists to close: previously a non-2xx from ITX
		// left zero trace in application logs. The response body is included
		// unless skipResponseLog or skipBodyLog is set: some ITX error bodies
		// echo the submitted fields verbatim, so an operation whose request
		// carries PII (skipBodyLog) is just as much at risk here as one whose
		// response does.
		errFields := []any{"method", req.method, "url", logURL, "statusCode", resp.StatusCode}
		if !req.skipResponseLog && !req.skipBodyLog {
			errFields = append(errFields, "response", truncateForLog(proxy.ResponseJSONForLog(respBody), maxLoggedResponseBytes))
		}
		slog.ErrorContext(ctx, "ITX "+op+" error response", errFields...)
		return nil, recordAndMapHTTPError(ctx, op, resp.StatusCode, respBody)
	}

	if req.debugOp != "" {
		respFields := []any{"statusCode", resp.StatusCode}
		if !req.skipResponseLog {
			respFields = append(respFields, "response", truncateForLog(proxy.ResponseJSONForLog(respBody), maxLoggedResponseBytes))
		}
		slog.DebugContext(ctx, "ITX "+op+" response", respFields...)
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

// recordAndMapHTTPError maps a non-2xx status to a domain error. For 5xx
// responses it also records a status-only error on the active OTel span so
// that Datadog surfaces error.message/error.type without leaking upstream
// PII. 4xx responses are expected client errors, not server faults, so they
// don't flip the span to an error status — instead they get status-code/
// operation attributes, keeping them traceable without polluting error-rate
// monitors.
func recordAndMapHTTPError(ctx context.Context, op string, statusCode int, body []byte) error {
	err := mapHTTPError(statusCode, body)
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.Int("itx.status_code", statusCode),
		attribute.String("itx.operation", op),
	)
	if statusCode >= 500 {
		span.RecordError(fmt.Errorf("HTTP %d", statusCode))
		span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", statusCode))
	}
	return err
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
