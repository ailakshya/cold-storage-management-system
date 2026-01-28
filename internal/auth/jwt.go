package auth

import (
	"errors"
	"time"

	"cold-backend/internal/config"
	"cold-backend/internal/models"
	"cold-backend/internal/timeutil"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID              int    `json:"user_id"`
	Name                string `json:"name"`
	Email               string `json:"email"`
	Role                string `json:"role"`
	HasAccountantAccess bool   `json:"has_accountant_access"`
	IsActive            bool   `json:"is_active"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	cfg *config.Config
}

func NewJWTManager(cfg *config.Config) *JWTManager {
	return &JWTManager{cfg: cfg}
}

// GenerateToken creates a new JWT token for a user
func (j *JWTManager) GenerateToken(user *models.User) (string, error) {
	now := timeutil.Now()
	expirationTime := now.Add(time.Duration(j.cfg.JWT.ExpirationHours) * time.Hour)

	claims := &Claims{
		UserID:              user.ID,
		Name:                user.Name,
		Email:               user.Email,
		Role:                user.Role,
		HasAccountantAccess: user.HasAccountantAccess,
		IsActive:            user.IsActive,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    j.cfg.JWT.Issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.cfg.JWT.Secret))
}

// ValidateToken verifies a JWT token and returns the claims
func (j *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return []byte(j.cfg.JWT.Secret), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// TempClaims for short-lived 2FA tokens (used between login step 1 and step 2)
type TempClaims struct {
	UserID int    `json:"user_id"`
	Email  string `json:"email"`
	Type   string `json:"type"` // "2fa_pending"
	jwt.RegisteredClaims
}

// GenerateTempToken creates a short-lived token for 2FA verification (5 minutes)
func (j *JWTManager) GenerateTempToken(user *models.User) (string, error) {
	now := timeutil.Now()
	expirationTime := now.Add(5 * time.Minute) // 5 minute expiry for temp token

	claims := &TempClaims{
		UserID: user.ID,
		Email:  user.Email,
		Type:   "2fa_pending",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    j.cfg.JWT.Issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.cfg.JWT.Secret))
}

// ValidateTempToken verifies a temporary 2FA token and returns the claims
func (j *JWTManager) ValidateTempToken(tokenString string) (*TempClaims, error) {
	claims := &TempClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return []byte(j.cfg.JWT.Secret), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	// Verify it's a temp 2FA token
	if claims.Type != "2fa_pending" {
		return nil, errors.New("invalid token type")
	}

	return claims, nil
}
