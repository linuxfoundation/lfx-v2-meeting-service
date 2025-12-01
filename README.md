# LFX v2 Meeting Service

The LFX v2 Meeting Service is a comprehensive microservice that handles all meeting-related operations for the Linux Foundation's LFX platform. Built with Go and the Goa framework, it provides robust CRUD operations for meetings and registrants with NATS JetStream persistence and JWT authentication.

## 🚀 Quick Start

### For Deployment (Helm)

If you just need to run the service without developing on the service, use the Helm chart:

```bash
# Install the meeting service
helm upgrade --install lfx-v2-meeting-service ./charts/lfx-v2-meeting-service \
  --namespace lfx \
  --create-namespace \
  --set image.tag=latest
```

### For Local Development

1. **Prerequisites**
   - Go 1.24+ installed
   - Make installed
   - Docker (optional, for containerized development)
   - NATS server running (for local testing)

2. **Clone and Setup**

   ```bash
   git clone https://github.com/linuxfoundation/lfx-v2-meeting-service.git
   cd lfx-v2-meeting-service
   
   # Install dependencies and generate API code
   make deps
   make apigen
   ```

3. **Configure Environment (Optional)**

   ```bash
   # Copy the example environment file and configure it
   cp .env.example .env
   # Edit .env with your local settings
   ```

4. **Run the Service**

   ```bash
   # Run with default settings
   make run
   
   # Or run with debug logging
   make debug
   ```

## 🏗️ Architecture

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
- **NATS JetStream Storage**: Scalable and resilient data persistence across 5 KV buckets
- **NATS Messaging**: Event-driven communication with other services
- **JWT Authentication**: Secure API access via Heimdall integration
- **OpenAPI Documentation**: Auto-generated API specifications
- **Comprehensive Testing**: Full unit test coverage with mocks

## 📁 Project Structure

```bash
lfx-v2-meeting-service/
├── cmd/                           # Application entry points
│   └── meeting-api/               # Main API server
├── charts/                        # Helm chart for Kubernetes deployment
│   └── lfx-v2-meeting-service/
├── design/                        # Goa API design files
│   ├── meeting-svc.go             # Main service definition
│   ├── meeting_types.go           # Meeting type definitions
│   ├── registrant_types.go        # Registrant type definitions
│   └── types.go                   # Common type definitions
├── gen/                           # Generated code (DO NOT EDIT)
│   ├── http/                      # HTTP transport layer
│   └── meeting_service/           # Service interfaces
├── internal/                      # Private application code
│   ├── domain/                    # Business domain layer
│   │   ├── models/                # Domain models and conversions
│   │   ├── errors.go              # Domain-specific errors
│   │   ├── messaging.go           # Messaging abstractions
│   │   └── repository.go          # Repository interfaces
│   ├── infrastructure/            # Infrastructure layer
│   │   ├── auth/                  # JWT authentication
│   │   ├── messaging/             # NATS messaging implementation
│   │   └── store/                 # NATS KV store repositories
│   ├── middleware/                # HTTP middleware
│   ├── handlers/                  # Message handlers
│   └── service/                   # Service layer implementation
├── pkg/                           # Public packages
│   ├── constants/                 # Application constants
│   └── utils/                     # Utility functions
├── Dockerfile                     # Container build configuration
├── Makefile                       # Build and development commands
└── go.mod                         # Go module definition
```

## Meeting and Participant Tags

The Meeting API service generates a comprehensive set of tags for meetings, registrants, and participants that are sent to the indexer-service. These tags enable searchability and discoverability of meeting-related content through OpenSearch.

### Tags Generated for Meetings and Settings

When meetings and meeting settings are created or updated, the following tags are automatically generated:

| Field | Tag Format | Example | Purpose |
|-------|------------|---------|---------|
| UID | Plain value | `061a110a-7c38-4cd3-bfcf-fc8511a37f35` | Direct lookup by ID |
| UID | `meeting_uid:<value>` | `meeting_uid:061a110a-7c38-4cd3-bfcf-fc8511a37f35` | Namespaced lookup by ID |
| ProjectUID | `project_uid:<value>` | `project_uid:9493eae5-cd73-4c4a-b28f-3b8ec5280f6c` | Find meetings for a project |
| Committee UIDs | `committee_uid:<value>` | `committee_uid:cbef1ed5-17dc-4a50-84e2-6cddd70f6878` | Find meetings for specific committees |
| Title | Plain value | `Weekly Technical Steering Committee` | Text search in meeting titles |
| Description | Plain value | `Weekly meeting to discuss technical decisions` | Text search in meeting descriptions |
| MeetingType | `meeting_type:<value>` | `meeting_type:Board` | Filter meetings by type (e.g., Board, Technical) |

