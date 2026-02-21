// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import "time"

// Config holds the configuration for event processing via NATS JetStream
type Config struct {
	// NATSURL is the NATS server URL
	NATSURL string

	// ConsumerName is the name of the durable consumer
	ConsumerName string

	// StreamName is the name of the JetStream stream
	StreamName string

	// FilterSubject is the subject pattern to filter events (e.g., "$KV.v1-objects.>")
	FilterSubject string

	// MaxDeliver is the maximum number of delivery attempts
	MaxDeliver int

	// AckWait is the time to wait for acknowledgment before redelivery
	AckWait time.Duration

	// MaxAckPending is the maximum number of outstanding acknowledgments
	MaxAckPending int

	// V1MappingsBucketName is the name of the KV bucket for storing v1 mappings
	V1MappingsBucketName string
}
