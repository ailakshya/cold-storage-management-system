package services

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
	"cold-backend/internal/sms"
)

const (
	OTPLength        = 6
	OTPExpiryMinutes = 5
	MaxOTPAttempts   = 3

	// Rate limiting
	OTPCooldownMinutes = 2
	MaxOTPPerHour      = 3
	MaxOTPPerDay       = 10
	MaxOTPPerIPHour    = 10
	MaxOTPPerIPDay     = 50

	// Budget limiting
	MaxDailySMS = 1000 // Adjust based on your budget
)

type OTPService struct {
	OTPRepo       *repositories.OTPRepository
	CustomerRepo  *repositories.CustomerRepository
	SMSService    sms.SMSProvider
	MaxDailySMS   int
}

func NewOTPService(
	otpRepo *repositories.OTPRepository,
	customerRepo *repositories.CustomerRepository,
	smsService sms.SMSProvider,
) *OTPService {
	return &OTPService{
		OTPRepo:      otpRepo,
		CustomerRepo: customerRepo,
		SMSService:   smsService,
		MaxDailySMS:  MaxDailySMS,
	}
}

// GenerateOTP creates a secure 6-digit OTP code
func (s *OTPService) GenerateOTP() string {
	max := big.NewInt(999999)
	n, _ := rand.Int(rand.Reader, max)
	return fmt.Sprintf("%06d", n.Int64())
}

// CanRequestOTP checks if a phone number can request an OTP (rate limiting)
func (s *OTPService) CanRequestOTP(ctx context.Context, phone string) error {
	// Check cooldown period (2 minutes)
	recentCount, err := s.OTPRepo.CountRecentRequests(ctx, phone, OTPCooldownMinutes*time.Minute)
	if err != nil {
		return fmt.Errorf("failed to check recent requests: %w", err)
	}

	if recentCount > 0 {
		return fmt.Errorf("please wait %d minutes before requesting another OTP", OTPCooldownMinutes)
	}

	// Check hourly limit
	hourlyCount, err := s.OTPRepo.CountRecentRequests(ctx, phone, 1*time.Hour)
	if err != nil {
		return fmt.Errorf("failed to check hourly limit: %w", err)
	}

	if hourlyCount >= MaxOTPPerHour {
		return fmt.Errorf("maximum OTP requests exceeded. Please try again after 1 hour")
	}

	// Check daily limit
	dailyCount, err := s.OTPRepo.CountRecentRequests(ctx, phone, 24*time.Hour)
	if err != nil {
		return fmt.Errorf("failed to check daily limit: %w", err)
	}

	if dailyCount >= MaxOTPPerDay {
		return fmt.Errorf("maximum daily OTP requests exceeded. Please try again tomorrow")
	}

	return nil
}

// CheckIPRateLimit checks if an IP address can request OTPs (prevent automated attacks)
func (s *OTPService) CheckIPRateLimit(ctx context.Context, ipAddress string) error {
	if ipAddress == "" {
		return nil // Skip if IP not available
	}

	// Check hourly limit per IP
	hourlyCount, err := s.OTPRepo.CountRequestsByIP(ctx, ipAddress, 1*time.Hour)
	if err != nil {
		return fmt.Errorf("failed to check IP hourly limit: %w", err)
	}

	if hourlyCount >= MaxOTPPerIPHour {
		return fmt.Errorf("too many requests from your network. Please try again later")
	}

	// Check daily limit per IP
	dailyCount, err := s.OTPRepo.CountRequestsByIP(ctx, ipAddress, 24*time.Hour)
	if err != nil {
		return fmt.Errorf("failed to check IP daily limit: %w", err)
	}

	if dailyCount >= MaxOTPPerIPDay {
		return fmt.Errorf("too many requests from your network. Please try again tomorrow")
	}

	return nil
}

