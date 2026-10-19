# ITX Past Meeting Summaries API Contracts

This document details the API contracts for past meeting summaries (AI-generated meeting summaries) endpoints between the LFX v2 Meeting Service proxy API and the underlying ITX Zoom API.

## Audit stamping

The LFX Meeting Service proxy adds a `modified_by` field (ITX uses `modified_by` here, not the more common `updated_by`) to summary `PUT` requests so ITX records who edited or approved each summary. This field is populated by the proxy from the authenticated JWT principal — clients do not send it in their proxy request; if provided, it is overwritten.

Shape:

```json
{
  "username": "jdoe",
  "name": "Jane Doe",
  "email": "jane.doe@example.com",
  "profile_picture": "https://avatars.example.com/jdoe.jpg"
}
```

The name / email / avatar are resolved from the auth service (via a NATS lookup) using the JWT's `username` claim. If the lookup fails or NATS is unavailable, the proxy degrades gracefully to `{ username, email }` (email from the JWT claim) and never blocks the write. `created_by` is not stamped by the proxy — summary records are created by ITX from the Zoom AI Companion pipeline, not by proxy calls.

## Endpoints

### Get Past Meeting Summary

Retrieves a specific AI-generated summary by ID for a past meeting occurrence.

**Proxy Endpoint**: `GET /itx/past_meetings/{past_meeting_id}/summaries/{summary_id}`

**ITX Endpoint**: `GET /v2/zoom/past_meetings/{past_meeting}/summaries/{summary}`

**Path Parameters**:
- `past_meeting_id` (string, required): The hyphenated meeting and occurrence ID
- `summary_id` (string, required): UUID of the summary record

**Authorization**: Requires `ai_summary_viewer` on `v1_past_meeting:{past_meeting_id}` (checked by Heimdall). A summary awaiting approval (`requires_approval=true` and `approved!=true`) is returned only to principals holding `organizer` on the same past meeting, and only when the summary's `meeting_and_occurrence_id` matches `past_meeting_id`; the proxy checks this after fetching the summary and returns `403` to everyone else (`503` if the organizer check cannot be completed). This matches the search index, which also restricts summaries awaiting approval to organizers.

---

### Update Past Meeting Summary

Updates an existing AI-generated summary for a past meeting occurrence. This is typically used to edit the summary content or approve/reject it.

**Proxy Endpoint**: `PUT /itx/past_meetings/{past_meeting_id}/summaries/{summary_id}`

**ITX Endpoint**: `PUT /v2/zoom/past_meetings/{past_meeting}/summaries/{summary}`

**Path Parameters**:
- `past_meeting_id` (string, required): The hyphenated meeting and occurrence ID
- `summary_id` (string, required): UUID of the summary record

**Authorization**: Requires `organizer` permission on the meeting

---

## Request Schemas

### Update Summary Request

