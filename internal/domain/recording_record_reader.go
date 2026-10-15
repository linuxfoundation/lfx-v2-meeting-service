// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import (
	"context"
	"errors"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// ErrRecordingRecordNotFound is returned by RecordingRecordReader when no recording
// record exists for the past meeting. Callers must treat only this error as absence;
// any other error is a failure to read.
var ErrRecordingRecordNotFound = errors.New("recording record not found")

// RecordingRecordReader reads a past meeting's recording record on demand. The
// past-meeting chat route uses it to find the CHAT files of a recording.
//
// A nil RecordingRecordReader means NATS was not configured at startup; routes
// that need it must then answer 503 rather than guess.
type RecordingRecordReader interface {
	// GetRecordingRecord returns the recording record keyed by meetingAndOccurrenceID.
	// It returns ErrRecordingRecordNotFound when there is none and an error of type
	// ErrorTypeUnavailable (transient read failure) or ErrorTypeInternal (the stored
	// record cannot be decoded) otherwise.
	GetRecordingRecord(ctx context.Context, meetingAndOccurrenceID string) (*models.RecordingRecord, error)
}
