// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT
// Google Meet Event Poller — Research Script
//
// Polls the Meet REST API to detect meeting lifecycle events in real time:
//   - Meeting started (new conference record appears)
//   - Meeting ended (conference record gets an endTime)
//   - Participant joined (new participant session appears)
//   - Participant left (participant session gets an endTime)
//
// Required environment variables:
//   GOOGLE_OAUTH_CLIENT_SECRET  - path to OAuth client_secret.json
//   MEET_SPACE_CODE             - Meet space code (e.g. abc-defg-hij from the meet URL)
//   POLL_INTERVAL               - polling interval in seconds (default: 5)
//
// Usage:
//   go run main.go
//   Then join the meeting to see events appear.

package main

import (
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
)

const tokenFile = "../google-meeting-research/token.json"

var oauthScopes = []string{
	"https://www.googleapis.com/auth/meetings.space.readonly",
	"https://www.googleapis.com/auth/meetings.space.created",
	"https://www.googleapis.com/auth/calendar",
	"https://www.googleapis.com/auth/contacts.readonly",
	"https://www.googleapis.com/auth/directory.readonly",
}

// ─────────────────────────────────────────────────────────────────────────────
// Auth (reuses token from google-meeting-research)
// ─────────────────────────────────────────────────────────────────────────────

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
	return cfg.Client(ctx, tok)
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
		return
	}
	defer f.Close()
	_ = json.NewEncoder(f).Encode(tok)
}

func runOAuthFlow(ctx context.Context, config *oauth2.Config) *oauth2.Token {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		log.Fatalf("Start callback server: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	config.RedirectURL = fmt.Sprintf("http://localhost:%d", port)
	codeCh := make(chan string, 1)
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, "<h2>Authorization complete — you can close this tab.</h2>")
			codeCh <- r.URL.Query().Get("code")
		}),
	}
	go func() { _ = srv.Serve(listener) }()
	authURL := config.AuthCodeURL("state", oauth2.AccessTypeOffline)
	fmt.Printf("  Opening browser for auth: %s\n", authURL)
	_ = exec.Command("open", authURL).Start()
	tok, err := config.Exchange(ctx, <-codeCh)
	_ = srv.Shutdown(ctx)
	if err != nil {
		log.Fatalf("Token exchange: %v", err)
	}
	return tok
}

// ─────────────────────────────────────────────────────────────────────────────
// Meet API types
// ─────────────────────────────────────────────────────────────────────────────

type ConferenceRecord struct {
	Name      string `json:"name"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Space     string `json:"space"`
}

type ConferenceRecordsResponse struct {
	ConferenceRecords []ConferenceRecord `json:"conferenceRecords"`
}

type ParticipantSession struct {
	Name      string `json:"name"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

type Participant struct {
	Name                string               `json:"name"`
	DisplayName         string               `json:"displayName"`
	ParticipantSessions []ParticipantSession `json:"signedinUser"`
}

type ParticipantsResponse struct {
	Participants []Participant `json:"participants"`
}

// ─────────────────────────────────────────────────────────────────────────────
// API helpers
// ─────────────────────────────────────────────────────────────────────────────

func get(client *http.Client, url string, out interface{}) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s → %d: %s", url, resp.StatusCode, string(body))
	}
	return json.Unmarshal(body, out)
}

func resolveSpaceName(client *http.Client, meetingCode string) (string, error) {
	url := fmt.Sprintf("https://meet.googleapis.com/v2/spaces/%s", meetingCode)
	var space map[string]interface{}
	if err := get(client, url, &space); err != nil {
		return "", err
	}
	name, _ := space["name"].(string)
	if name == "" {
		return "", fmt.Errorf("space response missing 'name'")
	}
	return name, nil
}

func listConferenceRecords(client *http.Client, spaceName string) ([]ConferenceRecord, error) {
	url := fmt.Sprintf(
		"https://meet.googleapis.com/v2/conferenceRecords?filter=space.name%%3D%%22%s%%22",
		spaceName,
	)
	var resp ConferenceRecordsResponse
	if err := get(client, url, &resp); err != nil {
		return nil, err
	}
	return resp.ConferenceRecords, nil
}

