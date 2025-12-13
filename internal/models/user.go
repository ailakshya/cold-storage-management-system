package models

import "time"

type User struct {
	ID                   int       `json:"id"`
	Name                 string    `json:"name"`
	Email                string    `json:"email"`
	Phone                string    `json:"phone"`
	Village              string    `json:"village"`
	PasswordHash         string    `json:"-"` // Never expose in JSON
	Role                 string    `json:"role"` // admin or employee
	HasAccountantAccess  bool      `json:"has_accountant_access"` // employees can have accountant access
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// SignupRequest represents the request body for signup
type SignupRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginRequest represents the request body for login
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse represents the response after successful authentication
type AuthResponse struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
}

// CreateUserRequest represents the request body for creating a user
type CreateUserRequest struct {
	Name                string `json:"name"`
	Email               string `json:"email"`
	Password            string `json:"password"`
	Role                string `json:"role"`
	HasAccountantAccess bool   `json:"has_accountant_access"`
}

// UpdateUserRequest represents the request body for updating a user
type UpdateUserRequest struct {
	Name                string `json:"name"`
	Email               string `json:"email"`
	Password            string `json:"password,omitempty"` // Optional
	Role                string `json:"role"`
	HasAccountantAccess bool   `json:"has_accountant_access"`
}
