// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/nats-io/nats.go/jetstream"
)

// NATS Key-Value store bucket names
const (
	KVStoreNameMeetings                = "meetings"
	KVStoreNameMeetingSettings         = "meeting-settings"
	KVStoreNameMeetingRegistrants      = "meeting-registrants"
	KVStoreNamePastMeetings            = "past-meetings"
	KVStoreNamePastMeetingParticipants = "past-meeting-participants"
	KVStoreNamePastMeetingRecordings   = "past-meeting-recordings"
	KVStoreNamePastMeetingTranscripts  = "past-meeting-transcripts"
	KVStoreNamePastMeetingSummaries    = "past-meeting-summaries"
)

// INatsKeyValue is a NATS KV interface needed for the [MeetingsService].
type INatsKeyValue interface {
	ListKeys(context.Context, ...jetstream.WatchOpt) (jetstream.KeyLister, error)
	Get(ctx context.Context, key string) (jetstream.KeyValueEntry, error)
	Put(context.Context, string, []byte) (uint64, error)
	Update(context.Context, string, []byte, uint64) (uint64, error)
	Delete(context.Context, string, ...jetstream.KVDeleteOpt) error
}

// NatsBaseRepository provides common NATS KV operations that can be reused across all repositories
type NatsBaseRepository[T any] struct {
	kvStore    INatsKeyValue
	entityName string // Used in error messages (e.g., "meeting", "registrant")
}

// NewNatsBaseRepository creates a new base repository for NATS KV operations
func NewNatsBaseRepository[T any](kvStore INatsKeyValue, entityName string) *NatsBaseRepository[T] {
	return &NatsBaseRepository[T]{
		kvStore:    kvStore,
		entityName: entityName,
	}
}

// IsReady checks if the repository is ready for use
func (r *NatsBaseRepository[T]) IsReady() bool {
	return r.kvStore != nil
}

// Get retrieves a raw entry from NATS KV store
func (r *NatsBaseRepository[T]) GetRaw(ctx context.Context, key string) (jetstream.KeyValueEntry, error) {
	if !r.IsReady() {
		return nil, domain.NewUnavailableError(fmt.Sprintf("%s repository is not available", r.entityName))
	}

	entry, err := r.kvStore.Get(ctx, key)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return nil, domain.NewNotFoundError(
				fmt.Sprintf("%s with key '%s' not found", r.entityName, key), err)
		}
		slog.ErrorContext(ctx, fmt.Sprintf("error getting %s from NATS KV", r.entityName),
			logging.ErrKey, err, "key", key)
		return nil, domain.NewInternalError(
			fmt.Sprintf("failed to retrieve %s from store", r.entityName), err)
	}

	return entry, nil
}

// Get retrieves and unmarshals an entity from NATS KV store
func (r *NatsBaseRepository[T]) Get(ctx context.Context, key string) (*T, error) {
	entity, _, err := r.GetWithRevision(ctx, key)
	return entity, err
}

// GetWithRevision retrieves an entity with its revision from NATS KV store
func (r *NatsBaseRepository[T]) GetWithRevision(ctx context.Context, key string) (*T, uint64, error) {
	entry, err := r.GetRaw(ctx, key)
	if err != nil {
		return nil, 0, err
	}

	entity, err := r.Unmarshal(ctx, entry)
	if err != nil {
		return nil, 0, domain.NewInternalError(
			fmt.Sprintf("failed to unmarshal %s data", r.entityName), err)
	}

	return entity, entry.Revision(), nil
}

// Unmarshal unmarshals a NATS KV entry into the entity type
func (r *NatsBaseRepository[T]) Unmarshal(ctx context.Context, entry jetstream.KeyValueEntry) (*T, error) {
	var entity T
	err := json.Unmarshal(entry.Value(), &entity)
	if err != nil {
		slog.ErrorContext(ctx, fmt.Sprintf("error unmarshaling %s", r.entityName),
			logging.ErrKey, err)
		return nil, err
	}

	return &entity, nil
}

