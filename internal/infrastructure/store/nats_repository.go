// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package store

import (
	"context"

	"github.com/nats-io/nats.go/jetstream"
)

// INatsKeyValue is a NATS KV interface needed for the [MeetingsService].
type INatsKeyValue interface {
	ListKeys(context.Context, ...jetstream.WatchOpt) (jetstream.KeyLister, error)
	Get(ctx context.Context, key string) (jetstream.KeyValueEntry, error)
	Put(context.Context, string, []byte) (uint64, error)
	Update(context.Context, string, []byte, uint64) (uint64, error)
	Delete(context.Context, string, ...jetstream.KVDeleteOpt) error
}
