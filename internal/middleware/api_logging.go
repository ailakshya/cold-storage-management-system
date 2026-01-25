package middleware

import (
	"net/http"
	"strings"
	"time"

	"cold-backend/internal/monitoring"
	"cold-backend/internal/timeutil"
)

// APILoggingMiddleware logs API requests to TimescaleDB
type APILoggingMiddleware struct {
	store *monitoring.TimescaleStore
}

// responseWriter wraps http.ResponseWriter to capture status code and size
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}

// NewAPILoggingMiddleware creates a new API logging middleware
func NewAPILoggingMiddleware(store *monitoring.TimescaleStore) *APILoggingMiddleware {
	return &APILoggingMiddleware{
		store: store,
	}
}

// Handler returns the middleware handler
func (m *APILoggingMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip logging for static files and health checks
		if shouldSkipLogging(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		start := timeutil.Now()

		// Wrap response writer to capture status and size
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Execute the request
		next.ServeHTTP(wrapped, r)

		// Calculate duration
		duration := time.Since(start)

		if m.store != nil {
			m.store.RecordAPIMetric(
				r.Method,
				sanitizePath(r.URL.Path),
				wrapped.statusCode,
				duration,
				getClientIP(r),
			)
		}
	})
}

// shouldSkipLogging returns true for paths that shouldn't be logged
func shouldSkipLogging(path string) bool {
	skipPaths := []string{
		"/static/",
		"/health",
		"/favicon.ico",
		"/robots.txt",
		"/api/monitoring/", // Don't log monitoring endpoints to avoid recursion
	}

	for _, skip := range skipPaths {
		if strings.HasPrefix(path, skip) {
			return true
		}
	}

	return false
}

// sanitizePath removes sensitive data from paths
func sanitizePath(path string) string {
	// Remove query parameters that might contain sensitive data
	if idx := strings.Index(path, "?"); idx != -1 {
		path = path[:idx]
	}

	// Truncate very long paths
	if len(path) > 500 {
		path = path[:500]
	}

	return path
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check CF-Connecting-IP header (Cloudflare)
	cfip := r.Header.Get("CF-Connecting-IP")
	if cfip != "" {
		return strings.TrimSpace(cfip)
	}

	// Check X-Forwarded-For header (for proxies/load balancers)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// Take the first IP in the list
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}

	return ip
}

// Close closes the middleware and flushes pending logs
func (m *APILoggingMiddleware) Close() {
	// Nothing to close now
}
