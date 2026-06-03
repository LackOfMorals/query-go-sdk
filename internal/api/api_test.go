package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/neo4j-contrib/aura-go-sdk/v2/internal/httpclient"
	"github.com/neo4j-contrib/aura-go-sdk/v2/internal/testutil"
)

func testLogger() *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelWarn}
	return slog.New(slog.NewTextHandler(os.Stderr, opts))
}

func newTestService(mock *testutil.MockHTTPService) *apiRequestService {
	return &apiRequestService{
		httpClient: mock,
		authMgr: &authManager{
			clientID:     "test-client-id",
			clientSecret: "test-client-secret",
			logger:       testLogger(),
		},
		baseURL:      "https://api.neo4j.io",
		endpointBase: "https://api.neo4j.io/v1",
		logger:       testLogger(),
	}
}

func newTestServiceWithToken(mock *testutil.MockHTTPService) *apiRequestService {
	svc := newTestService(mock)
	svc.authMgr.token = "test-access-token"
	svc.authMgr.tokenType = "Bearer"
	svc.authMgr.expiresAt = time.Now().Unix() + 3600
	return svc
}

func tokenResponseBody(accessToken, tokenType string, expiresIn int64) []byte {
	b, _ := json.Marshal(tokenResponse{ //nolint:gosec
		AccessToken: accessToken,
		TokenType:   tokenType,
		ExpiresIn:   expiresIn,
	})
	return b
}

// ============================================================================
// parseError
// ============================================================================

func TestParseError_EmptyBody(t *testing.T) {
	err := parseError(nil, http.StatusNotFound)
	if err.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", err.StatusCode)
	}
	if err.Message != "Not Found" {
		t.Errorf("expected message 'Not Found', got '%s'", err.Message)
	}
	if len(err.Details) != 0 {
		t.Errorf("expected no details, got %d", len(err.Details))
	}
}

func TestParseError_MessageField(t *testing.T) {
	body := []byte(`{"message":"Instance not found"}`)
	err := parseError(body, http.StatusNotFound)
	if err.Message != "Instance not found" {
		t.Errorf("expected message 'Instance not found', got '%s'", err.Message)
	}
}

func TestParseError_ErrorsArray(t *testing.T) {
	body := []byte(`{"message":"Validation failed","errors":[{"message":"name is required","field":"name"},{"message":"region is required","field":"region"}]}`)
	err := parseError(body, http.StatusBadRequest)
	if len(err.Details) != 2 {
		t.Fatalf("expected 2 details, got %d", len(err.Details))
	}
	if err.Details[0].Message != "name is required" {
		t.Errorf("expected first detail 'name is required', got '%s'", err.Details[0].Message)
	}
	if err.Details[0].Field != "name" {
		t.Errorf("expected first detail field 'name', got '%s'", err.Details[0].Field)
	}
}

func TestParseError_DetailsArray(t *testing.T) {
	body := []byte(`{"message":"Validation failed","details":[{"message":"memory must be positive","reason":"invalid_value"}]}`)
	err := parseError(body, http.StatusUnprocessableEntity)
	if len(err.Details) != 1 {
		t.Fatalf("expected 1 detail, got %d", len(err.Details))
	}
	if err.Details[0].Reason != "invalid_value" {
		t.Errorf("expected reason 'invalid_value', got '%s'", err.Details[0].Reason)
	}
}

func TestParseError_ErrorsArrayTakesPrecedenceOverDetails(t *testing.T) {
	body := []byte(`{"message":"conflict","errors":[{"message":"from errors"}],"details":[{"message":"from details"}]}`)
	err := parseError(body, http.StatusBadRequest)
	if err.Details[0].Message != "from errors" {
		t.Errorf("expected 'from errors', got '%s'", err.Details[0].Message)
	}
}

func TestParseError_InvalidJSON_FallsBackToDefault(t *testing.T) {
	body := []byte(`not valid json`)
	err := parseError(body, http.StatusInternalServerError)
	if err.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", err.StatusCode)
	}
	if err.Message != "Internal Server Error" {
		t.Errorf("expected 'Internal Server Error', got '%s'", err.Message)
	}
}

