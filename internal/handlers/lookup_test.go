package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tuan78/gogeoip/internal/geo"
)

type mockCache struct {
	store        map[string]string
	lastSetKey   string
	lastSetVal   string
	lastSetTTL   time.Duration
	setCallCount int
}

func (m *mockCache) Get(_ context.Context, key string) (string, bool) {
	if m.store == nil {
		return "", false
	}
	v, ok := m.store[key]
	return v, ok
}

func (m *mockCache) Set(_ context.Context, key string, val string, ttl time.Duration) {
	if m.store == nil {
		m.store = map[string]string{}
	}
	m.store[key] = val
	m.lastSetKey = key
	m.lastSetVal = val
	m.lastSetTTL = ttl
	m.setCallCount++
}

func decodeGeoData(t *testing.T, rr *httptest.ResponseRecorder) geo.Data {
	t.Helper()
	var out geo.Data
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("decode geo data: %v", err)
	}
	return out
}

func TestLookupHandler_ValidationErrors(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantCode  int
		wantError string
	}{
		{
			name:      "missing ip query",
			url:       "/lookup",
			wantCode:  http.StatusBadRequest,
			wantError: "ip query parameter is required",
		},
		{
			name:      "invalid ip",
			url:       "/lookup?ip=not-an-ip",
			wantCode:  http.StatusBadRequest,
			wantError: "invalid IP address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &mockDB{loaded: true}
			c := &mockCache{}
			h := LookupHandler(db, c, "prefix:", 30*time.Minute)
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tt.url, nil)
			rr := httptest.NewRecorder()

			h(rr, req)

			if rr.Code != tt.wantCode {
				t.Errorf("status: got %d, want %d", rr.Code, tt.wantCode)
			}
			body := decodeJSONMap(t, rr)
			if body["error"] != tt.wantError {
				t.Errorf("error body: got %q, want %q", body["error"], tt.wantError)
			}
			if db.lookupCalls != 0 {
				t.Errorf("lookup calls: got %d, want 0", db.lookupCalls)
			}
		})
	}
}

func TestLookupHandler_DBNotLoaded_Returns503(t *testing.T) {
	db := &mockDB{loaded: false}
	c := &mockCache{}
	h := LookupHandler(db, c, "prefix:", time.Hour)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/lookup?ip=8.8.8.8", nil)
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
	body := decodeJSONMap(t, rr)
	if body["error"] != "geo database not available" {
		t.Errorf("error body: got %q, want %q", body["error"], "geo database not available")
	}
	if db.lookupCalls != 0 {
		t.Errorf("lookup calls: got %d, want 0", db.lookupCalls)
	}
}

func TestLookupHandler_CacheHit_SkipsDBLookup(t *testing.T) {
	cached := geo.Data{
		IP:            "8.8.8.8",
		CountryCode:   "US",
		CountryName:   "United States",
		ContinentCode: "NA",
		ContinentName: "North America",
	}
	b, err := json.Marshal(cached)
	if err != nil {
		t.Fatalf("marshal cached data: %v", err)
	}

	db := &mockDB{loaded: true}
	c := &mockCache{store: map[string]string{"cache:8.8.8.8": string(b)}}
	h := LookupHandler(db, c, "cache:", time.Hour)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/lookup?ip=8.8.8.8", nil)
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}
	got := decodeGeoData(t, rr)
	if got.IP != cached.IP || got.CountryCode != cached.CountryCode {
		t.Errorf("cached response mismatch: got %+v, want %+v", got, cached)
	}
	if db.lookupCalls != 0 {
		t.Errorf("lookup calls: got %d, want 0", db.lookupCalls)
	}
	if c.setCallCount != 0 {
		t.Errorf("cache set calls: got %d, want 0", c.setCallCount)
	}
}

func TestLookupHandler_CacheContainsInvalidJSON_FallsBackToDB(t *testing.T) {
	lookupData := &geo.Data{
		IP:            "1.1.1.1",
		CountryCode:   "AU",
		CountryName:   "Australia",
		ContinentCode: "OC",
		ContinentName: "Oceania",
	}
	db := &mockDB{loaded: true, lookupData: lookupData}
	c := &mockCache{store: map[string]string{"cache:1.1.1.1": "{invalid-json"}}
	ttl := 2 * time.Hour
	h := LookupHandler(db, c, "cache:", ttl)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/lookup?ip=1.1.1.1", nil)
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}
	if db.lookupCalls != 1 {
		t.Errorf("lookup calls: got %d, want 1", db.lookupCalls)
	}
	if c.setCallCount != 1 {
		t.Errorf("cache set calls: got %d, want 1", c.setCallCount)
	}
	if c.lastSetTTL != ttl {
		t.Errorf("cache ttl: got %s, want %s", c.lastSetTTL, ttl)
	}
	if c.lastSetKey != "cache:1.1.1.1" {
		t.Errorf("cache key: got %q, want %q", c.lastSetKey, "cache:1.1.1.1")
	}
}

func TestLookupHandler_DBError_Returns500(t *testing.T) {
	db := &mockDB{loaded: true, lookupErr: errors.New("lookup failed")}
	c := &mockCache{}
	h := LookupHandler(db, c, "prefix:", time.Hour)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/lookup?ip=8.8.8.8", nil)
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusInternalServerError)
	}
	body := decodeJSONMap(t, rr)
	if body["error"] != "lookup failed" {
		t.Errorf("error body: got %q, want %q", body["error"], "lookup failed")
	}
	if c.setCallCount != 0 {
		t.Errorf("cache set calls: got %d, want 0", c.setCallCount)
	}
}

func TestLookupHandler_DBSuccess_CachesResult(t *testing.T) {
	lookupData := &geo.Data{
		IP:            "9.9.9.9",
		CountryCode:   "US",
		CountryName:   "United States",
		ContinentCode: "NA",
		ContinentName: "North America",
	}
	db := &mockDB{loaded: true, lookupData: lookupData}
	c := &mockCache{}
	ttl := 45 * time.Minute
	h := LookupHandler(db, c, "cache:", ttl)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/lookup?ip=9.9.9.9", nil)
	rr := httptest.NewRecorder()

	h(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}
	if db.lookupCalls != 1 {
		t.Errorf("lookup calls: got %d, want 1", db.lookupCalls)
	}
	if c.setCallCount != 1 {
		t.Errorf("cache set calls: got %d, want 1", c.setCallCount)
	}
	if c.lastSetKey != "cache:9.9.9.9" {
		t.Errorf("cache key: got %q, want %q", c.lastSetKey, "cache:9.9.9.9")
	}
	if c.lastSetTTL != ttl {
		t.Errorf("cache ttl: got %s, want %s", c.lastSetTTL, ttl)
	}
	if c.lastSetVal == "" {
		t.Error("expected cached json value to be non-empty")
	}
}
