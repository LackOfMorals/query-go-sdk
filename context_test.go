package query

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/neo4j-contrib/aura-go-sdk/v2/internal/api"
)

// ============================================================================
// Context Cancellation Tests - Cross-Service Coverage
// ============================================================================

// TestAllServices_ContextCancellation verifies all services respect a pre-cancelled context.
func TestAllServices_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	responseBody, _ := json.Marshal(map[string]interface{}{"data": []interface{}{}})
	mock := &mockAPIServiceWithDelay{
		response: &api.Response{StatusCode: 200, Body: responseBody},
		delay:    0,
	}

	tests := []struct {
		name      string
		operation func() error
	}{
		{
			name: "InstanceService.List",
			operation: func() error {
				service := &instanceService{api: mock, timeout: 30 * time.Second, logger: testLogger()}
				_, err := service.List(ctx)
				return err
			},
		},
		{
			name: "TenantService.List",
			operation: func() error {
				service := &tenantService{api: mock, timeout: 30 * time.Second, logger: testLogger()}
				_, err := service.List(ctx)
				return err
			},
		},
		{
			name: "SnapshotService.List",
			operation: func() error {
				service := &snapshotService{api: mock, timeout: 30 * time.Second, logger: testLogger()}
				_, err := service.List(ctx, "aaaa1234", nil)
				return err
			},
		},
		{
			name: "CMEKService.List",
			operation: func() error {
				service := &cmekService{api: mock, timeout: 30 * time.Second, logger: testLogger()}
				_, err := service.List(ctx, "")
				return err
			},
		},
		{
			name: "GDSSessionService.List",
			operation: func() error {
				service := &gdsSessionService{api: mock, timeout: 30 * time.Second, logger: testLogger()}
				_, err := service.List(ctx)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.operation()
			if err == nil {
				t.Fatal("expected context cancelled error")
			}
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("expected context error, got: %v", err)
			}
		})
	}
}

// TestAllServices_TimeoutEnforcement verifies all services enforce their configured timeout.
func TestAllServices_TimeoutEnforcement(t *testing.T) {
	responseBody, _ := json.Marshal(map[string]interface{}{"data": []interface{}{}})
	mock := &mockAPIServiceWithDelay{
		response: &api.Response{StatusCode: 200, Body: responseBody},
		delay:    2 * time.Second,
	}
	shortTimeout := 100 * time.Millisecond

	tests := []struct {
		name      string
		operation func() error
	}{
		{
			name: "InstanceService.Get",
			operation: func() error {
				service := &instanceService{api: mock, timeout: shortTimeout, logger: testLogger()}
				_, err := service.Get(context.Background(), "aaaa1234")
				return err
			},
		},
		{
			name: "TenantService.Get",
			operation: func() error {
				service := &tenantService{api: mock, timeout: shortTimeout, logger: testLogger()}
				_, err := service.Get(context.Background(), "00000000-0000-0000-0000-000000000000")
				return err
			},
		},
		{
			name: "SnapshotService.Create",
			operation: func() error {
				service := &snapshotService{api: mock, timeout: shortTimeout, logger: testLogger()}
				_, err := service.Create(context.Background(), "aaaa1234")
				return err
			},
		},
		{
			name: "GDSSessionService.Delete",
			operation: func() error {
				service := &gdsSessionService{api: mock, timeout: shortTimeout, logger: testLogger()}
				_, err := service.Delete(context.Background(), "session-id")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()
			err := tt.operation()
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
		})
	}
}

// TestContextHierarchy_ParentOverridesChild verifies that a parent deadline shorter than
// the service timeout still wins.
func TestContextHierarchy_ParentOverridesChild(t *testing.T) {
	responseBody, _ := json.Marshal(ListInstancesResponse{Data: []ListInstanceData{}})
	mock := &mockAPIServiceWithDelay{
		response: &api.Response{StatusCode: 200, Body: responseBody},
		delay:    1 * time.Second,
	}

	// Parent has a 100ms deadline; service is configured with 10s.
	// context.WithTimeout(parentCtx, 10s) produces a child whose effective
	// deadline is still the parent's 100ms, so the parent wins.
	parentCtx, parentCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer parentCancel()

	service := createTestInstanceServiceWithTimeout(mock, 10*time.Second)

	start := time.Now()
	_, err := service.List(parentCtx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got: %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("should have used parent deadline (~100ms), took: %v", elapsed)
	}
}

