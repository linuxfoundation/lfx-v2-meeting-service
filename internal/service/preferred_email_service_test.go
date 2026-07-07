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

// mockUserServiceClient is a testify mock for domain.UserServiceClient.
type mockUserServiceClient struct{ mock.Mock }

func (m *mockUserServiceClient) ResolveSFIDByUsername(ctx context.Context, username string) (string, error) {
	args := m.Called(ctx, username)
	return args.String(0), args.Error(1)
}

func (m *mockUserServiceClient) ResolveEmailID(ctx context.Context, sfid, email string) (string, error) {
	args := m.Called(ctx, sfid, email)
	return args.String(0), args.Error(1)
}

func (m *mockUserServiceClient) GetMeetingEmailPreference(ctx context.Context, sfid string) (*domain.PreferredEmail, error) {
	args := m.Called(ctx, sfid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PreferredEmail), args.Error(1)
}

func (m *mockUserServiceClient) SetMeetingEmailPreference(ctx context.Context, sfid, emailID string) (*domain.PreferredEmail, error) {
	args := m.Called(ctx, sfid, emailID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PreferredEmail), args.Error(1)
}

func (m *mockUserServiceClient) ClearMeetingEmailPreference(ctx context.Context, sfid string) error {
	return m.Called(ctx, sfid).Error(0)
}

var _ domain.UserServiceClient = (*mockUserServiceClient)(nil)

func TestGetPreferredEmail(t *testing.T) {
	ctx := context.Background()

	t.Run("resolves SFID then returns preference", func(t *testing.T) {
		client := &mockUserServiceClient{}
		client.On("ResolveSFIDByUsername", ctx, "alice").Return("SFID1", nil)
		want := &domain.PreferredEmail{PreferenceID: "p1", EmailID: "e1", Email: "alice@work.com"}
		client.On("GetMeetingEmailPreference", ctx, "SFID1").Return(want, nil)

		got, err := NewPreferredEmailService(client, slog.Default()).GetPreferredEmail(ctx, "alice")
		require.NoError(t, err)
		assert.Equal(t, want, got)
		client.AssertExpectations(t)
	})

	t.Run("propagates user-not-found", func(t *testing.T) {
		client := &mockUserServiceClient{}
		client.On("ResolveSFIDByUsername", ctx, "ghost").Return("", domain.ErrUserNotFound)

		_, err := NewPreferredEmailService(client, slog.Default()).GetPreferredEmail(ctx, "ghost")
		assert.ErrorIs(t, err, domain.ErrUserNotFound)
		client.AssertNotCalled(t, "GetMeetingEmailPreference", mock.Anything, mock.Anything)
	})

	t.Run("blank user is a validation error", func(t *testing.T) {
		client := &mockUserServiceClient{}
		_, err := NewPreferredEmailService(client, slog.Default()).GetPreferredEmail(ctx, "   ")
		assert.Equal(t, domain.ErrorTypeValidation, domain.GetErrorType(err))
		client.AssertNotCalled(t, "ResolveSFIDByUsername", mock.Anything, mock.Anything)
	})
}

func TestSetPreferredEmail(t *testing.T) {
	ctx := context.Background()

	t.Run("sets preference for a concrete email_id", func(t *testing.T) {
		client := &mockUserServiceClient{}
		client.On("ResolveSFIDByUsername", ctx, "alice").Return("SFID1", nil)
		want := &domain.PreferredEmail{PreferenceID: "p1", EmailID: "e1", Email: "alice@work.com"}
		client.On("SetMeetingEmailPreference", ctx, "SFID1", "e1").Return(want, nil)

		got, err := NewPreferredEmailService(client, slog.Default()).SetPreferredEmail(ctx, "alice", "", "e1")
		require.NoError(t, err)
		assert.Equal(t, want, got)
		client.AssertExpectations(t)
	})

	t.Run("resolves an address to its email_id (email wins over email_id)", func(t *testing.T) {
		client := &mockUserServiceClient{}
		client.On("ResolveSFIDByUsername", ctx, "alice").Return("SFID1", nil)
		client.On("ResolveEmailID", ctx, "SFID1", "alice@work.com").Return("e1", nil)
		want := &domain.PreferredEmail{PreferenceID: "p1", EmailID: "e1", Email: "alice@work.com"}
		client.On("SetMeetingEmailPreference", ctx, "SFID1", "e1").Return(want, nil)

		// email_id is also provided but must be ignored in favor of the resolved address.
		got, err := NewPreferredEmailService(client, slog.Default()).SetPreferredEmail(ctx, "alice", "alice@work.com", "ignored")
		require.NoError(t, err)
		assert.Equal(t, want, got)
		client.AssertExpectations(t)
	})

	t.Run("propagates retryable error for a not-yet-synced address", func(t *testing.T) {
		client := &mockUserServiceClient{}
		client.On("ResolveSFIDByUsername", ctx, "alice").Return("SFID1", nil)
		client.On("ResolveEmailID", ctx, "SFID1", "new@work.com").
			Return("", domain.NewUnavailableError("not yet available"))

		_, err := NewPreferredEmailService(client, slog.Default()).SetPreferredEmail(ctx, "alice", "new@work.com", "")
		assert.Equal(t, domain.ErrorTypeUnavailable, domain.GetErrorType(err))
		client.AssertNotCalled(t, "SetMeetingEmailPreference", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("empty selection clears the override", func(t *testing.T) {
		client := &mockUserServiceClient{}
		client.On("ResolveSFIDByUsername", ctx, "alice").Return("SFID1", nil)
		client.On("ClearMeetingEmailPreference", ctx, "SFID1").Return(nil)

		got, err := NewPreferredEmailService(client, slog.Default()).SetPreferredEmail(ctx, "alice", "", "")
		require.NoError(t, err)
		assert.Nil(t, got)
		client.AssertNotCalled(t, "SetMeetingEmailPreference", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("primary sentinel clears the override", func(t *testing.T) {
		client := &mockUserServiceClient{}
		client.On("ResolveSFIDByUsername", ctx, "alice").Return("SFID1", nil)
		client.On("ClearMeetingEmailPreference", ctx, "SFID1").Return(nil)

		got, err := NewPreferredEmailService(client, slog.Default()).SetPreferredEmail(ctx, "alice", "", "PRIMARY")
		require.NoError(t, err)
		assert.Nil(t, got)
		client.AssertExpectations(t)
	})

	t.Run("email=primary clears even when email_id is set (email wins)", func(t *testing.T) {
		client := &mockUserServiceClient{}
		client.On("ResolveSFIDByUsername", ctx, "alice").Return("SFID1", nil)
		client.On("ClearMeetingEmailPreference", ctx, "SFID1").Return(nil)

		// email takes precedence: "primary" must clear, ignoring the provided email_id.
		got, err := NewPreferredEmailService(client, slog.Default()).SetPreferredEmail(ctx, "alice", "primary", "e1")
		require.NoError(t, err)
		assert.Nil(t, got)
		client.AssertNotCalled(t, "ResolveEmailID", mock.Anything, mock.Anything, mock.Anything)
		client.AssertNotCalled(t, "SetMeetingEmailPreference", mock.Anything, mock.Anything, mock.Anything)
	})
}
