// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventstate

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/nats-io/nats.go/jetstream"
)

//go:embed schema.sql
var schema string

const operationTimeout = 5 * time.Second

// Store persists decomposed event state in meeting-owned Postgres tables.
// It never opens or falls back to the legacy mappings KV bucket.
type Store struct{ pool *pgxpool.Pool }

var _ domain.EventState = (*Store)(nil)

type databaseFailure struct{ cause error }

func (e databaseFailure) Error() string   { return domain.ErrEventStateUnavailable.Error() }
func (e databaseFailure) Unwrap() []error { return []error{domain.ErrEventStateUnavailable, e.cause} }
func databaseError(err error) error {
	if err == nil {
		return nil
	}
	return databaseFailure{cause: err}
}

func Open(ctx context.Context, dsn string) (*Store, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("MEETING_STATE_DATABASE_URL is required for meeting event state")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, errors.New("invalid meeting state database configuration")
	}
	config.ConnConfig.ConnectTimeout = operationTimeout
	config.MaxConns = 10
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, errors.New("cannot initialize meeting state database pool")
	}
	s := &Store{pool: pool}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, errors.New("meeting state database connection failed")
	}
	return s, nil
}

// ApplySchema is explicit: deployment must apply the schema and complete the
// verified backfill before runtime is enabled. Runtime does not seed empty state.
func (s *Store) ApplySchema(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(1948001)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, schema); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) Close() { s.pool.Close() }

// Ready fails closed until the offline importer has verified every owned entry.
func (s *Store) Ready(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	var ready bool
	err := s.pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM meeting_event_state_cutover)").Scan(&ready)
	if err != nil || !ready {
		return errors.New("meeting state cutover has not been verified")
	}
	return nil
}

func where(b binding) string {
	clauses := make([]string, len(b.keys))
	for i, col := range b.keys {
		clauses[i] = fmt.Sprintf("%s=$%d", col, i+1)
	}
	return strings.Join(clauses, " AND ")
}

type entry struct {
	key      string
	value    []byte
	revision uint64
	created  time.Time
}

func (e entry) Bucket() string                  { return "meeting-event-state" }
func (e entry) Key() string                     { return e.key }
func (e entry) Value() []byte                   { return e.value }
func (e entry) Revision() uint64                { return e.revision }
func (e entry) Created() time.Time              { return e.created }
func (e entry) Delta() uint64                   { return 0 }
func (e entry) Operation() jetstream.KeyValueOp { return jetstream.KeyValuePut }

func (s *Store) Get(ctx context.Context, key string) (jetstream.KeyValueEntry, error) {
	b, err := bind(key)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	e := entry{key: key}
	row := s.pool.QueryRow(ctx, "SELECT "+strings.Join(b.columns, ",")+",version,updated_at FROM "+b.table+" WHERE "+where(b), b.args...)
	switch b.table {
	case "meeting_resource_sync_state":
		var state string
		var p principal
		err = row.Scan(&state, &p.UID, &p.Username, &p.MeetingID, &p.Authoritative, &e.revision, &e.created)
		if err == nil {
			switch state {
			case "empty":
				e.value = []byte{}
			case "processed":
				e.value = []byte("1")
			case "deleted":
				e.value = []byte("!del")
			case "legacy":
				e.value = []byte(p.UID)
			case "principal":
				e.value, err = json.Marshal(p)
			}
		}
	case "meeting_committee_associations":
		var deleted bool
		err = row.Scan(&deleted, &e.value, &e.revision, &e.created)
		if deleted {
			e.value = []byte("!del")
		}
	case "meeting_summary_associations":
		var ids []string
		err = row.Scan(&ids, &e.revision, &e.created)
		if err == nil {
			e.value, err = json.Marshal(ids)
		}
	case "meeting_participant_lookup", "meeting_participant_aliases":
		var id string
		var deleted bool
		err = row.Scan(&id, &deleted, &e.revision, &e.created)
		e.value = []byte(id)
		if deleted {
			e.value = []byte("!del")
		}
	case "meeting_invitation_guards":
		var id string
		var pending bool
		err = row.Scan(&id, &pending, &e.revision, &e.created)
		e.value = []byte(id)
		if pending {
			e.value = []byte("pending")
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, jetstream.ErrKeyNotFound
	}
	if err != nil {
		return nil, databaseError(err)
	}
	return e, nil
}

func (s *Store) Put(ctx context.Context, key string, value []byte) (uint64, error) {
	return s.write(ctx, key, value, "put", 0)
}
func (s *Store) Create(ctx context.Context, key string, value []byte, opts ...jetstream.KVCreateOpt) (uint64, error) {
	if len(opts) > 0 {
		return 0, errors.New("event state create options are not supported")
	}
	return s.write(ctx, key, value, "create", 0)
}
func (s *Store) Update(ctx context.Context, key string, value []byte, rev uint64) (uint64, error) {
	return s.write(ctx, key, value, "update", rev)
}

func (s *Store) write(ctx context.Context, key string, value []byte, op string, rev uint64) (uint64, error) {
	b, err := bind(key)
	if err != nil {
		return 0, err
	}
	values, err := b.decode(value)
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	args := append(append([]any{}, b.args...), values...)
	assignments := make([]string, len(b.columns))
	for i, col := range b.columns {
		assignments[i] = fmt.Sprintf("%s=$%d", col, len(b.keys)+i+1)
	}
	var query string
	if op == "update" {
		args = append(args, rev)
		query = "UPDATE " + b.table + " SET " + strings.Join(assignments, ",") + ",version=nextval('meeting_event_state_revision'),updated_at=now() WHERE " + where(b) + fmt.Sprintf(" AND version=$%d RETURNING version", len(args))
	} else {
		cols := append(append([]string{}, b.keys...), b.columns...)
		params := make([]string, len(args))
		for i := range params {
			params[i] = fmt.Sprintf("$%d", i+1)
		}
		query = "INSERT INTO " + b.table + " (" + strings.Join(cols, ",") + ") VALUES (" + strings.Join(params, ",") + ") ON CONFLICT (" + strings.Join(b.keys, ",") + ") "
		if op == "create" {
			query += "DO NOTHING"
		} else {
			for i, col := range b.columns {
				assignments[i] = col + "=EXCLUDED." + col
			}
			query += "DO UPDATE SET " + strings.Join(assignments, ",") + ",version=nextval('meeting_event_state_revision'),updated_at=now()"
		}
		query += " RETURNING version"
	}
	var version uint64
	err = s.pool.QueryRow(ctx, query, args...).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		if op == "create" {
			return 0, jetstream.ErrKeyExists
		}
		return 0, jetstream.ErrKeyRevisionMismatch
	}
	return version, databaseError(err)
}

func (s *Store) Delete(ctx context.Context, key string, opts ...jetstream.KVDeleteOpt) error {
	if len(opts) > 0 {
		return errors.New("event state delete options are not supported")
	}
	b, err := bind(key)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	_, err = s.pool.Exec(ctx, "DELETE FROM "+b.table+" WHERE "+where(b), b.args...)
	return databaseError(err)
}
