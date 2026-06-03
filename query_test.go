package query

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/neo4j-contrib/aura-go-sdk/v2/internal/api"
)

// createTestInstanceService creates an instanceService with a mock API service for testing
func createTestInstanceService(mock *mockAPIService) *instanceService {
	return &instanceService{
		api:     mock,
		timeout: 30 * time.Second,
		logger:  testLogger(),
	}
}

// createTestInstanceServiceWithTimeout creates an instanceService with a specific timeout.
// Pass the desired context directly to each method call.
func createTestInstanceServiceWithTimeout(mock api.RequestService, timeout time.Duration) *instanceService {
	return &instanceService{
		api:     mock,
		timeout: timeout,
		logger:  testLogger(),
	}
}

// TestInstanceService_List_Success verifies successful instance listing
func TestInstanceService_List_Success(t *testing.T) {
	expectedResponse := ListInstancesResponse{
		Data: []ListInstanceData{
			{ID: "instance-1", Name: "test-instance-1", Created: "2024-01-01T00:00:00Z", TenantID: "tenant-1", CloudProvider: "gcp"},
			{ID: "instance-2", Name: "test-instance-2", Created: "2024-01-02T00:00:00Z", TenantID: "tenant-1", CloudProvider: "aws"},
		},
	}

	responseBody, _ := json.Marshal(expectedResponse)
	mock := &mockAPIService{
		response: &api.Response{StatusCode: 200, Body: responseBody},
	}

	service := createTestInstanceService(mock)
	result, err := service.List(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if mock.lastMethod != "GET" {
		t.Errorf("expected GET method, got %s", mock.lastMethod)
	}
	if mock.lastPath != "instances" {
		t.Errorf("expected path 'instances', got '%s'", mock.lastPath)
	}
	if len(result.Data) != 2 {
		t.Errorf("expected 2 instances, got %d", len(result.Data))
	}
	if result.Data[0].ID != "instance-1" {
		t.Errorf("expected first instance ID 'instance-1', got '%s'", result.Data[0].ID)
	}
}

// TestInstanceService_Get_Success verifies retrieving a specific instance
func TestInstanceService_Get_Success(t *testing.T) {
	instanceID := "aaaa5678"
	expectedResponse := GetInstanceResponse{
		Data: InstanceData{
			ID:            instanceID,
			Name:          "my-instance",
			Status:        "running",
			TenantID:      "tenant-1",
			CloudProvider: "gcp",
			ConnectionURL: "neo4j+s://xxxxx.databases.neo4j.io",
			Region:        "us-east-1",
			Type:          "enterprise-db",
			Memory:        "8GB",
		},
	}

	responseBody, _ := json.Marshal(expectedResponse)
	mock := &mockAPIService{
		response: &api.Response{StatusCode: 200, Body: responseBody},
	}

	service := createTestInstanceService(mock)
	result, err := service.Get(context.Background(), instanceID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if mock.lastPath != "instances/"+instanceID {
		t.Errorf("expected path 'instances/%s', got '%s'", instanceID, mock.lastPath)
	}
	if result.Data.ID != instanceID {
		t.Errorf("expected instance ID '%s', got '%s'", instanceID, result.Data.ID)
	}
	if result.Data.Status != "running" {
		t.Errorf("expected status 'running', got '%s'", result.Data.Status)
	}
}

// TestInstanceService_Get_InvalidID verifies validation of instance ID
func TestInstanceService_Get_InvalidID(t *testing.T) {
	tests := []struct {
		name       string
		instanceID string
	}{
		{"empty", ""},
		{"too short", "abc"},
		{"invalid chars", "!@#$%^&*"},
	}

	mock := &mockAPIService{}
	service := createTestInstanceService(mock)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.Get(context.Background(), tt.instanceID)
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

// TestInstanceService_Get_NotFound verifies 404 handling
func TestInstanceService_Get_NotFound(t *testing.T) {
	instanceID := "aaaaaaaa"
	mock := &mockAPIService{
		err: &api.Error{StatusCode: 404, Message: "Instance not found"},
	}

	service := createTestInstanceService(mock)
	result, err := service.Get(context.Background(), instanceID)

	if err == nil {
		t.Fatal("expected error for non-existent instance")
	}
	if result != nil {
		t.Error("expected result to be nil on error")
	}

	apiErr, ok := err.(*api.Error)
	if !ok {
		t.Fatalf("expected api.Error type, got %T: %v", err, err)
	}
	if !apiErr.IsNotFound() {
		t.Error("expected IsNotFound() to be true")
	}
}

// TestInstanceService_Create_Success verifies instance creation
func TestInstanceService_Create_Success(t *testing.T) {
	createRequest := &CreateInstanceConfigData{
		Name: "new-instance", TenantID: "ad69ff24-12fc-5a34-af02-ff8d3cc23611", CloudProvider: "gcp",
		Region: "us-central1", Type: "enterprise-db", Version: "5", Memory: "8GB",
	}

	expectedResponse := CreateInstanceResponse{
		Data: CreateInstanceData{
			ID: "instance-new", Name: "new-instance", TenantID: "ad69ff24-12fc-5a34-af02-ff8d3cc23611",
			CloudProvider: "gcp", ConnectionURL: "neo4j+s://xxxxx.databases.neo4j.io",
			Region: "us-central1", Type: "enterprise-db", Username: "neo4j", Password: "generated-password",
		},
	}

	responseBody, _ := json.Marshal(expectedResponse)
	mock := &mockAPIService{
		response: &api.Response{StatusCode: 200, Body: responseBody},
	}

	service := createTestInstanceService(mock)
	result, err := service.Create(context.Background(), createRequest)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if mock.lastMethod != "POST" {
		t.Errorf("expected POST method, got %s", mock.lastMethod)
	}
	if mock.lastPath != "instances" {
		t.Errorf("expected path 'instances', got '%s'", mock.lastPath)
	}
	if result.Data.Name != "new-instance" {
		t.Errorf("expected name 'new-instance', got '%s'", result.Data.Name)
	}
	if result.Data.Password == "" {
		t.Error("expected password to be populated")
	}

	var sentRequest CreateInstanceConfigData
	if err := json.Unmarshal([]byte(mock.lastBody), &sentRequest); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if sentRequest.Name != createRequest.Name {
		t.Errorf("expected sent name '%s', got '%s'", createRequest.Name, sentRequest.Name)
	}
}

// TestInstanceService_Delete_Success verifies instance deletion
func TestInstanceService_Delete_Success(t *testing.T) {
	instanceID := "aaaa1234"
	responseBody, _ := json.Marshal(GetInstanceResponse{Data: InstanceData{ID: instanceID, Status: "destroying"}})
	mock := &mockAPIService{
		response: &api.Response{StatusCode: 200, Body: responseBody},
	}

	service := createTestInstanceService(mock)
	result, err := service.Delete(context.Background(), instanceID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if mock.lastMethod != "DELETE" {
		t.Errorf("expected DELETE method, got %s", mock.lastMethod)
	}
	if mock.lastPath != "instances/"+instanceID {
		t.Errorf("expected path 'instances/%s', got '%s'", instanceID, mock.lastPath)
	}
	if result.Data.Status != "destroying" {
		t.Errorf("expected status 'destroying', got '%s'", result.Data.Status)
	}
}

// TestInstanceService_Pause_Success verifies instance pausing
func TestInstanceService_Pause_Success(t *testing.T) {
	instanceID := "bbbb5678"
	responseBody, _ := json.Marshal(GetInstanceResponse{Data: InstanceData{ID: instanceID, Status: "pausing"}})
	mock := &mockAPIService{
		response: &api.Response{StatusCode: 200, Body: responseBody},
	}

	service := createTestInstanceService(mock)
	result, err := service.Pause(context.Background(), instanceID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if mock.lastMethod != "POST" {
		t.Errorf("expected POST method, got %s", mock.lastMethod)
	}
	if mock.lastPath != "instances/"+instanceID+"/pause" {
		t.Errorf("expected path 'instances/%s/pause', got '%s'", instanceID, mock.lastPath)
	}
	if result.Data.Status != "pausing" {
		t.Errorf("expected status 'pausing', got '%s'", result.Data.Status)
	}
}

// TestInstanceService_Resume_Success verifies instance resuming
func TestInstanceService_Resume_Success(t *testing.T) {
	instanceID := "bbbb1234"
	responseBody, _ := json.Marshal(GetInstanceResponse{Data: InstanceData{ID: instanceID, Status: "resuming"}})
	mock := &mockAPIService{
		response: &api.Response{StatusCode: 200, Body: responseBody},
	}

	service := createTestInstanceService(mock)
	result, err := service.Resume(context.Background(), instanceID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if mock.lastPath != "instances/"+instanceID+"/resume" {
		t.Errorf("expected path 'instances/%s/resume', got '%s'", instanceID, mock.lastPath)
	}
	if result.Data.Status != "resuming" {
		t.Errorf("expected status 'resuming', got '%s'", result.Data.Status)
	}
}

// TestInstanceService_Update_Success verifies instance updates
func TestInstanceService_Update_Success(t *testing.T) {
	instanceID := "f1f1b2b2"
	updateRequest := &UpdateInstanceData{Name: "updated-name", Memory: "16GB"}
	responseBody, _ := json.Marshal(GetInstanceResponse{
		Data: InstanceData{ID: instanceID, Name: "updated-name", Memory: "16GB", Status: "updating"},
	})
	mock := &mockAPIService{
		response: &api.Response{StatusCode: 200, Body: responseBody},
	}

	service := createTestInstanceService(mock)
	result, err := service.Update(context.Background(), instanceID, updateRequest)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if mock.lastMethod != "PATCH" {
		t.Errorf("expected PATCH method, got %s", mock.lastMethod)
	}
	if mock.lastPath != "instances/"+instanceID {
		t.Errorf("expected path 'instances/%s', got '%s'", instanceID, mock.lastPath)
	}
	if result.Data.Name != "updated-name" {
		t.Errorf("expected name 'updated-name', got '%s'", result.Data.Name)
	}
	if result.Data.Memory != "16GB" {
		t.Errorf("expected memory '16GB', got '%s'", result.Data.Memory)
	}
}

// TestInstanceService_Overwrite_Success verifies overwrite with source instance
func TestInstanceService_Overwrite_Success(t *testing.T) {
	instanceID := "c1c1c2c2"
	sourceInstanceID := "f1f1f2f2"
	responseBody, _ := json.Marshal(OverwriteInstanceResponse{Data: "overwrite-job-123"})
	mock := &mockAPIService{
		response: &api.Response{StatusCode: 200, Body: responseBody},
	}

	service := createTestInstanceService(mock)
	result, err := service.OverwriteFromInstance(context.Background(), instanceID, sourceInstanceID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if mock.lastMethod != "POST" {
		t.Errorf("expected POST method, got %s", mock.lastMethod)
	}
	if mock.lastPath != "instances/"+instanceID+"/overwrite" {
		t.Errorf("expected path 'instances/%s/overwrite', got '%s'", instanceID, mock.lastPath)
	}
	if result.Data == "" {
		t.Error("expected job ID to be populated")
	}

	var sentRequest overwriteInstanceRequest
	if err := json.Unmarshal([]byte(mock.lastBody), &sentRequest); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if sentRequest.SourceInstanceID != sourceInstanceID {
		t.Errorf("expected source instance '%s', got '%s'", sourceInstanceID, sentRequest.SourceInstanceID)
	}
}

// TestInstanceService_Overwrite_WithSnapshot verifies overwrite with snapshot
func TestInstanceService_Overwrite_WithSnapshot(t *testing.T) {
	instanceID := "aaaa5678"
	snapshotID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	responseBody, _ := json.Marshal(OverwriteInstanceResponse{Data: "overwrite-job-456"})
	mock := &mockAPIService{
		response: &api.Response{StatusCode: 200, Body: responseBody},
	}

	service := createTestInstanceService(mock)
	result, err := service.OverwriteFromSnapshot(context.Background(), instanceID, snapshotID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Data == "" {
		t.Error("expected job ID to be populated")
	}

	var sentRequest overwriteInstanceRequest
	if err := json.Unmarshal([]byte(mock.lastBody), &sentRequest); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if sentRequest.SourceSnapshotID != snapshotID {
		t.Errorf("expected snapshot '%s', got '%s'", snapshotID, sentRequest.SourceSnapshotID)
	}
}

// TestInstanceService_OverwriteFromInstance_Validation verifies OverwriteFromInstance validation.
// The method takes exactly one source — sourceInstanceID — so the only validation
// cases are: empty, valid, and invalid format.
func TestInstanceService_OverwriteFromInstance_Validation(t *testing.T) {
	tests := []struct {
		name             string
		instanceID       string
		sourceInstanceID string
		expectError      bool
		errorContains    string
	}{
		{
			name: "empty source instance ID", instanceID: "aaaa1234",
			sourceInstanceID: "",
			expectError:      true, errorContains: "must provide sourceInstanceID",
		},
		{
			name: "valid source instance ID", instanceID: "aaaa1234",
			sourceInstanceID: "bbbb5678",
			expectError:      false,
		},
		{
			name: "invalid source instance ID format", instanceID: "aaaa1234",
			sourceInstanceID: "invalid",
			expectError:      true, errorContains: "invalid source instance ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responseBody, _ := json.Marshal(OverwriteInstanceResponse{Data: "job-123"})
			mock := &mockAPIService{
				response: &api.Response{StatusCode: 200, Body: responseBody},
			}
			service := createTestInstanceService(mock)

			_, err := service.OverwriteFromInstance(context.Background(), tt.instanceID, tt.sourceInstanceID)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("error should contain '%s', got '%s'", tt.errorContains, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestInstanceService_OverwriteFromSnapshot_Validation verifies OverwriteFromSnapshot validation.
// The method validates that sourceSnapshotID is non-empty and a valid UUID.
func TestInstanceService_OverwriteFromSnapshot_Validation(t *testing.T) {
	tests := []struct {
		name             string
		instanceID       string
		sourceSnapshotID string
		expectError      bool
		errorContains    string
	}{
		{
			name: "empty source snapshot ID", instanceID: "aaaa1234",
			sourceSnapshotID: "",
			expectError:      true, errorContains: "must provide sourceSnapshotID",
		},
		{
			name: "malformed source snapshot ID", instanceID: "aaaa1234",
			sourceSnapshotID: "snapshot-123",
			expectError:      true, errorContains: "invalid source snapshot ID",
		},
		{
			name: "valid source snapshot ID", instanceID: "aaaa1234",
			sourceSnapshotID: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			expectError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responseBody, _ := json.Marshal(OverwriteInstanceResponse{Data: "job-123"})
			mock := &mockAPIService{
				response: &api.Response{StatusCode: 200, Body: responseBody},
			}
			service := createTestInstanceService(mock)

			_, err := service.OverwriteFromSnapshot(context.Background(), tt.instanceID, tt.sourceSnapshotID)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("error should contain '%s', got '%s'", tt.errorContains, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestInstanceService_List_EmptyResult verifies empty list handling
func TestInstanceService_List_EmptyResult(t *testing.T) {
	responseBody, _ := json.Marshal(ListInstancesResponse{Data: []ListInstanceData{}})
	mock := &mockAPIService{
		response: &api.Response{StatusCode: 200, Body: responseBody},
	}

	service := createTestInstanceService(mock)
	result, err := service.List(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Data) != 0 {
		t.Errorf("expected 0 instances, got %d", len(result.Data))
	}
}

// TestInstanceService_AuthenticationError verifies auth error handling
func TestInstanceService_AuthenticationError(t *testing.T) {
	mock := &mockAPIService{
		err: &api.Error{StatusCode: 401, Message: "Invalid credentials"},
	}

	service := createTestInstanceService(mock)
	_, err := service.List(context.Background())

	if err == nil {
		t.Fatal("expected authentication error")
	}

	apiErr, ok := err.(*api.Error)
	if !ok {
		t.Fatal("expected api.Error type")
	}
	if !apiErr.IsUnauthorized() {
		t.Error("expected IsUnauthorized() to be true")
	}
}

// ============================================================================
// Context Cancellation Tests
// ============================================================================

// TestInstanceService_Get_ContextTimeout verifies timeout enforcement
func TestInstanceService_Get_ContextTimeout(t *testing.T) {
	instanceID := "aaaa5678"
	responseBody, _ := json.Marshal(GetInstanceResponse{
		Data: InstanceData{ID: instanceID, Name: "test"},
	})
	mock := &mockAPIServiceWithDelay{
		response: &api.Response{StatusCode: 200, Body: responseBody},
		delay:    2 * time.Second,
	}

	// Service timeout shorter than mock delay
	service := createTestInstanceServiceWithTimeout(mock, 100*time.Millisecond)

	start := time.Now()
	_, err := service.Get(context.Background(), instanceID)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got: %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("timeout took too long: %v (expected ~100ms)", elapsed)
	}
}

// TestInstanceService_Update_TimeoutRespected verifies service timeout acts as ceiling
func TestInstanceService_Update_TimeoutRespected(t *testing.T) {
	instanceID := "aaaa1234"
	updateRequest := &UpdateInstanceData{Name: "new-name", Memory: "16GB"}

	responseBody, _ := json.Marshal(GetInstanceResponse{
		Data: InstanceData{ID: instanceID, Name: "new-name"},
	})
	mock := &mockAPIServiceWithDelay{
		response: &api.Response{StatusCode: 200, Body: responseBody},
		delay:    500 * time.Millisecond,
	}

	// Parent has longer timeout; service has shorter one
	parentCtx, parentCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer parentCancel()

	service := createTestInstanceServiceWithTimeout(mock, 200*time.Millisecond)

	start := time.Now()
	_, err := service.Update(parentCtx, instanceID, updateRequest)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got: %v", err)
	}
	if elapsed > 400*time.Millisecond {
		t.Errorf("timeout took too long: %v (expected ~200ms)", elapsed)
	}
}

// TestInstanceService_Pause_ContextNotLeaked verifies defer cancel() prevents leaks
func TestInstanceService_Pause_ContextNotLeaked(t *testing.T) {
	instanceID := "bbbb5678"
	responseBody, _ := json.Marshal(GetInstanceResponse{
		Data: InstanceData{ID: instanceID, Status: "pausing"},
	})
	mock := &mockAPIService{
		response: &api.Response{StatusCode: 200, Body: responseBody},
	}

	service := createTestInstanceService(mock)

	for i := range 100 {
		_, err := service.Pause(context.Background(), instanceID)
		if err != nil {
			t.Fatalf("iteration %d failed: %v", i, err)
		}
	}
}

// TestInstanceService_Resume_QuickCancellation verifies cancellation before API call
func TestInstanceService_Resume_QuickCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(10 * time.Millisecond) // Let deadline expire

	instanceID := "bbbb1234"
	responseBody, _ := json.Marshal(GetInstanceResponse{
		Data: InstanceData{ID: instanceID, Status: "resuming"},
	})
	mock := &mockAPIServiceWithDelay{
		response: &api.Response{StatusCode: 200, Body: responseBody},
		delay:    0,
	}

	service := createTestInstanceServiceWithTimeout(mock, 30*time.Second)
	_, err := service.Resume(ctx, instanceID)

	if err == nil {
		t.Fatal("expected deadline exceeded error")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("expected context error, got: %v", err)
	}
}

// TestInstanceService_List_ContextPropagation verifies context values reach the API layer
func TestInstanceService_List_ContextPropagation(t *testing.T) {
	type contextKey string
	ctx := context.WithValue(context.Background(), contextKey("request-id"), "test-123")

	responseBody, _ := json.Marshal(ListInstancesResponse{Data: []ListInstanceData{}})

	contextChecked := false
	mock := &mockAPIServiceContextCheck{
		response: &api.Response{StatusCode: 200, Body: responseBody},
		onGet: func(receivedCtx context.Context) {
			if val := receivedCtx.Value(contextKey("request-id")); val == "test-123" {
				contextChecked = true
			}
		},
	}

	service := &instanceService{api: mock, timeout: 30 * time.Second, logger: testLogger()}
	_, err := service.List(ctx)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contextChecked {
		t.Error("context value was not propagated to API call")
	}
}

// TestInstanceService_Create_MultipleTimeouts verifies shorter deadline always wins
func TestInstanceService_Create_MultipleTimeouts(t *testing.T) {
	createRequest := &CreateInstanceConfigData{

		Name: "test-instance", TenantID: "ad69ff24-12fc-5a34-af02-ff8d3cc23611", CloudProvider: "gcp",
		Region: "us-central1", Type: "enterprise-db", Version: "5", Memory: "8GB",
	}

	responseBody, _ := json.Marshal(CreateInstanceResponse{
		Data: CreateInstanceData{ID: "new-id", Name: "test"},
	})
	mock := &mockAPIServiceWithDelay{
		response: &api.Response{StatusCode: 200, Body: responseBody},
		delay:    500 * time.Millisecond,
	}

	// Parent context: 5 seconds; service timeout: 100ms (shorter)
	parentCtx, parentCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer parentCancel()

	service := createTestInstanceServiceWithTimeout(mock, 100*time.Millisecond)

	start := time.Now()
	_, err := service.Create(parentCtx, createRequest)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got: %v", err)
	}
	if elapsed > 300*time.Millisecond {
		t.Errorf("timeout took too long: %v (expected ~100ms, not 5s)", elapsed)
	}
}

// TestInstanceService_Overwrite_CancellationDuringValidation verifies early cancellation
func TestInstanceService_Overwrite_CancellationDuringValidation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	mock := &mockAPIServiceWithDelay{
		response: &api.Response{StatusCode: 200, Body: []byte(`{"data":"job-123"}`)},
		delay:    0,
	}

	service := createTestInstanceServiceWithTimeout(mock, 30*time.Second)
	_, err := service.OverwriteFromInstance(ctx, "aaaa1234", "bbbb5678")

	if err == nil {
		t.Fatal("expected context error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

// ============================================================================
// Additional Test Mocks for Context Testing
// ============================================================================

// mockAPIServiceContextCheck is a mock that can verify context propagation
type mockAPIServiceContextCheck struct {
	response *api.Response
	err      error
	onGet    func(context.Context)
}

func (m *mockAPIServiceContextCheck) Get(ctx context.Context, _ string) (*api.Response, error) {
	if m.onGet != nil {
		m.onGet(ctx)
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return m.response, m.err
}

func (m *mockAPIServiceContextCheck) Post(ctx context.Context, _ string, _ string) (*api.Response, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return m.response, m.err
}

func (m *mockAPIServiceContextCheck) Put(ctx context.Context, _ string, _ string) (*api.Response, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return m.response, m.err
}

func (m *mockAPIServiceContextCheck) Patch(ctx context.Context, _ string, _ string) (*api.Response, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return m.response, m.err
}

func (m *mockAPIServiceContextCheck) Delete(ctx context.Context, _ string) (*api.Response, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return m.response, m.err
}

func (m *mockAPIServiceContextCheck) Close() {}

// ============================================================================
// Extended Create/Update Field Tests
// ============================================================================

// TestInstanceService_Create_WithExtendedFields verifies that optional fields are
// serialised into the request body when set.
func TestInstanceService_Create_WithExtendedFields(t *testing.T) {
	createRequest := &CreateInstanceConfigData{
		Name: "ext-instance", TenantID: "ad69ff24-12fc-5a34-af02-ff8d3cc23611",
		CloudProvider: "gcp", Region: "us-central1", Type: "enterprise-db", Version: "5", Memory: "8GB",
		VectorOptimized:      true,
		GraphAnalyticsPlugin: true,
		CustomerManagedKeyID: "cmk-uuid-1234",
	}

	responseBody, _ := json.Marshal(CreateInstanceResponse{
		Data: CreateInstanceData{ID: "new-id", Name: "ext-instance"},
	})
	mock := &mockAPIService{response: &api.Response{StatusCode: 200, Body: responseBody}}

	service := createTestInstanceService(mock)
	_, err := service.Create(context.Background(), createRequest)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var sent CreateInstanceConfigData
	if err := json.Unmarshal([]byte(mock.lastBody), &sent); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if !sent.VectorOptimized {
		t.Error("expected vector_optimized true")
	}
	if !sent.GraphAnalyticsPlugin {
		t.Error("expected graph_analytics_plugin true")
	}
	if sent.CustomerManagedKeyID != "cmk-uuid-1234" {
		t.Errorf("expected customer_managed_key_id 'cmk-uuid-1234', got '%s'", sent.CustomerManagedKeyID)
	}
}

// TestInstanceService_CreateFromInstance_Success verifies cloning from a source instance.
func TestInstanceService_CreateFromInstance_Success(t *testing.T) {
	baseConfig := &CreateInstanceConfigData{
		Name: "clone-instance", TenantID: "ad69ff24-12fc-5a34-af02-ff8d3cc23611",
		CloudProvider: "gcp", Region: "us-central1", Type: "enterprise-db", Version: "5", Memory: "8GB",
	}

	responseBody, _ := json.Marshal(CreateInstanceResponse{
		Data: CreateInstanceData{ID: "clone-id", Name: "clone-instance"},
	})
	mock := &mockAPIService{response: &api.Response{StatusCode: 200, Body: responseBody}}

	service := createTestInstanceService(mock)
	_, err := service.CreateFromInstance(context.Background(), "bbbb5678", baseConfig)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if mock.lastMethod != "POST" {
		t.Errorf("expected POST, got %s", mock.lastMethod)
	}
	if mock.lastPath != "instances" {
		t.Errorf("expected path 'instances', got '%s'", mock.lastPath)
	}

	var sent createInstanceFromSourceRequest
	if err := json.Unmarshal([]byte(mock.lastBody), &sent); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if sent.SourceInstanceID != "bbbb5678" {
		t.Errorf("expected source_instance_id 'bbbb5678', got '%s'", sent.SourceInstanceID)
	}
	if sent.SourceSnapshotID != "" {
		t.Errorf("expected source_snapshot_id to be empty, got '%s'", sent.SourceSnapshotID)
	}
}

// TestInstanceService_CreateFromSnapshot_Success verifies cloning from a specific snapshot.
func TestInstanceService_CreateFromSnapshot_Success(t *testing.T) {
	instanceID := "bbbb5678"
	snapshotID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	baseConfig := &CreateInstanceConfigData{
		Name: "snap-clone", TenantID: "ad69ff24-12fc-5a34-af02-ff8d3cc23611",
		CloudProvider: "gcp", Region: "us-central1", Type: "enterprise-db", Version: "5", Memory: "8GB",
	}

	responseBody, _ := json.Marshal(CreateInstanceResponse{
		Data: CreateInstanceData{ID: "snap-clone-id", Name: "snap-clone"},
	})
	mock := &mockAPIService{response: &api.Response{StatusCode: 200, Body: responseBody}}

	service := createTestInstanceService(mock)
	_, err := service.CreateFromSnapshot(context.Background(), instanceID, snapshotID, baseConfig)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if mock.lastMethod != "POST" {
		t.Errorf("expected POST, got %s", mock.lastMethod)
	}

	var sent createInstanceFromSourceRequest
	if err := json.Unmarshal([]byte(mock.lastBody), &sent); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if sent.SourceInstanceID != instanceID {
		t.Errorf("expected source_instance_id '%s', got '%s'", instanceID, sent.SourceInstanceID)
	}
	if sent.SourceSnapshotID != snapshotID {
		t.Errorf("expected source_snapshot_id '%s', got '%s'", snapshotID, sent.SourceSnapshotID)
	}
}

// TestInstanceService_CreateFromInstance_Validation verifies input validation.
func TestInstanceService_CreateFromInstance_Validation(t *testing.T) {
	validConfig := &CreateInstanceConfigData{
		Name: "test", TenantID: "ad69ff24-12fc-5a34-af02-ff8d3cc23611",
		CloudProvider: "gcp", Region: "us-central1", Type: "enterprise-db", Version: "5", Memory: "8GB",
	}
	tests := []struct {
		name             string
		sourceInstanceID string
		config           *CreateInstanceConfigData
		errContains      string
	}{
		{"nil config", "bbbb5678", nil, "must not be nil"},
		{"empty source ID", "", validConfig, "must provide sourceInstanceID"},
		{"invalid source ID format", "invalid", validConfig, "invalid source instance ID"},
	}
	mock := &mockAPIService{}
	service := createTestInstanceService(mock)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.CreateFromInstance(context.Background(), tt.sourceInstanceID, tt.config)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("expected error containing '%s', got: %v", tt.errContains, err)
			}
		})
	}
}

// TestInstanceService_CreateFromSnapshot_Validation verifies input validation.
func TestInstanceService_CreateFromSnapshot_Validation(t *testing.T) {
	validConfig := &CreateInstanceConfigData{
		Name: "test", TenantID: "ad69ff24-12fc-5a34-af02-ff8d3cc23611",
		CloudProvider: "gcp", Region: "us-central1", Type: "enterprise-db", Version: "5", Memory: "8GB",
	}
	validInstanceID := "bbbb5678"
	validSnapshotID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	tests := []struct {
		name             string
		sourceInstanceID string
		sourceSnapshotID string
		config           *CreateInstanceConfigData
		errContains      string
	}{
		{"nil config", validInstanceID, validSnapshotID, nil, "must not be nil"},
		{"empty instance ID", "", validSnapshotID, validConfig, "must provide sourceInstanceID"},
		{"invalid instance ID format", "invalid", validSnapshotID, validConfig, "invalid source instance ID"},
		{"empty snapshot ID", validInstanceID, "", validConfig, "must provide sourceSnapshotID"},
		{"invalid snapshot ID format", validInstanceID, "not-a-uuid", validConfig, "invalid source snapshot ID"},
	}
	mock := &mockAPIService{}
	service := createTestInstanceService(mock)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.CreateFromSnapshot(context.Background(), tt.sourceInstanceID, tt.sourceSnapshotID, tt.config)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("expected error containing '%s', got: %v", tt.errContains, err)
			}
		})
	}
}

// TestInstanceService_Update_WithExtendedFields verifies that cdc_enrichment_mode and
// secondaries_count are serialised into the PATCH request body when set.
func TestInstanceService_Update_WithExtendedFields(t *testing.T) {
	instanceID := "f1f1b2b2"
	updateRequest := &UpdateInstanceData{
		CDCEnrichmentMode: "FULL",
		SecondariesCount:  2,
	}

	responseBody, _ := json.Marshal(GetInstanceResponse{
		Data: InstanceData{ID: instanceID, CDCEnrichment: "FULL", Secondaries: 2, Status: "updating"},
	})
	mock := &mockAPIService{response: &api.Response{StatusCode: 200, Body: responseBody}}

	service := createTestInstanceService(mock)
	result, err := service.Update(context.Background(), instanceID, updateRequest)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var sent UpdateInstanceData
	if err := json.Unmarshal([]byte(mock.lastBody), &sent); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if sent.CDCEnrichmentMode != "FULL" {
		t.Errorf("expected cdc_enrichment_mode 'FULL', got '%s'", sent.CDCEnrichmentMode)
	}
	if sent.SecondariesCount != 2 {
		t.Errorf("expected secondaries_count 2, got %v", sent.SecondariesCount)
	}
	if result.Data.CDCEnrichment != "FULL" {
		t.Errorf("expected response CDCEnrichment 'FULL', got '%s'", result.Data.CDCEnrichment)
	}
}
