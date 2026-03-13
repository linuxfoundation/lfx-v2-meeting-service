// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

// User represents a user in the ITX system
type User struct {
	ID             string `json:"id"`
	Username       string `json:"username"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	ProfilePicture string `json:"profile_picture,omitempty"`
}

// ErrorResponse represents an error response from ITX
type ErrorResponse struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}
