// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package store

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/nats-io/nats.go"
)

// Common key prefixes
const (
	// Entity prefixes
	KeyPrefixRegistrant  = "registrant"
	KeyPrefixMeeting     = "meeting"
	KeyPrefixPastMeeting = "past-meeting"
	KeyPrefixParticipant = "participant"
	KeyPrefixRecording   = "recording"
	KeyPrefixTranscript  = "transcript"
	KeyPrefixSummary     = "summary"

	// Index prefixes
	KeyPrefixIndex           = "index"
	KeyPrefixIndexMeeting    = "meeting"
	KeyPrefixIndexEmail      = "email"
	KeyPrefixIndexCommittee  = "committee"
	KeyPrefixIndexProject    = "project"
	KeyPrefixIndexPlatform   = "platform"
	KeyPrefixIndexOccurrence = "occurrence"
)

// KeyBuilder provides utilities for building consistent NATS KV keys
type KeyBuilder struct {
	prefix string
}

// NewKeyBuilder creates a new key builder with an optional prefix
func NewKeyBuilder(prefix string) *KeyBuilder {
	return &KeyBuilder{
		prefix: prefix,
	}
}

// EntityKey builds a key for an entity (e.g., "registrant/uid-123")
func (kb *KeyBuilder) EntityKey(entityType, uid string) string {
	key := fmt.Sprintf("%s/%s", entityType, uid)
	return kb.applyPrefix(key, false)
}

// EntityKeyEncoded builds an encoded key for an entity
func (kb *KeyBuilder) EntityKeyEncoded(entityType, uid string) string {
	key := fmt.Sprintf("%s/%s", entityType, uid)
	return kb.applyPrefix(key, true)
}

// IndexKey builds a key for an index (e.g., "index/meeting/meeting-uid/registrant-uid")
func (kb *KeyBuilder) IndexKey(indexType, indexValue, entityUID string) string {
	key := fmt.Sprintf("%s/%s/%s/%s", KeyPrefixIndex, indexType, indexValue, entityUID)
	return kb.applyPrefix(key, false)
}

// IndexKeyEncoded builds an encoded key for an index
func (kb *KeyBuilder) IndexKeyEncoded(indexType, indexValue, entityUID string) string {
	key := fmt.Sprintf("%s/%s/%s/%s", KeyPrefixIndex, indexType, indexValue, entityUID)
	return kb.applyPrefix(key, true)
}

// CompoundKey builds a compound key from multiple parts
func (kb *KeyBuilder) CompoundKey(parts ...string) string {
	key := strings.Join(parts, "/")
	return kb.applyPrefix(key, false)
}

// applyPrefix adds the builder's prefix if one is set
func (kb *KeyBuilder) applyPrefix(key string, encode bool) string {
	var fullKey string
	if kb.prefix == "" {
		fullKey = key
	} else {
		fullKey = fmt.Sprintf("%s/%s", kb.prefix, key)
	}

	if encode {
		encodedKey, err := kb.EncodeKey(fullKey)
		if err != nil {
			slog.Error("error encoding key", logging.ErrKey, err, "key", fullKey)
			return fullKey
		}
		return encodedKey
	}
	return fullKey
}

// EncodeKey encodes a key for NATS KV store.
// From https://github.com/ripienaar/encodedkv
//
// NATS limitations: https://docs.nats.io/nats-concepts/jetstream/key-value-store#notes
func (kb *KeyBuilder) EncodeKey(key string) (string, error) {
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

// DecodeKey decodes a key for NATS KV store.
// From https://github.com/ripienaar/encodedkv
//
// NATS limitations: https://docs.nats.io/nats-concepts/jetstream/key-value-store#notes
func (kb *KeyBuilder) DecodeKey(key string) (string, error) {
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
