// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventstate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
)

func TestBinding(t *testing.T) {
	for _, key := range []string{"project.sfid.test", "committee.uid.test", "v1_participant_by_meeting_user.other.meeting.test", "v1_meetings.bad>key"} {
		t.Run(key, func(t *testing.T) { _, err := bind(key); require.Error(t, err) })
	}
	t.Run("username periods remain intact", func(t *testing.T) {
		b, err := bind("v1_participant_by_meeting_user.invitee.meeting.test.user")
		require.NoError(t, err)
		require.Equal(t, []any{"invitee", "meeting", "test.user"}, b.args)
	})
	t.Run("legacy slash keys remain addressable", func(t *testing.T) {
		b, err := bind("v1_participant_by_meeting_user.invitee.past.test/user")
		require.NoError(t, err)
		require.Equal(t, []any{"invitee", "past", "test/user"}, b.args)
	})
	t.Run("markers have distinct states", func(t *testing.T) {
		b, err := bind("v1_meetings.test")
		require.NoError(t, err)
		for value, state := range map[string]string{"": "empty", "1": "processed", "!del": "deleted"} {
			columns, err := b.decode([]byte(value))
			require.NoError(t, err)
			require.Equal(t, state, columns[0])
		}
	})
	t.Run("principal is decomposed without granting empty legacy authority", func(t *testing.T) {
		b, err := bind("v1_meeting_registrants.test")
		require.NoError(t, err)
		columns, err := b.decode([]byte(`{"uid":"test","username":"test|user","meeting_id":"meeting"}`))
		require.NoError(t, err)
		require.Equal(t, []any{"principal", "test", "test|user", "meeting", false}, columns)
	})
	t.Run("unknown association fields require reconciliation", func(t *testing.T) {
		b, err := bind("v1-mappings.meeting-mappings.test")
		require.NoError(t, err)
		_, err = b.decode([]byte(`{"assoc":{"committee_id":"committee","unknown":"value"}}`))
		require.Error(t, err)
	})
	t.Run("missing database never falls back to KV", func(t *testing.T) { _, err := Open(context.Background(), ""); require.Error(t, err) })
}

