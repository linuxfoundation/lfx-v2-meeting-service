// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeyBuilder_EntityKey(t *testing.T) {
	kb := NewKeyBuilder("")

	tests := []struct {
		name       string
		entityType string
		uid        string
		want       string
	}{
		{
			name:       "registrant key",
			entityType: KeyPrefixRegistrant,
			uid:        "abc-123",
			want:       "registrant/abc-123",
		},
		{
			name:       "meeting key",
			entityType: KeyPrefixMeeting,
			uid:        "def-456",
			want:       "meeting/def-456",
		},
		{
			name:       "past meeting key",
			entityType: KeyPrefixPastMeeting,
			uid:        "ghi-789",
			want:       "past-meeting/ghi-789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := kb.EntityKey(tt.entityType, tt.uid)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestKeyBuilder_EntityKeyEncoded(t *testing.T) {
	kb := NewKeyBuilder("")

	tests := []struct {
		name       string
		entityType string
		uid        string
	}{
		{
			name:       "registrant key encoded",
			entityType: KeyPrefixRegistrant,
			uid:        "abc-123",
		},
		{
			name:       "meeting key encoded with special chars",
			entityType: KeyPrefixMeeting,
			uid:        "def/456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := kb.EntityKeyEncoded(tt.entityType, tt.uid)

			// Verify we can decode it back
			decoded, err := kb.DecodeKey(encoded)
			assert.NoError(t, err)

			// Decoded should match the original pattern
			expected := "/" + tt.entityType + "/" + tt.uid
			assert.Equal(t, expected, decoded)
		})
	}
}

func TestKeyBuilder_IndexKey(t *testing.T) {
	kb := NewKeyBuilder("")

	tests := []struct {
		name       string
		indexType  string
		indexValue string
		entityUID  string
		want       string
	}{
		{
			name:       "meeting index",
			indexType:  KeyPrefixIndexMeeting,
			indexValue: "meeting-123",
			entityUID:  "registrant-456",
			want:       "index/meeting/meeting-123/registrant-456",
		},
		{
			name:       "email index",
			indexType:  KeyPrefixIndexEmail,
			indexValue: "test@example.com",
			entityUID:  "registrant-789",
			want:       "index/email/test@example.com/registrant-789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := kb.IndexKey(tt.indexType, tt.indexValue, tt.entityUID)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestKeyBuilder_IndexKeyEncoded(t *testing.T) {
	kb := NewKeyBuilder("")

	tests := []struct {
		name       string
		indexType  string
		indexValue string
		entityUID  string
	}{
		{
			name:       "email index with special chars",
			indexType:  KeyPrefixIndexEmail,
			indexValue: "user+tag@example.com",
			entityUID:  "registrant-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := kb.IndexKeyEncoded(tt.indexType, tt.indexValue, tt.entityUID)

			// Verify we can decode it back
			decoded, err := kb.DecodeKey(encoded)
			assert.NoError(t, err)

			// Decoded should contain our values
			assert.Contains(t, decoded, tt.indexType)
			assert.Contains(t, decoded, tt.indexValue)
			assert.Contains(t, decoded, tt.entityUID)
		})
	}
}

func TestKeyBuilder_EncodeKey(t *testing.T) {
	kb := NewKeyBuilder("")

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "simple key",
			key:     "test/key",
			wantErr: false,
		},
		{
			name:    "key with special chars",
			key:     "test/key/with/slashes",
			wantErr: false,
		},
		{
			name:    "key with email",
			key:     "registrant/user@example.com",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := kb.EncodeKey(tt.key)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, encoded)

				// Verify encoded key can be decoded back
				decoded, err := kb.DecodeKey(encoded)
				assert.NoError(t, err)

				// Add leading slash if not present in original (encodeKey behavior)
				expectedDecoded := tt.key
				if expectedDecoded[0] != '/' {
					expectedDecoded = "/" + expectedDecoded
				}
				assert.Equal(t, expectedDecoded, decoded)
			}
		})
	}
}

func TestKeyBuilder_DecodeKey(t *testing.T) {
	kb := NewKeyBuilder("")

	tests := []struct {
		name     string
		key      string
		expected string
		wantErr  bool
	}{
		{
			name:     "decode base64 encoded key",
			key:      "dGVzdA==.a2V5",
			expected: "/test/key",
			wantErr:  false,
		},
		{
			name:    "invalid base64",
			key:     "not-valid-base64!@#",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := kb.DecodeKey(tt.key)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, decoded)
			}
		})
	}
}