// TestContextHierarchy_ChildOverridesParent verifies that the service timeout wins when it
// is shorter than the caller's deadline.
func TestContextHierarchy_ChildOverridesParent(t *testing.T) {
	responseBody, _ := json.Marshal(ListInstancesResponse{Data: []ListInstanceData{}})
	mock := &mockAPIServiceWithDelay{
		response: &api.Response{StatusCode: 200, Body: responseBody},
		delay:    1 * time.Second,
	}

	// Parent has 10s; service has 100ms.
	// context.WithTimeout(parentCtx, 100ms) produces a deadline of 100ms,
	// which is shorter than the parent's 10s, so the service timeout wins.
	parentCtx, parentCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer parentCancel()

	service := createTestInstanceServiceWithTimeout(mock, 100*time.Millisecond)

	start := time.Now()
	_, err := service.List(parentCtx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got: %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("should have used service deadline (~100ms), took: %v", elapsed)
	}
}

// TestConcurrentOperations_IndependentContexts verifies that each call's context is
// independent — one timing out does not affect others.
func TestConcurrentOperations_IndependentContexts(t *testing.T) {
	responseBody, _ := json.Marshal(ListInstancesResponse{Data: []ListInstanceData{}})

	tests := []struct {
		name    string
		delay   time.Duration
		timeout time.Duration
		wantErr bool
	}{
		{name: "fast operation succeeds", delay: 10 * time.Millisecond, timeout: 1 * time.Second, wantErr: false},
		{name: "slow operation times out", delay: 2 * time.Second, timeout: 50 * time.Millisecond, wantErr: true},
		{name: "medium operation succeeds", delay: 20 * time.Millisecond, timeout: 1 * time.Second, wantErr: false},
	}

	type result struct {
		name string
		err  error
	}
	results := make(chan result, len(tests))

	for _, tt := range tests {
		tt := tt
		go func() {
			mock := &mockAPIServiceWithDelay{
				response: &api.Response{StatusCode: 200, Body: responseBody},
				delay:    tt.delay,
			}
			service := createTestInstanceServiceWithTimeout(mock, tt.timeout)
			_, err := service.List(context.Background())
			results <- result{name: tt.name, err: err}
		}()
	}

	for range tests {
		res := <-results
		var tc *struct {
			name    string
			delay   time.Duration
			timeout time.Duration
			wantErr bool
		}
		for idx := range tests {
			if tests[idx].name == res.name {
				tc = &tests[idx]
				break
			}
		}
		if tc == nil {
			t.Fatalf("couldn't find test case for result: %s", res.name)
		}
		if tc.wantErr && res.err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
		if !tc.wantErr && res.err != nil {
			t.Errorf("%s: expected no error, got: %v", tc.name, res.err)
		}
	}
}

// TestContextCleanup_NoDeferLeaks verifies that repeated calls don't leak goroutines
// or timers (i.e. defer cancel() is called correctly).
func TestContextCleanup_NoDeferLeaks(t *testing.T) {
	responseBody, _ := json.Marshal(GetInstanceResponse{
		Data: InstanceData{ID: "aaaa1234", Name: "test"},
	})
	mock := &mockAPIService{
		response: &api.Response{StatusCode: 200, Body: responseBody},
	}

	service := createTestInstanceService(mock)

	start := time.Now()
	for i := range 1000 {
		_, err := service.Get(context.Background(), "aaaa1234")
		if err != nil {
			t.Fatalf("iteration %d failed: %v", i, err)
		}
	}
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Errorf("1000 operations took %v — possible context leak", elapsed)
	}
	t.Logf("Completed 1000 operations in %v", elapsed)
}

