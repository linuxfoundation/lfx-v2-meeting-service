# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Architecture Overview

The ITX Meeting Proxy Service is a lightweight stateless proxy built with Go and the Goa framework. It provides a thin authentication and authorization layer between LFX clients and the ITX Zoom API service.

The service follows a clean architecture pattern with:

- **API Layer**: Goa-generated HTTP handlers and OpenAPI specifications
- **Service Layer**: Request validation and ITX client orchestration
- **Domain Layer**: Core request/response models and interfaces
- **Infrastructure Layer**: ITX HTTP client with OAuth2 authentication

### Key Features

- **Stateless Proxy**: No data persistence, all state managed by ITX service
- **ITX Meeting Operations**: Full CRUD operations for meetings via ITX API
- **ITX Registrant Operations**: Complete registrant management via ITX API
- **ITX Past Meeting Operations**: Full CRUD operations for past meeting records via ITX API
- **ITX Past Meeting Summary Operations**: Retrieve and update AI-generated meeting summaries
- **ITX Attachment Operations**: Create, read, update, delete, presign, and download attachments on both active meetings and past meetings
- **Event Processing**: NATS JetStream KV bucket watching for v1→v2 data sync (see [Event Processing Documentation](docs/event-processing.md))
- **LFID Invite Feature**: Outbound LFID invites for unregistered registrants, plus `invite_accepted` subscriber to enrich records when invites are accepted
- **JWT Authentication**: Secure API access via Heimdall integration
- **ID Mapping**: Optional v1/v2 ID translation via NATS (can be disabled)
- **OpenAPI Documentation**: Auto-generated API specifications served at `/_meetings/openapi.*`
- **OAuth2 M2M**: Machine-to-machine authentication with ITX service
- **Audit Stamping**: Resolves requesting principal into `created_by`/`updated_by` user objects on ITX write requests
- **PII Redaction**: Strips name/email from debug log output for all audit user fields

### Key Architectural Components

**API Layer (Goa-generated)**

- Design specifications in `design/` directory define API contracts
- Generated code in `gen/` directory (HTTP handlers, client, OpenAPI specs)
- Main API types: ITX meetings, registrants, past meetings, past meeting summaries, past meeting participants, and attachments

**Domain Layer** (`internal/domain/`)

- Core domain request/response models in `models/` (`CreateITXMeetingRequest`, `Committee`, `UpdatePastMeetingParticipant`)
- ITX wire types (enums, meeting types, recurrence) in `pkg/models/itx/`
- Domain interfaces: `ITXProxyClient`, `IDMapper`, `UserMetadataReader`, `UserServiceClient`, `InviteAcceptanceClient`, `InviteEmailSender`, `ProjectLookup`, `V1UserLookup`

**Service Layer** (`internal/service/`)

- Auth service for JWT validation
- ITX services in `itx/` subdirectory: `meeting_service.go`, `registrant_service.go`, `past_meeting_service.go`, `past_meeting_summary_service.go`, `past_meeting_participant_service.go`, `meeting_attachment_service.go`, `past_meeting_attachment_service.go`
- `audit.go` — `auditStamper` embedded by each ITX service; resolves requesting principal to `*itx.User` for `created_by`/`updated_by` stamps
- `preferred_email_service.go` — NATS RPC handler for preferred meeting-invite email

**Infrastructure Layer** (`internal/infrastructure/`)

- ITX HTTP client (`proxy/`) with OAuth2 authentication and PII-redacted debug logging
- JWT authentication (`auth/`)
- Optional NATS-based ID mapping (`idmapper/`)
- Event publishing infrastructure (`eventing/`) for indexer and FGA-sync, with OpenTelemetry tracing
- NATS subsystem (`nats/`): preferred-email responder, user-metadata reader, invite sender, interfaces
- User service HTTP client (`userservice/`) — calls v1 API gateway AS the user for preferred-email reads/writes

**Event Processing Layer** (`cmd/meeting-api/eventing/`)

