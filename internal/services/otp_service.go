package services

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
	"cold-backend/internal/sms"
	"cold-backend/internal/timeutil"
)

// Default rate limiting values (can be overridden via system settings)
const (
	OTPLength        = 6
	OTPExpiryMinutes = 5
	MaxOTPAttempts   = 3

	// Default rate limits (configurable via system settings)
	DefaultOTPCooldownMinutes = 2
	DefaultMaxOTPPerHour      = 3
	DefaultMaxOTPPerDay       = 10
	DefaultMaxOTPPerIPHour    = 10
	DefaultMaxOTPPerIPDay     = 50
	DefaultMaxDailySMS        = 1000
)

// Rate limit setting keys
const (
	SettingOTPCooldownMinutes = "sms_otp_cooldown_minutes"
	SettingMaxOTPPerHour      = "sms_max_otp_per_hour"
	SettingMaxOTPPerDay       = "sms_max_otp_per_day"
	SettingMaxOTPPerIPHour    = "sms_max_otp_per_ip_hour"
	SettingMaxOTPPerIPDay     = "sms_max_otp_per_ip_day"
	SettingMaxDailySMS        = "sms_max_daily_total"
)

type OTPService struct {
	OTPRepo         *repositories.OTPRepository
	CustomerRepo    *repositories.CustomerRepository
	SMSService      sms.SMSProvider
	SettingRepo     *repositories.SystemSettingRepository
	ActivityLogRepo *repositories.CustomerActivityLogRepository
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
	}
}

// SetSettingRepo sets the system setting repository for configurable rate limits
func (s *OTPService) SetSettingRepo(repo *repositories.SystemSettingRepository) {
	s.SettingRepo = repo
}

// SetActivityLogRepo sets the activity log repository for logging customer actions
func (s *OTPService) SetActivityLogRepo(repo *repositories.CustomerActivityLogRepository) {
	s.ActivityLogRepo = repo
}

// getRateLimitSetting fetches a rate limit setting from the database with fallback to default
func (s *OTPService) getRateLimitSetting(ctx context.Context, key string, defaultValue int) int {
	if s.SettingRepo == nil {
		return defaultValue
	}

	setting, err := s.SettingRepo.Get(ctx, key)
	if err != nil || setting == nil {
		return defaultValue
	}

	value, err := strconv.Atoi(setting.SettingValue)
	if err != nil {
		return defaultValue
	}

	return value
}

// LogActivity logs a customer activity
func (s *OTPService) LogActivity(ctx context.Context, customerID int, phone, action, details, ipAddress, userAgent string) {
	if s.ActivityLogRepo == nil {
		return
	}

	log := &models.CustomerActivityLog{
		CustomerID: customerID,
		Phone:      phone,
		Action:     action,
		Details:    details,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
	}

	// Non-blocking log - don't fail the main operation
	go func() {
		s.ActivityLogRepo.Create(context.Background(), log)
	}()
}

// GenerateOTP creates a secure 6-digit OTP code
func (s *OTPService) GenerateOTP() string {
	max := big.NewInt(999999)
	n, _ := rand.Int(rand.Reader, max)
	return fmt.Sprintf("%06d", n.Int64())
}

// CanRequestOTP checks if a phone number can request an OTP (rate limiting)
func (s *OTPService) CanRequestOTP(ctx context.Context, phone string) error {
	// Get configurable rate limits
	cooldownMinutes := s.getRateLimitSetting(ctx, SettingOTPCooldownMinutes, DefaultOTPCooldownMinutes)
	maxOTPPerHour := s.getRateLimitSetting(ctx, SettingMaxOTPPerHour, DefaultMaxOTPPerHour)
	maxOTPPerDay := s.getRateLimitSetting(ctx, SettingMaxOTPPerDay, DefaultMaxOTPPerDay)

	// Check cooldown period
	recentCount, err := s.OTPRepo.CountRecentRequests(ctx, phone, time.Duration(cooldownMinutes)*time.Minute)
	if err != nil {
		return fmt.Errorf("failed to check recent requests: %w", err)
	}

	if recentCount > 0 {
		return fmt.Errorf("please wait %d minutes before requesting another OTP", cooldownMinutes)
	}

	// Check hourly limit
	hourlyCount, err := s.OTPRepo.CountRecentRequests(ctx, phone, 1*time.Hour)
	if err != nil {
		return fmt.Errorf("failed to check hourly limit: %w", err)
	}

	if hourlyCount >= maxOTPPerHour {
		return fmt.Errorf("maximum OTP requests exceeded. Please try again after 1 hour")
	}

	// Check daily limit
	dailyCount, err := s.OTPRepo.CountRecentRequests(ctx, phone, 24*time.Hour)
	if err != nil {
		return fmt.Errorf("failed to check daily limit: %w", err)
	}

	if dailyCount >= maxOTPPerDay {
		return fmt.Errorf("maximum daily OTP requests exceeded. Please try again tomorrow")
	}

	return nil
}

