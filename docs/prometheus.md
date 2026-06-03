# Prometheus Client for Neo4j Aura

This package provides a Go client for fetching and parsing Prometheus metrics from Neo4j Aura instances.

## Features

- **Raw Metrics Fetching**: Fetch and parse Prometheus exposition format from Aura metrics endpoints
- **Label Filtering**: Query specific metrics by label filters
- **Health Monitoring**: Get comprehensive health metrics for instances with automatic assessment
- **Auto-parsing**: Automatically parse Prometheus text format into structured data

## Installation

```bash
go get github.com/neo4j-contrib/aura-go-sdk
```

## Understanding Aura Metrics

After enabling metrics in the Console , either on a project or instance basis, Neo4j Aura exposes metrics in Prometheus exposition format (the raw text format that Prometheus scrapes). The client parses this format and provides convenient access to the metrics.

The metrics endpoint ( instance level ) will look like this
```
https://customer-metrics-api.neo4j.io/api/v1/{Project Id}/{Instance Id}/metrics
```

This endpoint returns metrics in the format:
```
# HELP neo4j_aura_cpu_usage CPU usage (cores)
# TYPE neo4j_aura_cpu_usage gauge
neo4j_aura_cpu_usage{availability_zone="europe-west2-c",instance_id="c9f0d13a"} 0.023206
...A lot more !
```

## Usage

### Basic Setup

```go
import aura "github.com/neo4j-contrib/aura-go-sdk"

client, err := aura.NewClient(
    aura.WithCredentials("client-id", "client-secret"),
)
if err != nil {
    log.Fatal(err)
}
```

### Getting the Prometheus URL

Each Aura instance has a metrics endpoint:

```go
instance, err := client.Instances.Get("instance-id")
if err != nil {
    log.Fatal(err)
}

prometheusURL := instance.Data.MetricsURL
```

### Fetching Raw Metrics

Fetch and parse all metrics from the endpoint:

```go
metrics, err := client.Prometheus.FetchRawMetrics(prometheusURL)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Fetched %d unique metrics\n", len(metrics.Metrics))

// List all available metrics
for metricName := range metrics.Metrics {
    fmt.Printf("  - %s\n", metricName)
}
```

### Querying Specific Metrics

Get a specific metric value (averaged across all instances):

```go
cpuUsage, err := client.Prometheus.GetMetricValue(
    metrics, 
    "neo4j_aura_cpu_usage", 
    nil,  // no label filters - averages all
)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("CPU Usage: %.4f cores\n", cpuUsage)
```

### Filtering by Labels

Query metrics for specific instances or zones:

```go
// Get CPU usage for a specific availability zone
filters := map[string]string{
    "availability_zone": "europe-west2-b",
    "instance_mode":     "PRIMARY",
}

cpuUsage, err := client.Prometheus.GetMetricValue(
    metrics,
    "neo4j_aura_cpu_usage",
    filters,
)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("CPU Usage (zone b): %.4f cores\n", cpuUsage)
```

### Instance Health Monitoring

Get comprehensive health metrics with automatic assessment:

```go
health, err := client.Prometheus.GetInstanceHealth(instanceID, prometheusURL)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Status: %s\n", health.OverallStatus)
fmt.Printf("CPU Usage: %.2f%%\n", health.Resources.CPUUsagePercent)
fmt.Printf("Memory Usage: %.2f%%\n", health.Resources.MemoryUsagePercent)
fmt.Printf("Total Queries: %.0f\n", health.Query.QueryExecutionTotal)
fmt.Printf("Avg Latency (p50): %.2fms\n", health.Query.AvgLatencyMS)
fmt.Printf("Connections: %d/%d (%.1f%%)\n", 
    health.Connections.ActiveConnections,
    health.Connections.MaxConnections,
    health.Connections.UsagePercent)
fmt.Printf("Page Cache Hit Rate: %.2f%%\n", health.Storage.PageCacheHitRate)

if len(health.Issues) > 0 {
    fmt.Println("\nIssues detected:")
    for _, issue := range health.Issues {
        fmt.Printf("  ⚠️  %s\n", issue)
    }
}

if len(health.Recommendations) > 0 {
    fmt.Println("\nRecommendations:")
    for _, rec := range health.Recommendations {
        fmt.Printf("  💡 %s\n", rec)
    }
}
```

