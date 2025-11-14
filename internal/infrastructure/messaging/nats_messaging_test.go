// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/mock"
)

func TestMessageBuilder_publish(t *testing.T) {
	tests := []struct {
		name         string
		publishError error
		subject      string
		data         []byte
		expectError  bool
	}{
		{
			name:         "successful send",
			publishError: nil,
			subject:      "test.subject",
			data:         []byte("test data"),
			expectError:  false,
		},
		{
			name:         "publish error",
			publishError: errors.New("publish failed"),
			subject:      "test.subject",
			data:         []byte("test data"),
			expectError:  true,
		},
		{
			name:         "disconnected",
			publishError: nil,
			subject:      "test.subject",
			data:         []byte("test data"),
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := new(MockNATSConn)
			mockConn.On("Publish", tt.subject, tt.data).Return(tt.publishError)

			builder := &MessageBuilder{
				NatsConn: mockConn,
			}

			ctx := context.Background()
			err := builder.publish(ctx, tt.subject, tt.data)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}

			mockConn.AssertExpectations(t)
		})
	}
}

func TestMessageBuilder_setIndexerTags(t *testing.T) {
	builder := &MessageBuilder{}

	// Test setIndexerTags with empty input
	tags := builder.setIndexerTags()
	if len(tags) != 0 {
		t.Errorf("expected empty tags slice, got %d tags", len(tags))
	}

	// Test setIndexerTags with some tags
	tags = builder.setIndexerTags("tag1", "tag2", "tag3")
	expectedTags := []string{"tag1", "tag2", "tag3"}

	if len(tags) != len(expectedTags) {
		t.Errorf("expected %d tags, got %d", len(expectedTags), len(tags))
	}

	for i, expectedTag := range expectedTags {
		if i >= len(tags) {
			t.Errorf("missing tag at index %d: expected %q", i, expectedTag)
		} else if tags[i] != expectedTag {
			t.Errorf("tag at index %d: expected %q, got %q", i, expectedTag, tags[i])
		}
	}
}

// Core function tests

func TestMessageBuilder_request(t *testing.T) {
	ctx := context.Background()

	t.Run("successful request", func(t *testing.T) {
		mockConn := new(MockNATSConn)
		mockConn.On("Request", "test.request", []byte("request data"), 5*time.Second).Return(&nats.Msg{
			Subject: "test.request",
			Data:    []byte(`{"result":"success"}`),
		}, nil)

		builder := &MessageBuilder{
			NatsConn: mockConn,
		}

		msg, err := builder.request(ctx, "test.request", []byte("request data"), 5*time.Second)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
		if msg == nil {
			t.Fatal("expected non-nil message")
		}
		if string(msg.Data) != `{"result":"success"}` {
			t.Errorf("expected response data %q, got %q", `{"result":"success"}`, string(msg.Data))
		}

		mockConn.AssertExpectations(t)
	})

	t.Run("request with error", func(t *testing.T) {
		expectedErr := nats.ErrTimeout
		mockConn := new(MockNATSConn)
		mockConn.On("Request", "test.request", []byte("request data"), 5*time.Second).Return(nil, expectedErr)

		builder := &MessageBuilder{
			NatsConn: mockConn,
		}

		msg, err := builder.request(ctx, "test.request", []byte("request data"), 5*time.Second)
		if err != expectedErr {
			t.Errorf("expected error %v, got: %v", expectedErr, err)
		}
		if msg != nil {
			t.Errorf("expected nil message on error, got: %v", msg)
		}

		mockConn.AssertExpectations(t)
	})

	t.Run("request with disconnected connection", func(t *testing.T) {
		mockConn := new(MockNATSConn)
		mockConn.On("Request", "test.request", []byte("request data"), 5*time.Second).Return(nil, nats.ErrDisconnected)

		builder := &MessageBuilder{
			NatsConn: mockConn,
		}

		msg, err := builder.request(ctx, "test.request", []byte("request data"), 5*time.Second)
		if err != nats.ErrDisconnected {
			t.Errorf("expected disconnected error, got: %v", err)
		}
		if msg != nil {
			t.Errorf("expected nil message on error, got: %v", msg)
		}

		mockConn.AssertExpectations(t)
	})
}

