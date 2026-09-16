// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"testing"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

func TestMapHTTPError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		body       []byte
		wantType   domain.ErrorType
	}{
		{
			name:       "bad request",
			statusCode: 400,
			body:       []byte(`{"message":"invalid input"}`),
			wantType:   domain.ErrorTypeValidation,
		},
		{
			name:       "not found",
			statusCode: 404,
			body:       []byte(`{"error":"meeting not found"}`),
			wantType:   domain.ErrorTypeNotFound,
		},
		{
			name:       "conflict",
			statusCode: 409,
			body:       []byte(`{"message":"already exists"}`),
			wantType:   domain.ErrorTypeConflict,
		},
		{
			name:       "service unavailable",
			statusCode: 503,
			body:       []byte(`{"message":"downstream unavailable"}`),
			wantType:   domain.ErrorTypeUnavailable,
		},
		{
			name:       "unauthorized",
			statusCode: 401,
			body:       []byte(`{"message":"invalid token"}`),
			wantType:   domain.ErrorTypeValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := mapHTTPError(tt.statusCode, tt.body)
			if domain.GetErrorType(err) != tt.wantType {
				t.Fatalf("GetErrorType() = %v, want %v (err=%v)", domain.GetErrorType(err), tt.wantType, err)
			}
		})
	}
}
