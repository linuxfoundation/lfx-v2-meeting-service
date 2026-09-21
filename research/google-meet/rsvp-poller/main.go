// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// rsvp-poller: Creates a Google Meet meeting with an invitee, then polls the
// Calendar API to detect when the invitee submits an RSVP response
// (accept / decline / tentative).  Logs the full attendee payload on every
// status change so we can evaluate what data is available to store.
//
// The user runs the script, shares the meeting invite with an attendee (or
// responds from another Google account), and watches the terminal for events.
//
// Required environment variables:
//   GOOGLE_OAUTH_CLIENT_SECRET  - path to OAuth client_secret.json (Desktop app type)
//   TEST_ATTENDEE_EMAIL         - email address to invite
//   POLL_INTERVAL               - polling interval in seconds (default: 10)
//   SKIP_DELETE                 - set "true" to keep the calendar event after exit
//
// First run opens a browser for OAuth consent.  The token is saved to
// token.json and reused on subsequent runs.
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
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// ─────────────────────────────────────────────────────────────────────────────
// Logging transport
// ─────────────────────────────────────────────────────────────────────────────

type LoggingTransport struct{ base http.RoundTripper }

func (t *LoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	fmt.Printf("\n%s\n", strings.Repeat("─", 70))
	fmt.Printf("► %s %s\n", req.Method, req.URL)
	for k, vs := range req.Header {
		if strings.EqualFold(k, "authorization") {
			fmt.Printf("  Header: %s: Bearer <token>\n", k)
		} else {
			fmt.Printf("  Header: %s: %s\n", k, strings.Join(vs, ", "))
		}
	}
	if req.Body != nil && req.Body != http.NoBody {
		b, _ := io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(b))
		var pretty interface{}
		if json.Unmarshal(b, &pretty) == nil {
			p, _ := json.MarshalIndent(pretty, "  ", "  ")
			fmt.Printf("  Request body:\n  %s\n", p)
		} else {
			fmt.Printf("  Request body (raw): %s\n", b)
		}
	}
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		fmt.Printf("◄ TRANSPORT ERROR: %v\n", err)
		return nil, err
	}
	fmt.Printf("◄ %d %s\n", resp.StatusCode, resp.Status)
	b, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewReader(b))
	var pretty interface{}
	if json.Unmarshal(b, &pretty) == nil {
		p, _ := json.MarshalIndent(pretty, "  ", "  ")
		fmt.Printf("  Response body:\n  %s\n", p)
	} else {
		fmt.Printf("  Response body (raw): %s\n", b)
	}
	return resp, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// OAuth helpers
// ─────────────────────────────────────────────────────────────────────────────

const tokenFile = "token.json"

var oauthScopes = []string{
	"https://www.googleapis.com/auth/calendar",
	"https://www.googleapis.com/auth/calendar.events",
}

func getOAuthClient(ctx context.Context, clientSecretFile string) *http.Client {
	data, err := os.ReadFile(clientSecretFile)
	if err != nil {
		log.Fatalf("Read client secret: %v", err)
	}
	cfg, err := google.ConfigFromJSON(data, oauthScopes...)
	if err != nil {
		log.Fatalf("Parse client secret: %v", err)
	}
	tok, err := loadToken()
	if err != nil {
		tok = runOAuthFlow(ctx, cfg)
		saveToken(tok)
	}
	base := cfg.Client(ctx, tok)
	base.Transport = &LoggingTransport{base: base.Transport}
	return base
}

func loadToken() (*oauth2.Token, error) {
	f, err := os.Open(tokenFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	return tok, json.NewDecoder(f).Decode(tok)
}

func saveToken(tok *oauth2.Token) {
	f, err := os.Create(tokenFile)
	if err != nil {
		log.Printf("Warning: could not save token: %v", err)
		return
	}
	defer f.Close()
	_ = json.NewEncoder(f).Encode(tok)
	fmt.Printf("  Token saved to %s\n", tokenFile)
}

func runOAuthFlow(ctx context.Context, cfg *oauth2.Config) *oauth2.Token {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		log.Fatalf("Start callback server: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	cfg.RedirectURL = fmt.Sprintf("http://localhost:%d", port)
	codeCh := make(chan string, 1)
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, "<h2>Authorization complete — you can close this tab.</h2>")
			codeCh <- r.URL.Query().Get("code")
		}),
	}
	go func() { _ = srv.Serve(listener) }()
	authURL := cfg.AuthCodeURL("state", oauth2.AccessTypeOffline)
	fmt.Printf("  Opening browser for auth: %s\n", authURL)
	_ = exec.Command("open", authURL).Start()
	tok, err := cfg.Exchange(ctx, <-codeCh)
	_ = srv.Shutdown(ctx)
	if err != nil {
		log.Fatalf("Token exchange: %v", err)
	}
	return tok
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func header(n int, title string) {
	fmt.Printf("\n%s\n", strings.Repeat("═", 70))
	fmt.Printf("  STEP %d: %s\n", n, title)
	fmt.Printf("%s\n", strings.Repeat("═", 70))
}