func listParticipants(client *http.Client, conferenceRecord string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("https://meet.googleapis.com/v2/%s/participants", conferenceRecord)
	var result map[string]interface{}
	if err := get(client, url, &result); err != nil {
		return nil, err
	}
	raw, _ := result["participants"].([]interface{})
	participants := make([]map[string]interface{}, 0, len(raw))
	for _, p := range raw {
		if pm, ok := p.(map[string]interface{}); ok {
			participants = append(participants, pm)
		}
	}
	return participants, nil
}

func listParticipantSessions(client *http.Client, participantName string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("https://meet.googleapis.com/v2/%s/participantSessions", participantName)
	var result map[string]interface{}
	if err := get(client, url, &result); err != nil {
		return nil, err
	}
	raw, _ := result["participantSessions"].([]interface{})
	sessions := make([]map[string]interface{}, 0, len(raw))
	for _, s := range raw {
		if sm, ok := s.(map[string]interface{}); ok {
			sessions = append(sessions, sm)
		}
	}
	return sessions, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Event detection (diff-based)
// ─────────────────────────────────────────────────────────────────────────────

func logEvent(emoji, label, detail string) {
	fmt.Printf("\n%s  [%s] %s\n     %s\n",
		emoji, time.Now().Format("15:04:05"), label, detail)
}

type state struct {
	// conference record name → whether we've seen it start and end
	conferenceSeen  map[string]bool
	conferenceEnded map[string]bool
	// participant name → whether we've seen them join and leave
	participantSeen map[string]bool
	participantLeft map[string]bool
	// participant session name → seen
	sessionSeen  map[string]bool
	sessionEnded map[string]bool
}

func newState() *state {
	return &state{
		conferenceSeen:  make(map[string]bool),
		conferenceEnded: make(map[string]bool),
		participantSeen: make(map[string]bool),
		participantLeft: make(map[string]bool),
		sessionSeen:     make(map[string]bool),
		sessionEnded:    make(map[string]bool),
	}
}

func (s *state) poll(client *http.Client, spaceName string) {
	records, err := listConferenceRecords(client, spaceName)
	if err != nil {
		fmt.Printf("  ⚠ list conference records: %v\n", err)
		return
	}

	for _, rec := range records {
		// Meeting started
		if !s.conferenceSeen[rec.Name] {
			s.conferenceSeen[rec.Name] = true
			logEvent("🟢", "MEETING STARTED",
				fmt.Sprintf("conference=%s  started=%s", rec.Name, rec.StartTime))
		}

		// Meeting ended
		if rec.EndTime != "" && !s.conferenceEnded[rec.Name] {
			s.conferenceEnded[rec.Name] = true
			logEvent("🔴", "MEETING ENDED",
				fmt.Sprintf("conference=%s  ended=%s", rec.Name, rec.EndTime))
		}

		// Participants
		participants, err := listParticipants(client, rec.Name)
		if err != nil {
			fmt.Printf("  ⚠ list participants for %s: %v\n", rec.Name, err)
			continue
		}

		for _, p := range participants {
			pName, _ := p["name"].(string)
			displayName := participantDisplayName(p)

			if !s.participantSeen[pName] {
				s.participantSeen[pName] = true
				logEvent("👤", "PARTICIPANT JOINED",
					fmt.Sprintf("name=%s  display=%q  conference=%s", pName, displayName, rec.Name))

				// Extract Google account ID from participant name and look up profile
				// Participant name format: conferenceRecords/.../participants/{googleAccountId}
				parts := strings.Split(pName, "/")
				if googleAccountID := parts[len(parts)-1]; googleAccountID != "" {
					lookupPersonProfile(client, googleAccountID)
				}
			}

			// Fetch participant sessions for join/leave times
			sessions, err := listParticipantSessions(client, pName)
			if err != nil {
				continue
			}
			for _, sess := range sessions {
				sessName, _ := sess["name"].(string)
				endTime, _ := sess["endTime"].(string)

				if !s.sessionSeen[sessName] {
					s.sessionSeen[sessName] = true
					startTime, _ := sess["startTime"].(string)
					logEvent("➡️ ", "SESSION STARTED",
						fmt.Sprintf("participant=%q  session=%s  joined=%s", displayName, sessName, startTime))
				}

				if endTime != "" && !s.sessionEnded[sessName] {
					s.sessionEnded[sessName] = true
					logEvent("⬅️ ", "SESSION ENDED",
						fmt.Sprintf("participant=%q  session=%s  left=%s", displayName, sessName, endTime))
				}
			}

			// Mark participant as left if all their sessions have ended
			allLeft := len(sessions) > 0
			for _, sess := range sessions {
				if endTime, _ := sess["endTime"].(string); endTime == "" {
					allLeft = false
					break
				}
			}
			if allLeft && !s.participantLeft[pName] {
				s.participantLeft[pName] = true
				logEvent("🚪", "PARTICIPANT LEFT",
					fmt.Sprintf("name=%s  display=%q", pName, displayName))
			}
		}
	}
}

func lookupPersonProfile(client *http.Client, googleAccountID string) {
	url := fmt.Sprintf(
		"https://people.googleapis.com/v1/people/%s?personFields=emailAddresses,names,photos,metadata",
		googleAccountID,
	)
	fmt.Printf("\n  [People API] GET %s\n", url)
	resp, err := client.Get(url)
	if err != nil {
		fmt.Printf("  [People API] request error: %v\n", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var pretty interface{}
	if json.Unmarshal(body, &pretty) == nil {
		b, _ := json.MarshalIndent(pretty, "  ", "  ")
		fmt.Printf("  [People API] %d response:\n  %s\n", resp.StatusCode, b)
	} else {
		fmt.Printf("  [People API] %d response: %s\n", resp.StatusCode, string(body))
	}
}

func participantDisplayName(p map[string]interface{}) string {
	// Signed-in user
	if su, ok := p["signedinUser"].(map[string]interface{}); ok {
		if dn, ok := su["displayName"].(string); ok && dn != "" {
			return dn
		}
	}
	// Anonymous user
	if au, ok := p["anonymousUser"].(map[string]interface{}); ok {
		if dn, ok := au["displayName"].(string); ok && dn != "" {
			return dn + " (anon)"
		}
	}
	if dn, ok := p["displayName"].(string); ok && dn != "" {
		return dn
	}
	return "(unknown)"
}

// ─────────────────────────────────────────────────────────────────────────────
// Main
// ─────────────────────────────────────────────────────────────────────────────

func main() {
	clientSecretFile := os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")
	if clientSecretFile == "" {
		log.Fatal("GOOGLE_OAUTH_CLIENT_SECRET is required")
	}
	spaceCode := os.Getenv("MEET_SPACE_CODE")
	if spaceCode == "" {
		log.Fatal("MEET_SPACE_CODE is required (e.g. abc-defg-hij from the Meet URL)")
	}
	pollInterval := 5 * time.Second
	if v := os.Getenv("POLL_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v + "s"); err == nil {
			pollInterval = d
		}
	}

	fmt.Println(strings.Repeat("═", 70))
	fmt.Println("  Google Meet Event Poller")
	fmt.Println(strings.Repeat("═", 70))
	fmt.Printf("  Meet space:    %s\n", spaceCode)
	fmt.Printf("  Poll interval: %s\n", pollInterval)
	fmt.Println(strings.Repeat("═", 70))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client := getOAuthClient(ctx, clientSecretFile)

	// Resolve the internal space name
	fmt.Printf("\n  Resolving space name for %s...\n", spaceCode)
	spaceName, err := resolveSpaceName(client, spaceCode)
	if err != nil {
		log.Fatalf("Resolve space name: %v", err)
	}
	fmt.Printf("  Space name: %s\n", spaceName)
	fmt.Printf("\n  Polling every %s — join the meeting now to see events.\n", pollInterval)
	fmt.Println("  Press Ctrl+C to stop.")
	fmt.Println(strings.Repeat("═", 70))

	s := newState()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Poll immediately on start
	s.poll(client, spaceName)

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\n  Stopped.")
			return
		case <-ticker.C:
			s.poll(client, spaceName)
		}
	}
}
