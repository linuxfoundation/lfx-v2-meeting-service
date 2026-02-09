# ITX Past Meeting Participants API Contracts

This document details the API contracts for past meeting participants endpoints. The V2 proxy API provides unified participant endpoints that abstract over the underlying ITX invitee and attendee endpoints.

## Overview

The LFX v2 Meeting Service provides a unified participant model that combines both invitee (registered/invited) and attendee (actually attended) concepts into a single participant entity. This simplifies client interactions by providing a single set of CRUD operations.

**Architecture Flow**:
```
Client → V2 Proxy API (Unified Participants) → ITX API (Separate Invitees & Attendees)
```

**V2 Proxy API**:
- Base Path: `/itx/past_meetings/{past_meeting_id}/participants`
- Authorization: Bearer token (JWT via Heimdall/OpenFGA)
- Unified participant model with `is_invited` and `is_attended` flags

**ITX API**:
- Base Paths:
  - `/v2/zoom/past-meetings/{past_meeting}/invitees`
  - `/v2/zoom/past-meetings/{past_meeting}/attendees`
- Authorization: OAuth2 M2M (added automatically by proxy)
- Separate invitee and attendee resources

---

## Create Past Meeting Participant

### Proxy API Endpoint

**Method**: `POST /itx/past_meetings/{past_meeting_id}/participants?v=1`

**Authorization**: Requires `organizer` permission on the meeting

**Request Headers**:
```
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

**Path Parameters**:
- `past_meeting_id` (string, required) - The past meeting ID

**Request Body**:

```json
{
  "is_invited": true,
  "is_attended": true,
  "first_name": "Jane",
  "last_name": "Doe",
  "email": "jane.doe@example.com",
  "username": "janedoe",
  "lf_user_id": "user-uuid",
  "org_name": "Example Corp",
  "job_title": "Software Engineer",
  "avatar_url": "https://example.com/avatar.jpg",
  "committee_id": "committee-uuid",
  "committee_role": "member",
  "committee_voting_status": "active",
  "org_is_member": true,
  "org_is_project_member": true,
  "is_verified": true,
  "is_unknown": false,
  "sessions": [
    {
      "participant_uuid": "session-uuid",
      "join_time": "2024-01-15T10:00:00Z",
      "leave_time": "2024-01-15T11:00:00Z",
      "leave_reason": "Left meeting"
    }
  ]
}
```

**Required Fields**:
- At least one of: `email`, `lf_user_id`, or `username`
- At least one of: `is_invited` or `is_attended` must be `true`

**Optional Fields**:
- `first_name` (string) - First name of the participant
- `last_name` (string) - Last name of the participant
- `org_name` (string) - Organization name
- `job_title` (string) - Job title
- `avatar_url` (string) - URL to profile picture
- `committee_id` (string) - UUID of associated committee
- `committee_role` (string) - Role within the committee
- `committee_voting_status` (string) - Voting status in committee
- `org_is_member` (boolean) - Whether org has LF membership
- `org_is_project_member` (boolean) - Whether org has project membership
- `is_verified` (boolean) - Whether attendee has been verified (attendee-only field)
- `is_unknown` (boolean) - Whether attendee is marked as unknown (attendee-only field)
- `sessions` (array) - Session objects with join/leave times (attendee-only field)

**Response**: `201 Created`

```json
{
  "id": "participant-id",
  "invitee_id": "inv_abc123",
  "attendee_id": "att_xyz789",
  "past_meeting_id": "past-meeting-uuid",
  "meeting_id": "meeting-id",
  "is_invited": true,
  "is_attended": true,
  "first_name": "Jane",
  "last_name": "Doe",
  "email": "jane.doe@example.com",
  "username": "janedoe",
  "lf_user_id": "user-uuid",
  "org_name": "Example Corp",
  "job_title": "Software Engineer",
  "avatar_url": "https://example.com/avatar.jpg",
  "committee_id": "committee-uuid",
  "committee_role": "member",
  "is_committee_member": true,
  "committee_voting_status": "active",
  "org_is_member": true,
  "org_is_project_member": true,
  "is_verified": true,
  "is_unknown": false,
  "average_attendance": 0.85,
  "sessions": [
    {
      "participant_uuid": "session-uuid",
      "join_time": "2024-01-15T10:00:00Z",
      "leave_time": "2024-01-15T11:00:00Z",
      "leave_reason": "Left meeting"
    }
  ],
  "created_at": "2024-01-15T09:00:00Z",
  "modified_at": "2024-01-15T09:00:00Z",
  "created_by": {
    "username": "admin",
    "name": "Admin User",
    "email": "admin@example.com",
    "profile_picture": "https://example.com/admin.jpg"
  },
  "modified_by": {
    "username": "admin",
    "name": "Admin User",
    "email": "admin@example.com",
    "profile_picture": "https://example.com/admin.jpg"
  }
}
```

### ITX API Endpoints

The proxy creates invitee and/or attendee records based on the flags:

**Method (Invitee)**: `POST /v2/zoom/past-meetings/{past_meeting_id}/invitees`

Creates an invitee record when `is_invited: true`

**Method (Attendee)**: `POST /v2/zoom/past-meetings/{past_meeting_id}/attendees`

Creates an attendee record when `is_attended: true`

---

## Update Past Meeting Participant

### Proxy API Endpoint

**Method**: `PUT /itx/past_meetings/{past_meeting_id}/participants/{participant_id}?v=1`

**Authorization**: Requires `organizer` permission on the meeting

**Request Headers**:

```
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

