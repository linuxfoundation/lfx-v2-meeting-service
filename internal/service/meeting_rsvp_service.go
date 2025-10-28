// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
)

// MeetingRSVPService implements meeting RSVP operations for meeting registrants
type MeetingRSVPService struct {
	meetingRSVPRepository domain.MeetingRSVPRepository
	meetingRepository     domain.MeetingRepository
	registrantRepository  domain.RegistrantRepository
	occurrenceService     *OccurrenceService
	messageSender         domain.MeetingRSVPIndexSender
}

// NewMeetingRSVPService creates a new MeetingRSVPService.
func NewMeetingRSVPService(
	meetingRSVPRepository domain.MeetingRSVPRepository,
	meetingRepository domain.MeetingRepository,
	registrantRepository domain.RegistrantRepository,
	occurrenceService *OccurrenceService,
	messageSender domain.MeetingRSVPIndexSender,
) *MeetingRSVPService {
	return &MeetingRSVPService{
		meetingRSVPRepository: meetingRSVPRepository,
		meetingRepository:     meetingRepository,
		registrantRepository:  registrantRepository,
		occurrenceService:     occurrenceService,
		messageSender:         messageSender,
	}
}

// ServiceReady checks if the service is ready for use.
func (s *MeetingRSVPService) ServiceReady() bool {
	return s.meetingRSVPRepository != nil &&
		s.meetingRepository != nil &&
		s.registrantRepository != nil &&
		s.occurrenceService != nil &&
		s.messageSender != nil
}

// PutRSVP creates or updates an RSVP response for a meeting registrant.
// Following "most recent wins" rule, this replaces any existing RSVP for this registrant/meeting combination.
// Either RegistrantID or Username must be provided to identify the registrant.
func (s *MeetingRSVPService) PutRSVP(ctx context.Context, req *models.CreateRSVPRequest) (*models.RSVPResponse, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("rsvp service is not ready")
	}

	// Validate that at least one of RegistrantID or Username is provided
	if req.RegistrantID == "" && req.Username == "" {
		return nil, domain.NewValidationError("either registrant_id or username must be provided")
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", req.MeetingUID))
	ctx = logging.AppendCtx(ctx, slog.String("response", string(req.Response)))
	ctx = logging.AppendCtx(ctx, slog.String("scope", string(req.Scope)))

	// Validate scope and occurrence ID requirements
	if req.Scope == models.RSVPScopeSingle || req.Scope == models.RSVPScopeThisAndFollowing {
		if req.OccurrenceID == nil || *req.OccurrenceID == "" {
			return nil, domain.NewValidationError("occurrence_id is required for 'single' and 'this_and_following' scopes")
		}
	}

	// Verify meeting exists
	meeting, err := s.meetingRepository.GetBase(ctx, req.MeetingUID)
	if err != nil {
		return nil, err
	}

	// Get registrant by ID or username
	var registrant *models.Registrant
	if req.RegistrantID != "" {
		// Look up by registrant ID
		ctx = logging.AppendCtx(ctx, slog.String("registrant_id", req.RegistrantID))
		registrant, err = s.registrantRepository.Get(ctx, req.RegistrantID)
		if err != nil {
			return nil, err
		}
		if registrant.MeetingUID != req.MeetingUID {
			return nil, domain.NewValidationError(fmt.Sprintf("registrant %s does not belong to meeting %s", req.RegistrantID, req.MeetingUID))
		}
	} else {
		// Look up by username
		ctx = logging.AppendCtx(ctx, slog.String("username", req.Username))
		var revision uint64
		registrant, revision, err = s.registrantRepository.GetByMeetingAndUsername(ctx, req.MeetingUID, req.Username)
		if err != nil {
			return nil, err
		}
		// Set the RegistrantID for later use
		req.RegistrantID = registrant.UID
		ctx = logging.AppendCtx(ctx, slog.String("registrant_id", registrant.UID))
		_ = revision // We don't need the revision here, just the registrant
	}

	// If scope is single or this_and_following, verify the occurrence exists
	if req.OccurrenceID != nil && *req.OccurrenceID != "" {
		occurrences := s.occurrenceService.CalculateOccurrencesFromDate(meeting, time.Now(), 100)
		occurrenceExists := false
		for _, occ := range occurrences {
			if occ.OccurrenceID == *req.OccurrenceID {
				occurrenceExists = true
				break
			}
		}
		if !occurrenceExists {
			return nil, domain.NewNotFoundError(fmt.Sprintf("occurrence %s not found for meeting %s", *req.OccurrenceID, req.MeetingUID))
		}
	}

	// Check if an RSVP already exists for this specific registrant/meeting/scope/occurrence combination
	// Multiple RSVPs can exist per registrant (e.g., scope=all + scope=single for one occurrence)
	existingRSVP, revision, err := s.findMatchingRSVP(ctx, req.MeetingUID, req.RegistrantID, req.Scope, req.OccurrenceID)
	if err != nil && domain.GetErrorType(err) != domain.ErrorTypeNotFound {
		return nil, err
	}

	now := time.Now()

	if existingRSVP != nil {
		// Update existing RSVP (most recent wins)
		slog.InfoContext(ctx, "updating existing rsvp", "rsvp_id", existingRSVP.ID)

		existingRSVP.Response = req.Response
		existingRSVP.Scope = req.Scope
		existingRSVP.OccurrenceID = req.OccurrenceID
		existingRSVP.Username = registrant.Username
		existingRSVP.Email = registrant.Email
		existingRSVP.UpdatedAt = &now

		if err := s.meetingRSVPRepository.Update(ctx, existingRSVP, revision); err != nil {
			return nil, err
		}

		// Send indexing message for update
		if err := s.messageSender.SendIndexMeetingRSVP(ctx, models.ActionUpdated, *existingRSVP); err != nil {
			slog.ErrorContext(ctx, "failed to send index message for updated rsvp", logging.ErrKey, err, "rsvp_id", existingRSVP.ID)
			// Don't fail the operation if indexing fails
		}

		return existingRSVP, nil
	}

	// Create new RSVP
	rsvp := &models.RSVPResponse{
		ID:           uuid.New().String(),
		MeetingUID:   req.MeetingUID,
		RegistrantID: req.RegistrantID,
		Username:     registrant.Username,
		Email:        registrant.Email,
		Response:     req.Response,
		Scope:        req.Scope,
		OccurrenceID: req.OccurrenceID,
		CreatedAt:    &now,
		UpdatedAt:    &now,
	}

	if err := s.meetingRSVPRepository.Create(ctx, rsvp); err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "created new rsvp", "rsvp_id", rsvp.ID)

	// Send indexing message for create
	if err := s.messageSender.SendIndexMeetingRSVP(ctx, models.ActionCreated, *rsvp); err != nil {
		slog.ErrorContext(ctx, "failed to send index message for created rsvp", logging.ErrKey, err, "rsvp_id", rsvp.ID)
		// Don't fail the operation if indexing fails
	}

	return rsvp, nil
}