**Proxy API**: the edit is one `edited_content` string in the summary markdown (the same text the search index carries in `content`/`edited_content`, see [Summary markdown](#summary-markdown)), plus `approved`:

```json
{
  "edited_content": "## Overview\nThis meeting covered Q4 planning and resource allocation.\n\n## Key Topics\n### Key Discussion Points\nTeam discussed hiring plans and budget for Q4. Agreed on 3 new positions.\n\n### Decisions Made\nApproved budget increase of 15% for engineering team.\n\n## Next Steps\n- Finance team to draft budget proposal by end of week\n- HR to post job openings for approved positions\n- Schedule follow-up meeting for Q1 planning",
  "approved": true
}
```

**Optional Fields**:
- `edited_content` (string): the whole edited summary in the summary markdown. When present and non-empty it replaces all three edited fields in ITX; when absent, empty or whitespace-only, no edited field is sent (an approval-only update leaves the edits untouched).
- `approved` (boolean): Whether the summary has been approved for publication

**ITX API**: `edited_content` is parsed into ITX's three edited fields, plus a proxy-added `modified_by` field:

```json
{
  "edited_summary_overview": "This meeting covered Q4 planning and resource allocation.",
  "edited_summary_details": [
    {
      "label": "Key Discussion Points",
      "summary": "Team discussed hiring plans and budget for Q4. Agreed on 3 new positions."
    },
    {
      "label": "Decisions Made",
      "summary": "Approved budget increase of 15% for engineering team."
    }
  ],
  "edited_next_steps": [
    "Finance team to draft budget proposal by end of week",
    "HR to post job openings for approved positions",
    "Schedule follow-up meeting for Q1 planning"
  ],
  "approved": true,
  "modified_by": {
    "username": "jdoe",
    "name": "Jane Doe",
    "email": "jane.doe@example.com",
    "profile_picture": "https://avatars.example.com/jdoe.jpg"
  }
}
```

> **Proxy-added field**: `modified_by` is stamped by the proxy from the requesting user's authenticated JWT principal. See [Audit stamping](#audit-stamping).

**ITX fields**:
- `edited_summary_overview` (string): the `## Overview` section
- `edited_summary_details` (array): one item per `### <label>` section of `## Key Topics`, each with `label` (string) and `summary` (string)
- `edited_next_steps` (array of strings): the `- <step>` lines of `## Next Steps`
- `approved` (boolean): forwarded as sent

On an edit the proxy sends **all three** edited fields, with `""` for a removed overview and `[]` for a removed section, so that ITX replaces the whole edited summary (the v1 UI sends the same shape). On an approval-only update it sends **none** of them.

---

### Summary markdown

`content`, `edited_content` (proxy response) and `edited_content` (proxy request) all carry the summary parts in one line-oriented markdown format, rendered and parsed by `pkg/summaryformat` and shared with the index writer (`docs/indexer-contract.md`, `v1_past_meeting_summary`), so an edit made on the displayed text parses back into the structured fields ITX holds:

```markdown
## Overview
<overview>

## Key Topics
### <label>
<summary>

## Next Steps
- <step>
```

- Each section appears only when it has content, in that order; a label-less detail is its summary alone under `## Key Topics`.
- Parsing recognises exactly `## Overview`, `## Key Topics` and `## Next Steps` (case-sensitive, surrounding whitespace ignored) as section switches; text before any heading is the overview; a `### ` line starts a new detail; each non-blank line under `## Next Steps` is one step, with a leading `- `, `* ` or `• ` removed. Any other line is content of its section, verbatim.
- Limits: a step is one line; free text that puts a section heading or a `### ` line on its own line is read as structure.

---

## Response Schemas

### Summary Response

**Proxy API**: the `PastMeetingSummary` shape of the OpenAPI spec (`uid`, `past_meeting_id`, `meeting_id`, `platform`, `zoom_config`, `summary_data`, `requires_approval`, `approved`, `email_sent`, `created_at`, `updated_at`). `summary_data.content` renders ITX's `summary_overview`, `summary_details` and `next_steps` in the [summary markdown](#summary-markdown); `summary_data.edited_content` renders the three `edited_*` fields the same way. Either is omitted when its parts are all empty.

```json
{
  "uid": "ea1e8536-a985-4cf5-b981-a170927a1d11",
  "past_meeting_id": "12343245463-1630560600000",
  "meeting_id": "12343245463",
  "platform": "Zoom",
  "zoom_config": {
    "meeting_id": "12343245463",
    "meeting_uuid": "fb2f9647-b096-5dg6-c092-b281938b2e22"
  },
  "summary_data": {
    "start_time": "2021-06-27T05:30:00Z",
    "end_time": "2021-06-27T06:30:00Z",
    "title": "Q4 Planning Meeting",
    "content": "## Overview\nThe team discussed Q4 planning, resource allocation, and hiring needs.\n\n## Key Topics\n### Discussion Topics\nQ4 goals, budget allocation, team expansion plans\n\n### Decisions\nApproved 15% budget increase and 3 new positions\n\n## Next Steps\n- Finance to draft budget proposal\n- HR to post job openings\n- Schedule Q1 planning meeting",
    "edited_content": "## Overview\nThis meeting covered Q4 planning and resource allocation.\n\n## Key Topics\n### Key Discussion Points\nTeam discussed hiring plans and budget for Q4. Agreed on 3 new positions.\n\n## Next Steps\n- Finance team to draft budget proposal by end of week\n- HR to post job openings for approved positions"
  },
  "requires_approval": true,
  "approved": false,
  "email_sent": false,
  "created_at": "2021-06-27T05:45:00Z",
  "updated_at": "2021-06-27T06:15:00Z"
}
```

**ITX API**:
```json
{
  "id": "ea1e8536-a985-4cf5-b981-a170927a1d11",
  "meeting_and_occurrence_id": "12343245463-1630560600000",
  "meeting_id": "12343245463",
  "occurrence_id": "1630560600000",
  "zoom_meeting_uuid": "fb2f9647-b096-5dg6-c092-b281938b2e22",
  "summary_created_time": "2021-06-27T05:45:00Z",
  "summary_last_modified_time": "2021-06-27T06:15:00Z",
  "summary_start_time": "2021-06-27T05:30:00Z",
  "summary_end_time": "2021-06-27T06:30:00Z",
  "summary_title": "Q4 Planning Meeting",
  "summary_overview": "The team discussed Q4 planning, resource allocation, and hiring needs.",
  "summary_details": [
    {
      "label": "Discussion Topics",
      "summary": "Q4 goals, budget allocation, team expansion plans"
    },
    {
      "label": "Decisions",
      "summary": "Approved 15% budget increase and 3 new positions"
    }
  ],
  "next_steps": [
    "Finance to draft budget proposal",
    "HR to post job openings",
    "Schedule Q1 planning meeting"
  ],
  "edited_summary_overview": "This meeting covered Q4 planning and resource allocation.",
  "edited_summary_details": [
    {
      "label": "Key Discussion Points",
      "summary": "Team discussed hiring plans and budget for Q4. Agreed on 3 new positions."
    }
  ],
  "edited_next_steps": [
    "Finance team to draft budget proposal by end of week",
    "HR to post job openings for approved positions"
  ],
  "requires_approval": true,
  "approved": false,
  "created_at": "2021-06-27T05:45:00Z",
  "created_by": {
    "id": "system",
    "username": "zoom-ai",
    "name": "Zoom AI",
    "email": "noreply@zoom.us"
  },
  "modified_at": "2021-06-27T06:15:00Z",
  "modified_by": {
    "id": "user-uuid-456",
    "username": "jsmith",
    "name": "Jane Smith",
    "email": "jane.smith@example.com"
  }
}
```

**ITX Response Fields**:
- `id` (string): UUID of the summary record (read-only)
- `meeting_and_occurrence_id` (string): Hyphenated meeting and occurrence ID (read-only)
- `meeting_id` (string): Meeting ID (read-only)
- `occurrence_id` (string): Occurrence ID (read-only)
- `zoom_meeting_uuid` (string): Zoom's internal meeting UUID (read-only)
- `summary_created_time` (string, RFC3339): When Zoom AI created the summary (read-only)
- `summary_last_modified_time` (string, RFC3339): When Zoom AI last updated the summary (read-only)
- `summary_start_time` (string, RFC3339): Start time of the meeting session that was summarized (read-only)
- `summary_end_time` (string, RFC3339): End time of the meeting session that was summarized (read-only)
- `summary_title` (string): AI-generated title (read-only)
- `summary_overview` (string): AI-generated overview (read-only)
- `summary_details` (array): AI-generated detail sections (read-only)
  - Each item has `label` (string) and `summary` (string)
- `next_steps` (array of strings): AI-generated next steps (read-only)
- `edited_summary_overview` (string): Human-edited version of the overview
- `edited_summary_details` (array): Human-edited version of the detail sections
- `edited_next_steps` (array of strings): Human-edited version of the next steps
- `requires_approval` (boolean): Whether this summary requires approval before being public
- `approved` (boolean): Whether this summary has been approved
- `created_at` (string, RFC3339): When the summary record was created in ITX (read-only)
- `created_by` (object): User/system that created the record (read-only)
- `modified_at` (string, RFC3339): When the summary was last modified (read-only)
- `modified_by` (object): User who last modified the summary (read-only)

---

## Summary Details Object

**Structure**:
```json
{
  "label": "Discussion Topics",
  "summary": "Team discussed Q4 goals, budget allocation, and hiring plans."
}
```

**Fields**:
- `label` (string): Section heading/label
- `summary` (string): Content for this section

---

## Field Mapping

**Request** (proxy → ITX): `edited_content` → `edited_summary_overview` + `edited_summary_details` + `edited_next_steps`, parsed from the [summary markdown](#summary-markdown); all three are sent on an edit (`""`/`[]` for removed parts) and none on an approval-only update. `approved` passes through. `modified_by` is added by the proxy.

**Response** (ITX → proxy):

| Proxy field | ITX field(s) |
|---|---|
| `uid` | `id` |
| `past_meeting_id` | `meeting_and_occurrence_id` |
| `meeting_id` | `meeting_id` |
| `platform` | always `"Zoom"` |
| `zoom_config.meeting_id`, `zoom_config.meeting_uuid` | `meeting_id`, `zoom_meeting_uuid` (object omitted when both are empty) |
| `summary_data.start_time`, `summary_data.end_time`, `summary_data.title` | `summary_start_time`, `summary_end_time`, `summary_title` |
| `summary_data.content` | `summary_overview` + `summary_details` + `next_steps`, rendered as summary markdown |
| `summary_data.edited_content` | `edited_summary_overview` + `edited_summary_details` + `edited_next_steps`, rendered as summary markdown |
| `requires_approval`, `approved` | `requires_approval`, `approved` |
| `email_sent` | not tracked by ITX; always `false` |
| `created_at`, `updated_at` | `created_at`, `modified_at` |

---

## Important Notes

1. **AI-Generated Content**: Summaries are automatically created by Zoom's AI Companion feature when enabled for a meeting. The original AI-generated content is stored in read-only fields (`summary_overview`, `summary_details`, `next_steps`).

2. **Human Editing**: The `edited_*` fields allow meeting organizers to review and edit the AI-generated content. These edited versions are what get displayed to users when approved.

3. **Approval Workflow**:
   - If `requires_approval=true`, the summary must be explicitly approved (`approved=true`) before being publicly visible
   - If `requires_approval=false`, the summary is automatically available without approval
   - This setting is determined by the meeting's `require_ai_summary_approval` configuration

4. **Access Control**: Summary visibility is controlled by the meeting's `ai_summary_access` setting:
   - `meeting_hosts`: Only meeting hosts can view the summary
   - `meeting_participants`: Only meeting participants (invitees/attendees) can view
   - `public`: Anyone can view the summary (if approved)

5. **Read-Only AI Fields**: The original AI-generated fields cannot be modified:
   - `summary_title`
   - `summary_overview`
   - `summary_details`
   - `next_steps`
   - `summary_created_time`
   - `summary_last_modified_time`
   - `summary_start_time`
   - `summary_end_time`

6. **Editable Fields**: Only these fields can be updated via the API: `edited_content` (the proxy's single edit field, mapped to ITX's `edited_summary_overview`, `edited_summary_details` and `edited_next_steps`) and `approved`.

7. **Multiple Summaries**: A single meeting occurrence can have multiple summary records if:
   - The meeting was very long and Zoom split it into multiple summary sessions
   - The AI generated summaries at different times during the meeting
   - Multiple summary versions were created for different purposes

8. **Zoom Meeting UUID**: The `zoom_meeting_uuid` is Zoom's internal identifier for the meeting instance, which is different from our `meeting_id` and `occurrence_id`.

9. **Creation Metadata**: The `created_by` field typically shows "Zoom AI" or similar system identifier since summaries are auto-generated, not manually created.

10. **Modification Tracking**: The `modified_by` field tracks who last edited the summary content or changed its approval status.

11. **Timestamp Precision**: All timestamp fields use RFC3339 format with timezone information for accurate time tracking across different timezones.

12. **Summary Availability**: Summaries are only available if:
    - The meeting had Zoom AI Companion enabled
    - The meeting actually occurred (not a future meeting)
    - Zoom successfully generated the summary (may take time after meeting ends)
    - The meeting's recording/transcript settings allow AI processing

13. **Privacy Considerations**: AI summaries may contain sensitive meeting content. The approval workflow and access controls help organizations maintain proper data governance.
