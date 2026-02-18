# ITX Meeting Proxy Service

The ITX Meeting Proxy Service is a lightweight stateless proxy that forwards meeting-related requests to the ITX Zoom API service. It provides a thin authentication and authorization layer for the Linux Foundation's LFX platform.

## 🚀 Quick Start

### For Deployment (Helm)

```bash
# Install the proxy service
helm upgrade --install lfx-v2-meeting-service ./charts/lfx-v2-meeting-service \
  --namespace lfx \
  --create-namespace \
  --set image.tag=latest \
  --set app.environment.ITX_BASE_URL.value="https://api.itx.linuxfoundation.org" \
  --set app.environment.ITX_CLIENT_ID.value="your-client-id" \
  --set app.environment.ITX_CLIENT_SECRET.value="your-client-secret"
```

### For Local Development

1. **Prerequisites**
   - Go 1.24+ installed
   - Make installed

2. **Clone and Setup**

   ```bash
   git clone https://github.com/linuxfoundation/lfx-v2-meeting-service.git
   cd lfx-v2-meeting-service

   # Install dependencies
   make deps
   ```

3. **Configure Environment**

   ```bash
   # Copy the example environment file and configure it
   cp .env.example .env
   # Edit .env with ITX credentials
   ```

   Required environment variables:
   ```bash
   ITX_ENABLED=true
   ITX_BASE_URL=https://api.dev.itx.linuxfoundation.org
   ITX_CLIENT_ID=your-client-id
   ITX_CLIENT_SECRET=your-client-secret
   ITX_AUTH0_DOMAIN=linuxfoundation-dev.auth0.com
   ITX_AUDIENCE=https://api.dev.itx.linuxfoundation.org/
   ```

4. **Run the Service**

   ```bash
   # Run with default settings
   make run

   # Or run with debug logging
   make debug
   ```

## 🏗️ Architecture

The service is a stateless HTTP proxy built using a clean architecture pattern:

- **API Layer**: Goa-generated HTTP handlers and OpenAPI specifications
- **Service Layer**: Request validation and ITX client orchestration
- **Domain Layer**: Core request/response models
- **Infrastructure Layer**: ITX HTTP client with OAuth2 authentication

### Key Features

- **Stateless Proxy**: No data persistence, all state managed by ITX service
- **ITX Meeting Operations**: Create, read, update, delete meetings via ITX
- **ITX Registrant Operations**: Manage meeting registrants via ITX
- **ITX Past Meeting Operations**: Full CRUD operations for past meeting records via ITX
- **ITX Past Meeting Summary Operations**: Retrieve and update AI-generated meeting summaries
- **JWT Authentication**: Secure API access via Heimdall integration
- **ID Mapping**: Optional v1/v2 ID translation via NATS (can be disabled)
- **OpenAPI Documentation**: Auto-generated API specifications

## 📁 Project Structure

```bash
lfx-v2-meeting-service/
├── cmd/                           # Application entry points
│   └── meeting-api/               # Main API server
├── charts/                        # Helm chart for Kubernetes deployment
│   └── lfx-v2-meeting-service/
├── design/                        # Goa API design files
│   ├── meeting-svc.go             # Service definition
│   └── itx_types.go               # ITX type definitions
├── gen/                           # Generated code (DO NOT EDIT)
│   ├── http/                      # HTTP transport layer
│   └── meeting_service/           # Service interfaces
├── internal/                      # Private application code
│   ├── domain/                    # Business domain layer
│   │   ├── models/                # Domain models
│   │   ├── errors.go              # Domain-specific errors
│   │   ├── itx_proxy.go           # ITX proxy interface
│   │   └── id_mapper.go           # ID mapper interface
│   ├── infrastructure/            # Infrastructure layer
│   │   ├── auth/                  # JWT authentication
│   │   ├── proxy/                 # ITX HTTP client
│   │   └── idmapper/              # NATS-based ID mapping
│   ├── middleware/                # HTTP middleware
│   └── service/                   # Service layer implementation
│       ├── auth_service.go        # Auth service
│       └── itx/                   # ITX services
├── pkg/                           # Public packages
│   ├── models/itx/                # ITX request/response models
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
# Run with default settings
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

# View templates
make helm-templates
```

### Docker

```bash
# Build Docker image
make docker-build

# Run with Docker
docker run -p 8080:8080 \
  -e ITX_ENABLED=true \
  -e ITX_BASE_URL=https://api.itx.linuxfoundation.org \
  -e ITX_CLIENT_ID=your-client-id \
  -e ITX_CLIENT_SECRET=your-client-secret \
  linuxfoundation/lfx-v2-meeting-service:latest
```

## 📖 API Documentation

The service automatically generates OpenAPI documentation:

- **OpenAPI 2.0**: `gen/http/openapi.yaml`
- **OpenAPI 3.0**: `gen/http/openapi3.yaml`
- **JSON formats**: Also available in `gen/http/`

Access the documentation at: `http://localhost:8080/openapi.json`

### Available Endpoints

#### Health Checks
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/livez` | GET | Liveness check |
| `/readyz` | GET | Readiness check |

#### ITX Meeting Operations
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/itx/meetings` | POST | Create meeting via ITX |
| `/itx/meetings/{meeting_id}` | GET | Get meeting details |
| `/itx/meetings/{meeting_id}` | PUT | Update meeting |
| `/itx/meetings/{meeting_id}` | DELETE | Delete meeting |
| `/itx/meetings/{meeting_id}/join_link` | GET | Get join link for user |
| `/itx/meetings/{meeting_id}/occurrences/{occurrence_id}` | PATCH | Update occurrence |
| `/itx/meetings/{meeting_id}/occurrences/{occurrence_id}` | DELETE | Delete occurrence |
| `/itx/meeting_count` | GET | Get meeting count |

#### ITX Registrant Operations

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/itx/meetings/{meeting_id}/registrants` | POST | Add registrant |
| `/itx/meetings/{meeting_id}/registrants` | GET | List registrants |
| `/itx/meetings/{meeting_id}/registrants/{registrant_uid}` | GET | Get registrant |
| `/itx/meetings/{meeting_id}/registrants/{registrant_uid}` | PATCH | Update registrant |
| `/itx/meetings/{meeting_id}/registrants/{registrant_uid}` | PUT | Update registrant status |
| `/itx/meetings/{meeting_id}/registrants/{registrant_uid}` | DELETE | Delete registrant |

#### ITX Past Meeting Operations

| Endpoint                                 | Method | Description          |
|------------------------------------------|--------|----------------------|
| `/itx/past_meetings`                     | POST   | Create past meeting  |
| `/itx/past_meetings/{past_meeting_id}`   | GET    | Get past meeting     |
| `/itx/past_meetings/{past_meeting_id}`   | PUT    | Update past meeting  |
| `/itx/past_meetings/{past_meeting_id}`   | DELETE | Delete past meeting  |

## 🔧 Configuration

The service can be configured via environment variables:

### Required Configuration

| Variable | Description | Example |
|----------|-------------|---------|
| `ITX_BASE_URL` | Base URL for ITX service | `https://api.itx.linuxfoundation.org` |
| `ITX_CLIENT_ID` | OAuth2 client ID for ITX | `your-client-id` |
| `ITX_CLIENT_SECRET` | OAuth2 client secret for ITX | `your-client-secret` |
| `ITX_AUTH0_DOMAIN` | Auth0 domain for ITX OAuth2 | `linuxfoundation.auth0.com` |
| `ITX_AUDIENCE` | OAuth2 audience for ITX | `https://api.itx.linuxfoundation.org/` |

### Authentication Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `JWKS_URL` | JWKS URL for JWT verification | `http://lfx-platform-heimdall.lfx.svc.cluster.local:4457/.well-known/jwks` |
| `JWT_AUDIENCE` | JWT token audience | `lfx-v2-meeting-service` |
| `JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL` | Mock principal for local dev | `""` |

### Optional Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `LOG_LEVEL` | Log level (debug, info, warn, error) | `info` |
| `LOG_ADD_SOURCE` | Add source location to logs | `true` |
| `LFX_ENVIRONMENT` | LFX environment (dev, staging, prod) | `prod` |
| `ID_MAPPING_DISABLED` | Disable v1/v2 ID mapping | `false` |
| `NATS_URL` | NATS server URL (for ID mapping) | `nats://lfx-platform-nats.lfx.svc.cluster.local:4222` |

### ID Mapping

The service supports optional ID mapping between v1 and v2 systems via NATS:

- **Enabled** (default): Set `ID_MAPPING_DISABLED=false` and provide `NATS_URL`
- **Disabled**: Set `ID_MAPPING_DISABLED=true` to pass IDs through unchanged

### Tracing Configuration

The service supports distributed tracing via OpenTelemetry:

| Variable | Description | Default |
|----------|-------------|---------|
| `OTEL_SERVICE_NAME` | Service name for traces | `lfx-v2-meeting-service` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP collector endpoint | `""` |
| `OTEL_TRACES_EXPORTER` | Traces exporter (otlp/none) | `none` |
| `OTEL_TRACES_SAMPLE_RATIO` | Sampling ratio (0.0-1.0) | `1.0` |

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
- Maintain test coverage
- Write clear, self-documenting code
- Update documentation for API changes
- Run `make check` before committing

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
