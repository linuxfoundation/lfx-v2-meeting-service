// reindex_meetings re-triggers the full event processing pipeline for any
// combination of v1 object types by re-putting their NATS KV entries so the
// existing consumer event handler picks them up and re-enriches + re-indexes them.
//
// Usage:
//
//	NATS_URL=nats://localhost:4222 OPENSEARCH_URL=http://localhost:9200 \
//	  go run ./scripts/reindex_meetings/ -types v1_meeting,v1_past_meeting
//
// Optional flags:
//
//	-types   Comma-separated list of object types to reindex (required).
//	         Supported values:
//	           v1_meeting
//	           v1_meeting_registrant
//	           v1_meeting_rsvp
//	           v1_meeting_attachment
//	           v1_past_meeting
//	           v1_past_meeting_participant  (covers both invitees and attendees)
//	           v1_past_meeting_recording
//	           v1_past_meeting_summary
//	           v1_past_meeting_attachment
//	-reindex Actually re-put KV entries and trigger reindexing (default: false,
//	         logs what would be re-put without making any changes)
//	-batch   OpenSearch scroll page size (default: 200)
//
// Environment variables:
//
//	NATS_URL       NATS server URL (default: nats://127.0.0.1:4222)
//	OPENSEARCH_URL OpenSearch base URL (default: http://localhost:9200)
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
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	kvBucketName         = "v1-objects"
	kvMappingsBucketName = "v1-mappings"
)

// objectTypeConfig maps an OpenSearch object_type to one or more KV key prefixes.
// v1_past_meeting_participant is handled specially — see resolveParticipantKeys.
var objectTypeConfig = map[string][]string{
	"v1_meeting":                  {"itx-zoom-meetings-v2."},
	"v1_meeting_registrant":       {"itx-zoom-meetings-registrants-v2."},
	"v1_meeting_rsvp":             {"itx-zoom-meetings-invite-responses-v2."},
	"v1_meeting_attachment":       {"itx-zoom-meetings-attachments-v2."},
	"v1_past_meeting":             {"itx-zoom-past-meetings."},
	"v1_past_meeting_participant": nil, // handled specially via resolveParticipantKeys
	"v1_past_meeting_recording":   {"itx-zoom-past-meetings-recordings."},
	"v1_past_meeting_summary":     {"itx-zoom-past-meetings-summaries."},
	"v1_past_meeting_attachment":  {"itx-zoom-past-meetings-attachments."},
}

// osHit holds just enough of each OpenSearch hit to get the object_id and,
// for participants, the flags and username needed to resolve invitee/attendee KV keys.
type osHit struct {
	Source struct {
		ObjectID string `json:"object_id"`
		Data     struct {
			IsInvited              bool   `json:"is_invited"`
			IsAttended             bool   `json:"is_attended"`
			Username               string `json:"username"`
			MeetingAndOccurrenceID string `json:"meeting_and_occurrence_id"`
		} `json:"data"`
	} `json:"_source"`
}

// osScrollResponse is the minimal shape of a scroll API response.
type osScrollResponse struct {
	ScrollID string `json:"_scroll_id"`
	Hits     struct {
		Hits []osHit `json:"hits"`
	} `json:"hits"`
}

