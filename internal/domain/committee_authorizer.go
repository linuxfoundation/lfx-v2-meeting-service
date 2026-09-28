// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import "context"

// CommitteeAuthorizer checks whether a principal holds write access on a
// committee object in OpenFGA. It is called by UpdateMeeting to guard newly
// added committees — ones present in the request but absent from the live ITX
// record — before forwarding the update to ITX.
//
// Implementations may be nil (e.g. when NATS is unavailable); callers must
// treat nil as "skip the check" so that committee updates still work in
// degraded NATS environments.
type CommitteeAuthorizer interface {
	// HasWriteAccess reports whether principal (the JWT username) has the
	// "writer" relation on committee:{committeeID} in OpenFGA.
	// A false result with a nil error means access is explicitly denied.
	// An error means the check could not be completed.
	HasWriteAccess(ctx context.Context, principal, committeeID string) (bool, error)
}
