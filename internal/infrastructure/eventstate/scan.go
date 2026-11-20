// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventstate

import (
	"context"
	"errors"

	"github.com/nats-io/nats.go/jetstream"
)

// Scan returns current resource state for a repair pass. Each entry carries its
// PG revision; repair uses CAS and never applies a NATS revision to Postgres.
// Page bounds prevent a scan from holding a connection while doing point reads.
func (s *Store) Scan(ctx context.Context, kind string) ([]jetstream.KeyValueEntry, error) {
	if !resourceKinds[kind] {
		return nil, errors.New("unsupported repair resource kind")
	}
	var entries []jetstream.KeyValueEntry
	cursor := ""
	for {
		opCtx, cancel := context.WithTimeout(ctx, operationTimeout)
		rows, err := s.pool.Query(opCtx, "SELECT source_id FROM meeting_resource_sync_state WHERE resource_kind=$1 AND source_id>$2 ORDER BY source_id LIMIT 500", kind, cursor)
		if err != nil {
			cancel()
			return nil, err
		}
		var ids []string
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				break
			}
			ids = append(ids, id)
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		cancel()
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return entries, nil
		}
		for _, id := range ids {
			e, err := s.Get(ctx, kind+"."+id)
			if errors.Is(err, jetstream.ErrKeyNotFound) {
				continue
			}
			if err != nil {
				return nil, err
			}
			entries = append(entries, e)
		}
		cursor = ids[len(ids)-1]
	}
}