func TestMessageBuilder_sendIndexerMessage(t *testing.T) {
	t.Run("send created action with authorization", func(t *testing.T) {
		mockConn := new(MockNATSConn)

		// Use mock.MatchedBy to capture and verify the published message
		mockConn.On("Publish", "test.subject", mock.MatchedBy(func(data []byte) bool {
			var indexerMsg models.MeetingIndexerMessage
			err := json.Unmarshal(data, &indexerMsg)
			if err != nil {
				t.Errorf("failed to unmarshal message: %v", err)
				return false
			}

			if indexerMsg.Action != models.ActionCreated {
				t.Errorf("expected action %v, got %v", models.ActionCreated, indexerMsg.Action)
				return false
			}
			if indexerMsg.Headers[constants.AuthorizationHeader] != "Bearer test-token" {
				t.Errorf("expected authorization header %q, got %q", "Bearer test-token", indexerMsg.Headers[constants.AuthorizationHeader])
				return false
			}
			if indexerMsg.Headers[constants.XOnBehalfOfHeader] != "test-user" {
				t.Errorf("expected on-behalf-of header %q, got %q", "test-user", indexerMsg.Headers[constants.XOnBehalfOfHeader])
				return false
			}
			if len(indexerMsg.Tags) != 2 {
				t.Errorf("expected 2 tags, got %d", len(indexerMsg.Tags))
				return false
			}
			return true
		})).Return(nil)

		builder := &MessageBuilder{
			NatsConn: mockConn,
		}

		ctx := context.WithValue(context.Background(), constants.AuthorizationContextID, "Bearer test-token")
		ctx = context.WithValue(ctx, constants.PrincipalContextID, "test-user")

		data := map[string]string{"uid": "test-123", "title": "Test Meeting"}
		dataBytes, _ := json.Marshal(data)
		tags := []string{"tag1", "tag2"}

		err := builder.sendIndexerMessage(ctx, "test.subject", models.ActionCreated, dataBytes, tags)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}

		mockConn.AssertExpectations(t)
	})

	t.Run("send deleted action without authorization", func(t *testing.T) {
		mockConn := new(MockNATSConn)
		uid := "meeting-123"

		// Use mock.MatchedBy to capture and verify the published message
		mockConn.On("Publish", "test.subject", mock.MatchedBy(func(data []byte) bool {
			var indexerMsg models.MeetingIndexerMessage
			err := json.Unmarshal(data, &indexerMsg)
			if err != nil {
				t.Errorf("failed to unmarshal message: %v", err)
				return false
			}

			if indexerMsg.Action != models.ActionDeleted {
				t.Errorf("expected action %v, got %v", models.ActionDeleted, indexerMsg.Action)
				return false
			}
			// Should have fallback authorization for system-generated events
			if indexerMsg.Headers[constants.AuthorizationHeader] != "Bearer meeting-service" {
				t.Errorf("expected fallback authorization header %q, got %q", "Bearer meeting-service", indexerMsg.Headers[constants.AuthorizationHeader])
				return false
			}
			// Payload should be the UID string
			if dataStr, ok := indexerMsg.Data.(string); !ok || dataStr != uid {
				t.Errorf("expected data %q, got %v", uid, indexerMsg.Data)
				return false
			}
			return true
		})).Return(nil)

		builder := &MessageBuilder{
			NatsConn: mockConn,
		}

		ctx := context.Background()
		tags := []string{"meeting_uid:meeting-123"}

		err := builder.sendIndexerMessage(ctx, "test.subject", models.ActionDeleted, []byte(uid), tags)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}

		mockConn.AssertExpectations(t)
	})

	t.Run("send with invalid JSON data", func(t *testing.T) {
		mockConn := new(MockNATSConn)
		// No publish expected for invalid JSON

		builder := &MessageBuilder{
			NatsConn: mockConn,
		}

		ctx := context.Background()
		invalidJSON := []byte("{invalid json")
		tags := []string{"tag1"}

		err := builder.sendIndexerMessage(ctx, "test.subject", models.ActionCreated, invalidJSON, tags)
		if err == nil {
			t.Error("expected error for invalid JSON, got nil")
		}

		mockConn.AssertExpectations(t)
	})

	t.Run("send with publish error", func(t *testing.T) {
		expectedErr := errors.New("publish failed")
		mockConn := new(MockNATSConn)
		mockConn.On("Publish", "test.subject", mock.Anything).Return(expectedErr)

		builder := &MessageBuilder{
			NatsConn: mockConn,
		}

		ctx := context.Background()
		data := map[string]string{"uid": "test-123"}
		dataBytes, _ := json.Marshal(data)
		tags := []string{"tag1"}

		err := builder.sendIndexerMessage(ctx, "test.subject", models.ActionCreated, dataBytes, tags)
		if err == nil {
			t.Error("expected publish error, got nil")
		}

		mockConn.AssertExpectations(t)
	})
}

