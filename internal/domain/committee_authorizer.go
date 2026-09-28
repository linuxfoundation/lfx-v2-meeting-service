// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import "context"

// CommitteeAuthorizer checks whether a principal holds write access on a
// committee object in OpenFGA. It is called by UpdateMeeting to guard newly
// added committees — ones present in the request but absent from the live ITX
// record — before forwarding the update to ITX.
//
// A nil CommitteeAuthorizer means NATS was not configured at startup; callers
// skip the check entirely in that case. When non-nil, any HasWriteAccess error
// fails the request closed (503) — the service refuses to start with NATS
// configured but unreachable, so a nil authorizer only ever means NATS is
// intentionally absent, not transiently unavailable.
type CommitteeAuthorizer interface {
	// HasWriteAccess reports whether principal (the JWT username) has the
	// "writer" relation on committee:{committeeID} in OpenFGA.
	// A false result with a nil error means access is explicitly denied.
	// An error means the check could not be completed.
	HasWriteAccess(ctx context.Context, principal, committeeID string) (bool, error)
}