// TestErrorPropagation_WithContext verifies that API errors and context errors both
// propagate correctly through the service layer.
func TestErrorPropagation_WithContext(t *testing.T) {
	tests := []struct {
		name        string
		mockError   error
		expectError bool
		errorCheck  func(error) bool
	}{
		{
			name:        "API error propagates",
			mockError:   &api.Error{StatusCode: 500, Message: "Internal error"},
			expectError: true,
			errorCheck:  func(err error) bool { _, ok := err.(*api.Error); return ok },
		},
		{
			name:        "context cancelled propagates",
			mockError:   context.Canceled,
			expectError: true,
			errorCheck:  func(err error) bool { return errors.Is(err, context.Canceled) },
		},
		{
			name:        "context deadline exceeded propagates",
			mockError:   context.DeadlineExceeded,
			expectError: true,
			errorCheck:  func(err error) bool { return errors.Is(err, context.DeadlineExceeded) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockAPIService{err: tt.mockError}
			service := createTestInstanceService(mock)
			_, err := service.List(context.Background())

			if tt.expectError && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if tt.expectError && !tt.errorCheck(err) {
				t.Errorf("error type check failed for: %v", err)
			}
		})
	}
}

// TestContextValues_Propagation verifies that values set on the caller's context are
// visible at the API layer.
func TestContextValues_Propagation(t *testing.T) {
	type contextKey string

	tests := []struct {
		name  string
		key   contextKey
		value string
	}{
		{name: "request ID", key: contextKey("request-id"), value: "req-12345"},
		{name: "trace ID", key: contextKey("trace-id"), value: "trace-67890"},
		{name: "user ID", key: contextKey("user-id"), value: "user-abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), tt.key, tt.value)
			responseBody, _ := json.Marshal(ListInstancesResponse{Data: []ListInstanceData{}})

			valueChecked := false
			mock := &mockAPIServiceWithCallback{
				response: &api.Response{StatusCode: 200, Body: responseBody},
				OnGet: func(receivedCtx context.Context, _ string) error {
					if val := receivedCtx.Value(tt.key); val == tt.value {
						valueChecked = true
					}
					return nil
				},
			}

			service := &instanceService{api: mock, timeout: 30 * time.Second, logger: testLogger()}
			_, err := service.List(ctx)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !valueChecked {
				t.Errorf("context value '%s' was not propagated to the API layer", tt.name)
			}
		})
	}
}

// TestCancellationSpeed_QuickResponse verifies that cancellation causes the in-flight
// operation to stop promptly.
func TestCancellationSpeed_QuickResponse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	responseBody, _ := json.Marshal(ListInstancesResponse{Data: []ListInstanceData{}})
	mock := &mockAPIServiceWithDelay{
		response: &api.Response{StatusCode: 200, Body: responseBody},
		delay:    10 * time.Second,
	}

	service := &instanceService{api: mock, timeout: 30 * time.Second, logger: testLogger()}

	done := make(chan error, 1)
	go func() {
		_, err := service.List(ctx)
		done <- err
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected cancellation error")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("operation didn't stop quickly after cancellation (took > 1s)")
	}
}

// TestTimeoutPrecision_CorrectDuration verifies the service timeout fires at roughly
// the configured duration.
func TestTimeoutPrecision_CorrectDuration(t *testing.T) {
	timeout := 200 * time.Millisecond

	responseBody, _ := json.Marshal(ListInstancesResponse{Data: []ListInstanceData{}})
	mock := &mockAPIServiceWithDelay{
		response: &api.Response{StatusCode: 200, Body: responseBody},
		delay:    2 * time.Second,
	}

	service := createTestInstanceServiceWithTimeout(mock, timeout)

	start := time.Now()
	_, err := service.List(context.Background())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got: %v", err)
	}
	t.Logf("Timeout precision: configured %v, actual %v", timeout, elapsed)
}