### Tags Generated for Meeting Registrants

When meeting registrants are created or updated, the following tags are automatically generated:

| Field | Tag Format | Example | Purpose |
|-------|------------|---------|---------|
| UID | Plain value | `f3c5b4e4-f5a8-4902-a48d-e7ad963bf7d1` | Direct lookup by registrant ID |
| UID | `registrant_uid:<value>` | `registrant_uid:f3c5b4e4-f5a8-4902-a48d-e7ad963bf7d1` | Namespaced lookup by registrant ID |
| MeetingUID | `meeting_uid:<value>` | `meeting_uid:061a110a-7c38-4cd3-bfcf-fc8511a37f35` | Find registrants for a specific meeting |
| FirstName | Plain value | `John` | Text search by first name |
| LastName | Plain value | `Doe` | Text search by last name |
| Email | Plain value | `john.doe@example.com` | Text search by email address |
| Username | Plain value | `johndoe` | Text search by username |

### Tags Generated for Past Meetings

When past meetings are created or updated, the following tags are automatically generated:

| Field | Tag Format | Example | Purpose |
|-------|------------|---------|---------|
| UID | Plain value | `ad83feb8-d34d-438e-b203-87c5df64f1e2` | Direct lookup by past meeting ID |
| UID | `past_meeting_uid:<value>` | `past_meeting_uid:ad83feb8-d34d-438e-b203-87c5df64f1e2` | Namespaced lookup by past meeting ID |
| MeetingUID | `meeting_uid:<value>` | `meeting_uid:061a110a-7c38-4cd3-bfcf-fc8511a37f35` | Link to original meeting |
| ProjectUID | `project_uid:<value>` | `project_uid:9493eae5-cd73-4c4a-b28f-3b8ec5280f6c` | Find past meetings for a project |
| Committee UIDs | `committee_uid:<value>` | `committee_uid:cbef1ed5-17dc-4a50-84e2-6cddd70f6878` | Find meetings for specific committees |
| OccurrenceID | `occurrence_id:<value>` | `occurrence_id:occurrence-789-012-345` | Find specific meeting occurrence |
| Title | Plain value | `Weekly TSC Meeting - March 15, 2024` | Text search in past meeting titles |
| Description | Plain value | `Discussed project roadmap and priorities` | Text search in past meeting descriptions |

### Tags Generated for Past Meeting Participants

When past meeting participants are created or updated, the following tags are automatically generated:

| Field | Tag Format | Example | Purpose |
|-------|------------|---------|---------|
| UID | Plain value | `899a891c-0079-4ecf-b32c-c75fbc8c471e` | Direct lookup by participant ID |
| UID | `past_meeting_participant_uid:<value>` | `past_meeting_participant_uid:899a891c-0079-4ecf-b32c-c75fbc8c471e` | Namespaced lookup by participant ID |
| PastMeetingUID | `past_meeting_uid:<value>` | `past_meeting_uid:8ba24fb9-0d15-45f2-b6e6-11423fae572e` | Find participants for a past meeting |
| MeetingUID | `meeting_uid:<value>` | `meeting_uid:061a110a-7c38-4cd3-bfcf-fc8511a37f35` | Link to original meeting |
| FirstName | Plain value | `Jane` | Text search by first name |
| LastName | Plain value | `Smith` | Text search by last name |
| Username | Plain value | `janesmith` | Text search by username |
| Email | Plain value | `jane.smith@example.com` | Text search by email address |

### How Tags Are Used

Tags serve multiple important purposes in the LFX system:

1. **Indexed Search**: Tags are indexed in OpenSearch, enabling fast lookups and text searches across meetings, registrants, and participants

2. **Relationship Navigation**:
   - Meeting-project relationships can be traversed using the `project_uid` tags
   - Meeting-committee relationships can be traversed using the `committee_uid` tags
   - Meeting-registrant relationships can be traversed using the `meeting_uid` tags
   - Past meeting-participant relationships can be traversed using the `past_meeting_uid` tags

3. **Multiple Access Patterns**: Both plain value and prefixed tags support different query patterns:
   - Plain values support general text search (e.g., "find meetings containing 'TSC'")
   - Prefixed values support field-specific search (e.g., "find registrants with email '<john@example.com>'")

