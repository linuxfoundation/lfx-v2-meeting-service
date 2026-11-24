// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/eventing"
	infraNATS "github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/nats"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

// EventProcessor manages the lifecycle of event processing via NATS JetStream
type EventProcessor struct {
	nc           *nats.Conn
	js           jetstream.JetStream
	consumer     jetstream.Consumer
	publisher    domain.EventPublisher
	userLookup   domain.V1UserLookup
	idMapper     domain.IDMapper
	v1ObjectsKV  jetstream.KeyValue
	v1MappingsKV jetstream.KeyValue
	logger       *slog.Logger
	config       eventing.Config
	handlers     *EventHandlers
}

// NewEventProcessor creates a new event processor
func NewEventProcessor(config eventing.Config, idMapper domain.IDMapper, logger *slog.Logger, inviteCfg InviteFeatureConfig) (*EventProcessor, error) {
	// Connect to NATS
	nc, err := nats.Connect(config.NATSURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Get JetStream context
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create jetstream context: %w", err)
	}

	// Get v1-objects KV bucket (for lookups)
	v1ObjectsKV, err := js.KeyValue(context.Background(), "v1-objects")
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to get v1-objects KV bucket: %w", err)
	}

	// Get or create v1-mappings KV bucket (for tracking synced items)
	v1MappingsKV, err := js.CreateKeyValue(context.Background(), jetstream.KeyValueConfig{
		Bucket:      config.V1MappingsBucketName,
		Description: "Stores mappings of v1 objects synced to v2",
		TTL:         0, // No expiration
	})
	if err != nil {
		// If bucket already exists, just get it
		v1MappingsKV, err = js.KeyValue(context.Background(), config.V1MappingsBucketName)
		if err != nil {
			nc.Close()
			return nil, fmt.Errorf("failed to get/create v1-mappings KV bucket: %w", err)
		}
	}

	// Create publisher
	publisher, err := eventing.NewNATSPublisher(nc, logger)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create publisher: %w", err)
	}

	// Create user lookup
	userLookup := eventing.NewNATSUserLookup(v1ObjectsKV, logger)

	// Create project slug lookup
	projectLookup := eventing.NewNATSProjectLookup(nc)

	// Create event handlers, with optional invite feature wired in.
	handlerOpts := []EventHandlersOption{}
	if inviteCfg.Enabled {
		inviteSender := infraNATS.NewInviteSender(nc, logger)
		userReader := infraNATS.NewUserReader(nc, logger)
		handlerOpts = append(handlerOpts, WithInviteFeature(inviteSender, userReader, inviteCfg.SelfServeBaseURL))
	}
	handlers := NewEventHandlers(publisher, userLookup, idMapper, projectLookup, v1ObjectsKV, v1MappingsKV, logger, handlerOpts...)

	ep := &EventProcessor{
		nc:           nc,
		js:           js,
		publisher:    publisher,
		userLookup:   userLookup,
		idMapper:     idMapper,
		v1ObjectsKV:  v1ObjectsKV,
		v1MappingsKV: v1MappingsKV,
		logger:       logger,
		config:       config,
		handlers:     handlers,
	}

	return ep, nil
}

// Start begins processing events from the NATS JetStream.
// If the consumer is deleted on the server at runtime, it is automatically
// recreated and consumption resumes without requiring a service restart.
func (ep *EventProcessor) Start(ctx context.Context) error {
	ep.logger.InfoContext(ctx, "starting event processor", "consumer", ep.config.ConsumerName)

	for {
		if err := ep.setupConsumerWithRetry(ctx); err != nil {
			return fmt.Errorf("failed to setup consumer: %w", err)
		}

		consumerDeleted := make(chan struct{})

		consumeCtx, err := ep.consumer.Consume(ep.msgHandler(ctx), jetstream.ConsumeErrHandler(
			func(_ jetstream.ConsumeContext, err error) {
				if errors.Is(err, jetstream.ErrConsumerDeleted) {
					ep.logger.WarnContext(ctx, "consumer was deleted on the server, will recreate")
					close(consumerDeleted)
					return
				}
				ep.logger.With(logging.ErrKey, err).WarnContext(ctx, "consumer error")
			},
		))
		if err != nil {
			return fmt.Errorf("failed to start consuming: %w", err)
		}

		select {
		case <-ctx.Done():
			ep.logger.InfoContext(ctx, "context cancelled, stopping consumer")
			consumeCtx.Stop()
			return nil
		case <-consumerDeleted:
			// consumeCtx is already stopped by the terminal error — just loop and recreate.
			ep.logger.InfoContext(ctx, "recreating consumer after deletion")
		}
	}
}