// Marshal marshals an entity to JSON bytes
func (r *NatsBaseRepository[T]) Marshal(ctx context.Context, entity *T) ([]byte, error) {
	data, err := json.Marshal(entity)
	if err != nil {
		slog.ErrorContext(ctx, fmt.Sprintf("error marshaling %s", r.entityName),
			logging.ErrKey, err)
		return nil, err
	}

	return data, nil
}

// Exists checks if an entity exists in the store
func (r *NatsBaseRepository[T]) Exists(ctx context.Context, key string) (bool, error) {
	_, err := r.Get(ctx, key)
	if err != nil {
		if domain.GetErrorType(err) == domain.ErrorTypeNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Create creates a new entity in the store using Put
func (r *NatsBaseRepository[T]) Create(ctx context.Context, key string, entity *T) error {
	if !r.IsReady() {
		return domain.NewUnavailableError(fmt.Sprintf("%s repository is not available", r.entityName))
	}

	data, err := r.Marshal(ctx, entity)
	if err != nil {
		return domain.NewInternalError(fmt.Sprintf("failed to marshal %s", r.entityName), err)
	}

	_, err = r.kvStore.Put(ctx, key, data)
	if err != nil {
		slog.ErrorContext(ctx, fmt.Sprintf("error creating %s in NATS KV", r.entityName),
			logging.ErrKey, err, "key", key)
		return domain.NewInternalError(fmt.Sprintf("failed to create %s in store", r.entityName), err)
	}

	return nil
}

// Update updates an existing entity in the store with optimistic concurrency control
func (r *NatsBaseRepository[T]) Update(ctx context.Context, key string, entity *T, revision uint64) error {
	if !r.IsReady() {
		return domain.NewUnavailableError(fmt.Sprintf("%s repository is not available", r.entityName))
	}

	data, err := r.Marshal(ctx, entity)
	if err != nil {
		return domain.NewInternalError(fmt.Sprintf("failed to marshal %s", r.entityName), err)
	}

	_, err = r.kvStore.Update(ctx, key, data, revision)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return domain.NewNotFoundError(fmt.Sprintf("%s not found", r.entityName), err)
		}
		if strings.Contains(err.Error(), "wrong last sequence") {
			return domain.NewConflictError(fmt.Sprintf("%s has been modified", r.entityName), err)
		}
		slog.ErrorContext(ctx, fmt.Sprintf("error updating %s in NATS KV", r.entityName),
			logging.ErrKey, err, "key", key, "revision", revision)
		return domain.NewInternalError(fmt.Sprintf("failed to update %s in store", r.entityName), err)
	}

	return nil
}

// Delete removes an entity from the store with optimistic concurrency control
func (r *NatsBaseRepository[T]) Delete(ctx context.Context, key string, revision uint64) error {
	if !r.IsReady() {
		return domain.NewUnavailableError(fmt.Sprintf("%s repository is not available", r.entityName))
	}

	err := r.kvStore.Delete(ctx, key, jetstream.LastRevision(revision))
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return domain.NewNotFoundError(fmt.Sprintf("%s not found", r.entityName), err)
		}
		if strings.Contains(err.Error(), "wrong last sequence") {
			return domain.NewConflictError(fmt.Sprintf("%s has been modified", r.entityName), err)
		}
		slog.ErrorContext(ctx, fmt.Sprintf("error deleting %s from NATS KV", r.entityName),
			logging.ErrKey, err, "key", key, "revision", revision)
		return domain.NewInternalError(fmt.Sprintf("failed to delete %s from store", r.entityName), err)
	}

	return nil
}

// DeleteWithoutRevision removes an entity from the store without revision checking
// This will delete the key regardless of its current revision, useful for cleanup operations
func (r *NatsBaseRepository[T]) DeleteWithoutRevision(ctx context.Context, key string) error {
	if !r.IsReady() {
		return domain.NewUnavailableError(fmt.Sprintf("%s repository is not available", r.entityName))
	}

	err := r.kvStore.Delete(ctx, key)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return domain.NewNotFoundError(fmt.Sprintf("%s not found", r.entityName), err)
		}
		slog.ErrorContext(ctx, fmt.Sprintf("error deleting %s from NATS KV", r.entityName),
			logging.ErrKey, err, "key", key)
		return domain.NewInternalError(fmt.Sprintf("failed to delete %s from store", r.entityName), err)
	}

	return nil
}