- Event processor lifecycle management
- KV handler routing by key prefix
- 11 specialized event handlers: meetings, meeting attachments, registrants, RSVPs, past meetings, past meeting invitees/attendees, recordings, transcripts, summaries, past meeting attachments
- `invite_accepted_subscriber.go` — separate NATS queue subscriber (not KV-based); enriches DynamoDB records when an LFID invite is accepted
- RRULE occurrence calculation
- v1 user lookup and enrichment

**Middleware** (`internal/middleware/`)

- Request logging, authorization, and request ID handling

**Shared Packages** (`pkg/`)

- `pkg/constants/` — NATS subjects (`nats.go`), meeting role constants (`meeting.go`), HTTP context keys (`http.go`)
- `pkg/models/itx/` — ITX wire types: `meetings.go`, `meeting_registrants.go`, `past_meetings.go`, `past_meeting_summaries.go`, `past_meeting_participants.go`, `attachments.go`, `common.go`. The `itx.User` and `itx.CreatedUpdatedBy` types implement `slog.LogValuer` for automatic PII redaction when logged as slog attributes.
- `pkg/utils/` — pointer helpers (`ptr.go`), coalesce, map utilities, meeting utils, OTel span helpers
- `pkg/redaction/` — `Redact(s string)` and `RedactEmail(email string)` for safe log output

## Development Commands

### Core Development Workflow

- `make all` - Complete build pipeline: clean, deps, apigen, fmt, lint, test, build
- `make deps` - Install dependencies including goa CLI and golangci-lint (also installs git hooks)
- `make apigen` - Generate API code from Goa design files (required after design changes)
- `make build` - Build the meeting-api binary to bin/meeting-api
- `make run` - Run the service locally
- `make debug` - Run the service with debug logging enabled
- `make help` - Print all available make targets

### Testing

- `make test` - Run unit tests with race detection and coverage
- `make test-verbose` - Run tests with verbose output
- `make test-coverage` - Generate HTML coverage report in coverage/coverage.html

### Code Quality

- `make lint` - Run golangci-lint (automatically installed via deps)
- `make fmt` - Format Go code using gofmt
- `make check` - Verify formatting, linting, and license headers without modifying files
- `make license-check` - Check all Go files carry the LFX copyright and MIT SPDX headers
- `make verify` - Ensure generated code is up to date

### Git Hooks

- `make install-hooks` - Install git hooks from `scripts/hooks/` into `.git/hooks/` (also called by `make deps`)
- The pre-commit hook runs `gofmt` to enforce formatting before each commit

### Docker & Deployment

- `make docker-build` - Build Docker image
- `make helm-install` - Install Helm chart to lfx namespace (uses `values.yaml`, pulls from ghcr)
- `make helm-install-local` - Install Helm chart using `values.local.yaml` override (uses local Docker image)
- `make helm-templates` - Print Helm templates
- `make helm-templates-local` - Print Helm templates with local values override
- `make helm-uninstall` - Uninstall Helm chart

### Helm Image Workflows

- **Default**: `values.yaml` pulls the service image from `ghcr.io`. Use `make helm-install` when not making changes to service code.
- **Local development**: When making code changes, build the image with `make docker-build` and install with `make helm-install-local`. This uses `values.local.yaml` which points to the local Docker image (`linuxfoundation/lfx-v2-meeting-service`, `pullPolicy: Never`).
- Before using `make helm-install-local` for the first time, copy the example: `cp charts/lfx-v2-meeting-service/values.local.example.yaml charts/lfx-v2-meeting-service/values.local.yaml`. This file is gitignored.

## Local work cycle — post-commit and pre-PR review

This repo runs a local code review before a PR exists. It is an author-side
workflow: it produces evidence for the developer, and it never posts to GitHub,
opens or gates a PR, or touches a merge check. It is a **cross-model** review
only on the Pi path; when Pi is unavailable the trio falls back to Claude, which
is reported as such and is not cross-model evidence.

