// package query_test provides black-box integration tests for the aura package.
//
// These tests exercise the package's public API exclusively — no internal types,
// unexported symbols, or mock infrastructure from the main package are used.
// A local httptest.Server replaces the real Aura API, giving deterministic,
// network-free coverage of the full request/response path.
//
// The test server always handles /oauth/token so NewClient can authenticate;
// individual test cases register their own handlers for API paths.
package query_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/neo4j-contrib/aura-go-sdk/v2"
)

// ─── Test server helpers ─────────────────────────────────────────────────────

// fakeTokenPayload is returned by the test OAuth endpoint.
var fakeTokenPayload = map[string]any{
	"token_type":   "Bearer",
	"access_token": "test-token-for-blackbox-tests",
	"expires_in":   int64(3600),
}

// newTestServer starts an httptest.Server whose API handler is supplied by the
// caller. A /oauth/token handler is pre-registered so the Aura client can
// obtain a token without any additional setup.
//
// All test servers are closed automatically via t.Cleanup.
func newTestServer(t *testing.T, apiHandler http.Handler) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	// OAuth token endpoint — always issues a valid token.
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, fakeTokenPayload)
	})

	// Everything else is delegated to the caller's handler.
	mux.Handle("/", apiHandler)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// newClient creates an AuraAPIClient pointing at srv. MaxRetry is set to 1 to