// ListKeys lists all keys in the store with optional filtering
func (r *NatsBaseRepository[T]) ListKeys(ctx context.Context) ([]string, error) {
	if !r.IsReady() {
		return nil, domain.NewUnavailableError(fmt.Sprintf("%s repository is not available", r.entityName))
	}

	lister, err := r.kvStore.ListKeys(ctx)
	if err != nil {
		slog.ErrorContext(ctx, fmt.Sprintf("error listing %s keys from NATS KV", r.entityName),
			logging.ErrKey, err)
		return nil, domain.NewInternalError(
			fmt.Sprintf("failed to list %s keys from store", r.entityName), err)
	}

	var keys []string
	for key := range lister.Keys() {
		keys = append(keys, key)
	}

	return keys, nil
}

// ListEntities lists all entities matching a key pattern
func (r *NatsBaseRepository[T]) ListEntities(ctx context.Context, keyPattern string) ([]*T, error) {
	keys, err := r.ListKeys(ctx)
	if err != nil {
		return nil, err
	}

	var entities []*T
	for _, key := range keys {
		// If keyPattern is provided, filter keys
		if keyPattern != "" && !matchesPattern(key, keyPattern) {
			continue
		}

		entity, err := r.Get(ctx, key)
		if err != nil {
			// Log error but continue with other entities
			slog.WarnContext(ctx, fmt.Sprintf("failed to get %s, skipping", r.entityName),
				"key", key, logging.ErrKey, err)
			continue
		}

		entities = append(entities, entity)
	}

	return entities, nil
}

// ListEntitiesEncoded lists all entities where keys are base64 encoded and need decoding before pattern matching
func (r *NatsBaseRepository[T]) ListEntitiesEncoded(ctx context.Context, keyPattern string, kb *KeyBuilder) ([]*T, error) {
	keys, err := r.ListKeys(ctx)
	if err != nil {
		return nil, err
	}

	var entities []*T
	for _, encodedKey := range keys {
		// Decode the key first
		decodedKey, err := kb.DecodeKey(encodedKey)
		if err != nil {
			slog.WarnContext(ctx, "failed to decode key, skipping",
				"encoded_key", encodedKey, logging.ErrKey, err)
			continue
		}

		// If keyPattern is provided, check against decoded key
		if keyPattern != "" && !matchesPattern(decodedKey, keyPattern) {
			continue
		}

		// Fetch using the encoded key
		entity, err := r.Get(ctx, encodedKey)
		if err != nil {
			// Log error but continue with other entities
			slog.WarnContext(ctx, fmt.Sprintf("failed to get %s, skipping", r.entityName),
				"key", encodedKey, logging.ErrKey, err)
			continue
		}

		entities = append(entities, entity)
	}

	return entities, nil
}

// matchesPattern provides simple pattern matching (can be enhanced)
func matchesPattern(key, pattern string) bool {
	if pattern == "*" || pattern == "" {
		return true
	}
	return strings.Contains(key, pattern)
}

// PutIndex creates an index entry in the store (stores empty value, key is used for indexing)
func (r *NatsBaseRepository[T]) PutIndex(ctx context.Context, indexKey string) error {
	if !r.IsReady() {
		return domain.NewUnavailableError(fmt.Sprintf("%s repository is not available", r.entityName))
	}

	_, err := r.kvStore.Put(ctx, indexKey, []byte{})
	if err != nil {
		slog.ErrorContext(ctx, "error creating index",
			logging.ErrKey, err, "index_key", indexKey)
		return domain.NewInternalError("failed to create index", err)
	}

	return nil
}

// DeleteIndex removes an index entry from the store
func (r *NatsBaseRepository[T]) DeleteIndex(ctx context.Context, indexKey string) error {
	if !r.IsReady() {
		return domain.NewUnavailableError(fmt.Sprintf("%s repository is not available", r.entityName))
	}

	err := r.kvStore.Delete(ctx, indexKey)
	if err != nil {
		slog.WarnContext(ctx, "error deleting index",
			logging.ErrKey, err, "index_key", indexKey)
		return domain.NewInternalError("failed to delete index", err)
	}

	return nil
}