## Key Neo4j Aura Metrics

### Resource Metrics

```go
// CPU usage in cores
"neo4j_aura_cpu_usage"

// CPU limit (max cores available)
"neo4j_aura_cpu_limit"

// Heap memory usage ratio (0-1)
"neo4j_dbms_vm_heap_used_ratio"

// Storage limit in bytes
"neo4j_aura_storage_limit"
```

### Database Metrics

```go
// Number of nodes in the database
"neo4j_database_count_node"

// Number of relationships
"neo4j_database_count_relationship"

// Database size in bytes
"neo4j_database_store_size_database"
```

### Query Performance

```go
// Successful queries total
"neo4j_db_query_execution_success_total"

// Failed queries total
"neo4j_db_query_execution_failure_total"

// Query latency p50 (median) in milliseconds
"neo4j_db_query_execution_internal_latency_q50"

// Query latency p75 in milliseconds
"neo4j_db_query_execution_internal_latency_q75"

// Query latency p99 in milliseconds
"neo4j_db_query_execution_internal_latency_q99"
```

### Transaction Metrics

```go
// Active read transactions
"neo4j_database_transaction_active_read"

// Active write transactions
"neo4j_database_transaction_active_write"

// Total committed transactions
"neo4j_database_transaction_committed_total"

// Total rollbacks
"neo4j_database_transaction_rollbacks_total"

// Last committed transaction ID
"neo4j_database_transaction_last_committed_tx_id_total"

// Peak concurrent transactions
"neo4j_database_transaction_peak_concurrent_total"
```

### Connection Metrics

```go
// Idle Bolt connections
"neo4j_dbms_bolt_connections_idle"

// Running Bolt connections
"neo4j_dbms_bolt_connections_running"

// Total opened connections
"neo4j_dbms_bolt_connections_opened_total"

// Total closed connections
"neo4j_dbms_bolt_connections_closed_total"
```

### Cache Metrics

```go
// Page cache hit ratio per minute (0-1)
"neo4j_dbms_page_cache_hit_ratio_per_minute"

// Page cache usage ratio (0-1)
"neo4j_dbms_page_cache_usage_ratio"

// Page cache evictions total
"neo4j_dbms_page_cache_evictions_total"
```

### Checkpoint Metrics

```go
// Checkpoint events total
"neo4j_database_check_point_events_total"

// Checkpoint duration in milliseconds
"neo4j_database_check_point_duration"

// Total checkpoint time
"neo4j_database_check_point_total_time_total"
```

### Garbage Collection

```go
// Young generation GC time
"neo4j_dbms_vm_gc_time_g1_young_generation_total"

// Old generation GC time
"neo4j_dbms_vm_gc_time_g1_old_generation_total"
```

### Cluster Metrics (Business Critical instances)

```go
// Is this server the Raft leader? (0 or 1)
"neo4j_cluster_raft_is_leader"
```

### Error Metrics

```go
// Out of Memory errors total
"neo4j_aura_out_of_memory_errors_total"

// Cypher replan events (high values may indicate missing parameters)
"neo4j_database_cypher_replan_events_total"
```

## API Reference

### PrometheusService Interface

```go
type PrometheusService interface {
    // FetchRawMetrics fetches and parses all metrics
    FetchRawMetrics(prometheusURL string) (*PrometheusMetricsResponse, error)
    
    // GetMetricValue retrieves a specific metric with optional label filtering
    GetMetricValue(metrics *PrometheusMetricsResponse, name string, labelFilters map[string]string) (float64, error)
    
    // GetInstanceHealth retrieves comprehensive health metrics
    GetInstanceHealth(instanceID string, prometheusURL string) (*PrometheusHealthMetrics, error)
}
```

### Response Types

#### PrometheusMetric

```go
type PrometheusMetric struct {
    Name      string
    Labels    map[string]string
    Value     float64
    Timestamp int64
}
```

#### PrometheusMetricsResponse

```go
type PrometheusMetricsResponse struct {
    Metrics map[string][]PrometheusMetric
}

// Metrics is a map where:
// - Key: metric name (e.g., "neo4j_aura_cpu_usage")
// - Value: slice of PrometheusMetric (one per availability zone/instance)
```