// keep tests fast — we are not testing retry logic here.
// WithInsecureBaseURL is used because httptest.Server issues http:// URLs;
// the HTTPS enforcement in WithBaseURL is intentionally bypassed for tests.
func newClient(t *testing.T, srv *httptest.Server) *aura.AuraAPIClient {
	t.Helper()
	client, err := aura.NewClient(
		aura.WithCredentials("test-client-id", "test-client-secret"),
		aura.WithInsecureBaseURL(srv.URL),
		aura.WithTimeout(5*time.Second),
		aura.WithMaxRetry(1),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

// writeJSON sets the Content-Type header, writes status, and encodes v as JSON.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ─── Client construction ──────────────────────────────────────────────────────

func TestNewClient_ValidCredentials(t *testing.T) {
	client, err := aura.NewClient(aura.WithCredentials("my-id", "my-secret"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewClient_MissingClientID(t *testing.T) {
	_, err := aura.NewClient(aura.WithCredentials("", "secret"))
	if err == nil {
		t.Fatal("expected error for empty client ID")
	}
}

func TestNewClient_MissingClientSecret(t *testing.T) {
	_, err := aura.NewClient(aura.WithCredentials("id", ""))
	if err == nil {
		t.Fatal("expected error for empty client secret")
	}
}

func TestNewClient_BothCredentialsMissing(t *testing.T) {
	_, err := aura.NewClient(aura.WithCredentials("", ""))
	if err == nil {
		t.Fatal("expected error when both credentials are empty")
	}
}

func TestNewClient_ZeroTimeout(t *testing.T) {
	_, err := aura.NewClient(
		aura.WithCredentials("id", "secret"),
		aura.WithTimeout(0),
	)
	if err == nil {
		t.Fatal("expected error for zero timeout")
	}
}

func TestNewClient_NegativeTimeout(t *testing.T) {
	_, err := aura.NewClient(
		aura.WithCredentials("id", "secret"),
		aura.WithTimeout(-1*time.Second),
	)
	if err == nil {
		t.Fatal("expected error for negative timeout")
	}
}

func TestNewClient_EmptyBaseURL(t *testing.T) {
	_, err := aura.NewClient(
		aura.WithCredentials("id", "secret"),
		aura.WithBaseURL(""),
	)
	if err == nil {
		t.Fatal("expected error for empty base URL")
	}
}

func TestNewClient_ZeroMaxRetry(t *testing.T) {
	_, err := aura.NewClient(
		aura.WithCredentials("id", "secret"),
		aura.WithMaxRetry(0),
	)
	if err == nil {
		t.Fatal("expected error for zero max retry")
	}
}

func TestNewClient_NilLogger(t *testing.T) {
	_, err := aura.NewClient(
		aura.WithCredentials("id", "secret"),
		aura.WithLogger(nil),
	)
	if err == nil {
		t.Fatal("expected error for nil logger")
	}
}

func TestNewClient_CustomBaseURL(t *testing.T) {
	client, err := aura.NewClient(
		aura.WithCredentials("id", "secret"),
		aura.WithBaseURL("https://api.staging.neo4j.io"),
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewClient_MultipleOptions(t *testing.T) {
	client, err := aura.NewClient(
		aura.WithCredentials("id", "secret"),
		aura.WithTimeout(90*time.Second),
		aura.WithMaxRetry(5),
		aura.WithBaseURL("https://api.staging.neo4j.io"),
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

// ─── Exported fields and constants ────────────────────────────────────────────

func TestNewClient_AllServicesExposed(t *testing.T) {
	client, err := aura.NewClient(aura.WithCredentials("id", "secret"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client.Instances == nil {
		t.Error("Instances service must be non-nil")
	}
	if client.Tenants == nil {
		t.Error("Tenants service must be non-nil")
	}
	if client.Snapshots == nil {
		t.Error("Snapshots service must be non-nil")
	}
	if client.CMEK == nil {
		t.Error("CMEK service must be non-nil")
	}
	if client.GraphAnalytics == nil {
		t.Error("GraphAnalytics service must be non-nil")
	}
	if client.Prometheus == nil {
		t.Error("Prometheus service must be non-nil")
	}
}

func TestStatusConstants_AreAccessible(t *testing.T) {
	constants := map[string]aura.InstanceStatus{
		"StatusRunning":   aura.StatusRunning,
		"StatusStopped":   aura.StatusStopped,
		"StatusPaused":    aura.StatusPaused,
		"StatusAvailable": aura.StatusAvailable,
	}
	for name, val := range constants {
		if val == "" {
			t.Errorf("exported constant %s must not be empty", name)
		}
	}
}

func TestAuraAPIClientVersion_IsAccessible(t *testing.T) {
	if aura.ClientVersion == "" {
		t.Error("ClientVersion must not be empty")
	}
}

// ─── Error type — usable from an external package ─────────────────────────────

func TestErrorType_ImplementsError(t *testing.T) {
	var err error = &aura.Error{StatusCode: 500, Message: "server error"}
	if err.Error() == "" {
		t.Error("Error() must return a non-empty string")
	}
}

func TestErrorType_TypeAssertable(t *testing.T) {
	var err error = &aura.Error{StatusCode: 404, Message: "not found"}
	apiErr, ok := err.(*aura.Error)
	if !ok {
		t.Fatal("type assertion to *aura.Error must succeed")
	}
	if apiErr.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", apiErr.StatusCode)
	}
}

func TestErrorType_ErrorsAs(t *testing.T) {
	wrapped := &aura.Error{StatusCode: 404, Message: "not found"}
	var target *aura.Error
	if !errors.As(wrapped, &target) {
		t.Fatal("errors.As must match *aura.Error")
	}
}

func TestErrorType_IsNotFound(t *testing.T) {
	apiErr := &aura.Error{StatusCode: 404, Message: "not found"}
	if !apiErr.IsNotFound() {
		t.Error("expected IsNotFound() = true for 404")
	}
	other := &aura.Error{StatusCode: 200, Message: "ok"}
	if other.IsNotFound() {
		t.Error("expected IsNotFound() = false for 200")
	}
}

func TestErrorType_IsUnauthorized(t *testing.T) {
	apiErr := &aura.Error{StatusCode: 401, Message: "unauthorized"}
	if !apiErr.IsUnauthorized() {
		t.Error("expected IsUnauthorized() = true for 401")
	}
}

func TestErrorType_IsBadRequest(t *testing.T) {
	apiErr := &aura.Error{StatusCode: 400, Message: "bad request"}
	if !apiErr.IsBadRequest() {
		t.Error("expected IsBadRequest() = true for 400")
	}
}

func TestErrorType_ErrorMessage_ContainsStatusAndMessage(t *testing.T) {
	apiErr := &aura.Error{StatusCode: 404, Message: "Instance not found"}
	msg := apiErr.Error()
	if !strings.Contains(msg, "404") {
		t.Errorf("error message should contain status code 404, got: %s", msg)
	}
	if !strings.Contains(msg, "Instance not found") {
		t.Errorf("error message should contain the message text, got: %s", msg)
	}
}

func TestErrorType_AllErrors(t *testing.T) {
	apiErr := &aura.Error{
		StatusCode: 422,
		Message:    "Validation failed",
		Details: []aura.ErrorDetail{
			{Message: "name is required"},
			{Message: "region is invalid"},
		},
	}
	all := apiErr.AllErrors()
	if len(all) != 3 {
		t.Errorf("expected 3 errors (message + 2 details), got %d", len(all))
	}
}

func TestErrorType_HasMultipleErrors(t *testing.T) {
	single := &aura.Error{
		StatusCode: 400,
		Details:    []aura.ErrorDetail{{Message: "one error"}},
	}
	if single.HasMultipleErrors() {
		t.Error("expected HasMultipleErrors() = false for one detail")
	}

	multi := &aura.Error{
		StatusCode: 422,
		Details: []aura.ErrorDetail{
			{Message: "first"},
			{Message: "second"},
		},
	}
	if !multi.HasMultipleErrors() {
		t.Error("expected HasMultipleErrors() = true for two details")
	}
}

// ─── Instances service ────────────────────────────────────────────────────────

func TestInstances_List_Success(t *testing.T) {
	payload := map[string]any{
		"data": []map[string]any{
			{"id": "aaaa0001", "name": "prod-db", "tenant_id": "ad69ff24-12fc-5a34-af02-ff8d3cc23611", "cloud_provider": "gcp", "created_at": "2024-01-01T00:00:00Z"},
			{"id": "bbbb0002", "name": "dev-db", "tenant_id": "ad69ff24-12fc-5a34-af02-ff8d3cc23612", "cloud_provider": "aws", "created_at": "2024-01-02T00:00:00Z"},
		},
	}

	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/instances" && r.Method == http.MethodGet {
			writeJSON(w, http.StatusOK, payload)
			return
		}
		http.NotFound(w, r)
	}))

	result, err := newClient(t, srv).Instances.List(context.Background())
	if err != nil {
		t.Fatalf("Instances.List: %v", err)
	}
	if len(result.Data) != 2 {
		t.Errorf("expected 2 instances, got %d", len(result.Data))
	}
	if result.Data[0].ID != "aaaa0001" {
		t.Errorf("expected first ID 'aaaa0001', got '%s'", result.Data[0].ID)
	}
	if result.Data[1].Name != "dev-db" {
		t.Errorf("expected second name 'dev-db', got '%s'", result.Data[1].Name)
	}
}

func TestInstances_List_Empty(t *testing.T) {
	payload := map[string]any{"data": []any{}}

	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, payload)
	}))

	result, err := newClient(t, srv).Instances.List(context.Background())
	if err != nil {
		t.Fatalf("Instances.List: %v", err)
	}
	if len(result.Data) != 0 {
		t.Errorf("expected 0 instances, got %d", len(result.Data))
	}
}

func TestInstances_List_APIReturns401(t *testing.T) {
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"message": "Unauthorized"})
	}))

	_, err := newClient(t, srv).Instances.List(context.Background())
	if err == nil {
		t.Fatal("expected error for 401 response")
	}

	var apiErr *aura.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *aura.Error, got %T: %v", err, err)
	}
	if !apiErr.IsUnauthorized() {
		t.Errorf("expected IsUnauthorized() = true, got status %d", apiErr.StatusCode)
	}
}