// findMatchingRSVP finds an existing RSVP that matches the given scope and occurrence combination.
// This allows multiple RSVPs per registrant (e.g., scope=all + scope=single for specific occurrences).
func (s *MeetingRSVPService) findMatchingRSVP(ctx context.Context, meetingUID, registrantID string, scope models.RSVPScope, occurrenceID *string) (*models.RSVPResponse, uint64, error) {
	// Get all RSVPs for this meeting
	allRSVPs, err := s.meetingRSVPRepository.ListByMeeting(ctx, meetingUID)
	if err != nil {
		return nil, 0, err
	}

	// Filter to find matching RSVP for this registrant with the same scope and occurrence_id
	for _, rsvp := range allRSVPs {
		if rsvp.RegistrantID != registrantID {
			continue
		}
		if rsvp.Scope != scope {
			continue
		}

		// Match occurrence_id based on scope
		switch scope {
		case models.RSVPScopeAll:
			// For scope=all, occurrence_id should be nil
			if rsvp.OccurrenceID == nil || *rsvp.OccurrenceID == "" {
				// Found a match, get it with revision
				return s.meetingRSVPRepository.GetWithRevision(ctx, rsvp.ID)
			}
		case models.RSVPScopeSingle, models.RSVPScopeThisAndFollowing:
			// For single or this_and_following, occurrence_id must match
			if occurrenceID != nil && rsvp.OccurrenceID != nil && *rsvp.OccurrenceID == *occurrenceID {
				// Found a match, get it with revision
				return s.meetingRSVPRepository.GetWithRevision(ctx, rsvp.ID)
			}
		}
	}

	// No matching RSVP found
	return nil, 0, domain.NewNotFoundError(fmt.Sprintf("no rsvp found for registrant %s with scope %s", registrantID, scope))
}

// GetMeetingRSVPs retrieves all RSVP responses for a meeting
func (s *MeetingRSVPService) GetMeetingRSVPs(ctx context.Context, meetingUID string) ([]*models.RSVPResponse, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("rsvp service is not ready")
	}

	ctx = logging.AppendCtx(ctx, slog.String("meeting_uid", meetingUID))

	// Verify meeting exists
	_, err := s.meetingRepository.GetBase(ctx, meetingUID)
	if err != nil {
		return nil, err
	}

	// Get all RSVPs for the meeting
	rsvps, err := s.meetingRSVPRepository.ListByMeeting(ctx, meetingUID)
	if err != nil {
		return nil, err
	}

	slog.DebugContext(ctx, "returning meeting rsvps", "count", len(rsvps))

	return rsvps, nil
}

// GetRSVPForOccurrence determines what RSVP response applies to a specific occurrence
// based on the "most recent wins" rule and scope logic
func (s *MeetingRSVPService) GetRSVPForOccurrence(ctx context.Context, meetingUID, registrantID, occurrenceID string) (*models.RSVPResponse, error) {
	if !s.ServiceReady() {
		slog.ErrorContext(ctx, "service not initialized", logging.PriorityCritical())
		return nil, domain.NewUnavailableError("rsvp service is not ready")
	}

	// Get the registrant's RSVP for this meeting
	rsvp, _, err := s.meetingRSVPRepository.GetByMeetingAndRegistrant(ctx, meetingUID, registrantID)
	if err != nil {
		return nil, err
	}

	// Check if the RSVP applies to this occurrence
	switch rsvp.Scope {
	case models.RSVPScopeAll:
		return rsvp, nil
	case models.RSVPScopeSingle:
		if rsvp.OccurrenceID != nil && *rsvp.OccurrenceID == occurrenceID {
			return rsvp, nil
		}
	case models.RSVPScopeThisAndFollowing:
		// Need to check if the occurrence is >= the RSVP's occurrence
		// This requires occurrence ordering logic in the calling code
		if rsvp.OccurrenceID != nil {
			// For now, return the RSVP and let the caller handle ordering logic
			return rsvp, nil
		}
	}

	return nil, domain.NewNotFoundError(fmt.Sprintf("no rsvp applies to occurrence %s for registrant %s", occurrenceID, registrantID))
}
