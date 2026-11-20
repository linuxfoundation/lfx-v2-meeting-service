// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import (
	"context"
	"errors"

	"github.com/nats-io/nats.go/jetstream"
)

// ErrEventStateUnavailable is retryable and carries no connection or user data.
var ErrEventStateUnavailable = errors.New("meeting event state unavailable")

// EventState is the persistence boundary for meeting processing state. The
// legacy entry encoding is retained at this boundary to keep event behavior
// unchanged; the Postgres adapter decomposes it into service-owned columns.
// Source records remain in NATS. Revisions belong to this store only.
type EventState interface {
	Get(context.Context, string) (jetstream.KeyValueEntry, error)
	Put(context.Context, string, []byte) (uint64, error)
	Create(context.Context, string, []byte, ...jetstream.KVCreateOpt) (uint64, error)
	Update(context.Context, string, []byte, uint64) (uint64, error)
	Delete(context.Context, string, ...jetstream.KVDeleteOpt) error
}

// CommitteeAssociationState updates one association without replacing sibling
// associations from a stale read-modify-write aggregate.
type CommitteeAssociationState interface {
	UpsertCommitteeAssociation(context.Context, string, string, string, string, []string) error
	RemoveCommitteeAssociation(context.Context, string, string, string) error
}
