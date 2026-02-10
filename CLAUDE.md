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
- **JWT Authentication**: Secure API access via Heimdall integration
- **ID Mapping**: Optional v1/v2 ID translation via NATS (can be disabled)
- **OpenAPI Documentation**: Auto-generated API specifications
- **OAuth2 M2M**: Machine-to-machine authentication with ITX service

### Key Architectural Components

**API Layer (Goa-generated)**

- Design specifications in `design/` directory define API contracts
- Generated code in `gen/` directory (HTTP handlers, client, OpenAPI specs)
- Main API types: ITX meetings and registrants with full CRUD operations

**Domain Layer** (`internal/domain/`)

- Core request/response models in `models/` (ITXMeetingRequest, Committee, ITXRecurrence)
- Domain interfaces for ITX proxy and ID mapping
- Business logic isolated from infrastructure concerns

**Service Layer** (`internal/service/`)

- Auth service for JWT validation
- ITX services in `itx/` subdirectory (meeting_service.go, registrant_service.go, past_meeting_service.go, past_meeting_summary_service.go)
- Orchestrates between API layer and infrastructure

**Infrastructure Layer** (`internal/infrastructure/`)

- ITX HTTP client (`proxy/`) with OAuth2 authentication
- JWT authentication (`auth/`)
- Optional NATS-based ID mapping (`idmapper/`)

**Middleware** (`internal/middleware/`)

- Request logging, authorization, and request ID handling

## Development Commands

### Core Development Workflow

- `make all` - Complete build pipeline: clean, deps, apigen, fmt, lint, test, build
- `make deps` - Install dependencies including goa CLI and golangci-lint
- `make apigen` - Generate API code from Goa design files (required after design changes)
- `make build` - Build the meeting-api binary to bin/meeting-api
- `make run` - Run the service locally
- `make debug` - Run the service with debug logging enabled

### Testing

- `make test` - Run unit tests with race detection and coverage
- `make test-verbose` - Run tests with verbose output
- `make test-coverage` - Generate HTML coverage report in coverage/coverage.html

### Code Quality

- `make lint` - Run golangci-lint (automatically installed via deps)
- `make fmt` - Format Go code using gofmt
- `make check` - Verify formatting and linting without modifying files
- `make verify` - Ensure generated code is up to date

### Docker & Deployment

- `make docker-build` - Build Docker image
- `make helm-install` - Install Helm chart to lfx namespace
- `make helm-templates` - Print Helm templates
- `make helm-uninstall` - Uninstall Helm chart

## Development Guidelines

### Code Generation

- Always run `make apigen` after modifying files in `design/` directory
- The `gen/` directory contains generated code - do not edit manually
- Use `make verify` to ensure generated code is current before commits

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

### Dependencies

- Built with Go 1.24+
- Primary framework: Goa v3 for API generation
- Optional: NATS for ID mapping (can be disabled)
- Standard testing with testify

## Environment Variables

### ITX Configuration (Required)

For ITX proxy functionality, configure these environment variables:

- `ITX_BASE_URL`: Base URL for ITX service (e.g., `https://api.itx.linuxfoundation.org`)
- `ITX_CLIENT_ID`: OAuth2 client ID for ITX authentication
- `ITX_CLIENT_SECRET`: OAuth2 client secret for ITX authentication
- `ITX_AUTH0_DOMAIN`: Auth0 domain for OAuth2 (e.g., `linuxfoundation.auth0.com`)
- `ITX_AUDIENCE`: OAuth2 audience for ITX service (e.g., `https://api.itx.linuxfoundation.org/`)

### ID Mapping Configuration (Optional)

The service supports optional ID mapping between v1 and v2 systems:

- `ID_MAPPING_DISABLED`: Set to `true` to disable ID mapping (default: `false`)
- `NATS_URL`: NATS server URL for ID mapping (only needed if mapping is enabled)

**Note**: If ID mapping is disabled, IDs are passed through unchanged. If enabled and NATS is unavailable, the service falls back to no-op mapping with a warning.

### Authentication Configuration

- `JWKS_URL`: JWKS URL for JWT verification
- `JWT_AUDIENCE`: JWT token audience (default: `lfx-v2-meeting-service`)
- `JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL`: Mock principal for local dev (dev only)

### Logging Configuration

- `LOG_LEVEL`: Log level (debug, info, warn, error) - default: `info`
- `LOG_ADD_SOURCE`: Add source location to logs - default: `true`

## ITX API Integration

The service acts as a proxy to the ITX Zoom API service. All meeting and registrant operations are forwarded to ITX.

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

### Converter Functions

Converters in `cmd/meeting-api/service/`:

- `itx_meeting_converters.go`: Converts between Goa payloads and ITX meeting requests/responses
- `itx_registrant_converters.go`: Converts between Goa payloads and ITX registrant requests/responses

**Important**: Always use appropriate pointer conversion helpers (`ptrIfNotZero` for ints, `ptrIfNotEmpty` for strings, `ptrIfTrue` for bools).

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

- ITX proxy functionality (meetings, registrants, and past meetings)
- JWT authentication via Heimdall
- Optional ID mapping via NATS
- Goa-based API design and code generation
- Middleware (logging, authorization, request ID)
- OpenTelemetry tracing support

## API Endpoints

### Health Checks

- `GET /livez` - Liveness check
- `GET /readyz` - Readiness check

### ITX Meeting Operations

- `POST /itx/meetings` - Create meeting
- `GET /itx/meetings/{meeting_id}` - Get meeting details
- `PUT /itx/meetings/{meeting_id}` - Update meeting
- `DELETE /itx/meetings/{meeting_id}` - Delete meeting
- `GET /itx/meetings/{meeting_id}/join_link` - Get join link
- `PUT /itx/meetings/{meeting_id}/occurrences/{occurrence_id}` - Update occurrence
- `DELETE /itx/meetings/{meeting_id}/occurrences/{occurrence_id}` - Delete occurrence
- `GET /itx/meeting_count` - Get meeting count

### ITX Registrant Operations

- `POST /itx/meetings/{meeting_id}/registrants` - Add registrant
- `GET /itx/meetings/{meeting_id}/registrants` - List registrants
- `GET /itx/meetings/{meeting_id}/registrants/{registrant_uid}` - Get registrant
- `PUT /itx/meetings/{meeting_id}/registrants/{registrant_uid}` - Update registrant
- `DELETE /itx/meetings/{meeting_id}/registrants/{registrant_uid}` - Delete registrant

### ITX Past Meeting Operations

- `POST /itx/past_meetings` - Create past meeting
- `GET /itx/past_meetings/{past_meeting_id}` - Get past meeting
- `PUT /itx/past_meetings/{past_meeting_id}` - Update past meeting
- `DELETE /itx/past_meetings/{past_meeting_id}` - Delete past meeting
