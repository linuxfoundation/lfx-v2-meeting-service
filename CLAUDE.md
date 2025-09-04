# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Features

### Email Notifications

The service supports sending invitation and cancellation emails for meeting registrants:

- **Email Service**: SMTP-based email service with HTML and plain text templates
- **Local Development**: Uses Mailpit (localhost:1025) to capture emails without sending real emails
- **Templates**: Professional email templates with descriptive names:
  - `meeting_invitation.html/txt` - Meeting invitation emails
  - `meeting_invitation_cancellation.html/txt` - Registration cancellation emails
- **Configuration**: Configurable via environment variables and Helm chart values
- **Graceful Handling**: Email failures don't prevent registrant operations

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

## Architecture Overview

The LFX v2 Meeting Service is a comprehensive microservice built with Go and the Goa framework. It provides robust CRUD operations for meetings and registrants with NATS JetStream persistence and JWT authentication.

The service is built using a clean architecture pattern with the following layers:

- **API Layer**: Goa-generated HTTP handlers and OpenAPI specifications
- **Service Layer**: Business logic and orchestration
- **Domain Layer**: Core business models and interfaces
- **Infrastructure Layer**: NATS persistence, JWT authentication, and messaging

### Key Features

- **Meeting Management**: Complete CRUD operations with platform integration (Zoom support)
- **Registrant Management**: Registration handling with email uniqueness validation
- **Historical Tracking**: Past meeting records with session tracking and participant attendance
- **Webhook Integration**: Platform event processing for real-time meeting state updates
- **NATS JetStream Storage**: Scalable and resilient data persistence across 6 KV buckets
- **NATS Messaging**: Event-driven communication with other services
- **JWT Authentication**: Secure API access via Heimdall integration
- **OpenAPI Documentation**: Auto-generated API specifications
- **Comprehensive Testing**: Full unit test coverage with mocks

### Key Architectural Components

**API Layer (Goa-generated)**

- Design specifications in `design/` directory define API contracts
- Generated code in `gen/` directory (HTTP handlers, client, OpenAPI specs)
- Main API types: meetings, registrants, and past meetings with full CRUD operations

**Domain Layer** (`internal/domain/`)

- Core business models in `models/` (Meeting, Registrant, Committee, Recurrence, PastMeeting, PastMeetingRecording)
- Domain interfaces for repository and messaging abstractions
- Business logic isolated from infrastructure concerns

**Service Layer** (`internal/service/`)

- Business operations and handlers
- Orchestrates between domain models and infrastructure
- Implements Goa service interfaces

**Infrastructure Layer** (`internal/infrastructure/`)

- NATS integration for messaging (`messaging/`) and key-value storage (`store/`)
- JWT authentication (`auth/`)
- Zoom integration (`zoom/`) for meeting platform services
- Webhook handling (`webhook/`) for platform event processing
- Six NATS KV buckets: "meetings", "meeting-settings", "meeting-registrants", "past-meetings", "past-meeting-participants", and "past-meeting-recordings"

**Handlers Layer** (`internal/handlers/`)

- Message and webhook event handlers
- Orchestrates business logic for async event processing

**Middleware** (`internal/middleware/`)

- Request logging, authorization, and request ID handling

### Data Storage

- Uses NATS JetStream KV stores for persistence
- Six main buckets:
  - `meetings`: Core meeting information
  - `meeting-settings`: Meeting configuration and organizers
  - `meeting-registrants`: Meeting registration data
  - `past-meetings`: Historical meeting occurrences with session tracking
  - `past-meeting-participants`: Historical participant data with attendance tracking
  - `past-meeting-recordings`: Recording metadata with session-based access URLs and file information
- NATS messaging for event publishing (indexer integration)

### Meeting Types and Platforms

- Supports multiple meeting platforms (primary: Zoom)
- Meeting types include recurring meetings with complex recurrence patterns
- Platform-specific configurations (ZoomConfig for Zoom meetings)

## Development Guidelines

### Code Generation

- Always run `make apigen` after modifying files in `design/` directory
- The `gen/` directory contains generated code - do not edit manually
- Use `make verify` to ensure generated code is current before commits

### Testing Strategy

- Unit tests for all domain models and business logic
- Mock interfaces provided for external dependencies (including Zoom API clients)
- Test files follow `*_test.go` naming convention
- External service integrations use mock implementations in `/mocks/` subdirectories

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
- NATS for messaging and storage
- Standard testing with testify

## Environment Variables

### Zoom Integration

For Zoom meeting platform support, configure these environment variables:

- `ZOOM_ACCOUNT_ID`: OAuth Server-to-Server Account ID
- `ZOOM_CLIENT_ID`: OAuth App Client ID  
- `ZOOM_CLIENT_SECRET`: OAuth App Client Secret

**Note**: Get these values from 1Password (search for "LFX Zoom Integration"). Required only when creating meetings with `platform="Zoom"`.

### Zoom Webhook Development

For local webhook development, you'll need to expose your local service to receive webhook events from Zoom:

- `ZOOM_WEBHOOK_SECRET_TOKEN`: Webhook secret token for signature validation

#### Local Webhook Testing with ngrok

To test Zoom webhooks locally:

1. **Install ngrok**: Download from [ngrok.com](https://ngrok.com/) or use package manager

   ```bash
   brew install ngrok  # macOS
   # or download from https://ngrok.com/download
   ```

2. **Start your local service**:

   ```bash
   make run  # Starts service on localhost:8080
   ```

3. **Expose your service with ngrok** (in a separate terminal):

   ```bash
   ngrok http 8080
   ```

   This creates a public URL like `https://abc123.ngrok.io` that forwards to your local service.

4. **Configure Zoom webhook URL**: In your Zoom App settings, set webhook endpoint to:

   ```
   https://abc123.ngrok.io/webhooks/zoom
   ```

5. **Set webhook secret**: Copy the webhook secret from Zoom App settings to your environment:

   ```bash
   export ZOOM_WEBHOOK_SECRET_TOKEN="your_webhook_secret_here"
   ```

**Supported Zoom Webhook Events:**

- `meeting.started` - Meeting begins
- `meeting.ended` - Meeting concludes  
- `meeting.deleted` - Meeting is deleted
- `meeting.participant_joined` - Participant joins
- `meeting.participant_left` - Participant leaves
- `recording.completed` - Recording is ready
- `recording.transcript_completed` - Transcript is ready
- `meeting.summary_completed` - AI summary is ready

**Webhook Processing Flow:**

1. HTTP webhook endpoint validates Zoom signature
2. Event published to NATS for async processing
3. Service handlers process business logic (no reply expected)

## HTTP Header Conventions

### ETag and Conditional Requests

This service follows proper HTTP conditional request semantics:

- **GET responses**: Include `ETag` header with current resource version
- **PUT/DELETE requests**: Include `If-Match` header for optimistic concurrency control

**Example flow:**

1. Client makes GET request: `GET /meetings/{id}`
2. Server responds with: `ETag: "123"` header  
3. Client makes update request: `PUT /meetings/{id}` with `If-Match: "123"` header
4. Server validates the If-Match value against current resource version
