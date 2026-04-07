# FGA Contract — Meeting Service

This document is the authoritative reference for all messages the meeting service sends to the fga-sync service, which writes and deletes [OpenFGA](https://openfga.dev/) relationship tuples to enforce access control.

The full OpenFGA type definitions (relations, schema) for all object types are defined in the [platform model](https://github.com/linuxfoundation/lfx-v2-helm/blob/main/charts/lfx-platform/templates/openfga/model.yaml).

**Update this document in the same PR as any change to FGA message construction.**

> **Note:** `v1_meeting_rsvp`, `v1_meeting_attachment`, and `v1_past_meeting_attachment` do not send FGA messages — they are indexed only.

---

## Object Types

- [V1 Meeting](#v1-meeting)
- [V1 Past Meeting](#v1-past-meeting)
- [V1 Past Meeting Recording](#v1-past-meeting-recording)
- [V1 Past Meeting Transcript](#v1-past-meeting-transcript)
- [V1 Past Meeting Summary](#v1-past-meeting-summary)

---

## Message Format

This service uses three FGA operation types:

| Subject | Operation | Used for |
|---|---|---|
| `lfx.fga-sync.update_access` | `update_access` | Create and update — sets object-level access config and references |
| `lfx.fga-sync.member_put` | `member_put` | Adds a user to one or more relations on an object |
| `lfx.fga-sync.delete_access` | `delete_access` | Delete — removes all FGA tuples for the object |

---

## V1 Meeting

**Source struct:** `internal/domain/models/` — `MeetingEventData`

**Synced on:** create, update, delete of a meeting.

### update_access

Published to `lfx.fga-sync.update_access` on meeting create or update.

#### Access Config

| Field | Value |
|---|---|
| `object_type` | `v1_meeting` |
| `public` | `true` when `Visibility == "public"`, otherwise `false` |

#### Relations

| Relation | Value | Condition |
|---|---|---|
| `organizer` | `Organizers` ([]string of Auth0 `sub` values) | Only when `Organizers` is non-empty |

> ⚠️ **Known mismatch:** The `v1_meeting` FGA type defines `organizer` as a derived relation with no direct `[user]` assignment. Setting this relation directly has no effect in the current model. See the [platform model](https://github.com/linuxfoundation/lfx-v2-helm/blob/main/charts/lfx-platform/templates/openfga/model.yaml) for details.

#### References

| Reference | Value | Condition |
|---|---|---|
| `project` | `ProjectUID` | Only when `ProjectUID` is non-empty |
| `committee` | `CommitteeUID` per committee | One entry per committee with a non-empty `UID` |

#### Exclude Relations

`exclude_relations: ["participant", "host"]` — always set. These relations are managed separately via `member_put` (see registrant events below) and should not be overwritten by the `update_access` handler.

### member_put (Registrant)

Published to `lfx.fga-sync.member_put` when a registrant event is processed and the registrant has a non-empty `Username`. The username is resolved to an Auth0 `sub` value before sending.

The object UID is the **parent meeting ID**, not the registrant UID.

#### Member Data

| Field | Value | Condition |
|---|---|---|
| `object_type` | `v1_meeting` | Always |
| `uid` | `MeetingID` (parent meeting) | Always |
| `username` | Auth0 `sub` of the registrant | Always (skipped if Username is empty) |
| `relations` | `["host"]` | When `registrant.Host == true` |
| `relations` | `["participant"]` | When `registrant.Host == false` |
| `mutually_exclusive_with` | `["participant"]` | When `registrant.Host == true` |
| `mutually_exclusive_with` | `["host"]` | When `registrant.Host == false` |

### Delete

On delete, a `delete_access` message is sent to `lfx.fga-sync.delete_access` with only the meeting `uid` — all FGA tuples for `v1_meeting:{uid}` are removed by the fga-sync service.

---

## V1 Past Meeting

**Source struct:** `internal/domain/models/` — `PastMeetingEventData`

**Synced on:** create, update, delete of a past meeting.

### update_access

Published to `lfx.fga-sync.update_access` on past meeting create or update.

#### Access Config

| Field | Value |
|---|---|
| `object_type` | `v1_past_meeting` |
| `public` | `false` (always) |

#### Relations

_(none set by this service)_

#### References

| Reference | Value | Condition |
|---|---|---|
| `meeting` | `"v1_meeting:{MeetingID}"` | Only when `MeetingID` is non-empty |
| `project` | `ProjectUID` | Only when `ProjectUID` is non-empty |
| `committee` | `CommitteeUID` per committee | One entry per committee with a non-empty `UID` |

> Note: the `meeting` reference value includes the type prefix (`v1_meeting:`) because the FGA model defines `meeting: [v1_meeting]`.

### member_put (Participant)

Published to `lfx.fga-sync.member_put` when a participant event is processed and the participant has a non-empty `Username`. The username is resolved to an Auth0 `sub` value before sending.

The object UID is the **parent past meeting's `MeetingAndOccurrenceID`**, not the participant UID.

#### Member Data

| Field | Value | Condition |
|---|---|---|
| `object_type` | `v1_past_meeting` | Always |
| `uid` | `MeetingAndOccurrenceID` (parent past meeting) | Always |
| `username` | Auth0 `sub` of the participant | Always (skipped if Username is empty) |
| `relations` | Subset of `["host", "invitee", "attendee"]` | `"host"` added when `participant.Host == true`; `"invitee"` when `IsInvited == true`; `"attendee"` when `IsAttended == true` |
| `mutually_exclusive_with` | `["host", "invitee", "attendee"]` | Always (all three, regardless of which relations are set) |

### Delete

On delete, a `delete_access` message is sent to `lfx.fga-sync.delete_access` with only the past meeting `uid` — all FGA tuples for `v1_past_meeting:{uid}` are removed by the fga-sync service.

---

## V1 Past Meeting Recording

**Source struct:** `internal/domain/models/` — `RecordingEventData`

**Synced on:** create, update, delete of a past meeting recording.

### update_access

Published to `lfx.fga-sync.update_access` on recording create or update.

#### Access Config

| Field | Value |
|---|---|
| `object_type` | `v1_past_meeting_recording` |
| `public` | `true` when `RecordingAccess == "public"`, otherwise `false` |

#### Relations

_(none set by this service)_

#### References

The `past_meeting` reference is always set. Additional references depend on `RecordingAccess`:

| Reference | Value | Condition |
|---|---|---|
| `past_meeting` | `"v1_past_meeting:{MeetingAndOccurrenceID}"` | Always |
| `past_meeting_for_host_view` | `"v1_past_meeting:{MeetingAndOccurrenceID}"` | When `RecordingAccess == "meeting_hosts"` (default) or `"meeting_participants"` |
| `past_meeting_for_attendee_view` | `"v1_past_meeting:{MeetingAndOccurrenceID}"` | Only when `RecordingAccess == "meeting_participants"` |
| `past_meeting_for_participant_view` | `"v1_past_meeting:{MeetingAndOccurrenceID}"` | Only when `RecordingAccess == "meeting_participants"` |

> When `RecordingAccess == "public"`, `public=true` grants `viewer` access to all users via `[user:*]`. Only the base `past_meeting` reference is set.

### Delete

On delete, a `delete_access` message is sent to `lfx.fga-sync.delete_access` with only the recording `uid`.

---

## V1 Past Meeting Transcript

**Source struct:** `internal/domain/models/` — `TranscriptEventData`

**Synced on:** create, update, delete of a past meeting transcript.

### update_access

Published to `lfx.fga-sync.update_access` on transcript create or update. Identical access pattern to [V1 Past Meeting Recording](#v1-past-meeting-recording), driven by `TranscriptAccess` instead of `RecordingAccess`.

#### Access Config

| Field | Value |
|---|---|
| `object_type` | `v1_past_meeting_transcript` |
| `public` | `true` when `TranscriptAccess == "public"`, otherwise `false` |

#### Relations

_(none set by this service)_

#### References

| Reference | Value | Condition |
|---|---|---|
| `past_meeting` | `"v1_past_meeting:{MeetingAndOccurrenceID}"` | Always |
| `past_meeting_for_host_view` | `"v1_past_meeting:{MeetingAndOccurrenceID}"` | When `TranscriptAccess == "meeting_hosts"` (default) or `"meeting_participants"` |
| `past_meeting_for_attendee_view` | `"v1_past_meeting:{MeetingAndOccurrenceID}"` | Only when `TranscriptAccess == "meeting_participants"` |
| `past_meeting_for_participant_view` | `"v1_past_meeting:{MeetingAndOccurrenceID}"` | Only when `TranscriptAccess == "meeting_participants"` |

### Delete

On delete, a `delete_access` message is sent to `lfx.fga-sync.delete_access` with only the transcript `uid`.

---

## V1 Past Meeting Summary

**Source struct:** `internal/domain/models/` — `SummaryEventData`

**Synced on:** create, update, delete of a past meeting summary.

### update_access

Published to `lfx.fga-sync.update_access` on summary create or update. Identical access pattern to [V1 Past Meeting Recording](#v1-past-meeting-recording), driven by the parent past meeting's `AISummaryAccess` field (passed in at publish time, not stored on the summary itself).

#### Access Config

| Field | Value |
|---|---|
| `object_type` | `v1_past_meeting_summary` |
| `public` | `true` when parent past meeting's `AISummaryAccess == "public"`, otherwise `false` |

#### Relations

_(none set by this service)_

#### References

| Reference | Value | Condition |
|---|---|---|
| `past_meeting` | `"v1_past_meeting:{MeetingAndOccurrenceID}"` | Always |
| `past_meeting_for_host_view` | `"v1_past_meeting:{MeetingAndOccurrenceID}"` | When `AISummaryAccess == "meeting_hosts"` (default) or `"meeting_participants"` |
| `past_meeting_for_attendee_view` | `"v1_past_meeting:{MeetingAndOccurrenceID}"` | Only when `AISummaryAccess == "meeting_participants"` |
| `past_meeting_for_participant_view` | `"v1_past_meeting:{MeetingAndOccurrenceID}"` | Only when `AISummaryAccess == "meeting_participants"` |

### Delete

On delete, a `delete_access` message is sent to `lfx.fga-sync.delete_access` with only the summary `uid`.

---

## Triggers

| Operation | Object Type | Subject | Notes |
|---|---|---|---|
| Create/update meeting | `v1_meeting` | `lfx.fga-sync.update_access` | Always sent |
| Delete meeting | `v1_meeting` | `lfx.fga-sync.delete_access` | Always sent |
| Create/update registrant (with username) | `v1_meeting` | `lfx.fga-sync.member_put` | Skipped if `Username` is empty |
| Create/update meeting RSVP | _(none)_ | _(none)_ | Indexer only — no FGA message sent |
| Create/update past meeting | `v1_past_meeting` | `lfx.fga-sync.update_access` | Always sent |
| Delete past meeting | `v1_past_meeting` | `lfx.fga-sync.delete_access` | Always sent |
| Create/update participant (with username) | `v1_past_meeting` | `lfx.fga-sync.member_put` | Skipped if `Username` is empty |
| Create/update recording | `v1_past_meeting_recording` | `lfx.fga-sync.update_access` | Always sent |
| Delete recording | `v1_past_meeting_recording` | `lfx.fga-sync.delete_access` | Always sent |
| Create/update transcript | `v1_past_meeting_transcript` | `lfx.fga-sync.update_access` | Always sent |
| Delete transcript | `v1_past_meeting_transcript` | `lfx.fga-sync.delete_access` | Always sent |
| Create/update summary | `v1_past_meeting_summary` | `lfx.fga-sync.update_access` | Always sent |
| Delete summary | `v1_past_meeting_summary` | `lfx.fga-sync.delete_access` | Always sent |
| Create/update meeting attachment | _(none)_ | _(none)_ | Indexer only — no FGA message sent |
| Create/update past meeting attachment | _(none)_ | _(none)_ | Indexer only — no FGA message sent |
