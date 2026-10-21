# ITX Meeting Attachments API Contracts

This document details the API contracts for meeting attachments endpoints between the LFX v2 Meeting Service proxy API and the underlying ITX Zoom API.

## Audit stamping

The LFX Meeting Service proxy adds identity-attribution fields to ITX attachment write requests so ITX records who touched each attachment. These fields are populated by the proxy from the authenticated JWT principal — clients do not send them in their proxy request; if provided, they are overwritten.

- `created_by` — stamped on `POST` (create) requests, including the presigned-upload-URL request (ITX persists an attachment record with `file_upload_status="ongoing"` at the presign step).
- `updated_by` — stamped on `PUT` (update) requests.

Shape (attachment endpoints use the reduced `{ username, name, email }` shape — no `profile_picture`):

```json
{
  "username": "jdoe",
  "name": "Jane Doe",
  "email": "jane.doe@example.com"
}
```

The name / email are resolved from the auth service (via a NATS lookup) using the JWT's `username` claim. If the lookup fails or NATS is unavailable, the proxy degrades gracefully to `{ username, email }` (email from the JWT claim) and never blocks the write. Prior to July 2026 the attachment endpoints only sent `{ username }`; the `name` / `email` enrichment was added to bring them in line with the meeting endpoints.

## Occurrence scope

An attachment applies either to **all occurrences** of a meeting or to **one single occurrence** of a recurring meeting. The scope is the optional `occurrence_id` field: the occurrence's start time as a unix timestamp string, the same ID used in `/itx/meetings/{meeting_id}/occurrences/{occurrence_id}`.

- **Omitted** means all occurrences. This is the default, and every attachment created before this field existed behaves this way.
- **Set** scopes the attachment to that occurrence only. It must be an upcoming, non-cancelled occurrence of a recurring meeting; otherwise ITX rejects the request with `400`. Non-recurring meetings have no occurrences, so they cannot be scoped.
- `occurrence_id` is accepted on create, presign and update, and returned on every attachment response (omitted when series-wide).
- **Update replaces the scope.** `PUT` is a full replacement, so omitting `occurrence_id` on an update changes a scoped attachment back to all occurrences. A client that edits other fields must send the current `occurrence_id` back to keep the attachment scoped.
- Attachments created from the meeting description (`source: "description"`) always apply to all occurrences and cannot be scoped.
- ITX keeps scoped attachments in step with the occurrence: if the occurrence is rescheduled the attachment follows it to the new `occurrence_id`, and if the occurrence is cancelled the attachment is deleted.

## Endpoints

### List Meeting Attachments

Lists the attachments of a meeting. With `occurrence_id`, returns the attachments that apply to that occurrence: the ones scoped to it plus the ones that apply to all occurrences. Without it, every attachment on the meeting is returned, including those scoped to other occurrences.

**Proxy Endpoint**: `GET /itx/meetings/{meeting_id}/attachments`

**ITX Endpoint**: `GET /v2/zoom/meetings/{meeting_id}/attachments`

**Path Parameters**:
- `meeting_id` (string, required): The Zoom meeting ID

**Query Parameters**:
- `occurrence_id` (string, optional): Only return the attachments that apply to this occurrence

**Authorization**: Requires `viewer` permission on the meeting

