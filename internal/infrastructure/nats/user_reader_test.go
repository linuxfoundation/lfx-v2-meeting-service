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

func replyMsg(data []byte) *natsgo.Msg { return &natsgo.Msg{Data: data} }

func TestNATSUserReader_SubByEmail(t *testing.T) {
	tests := []struct {
		name       string
		reply      *natsgo.Msg
		replyErr   error
		wantUser   string
		wantErr    error
		wantErrStr string
	}{
		{
			name:     "plain-text subject returned on success",
			reply:    replyMsg([]byte("auth0|alice")),
			wantUser: "auth0|alice",
		},
		{
			name:     "trailing newline trimmed from subject",
			reply:    replyMsg([]byte("auth0|alice\n")),
			wantUser: "auth0|alice",
		},
		{
			name:    "empty body returns ErrUserNotFound",
			reply:   replyMsg([]byte("")),
			wantErr: domain.ErrUserNotFound,
		},
		{
			name:    "JSON error envelope returns ErrUserNotFound",
			reply:   replyMsg([]byte(`{"success":false,"error":"user not found"}`)),
			wantErr: domain.ErrUserNotFound,
		},
		{
			name:       "JSON envelope missing success field returns descriptive error",
			reply:      replyMsg([]byte(`{"error":"something unexpected"}`)),
			wantErrStr: "email_to_sub response missing success field",
		},
		{
			name:       "JSON success envelope returns error instead of leaking JSON as subject",
			reply:      replyMsg([]byte(`{"success":true,"username":"alice"}`)),
			wantErrStr: "unexpected email_to_sub success envelope",
		},
		{
			name:       "malformed JSON object returns parse error",
			reply:      replyMsg([]byte(`{"success":"true"}`)),
			wantErrStr: "failed to parse email_to_sub response",
		},
		{
			name:       "transport error is wrapped and returned",
			reply:      nil,
			replyErr:   errors.New("nats: connection closed"),
			wantErrStr: "email_to_sub request failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := &MockRequester{}
			mockConn.On("RequestWithContext", mock.Anything, constants.AuthEmailToSubSubject, mock.Anything).
				Return(tt.reply, tt.replyErr)

			reader := NewUserReader(mockConn, slog.Default())
			got, err := reader.SubByEmail(context.Background(), "test@example.com")

			switch {
			case tt.wantErr != nil:
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, got)
			case tt.wantErrStr != "":
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrStr)
				assert.Empty(t, got)
			default:
				require.NoError(t, err)
				assert.Equal(t, tt.wantUser, got)
			}
			mockConn.AssertExpectations(t)
		})
	}
}
