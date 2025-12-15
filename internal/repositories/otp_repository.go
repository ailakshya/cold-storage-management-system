package repositories

import (
	"context"
	"time"

	"cold-backend/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type OTPRepository struct {
	DB *pgxpool.Pool
}

func NewOTPRepository(db *pgxpool.Pool) *OTPRepository {
	return &OTPRepository{DB: db}
}

// Create inserts a new OTP record
func (r *OTPRepository) Create(ctx context.Context, otp *models.CustomerOTP) error {
	query := `
		INSERT INTO customer_otps(phone, otp_code, ip_address, expires_at)
		VALUES($1, $2, $3, $4)
		RETURNING id, created_at
	`

	return r.DB.QueryRow(ctx, query,
		otp.Phone,
		otp.OTPCode,
		otp.IPAddress,
		otp.ExpiresAt,
	).Scan(&otp.ID, &otp.CreatedAt)
}

// GetLatestByPhone retrieves the most recent OTP for a phone number
func (r *OTPRepository) GetLatestByPhone(ctx context.Context, phone string) (*models.CustomerOTP, error) {
	query := `
		SELECT id, phone, otp_code, ip_address, created_at, expires_at, verified, attempts
		FROM customer_otps
		WHERE phone = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	var otp models.CustomerOTP
	err := r.DB.QueryRow(ctx, query, phone).Scan(
		&otp.ID,
		&otp.Phone,
		&otp.OTPCode,
		&otp.IPAddress,
		&otp.CreatedAt,
		&otp.ExpiresAt,
		&otp.Verified,
		&otp.Attempts,
	)

	if err != nil {
		return nil, err
	}

	return &otp, nil
}

// IncrementAttempts increments the verification attempt counter
func (r *OTPRepository) IncrementAttempts(ctx context.Context, id int) error {
	query := `UPDATE customer_otps SET attempts = attempts + 1 WHERE id = $1`
	_, err := r.DB.Exec(ctx, query, id)
	return err
}

// MarkVerified marks an OTP as successfully verified
func (r *OTPRepository) MarkVerified(ctx context.Context, id int) error {
	query := `UPDATE customer_otps SET verified = TRUE WHERE id = $1`
	_, err := r.DB.Exec(ctx, query, id)
	return err
}

// CountRecentRequests counts OTP requests for a phone number within a time duration
func (r *OTPRepository) CountRecentRequests(ctx context.Context, phone string, duration time.Duration) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM customer_otps
		WHERE phone = $1 AND created_at > NOW() - $2::interval
	`

	var count int
	err := r.DB.QueryRow(ctx, query, phone, duration.String()).Scan(&count)
	return count, err
}

// CountRequestsByIP counts OTP requests from an IP address within a time duration
func (r *OTPRepository) CountRequestsByIP(ctx context.Context, ipAddress string, duration time.Duration) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM customer_otps
		WHERE ip_address = $1 AND created_at > NOW() - $2::interval
	`

	var count int
	err := r.DB.QueryRow(ctx, query, ipAddress, duration.String()).Scan(&count)
	return count, err
}

// CleanupExpiredOTPs removes old OTP records (should be run as a background job)
func (r *OTPRepository) CleanupExpiredOTPs(ctx context.Context) error {
	query := `DELETE FROM customer_otps WHERE expires_at < NOW() - INTERVAL '1 day'`
	_, err := r.DB.Exec(ctx, query)
	return err
}
