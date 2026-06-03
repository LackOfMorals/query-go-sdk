// Package main demonstrates querying Prometheus metrics for an Aura instance.
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	aura "github.com/neo4j-contrib/aura-go-sdk/v2"
)

func main() {

	// Load aura information from environment
	clientID := os.Getenv("AURA_CLIENT_ID")
	clientSecret := os.Getenv("AURA_CLIENT_SECRET")

	// Use a custom slog logger with warn level set
	opts := &slog.HandlerOptions{Level: slog.LevelWarn}
	handler := slog.NewTextHandler(os.Stderr, opts)
	customLogger := slog.New(handler)

	// Create the Aura client
	client, err := aura.NewClient(
		aura.WithCredentials(clientID, clientSecret),
		aura.WithLogger(customLogger),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Each call gets its own context so it can be individually cancelled or traced.
	ctx := context.Background()

	// Get instance details to retrieve the Prometheus URL
	instanceID := "c9f0d13a"
	instance, err := client.Instances.Get(ctx, instanceID)
	if err != nil {
		log.Fatalf("Failed to get instance: %v", err)
	}

	prometheusURL := instance.Data.MetricsURL
	fmt.Printf("Prometheus URL: %s\n", prometheusURL)

	// Example 1: Fetch all raw metrics
	fmt.Println("\n=== Fetch Raw Metrics Example ===")
	rawMetrics, err := client.Prometheus.FetchRawMetrics(ctx, prometheusURL)
	if err != nil {
		log.Printf("Failed to fetch metrics: %v", err)
	} else {
		fmt.Printf("Fetched %d unique metrics\n", len(rawMetrics.Metrics))
		fmt.Println("\nAvailable metrics:")
		count := 0
		for metricName := range rawMetrics.Metrics {
			fmt.Printf("  - %s\n", metricName)
			count++
			if count >= 10 {
				fmt.Printf("  ... and %d more\n", len(rawMetrics.Metrics)-10)
				break
			}
		}
	}

	// Example 2: Get specific metric values
	fmt.Println("\n=== Specific Metric Values ===")
	if rawMetrics != nil {
		// CPU Usage
		if cpuUsage, err := client.Prometheus.GetMetricValue(ctx, rawMetrics, "neo4j_aura_cpu_usage", nil); err == nil {
			if cpuLimit, err := client.Prometheus.GetMetricValue(ctx, rawMetrics, "neo4j_aura_cpu_limit", nil); err == nil {
				fmt.Printf("CPU Usage: %.2f cores / %.2f cores limit (%.1f%%)\n", cpuUsage, cpuLimit, (cpuUsage/cpuLimit)*100)
			}
		}

		// Memory (Heap) Usage
		if heapRatio, err := client.Prometheus.GetMetricValue(ctx, rawMetrics, "neo4j_dbms_vm_heap_used_ratio", nil); err == nil {
			fmt.Printf("Heap Usage: %.1f%%\n", heapRatio*100)
		}

		// Database size
		if nodeCount, err := client.Prometheus.GetMetricValue(ctx, rawMetrics, "neo4j_database_count_node", nil); err == nil {
			if relCount, err := client.Prometheus.GetMetricValue(ctx, rawMetrics, "neo4j_database_count_relationship", nil); err == nil {
				fmt.Printf("Database: %.0f nodes, %.0f relationships\n", nodeCount, relCount)
			}
		}

		// Page cache hit rate
		if hitRate, err := client.Prometheus.GetMetricValue(ctx, rawMetrics, "neo4j_dbms_page_cache_hit_ratio_per_minute", nil); err == nil {
			fmt.Printf("Page Cache Hit Rate: %.1f%%\n", hitRate*100)
		}
	}

	// Example 3: Get comprehensive instance health
	fmt.Println("\n=== Instance Health Example ===")
	health, err := client.Prometheus.GetInstanceHealth(ctx, instanceID, prometheusURL)
	if err != nil {
		log.Printf("Health check failed: %v", err)
	} else {
		fmt.Printf("Instance ID: %s\n", health.InstanceID)
		fmt.Printf("Timestamp: %s\n", health.Timestamp.Format("2006-01-02 15:04:05"))
		fmt.Printf("Overall Status: %s\n", health.OverallStatus)

		fmt.Println("\nResources:")
		fmt.Printf("  CPU Usage: %.2f%%\n", health.Resources.CPUUsagePercent)
		fmt.Printf("  Memory Usage: %.2f%%\n", health.Resources.MemoryUsagePercent)

		fmt.Println("\nQuery Performance:")
		fmt.Printf("  Total Queries: %.0f\n", health.Query.QueryExecutionTotal)
		fmt.Printf("  Avg Latency (p50): %.2fms\n", health.Query.AvgLatencyMS)

		fmt.Println("\nConnections:")
		fmt.Printf("  Active: %d/%d (%.1f%%)\n",
			health.Connections.ActiveConnections,
			health.Connections.MaxConnections,
			health.Connections.UsagePercent)

		fmt.Println("\nStorage:")
		fmt.Printf("  Page Cache Hit Rate: %.2f%%\n", health.Storage.PageCacheHitRate)

		if len(health.Issues) > 0 {
			fmt.Println("\nIssues:")
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
	}

	// Example 4: Query specific metrics with label filters
	fmt.Println("\n=== Filtered Metric Queries ===")
	if rawMetrics != nil {
		// Get metrics for a specific availability zone
		filters := map[string]string{
			"availability_zone": "europe-west2-b",
			"instance_mode":     "PRIMARY",
		}

		if cpuUsage, err := client.Prometheus.GetMetricValue(ctx, rawMetrics, "neo4j_aura_cpu_usage", filters); err == nil {
			fmt.Printf("CPU Usage (zone europe-west2-b): %.4f cores\n", cpuUsage)
		}

		// Check for raft leader
		if isLeader, err := client.Prometheus.GetMetricValue(ctx, rawMetrics, "neo4j_cluster_raft_is_leader", filters); err == nil {
			if isLeader == 1 {
				fmt.Println("This node is the Raft leader")
			} else {
				fmt.Println("This node is NOT the Raft leader")
			}
		}
	}

	// Example 5: Monitor for specific conditions
	fmt.Println("\n=== Health Checks ===")
	if rawMetrics != nil {
		// Check for OOM errors
		if oomErrors, err := client.Prometheus.GetMetricValue(ctx, rawMetrics, "neo4j_aura_out_of_memory_errors_total", nil); err == nil {
			if oomErrors > 0 {
				fmt.Printf("⚠️  WARNING: %0f Out of Memory errors detected!\n", oomErrors)
			} else {
				fmt.Println("✅ No Out of Memory errors")
			}
		}

		// Check page cache evictions
		if evictions, err := client.Prometheus.GetMetricValue(ctx, rawMetrics, "neo4j_dbms_page_cache_evictions_total", nil); err == nil {
			fmt.Printf("Page Cache Evictions: %.0f\n", evictions)
			if evictions > 1000 {
				fmt.Println("  ⚠️  High eviction rate - consider increasing instance size")
			}
		}

		// Check query failures
		if failures, err := client.Prometheus.GetMetricValue(ctx, rawMetrics, "neo4j_db_query_execution_failure_total", nil); err == nil {
			if failures > 0 {
				fmt.Printf("⚠️  Query Failures: %.0f\n", failures)
			} else {
				fmt.Println("✅ No query failures")
			}
		}
	}
}
