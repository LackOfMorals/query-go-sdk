package query

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/neo4j-contrib/aura-go-sdk/v2/internal/api"
)

// testLogger creates a logger for testing that writes warn+ to stderr.
func testLogger() *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelWarn}
	handler := slog.NewTextHandler(os.Stderr, opts)
	return slog.New(handler)
}

// capturingHandler is a slog.Handler that collects records for test assertions.
type capturingHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *capturingHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *capturingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}

func (h *capturingHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return h
}

func (h *capturingHandler) WithGroup(_ string) slog.Handler {
	return h
}

func (h *capturingHandler) hasRecord(level slog.Level, msgSubstr string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, r := range h.records {
		if r.Level == level && strings.Contains(r.Message, msgSubstr) {
			return true
		}
	}
	return false
}

func (h *capturingHandler) hasAttr(key, value string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, r := range h.records {
		found := false
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == key && a.Value.String() == value {
				found = true
				return false
			}
			return true
		})
		if found {
			return true
		}
	}
	return false
}

// ============================================================================
// Mock types
// ============================================================================

// mockAPIService is a basic mock of api.RequestService.
// It records the last call details but does not inspect the context.
type mockAPIService struct {
	response   *api.Response
	err        error
	lastMethod string
	lastPath   string
	lastBody   string
}

// mockAPIServiceWithDelay is a mock that can simulate slow responses and respects
// context cancellation / deadlines. mu guards the recording fields so the mock
// is safe to share across goroutines in concurrent tests.
type mockAPIServiceWithDelay struct {
	mu       sync.Mutex
	response *api.Response
	err      error
	delay    time.Duration

	// Fields below are written on every call; protect with mu.
	lastMethod string
	lastPath   string
	lastBody   string
	callCount  int
}

// mockAPIServiceWithCallback is a mock that accepts optional callback hooks so
// tests can inspect the context or other call parameters at the point the API
// is invoked.
type mockAPIServiceWithCallback struct {
	response   *api.Response
	err        error
	delay      time.Duration
	lastMethod string
	lastPath   string
	lastBody   string
	callCount  int

	OnGet    func(ctx context.Context, endpoint string) error
	OnPost   func(ctx context.Context, endpoint string, body string) error
	OnPut    func(ctx context.Context, endpoint string, body string) error
	OnPatch  func(ctx context.Context, endpoint string, body string) error
	OnDelete func(ctx context.Context, endpoint string) error
}

// ============================================================================
// mockAPIService — simple mock, does not check context
// ============================================================================

func (m *mockAPIService) Get(_ context.Context, endpoint string) (*api.Response, error) {
	m.lastMethod = "GET"
	m.lastPath = endpoint
	return m.response, m.err
}

func (m *mockAPIService) Post(_ context.Context, endpoint string, body string) (*api.Response, error) {
	m.lastMethod = "POST"
	m.lastPath = endpoint
	m.lastBody = body
	return m.response, m.err
}

func (m *mockAPIService) Put(_ context.Context, endpoint string, body string) (*api.Response, error) {
	m.lastMethod = "PUT"
	m.lastPath = endpoint
	m.lastBody = body
	return m.response, m.err
}

func (m *mockAPIService) Patch(_ context.Context, endpoint string, body string) (*api.Response, error) {
	m.lastMethod = "PATCH"
	m.lastPath = endpoint
	m.lastBody = body
	return m.response, m.err
}

func (m *mockAPIService) Delete(_ context.Context, endpoint string) (*api.Response, error) {
	m.lastMethod = "DELETE"
	m.lastPath = endpoint
	return m.response, m.err
}

func (m *mockAPIService) Close() {}

// ============================================================================
// mockAPIServiceWithDelay — respects context cancellation, can simulate slow APIs
// ============================================================================

func (m *mockAPIServiceWithDelay) Get(ctx context.Context, endpoint string) (*api.Response, error) {
	m.mu.Lock()
	m.lastMethod = "GET"
	m.lastPath = endpoint
	m.callCount++
	m.mu.Unlock()
	return m.executeWithDelay(ctx)
}