func TestParseError_EmptyMessageField_FallsBackToStatusText(t *testing.T) {
	body := []byte(`{"message":""}`)
	err := parseError(body, http.StatusForbidden)
	if err.Message != "Forbidden" {
		t.Errorf("expected 'Forbidden' fallback, got '%s'", err.Message)
	}
}

// ============================================================================
// HTTP method routing and URL construction
// ============================================================================

func TestAPIService_Get_RoutesCorrectly(t *testing.T) {
	mock := testutil.NewMockHTTPService()
	mock.WithResponse(http.StatusOK, `{"data":[]}`)
	svc := newTestServiceWithToken(mock)

	_, err := svc.Get(context.Background(), "instances")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.LastMethod != "GET" {
		t.Errorf("expected GET, got %s", mock.LastMethod)
	}
	if mock.LastURL != "https://api.neo4j.io/v1/instances" {
		t.Errorf("unexpected URL: %s", mock.LastURL)
	}
}

func TestAPIService_Post_RoutesCorrectly(t *testing.T) {
	mock := testutil.NewMockHTTPService()
	mock.WithResponse(http.StatusOK, `{"data":{}}`)
	svc := newTestServiceWithToken(mock)

	body := `{"name":"my-instance"}`
	_, err := svc.Post(context.Background(), "instances", body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.LastMethod != "POST" {
		t.Errorf("expected POST, got %s", mock.LastMethod)
	}
	if mock.LastURL != "https://api.neo4j.io/v1/instances" {
		t.Errorf("unexpected URL: %s", mock.LastURL)
	}
	if mock.LastBody != body {
		t.Errorf("expected body '%s', got '%s'", body, mock.LastBody)
	}
}

func TestAPIService_Put_RoutesCorrectly(t *testing.T) {
	mock := testutil.NewMockHTTPService()
	mock.WithResponse(http.StatusOK, `{"data":{}}`)
	svc := newTestServiceWithToken(mock)

	_, err := svc.Put(context.Background(), "instances/aaaa1234", `{"name":"updated"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.LastMethod != "PUT" {
		t.Errorf("expected PUT, got %s", mock.LastMethod)
	}
	if mock.LastURL != "https://api.neo4j.io/v1/instances/aaaa1234" {
		t.Errorf("unexpected URL: %s", mock.LastURL)
	}
}

func TestAPIService_Patch_RoutesCorrectly(t *testing.T) {
	mock := testutil.NewMockHTTPService()
	mock.WithResponse(http.StatusOK, `{"data":{}}`)
	svc := newTestServiceWithToken(mock)

	_, err := svc.Patch(context.Background(), "instances/aaaa1234", `{"memory":"16GB"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.LastMethod != "PATCH" {
		t.Errorf("expected PATCH, got %s", mock.LastMethod)
	}
}

func TestAPIService_Delete_RoutesCorrectly(t *testing.T) {
	mock := testutil.NewMockHTTPService()
	mock.WithResponse(http.StatusOK, `{"data":{}}`)
	svc := newTestServiceWithToken(mock)

	_, err := svc.Delete(context.Background(), "instances/aaaa1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.LastMethod != "DELETE" {
		t.Errorf("expected DELETE, got %s", mock.LastMethod)
	}
	if mock.LastURL != "https://api.neo4j.io/v1/instances/aaaa1234" {
		t.Errorf("unexpected URL: %s", mock.LastURL)
	}
}

func TestAPIService_URLConstruction_NestedPath(t *testing.T) {
	mock := testutil.NewMockHTTPService()
	mock.WithResponse(http.StatusOK, `{"data":[]}`)
	svc := newTestServiceWithToken(mock)

	_, err := svc.Get(context.Background(), "instances/aaaa1234/snapshots")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "https://api.neo4j.io/v1/instances/aaaa1234/snapshots"
	if mock.LastURL != expected {
		t.Errorf("expected URL '%s', got '%s'", expected, mock.LastURL)
	}
}

// ============================================================================
// Request headers
// ============================================================================

func TestAPIService_Headers_ContentType(t *testing.T) {
	mock := testutil.NewMockHTTPService()
	mock.WithResponse(http.StatusOK, `{"data":[]}`)
	svc := newTestServiceWithToken(mock)

	_, err := svc.Get(context.Background(), "instances")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.LastHeaders["Content-Type"] != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", mock.LastHeaders["Content-Type"])
	}
}

func TestAPIService_Headers_UserAgent(t *testing.T) {
	mock := testutil.NewMockHTTPService()
	mock.WithResponse(http.StatusOK, `{"data":[]}`)
	svc := newTestServiceWithToken(mock)
	svc.userAgent = "aura-go-client/v1.8.0"

	_, err := svc.Get(context.Background(), "instances")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.LastHeaders["User-Agent"] != "aura-go-client/v1.8.0" {
		t.Errorf("expected User-Agent 'aura-go-client/v1.8.0', got '%s'", mock.LastHeaders["User-Agent"])
	}
}

func TestAPIService_Headers_AuthorizationFormat(t *testing.T) {
	mock := testutil.NewMockHTTPService()
	mock.WithResponse(http.StatusOK, `{"data":[]}`)
	svc := newTestServiceWithToken(mock)

	_, err := svc.Get(context.Background(), "instances")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.LastHeaders["Authorization"] != "Bearer test-access-token" {
		t.Errorf("expected Authorization 'Bearer test-access-token', got '%s'", mock.LastHeaders["Authorization"])
	}
}

func TestAPIService_DefaultHeaders_ReachServer(t *testing.T) {
	mock := testutil.NewMockHTTPService()
	mock.WithResponse(http.StatusOK, `{"data":[]}`)
	svc := newTestServiceWithToken(mock)
	svc.defaultHeaders = map[string]string{
		"X-Request-ID": "req-abc-123",
		"X-Tenant":     "my-tenant",
	}

	_, err := svc.Get(context.Background(), "instances")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.LastHeaders["X-Request-ID"] != "req-abc-123" {
		t.Errorf("expected X-Request-ID 'req-abc-123', got '%s'", mock.LastHeaders["X-Request-ID"])
	}
	if mock.LastHeaders["X-Tenant"] != "my-tenant" {
		t.Errorf("expected X-Tenant 'my-tenant', got '%s'", mock.LastHeaders["X-Tenant"])
	}
}

func TestAPIService_DefaultHeaders_CannotOverrideProtected(t *testing.T) {
	mock := testutil.NewMockHTTPService()
	mock.WithResponse(http.StatusOK, `{"data":[]}`)
	svc := newTestServiceWithToken(mock)
	svc.userAgent = "aura-go-client/v1.0"
	// Attempt to override all three protected headers via defaultHeaders.
	// The values set here must NOT appear in the outgoing request.
	svc.defaultHeaders = map[string]string{
		"Authorization": "Bearer sneaky",
		"Content-Type":  "text/plain",
		"User-Agent":    "evil-agent/1.0",
	}

	_, err := svc.Get(context.Background(), "instances")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.LastHeaders["Authorization"] != "Bearer test-access-token" {
		t.Errorf("Authorization was overridden; got '%s'", mock.LastHeaders["Authorization"])
	}
	if mock.LastHeaders["Content-Type"] != "application/json" {
		t.Errorf("Content-Type was overridden; got '%s'", mock.LastHeaders["Content-Type"])
	}
	if mock.LastHeaders["User-Agent"] != "aura-go-client/v1.0" {
		t.Errorf("User-Agent was overridden; got '%s'", mock.LastHeaders["User-Agent"])
	}
}

func TestAPIService_DefaultHeaders_MergedOnEveryMethod(t *testing.T) {
	customHeader := map[string]string{"X-Correlation-ID": "corr-999"}

	methods := []struct {
		name string
		call func(svc *apiRequestService) error
	}{
		{"GET", func(svc *apiRequestService) error {
			_, err := svc.Get(context.Background(), "instances")
			return err
		}},
		{"POST", func(svc *apiRequestService) error {
			_, err := svc.Post(context.Background(), "instances", `{}`)
			return err
		}},
		{"PUT", func(svc *apiRequestService) error {
			_, err := svc.Put(context.Background(), "instances/id", `{}`)
			return err
		}},
		{"PATCH", func(svc *apiRequestService) error {
			_, err := svc.Patch(context.Background(), "instances/id", `{}`)
			return err
		}},
		{"DELETE", func(svc *apiRequestService) error {
			_, err := svc.Delete(context.Background(), "instances/id")
			return err
		}},
	}

	for _, m := range methods {
		t.Run(m.name, func(t *testing.T) {
			mock := testutil.NewMockHTTPService()
			mock.WithResponse(http.StatusOK, `{"data":[]}`)
			svc := newTestServiceWithToken(mock)
			svc.defaultHeaders = customHeader

			if err := m.call(svc); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if mock.LastHeaders["X-Correlation-ID"] != "corr-999" {
				t.Errorf("%s: expected X-Correlation-ID 'corr-999', got '%s'", m.name, mock.LastHeaders["X-Correlation-ID"])
			}
		})
	}
}

// ============================================================================
// Response handling
// ============================================================================

func TestAPIService_Response_BodyAndStatusReturned(t *testing.T) {
	expectedBody := []byte(`{"data":{"id":"aaaa1234"}}`)
	mock := testutil.NewMockHTTPService()
	mock.WithResponse(http.StatusOK, string(expectedBody))
	svc := newTestServiceWithToken(mock)

	resp, err := svc.Get(context.Background(), "instances/aaaa1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if string(resp.Body) != string(expectedBody) {
		t.Errorf("expected body '%s', got '%s'", expectedBody, resp.Body)
	}
}

func TestAPIService_Response_201IsSuccess(t *testing.T) {
	mock := testutil.NewMockHTTPService()
	mock.WithResponse(http.StatusCreated, `{"data":{"id":"new-id"}}`)
	svc := newTestServiceWithToken(mock)

	resp, err := svc.Post(context.Background(), "instances", `{}`)
	if err != nil {
		t.Fatalf("unexpected error for 201: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status 201, got %d", resp.StatusCode)
	}
}

func TestAPIService_Response_299IsSuccess(t *testing.T) {
	mock := testutil.NewMockHTTPService()
	mock.Response = &httpclient.HTTPResponse{StatusCode: 299, Body: []byte(`{}`)}
	svc := newTestServiceWithToken(mock)

	_, err := svc.Get(context.Background(), "instances")
	if err != nil {
		t.Fatalf("unexpected error for 299: %v", err)
	}
}

// ============================================================================
// API error responses (non-2xx → *Error)
// ============================================================================

func TestAPIService_ErrorResponse_400(t *testing.T) {
	body := `{"message":"Bad Request","errors":[{"message":"name is required","field":"name"}]}`
	mock := testutil.NewMockHTTPService()
	mock.WithResponse(http.StatusBadRequest, body)
	svc := newTestServiceWithToken(mock)

	_, err := svc.Post(context.Background(), "instances", `{}`)
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
	apiErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if !apiErr.IsBadRequest() {
		t.Error("expected IsBadRequest() to be true")
	}
	if apiErr.Details[0].Field != "name" {
		t.Errorf("expected field 'name', got '%s'", apiErr.Details[0].Field)
	}
}

func TestAPIService_ErrorResponse_401(t *testing.T) {
	mock := testutil.NewMockHTTPService()
	mock.WithResponse(http.StatusUnauthorized, `{"message":"Invalid credentials"}`)
	svc := newTestServiceWithToken(mock)

	_, err := svc.Get(context.Background(), "instances")
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	apiErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if !apiErr.IsUnauthorized() {
		t.Error("expected IsUnauthorized() to be true")
	}
}

func TestAPIService_ErrorResponse_404(t *testing.T) {
	mock := testutil.NewMockHTTPService()
	mock.WithResponse(http.StatusNotFound, `{"message":"Instance not found"}`)
	svc := newTestServiceWithToken(mock)

	_, err := svc.Get(context.Background(), "instances/aaaa1234")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	apiErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if !apiErr.IsNotFound() {
		t.Error("expected IsNotFound() to be true")
	}
	if apiErr.Message != "Instance not found" {
		t.Errorf("expected message 'Instance not found', got '%s'", apiErr.Message)
	}
}

func TestAPIService_HTTPClientError_Propagated(t *testing.T) {
	networkErr := fmt.Errorf("connection refused")
	mock := testutil.NewMockHTTPService()
	mock.WithError(networkErr)
	svc := newTestServiceWithToken(mock)

	_, err := svc.Get(context.Background(), "instances")
	if err == nil {
		t.Fatal("expected error to be propagated")
	}
	if !errors.Is(err, networkErr) {
		t.Errorf("expected networkErr, got %v", err)
	}
}

// ============================================================================
// Context handling
// ============================================================================

func TestAPIService_CancelledContext_RejectedBeforeHTTPCall(t *testing.T) {
	mock := testutil.NewMockHTTPService()
	mock.WithResponse(http.StatusOK, `{"data":[]}`)
	svc := newTestServiceWithToken(mock)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.Get(ctx, "instances")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if mock.CallCount != 0 {
		t.Errorf("expected 0 HTTP calls, got %d", mock.CallCount)
	}
}

func TestAPIService_ExpiredDeadline_RejectedBeforeHTTPCall(t *testing.T) {
	mock := testutil.NewMockHTTPService()
	mock.WithResponse(http.StatusOK, `{"data":[]}`)
	svc := newTestServiceWithToken(mock)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	_, err := svc.Get(ctx, "instances")
	if err == nil {
		t.Fatal("expected error for expired deadline")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
	if mock.CallCount != 0 {
		t.Errorf("expected 0 HTTP calls, got %d", mock.CallCount)
	}
}

// ============================================================================
// Token acquisition (ensureValidToken)
// ============================================================================

type sequencedMock struct {
	responses []*httpclient.HTTPResponse
	errors    []error
	mu        sync.Mutex
	callIndex int
	calls     []struct {
		method, url, body string
		headers           map[string]string
	}
}

func (m *sequencedMock) next() (*httpclient.HTTPResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	i := m.callIndex
	m.callIndex++
	if i >= len(m.responses) {
		return nil, fmt.Errorf("sequencedMock: unexpected call index %d", i)
	}
	return m.responses[i], m.errors[i]
}

func (m *sequencedMock) record(method, url, body string, headers map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, struct {
		method, url, body string
		headers           map[string]string
	}{method, url, body, headers})
}

func (m *sequencedMock) Get(_ context.Context, url string, headers map[string]string) (*httpclient.HTTPResponse, error) {
	m.record("GET", url, "", headers)
	return m.next()
}
func (m *sequencedMock) Post(_ context.Context, url string, headers map[string]string, body string) (*httpclient.HTTPResponse, error) {
	m.record("POST", url, body, headers)
	return m.next()
}
func (m *sequencedMock) Put(_ context.Context, url string, headers map[string]string, body string) (*httpclient.HTTPResponse, error) {
	m.record("PUT", url, body, headers)
	return m.next()
}
func (m *sequencedMock) Patch(_ context.Context, url string, headers map[string]string, body string) (*httpclient.HTTPResponse, error) {
	m.record("PATCH", url, body, headers)
	return m.next()
}
func (m *sequencedMock) Delete(_ context.Context, url string, headers map[string]string) (*httpclient.HTTPResponse, error) {
	m.record("DELETE", url, "", headers)
	return m.next()
}

func (m *sequencedMock) Close() {}

func newSequencedMock(responses []*httpclient.HTTPResponse, errs []error) *sequencedMock {
	return &sequencedMock{responses: responses, errors: errs}
}

func TestToken_FetchedOnFirstCall(t *testing.T) {
	tokenBody := tokenResponseBody("fresh-token", "Bearer", 3600)
	apiBody := []byte(`{"data":[]}`)

	mock := newSequencedMock(
		[]*httpclient.HTTPResponse{
			{StatusCode: http.StatusOK, Body: tokenBody},
			{StatusCode: http.StatusOK, Body: apiBody},
		},
		[]error{nil, nil},
	)

	svc := &apiRequestService{
		httpClient:   mock,
		authMgr:      &authManager{clientID: "id", clientSecret: "secret", logger: testLogger()},
		baseURL:      "https://api.neo4j.io",
		endpointBase: "https://api.neo4j.io/v1",
		logger:       testLogger(),
	}

	resp, err := svc.Get(context.Background(), "instances")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp.Body) != string(apiBody) {
		t.Errorf("expected api body, got %s", resp.Body)
	}
	if len(mock.calls) < 2 {
		t.Fatalf("expected 2 HTTP calls, got %d", len(mock.calls))
	}
	if !strings.HasSuffix(mock.calls[0].url, "/oauth/token") {
		t.Errorf("expected first call to /oauth/token, got %s", mock.calls[0].url)
	}
	if !strings.HasPrefix(mock.calls[0].headers["Authorization"], "Basic ") {
		t.Errorf("expected Basic auth on token call, got %s", mock.calls[0].headers["Authorization"])
	}
	if mock.calls[1].headers["Authorization"] != "Bearer fresh-token" {
		t.Errorf("expected Bearer fresh-token on API call, got %s", mock.calls[1].headers["Authorization"])
	}
}

func TestToken_ReusedWhenStillValid(t *testing.T) {
	mock := testutil.NewMockHTTPService()
	mock.WithResponse(http.StatusOK, `{"data":[]}`)
	svc := newTestServiceWithToken(mock)

	for i := range 2 {
		_, err := svc.Get(context.Background(), "instances")
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
	}
	if mock.CallCount != 2 {
		t.Errorf("expected 2 HTTP calls (no token refresh), got %d", mock.CallCount)
	}
}

func TestToken_RefreshedWhenExpired(t *testing.T) {
	tokenBody := tokenResponseBody("refreshed-token", "Bearer", 3600)
	apiBody := []byte(`{"data":[]}`)

	mock := newSequencedMock(
		[]*httpclient.HTTPResponse{
			{StatusCode: http.StatusOK, Body: tokenBody},
			{StatusCode: http.StatusOK, Body: apiBody},
		},
		[]error{nil, nil},
	)

	svc := &apiRequestService{
		httpClient: mock,
		authMgr: &authManager{
			clientID:     "id",
			clientSecret: "secret",
			token:        "expired-token",
			tokenType:    "Bearer",
			expiresAt:    time.Now().Unix() - 1,
			logger:       testLogger(),
		},
		baseURL:      "https://api.neo4j.io",
		endpointBase: "https://api.neo4j.io/v1",
		logger:       testLogger(),
	}

	_, err := svc.Get(context.Background(), "instances")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.calls) < 2 {
		t.Fatalf("expected 2 HTTP calls, got %d", len(mock.calls))
	}
	if !strings.HasSuffix(mock.calls[0].url, "/oauth/token") {
		t.Errorf("expected first call to be token refresh, got %s", mock.calls[0].url)
	}
	if mock.calls[1].headers["Authorization"] != "Bearer refreshed-token" {
		t.Errorf("expected refreshed token on API call, got %s", mock.calls[1].headers["Authorization"])
	}
}

