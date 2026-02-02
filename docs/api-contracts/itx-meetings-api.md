# ITX Meetings API Contracts

This document details the API contracts for ITX Zoom meeting proxy endpoints, showing both the proxy API (LFX Meeting Service) and the underlying ITX API schemas.

## Overview

The LFX Meeting Service proxies requests to the ITX Zoom API service with the following flow:

```
Client → LFX Meeting Service (Proxy) → ITX Service → Zoom API
```

**Proxy API (LFX Meeting Service)**:

- Base Path: `/itx`
- Authorization: Bearer token (JWT via Heimdall/OpenFGA)
- Version: Query parameter `v` (e.g., `?v=1`)

**ITX API (Underlying Service)**:

- Base Path: `/v2/zoom`
- Authorization: OAuth2 M2M (added automatically by proxy)
- Header: `x-scope: manage:zoom`

---

## Create Meeting

### Proxy API Endpoint

**Method**: `POST /itx/meetings?v=1`

**Authorization**: Requires `writer` permission on the project

**Request Headers**:

```
Authorization: Bearer <jwt_token>
X-Sync: true|false (optional)
Content-Type: application/json
```

**Request Body**:

```json
{
  "project_uid": "7cad5a8d-19d0-41a4-81a6-043453daf9ee",
  "title": "Weekly Team Sync",
  "start_time": "2024-01-15T10:00:00Z",
  "duration": 60,
  "timezone": "America/New_York",
  "visibility": "public",
  "description": "Weekly team synchronization meeting",
  "restricted": false,
  "committees": [
    {
      "uid": "committee-uuid",
      "allowed_voting_statuses": ["active", "pending"]
    }
  ],
  "meeting_type": "Technical",
  "early_join_time_minutes": 15,
  "recording_enabled": true,
  "transcript_enabled": true,
  "youtube_upload_enabled": false,
  "artifact_visibility": "meeting_participants",
  "recurrence": {
    "type": 2,
    "repeat_interval": 1,
    "weekly_days": "1,3,5",
    "monthly_day": 15,
    "monthly_week": 2,
    "monthly_week_day": 3,
    "end_times": 10,
    "end_date_time": "2024-12-31T23:59:59Z"
  }
}
```

**Response**: `201 Created`

```json
{
  "project_uid": "7cad5a8d-19d0-41a4-81a6-043453daf9ee",
  "title": "Weekly Team Sync",
  "start_time": "2024-01-15T10:00:00Z",
  "duration": 60,
  "timezone": "America/New_York",
  "visibility": "public",
  "description": "Weekly team synchronization meeting",
  "restricted": false,
  "committees": [
    {
      "uid": "committee-uuid",
      "allowed_voting_statuses": ["active", "pending"]
    }
  ],
  "meeting_type": "Technical",
  "early_join_time_minutes": 15,
  "recording_enabled": true,
  "transcript_enabled": true,
  "youtube_upload_enabled": false,
  "artifact_visibility": "meeting_participants",
  "recurrence": {
    "type": 2,
    "repeat_interval": 1,
    "weekly_days": "1,3,5",
    "end_times": 10
  },
  "id": "1234567890",
  "host_key": "123456",
  "passcode": "abc123",
  "password": "7cad5a8d-19d0-41a4-81a6-043453daf9ee",
  "public_link": "https://zoom-lfx.platform.linuxfoundation.org/meeting/1234567890",
  "created_at": "2024-01-10T08:00:00Z",
  "modified_at": "2024-01-10T08:00:00Z",
  "registrant_count": 0,
  "occurrences": [
    {
      "occurrence_id": "1640995200",
      "start_time": "2024-01-15T10:00:00Z",
      "duration": 60,
      "status": "available",
      "registrant_count": 0
    }
  ]
}
```

### ITX API Endpoint

**Method**: `POST /v2/zoom/meetings`

**Request Headers**:

```
Authorization: Bearer <oauth2_m2m_token>
x-scope: manage:zoom
Content-Type: application/json
```

**Request Body**:

```json
{
  "project": "7cad5a8d-19d0-41a4-81a6-043453daf9ee",
  "topic": "Weekly Team Sync",
  "start_time": "2024-01-15T10:00:00Z",
  "duration": 60,
  "timezone": "America/New_York",
  "visibility": "public",
  "agenda": "Weekly team synchronization meeting",
  "restricted": false,
  "committees": [
    {
      "id": "committee-uuid",
      "filters": ["active", "pending"]
    }
  ],
  "meeting_type": "Technical",
  "early_join_time": 15,
  "recording_enabled": true,
  "transcript_enabled": true,
  "youtube_upload_enabled": false,
  "recording_access": "meeting_participants",
  "recurrence": {
    "type": 2,
    "repeat_interval": 1,
    "weekly_days": "1,3,5",
    "monthly_day": 15,
    "monthly_week": 2,
    "monthly_week_day": 3,
    "end_times": 10,
    "end_date_time": "2024-12-31T23:59:59Z"
  }
}
```

**Response**: `201 Created`

```json
{
  "id": "1234567890",
  "project": "7cad5a8d-19d0-41a4-81a6-043453daf9ee",
  "topic": "Weekly Team Sync",
  "start_time": "2024-01-15T10:00:00Z",
  "duration": 60,
  "timezone": "America/New_York",
  "visibility": "public",
  "agenda": "Weekly team synchronization meeting",
  "restricted": false,
  "committees": [
    {
      "id": "committee-uuid",
      "filters": ["active", "pending"]
    }
  ],
  "meeting_type": "Technical",
  "early_join_time": 15,
  "recording_enabled": true,
  "transcript_enabled": true,
  "youtube_upload_enabled": false,
  "recording_access": "meeting_participants",
  "recurrence": {
    "type": 2,
    "repeat_interval": 1,
    "weekly_days": "1,3,5",
    "end_times": 10
  },
  "host_key": "123456",
  "passcode": "abc123",
  "password": "7cad5a8d-19d0-41a4-81a6-043453daf9ee",
  "public_link": "https://zoom-lfx.platform.linuxfoundation.org/meeting/1234567890",
  "created_at": "2024-01-10T08:00:00Z",
  "modified_at": "2024-01-10T08:00:00Z",
  "registrant_count": 0,
  "occurrences": [
    {
      "occurrence_id": "1640995200",
      "start_time": "2024-01-15T10:00:00Z",
      "duration": 60,
      "status": "available",
      "registrant_count": 0
    }
  ]
}
```

### Field Mapping

| Proxy API (LFX) | ITX API | Notes |
|-----------------|---------|-------|
| `project_uid` | `project` | Project identifier |
| `title` | `topic` | Meeting title |
| `description` | `agenda` | Meeting description |
| `artifact_visibility` | `recording_access` | Recording access level |
| `early_join_time_minutes` | `early_join_time` | Minutes users can join early |
| `committees[].uid` | `committees[].id` | Committee identifier |
| `committees[].allowed_voting_statuses` | `committees[].filters` | Voting status filters |
| (N/A - added by proxy) | `id` | Zoom meeting ID (response only) |

---

## Get Meeting

### Proxy API Endpoint

**Method**: `GET /itx/meetings/{meeting_id}?v=1`

**Authorization**: Requires `viewer` permission on the meeting

**Request Headers**:

```
Authorization: Bearer <jwt_token>
```

**Path Parameters**:

- `meeting_id` (string, required) - The Zoom meeting ID

**Response**: `200 OK`

Response body is identical to Create Meeting response.

### ITX API Endpoint

**Method**: `GET /v2/zoom/meetings/{meeting_id}`

**Request Headers**:

```
Authorization: Bearer <oauth2_m2m_token>
x-scope: manage:zoom
```

**Path Parameters**:

- `meeting_id` (string, required) - The Zoom meeting ID

**Response**: `200 OK`

Response body is identical to ITX Create Meeting response.

### Field Mapping

Same as Create Meeting field mapping.

---

## Update Meeting

### Proxy API Endpoint

**Method**: `PUT /itx/meetings/{meeting_id}?v=1`

