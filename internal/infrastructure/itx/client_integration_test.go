// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package itx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/linuxfoundation/lfx-v2-meeting-service/internal/domain"
	pkgitx "github.com/linuxfoundation/lfx-v2-meeting-service/pkg/models/itx"
)

func TestClient_GetZoomMeeting_HTTP(t *testing.T) {
	t.Parallel()

	const meetingID = "meeting-123"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/v2/zoom/meetings/"+meetingID {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("x-scope"); got != itxScope {
			t.Fatalf("x-scope = %q, want %q", got, itxScope)
		}
		if got := r.Header.Get("Accept"); got != acceptJSON {
			t.Fatalf("Accept = %q, want %q", got, acceptJSON)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(pkgitx.ZoomMeetingResponse{
			ID:    meetingID,
			Topic: "Weekly sync",
		})
	}))
	t.Cleanup(server.Close)

	client := NewClientWithHTTPClient(Config{BaseURL: server.URL}, server.Client())
	resp, err := client.Meetings().GetZoomMeeting(context.Background(), meetingID)
	if err != nil {
		t.Fatalf("GetZoomMeeting() error = %v", err)
	}
	if resp.ID != meetingID {
		t.Fatalf("ID = %q, want %q", resp.ID, meetingID)
	}
	if resp.Topic != "Weekly sync" {
		t.Fatalf("Topic = %q, want %q", resp.Topic, "Weekly sync")
	}
}

func TestClient_GetRegistrantICS_HTTP(t *testing.T) {
	t.Parallel()

	const (
		meetingID    = "meeting-123"
		registrantID = "registrant-456"
		icsBody      = "BEGIN:VCALENDAR\nEND:VCALENDAR"
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/zoom/meetings/"+meetingID+"/registrants/"+registrantID+"/ics" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Accept"); got != acceptCalendar {
			t.Fatalf("Accept = %q, want %q", got, acceptCalendar)
		}
		w.Header().Set("Content-Type", "text/calendar")
		_, _ = w.Write([]byte(icsBody))
	}))
	t.Cleanup(server.Close)

	client := NewClientWithHTTPClient(Config{BaseURL: server.URL}, server.Client())
	resp, err := client.Registrants().GetRegistrantICS(context.Background(), meetingID, registrantID)
	if err != nil {
		t.Fatalf("GetRegistrantICS() error = %v", err)
	}
	if string(resp.Content) != icsBody {
		t.Fatalf("content = %q, want %q", string(resp.Content), icsBody)
	}
}

func TestClient_GetZoomMeeting_HTTPErrorMapping(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"meeting not found"}`))
	}))
	t.Cleanup(server.Close)

	client := NewClientWithHTTPClient(Config{BaseURL: server.URL}, server.Client())
	_, err := client.Meetings().GetZoomMeeting(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if domain.GetErrorType(err) != domain.ErrorTypeNotFound {
		t.Fatalf("GetErrorType() = %v, want %v", domain.GetErrorType(err), domain.ErrorTypeNotFound)
	}
}

func TestClient_UpdatePastMeeting_NoContent(t *testing.T) {
	t.Parallel()

	const pastMeetingID = "123-456"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	client := NewClientWithHTTPClient(Config{BaseURL: server.URL}, server.Client())
	resp, err := client.PastMeetings().UpdatePastMeeting(context.Background(), pastMeetingID, &pkgitx.CreatePastMeetingRequest{
		Topic: "Updated topic",
	})
	if err != nil {
		t.Fatalf("UpdatePastMeeting() error = %v", err)
	}
	if resp != nil {
		t.Fatalf("resp = %#v, want nil on 204", resp)
	}
}

func TestClient_GetPastMeetingParticipantIDs_HTTP(t *testing.T) {
	t.Parallel()

	// The decoded IDs decide which invitee/attendee records a participant
	// update or delete may touch, so pin the route and the uuid/id keys.
	const pastMeetingID = "12345678901-1700000000000"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/v2/zoom/past_meetings/"+pastMeetingID {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Has("basic") {
			t.Fatalf("basic query parameter set; the full response carries the participant lists")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"past_meeting_id": "` + pastMeetingID + `",
			"invitees": [
				{"uuid": "00000000-0000-0000-0000-000000000001", "first_name": "Alice", "primary_email": "alice@example.com"},
				{"uuid": "00000000-0000-0000-0000-000000000002", "first_name": "Bob", "primary_email": "bob@example.com"}
			],
			"invitee_count": 2,
			"attendees": [
				{"id": "00000000-0000-0000-0000-000000000003", "name": "Alice Example", "email": "alice@example.com", "meeting_and_occurrence_id": "` + pastMeetingID + `"}
			],
			"attendee_count": 1
		}`))
	}))
	t.Cleanup(server.Close)

	client := NewClientWithHTTPClient(Config{BaseURL: server.URL}, server.Client())
	resp, err := client.Participants().GetPastMeetingParticipantIDs(context.Background(), pastMeetingID)
	if err != nil {
		t.Fatalf("GetPastMeetingParticipantIDs() error = %v", err)
	}

	var inviteeIDs, attendeeIDs []string
	for _, invitee := range resp.Invitees {
		inviteeIDs = append(inviteeIDs, invitee.UUID)
	}
	for _, attendee := range resp.Attendees {
		attendeeIDs = append(attendeeIDs, attendee.ID)
	}
	if want := []string{"00000000-0000-0000-0000-000000000001", "00000000-0000-0000-0000-000000000002"}; !slices.Equal(inviteeIDs, want) {
		t.Fatalf("invitee IDs = %v, want %v", inviteeIDs, want)
	}
	if want := []string{"00000000-0000-0000-0000-000000000003"}; !slices.Equal(attendeeIDs, want) {
		t.Fatalf("attendee IDs = %v, want %v", attendeeIDs, want)
	}
}
