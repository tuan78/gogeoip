package handlers

import (
	"encoding/json"
	"log"
	"net/http"
)

type dbLoaded interface {
	IsLoaded() bool
}

// PingHandler returns an HTTP handler that reports whether the geo database is loaded.
func PingHandler(db dbLoaded) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !db.IsLoaded() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "MaxMind DB not loaded"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("gogeoip: failed to encode JSON response: %v", err)
	}
}