// msgHandler returns the JetStream message handler closure.
func (ep *EventProcessor) msgHandler(ctx context.Context) jetstream.MessageHandler {
	return func(msg jetstream.Msg) {
		defer func() {
			if r := recover(); r != nil {
				ep.logger.ErrorContext(ctx, "panic in event handler, treating the delivery as failed", "subject", msg.Subject(), "panic", r)
				// A panic is a failed delivery like any other: it follows the same
				// backoff and is logged when it exhausts its redeliveries.
				settleMessage(ctx, ep.logger, msg, true, ep.config.MaxDeliver)
			}
		}()

		switch checkRedelivery(ctx, ep.v1ObjectsKV, msg) {
		case redeliverySuperseded:
			// A newer revision of this key has its own delivery. Applying this older
			// one now would overwrite the newer state, for example re-granting access
			// after a delete, so acknowledge it without processing.
			ep.logger.InfoContext(ctx, "skipping redelivery of a superseded revision", "subject", msg.Subject())
			if err := msg.Ack(); err != nil {
				ep.logger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to ACK superseded message")
			}
			return
		case redeliveryUnknown:
			// The current revision could not be read, so this message cannot be shown to
			// be current. Treat it as a failed delivery and try again later.
			settleMessage(ctx, ep.logger, msg, true, ep.config.MaxDeliver)
			return
		}

		shouldRetry := kvHandler(ctx, msg, ep.handlers)
		settleMessage(ctx, ep.logger, msg, shouldRetry, ep.config.MaxDeliver)
	}
}

// Stop gracefully stops the event processor
func (ep *EventProcessor) Stop(ctx context.Context) error {
	ep.logger.InfoContext(ctx, "stopping event processor")

	// Drain pending messages with timeout
	drainCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		if ep.nc != nil {
			done <- ep.nc.Drain()
		} else {
			done <- nil
		}
	}()

	var drainErr error
	select {
	case <-drainCtx.Done():
		ep.logger.WarnContext(ctx, "drain timeout exceeded, force closing")
	case drainErr = <-done:
		if drainErr != nil {
			ep.logger.WarnContext(ctx, "failed to drain NATS connection on shutdown", logging.ErrKey, drainErr)
		} else {
			ep.logger.InfoContext(ctx, "drain completed")
		}
	}

	// Close publisher
	if err := ep.publisher.Close(); err != nil {
		ep.logger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to close publisher")
	}

	// Close NATS connection
	if ep.nc != nil {
		ep.nc.Close()
	}

	ep.logger.InfoContext(ctx, "event processor stopped")
	return drainErr
}

// setupConsumerWithRetry calls setupConsumer with exponential backoff until it
// succeeds or the context is cancelled. This handles transient errors (e.g.
// context deadline exceeded) that can occur when recreating a deleted consumer.
func (ep *EventProcessor) setupConsumerWithRetry(ctx context.Context) error {
	delays := []time.Duration{1 * time.Second, 5 * time.Second, 15 * time.Second, 30 * time.Second}
	for attempt, maxAttempt := 0, len(delays); ; attempt++ {
		err := ep.setupConsumer(ctx)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		delay := delays[min(attempt, maxAttempt-1)]
		ep.logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to setup consumer, retrying",
			"attempt", attempt+1,
			"retry_in", delay,
		)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}

// setupConsumer creates or updates the durable consumer configuration
func (ep *EventProcessor) setupConsumer(ctx context.Context) error {
	consumerConfig := jetstream.ConsumerConfig{
		Name:           ep.config.ConsumerName,
		Durable:        ep.config.ConsumerName,
		Description:    "Meeting service KV bucket event consumer",
		FilterSubjects: ep.config.FilterSubjects,
		DeliverPolicy:  jetstream.DeliverLastPerSubjectPolicy, // Process latest version only
		AckPolicy:      jetstream.AckExplicitPolicy,           // Manual ACK required
		MaxDeliver:     ep.config.MaxDeliver,                  // Retry limit
		AckWait:        ep.config.AckWait,                     // ACK timeout
		MaxAckPending:  ep.config.MaxAckPending,               // Max in-flight messages
	}

	// Get or create consumer
	consumer, err := ep.js.CreateOrUpdateConsumer(ctx, ep.config.StreamName, consumerConfig)
	if err != nil {
		return fmt.Errorf("failed to create/update consumer: %w", err)
	}

	ep.consumer = consumer
	ep.logger.InfoContext(ctx, "consumer configured",
		"name", ep.config.ConsumerName,
		"stream", ep.config.StreamName,
		"filters", ep.config.FilterSubjects,
	)

	return nil
}

