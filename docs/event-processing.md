# Event Processing System

## Overview

The LFX Meeting Service implements a comprehensive event processing system that watches NATS JetStream KV buckets for meeting-related data changes from the v1 system. When changes are detected, the service transforms the data from v1 to v2 format and publishes events to both the indexer service (for search) and FGA-sync service (for access control).

This document describes the architecture, configuration, data transformation patterns, and operational aspects of the event processing system.

## Architecture

### System Components

```text
┌─────────────────┐
│  v1 DynamoDB    │
│  (Source)       │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  NATS KV        │
│  v1-objects     │  (Written by v1-sync-helper)
└────────┬────────┘
         │
         │ Watch for changes
         ▼
┌─────────────────────────────────────────┐
│  Meeting Service Event Processor        │
│                                         │
│  ┌───────────────┐                     │
│  │ KV Handler    │ (Routes by key)     │
│  └───────┬───────┘                     │
│          │                             │
│  ┌───────▼────────────────────┐       │
│  │ Event Handlers             │       │
│  │ - Meetings                 │       │
│  │ - Registrants              │       │
│  │ - Invite Responses         │       │
│  │ - Meeting Attachments      │       │
│  │ - Past Meetings            │       │
│  │ - Participants             │       │
│  │ - Recordings/Transcripts   │       │
│  │ - Summaries                │       │
│  │ - Past Meeting Attachments │       │
│  └───────┬────────────────────┘       │
│          │                             │
│  ┌───────▼──────────┐                 │
│  │ Data Transform   │                 │
│  │ - v1 → v2        │                 │
│  │ - RRULE calc     │                 │
│  │ - ID mapping     │                 │
│  │ - User enrich    │                 │
│  └───────┬──────────┘                 │
│          │                             │
│  ┌───────▼──────────┐                 │
│  │ Event Publisher  │                 │
│  └───────┬──────────┘                 │
└──────────┼──────────────────────────────┘
           │
           ├──────────────┬────────────────┐
           ▼              ▼                ▼
    ┌─────────────┐  ┌──────────┐  ┌──────────────┐
    │  Indexer    │  │ FGA-Sync │  │ v1-mappings  │
    │  Service    │  │ Service  │  │ KV Bucket    │
    └─────────────┘  └──────────┘  └──────────────┘
```

### Event Flow

1. **Source Event**: v1 system writes/updates data in DynamoDB
2. **KV Sync**: v1-sync-helper service syncs DynamoDB to NATS KV bucket (`v1-objects`)
3. **Event Detection**: Meeting service consumer watches KV bucket for changes
4. **Routing**: KV handler routes events by key prefix to appropriate handler
5. **Transformation**: Handler converts v1 data to v2 format, enriches with user data, calculates occurrences
6. **ID Mapping**: SFIDs mapped to UUIDs via ID mapper service
7. **Publishing**: Events published to indexer (search) and FGA-sync (access control)
8. **Mapping Storage**: v1→v2 ID mappings stored in `v1-mappings` KV bucket

## Event Types

The system processes 12 different event types:

### Active Meeting Events

| Event Type | Key Prefix | Description |
| ------------ | --------- | ----------- |
| Meeting | `itx-zoom-meetings-v2.` | Meeting creation, updates, and RRULE occurrence calculation |
| Meeting Mapping | `itx-zoom-meetings-mappings-v2.` | Committee-to-meeting associations |
| Registrant | `itx-zoom-meetings-registrants-v2.` | Meeting registrants with user enrichment |
| Invite Response | `itx-zoom-meetings-invite-responses-v2.` | RSVP responses (accepted, declined, maybe) |
| Meeting Attachment | `itx-zoom-meetings-attachments-v2.` | Files and links attached to active meetings |

### Past Meeting Events

