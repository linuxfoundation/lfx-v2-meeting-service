// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// co-host-assignment: Creates a Google Meet meeting, then probes the Meet REST
// API to understand what host/organizer controls are available via API.
//
// This script tests:
//   1. Create a Calendar event with a Meet link (event creator = host)
//   2. GET the Meet space — log all fields including any host/co-host info
//   3. Attempt PATCH /v2/spaces/{space} with co-host payload — log result
//   4. Attempt PATCH /v2/spaces/{space} with config fields (moderation, access)
//   5. GET the space again — compare before/after state
//   6. Wait (press Ctrl+C) so you can join the meeting and verify host controls
//
// Key research question: can co-host or organizer status be pre-assigned via
// the Meet REST API before the meeting starts?
//
// Required environment variables:
//   GOOGLE_OAUTH_CLIENT_SECRET  - path to OAuth client_secret.json (Desktop app type)
//   COHOST_EMAIL                - email address to attempt to assign as co-host
//   SKIP_DELETE                 - set "true" to keep the calendar event after exit
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
	"https://www.googleapis.com/auth/meetings.space.created",
	"https://www.googleapis.com/auth/meetings.space.readonly",
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
// Meet REST API helpers (raw HTTP, not the generated client)
// ─────────────────────────────────────────────────────────────────────────────

func meetGet(client *http.Client, path string) (map[string]interface{}, error) {
	url := "https://meet.googleapis.com/v2/" + path
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	_ = json.Unmarshal(body, &result)
	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("GET %s → %d", url, resp.StatusCode)
	}
	return result, nil
}