**Path Parameters**:

- `past_meeting_id` (string, required) - The past meeting ID
- `participant_id` (string, required) - The participant ID (V2 participant ID)

**Request Body**:

```json
{
  "invitee_id": "inv_abc123",
  "attendee_id": "att_xyz789",
  "is_invited": true,
  "is_attended": true,
  "first_name": "Jane",
  "last_name": "Smith",
  "email": "jane.smith@example.com",
  "username": "janesmith",
  "lf_user_id": "user-uuid",
  "org_name": "New Corp",
  "job_title": "Senior Engineer",
  "avatar_url": "https://example.com/new-avatar.jpg",
  "committee_role": "chair",
  "committee_voting_status": "active",
  "is_verified": true
}
```

**Optional Fields for Performance Optimization**:

- `invitee_id` (string, optional) - If provided, the service uses this ITX invitee ID directly instead of performing a NATS ID mapping lookup from the participant_id. This optimization reduces latency when the client already has the ITX ID.
- `attendee_id` (string, optional) - If provided, the service uses this ITX attendee ID directly instead of performing a NATS ID mapping lookup from the participant_id. This optimization reduces latency when the client already has the ITX ID.

**Behavior**:

1. **With optional IDs**: If `invitee_id` or `attendee_id` are provided, the service:
   - Uses the provided ID directly
   - Verifies the ID exists by mapping it back to a participant_id via NATS
   - Skips the participant_id → invitee_id/attendee_id lookup
   - Falls back to participant_id mapping if the provided ID doesn't exist

2. **Without optional IDs**: If `invitee_id` or `attendee_id` are not provided, the service:
   - Uses the `participant_id` from the path parameter
   - Performs NATS ID mapping to resolve invitee_id and/or attendee_id

3. **State transitions**:
   - Setting `is_invited: true` creates or updates the invitee record
   - Setting `is_invited: false` deletes the invitee record if it exists
   - Setting `is_attended: true` creates or updates the attendee record
   - Setting `is_attended: false` deletes the attendee record if it exists

**Response**: `200 OK`

Response body is identical to Create Past Meeting Participant response.

### ITX API Endpoints

Updates invitee and/or attendee records through the ITX API based on the provided fields.

**Method (Create Invitee)**: `POST /v2/zoom/past-meetings/{past_meeting_id}/invitees`

