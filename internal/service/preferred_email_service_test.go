// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
)

const testToken = "test-user-token"

// mockUserServiceClient is a testify mock for domain.UserServiceClient.
type mockUserServiceClient struct{ mock.Mock }

func (m *mockUserServiceClient) GetSelf(ctx context.Context, token string) (*domain.Self, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Self), args.Error(1)
}

func (m *mockUserServiceClient) GetMeetingEmailPreference(ctx context.Context, token, sfid string) (*domain.PreferredEmail, error) {
	args := m.Called(ctx, token, sfid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PreferredEmail), args.Error(1)
}

func (m *mockUserServiceClient) SetMeetingEmailPreference(ctx context.Context, token, sfid, emailID string) (*domain.PreferredEmail, error) {
	args := m.Called(ctx, token, sfid, emailID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PreferredEmail), args.Error(1)
}

func (m *mockUserServiceClient) ClearMeetingEmailPreference(ctx context.Context, token, sfid string) error {
	return m.Called(ctx, token, sfid).Error(0)
}

var _ domain.UserServiceClient = (*mockUserServiceClient)(nil)

func selfWithEmail() *domain.Self {
	return &domain.Self{
		SFID: "SFID1",
		Emails: []domain.SelfEmail{
			{ID: "e1", Address: "alice@work.com", Active: true, Verified: true},
			{ID: "e-unverified", Address: "new@work.com", Active: true, Verified: false},
		},
	}
}

func TestGetPreferredEmail(t *testing.T) {
	ctx := context.Background()

	t.Run("resolves self from token then returns preference", func(t *testing.T) {
		client := &mockUserServiceClient{}
		client.On("GetSelf", ctx, testToken).Return(selfWithEmail(), nil)
		want := &domain.PreferredEmail{PreferenceID: "p1", EmailID: "e1", Email: "alice@work.com"}
		client.On("GetMeetingEmailPreference", ctx, testToken, "SFID1").Return(want, nil)

		got, err := NewPreferredEmailService(client, slog.Default()).GetPreferredEmail(ctx, testToken)
		require.NoError(t, err)
		assert.Equal(t, want, got)
		client.AssertExpectations(t)
	})

	t.Run("blank token is a validation error", func(t *testing.T) {
		client := &mockUserServiceClient{}
		_, err := NewPreferredEmailService(client, slog.Default()).GetPreferredEmail(ctx, "   ")
		assert.Equal(t, domain.ErrorTypeValidation, domain.GetErrorType(err))
		client.AssertNotCalled(t, "GetSelf", mock.Anything, mock.Anything)
	})

	t.Run("propagates GetSelf error (e.g. rejected token)", func(t *testing.T) {
		client := &mockUserServiceClient{}
		client.On("GetSelf", ctx, testToken).Return(nil, domain.NewValidationError("user token rejected"))

		_, err := NewPreferredEmailService(client, slog.Default()).GetPreferredEmail(ctx, testToken)
		assert.Equal(t, domain.ErrorTypeValidation, domain.GetErrorType(err))
		client.AssertNotCalled(t, "GetMeetingEmailPreference", mock.Anything, mock.Anything, mock.Anything)
	})
}

func TestSetPreferredEmail(t *testing.T) {
	ctx := context.Background()

	t.Run("sets preference for a concrete email_id", func(t *testing.T) {
		client := &mockUserServiceClient{}
		client.On("GetSelf", ctx, testToken).Return(selfWithEmail(), nil)
		want := &domain.PreferredEmail{PreferenceID: "p1", EmailID: "e1", Email: "alice@work.com"}
		client.On("SetMeetingEmailPreference", ctx, testToken, "SFID1", "e1").Return(want, nil)

		got, err := NewPreferredEmailService(client, slog.Default()).SetPreferredEmail(ctx, testToken, "", "e1")
		require.NoError(t, err)
		assert.Equal(t, want, got)
		client.AssertExpectations(t)
	})

	t.Run("resolves a verified address to its email_id (email wins over email_id)", func(t *testing.T) {
		client := &mockUserServiceClient{}
		client.On("GetSelf", ctx, testToken).Return(selfWithEmail(), nil)
		want := &domain.PreferredEmail{PreferenceID: "p1", EmailID: "e1", Email: "alice@work.com"}
		client.On("SetMeetingEmailPreference", ctx, testToken, "SFID1", "e1").Return(want, nil)

		got, err := NewPreferredEmailService(client, slog.Default()).SetPreferredEmail(ctx, testToken, "alice@work.com", "ignored")
		require.NoError(t, err)
		assert.Equal(t, want, got)
		client.AssertExpectations(t)
	})

	t.Run("unverified address is a validation error", func(t *testing.T) {
		client := &mockUserServiceClient{}
		client.On("GetSelf", ctx, testToken).Return(selfWithEmail(), nil)

		_, err := NewPreferredEmailService(client, slog.Default()).SetPreferredEmail(ctx, testToken, "new@work.com", "")
		assert.Equal(t, domain.ErrorTypeValidation, domain.GetErrorType(err))
		client.AssertNotCalled(t, "SetMeetingEmailPreference", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("unknown address returns retryable unavailable error", func(t *testing.T) {
		client := &mockUserServiceClient{}
		client.On("GetSelf", ctx, testToken).Return(selfWithEmail(), nil)

		_, err := NewPreferredEmailService(client, slog.Default()).SetPreferredEmail(ctx, testToken, "ghost@work.com", "")
		assert.Equal(t, domain.ErrorTypeUnavailable, domain.GetErrorType(err))
	})

	t.Run("empty selection clears the override", func(t *testing.T) {
		client := &mockUserServiceClient{}
		client.On("GetSelf", ctx, testToken).Return(selfWithEmail(), nil)
		client.On("ClearMeetingEmailPreference", ctx, testToken, "SFID1").Return(nil)

		got, err := NewPreferredEmailService(client, slog.Default()).SetPreferredEmail(ctx, testToken, "", "")
		require.NoError(t, err)
		assert.Nil(t, got)
		client.AssertNotCalled(t, "SetMeetingEmailPreference", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("email=primary clears even when email_id is set (email wins)", func(t *testing.T) {
		client := &mockUserServiceClient{}
		client.On("GetSelf", ctx, testToken).Return(selfWithEmail(), nil)
		client.On("ClearMeetingEmailPreference", ctx, testToken, "SFID1").Return(nil)

		got, err := NewPreferredEmailService(client, slog.Default()).SetPreferredEmail(ctx, testToken, "primary", "e1")
		require.NoError(t, err)
		assert.Nil(t, got)
		client.AssertNotCalled(t, "SetMeetingEmailPreference", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}
