// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
	natsgo "github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

func TestNATSInviteSender_SendInvite(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name       string
		req        inviteapi.SendInviteRequest
		setupMocks func(*MockRequester)
		wantErr    bool
		wantResult domain.InviteResult
	}{
		{
			name: "successful request returns invite metadata",
			req: inviteapi.SendInviteRequest{
				Recipient: &inviteapi.Recipient{Email: "user@example.com", Name: "Jane Doe"},
				Resource:  &inviteapi.Resource{UID: "meeting-123", Name: "Demo Meeting", Type: "meeting"},
				Role:      "Registrant",
			},
			setupMocks: func(mockConn *MockRequester) {
				expiresAt := now.Add(30 * 24 * time.Hour)
				replyData, err := json.Marshal(inviteapi.SendInviteResponse{
					InviteData: &inviteapi.InviteData{
						UID:       "invite-abc-123",
						Email:     "user@example.com",
						ExpiresAt: expiresAt,
					},
				})
				require.NoError(t, err)
				mockConn.On("RequestWithContext", mock.Anything, inviteapi.SendInviteSubject, mock.Anything).
					Return(&natsgo.Msg{Data: replyData}, nil)
			},
			wantResult: domain.InviteResult{
				InviteUID:      "invite-abc-123",
				RecipientEmail: "user@example.com",
				ExpiresAt:      now.Add(30 * 24 * time.Hour),
			},
		},
		{
			name: "invite service returns error in response body",
			req: inviteapi.SendInviteRequest{
				Recipient: &inviteapi.Recipient{Email: "user@example.com"},
				Resource:  &inviteapi.Resource{UID: "meeting-123"},
				Role:      "Registrant",
			},
			setupMocks: func(mockConn *MockRequester) {
				replyData, err := json.Marshal(inviteapi.SendInviteResponse{Error: "recipient not found"})
				require.NoError(t, err)
				mockConn.On("RequestWithContext", mock.Anything, inviteapi.SendInviteSubject, mock.Anything).
					Return(&natsgo.Msg{Data: replyData}, nil)
			},
			wantErr: true,
		},
		{
			name: "NATS request error is returned",
			req: inviteapi.SendInviteRequest{
				Recipient: &inviteapi.Recipient{Email: "user@example.com"},
				Resource:  &inviteapi.Resource{UID: "meeting-123"},
				Role:      "Registrant",
			},
			setupMocks: func(mockConn *MockRequester) {
				mockConn.On("RequestWithContext", mock.Anything, inviteapi.SendInviteSubject, mock.Anything).
					Return(nil, errors.New("nats timeout"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := &MockRequester{}
			tt.setupMocks(mockConn)

			sender := NewInviteSender(mockConn, slog.Default())
			result, err := sender.SendInvite(context.Background(), tt.req)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.wantResult.InviteUID, result.InviteUID)
				assert.Equal(t, tt.wantResult.RecipientEmail, result.RecipientEmail)
				assert.True(t, tt.wantResult.ExpiresAt.Equal(result.ExpiresAt))
			}
			mockConn.AssertExpectations(t)
		})
	}
}