#### PrometheusHealthMetrics

```go
type PrometheusHealthMetrics struct {
    InstanceID      string                 `json:"instance_id"`
    Timestamp       time.Time              `json:"timestamp"`
    Resources       ResourceMetrics        `json:"resources"`
    Query           QueryMetrics           `json:"query"`
    Connections     ConnectionMetrics      `json:"connections"`
    Storage         StorageMetrics         `json:"storage"`
    OverallStatus   string                 `json:"overall_status"`
    Issues          []string               `json:"issues"`
    Recommendations []string               `json:"recommendations"`
}
```

## Health Status

The `GetInstanceHealth` method returns an overall health status:

- **healthy**: All metrics are within normal ranges
- **warning**: One or more metrics exceed recommended thresholds

### Health Checks

The health assessment checks:

1. **CPU Usage**: Warning if > 80% of limit
2. **Memory Usage**: Warning if heap ratio > 85%
3. **Connection Pool**: Warning if > 80% utilization
4. **Page Cache**: Warning if hit rate < 50%

### Recommendations

The system provides actionable recommendations:

- High CPU/Memory: Suggests scaling to larger instance
- High connections: Suggests reviewing connection pooling
- Low cache hit rate: Suggests increasing page cache size

## Complete Example

```go
package main

import (
    "fmt"
    "log"
    aura "github.com/neo4j-contrib/aura-go-sdk"
)

func main() {
    // Create client
    client, err := aura.NewClient(
        aura.WithCredentials("client-id", "client-secret"),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Get instance
    instance, err := client.Instances.Get("instance-id")
    if err != nil {
        log.Fatal(err)
    }

    prometheusURL := instance.Data.MetricsURL

    // Fetch all metrics
    metrics, err := client.Prometheus.FetchRawMetrics(prometheusURL)
    if err != nil {
        log.Fatal(err)
    }

    // Check for OOM errors
    if oomErrors, _ := client.Prometheus.GetMetricValue(metrics, "neo4j_aura_out_of_memory_errors_total", nil); oomErrors > 0 {
        log.Printf("WARNING: %.0f OOM errors detected!", oomErrors)
    }

    // Get health summary
    health, err := client.Prometheus.GetInstanceHealth("instance-id", prometheusURL)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Instance Status: %s\n", health.OverallStatus)
    fmt.Printf("CPU: %.1f%%, Memory: %.1f%%\n", 
        health.Resources.CPUUsagePercent,
        health.Resources.MemoryUsagePercent)
}
```

## Error Handling

Always check for errors when fetching or querying metrics:

```go
metrics, err := client.Prometheus.FetchRawMetrics(prometheusURL)
if err != nil {
    // Handle error - endpoint might be temporarily unavailable
    log.Printf("Failed to fetch metrics: %v", err)
    return
}

// Use metrics...
```

## Best Practices

1. **Cache Metrics URL**: The metrics URL doesn't change for an instance
2. **Poll Periodically**: Metrics are updated frequently (typically every minute)
3. **Handle Missing Metrics**: Use GetMetricValue which returns errors for missing metrics
4. **Monitor Key Indicators**: Focus on OOM errors, page cache evictions, and query failures
5. **Use Label Filters**: Query specific zones or instances when debugging
6. **Check Health Regularly**: Use GetInstanceHealth for quick status checks

## Common Patterns

### Monitoring Dashboard

