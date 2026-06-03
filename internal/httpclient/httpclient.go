// Package httpclient provides a low-level HTTP client with configurable retry
// behaviour. It is the transport layer beneath internal/api and has no knowledge
// of Aura-specific concepts such as base URLs, API versions, or authentication.
package httpclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

// networkOnlyRetryPolicy retries only on connection-level errors (e.g. refused,
// reset, DNS failure). HTTP responses — including 5xx — are returned as-is so
// the api layer above can inspect the status code and decide what to do.
func networkOnlyRetryPolicy(ctx context.Context, resp *http.Response, err error) (bool, error) {
	// Context cancelled/deadline exceeded — do not retry.
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	// Network-level error with no HTTP response — retry.
	if err != nil && resp == nil {
		return true, nil
	}
	// Any actual HTTP response, regardless of status code — do not retry.
	// Status-code interpretation is the responsibility of the api layer.
	return false, nil
}

// NewHTTPService creates a new HTTPService backed by a retryable HTTP client.
// Retries are attempted only on network-level errors (no response received);
// HTTP error responses (including 5xx) are always returned to the caller.
// The caller-supplied logger is used for debug output.
//
// When customClient is non-nil it is used as the base http.Client inside the
// retryable wrapper (replacing the default transport). When nil the service
// constructs a default client with production-suitable connection pool settings.
func NewHTTPService(timeout time.Duration, maxRetry int, logger *slog.Logger, customClient *http.Client) HTTPService {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = maxRetry
	retryClient.RetryWaitMin = 1 * time.Second
	retryClient.RetryWaitMax = 5 * time.Second
	retryClient.Logger = nil // suppress retryablehttp's own logger; we use slog
	retryClient.CheckRetry = networkOnlyRetryPolicy

	if customClient != nil {
		// Use the caller's client as-is. The caller is responsible for setting
		// a Timeout on the provided *http.Client; the cfg.Timeout value is not
		// applied automatically when a custom client is supplied.
		retryClient.HTTPClient = customClient
	} else {
		// Configure an explicit transport with production-suitable connection pool
		// settings. Go's default transport caps MaxIdleConnsPerHost at 2, which
		// causes connection exhaustion under concurrent load since all requests go
		// to the same host. These values are sized for a typical management-plane
		// workload; tune MaxIdleConnsPerHost upward if you issue many parallel calls.
		// Although Go 1.18+ defaults to TLS 1.2, it's non-obvious so we explicity set min TLS to 1.2
		retryClient.HTTPClient = &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:          100,
				MaxIdleConnsPerHost:   20,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
			},
		}
	}

	return &httpService{
		timeout: timeout,
		client:  retryClient,
		logger:  logger,
	}
}

// Close releases idle connections held by the underlying HTTP transport.
// It delegates to http.Client.CloseIdleConnections on the retryablehttp client's
// inner http.Client, draining the connection pool. Call this (typically via defer)
// when the service is no longer needed.
func (s *httpService) Close() {
	s.client.HTTPClient.CloseIdleConnections()
}

// Get performs an HTTP GET request with the provided headers.
func (s *httpService) Get(ctx context.Context, url string, headers map[string]string) (*HTTPResponse, error) {
	return s.doRequest(ctx, http.MethodGet, url, headers, "")
}

// Post performs an HTTP POST request with the provided headers and body.
func (s *httpService) Post(ctx context.Context, url string, headers map[string]string, body string) (*HTTPResponse, error) {
	return s.doRequest(ctx, http.MethodPost, url, headers, body)
}

// Put performs an HTTP PUT request with the provided headers and body.
func (s *httpService) Put(ctx context.Context, url string, headers map[string]string, body string) (*HTTPResponse, error) {
	return s.doRequest(ctx, http.MethodPut, url, headers, body)
}

// Patch performs an HTTP PATCH request with the provided headers and body.
func (s *httpService) Patch(ctx context.Context, url string, headers map[string]string, body string) (*HTTPResponse, error) {
	return s.doRequest(ctx, http.MethodPatch, url, headers, body)
}

// Delete performs an HTTP DELETE request with the provided headers.
func (s *httpService) Delete(ctx context.Context, url string, headers map[string]string) (*HTTPResponse, error) {
	return s.doRequest(ctx, http.MethodDelete, url, headers, "")
}

// doRequest is the shared implementation for all HTTP methods. It builds the
// request, attaches headers and the caller's context, executes it via the
// retryable client, and reads the response body up to DefaultMaxResponseSize.
func (s *httpService) doRequest(ctx context.Context, method, url string, headers map[string]string, body string) (*HTTPResponse, error) {
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := retryablehttp.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req = req.WithContext(ctx)

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	s.logger.DebugContext(ctx, "executing HTTP request",
		slog.String("method", method),
		slog.String("url", url),
	)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	limitedReader := io.LimitReader(resp.Body, DefaultMaxResponseSize)
	responseBody, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	s.logger.DebugContext(ctx, "HTTP response received",
		slog.String("method", method),
		slog.String("url", url),
		slog.Int("status", resp.StatusCode),
	)

	return &HTTPResponse{
		StatusCode: resp.StatusCode,
		Body:       responseBody,
		Headers:    resp.Header,
	}, nil
}
