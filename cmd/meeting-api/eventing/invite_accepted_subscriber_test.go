// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"log/slog"
	"testing"
	"time"
)

func TestInviteAcceptedSubscriber_StopWithNoInFlightMessages(t *testing.T) {
	sub := NewInviteAcceptedSubscriber(nil, nil, slog.Default())

	done := make(chan struct{})
	go func() {
		sub.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() blocked — possible WaitGroup misuse")
	}
}
