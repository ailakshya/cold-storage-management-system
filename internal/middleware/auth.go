package middleware

import (
	"context"
	"net/http"
	"strings"

	"cold-backend/internal/auth"
	"cold-backend/internal/repositories"
)

type contextKey string

const UserIDKey contextKey = "user_id"
const EmailKey contextKey = "email"
const RoleKey contextKey = "role"
const HasAccountantAccessKey contextKey = "has_accountant_access"

type AuthMiddleware struct {
	jwtManager *auth.JWTManager
	userRepo   *repositories.UserRepository
}

func NewAuthMiddleware(jwtManager *auth.JWTManager, userRepo *repositories.UserRepository) *AuthMiddleware {
	return &AuthMiddleware{
		jwtManager: jwtManager,
		userRepo:   userRepo,
	}
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

		// Check database for current user status (for immediate permission updates)
		user, err := m.userRepo.Get(r.Context(), claims.UserID)
		if err != nil {
			http.Error(w, "User not found", http.StatusUnauthorized)
			return
		}

		// Check if user is active (from database, not token)
		if !user.IsActive {
			http.Error(w, "Account suspended. Please contact administrator.", http.StatusForbidden)
			return
		}

		// Add user info to context (using database values for real-time updates)
		ctx := context.WithValue(r.Context(), UserIDKey, user.ID)
		ctx = context.WithValue(ctx, EmailKey, user.Email)
		ctx = context.WithValue(ctx, RoleKey, user.Role)
		ctx = context.WithValue(ctx, HasAccountantAccessKey, user.HasAccountantAccess)

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

			// Check database for current user status (for immediate permission updates)
			user, err := m.userRepo.Get(r.Context(), claims.UserID)
			if err != nil {
				if strings.Contains(r.Header.Get("Accept"), "text/html") {
					http.Redirect(w, r, "/login", http.StatusFound)
					return
				}
				http.Error(w, "User not found", http.StatusUnauthorized)
				return
			}

			// Check if user is active (from database)
			if !user.IsActive {
				if strings.Contains(r.Header.Get("Accept"), "text/html") {
					http.Redirect(w, r, "/login?error=suspended", http.StatusFound)
					return
				}
				http.Error(w, "Account suspended. Please contact administrator.", http.StatusForbidden)
				return
			}

			// Check if user has one of the allowed roles (from database)
			hasRole := false
			for _, role := range allowedRoles {
				if user.Role == role {
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

			// Add user info to context (using database values)
			ctx := context.WithValue(r.Context(), UserIDKey, user.ID)
			ctx = context.WithValue(ctx, EmailKey, user.Email)
			ctx = context.WithValue(ctx, RoleKey, user.Role)
			ctx = context.WithValue(ctx, HasAccountantAccessKey, user.HasAccountantAccess)

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

		// Check database for current user status (for immediate permission updates)
		user, err := m.userRepo.Get(r.Context(), claims.UserID)
		if err != nil {
			if strings.Contains(r.Header.Get("Accept"), "text/html") {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			http.Error(w, "User not found", http.StatusUnauthorized)
			return
		}

		// Check if user is active (from database)
		if !user.IsActive {
			if strings.Contains(r.Header.Get("Accept"), "text/html") {
				http.Redirect(w, r, "/login?error=suspended", http.StatusFound)
				return
			}
			http.Error(w, "Account suspended. Please contact administrator.", http.StatusForbidden)
			return
		}

		// Check if user has accountant access (from database)
		// Allow: admin, accountant role, OR employee with has_accountant_access=true
		hasAccess := user.Role == "admin" || user.Role == "accountant" || user.HasAccountantAccess

		if !hasAccess {
			if strings.Contains(r.Header.Get("Accept"), "text/html") {
				http.Redirect(w, r, "/dashboard", http.StatusFound)
				return
			}
			http.Error(w, "Forbidden: Accountant access required", http.StatusForbidden)
			return
		}

		// Add user info to context (using database values)
		ctx := context.WithValue(r.Context(), UserIDKey, user.ID)
		ctx = context.WithValue(ctx, EmailKey, user.Email)
		ctx = context.WithValue(ctx, RoleKey, user.Role)
		ctx = context.WithValue(ctx, HasAccountantAccessKey, user.HasAccountantAccess)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin is a middleware that ensures the user has admin role
func (m *AuthMiddleware) RequireAdmin(next http.Handler) http.Handler {
	return m.RequireRole("admin")(next)
}

// Customer authentication middleware

const CustomerIDKey contextKey = "customer_id"
const CustomerPhoneKey contextKey = "customer_phone"
const CustomerNameKey contextKey = "customer_name"

// AuthenticateCustomer is a middleware that validates customer JWT tokens
func (m *AuthMiddleware) AuthenticateCustomer(next http.Handler) http.Handler {
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
		claims, err := m.jwtManager.ValidateCustomerToken(token)
		if err != nil {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Add customer info to context
		ctx := context.WithValue(r.Context(), CustomerIDKey, claims.CustomerID)
		ctx = context.WithValue(ctx, CustomerPhoneKey, claims.Phone)
		ctx = context.WithValue(ctx, CustomerNameKey, claims.Name)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetCustomerIDFromContext extracts customer ID from request context
func GetCustomerIDFromContext(ctx context.Context) (int, bool) {
	customerID, ok := ctx.Value(CustomerIDKey).(int)
	return customerID, ok
}
