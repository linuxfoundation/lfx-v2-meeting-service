# ITX Past Meeting Attendees API Contracts

This document details the API contracts for past meeting attendees endpoints between the LFX v2 Meeting Service proxy API and the underlying ITX Zoom API.

## Endpoints

### Create Past Meeting Attendee

Adds a new attendee to a past meeting occurrence.

**Proxy Endpoint**: `POST /itx/past_meetings/{past_meeting_id}/attendees`

**ITX Endpoint**: `POST /v2/zoom/past_meetings/{past_meeting}/attendees`

**Path Parameters**:
- `past_meeting_id` (string, required): The hyphenated meeting and occurrence ID (e.g., "12343245463-1630560600000")

**Authorization**: Requires `organizer` permission on the meeting

---

### Update Past Meeting Attendee

Updates an existing attendee for a past meeting occurrence.

**Proxy Endpoint**: `PUT /itx/past_meetings/{past_meeting_id}/attendees/{attendee_id}`

**ITX Endpoint**: `PUT /v2/zoom/past_meetings/{past_meeting}/attendees/{attendee}`

**Path Parameters**:
- `past_meeting_id` (string, required): The hyphenated meeting and occurrence ID
- `attendee_id` (string, required): UUID of the attendee record

**Authorization**: Requires `organizer` permission on the meeting

---

### Delete Past Meeting Attendee

Deletes an attendee from a past meeting occurrence.

**Proxy Endpoint**: `DELETE /itx/past_meetings/{past_meeting_id}/attendees/{attendee_id}`

**ITX Endpoint**: `DELETE /v2/zoom/past_meetings/{past_meeting}/attendees/{attendee}`

**Path Parameters**:
- `past_meeting_id` (string, required): The hyphenated meeting and occurrence ID
- `attendee_id` (string, required): UUID of the attendee record

**Authorization**: Requires `organizer` permission on the meeting

---

## Request Schemas

### Create Attendee Request

**Proxy API & ITX API** (Identical):
```json
{
  "name": "John Doe",
  "email": "john.doe@example.com",
  "lf_user_id": "003P000001cRZVVI9A",
  "lf_sso": "jdoe",
  "org": "Google",
  "job_title": "Software Engineer",
  "profile_picture": "https://avatars.example.com/jdoe.jpg",
  "is_verified": true,
  "is_unknown": false,
  "committee_id": "ea1e8536-a985-4cf5-b981-a170927a1d11",
  "committee_role": "Developer Seat",
  "committee_voting_status": "Voting Rep",
  "sessions": [
    {
      "participant_uuid": "zoom-participant-uuid-123",
      "join_time": "2021-06-27T05:30:37Z",
      "leave_time": "2021-06-27T05:59:12Z",
      "leave_reason": "left the meeting. Reason : Host ended the meeting."
    }
  ]
}
```

**Required Fields**:
- At least one of: `email`, `lf_user_id`, or `lf_sso`

**Optional Fields**:
- `name` (string): Full name of the attendee
- `registrant_id` (string): UUID of the associated registrant record
- `org` (string): Organization name
- `job_title` (string): Job title
- `profile_picture` (string): URL to profile picture
- `is_verified` (boolean): Whether the attendee has been verified (default: false)
- `is_unknown` (boolean): Whether attendee is marked as unknown (default: false)
- `committee_id` (string): UUID of associated committee (if applicable)
- `committee_role` (string): Role within the committee
- `committee_voting_status` (string): Voting status in committee
- `org_is_member` (boolean): Whether org has LF membership
- `org_is_project_member` (boolean): Whether org has project membership
- `sessions` (array): Array of session objects with join/leave times

---

### Update Attendee Request

**Proxy API & ITX API** (Identical):
```json
{
  "org": "Microsoft",
  "job_title": "Senior Software Engineer",
  "is_verified": true,
  "committee_role": "Lead Developer",
  "committee_voting_status": "Alt Voting Rep"
}
```

All fields are optional - only include fields you want to update.

---

## Response Schemas

### Attendee Response

