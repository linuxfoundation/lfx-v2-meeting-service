// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package messaging

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/go-viper/mapstructure/v2"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
)

// INatsConn is a NATS connection interface needed for the [MeetingsService].
type INatsConn interface {
	IsConnected() bool
	Publish(subj string, data []byte) error
}

// MessageBuilder is the builder for the message and sends it to the NATS server.
type MessageBuilder struct {
	NatsConn INatsConn
}

// NewMessageBuilder creates a new MessageBuilder.
func NewMessageBuilder(natsConn INatsConn) *MessageBuilder {
	return &MessageBuilder{
		NatsConn: natsConn,
	}
}

// sendMessage sends the message to the NATS server.
func (m *MessageBuilder) sendMessage(ctx context.Context, subject string, data []byte) error {
	err := m.NatsConn.Publish(subject, data)
	if err != nil {
		slog.ErrorContext(ctx, "error sending message to NATS", logging.ErrKey, err, "subject", subject)
		return err
	}
	slog.DebugContext(ctx, "sent message to NATS", "subject", subject)
	return nil
}

// sendIndexerMessage sends the message to the NATS server for the indexer.
func (m *MessageBuilder) sendIndexerMessage(ctx context.Context, subject string, action models.MessageAction, data []byte, tags []string) error {
	headers := make(map[string]string)
	if authorization, ok := ctx.Value(constants.AuthorizationContextID).(string); ok {
		headers[constants.AuthorizationHeader] = authorization
	} else {
		// Fallback for system-generated events (webhooks, etc.) that don't have user auth context
		// This is just a dummy value so that the indexer service can still process the message,
		// given that it requires an authorization header.
		headers[constants.AuthorizationHeader] = "Bearer meeting-service"
	}
	if principal, ok := ctx.Value(constants.PrincipalContextID).(string); ok {
		headers[constants.XOnBehalfOfHeader] = principal
	}
	var payload any
	switch action {
	case models.ActionCreated, models.ActionUpdated:
		// The data should be a JSON object.
		var jsonData any
		if err := json.Unmarshal(data, &jsonData); err != nil {
			slog.ErrorContext(ctx, "error unmarshalling data into JSON", logging.ErrKey, err, "subject", subject)
			return err
		}

		// Decode the JSON data into a map[string]any since that is what the indexer expects.
		config := mapstructure.DecoderConfig{
			TagName: "json",
			Result:  &payload,
		}
		decoder, err := mapstructure.NewDecoder(&config)
		if err != nil {
			slog.ErrorContext(ctx, "error creating decoder", logging.ErrKey, err, "subject", subject)
			return err
		}
		err = decoder.Decode(jsonData)
		if err != nil {
			slog.ErrorContext(ctx, "error decoding data", logging.ErrKey, err, "subject", subject)
			return err
		}
	case models.ActionDeleted:
		// The data should just be a string of the UID being deleted.
		payload = string(data)
	}

	// TODO: use the model from the indexer service to keep the message body consistent.
	// Ticket https://linuxfoundation.atlassian.net/browse/LFXV2-147
	message := models.MeetingIndexerMessage{
		Action:  action,
		Headers: headers,
		Data:    payload,
		Tags:    tags,
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		slog.ErrorContext(ctx, "error marshalling message into JSON", logging.ErrKey, err, "subject", subject)
		return err
	}

	slog.DebugContext(ctx, "constructed indexer message",
		"subject", subject,
		"action", action,
		"tags_count", len(tags),
	)

	return m.sendMessage(ctx, subject, messageBytes)
}

// setIndexerTags sets the tags for the indexer.
func (m *MessageBuilder) setIndexerTags(tags ...string) []string {
	return tags
}

// SendIndexMeeting sends the message to the NATS server for the meeting indexing.
func (m *MessageBuilder) SendIndexMeeting(ctx context.Context, action models.MessageAction, data models.MeetingBase) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		slog.ErrorContext(ctx, "error marshalling data into JSON", logging.ErrKey, err)
		return err
	}

	tags := m.setIndexerTags(data.Tags()...)

	return m.sendIndexerMessage(ctx, models.IndexMeetingSubject, action, dataBytes, tags)
}

// SendDeleteIndexMeeting sends the message to the NATS server for the meeting indexing.
func (m *MessageBuilder) SendDeleteIndexMeeting(ctx context.Context, data string) error {
	return m.sendIndexerMessage(ctx, models.IndexMeetingSubject, models.ActionDeleted, []byte(data), nil)
}

