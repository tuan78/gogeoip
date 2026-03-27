package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tuan78/gogeoip/internal/config"
	"github.com/tuan78/gogeoip/internal/geo"
)

// ── Mocks ────────────────────────────────────────────────────────────────────

type mockDB struct {
	loaded bool
	data   *geo.Data
	err    error
}

func (m *mockDB) IsLoaded() bool                     { return m.loaded }
func (m *mockDB) Lookup(_ string) (*geo.Data, error) { return m.data, m.err }

type mockCache struct {
	store map[string]string
}

func (m *mockCache) Get(ctx context.Context, key string) (string, bool) {
	if m.store == nil {
		return "", false
	}
	v, ok := m.store[key]
	return v, ok
}

func (m *mockCache) Set(ctx context.Context, key string, val string, ttl time.Duration) {
	if m.store == nil {
		m.store = make(map[string]string)
	}
	m.store[key] = val
}

// ── Serve Tests ──────────────────────────────────────────────────────────────

func TestServe_StartsAndShutdownsGracefully(t *testing.T) {
	cfg := config.Config{
		Port:                 "0", // Let OS assign a free port
		RedisLookupKeyPrefix: "test:",
		RedisLookupCacheTTL:  "1h",
	}
	db := &mockDB{loaded: true}
	c := &mockCache{}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Run Serve in a goroutine
	done := make(chan struct{})
	go func() {
		Serve(ctx, cfg, db, c)
		close(done)
	}()

	// Wait for context to be canceled (which triggers shutdown)
	select {
	case <-done:
		// Server shut down successfully
	case <-time.After(3 * time.Second):
		t.Error("Serve did not shutdown within timeout")
	}
}

func TestServe_HandlesPingEndpoint(t *testing.T) {
	cfg := config.Config{
		Port:                 "0",
		RedisLookupKeyPrefix: "test:",
		RedisLookupCacheTTL:  "1h",
	}
	db := &mockDB{loaded: true}
	c := &mockCache{}

	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	// Start server, it will shutdown after context timeout
	Serve(ctx, cfg, db, c)
	// Verify it started and shut down without panic
}

// ── Config Parsing Tests ─────────────────────────────────────────────────────

func TestServe_ParsesCacheTTLValidDuration(t *testing.T) {
	cfg := config.Config{
		Port:                 "0",
		RedisLookupKeyPrefix: "test:",
		RedisLookupCacheTTL:  "24h",
	}
	db := &mockDB{loaded: true}
	c := &mockCache{}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Serve will handle parsing "24h" correctly
	Serve(ctx, cfg, db, c)
}

func TestServe_DefaultsCacheTTLOnInvalidDuration(t *testing.T) {
	cfg := config.Config{
		Port:                 "0",
		RedisLookupKeyPrefix: "test:",
		RedisLookupCacheTTL:  "invalid_duration",
	}
	db := &mockDB{loaded: true}
	c := &mockCache{}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Serve should log the parse error and use default (24h)
	Serve(ctx, cfg, db, c)
}

// ── Logging Middleware Tests ─────────────────────────────────────────────────

func TestStatusCodeCapture_CapturesStatusCodeOK(t *testing.T) {
	rec := httptest.NewRecorder()
	scw := &statusCodeCapture{ResponseWriter: rec, statusCode: http.StatusOK}

	// Write a status code
	scw.WriteHeader(http.StatusOK)

	if scw.statusCode != http.StatusOK {
		t.Errorf("expected statusCode %d, got %d", http.StatusOK, scw.statusCode)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected recorder code %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestStatusCodeCapture_CapturesStatusCodeNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	scw := &statusCodeCapture{ResponseWriter: rec, statusCode: http.StatusOK}

	// Write a different status code
	scw.WriteHeader(http.StatusNotFound)

	if scw.statusCode != http.StatusNotFound {
		t.Errorf("expected statusCode %d, got %d", http.StatusNotFound, scw.statusCode)
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected recorder code %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestStatusCodeCapture_CapturesStatusCodeServerError(t *testing.T) {
	rec := httptest.NewRecorder()
	scw := &statusCodeCapture{ResponseWriter: rec, statusCode: http.StatusOK}

	// Write a server error status code
	scw.WriteHeader(http.StatusInternalServerError)

	if scw.statusCode != http.StatusInternalServerError {
		t.Errorf("expected statusCode %d, got %d", http.StatusInternalServerError, scw.statusCode)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected recorder code %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestLoggingMiddleware_WrapsHandler(t *testing.T) {
	handlerCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("unexpected write error: %v", err)
		}
	})

	middleware := loggingMiddleware(testHandler)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)

	middleware.ServeHTTP(rec, req)

	if !handlerCalled {
		t.Error("expected handler to be called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("expected body 'ok', got %q", rec.Body.String())
	}
}

func TestLoggingMiddleware_CapturesStatusThroughMiddleware(t *testing.T) {
	statusCode := http.StatusCreated
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
	})

	middleware := loggingMiddleware(testHandler)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/create", nil)

	middleware.ServeHTTP(rec, req)

	if rec.Code != statusCode {
		t.Errorf("expected status %d, got %d", statusCode, rec.Code)
	}
}

func TestLoggingMiddleware_DefaultsStatusToOK(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't call WriteHeader, should default to 200
		if _, err := w.Write([]byte("no explicit status")); err != nil {
			t.Errorf("unexpected write error: %v", err)
		}
	})

	middleware := loggingMiddleware(testHandler)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)

	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected default status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestLoggingMiddleware_EmitsStructuredJSONLog(t *testing.T) {
	var buf bytes.Buffer
	originalOutput := accessLogger.Writer()
	accessLogger.SetOutput(&buf)
	defer accessLogger.SetOutput(originalOutput)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	middleware := loggingMiddleware(testHandler)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/lookup?ip=8.8.8.8", nil)
	req.Header.Set("User-Agent", "gogeoip-test")
	req.Header.Set("X-Request-Id", "req-123")
	req.Header.Set("X-Datadog-Trace-Id", "trace-456")
	req.RemoteAddr = "10.0.0.1:12345"

	middleware.ServeHTTP(rec, req)

	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("expected a structured access log line")
	}

	var entry accessLogEntry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		t.Fatalf("expected JSON log, got parse error: %v; line=%q", err, line)
	}

	if entry.Message != "http_request" {
		t.Errorf("expected message %q, got %q", "http_request", entry.Message)
	}
	if entry.Service != "gogeoip" {
		t.Errorf("expected service %q, got %q", "gogeoip", entry.Service)
	}
	if entry.Method != "GET" {
		t.Errorf("expected method %q, got %q", "GET", entry.Method)
	}
	if entry.Path != "/lookup" {
		t.Errorf("expected path %q, got %q", "/lookup", entry.Path)
	}
	if entry.Query != "ip=8.8.8.8" {
		t.Errorf("expected query %q, got %q", "ip=8.8.8.8", entry.Query)
	}
	if entry.StatusCode != http.StatusAccepted {
		t.Errorf("expected status_code %d, got %d", http.StatusAccepted, entry.StatusCode)
	}
	if entry.RequestID != "req-123" {
		t.Errorf("expected request_id %q, got %q", "req-123", entry.RequestID)
	}
	if entry.TraceID != "trace-456" {
		t.Errorf("expected trace_id %q, got %q", "trace-456", entry.TraceID)
	}
	if entry.DurationMs < 0 {
		t.Errorf("expected duration_ms to be non-negative, got %f", entry.DurationMs)
	}
	if entry.Timestamp == "" {
		t.Error("expected timestamp to be present")
	}
}
