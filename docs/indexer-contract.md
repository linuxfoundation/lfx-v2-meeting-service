# Indexer Contract — Meeting Service

This document is the authoritative reference for all data the meeting service sends to the indexer service, which makes resources searchable via the [query service](https://github.com/linuxfoundation/lfx-v2-query-service).

**Update this document in the same PR as any change to indexer message construction.**

**Convention:** Tags and parent refs containing a `{value}` placeholder are only emitted when the corresponding field is non-empty.

---

## Resource Types

- [V1 Meeting](#v1-meeting)
- [V1 Meeting Registrant](#v1-meeting-registrant)
- [V1 Meeting RSVP](#v1-meeting-rsvp)
- [V1 Past Meeting](#v1-past-meeting)
- [V1 Past Meeting Participant](#v1-past-meeting-participant)
- [V1 Past Meeting Recording](#v1-past-meeting-recording)
- [V1 Past Meeting Transcript](#v1-past-meeting-transcript)
- [V1 Past Meeting Summary](#v1-past-meeting-summary)
- [V1 Meeting Attachment](#v1-meeting-attachment)
- [V1 Past Meeting Attachment](#v1-past-meeting-attachment)

---

## V1 Meeting

**Object type:** `v1_meeting`

**NATS subject:** `lfx.index.v1_meeting`

**Source struct:** `internal/domain/models/event_models.go` — `MeetingEventData`

**Indexed on:** create, update, delete of a meeting (sourced from the v1 meeting data).

### Data Schema

These fields are indexed and queryable via `filters` or `cel_filter` in the query service.

| Field | Type | Description |
|---|---|---|
| `id` | string | Meeting ID (used as the object ID) |
| `project_sfid` | string | Salesforce ID of the associated LF project |
| `project_uid` | string | v2 UUID of the associated LF project |
| `committee` | string | v1 Salesforce ID of the primary committee |
| `committee_uid` | string (optional) | v2 UUID of the primary committee; omitted when empty |
| `committee_filters` | []string | List of committee filter values |
| `committees` | []object (optional) | Associated committees (see [Committee schema](#committee-schema)); omitted when empty |
| `user_id` | string | Zoom user ID of the meeting host |
| `title` | string | Meeting title (from Zoom `topic`) |
| `description` | string | Meeting description (from Zoom `agenda`) |
| `visibility` | string | Meeting visibility on LFX (e.g., `"public"`, `"private"`) |
| `meeting_type` | string | Zoom meeting type |
| `start_time` | string | Start time of the first occurrence (RFC3339) |
| `timezone` | string | Meeting timezone (IANA, e.g., `"America/Los_Angeles"`) |
| `duration` | int | Meeting duration in minutes |
| `early_join_time_minutes` | int | Minutes before start time that attendees can join |
| `last_end_time` | int64 | End time of the last occurrence (Unix timestamp) |
| `host_key` | string | Six-digit Zoom host PIN (rotated weekly) |
| `join_url` | string | LFX meeting join page URL |
| `password` | string | UUID password for the join page |
| `restricted` | bool | Whether only invited users can join |
| `artifact_visibility` | string | Visibility of meeting artifacts (recordings, transcripts, summaries) |
| `recording_enabled` | bool | Whether Zoom recording is enabled |
| `transcript_enabled` | bool | Whether Zoom transcript is enabled |
| `recording_access` | string | Recording access level (`"public"`, `"meeting_hosts"`, `"meeting_participants"`) |
| `transcript_access` | string | Transcript access level |
| `created_at` | string | Creation time (RFC3339) |
| `updated_at` | string | Last update time (RFC3339) |
| `created_by` | object | User who created the meeting (see [User Reference schema](#user-reference-schema)) |
| `updated_by` | object | User who last updated the meeting (see [User Reference schema](#user-reference-schema)) |
| `updated_by_list` | []object (optional) | All users who have updated the meeting |
| `use_new_invite_email_address` | bool | Whether to use the new invite email address |
| `recurrence` | object (optional) | Zoom recurrence pattern (see [Recurrence schema](#recurrence-schema)) |
| `occurrences` | []object (optional) | Upcoming meeting occurrences (see [Occurrence schema](#occurrence-schema)) |
| `cancelled_occurrences` | []string (optional) | Occurrence IDs that have been cancelled |
| `updated_occurrences` | []object (optional) | Occurrences with custom overrides (see [Updated Occurrence schema](#updated-occurrence-schema)) |
| `ics_uid_timezone` | string (optional) | Timezone anchored for calendar UID generation |
| `ics_additional_uids` | []string (optional) | Additional calendar event UIDs for updated occurrence sequences |
| `zoom_config` | object | Zoom-specific configuration (see [Zoom Config schema](#zoom-config-schema)) |
| `ai_summary_access` | string (optional) | AI summary access level |
| `youtube_upload_enabled` | bool (optional) | Whether recording is uploaded to YouTube |
| `concurrent_zoom_user_enabled` | bool (optional) | Whether meeting uses concurrent Zoom license pool |
| `last_bulk_registrant_job_status` | string | Status of the last bulk registrant import job |
| `last_bulk_registrants_job_failed_count` | int | Failed record count from the last bulk registrant job |
| `last_bulk_registrants_job_warning_count` | int | Warning record count from the last bulk registrant job |
| `last_mailing_list_members_sync_job_status` | string | Status of the last mailing list sync job |
| `last_mailing_list_members_sync_job_failed_count` | int | Failed count from last mailing list sync job |
| `mailing_list_group_ids` | []string | Mailing list group IDs associated with this meeting |
| `last_mailing_list_members_sync_job_warning_count` | int | Warning count from last mailing list sync job |
| `use_unique_ics_uid` | string | UUID used as the unique ICS UID for calendar events (empty string when not set) |
| `show_meeting_attendees` | bool | Whether attendee data is visible to other attendees |
| `organizers` | []string | Auth0 sub-format usernames of meeting organizers |

#### Committee Schema

Each entry in `committees` has:

| Field | Type | Description |
|---|---|---|
| `uid` | string | Committee v2 UUID |
| `allowed_voting_statuses` | []string (optional) | Voting status filter strings for the committee; omitted when empty |

#### User Reference Schema

Used by `created_by`, `updated_by`, and entries in `updated_by_list`:

| Field | Type | Description |
|---|---|---|
| `user_id` | string (optional) | Auth0 user identifier |
| `username` | string (optional) | LFX username |
| `email` | string (optional) | User email address |
| `name` | string (optional) | Display name |
| `profile_picture` | string (optional) | Profile picture URL |

#### Recurrence Schema

| Field | Type | Description |
|---|---|---|
| `type` | int | Recurrence type (1=daily, 2=weekly, 3=monthly) |
| `repeat_interval` | int | Interval between occurrences |
| `weekly_days` | string (optional) | Days of week for weekly recurrence |
| `monthly_day` | int (optional) | Day of month for monthly recurrence |
| `monthly_week` | int (optional) | Week of month for monthly recurrence |
| `monthly_week_day` | int (optional) | Day of week paired with `monthly_week` |
| `end_times` | int (optional) | Number of times to repeat |
| `end_date_time` | string (optional) | End date/time for the recurrence pattern (RFC3339) |

#### Occurrence Schema

| Field | Type | Description |
|---|---|---|
| `occurrence_id` | string | Unix timestamp of occurrence start (used as ID) |
| `start_time` | string | Occurrence start time (RFC3339) |
| `duration` | int | Duration in minutes |
| `is_cancelled` | bool | Whether this occurrence is cancelled |
| `title` | string | Occurrence title |
| `description` | string | Occurrence description |
| `recurrence` | object (optional) | Override recurrence pattern for this occurrence |
| `response_count_yes` | int | Accepted RSVP count |
| `response_count_no` | int | Declined RSVP count |
| `registrant_count` | int | Registrant count |

#### Updated Occurrence Schema

| Field | Type | Description |
|---|---|---|
| `old_occurrence_id` | string | Original occurrence ID (Unix timestamp) |
| `new_occurrence_id` | string | New occurrence ID after start time change |
| `timezone` | string | Updated timezone |
| `duration` | int | Updated duration in minutes |
| `title` | string | Updated title |
| `description` | string | Updated description |
| `recurrence` | object (optional) | Updated recurrence pattern |
| `all_following` | bool | Whether changes apply to all following occurrences |

#### Zoom Config Schema

| Field | Type | Description |
|---|---|---|
| `meeting_id` | string (optional) | Zoom numeric meeting ID |
| `passcode` | string (optional) | Zoom meeting passcode |
| `ai_companion_enabled` | bool | Whether Zoom AI Companion is enabled |
| `ai_summary_require_approval` | bool | Whether AI summaries require approval before publishing |

### Tags

| Tag Format | Example | Purpose |
|---|---|---|
| `{id}` | `93699735000` | Direct lookup by meeting ID |
| `meeting_id:{id}` | `meeting_id:93699735000` | Namespaced lookup by meeting ID |
| `project_uid:{value}` | `project_uid:cbef1ed5-17dc-4a50-84e2-6cddd70f6878` | Find meetings for a project |
| `title:{value}` | `title:TSC Monthly Meeting` | Find meetings by title |
| `visibility:{value}` | `visibility:public` | Find meetings by visibility |
| `meeting_type:{value}` | `meeting_type:recurring` | Find meetings by type |
| `committee_uid:{value}` | `committee_uid:061a110a-...` | Find meetings by committee |

### Access Control (IndexingConfig)

| Field | Value |
|---|---|
| `access_check_object` | `v1_meeting:{id}` |
| `access_check_relation` | `viewer` |
| `history_check_object` | `v1_meeting:{id}` |
| `history_check_relation` | `auditor` |
| `public` | `true` when `visibility == "public"`, `false` otherwise |

### Search Behavior

| Field | Value |
|---|---|
| `sort_name` | `title` (trimmed) |
| `name_and_aliases` | `[title]` (omitted when empty) |
| `fulltext` | `title` + `description` (space-joined, deduplicated, omits empty values) |

### Parent References

| Ref | Condition |
|---|---|
| `project:{project_uid}` | Only when `project_uid` is non-empty |
| `committee:{uid}` | For each entry in `committees` with a non-empty `uid` |

---

## V1 Meeting Registrant

**Object type:** `v1_meeting_registrant`

**NATS subject:** `lfx.index.v1_meeting_registrant`

**Source struct:** `internal/domain/models/event_models.go` — `RegistrantEventData`

**Indexed on:** create, update, delete of a meeting registrant.

### Data Schema

| Field | Type | Description |
|---|---|---|
| `uid` | string | Registrant unique identifier (UUID) |
| `meeting_id` | string | ID of the associated meeting |
| `type` | string | Registrant type |
| `committee_uid` | string | v2 UUID of the associated committee (empty when not a committee registrant) |
| `user_id` | string | Auth0 user ID of the registrant |
| `email` | string | Email address for meeting invites |
| `case_insensitive_email` | string | Lowercase version of `email` |
| `first_name` | string | Registrant first name |
| `last_name` | string | Registrant last name |
| `org_name` | string (optional) | Organization name |
| `org_is_member` | bool (optional) | Whether the organization is an LF member |
| `org_is_project_member` | bool (optional) | Whether the organization is a project member |
| `job_title` | string (optional) | Job title |
| `host` | bool | Whether the registrant is a host |
| `occurrence` | string (optional) | Occurrence ID if invited to a specific occurrence only |
| `avatar_url` | string | Profile picture URL |
| `username` | string (optional) | LFX username |
| `last_invite_received_time` | string | Timestamp of last invite sent (RFC3339) |
| `last_invite_received_message_id` | string (optional) | SES message ID of last invite |
| `last_invite_delivery_successful` | bool (optional) | Whether last invite was delivered |
| `last_invite_delivered_time` | string (optional) | Delivery timestamp (RFC3339) |
| `last_invite_bounced` | bool (optional) | Whether last invite bounced |
| `last_invite_bounced_time` | string (optional) | Bounce timestamp (RFC3339) |
| `last_invite_bounced_type` | string (optional) | SES bounce type |
| `last_invite_bounced_sub_type` | string (optional) | SES bounce subtype |
| `last_invite_bounced_diagnostic_code` | string (optional) | SES bounce diagnostic code |
| `created_at` | string | Creation time (RFC3339) |
| `updated_at` | string | Last update time (RFC3339) |
| `created_by` | object | User who created the registrant (see [User Reference schema](#user-reference-schema)) |
| `updated_by` | object | User who last updated the registrant (see [User Reference schema](#user-reference-schema)) |

### Tags

| Tag Format | Example | Purpose |
|---|---|---|
| `registrant_uid:{uid}` | `registrant_uid:a1b2c3d4-...` | Lookup by registrant UID |
| `committee_uid:{value}` | `committee_uid:061a110a-...` | Find registrants by committee |
| `username:{value}` | `username:jdoe` | Find registrants by username |
| `email:{value}` | `email:jdoe@example.com` | Find registrants by email |
| `host:true` | `host:true` | Find registrants who are hosts |

### Access Control (IndexingConfig)

| Field | Value |
|---|---|
| `access_check_object` | `v1_meeting:{meeting_id}` (access checked on the parent meeting) |
| `access_check_relation` | `viewer` |
| `history_check_object` | `v1_meeting:{meeting_id}` |
| `history_check_relation` | `auditor` |
| `public` | `false` (always) |

### Search Behavior

| Field | Value |
|---|---|
| `sort_name` | `email` (trimmed) |
| `name_and_aliases` | `[username, email]` (deduplicated, omits empty values) |
| `fulltext` | `sort_name` + `name_and_aliases` (space-joined, deduplicated) |

### Parent References

| Ref | Condition |
|---|---|
| `meeting:{meeting_id}` | Only when `meeting_id` is non-empty |
| `committee:{committee_uid}` | Only when `committee_uid` is non-empty |

---

## V1 Meeting RSVP

**Object type:** `v1_meeting_rsvp`

**NATS subject:** `lfx.index.v1_meeting_rsvp`

**Source struct:** `internal/domain/models/event_models.go` — `InviteResponseEventData`

**Indexed on:** create, update, delete of a meeting RSVP (invite response).

### Data Schema

| Field | Type | Description |
|---|---|---|
| `id` | string | RSVP unique identifier |
| `meeting_and_occurrence_id` | string | Combined meeting+occurrence ID (e.g., `{meeting_id}:{occurrence_id}`) |
| `meeting_id` | string | Parent meeting ID |
| `occurrence_id` | string (optional) | Occurrence ID for occurrence-specific RSVPs |
| `registrant_id` | string | UID of the associated registrant |
| `project_uid` | string | v2 UUID of the associated project |
| `user_id` | string | Auth0 user ID |
| `username` | string (optional) | LFX username |
| `name` | string (optional) | Respondent display name |
| `email` | string | Respondent email address |
| `org` | string (optional) | Organization name |
| `job_title` | string (optional) | Job title |
| `response_type` | string | RSVP response (`"accepted"`, `"declined"`, `"maybe"`) |
| `scope` | string | Response scope (`"all"`, `"single"`, `"this_and_following"`) |
| `is_recurring` | bool | Whether the parent meeting is recurring |
| `created_at` | string (RFC3339) | Creation time |
| `modified_at` | string (RFC3339) | Last modification time |

### Tags

| Tag Format | Example | Purpose |
|---|---|---|
| `{id}` | `abc123-...` | Direct lookup by RSVP ID |
| `invite_response_uid:{id}` | `invite_response_uid:abc123-...` | Namespaced lookup by RSVP ID |
| `meeting_and_occurrence_id:{value}` | `meeting_and_occurrence_id:93699735000:1700000000` | Find RSVPs for a specific occurrence |
| `meeting_id:{value}` | `meeting_id:93699735000` | Find RSVPs for a meeting |
| `registrant_uid:{value}` | `registrant_uid:a1b2c3d4-...` | Find RSVPs for a registrant |
| `email:{value}` | `email:jdoe@example.com` | Find RSVPs by email |
| `username:{value}` | `username:jdoe` | Find RSVPs by username |

### Access Control (IndexingConfig)

| Field | Value |
|---|---|
| `access_check_object` | `v1_meeting:{meeting_id}` (access checked on the parent meeting) |
| `access_check_relation` | `viewer` |
| `history_check_object` | `v1_meeting:{meeting_id}` |
| `history_check_relation` | `auditor` |
| `public` | `false` (always) |

### Search Behavior

| Field | Value |
|---|---|
| `sort_name` | `email` (trimmed) |
| `name_and_aliases` | `[username, email]` (deduplicated, omits empty values) |
| `fulltext` | `sort_name` + `name_and_aliases` (space-joined, deduplicated) |

### Parent References

| Ref | Condition |
|---|---|
| `meeting:{meeting_id}` | Only when `meeting_id` is non-empty |

---

## V1 Past Meeting

**Object type:** `v1_past_meeting`

**NATS subject:** `lfx.index.v1_past_meeting`

**Source struct:** `internal/domain/models/event_models.go` — `PastMeetingEventData`

**Indexed on:** create, update, delete of a past meeting (a completed occurrence of a meeting).

### Data Schema

| Field | Type | Description |
|---|---|---|
| `id` | string | Past meeting unique identifier (UUID) |
| `meeting_id` | string | ID of the originating active meeting |
| `meeting_and_occurrence_id` | string | Combined ID used across related resources (e.g., `{meeting_id}:{occurrence_id}`) |
| `occurrence_id` | string (optional) | Occurrence ID within the recurring meeting |
| `proj_id` | string (optional) | Salesforce project ID |
| `project_uid` | string | v2 UUID of the associated project |
| `project_slug` | string (optional) | URL slug of the associated project |
| `committee` | string (optional) | v1 Salesforce ID of the primary committee |
| `committee_uid` | string (optional) | v2 UUID of the primary committee |
| `committee_filters` | []string (optional) | Committee filter values |
| `title` | string | Meeting title |
| `description` | string | Meeting description |
| `start_time` | string (RFC3339) | Occurrence start time |
| `end_time` | string (RFC3339) | Occurrence end time |
| `duration` | int | Actual duration in minutes |
| `timezone` | string | Meeting timezone (IANA) |
| `meeting_type` | string (optional) | Zoom meeting type |
| `committees` | []object \| null | Associated committees (see [Committee schema](#committee-schema)); may be null when unset |
| `visibility` | string (optional) | Meeting visibility |
| `artifact_visibility` | string (optional) | Visibility of meeting artifacts |
| `restricted` | bool | Whether the meeting was restricted |
| `recording_enabled` | bool | Whether recording was enabled |
| `recording_access` | string (optional) | Recording access level |
| `transcript_enabled` | bool | Whether transcript was enabled |
| `transcript_access` | string (optional) | Transcript access level |
| `zoom_ai_enabled` | bool (optional) | Whether Zoom AI Companion was enabled |
| `ai_summary_access` | string (optional) | AI summary access level (`"public"`, `"meeting_hosts"`, `"meeting_participants"`) |
| `require_ai_summary_approval` | bool (optional) | Whether AI summary requires approval |
| `early_join_time_minutes` | int (optional) | Early join buffer in minutes |
| `youtube_link` | string (optional) | YouTube recording link |
| `platform` | string (optional) | Meeting platform (e.g., `"Zoom"`) |
| `platform_meeting_id` | string (optional) | Platform-specific meeting ID |
| `recording_password` | string (optional) | Password for the recording |
| `zoom_config` | object (optional) | Zoom-specific configuration (see [Zoom Config schema](#zoom-config-schema)) |
| `is_manually_created` | bool (optional) | Whether this past meeting was created manually |
| `sessions` | []object (optional) | Zoom meeting instances within this past meeting (each has `uuid`, `start_time`, `end_time`) |
| `created_at` | string (RFC3339) | Creation time |
| `updated_at` | string (RFC3339) | Last update time |
| `created_by` | object | User who created the record (see [User Reference schema](#user-reference-schema)) |
| `updated_by` | object | User who last updated the record (see [User Reference schema](#user-reference-schema)) |
| `updated_by_list` | []object (optional) | All users who have updated the record |

### Tags

| Tag Format | Example | Purpose |
|---|---|---|
| `past_meeting_id:{id}` | `past_meeting_id:a1b2c3d4-...` | Lookup by past meeting UUID |
| `meeting_id:{value}` | `meeting_id:93699735000` | Find past meetings for an active meeting |
| `project_uid:{value}` | `project_uid:cbef1ed5-...` | Find past meetings for a project |
| `title:{value}` | `title:TSC Monthly Meeting` | Find past meetings by title |
| `timezone:{value}` | `timezone:America/Los_Angeles` | Find past meetings by timezone |
| `committee_uid:{value}` | `committee_uid:061a110a-...` | Find past meetings for a committee |

### Access Control (IndexingConfig)

| Field | Value |
|---|---|
| `access_check_object` | `v1_past_meeting:{id}` |
| `access_check_relation` | `viewer` |
| `history_check_object` | `v1_past_meeting:{id}` |
| `history_check_relation` | `auditor` |
| `public` | `false` (always) |

### Search Behavior

| Field | Value |
|---|---|
| `sort_name` | `title` (trimmed) |
| `name_and_aliases` | `[title]` (omitted when empty) |
| `fulltext` | `title` + `description` (space-joined, deduplicated, omits empty values) |

### Parent References

| Ref | Condition |
|---|---|
| `project:{project_uid}` | Only when `project_uid` is non-empty |
| `committee:{uid}` | For each entry in `committees` with a non-empty `uid` |

---

## V1 Past Meeting Participant

**Object type:** `v1_past_meeting_participant`

**NATS subject:** `lfx.index.v1_past_meeting_participant`

**Source struct:** `internal/domain/models/event_models.go` — `PastMeetingParticipantEventData`

**Indexed on:** create, update, delete of a past meeting participant (invitee or attendee).

### Data Schema

| Field | Type | Description |
|---|---|---|
| `uid` | string | Participant unique identifier (UUID) |
| `meeting_and_occurrence_id` | string | Combined meeting+occurrence ID of the parent past meeting |
| `meeting_id` | string | ID of the originating active meeting |
| `project_uid` | string | v2 UUID of the associated project |
| `project_slug` | string (optional) | URL slug of the associated project, propagated from the parent past meeting at index time |
| `email` | string | Participant email address |
| `first_name` | string | Participant first name |
| `last_name` | string | Participant last name |
| `host` | bool | Whether the participant was the host |
| `job_title` | string (optional) | Job title |
| `org_name` | string (optional) | Organization name |
| `org_is_member` | bool | Whether the organization is an LF member |
| `org_is_project_member` | bool | Whether the organization is a project member |
| `avatar_url` | string (optional) | Profile picture URL |
| `username` | string (optional) | LFX username |
| `is_invited` | bool | Whether the participant was invited |
| `is_attended` | bool | Whether the participant attended |
| `is_unknown` | bool | Whether the attendee could not be matched to any known user (attendee records only; `false` for invitee-only records) |
| `is_ai_reconciled` | bool | Whether the attendee record was last updated via AI reconciliation (attendee records only; `false` for invitee-only records) |
| `is_auto_matched` | bool | Whether the attendee was automatically matched to an invitee by name (attendee records only; `false` for invitee-only records) |
| `zoom_user_name` | string | Zoom display name of the attendee (attendee records only; `""` for invitee-only records) |
| `mapped_invitee_name` | string | Full name of the invitee the attendee was auto-matched to (attendee records only; `""` for invitee-only records) |
| `sessions` | []object (optional) | Join/leave sessions (each has `uid`, `join_time`, `leave_time`, `leave_reason`) |
| `committee_uid` | string (optional) | v2 UUID of the committee the participant is associated with; sourced from the participant's own `committee_id` field |
| `created_at` | string (RFC3339) | Creation time |
| `updated_at` | string (RFC3339) | Last update time |

### Tags

| Tag Format | Example | Purpose |
|---|---|---|
| `past_meeting_participant_uid:{uid}` | `past_meeting_participant_uid:a1b2c3d4-...` | Lookup by participant UID |
| `meeting_and_occurrence_id:{value}` | `meeting_and_occurrence_id:93699735000:1700000000` | Find participants for a past meeting |
| `project_uid:{value}` | `project_uid:cbef1ed5-...` | Find participants for a project |
| `project_slug:{value}` | `project_slug:my-project` | Find participants by project slug |
| `committee_uid:{value}` | `committee_uid:061a110a-...` | Find participants by committee |
| `username:{value}` | `username:jdoe` | Find participants by username |
| `email:{value}` | `email:jdoe@example.com` | Find participants by email |
| `is_invited:true` | `is_invited:true` | Find invited participants |
| `is_attended:true` | `is_attended:true` | Find attendees |

### Access Control (IndexingConfig)

| Field | Value |
|---|---|
| `access_check_object` | `v1_past_meeting:{meeting_and_occurrence_id}` (access checked on the parent past meeting) |
| `access_check_relation` | `viewer` |
| `history_check_object` | `v1_past_meeting:{meeting_and_occurrence_id}` |
| `history_check_relation` | `auditor` |
| `public` | `false` (always) |

### Search Behavior

| Field | Value |
|---|---|
| `sort_name` | `email` (trimmed) |
| `name_and_aliases` | `[first_name, last_name, username]` (deduplicated, omits empty values) |
| `fulltext` | `sort_name` + `name_and_aliases` (space-joined, deduplicated) |

### Parent References

| Ref | Condition |
|---|---|
| `past_meeting:{meeting_and_occurrence_id}` | Only when `meeting_and_occurrence_id` is non-empty |
| `project:{project_uid}` | Only when `project_uid` is non-empty |
| `committee:{committee_uid}` | Only when `committee_uid` is non-empty |

---

## V1 Past Meeting Recording

**Object type:** `v1_past_meeting_recording`

**NATS subject:** `lfx.index.v1_past_meeting_recording`

**Source struct:** `internal/domain/models/event_models.go` — `RecordingEventData`

**Indexed on:** create, update, delete of a past meeting recording.

### Data Schema

| Field | Type | Description |
|---|---|---|
| `id` | string | Recording unique identifier |
| `meeting_and_occurrence_id` | string | Combined meeting+occurrence ID of the parent past meeting |
| `project_uid` | string | v2 UUID of the associated project |
| `project_slug` | string | URL slug of the associated project |
| `host_email` | string | Email of the meeting host |
| `host_id` | string | Zoom user ID of the host |
| `meeting_id` | string | ID of the originating active meeting |
| `occurrence_id` | string | Occurrence ID |
| `platform` | string | Meeting platform (always `"Zoom"`) |
| `platform_meeting_id` | string | Zoom numeric meeting ID |
| `recording_access` | string | Access level (`"public"`, `"meeting_hosts"`, `"meeting_participants"`) |
| `title` | string | Recording title |
| `transcript_access` | string (optional) | Transcript access level |
| `transcript_enabled` | bool | Whether transcript is enabled |
| `visibility` | string | Recording visibility |
| `recording_count` | int | Number of recording files |
| `recording_files` | []object | Recording files (see [Recording File schema](#recording-file-schema)) |
| `sessions` | []object | Recording sessions (see [Recording Session schema](#recording-session-schema)) |
| `start_time` | string (RFC3339) | Recording start time |
| `total_size` | int64 | Total size of all recording files in bytes |
| `committees` | []object (optional) | Associated committees sourced from the parent past meeting (see [Committee schema](#committee-schema)) |
| `created_at` | string (RFC3339) | Creation time |
| `updated_at` | string (RFC3339) | Last update time |
| `created_by` | object | User who created the record (see [User Reference schema](#user-reference-schema)) |
| `updated_by` | object | User who last updated the record (see [User Reference schema](#user-reference-schema)) |

#### Recording File Schema

| Field | Type | Description |
|---|---|---|
| `id` | string | File unique identifier |
| `meeting_id` | string | Associated meeting ID |
| `file_type` | string | File type (e.g., `"MP4"`, `"M4A"`) |
| `file_extension` | string | File extension |
| `file_size` | int64 | File size in bytes |
| `recording_type` | string | Zoom recording type (e.g., `"shared_screen_with_speaker_view"`) |
| `status` | string | File status |
| `recording_start` | string (RFC3339) | Recording start time |
| `recording_end` | string (RFC3339) | Recording end time |
| `download_url` | string (optional) | Download URL |
| `play_url` | string (optional) | Playback URL |

#### Recording Session Schema

| Field | Type | Description |
|---|---|---|
| `uuid` | string | Zoom meeting instance UUID |
| `start_time` | string (RFC3339) | Session start time |
| `total_size` | int64 | Session total size in bytes |
| `share_url` | string (optional) | Share URL for the session |

### Tags

| Tag Format | Example | Purpose |
|---|---|---|
| `{id}` | `abc123-...` | Direct lookup by recording ID |
| `past_meeting_recording_id:{id}` | `past_meeting_recording_id:abc123-...` | Namespaced lookup by recording ID |
| `meeting_and_occurrence_id:{value}` | `meeting_and_occurrence_id:93699735000:1700000000` | Find recordings for a past meeting |
| `platform:Zoom` | `platform:Zoom` | All recordings (platform is always Zoom) |
| `platform_meeting_id:{value}` | `platform_meeting_id:93699735000` | Find recordings by Zoom meeting ID |
| `platform_meeting_instance_id:{uuid}` | `platform_meeting_instance_id:abc...` | Find recordings by Zoom session UUID |
| `committee_uid:{value}` | `committee_uid:abc123...` | Find recordings by committee |

### Access Control (IndexingConfig)

| Field | Value |
|---|---|
| `access_check_object` | `v1_past_meeting:{meeting_and_occurrence_id}` (access checked on the parent past meeting) |
| `access_check_relation` | `viewer` |
| `history_check_object` | `v1_past_meeting:{meeting_and_occurrence_id}` |
| `history_check_relation` | `auditor` |
| `public` | `true` when `recording_access == "public"`, `false` otherwise |

### Search Behavior

| Field | Value |
|---|---|
| `sort_name` | `title` (trimmed) |
| `name_and_aliases` | `[title]` (omitted when empty) |
| `fulltext` | `sort_name` + `name_and_aliases` (space-joined, deduplicated) |

### Parent References

| Ref | Condition |
|---|---|
| `past_meeting:{meeting_and_occurrence_id}` | Always set |
| `project:{project_uid}` | Set when `project_uid` is non-empty |
| `committee:{uid}` | Set once per entry in `committees` with a non-empty `uid` |

---

## V1 Past Meeting Transcript

**Object type:** `v1_past_meeting_transcript`

**NATS subject:** `lfx.index.v1_past_meeting_transcript`

**Source struct:** `internal/domain/models/event_models.go` — `TranscriptEventData`

**Indexed on:** create, update, delete of a past meeting transcript.

### Data Schema

| Field | Type | Description |
|---|---|---|
| `id` | string | Transcript unique identifier |
| `meeting_and_occurrence_id` | string | Combined meeting+occurrence ID of the parent past meeting |
| `project_uid` | string | v2 UUID of the associated project |
| `project_slug` | string | URL slug of the associated project |
| `host_email` | string | Email of the meeting host |
| `host_id` | string | Zoom user ID of the host |
| `meeting_id` | string | ID of the originating active meeting |
| `occurrence_id` | string | Occurrence ID |
| `platform` | string | Meeting platform (always `"Zoom"`) |
| `transcript_access` | string | Access level (`"public"`, `"meeting_hosts"`, `"meeting_participants"`) |
| `title` | string | Transcript title |
| `visibility` | string | Transcript visibility |
| `recording_files` | []object | Associated recording files (see [Recording File schema](#recording-file-schema)) |
| `sessions` | []object | Recording sessions (see [Recording Session schema](#recording-session-schema)) |
| `start_time` | string (RFC3339) | Transcript start time |
| `total_size` | int64 | Total size in bytes |
| `committees` | []object (optional) | Associated committees sourced from the parent past meeting (see [Committee schema](#committee-schema)) |
| `created_at` | string (RFC3339) | Creation time |
| `updated_at` | string (RFC3339) | Last update time |
| `created_by` | object | User who created the record (see [User Reference schema](#user-reference-schema)) |
| `updated_by` | object | User who last updated the record (see [User Reference schema](#user-reference-schema)) |

### Tags

| Tag Format | Example | Purpose |
|---|---|---|
| `{id}` | `abc123-...` | Direct lookup by transcript ID |
| `past_meeting_transcript_id:{id}` | `past_meeting_transcript_id:abc123-...` | Namespaced lookup by transcript ID |
| `meeting_and_occurrence_id:{value}` | `meeting_and_occurrence_id:93699735000:1700000000` | Find transcripts for a past meeting |
| `platform:Zoom` | `platform:Zoom` | All transcripts (platform is always Zoom) |
| `platform_meeting_instance_id:{uuid}` | `platform_meeting_instance_id:abc...` | Find transcripts by Zoom session UUID |
| `committee_uid:{value}` | `committee_uid:abc123...` | Find transcripts by committee |

### Access Control (IndexingConfig)

| Field | Value |
|---|---|
| `access_check_object` | `v1_past_meeting:{meeting_and_occurrence_id}` (access checked on the parent past meeting) |
| `access_check_relation` | `viewer` |
| `history_check_object` | `v1_past_meeting:{meeting_and_occurrence_id}` |
| `history_check_relation` | `auditor` |
| `public` | `true` when `transcript_access == "public"`, `false` otherwise |

### Search Behavior

| Field | Value |
|---|---|
| `sort_name` | `title` (trimmed) |
| `name_and_aliases` | `[title]` (omitted when empty) |
| `fulltext` | `sort_name` + `name_and_aliases` (space-joined, deduplicated) |

### Parent References

| Ref | Condition |
|---|---|
| `past_meeting:{meeting_and_occurrence_id}` | Always set |
| `project:{project_uid}` | Set when `project_uid` is non-empty |
| `committee:{uid}` | Set once per entry in `committees` with a non-empty `uid` |

---

## V1 Past Meeting Summary

**Object type:** `v1_past_meeting_summary`

**NATS subject:** `lfx.index.v1_past_meeting_summary`

**Source struct:** `internal/domain/models/event_models.go` — `SummaryEventData`

**Indexed on:** create, update, delete of an AI-generated past meeting summary.

### Data Schema

| Field | Type | Description |
|---|---|---|
| `id` | string | Summary unique identifier |
| `meeting_and_occurrence_id` | string | Combined meeting+occurrence ID of the parent past meeting |
| `project_uid` | string | v2 UUID of the associated project |
| `meeting_id` | string | ID of the originating active meeting |
| `occurrence_id` | string | Occurrence ID |
| `zoom_meeting_uuid` | string | Zoom meeting UUID from the webhook event |
| `zoom_meeting_host_id` | string | Zoom user ID of the host |
| `zoom_meeting_host_email` | string | Email of the host |
| `zoom_meeting_topic` | string | Zoom meeting topic |
| `zoom_webhook_event` | string (optional) | Zoom webhook event type that triggered this summary |
| `summary_title` | string (optional) | AI-generated summary title |
| `summary_start_time` | string (optional) | Summary start time (RFC3339) |
| `summary_end_time` | string (optional) | Summary end time (RFC3339) |
| `summary_created_time` | string (optional) | Time the summary was generated (RFC3339) |
| `summary_last_modified_time` | string (optional) | Last modification time (RFC3339) |
| `content` | string | Consolidated markdown content |
| `edited_content` | string | Edited markdown content (may differ from `content`) |
| `requires_approval` | bool | Whether this summary requires human approval before publishing |
| `approved` | bool | Whether this summary has been approved |
| `platform` | string | Meeting platform (always `"Zoom"`) |
| `zoom_config` | object | Zoom identifiers (has `meeting_id` string and `meeting_uuid` string) |
| `email_sent` | bool | Whether a summary notification email has been sent |
| `committees` | []object (optional) | Associated committees sourced from the parent past meeting (see [Committee schema](#committee-schema)) |
| `created_at` | string (RFC3339) | Creation time |
| `updated_at` | string (RFC3339) | Last update time |
| `created_by` | object | User who created the record (see [User Reference schema](#user-reference-schema)) |
| `updated_by` | object | User who last updated the record (see [User Reference schema](#user-reference-schema)) |

### Tags

| Tag Format | Example | Purpose |
|---|---|---|
| `{id}` | `abc123-...` | Direct lookup by summary ID |
| `past_meeting_summary_id:{id}` | `past_meeting_summary_id:abc123-...` | Namespaced lookup by summary ID |
| `meeting_and_occurrence_id:{value}` | `meeting_and_occurrence_id:93699735000:1700000000` | Find summaries for a past meeting |
| `meeting_id:{value}` | `meeting_id:93699735000` | Find summaries for a meeting |
| `platform:Zoom` | `platform:Zoom` | All summaries (platform is always Zoom) |
| `title:{value}` | `title:TSC Monthly Meeting` | Find summaries by Zoom meeting topic |
| `committee_uid:{value}` | `committee_uid:abc123...` | Find summaries by committee |

### Access Control (IndexingConfig)

| Field | Value |
|---|---|
| `access_check_object` | `v1_past_meeting:{meeting_and_occurrence_id}` (access checked on the parent past meeting) |
| `access_check_relation` | `viewer` |
| `history_check_object` | `v1_past_meeting:{meeting_and_occurrence_id}` |
| `history_check_relation` | `auditor` |
| `public` | `true` when the parent past meeting's `ai_summary_access == "public"`, `false` otherwise |

### Search Behavior

| Field | Value |
|---|---|
| `sort_name` | `zoom_meeting_topic` (trimmed) |
| `name_and_aliases` | `[zoom_meeting_topic]` (omitted when empty) |
| `fulltext` | `sort_name` + `name_and_aliases` (space-joined, deduplicated) |

### Parent References

| Ref | Condition |
|---|---|
| `past_meeting:{meeting_and_occurrence_id}` | Always set |
| `project:{project_uid}` | Set when `project_uid` is non-empty |
| `committee:{uid}` | Set once per entry in `committees` with a non-empty `uid` |

---

## V1 Meeting Attachment

**Object type:** `v1_meeting_attachment`

**NATS subject:** `lfx.index.v1_meeting_attachment`

**Source struct:** `internal/domain/models/event_models.go` — `MeetingAttachmentEventData`

**Indexed on:** create, update, delete of an attachment on an active meeting.

### Data Schema

| Field | Type | Description |
|---|---|---|
| `uid` | string | Attachment unique identifier |
| `meeting_id` | string | ID of the parent meeting |
| `project_uid` | string (optional) | v2 UUID of the parent project (omitted when project not yet in v2) |
| `project_slug` | string (optional) | URL slug of the parent project (resolved via `lfx.projects-api.get_slug`; omitted when unavailable) |
| `type` | string | Attachment type (e.g., `"link"`, `"file"`) |
| `category` | string (optional) | Attachment category |
| `link` | string (optional) | Link URL (for link-type attachments) |
| `name` | string | Attachment display name |
| `description` | string (optional) | Attachment description |
| `source` | string (optional) | Attachment source |
| `file_name` | string (optional) | Uploaded file name |
| `file_size` | int (optional) | File size in bytes |
| `file_url` | string (optional) | URL to the uploaded file |
| `file_uploaded` | bool (optional) | Whether the file has been uploaded |
| `file_upload_status` | string (optional) | Upload status |
| `file_content_type` | string (optional) | MIME content type |
| `file_uploaded_by` | object (optional) | User who uploaded the file (see [User Reference schema](#user-reference-schema)) |
| `file_uploaded_at` | string (RFC3339) (optional) | Time the file was uploaded |
| `committees` | []object (optional) | Associated committees sourced from the parent meeting (see [Committee schema](#committee-schema)) |
| `created_at` | string (RFC3339) | Creation time |
| `modified_at` | string (RFC3339) | Last modification time |
| `created_by` | object | User who created the attachment (see [User Reference schema](#user-reference-schema)) |
| `updated_by` | object | User who last updated the attachment (see [User Reference schema](#user-reference-schema)) |

### Tags

| Tag Format | Example | Purpose |
|---|---|---|
| `meeting_attachment_uid:{uid}` | `meeting_attachment_uid:a1b2c3d4-...` | Lookup by attachment UID |
| `meeting_id:{value}` | `meeting_id:93699735000` | Find attachments for a meeting |
| `project_uid:{value}` | `project_uid:abc123...` | Find attachments by project |
| `project_slug:{value}` | `project_slug:cncf` | Find attachments by project slug |
| `type:{value}` | `type:file` | Find attachments by type |
| `committee_uid:{value}` | `committee_uid:abc123...` | Find attachments by committee |

### Access Control (IndexingConfig)

| Field | Value |
|---|---|
| `access_check_object` | `v1_meeting:{meeting_id}` (access checked on the parent meeting) |
| `access_check_relation` | `viewer` |
| `history_check_object` | `v1_meeting:{meeting_id}` |
| `history_check_relation` | `auditor` |
| `public` | `false` (always) |

### Search Behavior

| Field | Value |
|---|---|
| `sort_name` | `name` (trimmed) |
| `name_and_aliases` | `[file_name, link, name]` (deduplicated, omits empty values) |
| `fulltext` | `sort_name` + `name_and_aliases` + `description` (space-joined, deduplicated, omits empty values) |

### Parent References

| Ref | Condition |
|---|---|
| `meeting:{meeting_id}` | Always set |
| `project:{project_uid}` | Set when `project_uid` is non-empty |
| `committee:{uid}` | Set once per entry in `committees` with a non-empty `uid` |

---

## V1 Past Meeting Attachment

**Object type:** `v1_past_meeting_attachment`

**NATS subject:** `lfx.index.v1_past_meeting_attachment`

**Source struct:** `internal/domain/models/event_models.go` — `PastMeetingAttachmentEventData`

**Indexed on:** create, update, delete of an attachment on a past meeting.

### Data Schema

| Field | Type | Description |
|---|---|---|
| `uid` | string | Attachment unique identifier |
| `meeting_and_occurrence_id` | string | Combined meeting+occurrence ID of the parent past meeting |
| `meeting_id` | string | ID of the originating active meeting |
| `project_uid` | string (optional) | v2 UUID of the parent project (omitted when project not yet in v2) |
| `project_slug` | string (optional) | URL slug of the parent project (sourced from the past meeting KV record) |
| `type` | string | Attachment type (e.g., `"link"`, `"file"`) |
| `category` | string (optional) | Attachment category |
| `link` | string (optional) | Link URL (for link-type attachments) |
| `name` | string | Attachment display name |
| `description` | string (optional) | Attachment description |
| `source` | string (optional) | Attachment source |
| `file_name` | string (optional) | Uploaded file name |
| `file_size` | int (optional) | File size in bytes |
| `file_url` | string (optional) | URL to the uploaded file |
| `file_uploaded` | bool (optional) | Whether the file has been uploaded |
| `file_upload_status` | string (optional) | Upload status |
| `file_content_type` | string (optional) | MIME content type |
| `file_uploaded_by` | object (optional) | User who uploaded the file (see [User Reference schema](#user-reference-schema)) |
| `file_uploaded_at` | string (RFC3339) (optional) | Time the file was uploaded |
| `committees` | []object (optional) | Associated committees sourced from the parent past meeting (see [Committee schema](#committee-schema)) |
| `created_at` | string (RFC3339) | Creation time |
| `modified_at` | string (RFC3339) | Last modification time |
| `created_by` | object | User who created the attachment (see [User Reference schema](#user-reference-schema)) |
| `updated_by` | object | User who last updated the attachment (see [User Reference schema](#user-reference-schema)) |

### Tags

| Tag Format | Example | Purpose |
|---|---|---|
| `past_meeting_attachment_uid:{uid}` | `past_meeting_attachment_uid:a1b2c3d4-...` | Lookup by attachment UID |
| `meeting_and_occurrence_id:{value}` | `meeting_and_occurrence_id:93699735000:1700000000` | Find attachments for a past meeting |
| `meeting_id:{value}` | `meeting_id:93699735000` | Find attachments by originating meeting |
| `project_uid:{value}` | `project_uid:abc123...` | Find attachments by project |
| `project_slug:{value}` | `project_slug:my-project` | Find attachments by project slug |
| `type:{value}` | `type:link` | Find attachments by type |
| `committee_uid:{value}` | `committee_uid:abc123...` | Find attachments by committee |

### Access Control (IndexingConfig)

| Field | Value |
|---|---|
| `access_check_object` | `v1_past_meeting:{meeting_and_occurrence_id}` (access checked on the parent past meeting) |
| `access_check_relation` | `viewer` |
| `history_check_object` | `v1_past_meeting:{meeting_and_occurrence_id}` |
| `history_check_relation` | `auditor` |
| `public` | `false` (always) |

### Search Behavior

| Field | Value |
|---|---|
| `sort_name` | `name` (trimmed) |
| `name_and_aliases` | `[file_name, link, name]` (deduplicated, omits empty values) |
| `fulltext` | `sort_name` + `name_and_aliases` + `description` (space-joined, deduplicated, omits empty values) |

### Parent References

| Ref | Condition |
|---|---|
| `past_meeting:{meeting_and_occurrence_id}` | Always set |
| `project:{project_uid}` | Set when `project_uid` is non-empty |
| `committee:{uid}` | Set once per entry in `committees` with a non-empty `uid` |
