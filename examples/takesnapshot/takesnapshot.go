// Package main demonstrates creating and inspecting a snapshot for an Aura instance.
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

	// Acquire the instance id to take snapshot of
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

	fmt.Println("=== Taking Snapshot ===")
	takeSnapshotResponse, err := client.Snapshots.Create(ctx, instanceID)
	if err != nil {
		log.Fatalf("Failed to take snapshot: %v", err)
	}
	fmt.Printf("Snapshot taken, Id %s \n", takeSnapshotResponse.Data.SnapshotID)

	// Get more details about the snapshot
	detailSnapshotResponse, err := client.Snapshots.Get(ctx, instanceID, takeSnapshotResponse.Data.SnapshotID)
	if err != nil {
		log.Fatalf("Failed to get snapshot details: %v", err)
	}

	fmt.Printf("Snapshot details: \n Instance ID: %s \n Snapshot ID: %s \n Status: %s ", detailSnapshotResponse.Data.InstanceID, detailSnapshotResponse.Data.SnapshotID, detailSnapshotResponse.Data.Status)

	fmt.Println("\n✓ Client is working correctly!")
}
