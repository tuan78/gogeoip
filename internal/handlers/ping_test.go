package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tuan78/gogeoip/internal/geo"
)

type mockDB struct {
	loaded      bool
	lookupData  *geo.Data
	lookupErr   error
	lookupCalls int
}

func (m *mockDB) IsLoaded() bool {
	return m.loaded
}

func (m *mockDB) Lookup(_ string) (*geo.Data, error) {
	m.lookupCalls++
	if m.lookupErr != nil {
		return nil, m.lookupErr
	}
	return m.lookupData, nil
}

func decodeJSONMap(t *testing.T, rr *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	var out map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("decode json map: %v", err)
	}
	return out
}

func TestPingHandler_DBNotLoaded_Returns503(t *testing.T) {
	h := PingHandler(&mockDB{loaded: false})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", got, "application/json")
	}
	body := decodeJSONMap(t, rr)
	if body["status"] != "MaxMind DB not loaded" {
		t.Errorf("status body: got %q, want %q", body["status"], "MaxMind DB not loaded")
	}
}

func TestPingHandler_DBLoaded_Returns200(t *testing.T) {
	h := PingHandler(&mockDB{loaded: true})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", got, "application/json")
	}
	body := decodeJSONMap(t, rr)
	if body["status"] != "ok" {
		t.Errorf("status body: got %q, want %q", body["status"], "ok")
	}
}