func logRSVPEvent(label string, attendee *calendar.EventAttendee) {
	fmt.Printf("\n%s\n", strings.Repeat("★", 70))
	fmt.Printf("  %s [%s]\n", label, time.Now().Format("15:04:05"))
	fmt.Printf("%s\n", strings.Repeat("★", 70))

	// Marshal the attendee to JSON for full visibility
	raw, _ := json.MarshalIndent(attendee, "  ", "  ")
	fmt.Printf("  Attendee payload:\n  %s\n", raw)

	fmt.Printf("\n  Parsed fields:\n")
	fmt.Printf("    email:          %s\n", attendee.Email)
	fmt.Printf("    displayName:    %s\n", attendee.DisplayName)
	fmt.Printf("    responseStatus: %s\n", attendee.ResponseStatus)
	fmt.Printf("    organizer:      %v\n", attendee.Organizer)
	fmt.Printf("    self:           %v\n", attendee.Self)
	fmt.Printf("    optional:       %v\n", attendee.Optional)
	fmt.Printf("    comment:        %q\n", attendee.Comment)

	fmt.Printf("\n  LFX mapping:\n")
	fmt.Printf("    is_invited:  true  (attendee exists on event)\n")
	switch attendee.ResponseStatus {
	case "accepted":
		fmt.Printf("    is_attended: true  (responseStatus=accepted)\n")
	case "declined":
		fmt.Printf("    is_attended: false (responseStatus=declined)\n")
	case "tentative":
		fmt.Printf("    is_attended: nil   (responseStatus=tentative — awaiting final answer)\n")
	default:
		fmt.Printf("    is_attended: nil   (responseStatus=%s — not yet responded)\n", attendee.ResponseStatus)
	}
	fmt.Printf("%s\n", strings.Repeat("★", 70))
}

// ─────────────────────────────────────────────────────────────────────────────
// RSVP poller
// ─────────────────────────────────────────────────────────────────────────────

// knownStatus tracks the last-seen responseStatus per attendee email.
type rsvpState struct {
	status map[string]string // email → responseStatus
}

func newRSVPState() *rsvpState {
	return &rsvpState{status: make(map[string]string)}
}

// poll fetches the current event and fires logRSVPEvent on any status change.
// Returns true if all non-organizer attendees have given a final answer
// (accepted or declined).
func (s *rsvpState) poll(ctx context.Context, svc *calendar.Service, eventID string) bool {
	ev, err := svc.Events.Get("primary", eventID).Context(ctx).Do()
	if err != nil {
		fmt.Printf("  ⚠ poll error: %v\n", err)
		return false
	}

	allFinal := true
	for _, a := range ev.Attendees {
		if a.Organizer {
			continue // skip the organizer's own entry
		}
		prev, seen := s.status[a.Email]
		if !seen {
			// First time we see this attendee
			s.status[a.Email] = a.ResponseStatus
			logRSVPEvent("INITIAL STATUS", a)
		} else if prev != a.ResponseStatus {
			// Status changed
			s.status[a.Email] = a.ResponseStatus
			logRSVPEvent(fmt.Sprintf("RSVP CHANGED: %s → %s", prev, a.ResponseStatus), a)
		}
		if a.ResponseStatus != "accepted" && a.ResponseStatus != "declined" {
			allFinal = false
		}
	}
	return allFinal
}

// ─────────────────────────────────────────────────────────────────────────────
// Main
// ─────────────────────────────────────────────────────────────────────────────

