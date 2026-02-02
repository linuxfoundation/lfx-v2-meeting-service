# ITX Past Meeting Invitees API Contracts

This document details the API contracts for past meeting invitees endpoints between the LFX v2 Meeting Service proxy API and the underlying ITX Zoom API.

## Endpoints

### Create Past Meeting Invitee

Creates a new invitee record for a specific past meeting occurrence.

**Proxy Endpoint**: `POST /itx/past_meetings/{past_meeting_id}/invitees`

**ITX Endpoint**: `POST /v2/zoom/past_meetings/{past_meeting}/invitees`

**Path Parameters**:
- `past_meeting_id` (string, required): The hyphenated meeting and occurrence ID (e.g., "12343245463-1630560600000")

**Authorization**: Requires `organizer` permission on the meeting

---

### Update Past Meeting Invitee

Updates an existing invitee for a specific past meeting occurrence. If the invitee is associated with a current meeting registrant and their organization/job title is updated, the existing meeting registrant record will also be updated.

**Proxy Endpoint**: `PUT /itx/past_meetings/{past_meeting_id}/invitees/{invitee_id}`

**ITX Endpoint**: `PUT /v2/zoom/past_meetings/{past_meeting}/invitees/{invitee}`

**Path Parameters**:
- `past_meeting_id` (string, required): The hyphenated meeting and occurrence ID
- `invitee_id` (string, required): UUID of the invitee record

**Authorization**: Requires `organizer` permission on the meeting

---

### Delete Past Meeting Invitee

Deletes an invitee from a specific past meeting occurrence. This removes the invitee from only this occurrence, not from all past meeting occurrences.

**Proxy Endpoint**: `DELETE /itx/past_meetings/{past_meeting_id}/invitees/{invitee_id}`

**ITX Endpoint**: `DELETE /v2/zoom/past_meetings/{past_meeting}/invitees/{invitee}`

**Path Parameters**:
- `past_meeting_id` (string, required): The hyphenated meeting and occurrence ID
- `invitee_id` (string, required): UUID of the invitee record

**Authorization**: Requires `organizer` permission on the meeting

---

## Request Schemas

### Create Invitee Request

**Proxy API & ITX API** (Identical):
```json
{
  "first_name": "John",
  "last_name": "Doe",
  "primary_email": "john.doe@example.com",
  "lf_user_id": "003P000001cRZVVI9A",
  "lf_sso": "jdoe",
  "org": "Google",
  "job_title": "Software Engineer",
  "profile_picture": "https://avatars.example.com/jdoe.jpg",
  "committee_id": "ea1e8536-a985-4cf5-b981-a170927a1d11",
  "committee_role": "Developer Seat",
  "committee_voting_status": "Voting Rep"
}
```

**Required Fields**:
- At least one of: `primary_email`, `lf_user_id`, or `lf_sso`

**Optional Fields**:
- `first_name` (string): First name of the invitee
- `last_name` (string): Last name of the invitee
- `org` (string): Organization name
- `job_title` (string): Job title
- `profile_picture` (string): URL to profile picture
- `committee_id` (string): UUID of associated committee (if applicable)
- `committee_role` (string): Role within the committee
- `committee_voting_status` (string): Voting status in committee
- `org_is_member` (boolean): Whether org has LF membership
- `org_is_project_member` (boolean): Whether org has project membership

---

### Update Invitee Request

**Proxy API & ITX API** (Identical):
```json
{
  "org": "Microsoft",
  "job_title": "Senior Software Engineer",
  "committee_role": "Lead Developer",
  "committee_voting_status": "Alt Voting Rep"
}
```

All fields are optional - only include fields you want to update.

**Note**: If the invitee is linked to a current meeting registrant and you update `org` or `job_title`, those fields will also be updated on the registrant record.

---

## Response Schemas

### Invitee Response

