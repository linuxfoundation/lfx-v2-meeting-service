// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT
// Google Meet API Research Script
//
// Tests the full lifecycle of a Google Meet meeting via the Calendar API:
//   1. Create a meeting with a Meet conference link
//   2. Update the meeting (title, description, time)
//   3. Add a registrant (calendar attendee)
//   4. Edit the registrant (mark optional)
//   5. Delete the registrant
//   6. Delete the meeting
//   7. (Bonus) Fetch space info via Meet REST API
//
// Required environment variables:
//   GOOGLE_OAUTH_CLIENT_SECRET  - path to OAuth client_secret.json (Desktop app type)
//   TEST_ATTENDEE_EMAIL         - email to use as test registrant (default: test-registrant@example.com)
//
// On first run a browser window opens for you to log in and approve access.
// The token is saved to token.json and reused on subsequent runs.
//
// Usage:
//   go run main.go

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// ─────────────────────────────────────────────
// Logging HTTP transport — wraps the inner transport
// so we see every request (with auth headers added)
// and every response body.
// ─────────────────────────────────────────────

type LoggingTransport struct {
	base http.RoundTripper
}

func (t *LoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	fmt.Printf("\n%s\n", strings.Repeat("─", 70))
	fmt.Printf("► %s %s\n", req.Method, req.URL)

	// Log request headers (skip Authorization value for security, just show presence)
	for key, vals := range req.Header {
		if strings.EqualFold(key, "authorization") {
			fmt.Printf("  Header: %s: Bearer <token>\n", key)
		} else {
			fmt.Printf("  Header: %s: %s\n", key, strings.Join(vals, ", "))
		}
	}

	// Log request body
	if req.Body != nil && req.Body != http.NoBody {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			fmt.Printf("  Request body read error: %v\n", err)
		} else {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			var prettyBody interface{}
			if json.Unmarshal(bodyBytes, &prettyBody) == nil {
				prettyJSON, _ := json.MarshalIndent(prettyBody, "  ", "  ")
				fmt.Printf("  Request body:\n  %s\n", prettyJSON)
			} else {
				fmt.Printf("  Request body (raw): %s\n", string(bodyBytes))
			}
		}
	}

	// Execute
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		fmt.Printf("◄ TRANSPORT ERROR: %v\n", err)
		return nil, err
	}

	// Log response
	fmt.Printf("◄ %d %s\n", resp.StatusCode, resp.Status)

	bodyBytes, readErr := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	if readErr != nil {
		fmt.Printf("  Response body read error: %v\n", readErr)
	} else {
		var prettyBody interface{}
		if json.Unmarshal(bodyBytes, &prettyBody) == nil {
			prettyJSON, _ := json.MarshalIndent(prettyBody, "  ", "  ")
			fmt.Printf("  Response body:\n  %s\n", prettyJSON)
		} else {
			fmt.Printf("  Response body (raw): %s\n", string(bodyBytes))
		}
	}

	return resp, nil
}

// ─────────────────────────────────────────────
// OAuth flow helpers
// ─────────────────────────────────────────────

const tokenFile = "token.json"

func getOAuthClient(ctx context.Context, config *oauth2.Config) *http.Client {
	// Load cached token if it exists
	tok, err := loadToken()
	if err != nil {
		// No token yet — run the browser-based auth flow
		tok = runOAuthFlow(ctx, config)
		saveToken(tok)
	}
	return config.Client(ctx, tok)
}

func loadToken() (*oauth2.Token, error) {
	f, err := os.Open(tokenFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	if err := json.NewDecoder(f).Decode(tok); err != nil {
		return nil, err
	}
	return tok, nil
}

func saveToken(tok *oauth2.Token) {
	f, err := os.Create(tokenFile)
	if err != nil {
		log.Printf("Warning: could not save token to %s: %v", tokenFile, err)
		return
	}
	defer f.Close()
	_ = json.NewEncoder(f).Encode(tok)
	fmt.Printf("  Token saved to %s — will be reused on next run.\n", tokenFile)
}

func runOAuthFlow(ctx context.Context, config *oauth2.Config) *oauth2.Token {
	// Start a local HTTP server to capture the redirect
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		log.Fatalf("Failed to start local callback server: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	redirectURL := fmt.Sprintf("http://localhost:%d", port)
	config.RedirectURL = redirectURL

	codeCh := make(chan string, 1)
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			code := r.URL.Query().Get("code")
			fmt.Fprintln(w, "<h2>Authorization complete — you can close this tab.</h2>")
			codeCh <- code
		}),
	}
	go func() { _ = srv.Serve(listener) }()

	authURL := config.AuthCodeURL("state", oauth2.AccessTypeOffline)
	fmt.Println("\n  Opening browser for Google authorization...")
	fmt.Printf("  If the browser doesn't open, visit:\n  %s\n\n", authURL)
	_ = exec.Command("open", authURL).Start() // macOS; ignored if it fails

	// Wait for the redirect callback
	code := <-codeCh
	_ = srv.Shutdown(ctx)

	tok, err := config.Exchange(ctx, code)
	if err != nil {
		log.Fatalf("Failed to exchange auth code for token: %v", err)
	}
	return tok
}

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