// An explicitly supplied disposable database is required. Each test creates and
// removes only its own unique schema, never existing application tables.
func testStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("MEETING_STATE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set MEETING_STATE_TEST_DATABASE_URL to a disposable local Postgres database")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	schemaName := fmt.Sprintf("meeting_state_test_%d", time.Now().UnixNano())
	_, err = admin.Exec(ctx, "CREATE SCHEMA "+schemaName)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	query := u.Query()
	query.Set("search_path", schemaName)
	u.RawQuery = query.Encode()
	s, err := Open(ctx, u.String())
	require.NoError(t, err)
	t.Cleanup(func() {
		s.Close()
		_, err := admin.Exec(ctx, "DROP SCHEMA "+schemaName+" CASCADE")
		if err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	require.NoError(t, s.ApplySchema(ctx))
	require.NoError(t, s.ApplySchema(ctx))
	return s
}

func TestPostgresState(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	t.Run("requires verified cutover", func(t *testing.T) { require.Error(t, s.Ready(ctx)) })
	cases := map[string]string{
		"v1_meetings.meeting":                                             "1",
		"v1_past_meetings.past":                                           "!del",
		"v1_invite_responses.response":                                    "",
		"v1_meeting_registrants.reg":                                      `{"uid":"reg","username":"test.user","meeting_id":"meeting","principal_authoritative":true}`,
		"v1-mappings.meeting-mappings.meeting":                            `{"association":{"committee_id":"committee","committee_filters":["Voting Rep"]}}`,
		"v1-mappings.past-meeting-summaries.past":                         `["summary-one","summary-two"]`,
		"v1_participant_by_meeting_user.attendee.past.test.user":          "attendee-one",
		"v1_meeting_registrant_lfid_invite_sent.reg":                      "pending",
		"past_meeting_participant.invitee_id.invitee":                     "participant",
		"past_meeting_participant.participant_to_attendee_id.participant": "attendee",
		"v1_participant_by_meeting_user.invitee.past.test/user":           "invitee",
	}
	for key, value := range cases {
		t.Run(key, func(t *testing.T) {
			rev, err := s.Create(ctx, key, []byte(value))
			require.NoError(t, err)
			_, err = s.Create(ctx, key, []byte(value))
			require.ErrorIs(t, err, jetstream.ErrKeyExists)
			e, err := s.Get(ctx, key)
			require.NoError(t, err)
			require.Equal(t, rev, e.Revision())
			b, _ := bind(key)
			want, err := b.decode([]byte(value))
			require.NoError(t, err)
			got, err := b.decode(e.Value())
			require.NoError(t, err)
			require.Equal(t, want, got)
			next, err := s.Update(ctx, key, []byte(value), rev)
			require.NoError(t, err)
			require.Greater(t, next, rev)
			_, err = s.Update(ctx, key, []byte(value), rev)
			require.ErrorIs(t, err, jetstream.ErrKeyRevisionMismatch)
			require.NoError(t, s.Delete(ctx, key))
			_, err = s.Get(ctx, key)
			require.ErrorIs(t, err, jetstream.ErrKeyNotFound)
			recreated, err := s.Create(ctx, key, []byte(value))
			require.NoError(t, err)
			require.Greater(t, recreated, next)
			_, err = s.Update(ctx, key, []byte(value), next)
			require.ErrorIs(t, err, jetstream.ErrKeyRevisionMismatch)
		})
	}
	t.Run("one invitation dispatcher wins", func(t *testing.T) {
		var wg sync.WaitGroup
		results := make(chan error, 20)
		for range 20 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := s.Create(ctx, "v1_meeting_registrant_lfid_invite_sent.concurrent", []byte("pending"))
				results <- err
			}()
		}
		wg.Wait()
		close(results)
		winners := 0
		for err := range results {
			if err == nil {
				winners++
			} else {
				require.ErrorIs(t, err, jetstream.ErrKeyExists)
			}
		}
		require.Equal(t, 1, winners)
	})
	t.Run("JSON null removal works for both parent kinds without reviving deleted rows", func(t *testing.T) {
		for kind, prefix := range map[string]string{"meeting": "v1-mappings.meeting-mappings.", "past_meeting": "v1-mappings.past-meeting-mappings."} {
			id := "null-" + kind
			key := prefix + id
			_, err := s.Create(ctx, key, []byte("null"))
			require.NoError(t, err)
			require.NoError(t, s.RemoveCommitteeAssociation(ctx, kind, id, "absent"))
			e, err := s.Get(ctx, key)
			require.NoError(t, err)
			require.JSONEq(t, "{}", string(e.Value()))
			_, err = s.Put(ctx, key, []byte("!del"))
			require.NoError(t, err)
			require.NoError(t, s.RemoveCommitteeAssociation(ctx, kind, id, "absent"))
			e, err = s.Get(ctx, key)
			require.NoError(t, err)
			require.Equal(t, "!del", string(e.Value()))
		}
	})
	t.Run("concurrent associations never lose siblings", func(t *testing.T) {
		var wg sync.WaitGroup
		results := make(chan error, 20)
		for i := range 20 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results <- s.UpsertCommitteeAssociation(ctx, "meeting", "concurrent", fmt.Sprint(i), "committee", nil)
			}()
		}
		wg.Wait()
		close(results)
		for err := range results {
			require.NoError(t, err)
		}
		e, err := s.Get(ctx, "v1-mappings.meeting-mappings.concurrent")
		require.NoError(t, err)
		var links map[string]committeeAssociation
		require.NoError(t, json.Unmarshal(e.Value(), &links))
		require.Len(t, links, 20)
		require.NoError(t, s.RemoveCommitteeAssociation(ctx, "meeting", "concurrent", "0"))
		e, err = s.Get(ctx, "v1-mappings.meeting-mappings.concurrent")
		require.NoError(t, err)
		links = nil
		require.NoError(t, json.Unmarshal(e.Value(), &links))
		require.NotContains(t, links, "0")
	})
	t.Run("repair scan returns current PG revisions", func(t *testing.T) {
		entries, err := s.Scan(ctx, "v1_meeting_registrants")
		require.NoError(t, err)
		require.Len(t, entries, 1)
		require.Greater(t, entries[0].Revision(), uint64(0))
	})
}

type fakeStream struct {
	messages []*jetstream.RawStreamMsg
	changing bool
	infos    int
}

func (f *fakeStream) Info(context.Context) (*jetstream.StreamInfo, error) {
	f.infos++
	last := f.messages[len(f.messages)-1].Sequence
	if f.changing && f.infos > 1 {
		last++
	}
	return &jetstream.StreamInfo{State: jetstream.StreamState{FirstSeq: 1, LastSeq: last, Msgs: uint64(len(f.messages))}}, nil
}
func (f *fakeStream) Next(_ context.Context, seq uint64, subject string) (*jetstream.RawStreamMsg, error) {
	for _, msg := range f.messages {
		if msg.Sequence >= seq && strings.HasPrefix(msg.Subject, strings.TrimSuffix(subject, ">")) {
			return msg, nil
		}
	}
	return nil, jetstream.ErrMsgNotFound
}

