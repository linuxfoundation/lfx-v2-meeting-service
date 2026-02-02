# ITX Occurrences API Contracts

This document details the API contracts for ITX Zoom meeting occurrence proxy endpoints, showing both the proxy API (LFX Meeting Service) and the underlying ITX API schemas.

## Overview

The LFX Meeting Service proxies occurrence requests to the ITX Zoom API service:

```
Client → LFX Meeting Service (Proxy) → ITX Service → Zoom API
```

**Proxy API (LFX Meeting Service)**:

- Base Path: `/itx/meetings/{meeting_id}/occurrences`
- Authorization: Bearer token (JWT via Heimdall/OpenFGA)
- Version: Query parameter `v` (e.g., `?v=1`)

**ITX API (Underlying Service)**:

- Base Path: `/v2/zoom/meetings/{meeting_id}/occurrences`
- Authorization: OAuth2 M2M (added automatically by proxy)
- Header: `x-scope: manage:zoom`

---

## Update Occurrence

Updates a specific occurrence of a recurring meeting.

### Proxy API Endpoint

**Method**: `PUT /itx/meetings/{meeting_id}/occurrences/{occurrence_id}?v=1`

**Authorization**: Requires `organizer` permission on the meeting

**Request Headers**:

```
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

**Path Parameters**:

- `meeting_id` (string, required) - The Zoom meeting ID
- `occurrence_id` (string, required) - The occurrence ID (Unix timestamp)

**Request Body**:

```json
{
  "start_time": "2024-01-15T10:00:00Z",
  "duration": 60,
  "topic": "Updated Weekly Team Sync",
  "agenda": "Updated agenda for this specific occurrence",
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

**Request Fields**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `start_time` | string | No | Meeting start time (RFC3339 format) |
| `duration` | integer | No | Meeting duration in minutes (minimum 1) |
| `topic` | string | No | Meeting topic/title |
| `agenda` | string | No | Meeting agenda/description |
| `recurrence` | object | No | Recurrence settings (see Recurrence object) |

**Recurrence Object**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | integer | Yes | Recurrence type: 1=daily, 2=weekly, 3=monthly |
| `repeat_interval` | integer | Yes | Interval between occurrences |
| `weekly_days` | string | No | Days of week (1-7, comma-separated) |
| `monthly_day` | integer | No | Day of month (1-31) |
| `monthly_week` | integer | No | Week of month (-1=last, 1-4) |
| `monthly_week_day` | integer | No | Day of week (1=Sunday to 7=Saturday) |
| `end_times` | integer | No | Number of occurrences before ending |
| `end_date_time` | string | No | End date/time (RFC3339 format) |

**Response**: `204 No Content`

No response body on success.

**Error Responses**:

- `400 Bad Request` - Invalid request parameters
- `401 Unauthorized` - Missing or invalid authentication
- `403 Forbidden` - Insufficient permissions
- `404 Not Found` - Meeting or occurrence not found
- `500 Internal Server Error` - Server error
- `503 Service Unavailable` - Service temporarily unavailable

### ITX API Endpoint

**Method**: `PUT /v2/zoom/meetings/{meeting_id}/occurrences/{occurrence_id}`

**Request Headers**:

```
Authorization: Bearer <oauth2_m2m_token>
x-scope: manage:zoom
Content-Type: application/json
```

**Path Parameters**:

- `meeting_id` (string, required) - The Zoom meeting ID
- `occurrence_id` (string, required) - The occurrence ID (Unix timestamp)

**Request Body**: Identical to Proxy API

**Response**: `204 No Content`

### Field Mapping

All fields are identical between Proxy and ITX API for occurrence updates.

---

## Delete Occurrence

Deletes a specific occurrence of a recurring meeting.

### Proxy API Endpoint

**Method**: `DELETE /itx/meetings/{meeting_id}/occurrences/{occurrence_id}?v=1`

**Authorization**: Requires `organizer` permission on the meeting

**Request Headers**:

```
Authorization: Bearer <jwt_token>
```

**Path Parameters**:

- `meeting_id` (string, required) - The Zoom meeting ID
- `occurrence_id` (string, required) - The occurrence ID (Unix timestamp)

**Response**: `204 No Content`

No response body on success.

**Error Responses**:

- `400 Bad Request` - Invalid request parameters
- `401 Unauthorized` - Missing or invalid authentication
- `403 Forbidden` - Insufficient permissions
- `404 Not Found` - Meeting or occurrence not found
- `500 Internal Server Error` - Server error
- `503 Service Unavailable` - Service temporarily unavailable

### ITX API Endpoint

**Method**: `DELETE /v2/zoom/meetings/{meeting_id}/occurrences/{occurrence_id}`

**Request Headers**:

```
Authorization: Bearer <oauth2_m2m_token>
x-scope: manage:zoom
```

**Path Parameters**:

- `meeting_id` (string, required) - The Zoom meeting ID
- `occurrence_id` (string, required) - The occurrence ID (Unix timestamp)

**Response**: `204 No Content`

---

## Common Data Types

### Recurrence Object

The recurrence object structure is identical in both Proxy and ITX APIs:

```json
{
  "type": 2,
  "repeat_interval": 1,
  "weekly_days": "1,3,5",
  "monthly_day": 15,
  "monthly_week": 2,
  "monthly_week_day": 3,
  "end_times": 10,
  "end_date_time": "2024-12-31T23:59:59Z"
}
```

**Recurrence Types**:

- `1` - Daily recurrence
- `2` - Weekly recurrence
- `3` - Monthly recurrence

**Weekly Days** (comma-separated string):

- `1` - Sunday
- `2` - Monday
- `3` - Tuesday
- `4` - Wednesday
- `5` - Thursday
- `6` - Friday
- `7` - Saturday

**Monthly Week**:

- `-1` - Last week of the month
- `1` - First week
- `2` - Second week
- `3` - Third week
- `4` - Fourth week

**Monthly Week Day** (1-7, where 1=Sunday, 7=Saturday)

---

## Occurrence IDs

Occurrence IDs are Unix timestamps representing the start time of the occurrence:

**Example**:

```
occurrence_id: "1640995200"
```

This corresponds to `2021-12-31T20:00:00Z` in Unix time.

**How to get occurrence IDs**:

1. Create or retrieve a recurring meeting
2. The response includes an `occurrences` array
3. Each occurrence has an `occurrence_id` field

**Example from meeting response**:

```json
{
  "occurrences": [
    {
      "occurrence_id": "1640995200",
      "start_time": "2021-12-31T20:00:00Z",
      "duration": 60,
      "status": "available",
      "registrant_count": 0
    },
    {
      "occurrence_id": "1641600000",
      "start_time": "2022-01-07T20:00:00Z",
      "duration": 60,
      "status": "available",
      "registrant_count": 0
    }
  ]
}
```

---

## Summary of Key Differences

| Aspect | Proxy API (LFX) | ITX API |
|--------|-----------------|---------|
| **Base Path** | `/itx/meetings/{meeting_id}/occurrences` | `/v2/zoom/meetings/{meeting_id}/occurrences` |
| **Authentication** | JWT Bearer token | OAuth2 M2M token |
| **Authorization** | Heimdall/OpenFGA (organizer permission) | Handled by ITX service |
| **Field Names** | All identical | All identical |
| **Required Header** | `Authorization: Bearer <jwt>` | `Authorization: Bearer <oauth2>` + `x-scope: manage:zoom` |

**Note**: Unlike meetings and registrants, occurrence endpoints use completely identical field names between the Proxy and ITX APIs. No field name conversion is needed.

---

## Authorization Requirements

| Endpoint | Required Permission |
|----------|-------------------|
| Update Occurrence | `organizer` on meeting |
| Delete Occurrence | `organizer` on meeting |

Both occurrence operations require the `organizer` permission on the parent meeting, as modifying or deleting occurrences are administrative operations.

---

## Error Responses

Both APIs return similar error structures:

**HTTP Status Codes**:

- `400 Bad Request` - Invalid request parameters
- `401 Unauthorized` - Missing or invalid authentication
- `403 Forbidden` - Insufficient permissions
- `404 Not Found` - Meeting or occurrence not found
- `500 Internal Server Error` - Server error
- `503 Service Unavailable` - Service temporarily unavailable

**Error Response Body**:

```json
{
  "error": "Error message describing what went wrong",
  "message": "Detailed error message"
}
```

---

## Usage Examples

### Example 1: Update a Single Occurrence

Update only the start time and duration of a specific occurrence:

**Request**:

```http
PUT /itx/meetings/1234567890/occurrences/1640995200?v=1
Authorization: Bearer <jwt_token>
Content-Type: application/json

{
  "start_time": "2021-12-31T21:00:00Z",
  "duration": 90
}
```

**Response**: `204 No Content`

### Example 2: Update Occurrence with New Recurrence Pattern

Modify an occurrence and change the recurrence pattern for all future occurrences:

**Request**:

```http
PUT /itx/meetings/1234567890/occurrences/1640995200?v=1
Authorization: Bearer <jwt_token>
Content-Type: application/json

{
  "topic": "Rescheduled Team Sync",
  "recurrence": {
    "type": 2,
    "repeat_interval": 2,
    "weekly_days": "2,4",
    "end_times": 5
  }
}
```

**Response**: `204 No Content`

### Example 3: Delete a Specific Occurrence

Cancel a specific occurrence of a recurring meeting:

**Request**:

```http
DELETE /itx/meetings/1234567890/occurrences/1640995200?v=1
Authorization: Bearer <jwt_token>
```

**Response**: `204 No Content`

---

## Important Notes

### Updating vs Deleting Occurrences

- **Update**: Modifies the specified occurrence. Changes can be isolated to just that occurrence or can affect all future occurrences (when recurrence is updated).
- **Delete**: Permanently removes the specified occurrence. This cannot be undone. The occurrence will no longer appear in the meeting's occurrence list.

### Impact on Registrants

- When an occurrence is updated, all registrants for that occurrence remain registered
- When an occurrence is deleted, registrants specific to that occurrence are automatically unregistered
- If registrants are registered for "all occurrences," they remain registered for other occurrences

### Recurrence Pattern Changes

When updating an occurrence with a new `recurrence` object:

- The change typically applies to the specified occurrence and all future occurrences
- Past occurrences remain unchanged
- This behavior depends on the Zoom API implementation

### Occurrence Status

After deletion, the occurrence status changes:

- From: `"status": "available"`
- To: `"status": "deleted"` or removed from the occurrences list

Check the parent meeting's `occurrences` array to see the current status of all occurrences.