**Authorization**: Requires `organizer` permission on the meeting

**Request Headers**:

```
Authorization: Bearer <jwt_token>
X-Sync: true|false (optional)
Content-Type: application/json
```

**Path Parameters**:

- `meeting_id` (string, required) - The Zoom meeting ID

**Request Body**: Same as Create Meeting request body

**Response**: `204 No Content`

### ITX API Endpoint

**Method**: `PUT /v2/zoom/meetings/{meeting_id}`

**Request Headers**:

```
Authorization: Bearer <oauth2_m2m_token>
x-scope: manage:zoom
Content-Type: application/json
```

**Path Parameters**:

- `meeting_id` (string, required) - The Zoom meeting ID

**Request Body**: Same as ITX Create Meeting request body

**Response**: `204 No Content`

### Field Mapping

Same as Create Meeting field mapping.

---

## Delete Meeting

### Proxy API Endpoint

**Method**: `DELETE /itx/meetings/{meeting_id}?v=1`

**Authorization**: Requires `organizer` permission on the meeting

**Request Headers**:

```
Authorization: Bearer <jwt_token>
```

**Path Parameters**:

- `meeting_id` (string, required) - The Zoom meeting ID

**Response**: `204 No Content`

### ITX API Endpoint

**Method**: `DELETE /v2/zoom/meetings/{meeting_id}`

**Request Headers**:

```
Authorization: Bearer <oauth2_m2m_token>
x-scope: manage:zoom
```

**Path Parameters**:

- `meeting_id` (string, required) - The Zoom meeting ID

**Response**: `204 No Content`

---

## Get Meeting Count

### Proxy API Endpoint

**Method**: `GET /itx/meeting_count?v=1&project_uid={project_uid}`

**Authorization**: Requires `viewer` permission on the project

**Request Headers**:

```
Authorization: Bearer <jwt_token>
```

**Query Parameters**:

- `project_uid` (string, required) - The UID of the LF project
- `v` (string, required) - API version

**Response**: `200 OK`

```json
{
  "meeting_count": 42
}
```

### ITX API Endpoint

**Method**: `GET /v2/zoom/meeting_count?project={project_id}`

**Request Headers**:

```
Authorization: Bearer <oauth2_m2m_token>
x-scope: manage:zoom
```

**Query Parameters**:

- `project` (string, required) - The project identifier

**Response**: `200 OK`

```json
{
  "meeting_count": 42
}
```

### Field Mapping

| Proxy API (LFX) | ITX API | Notes |
|-----------------|---------|-------|
| `project_uid` (query param) | `project` (query param) | Project identifier |
| `meeting_count` | `meeting_count` | Identical |

---

## Get Join Link

### Proxy API Endpoint

**Method**: `GET /itx/meetings/{meeting_id}/join_link?v=1`

**Authorization**: Requires `viewer` permission on the meeting

**Request Headers**:

```
Authorization: Bearer <jwt_token>
```

**Path Parameters**:

- `meeting_id` (string, required) - The Zoom meeting ID

**Query Parameters**:

- `use_email` (boolean, optional) - Use email for identification
- `user_id` (string, optional) - LF user ID
- `name` (string, optional) - User's full name
- `email` (string, optional) - User's email address
- `register` (boolean, optional) - Register user as guest

**Response**: `200 OK`

```json
{
  "link": "https://zoom.us/j/1234567891?pwd=NTNubnB4bnpPTm9zT2VLZFJnQ1RkUT11"
}
```

### ITX API Endpoint

**Method**: `GET /v2/zoom/meetings/{meeting_id}/join_link`

**Request Headers**:

```
Authorization: Bearer <oauth2_m2m_token>
x-scope: manage:zoom
```

**Path Parameters**:

- `meeting_id` (string, required) - The Zoom meeting ID

**Query Parameters**: Same as Proxy API

**Response**: `200 OK`

```json
{
  "link": "https://zoom.us/j/1234567891?pwd=NTNubnB4bnpPTm9zT2VLZFJnQ1RkUT11"
}
```

