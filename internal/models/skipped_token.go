package models

import "time"

// SkippedToken represents a token number that was skipped (lost physical token)
type SkippedToken struct {
	ID              int       `json:"id"`
	TokenNumber     int       `json:"token_number"`
	SkipDate        time.Time `json:"skip_date"`
	Reason          string    `json:"reason"`
	SkippedByUserID int       `json:"skipped_by_user_id"`
	CreatedAt       time.Time `json:"created_at"`
}

// SkipTokenRequest represents the request body for skipping a token
type SkipTokenRequest struct {
	TokenNumber int    `json:"token_number"`
	Reason      string `json:"reason"`
}