**Proxy API & ITX API** (Identical):
```json
{
  "id": "ea1e8536-a985-4cf5-b981-a170927a1d11",
  "registrant_id": "fb2f9647-b096-5dg6-c092-b281938b2e22",
  "name": "John Doe",
  "email": "john.doe@example.com",
  "lf_sso": "jdoe",
  "lf_user_id": "003P000001cRZVVI9A",
  "is_verified": true,
  "is_unknown": false,
  "org": "Google",
  "job_title": "Software Engineer",
  "profile_picture": "https://avatars.example.com/jdoe.jpg",
  "average_attendance": 85,
  "meeting_id": "12343245463",
  "occurrence_id": "1630560600000",
  "committee_id": "ea1e8536-a985-4cf5-b981-a170927a1d11",
  "committee_role": "Developer Seat",
  "is_committee_member": true,
  "committee_voting_status": "Voting Rep",
  "org_is_member": true,
  "org_is_project_member": true,
  "sessions": [
    {
      "participant_uuid": "zoom-participant-uuid-123",
      "join_time": "2021-06-27T05:30:37Z",
      "leave_time": "2021-06-27T05:59:12Z",
      "leave_reason": "left the meeting. Reason : Host ended the meeting."
    }
  ]
}
```

**Response Fields**:
- `id` (string): UUID of the attendee record (read-only)
- `registrant_id` (string): UUID of associated registrant (if any)
- `name` (string): Full name of the attendee
- `email` (string): Email address
- `lf_sso` (string): LF SSO username
- `lf_user_id` (string): LF user ID
- `is_verified` (boolean): Whether the attendee has been verified
- `is_unknown` (boolean): Whether attendee is marked as unknown
- `org` (string): Organization name
- `job_title` (string): Job title
- `profile_picture` (string): URL to profile picture
- `average_attendance` (integer): Average attendance percentage (read-only, calculated)
- `meeting_id` (string): Meeting ID (read-only)
- `occurrence_id` (string): Occurrence ID (read-only)
- `committee_id` (string): Associated committee UUID
- `committee_role` (string): Role within committee
- `is_committee_member` (boolean): Whether attendee is a committee member
- `committee_voting_status` (string): Voting status in committee
- `org_is_member` (boolean): Whether org has LF membership
- `org_is_project_member` (boolean): Whether org has project membership
- `sessions` (array): Array of session objects with join/leave times

---

## Field Mapping

**No field mapping required** - all fields are identical between the proxy and ITX APIs.

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
- `participant_uuid` (string): Zoom participant UUID (from Zoom webhook or manual)
- `join_time` (string, RFC3339): When the participant joined
- `leave_time` (string, RFC3339): When the participant left
- `leave_reason` (string): Reason for leaving (from Zoom)

---

## Important Notes

1. **Past Meeting Only**: Attendees can only be added to past meeting occurrences, not future meetings. Use the registrants endpoints for future meetings.

2. **Attendees vs Registrants**:
   - **Registrants**: Users registered/invited to a meeting (may or may not attend)
   - **Attendees**: Users who actually joined and attended the meeting
   - An attendee record can be linked to a registrant via `registrant_id`

3. **Verification Status**:
   - `is_verified=true`: Attendee has been verified as a legitimate participant
   - `is_unknown=true`: Attendee has been flagged as unknown (no matching LFID found)
   - These flags are used for manual review and data quality

4. **Average Attendance**: Calculated as (number of occurrences attended / total occurrences) expressed as a percentage. This is a read-only field computed by the system.

5. **Committee Fields**: Only relevant for meetings associated with a committee. These fields track committee membership and voting status.

6. **Organization Membership**:
   - `org_is_member`: Organization has general LF membership
   - `org_is_project_member`: Organization has membership specific to this project
   - If not provided in request, the API will automatically determine these values

7. **Multiple Sessions**: An attendee can have multiple session objects if they left and rejoined the meeting during the occurrence

8. **Zoom Webhook Data**: For attendees created from Zoom webhook events, session data comes directly from Zoom. For manually created attendees, session data can be provided or left empty.

9. **Update Semantics**: When updating, only provide fields you want to change. Omitted fields remain unchanged.

10. **Identity Resolution**: The API will attempt to match attendees to existing LF users based on email, `lf_user_id`, or `lf_sso`. Providing accurate identity information improves data quality.
