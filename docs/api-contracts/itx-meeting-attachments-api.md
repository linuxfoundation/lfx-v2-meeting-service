# ITX Meeting Attachments API Contracts

This document details the API contracts for meeting attachments endpoints between the LFX v2 Meeting Service proxy API and the underlying ITX Zoom API.

## Endpoints

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

For external URL references (Google Docs, SharePoint, etc.):

**Proxy API & ITX API** (Identical):
```json
{
  "type": "link",
  "category": "meeting_agenda",
  "name": "Q4 Planning Agenda",
  "description": "Agenda for Q4 planning meeting",
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
  "category": "meeting_notes",
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
  "category": "meeting_notes",
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
  "name": "Presentation.pptx",
  "file_size": 2048576,
  "file_type": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
  "description": "Q4 Planning Presentation",
  "category": "presentation"
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

**Proxy API**:
```json
{
  "uid": "ea1e8536-a985-4cf5-b981-a170927a1d11",
  "meeting_id": "96365872728",
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

Common attachment categories include:

| Category | Description |
|----------|-------------|
| `meeting_agenda` | Meeting agenda documents |
| `meeting_notes` | Meeting notes and minutes |
| `presentation` | Slide decks and presentations |
| `document` | General documents |
| `spreadsheet` | Spreadsheets and data files |
| `other` | Other types of attachments |

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

7. **File Size Limits**: Check with ITX service for maximum file size limits

8. **Presigned URL Expiration**: Presigned URLs are time-limited (typically 15-60 minutes)

9. **Download URLs**: Download URLs are also time-limited and expire after a short period

10. **Content Type Field Name**:
    - Proxy API: `file_content_type`
    - ITX API: `content_type`