**Proxy API & ITX API** (Identical):
```json
{
  "uuid": "ea1e8536-a985-4cf5-b981-a170927a1d11",
  "first_name": "John",
  "last_name": "Doe",
  "primary_email": "john.doe@example.com",
  "lf_sso": "jdoe",
  "lf_user_id": "003P000001cRZVVI9A",
  "org": "Google",
  "job_title": "Software Engineer",
  "profile_picture": "https://avatars.example.com/jdoe.jpg",
  "committee_id": "ea1e8536-a985-4cf5-b981-a170927a1d11",
  "committee_role": "Developer Seat",
  "is_committee_member": true,
  "committee_voting_status": "Voting Rep",
  "org_is_member": true,
  "org_is_project_member": true,
  "created_at": "2021-06-27T05:30:00Z",
  "created_by": {
    "id": "user-uuid-123",
    "username": "jsmith",
    "name": "Jane Smith",
    "email": "jane.smith@example.com"
  },
  "modified_at": "2021-06-27T06:15:00Z",
  "updated_by": {
    "id": "user-uuid-456",
    "username": "bjones",
    "name": "Bob Jones",
    "email": "bob.jones@example.com"
  }
}
```

**Response Fields**:
- `uuid` (string): UUID of the invitee record (read-only)
- `first_name` (string): First name
- `last_name` (string): Last name
- `primary_email` (string): Primary email address
- `lf_sso` (string): LF SSO username
- `lf_user_id` (string): LF user ID
- `org` (string): Organization name
- `job_title` (string): Job title
- `profile_picture` (string): URL to profile picture
- `committee_id` (string): Associated committee UUID
- `committee_role` (string): Role within committee
- `is_committee_member` (boolean): Whether invitee is a committee member
- `committee_voting_status` (string): Voting status in committee
- `org_is_member` (boolean): Whether org has LF membership
- `org_is_project_member` (boolean): Whether org has project membership
- `created_at` (string, RFC3339): Creation timestamp (read-only)
- `created_by` (object): User who created the invitee (read-only)
- `modified_at` (string, RFC3339): Last modification timestamp (read-only)
- `updated_by` (object): User who last updated the invitee (read-only)

---

## Field Mapping

**No field mapping required** - all fields are identical between the proxy and ITX APIs.

---

## Important Notes

1. **Invitees vs Attendees vs Registrants**:
   - **Registrants**: Users registered for upcoming/future meetings (see registrants endpoints)
   - **Invitees**: Users who were invited/registered to a past meeting occurrence
   - **Attendees**: Users who actually joined and attended the past meeting occurrence
   - An invitee may or may not have attended (i.e., they may or may not have a corresponding attendee record)

2. **Past Meeting Scope**: Invitee records are specific to a single past meeting occurrence. Creating an invitee for one occurrence does not create it for other occurrences of the same recurring meeting.

3. **Occurrence-Specific Operations**: The hyphenated `past_meeting_id` format ensures invitees are always associated with a specific occurrence.

4. **Registrant Synchronization**: When updating an invitee's `org` or `job_title`, if that invitee is linked to an active meeting registrant, the registrant record will also be updated with the new values. This ensures data consistency across past and current meetings.

5. **Committee Fields**: Only relevant for meetings associated with a committee:
   - `committee_id`: Links the invitee to a specific committee
   - `committee_role`: Their role within that committee
   - `committee_voting_status`: Their voting status (e.g., "Voting Rep", "Alt Voting Rep", "Observer")
   - `is_committee_member`: Boolean flag indicating committee membership

6. **Organization Membership**:
   - `org_is_member`: Organization has general LF membership
   - `org_is_project_member`: Organization has membership specific to this project
   - If not provided in request, the API will automatically determine these values based on organization data

7. **Identity Resolution**: The API will attempt to match invitees to existing LF users based on `primary_email`, `lf_user_id`, or `lf_sso`. Providing accurate identity information improves data quality and enables proper cross-referencing.

8. **Audit Trail**: All invitee records track who created and last updated them, along with timestamps for full audit capability.

9. **Update Semantics**: When updating, only provide fields you want to change. Omitted fields remain unchanged.

10. **Delete Behavior**: Deleting an invitee removes them from this specific past meeting occurrence only. It does not affect:
    - Other past meeting occurrences
    - Current meeting registrants
    - Attendee records (if they attended)

11. **Profile Pictures**: The `profile_picture` field stores a URL to an external image resource, typically from the LF profile system.

12. **Bulk Operations**: For bulk inserting or deleting invitees across multiple past meeting occurrences, see the bulk attendee endpoints (`/zoom/past_meetings/attendee/bulk_insert` and `/zoom/past_meetings/attendee/bulk_delete`).
