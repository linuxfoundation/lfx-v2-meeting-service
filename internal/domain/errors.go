// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import "errors"

// Domain errors
var (
	// ErrMeetingNotFound is returned when a meeting is not found.
	ErrMeetingNotFound = errors.New("meeting not found")
	// ErrInternal is returned when an internal error occurs.
	ErrInternal = errors.New("internal error")
	// ErrRevisionMismatch is returned when a revision mismatch occurs.
	ErrRevisionMismatch = errors.New("revision mismatch")
	// ErrUnmarshal is returned when an unmarshal error occurs.
	ErrUnmarshal = errors.New("unmarshal error")
	// ErrServiceUnavailable is returned when a service is unavailable.
	ErrServiceUnavailable = errors.New("service unavailable")
	// ErrValidationFailed is returned when a validation failed.
	ErrValidationFailed = errors.New("validation failed")
)