4. **Historical Tracking**: Past meetings and participants maintain references to original meetings via `meeting_uid` tags, enabling historical analysis and reporting

5. **Data Synchronization**: When meetings, registrants, or participants are updated, their tags are automatically updated, ensuring search results remain current

## 🛠️ Development

### Prerequisites

- Go 1.24+
- Make
- Git

### Getting Started

1. **Install Dependencies**

   ```bash
   make deps
   ```

   This installs:
   - Go module dependencies
   - Goa CLI for code generation
   - golangci-lint for code linting

2. **Generate API Code**

   ```bash
   make apigen
   ```

   Generates HTTP transport, client, and OpenAPI documentation from design files.

3. **Build the Application**

   ```bash
   make build
   ```

   Creates the binary in `bin/meeting-api`.

### Development Workflow

#### Running the Service

```bash
# Run with auto-regeneration
make run

# Run with debug logging
make debug

# Build and run binary
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

# Run all tests
make test

# Check everything (format + lint + tests)
make check
```

#### Testing

```bash
# Run all tests with race detection and coverage
make test

# Run tests with verbose output
make test-verbose

# Generate HTML coverage report
make test-coverage
# Opens coverage/coverage.html
```

**Writing Tests:**

- Place test files alongside source files with `_test.go` suffix
- Use table-driven tests for multiple test cases
- Mock external dependencies using the provided mock interfaces
- Achieve high test coverage (aim for >80%)
- Test both happy path and error cases

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

#### Zoom Integration Development

The Zoom integration follows a layered architecture pattern:

**Adding New Zoom API Endpoints:**

1. **Add API Methods** in `internal/infrastructure/zoom/api/`:
   - Add new methods to appropriate API client interfaces
   - Implement the methods with proper error handling
   - Add corresponding mock implementations for testing

2. **Update Provider Layer** in `internal/infrastructure/zoom/provider.go`:
   - Add business logic methods that orchestrate API calls
   - Handle Zoom-specific data transformations
   - Implement domain interfaces

3. **Add Tests**:

   ```bash
   # Test the new API methods
   go test ./internal/infrastructure/zoom/api/...
   
   # Test the provider integration
   go test ./internal/infrastructure/zoom/...
   ```

**Zoom Package Structure:**

```text
internal/infrastructure/zoom/
├── api/                    # Low-level Zoom API clients
│   ├── client.go          # HTTP client and auth
│   ├── meetings.go        # Meetings API endpoints
│   ├── users.go           # Users API endpoints
│   └── mocks/             # Mock implementations
├── provider.go            # Business logic layer
└── provider_test.go       # Integration tests
```

**Environment Variables:**

- `ZOOM_ACCOUNT_ID`: OAuth Server-to-Server Account ID
- `ZOOM_CLIENT_ID`: OAuth App Client ID  
- `ZOOM_CLIENT_SECRET`: OAuth App Client Secret
- `ZOOM_WEBHOOK_SECRET_TOKEN`: OAuth App webhook secret token

For local development, copy `.env.example` to `.env` and fill in your Zoom credentials from 1Password.

### Available Make Targets

| Target | Description |
|--------|-------------|
| `make all` | Complete build pipeline (clean, deps, apigen, fmt, lint, test, build) |
| `make deps` | Install dependencies and tools |
| `make apigen` | Generate API code from design files |
| `make build` | Build the binary |
| `make run` | Run the service locally |
| `make debug` | Run with debug logging |
| `make test` | Run unit tests |
| `make test-verbose` | Run tests with verbose output |
| `make test-coverage` | Generate HTML coverage report |
| `make lint` | Run code linter |
| `make fmt` | Format code |
| `make check` | Verify formatting and run linter |
| `make verify` | Ensure generated code is up to date |
| `make clean` | Remove build artifacts |
| `make docker-build` | Build Docker image |
| `make helm-install` | Install Helm chart |
| `make helm-uninstall` | Uninstall Helm chart |

## 🧪 Testing

### Running Tests

```bash
# Run all tests
make test

# Run with verbose output
make test-verbose

# Generate coverage report
make test-coverage
```

### Test Structure

The project follows Go testing best practices:

- **Unit Tests**: Test individual components in isolation
- **Integration Tests**: Test component interactions
- **Mock Interfaces**: Located in `internal/domain/mock.go` and other mock files
- **Test Coverage**: Aim for high coverage with meaningful tests

