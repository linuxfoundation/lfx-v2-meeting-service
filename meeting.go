package meetingapi

import (
	"context"

	meeting "github.com/linuxfoundation/lfx-v2-meeting-service/gen/meeting"
	"goa.design/clue/log"
)

// Meeting service example implementation.
// The example methods log the requests and return zero values.
type meetingsrvc struct{}

// NewMeeting returns the Meeting service implementation.
func NewMeeting() meeting.Service {
	return &meetingsrvc{}
}

// Create implements create.
func (s *meetingsrvc) Create(ctx context.Context, p *meeting.CreatePayload) (res *meeting.CreateResult, err error) {
	id := "1234567890"
	res = &meeting.CreateResult{
		ID:        &id,
		ProjectID: &p.ProjectID,
		StartTime: &p.StartTime,
		Duration:  &p.Duration,
		Timezone:  &p.Timezone,
		Topic:     &p.Topic,
		Agenda:    &p.Agenda,
	}
	log.Printf(ctx, "meeting.create")
	return
}
