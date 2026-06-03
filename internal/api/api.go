// Package api implements the authenticated HTTP request layer for the Aura API.
// It handles OAuth token acquisition and refresh, URL construction, and error parsing.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/neo4j-contrib/aura-go-sdk/v2/internal/httpclient"
	"github.com/neo4j-contrib/aura-go-sdk/v2/internal/utils"
)

// Error implements the error interface.
func (e *Error) Error() string {
	if len(e.Details) == 0 {
		return fmt.Sprintf("API error (status %d): %s", e.StatusCode, e.Message)
	}

	detail := e.Details[0]
	msg := fmt.Sprintf("API error (status %d): %s - %s", e.StatusCode, e.Message, detail.Message)
	if len(e.Details) > 1 {
		msg += fmt.Sprintf(" (and %d more error(s))", len(e.Details)-1)
	}
	return msg
}

// AllErrors returns all error messages as a slice.
func (e *Error) AllErrors() []string {
	errors := []string{e.Message}
	for _, detail := range e.Details {
		errors = append(errors, detail.Message)
	}
	return errors
}

// HasMultipleErrors returns true if there are multiple error details.
func (e *Error) HasMultipleErrors() bool {
	return len(e.Details) > 1
}

// IsNotFound returns true if the error is a 404.
func (e *Error) IsNotFound() bool {
	return e.StatusCode == http.StatusNotFound
}

// IsUnauthorized returns true if the error is a 401.
func (e *Error) IsUnauthorized() bool {
	return e.StatusCode == http.StatusUnauthorized
}

// IsBadRequest returns true if the error is a 400.
func (e *Error) IsBadRequest() bool {
	return e.StatusCode == http.StatusBadRequest
}

// NewRequestService creates a new RequestService. It constructs its own HTTP
// transport layer internally — callers do not need to know about or create an
// httpclient.
//
// When cfg.HTTPClient is non-nil it is used as the base http.Client inside the
// retryable wrapper, letting callers inject custom transports (mTLS, proxies,
// testing). When nil a default client with production-suitable settings is used.
func NewRequestService(cfg Config, logger *slog.Logger) RequestService {
	httpSvc := httpclient.NewHTTPService(cfg.Timeout, cfg.MaxRetry, logger, cfg.HTTPClient)

	userAgent := cfg.UserAgent
	if userAgent == "" {
		userAgent = "query-go-client"
	}

	return &apiRequestService{
		httpClient: httpSvc,
		authMgr: &authManager{
			username: cfg.Username,
			password: cfg.Password,
			logger:   logger,
		},
		baseURL:        cfg.BaseURL,
		endpointBase:   cfg.BaseURL + "/" + cfg.APIVersion,
		userAgent:      userAgent,
		defaultHeaders: cfg.DefaultHeaders,
		logger:         logger,
	}
}

// Close releases idle connections held by the underlying HTTP transport by
// delegating to the HTTPService.Close() method. Call this (typically via defer)
// when the RequestService is no longer needed.
func (s *apiRequestService) Close() {
	s.httpClient.Close()
}

// Get performs an authenticated GET request.
func (s *apiRequestService) Get(ctx context.Context, endpoint string) (*Response, error) {
	return s.doAuthenticatedRequest(ctx, http.MethodGet, endpoint, "")
}

// Post performs an authenticated POST request.
func (s *apiRequestService) Post(ctx context.Context, endpoint string, body string) (*Response, error) {
	return s.doAuthenticatedRequest(ctx, http.MethodPost, endpoint, body)
}

// Put performs an authenticated PUT request.
func (s *apiRequestService) Put(ctx context.Context, endpoint string, body string) (*Response, error) {
	return s.doAuthenticatedRequest(ctx, http.MethodPut, endpoint, body)
}

// Patch performs an authenticated PATCH request.
func (s *apiRequestService) Patch(ctx context.Context, endpoint string, body string) (*Response, error) {
	return s.doAuthenticatedRequest(ctx, http.MethodPatch, endpoint, body)
}

// Delete performs an authenticated DELETE request.
func (s *apiRequestService) Delete(ctx context.Context, endpoint string) (*Response, error) {
	return s.doAuthenticatedRequest(ctx, http.MethodDelete, endpoint, "")
}