- **After every normal signed commit while still pre-PR**, run
  `/lfx-skills:lfx-local-review`. It runs the `general`, `repo_code` and
  `repo_learnings` reviewers in parallel against the committed target and returns
  **ordinary Markdown** reports.
- **The default is the newest commit only** — `HEAD^..HEAD`, the diff that
  commit introduced against its first parent. A caller may instead supply a
  direct base range, which may span more than one commit; it is reviewed exactly
  as supplied. Either way the reviewers use whatever base the host names and
  never derive one themselves.
- **Read the reports in this session and address the findings yourself.** The
  reviewers never edit code. Fixes are normal signed conventional commits —
  `fix(<scope>): …` or `fix: …` as appropriate — after which you **rerun the
  complete trio**.
- **Before opening a PR**, drain the reviews, then run this repo's native
  checks: `make check` (gofmt, `golangci-lint`, license headers) and `make test`
  (`-race -cover`). Run `make deps` first if `golangci-lint` is not installed.
  There is no separate readiness or preflight skill in this repo.

The trio is the central `general` brain plus this repo's two own brains, which
live in-repo and are versioned with the code they describe:

- `.claude/skills/meeting-service-code-reviewer/` — audits a change against this
  repo's written rule surface (`CLAUDE.md`, the FGA/indexer/event-processing/ITX
  contract docs, the `design/`→`gen/` boundary, the chart). Every finding quotes
  the rule it cites.
- `.claude/skills/meeting-service-learnings-reviewer/` — carries the empirical
  review method and matches a change against `docs/reviews/knowledge-base/`, this
  repo's patterns mined from real past PR review comments. Every finding quotes
  its knowledge-base entry. The KB lives under `docs/` because it is repo-owned
  knowledge versioned with the code it describes, and there is exactly one copy
  of it.

The generic `local-code-review` and `local-learnings-review` names beside them
are symlinks; they exist so the launcher's discovery is deterministic, and both
resolve to the two physical skills above. `.agents/skills/` links to the same
files for non-Claude agents. There is exactly one copy of each brain.

**An incomplete cycle is not a passing cycle.** If any reviewer's report starts
`INCOMPLETE — <reason>`, or the host reports a failed or empty reviewer, the
**whole cycle** is incomplete — successful reports from the other roles do not
rescue it. Resolve the cause and **rerun the complete trio under one harness**:
never rerun or replace a single role, and never assemble one cycle out of mixed
Pi and Claude evidence. A run that fell back to Claude because Pi was
unavailable is honestly reported as such and is not cross-model evidence.

The cycle **stops at PR open**. After verification the branch may be pushed and
the PR opened under the coordinator's release instruction; from that point review
is the PR-side Copilot surface's job (`.github/copilot-instructions.md` and
`.github/skills/**`), and nothing in the local cycle changes or feeds it.

## Development Guidelines

### Code Generation

- Always run `make apigen` after modifying files in `design/` directory
- The `gen/` directory contains generated code - do not edit manually
- Use `make verify` to ensure generated code is current before commits

### License Headers

Every Go file must carry these two lines at the top (before the `package` declaration):

```go
// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT
```

`make check` (which calls `make license-check`) will fail the build if headers are missing.

### Testing Strategy

- Unit tests for service logic and converters
- Mock interfaces provided for external dependencies (ITX client, ID mapper)
- Test files follow `*_test.go` naming convention

### Error Handling

- Uses domain-specific error types in `internal/domain/errors.go`
- Standard HTTP error responses defined in Goa design
- Structured logging with slog throughout the application

### Authentication & Authorization

- JWT-based authentication via Heimdall
- Bearer token required for all API endpoints except health checks
- Authorization middleware handles token validation

### Audit Stamping

Every ITX write request that supports `created_by`/`updated_by` must carry a stamped `*itx.User`. The `auditStamper` in `internal/service/itx/audit.go` is embedded by all ITX services. Call `buildRequestingUser(ctx)` to resolve the principal from context. It degrades gracefully: full profile on happy path (via UserMetadataReader NATS round-trip), `{username, email}` only when NATS is unavailable or returns an error, and `nil` when there is no principal on ctx. Never fail the caller's request due to audit resolution errors.