func main() {
	typesFlag := flag.String("types", "", "comma-separated list of object types to reindex (required)")
	reindex := flag.Bool("reindex", false, "actually re-put KV entries and trigger reindexing (default: logs only)")
	batchSize := flag.Int("batch", 200, "OpenSearch scroll page size")
	flag.Parse()

	osURL := os.Getenv("OPENSEARCH_URL")
	if osURL == "" {
		osURL = "http://localhost:9200"
	}

	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}

	if *typesFlag == "" {
		fmt.Fprintf(os.Stderr, "error: -types is required\nsupported types: %s\n", strings.Join(supportedTypes(), ", "))
		os.Exit(1)
	}

	// Validate and collect requested types.
	requestedTypes := strings.Split(*typesFlag, ",")
	for _, t := range requestedTypes {
		t = strings.TrimSpace(t)
		if _, ok := objectTypeConfig[t]; !ok {
			slog.Error("unsupported object type", "type", t)
			fmt.Fprintf(os.Stderr, "supported types: %s\n", strings.Join(supportedTypes(), ", "))
			os.Exit(1)
		}
	}

	ctx := context.Background()

	slog.InfoContext(ctx, "reindex_meetings starting",
		"opensearch_url", osURL,
		"nats_url", natsURL,
		"reindex", *reindex,
		"types", *typesFlag,
	)

	// --- NATS + JetStream ---
	nc, err := nats.Connect(natsURL,
		nats.Timeout(10*time.Second),
		nats.MaxReconnects(5),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		slog.ErrorContext(ctx, "failed to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create JetStream context", "error", err)
		os.Exit(1)
	}

	kv, err := js.KeyValue(ctx, kvBucketName)
	if err != nil {
		slog.ErrorContext(ctx, "failed to bind to KV bucket", "bucket", kvBucketName, "error", err)
		os.Exit(1)
	}

	kvMappings, err := js.KeyValue(ctx, kvMappingsBucketName)
	if err != nil {
		slog.ErrorContext(ctx, "failed to bind to KV bucket", "bucket", kvMappingsBucketName, "error", err)
		os.Exit(1)
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}

	var totalProcessed, totalFailed, totalSkipped, totalNotFound int

	for _, objectType := range requestedTypes {
		objectType = strings.TrimSpace(objectType)
		prefixes := objectTypeConfig[objectType]

		slog.InfoContext(ctx, "processing object type", "object_type", objectType)

		processed, failed, skipped, notFound, err := reindexType(ctx, httpClient, kv, kvMappings, osURL, objectType, prefixes, *batchSize, *reindex)
		if err != nil {
			slog.ErrorContext(ctx, "fatal error processing type", "object_type", objectType, "error", err)
			os.Exit(1)
		}

		slog.InfoContext(ctx, "finished object type",
			"object_type", objectType,
			"processed", processed,
			"failed", failed,
			"skipped", skipped,
			"not_found", notFound,
		)

		totalProcessed += processed
		totalFailed += failed
		totalSkipped += skipped
		totalNotFound += notFound
	}

	slog.InfoContext(ctx, "reindex_meetings complete",
		"processed", totalProcessed,
		"failed", totalFailed,
		"skipped", totalSkipped,
		"not_found", totalNotFound,
	)

	if totalFailed > 0 {
		os.Exit(1)
	}
}

// reindexType scrolls OpenSearch for a given object_type, then re-puts each
// matching entry across all KV prefixes for that type.
// For v1_past_meeting_participant, kvMappings is used to resolve the attendee ID cross-reference.
func reindexType(
	ctx context.Context,
	httpClient *http.Client,
	kv jetstream.KeyValue,
	kvMappings jetstream.KeyValue,
	osURL, objectType string,
	kvPrefixes []string,
	batchSize int,
	reindex bool,
) (processed, failed, skipped, notFound int, err error) {
	scrollID, firstPage, err := openScroll(ctx, httpClient, osURL, objectType, batchSize)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("open scroll: %w", err)
	}
	defer deleteScroll(ctx, httpClient, osURL, scrollID) //nolint:errcheck

	isParticipant := objectType == "v1_past_meeting_participant"

	page := firstPage
	for {
		if len(page) == 0 {
			break
		}

		for _, hit := range page {
			id := hit.Source.ObjectID
			if id == "" {
				skipped++
				continue
			}

			var kvKeys []string
			if isParticipant {
				kvKeys = resolveParticipantKeys(ctx, kvMappings, hit)
				if len(kvKeys) == 0 {
					slog.WarnContext(ctx, "skipping participant: neither is_invited nor is_attended set", "object_id", id)
					skipped++
					continue
				}
			} else {
				for _, prefix := range kvPrefixes {
					kvKeys = append(kvKeys, prefix+id)
				}
			}

			for _, kvKey := range kvKeys {
				if !reindex {
					slog.InfoContext(ctx, "[dry-run] would re-put", "key", kvKey)
					processed++
					continue
				}

				entry, getErr := kv.Get(ctx, kvKey)
				if getErr != nil {
					if getErr == jetstream.ErrKeyNotFound {
						slog.WarnContext(ctx, "key not found in KV bucket", "key", kvKey)
						notFound++
						continue
					}
					slog.ErrorContext(ctx, "failed to get KV entry", "key", kvKey, "error", getErr)
					failed++
					continue
				}

				if _, putErr := kv.Put(ctx, kvKey, entry.Value()); putErr != nil {
					slog.ErrorContext(ctx, "failed to re-put KV entry", "key", kvKey, "error", putErr)
					failed++
					continue
				}

				processed++
			}
		}

		slog.InfoContext(ctx, "progress",
			"object_type", objectType,
			"processed", processed,
			"failed", failed,
			"skipped", skipped,
			"not_found", notFound,
		)

		page, err = nextScrollPage(ctx, httpClient, osURL, scrollID)
		if err != nil {
			return processed, failed, skipped, notFound, fmt.Errorf("scroll page: %w", err)
		}
	}

	return processed, failed, skipped, notFound, nil
}

