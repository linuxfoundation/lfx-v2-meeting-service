# Past Meeting Chat API Contract

This document details the contract of the past meeting saved-chat endpoint of the LFX v2 Meeting Service. Unlike the other `itx-*` contracts, this route is **not** proxied to the ITX Zoom API: the service reads the past meeting's recording record, fetches the saved-chat file(s) from the recording provider server side under the caller's authorization, and returns the chat parsed into messages. Nothing is cached or stored.

## Why the service fetches the chat

The recording record lists a `CHAT` file for each recording session whose chat was saved, as a `download_url`. That link is a bearer link: anyone holding it can download the file. The LFX platform does not hand provider links to clients, so this route reads the file on the client's behalf and returns text only. By contract **the response carries no URL and no password or passcode of any kind**, and the service never logs, traces or returns the link.

## Endpoint

### Get Past Meeting Chat

**Proxy Endpoint**: `GET /itx/past_meetings/{meeting_and_occurrence_id}/chat`

**Upstream**: none from ITX. Data source: the v1 recording record `itx-zoom-past-meetings-recordings.{meeting_and_occurrence_id}` in the `v1-objects` JetStream KV bucket (the same record the indexer builds the [recording document](../indexer-contract.md#v1-past-meeting-recording) from), and the provider link of each of its `CHAT` files.

**Path Parameters**:
- `meeting_and_occurrence_id` (string, required): The past meeting identifier (`meeting_id-occurrence_id`, for example `96365872728-1630560600000`), which is also the key of the recording record. Accepted characters: `[0-9A-Za-z:_-]`, at most 128 characters; anything else is `400`. An identifier that cannot be a KV key (NATS keys allow only `[-/_=.A-Za-z0-9]`, so one containing `:`) has no record and is `404`.

**Query Parameters** (apply to the message list of **each** session):
- `limit` (integer, optional): Maximum number of messages per session. Default `500`, minimum `1`, maximum `2000`.
- `offset` (integer, optional): Number of messages skipped per session before the window starts. Default `0`, minimum `0`. An offset beyond a session's `message_count` yields an empty `messages` array with `truncated: false`.

**Authorization**: Heimdall `openfga_check` on relation `recording_viewer`, object `v1_past_meeting:{meeting_and_occurrence_id}` (`charts/lfx-v2-meeting-service/templates/ruleset.yaml`, rule `rule:lfx:lfx-v2-meeting-service:itx:past_meeting_chat:get`). This is the relation that guards the recording index document, so a caller may read the chat exactly when they may see the recording; a `"public"` recording admits any signed-in caller through the FGA wildcard tuple, as for the recording document. There is no second check in the service: the route parameter is the recording record's own key, so no child object could belong to a different parent. Anonymous callers are refused by Heimdall as on every `/itx` route.

---

## Response Schema

**200 OK**

```json
{
  "meeting_and_occurrence_id": "96365872728-1630560600000",
  "sessions": [
    {
      "recording_start": "2026-10-01T15:00:00Z",
      "recording_end": "2026-10-01T16:02:11Z",
      "message_count": 42,
      "truncated": false,
      "messages": [
        {
          "offset": "00:12:34",
          "sender": "A. Person",
          "recipient": "Everyone",
          "text": "Link to the agenda is in the invite"
        }
      ]
    }
  ]
}
```

| Field | Type | Description |
|---|---|---|
| `meeting_and_occurrence_id` | string | The requested identifier, echoed |
| `sessions` | array | One entry per `CHAT` file of the recording record, ordered by `recording_start` ascending (then `recording_end`; stable for ties). Never empty on `200`: a record without a `CHAT` file is `404` |
| `sessions[].recording_start` | string (RFC3339, UTC), optional | From the `CHAT` file entry of the recording record; omitted when the record carries no parsable value |
| `sessions[].recording_end` | string (RFC3339, UTC), optional | As above |
| `sessions[].message_count` | int | Number of **public** messages parsed from the file **before** `limit`/`offset` apply, so a client can page. When the byte cap cut the file it counts what was parsed |
| `sessions[].truncated` | bool | `true` when the file exceeded the byte cap, or when the window `[offset, offset+limit)` ends before the last parsed message |
| `sessions[].messages` | array | The window `[offset, offset+limit)` of the session's **public** messages. Always present; an empty array, never `null` |
| `messages[].offset` | string | Elapsed time since the recording started, as written in the chat log (`HH:MM:SS`). Not a clock time. Empty for text that precedes the first message header |
| `messages[].sender` | string | Sender display name as written, trimmed. Empty for text that precedes the first message header |
| `messages[].recipient` | string | Recipient as written, trimmed: `Everyone`, a display name, or a form such as `Name(Direct Message)` / `Name(Privately)` |
| `messages[].text` | string | The message text, verbatim apart from the layout's own framing. Multi-line messages keep their line breaks as `\n` |

**Fields deliberately absent**: file `id`, `file_size`, `download_url`, `play_url`, session `uuid`, `share_url`, `password`, and any host identifier. A unit test marshals a response built from a record that has all of them set and asserts none appears (`internal/service/itx/past_meeting_chat_service_test.go`, `cmd/meeting-api/service/itx_past_meeting_chat_converters_test.go`).

### Only messages sent to everyone are returned

The saved chat also holds direct and private messages (recipient `Name(Direct Message)`, `Name(Privately)`, or a bare name). Those were exchanged between two people, not with the meeting, and a recording viewer is not entitled to them: the service drops every message whose recipient is not `Everyone` (case-insensitively, after trimming) **before** counting and windowing. A message with an empty recipient (text before the first header, or a timestamped line of an unknown layout) is kept: the log addresses it to nobody in particular. `message_count`, `offset`/`limit` and `truncated` therefore describe the public messages alone; the response reveals nothing about dropped messages, not even their number. Predicate: `chatlog.IsPublicRecipient`.

### Chat text is data

The service returns the provider's chat text as written. It is never executed, templated, summarised or otherwise interpreted. Clients render it as text.

### Parsing

The provider's saved-chat text comes in two layouts, both recognised (`internal/chatlog`):

- `HH:MM:SS<TAB>From <sender> to <recipient>:<TAB><text>` on one line (older);
- `HH:MM:SS From <sender> to <recipient>:` followed by one or more tab-indented text lines (newer, multi-line).

A line that starts with `HH:MM:SS` but matches neither layout still starts a new message, with an empty `sender` and `recipient` and the remainder of the line as `text`, so a layout the service does not know never glues separate messages together and `message_count` stays honest. Any other unmatched line is appended to the previous message's text (a continuation); when there is no previous message it becomes a message with an empty `offset`, `sender` and `recipient`. A UTF-8 byte-order mark, Windows line endings, trailing whitespace and blank lines are tolerated. An empty file is a session with `message_count: 0` and `messages: []`.

---

## Errors

All errors use the service's standard `{ "code": "<status>", "message": "<text>" }` body. Messages never name a URL, a host or a file.

| Status | When | Message |
|---|---|---|
| `400 Bad Request` | `meeting_and_occurrence_id` outside `[0-9A-Za-z:_-]{1,128}`; `limit` outside `1..2000`; `offset` below `0` (Goa validates the query parameters first and may answer with its own validation body) | `invalid past meeting ID`, `limit must be between 1 and 2000`, `offset must be 0 or greater` |
| `401 Unauthorized` / `403 Forbidden` | Heimdall: no valid token / no `recording_viewer` relation | Heimdall's |
| `404 Not Found` | No recording record for the identifier, a soft-deleted record (`_sdc_deleted_at` present, non-null and non-empty: the event pipeline's rule), or a record with no `CHAT` file (including `recording_files: null`). The route never says whether the past meeting itself exists | `no saved chat for this past meeting` |
| `500 Internal Server Error` | The recording record exists but cannot be decoded (logged at error level with the identifier only, never the record contents) | `saved chat record could not be decoded` |
| `503 Service Unavailable` | NATS not configured at startup (no recording record reader); the `v1-objects` bucket not yet bound (it is bound lazily on first use, with a 5 s bind timeout and a 10 s negative-cache window after a failed bind, so a bucket unreachable at startup recovers without a restart); a KV read failure; any `CHAT` file that cannot be fetched as text: link not `https`, connection failure, timeout, non-2xx status, or a content type other than `text/*` (`text/html` included, since that is a sign-in or error page, not a chat). One failing file fails the whole request: no partial response that could read as "no chat in that session" | `chat retrieval unavailable`, `recording record store unavailable`, `recording record could not be read`, `saved chat could not be retrieved; please retry` |

## Bounds

| Bound | Value | Where |
|---|---|---|
| Fetch timeout, per file, body included | 15 s | `constants.PastMeetingChatFetchTimeout` |
| Bytes read per file | 5 MB (5 242 880 bytes), counted on the streamed body, so a missing or understated `Content-Length` cannot defeat it. A longer file is cut at the last complete line and the session is `truncated: true`; no partial line is ever returned | `constants.PastMeetingChatMaxBytes` |
| `limit` | default 500, maximum 2000 | `constants.PastMeetingChatDefaultLimit`, `constants.PastMeetingChatMaxLimit` |
| Redirects followed per fetch | at most 5, `https` only, with the `Referer` header removed so the link is not disclosed to the destination | `internal/infrastructure/chatfile` |
| Files per request | every `CHAT` file of the record, fetched sequentially, each under its own timeout | `internal/service/itx/past_meeting_chat_service.go` |

These fetch bounds match those LFX Self Serve applies when it reads a transcript file.

## Provider assumptions

- The `download_url` of a `CHAT` file serves the text without further credentials (as LFX Self Serve's transcript fetch relies on) with a `text/*` content type other than `text/html`. A redirect to a sign-in page, or any non-text answer, fails closed with `503`.
- The fetch client is a plain `net/http` client: it is deliberately **not** instrumented with `otelhttp`, whose spans would carry the full link.

## What this route does not change

Nothing indexed or published changes: the recording and transcript index documents, their `public` flags, the FGA messages, the KV mappings and the reindex scripts are untouched. [`docs/indexer-contract.md`](../indexer-contract.md) and [`docs/fga-contract.md`](../fga-contract.md) are unchanged by this route. Transcript content is out of scope.