### PII Redaction in Logs

User-identity fields (`created_by`, `updated_by`, `modified_by`, `file_uploaded_by`) must never appear in plaintext in debug logs:

- `itx.User` and `itx.CreatedUpdatedBy` implement `slog.LogValuer` and emit only `username` when logged as a slog attribute value
- `requestJSONForLog(req)` and `responseJSONForLog(respBody)` in `internal/infrastructure/proxy/logredact.go` handle the harder case of audit fields nested inside a JSON-encoded body — use these when passing bodies to `slog.DebugContext`
- For other sensitive strings, use `pkg/redaction.Redact(s)` or `pkg/redaction.RedactEmail(email)`

### Pointer Conversion Helpers

Use the canonical helpers from `pkg/utils/ptr.go` for optional fields:

- `utils.StringPtrOmitEmpty(s string) *string` — returns `nil` when `s` is empty
- `utils.IntPtrOmitZero(i int) *int` — returns `nil` when `i` is zero
- `utils.BoolPtrOmitFalse(b bool) *bool` — returns `nil` when `b` is false
- `utils.StringValue(s *string) string` — safely dereferences; returns `""` on nil
- `utils.IntValue(i *int) int`, `utils.BoolValue(b *bool) bool` — safe dereferences

### Dependencies

- Built with Go 1.25+
- Primary framework: Goa v3 for API generation
- Optional: NATS for ID mapping, preferred-email, user-metadata, and invite features (can be disabled)
- Standard testing with testify

### Operational Scripts

`scripts/` contains standalone programs for one-off data operations:

- `scripts/backfill_meeting_host_credentials/` — backfills missing `MeetingHostCredentials` documents in OpenSearch for meetings indexed before the host_key separation
- `scripts/backfill_participant_mappings/` — migrates legacy participant mapping records to the new JSON format
- `scripts/reindex_meetings/` — re-triggers the full event processing pipeline by re-putting NATS KV entries
- `scripts/reconcile_meeting_registrants/` — reconciles registrant records

`tmp/` contains temporary one-off migration scripts (fix_*_access_check, cleanup_orphaned_meetings, reindex_past_meeting_participants). These are not part of the service binary; they are run ad-hoc against a live environment.

## Environment Variables

### Service Configuration

- `PORT`: HTTP listen port (default: `8080`)
- `LFX_ENVIRONMENT`: Deployment environment; normalizes to `dev`, `staging`, or `prod` (default: `prod`). Affects default URLs for ITX, user-service, and self-serve.

### ITX Configuration (Required)

For ITX proxy functionality, configure these environment variables:

- `ITX_BASE_URL`: Base URL for ITX service (default: `https://api.dev.itx.linuxfoundation.org`)
- `ITX_CLIENT_ID`: OAuth2 client ID for ITX authentication (**required**)
- `ITX_CLIENT_PRIVATE_KEY`: RSA private key in PEM format for ITX OAuth2 M2M authentication (**required**; load from file: `export ITX_CLIENT_PRIVATE_KEY="$(cat path/to/private.key)"`)
- `ITX_AUTH0_DOMAIN`: Auth0 domain for OAuth2 (default: `linuxfoundation-dev.auth0.com`)
- `ITX_AUDIENCE`: OAuth2 audience for ITX service (default: `https://api.dev.itx.linuxfoundation.org/`)

### User Service Configuration (Optional)

Backs the preferred meeting-invite email NATS RPC (LFXV2-2599). The RPC calls the v1 user-service preferences API **as the user**, using the bearer token forwarded in the request payload by self-serve (`{"token": "..."}`) — so no service credentials are configured here. The responder starts whenever `NATS_URL` is configured.

- `USER_SERVICE_BASE_URL`: v1 API-gateway base URL (defaults per `LFX_ENVIRONMENT`, e.g. `https://api-gw.dev.platform.linuxfoundation.org`)