func TestToken_RefreshedWithin60SecondsOfExpiry(t *testing.T) {
	tokenBody := tokenResponseBody("renewed-token", "Bearer", 3600)
	apiBody := []byte(`{"data":[]}`)

	mock := newSequencedMock(
		[]*httpclient.HTTPResponse{
			{StatusCode: http.StatusOK, Body: tokenBody},
			{StatusCode: http.StatusOK, Body: apiBody},
		},
		[]error{nil, nil},
	)

	svc := &apiRequestService{
		httpClient: mock,
		authMgr: &authManager{
			clientID:     "id",
			clientSecret: "secret",
			token:        "nearly-expired-token",
			tokenType:    "Bearer",
			expiresAt:    time.Now().Unix() + 30, // within 60s buffer
			logger:       testLogger(),
		},
		baseURL:      "https://api.neo4j.io",
		endpointBase: "https://api.neo4j.io/v1",
		logger:       testLogger(),
	}

	_, err := svc.Get(context.Background(), "instances")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.calls) < 2 || !strings.HasSuffix(mock.calls[0].url, "/oauth/token") {
		t.Error("expected token refresh for token expiring within 60 seconds")
	}
}

func TestToken_TokenEndpointError_Propagated(t *testing.T) {
	networkErr := fmt.Errorf("token endpoint unreachable")
	mock := newSequencedMock([]*httpclient.HTTPResponse{nil}, []error{networkErr})

	svc := &apiRequestService{
		httpClient:   mock,
		authMgr:      &authManager{clientID: "id", clientSecret: "secret", logger: testLogger()},
		baseURL:      "https://api.neo4j.io",
		endpointBase: "https://api.neo4j.io/v1",
		logger:       testLogger(),
	}

	_, err := svc.Get(context.Background(), "instances")
	if err == nil {
		t.Fatal("expected error from token endpoint failure")
	}
	if !errors.Is(err, networkErr) {
		t.Errorf("expected networkErr, got %v", err)
	}
}

