// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import "errors"

// Domain errors
var (
	// ErrMeetingNotFound is returned when a meeting is not found.
	ErrMeetingNotFound = errors.New("meeting not found")
	// ErrPastMeetingNotFound is returned when a past meeting is not found.
	ErrPastMeetingNotFound = errors.New("past meeting not found")
	// ErrPastMeetingParticipantNotFound is returned when a past meeting participant is not found.
	ErrPastMeetingParticipantNotFound = errors.New("past meeting participant not found")
	// ErrPastMeetingParticipantAlreadyExists is returned when a past meeting participant already exists.
	ErrPastMeetingParticipantAlreadyExists = errors.New("past meeting participant already exists")
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
	// ErrRegistrantNotFound is returned when a registrant is not found.
	ErrRegistrantNotFound = errors.New("registrant not found")
	// ErrRegistrantAlreadyExists is returned when a registrant already exists.
	ErrRegistrantAlreadyExists = errors.New("registrant already exists")
	// ErrPlatformProviderNotFound is returned when a platform provider is not found.
	ErrPlatformProviderNotFound = errors.New("platform provider not found")
	// ErrMarshal is returned when a marshal error occurs.
	ErrMarshal = errors.New("marshal error")
)