**Returns**: 200 with a JSON array of [Attachment Response](#attachment-response) objects (an empty array when there are none)

---

### Create Meeting Attachment

Creates a new attachment record for a meeting. Supports both file attachments (via presigned URL) and external links.

**Proxy Endpoint**: `POST /itx/meetings/{meeting_id}/attachments`

**ITX Endpoint**: `POST /v2/zoom/meetings/{meeting_id}/attachments`

**Path Parameters**:
- `meeting_id` (string, required): The Zoom meeting ID

**Authorization**: Requires `organizer` permission on the meeting

---

### Get Meeting Attachment

Retrieves metadata for a specific attachment.

**Proxy Endpoint**: `GET /itx/meetings/{meeting_id}/attachments/{attachment_id}`

**ITX Endpoint**: `GET /v2/zoom/meetings/{meeting_id}/attachments/{attachment_id}`

**Path Parameters**:
- `meeting_id` (string, required): The Zoom meeting ID
- `attachment_id` (string, required): UUID of the attachment

**Authorization**: Requires `viewer` permission on the meeting

---

### Update Meeting Attachment

Updates an existing attachment's metadata.

**Proxy Endpoint**: `PUT /itx/meetings/{meeting_id}/attachments/{attachment_id}`

**ITX Endpoint**: `PUT /v2/zoom/meetings/{meeting_id}/attachments/{attachment_id}`

**Path Parameters**:
- `meeting_id` (string, required): The Zoom meeting ID
- `attachment_id` (string, required): UUID of the attachment

**Authorization**: Requires `organizer` permission on the meeting

**Returns**: 204 No Content on success

---

### Delete Meeting Attachment

Deletes an attachment from a meeting.

**Proxy Endpoint**: `DELETE /itx/meetings/{meeting_id}/attachments/{attachment_id}`

**ITX Endpoint**: `DELETE /v2/zoom/meetings/{meeting_id}/attachments/{attachment_id}`

**Path Parameters**:
- `meeting_id` (string, required): The Zoom meeting ID
- `attachment_id` (string, required): UUID of the attachment

**Authorization**: Requires `organizer` permission on the meeting

---

### Create Presigned Upload URL

Generates a presigned URL for uploading a file attachment directly to object storage (S3).

**Proxy Endpoint**: `POST /itx/meetings/{meeting_id}/attachments/presign`

**ITX Endpoint**: `POST /v2/zoom/meetings/{meeting_id}/attachments/presign`

**Path Parameters**:
- `meeting_id` (string, required): The Zoom meeting ID

**Authorization**: Requires `organizer` permission on the meeting

---

### Get Attachment Download URL

Generates a presigned download URL for a file attachment.

**Proxy Endpoint**: `GET /itx/meetings/{meeting_id}/attachments/{attachment_id}/download`

**ITX Endpoint**: `GET /v2/zoom/meetings/{meeting_id}/attachments/{attachment_id}/download`

**Path Parameters**:
- `meeting_id` (string, required): The Zoom meeting ID
- `attachment_id` (string, required): UUID of the attachment

**Authorization**: Requires `viewer` permission on the meeting

---

## Request Schemas

### Create Attachment Request (Link Type)

Every create, presign and update request accepts the optional `occurrence_id` field described in [Occurrence scope](#occurrence-scope), for example `"occurrence_id": "1788000000"`.

For external URL references (Google Docs, SharePoint, etc.):

**Proxy API**:
```json
{
  "type": "link",
  "category": "meeting_agenda",
  "name": "Q4 Planning Agenda",
  "description": "Agenda for Q4 planning meeting",
  "link": "https://docs.google.com/document/d/abc123"
}
```

**ITX API**: Identical to the Proxy API request, plus a proxy-added `created_by` field:

```json
{
  "type": "link",
  "category": "meeting_agenda",
  "name": "Q4 Planning Agenda",
  "description": "Agenda for Q4 planning meeting",
  "link": "https://docs.google.com/document/d/abc123",
  "created_by": {
    "username": "jdoe",
    "name": "Jane Doe",
    "email": "jane.doe@example.com"
  }
}
```

> **Proxy-added field**: `created_by` is stamped by the proxy from the requesting user's authenticated JWT principal. See [Audit stamping](#audit-stamping).

**Fields**:
- `type` (string, required): Must be `"link"` for external URLs
- `category` (string, required): Category of the attachment
- `name` (string, required): Name/title of the attachment (1-255 chars)
- `description` (string, optional): Optional description (max 500 chars)
- `link` (string, required for type=link): URL to external resource (max 2048 chars)
- `occurrence_id` (string, optional): Scope the attachment to one occurrence; omit for all occurrences (see [Occurrence scope](#occurrence-scope))

---

### Create Attachment Request (File Type)

For file uploads using presigned URL flow:

**Proxy API**:
```json
{
  "type": "file",
  "category": "meeting_notes",
  "name": "Meeting Notes"
}
```

**ITX API**: Identical to the Proxy API request, plus a proxy-added `created_by` field (same shape as the Link Type example above — see [Audit stamping](#audit-stamping)).

**Fields**:
- `type` (string, required): Must be `"file"` for file attachments
- `category` (string, required): Category of the attachment
- `name` (string, required): Name/title of the attachment (1-255 chars)
- `description` (string, optional): Optional description (max 500 chars)
- `occurrence_id` (string, optional): Scope the attachment to one occurrence; omit for all occurrences (see [Occurrence scope](#occurrence-scope))

**Note**: File uploads use a two-step process:
1. Call `/attachments/presign` to get an upload URL
2. Upload file directly to S3 using the presigned URL
3. Call `/attachments` with type=file to create the attachment record

---

### Update Attachment Request

**Proxy API**:
```json
{
  "type": "link",
  "category": "meeting_notes",
  "name": "Updated Meeting Notes",
  "description": "Updated description",
  "link": "https://docs.google.com/document/d/xyz789"
}
```

**ITX API**: Identical to the Proxy API request, plus a proxy-added `updated_by` field (same shape as `created_by` — see [Audit stamping](#audit-stamping)).

`type`, `category` and `name` are required on every update, and `type` must match the attachment's existing type. `link` is required for link-type attachments and `description` is optional. `occurrence_id` is replaced on every update: omit it to make the attachment apply to all occurrences, or send it to scope the attachment to that occurrence (see [Occurrence scope](#occurrence-scope)).

**Returns**: 204 No Content (no response body)

---

### Create Presigned URL Request

**Proxy API**:
```json
{
  "name": "Presentation.pptx",
  "file_size": 2048576,
  "file_type": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
  "description": "Q4 Planning Presentation",
  "category": "presentation"
}
```

**ITX API**: Identical to the Proxy API request, plus a proxy-added `created_by` field. The presign step is treated as a create because ITX persists an attachment record with `file_upload_status="ongoing"` at this point. See [Audit stamping](#audit-stamping).

**Fields**:
- `name` (string, required): File name with extension
- `file_size` (integer, required): File size in bytes
- `file_type` (string, required): MIME type of the file
- `description` (string, optional): Optional description
- `category` (string, optional): Attachment category
- `occurrence_id` (string, optional): Scope the attachment to one occurrence; omit for all occurrences. The presign step persists the attachment record, so the scope is set here (see [Occurrence scope](#occurrence-scope))

---

## Response Schemas

### Attachment Response

`occurrence_id` is present only when the attachment is scoped to a single occurrence; it is omitted for attachments that apply to all occurrences.

**Proxy API**:
```json
{
  "uid": "ea1e8536-a985-4cf5-b981-a170927a1d11",
  "meeting_id": "96365872728",
  "occurrence_id": "1788000000",
  "type": "file",
  "category": "meeting_notes",
  "name": "Meeting Notes",
  "description": "Notes from planning session",
  "file_uploaded": true,
  "file_name": "notes.pdf",
  "file_size": 204800,
  "file_url": "https://storage.example.com/...",
  "file_upload_status": "completed",
  "file_content_type": "application/pdf",
  "created_at": "2024-01-31T10:00:00Z",
  "created_by": {
    "username": "jdoe",
    "email": "john.doe@example.com",
    "name": "John Doe"
  },
  "updated_at": "2024-01-31T11:00:00Z",
  "updated_by": {
    "username": "jsmith",
    "email": "jane.smith@example.com",
    "name": "Jane Smith"
  }
}
```

**ITX API**:
```json
{
  "id": "ea1e8536-a985-4cf5-b981-a170927a1d11",
  "meeting_id": "96365872728",
  "type": "file",
  "category": "meeting_notes",
  "name": "Meeting Notes",
  "description": "Notes from planning session",
  "file_uploaded": true,
  "file_name": "notes.pdf",
  "file_size": 204800,
  "file_url": "https://storage.example.com/...",
  "file_upload_status": "completed",
  "content_type": "application/pdf",
  "created_at": "2024-01-31T10:00:00Z",
  "created_by": {
    "username": "jdoe",
    "email": "john.doe@example.com",
    "name": "John Doe"
  },
  "updated_at": "2024-01-31T11:00:00Z",
  "updated_by": {
    "username": "jsmith",
    "email": "jane.smith@example.com",
    "name": "Jane Smith"
  }
}
```

**Response Fields**:
- `uid`/`id` (string): UUID of the attachment - *Field name differs between APIs*
- `meeting_id` (string): Zoom meeting ID
- `type` (string): Type of attachment (`"file"` or `"link"`)
- `category` (string): Category of the attachment
- `name` (string): Name/title of the attachment
- `description` (string, optional): Description of the attachment
- `link` (string, optional): External URL (only for type=link)
- `file_uploaded` (boolean, optional): Whether file has been uploaded (omitted when false)
- `file_name` (string, optional): Original file name (only for type=file)
- `file_size` (integer, optional): File size in bytes (only for type=file)
- `file_url` (string, optional): Storage URL for the file (only for type=file)
- `file_upload_status` (string, optional): Upload status (only for type=file)
- `file_content_type`/`content_type` (string, optional): MIME type - *Field name differs between APIs*
- `created_at` (string): ISO 8601 timestamp
- `created_by` (object): User who created the attachment
- `updated_at` (string): ISO 8601 timestamp
- `updated_by` (object): User who last updated the attachment

---

### Presigned URL Response

`occurrence_id` is echoed back when the attachment is scoped to a single occurrence and omitted otherwise.

**Proxy API**:
```json
{
  "uid": "ea1e8536-a985-4cf5-b981-a170927a1d11",
  "meeting_id": "96365872728",
  "occurrence_id": "1788000000",
  "file_url": "https://storage.example.com/presigned-url?token=...",
  "type": "file",
  "category": "presentation",
  "name": "Presentation.pptx",
  "file_name": "Presentation.pptx",
  "file_size": 2048576,
  "file_upload_status": "pending",
  "file_content_type": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
  "created_at": "2024-01-31T10:00:00Z",
  "created_by": {
    "username": "jdoe",
    "email": "john.doe@example.com",
    "name": "John Doe"
  }
}
```

**ITX API**:
```json
{
  "id": "ea1e8536-a985-4cf5-b981-a170927a1d11",
  "meeting_id": "96365872728",
  "file_url": "https://storage.example.com/presigned-url?token=...",
  "type": "file",
  "category": "presentation",
  "name": "Presentation.pptx",
  "file_name": "Presentation.pptx",
  "file_size": 2048576,
  "file_upload_status": "pending",
  "content_type": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
  "created_at": "2024-01-31T10:00:00Z",
  "created_by": {
    "username": "jdoe",
    "email": "john.doe@example.com",
    "name": "John Doe"
  }
}
```

The `file_url` field contains the presigned URL where the client should upload the file using HTTP PUT.

---

### Download URL Response

**Proxy API & ITX API** (Identical):
```json
{
  "download_url": "https://storage.example.com/download?token=..."
}
```

The `download_url` is a time-limited presigned URL that can be used to download the file.

---

## Field Mapping

### Proxy API ↔ ITX API

| Proxy Field | ITX Field | Direction | Notes |
|-------------|-----------|-----------|-------|
| `uid` | `id` | Both | UUID identifier |
| `file_content_type` | `content_type` | Both | MIME type field name |

All other fields are identical between proxy and ITX APIs.

---

## Attachment Categories

The following attachment categories are supported:

| Category | Description |
|----------|-------------|
| `Meeting Minutes` | Official meeting minutes and notes |
| `Notes` | General notes or documentation |
| `Presentation` | Slide decks and presentations |
| `Other` | Any other type of attachment |

---

## Attachment Types

| Type | Description | Upload Method |
|------|-------------|---------------|
| `file` | File stored in object storage | Presigned URL → S3 upload → Create record |
| `link` | External URL reference | Direct creation with URL |

---

## File Upload Flow

For file attachments, use this three-step process:

1. **Get Presigned URL**:
   ```
   POST /itx/meetings/{meeting_id}/attachments/presign
   {
     "name": "document.pdf",
     "file_size": 204800,
     "file_type": "application/pdf",
     "category": "meeting_notes"
   }

   Response: { "file_url": "https://s3.amazonaws.com/...", "uid": "..." }
   ```

2. **Upload File to S3**:
   ```
   PUT https://s3.amazonaws.com/...
   Content-Type: application/pdf
   <binary file data>
   ```

3. **Create Attachment Record** (optional - record may be auto-created):
   ```
   POST /itx/meetings/{meeting_id}/attachments
   {
     "type": "file",
     "category": "meeting_notes",
     "name": "Meeting Notes"
   }
   ```

---

## Important Notes

1. **Presigned URL Upload**: Files are uploaded directly to S3 using presigned URLs, not through the API

2. **Type-Specific Fields**:
   - `link` type: requires `link` field, no file-related fields
   - `file` type: requires presigned URL flow, has file metadata fields

3. **File Upload Status**: Track upload status with `file_upload_status`:
   - `pending`: Presigned URL generated, file not yet uploaded
   - `uploading`: Upload in progress
   - `completed`: File successfully uploaded
   - `failed`: Upload failed

4. **Audit Trail**: All attachments track creator and updater information with timestamps

5. **UUID Identifiers**:
   - Proxy API uses `uid` field
   - ITX API uses `id` field
   - Same UUID value, different field name

6. **Update Returns 204**: PUT endpoint returns 204 No Content with no response body

7. **Occurrence Scope**: `occurrence_id` is passed through to ITX unchanged and is replaced on every update. See [Occurrence scope](#occurrence-scope)

8. **File Size Limits**: Check with ITX service for maximum file size limits

9. **Presigned URL Expiration**: Presigned URLs are time-limited (typically 15-60 minutes)

10. **Download URLs**: Download URLs are also time-limited and expire after a short period

11. **Content Type Field Name**:
    - Proxy API: `file_content_type`
    - ITX API: `content_type`
