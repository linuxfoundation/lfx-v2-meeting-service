// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/infrastructure/eventing"
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
func NewEventProcessor(config eventing.Config, idMapper domain.IDMapper, logger *slog.Logger) (*EventProcessor, error) {
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

	// Create event handlers
	handlers := NewEventHandlers(publisher, userLookup, idMapper, v1ObjectsKV, v1MappingsKV, logger)

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

// Start begins processing events from the NATS JetStream
func (ep *EventProcessor) Start(ctx context.Context) error {
	ep.logger.Info("starting event processor", "consumer", ep.config.ConsumerName)

	// Setup consumer
	if err := ep.setupConsumer(ctx); err != nil {
		return fmt.Errorf("failed to setup consumer: %w", err)
	}

	// Consume messages
	consumeCtx, err := ep.consumer.Consume(func(msg jetstream.Msg) {
		// Process message in KV handler
		shouldRetry := kvHandler(ctx, msg, ep.handlers)

		// Handle acknowledgment
		if shouldRetry {
			// NAK with delay for retry
			metadata, _ := msg.Metadata()
			delay := getRetryDelay(metadata.NumDelivered)

			if err := msg.NakWithDelay(delay); err != nil {
				ep.logger.With(logging.ErrKey, err).Error("failed to NAK message")
			}
			ep.logger.Info("message NAKed for retry", "subject", msg.Subject(), "delay", delay)
		} else {
			// ACK - message processed successfully or permanently failed
			if err := msg.Ack(); err != nil {
				ep.logger.With(logging.ErrKey, err).Error("failed to ACK message")
			}
		}
	})
	if err != nil {
		return fmt.Errorf("failed to start consuming: %w", err)
	}

	// Wait for context cancellation
	<-ctx.Done()
	ep.logger.Info("context cancelled, stopping consumer")

	// Stop consuming
	consumeCtx.Stop()

	return nil
}

// Stop gracefully stops the event processor
func (ep *EventProcessor) Stop(ctx context.Context) error {
	ep.logger.Info("stopping event processor")

	// Drain pending messages with timeout
	drainCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		if ep.nc != nil {
			ep.nc.Drain()
		}
		close(done)
	}()

	select {
	case <-drainCtx.Done():
		ep.logger.Warn("drain timeout exceeded, force closing")
	case <-done:
		ep.logger.Info("drain completed")
	}

	// Close publisher
	if err := ep.publisher.Close(); err != nil {
		ep.logger.With(logging.ErrKey, err).Error("failed to close publisher")
	}

	// Close NATS connection
	if ep.nc != nil {
		ep.nc.Close()
	}

	ep.logger.Info("event processor stopped")
	return nil
}

// setupConsumer creates or updates the durable consumer configuration
func (ep *EventProcessor) setupConsumer(ctx context.Context) error {
	consumerConfig := jetstream.ConsumerConfig{
		Name:          ep.config.ConsumerName,
		Durable:       ep.config.ConsumerName,
		Description:   "Meeting service KV bucket event consumer",
		FilterSubject: ep.config.FilterSubject,
		DeliverPolicy: jetstream.DeliverLastPerSubjectPolicy, // Process latest version only
		AckPolicy:     jetstream.AckExplicitPolicy,           // Manual ACK required
		MaxDeliver:    ep.config.MaxDeliver,                  // Retry limit
		AckWait:       ep.config.AckWait,                     // ACK timeout
		MaxAckPending: ep.config.MaxAckPending,               // Max in-flight messages
	}

	// Get or create consumer
	consumer, err := ep.js.CreateOrUpdateConsumer(ctx, ep.config.StreamName, consumerConfig)
	if err != nil {
		return fmt.Errorf("failed to create/update consumer: %w", err)
	}

	ep.consumer = consumer
	ep.logger.Info("consumer configured",
		"name", ep.config.ConsumerName,
		"stream", ep.config.StreamName,
		"filter", ep.config.FilterSubject,
	)

	return nil
}

// getRetryDelay returns the delay duration based on delivery attempt
func getRetryDelay(numDelivered uint64) time.Duration {
	switch numDelivered {
	case 1:
		return 2 * time.Second
	case 2:
		return 10 * time.Second
	default:
		return 20 * time.Second
	}
}