// CheckIPRateLimit checks if an IP address can request OTPs (prevent automated attacks)
func (s *OTPService) CheckIPRateLimit(ctx context.Context, ipAddress string) error {
	if ipAddress == "" {
		return nil // Skip if IP not available
	}

	// Get configurable rate limits
	maxOTPPerIPHour := s.getRateLimitSetting(ctx, SettingMaxOTPPerIPHour, DefaultMaxOTPPerIPHour)
	maxOTPPerIPDay := s.getRateLimitSetting(ctx, SettingMaxOTPPerIPDay, DefaultMaxOTPPerIPDay)

	// Check hourly limit per IP
	hourlyCount, err := s.OTPRepo.CountRequestsByIP(ctx, ipAddress, 1*time.Hour)
	if err != nil {
		return fmt.Errorf("failed to check IP hourly limit: %w", err)
	}

	if hourlyCount >= maxOTPPerIPHour {
		return fmt.Errorf("too many requests from your network. Please try again later")
	}

	// Check daily limit per IP
	dailyCount, err := s.OTPRepo.CountRequestsByIP(ctx, ipAddress, 24*time.Hour)
	if err != nil {
		return fmt.Errorf("failed to check IP daily limit: %w", err)
	}

	if dailyCount >= maxOTPPerIPDay {
		return fmt.Errorf("too many requests from your network. Please try again tomorrow")
	}

	return nil
}

// CheckDailyBudget checks if daily SMS budget has been exceeded
func (s *OTPService) CheckDailyBudget(ctx context.Context) error {
	// Get configurable daily limit
	maxDailySMS := s.getRateLimitSetting(ctx, SettingMaxDailySMS, DefaultMaxDailySMS)

	// Count total OTPs sent today
	todayCount, err := s.OTPRepo.CountRecentRequests(ctx, "", 24*time.Hour)
	if err != nil {
		// Don't fail if we can't check budget, just log
		return nil
	}

	if todayCount >= maxDailySMS {
		// Log alert for admin
		fmt.Printf("ALERT: Daily SMS budget limit reached (%d/%d)\n", todayCount, maxDailySMS)
		return fmt.Errorf("service temporarily unavailable. Please try again later")
	}

	// Alert when approaching limit (80%)
	if todayCount >= int(float64(maxDailySMS)*0.8) {
		fmt.Printf("WARNING: Approaching daily SMS limit (%d/%d - 80%%)\n", todayCount, maxDailySMS)
	}

	return nil
}

// GetRateLimitSettings returns current rate limit settings for display
func (s *OTPService) GetRateLimitSettings(ctx context.Context) map[string]int {
	return map[string]int{
		"cooldown_minutes":    s.getRateLimitSetting(ctx, SettingOTPCooldownMinutes, DefaultOTPCooldownMinutes),
		"max_per_hour":        s.getRateLimitSetting(ctx, SettingMaxOTPPerHour, DefaultMaxOTPPerHour),
		"max_per_day":         s.getRateLimitSetting(ctx, SettingMaxOTPPerDay, DefaultMaxOTPPerDay),
		"max_per_ip_hour":     s.getRateLimitSetting(ctx, SettingMaxOTPPerIPHour, DefaultMaxOTPPerIPHour),
		"max_per_ip_day":      s.getRateLimitSetting(ctx, SettingMaxOTPPerIPDay, DefaultMaxOTPPerIPDay),
		"max_daily_total":     s.getRateLimitSetting(ctx, SettingMaxDailySMS, DefaultMaxDailySMS),
	}
}

// SendOTP generates and sends an OTP to a customer's phone
func (s *OTPService) SendOTP(ctx context.Context, phone, ipAddress, userAgent string) error {
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
	expiresAt := timeutil.Now().Add(OTPExpiryMinutes * time.Minute)
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

	// Log OTP request with OTP code for admin visibility
	s.LogActivity(ctx, customer.ID, phone, models.ActionOTPRequested,
		fmt.Sprintf("OTP sent: %s", otpCode), ipAddress, userAgent)

	return nil
}

// VerifyOTP checks if an OTP code is valid for a phone number
func (s *OTPService) VerifyOTP(ctx context.Context, phone, otpCode, ipAddress, userAgent string) (*models.Customer, error) {
	// Get latest OTP for this phone
	otp, err := s.OTPRepo.GetLatestByPhone(ctx, phone)
	if err != nil {
		return nil, fmt.Errorf("no OTP found for this phone number")
	}

	// Check if expired
	if timeutil.Now().After(otp.ExpiresAt) {
		s.LogActivity(ctx, 0, phone, models.ActionOTPFailed, "OTP expired", ipAddress, userAgent)
		return nil, fmt.Errorf("OTP has expired. Please request a new one")
	}

	// Check if already verified
	if otp.Verified {
		s.LogActivity(ctx, 0, phone, models.ActionOTPFailed, "OTP already used", ipAddress, userAgent)
		return nil, fmt.Errorf("OTP has already been used. Please request a new one")
	}

	// Check attempts
	if otp.Attempts >= MaxOTPAttempts {
		s.LogActivity(ctx, 0, phone, models.ActionOTPFailed, "Max attempts exceeded", ipAddress, userAgent)
		return nil, fmt.Errorf("maximum verification attempts exceeded. Please request a new OTP")
	}

	// Increment attempts
	if err := s.OTPRepo.IncrementAttempts(ctx, otp.ID); err != nil {
		// Log error but continue
		fmt.Printf("Warning: failed to increment OTP attempts: %v\n", err)
	}

	// Verify OTP code
	if otp.OTPCode != otpCode {
		s.LogActivity(ctx, 0, phone, models.ActionOTPFailed,
			fmt.Sprintf("Invalid OTP entered: %s (expected: %s)", otpCode, otp.OTPCode), ipAddress, userAgent)
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

	// Log successful verification and login
	s.LogActivity(ctx, customer.ID, phone, models.ActionOTPVerified, "OTP verified successfully", ipAddress, userAgent)
	s.LogActivity(ctx, customer.ID, phone, models.ActionLogin, "Customer logged in via OTP", ipAddress, userAgent)

	return customer, nil
}
