package auth

import (
	"errors"
	"time"

	"cold-backend/internal/models"

	"github.com/golang-jwt/jwt/v5"
)

// CustomerClaims represents JWT claims for customer authentication
type CustomerClaims struct {
	CustomerID int    `json:"customer_id"`
	Phone      string `json:"phone"`
	Name       string `json:"name"`
	IsCustomer bool   `json:"is_customer"`
	jwt.RegisteredClaims
}

// GenerateCustomerToken creates a new JWT token for a customer
func (j *JWTManager) GenerateCustomerToken(customer *models.Customer, rememberMe bool) (string, error) {
	var expirationTime time.Time

	if rememberMe {
		// 30 days for "Remember Me"
		expirationTime = time.Now().Add(30 * 24 * time.Hour)
	} else {
		// 24 hours for regular session
		expirationTime = time.Now().Add(24 * time.Hour)
	}

	claims := &CustomerClaims{
		CustomerID: customer.ID,
		Phone:      customer.Phone,
		Name:       customer.Name,
		IsCustomer: true,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    j.cfg.JWT.Issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.cfg.JWT.Secret))
}

// ValidateCustomerToken verifies a customer JWT token and returns the claims
func (j *JWTManager) ValidateCustomerToken(tokenString string) (*CustomerClaims, error) {
	claims := &CustomerClaims{}

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

	// Security check: Ensure this is a customer token
	if !claims.IsCustomer {
		return nil, errors.New("not a customer token")
	}

	return claims, nil
}
