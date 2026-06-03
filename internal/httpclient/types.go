package httpclient

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

const (
	// DefaultMaxResponseSize is the maximum size of response body to read (10MB).
	DefaultMaxResponseSize = 10 * 1024 * 1024
)

// HTTPResponse stores the response from a request, including the payload and headers.
type HTTPResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// HTTPService defines the interface for HTTP operations.
// This is the low-level HTTP layer that handles raw HTTP requests.
type HTTPService interface {
	Get(ctx context.Context, url string, headers map[string]string) (*HTTPResponse, error)
	Post(ctx context.Context, url string, headers map[string]string, body string) (*HTTPResponse, error)
	Put(ctx context.Context, url string, headers map[string]string, body string) (*HTTPResponse, error)
	Patch(ctx context.Context, url string, headers map[string]string, body string) (*HTTPResponse, error)
	Delete(ctx context.Context, url string, headers map[string]string) (*HTTPResponse, error)
	// Close releases idle connections held by the underlying HTTP transport.
	// It should be called when the service is no longer needed.
	Close()
}

// httpService is the concrete implementation of HTTPService.
// It handles HTTP requests with configurable timeouts, retries, and connection
// pooling. All URLs are passed in fully-formed by the caller — this layer has
// no knowledge of base URLs, API versions, or any other higher-level routing.
type httpService struct {
	timeout time.Duration
	client  *retryablehttp.Client
	logger  *slog.Logger
}

// Compile-time interface compliance check.
var _ HTTPService = (*httpService)(nil)
