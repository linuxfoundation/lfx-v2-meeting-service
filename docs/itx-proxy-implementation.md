# ITX Proxy Architecture

This document describes how the ITX Meeting Proxy Service is implemented as a stateless proxy to the ITX Zoom API service.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Code Organization](#code-organization)
- [Implementation Patterns](#implementation-patterns)
- [Data Flow](#data-flow)
- [Configuration](#configuration)

---

## Overview

The ITX Meeting Proxy Service is a lightweight stateless proxy that forwards meeting-related requests to the ITX Zoom API service. It provides:

- **Authentication**: JWT-based authentication via Heimdall
- **Authorization**: Request validation and principal extraction
- **ID Mapping**: Optional v1/v2 ID translation (can be disabled)
- **Protocol Translation**: Converts between Goa API format and ITX API format

---

## Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────┐
│               ITX Meeting Proxy Service                  │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  ┌────────────────────────────────────────────┐         │
│  │         API Layer (Goa-generated)          │         │
│  │         /itx/meetings/*                    │         │
│  │         /itx/meeting_count                 │         │
│  └────────────────┬───────────────────────────┘         │
│                   │                                      │
│                   ▼                                      │
│  ┌────────────────────────────────────────────┐         │
│  │         Service Layer                      │         │
│  │         - MeetingService                   │         │
│  │         - RegistrantService                │         │
│  │         - AuthService                      │         │
│  └────────────────┬───────────────────────────┘         │
│                   │                                      │
│                   ▼                                      │
│  ┌────────────────────────────────────────────┐         │
│  │      Infrastructure Layer                  │         │
│  │      - ITX Proxy Client (OAuth2)           │         │
│  │      - ID Mapper (optional, via NATS)      │         │
│  │      - JWT Auth (Heimdall)                 │         │
│  └────────────────┬───────────────────────────┘         │
│                   │                                      │
└───────────────────┼──────────────────────────────────────┘
                    │
                    ▼
          ┌──────────────────┐
          │   ITX Service    │
          │  (OAuth2 M2M)    │
          └────────┬─────────┘
                   ▼
          ┌──────────────────┐
          │   Zoom API       │
          └──────────────────┘
```

### Key Characteristics

- **Stateless**: No data persistence - all state managed by ITX
- **Thin Proxy**: Minimal business logic, primarily format conversion
- **OAuth2 M2M**: Machine-to-machine authentication with ITX
- **Optional ID Mapping**: Can translate v1/v2 IDs via NATS or pass through unchanged

---

## Code Organization

### Directory Structure

```
cmd/meeting-api/
├── api.go                       # Main API struct and handlers
├── api_itx_meetings.go          # ITX meeting endpoint handlers
├── api_itx_registrants.go       # ITX registrant endpoint handlers
├── config.go                    # Environment configuration
├── infrastructure.go            # JWT auth setup
├── main.go                      # Application entry point
├── server.go                    # HTTP server setup
└── service/
    ├── itx_meeting_converters.go     # Meeting format converters
    └── itx_registrant_converters.go  # Registrant format converters

internal/
├── domain/
│   ├── errors.go                # Domain error types
│   ├── itx_proxy.go             # ITX proxy interface
│   ├── id_mapper.go             # ID mapper interface
│   └── models/
│       └── itx_meeting.go       # ITX domain models
├── service/
│   ├── auth_service.go          # JWT authentication service
│   └── itx/
│       ├── meeting_service.go      # ITX meeting service
│       └── registrant_service.go   # ITX registrant service
└── infrastructure/
    ├── auth/
    │   └── jwt.go               # JWT validation
    ├── idmapper/
    │   ├── nats_mapper.go       # NATS-based ID mapping
    │   └── noop_mapper.go       # Pass-through mapper
    └── proxy/
        └── client.go            # ITX HTTP client

pkg/
├── models/itx/
│   └── models.go                # ITX API request/response types
└── utils/
    └── pointer.go               # Pointer conversion helpers

design/
├── meeting-svc.go               # Goa service definition
└── itx_types.go                 # ITX type definitions
```

---

## Implementation Patterns

### 1. Request Flow

```
Client Request
    │
    ▼
┌─────────────────────────────────────┐
│ 1. API Handler                      │
│    - Extract bearer token           │
│    - Validate JWT (Heimdall)        │
│    - Extract principal              │
└──────────────┬──────────────────────┘
               ▼
┌─────────────────────────────────────┐
│ 2. Service Layer                    │
│    - Convert Goa payload to domain  │
│    - Apply ID mapping (if enabled)  │
│    - Create ITX request             │
└──────────────┬──────────────────────┘
               ▼
┌─────────────────────────────────────┐
│ 3. ITX Proxy Client                 │
│    - Obtain OAuth2 token (cached)   │
│    - Forward request to ITX         │
│    - Handle ITX response            │
└──────────────┬──────────────────────┘
               ▼
┌─────────────────────────────────────┐
│ 4. Response Conversion              │
│    - Convert ITX response to Goa    │
│    - Apply reverse ID mapping       │
│    - Return to client               │
└─────────────────────────────────────┘
```

### 2. Converter Pattern

All conversions between Goa payloads and ITX requests/responses use dedicated converter functions:

**Example: Create Meeting**
```go
// Convert Goa payload to domain model
domainReq := ConvertCreateITXMeetingPayloadToDomain(goaPayload)

// Service layer converts domain to ITX API format
itxReq := service.ToITXRequest(domainReq)

// Call ITX API
itxResp := itxClient.CreateMeeting(itxReq)

// Convert ITX response back to Goa format
goaResp := ConvertITXMeetingResponseToGoa(itxResp)
```

### 3. Pointer Conversion Helpers

ITX responses use non-pointer types, while Goa uses pointer types for optional fields:

```go
// Helper functions in converters
func ptrIfNotEmpty(s string) *string {
    if s == "" {
        return nil
    }
    return &s
}

func ptrIfNotZero(i int) *int {
    if i == 0 {
        return nil
    }
    return &i
}

func ptrIfTrue(b bool) *bool {
    if !b {
        return nil
    }
    return &b
}
```

### 4. OAuth2 Client Credentials Flow

The ITX proxy client handles OAuth2 authentication:

```go
// Token is cached and automatically refreshed
type Client struct {
    baseURL      string
    clientID     string
    clientSecret string
    auth0Domain  string
    audience     string
    httpClient   *http.Client
    tokenCache   *oauth2.Token  // Cached token
    tokenMu      sync.RWMutex
}

// Automatically obtains/refreshes token before each request
func (c *Client) ensureValidToken(ctx context.Context) error {
    c.tokenMu.RLock()
    token := c.tokenCache
    c.tokenMu.RUnlock()

    if token != nil && token.Valid() {
        return nil
    }

    // Obtain new token...
}
```

---

## Data Flow

### Creating a Meeting

```
1. Client → POST /itx/meetings
   {
     "project_uid": "abc-123",
     "title": "Team Meeting",
     "start_time": "2024-01-15T10:00:00Z",
     "duration": 60,
     ...
   }

2. API Handler validates JWT and extracts principal

3. Service Layer:
   - Converts Goa CreateItxMeetingPayload → CreateITXMeetingRequest
   - Maps v2 project_uid to v1 ID (if ID mapping enabled)
   - Converts CreateITXMeetingRequest → itx.ZoomMeetingRequest

4. ITX Proxy Client:
   - Obtains OAuth2 token from Auth0
   - POST https://api.itx.linuxfoundation.org/v2/zoom/meetings
   - Receives itx.ZoomMeetingResponse

5. Service Layer converts itx.ZoomMeetingResponse → Goa ITXZoomMeetingResponse

6. Client ← 201 Created
   {
     "id": "123456789",
     "project_uid": "abc-123",
     "title": "Team Meeting",
     "host_key": "012345",
     "passcode": "abc123",
     "public_link": "https://zoom.us/j/123456789",
     ...
   }
```

### Field Mapping Example

| Client Field (Goa) | ITX API Field | Notes |
|--------------------|---------------|-------|
| `project_uid` | `project` | May require v1/v2 ID mapping |
| `title` | `topic` | Direct mapping |
| `start_time` | `start_time` | RFC3339 format |
| `description` | `agenda` | Direct mapping |
| `committees[].uid` | `committees[].id` | Array mapping |
| `recurrence.type` | `recurrence.type` | Integer (1=Daily, 2=Weekly, 3=Monthly) |

---

## Configuration

### Required Environment Variables

```bash
# ITX Service Configuration
ITX_BASE_URL=https://api.itx.linuxfoundation.org         # ITX service URL
ITX_CLIENT_ID=your-client-id                             # OAuth2 client ID
ITX_CLIENT_SECRET=your-client-secret                      # OAuth2 client secret
ITX_AUTH0_DOMAIN=linuxfoundation.auth0.com               # Auth0 domain
ITX_AUDIENCE=https://api.itx.linuxfoundation.org/        # OAuth2 audience

# Authentication
JWKS_URL=http://lfx-platform-heimdall.lfx.svc.cluster.local:4457/.well-known/jwks
JWT_AUDIENCE=lfx-v2-meeting-service
```

### Optional Environment Variables

```bash
# ID Mapping (v1/v2 translation)
ID_MAPPING_DISABLED=false                                 # Set to "true" to disable mapping
NATS_URL=nats://lfx-platform-nats.lfx.svc.cluster.local:4222  # Only if mapping enabled

# Logging
LOG_LEVEL=info                                            # debug, info, warn, error
LOG_ADD_SOURCE=true                                       # Include source location in logs

# LFX Environment
LFX_ENVIRONMENT=prod                                      # dev, staging, prod
```

### Helm Chart Configuration

**File**: [charts/lfx-v2-meeting-service/values.yaml](../charts/lfx-v2-meeting-service/values.yaml)

```yaml
app:
  environment:
    # ITX Proxy Configuration
    ITX_BASE_URL:
      value: https://api.itx.linuxfoundation.org
    ITX_CLIENT_ID:
      value: null  # Set via sealed secret
    ITX_CLIENT_SECRET:
      value: null  # Set via sealed secret
    ITX_AUTH0_DOMAIN:
      value: linuxfoundation.auth0.com
    ITX_AUDIENCE:
      value: https://api.itx.linuxfoundation.org/

    # ID Mapping (optional)
    ID_MAPPING_DISABLED:
      value: "false"
    NATS_URL:
      value: nats://lfx-platform-nats.lfx.svc.cluster.local:4222

    # Authentication
    JWKS_URL:
      value: http://lfx-platform-heimdall.lfx.svc.cluster.local:4457/.well-known/jwks
    JWT_AUDIENCE:
      value: lfx-v2-meeting-service
```

---

## Summary

| Aspect | ITX Proxy Service |
|--------|-------------------|
| **Purpose** | Lightweight Zoom meeting proxy |
| **Storage** | None (stateless) |
| **Platform Support** | Zoom only (via ITX) |
| **Business Logic** | Minimal (format conversion) |
| **Field Mapping** | Required (uid/id, title/topic, etc.) |
| **Infrastructure** | HTTP proxy client + OAuth2 |
| **Features** | Basic CRUD for meetings and registrants |
| **State** | Stateless |
| **Implementation** | ~1500 LOC |
| **Dependencies** | ITX service, Heimdall (JWT), optional NATS (ID mapping) |

The service provides:
- Authentication via Heimdall JWT
- Authorization via principal extraction
- Optional ID mapping via NATS
- Stateless proxy pattern
- OAuth2 M2M authentication with ITX
- Goa-based API design and code generation