func TestInstances_List_APIReturns404(t *testing.T) {
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]any{"message": "Not Found"})
	}))

	_, err := newClient(t, srv).Instances.List(context.Background())
	if err == nil {
		t.Fatal("expected error for 404 response")
	}

	var apiErr *aura.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *aura.Error, got %T", err)
	}
	if !apiErr.IsNotFound() {
		t.Error("expected IsNotFound() = true")
	}
}

func TestInstances_Get_Success(t *testing.T) {
	instanceID := "abcd1234"
	payload := map[string]any{
		"data": map[string]any{
			"id":             instanceID,
			"name":           "my-instance",
			"status":         aura.StatusRunning,
			"tenant_id":      "tenant-1",
			"cloud_provider": "gcp",
			"connection_url": "neo4j+s://abcd1234.databases.neo4j.io",
			"region":         "us-central1",
			"type":           "enterprise-db",
			"memory":         "8GB",
		},
	}

	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/instances/"+instanceID && r.Method == http.MethodGet {
			writeJSON(w, http.StatusOK, payload)
			return
		}
		http.NotFound(w, r)
	}))

	result, err := newClient(t, srv).Instances.Get(context.Background(), instanceID)
	if err != nil {
		t.Fatalf("Instances.Get: %v", err)
	}
	if result.Data.ID != instanceID {
		t.Errorf("expected ID '%s', got '%s'", instanceID, result.Data.ID)
	}
	if result.Data.Status != aura.StatusRunning {
		t.Errorf("expected status '%s', got '%s'", aura.StatusRunning, result.Data.Status)
	}
	if result.Data.ConnectionURL == "" {
		t.Error("expected non-empty connection URL")
	}
}

func TestInstances_Get_InvalidID_Empty(t *testing.T) {
	// Validation is local — no test server needed.
	client, _ := aura.NewClient(aura.WithCredentials("id", "secret"))
	_, err := client.Instances.Get(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error for empty instance ID")
	}
}

func TestInstances_Get_InvalidID_TooShort(t *testing.T) {
	client, _ := aura.NewClient(aura.WithCredentials("id", "secret"))
	_, err := client.Instances.Get(context.Background(), "abc")
	if err == nil {
		t.Fatal("expected validation error for 3-char instance ID")
	}
}

func TestInstances_Get_InvalidID_IllegalChars(t *testing.T) {
	client, _ := aura.NewClient(aura.WithCredentials("id", "secret"))
	_, err := client.Instances.Get(context.Background(), "!@#$%^&*")
	if err == nil {
		t.Fatal("expected validation error for instance ID with illegal characters")
	}
}