// SendIndexMeetingSettings sends the message to the NATS server for the meeting settings indexing.
func (m *MessageBuilder) SendIndexMeetingSettings(ctx context.Context, action models.MessageAction, data models.MeetingSettings) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		slog.ErrorContext(ctx, "error marshalling data into JSON", logging.ErrKey, err)
		return err
	}

	tags := m.setIndexerTags(data.Tags()...)

	return m.sendIndexerMessage(ctx, models.IndexMeetingSettingsSubject, action, dataBytes, tags)
}

// SendDeleteIndexMeetingSettings sends the message to the NATS server for the meeting settings indexing.
func (m *MessageBuilder) SendDeleteIndexMeetingSettings(ctx context.Context, data string) error {
	return m.sendIndexerMessage(ctx, models.IndexMeetingSettingsSubject, models.ActionDeleted, []byte(data), nil)
}

// SendIndexMeeting sends the message to the NATS server for the meeting indexing.
func (m *MessageBuilder) SendIndexMeetingRegistrant(ctx context.Context, action models.MessageAction, data models.Registrant) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		slog.ErrorContext(ctx, "error marshalling data into JSON", logging.ErrKey, err)
		return err
	}

	tags := m.setIndexerTags(data.Tags()...)

	return m.sendIndexerMessage(ctx, models.IndexMeetingRegistrantSubject, action, dataBytes, tags)
}

// SendDeleteIndexMeetingRegistrant sends the message to the NATS server for the meeting registrant indexing.
func (m *MessageBuilder) SendDeleteIndexMeetingRegistrant(ctx context.Context, data string) error {
	return m.sendIndexerMessage(ctx, models.IndexMeetingRegistrantSubject, models.ActionDeleted, []byte(data), nil)
}

// SendIndexPastMeeting sends the message to the NATS server for the past meeting indexing.
func (m *MessageBuilder) SendIndexPastMeeting(ctx context.Context, action models.MessageAction, data models.PastMeeting) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		slog.ErrorContext(ctx, "error marshalling data into JSON", logging.ErrKey, err)
		return err
	}

	tags := m.setIndexerTags(data.Tags()...)

	return m.sendIndexerMessage(ctx, models.IndexPastMeetingSubject, action, dataBytes, tags)
}

// SendDeleteIndexPastMeeting sends the message to the NATS server for the past meeting indexing.
func (m *MessageBuilder) SendDeleteIndexPastMeeting(ctx context.Context, data string) error {
	return m.sendIndexerMessage(ctx, models.IndexPastMeetingSubject, models.ActionDeleted, []byte(data), nil)
}

// SendIndexPastMeetingParticipant sends the message to the NATS server for the past meeting participant indexing.
func (m *MessageBuilder) SendIndexPastMeetingParticipant(ctx context.Context, action models.MessageAction, data models.PastMeetingParticipant) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		slog.ErrorContext(ctx, "error marshalling data into JSON", logging.ErrKey, err)
		return err
	}

	tags := m.setIndexerTags(data.Tags()...)

	return m.sendIndexerMessage(ctx, models.IndexPastMeetingParticipantSubject, action, dataBytes, tags)
}

// SendDeleteIndexPastMeetingParticipant sends the message to the NATS server for the past meeting participant indexing.
func (m *MessageBuilder) SendDeleteIndexPastMeetingParticipant(ctx context.Context, data string) error {
	return m.sendIndexerMessage(ctx, models.IndexPastMeetingParticipantSubject, models.ActionDeleted, []byte(data), nil)
}

// SendIndexPastMeetingRecording sends the message to the NATS server for the past meeting recording indexing.
func (m *MessageBuilder) SendIndexPastMeetingRecording(ctx context.Context, action models.MessageAction, data models.PastMeetingRecording) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		slog.ErrorContext(ctx, "error marshalling data into JSON", logging.ErrKey, err)
		return err
	}

	tags := m.setIndexerTags(data.Tags()...)

	return m.sendIndexerMessage(ctx, models.IndexPastMeetingRecordingSubject, action, dataBytes, tags)
}

// SendDeleteIndexPastMeetingRecording sends the message to the NATS server for the past meeting recording indexing.
func (m *MessageBuilder) SendDeleteIndexPastMeetingRecording(ctx context.Context, data string) error {
	return m.sendIndexerMessage(ctx, models.IndexPastMeetingRecordingSubject, models.ActionDeleted, []byte(data), nil)
}

// SendUpdateAccessPastMeeting sends the message to the NATS server for the past meeting access control updates.
func (m *MessageBuilder) SendUpdateAccessPastMeeting(ctx context.Context, data models.PastMeetingAccessMessage) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		slog.ErrorContext(ctx, "error marshalling data into JSON", logging.ErrKey, err)
		return err
	}

	return m.sendMessage(ctx, models.UpdateAccessPastMeetingSubject, dataBytes)
}

