// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

// fgaAccessCheckSubject is the NATS request/reply subject exposed by fga-sync.
const fgaAccessCheckSubject = "lfx.access_check.request"

// committeeAuthorizerTimeout is the maximum time to wait for fga-sync to reply.
// Committee writes are infrequent; 10 s is generous but bounded.
const committeeAuthorizerTimeout = 10 * time.Second

// NATSCommitteeAuthorizer implements domain.CommitteeAuthorizer by calling the
// fga-sync access-check RPC over NATS (subject: lfx.access_check.request).
//
// Wire format (same as the lfx-v2-access-check service):
//
//	request:  "committee:{id}#writer@user:{principal}"
//	response: "committee:{id}#writer@user:{principal}\t{true|false}"
type NATSCommitteeAuthorizer struct {
	nc Requester
}

// Verify interface compliance at compile time.
var _ domain.CommitteeAuthorizer = (*NATSCommitteeAuthorizer)(nil)

// NewNATSCommitteeAuthorizer creates a NATSCommitteeAuthorizer backed by nc.
func NewNATSCommitteeAuthorizer(nc Requester) *NATSCommitteeAuthorizer {
	return &NATSCommitteeAuthorizer{nc: nc}
}

// HasWriteAccess asks fga-sync whether principal has the "writer" relation on
// committee:{committeeID}. It returns (false, nil) when access is explicitly
// denied, (true, nil) when granted, and (false, err) when the RPC fails.
func (a *NATSCommitteeAuthorizer) HasWriteAccess(ctx context.Context, principal, committeeID string) (bool, error) {
	tuple := fmt.Sprintf("committee:%s#writer@user:%s", committeeID, principal)

	// Always cap at committeeAuthorizerTimeout. context.WithTimeout takes the sooner
	// of the parent deadline and the specified duration, so this never extends a
	// caller's shorter deadline.
	ctx, cancel := context.WithTimeout(ctx, committeeAuthorizerTimeout)
	defer cancel()

	msg, err := a.nc.RequestWithContext(ctx, fgaAccessCheckSubject, []byte(tuple))
	if err != nil {
		return false, fmt.Errorf("fga access check NATS request failed: %w", err)
	}

	return parseAccessCheckResponse(msg.Data, tuple)
}

// parseAccessCheckResponse reads the first matching result line from the
// fga-sync response. The format is newline-delimited
// "object#relation@user\tallowed" lines. A space within the first 20 bytes
// indicates an error message from fga-sync rather than a valid payload.
func parseAccessCheckResponse(data []byte, expectedTuple string) (bool, error) {
	// Sanity check: fga-sync error messages contain spaces; valid result lines do not.
	topRange := 20
	if len(data) < topRange {
		topRange = len(data)
	}
	if bytes.Contains(data[:topRange], []byte(" ")) {
		return false, fmt.Errorf("fga-sync returned error: %q", string(data[:topRange]))
	}

	for _, rawLine := range bytes.Split(data, []byte("\n")) {
		line := string(bytes.TrimSpace(rawLine))
		if line == "" {
			continue
		}
		// Each line: "object#relation@user\tallowed"
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		if parts[0] == expectedTuple {
			return parts[1] == "true", nil
		}
	}

	return false, fmt.Errorf("fga-sync response did not contain result for %q", expectedTuple)
}
