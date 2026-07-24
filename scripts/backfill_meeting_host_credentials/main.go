// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// backfill_meeting_host_credentials scrolls all v1_meeting documents in OpenSearch
// that still carry a host_key in their data payload and publishes a corresponding
// v1_meeting_host_credentials document to the indexer via NATS for each one.
//
// Background: host_key was moved out of the Meeting object into a separate
// MeetingHostCredentials object (LFXV2-2358). Meetings indexed before that change
// still have data.host_key in OpenSearch but no companion credentials document.
// This script creates those missing credentials documents.
//
// Usage:
//
//	OPENSEARCH_URL=http://localhost:9200 NATS_URL=nats://localhost:4222 \
//	  go run ./scripts/backfill_meeting_host_credentials/
//
// Flags:
//
//	-update      Actually publish credentials documents to NATS (default: false, dry-run only)
//	-page-size   Documents per scroll page (default: 200)
//
// Environment variables:
//
//	OPENSEARCH_URL  OpenSearch base URL (default: http://localhost:9200)
//	NATS_URL        NATS server URL (default: nats://127.0.0.1:4222)
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	osIndex           = "resources"
	natsSubject       = "lfx.index.v1_meeting_host_credentials"
	authHeaderValue   = "Bearer lfx-v2-meeting-service"
	scrollPageSize    = 200
	natsPublishAction = "updated"
)

// osHit holds the fields we need from each v1_meeting OpenSearch document.
type osHit struct {
	Source struct {
		ObjectID string `json:"object_id"`
		Data     struct {
			HostKey string `json:"host_key"`
		} `json:"data"`
	} `json:"_source"`
}

type osScrollResponse struct {
	ScrollID string `json:"_scroll_id"`
	Hits     struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []osHit `json:"hits"`
	} `json:"hits"`
}

// indexingConfig mirrors the indexer service's IndexingConfig struct.
type indexingConfig struct {
	ObjectID             string   `json:"object_id"`
	Public               bool     `json:"public"`
	AccessCheckObject    string   `json:"access_check_object"`
	AccessCheckRelation  string   `json:"access_check_relation"`
	HistoryCheckObject   string   `json:"history_check_object"`
	HistoryCheckRelation string   `json:"history_check_relation"`
	ParentRefs           []string `json:"parent_refs"`
	Tags                 []string `json:"tags"`
	SortName             string   `json:"sort_name"`
	NameAndAliases       []string `json:"name_and_aliases"`
	Fulltext             string   `json:"fulltext"`
}

// credentialsData is the data payload for a v1_meeting_host_credentials document.
type credentialsData struct {
	MeetingID string `json:"meeting_id"`
	HostKey   string `json:"host_key"`
}

// indexerEnvelope is the message shape the indexer service consumes from NATS.
type indexerEnvelope struct {
	Action         string            `json:"action"`
	Headers        map[string]string `json:"headers"`
	Data           credentialsData   `json:"data"`
	Tags           []string          `json:"tags"`
	IndexingConfig indexingConfig    `json:"indexing_config"`
}

func main() {
	update := flag.Bool("update", false, "publish credentials documents to NATS (default: dry-run, logs only)")
	pageSize := flag.Int("page-size", scrollPageSize, "documents per scroll page")
	flag.Parse()

	osURL := os.Getenv("OPENSEARCH_URL")
	if osURL == "" {
		osURL = "http://localhost:9200"
	}
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}

	slog.Info("backfill_meeting_host_credentials starting",
		"opensearch_url", osURL,
		"nats_url", natsURL,
		"update", *update,
		"page_size", *pageSize,
	)

	ctx := context.Background()
	httpClient := &http.Client{Timeout: 30 * time.Second}

	var nc *nats.Conn
	if *update {
		var err error
		nc, err = nats.Connect(natsURL,
			nats.Timeout(10*time.Second),
			nats.MaxReconnects(5),
			nats.ReconnectWait(2*time.Second),
		)
		if err != nil {
			slog.ErrorContext(ctx, "failed to connect to NATS", "error", err)
			os.Exit(1)
		}
		defer nc.Close()
	}

	os.Exit(run(ctx, httpClient, nc, osURL, *update, *pageSize))
}

