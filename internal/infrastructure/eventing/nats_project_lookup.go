// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	nats "github.com/nats-io/nats.go"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

// projectServiceErrorCode returns the error code from a project-service error
// envelope ({"error":"not_found",...} or {"error":"internal",...}), or "" if
// data is a normal success payload.
func projectServiceErrorCode(data []byte) string {
	var env struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(data, &env) != nil {
		return ""
	}
	return env.Error
}

const (
	projectGetSlugSubject    = "lfx.projects-api.get_slug"
	projectSlugLookupTimeout = 5 * time.Second
)

// NATSProjectLookup implements domain.ProjectLookup using NATS request/reply.
type NATSProjectLookup struct {
	nc      *nats.Conn
	timeout time.Duration
}

// NewNATSProjectLookup creates a new NATS-based project slug lookup.
func NewNATSProjectLookup(nc *nats.Conn) *NATSProjectLookup {
	return &NATSProjectLookup{
		nc:      nc,
		timeout: projectSlugLookupTimeout,
	}
}

// GetProjectSlug returns the URL slug for the given project UID by calling the
// projects API over NATS on subject lfx.projects-api.get_slug.
// Returns ("", nil) when the project exists but has no slug assigned.
// Returns a non-nil error for NATS failures or a project-service error envelope.
func (p *NATSProjectLookup) GetProjectSlug(ctx context.Context, projectUID string) (string, error) {
	if projectUID == "" {
		return "", nil
	}
	reqCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	msg, err := p.nc.RequestWithContext(reqCtx, projectGetSlugSubject, []byte(projectUID))
	if err != nil {
		return "", fmt.Errorf("project slug lookup failed for uid %q: %w", projectUID, err)
	}

	// Project-service returns {"error":"<code>",...} on errors; any other response
	// (plain string or empty body for a project with no slug) is a success value.
	if code := projectServiceErrorCode(msg.Data); code != "" {
		if code == "not_found" {
			return "", domain.NewNotFoundError(fmt.Sprintf("project %q not found", projectUID))
		}
		return "", domain.NewInternalError(fmt.Sprintf("project service error for uid %q: %s", projectUID, code))
	}
	return string(msg.Data), nil
}

// Ensure NATSProjectLookup implements domain.ProjectLookup.
var _ domain.ProjectLookup = (*NATSProjectLookup)(nil)