### Field Mapping

All fields are identical between Proxy and ITX API.

---

## Resend Meeting Invitations

### Proxy API Endpoint

**Method**: `POST /itx/meetings/{meeting_id}/resend?v=1`

**Authorization**: Requires `organizer` permission on the meeting

**Request Headers**:

```
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

**Path Parameters**:

- `meeting_id` (string, required) - The Zoom meeting ID

**Request Body**:

```json
{
  "exclude_registrant_ids": ["reg123", "reg456"]
}
```

**Response**: `204 No Content`

### ITX API Endpoint

**Method**: `POST /v2/zoom/meetings/{meeting_id}/resend`

**Request Headers**:

```
Authorization: Bearer <oauth2_m2m_token>
x-scope: manage:zoom
Content-Type: application/json
```

**Path Parameters**:

- `meeting_id` (string, required) - The Zoom meeting ID

**Request Body**: Identical to Proxy API

**Response**: `204 No Content`

### Field Mapping

All fields are identical between Proxy and ITX API.

---

## Register Committee Members

### Proxy API Endpoint

**Method**: `POST /itx/meetings/{meeting_id}/register_committee_members?v=1`

**Authorization**: Requires `organizer` permission on the meeting

**Request Headers**:

```
Authorization: Bearer <jwt_token>
```

**Path Parameters**:

- `meeting_id` (string, required) - The Zoom meeting ID

**Response**: `204 No Content`

Note: Registration happens asynchronously.

### ITX API Endpoint

**Method**: `POST /v2/zoom/meetings/{meeting_id}/register_committee_members`

**Request Headers**:

```
Authorization: Bearer <oauth2_m2m_token>
x-scope: manage:zoom
```

**Path Parameters**:

- `meeting_id` (string, required) - The Zoom meeting ID

**Response**: `204 No Content`

---

## Common Data Types

### Recurrence Object

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | integer | Yes | 1=daily, 2=weekly, 3=monthly |
| `repeat_interval` | integer | Yes | Interval between occurrences |
| `weekly_days` | string | No | Days of week (1-7, comma-separated) |
| `monthly_day` | integer | No | Day of month (1-31) |
| `monthly_week` | integer | No | Week of month (-1=last, 1-4) |
| `monthly_week_day` | integer | No | Day of week (1=Sunday to 7=Saturday) |
| `end_times` | integer | No | Number of occurrences before ending |
| `end_date_time` | string | No | End date/time (RFC3339 format) |

**Note**: Recurrence object structure is identical in both Proxy and ITX APIs.

### Committee Object

**Proxy API**:

```json
{
  "uid": "committee-uuid",
  "allowed_voting_statuses": ["active", "pending"]
}
```

**ITX API**:

```json
{
  "id": "committee-uuid",
  "filters": ["active", "pending"]
}
```

### Occurrence Object

**Structure is identical in both APIs**:

```json
{
  "occurrence_id": "1640995200",
  "start_time": "2024-01-15T10:00:00Z",
  "duration": 60,
  "status": "available",
  "registrant_count": 0
}
```

---

## Summary of Key Differences

| Aspect | Proxy API (LFX) | ITX API |
|--------|-----------------|---------|
| **Base Path** | `/itx` | `/v2/zoom` |
| **Authentication** | JWT Bearer token | OAuth2 M2M token |
| **Authorization** | Heimdall/OpenFGA | Handled by ITX service |
| **Project Field** | `project_uid` | `project` |
| **Meeting Title** | `title` | `topic` |
| **Meeting Description** | `description` | `agenda` |
| **Recording Access** | `artifact_visibility` | `recording_access` |
| **Early Join Time** | `early_join_time_minutes` | `early_join_time` |
| **Committee ID** | `committees[].uid` | `committees[].id` |
| **Committee Filters** | `committees[].allowed_voting_statuses` | `committees[].filters` |
| **Required Header** | `Authorization: Bearer <jwt>` | `Authorization: Bearer <oauth2>` + `x-scope: manage:zoom` |

---

## Error Responses

Both APIs return similar error structures:

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