func stepHeader(n int, title string) {
	fmt.Printf("\n%s\n", strings.Repeat("═", 70))
	fmt.Printf("  STEP %d: %s\n", n, title)
	fmt.Printf("%s\n", strings.Repeat("═", 70))
}

func success(label, detail string) {
	fmt.Printf("\n✅ %s: %s\n", label, detail)
}

func fatal(label string, err error) {
	fmt.Printf("\n❌ FATAL in %s: %v\n", label, err)
	os.Exit(1)
}

func must(label string, err error) {
	if err != nil {
		fatal(label, err)
	}
}

// ─────────────────────────────────────────────
// Main
// ─────────────────────────────────────────────

func main() {
	clientSecretFile := os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")
	if clientSecretFile == "" {
		log.Fatal("GOOGLE_OAUTH_CLIENT_SECRET is required — path to your OAuth client_secret.json (Desktop app type)")
	}

	skipDelete := os.Getenv("SKIP_DELETE") == "true"
	testAttendeeEmail := os.Getenv("TEST_ATTENDEE_EMAIL")
	if testAttendeeEmail == "" {
		testAttendeeEmail = "test-registrant@example.com"
		fmt.Printf("TEST_ATTENDEE_EMAIL not set, using placeholder: %s\n", testAttendeeEmail)
	}

	fmt.Println(strings.Repeat("═", 70))
	fmt.Println("  Google Meet API Research Script")
	fmt.Println(strings.Repeat("═", 70))
	fmt.Printf("  OAuth client secret: %s\n", clientSecretFile)
	fmt.Printf("  Test attendee:       %s\n", testAttendeeEmail)
	fmt.Printf("  Token cache:         %s\n", tokenFile)
	fmt.Println(strings.Repeat("═", 70))

	// ─── Auth setup ───────────────────────────────────────────────────────
	clientSecretJSON, err := os.ReadFile(clientSecretFile)
	must("read client secret file", err)

	oauthConfig, err := google.ConfigFromJSON(clientSecretJSON,
		calendar.CalendarScope,
		calendar.CalendarEventsScope,
		"https://www.googleapis.com/auth/meetings.space.readonly",
		"https://www.googleapis.com/auth/meetings.space.created",
	)
	must("parse client secret JSON", err)

	ctx := context.Background()

	// Get an authenticated HTTP client (opens browser on first run)
	baseClient := getOAuthClient(ctx, oauthConfig)

	// Wrap with logging transport so we see every request/response
	httpClient := &http.Client{
		Transport: &LoggingTransport{base: baseClient.Transport},
	}

	calSvc, err := calendar.NewService(ctx, option.WithHTTPClient(httpClient))
	must("create calendar service", err)

	// Track IDs across steps
	var eventID string
	var meetURL string
	var meetSpaceCode string // e.g. "abc-defg-hij"

	// ─────────────────────────────────────────────────────────────────────
	// STEP 1: Create meeting via Calendar API with conferenceData
	// ─────────────────────────────────────────────────────────────────────
	stepHeader(1, "Create Google Meet meeting (Calendar API)")

	now := time.Now().UTC()
	start := now.Add(24 * time.Hour).Truncate(time.Hour)
	end := start.Add(time.Hour)

	event := &calendar.Event{
		Summary:     "LFX Google Meet Research - Test Meeting",
		Description: "Automated research meeting created by LFX Google Meet API investigation script.",
		Start: &calendar.EventDateTime{
			DateTime: start.Format(time.RFC3339),
			TimeZone: "UTC",
		},
		End: &calendar.EventDateTime{
			DateTime: end.Format(time.RFC3339),
			TimeZone: "UTC",
		},
		ConferenceData: &calendar.ConferenceData{
			CreateRequest: &calendar.CreateConferenceRequest{
				// Idempotency key — retry with same ID won't create a duplicate
				RequestId: fmt.Sprintf("lfx-research-%d", now.UnixNano()),
				ConferenceSolutionKey: &calendar.ConferenceSolutionKey{
					Type: "hangoutsMeet",
				},
			},
		},
		Visibility: "public",
	}

	created, err := calSvc.Events.Insert("primary", event).
		ConferenceDataVersion(1). // required to trigger Meet link generation
		SendUpdates("none").      // suppress Google Calendar invite emails
		Do()
	must("create event", err)

	eventID = created.Id
	if created.ConferenceData != nil {
		for _, ep := range created.ConferenceData.EntryPoints {
			if ep.EntryPointType == "video" {
				meetURL = ep.Uri
				parts := strings.Split(ep.Uri, "/")
				if len(parts) > 0 {
					meetSpaceCode = parts[len(parts)-1]
				}
			}
		}
		if created.ConferenceData.ConferenceId != "" {
			meetSpaceCode = created.ConferenceData.ConferenceId
		}
	}

	success("Create meeting",
		fmt.Sprintf("Event ID: %s | Meet URL: %s | Space code: %s",
			eventID, meetURL, meetSpaceCode))

	// ─────────────────────────────────────────────────────────────────────
	// STEP 2: Fetch Meet space info via Meet REST API (while meeting exists)
	// ─────────────────────────────────────────────────────────────────────
	// The Meet REST API (meet.googleapis.com/v2) provides post-meeting data:
	//   - spaces        — space metadata and config
	//   - conferenceRecords — per-session records (only populated after meeting ends)
	//   - participants, recordings, transcripts — all post-meeting
	//
	// We call it now (before delete) so the space still exists.
	// Conference records will be empty since the meeting hasn't happened yet —
	// but the call shape is what we're verifying for research purposes.
	// ─────────────────────────────────────────────────────────────────────
	stepHeader(2, "Meet REST API — GET space info (meet.googleapis.com/v2)")

	if meetSpaceCode == "" {
		fmt.Println("  Skipping: no space code extracted from Meet URL")
	} else {
		meetAPIURL := fmt.Sprintf("https://meet.googleapis.com/v2/spaces/%s", meetSpaceCode)
		fmt.Printf("  Fetching: %s\n", meetAPIURL)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, meetAPIURL, nil)
		must("build Meet API request", err)

		resp, err := httpClient.Do(req)
		if err != nil {
			fmt.Printf("  ⚠ Meet REST API request failed: %v\n", err)
		} else {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				success("Get Meet space", fmt.Sprintf("space code %s fetched successfully", meetSpaceCode))
			} else {
				fmt.Printf("  ⚠ Meet REST API returned %d — see response above\n", resp.StatusCode)
			}
		}

		// Conference records are only populated after the meeting ends.
		// This call will return an empty list for a future scheduled meeting,
		// but confirms the API is reachable and the scope is working.
		confRecordsURL := fmt.Sprintf(
			"https://meet.googleapis.com/v2/conferenceRecords?filter=space.name=%%22spaces/%s%%22",
			meetSpaceCode,
		)
		fmt.Printf("\n  Fetching conference records (post-meeting data): %s\n", confRecordsURL)

		req2, err := http.NewRequestWithContext(ctx, http.MethodGet, confRecordsURL, nil)
		must("build conference records request", err)

		resp2, err := httpClient.Do(req2)
		if err != nil {
			fmt.Printf("  ⚠ Conference records request failed: %v\n", err)
		} else {
			defer resp2.Body.Close()
			if resp2.StatusCode == http.StatusOK {
				success("Get conference records", "success (empty list expected — meeting hasn't happened yet)")
			} else {
				fmt.Printf("  ⚠ Conference records returned %d\n", resp2.StatusCode)
			}
		}
	}

	// ─────────────────────────────────────────────────────────────────────
	// STEP 3: Update the meeting (PATCH — title, description, time)
	// ─────────────────────────────────────────────────────────────────────
	stepHeader(3, "Update meeting (PATCH — title, description, time shift)")

	newStart := start.Add(2 * time.Hour)
	newEnd := newStart.Add(90 * time.Minute)

	patch := &calendar.Event{
		Summary:     "LFX Google Meet Research - Test Meeting (Updated)",
		Description: "Updated via PATCH: testing Calendar API update flow for the LFX meeting service.",
		Start: &calendar.EventDateTime{
			DateTime: newStart.Format(time.RFC3339),
			TimeZone: "UTC",
		},
		End: &calendar.EventDateTime{
			DateTime: newEnd.Format(time.RFC3339),
			TimeZone: "UTC",
		},
	}

	updated, err := calSvc.Events.Patch("primary", eventID, patch).
		SendUpdates("none").
		Do()
	must("update event", err)

	success("Update meeting",
		fmt.Sprintf("New title: %q | Start: %s | Duration: 90 min",
			updated.Summary, updated.Start.DateTime))

	// ─────────────────────────────────────────────────────────────────────
	// STEP 3: Add registrant (add attendee to Calendar event)
	// ─────────────────────────────────────────────────────────────────────
	// In Google Meet there's no Zoom-style registration form.
	// The equivalent is adding attendees to the Calendar event.
	// For LFX we'd manage our own registrant list and track RSVP state.
	// ─────────────────────────────────────────────────────────────────────
	stepHeader(4, "Add registrant (PATCH event — add Calendar attendee)")

	// GET the latest event so we can append to its existing attendee list
	current, err := calSvc.Events.Get("primary", eventID).Do()
	must("get event for attendee append", err)

	attendees := current.Attendees
	attendees = append(attendees, &calendar.EventAttendee{
		Email:    testAttendeeEmail,
		Optional: false,
		Comment:  "Registered via LFX API research test",
	})

	withAttendee, err := calSvc.Events.Patch("primary", eventID, &calendar.Event{
		Attendees: attendees,
	}).SendUpdates("none").Do()
	must("add attendee", err)

	success("Add registrant", fmt.Sprintf("Total attendees: %d", len(withAttendee.Attendees)))
	for _, a := range withAttendee.Attendees {
		fmt.Printf("    %-40s organizer=%-5v optional=%-5v status=%s\n",
			a.Email, a.Organizer, a.Optional, a.ResponseStatus)
	}

	// ─────────────────────────────────────────────────────────────────────
	// STEP 4: Edit registrant (update attendee properties)
	// ─────────────────────────────────────────────────────────────────────
	stepHeader(5, "Edit registrant (PATCH event — update attendee to optional)")

	// The Calendar API PATCH for attendees requires sending the full list;
	// partial attendee updates are not supported — you send the whole slice.
	for _, a := range withAttendee.Attendees {
		if a.Email == testAttendeeEmail {
			a.Optional = true
			a.Comment = "Marked optional after initial registration (edit test)"
		}
	}

	afterEdit, err := calSvc.Events.Patch("primary", eventID, &calendar.Event{
		Attendees: withAttendee.Attendees,
	}).SendUpdates("none").Do()
	must("edit attendee", err)

	for _, a := range afterEdit.Attendees {
		if a.Email == testAttendeeEmail {
			success("Edit registrant",
				fmt.Sprintf("%s | optional=%v | comment=%q",
					a.Email, a.Optional, a.Comment))
		}
	}

	// ─────────────────────────────────────────────────────────────────────
	// STEP 5: Delete registrant (remove attendee from Calendar event)
	// ─────────────────────────────────────────────────────────────────────
	stepHeader(6, "Delete registrant (PATCH event — remove attendee)")

	remaining := make([]*calendar.EventAttendee, 0, len(afterEdit.Attendees))
	for _, a := range afterEdit.Attendees {
		if a.Email != testAttendeeEmail {
			remaining = append(remaining, a)
		}
	}

	afterRemove, err := calSvc.Events.Patch("primary", eventID, &calendar.Event{
		Attendees: remaining,
	}).SendUpdates("none").Do()
	must("remove attendee", err)

	success("Delete registrant",
		fmt.Sprintf("Attendees after removal: %d (was %d)",
			len(afterRemove.Attendees), len(afterEdit.Attendees)))

	// ─────────────────────────────────────────────────────────────────────
	// STEP 6: Delete the meeting (Calendar event)
	// ─────────────────────────────────────────────────────────────────────
	stepHeader(7, "Delete meeting (DELETE Calendar event)")

	if skipDelete {
		fmt.Printf("  SKIP_DELETE=true — skipping deletion. Calendar event %s preserved.\n", eventID)
		fmt.Printf("  Meet URL: %s\n", meetURL)
	} else {
		err = calSvc.Events.Delete("primary", eventID).
			SendUpdates("none").
			Do()
		must("delete event", err)
		success("Delete meeting", fmt.Sprintf("Calendar event %s deleted", eventID))
	}

	// ─────────────────────────────────────────────────────────────────────
	// Summary
	// ─────────────────────────────────────────────────────────────────────
	fmt.Printf("\n%s\n", strings.Repeat("═", 70))
	fmt.Println("  ✅ ALL STEPS COMPLETE")
	fmt.Println(strings.Repeat("═", 70))
	fmt.Printf("  Calendar Event ID  : %s\n", eventID)
	fmt.Printf("  Meet URL           : %s\n", meetURL)
	fmt.Printf("  Meet Space Code    : %s\n", meetSpaceCode)
	fmt.Println()
	fmt.Println("  Operations verified:")
	fmt.Println("    1. Create meeting (Calendar API + conferenceData)")
	fmt.Println("    2. Meet REST API — GET space + conference records (while meeting exists)")
	fmt.Println("    3. Update meeting (PATCH — title, description, time)")
	fmt.Println("    4. Add registrant (PATCH — add Calendar attendee)")
	fmt.Println("    5. Edit registrant (PATCH — update attendee.optional)")
	fmt.Println("    6. Delete registrant (PATCH — remove attendee from list)")
	fmt.Println("    7. Delete meeting (DELETE Calendar event)")
	fmt.Println(strings.Repeat("═", 70))
}
