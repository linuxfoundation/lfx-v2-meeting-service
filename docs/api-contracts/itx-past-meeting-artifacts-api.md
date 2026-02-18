# ITX Past Meeting Artifacts API Contracts

This document details the API contracts for past meeting artifacts endpoints between the LFX v2 Meeting Service proxy API and the underlying ITX Zoom API.

## Endpoints

### Get Past Meeting Artifact

Retrieves a specific artifact by ID for a past meeting occurrence.

**Proxy Endpoint**: `GET /itx/past_meetings/{past_meeting_id}/artifacts/{artifact_id}`

**ITX Endpoint**: `GET /v2/zoom/past_meetings/{past_meeting}/artifacts/{artifact}`

**Path Parameters**:
- `past_meeting_id` (string, required): The hyphenated meeting and occurrence ID
- `artifact_id` (string, required): UUID of the artifact

**Authorization**: Requires `viewer` permission on the meeting

---

### Create Past Meeting Artifact

Creates a new artifact for a past meeting occurrence.

**Proxy Endpoint**: `POST /itx/past_meetings/{past_meeting_id}/artifacts`

**ITX Endpoint**: `POST /v2/zoom/past_meetings/{past_meeting}/artifacts`

**Path Parameters**:
- `past_meeting_id` (string, required): The hyphenated meeting and occurrence ID

**Authorization**: Requires `organizer` permission on the meeting

---

### Update Past Meeting Artifact

Updates an existing artifact for a past meeting occurrence.

**Proxy Endpoint**: `PUT /itx/past_meetings/{past_meeting_id}/artifacts/{artifact_id}`

**ITX Endpoint**: `PUT /v2/zoom/past_meetings/{past_meeting}/artifacts/{artifact}`

**Path Parameters**:
- `past_meeting_id` (string, required): The hyphenated meeting and occurrence ID
- `artifact_id` (string, required): UUID of the artifact

**Authorization**: Requires `organizer` permission on the meeting

---

### Delete Past Meeting Artifact

Deletes an artifact from a past meeting occurrence.

**Proxy Endpoint**: `DELETE /itx/past_meetings/{past_meeting_id}/artifacts/{artifact_id}`

**ITX Endpoint**: `DELETE /v2/zoom/past_meetings/{past_meeting}/artifacts/{artifact}`

**Path Parameters**:
- `past_meeting_id` (string, required): The hyphenated meeting and occurrence ID
- `artifact_id` (string, required): UUID of the artifact

**Authorization**: Requires `organizer` permission on the meeting

---

## Request Schemas

### Create Artifact Request

**Proxy API & ITX API** (Identical):
```json
{
  "category": "Meeting Minutes",
  "name": "Q4 Planning Meeting Minutes",
  "link": "https://docs.google.com/document/d/abc123"
}
```

**Fields**:
- `category` (string, required): Category of the artifact
  - Allowed values: `"Meeting Minutes"`, `"Notes"`, `"Presentation"`, `"Other"`
- `name` (string, required): Name/title of the artifact
- `link` (string, required): URL link to the artifact resource

---

### Update Artifact Request

**Proxy API & ITX API** (Identical):
```json
{
  "category": "Notes",
  "name": "Updated Meeting Notes",
  "link": "https://docs.google.com/document/d/xyz789"
}
```

All fields are optional - only include fields you want to update.

---

## Response Schemas

### Artifact Response

**Proxy API & ITX API** (Identical):
```json
{
  "id": "ea1e8536-a985-4cf5-b981-a170927a1d11",
  "category": "Meeting Minutes",
  "name": "Q4 Planning Meeting Minutes",
  "link": "https://docs.google.com/document/d/abc123",
  "created_at": "1706710000",
  "created_by": {
    "id": "user-uuid-123",
    "username": "jdoe",
    "name": "John Doe",
    "email": "john.doe@example.com"
  },
  "updated_at": "1706710000",
  "updated_by": {
    "id": "user-uuid-456",
    "username": "jsmith",
    "name": "Jane Smith",
    "email": "jane.smith@example.com"
  }
}
```

**Response Fields**:
- `id` (string): UUID of the artifact (read-only)
- `category` (string): Category of the artifact
- `name` (string): Name/title of the artifact
- `link` (string): URL to the artifact resource
- `created_at` (string): Unix timestamp of creation (read-only)
- `created_by` (object): User who created the artifact (read-only)
- `updated_at` (string): Unix timestamp of last update (read-only)
- `updated_by` (object): User who last updated the artifact (read-only)

---

## Field Mapping

**No field mapping required** - all fields are identical between the proxy and ITX APIs.

---

## Artifact Categories

The following artifact categories are supported:

| Category | Description |
|----------|-------------|
| `Meeting Minutes` | Official meeting minutes/notes |
| `Notes` | General notes or documentation |
| `Presentation` | Slide decks or presentations |
| `Other` | Any other type of artifact |

---

## Important Notes

1. **Past Meeting Identification**: Artifacts are always associated with a specific past meeting occurrence using the hyphenated `past_meeting_id` format (e.g., "12343245463-1630560600000")

2. **External Links Only**: Artifacts store links to external resources (Google Docs, SharePoint, etc.) - they do not store file uploads directly

3. **Audit Trail**: All artifacts track who created and last updated them, along with timestamps for full audit capability

4. **UUID Identifiers**: Each artifact has a unique UUID identifier that is generated automatically upon creation

5. **Link Validation**: The `link` field should be a valid URL - the API may validate URL format

6. **Category Enforcement**: The `category` field must be one of the predefined enum values

7. **Update Semantics**: When updating an artifact, only provide fields you want to change - omitted fields remain unchanged

8. **Delete Behavior**: Deleting an artifact is permanent and cannot be undone
