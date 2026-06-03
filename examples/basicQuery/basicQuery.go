// Package main demonstrates listing all Aura instances.
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	query "github.com/LackOfMorals/query-go-sdk"
)

func main() {
	username := "neo4j"
	password := "password"
	cypherQry := `MATCH(n)-[a]-(m) RETURN * LIMIT 5`

	var cypherParams map[string]string

	opts := &slog.HandlerOptions{Level: slog.LevelDebug}
	handler := slog.NewTextHandler(os.Stderr, opts)
	customLogger := slog.New(handler)

	client, err := query.NewClient(
		query.WithBaseURL("http://localhost:7474"),
		query.WithTimeout(120*time.Second),
		query.WithLogger(customLogger),
		query.WithBasicAuth(username, password),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Each call gets its own context so it can be individually cancelled or traced.
	ctx := context.Background()

	response, err := client.Query.Query(ctx, cypherQry, cypherParams)
	if err != nil {
		log.Fatalf("Failed to query: %v", err)
	}

	fmt.Printf("Response: %v", response)

	fmt.Println("\n✓ Client is working correctly!")
}