func TestInstances_Get_NotFound(t *testing.T) {
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]any{"message": "Instance not found"})
	}))

	_, err := newClient(t, srv).Instances.Get(context.Background(), "aaaa1234")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}

	var apiErr *aura.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *aura.Error, got %T", err)
	}
	if !apiErr.IsNotFound() {
		t.Error("expected IsNotFound() = true")
	}
}

func TestInstances_Create_Success(t *testing.T) {
	payload := map[string]any{
		"data": map[string]any{
			"id":             "neww0001",
			"name":           "fresh-db",
			"tenant_id":      "ad69ff24-12fc-5a34-af02-ff8d3cc23611",
			"cloud_provider": "gcp",
			"connection_url": "neo4j+s://neww0001.databases.neo4j.io",
			"region":         "us-central1",
			"type":           "enterprise-db",
			"username":       "neo4j",
			"password":       "s3cr3t-p@ss",
		},
	}

	var gotRequest aura.CreateInstanceConfigData
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/instances" && r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&gotRequest)
			writeJSON(w, http.StatusOK, payload)
			return
		}
		http.NotFound(w, r)
	}))

	req := &aura.CreateInstanceConfigData{
		Name:          "fresh-db",
		TenantID:      "ad69ff24-12fc-5a34-af02-ff8d3cc23611",
		CloudProvider: "gcp",
		Region:        "us-central1",
		Type:          "enterprise-db",
		Version:       "5",
		Memory:        "8GB",
	}
	result, err := newClient(t, srv).Instances.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Instances.Create: %v", err)
	}
	if result.Data.ID == "" {
		t.Error("expected non-empty instance ID in response")
	}
	if result.Data.Password == "" {
		t.Error("expected password to be populated in create response")
	}
	if gotRequest.Name != "fresh-db" {
		t.Errorf("expected request body name 'fresh-db', got '%s'", gotRequest.Name)
	}
	if gotRequest.CloudProvider != "gcp" {
		t.Errorf("expected cloud provider 'gcp', got '%s'", gotRequest.CloudProvider)
	}
}

func TestInstances_Create_NilRequest(t *testing.T) {
	client, _ := aura.NewClient(aura.WithCredentials("id", "secret"))
	_, err := client.Instances.Create(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil create request")
	}
}

func TestInstances_Delete_Success(t *testing.T) {
	instanceID := "dddd1234"
	payload := map[string]any{
		"data": map[string]any{"id": instanceID, "status": "destroying"},
	}

	var gotMethod, gotPath string
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		writeJSON(w, http.StatusOK, payload)
	}))

	result, err := newClient(t, srv).Instances.Delete(context.Background(), instanceID)
	if err != nil {
		t.Fatalf("Instances.Delete: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("expected DELETE, got %s", gotMethod)
	}
	if gotPath != "/v1/instances/"+instanceID {
		t.Errorf("expected path '/v1/instances/%s', got '%s'", instanceID, gotPath)
	}
	if result.Data.Status != "destroying" {
		t.Errorf("expected status 'destroying', got '%s'", result.Data.Status)
	}
}

func TestInstances_Delete_InvalidID(t *testing.T) {
	client, _ := aura.NewClient(aura.WithCredentials("id", "secret"))
	_, err := client.Instances.Delete(context.Background(), "bad")
	if err == nil {
		t.Fatal("expected validation error for invalid instance ID")
	}
}

func TestInstances_Pause_Success(t *testing.T) {
	instanceID := "eeee5678"
	payload := map[string]any{
		"data": map[string]any{"id": instanceID, "status": "pausing"},
	}

	var gotPath string
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		writeJSON(w, http.StatusOK, payload)
	}))

	result, err := newClient(t, srv).Instances.Pause(context.Background(), instanceID)
	if err != nil {
		t.Fatalf("Instances.Pause: %v", err)
	}
	if !strings.HasSuffix(gotPath, "/pause") {
		t.Errorf("expected request path to end in /pause, got '%s'", gotPath)
	}
	if result.Data.Status != "pausing" {
		t.Errorf("expected status 'pausing', got '%s'", result.Data.Status)
	}
}

func TestInstances_Resume_Success(t *testing.T) {
	instanceID := "ffff1234"
	payload := map[string]any{
		"data": map[string]any{"id": instanceID, "status": "resuming"},
	}

	var gotPath string
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		writeJSON(w, http.StatusOK, payload)
	}))

	result, err := newClient(t, srv).Instances.Resume(context.Background(), instanceID)
	if err != nil {
		t.Fatalf("Instances.Resume: %v", err)
	}
	if !strings.HasSuffix(gotPath, "/resume") {
		t.Errorf("expected request path to end in /resume, got '%s'", gotPath)
	}
	if result.Data.Status != "resuming" {
		t.Errorf("expected status 'resuming', got '%s'", result.Data.Status)
	}
}

