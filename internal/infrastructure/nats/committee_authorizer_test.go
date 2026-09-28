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

func TestNATSCommitteeAuthorizer_HasWriteAccess(t *testing.T) {
	const principal = "alice"
	const committeeID = "00000000-0000-0000-0000-000000000001"
	expectedTuple := fmt.Sprintf("committee:%s#writer@user:%s", committeeID, principal)
	replyData := func(granted bool) []byte {
		return []byte(fmt.Sprintf("%s\t%v", expectedTuple, granted))
	}

	t.Run("sends request to correct subject with correct tuple payload", func(t *testing.T) {
		nc := &MockRequester{}
		nc.On("RequestWithContext", mock.Anything, fgaconstants.AccessCheckSubject, []byte(expectedTuple)).
			Return(&natsgo.Msg{Data: replyData(true)}, nil)

		authz := NewNATSCommitteeAuthorizer(nc)
		ok, err := authz.HasWriteAccess(context.Background(), principal, committeeID)
		require.NoError(t, err)
		assert.True(t, ok)
		nc.AssertExpectations(t)
	})

	t.Run("returns true when fga-sync grants access", func(t *testing.T) {
		nc := &MockRequester{}
		nc.On("RequestWithContext", mock.Anything, fgaconstants.AccessCheckSubject, mock.Anything).
			Return(&natsgo.Msg{Data: replyData(true)}, nil)

		authz := NewNATSCommitteeAuthorizer(nc)
		ok, err := authz.HasWriteAccess(context.Background(), principal, committeeID)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("returns false with nil error when fga-sync denies access", func(t *testing.T) {
		nc := &MockRequester{}
		nc.On("RequestWithContext", mock.Anything, fgaconstants.AccessCheckSubject, mock.Anything).
			Return(&natsgo.Msg{Data: replyData(false)}, nil)

		authz := NewNATSCommitteeAuthorizer(nc)
		ok, err := authz.HasWriteAccess(context.Background(), principal, committeeID)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("returns error when fga-sync returns a space-prefixed error payload", func(t *testing.T) {
		nc := &MockRequester{}
		nc.On("RequestWithContext", mock.Anything, fgaconstants.AccessCheckSubject, mock.Anything).
			Return(&natsgo.Msg{Data: []byte("error: unknown object type")}, nil)

		authz := NewNATSCommitteeAuthorizer(nc)
		ok, err := authz.HasWriteAccess(context.Background(), principal, committeeID)
		require.Error(t, err)
		assert.False(t, ok)
		// Error message must not embed the principal or the full tuple.
		assert.NotContains(t, err.Error(), principal)
		assert.NotContains(t, err.Error(), committeeID)
	})

	t.Run("returns error when reply omits the requested tuple", func(t *testing.T) {
		nc := &MockRequester{}
		nc.On("RequestWithContext", mock.Anything, fgaconstants.AccessCheckSubject, mock.Anything).
			Return(&natsgo.Msg{Data: []byte("committee:other-id#writer@user:alice\ttrue")}, nil)

		authz := NewNATSCommitteeAuthorizer(nc)
		ok, err := authz.HasWriteAccess(context.Background(), principal, committeeID)
		require.Error(t, err)
		assert.False(t, ok)
		// Error message must not embed the principal or the tuple.
		assert.NotContains(t, err.Error(), principal)
	})

	t.Run("returns error on NATS transport failure", func(t *testing.T) {
		nc := &MockRequester{}
		nc.On("RequestWithContext", mock.Anything, fgaconstants.AccessCheckSubject, mock.Anything).
			Return(nil, errors.New("nats: no servers available"))

		authz := NewNATSCommitteeAuthorizer(nc)
		ok, err := authz.HasWriteAccess(context.Background(), principal, committeeID)
		require.Error(t, err)
		assert.False(t, ok)
	})

	t.Run("returns error when result value is neither true nor false", func(t *testing.T) {
		// A malformed value such as "TRUE" or "garbage" must fail closed (503),
		// not be silently treated as a denial (403).
		body := []byte(fmt.Sprintf("%s\tgarbage", expectedTuple))
		nc := &MockRequester{}
		nc.On("RequestWithContext", mock.Anything, fgaconstants.AccessCheckSubject, mock.Anything).
			Return(&natsgo.Msg{Data: body}, nil)

		authz := NewNATSCommitteeAuthorizer(nc)
		ok, err := authz.HasWriteAccess(context.Background(), principal, committeeID)
		require.Error(t, err)
		assert.False(t, ok)
	})

	t.Run("handles multi-line response and matches correct tuple", func(t *testing.T) {
		otherLine := "committee:other-id#writer@user:alice\tfalse\n"
		body := []byte(otherLine + expectedTuple + "\ttrue\n")
		nc := &MockRequester{}
		nc.On("RequestWithContext", mock.Anything, fgaconstants.AccessCheckSubject, mock.Anything).
			Return(&natsgo.Msg{Data: body}, nil)

		authz := NewNATSCommitteeAuthorizer(nc)
		ok, err := authz.HasWriteAccess(context.Background(), principal, committeeID)
		require.NoError(t, err)
		assert.True(t, ok)
	})
}
