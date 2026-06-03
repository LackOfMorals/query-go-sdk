# Architecture Overview

## Layered Service Architecture

The Aura API client follows a clean, layered architecture that separates concerns and promotes code reuse:

```
┌─────────────────────────────────────────────────────┐
│              Client (AuraAPIClient)                  │
│  - Tenants, Instances, Snapshots, CMEK, GDS,       │
│    Prometheus Services                              │
└───────────────────┬─────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────┐
│           API Service (RequestService)            │
│  - Handles OAuth authentication                      │
│  - Token management and refresh                      │
│  - Request/response handling                         │
│  - Supports both relative paths and full URLs       │
└───────────────────┬─────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────┐
│           HTTP Service (HTTPService)                 │
│  - Low-level HTTP operations                         │
│  - Retry logic and connection pooling                │
│  - Auto-detects full URLs vs relative paths         │
│  - Timeout management                                │
└─────────────────────────────────────────────────────┘
```

## Key Design Principles

### 1. Automatic URL Handling

The HTTP service automatically detects whether an endpoint is a full URL or a relative path:

```go
// Relative path - gets base URL prepended
resp := httpSvc.Get(ctx, "v1/instances", headers)
// → https://api.neo4j.io/v1/instances

// Full URL - used as-is
resp := httpSvc.Get(ctx, "https://prometheus.example.com/api/v1/query", headers)
// → https://prometheus.example.com/api/v1/query
```

This eliminates the need for special-case methods and keeps the API consistent.

### 2. Unified Authentication

All services use the same API service layer for authentication:

```go
// Aura API endpoint (relative path)
resp := apiSvc.Get(ctx, "instances")
// → Authenticates → Prepends API version → Calls HTTP service

// Prometheus endpoint (full URL)
resp := apiSvc.Get(ctx, "https://c9f0d13a.metrics.neo4j.io/prometheus/api/v1/query?...")
// → Authenticates → Passes to HTTP service directly
```

Both benefit from:
- Automatic OAuth token management
- Token refresh when expired
- Consistent error handling
- Structured logging

### 3. Service Isolation

Each service is responsible for its domain:

**HTTP Service**
- URL construction (base URL + path OR full URL)
- HTTP communication
- Retry logic
- Connection pooling

**API Service**
- Authentication and authorization
- Token lifecycle management
- Request preparation
- Response validation

**Domain Services** (Instances, Prometheus, etc.)
- Business logic
- Request/response mapping
- Domain-specific validation

## Example: Prometheus Service Flow

When you query Prometheus metrics:

```go
// 1. User calls Prometheus service
health, err := client.Prometheus.GetInstanceHealth(instanceID, prometheusURL)

// 2. Prometheus service constructs full URL
fullURL := prometheusURL + "/api/v1/query?query=up&time=..."

// 3. Calls API service with full URL
resp, err := p.api.Get(p.ctx, fullURL)

// 4. API service adds authentication
headers := map[string]string{
    "Authorization": "Bearer " + token,
    ...
}

// 5. Calls HTTP service
resp, err := s.httpClient.Get(ctx, fullURL, headers)

// 6. HTTP service detects full URL and uses it as-is
if strings.HasPrefix(fullURL, "https://") {
    // Use full URL directly
} else {
    // Prepend base URL
}

// 7. Response flows back through layers
```

## Benefits of This Architecture

1. **Code Reuse**: No special methods needed for different URL types
2. **Consistency**: All services use the same interfaces
3. **Maintainability**: Changes to authentication or HTTP handling affect all services
4. **Testability**: Each layer can be mocked independently
5. **Flexibility**: Easy to add new services (internal or external)
6. **Separation of Concerns**: Each layer has a single, clear responsibility

## Adding New Services

To add a new service (e.g., for a third-party API):

```go
type MyService struct {
    api    api.RequestService  // Use the API service
    ctx    context.Context
    logger *slog.Logger
}

func (s *MyService) CallExternalAPI() error {
    // Just pass the full URL - authentication is automatic
    resp, err := s.api.Get(s.ctx, "https://external-api.com/endpoint")
    // ...
}
```

No special setup needed - authentication and URL handling work automatically!

## Backward Compatibility

This architecture maintains full backward compatibility:

- Relative paths work exactly as before (base URL prepended)
- Existing services (Instances, Tenants, etc.) are unaffected
- No breaking changes to public APIs
- Internal refactoring only

## Performance Considerations

- **Token Caching**: OAuth tokens are cached and only refreshed when needed
- **Connection Pooling**: HTTP client reuses connections
- **Concurrent Requests**: Thread-safe token management
- **Timeout Control**: Configurable timeouts at each layer
- **Retry Logic**: Automatic retries for transient failures

## Testing Strategy

Each layer has its own test coverage:

- **HTTP Service**: Tests URL handling, retries, timeouts
- **API Service**: Tests authentication, token refresh
- **Domain Services**: Tests business logic, data mapping

Integration tests verify the full stack works together.