```go
func monitorInstance(client *aura.AuraAPIClient, instanceID, prometheusURL string) {
    // Fetch metrics
    metrics, err := client.Prometheus.FetchRawMetrics(prometheusURL)
    if err != nil {
        log.Printf("Error: %v", err)
        return
    }

    // CPU utilization
    cpuUsage, _ := client.Prometheus.GetMetricValue(metrics, "neo4j_aura_cpu_usage", nil)
    cpuLimit, _ := client.Prometheus.GetMetricValue(metrics, "neo4j_aura_cpu_limit", nil)
    fmt.Printf("CPU: %.2f / %.2f cores (%.1f%%)\n", cpuUsage, cpuLimit, (cpuUsage/cpuLimit)*100)

    // Memory utilization  
    heapRatio, _ := client.Prometheus.GetMetricValue(metrics, "neo4j_dbms_vm_heap_used_ratio", nil)
    fmt.Printf("Heap: %.1f%%\n", heapRatio*100)

    // Database size
    nodes, _ := client.Prometheus.GetMetricValue(metrics, "neo4j_database_count_node", nil)
    rels, _ := client.Prometheus.GetMetricValue(metrics, "neo4j_database_count_relationship", nil)
    fmt.Printf("Graph: %.0f nodes, %.0f relationships\n", nodes, rels)

    // Performance
    success, _ := client.Prometheus.GetMetricValue(metrics, "neo4j_db_query_execution_success_total", nil)
    failures, _ := client.Prometheus.GetMetricValue(metrics, "neo4j_db_query_execution_failure_total", nil)
    fmt.Printf("Queries: %.0f success, %.0f failures\n", success, failures)
}
```

### Alert on Errors

```go
func checkForErrors(metrics *aura.PrometheusMetricsResponse, client *aura.AuraAPIClient) {
    // Check for OOM errors
    if oomErrors, err := client.Prometheus.GetMetricValue(metrics, "neo4j_aura_out_of_memory_errors_total", nil); err == nil && oomErrors > 0 {
        sendAlert(fmt.Sprintf("OOM errors detected: %.0f", oomErrors))
    }

    // Check for query failures
    if failures, err := client.Prometheus.GetMetricValue(metrics, "neo4j_db_query_execution_failure_total", nil); err == nil && failures > 0 {
        sendAlert(fmt.Sprintf("Query failures: %.0f", failures))
    }

    // Check page cache eviction rate
    if evictions, err := client.Prometheus.GetMetricValue(metrics, "neo4j_dbms_page_cache_evictions_total", nil); err == nil && evictions > 1000 {
        sendAlert(fmt.Sprintf("High page cache evictions: %.0f", evictions))
    }
}
```

### Multi-Zone Analysis

For Business Critical instances with multiple availability zones:

```go
zones := []string{"europe-west2-a", "europe-west2-b", "europe-west2-c"}

for _, zone := range zones {
    filters := map[string]string{
        "availability_zone": zone,
        "instance_mode":     "PRIMARY",
    }

    cpuUsage, err := client.Prometheus.GetMetricValue(metrics, "neo4j_aura_cpu_usage", filters)
    if err != nil {
        continue
    }

    isLeader, _ := client.Prometheus.GetMetricValue(metrics, "neo4j_cluster_raft_is_leader", filters)
    leader := ""
    if isLeader == 1 {
        leader = " (LEADER)"
    }

    fmt.Printf("%s: CPU=%.4f cores%s\n", zone, cpuUsage, leader)
}
```

## Understanding the Data

### Labels

Each metric includes labels that provide context:

- `instance_id`: The Aura instance ID
- `availability_zone`: The cloud availability zone (e.g., "europe-west2-b")
- `instance_mode`: Usually "PRIMARY" for Aura instances
- `database`: The database name (usually "neo4j")
- `aggregation`: How the metric is aggregated ("MAX", "MIN", "AVG", "SUM")

### Aggregation

Aura instances (especially Business Critical) run across multiple availability zones. The `aggregation` label indicates how the metric is aggregated:

- `MAX`: Maximum value across zones
- `MIN`: Minimum value across zones
- `AVG`: Average value across zones
- `SUM`: Sum across zones

When you use `GetMetricValue` without filters, it averages across all zones.

### Counters vs Gauges

- **Counters** (e.g., `_total` suffix): Cumulative values that only increase
  - `neo4j_database_transaction_committed_total`
  - `neo4j_db_query_execution_success_total`
  
- **Gauges** (e.g., ratios, counts): Values that can increase or decrease
  - `neo4j_aura_cpu_usage`
  - `neo4j_dbms_vm_heap_used_ratio`
  - `neo4j_database_count_node`

## Authentication

The Prometheus client uses the same OAuth credentials as the Aura API. Authentication is handled automatically by the client.

## Examples

See the [examples directory](../example/prometheus_example.go) for complete working examples.

## License

See the main package [LICENSE](../LICENSE) file.