func TestInstances_Update_Success(t *testing.T) {
	instanceID := "aaaa5678"
	payload := map[string]any{
		"data": map[string]any{
			"id":     instanceID,
			"name":   "renamed-db",
			"memory": "16GB",
			"status": "updating",
		},
	}

	var gotMethod string
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		writeJSON(w, http.StatusOK, payload)
	}))

	updateReq := &aura.UpdateInstanceData{Name: "renamed-db", Memory: "16GB"}
	result, err := newClient(t, srv).Instances.Update(context.Background(), instanceID, updateReq)
	if err != nil {
		t.Fatalf("Instances.Update: %v", err)
	}
	if gotMethod != http.MethodPatch {
		t.Errorf("expected PATCH, got %s", gotMethod)
	}
	if result.Data.Name != "renamed-db" {
		t.Errorf("expected name 'renamed-db', got '%s'", result.Data.Name)
	}
	if result.Data.Memory != "16GB" {
		t.Errorf("expected memory '16GB', got '%s'", result.Data.Memory)
	}
}

func TestInstances_Update_NilRequest(t *testing.T) {
	client, _ := aura.NewClient(aura.WithCredentials("id", "secret"))
	_, err := client.Instances.Update(context.Background(), "aaaa1234", nil)
	if err == nil {
		t.Fatal("expected error for nil update request")
	}
}

func TestInstances_Overwrite_WithSourceInstance(t *testing.T) {
	instanceID := "aaaa1234"
	sourceID := "bbbb5678"
	payload := map[string]any{"data": "overwrite-job-xyz"}

	var gotBody map[string]string
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		writeJSON(w, http.StatusOK, payload)
	}))

	result, err := newClient(t, srv).Instances.OverwriteFromInstance(context.Background(), instanceID, sourceID)
	if err != nil {
		t.Fatalf("Instances.Overwrite: %v", err)
	}
	if result.Data == "" {
		t.Error("expected non-empty job ID in overwrite response")
	}
	if gotBody["source_instance_id"] != sourceID {
		t.Errorf("expected request body source_instance_id '%s', got '%s'", sourceID, gotBody["source_instance_id"])
	}
	if gotBody["source_snapshot_id"] != "" {
		t.Errorf("expected empty source_snapshot_id, got '%s'", gotBody["source_snapshot_id"])
	}
}

func TestInstances_Overwrite_WithSourceSnapshot(t *testing.T) {
	instanceID := "cccc1234"
	snapshotID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	payload := map[string]any{"data": "overwrite-job-abc"}

	var gotBody map[string]string
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		writeJSON(w, http.StatusOK, payload)
	}))

	result, err := newClient(t, srv).Instances.OverwriteFromSnapshot(context.Background(), instanceID, snapshotID)
	if err != nil {
		t.Fatalf("Instances.Overwrite with snapshot: %v", err)
	}
	if result.Data == "" {
		t.Error("expected non-empty job ID in overwrite response")
	}
	if gotBody["source_snapshot_id"] != snapshotID {
		t.Errorf("expected source_snapshot_id '%s', got '%s'", snapshotID, gotBody["source_snapshot_id"])
	}
}

func TestInstances_Overwrite_NoSource_Error(t *testing.T) {
	client, _ := aura.NewClient(aura.WithCredentials("id", "secret"))
	_, err := client.Instances.OverwriteFromInstance(context.Background(), "aaaa1234", "")
	if err == nil {
		t.Fatal("expected error when neither source is provided")
	}
}

func TestInstances_Overwrite_InvalidInstanceID(t *testing.T) {
	client, _ := aura.NewClient(aura.WithCredentials("id", "secret"))
	_, err := client.Instances.OverwriteFromInstance(context.Background(), "bad", "bbbb5678")
	if err == nil {
		t.Fatal("expected validation error for invalid instance ID")
	}
}

// ─── Tenants service ──────────────────────────────────────────────────────────

// validTenantID is a UUID that satisfies the tenant ID validation format.
const validTenantID = "12345678-abcd-4321-efef-000000000001"

func TestTenants_List_Success(t *testing.T) {
	payload := map[string]any{
		"data": []map[string]any{
			{"id": "tenant-001", "name": "My Org"},
			{"id": "tenant-002", "name": "Other Org"},
		},
	}

	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/tenants" && r.Method == http.MethodGet {
			writeJSON(w, http.StatusOK, payload)
			return
		}
		http.NotFound(w, r)
	}))

	result, err := newClient(t, srv).Tenants.List(context.Background())
	if err != nil {
		t.Fatalf("Tenants.List: %v", err)
	}
	if len(result.Data) != 2 {
		t.Errorf("expected 2 tenants, got %d", len(result.Data))
	}
	if result.Data[0].ID != "tenant-001" {
		t.Errorf("expected first tenant ID 'tenant-001', got '%s'", result.Data[0].ID)
	}
}

