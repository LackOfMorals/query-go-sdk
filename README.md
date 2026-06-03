# Aura API Client

## Overview

A Go package that enables the use of Neo4j Aura API in a friendly way e.g `client.Instances.List(ctx)` to return a list of instances in Aura.

Client Id and Secret are required and these can be obtained from the [Neo4j Aura Console](https://neo4j.com/docs/aura/api/authentication/).

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Context and Timeouts](#context-and-timeouts)
- [Tenant Operations](#tenant-operations)
- [Instance Operations](#instance-operations)
- [Snapshot Operations](#snapshot-operations)
- [CMEK Operations](#cmek-operations)
- [GDS Session Operations](#gds-session-operations)
- [Prometheus Metrics Operations](#prometheus-metrics-operations)
- [Error Handling](#error-handling)
- [Best Practices](#best-practices)
- [CI & Releases](#ci--releases)

---

## Installation

```bash
go get github.com/neo4j-contrib/aura-go-sdk
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    aura "github.com/neo4j-contrib/aura-go-sdk"
)

func main() {
    client, err := aura.NewClient(
        aura.WithCredentials("your-client-id", "your-client-secret"),
    )
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }
    defer client.Close()

    ctx := context.Background()

    instances, err := client.Instances.List(ctx)
    if err != nil {
        log.Fatalf("Failed to list instances: %v", err)
    }

    for _, instance := range instances.Data {
        log.Printf("Instance: %s (ID: %s)\n", instance.Name, instance.ID)
    }
}
```

---

## Configuration

### Simple Configuration

```go
client, err := aura.NewClient(
    aura.WithCredentials("client-id", "client-secret"),
)
```

### Advanced Configuration

```go
client, err := aura.NewClient(
    aura.WithCredentials("client-id", "client-secret"),
    aura.WithTimeout(60 * time.Second),
    aura.WithMaxRetry(5),
)
```

### Custom Logger

```go
import "log/slog"

opts := &slog.HandlerOptions{Level: slog.LevelDebug}
handler := slog.NewTextHandler(os.Stderr, opts)
logger := slog.New(handler)

client, err := aura.NewClient(
    aura.WithCredentials("client-id", "client-secret"),
    aura.WithLogger(logger),
)
```

### Targeting a Different Base URL

Use `WithBaseURL` to point the client at a staging or sandbox environment:

```go
client, err := aura.NewClient(
    aura.WithCredentials("client-id", "client-secret"),
    aura.WithBaseURL("https://api.staging.neo4j.io"),
)
```

### Custom HTTP Transport

Use `WithHTTPClient` to inject a custom `*http.Client`. This is useful for
configuring mTLS, HTTP proxies, or controlling low-level transport settings:

```go
import "net/http"

transport := &http.Transport{
    MaxIdleConns:    100,
    IdleConnTimeout: 90 * time.Second,
}
httpClient := &http.Client{Transport: transport}

client, err := aura.NewClient(
    aura.WithCredentials("client-id", "client-secret"),
    aura.WithHTTPClient(httpClient),
)
```

### Custom User-Agent

Use `WithUserAgent` to override the default `User-Agent` header. This is
useful when your application needs to be identifiable in API server logs:

```go
client, err := aura.NewClient(
    aura.WithCredentials("client-id", "client-secret"),
    aura.WithUserAgent("my-app/2.1.0"),
)
```

### Default Headers

Use `WithDefaultHeaders` to attach custom headers to every API request.
`Authorization`, `Content-Type`, and `User-Agent` are silently ignored to
prevent accidental overrides of security-critical headers:

```go
client, err := aura.NewClient(
    aura.WithCredentials("client-id", "client-secret"),
    aura.WithDefaultHeaders(map[string]string{
        "X-Request-Source": "my-service",
        "X-Correlation-ID": "abc-123",
    }),
)
```

---

## Context and Timeouts

Every service method accepts a `context.Context` as its first argument. This is the standard Go pattern and gives you full control over cancellation and deadlines on a per-call basis.

The client is configured with a default timeout (120 seconds, overridable with `WithTimeout`). This timeout is applied as a ceiling on each call — if the context you pass already has a shorter deadline, that shorter deadline wins.

### Basic usage

```go
ctx := context.Background()
instances, err := client.Instances.List(ctx)
```

### Per-call deadline

```go
// This specific call must complete within 10 seconds
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

instance, err := client.Instances.Get(ctx, "instance-id")
```

### Cancellation

```go
ctx, cancel := context.WithCancel(context.Background())

// Cancel all in-flight calls (e.g. on OS signal or user action)
go func() {
    <-shutdownSignal
    cancel()
}()

instances, err := client.Instances.List(ctx)
if err != nil {
    if ctx.Err() == context.Canceled {
        log.Println("Request was cancelled")
    }
}
```

### Distributed tracing

Because context flows through every call, you can attach trace spans from any OpenTelemetry-compatible library:

```go
ctx, span := tracer.Start(r.Context(), "list-instances")
defer span.End()

instances, err := client.Instances.List(ctx)
```

---

## Tenant Operations

### List All Tenants

```go
ctx := context.Background()

tenants, err := client.Tenants.List(ctx)
if err != nil {
    log.Fatalf("Error: %v", err)
}

for _, tenant := range tenants.Data {
    fmt.Printf("Tenant: %s (ID: %s)\n", tenant.Name, tenant.ID)
}
```

### Get Tenant Details

```go
ctx := context.Background()

tenant, err := client.Tenants.Get(ctx, "your-tenant-id")
if err != nil {
    log.Fatalf("Error: %v", err)
}

fmt.Printf("Tenant: %s\n", tenant.Data.Name)
fmt.Printf("Available instance configurations:\n")

for _, config := range tenant.Data.InstanceConfigurations {
    fmt.Printf("  - %s in %s: %s memory, Type: %s\n",
        config.CloudProvider,
        config.RegionName,
        config.Memory,
        config.Type,
    )
}
```

---

## Instance Operations

### List All Instances

```go
ctx := context.Background()

instances, err := client.Instances.List(ctx)
if err != nil {
    log.Fatalf("Error: %v", err)
}

fmt.Printf("Found %d instances:\n", len(instances.Data))
for _, instance := range instances.Data {
    fmt.Printf("  - %s (ID: %s) on %s\n",
        instance.Name,
        instance.ID,
        instance.CloudProvider,
    )
}
```

### Get Instance Details

```go
ctx := context.Background()

instance, err := client.Instances.Get(ctx, "your-instance-id")
if err != nil {
    log.Fatalf("Error: %v", err)
}

fmt.Printf("Instance: %s\n", instance.Data.Name)
fmt.Printf("Status: %s\n", instance.Data.Status)
fmt.Printf("Connection URL: %s\n", instance.Data.ConnectionURL)
fmt.Printf("Memory: %s\n", instance.Data.Memory)
fmt.Printf("Type: %s\n", instance.Data.Type)
fmt.Printf("Region: %s\n", instance.Data.Region)
if instance.Data.CreatedAt != nil {
    fmt.Printf("Created At: %s\n", instance.Data.CreatedAt.Format(time.RFC3339))
}
```

### Create a New Instance

```go
ctx := context.Background()

config := &aura.CreateInstanceConfigData{
    Name:          "my-neo4j-db",
    TenantID:      "your-tenant-id",
    CloudProvider: "gcp",
    Region:        "europe-west1",
    Type:          "enterprise-db",
    Version:       "5",
    Memory:        "8GB",
}

instance, err := client.Instances.Create(ctx, config)
if err != nil {
    log.Fatalf("Error creating instance: %v", err)
}

fmt.Printf("Instance created!\n")
fmt.Printf("  ID: %s\n", instance.Data.ID)
fmt.Printf("  Connection URL: %s\n", instance.Data.ConnectionURL)
fmt.Printf("  Username: %s\n", instance.Data.Username)
fmt.Printf("  Password: %s\n", instance.Data.Password)

// ⚠️ IMPORTANT: Save these credentials securely!
// The password is only shown once during creation.
```

The optional fields `VectorOptimized`, `GraphAnalyticsPlugin`, and `CustomerManagedKeyID` can also be set at creation time:

```go
config := &aura.CreateInstanceConfigData{
    Name:                 "my-neo4j-db",
    TenantID:             "your-tenant-id",
    CloudProvider:        "gcp",
    Region:               "europe-west1",
    Type:                 "enterprise-db",
    Version:              "5",
    Memory:               "8GB",
    VectorOptimized:      true,
    GraphAnalyticsPlugin: true,
    CustomerManagedKeyID: "your-cmk-id",  // enterprise tiers only
}
```

### Clone an Instance from Another Instance

Creates a new instance pre-loaded with the data from the latest snapshot of an existing instance.

```go
ctx := context.Background()

config := &aura.CreateInstanceConfigData{
    Name:          "my-clone",
    TenantID:      "your-tenant-id",
    CloudProvider: "gcp",
    Region:        "europe-west1",
    Type:          "enterprise-db",
    Version:       "5",
    Memory:        "8GB",
}

instance, err := client.Instances.CreateFromInstance(ctx, "source-instance-id", config)
if err != nil {
    log.Fatalf("Error: %v", err)
}

fmt.Printf("Clone created: %s\n", instance.Data.ID)
```

### Clone an Instance from a Snapshot

Creates a new instance from a specific exportable snapshot. Both the source instance ID and the snapshot ID are required.

```go
ctx := context.Background()

config := &aura.CreateInstanceConfigData{
    Name:          "my-snapshot-clone",
    TenantID:      "your-tenant-id",
    CloudProvider: "gcp",
    Region:        "europe-west1",
    Type:          "enterprise-db",
    Version:       "5",
    Memory:        "8GB",
}

instance, err := client.Instances.CreateFromSnapshot(ctx, "source-instance-id", "snapshot-id", config)
if err != nil {
    log.Fatalf("Error: %v", err)
}

fmt.Printf("Clone created: %s\n", instance.Data.ID)
```

### Update an Instance

```go
ctx := context.Background()

updateData := &aura.UpdateInstanceData{
    Name:              "my-renamed-instance",
    Memory:            "16GB",
    CDCEnrichmentMode: "FULL",   // "" | "DIFF" | "FULL"
    SecondariesCount:  2,
}

instance, err := client.Instances.Update(ctx, "instance-id", updateData)
if err != nil {
    log.Fatalf("Error: %v", err)
}

fmt.Printf("Instance updated: %s with %s memory\n",
    instance.Data.Name,
    instance.Data.Memory,
)
```

### Pause an Instance

```go
ctx := context.Background()

instance, err := client.Instances.Pause(ctx, "instance-id")
if err != nil {
    log.Fatalf("Error: %v", err)
}

fmt.Printf("Instance paused. Status: %s\n", instance.Data.Status)
```

### Resume an Instance

```go
ctx := context.Background()

instance, err := client.Instances.Resume(ctx, "instance-id")
if err != nil {
    log.Fatalf("Error: %v", err)
}

fmt.Printf("Instance resumed. Status: %s\n", instance.Data.Status)
```

### Delete an Instance

```go
ctx := context.Background()

// ⚠️ WARNING: This is irreversible!
instance, err := client.Instances.Delete(ctx, "instance-to-delete")
if err != nil {
    log.Fatalf("Error: %v", err)
}

fmt.Printf("Instance %s deleted\n", instance.Data.ID)
```

### Overwrite Instance from Another Instance

```go
ctx := context.Background()

result, err := client.Instances.OverwriteFromInstance(ctx, "target-instance-id", "source-instance-id")
if err != nil {
    log.Fatalf("Error: %v", err)
}

fmt.Printf("Overwrite initiated: %s\n", result.Data)
// Note: This is asynchronous. Monitor instance status.
```

### Overwrite Instance from Snapshot

```go
ctx := context.Background()

result, err := client.Instances.OverwriteFromSnapshot(ctx, "target-instance-id", "snapshot-id")
if err != nil {
    log.Fatalf("Error: %v", err)
}

fmt.Printf("Overwrite from snapshot initiated\n")
```

---

## Snapshot Operations

### List Snapshots
Snapshots.List accepts an optional filter to return snapshots for a particular day.  If this is not given , nil is used instead, then snapshots for the current day are returned. 

The date is of type SnapshotDate that holds the Year, Month and Day.  For example, to see snapshots for 23rd March 2026

filter := aura.SnapshotDate{Year: 2026, Month: time.March, Day: 23})

Then call List 

snapshots, err := client.Snapshots.List(ctx, "your-instance-id", &filter )


```go
ctx := context.Background()

// Empty date string returns today's snapshots
snapshots, err := client.Snapshots.List(ctx, "your-instance-id", nil)
if err != nil {
    log.Fatalf("Error: %v", err)
}

fmt.Printf("Found %d snapshots:\n", len(snapshots.Data))
for _, snapshot := range snapshots.Data {
    fmt.Printf("  - ID: %s, Profile: %s, Status: %s\n",
        snapshot.SnapshotID,
        snapshot.Profile,
        snapshot.Status,
    )
}
```

### List Snapshots for a Specific Date

```go
ctx := context.Background()

snapshots, err := client.Snapshots.List(ctx, "your-instance-id", &aura.SnapshotDate{Year: 2026, Month: time.March, Day: 23})
if err != nil {
    log.Fatalf("Error: %v", err)
}

for _, snapshot := range snapshots.Data {
    fmt.Printf("  - %s at %s\n", snapshot.SnapshotID, snapshot.Timestamp.Format(time.RFC3339))
}
```

### Get Snapshot Details

```go
ctx := context.Background()

snapshot, err := client.Snapshots.Get(ctx, "your-instance-id", "your-snapshot-id")
if err != nil {
    log.Fatalf("Error: %v", err)
}

fmt.Printf("Instance ID: %s\nSnapshot ID: %s\nStatus: %s\nTimestamp: %s\n",
    snapshot.Data.InstanceID,
    snapshot.Data.SnapshotID,
    snapshot.Data.Status,
    snapshot.Data.Timestamp.Format(time.RFC3339),
)
```

### Create an On-Demand Snapshot

```go
ctx := context.Background()

snapshot, err := client.Snapshots.Create(ctx, "your-instance-id")
if err != nil {
    log.Fatalf("Error: %v", err)
}

fmt.Printf("Snapshot creation initiated. Snapshot ID: %s\n", snapshot.Data.SnapshotID)
// Note: Snapshot creation is asynchronous. Poll List() to check completion status.
```

### Restore from a Snapshot

```go
ctx := context.Background()

result, err := client.Snapshots.Restore(ctx, "your-instance-id", "your-snapshot-id")
if err != nil {
    log.Fatalf("Error: %v", err)
}

fmt.Printf("Instance ID: %s\nStatus: %s\n", result.Data.ID, result.Data.Status)
```

---

## CMEK Operations

### List Customer Managed Encryption Keys

```go
ctx := context.Background()

// Pass an empty string to list all CMEKs regardless of tenant
cmeks, err := client.CMEK.List(ctx, "")
if err != nil {
    log.Fatalf("Error: %v", err)
}

for _, cmek := range cmeks.Data {
    fmt.Printf("  - %s (ID: %s) in tenant %s\n", cmek.Name, cmek.ID, cmek.TenantID)
}
```

### Filter CMEKs by Tenant

```go
ctx := context.Background()

cmeks, err := client.CMEK.List(ctx, "your-tenant-id")
if err != nil {
    log.Fatalf("Error: %v", err)
}

for _, cmek := range cmeks.Data {
    fmt.Printf("  - %s\n", cmek.Name)
}
```

---

## GDS Session Operations

### List Graph Data Science Sessions

```go
ctx := context.Background()

sessions, err := client.GraphAnalytics.List(ctx)
if err != nil {
    log.Fatalf("Error: %v", err)
}

for _, session := range sessions.Data {
    fmt.Printf("  - %s (ID: %s)\n", session.Name, session.ID)
    fmt.Printf("    Memory: %s, Status: %s\n", session.Memory, session.Status)
    fmt.Printf("    Instance: %s, Expires: %s\n", session.InstanceID, session.ExpiresAt.Format(time.RFC3339))
}
```

---

## Prometheus Metrics Operations

Each Aura instance exposes Prometheus metrics for monitoring.

### Get the Prometheus URL for an Instance

```go
ctx := context.Background()

instance, err := client.Instances.Get(ctx, "your-instance-id")
if err != nil {
    log.Fatalf("Error: %v", err)
}

prometheusURL := instance.Data.MetricsURL
```

### Get Instance Health Metrics

```go
ctx := context.Background()

health, err := client.Prometheus.GetInstanceHealth(ctx, "your-instance-id", prometheusURL)
if err != nil {
    log.Fatalf("Error: %v", err)
}

// OverallStatus is one of: "healthy", "warning", or "critical".
// "warning"  — one or more metrics are elevated; monitor closely.
// "critical" — one or more metrics have breached a severe threshold
//              and immediate action is recommended.
fmt.Printf("Health Status: %s\n", health.OverallStatus)
fmt.Printf("CPU Usage: %.2f%%\n", health.Resources.CPUUsagePercent)
fmt.Printf("Memory Usage: %.2f%%\n", health.Resources.MemoryUsagePercent)
fmt.Printf("Total Queries: %.0f\n", health.Query.QueryExecutionTotal)

if health.Connections.MaxConnections > 0 {
    fmt.Printf("Active Connections: %d/%d (%.1f%%)\n",
        health.Connections.ActiveConnections,
        health.Connections.MaxConnections,
        health.Connections.UsagePercent,
    )
} else {
    fmt.Printf("Active Connections: %d (max unknown for this plan)\n",
        health.Connections.ActiveConnections,
    )
}

if len(health.Issues) > 0 {
    fmt.Println("\nIssues detected:")
    for _, issue := range health.Issues {
        fmt.Printf("  - %s\n", issue)
    }
}

if len(health.Recommendations) > 0 {
    fmt.Println("\nRecommendations:")
    for _, rec := range health.Recommendations {
        fmt.Printf("  - %s\n", rec)
    }
}
```

For more detailed information on Prometheus operations, see the [Prometheus documentation](./docs/prometheus.md).

---

## Error Handling

### Basic Error Handling

```go
ctx := context.Background()

instance, err := client.Instances.Get(ctx, "instance-id")
if err != nil {
    log.Printf("Error: %v\n", err)
    return
}
```

### Typed API Errors

```go
ctx := context.Background()

instance, err := client.Instances.Get(ctx, "non-existent-id")
if err != nil {
    if apiErr, ok := err.(*aura.Error); ok {
        fmt.Printf("API Error %d: %s\n", apiErr.StatusCode, apiErr.Message)

        switch {
        case apiErr.IsNotFound():
            fmt.Println("Instance not found")
        case apiErr.IsUnauthorized():
            fmt.Println("Authentication failed - check credentials")
        case apiErr.IsBadRequest():
            fmt.Println("Invalid request parameters")
        }

        if apiErr.HasMultipleErrors() {
            fmt.Println("All errors:")
            for _, msg := range apiErr.AllErrors() {
                fmt.Printf("  - %s\n", msg)
            }
        }
        return
    }

    log.Printf("Unexpected error: %v\n", err)
    return
}
```

### Context Errors

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

instances, err := client.Instances.List(ctx)
if err != nil {
    switch ctx.Err() {
    case context.DeadlineExceeded:
        log.Println("Request timed out")
    case context.Canceled:
        log.Println("Request was cancelled")
    default:
        log.Printf("Error: %v\n", err)
    }
    return
}
```

---

## Best Practices

### 1. Secure Credential Management

```go
clientID := os.Getenv("AURA_CLIENT_ID")
clientSecret := os.Getenv("AURA_CLIENT_SECRET")

if clientID == "" || clientSecret == "" {
    log.Fatal("Missing AURA credentials in environment")
}

client, err := aura.NewClient(
    aura.WithCredentials(clientID, clientSecret),
)
```

### 2. Save Instance Credentials Immediately After Creation

```go
ctx := context.Background()

instance, err := client.Instances.Create(ctx, config)
if err != nil {
    log.Fatal(err)
}

// ⚠️ CRITICAL: Save these immediately — they are only shown once!
credentials := map[string]string{
    "instance_id":    instance.Data.ID,
    "connection_url": instance.Data.ConnectionURL,
    "username":       instance.Data.Username,
    "password":       instance.Data.Password,
}
// Store in a secrets manager. Do NOT log passwords in production.
```

### 3. Polling for Async Operations

```go
ctx := context.Background()

instanceID := newInstance.Data.ID

for range 30 {
    inst, err := client.Instances.Get(ctx, instanceID)
    if err != nil {
        log.Printf("Error checking status: %v", err)
    } else if inst.Data.Status == aura.StatusRunning {
        fmt.Println("Instance is ready!")
        break
    } else {
        fmt.Printf("Status: %s, waiting...\n", inst.Data.Status)
    }
    time.Sleep(10 * time.Second)
}
```

### 4. Graceful Shutdown

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

go func() {
    <-sigChan
    fmt.Println("\nShutting down gracefully...")
    cancel()
}()

// Pass ctx to any in-flight calls — they will be cancelled on signal
instances, err := client.Instances.List(ctx)
```

### 5. Retry Logic for Transient Failures

```go
func retryOperation(maxRetries int, fn func() error) error {
    var err error
    for i := range maxRetries {
        err = fn()
        if err == nil {
            return nil
        }

        if apiErr, ok := err.(*aura.Error); ok {
            // Don't retry client errors (4xx except 429 Too Many Requests)
            if apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 && apiErr.StatusCode != 429 {
                return err
            }
        }

        wait := time.Duration(math.Pow(2, float64(i))) * time.Second
        fmt.Printf("Attempt %d failed, retrying in %v...\n", i+1, wait)
        time.Sleep(wait)
    }
    return fmt.Errorf("operation failed after %d retries: %w", maxRetries, err)
}

// Usage
ctx := context.Background()
err := retryOperation(3, func() error {
    _, err := client.Instances.List(ctx)
    return err
})
```

---

## Complete Example Application

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "time"

    aura "github.com/neo4j-contrib/aura-go-sdk"
)

func main() {
    clientID := os.Getenv("AURA_CLIENT_ID")
    clientSecret := os.Getenv("AURA_CLIENT_SECRET")
    tenantID := os.Getenv("AURA_TENANT_ID")

    if clientID == "" || clientSecret == "" {
        log.Fatal("Missing required environment variables")
    }

    client, err := aura.NewClient(
        aura.WithCredentials(clientID, clientSecret),
        aura.WithTimeout(120 * time.Second),
    )
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }

    ctx := context.Background()

    fmt.Println("=== Current Instances ===")
    instances, err := client.Instances.List(ctx)
    if err != nil {
        log.Fatalf("Failed to list instances: %v", err)
    }

    for _, inst := range instances.Data {
        fmt.Printf("- %s: %s (%s)\n", inst.Name, inst.ID, inst.CloudProvider)
    }

    if tenantID != "" {
        fmt.Println("\n=== Tenant Configuration ===")
        tenant, err := client.Tenants.Get(ctx, tenantID)
        if err != nil {
            log.Printf("Warning: Could not get tenant: %v", err)
        } else {
            fmt.Printf("Tenant: %s\n", tenant.Data.Name)
            fmt.Printf("Available configurations: %d\n", len(tenant.Data.InstanceConfigurations))
        }
    }

    fmt.Println("\n✓ Client is working correctly!")
}
```

Run with:
```bash
export AURA_CLIENT_ID="your-client-id"
export AURA_CLIENT_SECRET="your-client-secret"
export AURA_TENANT_ID="your-tenant-id"
go run main.go
```

---

## Additional Resources

- [Neo4j Aura API Documentation](https://neo4j.com/docs/aura/platform/api/)
- [GitHub Repository](https://github.com/neo4j-contrib/aura-go-sdk)
- [Report Issues](https://github.com/neo4j-contrib/aura-go-sdk/issues)
- [Prometheus Metrics Guide](./docs/prometheus.md)

---

## CI & Releases

Three GitHub Actions workflows manage CI and the release process.

### Workflows

| Workflow | Trigger | What it does |
|---|---|---|
| **CI** | Push to `main`, every PR | Runs tests with the race detector, golangci-lint, and `go build ./...` |
| **Changelog check** | Every PR | Fails if the PR changes `.go` files but has no entry in `.changes/unreleased/` |
| **Release** | Push of a `vX.Y.Z` tag | Gates on tests, extracts the changelog section, creates a GitHub Release |

### Making a release

Releases follow a three-step process. changie collects the unreleased fragment files and determines the correct semver bump automatically from the change kinds (`Added` → minor, `Fixed`/`Security` → patch, `Changed`/`Removed` → major).

There is **no manual version bump** required. `ClientVersion` uses `debug.ReadBuildInfo()` at runtime to read the module version that the Go toolchain embeds when a consumer builds their application. It falls back to `"development"` only in local and test builds.

**1. Batch and merge the changelog**

```bash
changie batch   # collects .changes/unreleased/*.yaml → .changes/vX.Y.Z.md
changie merge   # folds that file into CHANGELOG.md
```

**2. Commit and tag**

```bash
git add CHANGELOG.md .changes/
git commit -m "chore: release vX.Y.Z"
git tag vX.Y.Z
git push origin main --tags
```

**3. Workflow takes over**

Pushing the tag fires the Release workflow, which:
- Runs `go test -race ./...` — the release is aborted if any test fails
- Extracts the `## vX.Y.Z` section from `CHANGELOG.md`
- Creates a GitHub Release with that text as the release notes

Because this is a Go module with no compiled binaries, the tag itself is what consumers reference:

```bash
go get github.com/neo4j-contrib/aura-go-sdk@vX.Y.Z
```

### Adding a changelog entry

Every PR that changes Go source files needs a changie fragment. Run:

```bash
changie new
```

Choose a kind and write a one-line summary, then commit the generated `.yaml` file alongside your code changes. The Changelog check workflow will fail the PR if this step is skipped.

To bypass the check for a PR that genuinely needs no entry (docs-only, CI-only, or test-only changes), add the **`no-changelog`** label to the PR.

---

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## License

See [LICENSE](LICENSE) file for details.
