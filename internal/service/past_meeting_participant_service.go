package service

import "github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"

// PastMeetingParticipantService implements the meetingsvc.Service interface and domain.MessageHandler
type PastMeetingParticipantService struct {
	MeetingRepository                domain.MeetingRepository
	PastMeetingRepository            domain.PastMeetingRepository
	PastMeetingParticipantRepository domain.PastMeetingParticipantRepository
	MessageBuilder                   domain.MessageBuilder
	Config                           ServiceConfig
}

// NewPastMeetingParticipantService creates a new PastMeetingParticipantService.
func NewPastMeetingParticipantService(
	meetingRepository domain.MeetingRepository,
	pastMeetingRepository domain.PastMeetingRepository,
	pastMeetingParticipantRepository domain.PastMeetingParticipantRepository,
	messageBuilder domain.MessageBuilder,
	config ServiceConfig,
) *PastMeetingParticipantService {
	return &PastMeetingParticipantService{
		Config:                           config,
		MeetingRepository:                meetingRepository,
		PastMeetingRepository:            pastMeetingRepository,
		PastMeetingParticipantRepository: pastMeetingParticipantRepository,
		MessageBuilder:                   messageBuilder,
	}
}

// ServiceReady checks if the service is ready for use.
func (s *PastMeetingParticipantService) ServiceReady() bool {
	return s.MeetingRepository != nil &&
		s.PastMeetingRepository != nil &&
		s.MessageBuilder != nil
}
