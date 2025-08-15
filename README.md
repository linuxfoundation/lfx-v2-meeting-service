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
- **NATS JetStream Storage**: Scalable and resilient data persistence
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
│   └── service/                   # Service layer implementation
├── pkg/                           # Public packages
│   ├── constants/                 # Application constants
│   └── utils/                     # Utility functions
├── Dockerfile                     # Container build configuration
├── Makefile                       # Build and development commands
└── go.mod                         # Go module definition
```

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

## 🔧 Configuration

The service can be configured via environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `NATS_URL` | NATS server URL | `nats://lfx-platform-nats.lfx.svc.cluster.local:4222` |
| `LOG_LEVEL` | Log level (debug, info, warn, error) | `info` |
| `LOG_ADD_SOURCE` | Add source location to logs | `true` |
| `JWKS_URL` | JWKS URL for JWT verification | `http://lfx-platform-heimdall.lfx.svc.cluster.local:4456/.well-known/jwks` |
| `JWT_AUDIENCE` | JWT token audience | `http://lfx-api.k8s.orb.local` |
| `SKIP_ETAG_VALIDATION` | Skip ETag validation (dev only) | `false` |
| `JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL` | Mock principal for local dev (dev only) | `""` |

### Zoom Integration Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `ZOOM_ACCOUNT_ID` | Zoom OAuth Server-to-Server Account ID | `""` |
| `ZOOM_CLIENT_ID` | Zoom OAuth App Client ID | `""` |
| `ZOOM_CLIENT_SECRET` | Zoom OAuth App Client Secret | `""` |

When all three Zoom variables are configured, the service will automatically integrate with Zoom API for meetings where `platform="Zoom"`.

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
