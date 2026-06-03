# AGENTS.md — aura-client

## Feedback Instructions

### TEST COMMANDS
```
go test -race ./... -timeout 120s
```

### BUILD COMMANDS
```
go build ./...
```

### LINT COMMANDS
```
golangci-lint run
```

### FORMAT COMMANDS
```
go fmt ./...
```

## Architecture

- Package `aura` (root): public API surface — service types, client, options
- `internal/api`: HTTP request service abstraction
- `internal/httpclient`: retryable HTTP client wrapping `retryablehttp`
- `internal/utils`: validation helpers (ValidateInstanceID, ValidateTenantID, ValidateSnapshotID, etc.)
- Each service (instanceService, tenantService, etc.) holds `api api.RequestService`, `timeout time.Duration`, `logger *slog.Logger`
- Services apply `context.WithTimeout` on every method call; cancellation propagates through the HTTP layer

## Test Mocks

- `mockAPIService` — ignores context entirely; use only when context behaviour is irrelevant
- `mockAPIServiceWithDelay` — respects context cancellation/deadline via `executeWithDelay`; use for context tests
- `mockAPIServiceWithCallback` — supports `OnGet`/`OnPost` hooks; use to inspect context values at the API layer
- Tests that assert a pre-cancelled or expired context is rejected MUST use `mockAPIServiceWithDelay` (not `mockAPIService`), because `mockAPIServiceWithDelay.executeWithDelay` calls `ctx.Err()` just as a real HTTP transport would

## Conventions

- Changie fragments go in `.changes/unreleased/`; kinds: `Fixed`, `Changed` (breaking), `Added`
- All service methods validate IDs before calling the API; empty check first, then format check
- Error messages follow the pattern `"invalid <thing> ID: <wrapped error>"`
- Log `ErrorContext` on API errors; `DebugContext` on success paths; `InfoContext` for mutating operations

## Aura API

- Module path: `github.com/neo4j-contrib/aura-go-sdk`
- Base URL: `https://api.neo4j.io`
- API version: `v1` (hardcoded in `client.go`; a future v2 will be a separate module, not a path suffix)
- Auth: OAuth2 client-credentials flow; token endpoint is `POST /oauth/token`
- Pagination: none — all list endpoints return the full result set in one response
- Rate limiting: 429 responses include `Retry-After`; the `retryablehttp` layer handles retries automatically
- Spec: https://neo4j.com/docs/aura/api/specification/

## Known Inconsistencies

- `AuraAPIClient.GraphAnalytics` field (public) vs `GDSSessionService` interface and `gdsSessionService` struct — the field was named before the service was formalised. Do not rename it unilaterally; it is a known breaking-change candidate for a future major version.

## Adding a New Endpoint

Follow the pattern used by `instances.go` / `instances_test.go`. Every new resource or sub-action touches the same five files in the same order.

### 1. Types — `<resource>.go`

Declare all request and response types in the same file as the service. Use a `// === Types ===` comment block to separate them from the service struct.

- Response wrapper: `type List<X>Response struct { Data []<X>Data }` / `type Get<X>Response struct { Data <X>Data }`
- Request body: unexported struct for internal use (e.g. `type overwriteInstanceRequest struct { … }`)
- JSON tags use `snake_case`. Fields that may be absent from the API response use `omitempty`.

### 2. Service struct and methods — `<resource>.go`

```go
type fooService struct {
    api     api.RequestService
    timeout time.Duration
    logger  *slog.Logger
}
```

Every method follows this exact skeleton — do not deviate:

```go
func (s *fooService) DoThing(ctx context.Context, id string) (*DoThingResponse, error) {
    ctx, cancel := context.WithTimeout(ctx, s.timeout)
    defer cancel()

    s.logger.DebugContext(ctx, "doing thing", slog.String("id", id))

    if err := utils.ValidateFooID(id); err != nil {          // validate before every API call
        s.logger.ErrorContext(ctx, "invalid foo ID", slog.String("error", err.Error()))
        return nil, err
    }

    resp, err := s.api.Get(ctx, "foos/"+id)                  // or Post / Patch / Delete
    if err != nil {
        s.logger.ErrorContext(ctx, "failed to do thing", slog.String("id", id), slog.String("error", err.Error()))
        return nil, err
    }

    var result DoThingResponse
    if err := json.Unmarshal(resp.Body, &result); err != nil {
        s.logger.ErrorContext(ctx, "failed to unmarshal response", slog.String("error", err.Error()))
        return nil, err
    }

    s.logger.DebugContext(ctx, "thing done", slog.String("id", id))  // InfoContext for mutations
    return &result, nil
}
```

