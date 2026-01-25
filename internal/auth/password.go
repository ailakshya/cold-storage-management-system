package auth

import "golang.org/x/crypto/bcrypt"

// bcryptCost of 8 for performance on low-CPU K3s nodes
// Cost 8 = ~25ms, Cost 10 = ~100ms, Cost 12 = ~400ms per hash
// Acceptable for internal cold storage system with rate limiting
const bcryptCost = 10

// HashPassword generates a bcrypt hash of the password
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword checks if the provided password matches the hash
func VerifyPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}
