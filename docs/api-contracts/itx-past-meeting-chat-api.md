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

**Authorization**: the route requires a signed-in caller. Heimdall authenticates with `oidc` only (this rule has no `anonymous_authenticator` fallback, unlike the chart's other rules) and then runs `openfga_check` on relation `recording_viewer`, object `v1_past_meeting:{meeting_and_occurrence_id}` (`charts/lfx-v2-meeting-service/templates/ruleset.yaml`, rule `rule:lfx:lfx-v2-meeting-service:itx:past_meeting_chat:get`). This is the relation that guards the recording index document, so a signed-in caller may read the chat exactly when they may see the recording; for a `"public"` recording that means any signed-in user (the FGA wildcard tuple), never an anonymous one. There is no second check in the service: the route parameter is the recording record's own key, so no child object could belong to a different parent.

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
| `messages[].offset` | string | Elapsed time since the recording started, as written in the chat log (`HH:MM:SS`). Not a clock time |
| `messages[].sender` | string | Sender display name as written, trimmed |
| `messages[].recipient` | string | Recipient as written, trimmed. Always `Everyone` (in the provider's casing), since only messages sent to everyone are returned; in the name header layout, which writes no recipient, the parser supplies it |
| `messages[].text` | string | The message text, verbatim apart from the layout's own framing. Multi-line messages keep their line breaks as `\n` |

**Fields deliberately absent**: file `id`, `file_size`, `download_url`, `play_url`, session `uuid`, `share_url`, `password`, and any host identifier. A unit test marshals a response built from a record that has all of them set and asserts none appears (`internal/service/itx/past_meeting_chat_service_test.go`, `cmd/meeting-api/service/itx_past_meeting_chat_converters_test.go`).

### Only messages sent to everyone are returned

The saved chat may hold direct and private messages (recipient `Name(Direct Message)`, `Name(Privately)`, or a bare name). Those were exchanged between two people, not with the meeting, and a recording viewer is not entitled to them. **Only messages explicitly sent to everyone are returned**: the service keeps a message only when its recipient is `Everyone` (case-insensitively, after trimming) and drops every other message **before** counting and windowing. Text the parser cannot attribute to a recipient (text before the first header, or a timestamped line in a layout the parser does not know) is dropped, never returned: such a line may be a direct message in a drifted layout, so the route fails closed. Continuation lines are unaffected; they belong to the message above them and share its recipient, so a private message's continuation lines are dropped with it. `message_count`, `offset`/`limit` and `truncated` therefore describe the public messages alone; the response reveals nothing about dropped messages, not even their number. Predicate: `chatlog.IsPublicRecipient`.

**The name header layout writes no recipient** (see [Parsing](#parsing)), so the parser derives one from the header (`chatlog.classifyNameHeader`), and `IsPublicRecipient` stays the one rule:

- a header containing ` to ` (spaces on both sides) is read as *sender* to *recipient*, as in the older layouts, and the recipient is judged like any other: `Alice Example to Bob Fixture` is private, `Alice Example to Everyone` public;
- a header containing `(Direct Message)` or `(Privately)` (case-insensitively) is private: the sender is the whole header and the recipient the marker, which `IsPublicRecipient` refuses;
- any other header is a plain display name and the message is **classified as sent to everyone** (`recipient: "Everyone"`).

The plain-name rule rests on the provider's documented behaviour. Its support article **"Saving in-meeting chat"** (Zoom Support, article `KB0067312`, `https://support.zoom.com/hc/en/article?id=zm_kb&sysparm_article=KB0067312`) states: *"If you save the chat to the cloud, it will only save chats that were sent to everyone and messages sent while you were cloud recording."* It contrasts this with a local save, which *"will save any chats that you can see—those sent directly to you and those sent to everyone in the meeting or webinar."* The `CHAT` recording file this route reads is the cloud-saved chat, so a message it holds under a plain name was sent to everyone; the ` to ` and marker rules above are kept for defence in depth, should the provider ever write a private message with a marker. The same article notes that a **webinar** recording may also save the messages panelists sent to all panelists when *"Save panelist chat to the recording"* is enabled; this route serves meeting recordings. Should the premise fail (a private message observed unmarked in a cloud-saved chat), the rule flips to fail closed with a one-line change, the constant `recipientlessNameHeaderIsPublic` in `internal/chatlog/chatlog.go`: a plain-name header then gets an empty recipient, every such message is dropped, and the route returns no messages on this layout until the provider marks private messages.

### Chat text is data

The service returns the provider's chat text as written. It is never executed, templated, summarised or otherwise interpreted. Clients render it as text.

### Parsing

The provider's saved-chat text comes in three layouts, all recognised (`internal/chatlog`), tried in this order:

- `HH:MM:SS<TAB>From <sender> to <recipient>:<TAB><text>` on one line (older);
- `HH:MM:SS From <sender> to <recipient>:` followed by one or more tab-indented text lines (newer, multi-line);
- `HH:MM:SS<TAB><name>:<TAB><text>` (the layout the provider writes today): the header is the sender's display name alone, as written, which may contain spaces, parentheses (an affiliation such as `Name (Example Foundation, Inc.)`), punctuation, `:` and non-ASCII; the header ends at the last `:` before the tab (or the end of the line, when the text is empty). No recipient is written: see [Only messages sent to everyone are returned](#only-messages-sent-to-everyone-are-returned) for how one is derived. A multi-line message continues on the following lines **without** a leading tab or timestamp. The provider's reply marker (`Replying to "…"`) is text, kept verbatim.

The timestamp may be `H:MM:SS` or `HH:MM:SS`, with optional fractional seconds. A line that starts with a timestamp but matches no layout still starts a new message in the parser, with an empty `sender` and `recipient`, so a layout the service does not know never glues separate messages together; because its recipient is unknown it is then dropped from the response (see above). **A continuation is a tab-indented line** (the newer layout's indentation) following any message, or **an unindented line following a message opened by the name header layout**; it is appended to that message's text. An unindented line after a message of the older layouts, or after an unattributed message, starts a new message with an empty `offset`, `sender` and `recipient`, which is likewise dropped: text the parser cannot attribute is never appended to a message that has a recipient. A continuation line of the name layout that itself starts with a timestamp is read as a timestamped line of an unknown layout and dropped the same way. A UTF-8 byte-order mark, Windows line endings, trailing whitespace and blank lines are tolerated. An empty file is a session with `message_count: 0` and `messages: []`.

---

## Errors

All errors use the service's standard `{ "code": "<status>", "message": "<text>" }` body. Messages never name a URL, a host or a file.

| Status | When | Message |
|---|---|---|
| `400 Bad Request` | `meeting_and_occurrence_id` outside `[0-9A-Za-z:_-]{1,128}`; `limit` outside `1..2000`; `offset` below `0` (Goa validates the query parameters first and may answer with its own validation body) | `invalid past meeting ID`, `limit must be between 1 and 2000`, `offset must be 0 or greater` |
| `401 Unauthorized` / `403 Forbidden` | Heimdall: no valid token / no `recording_viewer` relation | Heimdall's |
| `404 Not Found` | No recording record for the identifier, a soft-deleted record (`_sdc_deleted_at` present, non-null and non-empty: the event pipeline's rule), or a record with no `CHAT` file (including `recording_files: null`). The route never says whether the past meeting itself exists | `no saved chat for this past meeting` |
| `500 Internal Server Error` | The recording record exists but cannot be decoded (logged at error level with the identifier only, never the record contents) | `saved chat record could not be decoded` |
| `503 Service Unavailable` | NATS not configured at startup (no recording record reader); the `v1-objects` bucket not yet bound (it is bound lazily on first use, with a 5 s bind timeout and a 10 s negative-cache window after a failed bind, so a bucket unreachable at startup recovers without a restart); a KV read failure; any `CHAT` file that cannot be fetched as text: link not `https`, connection failure, timeout, non-2xx status, a content type other than `text/*` or `application/octet-stream` (`text/html` included, since that is a sign-in or error page, not a chat), or a body that opens with an HTML tag or document declaration (a sign-in or error page, whatever its label). One failing file fails the whole request: no partial response that could read as "no chat in that session" | `chat retrieval unavailable`, `recording record store unavailable`, `recording record could not be read`, `saved chat could not be retrieved; please retry` |

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

- The `download_url` of a `CHAT` file serves the text without further credentials (as LFX Self Serve's transcript fetch relies on), labelled `application/octet-stream` (the provider's media type for recording files, observed on production) or `text/*` other than `text/html`. An HTML answer, under any label (a redirect to a sign-in page, an error page), fails closed with `503`: a body that opens with an HTML tag or document declaration, after any leading comment or XML prolog, is refused by content, not only by label (a chat file opens with a timestamp). Any other media type fails closed too.
- The fetch client is a plain `net/http` client: it is deliberately **not** instrumented with `otelhttp`, whose spans would carry the full link.

## What this route does not change

Nothing indexed or published changes: the recording and transcript index documents, their `public` flags, the FGA messages, the KV mappings and the reindex scripts are untouched. [`docs/indexer-contract.md`](../indexer-contract.md) and [`docs/fga-contract.md`](../fga-contract.md) are unchanged by this route. Transcript content is out of scope.