### ID Mapping Configuration (Optional)

The service supports optional ID mapping between v1 and v2 systems:

- `ID_MAPPING_DISABLED`: Set to `true` to disable ID mapping (default: `false`)
- `NATS_URL`: NATS server URL. Required when ID mapping is enabled. Also enables the preferred-email responder, user-metadata resolution, and (when `INVITES_ENABLED=true`) the invite feature.

**Note**: If ID mapping is disabled, IDs are passed through unchanged. If enabled and NATS is unavailable, the service falls back to no-op mapping with a warning.

### Authentication Configuration

- `JWKS_URL`: JWKS URL for JWT verification
- `JWT_AUDIENCE`: JWT token audience (default: `lfx-v2-meeting-service`)
- `JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL`: Mock principal for local dev (dev only)

### Logging Configuration

- `LOG_LEVEL`: Log level (debug, info, warn, error) - default: `info`
- `LOG_ADD_SOURCE`: Add source location to logs - default: `true`

### Event Processing Configuration (Optional)

The service includes event processing for v1→v2 data synchronization. See [Event Processing Documentation](docs/event-processing.md) for details.

- `EVENT_PROCESSING_ENABLED`: Enable event processing (default: `true`)
- `EVENT_CONSUMER_NAME`: JetStream consumer name (default: `meeting-service-kv-consumer`)
- `EVENT_STREAM_NAME`: KV bucket stream name (default: `KV_v1-objects`)
- `EVENT_V1_MAPPINGS_BUCKET`: KV bucket name for v1 ID mappings (default: `v1-mappings`)
- `EVENT_MAX_DELIVER`: Max delivery attempts (default: `3`)
- `EVENT_ACK_WAIT`: Ack timeout (default: `30s`)
- `EVENT_MAX_ACK_PENDING`: Max pending acks (default: `1000`)

**Note**: The KV filter subjects are hardcoded to 12 specific buckets (meetings, mappings, registrants, invite-responses, attachments, past-meetings, past-meeting invitees/attendees/recordings/summaries/attachments, and their mapping buckets). There is no `EVENT_FILTER_SUBJECT` env var.

**Event Types Processed:**

- Active meetings with RRULE occurrence calculation
- Meeting-committee mappings
- Meeting attachments
- Registrants with user enrichment
- Invite responses (RSVPs)
- Past meetings
- Past meeting participants (invitees and attendees)
- Recordings and transcripts
- AI-generated summaries
- Past meeting attachments

### LFID Invite Configuration (Optional)

Controls outbound LFID invites for registrants who do not yet have an LFX account.

- `INVITES_ENABLED`: Set to `true` to enable outbound invite sending and the `invite_accepted` subscriber (default: `false`)
- `LFX_SELF_SERVE_BASE_URL`: LFX self-serve app URL embedded in invite emails as `return_url` (defaults per `LFX_ENVIRONMENT`; outbound invite sending is disabled when empty even if `INVITES_ENABLED=true`)

### Additional Configuration

- `PROJECT_LOGO_BASE_URL`: Base URL for project logo images (defaults to `https://lfx-one-project-logos-png-<env>.s3.us-west-2.amazonaws.com`)
- `LFX_APP_ORIGIN`: LFX app origin URL, used in CORS and response headers

## ITX API Integration

The service acts as a proxy to the ITX Zoom API service. All meeting and registrant operations are forwarded to ITX. See [ITX Proxy Implementation](docs/itx-proxy-implementation.md) for detailed integration notes.

### ITX Request Flow

1. Client sends authenticated request to proxy service
2. Proxy validates JWT token via Heimdall
3. Proxy converts Goa payload to ITX request format
4. Proxy authenticates with ITX using OAuth2 M2M flow
5. Proxy forwards request to ITX service
6. ITX processes request and returns response
7. Proxy converts ITX response to Goa format
8. Proxy returns response to client

### ITX Data Models

Key models in `pkg/models/itx/`:

- `ZoomMeetingRequest`: Request to create/update meetings
- `ZoomMeetingResponse`: Response with meeting details
- `Recurrence`: Meeting recurrence settings (Type is integer: 1=Daily, 2=Weekly, 3=Monthly)
- `Committee`: Committee associated with meeting
- `GetJoinLinkRequest`: Request for user-specific join link
- `ZoomMeetingJoinLink`: Join link response
- `User`: Full audit-user shape with username, name, email, profile_picture
- `CreatedUpdatedBy`: Audit-user shape for attachments (username, email, name; no profile_picture)

### Converter Functions

Converters in `cmd/meeting-api/service/`:

- `itx_meeting_converters.go`: Converts between Goa payloads and ITX meeting requests/responses
- `itx_registrant_converters.go`: Converts between Goa payloads and ITX registrant requests/responses
- `itx_past_meeting_converters.go`: Converts between Goa payloads and ITX past meeting requests/responses
- `itx_past_meeting_participant_converters.go`: Converts between Goa payloads and ITX participant requests/responses
- `itx_past_meeting_summary_converters.go`: Converts between Goa payloads and ITX summary requests/responses
- `itx_attachment_converters.go`: Converts between Goa payloads and ITX attachment requests/responses (both meeting and past-meeting)

**Important**: Always use the canonical pointer conversion helpers from `pkg/utils/ptr.go` (`StringPtrOmitEmpty`, `IntPtrOmitZero`, `BoolPtrOmitFalse`). Always stamp `created_by`/`updated_by` on write requests using `auditStamper.buildRequestingUser(ctx)` or `buildRequestingCreatedUpdatedBy(ctx)`.

## Project Structure Notes

### What Was Removed

This service was originally a comprehensive V2 meeting service with:
- NATS JetStream storage (6 KV buckets)
- Direct Zoom API integration
- Email notification service
- Past meeting tracking
- Webhook processing
- NATS messaging for indexing

All V2 functionality has been removed. The service is now a lightweight stateless proxy similar to lfx-v2-voting-service.

### What Remains

- ITX proxy functionality (meetings, registrants, past meetings, summaries, participants, attachments)
- JWT authentication via Heimdall
- Optional ID mapping via NATS
- Event processing system for v1→v2 data sync (see [Event Processing Documentation](docs/event-processing.md))
- LFID invite feature (outbound invites + invite_accepted subscriber)
- Goa-based API design and code generation
- Middleware (logging, authorization, request ID)
- OpenTelemetry tracing support (see [Tracing Documentation](docs/tracing.md))

## API Endpoints

### Health Checks

- `GET /livez` - Liveness check
- `GET /readyz` - Readiness check

### OpenAPI Documentation

- `GET /_meetings/openapi.json` - OpenAPI 2 spec (JSON)
- `GET /_meetings/openapi.yaml` - OpenAPI 2 spec (YAML)
- `GET /_meetings/openapi3.json` - OpenAPI 3 spec (JSON)
- `GET /_meetings/openapi3.yaml` - OpenAPI 3 spec (YAML)

### ITX Meeting Operations

- `POST /itx/meetings` - Create meeting
- `GET /itx/meetings/{meeting_id}` - Get meeting details
- `PUT /itx/meetings/{meeting_id}` - Update meeting
- `DELETE /itx/meetings/{meeting_id}` - Delete meeting
- `GET /itx/meetings/{meeting_id}/join_link` - Get join link
- `PUT /itx/meetings/{meeting_id}/occurrences/{occurrence_id}` - Update occurrence
- `DELETE /itx/meetings/{meeting_id}/occurrences/{occurrence_id}` - Delete occurrence
- `GET /itx/meeting_count` - Get meeting count
- `POST /itx/meetings/{meeting_id}/register_committee_members` - Register all committee members
- `POST /itx/meetings/{meeting_id}/resend` - Resend invites to all registrants
- `POST /itx/meetings/{meeting_id}/responses` - Submit RSVP / meeting response

### ITX Registrant Operations