**Method (Update Invitee)**: `PUT /v2/zoom/past-meetings/{past_meeting_id}/invitees/{invitee_id}`

**Method (Delete Invitee)**: `DELETE /v2/zoom/past-meetings/{past_meeting_id}/invitees/{invitee_id}`

**Method (Create Attendee)**: `POST /v2/zoom/past-meetings/{past_meeting_id}/attendees`

**Method (Update Attendee)**: `PUT /v2/zoom/past-meetings/{past_meeting_id}/attendees/{attendee_id}`

**Method (Delete Attendee)**: `DELETE /v2/zoom/past-meetings/{past_meeting_id}/attendees/{attendee_id}`

---

## Delete Past Meeting Participant

### Proxy API Endpoint

**Method**: `DELETE /itx/past_meetings/{past_meeting_id}/participants/{participant_id}?v=1`

**Authorization**: Requires `organizer` permission on the meeting

**Request Headers**:

```
Authorization: Bearer <jwt_token>
```

**Path Parameters**:

- `past_meeting_id` (string, required) - The past meeting ID
- `participant_id` (string, required) - The participant ID

**Response**: `204 No Content`

### ITX API Endpoints

Deletes both invitee and attendee records if they exist.

**Method (Invitee)**: `DELETE /v2/zoom/past-meetings/{past_meeting_id}/invitees/{invitee_id}`

**Method (Attendee)**: `DELETE /v2/zoom/past-meetings/{past_meeting_id}/attendees/{attendee_id}`

---

## Field Mapping

### Invitee Fields

| V2 Proxy Field | ITX Invitee Field | Notes |
|----------------|-------------------|-------|
| `first_name` | `first_name` | Identical |
| `last_name` | `last_name` | Identical |
| `email` | `primary_email` | V2 uses `email`, ITX uses `primary_email` |
| `username` | `lf_sso` | V2 uses `username`, ITX uses `lf_sso` |
| `lf_user_id` | `lf_user_id` | Identical |
| `org_name` | `org` | V2 uses `org_name`, ITX uses `org` |
| `job_title` | `job_title` | Identical |
| `avatar_url` | `profile_picture` | V2 uses `avatar_url`, ITX uses `profile_picture` |
| `committee_id` | `committee_id` | Identical |
| `committee_role` | `committee_role` | Identical |
| `committee_voting_status` | `committee_voting_status` | Identical |
| `org_is_member` | `org_is_member` | Identical |
| `org_is_project_member` | `org_is_project_member` | Identical |

### Attendee Fields

| V2 Proxy Field | ITX Attendee Field | Notes |
|----------------|-------------------|-------|
| `first_name` + `last_name` | `name` | V2 splits into first/last, ITX uses full name |
| `email` | `email` | Identical |
| `username` | `lf_sso` | V2 uses `username`, ITX uses `lf_sso` |
| `lf_user_id` | `lf_user_id` | Identical |
| `org_name` | `org` | V2 uses `org_name`, ITX uses `org` |
| `job_title` | `job_title` | Identical |
| `avatar_url` | `profile_picture` | V2 uses `avatar_url`, ITX uses `profile_picture` |
| `committee_id` | `committee_id` | Identical |
| `committee_role` | `committee_role` | Identical |
| `committee_voting_status` | `committee_voting_status` | Identical |
| `org_is_member` | `org_is_member` | Identical |
| `org_is_project_member` | `org_is_project_member` | Identical |
| `is_verified` | `is_verified` | Identical |
| `is_unknown` | `is_unknown` | Identical |
| `sessions` | `sessions` | Identical |
| `average_attendance` | `average_attendance` | Identical (read-only) |

---

## Session Objects

Sessions represent individual join/leave cycles within a meeting occurrence. An attendee can have multiple sessions if they leave and rejoin.

**Session Object Structure**:

```json
{
  "participant_uuid": "zoom-participant-uuid-123",
  "join_time": "2021-06-27T05:30:37Z",
  "leave_time": "2021-06-27T05:59:12Z",
  "leave_reason": "left the meeting. Reason : Host ended the meeting."
}
```

**Fields**:
- `participant_uuid` (string) - Zoom participant UUID
- `join_time` (string, RFC3339) - When the participant joined
- `leave_time` (string, RFC3339) - When the participant left
- `leave_reason` (string) - Reason for leaving

---

## Important Notes

### Participant Model

1. **Unified Abstraction**: The V2 API provides a unified participant model that abstracts over ITX's separate invitee and attendee resources. Clients work with a single participant entity using `is_invited` and `is_attended` flags.

2. **Flag Semantics**:
   - `is_invited: true` means the participant has an invitee record (was registered/invited)
   - `is_attended: true` means the participant has an attendee record (actually attended)
   - A participant can have both flags set (invited and attended)
   - A participant can have only one flag set (invited but didn't attend, or attended without prior invitation)

3. **ID Mapping**: The V2 proxy maintains ID mappings between:
   - V2 participant_id ↔ ITX invitee_id
   - V2 participant_id ↔ ITX attendee_id
   - These mappings are stored in NATS and enable the unified participant model

### Performance Optimization

4. **Optional ITX IDs**: The `invitee_id` and `attendee_id` fields in update requests are optional performance optimizations. When provided:
   - The service skips NATS ID mapping lookups
   - Reduces latency for clients that already have ITX IDs
   - Still verifies existence by reverse mapping the ITX ID back to participant_id
   - Falls back to standard participant_id mapping if ITX ID doesn't exist

### Data Quality

5. **Identity Resolution**: The API attempts to match participants to existing LF users based on `email`, `lf_user_id`, or `username`. Providing accurate identity information improves data quality and enables proper cross-referencing.

6. **Verification Status**:
   - `is_verified: true` - Attendee has been verified as a legitimate participant
   - `is_unknown: true` - Attendee has been flagged as unknown (no matching LFID found)
   - These flags are used for manual review and data quality (attendee-only)

### Committee Fields

7. **Committee Association**: Only relevant for meetings associated with a committee:
   - `committee_id` - Links the participant to a specific committee
   - `committee_role` - Their role within that committee
   - `committee_voting_status` - Their voting status (e.g., "active", "pending")
   - `is_committee_member` - Boolean flag indicating committee membership (response only)

### Organization Membership

8. **Organization Fields**:
   - `org_is_member` - Organization has general LF membership
   - `org_is_project_member` - Organization has membership specific to this project
   - If not provided in request, the API will automatically determine these values based on organization data

### Sessions and Attendance

9. **Multiple Sessions**: An attendee can have multiple session objects if they left and rejoined the meeting during the occurrence.

10. **Average Attendance**: Calculated as (number of occurrences attended / total occurrences) expressed as a percentage. This is a read-only field computed by the system (attendee-only).

### Update Semantics

11. **Partial Updates**: When updating, only provide fields you want to change. Omitted fields remain unchanged.

12. **State Transitions**: Changing `is_invited` or `is_attended` flags triggers create, update, or delete operations on the corresponding ITX resources:
    - `true → false` deletes the ITX record
    - `false → true` creates the ITX record
    - `true → true` updates the ITX record

### Audit Trail

13. **Audit Fields**: All participant records track who created and last updated them, along with timestamps for full audit capability (response only).

---

## Error Responses

**HTTP Status Codes**:

- `400 Bad Request` - Invalid request parameters
- `401 Unauthorized` - Missing or invalid authentication
- `403 Forbidden` - Insufficient permissions
- `404 Not Found` - Resource not found
- `409 Conflict` - Resource conflict
- `500 Internal Server Error` - Server error
- `503 Service Unavailable` - Service temporarily unavailable

**Error Response Body**:

```json
{
  "error": "Error message describing what went wrong",
  "message": "Detailed error message"
}
```