// doAuthenticatedRequest handles the common pattern of making an authenticated
// API request. It trusts the deadline already set on ctx by the calling service
// layer — no additional timeout is applied here.
func (s *apiRequestService) doAuthenticatedRequest(ctx context.Context, method, endpoint, body string) (*Response, error) {
	if err := ctx.Err(); err != nil {
		s.logger.ErrorContext(ctx, "context already cancelled before function", slog.String("error", err.Error()))
		return nil, err
	}

	// Handle both relative endpoints (the common case) and absolute URLs
	// (used by the Prometheus service). For relative endpoints, trim any
	// stray leading/trailing slashes before joining so a misplaced "/"
	// never produces a double-slash in the final URL.
	var fullURL string
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		fullURL = endpoint
	} else {
		fullURL = strings.TrimRight(s.endpointBase, "/") + "/" + strings.TrimLeft(endpoint, "/")
	}

	tokenType, token, err := s.authMgr.ensureValidToken(ctx, s.baseURL, s.httpClient)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to obtain authentication token", slog.String("error", err.Error()))
		return nil, err
	}

	// Start with any caller-supplied default headers, then overwrite with the
	// required protocol headers so they can never be replaced.
	headers := make(map[string]string, len(s.defaultHeaders)+3)
	maps.Copy(headers, s.defaultHeaders)
	headers["Content-Type"] = "application/json"
	headers["User-Agent"] = s.userAgent
	headers["Authorization"] = tokenType + " " + token

	s.logger.DebugContext(ctx, "making authenticated API request",
		slog.String("method", method),
		slog.String("endpoint", fullURL),
	)

	var resp *httpclient.HTTPResponse

	switch method {
	case http.MethodGet:
		resp, err = s.httpClient.Get(ctx, fullURL, headers)
	case http.MethodPost:
		resp, err = s.httpClient.Post(ctx, fullURL, headers, body)
	case http.MethodPut:
		resp, err = s.httpClient.Put(ctx, fullURL, headers, body)
	case http.MethodPatch:
		resp, err = s.httpClient.Patch(ctx, fullURL, headers, body)
	case http.MethodDelete:
		resp, err = s.httpClient.Delete(ctx, fullURL, headers)
	default:
		return nil, fmt.Errorf("unsupported HTTP method: %s", method)
	}

	if err != nil {
		s.logger.ErrorContext(ctx, "HTTP request failed",
			slog.String("method", method),
			slog.String("endpoint", fullURL),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := parseError(resp.Body, resp.StatusCode)
		s.logger.DebugContext(ctx, "API returned error",
			slog.String("method", method),
			slog.String("endpoint", fullURL),
			slog.Int("statusCode", resp.StatusCode),
			slog.String("message", apiErr.Message),
		)
		return nil, apiErr
	}

	s.logger.DebugContext(ctx, "API request successful",
		slog.String("method", method),
		slog.String("endpoint", fullURL),
		slog.Int("statusCode", resp.StatusCode),
	)

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       resp.Body,
	}, nil
}

// ensureValidToken gets or refreshes the authentication token and returns it
// to the caller. Token fields are always read while the mutex is held to
// prevent data races.
func (am *authManager) ensureValidToken(ctx context.Context, baseURL string, httpSvc httpclient.HTTPService) (tokenType, token string, err error) {
	am.mu.RLock()
	if len(am.token) > 0 && time.Now().Unix() <= am.expiresAt-60 {
		t, tt := am.token, am.tokenType
		am.mu.RUnlock()
		return tt, t, nil
	}
	am.mu.RUnlock()

	am.mu.Lock()
	defer am.mu.Unlock()

	// Double-check after acquiring the write lock — another goroutine may have
	// refreshed the token while we were waiting.
	if len(am.token) > 0 && time.Now().Unix() <= am.expiresAt-60 {
		return am.tokenType, am.token, nil
	}

	am.logger.DebugContext(ctx, "obtaining new authentication token")

	auth := "Basic " + utils.Base64Encode(am.clientID, am.clientSecret)

	headers := map[string]string{
		"Content-Type":  "application/x-www-form-urlencoded",
		"Authorization": auth,
	}

	body := url.Values{}
	body.Set("grant_type", "client_credentials")

	resp, err := httpSvc.Post(ctx, baseURL+"/oauth/token", headers, body.Encode())
	if err != nil {
		am.logger.DebugContext(ctx, "failed to obtain token", slog.String("error", err.Error()))
		return "", "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := parseError(resp.Body, resp.StatusCode)
		am.logger.DebugContext(ctx, "token request failed",
			slog.Int("statusCode", resp.StatusCode),
			slog.String("error", apiErr.Message),
		)
		return "", "", apiErr
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(resp.Body, &tokenResp); err != nil {
		am.logger.DebugContext(ctx, "failed to parse token response", slog.String("error", err.Error()))
		return "", "", fmt.Errorf("failed to parse token response: %w", err)
	}

	// Check that we have a bearer token as this is the only supported token type
	// returned by the Aura API.  Anything else could indicate interference
	if tokenResp.TokenType != "Bearer" {
		return "", "", fmt.Errorf("token type is not valid: %s", tokenResp.TokenType)
	}

	// Check tokenResp.ExpiresIn
	// A value of 0 or negative causes the SDK to re-fetch a token on every single API request (token endpoint flood).
	// A very large value keeps a revoked token in use indefinitely.
	if tokenResp.ExpiresIn <= 0 || tokenResp.ExpiresIn > 86400*365 {
		return "", "", fmt.Errorf("invalid expires_in value: %d", tokenResp.ExpiresIn)
	}

	am.token = tokenResp.AccessToken
	am.tokenType = tokenResp.TokenType
	am.expiresAt = time.Now().Unix() + tokenResp.ExpiresIn

	am.logger.DebugContext(ctx, "token obtained successfully", slog.Int64("expiresIn", tokenResp.ExpiresIn))

	return am.tokenType, am.token, nil
}

// parseError attempts to parse an error response body from the API.
func parseError(responseBody []byte, statusCode int) *Error {
	apiErr := &Error{
		StatusCode: statusCode,
		Message:    http.StatusText(statusCode),
	}

	if len(responseBody) == 0 {
		return apiErr
	}

	var errResponse struct {
		Message string        `json:"message"`
		Errors  []ErrorDetail `json:"errors"`
		Details []ErrorDetail `json:"details"`
	}

	if err := json.Unmarshal(responseBody, &errResponse); err == nil {
		if errResponse.Message != "" {
			apiErr.Message = errResponse.Message
		}
		if len(errResponse.Errors) > 0 {
			apiErr.Details = errResponse.Errors
		} else if len(errResponse.Details) > 0 {
			apiErr.Details = errResponse.Details
		}
	}

	return apiErr
}
