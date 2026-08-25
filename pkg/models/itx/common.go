// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import "log/slog"

// User represents a user in the ITX system
type User struct {
	ID             string `json:"id"`
	Username       string `json:"username"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	ProfilePicture string `json:"profile_picture,omitempty"`
}

// LogValue implements slog.LogValuer so that User values logged directly as a
// slog attribute (e.g. `slog.Any("user", u)`) don't leak the requester's name
// and email into logs — only the username is emitted. Does NOT redact User
// values nested inside a JSON-marshaled request body; the proxy client applies
// a separate body-level redaction for that case (see requestJSONForLog in
// internal/infrastructure/proxy).
func (u User) LogValue() slog.Value {
	return slog.GroupValue(slog.String("username", u.Username))
}

// ErrorResponse represents an error response from ITX
type ErrorResponse struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}