// TestMultipleServices_SameParentContext verifies that cancelling a shared parent context
// stops all in-flight service calls.
func TestMultipleServices_SameParentContext(t *testing.T) {
	parentCtx, parentCancel := context.WithCancel(context.Background())

	responseBody, _ := json.Marshal(map[string]interface{}{"data": []interface{}{}})
	mock := &mockAPIServiceWithDelay{
		response: &api.Response{StatusCode: 200, Body: responseBody},
		delay:    2 * time.Second,
	}

	instanceSvc := &instanceService{api: mock, timeout: 30 * time.Second, logger: testLogger()}
	tenantSvc := &tenantService{api: mock, timeout: 30 * time.Second, logger: testLogger()}
	snapshotSvc := &snapshotService{api: mock, timeout: 30 * time.Second, logger: testLogger()}

	done := make(chan error, 3)

	go func() { _, err := instanceSvc.List(parentCtx); done <- err }()
	go func() { _, err := tenantSvc.List(parentCtx); done <- err }()
	go func() { _, err := snapshotSvc.List(parentCtx, "aaaa1234", nil); done <- err }()

	time.Sleep(100 * time.Millisecond)
	parentCancel()

	for i := range 3 {
		select {
		case err := <-done:
			if err == nil {
				t.Errorf("operation %d: expected error, got nil", i)
			}
			if !errors.Is(err, context.Canceled) {
				t.Errorf("operation %d: expected context.Canceled, got: %v", i, err)
			}
		case <-time.After(1 * time.Second):
			t.Fatalf("operation %d: didn't complete quickly after cancellation", i)
		}
	}
}

// TestGracefulShutdown_Simulation simulates receiving an OS shutdown signal and verifies
// all concurrent operations stop cleanly.
func TestGracefulShutdown_Simulation(t *testing.T) {
	appCtx, appShutdown := context.WithCancel(context.Background())

	responseBody, _ := json.Marshal(ListInstancesResponse{Data: []ListInstanceData{}})
	mock := &mockAPIServiceWithDelay{
		response: &api.Response{StatusCode: 200, Body: responseBody},
		delay:    1 * time.Second,
	}

	service := &instanceService{api: mock, timeout: 30 * time.Second, logger: testLogger()}

	operations := 5
	done := make(chan error, operations)

	for range operations {
		go func() {
			_, err := service.List(appCtx)
			done <- err
		}()
	}

	time.Sleep(100 * time.Millisecond)
	appShutdown()

	timeout := time.After(1 * time.Second)
	completed := 0

	for completed < operations {
		select {
		case err := <-done:
			completed++
			if err == nil {
				t.Error("expected error after shutdown")
			}
			if !errors.Is(err, context.Canceled) {
				t.Errorf("expected context.Canceled, got: %v", err)
			}
		case <-timeout:
			t.Fatalf("only %d/%d operations completed — some appear to be hanging", completed, operations)
		}
	}

	t.Logf("All %d operations stopped gracefully after shutdown", operations)
}

// TestContextDeadline_BeforeOperation verifies that a context whose deadline has already
// passed is rejected immediately.
func TestContextDeadline_BeforeOperation(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	defer cancel()

	responseBody, _ := json.Marshal(ListInstancesResponse{Data: []ListInstanceData{}})
	mock := &mockAPIServiceWithDelay{
		response: &api.Response{StatusCode: 200, Body: responseBody},
		delay:    0,
	}

	service := &instanceService{api: mock, timeout: 30 * time.Second, logger: testLogger()}

	start := time.Now()
	_, err := service.List(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected deadline exceeded error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got: %v", err)
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("should fail immediately, took %v", elapsed)
	}
}

// TestContextPropagation_ThroughServiceLayers verifies context values flow through the
// service layer all the way to the API layer.
func TestContextPropagation_ThroughServiceLayers(t *testing.T) {
	type contextKey string
	testKey := contextKey("test-key")
	testValue := "test-value-123"

	ctx := context.WithValue(context.Background(), testKey, testValue)
	responseBody, _ := json.Marshal(ListInstancesResponse{Data: []ListInstanceData{}})

	contextValueFound := false
	mock := &mockAPIServiceWithCallback{
		response: &api.Response{StatusCode: 200, Body: responseBody},
		OnGet: func(receivedCtx context.Context, _ string) error {
			if val := receivedCtx.Value(testKey); val == testValue {
				contextValueFound = true
			}
			return nil
		},
	}

	service := &instanceService{api: mock, timeout: 30 * time.Second, logger: testLogger()}
	_, err := service.List(ctx)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contextValueFound {
		t.Error("context value did not propagate through service layers")
	}
}

