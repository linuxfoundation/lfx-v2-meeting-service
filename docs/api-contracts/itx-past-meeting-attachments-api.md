# ITX Past Meeting Attachments API Contracts

This document details the API contracts for past meeting attachments endpoints between the LFX v2 Meeting Service proxy API and the underlying ITX Zoom API.

## Endpoints

### Create Past Meeting Attachment

Creates a new attachment record for a past meeting. Supports both file attachments (via presigned URL) and external links.

**Proxy Endpoint**: `POST /itx/past_meetings/{meeting_and_occurrence_id}/attachments`

**ITX Endpoint**: `POST /v2/zoom/past_meetings/{meeting_and_occurrence_id}/attachments`

**Path Parameters**:
- `meeting_and_occurrence_id` (string, required): The hyphenated meeting and occurrence ID (e.g., "12343245463-1630560600000")

**Authorization**: Requires `organizer` permission on the meeting

---

### Get Past Meeting Attachment

Retrieves metadata for a specific past meeting attachment.

**Proxy Endpoint**: `GET /itx/past_meetings/{meeting_and_occurrence_id}/attachments/{attachment_id}`

**ITX Endpoint**: `GET /v2/zoom/past_meetings/{meeting_and_occurrence_id}/attachments/{attachment_id}`

**Path Parameters**:
- `meeting_and_occurrence_id` (string, required): The hyphenated meeting and occurrence ID
- `attachment_id` (string, required): UUID of the attachment

**Authorization**: Requires `viewer` permission on the meeting

---

### Update Past Meeting Attachment

Updates an existing past meeting attachment's metadata.

**Proxy Endpoint**: `PUT /itx/past_meetings/{meeting_and_occurrence_id}/attachments/{attachment_id}`

**ITX Endpoint**: `PUT /v2/zoom/past_meetings/{meeting_and_occurrence_id}/attachments/{attachment_id}`

**Path Parameters**:
- `meeting_and_occurrence_id` (string, required): The hyphenated meeting and occurrence ID
- `attachment_id` (string, required): UUID of the attachment

**Authorization**: Requires `organizer` permission on the meeting

**Returns**: 204 No Content on success

---

### Delete Past Meeting Attachment

Deletes an attachment from a past meeting.

**Proxy Endpoint**: `DELETE /itx/past_meetings/{meeting_and_occurrence_id}/attachments/{attachment_id}`

**ITX Endpoint**: `DELETE /v2/zoom/past_meetings/{meeting_and_occurrence_id}/attachments/{attachment_id}`

**Path Parameters**:
- `meeting_and_occurrence_id` (string, required): The hyphenated meeting and occurrence ID
- `attachment_id` (string, required): UUID of the attachment

**Authorization**: Requires `organizer` permission on the meeting

---

### Create Presigned Upload URL

Generates a presigned URL for uploading a file attachment directly to object storage (S3).

**Proxy Endpoint**: `POST /itx/past_meetings/{meeting_and_occurrence_id}/attachments/presign`

**ITX Endpoint**: `POST /v2/zoom/past_meetings/{meeting_and_occurrence_id}/attachments/presign`

**Path Parameters**:
- `meeting_and_occurrence_id` (string, required): The hyphenated meeting and occurrence ID

**Authorization**: Requires `organizer` permission on the meeting

---

### Get Attachment Download URL

Generates a presigned download URL for a file attachment.

**Proxy Endpoint**: `GET /itx/past_meetings/{meeting_and_occurrence_id}/attachments/{attachment_id}/download`

**ITX Endpoint**: `GET /v2/zoom/past_meetings/{meeting_and_occurrence_id}/attachments/{attachment_id}/download`

**Path Parameters**:
- `meeting_and_occurrence_id` (string, required): The hyphenated meeting and occurrence ID
- `attachment_id` (string, required): UUID of the attachment

**Authorization**: Requires `viewer` permission on the meeting

---

## Request Schemas

### Create Attachment Request (Link Type)

For external URL references (Google Docs, SharePoint, etc.):

**Proxy API & ITX API** (Identical):
```json
{
  "type": "link",
  "category": "Notes",
  "name": "Q4 Planning Notes",
  "description": "Notes from Q4 planning meeting",
  "link": "https://docs.google.com/document/d/abc123"
}
```

