// Package main demonstrates restoring an Aura instance from a snapshot.
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
	opts := &slog.HandlerOptions{Level: slog.LevelWarn}
	handler := slog.NewTextHandler(os.Stderr, opts)
	customLogger := slog.New(handler)

	if clientID == "" || clientSecret == "" {
		log.Fatal("Missing required environment variables")
	}

	// Acquire the instance id to list snapshots of
	fmt.Println("===  Enter instance id ===")
	fmt.Printf("input the ID of the instance to snapshot:")
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

	// Take snapshot
	fmt.Println("=== List Snapshots ===")
	getInstanceSnapshots, err := client.Snapshots.List(ctx, instanceID, nil)
	if err != nil {
		log.Fatalf("Failed to list snapshots: %v", err)
	}

	for _, instSnapShot := range getInstanceSnapshots.Data {
		fmt.Printf("- %s %s %s %s %v \n",
			instSnapShot.InstanceID,
			instSnapShot.SnapshotID,
			instSnapShot.Status,
			instSnapShot.Timestamp,
			instSnapShot.Exportable,
		)
	}
	fmt.Println("\n✓ Client is working correctly!")

	fmt.Println("=== Restoring from Snapshot f66d9073-6b7b-4162-9847-043dbdc02faa ===")
	restoreInstanceResponse, err := client.Snapshots.Restore(ctx, instanceID, "f66d9073-6b7b-4162-9847-043dbdc02faa")
	if err != nil {
		log.Fatalf("Failed to restore snapshot: %v", err)
	}

	fmt.Printf("- %s %s \n",
		restoreInstanceResponse.Data.ID,
		restoreInstanceResponse.Data.Status,
	)
}