func main() {
	clientSecretFile := os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")
	if clientSecretFile == "" {
		log.Fatal("GOOGLE_OAUTH_CLIENT_SECRET is required")
	}
	attendeeEmail := os.Getenv("TEST_ATTENDEE_EMAIL")
	if attendeeEmail == "" {
		log.Fatal("TEST_ATTENDEE_EMAIL is required")
	}
	skipDelete := os.Getenv("SKIP_DELETE") == "true"

	pollInterval := 10 * time.Second
	if v := os.Getenv("POLL_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v + "s"); err == nil {
			pollInterval = d
		}
	}

	fmt.Println(strings.Repeat("═", 70))
	fmt.Println("  Google Meet RSVP Poller — Research Script")
	fmt.Println(strings.Repeat("═", 70))
	fmt.Printf("  Inviting:      %s\n", attendeeEmail)
	fmt.Printf("  Poll interval: %s\n", pollInterval)
	fmt.Printf("  Skip delete:   %v\n", skipDelete)
	fmt.Println(strings.Repeat("═", 70))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpClient := getOAuthClient(ctx, clientSecretFile)
	calSvc, err := calendar.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		log.Fatalf("Create Calendar service: %v", err)
	}

	// ── Step 1: Create meeting ────────────────────────────────────────────────

	header(1, "CREATE MEETING WITH INVITEE")

	start := time.Now().Add(10 * time.Minute).UTC()
	end := start.Add(30 * time.Minute)

	event := &calendar.Event{
		Summary:     "LFX RSVP Test Meeting",
		Description: "Research script: testing RSVP / attendee response via Calendar API.",
		Start:       &calendar.EventDateTime{DateTime: start.Format(time.RFC3339), TimeZone: "UTC"},
		End:         &calendar.EventDateTime{DateTime: end.Format(time.RFC3339), TimeZone: "UTC"},
		Attendees: []*calendar.EventAttendee{
			{Email: attendeeEmail},
		},
		ConferenceData: &calendar.ConferenceData{
			CreateRequest: &calendar.CreateConferenceRequest{
				RequestId: fmt.Sprintf("rsvp-research-%d", time.Now().UnixNano()),
				ConferenceSolutionKey: &calendar.ConferenceSolutionKey{
					Type: "hangoutsMeet",
				},
			},
		},
	}

	// sendUpdates=all so the invitee actually receives a calendar email they can RSVP from.
	created, err := calSvc.Events.Insert("primary", event).
		ConferenceDataVersion(1).
		SendUpdates("all").
		Do()
	if err != nil {
		log.Fatalf("Create event: %v", err)
	}

	fmt.Printf("\n  Event created: %s\n", created.Id)
	fmt.Printf("  Summary:       %s\n", created.Summary)
	fmt.Printf("  Start:         %s\n", created.Start.DateTime)
	if created.HangoutLink != "" {
		fmt.Printf("  Meet link:     %s\n", created.HangoutLink)
	}
	fmt.Printf("\n  ► Invite sent to %s via Google Calendar email.\n", attendeeEmail)
	fmt.Printf("    The invitee can respond via the email or at:\n")
	fmt.Printf("    https://calendar.google.com\n")

	// ── Step 2: Log initial attendee state ───────────────────────────────────

	header(2, "INITIAL ATTENDEE STATE")

	for _, a := range created.Attendees {
		fmt.Printf("\n  Attendee: %s\n", a.Email)
		raw, _ := json.MarshalIndent(a, "  ", "  ")
		fmt.Printf("  %s\n", raw)
	}

	// ── Step 3: Poll for RSVP changes ────────────────────────────────────────

	header(3, "POLLING FOR RSVP RESPONSES")
	fmt.Printf("  Polling every %s — press Ctrl+C to stop.\n", pollInterval)
	fmt.Println(strings.Repeat("═", 70))

	state := newRSVPState()
	// Seed initial state from the created event (avoids firing "INITIAL STATUS" twice)
	for _, a := range created.Attendees {
		if !a.Organizer {
			state.status[a.Email] = a.ResponseStatus
		}
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\n  Stopping poller...")
			goto cleanup
		case <-ticker.C:
			fmt.Printf("  [%s] polling...\n", time.Now().Format("15:04:05"))
			allFinal := state.poll(ctx, calSvc, created.Id)
			if allFinal {
				fmt.Println("\n  All attendees have responded. Stopping poller.")
				goto cleanup
			}
		}
	}

cleanup:
	// ── Step 4: Final attendee state ─────────────────────────────────────────

	header(4, "FINAL ATTENDEE STATE")

	final, err := calSvc.Events.Get("primary", created.Id).Do()
	if err != nil {
		fmt.Printf("  Could not fetch final state: %v\n", err)
	} else {
		for _, a := range final.Attendees {
			raw, _ := json.MarshalIndent(a, "  ", "  ")
			fmt.Printf("\n  Attendee: %s\n  %s\n", a.Email, raw)
		}
	}

	// ── Step 5: Delete event ──────────────────────────────────────────────────

	if skipDelete {
		fmt.Printf("\n  SKIP_DELETE=true — keeping event %s\n", created.Id)
		fmt.Printf("  Delete manually: https://calendar.google.com\n")
		return
	}

	header(5, "DELETE EVENT")

	if err := calSvc.Events.Delete("primary", created.Id).SendUpdates("all").Do(); err != nil {
		log.Printf("  Delete event error: %v", err)
	} else {
		fmt.Printf("  Event %s deleted. Cancellation notice sent to attendees.\n", created.Id)
	}
}