**Fields**:
- `type` (string, required): Must be `"link"` for external URLs
- `category` (string, required): Category of the attachment
- `name` (string, required): Name/title of the attachment (1-255 chars)
- `description` (string, optional): Optional description (max 500 chars)
- `link` (string, required for type=link): URL to external resource (max 2048 chars)

---

### Create Attachment Request (File Type)

For file uploads using presigned URL flow:

**Proxy API & ITX API** (Identical):
```json
{
  "type": "file",
  "category": "Notes",
  "name": "Meeting Notes"
}
```

**Fields**:
- `type` (string, required): Must be `"file"` for file attachments
- `category` (string, required): Category of the attachment
- `name` (string, required): Name/title of the attachment (1-255 chars)
- `description` (string, optional): Optional description (max 500 chars)

**Note**: File uploads use a two-step process:
1. Call `/attachments/presign` to get an upload URL
2. Upload file directly to S3 using the presigned URL
3. Call `/attachments` with type=file to create the attachment record

---

### Update Attachment Request

**Proxy API & ITX API** (Identical):
```json
{
  "type": "link",
  "category": "Notes",
  "name": "Updated Meeting Notes",
  "description": "Updated description",
  "link": "https://docs.google.com/document/d/xyz789"
}
```

All fields are optional - only include fields you want to update.

**Returns**: 204 No Content (no response body)

---

### Create Presigned URL Request

**Proxy API & ITX API** (Identical):
```json
{
  "name": "Recording.mp4",
  "file_size": 52428800,
  "file_type": "video/mp4",
  "description": "Q4 Planning Meeting Recording",
  "category": "Other"
}
```

**Fields**:
- `name` (string, required): File name with extension
- `file_size` (integer, required): File size in bytes
- `file_type` (string, required): MIME type of the file
- `description` (string, optional): Optional description
- `category` (string, optional): Attachment category

---

## Response Schemas

### Attachment Response

**Proxy API**:
```json
{
  "uid": "ea1e8536-a985-4cf5-b981-a170927a1d11",
  "meeting_and_occurrence_id": "96365872728-1630560600000",
  "type": "file",
  "category": "Notes",
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
  "meeting_and_occurrence_id": "96365872728-1630560600000",
  "type": "file",
  "category": "Notes",
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
- `meeting_and_occurrence_id` (string): Hyphenated meeting and occurrence ID
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

**Proxy API**:
```json
{
  "uid": "ea1e8536-a985-4cf5-b981-a170927a1d11",
  "meeting_and_occurrence_id": "96365872728-1630560600000",
  "file_url": "https://storage.example.com/presigned-url?token=...",
  "type": "file",
  "category": "Other",
  "name": "Recording.mp4",
  "file_name": "Recording.mp4",
  "file_size": 52428800,
  "file_upload_status": "pending",
  "file_content_type": "video/mp4",
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
  "meeting_and_occurrence_id": "96365872728-1630560600000",
  "file_url": "https://storage.example.com/presigned-url?token=...",
  "type": "file",
  "category": "Other",
  "name": "Recording.mp4",
  "file_name": "Recording.mp4",
  "file_size": 52428800,
  "file_upload_status": "pending",
  "content_type": "video/mp4",
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
   POST /itx/past_meetings/{meeting_and_occurrence_id}/attachments/presign
   {
     "name": "recording.mp4",
     "file_size": 52428800,
     "file_type": "video/mp4",
     "category": "Other"
   }

   Response: { "file_url": "https://s3.amazonaws.com/...", "uid": "..." }
   ```

2. **Upload File to S3**:
   ```
   PUT https://s3.amazonaws.com/...
   Content-Type: video/mp4
   <binary file data>
   ```

3. **Create Attachment Record** (optional - record may be auto-created):
   ```
   POST /itx/past_meetings/{meeting_and_occurrence_id}/attachments
   {
     "type": "file",
     "category": "Other",
     "name": "Meeting Recording"
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

7. **File Size Limits**: Check with ITX service for maximum file size limits

8. **Presigned URL Expiration**: Presigned URLs are time-limited (typically 15-60 minutes)

9. **Download URLs**: Download URLs are also time-limited and expire after a short period

10. **Content Type Field Name**:
    - Proxy API: `file_content_type`
    - ITX API: `content_type`

11. **Past Meeting Identification**: Attachments are always associated with a specific past meeting occurrence using the hyphenated `meeting_and_occurrence_id` format (e.g., "12343245463-1630560600000")
