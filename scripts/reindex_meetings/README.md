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

Each re-put is a compare-and-set on the revision the script just read (`kv.Update` with that revision). If the v1 sync or any other writer changes or deletes the key between the read and the write, the script skips the key and does not overwrite the newer value. That newer write already triggers the consumer. Skipped keys are logged and counted as `superseded`, and they do not cause a non-zero exit.

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
| `-missing-only` | `false` | Find and re-put only records that were never indexed (see [Finding records that were never indexed](#finding-records-that-were-never-indexed)); needs no OpenSearch |

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `NATS_URL` | `nats://127.0.0.1:4222` | NATS server URL |
| `OPENSEARCH_URL` | `http://localhost:9200` | OpenSearch base URL (not used with `-missing-only`) |

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

## Finding records that were never indexed

The default mode starts from OpenSearch, so it can only refresh records that are already indexed. It cannot find a record the event handler never indexed, for example one whose event was dropped after its last redelivery.

`-missing-only` starts from the source instead. It lists the `v1-objects` keys of each requested type and checks `v1-mappings` for the success marker the handler writes once it has published a record to the indexer. A key with no marker was never processed successfully. It needs only `NATS_URL`, not OpenSearch. Like `-reindex`, it is a dry-run unless you pass `-reindex`, and each re-put is the same compare-and-set.

```sh
# Dry-run: list the records with no success marker
NATS_URL=nats://localhost:4222 \
  go run ./scripts/reindex_meetings/ -missing-only -types v1_meeting,v1_past_meeting

# Re-put them so the consumer processes them again
NATS_URL=nats://localhost:4222 \
  go run ./scripts/reindex_meetings/ -missing-only -types v1_meeting,v1_past_meeting -reindex
```

Each type logs `scanned`, `with_marker`, `missing`, `processed`, `failed`, `not_found` and `superseded` counts. Every type listed under [Supported object types](#supported-object-types) works in this mode. Participants are scanned in two passes, one for invitee keys and one for attendee keys.

| Type | `v1-objects` key prefix | Marker in `v1-mappings` |
|------|-------------------------|-------------------------|
| `v1_meeting` | `itx-zoom-meetings-v2.` | `v1_meetings.<id>` |
| `v1_meeting_registrant` | `itx-zoom-meetings-registrants-v2.` | `v1_meeting_registrants.<id>` |
| `v1_meeting_rsvp` | `itx-zoom-meetings-invite-responses-v2.` | `v1_invite_responses.<id>` |
| `v1_meeting_attachment` | `itx-zoom-meetings-attachments-v2.` | `v1_meeting_attachments.<id>` |
| `v1_past_meeting` | `itx-zoom-past-meetings.` | `v1_past_meetings.<id>` |
| `v1_past_meeting_participant` | `itx-zoom-past-meetings-invitees.` | `v1_past_meeting_invitees.<id>` |
| | `itx-zoom-past-meetings-attendees.` | `v1_past_meeting_attendees.<id>` |
| `v1_past_meeting_recording` | `itx-zoom-past-meetings-recordings.` | `v1_past_meeting_recordings.<id>` |
| `v1_past_meeting_summary` | `itx-zoom-past-meetings-summaries.` | `v1_past_meeting_summaries.<id>` |
| `v1_past_meeting_attachment` | `itx-zoom-past-meetings-attachments.` | `v1_past_meeting_attachments.<id>` |

**Read the dry-run before re-putting.**

- **A record with no marker is not always a dropped event.** The handler also skips records on purpose, for example when the project is not in v2 yet, when the record was written by this service, or when a participant belongs to a past meeting that has been deleted. Those have no marker either. Re-putting them is harmless, since the handler skips them again, but they will show up in the list.
- **A record can be indexed and still be reported as missing.** The recording, summary and attachment handlers only log a failed marker write and carry on, so a record whose marker write failed is indexed but has no marker. Re-putting it is harmless.
- **Delete tombstones.** A `!del` marker means the record was deleted, and the script counts it as handled only if the `v1-objects` record is actually soft-deleted (`_sdc_deleted_at` set). A live record that still has a `!del` marker had its restore event dropped, so it is reported as missing and re-put. The reason is logged with each key.
- **Registrants can trigger invites.** Re-putting a registrant that has no marker takes the create path. When LFID invites are enabled and the registrant has no username, that path sends an invite email. The OpenSearch-driven mode takes the update path and never does. Run the dry-run on `v1_meeting_registrant` and review the list before using `-reindex` on it.
- **A suspicious type.** If a type reports `with_marker=0` for a non-empty bucket, the marker mapping for that type is probably wrong, and the script warns about it. Check a few keys by hand before trusting the result.
- **A failed lookup** (a marker or a record that cannot be read or decoded) is counted as `failed` and the record is not re-put.
