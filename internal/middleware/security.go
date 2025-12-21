package middleware

import (
	"net/http"
	"os"
	"sync"
	"time"
)

// HTTPSRedirect redirects HTTP requests to HTTPS
// Only active when FORCE_HTTPS environment variable is set to "true"
func HTTPSRedirect(next http.Handler) http.Handler {
	forceHTTPS := os.Getenv("FORCE_HTTPS") == "true"

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if forceHTTPS {
			// Check if request is already HTTPS
			// X-Forwarded-Proto header is set by reverse proxy/load balancer
			if r.Header.Get("X-Forwarded-Proto") != "https" && r.TLS == nil {
				// Redirect to HTTPS
				httpsURL := "https://" + r.Host + r.URL.RequestURI()
				http.Redirect(w, r, httpsURL, http.StatusMovedPermanently)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// SecurityHeaders adds security headers to responses
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// Prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")
		// Enable XSS filter in browsers
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		// Referrer policy
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// Content Security Policy - helps prevent XSS
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'")
		// HSTS - force HTTPS for 1 year
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}

// RateLimiter implements a simple in-memory rate limiter
type RateLimiter struct {
	requests map[string]*rateLimitEntry
	mu       sync.RWMutex
	limit    int           // max requests per window
	window   time.Duration // time window
}

type rateLimitEntry struct {
	count     int
	resetTime time.Time
}

// NewRateLimiter creates a new rate limiter
// limit: maximum requests per window
// window: time window duration
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string]*rateLimitEntry),
		limit:    limit,
		window:   window,
	}
	// Start cleanup goroutine
	go rl.cleanup()
	return rl
}

// cleanup removes expired entries periodically
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, entry := range rl.requests {
			if now.After(entry.resetTime) {
				delete(rl.requests, key)
			}
		}
		rl.mu.Unlock()
	}
}

// Allow checks if a request from the given key (usually IP) is allowed
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, exists := rl.requests[key]

	if !exists || now.After(entry.resetTime) {
		rl.requests[key] = &rateLimitEntry{
			count:     1,
			resetTime: now.Add(rl.window),
		}
		return true
	}

	if entry.count >= rl.limit {
		return false
	}

	entry.count++
	return true
}

// Middleware returns an HTTP middleware that applies rate limiting
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get client IP - check X-Forwarded-For first (for reverse proxy)
		ip := r.Header.Get("X-Real-IP")
		if ip == "" {
			ip = r.Header.Get("X-Forwarded-For")
		}
		if ip == "" {
			ip = r.RemoteAddr
		}

		if !rl.Allow(ip) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// LoginRateLimiter is a stricter rate limiter for login endpoints
// 5 attempts per minute per IP
var LoginRateLimiter = NewRateLimiter(5, time.Minute)

// APIRateLimiter is a general rate limiter for API endpoints
// 100 requests per minute per IP
var APIRateLimiter = NewRateLimiter(100, time.Minute)