func TestToken_TokenEndpointNonSuccess_ReturnsAPIError(t *testing.T) {
	body := []byte(`{"message":"invalid_client"}`)
	mock := newSequencedMock(
		[]*httpclient.HTTPResponse{{StatusCode: http.StatusUnauthorized, Body: body}},
		[]error{nil},
	)

	svc := &apiRequestService{
		httpClient:   mock,
		authMgr:      &authManager{clientID: "id", clientSecret: "secret", logger: testLogger()},
		baseURL:      "https://api.neo4j.io",
		endpointBase: "https://api.neo4j.io/v1",
		logger:       testLogger(),
	}

	_, err := svc.Get(context.Background(), "instances")
	if err == nil {
		t.Fatal("expected error for 401 token response")
	}
	apiErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", apiErr.StatusCode)
	}
}

func TestToken_MalformedTokenResponse_ReturnsError(t *testing.T) {
	mock := newSequencedMock(
		[]*httpclient.HTTPResponse{{StatusCode: http.StatusOK, Body: []byte(`not json`)}},
		[]error{nil},
	)

	svc := &apiRequestService{
		httpClient:   mock,
		authMgr:      &authManager{clientID: "id", clientSecret: "secret", logger: testLogger()},
		baseURL:      "https://api.neo4j.io",
		endpointBase: "https://api.neo4j.io/v1",
		logger:       testLogger(),
	}

	_, err := svc.Get(context.Background(), "instances")
	if err == nil {
		t.Fatal("expected error for malformed token response")
	}
	if !strings.Contains(err.Error(), "failed to parse token response") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestToken_OAuthBodyFormat(t *testing.T) {
	tokenBody := tokenResponseBody("tok", "Bearer", 3600)
	apiBody := []byte(`{"data":[]}`)

	mock := newSequencedMock(
		[]*httpclient.HTTPResponse{
			{StatusCode: http.StatusOK, Body: tokenBody},
			{StatusCode: http.StatusOK, Body: apiBody},
		},
		[]error{nil, nil},
	)

	svc := &apiRequestService{
		httpClient:   mock,
		authMgr:      &authManager{clientID: "id", clientSecret: "secret", logger: testLogger()},
		baseURL:      "https://api.neo4j.io",
		endpointBase: "https://api.neo4j.io/v1",
		logger:       testLogger(),
	}

	_, err := svc.Get(context.Background(), "instances")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tokenCall := mock.calls[0]
	if tokenCall.headers["Content-Type"] != "application/x-www-form-urlencoded" {
		t.Errorf("expected Content-Type 'application/x-www-form-urlencoded', got '%s'", tokenCall.headers["Content-Type"])
	}
	if !strings.Contains(tokenCall.body, "grant_type=client_credentials") {
		t.Errorf("expected grant_type=client_credentials in token body, got '%s'", tokenCall.body)
	}
}

// ============================================================================
// Concurrent token refresh — double-checked locking
// ============================================================================

func TestToken_ConcurrentRefresh_OnlyOneFetch(t *testing.T) {
	const goroutines = 20

	tokenBody := tokenResponseBody("concurrent-token", "Bearer", 3600)
	apiBody := []byte(`{"data":[]}`)

	var responses []*httpclient.HTTPResponse
	var errs []error
	for range goroutines {
		responses = append(responses, &httpclient.HTTPResponse{StatusCode: http.StatusOK, Body: tokenBody})
		errs = append(errs, nil)
	}
	for range goroutines {
		responses = append(responses, &httpclient.HTTPResponse{StatusCode: http.StatusOK, Body: apiBody})
		errs = append(errs, nil)
	}

	mock := newSequencedMock(responses, errs)

	svc := &apiRequestService{
		httpClient:   mock,
		authMgr:      &authManager{clientID: "id", clientSecret: "secret", logger: testLogger()},
		baseURL:      "https://api.neo4j.io",
		endpointBase: "https://api.neo4j.io/v1",
		logger:       testLogger(),
	}

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			svc.Get(context.Background(), "instances") //nolint:errcheck,gosec
		}()
	}
	wg.Wait()

	tokenCallCount := 0
	mock.mu.Lock()
	for _, c := range mock.calls {
		if strings.HasSuffix(c.url, "/oauth/token") {
			tokenCallCount++
		}
	}
	mock.mu.Unlock()

	if tokenCallCount != 1 {
		t.Errorf("expected exactly 1 token fetch under concurrent load, got %d", tokenCallCount)
	}
}

