// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import "errors"

// ErrorType represents the semantic category of an error
type ErrorType int

const (
	ErrorTypeValidation  ErrorType = iota // Input validation errors (400 Bad Request)
	ErrorTypeNotFound                     // Resource not found errors (404 Not Found)
	ErrorTypeConflict                     // Resource conflict errors (409 Conflict)
	ErrorTypeInternal                     // Internal server errors (500 Internal Server Error)
	ErrorTypeUnavailable                  // Service unavailable errors (503 Service Unavailable)
)

// DomainError represents an error with semantic type information
type DomainError struct {
	Type    ErrorType
	Message string
	Err     error // underlying error for wrapping
}

func (e *DomainError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *DomainError) Unwrap() error {
	return e.Err
}

// GetErrorType returns the semantic type of an error
func GetErrorType(err error) ErrorType {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Type
	}
	return ErrorTypeInternal // default fallback
}

// Error constructors for different types
func NewValidationError(message string, err error) *DomainError {
	return &DomainError{Type: ErrorTypeValidation, Message: message, Err: err}
}

func NewNotFoundError(message string, err error) *DomainError {
	return &DomainError{Type: ErrorTypeNotFound, Message: message, Err: err}
}

func NewConflictError(message string, err error) *DomainError {
	return &DomainError{Type: ErrorTypeConflict, Message: message, Err: err}
}

func NewInternalError(message string, err error) *DomainError {
	return &DomainError{Type: ErrorTypeInternal, Message: message, Err: err}
}

func NewUnavailableError(message string, err error) *DomainError {
	return &DomainError{Type: ErrorTypeUnavailable, Message: message, Err: err}
}

// Domain errors
var (
	// ErrProjectNotFound is returned when a project is not found.
	ErrProjectNotFound = errors.New("project not found")
	// ErrCommitteeNotFound is returned when a committee is not found.
	ErrCommitteeNotFound = errors.New("committee not found")
	// ErrMeetingNotFound is returned when a meeting is not found.
	ErrMeetingNotFound = errors.New("meeting not found")
	// ErrPastMeetingNotFound is returned when a past meeting is not found.
	ErrPastMeetingNotFound = errors.New("past meeting not found")
	// ErrPastMeetingParticipantNotFound is returned when a past meeting participant is not found.
	ErrPastMeetingParticipantNotFound = errors.New("past meeting participant not found")
	// ErrPastMeetingParticipantAlreadyExists is returned when a past meeting participant already exists.
	ErrPastMeetingParticipantAlreadyExists = errors.New("past meeting participant already exists")
	// ErrPastMeetingRecordingNotFound is returned when a past meeting recording is not found.
	ErrPastMeetingRecordingNotFound = errors.New("past meeting recording not found")
	// ErrInternal is returned when an internal error occurs.
	ErrInternal = errors.New("internal error")
	// ErrRevisionMismatch is returned when a revision mismatch occurs.
	ErrRevisionMismatch = errors.New("revision mismatch")
	// ErrUnmarshal is returned when an unmarshal error occurs.
	ErrUnmarshal = errors.New("unmarshal error")
	// ErrServiceUnavailable is returned when a service is unavailable.
	ErrServiceUnavailable = errors.New("service unavailable")
	// ErrValidationFailed is returned when a generic validation failed.
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
