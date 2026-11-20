// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"testing"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
)

type repairObjects struct{ jetstream.KeyValue }

func (repairObjects) Get(context.Context, string) (jetstream.KeyValueEntry, error) {
	return repairEntry{value: []byte(`{"lf_sso":"testuser","meeting_and_occurrence_id":"past"}`)}, nil
}

type repairEntry struct {
	jetstream.KeyValueEntry
	value []byte
}

func (e repairEntry) Value() []byte { return e.value }

type conflictState struct {
	domain.EventState
	err      error
	revision uint64
}

func (s *conflictState) Update(_ context.Context, _ string, _ []byte, revision uint64) (uint64, error) {
	s.revision = revision
	return 0, s.err
}

func TestProcessEntry(t *testing.T) {
	for name, err := range map[string]error{"legacy conflict": jetstream.ErrKeyExists, "PG CAS conflict": jetstream.ErrKeyRevisionMismatch, "wrapped PG conflict": errors.Join(errors.New("wrapped"), jetstream.ErrKeyRevisionMismatch)} {
		t.Run(name, func(t *testing.T) {
			state := &conflictState{err: err}
			cfg := mappingConfig{mappingPrefix: "v1_past_meeting_invitees", objectPrefix: "itx-zoom-past-meetings-invitees.", usernameField: "lf_sso", meetingIDField: "meeting_and_occurrence_id"}
			result, _ := processEntry(context.Background(), repairObjects{}, state, cfg, legacyEntry{mappingKey: "v1_past_meeting_invitees.test", uid: "test", currentValue: "test", revision: 42}, true)
			require.Equal(t, resultUpdated, result)
			require.Equal(t, uint64(42), state.revision)
		})
	}
}
