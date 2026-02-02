# ITX Proxy Implementation Architecture

This document describes how the ITX proxy endpoints are implemented in the codebase, how they compare to the LFX v2 endpoints, and the architectural patterns used.

## Table of Contents

- [Overview](#overview)
- [Architecture Comparison](#architecture-comparison)
- [Code Organization](#code-organization)
- [Implementation Patterns](#implementation-patterns)
- [Similarities](#similarities)
- [Differences](#differences)
- [Data Flow](#data-flow)

---

## Overview

The LFX Meeting Service supports two types of meeting endpoints:

1. **LFX v2 Endpoints** (`/v2/meetings/*`) - Full-featured meeting management with NATS persistence
2. **ITX Proxy Endpoints** (`/itx/meetings/*`) - Lightweight proxy to ITX Zoom API service

Both endpoint families handle meetings and registrants but use different storage and integration approaches.

---

## Architecture Comparison

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    LFX Meeting Service                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌────────────────────┐         ┌──────────────────────┐       │
│  │  LFX v2 Endpoints  │         │  ITX Proxy Endpoints │       │
│  │  /v2/meetings/*    │         │  /itx/meetings/*     │       │
│  └─────────┬──────────┘         └──────────┬───────────┘       │
│            │                               │                   │
│            ▼                               ▼                   │
│  ┌─────────────────────┐         ┌────────────────────┐        │
│  │   Service Layer     │         │  Service Layer     │        │
│  │   (LFX Logic)       │         │  (Proxy Logic)     │        │
│  └─────────┬───────────┘         └─────────┬──────────┘        │
│            │                               │                   │
│            ▼                               ▼                   │
│  ┌─────────────────────┐         ┌────────────────────┐        │
│  │  NATS KV Store      │         │  ITX Proxy Client  │        │
│  │  + Zoom Integration │         │  (HTTP Client)     │        │
│  └─────────────────────┘         └─────────┬──────────┘        │
│                                            │                   │
└────────────────────────────────────────────┼───────────────────┘
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

---

## Code Organization

### Directory Structure

```
cmd/meeting-api/
├── api_meetings.go              # LFX v2 meeting handlers
├── api_registrants.go           # LFX v2 registrant handlers
├── api_itx_meetings.go          # ITX proxy meeting handlers
├── api_itx_registrants.go       # ITX proxy registrant handlers
└── service/
    ├── meeting_converters.go         # LFX v2 converters
    ├── itx_meeting_converters.go     # ITX proxy meeting converters
    └── itx_registrant_converters.go  # ITX proxy registrant converters

internal/
├── domain/
│   ├── repository.go            # LFX v2 repository interface
│   └── itx_proxy.go            # ITX proxy client interface
├── service/
│   ├── meetings/               # LFX v2 meeting service
│   └── itx/
│       ├── meeting_service.go      # ITX meeting service
│       └── registrant_service.go   # ITX registrant service
└── infrastructure/
    ├── store/                  # NATS KV implementations (LFX v2)
    ├── zoom/                   # Zoom API client (LFX v2)
    └── proxy/
        └── client.go          # ITX HTTP proxy client

design/
├── meeting-svc.go             # Goa design for LFX v2 endpoints
├── meeting_types.go           # LFX v2 type definitions
└── itx_types.go              # ITX proxy type definitions
```

---

## Implementation Patterns

### LFX v2 Endpoints Pattern

**File**: [cmd/meeting-api/api_meetings.go](../cmd/meeting-api/api_meetings.go)

```go
// LFX v2 Create Meeting Handler
func (s *MeetingsAPI) CreateMeeting(ctx context.Context, p *meetingsvc.CreateMeetingPayload) (*meetingsvc.Meeting, error) {
    // 1. Convert Goa payload to domain model
    req := service.ConvertCreatePayloadToDomain(p)

    // 2. Call service layer (business logic)
    meeting, err := s.meetingService.CreateMeeting(ctx, req)
    if err != nil {
        return nil, handleError(err)
    }

    // 3. Convert domain model to Goa response
    resp := service.ConvertMeetingToGoa(meeting)
    return resp, nil
}
```

**Service Layer**: [internal/service/meetings/service.go](../internal/service/meetings/service.go)

```go
func (s *Service) CreateMeeting(ctx context.Context, req *models.CreateMeetingRequest) (*models.Meeting, error) {
    // 1. Validate request
    // 2. Create meeting in NATS
    // 3. Create Zoom meeting if platform = "Zoom"
    // 4. Publish NATS message for indexing
    // 5. Return meeting
}
```

**Storage**: NATS KV Store + Zoom API

### ITX Proxy Endpoints Pattern

**File**: [cmd/meeting-api/api_itx_meetings.go](../cmd/meeting-api/api_itx_meetings.go)

```go
// ITX Proxy Create Meeting Handler
func (s *MeetingsAPI) CreateItxMeeting(ctx context.Context, p *meetingsvc.CreateItxMeetingPayload) (*meetingsvc.ITXZoomMeetingResponse, error) {
    // 1. Convert Goa payload to ITX request (with field mapping)
    req := service.ConvertCreateITXMeetingPayloadToDomain(p)

    // 2. Call ITX service (thin proxy logic)
    resp, err := s.itxMeetingService.CreateMeeting(ctx, req)
    if err != nil {
        return nil, handleError(err)
    }

    // 3. Convert ITX response to Goa response (with field mapping)
    goaResp := service.ConvertITXMeetingResponseToGoa(resp)
    return goaResp, nil
}
```

**Service Layer**: [internal/service/itx/meeting_service.go](../internal/service/itx/meeting_service.go)

```go
func (s *MeetingService) CreateMeeting(ctx context.Context, req *models.CreateITXMeetingRequest) (*itx.ZoomMeetingResponse, error) {
    // 1. Convert domain request to ITX request
    // 2. Call ITX proxy client
    return s.proxyClient.CreateZoomMeeting(ctx, itxReq)
}
```

**Storage**: None (stateless proxy to ITX service)

---

## Similarities

### 1. API Layer Structure

Both use the same Goa framework and handler pattern:

- Handler methods in `cmd/meeting-api/api_*.go`
- Same error handling via `handleError()`
- Same authentication (JWT via Heimdall)
- Same authorization pattern (OpenFGA)

### 2. Service Layer Separation

Both have dedicated service layers:

- LFX v2: `internal/service/meetings/`
- ITX Proxy: `internal/service/itx/`

Both services are injected into the API handlers via the `MeetingsAPI` struct:

```go
type MeetingsAPI struct {
    // LFX v2 services
    meetingService    *meetings.Service
    registrantService *registrants.Service

    // ITX proxy services
    itxMeetingService    *itx.MeetingService
    itxRegistrantService *itx.RegistrantService
}
```

### 3. Converter Pattern

Both use converter functions for payload/response transformation:

- LFX v2: `cmd/meeting-api/service/meeting_converters.go`
- ITX Proxy: `cmd/meeting-api/service/itx_meeting_converters.go`

### 4. Domain Models

Both define request/response models:

- LFX v2: `internal/domain/models/`
- ITX Proxy: `pkg/models/itx/`

### 5. Authorization

Both use Heimdall rulesets with OpenFGA:

- File: `charts/lfx-v2-meeting-service/templates/ruleset.yaml`
- Same permission model: `viewer`, `organizer`, `auditor`

---

## Differences

### 1. Storage Approach

| Aspect | LFX v2 | ITX Proxy |
|--------|--------|-----------|
| **Primary Storage** | NATS KV Store (6 buckets) | None (stateless proxy) |
| **Data Persistence** | Full meeting/registrant data | No local persistence |
| **Platform Integration** | Optional Zoom via direct API | Always Zoom via ITX service |
| **Meeting Lifecycle** | Managed by service | Managed by ITX/Zoom |

### 2. Business Logic Complexity

**LFX v2**: Rich business logic

- Manages full meeting lifecycle
- Handles settings, organizers, attachments
- Processes webhook events
- Publishes indexer messages
- Email notifications
- Committee integration

**ITX Proxy**: Thin proxy layer

- Simple request/response transformation
- Field name mapping
- No business logic
- No state management

### 3. Data Models

**LFX v2 Meeting**:

```go
type Meeting struct {
    UID              string
    ProjectUID       string
    Title            string
    Description      string
    StartTime        string
    Duration         int
    Timezone         string
    Platform         string        // "Zoom" or others
    ZoomConfig       *ZoomConfig   // Optional
    Recurrence       *Recurrence
    Occurrences      []Occurrence
    RegistrantCount  int
    // ... many more fields
}
```

**ITX Proxy Meeting** (via ITX service):

```go
type ZoomMeetingResponse struct {
    ID               string         // From Zoom
    Project          string         // Maps to project_uid
    Topic            string         // Maps to title
    Agenda           string         // Maps to description
    StartTime        string
    Duration         int
    Timezone         string
    Visibility       string
    // ITX-specific fields
    HostKey          string
    Passcode         string
    PublicLink       string
    Occurrences      []Occurrence
    // ... ITX fields
}
```

### 4. Field Name Conventions

**LFX v2**: Uses LFX conventions throughout

- `project_uid`, `meeting_uid`, `committee_uid`
- Consistent with LFX ecosystem

**ITX Proxy**: Converts between conventions

- Proxy API: `project_uid`, `title`, `description`, `committee_uid`
- ITX API: `project`, `topic`, `agenda`, `committee_id`

**Converter Example**:

```go
// Proxy → ITX
func ConvertCreateITXMeetingPayloadToDomain(p *goa.Payload) *models.Request {
    return &models.Request{
        Project:   p.ProjectUID,      // project_uid → project
        Topic:     p.Title,            // title → topic
        Agenda:    p.Description,      // description → agenda
    }
}

// ITX → Proxy
func ConvertITXMeetingResponseToGoa(resp *itx.Response) *goa.Response {
    return &goa.Response{
        ProjectUID:  resp.Project,     // project → project_uid
        Title:       resp.Topic,       // topic → title
        Description: resp.Agenda,      // agenda → description
    }
}
```

### 5. Infrastructure Layer

**LFX v2**: Multiple infrastructure components

```go
// Repository for NATS storage
type Repository interface {
    CreateMeeting(ctx context.Context, meeting *models.Meeting) error
    GetMeeting(ctx context.Context, uid string) (*models.Meeting, error)
    // ... CRUD operations
}

// Zoom client for direct Zoom API calls
type ZoomClient interface {
    CreateMeeting(ctx context.Context, req *ZoomRequest) (*ZoomResponse, error)
    // ... Zoom operations
}

// Messaging for NATS publish
type Publisher interface {
    PublishMeetingCreated(ctx context.Context, event *Event) error
    // ... event publishing
}
```

**ITX Proxy**: Single HTTP client

```go
// Simple HTTP proxy client
type ITXProxyClient interface {
    CreateZoomMeeting(ctx context.Context, req *Request) (*Response, error)
    GetZoomMeeting(ctx context.Context, meetingID string) (*Response, error)
    UpdateZoomMeeting(ctx context.Context, meetingID string, req *Request) error
    DeleteZoomMeeting(ctx context.Context, meetingID string) error
    // ... proxy operations
}
```

**ITX Client Implementation**: [internal/infrastructure/proxy/client.go](../internal/infrastructure/proxy/client.go)

```go
func (c *Client) CreateZoomMeeting(ctx context.Context, req *Request) (*Response, error) {
    // 1. Marshal request to JSON
    // 2. Create HTTP request to ITX service
    // 3. Add OAuth2 M2M token (automatic via transport)
    // 4. Add x-scope: manage:zoom header
    // 5. Execute HTTP request
    // 6. Parse response
    // 7. Map HTTP errors to domain errors
}
```

### 6. Error Handling

Both use domain errors but map from different sources:

**LFX v2**: Maps from NATS and Zoom API errors

```go
// From NATS KeyValue errors
if errors.Is(err, nats.ErrKeyNotFound) {
    return domain.NewNotFoundError("meeting not found")
}

// From Zoom API errors
if zoomErr.StatusCode == 404 {
    return domain.NewNotFoundError("Zoom meeting not found")
}
```

**ITX Proxy**: Maps from ITX HTTP status codes

```go
func (c *Client) mapHTTPError(statusCode int, body []byte) error {
    switch statusCode {
    case http.StatusBadRequest:
        return domain.NewBadRequestError(message)
    case http.StatusNotFound:
        return domain.NewNotFoundError(message)
    case http.StatusConflict:
        return domain.NewConflictError(message)
    // ... etc
    }
}
```

### 7. Endpoint Paths

| Resource | LFX v2 Path | ITX Proxy Path |
|----------|-------------|----------------|
| Meetings | `/v2/meetings` | `/itx/meetings` |
| Registrants | `/v2/meetings/{uid}/registrants` | `/itx/meetings/{id}/registrants` |
| Occurrences | `/v2/meetings/{uid}/occurrences` | `/itx/meetings/{id}/occurrences` |

Note: LFX v2 uses UUIDs, ITX uses Zoom meeting IDs

### 8. Authentication & Headers

**LFX v2**:

```
Authorization: Bearer <jwt_token>
X-Sync: true|false (optional)
```

**ITX Proxy** (Client → Service):

```
Authorization: Bearer <jwt_token>
X-Sync: true|false (optional)
```

**ITX Proxy** (Service → ITX):

```
Authorization: Bearer <oauth2_m2m_token>  (added by proxy)
x-scope: manage:zoom                       (added by proxy)
```

---

## Data Flow

### LFX v2 Meeting Creation Flow

```
1. Client Request
   POST /v2/meetings
   ↓
2. API Handler (api_meetings.go)
   CreateMeeting()
   ↓
3. Converter
   ConvertCreatePayloadToDomain()
   ↓
4. Service Layer (service/meetings/)
   CreateMeeting()
   ├─→ Validate request
   ├─→ Store in NATS KV
   ├─→ Create Zoom meeting (if platform="Zoom")
   ├─→ Publish NATS message
   └─→ Return meeting
   ↓
5. Converter
   ConvertMeetingToGoa()
   ↓
6. API Response
   201 Created
```

### ITX Proxy Meeting Creation Flow

```
1. Client Request
   POST /itx/meetings
   ↓
2. API Handler (api_itx_meetings.go)
   CreateItxMeeting()
   ↓
3. Converter (field mapping)
   ConvertCreateITXMeetingPayloadToDomain()
   ├─→ project_uid → project
   ├─→ title → topic
   ├─→ description → agenda
   └─→ committees[].uid → committees[].id
   ↓
4. Service Layer (service/itx/)
   CreateMeeting()
   └─→ Call proxy client
   ↓
5. Proxy Client (infrastructure/proxy/)
   CreateZoomMeeting()
   ├─→ Marshal to JSON
   ├─→ HTTP POST to ITX service
   ├─→ Add OAuth2 token (automatic)
   ├─→ Add x-scope header
   └─→ Parse response
   ↓
6. ITX Service
   POST /v2/zoom/meetings
   ↓
7. Zoom API
   Creates meeting
   ↓
8. Response flows back
   ↓
9. Converter (field mapping)
   ConvertITXMeetingResponseToGoa()
   ├─→ project → project_uid
   ├─→ topic → title
   ├─→ agenda → description
   └─→ committees[].id → committees[].uid
   ↓
10. API Response
    201 Created
```

---

## Key Architectural Decisions

### Why Separate ITX Proxy Endpoints?

1. **Different Storage Model**: LFX v2 uses NATS persistence, ITX is stateless
2. **Different Data Model**: LFX v2 has richer meeting model with settings, attachments, etc.
3. **Different Integration**: LFX v2 can support multiple platforms, ITX is Zoom-only via ITX service
4. **Separation of Concerns**: Clean separation between persisted meetings and proxied meetings
5. **Independent Evolution**: ITX proxy can evolve independently of LFX v2 features

### Why Use ITX Proxy?

1. **Simplified Zoom Integration**: ITX service handles OAuth2 M2M and Zoom API complexity
2. **Centralized Zoom Management**: ITX service manages Zoom credentials and rate limiting
3. **No Local State**: Meetings are managed entirely by ITX/Zoom
4. **Faster Implementation**: Thin proxy requires minimal business logic

### Why Keep LFX v2 Endpoints?

1. **Full Feature Set**: Supports attachments, settings, organizers, email notifications
2. **Platform Flexibility**: Can support multiple meeting platforms
3. **Event Processing**: Handles webhooks and publishes indexer events
4. **Rich Querying**: NATS storage enables complex queries and filtering
5. **Audit Trail**: Full meeting history and changes tracked

---

## Testing Strategy

### LFX v2 Testing

- Unit tests for service layer with mock repository
- Unit tests for Zoom client with mock HTTP
- Integration tests with NATS test server
- Mock webhook tests

### ITX Proxy Testing

- Unit tests for service layer with mock proxy client
- Unit tests for proxy client with mock HTTP server
- Mock ITX service responses
- Field mapping validation tests

**Example Test**:

```go
// ITX Proxy converter test
func TestConvertCreateITXMeetingPayloadToDomain(t *testing.T) {
    payload := &goa.CreateItxMeetingPayload{
        ProjectUID:  "project-123",
        Title:       "Team Meeting",
        Description: "Weekly sync",
    }

    req := ConvertCreateITXMeetingPayloadToDomain(payload)

    assert.Equal(t, "project-123", req.Project)      // project_uid → project
    assert.Equal(t, "Team Meeting", req.Topic)       // title → topic
    assert.Equal(t, "Weekly sync", req.Agenda)       // description → agenda
}
```

---

## Configuration

### Environment Variables

**LFX v2** (requires Zoom credentials for direct integration):

```bash
ZOOM_ACCOUNT_ID=<zoom-account-id>
ZOOM_CLIENT_ID=<zoom-client-id>
ZOOM_CLIENT_SECRET=<zoom-client-secret>
NATS_URL=nats://localhost:4222
```

**ITX Proxy** (requires ITX service configuration):

```bash
ITX_BASE_URL=https://api.dev.itx.linuxfoundation.org
ITX_CLIENT_ID=<oauth2-client-id>
ITX_CLIENT_SECRET=<oauth2-client-secret>
ITX_AUDIENCE=https://api.dev.itx.linuxfoundation.org/
```

### Helm Configuration

Both endpoint families are configured in the same Helm chart:

**File**: [charts/lfx-v2-meeting-service/values.yaml](../charts/lfx-v2-meeting-service/values.yaml)

```yaml
app:
  environment:
    # LFX v2 configuration (Zoom direct integration)
    ZOOM_ACCOUNT_ID:
      value: null
    ZOOM_CLIENT_ID:
      value: null
    ZOOM_CLIENT_SECRET:
      value: null
    ZOOM_WEBHOOK_SECRET_TOKEN:
      value: null

    # NATS configuration (used by LFX v2 endpoints)
    NATS_URL:
      value: nats://lfx-platform-nats.lfx.svc.cluster.local:4222

# Note: ITX proxy configuration is currently managed through environment variables
# that are not yet defined in values.yaml. The service expects these environment variables:
# - ITX_BASE_URL: Base URL for ITX service
# - ITX_CLIENT_ID: OAuth2 client ID for ITX authentication
# - ITX_CLIENT_SECRET: OAuth2 client secret for ITX authentication
# - ITX_AUDIENCE: OAuth2 audience for ITX token requests
```

---

## Summary

| Aspect | LFX v2 Endpoints | ITX Proxy Endpoints |
|--------|------------------|---------------------|
| **Purpose** | Full-featured meeting management | Lightweight Zoom proxy |
| **Storage** | NATS KV Store | None (stateless) |
| **Platform Support** | Multiple (Zoom, others) | Zoom only (via ITX) |
| **Business Logic** | Complex | Minimal |
| **Field Mapping** | None needed | Required (uid/id, title/topic, etc.) |
| **Infrastructure** | NATS + Zoom API + Messaging | HTTP proxy client only |
| **Features** | Attachments, settings, webhooks, email | Basic CRUD only |
| **State** | Stateful | Stateless |
| **Implementation** | ~5000 LOC | ~1000 LOC |

Both implementations coexist in the same service, sharing:

- Authentication (Heimdall JWT)
- Authorization (OpenFGA)
- Error handling patterns
- Goa framework
- API handler structure

They differ in storage, business logic complexity, and integration approach, making them suitable for different use cases within the LFX ecosystem.
