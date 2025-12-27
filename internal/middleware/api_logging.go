package middleware

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
	"cold-backend/internal/timeutil"
)

// APILoggingMiddleware logs API requests to TimescaleDB
type APILoggingMiddleware struct {
	repo    *repositories.MetricsRepository
	logChan chan *models.APIRequestLog
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
func NewAPILoggingMiddleware(repo *repositories.MetricsRepository) *APILoggingMiddleware {
	m := &APILoggingMiddleware{
		repo:    repo,
		logChan: make(chan *models.APIRequestLog, 1000), // Buffer for async logging
	}

	// Start async log writer
	go m.asyncLogWriter()

	return m
}

// asyncLogWriter writes logs asynchronously to avoid blocking requests
func (m *APILoggingMiddleware) asyncLogWriter() {
	for log := range m.logChan {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := m.repo.InsertAPILog(ctx, log); err != nil {
			// Log error but don't block
			// Use standard log since we can't import a logger here
			_ = err // Silently ignore - metrics logging shouldn't affect requests
		}
		cancel()
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

		// Capture request size
		var requestSize int
		if r.Body != nil {
			body, _ := io.ReadAll(r.Body)
			requestSize = len(body)
			r.Body = io.NopCloser(bytes.NewBuffer(body))
		}

		// Wrap response writer to capture status and size
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Execute the request
		next.ServeHTTP(wrapped, r)

		// Calculate duration
		duration := time.Since(start)

		// Extract user info from context
		var userID *int
		var userEmail, userRole *string
		if claims, ok := r.Context().Value("claims").(map[string]interface{}); ok {
			if id, ok := claims["user_id"].(float64); ok {
				intID := int(id)
				userID = &intID
			}
			if email, ok := claims["email"].(string); ok {
				userEmail = &email
			}
			if role, ok := claims["role"].(string); ok {
				userRole = &role
			}
		}

		// Create log entry
		logEntry := &models.APIRequestLog{
			Time:         timeutil.Now(),
			Method:       r.Method,
			Path:         sanitizePath(r.URL.Path),
			StatusCode:   wrapped.statusCode,
			DurationMs:   float64(duration.Microseconds()) / 1000.0,
			RequestSize:  requestSize,
			ResponseSize: wrapped.bytesWritten,
			UserID:       userID,
			UserEmail:    userEmail,
			UserRole:     userRole,
			IPAddress:    getClientIP(r),
			UserAgent:    r.UserAgent(),
		}

		// Capture error message for error responses
		if wrapped.statusCode >= 400 {
			errMsg := http.StatusText(wrapped.statusCode)
			logEntry.ErrorMessage = &errMsg
		}

		// Send to async writer (non-blocking)
		select {
		case m.logChan <- logEntry:
		default:
			// Channel full, log dropped (shouldn't happen often with 1000 buffer)
			log.Printf("[APILogging] Log buffer full, dropping log entry for %s", r.URL.Path)
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
	close(m.logChan)
}