func TestTenants_List_Empty(t *testing.T) {
	payload := map[string]any{"data": []any{}}

	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, payload)
	}))

	result, err := newClient(t, srv).Tenants.List(context.Background())
	if err != nil {
		t.Fatalf("Tenants.List: %v", err)
	}
	if len(result.Data) != 0 {
		t.Errorf("expected 0 tenants, got %d", len(result.Data))
	}
}

func TestTenants_Get_Success(t *testing.T) {
	payload := map[string]any{
		"data": map[string]any{
			"id":   validTenantID,
			"name": "My Org",
			"instance_configurations": []map[string]any{
				{
					"cloud_provider": "gcp",
					"region":         "us-central1",
					"region_name":    "US Central",
					"type":           "enterprise-db",
					"memory":         "8GB",
					"storage":        "64GB",
					"version":        "5",
				},
			},
		},
	}

	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/tenants/") && r.Method == http.MethodGet {
			writeJSON(w, http.StatusOK, payload)
			return
		}
		http.NotFound(w, r)
	}))

	result, err := newClient(t, srv).Tenants.Get(context.Background(), validTenantID)
	if err != nil {
		t.Fatalf("Tenants.Get: %v", err)
	}
	if result.Data.ID != validTenantID {
		t.Errorf("expected tenant ID '%s', got '%s'", validTenantID, result.Data.ID)
	}
	if result.Data.Name != "My Org" {
		t.Errorf("expected name 'My Org', got '%s'", result.Data.Name)
	}
	if len(result.Data.InstanceConfigurations) != 1 {
		t.Errorf("expected 1 instance configuration, got %d", len(result.Data.InstanceConfigurations))
	}
}

func TestTenants_Get_InvalidID_Empty(t *testing.T) {
	client, _ := aura.NewClient(aura.WithCredentials("id", "secret"))
	_, err := client.Tenants.Get(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error for empty tenant ID")
	}
}

func TestTenants_Get_InvalidID_NotUUID(t *testing.T) {
	client, _ := aura.NewClient(aura.WithCredentials("id", "secret"))
	_, err := client.Tenants.Get(context.Background(), "not-a-uuid")
	if err == nil {
		t.Fatal("expected validation error for non-UUID tenant ID")
	}
}

func TestTenants_Get_NotFound(t *testing.T) {
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]any{"message": "Tenant not found"})
	}))

	_, err := newClient(t, srv).Tenants.Get(context.Background(), validTenantID)
	if err == nil {
		t.Fatal("expected error for 404 response")
	}

	var apiErr *aura.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *aura.Error, got %T", err)
	}
	if !apiErr.IsNotFound() {
		t.Error("expected IsNotFound() = true")
	}
}

func TestTenants_GetMetrics_Success(t *testing.T) {
	metricsURL := "https://metrics.neo4j.io/d8e1f2a3"
	payload := map[string]any{
		"data": map[string]any{"endpoint": metricsURL},
	}

	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "metrics-integration") {
			writeJSON(w, http.StatusOK, payload)
			return
		}
		http.NotFound(w, r)
	}))

	result, err := newClient(t, srv).Tenants.GetMetrics(context.Background(), validTenantID)
	if err != nil {
		t.Fatalf("Tenants.GetMetrics: %v", err)
	}
	if result.Data.Endpoint != metricsURL {
		t.Errorf("expected endpoint '%s', got '%s'", metricsURL, result.Data.Endpoint)
	}
}

// ─── Snapshots service ────────────────────────────────────────────────────────

func TestSnapshots_List_NoDateFilter(t *testing.T) {
	instanceID := "aaaa1234"
	payload := map[string]any{
		"data": []map[string]any{
			{
				"instance_id": instanceID,
				"snapshot_id": "snap-001",
				"status":      "Completed",
				"timestamp":   "2024-01-15T10:00:00Z",
				"exportable":  true,
			},
			{
				"instance_id": instanceID,
				"snapshot_id": "snap-002",
				"status":      "Completed",
				"timestamp":   "2024-01-16T10:00:00Z",
				"exportable":  false,
			},
		},
	}

	var gotPath string
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		writeJSON(w, http.StatusOK, payload)
	}))

	result, err := newClient(t, srv).Snapshots.List(context.Background(), instanceID, nil)
	if err != nil {
		t.Fatalf("Snapshots.List: %v", err)
	}
	if len(result.Data) != 2 {
		t.Errorf("expected 2 snapshots, got %d", len(result.Data))
	}
	if result.Data[0].SnapshotID != "snap-001" {
		t.Errorf("expected snapshot ID 'snap-001', got '%s'", result.Data[0].SnapshotID)
	}
	if !strings.Contains(gotPath, instanceID) {
		t.Errorf("expected request path to contain instance ID '%s', got '%s'", instanceID, gotPath)
	}
}

