// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
)

// Message represents a domain message interface
type Message interface {
	Subject() string
	Data() []byte
	Respond(data []byte) error
	HasReply() bool
}

// MessageHandler defines how the service handles incoming messages
type MessageHandler interface {
	HandleMessage(ctx context.Context, msg Message)
}

// MessageBuilder is a interface for the message builder.
type MessageBuilder interface {
	SendIndexMeeting(ctx context.Context, action models.MessageAction, data models.MeetingBase) error
	SendIndexMeetingSettings(ctx context.Context, action models.MessageAction, data models.MeetingSettings) error
	SendIndexMeetingRegistrant(ctx context.Context, action models.MessageAction, data models.Registrant) error
	SendDeleteIndexMeeting(ctx context.Context, data string) error
	SendDeleteIndexMeetingSettings(ctx context.Context, data string) error
	SendDeleteIndexMeetingRegistrant(ctx context.Context, data string) error
	SendUpdateAccessMeeting(ctx context.Context, data models.MeetingAccessMessage) error
	SendDeleteAllAccessMeeting(ctx context.Context, data string) error
	SendPutMeetingRegistrantAccess(ctx context.Context, data models.MeetingRegistrantAccessMessage) error
	SendRemoveMeetingRegistrantAccess(ctx context.Context, data models.MeetingRegistrantAccessMessage) error
	SendMeetingDeleted(ctx context.Context, data models.MeetingDeletedMessage) error
}
