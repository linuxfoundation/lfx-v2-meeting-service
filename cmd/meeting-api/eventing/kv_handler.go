// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"fmt"
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

const tombstoneMarker = "!del"

// meetingDeleteConfig holds the configuration for deleting a meeting-related resource.
type meetingDeleteConfig struct {
	// indexerSubject is the NATS subject to send the indexer delete message to.
	indexerSubject string
	// deleteAccessSubject is the NATS subject for the access control delete message
	// (e.g. lfx.fga-sync.delete_access or lfx.fga-sync.member_remove).
	// Leave empty to skip sending an access control delete message.
	deleteAccessSubject string
	// tombstoneKeyFmts are fmt format strings (each with one %s for the ID) for
	// mappings that should be tombstoned on delete.
	tombstoneKeyFmts []string
}

// buildGenericDeleteAccessPayload builds the JSON payload for a lfx.fga-sync.delete_access message.
func buildGenericDeleteAccessPayload(objectType, uid string) ([]byte, error) {
	msg := map[string]interface{}{
		"object_type": objectType,
		"operation":   "delete_access",
		"data": map[string]interface{}{
			"uid": uid,
		},
	}
	return json.Marshal(msg)
}

// buildGenericMemberRemovePayload builds the JSON payload for a lfx.fga-sync.member_remove message.
// An empty relations slice instructs fga-sync to remove ALL relations for the user.
func buildGenericMemberRemovePayload(objectType, uid, username string) ([]byte, error) {
	msg := map[string]interface{}{
		"object_type": objectType,
		"operation":   "member_remove",
		"data": map[string]interface{}{
			"uid":       uid,
			"username":  username,
			"relations": []string{},
		},
	}
	return json.Marshal(msg)
}

// isTombstoned returns true if mappingKey holds a tombstone marker,
// meaning this delete was already processed and should be skipped.
func (h *EventHandlers) isTombstoned(ctx context.Context, mappingKey string) bool {
	entry, err := h.v1MappingsKV.Get(ctx, mappingKey)
	return err == nil && string(entry.Value()) == tombstoneMarker
}

// tombstoneMapping writes "!del" to mappingKey so that re-deliveries of the same
// delete event are detected and skipped.
func (h *EventHandlers) tombstoneMapping(ctx context.Context, mappingKey string) {
	if _, err := h.v1MappingsKV.Put(ctx, mappingKey, []byte(tombstoneMarker)); err != nil {
		h.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to tombstone mapping", "mapping_key", mappingKey)
	}
}

