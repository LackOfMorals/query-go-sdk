// Package main demonstrates listing snapshots for an Aura instance.
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	aura "github.com/neo4j-contrib/aura-go-sdk/v2"
)

func main() {
	// Load aura information from environment
	clientID := os.Getenv("AURA_CLIENT_ID")
	clientSecret := os.Getenv("AURA_CLIENT_SECRET")

	// Use a custom slog logger with warn level set
	opts := &slog.HandlerOptions{Level: slog.LevelDebug}
	handler := slog.NewTextHandler(os.Stderr, opts)
	customLogger := slog.New(handler)

	if clientID == "" || clientSecret == "" {
		log.Fatal("Missing required environment variables")
	}

	// Acquire the instance id we want to lists snapshots for
	fmt.Println("===  Enter instance id ===")
	fmt.Printf("input the ID of the instance to lists its snapshots:")
	var instanceID string
	n, err := fmt.Scanln(&instanceID)
	if err != nil {
		log.Println("Error entering instance ID to read: ", err)
		os.Exit(1)
	}

	// Check instance id is valid
	if n > 2 {
		log.Println("Only a single value can be entered for the Instance ID. You entered ", n)
		os.Exit(1)
	}

	if len(instanceID) != 8 {
		log.Println("Instance ID is made up of 8 characters. You entered ", len(instanceID))
		os.Exit(1)
	}

	// Create aura client
	client, err := aura.NewClient(
		aura.WithCredentials(clientID, clientSecret),
		aura.WithTimeout(120*time.Second),
		aura.WithLogger(customLogger),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Each call gets its own context so it can be individually cancelled or traced.
	ctx := context.Background()

	// List snapshot

	fmt.Printf("=== Snapshot List for %s ===\n", instanceID)
	listSnapshotResponse, err := client.Snapshots.List(ctx, instanceID, &aura.SnapshotDate{Year: 2026, Month: time.March, Day: 23})
	if err != nil {
		log.Fatalf("Failed to take snapshot: %v", err)
	}

	fmt.Printf("Number of snapshots: %v\n", len(listSnapshotResponse.Data))

	for _, snapshotDetail := range listSnapshotResponse.Data {
		fmt.Printf("- %s: %s %s %v\n",
			snapshotDetail.SnapshotID,
			snapshotDetail.Status,
			snapshotDetail.Timestamp,
			snapshotDetail.Exportable,
		)
	}

}
