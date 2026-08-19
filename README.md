# ITX Meeting Proxy Service

The ITX Meeting Proxy Service is a lightweight stateless proxy that forwards meeting-related requests to the ITX Zoom API service. It provides a thin authentication and authorization layer for the Linux Foundation's LFX platform.

## 🤖 AI Agent Development

If you are an AI agent (Claude, Cursor, Copilot, etc.) working on this codebase, read **[CLAUDE.md](CLAUDE.md)** in full before making any changes. It contains the authoritative architecture overview, coding conventions (audit stamping, PII redaction, pointer helpers, license headers), all environment variables, and the complete API endpoint inventory.

`AGENTS.md` at the repo root also points here for agent-oriented runtimes.

## 🚀 Quick Start

### For Local Development

1. **Prerequisites**
   - Go 1.25+ installed
   - Make installed

2. **Clone and Setup**

   ```bash
   git clone https://github.com/linuxfoundation/lfx-v2-meeting-service.git
   cd lfx-v2-meeting-service

   # Install dependencies and git hooks
   make deps
   ```

3. **Configure Environment**

   ```bash
   # Copy the example environment file and fill in your ITX credentials
   cp .env.example .env
   # Edit .env — at minimum set ITX_CLIENT_ID and ITX_CLIENT_PRIVATE_KEY
   ```

   Minimum required environment variables:

   ```bash
   ITX_CLIENT_ID=your-client-id
   ITX_CLIENT_PRIVATE_KEY="$(cat path/to/private.key)"
   ```

   For local JWT bypass (skips Heimdall validation):

   ```bash
   JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL=testuser
   ```

4. **Run the Service**

   ```bash
   # Run with default settings
   make run

   # Or run with debug logging
   make debug
   ```

### For Deployment (Helm)

