// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	natsgo "github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
)

func TestNATSUserMetadataReader_ResolveProfile(t *testing.T) {
	tests := []struct {
		name          string
		metadataReply *natsgo.Msg
		metadataErr   error
		emailsReply   *natsgo.Msg
		emailsErr     error
		wantErr       error
		wantErrStr    string
		wantProfile   *domain.UserProfile
	}{
		{
			name:          "full profile resolved from both subjects",
			metadataReply: replyMsg([]byte(`{"success":true,"data":{"name":"Alice Example","picture":"https://example.com/a.jpg"}}`)),
			emailsReply:   replyMsg([]byte(`{"success":true,"data":{"primary_email":"alice@example.com"}}`)),
			wantProfile: &domain.UserProfile{
				Username:  "alice",
				Name:      "Alice Example",
				AvatarURL: "https://example.com/a.jpg",
				Email:     "alice@example.com",
			},
		},
		{
			name:          "name composed from given+family when name is blank",
			metadataReply: replyMsg([]byte(`{"success":true,"data":{"given_name":"Alice","family_name":"Example"}}`)),
			emailsReply:   replyMsg([]byte(`{"success":true,"data":{"primary_email":"alice@example.com"}}`)),
			wantProfile: &domain.UserProfile{
				Username: "alice",
				Name:     "Alice Example",
				Email:    "alice@example.com",
			},
		},
		{
			name:          "email lookup failure degrades to empty email, not an error",
			metadataReply: replyMsg([]byte(`{"success":true,"data":{"name":"Alice Example"}}`)),
			emailsErr:     errors.New("nats: timeout"),
			wantProfile: &domain.UserProfile{
				Username: "alice",
				Name:     "Alice Example",
			},
		},
		{
			name:          "email lookup miss degrades to empty email, not an error",
			metadataReply: replyMsg([]byte(`{"success":true,"data":{"name":"Alice Example"}}`)),
			emailsReply:   replyMsg([]byte(`{"success":false,"error":"user not found"}`)),
			wantProfile: &domain.UserProfile{
				Username: "alice",
				Name:     "Alice Example",
			},
		},
		{
			name:          "metadata miss returns ErrUserNotFound",
			metadataReply: replyMsg([]byte(`{"success":false,"error":"user not found"}`)),
			wantErr:       domain.ErrUserNotFound,
		},
		{
			name:        "metadata transport error is wrapped",
			metadataErr: errors.New("nats: connection closed"),
			wantErrStr:  "user_metadata request failed",
		},
		{
			name:          "malformed metadata JSON returns parse error",
			metadataReply: replyMsg([]byte(`not json`)),
			wantErrStr:    "failed to parse user_metadata response",
		},
		{
			name:          "metadata envelope missing success field",
			metadataReply: replyMsg([]byte(`{"data":{"name":"Alice"}}`)),
			wantErrStr:    "user_metadata response missing success field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := &MockRequester{}
			mockConn.On("RequestWithContext", mock.Anything, constants.AuthUserMetadataSubject, mock.Anything).
				Return(tt.metadataReply, tt.metadataErr)
			if tt.metadataErr == nil && tt.metadataReply != nil {
				// Only stub the emails subject when metadata resolution succeeds and would
				// reach the email lookup step.
				mockConn.On("RequestWithContext", mock.Anything, constants.AuthUserEmailsSubject, mock.Anything).
					Return(tt.emailsReply, tt.emailsErr)
			}

			reader := NewUserMetadataReader(mockConn, slog.Default())
			got, err := reader.ResolveProfile(context.Background(), "alice")

			switch {
			case tt.wantErr != nil:
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, got)
			case tt.wantErrStr != "":
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrStr)
				assert.Nil(t, got)
			default:
				require.NoError(t, err)
				assert.Equal(t, tt.wantProfile, got)
			}
		})
	}
}

func TestNATSUserMetadataReader_ResolveProfile_RequiresUsername(t *testing.T) {
	mockConn := &MockRequester{}
	reader := NewUserMetadataReader(mockConn, slog.Default())

	got, err := reader.ResolveProfile(context.Background(), "  ")
	require.Error(t, err)
	assert.Nil(t, got)
	mockConn.AssertNotCalled(t, "RequestWithContext", mock.Anything, mock.Anything, mock.Anything)
}