func TestImport(t *testing.T) {
	for _, name := range []string{"stable", "changing", "conflicting", "invalid"} {
		t.Run(name, func(t *testing.T) {
			s := testStore(t)
			ctx := context.Background()
			value := []byte("1")
			key := "v1_meetings.test"
			if name == "invalid" {
				key = "v1-mappings.meeting-mappings.test"
				value = []byte("bad-json")
			}
			f := &fakeStream{changing: name == "changing", messages: []*jetstream.RawStreamMsg{{Sequence: 1, Subject: "$KV.v1-mappings." + key, Data: value}}}
			if name == "conflicting" {
				_, err := s.Put(ctx, key, []byte("!del"))
				require.NoError(t, err)
			}
			count, err := s.importSource(ctx, f, true)
			if name != "stable" {
				require.Error(t, err)
				require.Error(t, s.Ready(ctx))
				return
			}
			require.NoError(t, err)
			require.Equal(t, 1, count)
			require.NoError(t, s.Ready(ctx))
			_, err = s.importSource(ctx, f, true)
			require.Error(t, err)
		})
	}
}

func TestImportOwnedFamiliesAndNativeDeletion(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	f := &fakeStream{}
	seq := uint64(0)
	add := func(key, value, op string) {
		seq += 3 // Sparse stream sequences are normal after retention/purges.
		f.messages = append(f.messages, &jetstream.RawStreamMsg{Sequence: seq, Subject: "$KV.v1-mappings." + key, Data: []byte(value), Header: nats.Header{"KV-Operation": []string{op}}})
	}
	want := map[string]string{}
	// Independent fixture list: removing an importer family must fail this test.
	for _, kind := range []string{"v1_meetings", "v1_meeting_registrants", "v1_invite_responses", "v1_meeting_attachments", "v1_past_meetings", "v1_past_meeting_invitees", "v1_past_meeting_attendees", "v1_past_meeting_recordings", "v1_past_meeting_summaries", "v1_past_meeting_attachments"} {
		key := kind + ".test"
		want[key] = "1"
		add("project.sfid.unrelated", "not-meeting-state", "")
		add(key, "", "")
		add(key, "1", "")
	}
	for _, kind := range []string{"invitee_id", "attendee_id", "participant_to_invitee_id", "participant_to_attendee_id"} {
		key := "past_meeting_participant." + kind + ".test"
		want[key] = "target-" + kind
		add(key, "old-target", "")
		add("survey.unrelated", "1", "")
		add(key, want[key], "")
	}
	for key, value := range map[string]string{
		"v1-mappings.meeting-mappings.meeting":                  `{"association":{"committee_id":"committee"}}`,
		"v1-mappings.past-meeting-mappings.past":                `{}`,
		"v1-mappings.past-meeting-summaries.past":               `["summary"]`,
		"v1_participant_by_meeting_user.invitee.past.test.user": "invitee",
		"v1_participant_by_meeting_user.invitee.past.test/user": "invitee-slash",
		"v1_meeting_registrant_lfid_invite_sent.reg":            "invite",
	} {
		want[key] = value
		add(key, value, "")
	}
	for _, op := range []string{"DEL", "PURGE"} {
		key := "v1_meetings.deleted-" + op
		add(key, "1", "")
		add(key, "", op)
		key = "v1_meetings.recreated-" + op
		add(key, "old", "")
		add(key, "", op)
		add(key, "1", "")
		want[key] = "1"
	}
	add("v1_past_meetings.retained-tombstone", "!del", "")
	want["v1_past_meetings.retained-tombstone"] = "!del"
	count, err := s.importSource(ctx, f, true)
	require.NoError(t, err)
	require.Equal(t, len(want), count)
	for key, value := range want {
		e, err := s.Get(ctx, key)
		require.NoError(t, err, "missing owned family: %s", key)
		b, _ := bind(key)
		expected, err := b.decode([]byte(value))
		require.NoError(t, err)
		actual, err := b.decode(e.Value())
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	}
	for _, op := range []string{"DEL", "PURGE"} {
		_, err := s.Get(ctx, "v1_meetings.deleted-"+op)
		require.ErrorIs(t, err, jetstream.ErrKeyNotFound)
	}
	_, err = s.Get(ctx, "project.sfid.unrelated")
	require.Error(t, err)
	require.NoError(t, s.Ready(ctx))
}