See the [Deployment](#-deployment) section below.

## 🏗️ Architecture

The service is a stateless HTTP proxy built using a clean architecture pattern:

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
- **ITX Past Meeting Participant Operations**: Add, update, and delete past meeting participants
- **ITX Attachment Operations**: Create, read, update, delete, presign, and download attachments on both active meetings and past meetings
- **Event Processing**: NATS JetStream KV bucket watching for v1→v2 data sync (11 event types)
- **LFID Invite Feature**: Outbound LFID invites for unregistered registrants, plus `invite_accepted` subscriber to enrich records when invites are accepted
- **JWT Authentication**: Secure API access via Heimdall integration
- **ID Mapping**: Optional v1/v2 ID translation via NATS (can be disabled)
- **OpenAPI Documentation**: Auto-generated API specifications served at `/_meetings/openapi.*`
- **OAuth2 M2M**: Machine-to-machine authentication with ITX service
- **Audit Stamping**: Resolves requesting principal into `created_by`/`updated_by` user objects on ITX write requests
- **PII Redaction**: Strips name/email from debug log output for all audit user fields

## 📁 Project Structure

```
lfx-v2-meeting-service/
├── cmd/                           # Application entry points
│   └── meeting-api/               # Main API server
│       ├── eventing/              # Event processor, KV handlers, invite_accepted subscriber
│       └── service/               # Goa-to-ITX converter functions
├── charts/                        # Helm chart for Kubernetes deployment
│   └── lfx-v2-meeting-service/
├── design/                        # Goa API design files
│   ├── meeting-svc.go             # Service definition (source of truth for endpoints)
│   └── itx_types.go               # ITX type definitions
├── docs/                          # Architecture and contract documentation
│   ├── event-processing.md        # Event processing deep-dive
│   ├── itx-proxy-implementation.md
│   ├── tracing.md
│   ├── indexer-contract.md
│   └── fga-contract.md
├── gen/                           # Generated code (DO NOT EDIT)
│   ├── http/                      # HTTP transport layer and OpenAPI specs
│   └── meeting_service/           # Service interfaces
├── internal/                      # Private application code
│   ├── domain/                    # Business domain layer
│   │   ├── models/                # Domain models (CreateITXMeetingRequest, etc.)
│   │   ├── errors.go              # Domain-specific errors
│   │   ├── itx_proxy.go           # ITX proxy interface
│   │   ├── id_mapper.go           # ID mapper interface
│   │   ├── user_metadata.go       # UserMetadataReader interface
│   │   └── user_service.go        # UserServiceClient interface (preferred email)
│   ├── infrastructure/            # Infrastructure layer
│   │   ├── auth/                  # JWT authentication
│   │   ├── proxy/                 # ITX HTTP client with PII-redacted debug logging
│   │   ├── idmapper/              # NATS-based ID mapping
│   │   ├── nats/                  # NATS subsystem (preferred-email, user-metadata, invites)
│   │   ├── userservice/           # v1 user-service HTTP client
│   │   └── eventing/              # Event publishing (indexer + FGA-sync)
│   ├── middleware/                # HTTP middleware (logging, auth, request ID)
│   └── service/                   # Service layer implementation
│       ├── auth_service.go        # Auth service
│       ├── preferred_email_service.go  # NATS RPC handler for preferred email
│       └── itx/                   # ITX services + auditStamper
├── pkg/                           # Shared packages
│   ├── constants/                 # NATS subjects, meeting roles, HTTP context keys
│   ├── models/itx/                # ITX wire types (meetings, registrants, attachments, etc.)
│   ├── redaction/                 # Redact(s) and RedactEmail(email) helpers
│   └── utils/                     # Pointer helpers, coalesce, map utils, OTel helpers
├── scripts/                       # Standalone one-off data operation scripts
│   ├── backfill_meeting_host_credentials/
│   ├── backfill_participant_mappings/
│   ├── reindex_meetings/
│   └── reconcile_meeting_registrants/
├── tmp/                           # Temporary ad-hoc migration scripts (not part of binary)
├── Dockerfile                     # Container build configuration
├── Makefile                       # Build and development commands
├── CLAUDE.md                      # AI agent guide (architecture, conventions, env vars)
├── AGENTS.md                      # Agent runtime pointer → CLAUDE.md
└── go.mod                         # Go module definition
```

## 📡 Event Processing

The service includes a comprehensive event processing system for v1→v2 data synchronization. It watches NATS JetStream KV buckets for meeting-related data changes and publishes events to both indexer and FGA-sync services.

**Features:**

- 11 event types: meetings, meeting attachments, registrants, RSVPs, past meetings, past meeting invitees/attendees, recordings, transcripts, AI summaries, past meeting attachments
- RRULE occurrence calculation for recurring meetings
- v1 user enrichment and Auth0 mapping
- Dual publishing architecture (indexer + FGA-sync)
- Parent-child dependency handling with retry logic
- Separate `invite_accepted` NATS queue subscriber (not KV-based)

For complete details, see **[Event Processing Documentation](docs/event-processing.md)**.

For the data schemas, tags, access control values, and parent references for all indexed resource types — see **[Indexer Contract](docs/indexer-contract.md)**.

## 🛠️ Development

### Prerequisites

- Go 1.25+
- Make
- Git (configured with GPG signing and DCO signoff — see [Contributing](#-contributing))

### Getting Started

1. **Install Dependencies**

   ```bash
   make deps
   ```

   This also installs the pre-commit hook that runs `gofmt` before each commit.

2. **Generate API Code**

   ```bash
   make apigen
   ```

   Generates HTTP transport, client, and OpenAPI documentation from `design/` files. Run this whenever you change `design/`.

3. **Build the Application**

   ```bash
   make build
   ```

   Creates the binary in `bin/meeting-api`.

### Development Workflow

#### Running the Service

```bash
# Run with default settings
make run

# Run with debug logging
make debug

# Build and run binary directly
make build
./bin/meeting-api
```

#### Code Quality

**Always run these before committing:**

```bash
# Format code
make fmt

# Run linter
make lint

# Check license headers on all Go files
make license-check

# Run all tests (with race detection)
make test

# Check everything (format + lint + license headers) without modifying files
make check
```

#### API Development

When modifying the API:

1. **Update Design Files** in `design/` directory
2. **Regenerate Code**:

   ```bash
   make apigen
   ```

3. **Verify Generation**:

   ```bash
   make verify
   ```

4. **Run Tests** to ensure nothing breaks:

   ```bash
   make test
   ```

### Available Make Targets

| Target | Description |
|--------|-------------|
| `make all` | Complete build pipeline (clean, deps, apigen, fmt, lint, test, build) |
| `make deps` | Install dependencies, tools, and git hooks |
| `make install-hooks` | Install git hooks from `scripts/hooks/` into `.git/hooks/` |
| `make apigen` | Generate API code from design files |
| `make build` | Build the binary to `bin/meeting-api` |
| `make run` | Run the service locally |
| `make debug` | Run with debug logging |
| `make test` | Run unit tests with race detection |
| `make test-verbose` | Run tests with verbose output |
| `make test-coverage` | Generate HTML coverage report in `coverage/` |
| `make lint` | Run golangci-lint |
| `make fmt` | Format Go code with gofmt |
| `make license-check` | Verify all Go files carry LFX copyright and MIT SPDX headers |
| `make check` | Verify formatting, linting, and license headers without modifying files |
| `make verify` | Ensure generated code is up to date |
| `make clean` | Remove build artifacts |
| `make docker-build` | Build Docker image |
| `make helm-install` | Install Helm chart from GHCR |
| `make helm-install-local` | Install Helm chart using local Docker image |
| `make helm-templates` | Print rendered Helm templates |
| `make helm-uninstall` | Uninstall Helm chart |
| `make help` | List all available targets |

## 🧪 Testing

```bash
# Run all tests
make test

# Run with verbose output
make test-verbose

# Generate coverage report (opens at coverage/coverage.html)
make test-coverage
```

## 🚀 Deployment

### Helm Chart

The service includes a Helm chart for Kubernetes deployment.

#### Prerequisites: Kubernetes Secret

Before installing the chart, create the `meeting-secrets` secret in the `lfx` namespace. The `auth0_client_id` and `auth0_client_private_key` values are in 1Password under the **LFX V2** vault, in the note **LFX Platform Chart Values Secrets - Local Development**.

```bash
kubectl create secret generic meeting-secrets -n lfx \
  --from-literal=auth0_client_id="<client-id-from-1password>" \
  --from-file=auth0_client_private_key=./path/to/private.key
```

#### Option 1: Install from GHCR (no local code changes)

Use this if you just want to run the service without modifying its code. The image is pulled directly from the container registry:

```bash
make helm-install

# Or using Helm directly
helm upgrade --install lfx-v2-meeting-service ./charts/lfx-v2-meeting-service \
  --namespace lfx \
  --create-namespace
```

#### Option 2: Install from a Local Build (active development)

Use this if you are making changes to the service code. First, copy the example local values file (it is gitignored):

```bash
cp charts/lfx-v2-meeting-service/values.local.example.yaml \
   charts/lfx-v2-meeting-service/values.local.yaml
```

Then, whenever you make a code change and want to apply it:

```bash
# Rebuild the local image
make docker-build

# Install/upgrade the chart using the local image
make helm-install-local
```

### Docker

```bash
# Build Docker image
make docker-build

# Run with Docker
docker run -p 8080:8080 \
  -e ITX_BASE_URL=https://api.dev.itx.linuxfoundation.org \
  -e ITX_CLIENT_ID=your-client-id \
  -e ITX_CLIENT_PRIVATE_KEY="$(cat path/to/private.key)" \
  linuxfoundation/lfx-v2-meeting-service:latest
```

## 📖 API Documentation

The service automatically generates OpenAPI documentation:

- **OpenAPI 2.0**: `gen/http/openapi.yaml` / `gen/http/openapi.json`
- **OpenAPI 3.0**: `gen/http/openapi3.yaml` / `gen/http/openapi3.json`

Access the live docs when the service is running:

- `http://localhost:8080/_meetings/openapi.json`
- `http://localhost:8080/_meetings/openapi3.yaml`

### Available Endpoints

#### Health Checks

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/livez` | GET | Liveness check |
| `/readyz` | GET | Readiness check |

#### OpenAPI Documentation

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/_meetings/openapi.json` | GET | OpenAPI 2 spec (JSON) |
| `/_meetings/openapi.yaml` | GET | OpenAPI 2 spec (YAML) |
| `/_meetings/openapi3.json` | GET | OpenAPI 3 spec (JSON) |
| `/_meetings/openapi3.yaml` | GET | OpenAPI 3 spec (YAML) |

#### ITX Meeting Operations

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/itx/meetings` | POST | Create meeting |
| `/itx/meetings/{meeting_id}` | GET | Get meeting details |
| `/itx/meetings/{meeting_id}` | PUT | Update meeting |
| `/itx/meetings/{meeting_id}` | DELETE | Delete meeting |
| `/itx/meetings/{meeting_id}/join_link` | GET | Get join link for user |
| `/itx/meetings/{meeting_id}/responses` | POST | Submit RSVP (accepted/declined/maybe) |
| `/itx/meetings/{meeting_id}/occurrences/{occurrence_id}` | PUT | Update occurrence |
| `/itx/meetings/{meeting_id}/occurrences/{occurrence_id}` | DELETE | Delete occurrence |
| `/itx/meeting_count` | GET | Get meeting count |
| `/itx/meetings/{meeting_id}/register_committee_members` | POST | Register all committee members |
| `/itx/meetings/{meeting_id}/resend` | POST | Resend invites to all registrants |

#### ITX Registrant Operations

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/itx/meetings/{meeting_id}/registrants` | POST | Add registrant |
| `/itx/meetings/{meeting_id}/registrants/self` | POST | Self-register (caller registers themselves) |
| `/itx/meetings/{meeting_id}/registrants/{registrant_id}` | GET | Get registrant |
| `/itx/meetings/{meeting_id}/registrants/{registrant_id}` | PUT | Update registrant |
| `/itx/meetings/{meeting_id}/registrants/{registrant_id}` | DELETE | Delete registrant |
| `/itx/meetings/{meeting_id}/registrants/{registrant_id}/ics` | GET | Download ICS calendar file |
| `/itx/meetings/{meeting_id}/registrants/{registrant_id}/resend` | POST | Resend invite to one registrant |

#### ITX Past Meeting Operations

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/itx/past_meetings` | POST | Create past meeting |
| `/itx/past_meetings/{past_meeting_id}` | GET | Get past meeting |
| `/itx/past_meetings/{past_meeting_id}` | PUT | Update past meeting |
| `/itx/past_meetings/{past_meeting_id}` | DELETE | Delete past meeting |

#### ITX Past Meeting Summary Operations

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/itx/past_meetings/{past_meeting_id}/summaries/{summary_uid}` | GET | Get AI-generated summary |
| `/itx/past_meetings/{past_meeting_id}/summaries/{summary_uid}` | PUT | Update summary |

#### ITX Past Meeting Participant Operations

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/itx/past_meetings/{past_meeting_id}/participants` | POST | Add participant |
| `/itx/past_meetings/{past_meeting_id}/participants/{participant_id}` | PUT | Update participant |
| `/itx/past_meetings/{past_meeting_id}/participants/{participant_id}` | DELETE | Delete participant |

#### ITX Meeting Attachment Operations

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/itx/meetings/{meeting_id}/attachments` | POST | Create attachment |
| `/itx/meetings/{meeting_id}/attachments/{attachment_id}` | GET | Get attachment metadata |
| `/itx/meetings/{meeting_id}/attachments/{attachment_id}` | PUT | Update attachment |
| `/itx/meetings/{meeting_id}/attachments/{attachment_id}` | DELETE | Delete attachment |
| `/itx/meetings/{meeting_id}/attachments/presign` | POST | Generate presigned upload URL |
| `/itx/meetings/{meeting_id}/attachments/{attachment_id}/download` | GET | Get download URL |

#### ITX Past Meeting Attachment Operations

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/itx/past_meetings/{meeting_and_occurrence_id}/attachments` | POST | Create past meeting attachment |
| `/itx/past_meetings/{meeting_and_occurrence_id}/attachments/{attachment_id}` | GET | Get attachment metadata |
| `/itx/past_meetings/{meeting_and_occurrence_id}/attachments/{attachment_id}` | PUT | Update attachment |
| `/itx/past_meetings/{meeting_and_occurrence_id}/attachments/{attachment_id}` | DELETE | Delete attachment |
| `/itx/past_meetings/{meeting_and_occurrence_id}/attachments/presign` | POST | Generate presigned upload URL |
| `/itx/past_meetings/{meeting_and_occurrence_id}/attachments/{attachment_id}/download` | GET | Get download URL |

## 🔧 Configuration

Copy `.env.example` to `.env` for local development. All variables can also be exported directly.

### Service Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | HTTP listen port | `8080` |
| `LFX_ENVIRONMENT` | Deployment environment (`dev`, `staging`, `prod`) | `prod` |
| `PROJECT_LOGO_BASE_URL` | Base URL for project logo images | `https://lfx-one-project-logos-png-<env>.s3.us-west-2.amazonaws.com` |
| `LFX_APP_ORIGIN` | LFX app origin URL, used in CORS and response headers | `""` |

### ITX Configuration (Required)

| Variable | Description | Default |
|----------|-------------|---------|
| `ITX_BASE_URL` | Base URL for ITX service | `https://api.dev.itx.linuxfoundation.org` |
| `ITX_CLIENT_ID` | OAuth2 client ID for ITX | **required** |
| `ITX_CLIENT_PRIVATE_KEY` | RSA private key in PEM format for ITX OAuth2 M2M | **required** |
| `ITX_AUTH0_DOMAIN` | Auth0 domain for ITX OAuth2 | `linuxfoundation-dev.auth0.com` |
| `ITX_AUDIENCE` | OAuth2 audience for ITX | `https://api.dev.itx.linuxfoundation.org/` |

Load the private key from file: `export ITX_CLIENT_PRIVATE_KEY="$(cat path/to/private.key)"`

### Authentication Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `JWKS_URL` | JWKS URL for JWT verification | `http://lfx-platform-heimdall.lfx.svc.cluster.local:4457/.well-known/jwks` |
| `JWT_AUDIENCE` | JWT token audience | `lfx-v2-meeting-service` |
| `JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL` | Mock principal for local dev (bypasses Heimdall) | `""` |

### ID Mapping Configuration (Optional)

| Variable | Description | Default |
|----------|-------------|---------|
| `ID_MAPPING_DISABLED` | Set to `true` to disable v1/v2 ID mapping | `false` |
| `NATS_URL` | NATS server URL. Required when ID mapping is enabled. Also enables the preferred-email responder, user-metadata resolution, and (when `INVITES_ENABLED=true`) the invite feature. | `""` |

### User Service Configuration (Optional)

| Variable | Description | Default |
|----------|-------------|---------|
| `USER_SERVICE_BASE_URL` | v1 API gateway base URL for preferred-email RPC | Per `LFX_ENVIRONMENT` |

### LFID Invite Configuration (Optional)

| Variable | Description | Default |
|----------|-------------|---------|
| `INVITES_ENABLED` | Enable outbound LFID invite sending and `invite_accepted` subscriber | `false` |
| `LFX_SELF_SERVE_BASE_URL` | LFX self-serve app URL embedded in invite emails as `return_url` | Per `LFX_ENVIRONMENT` |

### Event Processing Configuration (Optional)

| Variable | Description | Default |
|----------|-------------|---------|
| `EVENT_PROCESSING_ENABLED` | Enable KV-based event processing | `true` |
| `EVENT_CONSUMER_NAME` | JetStream consumer name | `meeting-service-kv-consumer` |
| `EVENT_STREAM_NAME` | KV bucket stream name | `KV_v1-objects` |
| `EVENT_V1_MAPPINGS_BUCKET` | KV bucket name for v1 ID mappings | `v1-mappings` |
| `EVENT_MAX_DELIVER` | Max delivery attempts per message | `3` |
| `EVENT_ACK_WAIT` | Ack timeout | `30s` |
| `EVENT_MAX_ACK_PENDING` | Max pending acks | `1000` |

> The KV filter subjects are hardcoded to 12 specific buckets; there is no `EVENT_FILTER_SUBJECT` env var.

### Logging Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `LOG_LEVEL` | Log level (`debug`, `info`, `warn`, `error`) | `info` |
| `LOG_ADD_SOURCE` | Add source location to log lines | `true` |

### Tracing Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `OTEL_SERVICE_NAME` | Service name for traces | `lfx-v2-meeting-service` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP collector endpoint | `""` |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | OTLP protocol (`grpc` or `http`) | `grpc` |
| `OTEL_TRACES_EXPORTER` | Traces exporter (`otlp` or `none`) | `none` |

See **[Tracing Documentation](docs/tracing.md)** for full configuration details.

## 🤝 Contributing

This repository follows the Linux Foundation contribution conventions.

### Commit Requirements

All commits must be:
- **GPG-signed**: `git commit -S -s` (the `-s` adds DCO `Signed-off-by` trailer)
- **Conventional commit format**: `<type>(<scope>): <summary>` — e.g. `feat(registrants): add self-registration endpoint`

Types: `feat` | `fix` | `refactor` | `docs` | `chore`

### Code Standards

Before opening a PR, ensure all of these pass:

```bash
make check   # gofmt + golangci-lint + license-header check
make test    # unit tests with -race -cover
```

Every Go file must carry these two header lines before the `package` declaration:

```go
// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT
```

`make check` will fail if any file is missing them.

### Pull Request Workflow

1. Create a feature branch from `main`
2. Make changes, run `make check` and `make test`
3. Run the local review cycle: `/lfx-skills:lfx-local-review` (see CLAUDE.md for details)
4. Open a PR — title must follow `<type>(<scope>): <summary> [LFXV2-XXXX]` format

### API Changes

When changing the API:
1. Edit files in `design/` (never edit `gen/` directly)
2. Run `make apigen` to regenerate
3. Run `make verify` to confirm generated code is current
4. Commit both the design changes and the regenerated files

## 📄 License

This project is licensed under the MIT License — see the [LICENSE](LICENSE) file for details.