// resolveParticipantKeys returns the KV key(s) to re-put for a v1_past_meeting_participant hit.
//
// In v1, invitees and attendees live in separate KV buckets with separate IDs:
//   - Invitee: itx-zoom-past-meetings-invitees.<invitee_id>
//   - Attendee: itx-zoom-past-meetings-attendees.<attendee_id>
//
// The OpenSearch object_id is the invitee_id when is_invited=true, or the attendee_id
// when is_attended=true only. When both flags are set, the merged document carries the
// invitee_id as object_id; the attendee_id is resolved via the
// v1_participant_by_meeting_user.attendee.<meeting_and_occurrence_id>.<username>
// cross-reference stored in the v1-mappings KV bucket.
func resolveParticipantKeys(ctx context.Context, kvMappings jetstream.KeyValue, hit osHit) []string {
	id := hit.Source.ObjectID
	isInvited := hit.Source.Data.IsInvited
	isAttended := hit.Source.Data.IsAttended
	username := hit.Source.Data.Username
	meetingAndOccurrenceID := hit.Source.Data.MeetingAndOccurrenceID

	switch {
	case isInvited && isAttended:
		// When both flags are set the object_id may be either the invitee or attendee ID,
		// so look up both via the v1_participant_by_meeting_user cross-references.
		var keys []string
		if username != "" && meetingAndOccurrenceID != "" {
			// The xref key uses the raw lf_sso value without the auth0| prefix that
			// some OpenSearch documents may carry in the username field.
			rawUsername := strings.TrimPrefix(username, "auth0|")

			inviteeXref := fmt.Sprintf("v1_participant_by_meeting_user.invitee.%s.%s", meetingAndOccurrenceID, rawUsername)
			if entry, xrefErr := kvMappings.Get(ctx, inviteeXref); xrefErr == nil {
				if inviteeID := string(entry.Value()); inviteeID != "" {
					keys = append(keys, "itx-zoom-past-meetings-invitees."+inviteeID)
				}
			} else if xrefErr != jetstream.ErrKeyNotFound {
				slog.WarnContext(ctx, "failed to look up invitee cross-reference", "xref_key", inviteeXref, "error", xrefErr)
			}

			attendeeXref := fmt.Sprintf("v1_participant_by_meeting_user.attendee.%s.%s", meetingAndOccurrenceID, rawUsername)
			if entry, xrefErr := kvMappings.Get(ctx, attendeeXref); xrefErr == nil {
				if attendeeID := string(entry.Value()); attendeeID != "" {
					keys = append(keys, "itx-zoom-past-meetings-attendees."+attendeeID)
				}
			} else if xrefErr != jetstream.ErrKeyNotFound {
				slog.WarnContext(ctx, "failed to look up attendee cross-reference", "xref_key", attendeeXref, "error", xrefErr)
			}
		}
		return keys

	case isInvited:
		return []string{"itx-zoom-past-meetings-invitees." + id}

	case isAttended:
		return []string{"itx-zoom-past-meetings-attendees." + id}

	default:
		return nil
	}
}

// openScroll opens an OpenSearch scroll for all documents of the given object_type.
func openScroll(ctx context.Context, client *http.Client, osURL, objectType string, pageSize int) (string, []osHit, error) {
	sourceFields := []string{"object_id"}
	if objectType == "v1_past_meeting_participant" {
		sourceFields = append(sourceFields, "data.is_invited", "data.is_attended", "data.username", "data.meeting_and_occurrence_id")
	}

	query := map[string]any{
		"query": map[string]any{
			"term": map[string]any{"object_type": objectType},
		},
		"_source": sourceFields,
		"size":    pageSize,
	}
	body, _ := json.Marshal(query)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, osURL+"/resources/_search?scroll=2m", bytes.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	raw, _ := io.ReadAll(resp.Body)
	var result osScrollResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", nil, fmt.Errorf("unmarshal scroll response: %w", err)
	}
	return result.ScrollID, result.Hits.Hits, nil
}

// nextScrollPage fetches the next page using the scroll ID.
func nextScrollPage(ctx context.Context, client *http.Client, osURL, scrollID string) ([]osHit, error) {
	payload := map[string]string{"scroll": "2m", "scroll_id": scrollID}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, osURL+"/_search/scroll", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	raw, _ := io.ReadAll(resp.Body)
	var result osScrollResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshal scroll page: %w", err)
	}
	return result.Hits.Hits, nil
}

// deleteScroll cleans up the scroll context in OpenSearch.
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

// supportedTypes returns the sorted list of valid object type names.
func supportedTypes() []string {
	types := make([]string, 0, len(objectTypeConfig))
	for t := range objectTypeConfig {
		types = append(types, t)
	}
	return types
}