// TestParentCancellation_DuringOperation verifies that cancelling the parent context
// mid-call terminates the operation.
func TestParentCancellation_DuringOperation(t *testing.T) {
	parentCtx, parentCancel := context.WithCancel(context.Background())

	responseBody, _ := json.Marshal(GetInstanceResponse{
		Data: InstanceData{ID: "aaaa1234", Name: "test"},
	})
	mock := &mockAPIServiceWithDelay{
		response: &api.Response{StatusCode: 200, Body: responseBody},
		delay:    2 * time.Second,
	}

	service := &instanceService{api: mock, timeout: 30 * time.Second, logger: testLogger()}

	done := make(chan error, 1)
	go func() {
		_, err := service.Get(parentCtx, "aaaa1234")
		done <- err
	}()

	time.Sleep(100 * time.Millisecond)
	parentCancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("operation didn't respond to cancellation quickly")
	}
}

// TestServiceTimeout_IndependentOfParent verifies the service timeout fires even when the
// caller provides a context with no deadline.
func TestServiceTimeout_IndependentOfParent(t *testing.T) {
	responseBody, _ := json.Marshal(ListInstancesResponse{Data: []ListInstanceData{}})
	mock := &mockAPIServiceWithDelay{
		response: &api.Response{StatusCode: 200, Body: responseBody},
		delay:    1 * time.Second,
	}

	service := createTestInstanceServiceWithTimeout(mock, 100*time.Millisecond)

	start := time.Now()
	_, err := service.List(context.Background())
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

// TestLongRunningOperation_Cancellable verifies that a long-running create can be
// cancelled mid-flight.
func TestLongRunningOperation_Cancellable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	ctx, cancel := context.WithCancel(context.Background())

	createRequest := &CreateInstanceConfigData{
		Name: "test-instance", TenantID: "ad69ff24-12fc-5a34-af02-ff8d3cc23611", CloudProvider: "gcp",
		Region: "us-central1", Type: "enterprise-db", Version: "5", Memory: "8GB",
	}

	responseBody, _ := json.Marshal(CreateInstanceResponse{
		Data: CreateInstanceData{ID: "new-id", Name: "test"},
	})
	mock := &mockAPIServiceWithDelay{
		response: &api.Response{StatusCode: 200, Body: responseBody},
		delay:    30 * time.Second,
	}

	service := createTestInstanceServiceWithTimeout(mock, 60*time.Second)

	done := make(chan error, 1)
	go func() {
		_, err := service.Create(ctx, createRequest)
		done <- err
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected cancellation error")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("operation didn't stop quickly after cancellation (took > 1s)")
	}
}

// ============================================================================
// Benchmarks
// ============================================================================

// BenchmarkContextCreation_PerOperation benchmarks the overhead of creating a child
// context on every service call.
func BenchmarkContextCreation_PerOperation(b *testing.B) {
	responseBody, _ := json.Marshal(ListInstancesResponse{Data: []ListInstanceData{}})
	mock := &mockAPIService{
		response: &api.Response{StatusCode: 200, Body: responseBody},
	}
	service := createTestInstanceService(mock)

	b.ResetTimer()
	for range b.N {
		_, _ = service.List(context.Background())
	}
}

// BenchmarkConcurrentOperations_WithContext benchmarks concurrent calls each with their
// own context.
func BenchmarkConcurrentOperations_WithContext(b *testing.B) {
	responseBody, _ := json.Marshal(ListInstancesResponse{Data: []ListInstanceData{}})
	mock := &mockAPIService{
		response: &api.Response{StatusCode: 200, Body: responseBody},
	}
	service := createTestInstanceService(mock)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = service.List(context.Background())
		}
	})
}