// retryDelays is the NAK delay applied after each failed delivery, indexed by
// delivery attempt (the first entry follows the first delivery). The schedule
// escalates so that an outage of a dependency such as the v1-sync-helper ID
// lookup, lasting tens of minutes, does not exhaust the redeliveries: with the
// default EVENT_MAX_DELIVER of 8 the seven delays below keep a message retrying
// for about 30 minutes. A MaxDeliver longer than the schedule reuses the last
// delay.
var retryDelays = []time.Duration{
	2 * time.Second,
	10 * time.Second,
	30 * time.Second,
	1 * time.Minute,
	5 * time.Minute,
	10 * time.Minute,
	15 * time.Minute,
}

// unknownAttemptRetryDelay is used when the delivery count is unavailable.
const unknownAttemptRetryDelay = 20 * time.Second

// kvGetter reads the current entry of a key. jetstream.KeyValue satisfies it.
type kvGetter interface {
	Get(ctx context.Context, key string) (jetstream.KeyValueEntry, error)
}

// redeliveryVerdict is the outcome of checkRedelivery.
type redeliveryVerdict int

const (
	// redeliveryCurrent means the message is still the latest state of its key,
	// or is not a redelivery, and should be processed.
	redeliveryCurrent redeliveryVerdict = iota
	// redeliverySuperseded means a newer revision of the key exists.
	redeliverySuperseded
	// redeliveryUnknown means the current revision could not be read.
	redeliveryUnknown
)

// checkRedelivery decides whether a redelivered message still describes the
// current state of its key. JetStream redelivers a failed message with its
// original payload, and with retries that span tens of minutes the key may have
// been updated or deleted since. The first delivery is always processed: only a
// redelivery can be stale. The message's stream sequence is the key's revision at
// the time it was written, so a higher current revision means it was superseded.
//
// A put whose key is now deleted is superseded by the delete. A delete whose key
// is still deleted is not: it is the current state and must still be applied.
func checkRedelivery(ctx context.Context, kv kvGetter, msg jetstream.Msg) redeliveryVerdict {
	if kv == nil {
		return redeliveryCurrent
	}
	metadata, err := msg.Metadata()
	if err != nil || metadata.NumDelivered <= 1 {
		return redeliveryCurrent
	}

	// Subject format: $KV.<bucket>.<key>. A malformed subject is left for kvHandler to reject.
	parts := strings.SplitN(msg.Subject(), ".", 3)
	if len(parts) < 3 || parts[2] == "" {
		return redeliveryCurrent
	}

	entry, err := kv.Get(ctx, parts[2])
	if err != nil {
		if !errors.Is(err, jetstream.ErrKeyNotFound) {
			return redeliveryUnknown
		}
		// The key is deleted now.
		if op := getOperation(msg); op == jetstream.KeyValueDelete || op == jetstream.KeyValuePurge {
			return redeliveryCurrent
		}
		return redeliverySuperseded
	}
	if entry.Revision() > metadata.Sequence.Stream {
		return redeliverySuperseded
	}
	return redeliveryCurrent
}

// getRetryDelay returns the delay before redelivery based on the delivery attempt.
func getRetryDelay(numDelivered uint64) time.Duration {
	if numDelivered == 0 {
		return unknownAttemptRetryDelay
	}
	idx := numDelivered - 1
	if idx >= uint64(len(retryDelays)) {
		return retryDelays[len(retryDelays)-1]
	}
	return retryDelays[idx]
}

// settleMessage acknowledges the message, or, when the handler asked for a retry,
// schedules a delayed redelivery. When the retry would exceed maxDeliver the
// event can never be delivered again: JetStream would otherwise discard it
// without a trace, so the drop is logged at error level and the message is
// terminated, which also frees its slot in the consumer's pending window.
// A maxDeliver of zero or less means unlimited deliveries, so the message is
// never treated as exhausted.
func settleMessage(ctx context.Context, logger *slog.Logger, msg jetstream.Msg, shouldRetry bool, maxDeliver int) {
	if !shouldRetry {
		if err := msg.Ack(); err != nil {
			logger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to ACK message")
		}
		return
	}

	var numDelivered uint64
	if metadata, err := msg.Metadata(); err != nil {
		logger.With(logging.ErrKey, err).WarnContext(ctx, "failed to get message metadata, using default retry delay")
	} else {
		numDelivered = metadata.NumDelivered
	}

	if maxDeliver > 0 && numDelivered >= uint64(maxDeliver) {
		logger.ErrorContext(ctx, "event dropped after max deliveries, it will not be indexed until it is reindexed",
			"subject", msg.Subject(),
			"num_delivered", numDelivered,
			"max_deliver", maxDeliver,
		)
		if err := msg.Term(); err != nil {
			logger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to terminate message")
		}
		return
	}

	delay := getRetryDelay(numDelivered)
	if err := msg.NakWithDelay(delay); err != nil {
		logger.With(logging.ErrKey, err).ErrorContext(ctx, "failed to NAK message")
	}
	logger.InfoContext(ctx, "message NAKed for retry", "subject", msg.Subject(), "delay", delay)
}
