// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHasRecordingForPastMeeting(t *testing.T) {
	const moid = "meeting-occurrence-1"
	recordingKey := "itx-zoom-past-meetings-recordings." + moid

	tests := []struct {
		name         string
		moid         string
		setupObjects func(kv *mockKeyValue)
		want         bool
		wantErr      bool
	}{
		{
			name:         "empty meeting_and_occurrence_id short-circuits without KV lookup",
			moid:         "",
			setupObjects: func(_ *mockKeyValue) {},
			want:         false,
			wantErr:      false,
		},
		{
			name: "missing recording object is a permanent miss",
			moid: moid,
			setupObjects: func(kv *mockKeyValue) {
				kv.On("Get", mock.Anything, recordingKey).Return(nil, jetstream.ErrKeyNotFound)
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "recording with a session share_url is available",
			moid: moid,
			setupObjects: func(kv *mockKeyValue) {
				payload := []byte(`{"meeting_and_occurrence_id":"` + moid + `","sessions":[{"uuid":"s1","share_url":"https://zoom.us/rec/share/abc","total_size":1024}]}`)
				kv.On("Get", mock.Anything, recordingKey).Return(mockKeyValueEntry{key: recordingKey, value: payload}, nil)
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "recording whose sessions all lack a share_url is not available",
			moid: moid,
			setupObjects: func(kv *mockKeyValue) {
				payload := []byte(`{"meeting_and_occurrence_id":"` + moid + `","sessions":[{"uuid":"s1","share_url":"","total_size":1024}]}`)
				kv.On("Get", mock.Anything, recordingKey).Return(mockKeyValueEntry{key: recordingKey, value: payload}, nil)
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "largest session has a share_url even when a smaller session lacks one",
			moid: moid,
			setupObjects: func(kv *mockKeyValue) {
				payload := []byte(`{"meeting_and_occurrence_id":"` + moid + `","sessions":[{"uuid":"s1","share_url":"","total_size":10},{"uuid":"s2","share_url":"https://zoom.us/rec/share/big","total_size":2048}]}`)
				kv.On("Get", mock.Anything, recordingKey).Return(mockKeyValueEntry{key: recordingKey, value: payload}, nil)
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "smaller-session-only share_url is not available (parity: card surfaces only the largest session)",
			moid: moid,
			setupObjects: func(kv *mockKeyValue) {
				payload := []byte(`{"meeting_and_occurrence_id":"` + moid + `","sessions":[{"uuid":"s1","share_url":"https://zoom.us/rec/share/small","total_size":10},{"uuid":"s2","share_url":"","total_size":2048}]}`)
				kv.On("Get", mock.Anything, recordingKey).Return(mockKeyValueEntry{key: recordingKey, value: payload}, nil)
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "recording with no sessions is not available",
			moid: moid,
			setupObjects: func(kv *mockKeyValue) {
				payload := []byte(`{"meeting_and_occurrence_id":"` + moid + `"}`)
				kv.On("Get", mock.Anything, recordingKey).Return(mockKeyValueEntry{key: recordingKey, value: payload}, nil)
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "transient KV fetch error returns an error for retry",
			moid: moid,
			setupObjects: func(kv *mockKeyValue) {
				kv.On("Get", mock.Anything, recordingKey).Return(nil, errors.New("connection reset"))
			},
			want:    false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objectsKV := &mockKeyValue{}
			tt.setupObjects(objectsKV)

			got, err := hasRecordingForPastMeeting(context.Background(), tt.moid, objectsKV, slog.Default())

			if tt.wantErr {
				assert.Error(t, err)
				assert.True(t, isTransientError(err), "recording lookup errors must be classified transient so the update retries")
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
			objectsKV.AssertExpectations(t)
		})
	}
}
