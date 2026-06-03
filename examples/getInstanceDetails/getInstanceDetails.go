// Package main demonstrates retrieving details for a specific Aura instance.
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

	// Acquire the instance id to get details for
	fmt.Println("===  Enter instance id ===")
	fmt.Printf("input the ID of the instance to inspect:")
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

	instanceDetails, err := client.Instances.Get(ctx, instanceID)
	if err != nil {
		log.Fatalf("Failed to get instance details: %v", err)
	}

	fmt.Printf("Name: %s\n Id: %s\n Status: %s\n Cloud Provider: %s\n Memory: %s\n Tier: %s\n Connection URL: %s\n",
		instanceDetails.Data.Name,
		instanceDetails.Data.ID,
		instanceDetails.Data.Status,
		instanceDetails.Data.CloudProvider,
		instanceDetails.Data.Memory,
		instanceDetails.Data.Type,
		instanceDetails.Data.ConnectionURL,
	)
}
