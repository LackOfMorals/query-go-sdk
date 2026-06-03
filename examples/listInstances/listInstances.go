// Package main demonstrates listing all Aura instances.
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
	clientID := os.Getenv("AURA_CLIENT_ID")
	clientSecret := os.Getenv("AURA_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		log.Fatal("Missing required environment variables: AURA_CLIENT_ID, AURA_CLIENT_SECRET")
	}

	opts := &slog.HandlerOptions{Level: slog.LevelWarn}
	handler := slog.NewTextHandler(os.Stderr, opts)
	customLogger := slog.New(handler)

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

	instances, err := client.Instances.List(ctx)
	if err != nil {
		log.Fatalf("Failed to list instances: %v", err)
	}

	fmt.Println("=== Current Instances ===")
	for _, inst := range instances.Data {
		instanceDetails, err := client.Instances.Get(ctx, inst.ID)
		if err != nil {
			log.Fatalf("Failed to get instance details for %s: %v", inst.ID, err)
		}
		fmt.Printf("- %s: %s %s %s (%s) (%s) (%s)\n",
			instanceDetails.Data.Name,
			instanceDetails.Data.ID,
			instanceDetails.Data.Status,
			instanceDetails.Data.CloudProvider,
			instanceDetails.Data.Memory,
			instanceDetails.Data.Type,
			instanceDetails.Data.ConnectionURL,
		)
	}

	fmt.Println("\n✓ Client is working correctly!")
}
