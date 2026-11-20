// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventstate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Import copies only meeting-owned state. All legacy writers must be stopped
// externally; a stable sequence is checked, never manufactured by this tool.
// A verified target is immutable to imports: it may already contain newer PG
// state. Partial imports can be resumed only when existing rows agree.
func (s *Store) Import(ctx context.Context, js jetstream.JetStream, apply bool) (int, error) {
	stream, err := js.Stream(ctx, "KV_v1-mappings")
	if err != nil {
		return 0, err
	}
	return s.importSource(ctx, natsStateSource{stream: stream, requester: js.Conn(), apiPrefix: js.Options().APIPrefix}, apply)
}

// stateSource exposes next-by-subject semantics explicitly, so synthetic tests
// exercise the same filtering and sparse-sequence contract as the NATS adapter.
type stateSource interface {
	Info(context.Context) (*jetstream.StreamInfo, error)
	Next(context.Context, uint64, string) (*jetstream.RawStreamMsg, error)
}
type messageRequester interface {
	RequestWithContext(context.Context, string, []byte) (*nats.Msg, error)
}

type natsStateSource struct {
	stream    jetstream.Stream
	requester messageRequester
	apiPrefix string
}

func (s natsStateSource) Info(ctx context.Context) (*jetstream.StreamInfo, error) {
	return s.stream.Info(ctx)
}
func (s natsStateSource) Next(ctx context.Context, seq uint64, subject string) (*jetstream.RawStreamMsg, error) {
	// GetMsg may switch to follower-served direct reads when AllowDirect is
	// enabled. Cutover needs the leader-served API regardless of that setting.
	request, err := json.Marshal(struct {
		Sequence    uint64 `json:"seq"`
		NextSubject string `json:"next_by_subj"`
	}{seq, subject})
	if err != nil {
		return nil, err
	}
	apiSubject := strings.TrimSuffix(s.apiPrefix, ".") + ".STREAM.MSG.GET." + s.stream.CachedInfo().Config.Name
	response, err := s.requester.RequestWithContext(ctx, apiSubject, request)
	if err != nil {
		return nil, err
	}
	var result struct {
		Error   *jetstream.APIError `json:"error"`
		Message *struct {
			Subject  string    `json:"subject"`
			Sequence uint64    `json:"seq"`
			Data     []byte    `json:"data"`
			Headers  []byte    `json:"hdrs"`
			Time     time.Time `json:"time"`
		} `json:"message"`
	}
	if err := json.Unmarshal(response.Data, &result); err != nil {
		return nil, jetstream.ErrInvalidJetStreamResponse
	}
	if result.Error != nil {
		if result.Error.ErrorCode == jetstream.JSErrCodeMessageNotFound {
			return nil, jetstream.ErrMsgNotFound
		}
		return nil, result.Error
	}
	if result.Message == nil {
		return nil, jetstream.ErrInvalidJetStreamResponse
	}
	var headers nats.Header
	if len(result.Message.Headers) > 0 {
		headers, err = nats.DecodeHeadersMsg(result.Message.Headers)
		if err != nil {
			return nil, jetstream.ErrInvalidJetStreamResponse
		}
	}
	return &jetstream.RawStreamMsg{Subject: result.Message.Subject, Sequence: result.Message.Sequence, Data: result.Message.Data, Header: headers, Time: result.Message.Time}, nil
}