| Event Type | Key Prefix | Description |
| ------------ | --------- | ----------- |
| Past Meeting | `itx-zoom-past-meetings.` | Completed meeting records |
| Past Meeting Mapping | `itx-zoom-past-meetings-mappings.` | Past meeting committee associations |
| Invitee | `itx-zoom-past-meetings-invitees.` | Users invited to past meetings |
| Attendee | `itx-zoom-past-meetings-attendees.` | Users who attended with session tracking |
| Recording | `itx-zoom-past-meetings-recordings.` | One record per past meeting with all its Zoom files; indexed as a recording document and, when it has transcript files, a transcript document (see [Recording and Transcript Split](#recording-and-transcript-split)) |
| Summary | `itx-zoom-past-meetings-summaries.` | AI-generated meeting summaries |
| Past Meeting Attachment | `itx-zoom-past-meetings-attachments.` | Files and links attached to past meetings |

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
| -------- | -------- | ------- | ----------- |
| `EVENT_PROCESSING_ENABLED` | No | `true` | Enable/disable event processing |
| `EVENT_CONSUMER_NAME` | No | `meeting-service-kv-consumer` | JetStream consumer name |
| `EVENT_STREAM_NAME` | No | `KV_v1-objects` | KV bucket stream name |
| `EVENT_FILTER_SUBJECT` | No | `$KV.v1-objects.>` | Subject filter pattern |
| `EVENT_MAX_DELIVER` | No | `3` | Maximum delivery attempts |
| `EVENT_ACK_WAIT` | No | `30s` | Acknowledgment wait timeout |
| `EVENT_MAX_ACK_PENDING` | No | `1000` | Maximum pending acks |
| `NATS_URL` | Yes | - | NATS server connection URL |
| `INVITES_ENABLED` | No | `false` | Enable LFID invite sending (registrant handler) and `invite_accepted` enrichment |
| `LFX_SELF_SERVE_BASE_URL` | No | derived from `LFX_ENVIRONMENT` | Base URL embedded in invite emails as `return_url` |

### LFID Invite Flow

When `INVITES_ENABLED=true`, the meeting service participates in the platform LFID invite flow in two independent paths:

#### Outbound invites (requires event processing)

During registrant create processing (`itx-zoom-meetings-registrants-v2.*`), after the registrant is indexed:

1. Skip if the registrant already has a username (LFID) or if an invite was already sent for this registrant UID (tracked in `v1-mappings` as `v1_meeting_registrant_lfid_invite_sent.{registrant_uid}`).
2. Look up the email via `lfx.auth-service.email_to_username` — skip if an LFID already exists.
3. Resolve the meeting title from the `v1-objects` KV bucket; skip if unavailable.
4. Send `lfx.invite-service.send_invite` with `resource.type=meeting` and role `Registrant`.
5. Store the invite UID in the sent-marker mapping key (best-effort).

All invite operations are best-effort: errors are logged and never cause KV message retries.

#### Invite acceptance enrichment (independent of event processing)

When `INVITES_ENABLED=true` and `NATS_URL` is set, `main.go` starts a NATS queue subscriber on
`lfx.invite-service.invite_accepted` (queue group: `meeting-service-invite-accepted`). On acceptance it
calls the ITX endpoint `POST /v2/zoom/meetings/invite_accepted` to enrich all Zoom DynamoDB records for
the acceptor's email, regardless of the invite's `resource_type` (mirroring project/committee reconciliation
behavior).

The subscriber uses the process shutdown context, drains on stop, and waits for in-flight handlers to finish.

### Consumer Configuration

The event processor creates a durable JetStream consumer with these settings:

```go
consumer, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
    Name:              cfg.ConsumerName,
    Durable:           cfg.ConsumerName,
    DeliverPolicy:     jetstream.DeliverLastPerSubjectPolicy,
    FilterSubject:     cfg.FilterSubject,
    MaxDeliver:        cfg.MaxDeliver,
    AckWait:           cfg.AckWait,
    AckPolicy:         jetstream.AckExplicitPolicy,
    MaxAckPending:     cfg.MaxAckPending,
})
```

**Key behaviors:**

- **DeliverLastPerSubjectPolicy**: Only processes the latest update for each key (skips intermediate states)
- **AckExplicitPolicy**: Requires explicit ACK/NAK for each message
- **MaxDeliver: 3**: Retries failed messages up to 3 times with exponential backoff
- **AckWait: 30s**: Handler has 30 seconds to process before automatic redelivery
- **Auto-recreate on deletion**: If the consumer is deleted on the NATS server at runtime (e.g. via CLI or server-side cleanup), the service detects the deletion and automatically recreates it with exponential backoff (1s → 5s → 15s → 30s). The service never requires a restart for this condition.

## Data Transformation

### v1 → v2 Field Mappings

#### Meeting Fields

| v1 Field | v2 Field | Transformation |
| -------- | -------- | -------------- |
| `meeting_id` | `id` | Direct copy |
| `topic` | `title` | Direct copy |
| `agenda` | `description` | Direct copy |
| `start_time` | `start_time` | Parse RFC3339 string to time.Time |
| `duration` | `duration` | Parse string to int (minutes) |
| `proj_id` | `project_uid` | Map SFID → UUID via IDMapper |
| `timezone` | `timezone` | Direct copy |
| `type` | `meeting_type` | Direct copy |
| `early_join_time_minutes` | `early_join_time_minutes` | Parse string to int |
| `recording_enabled` | `recording_enabled` | Parse string to bool |
| `transcript_enabled` | `transcript_enabled` | Parse string to bool |
| `youtube_upload_enabled` | `youtube_upload_enabled` | Parse string to bool |
| `recording_access` | `artifact_visibility` | Direct copy |
| `recurrence` | `occurrences` | Calculate via RRULE library |

#### Registrant Fields

| v1 Field | v2 Field | Transformation |
| -------- | -------- | -------------- |
| `id` | `uid` | Direct copy |
| `first_name` | `first_name` | Direct copy |
| `last_name` | `last_name` | Direct copy |
| `email` | `email` | Direct copy |
| `username` | `username` | Direct copy; fallback to v1 user lookup (`lf_sso`) if empty |
| `user_id` | `user_id` | Platform user ID (passed through); used for v1 user lookup when username is empty |
| `host` | `host` | Parse string to bool |
| `org` | `org_name` | Direct copy |
| `profile_picture` | `avatar_url` | Direct copy |
| `proj_id` | `project_uid` | Map SFID → UUID via IDMapper |

#### Participant Fields

| v1 Field | v2 Field | Transformation |
| -------- | -------- | -------------- |
| `id` | `uid` | Direct copy |
| `meeting_and_occurrence_id` | `meeting_and_occurrence_id` | Direct copy |
| `meeting_id` | `meeting_id` | Direct copy |
| `first_name` | `first_name` | Direct copy or parse from name |
| `last_name` | `last_name` | Direct copy or parse from name |
| `email` | `email` | Direct copy |
| `lf_sso` | `username` | Direct copy |
| `org` | `org_name` | Direct copy |
| `org_is_member` | `org_is_member` | Parse bool pointer |
| `org_is_project_member` | `org_is_project_member` | Parse bool pointer |
| `job_title` | `job_title` | Direct copy |
| `profile_picture` | `avatar_url` | Direct copy |
| `registrant_id` | - | Used for host lookup |
| `sessions` | `sessions` | Transform session array |

#### Recording Fields

One v1 recording record produces the recording document and, when it has transcript files, the transcript document. Unless noted, a field is copied to both.

| v1 Field | v2 Field | Transformation |
| -------- | -------- | -------------- |
| `meeting_and_occurrence_id` | `id`, `meeting_and_occurrence_id` | Direct copy; the record has no separate recording ID |
| `proj_id` | `project_uid` | Map SFID → UUID via IDMapper; the record is skipped when `proj_id` is empty or has no mapping |
| `project_slug` | `project_slug` | Direct copy |
| `topic` | `title` | Direct copy |
| `host_email`, `host_id`, `meeting_id`, `occurrence_id`, `visibility` | same names | Direct copy |
| `recording_access` | `recording_access` | Recording document only; defaults to `"meeting_hosts"` when empty |
| `transcript_access` | `transcript_access` | Defaults to `"meeting_hosts"` when empty and the record has transcript files |
| `transcript_enabled` | - | Ignored; `transcript_enabled` on the recording document is `true` when the record has a `TRANSCRIPT` or `TIMELINE` file |
| `recording_files` | `recording_files` | Split by `file_type`: `TRANSCRIPT` and `TIMELINE` files go to the transcript document, all others to the recording document |
| `recording_count` | `recording_count` | Recording document only; direct copy (counts the files before the split) |
| `sessions` | `sessions` | Copied without the session `password` |
| `start_time`, `created_at` | same names | Parse to time |
| `modified_at` | `updated_at` | Parse to time |
| `total_size` | `total_size` | Direct copy (string or number accepted) |
| `created_by`, `updated_by` | same names | Direct copy |

#### Attachment Fields

Meeting attachment and past meeting attachment records use the same field names; past meeting attachments also carry `meeting_and_occurrence_id`.

| v1 Field | v2 Field | Transformation |
| -------- | -------- | -------------- |
| `id` | `uid` | Direct copy |
| `meeting_id`, `meeting_and_occurrence_id` | same names | Direct copy |
| `type`, `category`, `link`, `name`, `description`, `source` | same names | Direct copy |
| `file_name`, `file_url`, `file_uploaded`, `file_upload_status`, `file_content_type` | same names | Direct copy |
| `file_size` | `file_size` | String or number accepted |
| `file_uploaded_at` | `file_uploaded_at` | Parse to time; omitted when empty or not parseable |
| `created_at` | `created_at` | Parse to time |
| `updated_at` | `modified_at` | Parse to time |
| `created_by`, `updated_by`, `file_uploaded_by` | same names | `id` → `user_id`; `username`, `email`, `name`, `profile_picture` copied |
| - | `project_uid` | Mapped from the parent meeting's (or past meeting's) `proj_id`; the attachment is skipped when no mapping exists |
| - | `project_slug` | Meeting attachments: looked up by `project_uid` on `lfx.projects-api.get_slug`. Past meeting attachments: the parent past meeting's `project_slug` |

### RRULE Occurrence Calculation

Meeting occurrences are calculated from v1 recurrence rules using the `github.com/teambition/rrule-go` library:

```go
// Example v1 recurrence data
{
    "type": 2,              // 1=Daily, 2=Weekly, 3=Monthly
    "repeat_interval": 1,
    "weekly_days": "1,3,5", // Monday, Wednesday, Friday
    "end_times": 10,        // 10 occurrences
    "cancelled_occurrences": ["2024-01-15T10:00:00Z"],
    "updated_occurrences": [
        {
            "occurrence_id": "2024-01-17T10:00:00Z",
            "start_time": "2024-01-17T14:00:00Z",
            "duration": 60
        }
    ]
}

// Transformed to v2 occurrences
[
    {
        "occurrence_id": "2024-01-08T10:00:00Z",
        "start_time": "2024-01-08T10:00:00Z",
        "duration": 30,
        "status": "available"
    },
    // 2024-01-15 cancelled - not included
    {
        "occurrence_id": "2024-01-17T10:00:00Z",
        "start_time": "2024-01-17T14:00:00Z",  // Updated time
        "duration": 60,                          // Updated duration
        "status": "available"
    },
    // ... more occurrences
]
```

**Calculation steps:**

1. Parse recurrence type (daily/weekly/monthly)
2. Generate RRULE string with interval and frequency
3. Calculate up to 100 future occurrences
4. Filter cancelled occurrences
5. Apply updated occurrence overrides
6. Handle "all following" updates

### User Enrichment

When registrant/participant data lacks a username but has a user_id, the system performs v1 user lookup:

```go
// Lookup from v1-objects KV bucket
key := "user.{user_id}"
userData := v1ObjectsKV.Get(key)

// Extract enrichment data
{
    "lf_sso": "jdoe",           // → username
    "lf_email": "john@example.com",
    "first_name": "John",
    "last_name": "Doe",
    "profile_picture": "https://...",
    "org": "ACME Corp"
}
```

### FGA Member Usernames

Registrant and past meeting participant FGA `member_put` / `member_remove` messages use the v1 LFX username directly (`username` for registrants, `lf_sso` for past meeting participants). No Auth0 sub conversion is performed before publishing to fga-sync.

### ID Mapping

Project and committee IDs are mapped from v1 SFIDs to v2 UUIDs:

```go
// Map project SFID to UUID
projectUID, err := idMapper.MapProjectV1ToV2(ctx, v1ProjectSFID)
if err != nil {
    // NAK for retry if mapper unavailable
    return true
}

// Map committee SFIDs
for _, committeeSFID := range v1CommitteeSFIDs {
    committeeUID, err := idMapper.MapCommitteeV1ToV2(ctx, committeeSFID)
    if err != nil {
        logger.Warn("failed to map committee SFID", "sfid", committeeSFID)
        continue // Skip unmappable committees
    }
    committees = append(committees, Committee{UID: committeeUID})
}
```

Mappings are stored in the `v1-mappings` KV bucket for future reference:

```text
Key: itx-zoom-meetings-v2.{v1_meeting_id}
Value: {
    "v1_id": "abc123",
    "v2_id": "550e8400-e29b-41d4-a716-446655440000",
    "entity_type": "meeting"
}
```

## Error Handling

### Error Classification

The system distinguishes between transient and permanent errors:

#### Transient Errors (NAK for retry)

- NATS connection timeouts
- ID mapper service unavailable
- Network failures
- Parent resource lookup returns a non-`ErrKeyNotFound` error (e.g., NATS transient failure)
- Temporary v1 user lookup failures

**Retry behavior:**

- Attempt 1: Immediate redelivery
- Attempt 2: ~2 second delay
- Attempt 3: ~10 second delay
- After 3 attempts: Message moved to dead letter queue

#### Permanent Errors (ACK to skip)

- Invalid JSON format
- Missing required fields
- Malformed data (invalid timestamps, negative numbers)
- Parent meeting not found in v1-mappings KV bucket (`jetstream.ErrKeyNotFound`) — meeting was filtered out or not indexed
- Parent missing after max retries
- Filtered emails (MAILER-DAEMON)
- `MeetingAndOccurrenceID` empty on participant publish (returns `domain.ValidationError` immediately)

### Parent-Child Ordering

The system handles parent-child dependencies through retry logic:

```go
// Example: Registrant handler checks for parent meeting in v1-mappings KV bucket
meetingMappingKey := fmt.Sprintf("v1_meetings.%s", registrantData.MeetingID)
_, err = h.v1MappingsKV.Get(ctx, meetingMappingKey)
if err != nil {
    if errors.Is(err, jetstream.ErrKeyNotFound) {
        // Meeting was filtered out or is not being indexed — permanent skip
        logger.Info("parent meeting not in mappings (filtered/not indexed), skipping registrant")
        return false // ACK - permanent skip, will not retry
    }
    // Transient error (NATS unavailable, etc.) — retry
    logger.Warn("transient error looking up parent meeting mapping, will retry")
    return true // NAK - retry later
}
```

**Error distinction for parent-meeting check:**

- `jetstream.ErrKeyNotFound`: The parent meeting was deliberately not indexed (filtered out or not yet written). This is a **permanent skip** — ACK the message without retrying, because retrying will never succeed if the meeting is excluded.
- Any other error: Transient infrastructure failure (NATS connectivity, timeout). NAK the message for retry.

**Parent-child relationships:**

- Registrant → Meeting (validate meeting exists)
- Invite Response → Meeting (validate meeting exists)
- Meeting Attachment → Meeting (retries while the meeting record is missing from `v1-objects`; skips when the meeting has no project or the project has no v2 mapping)
- Past Meeting Invitee → Past Meeting (validate past meeting exists)
- Past Meeting Attendee → Past Meeting (validate past meeting exists)
- Recording → Past Meeting (committees only: the recording is published whether or not the past meeting record exists; it is skipped when its own `proj_id` is empty or has no v2 mapping)
- Summary → Past Meeting (validate past meeting exists)
- Past Meeting Attachment → Past Meeting (skips without retry when the past meeting record is missing, has no project, or the project has no v2 mapping)

## Publishing

### Dual Publishing Architecture

Most events are published to **both** indexer and FGA-sync services:

#### Indexer Service

**Purpose**: Enable full-text search and filtering

**Subject pattern**: `lfx.index.{object_type}`

**Message format**: an `IndexerMessageEnvelope` from `lfx-v2-indexer-service/pkg/types`. The object type is taken from the subject, not from a field. This example is a past meeting recording on `lfx.index.v1_past_meeting_recording`, with `data` abbreviated (see [Recording Event](#recording-event) for the full object):

```json
{
    "action": "created",
    "headers": {
        "authorization": "Bearer lfx-v2-meeting-service"
    },
    "data": {
        "id": "meeting-uuid_2024-01-15T10:00:00Z",
        "meeting_and_occurrence_id": "meeting-uuid_2024-01-15T10:00:00Z",
        "project_uid": "proj-uuid",
        "title": "Weekly Team Sync"
    },
    "tags": ["meeting-uuid_2024-01-15T10:00:00Z", "past_meeting_recording_id:meeting-uuid_2024-01-15T10:00:00Z", "meeting_and_occurrence_id:meeting-uuid_2024-01-15T10:00:00Z", "platform:Zoom", "platform_meeting_id:", "project_uid:proj-uuid"],
    "indexing_config": {
        "object_id": "meeting-uuid_2024-01-15T10:00:00Z",
        "public": false,
        "access_check_object": "v1_past_meeting:meeting-uuid_2024-01-15T10:00:00Z",
        "access_check_relation": "recording_viewer",
        "history_check_object": "v1_past_meeting:meeting-uuid_2024-01-15T10:00:00Z",
        "history_check_relation": "auditor",
        "sort_name": "Weekly Team Sync",
        "name_and_aliases": ["Weekly Team Sync"],
        "parent_refs": ["past_meeting:meeting-uuid_2024-01-15T10:00:00Z", "project:proj-uuid"],
        "tags": ["meeting-uuid_2024-01-15T10:00:00Z", "past_meeting_recording_id:meeting-uuid_2024-01-15T10:00:00Z", "meeting_and_occurrence_id:meeting-uuid_2024-01-15T10:00:00Z", "platform:Zoom", "platform_meeting_id:", "project_uid:proj-uuid"],
        "fulltext": "Weekly Team Sync"
    }
}
```

A delete is sent on the same subject as `{"action": "deleted", "headers": {...}, "data": "<object id>", "tags": []}`, with no `indexing_config`.

#### FGA-Sync Service

**Purpose**: Maintain access control tuples in OpenFGA

**Subject pattern**: `lfx.fga-sync.{operation}`

All FGA messages use the `GenericFGAMessage` format. There are two operation types:

**Access control** (meetings, past meetings):

Recordings, transcripts and summaries send no FGA messages; they are indexed only and access-checked against their parent `v1_past_meeting` (see the [FGA contract](fga-contract.md)). Meeting and past meeting attachments send none either; they are access-checked with `viewer` on their parent `v1_meeting` or `v1_past_meeting`.

Subject: `lfx.fga-sync.update_access`

```json
{
    "object_type": "v1_meeting",
    "operation": "update_access",
    "data": {
        "uid": "550e8400-e29b-41d4-a716-446655440000",
        "public": false,
        "relations": {
            "organizer": ["jdoe"]
        },
        "references": {
            "project": ["project-uuid"],
            "committee": ["committee-uuid-1", "committee-uuid-2"]
        }
    }
}
```

**Member access** (registrants, participants):

Subject: `lfx.fga-sync.member_put`

For registrants, exactly one relation is sent — `"host"` or `"participant"` — with the other listed as `mutually_exclusive_with` so fga-sync removes any prior conflicting tuple:

```json
{
    "object_type": "v1_meeting",
    "operation": "member_put",
    "data": {
        "uid": "meeting-uuid",
        "username": "jdoe",
        "relations": ["host"],
        "mutually_exclusive_with": ["participant"]
    }
}
```

For past meeting participants, all applicable relations (`"host"`, `"invitee"`, `"attendee"`) are included in `relations` based on the participant's flags, and all three are listed in `mutually_exclusive_with` so any stale tuples are replaced atomically:

```json
{
    "object_type": "v1_past_meeting",
    "operation": "member_put",
    "data": {
        "uid": "meeting-and-occurrence-uuid",
        "username": "jdoe",
        "relations": ["invitee", "attendee"],
        "mutually_exclusive_with": ["host", "invitee", "attendee"]
    }
}
```

**Artifact visibility on the past meeting** (recordings, transcripts, summaries):

The `v1_past_meeting` `update_access` message, sent from the past meeting record, carries the per-artifact viewer relations. For each artifact kind (`{kind}` is `recording`, `transcript` or `summary`), the matching access field on the past meeting selects what is written; each reference points at the same `v1_past_meeting` object:

| `recording_access` / `transcript_access` / `ai_summary_access` | What is written |
| --------------------------------------------------------------- | --------------- |
| `"public"` | `recording_viewer` / `transcript_viewer` / `ai_summary_viewer` relation to `*` |
| `"meeting_participants"` | `references` keys `past_meeting_for_host_{kind}_view`, `past_meeting_for_attendee_{kind}_view`, `past_meeting_for_participant_{kind}_view` |
| `"meeting_hosts"`, unset, or any other value | `references` key `past_meeting_for_host_{kind}_view` |

Summary index documents check `v1_past_meeting#organizer` instead of `ai_summary_viewer` while the summary awaits approval; see [Summary `ai_summary_access` Lookup](#summary-ai_summary_access-lookup) below and the [indexer contract](indexer-contract.md#v1-past-meeting-summary).

### Actions

Events use these action types:

| Action | Indexer Subject | FGA-Sync Subject | Use Case |
| ------ | --------------- | ---------------- | -------- |
| `created` | `lfx.index.{type}` | `lfx.fga-sync.update_access` | New resource created |
| `updated` | `lfx.index.{type}` | `lfx.fga-sync.update_access` | Resource modified |
| `deleted` | `lfx.index.{type}` | `lfx.fga-sync.delete_access` | Resource removed |

### Special Cases

#### Recording and Transcript Split

A v1 recording record (`itx-zoom-past-meetings-recordings.{meeting_and_occurrence_id}`) is the only source of both the recording and the transcript documents. `convertMapToRecordingData` sorts the record's `recording_files` by `file_type`:

- `TRANSCRIPT` (VTT transcript) and `TIMELINE` (JSON timeline) files go to the transcript document.
- Every other file type (for example `MP4` video, `M4A` audio, `CHAT` chat log, `CC` closed captions) goes to the recording document.

`handlePastMeetingRecordingUpdate` then publishes:

1. **Recording document** (`lfx.index.v1_past_meeting_recording`): always, with the non-transcript files.
2. **Transcript document** (`lfx.index.v1_past_meeting_transcript`): only when the record has at least one `TRANSCRIPT` or `TIMELINE` file, with those files only.

Both documents use the record's `meeting_and_occurrence_id` as their ID and share one `created`/`updated` action, taken from the `v1_past_meeting_recordings.{id}` key in `v1-mappings`. Neither sends an FGA message. A recording delete sends a `deleted` indexer message on both subjects. An update whose record no longer has transcript files publishes only the recording document; it does not delete an already indexed transcript document.

#### Past Meeting FGA References

Past meeting FGA `update_access` messages carry the `meeting`, `project` and `committee` references plus the per-artifact viewer relations and `past_meeting_for_*_{kind}_view` self-references described in [Artifact visibility on the past meeting](#fga-sync-service) above; see also the [FGA contract](fga-contract.md#v1-past-meeting). The `meeting` reference links the past meeting record back to its originating active meeting and carries the `v1_meeting:` type prefix. `exclude_relations` is always `["host", "invitee", "attendee"]`, because those tuples are managed by the participant `member_put` messages.

This sample assumes `recording_access` is `"public"`, `transcript_access` is `"meeting_participants"` and `ai_summary_access` is `"meeting_hosts"`:

```json
{
    "object_type": "v1_past_meeting",
    "operation": "update_access",
    "data": {
        "uid": "past-meeting-uuid",
        "public": false,
        "relations": {
            "recording_viewer": ["*"]
        },
        "references": {
            "meeting": ["v1_meeting:meeting-uuid"],
            "project": ["project-uuid"],
            "committee": ["committee-uuid-1", "committee-uuid-2"],
            "past_meeting_for_host_transcript_view": ["v1_past_meeting:past-meeting-uuid"],
            "past_meeting_for_attendee_transcript_view": ["v1_past_meeting:past-meeting-uuid"],
            "past_meeting_for_participant_transcript_view": ["v1_past_meeting:past-meeting-uuid"],
            "past_meeting_for_host_summary_view": ["v1_past_meeting:past-meeting-uuid"]
        },
        "exclude_relations": ["host", "invitee", "attendee"]
    }
}
```

#### Summary `ai_summary_access` Lookup

The `ai_summary_access` value used for summary indexing is **not stored on the summary record itself**. It is looked up at publish time from the parent past meeting record in the `v1-objects` KV bucket:

```go
// Key: itx-zoom-past-meetings.{meeting_and_occurrence_id}
pastMeetingKey := fmt.Sprintf("itx-zoom-past-meetings.%s", summaryData.MeetingAndOccurrenceID)
entry, _ := h.v1ObjectsKV.Get(ctx, pastMeetingKey)
// Extract ai_summary_access; missing or non-string defaults to ""
aiSummaryAccess := ""
if access, ok := pastMeetingData["ai_summary_access"].(string); ok {
    aiSummaryAccess = access
}

// Pass to publisher
publisher.PublishPastMeetingSummaryEvent(ctx, action, summaryData, aiSummaryAccess)
```

If the parent past meeting record does not exist, the summary is skipped without retry; any other KV or decode error is retried. A missing `ai_summary_access` field defaults to `""` (which maps to the `"meeting_hosts"` visibility case).

`ai_summary_access` only applies once the summary is approved or does not require approval. While a summary awaits approval (`requires_approval && !approved`), the publisher indexes it as non-public with `v1_past_meeting#organizer` for both the access and the history check, matching LFX Self Serve's organizer-only rule. Approving the summary changes the v1 summary record, the KV watcher republishes it as `updated`, and the document is re-indexed with `ai_summary_viewer`. See the [indexer contract](indexer-contract.md#v1-past-meeting-summary).

#### `UpdatedOccurrences` Duration Coercion

The `Duration` field in each `updated_occurrences` entry is safely coerced from either a JSON string or a number during unmarshaling. This handles v1 data where numeric fields may be stored as strings:

```json
// Both of these are handled correctly:
{"duration": 60}
{"duration": "60"}
```

Additionally, the `occurrences` array from the raw KV data is **always ignored** and discarded during deserialization. Occurrences are always recomputed from the RRULE calculation — the stored `occurrences` field is never used.

#### `FileUploadedAt` as Optional Pointer

The `FileUploadedAt` field on `MeetingAttachmentEventData` and `PastMeetingAttachmentEventData` is typed as `*time.Time`. When the field is absent, empty or not a parseable time in the source data, it is `nil` and is omitted from the serialized JSON output. This prevents zero-value timestamps (`0001-01-01T00:00:00Z`) from being published for attachments that have not been uploaded yet.

#### `MeetingAndOccurrenceID` Validation for Participants

In `PublishPastMeetingParticipantEvent`, if `MeetingAndOccurrenceID` is empty, a `domain.ValidationError` is returned immediately before any publishing occurs. This is treated as a permanent error (ACK) by the handler since the data is structurally invalid and retrying will not resolve it.

#### Summary Content Assembly

Summaries consolidate sparse Zoom summary fields into structured markdown:

```go
// Input v1 data
{
    "summary_overview": "Team discussed Q1 roadmap",
    "summary_details": [
        {
            "label": "Feature Planning",
            "summary": "Prioritized auth improvements"
        },
        {
            "label": "Bug Review",
            "summary": "Identified 3 critical issues"
        }
    ],
    "next_steps": [
        "Schedule design review",
        "Update roadmap doc"
    ]
}

// Output markdown
## Overview
Team discussed Q1 roadmap

## Key Topics
### Feature Planning
Prioritized auth improvements

### Bug Review
Identified 3 critical issues

## Next Steps
- Schedule design review
- Update roadmap doc
```

## Operations

### Starting the Event Processor

The event processor starts automatically when the service boots:

```bash
# Enable event processing (default: true)
export EVENT_PROCESSING_ENABLED=true

# Start service
./bin/meeting-api
```

**Startup logs:**

```text
INFO initializing event processor
INFO event processor started consumer=meeting-service-kv-consumer
```

**Consumer deletion and auto-recreation logs:**

If the consumer is deleted on the server while the service is running, it is automatically recreated without a service restart:

```text
WARN consumer was deleted on the server, will recreate
INFO recreating consumer after deletion
INFO consumer configured name=meeting-service-kv-consumer stream=KV_v1-objects
```

If recreation fails transiently (e.g. context deadline exceeded), the service retries with backoff:

```text
WARN failed to setup consumer, retrying attempt=1 retry_in=1s error=...
WARN failed to setup consumer, retrying attempt=2 retry_in=5s error=...
INFO consumer configured name=meeting-service-kv-consumer stream=KV_v1-objects
```

### Stopping the Event Processor

The event processor gracefully drains during shutdown:

```bash
# Send SIGTERM (Kubernetes does this automatically)
kill -TERM <pid>
```

**Shutdown logs:**

```text
INFO shutting down event processor
INFO event processor stopped successfully timeout=30s pending_messages=0
```

**Graceful shutdown behavior:**

- Stop accepting new messages
- Drain pending messages for up to 30 seconds
- NAK any messages that can't be processed in time
- Close NATS connections

### Monitoring

#### Consumer Health

Check consumer status via NATS CLI:

```bash
nats consumer info KV_v1-objects meeting-service-kv-consumer
```

**Key metrics:**

- **Num Pending**: Messages waiting to be processed
- **Num Ack Pending**: Messages being processed
- **Num Redelivered**: Messages that failed and are retrying
- **Last Delivered**: Timestamp of last message delivery

#### Processing Logs

Event processing emits structured logs:

```json
{
    "level": "info",
    "msg": "processing KV event",
    "key": "itx-zoom-meetings-v2.abc123",
    "operation": "PUT",
    "num_delivered": 1
}
```

```json
{
    "level": "info",
    "msg": "successfully published meeting event",
    "meeting_id": "abc123",
    "action": "created",
    "duration_ms": 45
}
```

**Error logs:**

```json
{
    "level": "warn",
    "msg": "parent meeting not yet synced, will retry",
    "meeting_id": "abc123",
    "registrant_id": "reg-456",
    "num_delivered": 2
}
```

```json
{
    "level": "error",
    "msg": "failed to publish event after max retries",
    "key": "itx-zoom-meetings-v2.abc123",
    "error": "context deadline exceeded",
    "num_delivered": 3
}
```

### Troubleshooting

#### Consumer Not Processing Messages

**Symptoms:**

- `Num Pending` increasing in consumer info
- No processing logs

**Checks:**

1. Verify consumer is running: `nats consumer info KV_v1-objects meeting-service-kv-consumer`
2. Check service logs for startup errors
3. Verify NATS connectivity: `nats account info`

**Resolution:**

```bash
# Restart service
kubectl rollout restart deployment/meeting-service -n lfx
```

#### Messages Repeatedly Redelivered

**Symptoms:**

- `Num Redelivered` increasing
- Same `num_delivered` value in logs (2 or 3)

**Checks:**

1. Review error logs for specific failure reasons
2. Check parent-child ordering issues
3. Verify ID mapper service availability
4. Confirm project/committee IDs exist

**Resolution:**

```bash
# Check dead letter queue for permanently failed messages
nats stream info KV_v1-objects

# Inspect specific message
nats consumer next KV_v1-objects meeting-service-kv-consumer
```

#### ID Mapping Failures

**Symptoms:**

- Warnings about failed SFID→UUID mappings
- NAK retries for ID mapper errors

**Checks:**

1. Verify ID mapper service health
2. Check if SFIDs exist in v1 system
3. Review `v1-mappings` KV bucket contents

**Resolution:**

```bash
# Query ID mapper directly
curl -H "Authorization: Bearer $TOKEN" \
  "http://id-mapper-service/api/v1/mappings/sfid/{sfid}"

# Check v1-mappings KV bucket
nats kv get v1-mappings "itx-zoom-meetings-v2.{meeting_id}"
```

#### User Enrichment Failures

**Symptoms:**

- Missing usernames in registrant/participant events
- Warnings about v1 user lookup failures

**Checks:**

1. Verify user exists in v1-objects KV bucket: `nats kv get v1-objects "user.{user_id}"`
2. Check if `lf_user_id` field is populated in v1 data

**Resolution:**

User enrichment failures are non-fatal. Events publish with available data:

```json
{
    "uid": "reg-123",
    "username": "",  // Empty if lookup failed
    "email": "user@example.com",
    "first_name": "Unknown",
    "last_name": "User"
}
```

## Performance

### Throughput

**Expected performance:**

- ~1000 events/second per service instance
- Latency p50: 20ms, p95: 50ms, p99: 100ms
- Concurrent message processing: Up to 1000 (MaxAckPending)

### Backpressure Handling

The system handles backpressure through:

- **MaxAckPending: 1000**: Limits concurrent processing to prevent memory exhaustion
- **DeliverLastPerSubjectPolicy**: Skips intermediate updates for same key
- **AckWait: 30s**: Allows sufficient time for complex transformations

### Resource Usage

**Per service instance:**

- Memory: ~200MB baseline + ~50KB per pending message
- CPU: ~0.1 cores baseline + ~0.5 cores per 1000 events/sec
- Network: Dependent on event size (avg ~5KB per event)

## Related Services

### Dependencies

| Service | Purpose | Failure Mode |
| ------- | ------- | ------------ |
| **NATS JetStream** | Event storage and delivery | Service unavailable, all processing stops |
| **ID Mapper** | SFID→UUID mapping | NAK for retry, fallback to SFID if persistent failure |
| **Indexer Service** | Search indexing | Event lost if publish fails after 3 retries |
| **FGA-Sync Service** | Access control sync | Event lost if publish fails after 3 retries |

### Data Flow

```text
v1-sync-helper → NATS KV → Meeting Service → Indexer Service
                                           → FGA-Sync Service
                                           → v1-mappings KV
```

## Development

### Running Locally

```bash
# Set required environment variables
export NATS_URL=nats://localhost:4222
export EVENT_PROCESSING_ENABLED=true
export ITX_BASE_URL=http://localhost:8080
export ITX_CLIENT_ID=your-client-id
export ITX_CLIENT_PRIVATE_KEY="$(cat path/to/private.key)"

# Run service
make run
```

### Testing

#### Unit Tests

```bash
# Run all tests
make test

# Run event handler tests only
go test ./cmd/meeting-api/eventing/...
```

#### Integration Tests

```bash
# Start local NATS server
docker run -p 4222:4222 nats:latest -js

# Run integration tests
go test -tags=integration ./cmd/meeting-api/eventing/...
```

### Adding New Event Types

To add a new event type:

1. **Define event model** in `internal/domain/models/event_models.go`:

   ```go
   type NewEventData struct {
       ID         string    `json:"id"`
       ProjectUID string    `json:"project_uid"`
       // ... more fields
   }
   ```

2. **Add publisher method** to `internal/domain/event_publisher.go`:

   ```go
   PublishNewEvent(ctx context.Context, action string, data *models.NewEventData) error
   ```

3. **Implement handler** in `cmd/meeting-api/eventing/new_event_handler.go`:

   ```go
   func handleNewEventUpdate(ctx context.Context, key string, data map[string]any, ...) bool {
       // Validation
       // Transformation
       // Publishing
       return false // ACK
   }
   ```

4. **Add routing** to `cmd/meeting-api/eventing/kv_handler.go`:

   ```go
   case strings.HasPrefix(key, "new-event-prefix."):
       return handleNewEventUpdate(ctx, key, data, handlers...)
   ```

5. **Implement publisher** in `internal/infrastructure/eventing/nats_publisher.go`:

   ```go
   func (p *NATSPublisher) PublishNewEvent(ctx context.Context, action string, data *models.NewEventData) error {
       return p.publish(ctx, "lfx.index.new_event", data)
   }
   ```

## Message Formats

### Meeting Event

```json
{
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "project_uid": "proj-uuid",
    "title": "Weekly Team Sync",
    "description": "Discuss project progress and blockers",
    "start_time": "2024-01-15T10:00:00Z",
    "duration": 30,
    "timezone": "America/Los_Angeles",
    "visibility": "public",
    "restricted": false,
    "meeting_type": "recurring",
    "early_join_time_minutes": 5,
    "recording_enabled": true,
    "transcript_enabled": true,
    "youtube_upload_enabled": false,
    "artifact_visibility": "meeting_hosts",
    "committees": [
        {
            "uid": "committee-uuid-1",
            "name": "Technical Steering Committee"
        }
    ],
    "occurrences": [
        {
            "occurrence_id": "2024-01-15T10:00:00Z",
            "start_time": "2024-01-15T10:00:00Z",
            "duration": 30,
            "status": "available"
        },
        {
            "occurrence_id": "2024-01-22T10:00:00Z",
            "start_time": "2024-01-22T10:00:00Z",
            "duration": 30,
            "status": "available"
        }
    ],
    "created_at": "2024-01-10T08:00:00Z",
    "modified_at": "2024-01-10T08:00:00Z"
}
```

Envelope `tags` (also sent as `indexing_config.tags`; the data object has no `tags` field):

```json
["550e8400-e29b-41d4-a716-446655440000", "meeting_id:550e8400-e29b-41d4-a716-446655440000", "project_uid:proj-uuid", "title:Weekly Team Sync", "visibility:public", "meeting_type:recurring", "committee_uid:committee-uuid-1"]
```

### Registrant Event

```json
{
    "uid": "reg-uuid",
    "meeting_id": "meeting-uuid",
    "project_uid": "proj-uuid",
    "committee_uid": "committee-uuid",
    "user_id": "platform-user-id",
    "username": "jdoe",
    "email": "john.doe@example.com",
    "first_name": "John",
    "last_name": "Doe",
    "avatar_url": "https://avatars.example.com/jdoe.jpg",
    "org_name": "ACME Corporation",
    "host": true,
    "created_at": "2024-01-10T08:30:00Z",
    "modified_at": "2024-01-10T08:30:00Z"
}
```

Envelope `tags`:

```json
["registrant_uid:reg-uuid", "committee_uid:committee-uuid", "username:jdoe", "email:john.doe@example.com", "host:true"]
```

### Participant Event

```json
{
    "uid": "participant-uuid",
    "meeting_and_occurrence_id": "meeting-uuid_2024-01-15T10:00:00Z",
    "meeting_id": "meeting-uuid",
    "project_uid": "proj-uuid",
    "email": "jane.smith@example.com",
    "first_name": "Jane",
    "last_name": "Smith",
    "host": false,
    "job_title": "Software Engineer",
    "org_name": "Tech Corp",
    "org_is_member": true,
    "org_is_project_member": false,
    "avatar_url": "https://avatars.example.com/jsmith.jpg",
    "username": "jsmith",
    "is_invited": true,
    "is_attended": true,
    "sessions": [
        {
            "uid": "session-1",
            "join_time": "2024-01-15T10:02:00Z",
            "leave_time": "2024-01-15T10:28:00Z",
            "leave_reason": "Left Meeting"
        },
        {
            "uid": "session-2",
            "join_time": "2024-01-15T10:29:00Z",
            "leave_time": "2024-01-15T10:32:00Z",
            "leave_reason": "Left Meeting"
        }
    ],
    "created_at": "2024-01-15T10:02:00Z",
    "modified_at": "2024-01-15T10:32:00Z"
}
```

Envelope `tags`:

```json
["past_meeting_participant_uid:participant-uuid", "meeting_and_occurrence_id:meeting-uuid_2024-01-15T10:00:00Z", "project_uid:proj-uuid", "username:jsmith", "email:jane.smith@example.com", "is_invited:true", "is_attended:true"]
```

### Recording Event

Recording, transcript, and summary tags start with the document's own ID as a bare value, followed by `key:value` tags.

The sample below is the `data` object of a `v1_past_meeting_recording` envelope, followed by the envelope's `tags`. It comes from a record with three files: the `MP4` and `M4A` files stay on the recording, while the `TRANSCRIPT` file goes to the [transcript document](#transcript-event). `recording_count` and `total_size` are copied from the record and still count the transcript file. `platform_meeting_id` is not populated by the handler.

```json
{
    "id": "meeting-uuid_2024-01-15T10:00:00Z",
    "meeting_and_occurrence_id": "meeting-uuid_2024-01-15T10:00:00Z",
    "project_uid": "proj-uuid",
    "project_slug": "my-project",
    "host_email": "host@example.com",
    "host_id": "host-zoom-id",
    "meeting_id": "meeting-uuid",
    "occurrence_id": "2024-01-15T10:00:00Z",
    "platform": "Zoom",
    "platform_meeting_id": "",
    "recording_access": "meeting_hosts",
    "title": "Weekly Team Sync",
    "transcript_access": "meeting_hosts",
    "transcript_enabled": true,
    "visibility": "public",
    "recording_count": 3,
    "recording_files": [
        {
            "download_url": "https://zoom.us/rec/download/...",
            "file_extension": "MP4",
            "file_size": 52428800,
            "file_type": "MP4",
            "id": "file-uuid-1",
            "meeting_id": "123456789",
            "play_url": "https://zoom.us/rec/play/...",
            "recording_start": "2024-01-15T10:00:00Z",
            "recording_end": "2024-01-15T10:30:00Z",
            "recording_type": "shared_screen_with_speaker_view",
            "status": "completed"
        },
        {
            "download_url": "https://zoom.us/rec/download/...",
            "file_extension": "M4A",
            "file_size": 4194304,
            "file_type": "M4A",
            "id": "file-uuid-2",
            "meeting_id": "123456789",
            "play_url": "https://zoom.us/rec/play/...",
            "recording_start": "2024-01-15T10:00:00Z",
            "recording_end": "2024-01-15T10:30:00Z",
            "recording_type": "audio_only",
            "status": "completed"
        }
    ],
    "sessions": [
        {
            "uuid": "session-uuid",
            "share_url": "https://zoom.us/rec/share/...",
            "total_size": 56633344,
            "start_time": "2024-01-15T10:00:00Z"
        }
    ],
    "start_time": "2024-01-15T10:00:00Z",
    "total_size": 56633344,
    "committees": [
        {
            "uid": "committee-uuid-1"
        }
    ],
    "created_at": "2024-01-15T10:35:00Z",
    "updated_at": "2024-01-15T10:35:00Z",
    "created_by": {},
    "updated_by": {}
}
```

Envelope `tags` (also sent as `indexing_config.tags`):

```json
["meeting-uuid_2024-01-15T10:00:00Z", "past_meeting_recording_id:meeting-uuid_2024-01-15T10:00:00Z", "meeting_and_occurrence_id:meeting-uuid_2024-01-15T10:00:00Z", "platform:Zoom", "platform_meeting_id:", "project_uid:proj-uuid", "project_slug:my-project", "platform_meeting_instance_id:session-uuid", "committee_uid:committee-uuid-1"]
```

### Transcript Event

The transcript document built from the same record, sent on `lfx.index.v1_past_meeting_transcript`. It carries only the `TRANSCRIPT` and `TIMELINE` files and has no `recording_access`, `transcript_enabled`, `recording_count` or `platform_meeting_id` fields.

```json
{
    "id": "meeting-uuid_2024-01-15T10:00:00Z",
    "meeting_and_occurrence_id": "meeting-uuid_2024-01-15T10:00:00Z",
    "project_uid": "proj-uuid",
    "project_slug": "my-project",
    "host_email": "host@example.com",
    "host_id": "host-zoom-id",
    "meeting_id": "meeting-uuid",
    "occurrence_id": "2024-01-15T10:00:00Z",
    "platform": "Zoom",
    "transcript_access": "meeting_hosts",
    "title": "Weekly Team Sync",
    "visibility": "public",
    "recording_files": [
        {
            "download_url": "https://zoom.us/rec/download/...",
            "file_extension": "VTT",
            "file_size": 10240,
            "file_type": "TRANSCRIPT",
            "id": "file-uuid-3",
            "meeting_id": "123456789",
            "recording_start": "2024-01-15T10:00:00Z",
            "recording_end": "2024-01-15T10:30:00Z",
            "recording_type": "audio_transcript",
            "status": "completed"
        }
    ],
    "sessions": [
        {
            "uuid": "session-uuid",
            "share_url": "https://zoom.us/rec/share/...",
            "total_size": 56633344,
            "start_time": "2024-01-15T10:00:00Z"
        }
    ],
    "start_time": "2024-01-15T10:00:00Z",
    "total_size": 56633344,
    "committees": [
        {
            "uid": "committee-uuid-1"
        }
    ],
    "created_at": "2024-01-15T10:35:00Z",
    "updated_at": "2024-01-15T10:35:00Z",
    "created_by": {},
    "updated_by": {}
}
```

Envelope `tags`:

```json
["meeting-uuid_2024-01-15T10:00:00Z", "past_meeting_transcript_id:meeting-uuid_2024-01-15T10:00:00Z", "meeting_and_occurrence_id:meeting-uuid_2024-01-15T10:00:00Z", "platform:Zoom", "project_uid:proj-uuid", "project_slug:my-project", "platform_meeting_instance_id:session-uuid", "committee_uid:committee-uuid-1"]
```

### Summary Event

```json
{
    "id": "summary-uuid",
    "meeting_and_occurrence_id": "meeting-uuid_2024-01-15T10:00:00Z",
    "project_uid": "proj-uuid",
    "project_slug": "my-project",
    "meeting_id": "meeting-uuid",
    "occurrence_id": "2024-01-15T10:00:00Z",
    "zoom_meeting_uuid": "zoom-uuid",
    "zoom_meeting_host_id": "host-zoom-id",
    "zoom_meeting_host_email": "host@example.com",
    "zoom_meeting_topic": "Weekly Team Sync",
    "content": "## Overview\nTeam discussed Q1 roadmap.\n\n## Key Topics\n### Feature Planning\nFocus on authentication improvements.\n\n### Bug Review\nIdentified 3 critical bugs.\n\n## Next Steps\n- Schedule design review\n- Update roadmap documentation\n- Assign bug fixes",
    "edited_content": "## Overview\nTeam discussed Q1 roadmap.\n\n## Key Topics\n### Feature Planning\nFocus on OAuth2 authentication.\n\n### Bug Review\nIdentified 3 critical bugs (#123, #124, #125).\n\n## Next Steps\n- Design review (John)\n- Update roadmap (Jane)\n- Assign bug fixes",
    "requires_approval": true,
    "approved": false,
    "platform": "Zoom",
    "zoom_config": {
        "meeting_id": "123456789",
        "meeting_uuid": "zoom-uuid"
    },
    "email_sent": false,
    "committees": [
        {
            "uid": "committee-uuid-1"
        }
    ],
    "created_at": "2024-01-15T11:00:00Z",
    "updated_at": "2024-01-15T11:30:00Z",
    "created_by": {},
    "updated_by": {}
}
```

Envelope `tags` (also sent as `indexing_config.tags`):

```json
["summary-uuid", "past_meeting_summary_id:summary-uuid", "meeting_and_occurrence_id:meeting-uuid_2024-01-15T10:00:00Z", "meeting_id:meeting-uuid", "platform:Zoom", "project_uid:proj-uuid", "project_slug:my-project", "title:Weekly Team Sync", "committee_uid:committee-uuid-1"]
```

---

**Document Version**: 1.0
**Last Updated**: 2026-03-31
**Maintained By**: LFX Platform Team