func meetPatch(client *http.Client, path string, updateMask string, payload interface{}) (map[string]interface{}, int, error) {
	url := fmt.Sprintf("https://meet.googleapis.com/v2/%s?updateMask=%s", path, updateMask)
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPatch, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	_ = json.Unmarshal(b, &result)
	return result, resp.StatusCode, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func header(n int, title string) {
	fmt.Printf("\n%s\n", strings.Repeat("═", 70))
	fmt.Printf("  STEP %d: %s\n", n, title)
	fmt.Printf("%s\n", strings.Repeat("═", 70))
}

func subheader(title string) {
	fmt.Printf("\n  ── %s\n", title)
}

func logResult(label string, code int, body map[string]interface{}) {
	status := "✓ OK"
	if code >= 400 {
		status = fmt.Sprintf("✗ ERROR %d", code)
	}
	fmt.Printf("\n  %s  %s\n", status, label)
	if body != nil {
		p, _ := json.MarshalIndent(body, "  ", "  ")
		fmt.Printf("  %s\n", p)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Main
// ─────────────────────────────────────────────────────────────────────────────

func main() {
	clientSecretFile := os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")
	if clientSecretFile == "" {
		log.Fatal("GOOGLE_OAUTH_CLIENT_SECRET is required")
	}
	cohostEmail := os.Getenv("COHOST_EMAIL")
	if cohostEmail == "" {
		log.Fatal("COHOST_EMAIL is required — the email to attempt to assign as co-host")
	}
	skipDelete := os.Getenv("SKIP_DELETE") == "true"

	fmt.Println(strings.Repeat("═", 70))
	fmt.Println("  Google Meet Co-Host Assignment — Research Script")
	fmt.Println(strings.Repeat("═", 70))
	fmt.Printf("  Co-host email: %s\n", cohostEmail)
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

	header(1, "CREATE MEETING")

	start := time.Now().Add(5 * time.Minute).UTC()
	end := start.Add(60 * time.Minute)

	event := &calendar.Event{
		Summary:     "LFX Co-Host Assignment Test",
		Description: "Research script: testing organizer/co-host assignment via Meet REST API.",
		Start:       &calendar.EventDateTime{DateTime: start.Format(time.RFC3339), TimeZone: "UTC"},
		End:         &calendar.EventDateTime{DateTime: end.Format(time.RFC3339), TimeZone: "UTC"},
		Attendees:   []*calendar.EventAttendee{{Email: cohostEmail}},
		ConferenceData: &calendar.ConferenceData{
			CreateRequest: &calendar.CreateConferenceRequest{
				RequestId: fmt.Sprintf("cohost-research-%d", time.Now().UnixNano()),
				ConferenceSolutionKey: &calendar.ConferenceSolutionKey{
					Type: "hangoutsMeet",
				},
			},
		},
	}

	created, err := calSvc.Events.Insert("primary", event).
		ConferenceDataVersion(1).
		SendUpdates("all").
		Context(ctx).
		Do()
	if err != nil {
		log.Fatalf("Create event: %v", err)
	}

	meetLink := created.HangoutLink
	meetCode := ""
	for _, ep := range created.ConferenceData.EntryPoints {
		if ep.EntryPointType == "video" {
			meetCode = ep.MeetingCode
		}
	}
	if meetCode == "" && meetLink != "" {
		// Extract code from the link: https://meet.google.com/abc-defg-hij
		parts := strings.Split(meetLink, "/")
		meetCode = parts[len(parts)-1]
	}

	fmt.Printf("\n  Event ID:   %s\n", created.Id)
	fmt.Printf("  Meet link:  %s\n", meetLink)
	fmt.Printf("  Meet code:  %s\n", meetCode)
	fmt.Printf("  Invited:    %s (Calendar attendee)\n", cohostEmail)

	// ── Step 2: Resolve internal space name ───────────────────────────────────

	header(2, "GET MEET SPACE (before patch)")

	spaceBefore, err := meetGet(httpClient, "spaces/"+meetCode)
	if err != nil {
		log.Fatalf("GET space: %v", err)
	}
	spaceName, _ := spaceBefore["name"].(string)
	fmt.Printf("\n  Internal space name: %s\n", spaceName)
	fmt.Println("\n  All space fields returned by the API:")
	p, _ := json.MarshalIndent(spaceBefore, "  ", "  ")
	fmt.Printf("  %s\n", p)

	spacePath := strings.TrimPrefix(spaceName, "/") // "spaces/XYZ"

	// ── Step 3: Attempt co-host assignment (various approaches) ───────────────

	header(3, "ATTEMPT CO-HOST ASSIGNMENT VIA MEET REST API")
	fmt.Println("  The public Meet API documents these updateMask fields for spaces.patch:")
	fmt.Println("    config.accessType, config.entryPointAccess, config.moderationEnabled")
	fmt.Println("  We will try several payloads to discover co-host API surface.")

	// 3a: Try undocumented coHosts field
	subheader("3a: PATCH with coHosts field (email-based)")
	_, code3a, _ := meetPatch(httpClient, spacePath, "coHosts", map[string]interface{}{
		"coHosts": []map[string]interface{}{
			{"email": cohostEmail},
		},
	})
	fmt.Printf("  Result: HTTP %d (200=success, 400=bad field, 403=forbidden)\n", code3a)

	// 3b: Try coHosts with signedInUser format
	subheader("3b: PATCH with coHosts field (signedInUser format)")
	_, code3b, _ := meetPatch(httpClient, spacePath, "coHosts", map[string]interface{}{
		"coHosts": []map[string]interface{}{
			{"signedInUser": map[string]interface{}{"user": "users/me", "displayName": ""}},
		},
	})
	fmt.Printf("  Result: HTTP %d\n", code3b)

	// 3c: Try enabling moderation (a documented field) — this restricts who can share screen/mute
	subheader("3c: PATCH config.moderationEnabled=true (documented field)")
	afterMod, codeMod, errMod := meetPatch(httpClient, spacePath, "config.moderationEnabled", map[string]interface{}{
		"config": map[string]interface{}{
			"moderationEnabled": true,
		},
	})
	if errMod != nil {
		fmt.Printf("  Error: %v\n", errMod)
	} else {
		logResult("moderationEnabled patch", codeMod, afterMod)
	}

	// 3d: Try setting access type to TRUSTED (limits who can join without knocking)
	subheader("3d: PATCH config.accessType=TRUSTED (documented field)")
	afterAccess, codeAccess, errAccess := meetPatch(httpClient, spacePath, "config.accessType", map[string]interface{}{
		"config": map[string]interface{}{
			"accessType": "TRUSTED",
		},
	})
	if errAccess != nil {
		fmt.Printf("  Error: %v\n", errAccess)
	} else {
		logResult("accessType patch", codeAccess, afterAccess)
	}

	// ── Step 4: GET space after patches ──────────────────────────────────────

	header(4, "GET MEET SPACE (after patches)")

	spaceAfter, err := meetGet(httpClient, spacePath)
	if err != nil {
		fmt.Printf("  GET space error: %v\n", err)
	} else {
		fmt.Println("\n  Space fields after all patches:")
		p, _ := json.MarshalIndent(spaceAfter, "  ", "  ")
		fmt.Printf("  %s\n", p)

		// Compare config before/after
		fmt.Println("\n  Config comparison:")
		beforeConfig, _ := spaceBefore["config"].(map[string]interface{})
		afterConfig, _ := spaceAfter["config"].(map[string]interface{})
		if beforeConfig == nil {
			fmt.Println("    before: no config field returned")
		} else {
			bp, _ := json.MarshalIndent(beforeConfig, "    ", "  ")
			fmt.Printf("    before: %s\n", bp)
		}
		if afterConfig == nil {
			fmt.Println("    after:  no config field returned")
		} else {
			ap, _ := json.MarshalIndent(afterConfig, "    ", "  ")
			fmt.Printf("    after:  %s\n", ap)
		}
	}

	// ── Step 5: Summary and join instructions ────────────────────────────────

	header(5, "FINDINGS SUMMARY")

	fmt.Println()
	fmt.Printf("  co-host email tested:   %s\n", cohostEmail)
	fmt.Printf("  coHosts email payload:  HTTP %d ", code3a)
	if code3a == 200 {
		fmt.Println("→ co-host field EXISTS and was accepted")
	} else if code3a == 400 {
		fmt.Println("→ field not recognised (not in API surface)")
	} else if code3a == 403 {
		fmt.Println("→ field exists but not permitted with current scope")
	} else {
		fmt.Printf("→ unexpected status\n")
	}
	fmt.Printf("  coHosts signedInUser:   HTTP %d\n", code3b)
	fmt.Printf("  moderationEnabled:      HTTP %d\n", codeMod)
	fmt.Printf("  accessType TRUSTED:     HTTP %d\n", codeAccess)

	fmt.Println()
	fmt.Println("  ► Now join the meeting as BOTH accounts to verify in-meeting controls.")
	fmt.Printf("    Join link: %s\n", meetLink)
	fmt.Println("    Check: does the co-host account see 'Host Controls'?")
	fmt.Println("    Check: can the co-host mute/remove others?")
	fmt.Println("    Check: does enabling moderation restrict participant controls?")
	fmt.Println()
	fmt.Println("  Press Ctrl+C when done to clean up the event.")
	fmt.Println(strings.Repeat("═", 70))

	<-ctx.Done()

	// ── Step 6: Delete event ──────────────────────────────────────────────────

	header(6, "CLEANUP")

	if skipDelete {
		fmt.Printf("  SKIP_DELETE=true — keeping event %s\n", created.Id)
		fmt.Printf("  Delete manually: https://calendar.google.com\n")
		return
	}

	if err := calSvc.Events.Delete("primary", created.Id).SendUpdates("all").Do(); err != nil {
		log.Printf("  Delete event error: %v", err)
	} else {
		fmt.Printf("  Event %s deleted.\n", created.Id)
	}
}