// ============================================================================
// Error type helper methods
// ============================================================================

func TestError_IsNotFound(t *testing.T) {
	tests := []struct {
		code int
		want bool
	}{
		{http.StatusNotFound, true},
		{http.StatusOK, false},
		{http.StatusUnauthorized, false},
	}
	for _, tt := range tests {
		e := &Error{StatusCode: tt.code}
		if e.IsNotFound() != tt.want {
			t.Errorf("status %d: IsNotFound() = %v, want %v", tt.code, e.IsNotFound(), tt.want)
		}
	}
}

func TestError_IsUnauthorized(t *testing.T) {
	tests := []struct {
		code int
		want bool
	}{
		{http.StatusUnauthorized, true},
		{http.StatusForbidden, false},
		{http.StatusOK, false},
	}
	for _, tt := range tests {
		e := &Error{StatusCode: tt.code}
		if e.IsUnauthorized() != tt.want {
			t.Errorf("status %d: IsUnauthorized() = %v, want %v", tt.code, e.IsUnauthorized(), tt.want)
		}
	}
}

func TestError_IsBadRequest(t *testing.T) {
	tests := []struct {
		code int
		want bool
	}{
		{http.StatusBadRequest, true},
		{http.StatusUnprocessableEntity, false},
		{http.StatusOK, false},
	}
	for _, tt := range tests {
		e := &Error{StatusCode: tt.code}
		if e.IsBadRequest() != tt.want {
			t.Errorf("status %d: IsBadRequest() = %v, want %v", tt.code, e.IsBadRequest(), tt.want)
		}
	}
}