func (m *mockAPIServiceWithDelay) Post(ctx context.Context, endpoint string, body string) (*api.Response, error) {
	m.mu.Lock()
	m.lastMethod = "POST"
	m.lastPath = endpoint
	m.lastBody = body
	m.callCount++
	m.mu.Unlock()
	return m.executeWithDelay(ctx)
}

func (m *mockAPIServiceWithDelay) Put(ctx context.Context, endpoint string, body string) (*api.Response, error) {
	m.mu.Lock()
	m.lastMethod = "PUT"
	m.lastPath = endpoint
	m.lastBody = body
	m.callCount++
	m.mu.Unlock()
	return m.executeWithDelay(ctx)
}

func (m *mockAPIServiceWithDelay) Patch(ctx context.Context, endpoint string, body string) (*api.Response, error) {
	m.mu.Lock()
	m.lastMethod = "PATCH"
	m.lastPath = endpoint
	m.lastBody = body
	m.callCount++
	m.mu.Unlock()
	return m.executeWithDelay(ctx)
}

func (m *mockAPIServiceWithDelay) Delete(ctx context.Context, endpoint string) (*api.Response, error) {
	m.mu.Lock()
	m.lastMethod = "DELETE"
	m.lastPath = endpoint
	m.callCount++
	m.mu.Unlock()
	return m.executeWithDelay(ctx)
}

func (m *mockAPIServiceWithDelay) executeWithDelay(ctx context.Context) (*api.Response, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return m.response, m.err
}

func (m *mockAPIServiceWithDelay) Close() {}

// ============================================================================
// mockAPIServiceWithCallback — supports hooks to inspect context values and
// verify propagation through service layers
// ============================================================================

func (m *mockAPIServiceWithCallback) Get(ctx context.Context, endpoint string) (*api.Response, error) {
	m.lastMethod = "GET"
	m.lastPath = endpoint
	m.callCount++
	if m.OnGet != nil {
		if err := m.OnGet(ctx, endpoint); err != nil {
			return nil, err
		}
	}
	return m.executeWithDelay(ctx)
}

func (m *mockAPIServiceWithCallback) Post(ctx context.Context, endpoint string, body string) (*api.Response, error) {
	m.lastMethod = "POST"
	m.lastPath = endpoint
	m.lastBody = body
	m.callCount++
	if m.OnPost != nil {
		if err := m.OnPost(ctx, endpoint, body); err != nil {
			return nil, err
		}
	}
	return m.executeWithDelay(ctx)
}

func (m *mockAPIServiceWithCallback) Put(ctx context.Context, endpoint string, body string) (*api.Response, error) {
	m.lastMethod = "PUT"
	m.lastPath = endpoint
	m.lastBody = body
	m.callCount++
	if m.OnPut != nil {
		if err := m.OnPut(ctx, endpoint, body); err != nil {
			return nil, err
		}
	}
	return m.executeWithDelay(ctx)
}

func (m *mockAPIServiceWithCallback) Patch(ctx context.Context, endpoint string, body string) (*api.Response, error) {
	m.lastMethod = "PATCH"
	m.lastPath = endpoint
	m.lastBody = body
	m.callCount++
	if m.OnPatch != nil {
		if err := m.OnPatch(ctx, endpoint, body); err != nil {
			return nil, err
		}
	}
	return m.executeWithDelay(ctx)
}

func (m *mockAPIServiceWithCallback) Delete(ctx context.Context, endpoint string) (*api.Response, error) {
	m.lastMethod = "DELETE"
	m.lastPath = endpoint
	m.callCount++
	if m.OnDelete != nil {
		if err := m.OnDelete(ctx, endpoint); err != nil {
			return nil, err
		}
	}
	return m.executeWithDelay(ctx)
}

func (m *mockAPIServiceWithCallback) executeWithDelay(ctx context.Context) (*api.Response, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return m.response, m.err
}

func (m *mockAPIServiceWithCallback) Close() {}