// handleMeetingTypeDelete is the generic delete handler for all meeting-related resources.
// It sends the indexer delete message, optionally sends an access control message,
// and tombstones any configured mapping keys.
// accessPayload is the pre-built payload for the access control message; callers are responsible for constructing it.
func (h *EventHandlers) handleMeetingTypeDelete(
	ctx context.Context,
	key, id string,
	accessPayload []byte,
	cfg meetingDeleteConfig,
) (retry bool) {
	funcLogger := h.logger.With("key", key, "id", id)
	funcLogger.DebugContext(ctx, "processing meeting-related delete")

	if err := h.publisher.PublishIndexerDelete(ctx, cfg.indexerSubject, id); err != nil {
		funcLogger.With(logging.ErrKey, err, "subject", cfg.indexerSubject).ErrorContext(ctx, "failed to send delete indexer message")
		return isTransientError(err)
	}

	if cfg.deleteAccessSubject != "" {
		if err := h.publisher.PublishAccessDelete(ctx, cfg.deleteAccessSubject, accessPayload); err != nil {
			funcLogger.With(logging.ErrKey, err, "subject", cfg.deleteAccessSubject).ErrorContext(ctx, "failed to send access control delete message")
			return isTransientError(err)
		}
	}

	for _, keyFmt := range cfg.tombstoneKeyFmts {
		h.tombstoneMapping(ctx, fmt.Sprintf(keyFmt, id))
	}

	funcLogger.InfoContext(ctx, "successfully processed delete")
	return false
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
func kvHandler(ctx context.Context, msg jetstream.Msg, handlers *EventHandlers) (retry bool) {
	// Extract key from subject (format: $KV.v1-objects.{key})
	subject := msg.Subject()
	parts := strings.Split(subject, ".")
	if len(parts) < 3 {
		handlers.logger.Error("invalid subject format", "subject", subject)
		return false
	}
	key := strings.Join(parts[2:], ".")

	// Get operation type
	metadata, err := msg.Metadata()
	if err != nil {
		handlers.logger.With(logging.ErrKey, err).Error("failed to get message metadata")
		return false
	}

	operation := getOperation(msg)
	handlers.logger.Info("processing KV event",
		"key", key,
		"operation", operation,
		"num_delivered", metadata.NumDelivered,
	)

	// Handle delete operations
	if operation == jetstream.KeyValueDelete || operation == jetstream.KeyValuePurge {
		return routeDelete(ctx, key, nil, handlers)
	}

	// Handle put operations - decode the data
	data, err := decodeData(msg.Data())
	if err != nil {
		handlers.logger.With(logging.ErrKey, err).Error("failed to decode message data", "key", key)
		return false
	}

	return handleKVPut(ctx, key, data, handlers)
}

// handleKVPut routes put/update operations to specific handlers.
// If the record carries a _sdc_deleted_at field it is treated as a soft delete
// and routed to handleKVSoftDelete instead of the normal update handler.
func handleKVPut(ctx context.Context, key string, data map[string]any, handlers *EventHandlers) (retry bool) {
	// Check for soft delete (record written to KV with _sdc_deleted_at set).
	if deletedAt, exists := data["_sdc_deleted_at"]; exists && deletedAt != nil && deletedAt != "" {
		handlers.logger.Info("processing soft delete", "key", key, "_sdc_deleted_at", deletedAt)
		return routeDelete(ctx, key, data, handlers)
	}

	switch {
	case strings.HasPrefix(key, "itx-zoom-meetings-v2."):
		return handlers.handleMeetingUpdate(ctx, key, data)

	case strings.HasPrefix(key, "itx-zoom-meetings-mappings-v2."):
		return handlers.handleMeetingMappingUpdate(ctx, key, data)

	case strings.HasPrefix(key, "itx-zoom-meetings-registrants-v2."):
		return handlers.handleRegistrantUpdate(ctx, key, data)

	case strings.HasPrefix(key, "itx-zoom-meetings-invite-responses-v2."):
		return handlers.handleInviteResponseUpdate(ctx, key, data)

	case strings.HasPrefix(key, "itx-zoom-past-meetings-mappings."):
		return handlers.handlePastMeetingMappingUpdate(ctx, key, data)

	case strings.HasPrefix(key, "itx-zoom-past-meetings."):
		return handlers.handlePastMeetingUpdate(ctx, key, data)

	case strings.HasPrefix(key, "itx-zoom-past-meetings-invitees."):
		return handlers.handlePastMeetingInviteeUpdate(ctx, key, data)

	case strings.HasPrefix(key, "itx-zoom-past-meetings-attendees."):
		return handlers.handlePastMeetingAttendeeUpdate(ctx, key, data)

	case strings.HasPrefix(key, "itx-zoom-past-meetings-recordings."):
		return handlers.handlePastMeetingRecordingUpdate(ctx, key, data)

	case strings.HasPrefix(key, "itx-zoom-past-meetings-summaries."):
		return handlers.handlePastMeetingSummaryUpdate(ctx, key, data)

	case strings.HasPrefix(key, "itx-zoom-meetings-attachments-v2."):
		return handlers.handleMeetingAttachmentUpdate(ctx, key, data)

	case strings.HasPrefix(key, "itx-zoom-past-meetings-attachments."):
		return handlers.handlePastMeetingAttachmentUpdate(ctx, key, data)

	default:
		// Not a meeting-related event, skip
		handlers.logger.Debug("skipping non-meeting event", "key", key)
		return false
	}
}

// routeDelete routes a delete operation to the appropriate entity-specific delete handler.
// v1Data is nil for hard KV deletes (DEL/PURGE) and populated for soft deletes
// (_sdc_deleted_at), allowing handlers to extract fields needed for access control messages.
func routeDelete(ctx context.Context, key string, v1Data map[string]any, handlers *EventHandlers) (retry bool) {
	handlers.logger.Info("routing delete operation", "key", key)

	switch {
	case strings.HasPrefix(key, "itx-zoom-meetings-v2."):
		return handlers.handleMeetingDelete(ctx, key, v1Data)

	case strings.HasPrefix(key, "itx-zoom-meetings-mappings-v2."):
		return handlers.handleMeetingMappingDelete(ctx, key, v1Data)

	case strings.HasPrefix(key, "itx-zoom-meetings-registrants-v2."):
		return handlers.handleRegistrantDelete(ctx, key, v1Data)

	case strings.HasPrefix(key, "itx-zoom-meetings-invite-responses-v2."):
		return handlers.handleInviteResponseDelete(ctx, key, v1Data)

	case strings.HasPrefix(key, "itx-zoom-past-meetings-mappings."):
		return handlers.handlePastMeetingMappingDelete(ctx, key, v1Data)

	case strings.HasPrefix(key, "itx-zoom-past-meetings."):
		return handlers.handlePastMeetingDelete(ctx, key, v1Data)

	case strings.HasPrefix(key, "itx-zoom-past-meetings-invitees."):
		return handlers.handlePastMeetingInviteeDelete(ctx, key, v1Data)

	case strings.HasPrefix(key, "itx-zoom-past-meetings-attendees."):
		return handlers.handlePastMeetingAttendeeDelete(ctx, key, v1Data)

	case strings.HasPrefix(key, "itx-zoom-past-meetings-recordings."):
		return handlers.handlePastMeetingRecordingDelete(ctx, key, v1Data)

	case strings.HasPrefix(key, "itx-zoom-past-meetings-summaries."):
		return handlers.handlePastMeetingSummaryDelete(ctx, key, v1Data)

	case strings.HasPrefix(key, "itx-zoom-meetings-attachments-v2."):
		return handlers.handleMeetingAttachmentDelete(ctx, key, v1Data)

	case strings.HasPrefix(key, "itx-zoom-past-meetings-attachments."):
		return handlers.handlePastMeetingAttachmentDelete(ctx, key, v1Data)

	default:
		handlers.logger.Debug("skipping delete for unrecognized key", "key", key)
		return false
	}
}

// getOperation determines the operation type from the KV-Operation message header.
// PUT is the default when the header is absent.
func getOperation(msg jetstream.Msg) jetstream.KeyValueOp {
	switch msg.Headers().Get("KV-Operation") {
	case "DEL":
		return jetstream.KeyValueDelete
	case "PURGE":
		return jetstream.KeyValuePurge
	default:
		return jetstream.KeyValuePut
	}
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
