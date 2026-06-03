// Package main demonstrates listing all tenants in an Aura organisation.
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

	listOfTenants, err := client.Tenants.List(ctx)
	if err != nil {
		log.Fatalf("Failed to list tenants: %v", err)
	}

	fmt.Println("=== Current Tenants ===")
	for _, tenant := range listOfTenants.Data {
		fmt.Printf("- %s %s)\n",
			tenant.Name,
			tenant.ID,
		)
	}

	fmt.Println("\n✓ Client is working correctly!")

}