### Writing Tests

When adding new functionality:

1. **Write tests first** (TDD approach recommended)
2. **Use table-driven tests** for multiple scenarios
3. **Mock external dependencies** using provided interfaces
4. **Test error conditions** not just happy paths
5. **Keep tests focused** and independent

Example test structure:

```go
func TestServiceMethod(t *testing.T) {
    tests := []struct {
        name        string
        input       InputType
        setupMocks  func(*MockRepository)
        expected    ExpectedType
        expectError bool
    }{
        // Test cases here
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

## 🚀 Deployment

### Helm Chart

The service includes a Helm chart for Kubernetes deployment:

```bash
# Install with default values
make helm-install

# Install with custom values
helm upgrade --install lfx-v2-meeting-service ./charts/lfx-v2-meeting-service \
  --namespace lfx \
  --values custom-values.yaml

# Install with Zoom integration
helm upgrade --install lfx-v2-meeting-service ./charts/lfx-v2-meeting-service \
  --namespace lfx \
  --set zoom.accountId="your-zoom-account-id" \
  --set zoom.clientId="your-zoom-client-id" \
  --set zoom.clientSecret="your-zoom-client-secret"

# View templates
make helm-templates
```

### Docker

```bash
# Build Docker image
make docker-build

# Run with Docker
docker run -p 8080:8080 linuxfoundation/lfx-v2-meeting-service:latest
```

## 🔄 HTTP Header Conventions

### X-Sync Header for Synchronous Operations

All POST, PUT, and DELETE endpoints support the `X-Sync` header to control whether operations are processed synchronously or asynchronously:

**Header Format:**

```text
X-Sync: true   # Synchronous - waits for downstream services
X-Sync: false  # Asynchronous (default) - returns immediately
```

**Behavior:**

- **Asynchronous (default)**: The API returns immediately after persisting data. Indexing and access control messages are sent to NATS without waiting for responses.
- **Synchronous (`X-Sync: true`)**: The API waits for confirmation from downstream services (indexer and FGA-sync) before responding. Uses NATS request/reply pattern with a 10-second timeout.

**When to Use Synchronous Mode:**

- Integration tests that need to verify downstream effects
- Critical operations requiring confirmation before proceeding
- Admin operations where consistency is more important than performance

**When to Use Asynchronous Mode (default):**

- Normal user operations where speed is important
- Bulk operations where eventual consistency is acceptable
- High-throughput scenarios

## 📡 NATS Messaging

The service uses NATS for event-driven communication with other LFX platform services.

### Published Subjects

The service publishes messages to the following NATS subjects:

| Subject | Purpose | Message Schema |
|---------|---------|----------------|
| `lfx.index.meeting` | Meeting indexing events | `MeetingIndexerMessage` |
| `lfx.index.meeting_settings` | Meeting settings indexing events | `MeetingIndexerMessage` |
| `lfx.index.meeting_registrant` | Registrant indexing events | `MeetingIndexerMessage` |
| `lfx.update_access.meeting` | Meeting access control updates | `MeetingAccessMessage` |
| `lfx.delete_all_access.meeting` | Meeting access control deletion | `MeetingAccessMessage` |
| `lfx.put_registrant.meeting` | Registrant access control updates | `MeetingRegistrantAccessMessage` |
| `lfx.remove_registrant.meeting` | Registrant access control deletion | `MeetingRegistrantAccessMessage` |
| `lfx.meetings-api.meeting_deleted` | Meeting deletion events (internal) | `MeetingDeletedMessage` |

### Handled Subjects

The service handles incoming messages on these subjects:

| Subject | Purpose |
|---------|---------|
| `lfx.meetings-api.get_title` | Meeting title requests |
| `lfx.meetings-api.meeting_deleted` | Meeting deletion cleanup |
| `lfx.webhook.zoom.meeting.started` | Zoom meeting started event |
| `lfx.webhook.zoom.meeting.ended` | Zoom meeting ended event |
| `lfx.webhook.zoom.meeting.deleted` | Zoom meeting deleted event |
| `lfx.webhook.zoom.meeting.summary_completed` | Zoom meeting summary completed event |
| `lfx.webhook.zoom.meeting.participant_joined` | Zoom meeting participant joined event |
| `lfx.webhook.zoom.meeting.participant_left` | Zoom meeting participant left event |
| `lfx.webhook.zoom.meeting.recording.completed` | Zoom meeting recording completed event |
| `lfx.webhook.zoom.meeting.recording.transcript_completed` | Zoom meeting recording transcript completed event |

### Message Schemas

All message schemas are defined in `internal/domain/models/messaging.go`. Key schemas include:

- **MeetingIndexerMessage**: For search indexing operations
- **MeetingAccessMessage**: For meeting-level access control
- **MeetingRegistrantAccessMessage**: For registrant-level access control  
- **MeetingDeletedMessage**: For internal cleanup when meetings are deleted

### Event Flow

When meetings or registrants are modified, the service automatically:

1. **Updates NATS KV storage** for persistence
2. **Publishes indexing messages** for search services
3. **Publishes access control messages** for permission services
4. **Handles cleanup messages** for cascading deletions

## 📖 API Documentation

The service automatically generates OpenAPI documentation:

- **OpenAPI 2.0**: `gen/http/openapi.yaml`
- **OpenAPI 3.0**: `gen/http/openapi3.yaml`
- **JSON formats**: Also available in `gen/http/`

Access the documentation at: `http://localhost:8080/openapi.json`

