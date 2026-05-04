# FGA Contract — Meeting Service

This document is the authoritative reference for all messages the meeting service sends to the fga-sync service, which writes and deletes [OpenFGA](https://openfga.dev/) relationship tuples to enforce access control.

The full OpenFGA type definitions (relations, schema) for all object types are defined in the [platform model](https://github.com/linuxfoundation/lfx-v2-helm/blob/main/charts/lfx-platform/templates/openfga/model.yaml).

**Update this document in the same PR as any change to FGA message construction.**

> **Note:** `v1_meeting_rsvp`, `v1_meeting_attachment`, `v1_past_meeting_attachment`, `v1_past_meeting_recording`, `v1_past_meeting_transcript`, and `v1_past_meeting_summary` do not send FGA messages — they are indexed only. Access for recordings, transcripts, and summaries is checked via the parent `v1_past_meeting` object.

---

## Object Types

- [V1 Meeting](#v1-meeting)
- [V1 Past Meeting](#v1-past-meeting)

---

## Message Format

This service uses four FGA operation types:

| Subject | Operation | Used for |
|---|---|---|
| `lfx.fga-sync.update_access` | `update_access` | Create and update — sets object-level access config and references |
| `lfx.fga-sync.member_put` | `member_put` | Adds a user to one or more relations on an object |
| `lfx.fga-sync.member_remove` | `member_remove` | Removes a user from an object; sent on registrant delete and on full participant deletes. An empty `relations` array removes all relations for that user on the object |
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

_(none set by this service — `organizer` is a derived relation in the FGA model and cannot be directly assigned)_

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

### member_remove (Registrant Delete)

Published to `lfx.fga-sync.member_remove` when a registrant delete event is processed and the registrant has a non-empty `Username`. The username is resolved to an Auth0 `sub` value before sending.

| Field | Value |
|---|---|
| `object_type` | `v1_meeting` |
| `uid` | `MeetingID` (parent meeting) |
| `username` | Auth0 `sub` of the registrant |
| `relations` | `[]` (empty — removes all relations for the user) |

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

### member_remove (Participant Full Delete)

Published to `lfx.fga-sync.member_remove` when a participant is fully deleted — i.e., no sibling invitee or attendee record remains for the same user on the same past meeting. Skipped if `Username` is empty.

When a participant record is deleted but a sibling record still exists (e.g., an attendee record remains when an invitee record is deleted), a `member_put` is sent instead to update the user's remaining relations rather than remove them entirely.

| Field | Value |
|---|---|
| `object_type` | `v1_past_meeting` |
| `uid` | `MeetingAndOccurrenceID` (parent past meeting) |
| `username` | Auth0 `sub` of the participant |
| `relations` | `[]` (empty — removes all relations for the user) |

### Delete

On delete, a `delete_access` message is sent to `lfx.fga-sync.delete_access` with only the past meeting `uid` — all FGA tuples for `v1_past_meeting:{uid}` are removed by the fga-sync service.

---

## Triggers

| Operation | Object Type | Subject | Notes |
|---|---|---|---|
| Create/update meeting | `v1_meeting` | `lfx.fga-sync.update_access` | Always sent |
| Delete meeting | `v1_meeting` | `lfx.fga-sync.delete_access` | Always sent |
| Create/update registrant (with username) | `v1_meeting` | `lfx.fga-sync.member_put` | Skipped if `Username` is empty |
| Delete registrant (with username) | `v1_meeting` | `lfx.fga-sync.member_remove` | Skipped if `Username` is empty |
| Create/update meeting RSVP | _(none)_ | _(none)_ | Indexer only — no FGA message sent |
| Create/update past meeting | `v1_past_meeting` | `lfx.fga-sync.update_access` | Always sent |
| Delete past meeting | `v1_past_meeting` | `lfx.fga-sync.delete_access` | Always sent |
| Create/update participant (with username) | `v1_past_meeting` | `lfx.fga-sync.member_put` | Skipped if `Username` is empty |
| Delete participant (full delete, with username) | `v1_past_meeting` | `lfx.fga-sync.member_remove` | Sent when no invitee/attendee sibling record remains; skipped if `Username` is empty |
| Delete participant (partial delete, with username) | `v1_past_meeting` | `lfx.fga-sync.member_put` | Sent when a sibling invitee/attendee record still exists — updates remaining relations instead of removing |
| Create/update recording | _(none)_ | _(none)_ | Indexer only — access checked via parent `v1_past_meeting` |
| Delete recording | _(none)_ | _(none)_ | Indexer only — access checked via parent `v1_past_meeting` |
| Create/update transcript | _(none)_ | _(none)_ | Indexer only — access checked via parent `v1_past_meeting` |
| Delete transcript | _(none)_ | _(none)_ | Indexer only — access checked via parent `v1_past_meeting` |
| Create/update summary | _(none)_ | _(none)_ | Indexer only — access checked via parent `v1_past_meeting` |
| Delete summary | _(none)_ | _(none)_ | Indexer only — access checked via parent `v1_past_meeting` |
| Create/update meeting attachment | _(none)_ | _(none)_ | Indexer only — no FGA message sent |
| Create/update past meeting attachment | _(none)_ | _(none)_ | Indexer only — no FGA message sent |