func TestMessageBuilder_prepareMeetingBaseForIndexing(t *testing.T) {
	builder := &MessageBuilder{}

	t.Run("clears sensitive fields", func(t *testing.T) {
		meeting := models.MeetingBase{
			UID:     "meeting-123",
			Title:   "Test Meeting",
			JoinURL: "https://zoom.us/j/123456?pwd=secret",
		}

		result := builder.prepareMeetingBaseForIndexing(meeting)

		if result.JoinURL != "" {
			t.Errorf("expected JoinURL to be cleared, got %q", result.JoinURL)
		}
		if result.UID != "meeting-123" {
			t.Errorf("expected UID to be preserved, got %q", result.UID)
		}
		if result.Title != "Test Meeting" {
			t.Errorf("expected Title to be preserved, got %q", result.Title)
		}
	})

	t.Run("handles empty meeting", func(t *testing.T) {
		meeting := models.MeetingBase{}
		result := builder.prepareMeetingBaseForIndexing(meeting)

		if result.JoinURL != "" {
			t.Errorf("expected JoinURL to be empty, got %q", result.JoinURL)
		}
	})

	t.Run("does not modify original meeting", func(t *testing.T) {
		meeting := models.MeetingBase{
			UID:     "meeting-123",
			JoinURL: "https://zoom.us/j/123456?pwd=secret",
		}
		originalJoinURL := meeting.JoinURL

		_ = builder.prepareMeetingBaseForIndexing(meeting)

		if meeting.JoinURL != originalJoinURL {
			t.Errorf("original meeting should not be modified, JoinURL changed from %q to %q", originalJoinURL, meeting.JoinURL)
		}
	})
}

// Request-reply function tests

func TestMessageBuilder_GetCommitteeName(t *testing.T) {
	ctx := context.Background()

	t.Run("successful lookup", func(t *testing.T) {
		mockConn := new(MockNATSConn)
		mockConn.On("Request", models.CommitteeGetNameSubject, mock.Anything, mock.Anything).Return(&nats.Msg{
			Subject: models.CommitteeGetNameSubject,
			Data:    []byte("Technical Committee"),
		}, nil)

		builder := &MessageBuilder{NatsConn: mockConn}

		name, err := builder.GetCommitteeName(ctx, "committee-123")
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
		if name != "Technical Committee" {
			t.Errorf("expected name %q, got %q", "Technical Committee", name)
		}

		mockConn.AssertExpectations(t)
	})

	t.Run("committee not found - JSON error response", func(t *testing.T) {
		mockConn := new(MockNATSConn)
		mockConn.On("Request", models.CommitteeGetNameSubject, mock.Anything, mock.Anything).Return(&nats.Msg{
			Subject: models.CommitteeGetNameSubject,
			Data:    []byte(`{"error":"committee not found"}`),
		}, nil)

		builder := &MessageBuilder{NatsConn: mockConn}

		name, err := builder.GetCommitteeName(ctx, "nonexistent-committee")
		if err == nil {
			t.Error("expected error for committee not found, got nil")
		}
		if name != "" {
			t.Errorf("expected empty name, got %q", name)
		}
		var committeeErr *CommitteeNotFoundError
		if !errors.As(err, &committeeErr) {
			t.Errorf("expected CommitteeNotFoundError, got %T", err)
		}

		mockConn.AssertExpectations(t)
	})

	t.Run("request timeout", func(t *testing.T) {
		mockConn := new(MockNATSConn)
		mockConn.On("Request", models.CommitteeGetNameSubject, mock.Anything, mock.Anything).Return(nil, nats.ErrTimeout)

		builder := &MessageBuilder{NatsConn: mockConn}

		name, err := builder.GetCommitteeName(ctx, "committee-123")
		if err != nats.ErrTimeout {
			t.Errorf("expected timeout error, got: %v", err)
		}
		if name != "" {
			t.Errorf("expected empty name on error, got %q", name)
		}

		mockConn.AssertExpectations(t)
	})
}

