// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"fmt"
	"time"

	nats "github.com/nats-io/nats.go"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

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
// Returns ("", nil) when no slug can be resolved — an empty reply can indicate
// the project was not found or that it exists but has no slug.
// Returns a non-nil error for transient NATS failures.
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
	return string(msg.Data), nil
}

// Ensure NATSProjectLookup implements domain.ProjectLookup.
var _ domain.ProjectLookup = (*NATSProjectLookup)(nil)