**Path construction:**
| Pattern | Example |
|---|---|
| Collection | `"instances"` |
| Single resource | `"instances/" + instanceID` |
| Sub-action | `fmt.Sprintf("instances/%s/pause", instanceID)` |

**Logging level on success path:**
- Read operations (List, Get): `DebugContext`
- Mutating operations (Create, Update, Delete, and action sub-paths): `InfoContext`

**ID validation helpers** (`internal/utils`):
- `utils.ValidateInstanceID(id)` — 8-char hex string
- `utils.ValidateTenantID(id)` — standard UUID
- `utils.ValidateSnapshotID(id)` — standard UUID

**Request bodies:** marshal with `json.Marshal`, pass the result as `string(body)` to `api.Post` / `api.Patch`. Use `errors.New` for static error strings; `fmt.Errorf("… %w", err)` to wrap.

### 3. Interface — `interfaces.go`

Add the new method to the relevant service interface. Add (or create) a compile-time check:

```go
var _ FooService = (*fooService)(nil)
```

Adding a method to an existing interface is a **breaking change** — it requires a `Changed` changie entry.

### 4. Client wiring — `client.go`

Add a public field to `AuraAPIClient`:

```go
Foos FooService
```

Wire it in `NewClient` alongside the existing services:

```go
service.Foos = &fooService{
    api:     apiSvc,
    timeout: o.config.apiTimeout,
    logger:  clientLogger.With(slog.String("service", "fooService")),
}
```

Update the `slog.Int("services", N)` count in the `Info` log at the bottom of `NewClient`.

### 5. Tests — `<resource>_test.go`

Add two constructor helpers at the top of the file:

```go
func createTestFooService(mock *mockAPIService) *fooService {
    return &fooService{api: mock, timeout: 30 * time.Second, logger: testLogger()}
}

func createTestFooServiceWithTimeout(mock api.RequestService, timeout time.Duration) *fooService {
    return &fooService{api: mock, timeout: timeout, logger: testLogger()}
}
```

**Required tests per method:**

| Test | Mock to use | What to assert |
|---|---|---|
| `_Success` | `mockAPIService` | correct HTTP method, exact path, response fields correctly mapped |
| `_InvalidID` (table-driven) | `mockAPIService` | empty / too-short / bad-chars all return errors without calling API |
| `_NotFound` | `mockAPIService` with `err: &api.Error{StatusCode: 404, …}` | error type is `*api.Error`, `IsNotFound()` true |
| `_AuthenticationError` | `mockAPIService` with `err: &api.Error{StatusCode: 401, …}` | `IsUnauthorized()` true |
| `_EmptyResult` (List only) | `mockAPIService` | empty slice returned, no error |
| `_ContextTimeout` | `mockAPIServiceWithDelay` with delay > service timeout | `errors.Is(err, context.DeadlineExceeded)` |
| `_QuickCancellation` | `mockAPIServiceWithDelay`, pre-expired context | `context.DeadlineExceeded` or `context.Canceled` |

For POST/PATCH methods also unmarshal `mock.lastBody` and assert request fields were serialised correctly.

Use `mockAPIServiceWithDelay` (never `mockAPIService`) whenever the test must prove a context error is returned.

### 6. README.md

Add a new H2 section `## Foo Operations` to `README.md` and a matching entry in the Table of Contents at the top of the file. Each operation gets a fenced Go code block showing a minimal working call. Follow the style and verbosity of the existing `## Instance Operations` section exactly — no prose explanation inside the code block, no extra context beyond what's needed to call the method.

## PR Conventions

- One logical change per PR; breaking changes (renamed types, removed fields, interface additions) get their own PR
- A changie fragment in `.changes/unreleased/` is required for every user-visible change — run `changie new` and commit the generated YAML alongside your code
- PRs that touch only docs, CI, or tests may add the `no-changelog` label to bypass the changelog check workflow
- Required checks before review: `go test -race ./...`, `golangci-lint run`, `go build ./...` (includes `examples/`)