func TestMessageBuilder_GetProjectName(t *testing.T) {
	ctx := context.Background()

	t.Run("successful lookup", func(t *testing.T) {
		mockConn := new(MockNATSConn)
		mockConn.On("Request", models.ProjectGetNameSubject, mock.Anything, mock.Anything).Return(&nats.Msg{
			Subject: models.ProjectGetNameSubject,
			Data:    []byte("LFX Platform"),
		}, nil)

		builder := &MessageBuilder{NatsConn: mockConn}

		name, err := builder.GetProjectName(ctx, "project-123")
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
		if name != "LFX Platform" {
			t.Errorf("expected name %q, got %q", "LFX Platform", name)
		}

		mockConn.AssertExpectations(t)
	})

	t.Run("project not found - empty response", func(t *testing.T) {
		mockConn := new(MockNATSConn)
		mockConn.On("Request", models.ProjectGetNameSubject, mock.Anything, mock.Anything).Return(&nats.Msg{
			Subject: models.ProjectGetNameSubject,
			Data:    []byte(""),
		}, nil)

		builder := &MessageBuilder{NatsConn: mockConn}

		name, err := builder.GetProjectName(ctx, "nonexistent-project")
		if err == nil {
			t.Error("expected error for project not found, got nil")
		}
		if name != "" {
			t.Errorf("expected empty name, got %q", name)
		}
		var projectErr *ProjectNotFoundError
		if !errors.As(err, &projectErr) {
			t.Errorf("expected ProjectNotFoundError, got %T", err)
		}

		mockConn.AssertExpectations(t)
	})

	t.Run("request error", func(t *testing.T) {
		mockConn := new(MockNATSConn)
		mockConn.On("Request", models.ProjectGetNameSubject, mock.Anything, mock.Anything).Return(nil, errors.New("network error"))

		builder := &MessageBuilder{NatsConn: mockConn}

		name, err := builder.GetProjectName(ctx, "project-123")
		if err == nil {
			t.Error("expected error, got nil")
		}
		if name != "" {
			t.Errorf("expected empty name on error, got %q", name)
		}

		mockConn.AssertExpectations(t)
	})
}

func TestMessageBuilder_GetProjectLogo(t *testing.T) {
	ctx := context.Background()

	t.Run("successful lookup", func(t *testing.T) {
		mockConn := new(MockNATSConn)
		mockConn.On("Request", models.ProjectGetLogoSubject, mock.Anything, mock.Anything).Return(&nats.Msg{
			Subject: models.ProjectGetLogoSubject,
			Data:    []byte("https://example.com/logo.png"),
		}, nil)

		builder := &MessageBuilder{NatsConn: mockConn}

		logo, err := builder.GetProjectLogo(ctx, "project-123")
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
		if logo != "https://example.com/logo.png" {
			t.Errorf("expected logo URL %q, got %q", "https://example.com/logo.png", logo)
		}

		mockConn.AssertExpectations(t)
	})

	t.Run("project logo not found - empty response", func(t *testing.T) {
		mockConn := new(MockNATSConn)
		mockConn.On("Request", models.ProjectGetLogoSubject, mock.Anything, mock.Anything).Return(&nats.Msg{
			Subject: models.ProjectGetLogoSubject,
			Data:    []byte(""),
		}, nil)

		builder := &MessageBuilder{NatsConn: mockConn}

		logo, err := builder.GetProjectLogo(ctx, "project-without-logo")
		if err == nil {
			t.Error("expected error for logo not found, got nil")
		}
		if logo != "" {
			t.Errorf("expected empty logo URL, got %q", logo)
		}
		var projectErr *ProjectNotFoundError
		if !errors.As(err, &projectErr) {
			t.Errorf("expected ProjectNotFoundError, got %T", err)
		}

		mockConn.AssertExpectations(t)
	})
}