// CheckDailyBudget checks if daily SMS budget has been exceeded
func (s *OTPService) CheckDailyBudget(ctx context.Context) error {
	// Count total OTPs sent today
	todayCount, err := s.OTPRepo.CountRecentRequests(ctx, "", 24*time.Hour)
	if err != nil {
		// Don't fail if we can't check budget, just log
		return nil
	}

	if todayCount >= s.MaxDailySMS {
		// Log alert for admin
		fmt.Printf("ALERT: Daily SMS budget limit reached (%d/%d)\n", todayCount, s.MaxDailySMS)
		return fmt.Errorf("service temporarily unavailable. Please try again later")
	}

	// Alert when approaching limit (80%)
	if todayCount >= int(float64(s.MaxDailySMS)*0.8) {
		fmt.Printf("WARNING: Approaching daily SMS limit (%d/%d - 80%%)\n", todayCount, s.MaxDailySMS)
	}

	return nil
}

// SendOTP generates and sends an OTP to a customer's phone
func (s *OTPService) SendOTP(ctx context.Context, phone, ipAddress string) error {
	// Check if customer exists
	customer, err := s.CustomerRepo.GetByPhone(ctx, phone)
	if err != nil {
		return fmt.Errorf("customer not found with this phone number")
	}

	if customer == nil {
		return fmt.Errorf("customer not found")
	}

	// Check rate limits
	if err := s.CanRequestOTP(ctx, phone); err != nil {
		return err
	}

	// Check IP rate limit
	if err := s.CheckIPRateLimit(ctx, ipAddress); err != nil {
		return err
	}

	// Check daily budget
	if err := s.CheckDailyBudget(ctx); err != nil {
		return err
	}

	// Generate OTP
	otpCode := s.GenerateOTP()

	// Store in database
	expiresAt := time.Now().Add(OTPExpiryMinutes * time.Minute)
	otp := &models.CustomerOTP{
		Phone:     phone,
		OTPCode:   otpCode,
		ExpiresAt: expiresAt,
	}

	if ipAddress != "" {
		otp.IPAddress = &ipAddress
	}

	err = s.OTPRepo.Create(ctx, otp)
	if err != nil {
		return fmt.Errorf("failed to create OTP record: %w", err)
	}

	// Send SMS
	err = s.SMSService.SendOTP(phone, otpCode)
	if err != nil {
		return fmt.Errorf("failed to send SMS: %w", err)
	}

	return nil
}

// VerifyOTP checks if an OTP code is valid for a phone number
func (s *OTPService) VerifyOTP(ctx context.Context, phone, otpCode string) (*models.Customer, error) {
	// Get latest OTP for this phone
	otp, err := s.OTPRepo.GetLatestByPhone(ctx, phone)
	if err != nil {
		return nil, fmt.Errorf("no OTP found for this phone number")
	}

	// Check if expired
	if time.Now().After(otp.ExpiresAt) {
		return nil, fmt.Errorf("OTP has expired. Please request a new one")
	}

	// Check if already verified
	if otp.Verified {
		return nil, fmt.Errorf("OTP has already been used. Please request a new one")
	}

	// Check attempts
	if otp.Attempts >= MaxOTPAttempts {
		return nil, fmt.Errorf("maximum verification attempts exceeded. Please request a new OTP")
	}

	// Increment attempts
	if err := s.OTPRepo.IncrementAttempts(ctx, otp.ID); err != nil {
		// Log error but continue
		fmt.Printf("Warning: failed to increment OTP attempts: %v\n", err)
	}

	// Verify OTP code
	if otp.OTPCode != otpCode {
		return nil, fmt.Errorf("invalid OTP code")
	}

	// Mark as verified
	if err := s.OTPRepo.MarkVerified(ctx, otp.ID); err != nil {
		// Log error but continue
		fmt.Printf("Warning: failed to mark OTP as verified: %v\n", err)
	}

	// Get customer details
	customer, err := s.CustomerRepo.GetByPhone(ctx, phone)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve customer details: %w", err)
	}

	return customer, nil
}