### Available Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/livez` | GET | Health check |
| `/readyz` | GET | Readiness check |
| `/meetings` | GET, POST | List/create meetings |
| `/meetings/{uid}` | GET, PUT, DELETE | Get/update/delete meeting |
| `/meetings/{uid}/registrants` | GET, POST | List/create registrants |
| `/meetings/{uid}/registrants/{id}` | GET, PUT, DELETE | Get/update/delete registrant |
| `/past_meetings` | GET, POST | List/create past meetings |
| `/past_meetings/{uid}` | GET, DELETE | Get/delete past meeting |
| `/past_meetings/{uid}/participants` | GET, POST | List/create past meeting participants |
| `/past_meetings/{uid}/participants/{id}` | GET, PUT, DELETE | Get/update/delete past meeting participant |

## 🔧 Configuration

The service can be configured via environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `NATS_URL` | NATS server URL | `nats://lfx-platform-nats.lfx.svc.cluster.local:4222` |
| `LOG_LEVEL` | Log level (debug, info, warn, error) | `info` |
| `LOG_ADD_SOURCE` | Add source location to logs | `true` |
| `JWKS_URL` | JWKS URL for JWT verification | `http://lfx-platform-heimdall.lfx.svc.cluster.local:4457/.well-known/jwks` |
| `JWT_AUDIENCE` | JWT token audience | `lfx-v2-meeting-service` |
| `SKIP_ETAG_VALIDATION` | Skip ETag validation (dev only) | `false` |
| `JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL` | Mock principal for local dev (dev only) | `""` |
| `LFX_ENVIRONMENT` | LFX app domain environment (dev, staging, prod) | `prod` |

### Zoom Integration Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `ZOOM_ACCOUNT_ID` | Zoom OAuth Server-to-Server Account ID | `""` |
| `ZOOM_CLIENT_ID` | Zoom OAuth App Client ID | `""` |
| `ZOOM_CLIENT_SECRET` | Zoom OAuth App Client Secret | `""` |

When all three Zoom variables are configured, the service will automatically integrate with Zoom API for meetings where `platform="Zoom"`.

### Zoom Webhook Development

For webhook development, add this additional environment variable:

| Variable | Description | Default |
|----------|-------------|---------|
| `ZOOM_WEBHOOK_SECRET_TOKEN` | Webhook secret token for signature validation | `""` |

#### Local Webhook Testing with ngrok

To test Zoom webhooks locally, you'll need to expose your local service to receive webhook events from Zoom:

1. **Install ngrok**: Download from [ngrok.com](https://ngrok.com/) or use package manager:

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

   ```text
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

### Development Environment Variables

For local development, you may want to override these settings:

```bash
export NATS_URL="nats://localhost:4222"
export LOG_LEVEL="debug"
export SKIP_ETAG_VALIDATION="true"
export JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL="local-dev-user"
```

## 🤝 Contributing

1. **Fork** the repository
2. **Create** a feature branch (`git checkout -b feature/amazing-feature`)
3. **Make changes** and ensure tests pass (`make test`)
4. **Run quality checks** (`make check`)
5. **Commit** changes (`git commit -m 'Add amazing feature'`)
6. **Push** to branch (`git push origin feature/amazing-feature`)
7. **Create** a Pull Request

### Code Standards

- Follow Go conventions and best practices
- Maintain high test coverage
- Write clear, self-documenting code
- Update documentation for API changes
- Run `make check` before committing

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
