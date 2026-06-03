package query

import (
	"log/slog"
	"os"
	"testing"
	"time"
)

// TestNewClient_Success verifies successful client creation with credentials
func TestNewClient_Success(t *testing.T) {
	client, err := NewClient(
		WithCredentials("test-id", "test-secret"),
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client == nil {
		t.Fatal("expected client to be non-nil")
	}
	if client.api == nil {
		t.Error("expected api service to be initialized")
	}
	if client.logger == nil {
		t.Error("expected logger to be initialized")
	}
}

// TestNewClient_SubServicesInitialized verifies all sub-services are created
func TestNewClient_SubServicesInitialized(t *testing.T) {
	client, err := NewClient(
		WithCredentials("test-id", "test-secret"),
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if client.Tenants == nil {
		t.Error("expected Tenants service to be initialized")
	}
	if client.Instances == nil {
		t.Error("expected Instances service to be initialized")
	}
	if client.Snapshots == nil {
		t.Error("expected Snapshots service to be initialized")
	}
	if client.CMEK == nil {
		t.Error("expected CMEK service to be initialized")
	}
	if client.GraphAnalytics == nil {
		t.Error("expected GraphAnalytics service to be initialized")
	}
	if client.Prometheus == nil {
		t.Error("expected Prometheus service to be initialized")
	}
}

// TestNewClient_EmptyCredentials validates both credentials must be provided
func TestNewClient_EmptyCredentials(t *testing.T) {
	tests := []struct {
		name         string
		clientID     string
		clientSecret string
		expectedErr  string
	}{
		{
			name:         "both empty",
			clientID:     "",
			clientSecret: "",
			expectedErr:  "client ID must not be empty",
		},
		{
			name:         "empty ID only",
			clientID:     "",
			clientSecret: "secret",
			expectedErr:  "client ID must not be empty",
		},
		{
			name:         "empty secret only",
			clientID:     "id",
			clientSecret: "",
			expectedErr:  "client secret must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(WithCredentials(tt.clientID, tt.clientSecret))

			if err == nil {
				t.Error("expected error, got nil")
			}
			if err.Error() != tt.expectedErr {
				t.Errorf("expected error '%s', got '%s'", tt.expectedErr, err.Error())
			}
			if client != nil {
				t.Error("expected client to be nil")
			}
		})
	}
}

// TestWithTimeout_Valid verifies custom timeout configuration
func TestWithTimeout_Valid(t *testing.T) {
	client, err := NewClient(
		WithCredentials("test-id", "test-secret"),
		WithTimeout(60*time.Second),
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client == nil {
		t.Error("expected client to be non-nil")
	}
}

// TestWithTimeout_Zero validates error for zero timeout
func TestWithTimeout_Zero(t *testing.T) {
	client, err := NewClient(
		WithCredentials("test-id", "test-secret"),
		WithTimeout(0),
	)

	if err == nil {
		t.Error("expected error for zero timeout, got nil")
	}
	if err.Error() != "timeout must be greater than zero" {
		t.Errorf("expected timeout error, got '%s'", err.Error())
	}
	if client != nil {
		t.Error("expected client to be nil")
	}
}

// TestWithTimeout_Negative validates error for negative timeout
func TestWithTimeout_Negative(t *testing.T) {
	client, err := NewClient(
		WithCredentials("test-id", "test-secret"),
		WithTimeout(-10*time.Second),
	)

	if err == nil {
		t.Error("expected error for negative timeout, got nil")
	}
	if client != nil {
		t.Error("expected client to be nil")
	}
}

// TestWithLogger_Valid verifies custom logger configuration
func TestWithLogger_Valid(t *testing.T) {
	opts := &slog.HandlerOptions{Level: slog.LevelDebug}
	handler := slog.NewTextHandler(os.Stderr, opts)
	customLogger := slog.New(handler)

	client, err := NewClient(
		WithCredentials("test-id", "test-secret"),
		WithLogger(customLogger),
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client.logger == nil {
		t.Error("expected logger to be set")
	}
}

// TestWithLogger_Nil validates error for nil logger
func TestWithLogger_Nil(t *testing.T) {
	client, err := NewClient(
		WithCredentials("test-id", "test-secret"),
		WithLogger(nil),
	)

	if err == nil {
		t.Error("expected error for nil logger, got nil")
	}
	if err.Error() != "logger cannot be nil" {
		t.Errorf("expected logger error, got '%s'", err.Error())
	}
	if client != nil {
		t.Error("expected client to be nil")
	}
}

// TestWithBaseURL_Valid verifies custom base URL configuration
func TestWithBaseURL_Valid(t *testing.T) {
	client, err := NewClient(
		WithCredentials("test-id", "test-secret"),
		WithBaseURL("https://api.staging.neo4j.io"),
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client == nil {
		t.Error("expected client to be non-nil")
	}
}

// TestWithBaseURL_Empty validates error for empty base URL
func TestWithBaseURL_Empty(t *testing.T) {
	client, err := NewClient(
		WithCredentials("test-id", "test-secret"),
		WithBaseURL(""),
	)

	if err == nil {
		t.Error("expected error for empty base URL, got nil")
	}
	if err.Error() != "base URL must not be empty" {
		t.Errorf("expected base URL error, got '%s'", err.Error())
	}
	if client != nil {
		t.Error("expected client to be nil")
	}
}

// TestDefaultOptions verifies default configuration values.
// Note: API version is intentionally not configurable — it is fixed to the
// version this module targets and is not exposed via defaultOptions.
func TestDefaultOptions(t *testing.T) {
	opts := defaultOptions()

	if opts.config.baseURL != "https://api.neo4j.io" {
		t.Errorf("expected default baseURL 'https://api.neo4j.io', got '%s'", opts.config.baseURL)
	}
	if opts.config.apiTimeout != 120*time.Second {
		t.Errorf("expected default timeout 120s, got %v", opts.config.apiTimeout)
	}
	if opts.config.apiRetryMax != 3 {
		t.Errorf("expected default apiRetryMax 3, got %d", opts.config.apiRetryMax)
	}
	if opts.logger == nil {
		t.Error("expected default logger to be initialized")
	}
}

// TestAuraAPIVersion verifies the API version constant is set correctly
func TestAuraAPIVersion(t *testing.T) {
	if auraAPIVersion != "v1" {
		t.Errorf("expected auraAPIVersion 'v1', got '%s'", auraAPIVersion)
	}
}

// TestNewClient_MultipleOptions verifies combining multiple options
func TestNewClient_MultipleOptions(t *testing.T) {
	client, err := NewClient(
		WithCredentials("test-id", "test-secret"),
		WithTimeout(90*time.Second),
		WithBaseURL("https://api.staging.neo4j.io"),
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client == nil {
		t.Error("expected client to be non-nil")
	}
}

// TestNewClient_DefaultValues verifies defaults when options not provided
func TestNewClient_DefaultValues(t *testing.T) {
	client, err := NewClient(
		WithCredentials("test-id", "test-secret"),
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client == nil {
		t.Error("expected client to be non-nil")
	}
}

// TestWithCredentials verifies the convenience method
func TestWithCredentials(t *testing.T) {
	client, err := NewClient(
		WithCredentials("test-id", "test-secret"),
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client == nil {
		t.Error("expected client to be non-nil")
	}
}

// TestWithMaxRetry_Valid verifies custom max retry configuration
func TestWithMaxRetry_Valid(t *testing.T) {
	client, err := NewClient(
		WithCredentials("test-id", "test-secret"),
		WithMaxRetry(5),
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client == nil {
		t.Error("expected client to be non-nil")
	}
}

// TestWithMaxRetry_Zero validates error for zero max retry
func TestWithMaxRetry_Zero(t *testing.T) {
	client, err := NewClient(
		WithCredentials("test-id", "test-secret"),
		WithMaxRetry(0),
	)

	if err == nil {
		t.Error("expected error for zero max retry, got nil")
	}
	if client != nil {
		t.Error("expected client to be nil")
	}
}
