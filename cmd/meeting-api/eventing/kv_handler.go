// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/vmihailenco/msgpack/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	fgatypes "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/types"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

// EventHandlers contains all the specific event type handlers
type EventHandlers struct {
	publisher     domain.EventPublisher
	userLookup    domain.V1UserLookup
	idMapper      domain.IDMapper
	projectLookup domain.ProjectLookup
	v1ObjectsKV   jetstream.KeyValue
	v1MappingsKV  jetstream.KeyValue
	logger        *slog.Logger

	// Invite feature fields. inviteSender and userReader must be non-nil, and
	// selfServeBaseURL must be non-empty, for invite sending to be active.
	inviteSender     domain.InviteSender
	userReader       domain.UserReader
	selfServeBaseURL string
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
	msg := fgatypes.GenericFGAMessage{
		ObjectType: objectType,
		Operation:  "delete_access",
		Data:       fgatypes.GenericDeleteData{UID: uid},
	}
	return json.Marshal(msg)
}

// buildGenericMemberRemovePayload builds the JSON payload for a lfx.fga-sync.member_remove message.
// An empty relations slice instructs fga-sync to remove ALL relations for the user.
// Callers treat marshal failures as permanent errors (ACK, no retry) — they indicate code bugs,
// not transient conditions like the old auth-service username_to_sub lookup could return.
func buildGenericMemberRemovePayload(objectType, uid, username string) ([]byte, error) {
	msg := fgatypes.GenericFGAMessage{
		ObjectType: objectType,
		Operation:  "member_remove",
		Data: fgatypes.GenericMemberData{
			UID:       uid,
			Username:  username,
			Relations: []string{},
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

// entryIsTombstoned returns true if the already-fetched entry holds a tombstone marker.
// Use this when you have already called Get to avoid a second KV round-trip.
func entryIsTombstoned(entry jetstream.KeyValueEntry) bool {
	return string(entry.Value()) == tombstoneMarker
}

// tombstoneMapping writes "!del" to mappingKey so that re-deliveries of the same
// delete event are detected and skipped. Returns nil on success and on ErrInvalidKey
// (the key contains characters the KV store rejects; the xref could never have existed
// so the tombstone is a no-op). Any other error is returned to the caller for retry.
func (h *EventHandlers) tombstoneMapping(ctx context.Context, mappingKey string) error {
	if _, err := h.v1MappingsKV.Put(ctx, mappingKey, []byte(tombstoneMarker)); err != nil {
		if errors.Is(err, jetstream.ErrInvalidKey) {
			h.logger.WarnContext(ctx, "skipping tombstone: mapping key contains invalid characters", "mapping_key", mappingKey)
			return nil
		}
		h.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to tombstone mapping, will retry", "mapping_key", mappingKey)
		return err
	}
	return nil
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
		if err := h.tombstoneMapping(ctx, fmt.Sprintf(keyFmt, id)); err != nil {
			return true
		}
	}

	funcLogger.InfoContext(ctx, "successfully processed delete")
	return false
}

// NewEventHandlers creates a new event handlers struct
func NewEventHandlers(
	publisher domain.EventPublisher,
	userLookup domain.V1UserLookup,
	idMapper domain.IDMapper,
	projectLookup domain.ProjectLookup,
	v1ObjectsKV jetstream.KeyValue,
	v1MappingsKV jetstream.KeyValue,
	logger *slog.Logger,
	opts ...EventHandlersOption,
) *EventHandlers {
	h := &EventHandlers{
		publisher:     publisher,
		userLookup:    userLookup,
		idMapper:      idMapper,
		projectLookup: projectLookup,
		v1ObjectsKV:   v1ObjectsKV,
		v1MappingsKV:  v1MappingsKV,
		logger:        logger,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// EventHandlersOption is a functional option for EventHandlers.
type EventHandlersOption func(*EventHandlers)

// WithInviteFeature wires invite-sending capability into the event handlers.
// sender and reader must be non-nil, and selfServeBaseURL must be non-empty;
// if any of these conditions is not met, inviteEnabled() returns false and no
// invites are sent.
func WithInviteFeature(sender domain.InviteSender, reader domain.UserReader, selfServeBaseURL string) EventHandlersOption {
	return func(h *EventHandlers) {
		h.inviteSender = sender
		h.userReader = reader
		h.selfServeBaseURL = selfServeBaseURL
	}
}

// inviteEnabled reports whether the invite feature is fully wired up.
func (h *EventHandlers) inviteEnabled() bool {
	return h.inviteSender != nil &&
		h.userReader != nil &&
		strings.TrimSpace(h.selfServeBaseURL) != ""
}

// kvHandler routes KV bucket events to appropriate handlers
func kvHandler(ctx context.Context, msg jetstream.Msg, handlers *EventHandlers) (retry bool) {
	msgCtx := otel.GetTextMapPropagator().Extract(ctx, natsHeaderCarrier(msg.Headers()))
	msgCtx, span := tracer.Start(msgCtx, "nats.process",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "nats"),
			attribute.String("messaging.destination.name", msg.Subject()),
			attribute.String("messaging.operation.type", "process"),
		),
	)
	defer span.End()
	ctx = msgCtx

	// Extract key from subject (format: $KV.v1-objects.{key})
	subject := msg.Subject()
	parts := strings.Split(subject, ".")
	if len(parts) < 3 {
		span.SetStatus(codes.Error, "invalid subject format")
		handlers.logger.ErrorContext(ctx, "invalid subject format", "subject", subject)
		return false
	}
	key := strings.Join(parts[2:], ".")

	// Get operation type
	metadata, err := msg.Metadata()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		handlers.logger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to get message metadata")
		return false
	}

	operation := getOperation(msg)
	handlers.logger.InfoContext(ctx, "processing KV event",
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
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		handlers.logger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to decode message data", "key", key)
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
		handlers.logger.InfoContext(ctx, "processing soft delete", "key", key, "_sdc_deleted_at", deletedAt)
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
		handlers.logger.DebugContext(ctx, "skipping non-meeting event", "key", key)
		return false
	}
}

// routeDelete routes a delete operation to the appropriate entity-specific delete handler.
// v1Data is nil for hard KV deletes (DEL/PURGE) and populated for soft deletes
// (_sdc_deleted_at), allowing handlers to extract fields needed for access control messages.
func routeDelete(ctx context.Context, key string, v1Data map[string]any, handlers *EventHandlers) (retry bool) {
	handlers.logger.InfoContext(ctx, "routing delete operation", "key", key)

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
		handlers.logger.DebugContext(ctx, "skipping delete for unrecognized key", "key", key)
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

// msgpackMaxNestingDepth is the maximum container-nesting depth we allow
// before refusing to call msgpack.Unmarshal on untrusted KV data.
//
// Background: vmihailenco/msgpack v5 decodes into interface{} by recursing
// once per nesting level (DecodeInterface → decodeSlice/decodeMap →
// DecodeInterface).  There is no built-in depth cap.  A crafted ~1 MB
// fixarray(1)-chain payload encodes ~1 M levels in 1 byte each; at a few
// hundred bytes of stack frame per level the goroutine stack exceeds the
// pod's 512 Mi memory limit before Go's 1 GB ceiling is reached.
// runtime.recover() cannot intercept a fatal stack-overflow, so the whole
// meeting-api process (HTTP proxy + NATS subscribers) is killed.
//
// Legitimate v1-objects records are flat; 64 is very generous.
const msgpackMaxNestingDepth = 64

// checkMsgpackNestingDepth iteratively scans raw msgpack bytes and returns a
// non-nil error if any container (array or map) would push the nesting depth
// past msgpackMaxNestingDepth.  It allocates only a small depth-tracking
// slice and never recurses, so it is safe to call on untrusted input before
// passing the same bytes to msgpack.Unmarshal.
//
// Returns an error also for structurally truncated type-header fields so
// that Unmarshal is never called on malformed data.
func checkMsgpackNestingDepth(data []byte) error {
	// remaining[level] = number of msgpack values still to consume at that
	// level.  We start with 1 root value.
	remaining := make([]int, 1, msgpackMaxNestingDepth+1)
	remaining[0] = 1

	i := 0
	for i < len(data) && len(remaining) > 0 {
		b := data[i]
		i++

		var skip int     // raw bytes to skip for this type's inline payload
		var children int // child-value count for containers (0 = leaf / empty container)

		switch {
		// single-byte atoms (no payload)
		case b == 0xc0, b == 0xc2, b == 0xc3: // nil, false, true
		case b <= 0x7f: // positive fixint
		case b >= 0xe0: // negative fixint

		// integers
		case b == 0xcc:
			skip = 1 // uint8
		case b == 0xcd:
			skip = 2 // uint16
		case b == 0xce:
			skip = 4 // uint32
		case b == 0xcf:
			skip = 8 // uint64
		case b == 0xd0:
			skip = 1 // int8
		case b == 0xd1:
			skip = 2 // int16
		case b == 0xd2:
			skip = 4 // int32
		case b == 0xd3:
			skip = 8 // int64

		// floats
		case b == 0xca:
			skip = 4 // float32
		case b == 0xcb:
			skip = 8 // float64

		// str
		case b >= 0xa0 && b <= 0xbf: // fixstr
			skip = int(b & 0x1f)
		case b == 0xd9: // str8
			if i >= len(data) {
				return errors.New("truncated msgpack: str8 length byte missing")
			}
			skip = int(data[i])
			i++
		case b == 0xda: // str16
			if i+2 > len(data) {
				return errors.New("truncated msgpack: str16 length bytes missing")
			}
			skip = int(data[i])<<8 | int(data[i+1])
			i += 2
		case b == 0xdb: // str32
			if i+4 > len(data) {
				return errors.New("truncated msgpack: str32 length bytes missing")
			}
			skip = int(data[i])<<24 | int(data[i+1])<<16 | int(data[i+2])<<8 | int(data[i+3])
			i += 4

		// bin
		case b == 0xc4: // bin8
			if i >= len(data) {
				return errors.New("truncated msgpack: bin8 length byte missing")
			}
			skip = int(data[i])
			i++
		case b == 0xc5: // bin16
			if i+2 > len(data) {
				return errors.New("truncated msgpack: bin16 length bytes missing")
			}
			skip = int(data[i])<<8 | int(data[i+1])
			i += 2
		case b == 0xc6: // bin32
			if i+4 > len(data) {
				return errors.New("truncated msgpack: bin32 length bytes missing")
			}
			skip = int(data[i])<<24 | int(data[i+1])<<16 | int(data[i+2])<<8 | int(data[i+3])
			i += 4

		// ext (type byte + data bytes; type byte already counted in skip)
		case b == 0xd4:
			skip = 2 // fixext1: 1 type + 1 data
		case b == 0xd5:
			skip = 3 // fixext2
		case b == 0xd6:
			skip = 5 // fixext4
		case b == 0xd7:
			skip = 9 // fixext8
		case b == 0xd8:
			skip = 17 // fixext16
		case b == 0xc7: // ext8: length(1) + type(1) + data(length)
			if i >= len(data) {
				return errors.New("truncated msgpack: ext8 length byte missing")
			}
			skip = int(data[i]) + 1 // +1 for type byte
			i++
		case b == 0xc8: // ext16
			if i+2 > len(data) {
				return errors.New("truncated msgpack: ext16 length bytes missing")
			}
			skip = (int(data[i])<<8 | int(data[i+1])) + 1
			i += 2
		case b == 0xc9: // ext32
			if i+4 > len(data) {
				return errors.New("truncated msgpack: ext32 length bytes missing")
			}
			skip = (int(data[i])<<24 | int(data[i+1])<<16 | int(data[i+2])<<8 | int(data[i+3])) + 1
			i += 4

		// arrays
		case b >= 0x90 && b <= 0x9f: // fixarray
			children = int(b & 0x0f)
		case b == 0xdc: // array16
			if i+2 > len(data) {
				return errors.New("truncated msgpack: array16 length bytes missing")
			}
			children = int(data[i])<<8 | int(data[i+1])
			i += 2
		case b == 0xdd: // array32
			if i+4 > len(data) {
				return errors.New("truncated msgpack: array32 length bytes missing")
			}
			children = int(data[i])<<24 | int(data[i+1])<<16 | int(data[i+2])<<8 | int(data[i+3])
			i += 4

		// maps: each entry is a key-value pair → 2 values per entry
		case b >= 0x80 && b <= 0x8f: // fixmap
			children = int(b&0x0f) * 2
		case b == 0xde: // map16
			if i+2 > len(data) {
				return errors.New("truncated msgpack: map16 length bytes missing")
			}
			children = (int(data[i])<<8 | int(data[i+1])) * 2
			i += 2
		case b == 0xdf: // map32
			if i+4 > len(data) {
				return errors.New("truncated msgpack: map32 length bytes missing")
			}
			children = (int(data[i])<<24 | int(data[i+1])<<16 | int(data[i+2])<<8 | int(data[i+3])) * 2
			i += 4

		default:
			return fmt.Errorf("unknown msgpack format byte 0x%02x at offset %d", b, i-1)
		}

		// Advance past inline scalar payload bytes.
		if skip > 0 {
			if i+skip > len(data) {
				return fmt.Errorf("msgpack payload truncated at offset %d: need %d more bytes", i, skip)
			}
			i += skip
		}

		if children > 0 {
			// Non-empty container: push a new depth level.
			if len(remaining) >= msgpackMaxNestingDepth {
				return fmt.Errorf("msgpack nesting depth exceeds limit of %d", msgpackMaxNestingDepth)
			}
			remaining = append(remaining, children)
		} else {
			// Leaf or empty container: one value consumed; close any finished levels.
			for len(remaining) > 0 {
				top := len(remaining) - 1
				remaining[top]--
				if remaining[top] > 0 {
					break
				}
				remaining = remaining[:top] // pop finished level; loop to decrement parent
			}
		}
	}

	return nil
}

// decodeData attempts to decode message data as JSON or MessagePack.
func decodeData(data []byte) (map[string]any, error) {
	var result map[string]any

	// Try JSON first.  encoding/json enforces an ~10000-level nesting cap
	// internally, so this path is safe against deeply-nested payloads.
	if err := json.Unmarshal(data, &result); err == nil {
		return result, nil
	}

	// Guard against deeply-nested msgpack payloads before calling Unmarshal.
	// msgpack.Unmarshal recurses once per nesting level when decoding into
	// interface{}, with no built-in depth cap.  A crafted ~1 MB payload can
	// exhaust the goroutine stack and kill the process; recover() cannot
	// intercept a fatal stack-overflow.  checkMsgpackNestingDepth scans the
	// bytes iteratively (no recursion) and rejects inputs that exceed
	// msgpackMaxNestingDepth.  A rejected message is treated as a permanent
	// decode failure: the caller logs the error and ACKs the message so it is
	// not redelivered.
	if err := checkMsgpackNestingDepth(data); err != nil {
		return nil, fmt.Errorf("msgpack depth check failed: %w", err)
	}

	// Try MessagePack.
	if err := msgpack.Unmarshal(data, &result); err == nil {
		return result, nil
	}

	// If both fail, return the JSON error for a consistent error type.
	return nil, json.Unmarshal(data, &result)
}