func TestMessageBuilder_GetProjectSlug(t *testing.T) {
	ctx := context.Background()

	t.Run("successful lookup", func(t *testing.T) {
		mockConn := new(MockNATSConn)
		mockConn.On("Request", models.ProjectGetSlugSubject, mock.Anything, mock.Anything).Return(&nats.Msg{
			Subject: models.ProjectGetSlugSubject,
			Data:    []byte("lfx-platform"),
		}, nil)

		builder := &MessageBuilder{NatsConn: mockConn}

		slug, err := builder.GetProjectSlug(ctx, "project-123")
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
		if slug != "lfx-platform" {
			t.Errorf("expected slug %q, got %q", "lfx-platform", slug)
		}

		mockConn.AssertExpectations(t)
	})

	t.Run("project slug not found - empty response", func(t *testing.T) {
		mockConn := new(MockNATSConn)
		mockConn.On("Request", models.ProjectGetSlugSubject, mock.Anything, mock.Anything).Return(&nats.Msg{
			Subject: models.ProjectGetSlugSubject,
			Data:    []byte(""),
		}, nil)

		builder := &MessageBuilder{NatsConn: mockConn}

		slug, err := builder.GetProjectSlug(ctx, "project-without-slug")
		if err == nil {
			t.Error("expected error for slug not found, got nil")
		}
		if slug != "" {
			t.Errorf("expected empty slug, got %q", slug)
		}
		var projectErr *ProjectNotFoundError
		if !errors.As(err, &projectErr) {
			t.Errorf("expected ProjectNotFoundError, got %T", err)
		}

		mockConn.AssertExpectations(t)
	})
}

func TestMessageBuilder_EmailToUsernameLookup(t *testing.T) {
	ctx := context.Background()

	t.Run("successful lookup", func(t *testing.T) {
		mockConn := new(MockNATSConn)
		mockConn.On("Request", models.AuthEmailToUsernameLookupSubject, mock.Anything, mock.Anything).Return(&nats.Msg{
			Subject: models.AuthEmailToUsernameLookupSubject,
			Data:    []byte("johndoe"),
		}, nil)

		builder := &MessageBuilder{NatsConn: mockConn}

		username, err := builder.EmailToUsernameLookup(ctx, "john.doe@example.com")
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
		if username != "johndoe" {
			t.Errorf("expected username %q, got %q", "johndoe", username)
		}

		mockConn.AssertExpectations(t)
	})

	t.Run("user not found - JSON error response", func(t *testing.T) {
		mockConn := new(MockNATSConn)
		mockConn.On("Request", models.AuthEmailToUsernameLookupSubject, mock.Anything, mock.Anything).Return(&nats.Msg{
			Subject: models.AuthEmailToUsernameLookupSubject,
			Data:    []byte(`{"success":false,"error":"user not found"}`),
		}, nil)

		builder := &MessageBuilder{NatsConn: mockConn}

		username, err := builder.EmailToUsernameLookup(ctx, "nonexistent@example.com")
		if err == nil {
			t.Error("expected error for user not found, got nil")
		}
		if username != "" {
			t.Errorf("expected empty username, got %q", username)
		}

		mockConn.AssertExpectations(t)
	})

	t.Run("request error", func(t *testing.T) {
		mockConn := new(MockNATSConn)
		mockConn.On("Request", models.AuthEmailToUsernameLookupSubject, mock.Anything, mock.Anything).Return(nil, errors.New("auth service unavailable"))

		builder := &MessageBuilder{NatsConn: mockConn}

		username, err := builder.EmailToUsernameLookup(ctx, "test@example.com")
		if err == nil {
			t.Error("expected error, got nil")
		}
		if username != "" {
			t.Errorf("expected empty username on error, got %q", username)
		}

		mockConn.AssertExpectations(t)
	})
}

