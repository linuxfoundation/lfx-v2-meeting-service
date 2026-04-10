# reindex_meetings

Operational script that re-triggers the full event processing pipeline for v1 meeting objects by re-putting their NATS KV entries. The existing consumer event handler picks them up and re-enriches and re-indexes them into OpenSearch.

## When to use

Run this when indexed meeting data is stale, missing, or incorrect and you need to force a full re-enrichment without waiting for new events. Typical scenarios:

- A bug fix was deployed to the event handler and affected records need to be reprocessed
- OpenSearch documents are missing fields that the handler now populates
- A bulk data issue needs to be corrected across many records

## Prerequisites

- Access to the NATS server for the target environment
- Access to the OpenSearch cluster for the target environment
- The meeting service's event processing consumer must be running to handle the re-put entries

## Usage

By default the script runs in dry-run mode — it logs what it would do without making any changes. Pass `-reindex` to actually re-put the KV entries.

```sh
# Dry-run (default): log what would be reindexed
NATS_URL=nats://localhost:4222 OPENSEARCH_URL=http://localhost:9200 \
  go run ./scripts/reindex_meetings/ -types v1_meeting,v1_past_meeting

# Actually reindex
NATS_URL=nats://localhost:4222 OPENSEARCH_URL=http://localhost:9200 \
  go run ./scripts/reindex_meetings/ -types v1_meeting,v1_past_meeting -reindex
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-types` | *(required)* | Comma-separated list of object types to reindex |
| `-reindex` | `false` | Actually re-put KV entries; omit to log only |
| `-batch` | `200` | OpenSearch scroll page size |

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `NATS_URL` | `nats://127.0.0.1:4222` | NATS server URL |
| `OPENSEARCH_URL` | `http://localhost:9200` | OpenSearch base URL |

## Supported object types

| Type | KV key prefix |
|------|---------------|
| `v1_meeting` | `itx-zoom-meetings-v2.` |
| `v1_meeting_registrant` | `itx-zoom-meetings-registrants-v2.` |
| `v1_meeting_rsvp` | `itx-zoom-meetings-invite-responses-v2.` |
| `v1_meeting_attachment` | `itx-zoom-meetings-attachments-v2.` |
| `v1_past_meeting` | `itx-zoom-past-meetings.` |
| `v1_past_meeting_participant` | resolved via `v1-mappings` KV bucket (covers both invitees and attendees) |
| `v1_past_meeting_recording` | `itx-zoom-past-meetings-recordings.` |
| `v1_past_meeting_summary` | `itx-zoom-past-meetings-summaries.` |
| `v1_past_meeting_attachment` | `itx-zoom-past-meetings-attachments.` |
