// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/nats-io/nats.go"
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

// encodeKey encodes a key for NATS KV store.
// From https://github.com/ripienaar/encodedkv
//
// NATS limitations: https://docs.nats.io/nats-concepts/jetstream/key-value-store#notes
func encodeKey(key string) (string, error) {
	res := []string{}
	for _, part := range strings.Split(strings.TrimPrefix(key, "/"), "/") {
		if part == ">" || part == "*" {
			res = append(res, part)
			continue
		}

		dst := make([]byte, base64.StdEncoding.EncodedLen(len(part)))
		base64.StdEncoding.Encode(dst, []byte(part))
		res = append(res, string(dst))
	}

	if len(res) == 0 {
		return "", nats.ErrInvalidKey
	}

	return strings.Join(res, "."), nil
}

// decodeKey decodes a key for NATS KV store.
// From https://github.com/ripienaar/encodedkv
//
// NATS limitations: https://docs.nats.io/nats-concepts/jetstream/key-value-store#notes
func decodeKey(key string) (string, error) {
	res := []string{}
	for _, part := range strings.Split(key, ".") {
		k, err := base64.StdEncoding.DecodeString(part)
		if err != nil {
			return "", err
		}

		res = append(res, string(k))
	}

	if len(res) == 0 {
		return "", nats.ErrInvalidKey
	}

	return fmt.Sprintf("/%s", strings.Join(res, "/")), nil
}
