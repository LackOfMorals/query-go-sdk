package api

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/LackOfMorals/query-go-sdk/internal/httpclient"
)

// Response represents a response from the Query API.
type Response struct {
	StatusCode int
	Body       []byte
}

// Error represents an error response from the Query API.
type Error struct {
	StatusCode int           `json:"status_code"`
	Message    string        `json:"message"`
	Details    []ErrorDetail `json:"details,omitempty"`
}

// ErrorDetail represents individual error details.
type ErrorDetail struct {
	Message string `json:"message"`
	Reason  string `json:"reason,omitempty"`
	Field   string `json:"field,omitempty"`
}

// Credentials produces the Authorization header value on demand.
type Credentials interface {
	Authorize() (headerValue string)
}

// basicCredentials uses username+password to set Basic auth on every request.
type BasicCredentials struct {
	Username string
	Password string
}

// staticCredentials holds a pre-supplied bearer token.
type StaticCredentials struct {
	Token string
}

// Config holds configuration for the API service.
type Config struct {
	AuthHeader     Credentials
	BaseURL        string
	APIVersion     string
	Timeout        time.Duration
	MaxRetry       int
	UserAgent      string            // e.g. "aura-go-client/v1.8.0"; defaults to "aura-go-client" if empty
	HTTPClient     *http.Client      // optional custom HTTP client; when non-nil it replaces the default transport
	DefaultHeaders map[string]string // optional headers merged into every authenticated request
}

// apiRequestService is the concrete implementation of RequestService.
type apiRequestService struct {
	httpClient     httpclient.HTTPService
	authHeader     Credentials
	baseURL        string
	endpointBase   string
	userAgent      string
	defaultHeaders map[string]string
	logger         *slog.Logger
}

// Compile-time interface compliance check.
var _ RequestService = (*apiRequestService)(nil)

// authManager handles token management for the API.
type authManager struct {
	username  string
	password  string
	tokenType string
	token     string
	expiresAt int64
	logger    *slog.Logger
	mu        sync.RWMutex
}

// tokenResponse represents the OAuth token response.
type tokenResponse struct {
	TokenType   string `json:"token_type"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

// RequestService defines the interface for making authenticated API requests.
// This is the middle layer that handles authentication and common API patterns.
type RequestService interface {
	Get(ctx context.Context, endpoint string) (*Response, error)
	Post(ctx context.Context, endpoint string, body string) (*Response, error)
	Put(ctx context.Context, endpoint string, body string) (*Response, error)
	Patch(ctx context.Context, endpoint string, body string) (*Response, error)
	Delete(ctx context.Context, endpoint string) (*Response, error)
	// Close releases idle connections held by the underlying HTTP transport.
	// It should be called (typically via defer) when the client is no longer needed.
	Close()
}
