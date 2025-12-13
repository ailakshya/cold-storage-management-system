package middleware

import (
	"context"
	"net/http"
	"strings"

	"cold-backend/internal/auth"
)

type contextKey string

const UserIDKey contextKey = "user_id"
const EmailKey contextKey = "email"
const RoleKey contextKey = "role"
const HasAccountantAccessKey contextKey = "has_accountant_access"

type AuthMiddleware struct {
	jwtManager *auth.JWTManager
}

func NewAuthMiddleware(jwtManager *auth.JWTManager) *AuthMiddleware {
	return &AuthMiddleware{jwtManager: jwtManager}
}

// Authenticate is a middleware that validates JWT tokens
func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Extract token from "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
			return
		}

		token := parts[1]
		claims, err := m.jwtManager.ValidateToken(token)
		if err != nil {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Add user info to context
		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, EmailKey, claims.Email)
		ctx = context.WithValue(ctx, RoleKey, claims.Role)
		ctx = context.WithValue(ctx, HasAccountantAccessKey, claims.HasAccountantAccess)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUserIDFromContext extracts user ID from request context
func GetUserIDFromContext(ctx context.Context) (int, bool) {
	userID, ok := ctx.Value(UserIDKey).(int)
	return userID, ok
}

// GetEmailFromContext extracts email from request context
func GetEmailFromContext(ctx context.Context) (string, bool) {
	email, ok := ctx.Value(EmailKey).(string)
	return email, ok
}

// GetRoleFromContext extracts role from request context
func GetRoleFromContext(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(RoleKey).(string)
	return role, ok
}

// RequireRole is a middleware that ensures the user has one of the allowed roles
func (m *AuthMiddleware) RequireRole(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// First authenticate
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// For HTML pages, redirect to login
				if strings.Contains(r.Header.Get("Accept"), "text/html") {
					http.Redirect(w, r, "/login", http.StatusFound)
					return
				}
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}

			// Extract token from "Bearer <token>"
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				if strings.Contains(r.Header.Get("Accept"), "text/html") {
					http.Redirect(w, r, "/login", http.StatusFound)
					return
				}
				http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
				return
			}

			token := parts[1]
			claims, err := m.jwtManager.ValidateToken(token)
			if err != nil {
				if strings.Contains(r.Header.Get("Accept"), "text/html") {
					http.Redirect(w, r, "/login", http.StatusFound)
					return
				}
				http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
				return
			}

			// Check if user has one of the allowed roles
			hasRole := false
			for _, role := range allowedRoles {
				if claims.Role == role {
					hasRole = true
					break
				}
			}

			if !hasRole {
				if strings.Contains(r.Header.Get("Accept"), "text/html") {
					http.Redirect(w, r, "/dashboard", http.StatusFound)
					return
				}
				http.Error(w, "Forbidden: Insufficient permissions", http.StatusForbidden)
				return
			}

			// Add user info to context
			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, EmailKey, claims.Email)
			ctx = context.WithValue(ctx, RoleKey, claims.Role)
			ctx = context.WithValue(ctx, HasAccountantAccessKey, claims.HasAccountantAccess)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAccountantAccess is a middleware that ensures the user has accountant permissions
// This includes: admin role, accountant role, OR employee with has_accountant_access=true
func (m *AuthMiddleware) RequireAccountantAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// First authenticate
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			if strings.Contains(r.Header.Get("Accept"), "text/html") {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Extract token from "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			if strings.Contains(r.Header.Get("Accept"), "text/html") {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
			return
		}

		token := parts[1]
		claims, err := m.jwtManager.ValidateToken(token)
		if err != nil {
			if strings.Contains(r.Header.Get("Accept"), "text/html") {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Check if user has accountant access
		// Allow: admin, accountant role, OR employee with has_accountant_access=true
		hasAccess := claims.Role == "admin" || claims.Role == "accountant" || claims.HasAccountantAccess

		if !hasAccess {
			if strings.Contains(r.Header.Get("Accept"), "text/html") {
				http.Redirect(w, r, "/dashboard", http.StatusFound)
				return
			}
			http.Error(w, "Forbidden: Accountant access required", http.StatusForbidden)
			return
		}

		// Add user info to context
		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, EmailKey, claims.Email)
		ctx = context.WithValue(ctx, RoleKey, claims.Role)
		ctx = context.WithValue(ctx, HasAccountantAccessKey, claims.HasAccountantAccess)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin is a middleware that ensures the user has admin role
func (m *AuthMiddleware) RequireAdmin(next http.Handler) http.Handler {
	return m.RequireRole("admin")(next)
}
