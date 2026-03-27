package handlers

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/tuan78/gogeoip/internal/cache"
	"github.com/tuan78/gogeoip/internal/geo"
)

// LookupHandler returns an HTTP handler that resolves an IP address to geolocation data.
func LookupHandler(db geo.Database, c cache.Cache, keyPrefix string, cacheTTL time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ipStr := r.URL.Query().Get("ip")
		if ipStr == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ip query parameter is required"})
			return
		}
		if net.ParseIP(ipStr) == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid IP address"})
			return
		}

		if !db.IsLoaded() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "geo database not available"})
			return
		}

		ctx := context.Background()
		cacheKey := keyPrefix + ipStr

		// Check cache first.
		if cached, ok := c.Get(ctx, cacheKey); ok && cached != "" {
			var data geo.Data
			if err := json.Unmarshal([]byte(cached), &data); err == nil {
				writeJSON(w, http.StatusOK, data)
				return
			}
		}

		data, err := db.Lookup(ipStr)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		// Cache result; ignore write errors.
		if b, jsonErr := json.Marshal(data); jsonErr == nil {
			c.Set(ctx, cacheKey, string(b), cacheTTL)
		}

		writeJSON(w, http.StatusOK, data)
	}
}
