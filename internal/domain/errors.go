// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import "errors"

// ErrorType represents the semantic category of an error
type ErrorType int

const (
	ErrorTypeValidation  ErrorType = iota // Input validation errors (400 Bad Request)
	ErrorTypeForbidden                    // Business-logic forbidden errors (403 Forbidden)
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

// String returns the stable wire-format string for the error type. Callers such as the
// preferred_email NATS responder forward this in the reply envelope so consumers can
// classify failures without matching on the free-text error message.
func (t ErrorType) String() string {
	switch t {
	case ErrorTypeValidation:
		return "validation"
	case ErrorTypeForbidden:
		return "forbidden"
	case ErrorTypeNotFound:
		return "not_found"
	case ErrorTypeConflict:
		return "conflict"
	case ErrorTypeUnavailable:
		return "unavailable"
	case ErrorTypeInternal:
		return "internal"
	default:
		return "internal"
	}
}

// Error constructors for different types
func NewValidationError(message string, err ...error) *DomainError {
	return &DomainError{Type: ErrorTypeValidation, Message: message, Err: errors.Join(err...)}
}

func NewNotFoundError(message string, err ...error) *DomainError {
	return &DomainError{Type: ErrorTypeNotFound, Message: message, Err: errors.Join(err...)}
}

func NewConflictError(message string, err ...error) *DomainError {
	return &DomainError{Type: ErrorTypeConflict, Message: message, Err: errors.Join(err...)}
}

func NewInternalError(message string, err ...error) *DomainError {
	return &DomainError{Type: ErrorTypeInternal, Message: message, Err: errors.Join(err...)}
}

func NewUnavailableError(message string, err ...error) *DomainError {
	return &DomainError{Type: ErrorTypeUnavailable, Message: message, Err: errors.Join(err...)}
}

func NewForbiddenError(message string, err ...error) *DomainError {
	return &DomainError{Type: ErrorTypeForbidden, Message: message, Err: errors.Join(err...)}
}

// ErrUserNotFound is returned by UserReader when no registered user matches the lookup.
var ErrUserNotFound = errors.New("user not found")

// ErrInviteNotFound is returned by InviteLookup when the invite service has no record
// for the requested invite UID.
var ErrInviteNotFound = errors.New("invite not found")

// ErrEmailNotSynced marks a preferred-email selection that matched a known address on the
// user's profile but has not yet synced from Auth0 to SFDC. It is wrapped inside a
// NewUnavailableError so callers can detect this specific retryable case with errors.Is
// instead of matching on the error message.
var ErrEmailNotSynced = errors.New("email not yet synced")