func TestMessageBuilder_GetCommitteeMembers(t *testing.T) {
	ctx := context.Background()

	t.Run("successful lookup", func(t *testing.T) {
		membersJSON := `[
			{"username":"alice","email":"alice@example.com","role":{"name":"chair"},"status":"active","voting":{"status":"active"},"appointed_by":"","uid":"member-1","first_name":"Alice","last_name":"Smith","job_title":""},
			{"username":"bob","email":"bob@example.com","role":{"name":"member"},"status":"active","voting":{"status":"active"},"appointed_by":"","uid":"member-2","first_name":"Bob","last_name":"Jones","job_title":""}
		]`
		mockConn := new(MockNATSConn)
		mockConn.On("Request", models.CommitteeListMembersSubject, mock.Anything, mock.Anything).Return(&nats.Msg{
			Subject: models.CommitteeListMembersSubject,
			Data:    []byte(membersJSON),
		}, nil)

		builder := &MessageBuilder{NatsConn: mockConn}

		members, err := builder.GetCommitteeMembers(ctx, "committee-123")
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
		if len(members) != 2 {
			t.Errorf("expected 2 members, got %d", len(members))
		}
		if len(members) > 0 && members[0].Username != "alice" {
			t.Errorf("expected first member username %q, got %q", "alice", members[0].Username)
		}

		mockConn.AssertExpectations(t)
	})

	t.Run("committee not found - JSON error response", func(t *testing.T) {
		mockConn := new(MockNATSConn)
		mockConn.On("Request", models.CommitteeListMembersSubject, mock.Anything, mock.Anything).Return(&nats.Msg{
			Subject: models.CommitteeListMembersSubject,
			Data:    []byte(`{"error":"committee not found"}`),
		}, nil)

		builder := &MessageBuilder{NatsConn: mockConn}

		members, err := builder.GetCommitteeMembers(ctx, "nonexistent-committee")
		if err == nil {
			t.Error("expected error for committee not found, got nil")
		}
		if members != nil {
			t.Errorf("expected nil members, got %d members", len(members))
		}
		var committeeErr *CommitteeNotFoundError
		if !errors.As(err, &committeeErr) {
			t.Errorf("expected CommitteeNotFoundError, got %T", err)
		}

		mockConn.AssertExpectations(t)
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		mockConn := new(MockNATSConn)
		mockConn.On("Request", models.CommitteeListMembersSubject, mock.Anything, mock.Anything).Return(&nats.Msg{
			Subject: models.CommitteeListMembersSubject,
			Data:    []byte(`{invalid json}`),
		}, nil)

		builder := &MessageBuilder{NatsConn: mockConn}

		members, err := builder.GetCommitteeMembers(ctx, "committee-123")
		if err == nil {
			t.Error("expected error for invalid JSON, got nil")
		}
		if members != nil {
			t.Errorf("expected nil members on error, got %d members", len(members))
		}

		mockConn.AssertExpectations(t)
	})

	t.Run("empty members list", func(t *testing.T) {
		mockConn := new(MockNATSConn)
		mockConn.On("Request", models.CommitteeListMembersSubject, mock.Anything, mock.Anything).Return(&nats.Msg{
			Subject: models.CommitteeListMembersSubject,
			Data:    []byte(`[]`),
		}, nil)

		builder := &MessageBuilder{NatsConn: mockConn}

		members, err := builder.GetCommitteeMembers(ctx, "committee-empty")
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
		if len(members) != 0 {
			t.Errorf("expected empty members list, got %d members", len(members))
		}

		mockConn.AssertExpectations(t)
	})

	t.Run("request error", func(t *testing.T) {
		mockConn := new(MockNATSConn)
		mockConn.On("Request", models.CommitteeListMembersSubject, mock.Anything, mock.Anything).Return(nil, nats.ErrTimeout)

		builder := &MessageBuilder{NatsConn: mockConn}

		members, err := builder.GetCommitteeMembers(ctx, "committee-123")
		if err != nats.ErrTimeout {
			t.Errorf("expected timeout error, got: %v", err)
		}
		if members != nil {
			t.Errorf("expected nil members on error, got %d members", len(members))
		}

		mockConn.AssertExpectations(t)
	})
}
