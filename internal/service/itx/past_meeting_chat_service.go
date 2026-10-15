// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"sort"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/chatlog"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain/models"
	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/logging"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

// chatFileType is the provider's file_type of a saved chat in a recording record.
const chatFileType = "CHAT"

// meetingAndOccurrenceIDPattern bounds the path parameter before any KV read. The
// record keys are hyphenated ("12343245463-1630560600000"); the colon form some
// documents show as an example is admitted here and answered 404 by the reader,
// since it cannot be a KV key.
var meetingAndOccurrenceIDPattern = regexp.MustCompile(`^[0-9A-Za-z:_-]+$`)

// Client-facing messages. They name no URL, host or file.
const (
	msgNoSavedChat          = "no saved chat for this past meeting"
	msgChatUnavailable      = "chat retrieval unavailable"
	msgChatFetchFailed      = "saved chat could not be retrieved; please retry"
	msgInvalidPastMeetingID = "invalid past meeting ID"
)

// PastMeetingChatWindow is the per-session message window of a chat request.
type PastMeetingChatWindow struct {
	Limit  int
	Offset int
}

// PastMeetingChatService serves the chat saved with a past meeting's recording.
//
// It is not an ITX proxy: the recording record (read from the v1-objects KV
// bucket) names the CHAT files, the service fetches them from the provider under
// the caller's authorization and returns them parsed and bounded. Authorization is
// Heimdall's recording_viewer check on v1_past_meeting:{id} at the edge; the route
// parameter is the record key itself, so there is no child object to re-check here.
type PastMeetingChatService struct {
	records  domain.RecordingRecordReader // nil when NATS is not configured
	fetcher  domain.RecordingFileFetcher
	maxBytes int64
}

// NewPastMeetingChatService creates a PastMeetingChatService. records may be nil
// when NATS is not configured; GetPastMeetingChat then answers 503. maxBytes caps
// the bytes read from each CHAT file (constants.PastMeetingChatMaxBytes in
// production; tests pass a small cap).
func NewPastMeetingChatService(records domain.RecordingRecordReader, fetcher domain.RecordingFileFetcher, maxBytes int64) *PastMeetingChatService {
	return &PastMeetingChatService{records: records, fetcher: fetcher, maxBytes: maxBytes}
}

// GetPastMeetingChat returns the saved chat of a past meeting, one session per CHAT
// file ordered by recording start, each session windowed by window.
//
// Outcomes: an ID outside [0-9A-Za-z:_-] or a window out of range is a validation
// error (400); no recording record or no CHAT file is not found (404), and the
// route never says whether the past meeting itself exists; a nil reader, a KV read
// failure or any failure to fetch a file as text is unavailable (503) and the whole
// request fails, so a partial response is never mistaken for an empty chat.
func (s *PastMeetingChatService) GetPastMeetingChat(ctx context.Context, meetingAndOccurrenceID string, window PastMeetingChatWindow) (*itx.PastMeetingChatResponse, error) {
	if err := validateChatRequest(meetingAndOccurrenceID, window); err != nil {
		return nil, err
	}
	if s.records == nil {
		slog.WarnContext(ctx, "past meeting chat requested but no recording record reader is configured",
			"meeting_and_occurrence_id", meetingAndOccurrenceID)
		return nil, domain.NewUnavailableError(msgChatUnavailable)
	}

	record, err := s.records.GetRecordingRecord(ctx, meetingAndOccurrenceID)
	if err != nil {
		if errors.Is(err, domain.ErrRecordingRecordNotFound) {
			return nil, domain.NewNotFoundError(msgNoSavedChat)
		}
		// The reader has already classified (unavailable or internal) and logged.
		return nil, err
	}

	chatFiles := selectChatFiles(record.RecordingFiles)
	if len(chatFiles) == 0 {
		return nil, domain.NewNotFoundError(msgNoSavedChat)
	}

	resp := &itx.PastMeetingChatResponse{
		MeetingAndOccurrenceID: meetingAndOccurrenceID,
		Sessions:               make([]itx.PastMeetingChatSession, 0, len(chatFiles)),
	}
	for i, file := range chatFiles {
		// Fetched sequentially, each under its own timeout. The fetcher's errors carry
		// no URL, so they are safe to log; the client gets a fixed message.
		data, capped, err := s.fetcher.FetchText(ctx, file.DownloadURL, s.maxBytes)
		if err != nil {
			slog.With(logging.ErrKey, err).WarnContext(ctx, "failed to fetch saved chat file",
				"meeting_and_occurrence_id", meetingAndOccurrenceID, "session_index", i, "session_count", len(chatFiles))
			return nil, domain.NewUnavailableError(msgChatFetchFailed)
		}
		resp.Sessions = append(resp.Sessions, buildChatSession(file, chatlog.Parse(data, capped), capped, window))
	}
	return resp, nil
}

// validateChatRequest rejects obvious garbage before any read without revealing
// anything, and enforces the window bounds even when the caller bypasses Goa.
func validateChatRequest(meetingAndOccurrenceID string, window PastMeetingChatWindow) error {
	if meetingAndOccurrenceID == "" || len(meetingAndOccurrenceID) > constants.PastMeetingChatMaxIDLength ||
		!meetingAndOccurrenceIDPattern.MatchString(meetingAndOccurrenceID) {
		return domain.NewValidationError(msgInvalidPastMeetingID)
	}
	if window.Limit < 1 || window.Limit > constants.PastMeetingChatMaxLimit {
		return domain.NewValidationError("limit must be between 1 and 2000")
	}
	if window.Offset < 0 {
		return domain.NewValidationError("offset must be 0 or greater")
	}
	return nil
}

// selectChatFiles returns the CHAT entries of a recording record ordered by
// recording start (then end), one per session. The order is stable so two files
// with equal timestamps keep the record's order.
func selectChatFiles(files []models.RecordingFile) []models.RecordingFile {
	var chat []models.RecordingFile
	for _, f := range files {
		if f.FileType == chatFileType {
			chat = append(chat, f)
		}
	}
	sort.SliceStable(chat, func(i, j int) bool {
		if !chat[i].RecordingStart.Equal(chat[j].RecordingStart) {
			return chat[i].RecordingStart.Before(chat[j].RecordingStart)
		}
		return chat[i].RecordingEnd.Before(chat[j].RecordingEnd)
	})
	return chat
}

// buildChatSession applies the window to the parsed messages of one file. The
// session copies only timestamps from the file entry: no link, size or identifier.
func buildChatSession(file models.RecordingFile, parsed []chatlog.Message, capped bool, window PastMeetingChatWindow) itx.PastMeetingChatSession {
	total := len(parsed)
	start := min(window.Offset, total)
	end := min(start+window.Limit, total)

	messages := make([]itx.PastMeetingChatMessage, 0, end-start)
	for _, m := range parsed[start:end] {
		messages = append(messages, itx.PastMeetingChatMessage{
			Offset:    m.Offset,
			Sender:    m.Sender,
			Recipient: m.Recipient,
			Text:      m.Text,
		})
	}
	return itx.PastMeetingChatSession{
		RecordingStart: file.RecordingStart,
		RecordingEnd:   file.RecordingEnd,
		MessageCount:   total,
		Truncated:      capped || end < total,
		Messages:       messages,
	}
}