func run(ctx context.Context, httpClient *http.Client, nc *nats.Conn, osURL string, update bool, pageSize int) int {
	scrollID, firstPage, total, err := openScroll(ctx, httpClient, osURL, pageSize)
	if err != nil {
		slog.ErrorContext(ctx, "failed to open scroll", "error", err)
		return 1
	}
	defer deleteScroll(ctx, httpClient, osURL, scrollID) //nolint:errcheck

	slog.InfoContext(ctx, "found v1_meeting documents with host_key", "total", total)
	if total == 0 {
		slog.InfoContext(ctx, "nothing to backfill")
		return 0
	}

	var published, skipped, failed int

	page := firstPage
	for len(page) > 0 {
		p, s, f := processPage(ctx, nc, page, update)
		published += p
		skipped += s
		failed += f

		slog.InfoContext(ctx, "progress", "published", published, "skipped", skipped, "failed", failed)

		page, scrollID, err = nextScrollPage(ctx, httpClient, osURL, scrollID)
		if err != nil {
			slog.ErrorContext(ctx, "scroll error", "error", err)
			return 1
		}
	}

	slog.InfoContext(ctx, "backfill_meeting_host_credentials complete",
		"published", published,
		"skipped", skipped,
		"failed", failed,
		"update", update,
	)
	if failed > 0 {
		return 1
	}
	return 0
}

func processPage(ctx context.Context, nc *nats.Conn, hits []osHit, update bool) (published, skipped, failed int) {
	for _, hit := range hits {
		meetingID := hit.Source.ObjectID
		hostKey := hit.Source.Data.HostKey

		if meetingID == "" {
			slog.WarnContext(ctx, "skipping: missing object_id")
			skipped++
			continue
		}
		if hostKey == "" {
			slog.WarnContext(ctx, "skipping: empty host_key", "meeting_id", meetingID)
			skipped++
			continue
		}

		tags := []string{meetingID, "meeting_id:" + meetingID}
		envelope := indexerEnvelope{
			Action:  natsPublishAction,
			Headers: map[string]string{"authorization": authHeaderValue},
			Data: credentialsData{
				MeetingID: meetingID,
				HostKey:   hostKey,
			},
			Tags: tags,
			IndexingConfig: indexingConfig{
				ObjectID:             meetingID,
				Public:               false,
				AccessCheckObject:    "v1_meeting:" + meetingID,
				AccessCheckRelation:  "host",
				HistoryCheckObject:   "v1_meeting:" + meetingID,
				HistoryCheckRelation: "auditor",
				ParentRefs:           []string{"v1_meeting:" + meetingID},
				Tags:                 tags,
			},
		}

		if !update {
			slog.InfoContext(ctx, "[dry-run] would publish credentials doc", "meeting_id", meetingID)
			published++
			continue
		}

		payload, err := json.Marshal(envelope)
		if err != nil {
			slog.ErrorContext(ctx, "failed to marshal envelope", "meeting_id", meetingID, "error", err)
			failed++
			continue
		}

		if err := nc.Publish(natsSubject, payload); err != nil {
			slog.ErrorContext(ctx, "failed to publish to NATS", "meeting_id", meetingID, "error", err)
			failed++
			continue
		}

		slog.InfoContext(ctx, "published credentials doc", "meeting_id", meetingID)
		published++
	}
	return published, skipped, failed
}

// openScroll opens a scroll for all v1_meeting documents that have a non-empty host_key.
func openScroll(ctx context.Context, client *http.Client, osURL string, pageSize int) (string, []osHit, int, error) {
	query := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"must": []any{
					map[string]any{"term": map[string]any{"object_type": "v1_meeting"}},
					map[string]any{"exists": map[string]any{"field": "data.host_key"}},
				},
				"must_not": []any{
					map[string]any{"term": map[string]any{"data.host_key": ""}},
				},
			},
		},
		"_source": []string{"object_id", "data.host_key"},
		"size":    pageSize,
	}
	body, _ := json.Marshal(query)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/%s/_search?scroll=2m", osURL, osIndex),
		bytes.NewReader(body),
	)
	if err != nil {
		return "", nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", nil, 0, err
	}
	defer resp.Body.Close() //nolint:errcheck

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, 0, fmt.Errorf("read scroll response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return "", nil, 0, fmt.Errorf("opensearch returned %d: %s", resp.StatusCode, string(raw))
	}

	var result osScrollResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", nil, 0, fmt.Errorf("unmarshal scroll response: %w", err)
	}
	return result.ScrollID, result.Hits.Hits, result.Hits.Total.Value, nil
}

// nextScrollPage fetches the next page and returns the updated scroll ID.
func nextScrollPage(ctx context.Context, client *http.Client, osURL, scrollID string) ([]osHit, string, error) {
	payload := map[string]string{"scroll": "2m", "scroll_id": scrollID}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, osURL+"/_search/scroll", bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close() //nolint:errcheck

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read scroll page: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("opensearch returned %d: %s", resp.StatusCode, string(raw))
	}

	var result osScrollResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, "", fmt.Errorf("unmarshal scroll page: %w", err)
	}
	return result.Hits.Hits, result.ScrollID, nil
}

// deleteScroll cleans up the OpenSearch scroll context.
func deleteScroll(ctx context.Context, client *http.Client, osURL, scrollID string) error {
	payload := map[string]string{"scroll_id": scrollID}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, osURL+"/_search/scroll", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close() //nolint:errcheck
	return nil
}