func TestSnapshots_List_WithDateFilter(t *testing.T) {
	instanceID := "bbbb5678"
	payload := map[string]any{
		"data": []map[string]any{
			{
				"instance_id": instanceID,
				"snapshot_id": "snap-003",
				"status":      "Completed",
				"timestamp":   "2024-01-01T08:00:00Z",
				"exportable":  true,
			},
		},
	}

	var gotQuery string
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		writeJSON(w, http.StatusOK, payload)
	}))

	filter := aura.SnapshotDate{Year: 2024, Month: time.January, Day: 01}
	result, err := newClient(t, srv).Snapshots.List(context.Background(), instanceID, &filter)
	if err != nil {
		t.Fatalf("Snapshots.List with date: %v", err)
	}
	if len(result.Data) != 1 {
		t.Errorf("expected 1 snapshot, got %d", len(result.Data))
	}
	if !strings.Contains(gotQuery, "date=2024-01-01") {
		t.Errorf("expected query string to contain date=2024-01-01, got '%s'", gotQuery)
	}
}

func TestSnapshots_Create_Success(t *testing.T) {
	instanceID := "cccc5678"
	payload := map[string]any{
		"data": map[string]any{"snapshot_id": "snap-new-001"},
	}

	var gotMethod, gotPath string
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		writeJSON(w, http.StatusOK, payload)
	}))

	result, err := newClient(t, srv).Snapshots.Create(context.Background(), instanceID)
	if err != nil {
		t.Fatalf("Snapshots.Create: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if !strings.Contains(gotPath, instanceID) {
		t.Errorf("expected request path to contain instance ID '%s', got '%s'", instanceID, gotPath)
	}
	if result.Data.SnapshotID == "" {
		t.Error("expected non-empty snapshot ID in response")
	}
}

func TestSnapshots_Get_Success(t *testing.T) {
	instanceID := "dddd5678"
	snapshotID := "d4e5f6a7-b8c9-0123-defa-123456789013"
	payload := map[string]any{
		"data": map[string]any{
			"instance_id": instanceID,
			"snapshot_id": snapshotID,
			"status":      "Completed",
			"timestamp":   "2024-02-10T09:00:00Z",
			"exportable":  true,
		},
	}

	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, payload)
	}))

	result, err := newClient(t, srv).Snapshots.Get(context.Background(), instanceID, snapshotID)
	if err != nil {
		t.Fatalf("Snapshots.Get: %v", err)
	}
	if result.Data.SnapshotID != snapshotID {
		t.Errorf("expected snapshot ID '%s', got '%s'", snapshotID, result.Data.SnapshotID)
	}
	if !result.Data.Exportable {
		t.Error("expected Exportable = true")
	}
}

// ─── CMEK service ─────────────────────────────────────────────────────────────

func TestCmek_List_NoTenantFilter(t *testing.T) {
	payload := map[string]any{
		"data": []map[string]any{
			{"id": "cmek-001", "name": "my-key", "tenant_id": "tenant-1"},
			{"id": "cmek-002", "name": "other-key", "tenant_id": "tenant-2"},
		},
	}

	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, payload)
	}))

	result, err := newClient(t, srv).CMEK.List(context.Background(), "")
	if err != nil {
		t.Fatalf("CMEK.List: %v", err)
	}
	if len(result.Data) != 2 {
		t.Errorf("expected 2 CMEK entries, got %d", len(result.Data))
	}
}

func TestCmek_List_WithTenantFilter(t *testing.T) {
	payload := map[string]any{
		"data": []map[string]any{
			{"id": "cmek-003", "name": "filtered-key", "tenant_id": validTenantID},
		},
	}

	var gotQuery string
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		writeJSON(w, http.StatusOK, payload)
	}))

	result, err := newClient(t, srv).CMEK.List(context.Background(), validTenantID)
	if err != nil {
		t.Fatalf("CMEK.List with tenant: %v", err)
	}
	if len(result.Data) != 1 {
		t.Errorf("expected 1 CMEK entry, got %d", len(result.Data))
	}
	if !strings.Contains(gotQuery, validTenantID) {
		t.Errorf("expected query string to contain tenant ID, got '%s'", gotQuery)
	}
}

func TestCmek_List_InvalidTenantID(t *testing.T) {
	client, _ := aura.NewClient(aura.WithCredentials("id", "secret"))
	_, err := client.CMEK.List(context.Background(), "not-a-uuid-at-all")
	if err == nil {
		t.Fatal("expected validation error for invalid tenant ID format")
	}
}