func TestError_Error_NoDetails(t *testing.T) {
	e := &Error{StatusCode: 404, Message: "Not Found"}
	expected := "API error (status 404): Not Found"
	if e.Error() != expected {
		t.Errorf("expected '%s', got '%s'", expected, e.Error())
	}
}

func TestError_Error_SingleDetail(t *testing.T) {
	e := &Error{
		StatusCode: 400,
		Message:    "Bad Request",
		Details:    []ErrorDetail{{Message: "name is required"}},
	}
	expected := "API error (status 400): Bad Request - name is required"
	if e.Error() != expected {
		t.Errorf("expected '%s', got '%s'", expected, e.Error())
	}
}

func TestError_Error_MultipleDetails(t *testing.T) {
	e := &Error{
		StatusCode: 422,
		Message:    "Validation Error",
		Details: []ErrorDetail{
			{Message: "field A"},
			{Message: "field B"},
			{Message: "field C"},
		},
	}
	msg := e.Error()
	if !strings.Contains(msg, "and 2 more error(s)") {
		t.Errorf("expected '2 more error(s)' in message, got '%s'", msg)
	}
}

func TestError_AllErrors(t *testing.T) {
	e := &Error{
		StatusCode: 400,
		Message:    "top-level",
		Details:    []ErrorDetail{{Message: "detail-1"}, {Message: "detail-2"}},
	}
	all := e.AllErrors()
	if len(all) != 3 {
		t.Fatalf("expected 3 errors, got %d", len(all))
	}
	if all[0] != "top-level" {
		t.Errorf("expected first to be top-level message, got '%s'", all[0])
	}
}

func TestError_HasMultipleErrors(t *testing.T) {
	single := &Error{Details: []ErrorDetail{{Message: "one"}}}
	if single.HasMultipleErrors() {
		t.Error("single detail: expected HasMultipleErrors() = false")
	}
	multi := &Error{Details: []ErrorDetail{{Message: "one"}, {Message: "two"}}}
	if !multi.HasMultipleErrors() {
		t.Error("two details: expected HasMultipleErrors() = true")
	}
}