- `POST /itx/meetings/{meeting_id}/registrants` - Add registrant
- `POST /itx/meetings/{meeting_id}/registrants/self` - Self-register (caller registers themselves)
- `GET /itx/meetings/{meeting_id}/registrants/{registrant_id}` - Get registrant
- `PUT /itx/meetings/{meeting_id}/registrants/{registrant_id}` - Update registrant
- `DELETE /itx/meetings/{meeting_id}/registrants/{registrant_id}` - Delete registrant
- `GET /itx/meetings/{meeting_id}/registrants/{registrant_id}/ics` - Download ICS calendar file
- `POST /itx/meetings/{meeting_id}/registrants/{registrant_id}/resend` - Resend invite to one registrant

### ITX Past Meeting Operations

- `POST /itx/past_meetings` - Create past meeting
- `GET /itx/past_meetings/{past_meeting_id}` - Get past meeting
- `PUT /itx/past_meetings/{past_meeting_id}` - Update past meeting
- `DELETE /itx/past_meetings/{past_meeting_id}` - Delete past meeting

### ITX Past Meeting Summary Operations

- `GET /itx/past_meetings/{past_meeting_id}/summaries/{summary_uid}` - Get past meeting summary
- `PUT /itx/past_meetings/{past_meeting_id}/summaries/{summary_uid}` - Update past meeting summary

### ITX Past Meeting Participant Operations

- `POST /itx/past_meetings/{past_meeting_id}/participants` - Add participant
- `PUT /itx/past_meetings/{past_meeting_id}/participants/{participant_id}` - Update participant
- `DELETE /itx/past_meetings/{past_meeting_id}/participants/{participant_id}` - Delete participant

### ITX Meeting Attachment Operations

- `POST /itx/meetings/{meeting_id}/attachments` - Create attachment
- `GET /itx/meetings/{meeting_id}/attachments/{attachment_id}` - Get attachment
- `PUT /itx/meetings/{meeting_id}/attachments/{attachment_id}` - Update attachment
- `DELETE /itx/meetings/{meeting_id}/attachments/{attachment_id}` - Delete attachment
- `POST /itx/meetings/{meeting_id}/attachments/presign` - Get presigned upload URL
- `GET /itx/meetings/{meeting_id}/attachments/{attachment_id}/download` - Get download URL

### ITX Past Meeting Attachment Operations

- `POST /itx/past_meetings/{meeting_and_occurrence_id}/attachments` - Create past meeting attachment
- `GET /itx/past_meetings/{meeting_and_occurrence_id}/attachments/{attachment_id}` - Get past meeting attachment
- `PUT /itx/past_meetings/{meeting_and_occurrence_id}/attachments/{attachment_id}` - Update past meeting attachment
- `DELETE /itx/past_meetings/{meeting_and_occurrence_id}/attachments/{attachment_id}` - Delete past meeting attachment
- `POST /itx/past_meetings/{meeting_and_occurrence_id}/attachments/presign` - Get presigned upload URL
- `GET /itx/past_meetings/{meeting_and_occurrence_id}/attachments/{attachment_id}/download` - Get download URL

### NATS RPC (preferred meeting-invite email — LFXV2-2599)

Request/reply subjects served by the preferred-email responder (see User Service Configuration). The caller forwards the user's bearer token; meeting-service resolves the user (SFID + emails) from it via `GET /v1/me` and proxies to the user-service preferences API **as the user** (Phase 1 storage).

- `lfx.meeting-service.preferred_email.get` — request `{"token":"<user bearer token>"}` → reply `{"email_id":string|null,"email":string|null}` (`null` ⇒ use primary)
- `lfx.meeting-service.preferred_email.set` — request `{"token":"<user bearer token>","email":"<verified-address>"}` (or `{"token":"...","email_id":<sfid>}`; `email` wins, `null`/`"primary"` clears) → reply `{"email_id","email"}` or `{"error":"..."}`. A verified `email` is resolved to its (auth0→SFDC synced) email-record ID; a not-yet-synced address returns a retryable error.