// ─── Context cancellation (observable from outside) ──────────────────────────

func TestInstances_List_SlowServer_ContextTimeout(t *testing.T) {
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Simulate a slow API that takes longer than the caller's deadline.
		time.Sleep(300 * time.Millisecond)
		writeJSON(w, http.StatusOK, map[string]any{"data": []any{}})
	}))

	client := newClient(t, srv)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Instances.List(ctx)
	if err == nil {
		t.Fatal("expected error when context deadline is exceeded")
	}
	// The error should be context-related (deadline exceeded or cancelled).
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("expected context error, got: %v", err)
	}
}

func TestInstances_Get_CancelledContext(t *testing.T) {
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{}})
	}))

	client := newClient(t, srv)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		_, err := client.Instances.Get(ctx, "aaaa1234")
		done <- err
	}()

	// Cancel while the request is in-flight.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected error after context cancellation")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("operation did not stop promptly after cancellation")
	}
}

func TestInstances_Create_PreCancelledContext(t *testing.T) {
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{}})
	}))

	client := newClient(t, srv)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling

	req := &aura.CreateInstanceConfigData{
		Name: "test", TenantID: "t1", CloudProvider: "gcp",
		Region: "us-east1", Type: "enterprise-db", Version: "5", Memory: "4GB",
	}
	_, err := client.Instances.Create(ctx, req)
	if err == nil {
		t.Fatal("expected error for pre-cancelled context")
	}
}

// ─── Request routing verification ─────────────────────────────────────────────

// TestInstances_RequestRouting verifies that each method sends the correct HTTP
// verb and path prefix, without asserting on exact path composition already
// covered by unit tests.
func TestInstances_RequestRouting(t *testing.T) {
	instanceID := "a1b2c3d4"

	cases := []struct {
		name       string
		wantMethod string
		wantSuffix string
		call       func(client *aura.AuraAPIClient, srv *httptest.Server) error
		payload    map[string]any
	}{
		{
			name:       "List uses GET /v1/instances",
			wantMethod: http.MethodGet,
			wantSuffix: "/v1/instances",
			payload:    map[string]any{"data": []any{}},
			call: func(c *aura.AuraAPIClient, _ *httptest.Server) error {
				_, err := c.Instances.List(context.Background())
				return err
			},
		},
		{
			name:       "Create uses POST /v1/instances",
			wantMethod: http.MethodPost,
			wantSuffix: "/v1/instances",
			payload: map[string]any{"data": map[string]any{
				"id": "newwwwww", "name": "x", "tenant_id": "ad69ff24-12fc-5a34-af02-ff8d3cc23611",
				"cloud_provider": "gcp", "connection_url": "neo4j+s://x.io",
				"region": "us-east1", "type": "enterprise-db",
				"username": "neo4j", "password": "pw",
			}},
			call: func(c *aura.AuraAPIClient, _ *httptest.Server) error {
				_, err := c.Instances.Create(context.Background(), &aura.CreateInstanceConfigData{
					Name: "x", TenantID: "ad69ff24-12fc-5a34-af02-ff8d3cc23611", CloudProvider: "gcp",
					Region: "us-east1", Type: "enterprise-db", Version: "5", Memory: "4GB",
				})
				return err
			},
		},
		{
			name:       "Delete uses DELETE /v1/instances/{id}",
			wantMethod: http.MethodDelete,
			wantSuffix: "/v1/instances/" + instanceID,
			payload:    map[string]any{"data": map[string]any{"id": instanceID, "status": "destroying"}},
			call: func(c *aura.AuraAPIClient, _ *httptest.Server) error {
				_, err := c.Instances.Delete(context.Background(), instanceID)
				return err
			},
		},
		{
			name:       "Update uses PATCH /v1/instances/{id}",
			wantMethod: http.MethodPatch,
			wantSuffix: "/v1/instances/" + instanceID,
			payload:    map[string]any{"data": map[string]any{"id": instanceID, "name": "n", "memory": "8GB"}},
			call: func(c *aura.AuraAPIClient, _ *httptest.Server) error {
				_, err := c.Instances.Update(context.Background(), instanceID, &aura.UpdateInstanceData{Name: "n"})
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := tc.payload
			var gotMethod, gotPath string

			srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				writeJSON(w, http.StatusOK, payload)
			}))

			client := newClient(t, srv)
			if err := tc.call(client, srv); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotMethod != tc.wantMethod {
				t.Errorf("expected HTTP method %s, got %s", tc.wantMethod, gotMethod)
			}
			if !strings.HasSuffix(gotPath, tc.wantSuffix) {
				t.Errorf("expected path suffix '%s', got '%s'", tc.wantSuffix, gotPath)
			}
		})
	}
}
