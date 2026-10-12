// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"errors"
	"fmt"
	"testing"

	fgaconstants "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/constants"
	natsgo "github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNATSPastMeetingAuthorizer_IsOrganizer(t *testing.T) {
	const principal = "alice"
	const pastMeetingID = "93699735000-1700000000"
	expectedTuple := fmt.Sprintf("v1_past_meeting:%s#organizer@user:%s", pastMeetingID, principal)

	t.Run("sends the organizer tuple and returns the granted result", func(t *testing.T) {
		nc := &MockRequester{}
		nc.On("RequestWithContext", mock.Anything, fgaconstants.AccessCheckSubject, []byte(expectedTuple)).
			Return(&natsgo.Msg{Data: []byte(expectedTuple + "\ttrue")}, nil)

		ok, err := NewNATSPastMeetingAuthorizer(nc).IsOrganizer(context.Background(), principal, pastMeetingID)
		require.NoError(t, err)
		assert.True(t, ok)
		nc.AssertExpectations(t)
	})

	t.Run("returns false with nil error when fga-sync denies access", func(t *testing.T) {
		nc := &MockRequester{}
		nc.On("RequestWithContext", mock.Anything, fgaconstants.AccessCheckSubject, mock.Anything).
			Return(&natsgo.Msg{Data: []byte(expectedTuple + "\tfalse")}, nil)

		ok, err := NewNATSPastMeetingAuthorizer(nc).IsOrganizer(context.Background(), principal, pastMeetingID)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("returns error on NATS transport failure", func(t *testing.T) {
		nc := &MockRequester{}
		nc.On("RequestWithContext", mock.Anything, fgaconstants.AccessCheckSubject, mock.Anything).
			Return(nil, errors.New("nats: no servers available"))

		ok, err := NewNATSPastMeetingAuthorizer(nc).IsOrganizer(context.Background(), principal, pastMeetingID)
		require.Error(t, err)
		assert.False(t, ok)
	})

	t.Run("denies without a request when the past meeting ID carries tuple delimiters", func(t *testing.T) {
		for _, id := range []string{"", "pm#organizer@user:alice\nv1_past_meeting:pm", "pm@x", "pm 1"} {
			nc := &MockRequester{}
			ok, err := NewNATSPastMeetingAuthorizer(nc).IsOrganizer(context.Background(), principal, id)
			require.NoError(t, err)
			assert.False(t, ok)
			nc.AssertNotCalled(t, "RequestWithContext", mock.Anything, mock.Anything, mock.Anything)
		}
	})

	t.Run("denies without a request when the principal is empty", func(t *testing.T) {
		nc := &MockRequester{}
		ok, err := NewNATSPastMeetingAuthorizer(nc).IsOrganizer(context.Background(), "", pastMeetingID)
		require.NoError(t, err)
		assert.False(t, ok)
		nc.AssertNotCalled(t, "RequestWithContext", mock.Anything, mock.Anything, mock.Anything)
	})
}
