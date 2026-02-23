# ITX Past Meetings API Contracts

This document details the API contracts for past meeting endpoints between the LFX v2 Meeting Service proxy API and the underlying ITX Zoom API.

## Endpoints

### Get Past Meeting

Retrieves a single past meeting by its ID.

**Proxy Endpoint**: `GET /itx/past_meetings/{past_meeting_id}`

**ITX Endpoint**: `GET /v2/zoom/past_meetings/{past_meeting}`

**Path Parameters**:

- `past_meeting_id` (string, required): The hyphenated meeting and occurrence ID (e.g., "12343245463-1630560600000")

**Query Parameters**:

- `basic` (boolean, optional): If true, returns basic info without invitees/attendees lists

**Authorization**: Requires `viewer` permission on the meeting's project

---

### Create Past Meeting

Manually adds a past meeting record.

**Proxy Endpoint**: `POST /itx/past_meetings`

**ITX Endpoint**: `POST /v2/zoom/past_meetings`

**Authorization**: Requires `organizer` permission on the specified project

---

### Update Past Meeting

Updates a past meeting record, including the invitees lists.

**Proxy Endpoint**: `PUT /itx/past_meetings/{past_meeting_id}`

**ITX Endpoint**: `PUT /v2/zoom/past_meetings/{past_meeting}`

**Path Parameters**:

- `past_meeting_id` (string, required): The hyphenated meeting and occurrence ID

**Authorization**: Requires `organizer` permission on the meeting

---

### Delete Past Meeting

Deletes a past meeting record.

**Proxy Endpoint**: `DELETE /itx/past_meetings/{past_meeting_id}`

**ITX Endpoint**: `DELETE /v2/zoom/past_meetings/{past_meeting}`

**Path Parameters**:

- `past_meeting_id` (string, required): The hyphenated meeting and occurrence ID

**Authorization**: Requires `organizer` permission on the meeting

---

## Request Schemas

### Create/Update Past Meeting Request

**Proxy API**:

```json
{
  "project_uid": "7cad5a8d-19d0-41a4-81a6-043453daf9ee",
  "meeting_id": "12343245463",
  "occurrence_id": "1630560600000",
  "title": "Team Standup",
  "description": "Daily sync on team progress",
  "start_time": "2021-06-27T05:30:00Z",
  "duration": 60,
  "timezone": "America/Los_Angeles",
  "visibility": "public",
  "recording_enabled": true,
  "recording_access": "meeting_participants",
  "transcript_enabled": true,
  "transcript_access": "meeting_participants",
  "ai_summary_access": "public",
  "require_ai_summary_approval": false,
  "restricted": false,
  "committees": [
    {
      "uid": "committee-uuid-123",
      "allowed_voting_statuses": ["voting_rep", "observer"]
    }
  ]
}
```

**ITX API**:

```json
{
  "project_id": "7cad5a8d-19d0-41a4-81a6-043453daf9ee",
  "meeting_id": "12343245463",
  "occurrence_id": "1630560600000",
  "topic": "Team Standup",
  "agenda": "Daily sync on team progress",
  "start_time": "2021-06-27T05:30:00Z",
  "duration": 60,
  "timezone": "America/Los_Angeles",
  "visibility": "public",
  "recording_enabled": true,
  "recording_access": "meeting_participants",
  "transcript_enabled": true,
  "transcript_access": "meeting_participants",
  "ai_summary_access": "public",
  "require_ai_summary_approval": false,
  "restricted": false,
  "committees": [
    {
      "id": "committee-uuid-123",
      "filters": ["voting_rep", "observer"]
    }
  ]
}
```

---

## Response Schemas

### Past Meeting Response

**Proxy API**:

```json
{
  "project_uid": "7cad5a8d-19d0-41a4-81a6-043453daf9ee",
  "past_meeting_id": "12343245463-1630560600000",
  "meeting_id": "12343245463",
  "occurrence_id": "1630560600000",
  "is_manually_created": false,
  "title": "Team Standup",
  "description": "Daily sync on team progress",
  "start_time": "2021-06-27T05:30:00Z",
  "duration": 60,
  "timezone": "America/Los_Angeles",
  "visibility": "public",
  "meeting_type": 2,
  "recording_enabled": true,
  "recording_access": "meeting_participants",
  "transcript_enabled": true,
  "transcript_access": "meeting_participants",
  "ai_summary_access": "public",
  "require_ai_summary_approval": false,
  "restricted": false,
  "share_url": "https://zoom.us/rec/play/abc123",
  "password": "EsECRa5",
  "ai_summary_url": "https://zoom-lfx.dev.platform.linuxfoundation.org/meeting/1234567890/summaries?password=111",
  "youtube_link": "https://www.youtube.com/watch?v=1234567890",
  "committees": [...],
  "invitees": [...],
  "invitee_count": 25,
  "attendees": [...],
  "attendee_count": 20,
  "unverified_attendee_count": 2
}
```

**ITX API**:

```json
{
  "project_id": "7cad5a8d-19d0-41a4-81a6-043453daf9ee",
  "past_meeting_id": "12343245463-1630560600000",
  "meeting_id": "12343245463",
  "occurrence_id": "1630560600000",
  "is_manually_created": false,
  "topic": "Team Standup",
  "agenda": "Daily sync on team progress",
  "start_time": "2021-06-27T05:30:00Z",
  "duration": 60,
  "timezone": "America/Los_Angeles",
  "visibility": "public",
  "meeting_type": 2,
  "recording_enabled": true,
  "recording_access": "meeting_participants",
  "transcript_enabled": true,
  "transcript_access": "meeting_participants",
  "ai_summary_access": "public",
  "require_ai_summary_approval": false,
  "restricted": false,
  "share_url": "https://zoom.us/rec/play/abc123",
  "password": "EsECRa5",
  "ai_summary_url": "https://zoom-lfx.dev.platform.linuxfoundation.org/meeting/1234567890/summaries?password=111",
  "youtube_link": "https://www.youtube.com/watch?v=1234567890",
  "committees": [...],
  "invitees": [...],
  "invitee_count": 25,
  "attendees": [...],
  "attendee_count": 20,
  "unverified_attendee_count": 2
}
```

---

## Field Mapping

| Proxy API Field | ITX API Field | Notes |
|----------------|---------------|-------|
| `project_uid` | `project_id` | LFX project identifier |
| `title` | `topic` | Meeting title/subject |
| `description` | `agenda` | Meeting description/agenda |
| `committees[].uid` | `committees[].id` | Committee identifier |
| `committees[].allowed_voting_statuses` | `committees[].filters` | Committee member filters |

All other fields use identical naming between the proxy and ITX APIs.

---

## Important Notes

1. **Past Meeting ID Format**: The `past_meeting_id` is a hyphenated combination of `meeting_id` and `occurrence_id` (e.g., "12343245463-1630560600000")

2. **Basic vs Full Response**: Use the `basic=true` query parameter to get lightweight responses without invitees/attendees lists

3. **Manual vs Webhook**: The `is_manually_created` field indicates whether the past meeting was created manually via the API (true) or came from a Zoom webhook event (false)

4. **Read-only Fields**: Fields like `share_url`, `password`, `ai_summary_url`, and `youtube_link` are read-only and returned only in responses

5. **Invitees vs Attendees**:
   - **Invitees**: Users who were registered/invited to the meeting
   - **Attendees**: Users who actually joined and attended the meeting