// SendDeleteAllAccessPastMeeting sends the message to the NATS server for the past meeting access control deletion.
func (m *MessageBuilder) SendDeleteAllAccessPastMeeting(ctx context.Context, data string) error {
	return m.sendMessage(ctx, models.DeleteAllAccessPastMeetingSubject, []byte(data))
}

// SendPutPastMeetingParticipantAccess sends a message about a new participant being added to a past meeting.
func (m *MessageBuilder) SendPutPastMeetingParticipantAccess(ctx context.Context, data models.PastMeetingParticipantAccessMessage) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		slog.ErrorContext(ctx, "error marshalling data into JSON", logging.ErrKey, err)
		return err
	}

	return m.sendMessage(ctx, models.PutParticipantPastMeetingSubject, dataBytes)
}

// SendRemovePastMeetingParticipantAccess sends a message about a participant being removed from a past meeting.
func (m *MessageBuilder) SendRemovePastMeetingParticipantAccess(ctx context.Context, data models.PastMeetingParticipantAccessMessage) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		slog.ErrorContext(ctx, "error marshalling data into JSON", logging.ErrKey, err)
		return err
	}

	return m.sendMessage(ctx, models.RemoveParticipantPastMeetingSubject, dataBytes)
}

// SendUpdateAccessMeeting sends the message to the NATS server for the access control updates.
func (m *MessageBuilder) SendUpdateAccessMeeting(ctx context.Context, data models.MeetingAccessMessage) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		slog.ErrorContext(ctx, "error marshalling data into JSON", logging.ErrKey, err)
		return err
	}

	return m.sendMessage(ctx, models.UpdateAccessMeetingSubject, dataBytes)
}

// SendDeleteAllAccessMeeting sends the message to the NATS server for the access control deletion.
func (m *MessageBuilder) SendDeleteAllAccessMeeting(ctx context.Context, data string) error {
	return m.sendMessage(ctx, models.DeleteAllAccessMeetingSubject, []byte(data))
}

// SendPutMeetingRegistrantAccess sends a message about a new registrant being added to a meeting.
func (m *MessageBuilder) SendPutMeetingRegistrantAccess(ctx context.Context, data models.MeetingRegistrantAccessMessage) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		slog.ErrorContext(ctx, "error marshalling data into JSON", logging.ErrKey, err)
		return err
	}

	return m.sendMessage(ctx, models.PutRegistrantMeetingSubject, dataBytes)
}

// SendRemoveMeetingRegistrantAccess sends a message about a registrant being removed from a meeting.
func (m *MessageBuilder) SendRemoveMeetingRegistrantAccess(ctx context.Context, data models.MeetingRegistrantAccessMessage) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		slog.ErrorContext(ctx, "error marshalling data into JSON", logging.ErrKey, err)
		return err
	}

	return m.sendMessage(ctx, models.RemoveRegistrantMeetingSubject, dataBytes)
}

// SendMeetingDeleted sends a message about a meeting being deleted to trigger registrant cleanup.
func (m *MessageBuilder) SendMeetingDeleted(ctx context.Context, data models.MeetingDeletedMessage) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		slog.ErrorContext(ctx, "error marshalling data into JSON", logging.ErrKey, err)
		return err
	}

	return m.sendMessage(ctx, models.MeetingDeletedSubject, dataBytes)
}

// SendMeetingUpdated sends a message about a meeting being updated to trigger registrant notifications.
func (m *MessageBuilder) SendMeetingUpdated(ctx context.Context, data models.MeetingUpdatedMessage) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		slog.ErrorContext(ctx, "error marshalling data into JSON", logging.ErrKey, err)
		return err
	}

	return m.sendMessage(ctx, models.MeetingUpdatedSubject, dataBytes)
}

// PublishZoomWebhookEvent publishes a Zoom webhook event to NATS for async processing.
func (m *MessageBuilder) PublishZoomWebhookEvent(ctx context.Context, subject string, message models.ZoomWebhookEventMessage) error {
	messageBytes, err := json.Marshal(message)
	if err != nil {
		slog.ErrorContext(ctx, "error marshalling Zoom webhook event into JSON", logging.ErrKey, err, "subject", subject)
		return err
	}

	slog.DebugContext(ctx, "publishing Zoom webhook event to NATS",
		"subject", subject,
		"event_type", message.EventType,
		"event_ts", message.EventTS,
	)

	return m.sendMessage(ctx, subject, messageBytes)
}