func (s *Store) importSource(ctx context.Context, source stateSource, apply bool) (int, error) {
	if apply {
		if err := s.ApplySchema(ctx); err != nil {
			return 0, err
		}
		conn, err := s.pool.Acquire(ctx)
		if err != nil {
			return 0, err
		}
		defer conn.Release()
		lockCtx, cancel := context.WithTimeout(ctx, operationTimeout)
		var locked bool
		err = conn.QueryRow(lockCtx, "SELECT pg_try_advisory_lock(1948002)").Scan(&locked)
		cancel()
		if err != nil {
			return 0, err
		}
		if !locked {
			return 0, errors.New("another meeting state import is active")
		}
		defer func() {
			cleanup, cancel := context.WithTimeout(context.Background(), operationTimeout)
			defer cancel()
			_, _ = conn.Exec(cleanup, "SELECT pg_advisory_unlock(1948002)")
		}()
		var ready bool
		checkCtx, cancel := context.WithTimeout(ctx, operationTimeout)
		err = conn.QueryRow(checkCtx, "SELECT EXISTS (SELECT 1 FROM meeting_event_state_cutover)").Scan(&ready)
		cancel()
		if err != nil {
			return 0, err
		}
		if ready {
			return 0, errors.New("verified cutover already exists; refusing to overwrite authoritative Postgres state")
		}
	}
	infoCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	before, err := source.Info(infoCtx)
	cancel()
	if err != nil {
		return 0, err
	}
	latest := map[string][]byte{}
	prefixes := []string{"v1-mappings.meeting-mappings.", "v1-mappings.past-meeting-mappings.", "v1-mappings.past-meeting-summaries.", "v1_participant_by_meeting_user.", "v1_meeting_registrant_lfid_invite_sent.", "past_meeting_participant.invitee_id.", "past_meeting_participant.attendee_id.", "past_meeting_participant.participant_to_invitee_id.", "past_meeting_participant.participant_to_attendee_id."}
	for kind := range resourceKinds {
		prefixes = append(prefixes, kind+".")
	}
	for _, prefix := range prefixes {
		for seq := uint64(1); seq <= before.State.LastSeq; {
			opCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			msg, getErr := source.Next(opCtx, seq, "$KV.v1-mappings."+prefix+">")
			cancel()
			if errors.Is(getErr, jetstream.ErrMsgNotFound) {
				break
			}
			if getErr != nil {
				return 0, getErr
			}
			if msg.Sequence > before.State.LastSeq {
				break
			}
			seq = msg.Sequence + 1
			key := msg.Subject[len("$KV.v1-mappings."):]
			if msg.Header.Get("KV-Operation") == "DEL" || msg.Header.Get("KV-Operation") == "PURGE" {
				delete(latest, key)
				continue
			}
			latest[key] = bytes.Clone(msg.Data)
		}
	}
	counts := map[string]int{}
	if len(latest) == 0 {
		return 0, errors.New("source contains no meeting state; refusing an unproven empty cutover")
	}
	for key, value := range latest {
		b, err := bind(key)
		if err != nil {
			return 0, errors.New("invalid owned source key; manual reconciliation required")
		}
		if _, err := b.decode(value); err != nil {
			return 0, errors.New("invalid owned source state; manual reconciliation required")
		}
		counts[b.table]++
	}
	if apply {
		for key, value := range latest {
			if _, err := s.Create(ctx, key, value); err != nil {
				if !errors.Is(err, jetstream.ErrKeyExists) {
					return 0, err
				}
			}
			entry, err := s.Get(ctx, key)
			if err != nil {
				return 0, err
			}
			b, _ := bind(key)
			want, _ := b.decode(value)
			got, err := b.decode(entry.Value())
			if err != nil || !reflect.DeepEqual(want, got) {
				return 0, errors.New("target state differs from source; refusing to overwrite")
			}
		}
		for _, table := range []string{"meeting_resource_sync_state", "meeting_committee_associations", "meeting_summary_associations", "meeting_participant_lookup", "meeting_invitation_guards", "meeting_participant_aliases"} {
			opCtx, cancel := context.WithTimeout(ctx, operationTimeout)
			var count int
			err := s.pool.QueryRow(opCtx, "SELECT count(*) FROM "+table).Scan(&count)
			cancel()
			if err != nil {
				return 0, err
			}
			if count != counts[table] {
				return 0, fmt.Errorf("target contains unmatched rows in %s; manual reconciliation required", table)
			}
		}
	}
	infoCtx, cancel = context.WithTimeout(ctx, 30*time.Second)
	after, err := source.Info(infoCtx)
	cancel()
	if err != nil {
		return 0, err
	}
	if before.State.FirstSeq != after.State.FirstSeq || before.State.LastSeq != after.State.LastSeq || before.State.Msgs != after.State.Msgs {
		return 0, errors.New("legacy source changed during import; cutover is not verified")
	}
	if apply {
		opCtx, cancel := context.WithTimeout(ctx, operationTimeout)
		defer cancel()
		_, err = s.pool.Exec(opCtx, "INSERT INTO meeting_event_state_cutover(singleton,source_last_sequence) VALUES(true,$1)", after.State.LastSeq)
		if err != nil {
			return 0, err
		}
	}
	return len(latest), nil
}
