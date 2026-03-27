package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/tuan78/gogeoip/internal/cache"
	"github.com/tuan78/gogeoip/internal/config"
	"github.com/tuan78/gogeoip/internal/geo"
	"github.com/tuan78/gogeoip/internal/handlers"
)

// statusCodeCapture wraps http.ResponseWriter to capture the status code.
type statusCodeCapture struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusCodeCapture) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

const (
	readTimeout  = 30 * time.Second
	writeTimeout = 30 * time.Second
	idleTimeout  = 30 * time.Second
)

const defaultCacheTTL = 24 * time.Hour

var accessLogger = log.New(os.Stdout, "", 0)

type accessLogEntry struct {
	Timestamp  string  `json:"timestamp"`
	Level      string  `json:"level"`
	Message    string  `json:"message"`
	Service    string  `json:"service"`
	Method     string  `json:"method"`
	Path       string  `json:"path"`
	Query      string  `json:"query,omitempty"`
	StatusCode int     `json:"status_code"`
	DurationMs float64 `json:"duration_ms"`
	UserAgent  string  `json:"user_agent,omitempty"`
	RemoteAddr string  `json:"remote_addr,omitempty"`
	RequestID  string  `json:"request_id,omitempty"`
	TraceID    string  `json:"trace_id,omitempty"`
}

// loggingMiddleware wraps an http.Handler to log each request with method, path, query, status, and duration.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Capture status code
		scw := &statusCodeCapture{ResponseWriter: w, statusCode: http.StatusOK}

		// Call the next handler
		next.ServeHTTP(scw, r)

		duration := time.Since(start)
		entry := accessLogEntry{
			Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
			Level:      "info",
			Message:    "http_request",
			Service:    "gogeoip",
			Method:     r.Method,
			Path:       r.URL.Path,
			StatusCode: scw.statusCode,
			DurationMs: float64(duration.Microseconds()) / 1000.0,
			UserAgent:  r.UserAgent(),
			RemoteAddr: r.RemoteAddr,
		}
		if r.URL.RawQuery != "" {
			entry.Query = r.URL.RawQuery
		}
		if requestID := r.Header.Get("X-Request-Id"); requestID != "" {
			entry.RequestID = requestID
		}
		if traceID := r.Header.Get("X-Datadog-Trace-Id"); traceID != "" {
			entry.TraceID = traceID
		} else if traceparent := r.Header.Get("Traceparent"); traceparent != "" {
			entry.TraceID = traceparent
		}

		b, err := json.Marshal(entry)
		if err != nil {
			log.Printf("gogeoip: %s %s -> %d (%v)", r.Method, r.URL.Path, scw.statusCode, duration)
			return
		}
		accessLogger.Println(string(b))
	})
}

// Serve starts the gogeoip HTTP server. It blocks until ctx is canceled.
func Serve(ctx context.Context, cfg config.Config, db geo.Database, c cache.Cache) {
	cacheTTL, err := time.ParseDuration(cfg.RedisLookupCacheTTL)
	if err != nil {
		log.Printf("gogeoip: invalid REDIS_LOOKUP_CACHE_TTL %q, using default 24h: %v", cfg.RedisLookupCacheTTL, err)
		cacheTTL = defaultCacheTTL
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", handlers.PingHandler(db))
	mux.HandleFunc("GET /lookup", handlers.LookupHandler(db, c, cfg.RedisLookupKeyPrefix, cacheTTL))

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	go func() {
		<-ctx.Done()
		log.Println("gogeoip: shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("gogeoip: graceful shutdown error: %v", err)
		}
	}()

	log.Printf("gogeoip: listening on :%s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("gogeoip: server error: %v", err)
	}
	log.Println("gogeoip: server stopped")
}
