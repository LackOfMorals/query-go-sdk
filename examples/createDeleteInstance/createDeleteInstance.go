// Package main demonstrates creating and deleting an Aura instance.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	aura "github.com/neo4j-contrib/aura-go-sdk/v2"
)

// Polls until instance state is running
// context should be set to use a timeout
func pollInstance(ctx context.Context, client aura.AuraAPIClient, id string, status aura.InstanceStatus) error {
	i := 1

	// The context given to us has a timeout of 10 minutes
	// Creating takes longer so we will create a 'child' context to use
	// with the passed in context as parent
	ctxPolling, cancel := context.WithTimeout(ctx, time.Duration(time.Minute*10))
	defer cancel()

	for {
		select {
		case <-ctxPolling.Done():
			return errors.New("timed out")
		default:
			result, err := client.Instances.Get(ctx, id)
			if err != nil {
				return err
			}
			// Check the status for running
			// and return if it is
			if result.Data.Status == status {
				return nil
			}
			time.Sleep(1 * time.Second)
			fmt.Printf("Instance status is %s .  Poll no. %v\n", result.Data.Status, i)

			i = i + 1
		}
	}
}

func main() {
	clientID := os.Getenv("AURA_CLIENT_ID")
	clientSecret := os.Getenv("AURA_CLIENT_SECRET")
	clientTenant := os.Getenv("AURA_TENANT_ID")

	if clientID == "" || clientSecret == "" {
		log.Fatal("Missing required environment variables: AURA_CLIENT_ID, AURA_CLIENT_SECRET, AURA_TENANT_ID")
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

	// The spec of the instance to be created
	// As we're using the free tier, only a single free instance can exist
	// Change to another tied if needed
	// Else this will exit

	instanceSpec := aura.CreateInstanceConfigData{
		Name:          "auraClientExample",
		Type:          "free-db",
		TenantID:      clientTenant,
		CloudProvider: "gcp",
		Region:        "europe-west1",
		Memory:        "1GB",
	}

	// Each call gets its own context so it can be individually cancelled or traced.
	// We will use a context with a timeout of 6 seconds
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*6))
	defer cancel()

	// First get the list of instances
	instances, err := client.Instances.List(ctx)
	if err != nil {
		log.Fatalf("Failed to list instances: %v", err)
	}

	// Now we look for any instance that is on the free tier
	for _, inst := range instances.Data {
		instanceDetails, err := client.Instances.Get(ctx, inst.ID)
		if err != nil {
			log.Fatalf("Failed to get instance details for %s: %v", inst.ID, err)
		}
		if instanceDetails.Data.Type == "free-db" {
			fmt.Printf("Instance %s with id %s is already using the free tier\n", instanceDetails.Data.Name, instanceDetails.Data.ID)
			log.Fatal("Free tier is used. Cannot proceed\n")
		}
	}

	// No instances are on free tier.  Move ahead
	// Create a new instance
	newInstance, err := client.Instances.Create(ctx, &instanceSpec)
	if err != nil {
		log.Fatalf("Failed to create instance: %v", err)
	}

	// Show new instance details
	fmt.Println("\n Instance created")
	fmt.Printf("- %s: %s %s %s (%s) (%s)\n",
		newInstance.Data.Name,
		newInstance.Data.ID,
		newInstance.Data.CloudProvider,
		newInstance.Data.ConnectionURL,
		newInstance.Data.Username,
		newInstance.Data.Password,
	)

	// It's going to take several minutes for the instance to come up
	// We'll poll for status until it's running
	err = pollInstance(ctx, *client, newInstance.Data.ID, "running")
	if err != nil {
		log.Fatal("Unable to obtain status of new instance", err)
	}

	// Instance is running
	fmt.Printf("\n Instance %s is now running", newInstance.Data.Name)
	fmt.Printf("\n  Will delete in two seconds")

	// Wait for two seconds
	time.Sleep(2 * time.Second)

	// We're running, now to delete it
	delInstance, err := client.Instances.Delete(ctx, newInstance.Data.ID)
	if err != nil {
		log.Fatal("Unable to delete instance", err)
	}

	fmt.Println("\n Instance being deleted")
	fmt.Printf("- %s: %s %s \n",
		delInstance.Data.Name,
		delInstance.Data.ID,
		delInstance.Data.Status,
	)

	fmt.Println("\n✓ Client is working correctly!")
}
