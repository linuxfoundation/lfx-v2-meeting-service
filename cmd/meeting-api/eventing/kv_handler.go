// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/vmihailenco/msgpack/v5"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

// EventHandlers contains all the specific event type handlers
type EventHandlers struct {
	publisher    domain.EventPublisher
	userLookup   domain.V1UserLookup
	idMapper     domain.IDMapper
	v1ObjectsKV  jetstream.KeyValue
	v1MappingsKV jetstream.KeyValue
	logger       *slog.Logger
}

// NewEventHandlers creates a new event handlers struct
func NewEventHandlers(
	publisher domain.EventPublisher,
	userLookup domain.V1UserLookup,
	idMapper domain.IDMapper,
	v1ObjectsKV jetstream.KeyValue,
	v1MappingsKV jetstream.KeyValue,
	logger *slog.Logger,
) *EventHandlers {
	return &EventHandlers{
		publisher:    publisher,
		userLookup:   userLookup,
		idMapper:     idMapper,
		v1ObjectsKV:  v1ObjectsKV,
		v1MappingsKV: v1MappingsKV,
		logger:       logger,
	}
}

// kvHandler routes KV bucket events to appropriate handlers
// Returns true if the message should be retried (NAK), false if done (ACK)
func kvHandler(ctx context.Context, msg jetstream.Msg, handlers *EventHandlers) bool {
	// Extract key from subject (format: $KV.v1-objects.{key})
	subject := msg.Subject()
	parts := strings.Split(subject, ".")
	if len(parts) < 3 {
		handlers.logger.Error("invalid subject format", "subject", subject)
		return false // ACK - malformed subject
	}
	key := strings.Join(parts[2:], ".")

	// Get operation type
	metadata, err := msg.Metadata()
	if err != nil {
		handlers.logger.With(logging.ErrKey, err).Error("failed to get message metadata")
		return false // ACK - can't get metadata
	}

	operation := getOperation(metadata)
	handlers.logger.Info("processing KV event",
		"key", key,
		"operation", operation,
		"num_delivered", metadata.NumDelivered,
	)

	// Handle delete operations
	if operation == jetstream.KeyValueDelete || operation == jetstream.KeyValuePurge {
		return handleKVDelete(ctx, key, handlers)
	}

	// Handle put operations - decode the data
	data, err := decodeData(msg.Data())
	if err != nil {
		handlers.logger.With(logging.ErrKey, err).Error("failed to decode message data", "key", key)
		return false // ACK - permanent error
	}

	return handleKVPut(ctx, key, data, handlers)
}

// handleKVPut routes put/update operations to specific handlers
func handleKVPut(ctx context.Context, key string, data map[string]any, handlers *EventHandlers) bool {
	switch {
	case strings.HasPrefix(key, "itx-zoom-meetings-v2."):
		return handleMeetingUpdate(ctx, key, data, handlers.publisher, handlers.userLookup, handlers.idMapper, handlers.v1ObjectsKV, handlers.v1MappingsKV, handlers.logger)

	case strings.HasPrefix(key, "itx-zoom-meetings-mappings-v2."):
		return handleMeetingMappingUpdate(ctx, key, data, handlers.publisher, handlers.userLookup, handlers.idMapper, handlers.v1ObjectsKV, handlers.v1MappingsKV, handlers.logger)

	case strings.HasPrefix(key, "itx-zoom-meetings-registrants-v2."):
		return handleRegistrantUpdate(ctx, key, data, handlers.publisher, handlers.userLookup, handlers.idMapper, handlers.v1ObjectsKV, handlers.v1MappingsKV, handlers.logger)

	case strings.HasPrefix(key, "itx-zoom-meetings-invite-responses-v2."):
		return handleInviteResponseUpdate(ctx, key, data, handlers.publisher, handlers.userLookup, handlers.idMapper, handlers.v1ObjectsKV, handlers.v1MappingsKV, handlers.logger)

	case strings.HasPrefix(key, "itx-zoom-past-meetings-mappings."):
		return handlePastMeetingMappingUpdate(ctx, key, data, handlers.publisher, handlers.userLookup, handlers.idMapper, handlers.v1ObjectsKV, handlers.v1MappingsKV, handlers.logger)

	case strings.HasPrefix(key, "itx-zoom-past-meetings."):
		return handlePastMeetingUpdate(ctx, key, data, handlers.publisher, handlers.userLookup, handlers.idMapper, handlers.v1ObjectsKV, handlers.v1MappingsKV, handlers.logger)

	case strings.HasPrefix(key, "itx-zoom-past-meetings-invitees."):
		handlers.logger.Debug("past meeting invitee event - not yet implemented", "key", key)
		return false // ACK for now - will implement in phase 5

	case strings.HasPrefix(key, "itx-zoom-past-meetings-attendees."):
		handlers.logger.Debug("past meeting attendee event - not yet implemented", "key", key)
		return false // ACK for now - will implement in phase 5

	case strings.HasPrefix(key, "itx-zoom-past-meetings-recordings."):
		handlers.logger.Debug("past meeting recording event - not yet implemented", "key", key)
		return false // ACK for now - will implement in phase 5

	case strings.HasPrefix(key, "itx-zoom-past-meetings-summaries."):
		handlers.logger.Debug("past meeting summary event - not yet implemented", "key", key)
		return false // ACK for now - will implement in phase 5

	default:
		// Not a meeting-related event, skip
		handlers.logger.Debug("skipping non-meeting event", "key", key)
		return false // ACK
	}
}

// handleKVDelete routes delete operations to specific handlers
func handleKVDelete(ctx context.Context, key string, handlers *EventHandlers) bool {
	handlers.logger.Info("handling delete operation", "key", key)

	// TODO: Implement delete handlers for each event type
	// For now, just ACK all deletes
	return false
}

// getOperation determines the operation type from metadata
func getOperation(metadata *jetstream.MsgMetadata) jetstream.KeyValueOp {
	// The operation is encoded in the metadata
	// For KV buckets, we check the headers
	return jetstream.KeyValuePut // Default to PUT
}

// decodeData attempts to decode message data as JSON or MessagePack
func decodeData(data []byte) (map[string]any, error) {
	var result map[string]any

	// Try JSON first
	if err := json.Unmarshal(data, &result); err == nil {
		return result, nil
	}

	// Try MessagePack
	if err := msgpack.Unmarshal(data, &result); err == nil {
		return result, nil
	}

	// If both fail, return JSON error
	return nil, json.Unmarshal(data, &result)
}
