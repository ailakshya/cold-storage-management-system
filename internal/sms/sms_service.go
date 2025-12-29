package sms

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"cold-backend/internal/models"
)

// SMSProvider is an interface for sending SMS messages
type SMSProvider interface {
	SendOTP(phone, otp string) error
	SendSMS(phone, message, messageType string, customerID int) error
	SendBulkSMS(phones []string, message string, customerIDs []int) (int, int, error)
	SetLogRepository(repo SMSLogRepo)
	SetConfig(config *SMSConfig)
}

// SMSLogRepo interface for logging
type SMSLogRepo interface {
	Create(ctx context.Context, log *models.SMSLog) error
}

// SMSConfig holds SMS configuration
type SMSConfig struct {
	Route       string // "q" (quick/expensive), "dlt" (cheap/production), "v3" (promotional)
	SenderID    string // For DLT route (e.g., "COLDST")
	TemplateID  string // For DLT route
	EntityID    string // For DLT route (PEID)
	CostPerSMS  float64
}

// Fast2SMSService implements SMSProvider for Fast2SMS (India)
type Fast2SMSService struct {
	APIKey  string
	Config  *SMSConfig
	LogRepo SMSLogRepo
}

// NewFast2SMSService creates a new Fast2SMS service
func NewFast2SMSService(apiKey string) *Fast2SMSService {
	return &Fast2SMSService{
		APIKey: apiKey,
		Config: &SMSConfig{
			Route:      "q", // Default to quick route, can be changed via settings
			CostPerSMS: 5.0, // Quick route cost
		},
	}
}

// SetLogRepository sets the SMS log repository
func (s *Fast2SMSService) SetLogRepository(repo SMSLogRepo) {
	s.LogRepo = repo
}

// SetConfig sets the SMS configuration
func (s *Fast2SMSService) SetConfig(config *SMSConfig) {
	if config != nil {
		s.Config = config
	}
}

// SendOTP sends an OTP code via Fast2SMS
func (s *Fast2SMSService) SendOTP(phone, otp string) error {
	message := fmt.Sprintf("Your Cold Storage OTP is %s. Valid for 5 minutes. Do not share this code with anyone.", otp)
	return s.SendSMS(phone, message, models.SMSTypeOTP, 0)
}

// SendSMS sends a single SMS message
func (s *Fast2SMSService) SendSMS(phone, message, messageType string, customerID int) error {
	// Build API URL based on route
	var apiURL string

	switch s.Config.Route {
	case "dlt":
		// DLT route (cheaper, requires registration)
		apiURL = fmt.Sprintf(
			"https://www.fast2sms.com/dev/bulkV2?authorization=%s&route=dlt&sender_id=%s&message=%s&variables_values=%s&flash=0&numbers=%s",
			url.QueryEscape(s.APIKey),
			url.QueryEscape(s.Config.SenderID),
			url.QueryEscape(s.Config.TemplateID),
			url.QueryEscape(message),
			url.QueryEscape(phone),
		)
	case "v3":
		// Promotional route (cheapest, 9am-9pm only)
		apiURL = fmt.Sprintf(
			"https://www.fast2sms.com/dev/bulkV2?authorization=%s&route=v3&sender_id=%s&message=%s&language=english&numbers=%s",
			url.QueryEscape(s.APIKey),
			url.QueryEscape(s.Config.SenderID),
			url.QueryEscape(message),
			url.QueryEscape(phone),
		)
	default:
		// Quick route (expensive but works immediately)
		apiURL = fmt.Sprintf(
			"https://www.fast2sms.com/dev/bulkV2?authorization=%s&route=q&message=%s&language=english&flash=0&numbers=%s",
			url.QueryEscape(s.APIKey),
			url.QueryEscape(message),
			url.QueryEscape(phone),
		)
	}

	// Create log entry
	smsLog := &models.SMSLog{
		CustomerID:  customerID,
		Phone:       phone,
		MessageType: messageType,
		Message:     message,
		Status:      models.SMSStatusPending,
		Cost:        s.Config.CostPerSMS,
	}

	// Send request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		smsLog.Status = models.SMSStatusFailed
		smsLog.ErrorMessage = err.Error()
		s.logSMS(smsLog)
		return fmt.Errorf("failed to create SMS request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		smsLog.Status = models.SMSStatusFailed
		smsLog.ErrorMessage = err.Error()
		s.logSMS(smsLog)
		return fmt.Errorf("failed to send SMS: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Parse response
	var apiResp map[string]interface{}
	json.Unmarshal(body, &apiResp)

	if resp.StatusCode != http.StatusOK {
		smsLog.Status = models.SMSStatusFailed
		smsLog.ErrorMessage = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
		s.logSMS(smsLog)
		return fmt.Errorf("SMS API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Check for API-level errors
	if strings.Contains(string(body), "\"return\":false") {
		smsLog.Status = models.SMSStatusFailed
		smsLog.ErrorMessage = string(body)
		s.logSMS(smsLog)
		return fmt.Errorf("SMS API error: %s", string(body))
	}

	// Success
	smsLog.Status = models.SMSStatusSent
	if requestID, ok := apiResp["request_id"].(string); ok {
		smsLog.ReferenceID = requestID
	}
	s.logSMS(smsLog)

	return nil
}

// SendBulkSMS sends SMS to multiple phones
func (s *Fast2SMSService) SendBulkSMS(phones []string, message string, customerIDs []int) (int, int, error) {
	success := 0
	failed := 0

	for i, phone := range phones {
		customerID := 0
		if i < len(customerIDs) {
			customerID = customerIDs[i]
		}

		err := s.SendSMS(phone, message, models.SMSTypeBulk, customerID)
		if err != nil {
			failed++
		} else {
			success++
		}

		// Rate limit: 1 SMS per 100ms to avoid API throttling
		time.Sleep(100 * time.Millisecond)
	}

	return success, failed, nil
}

// logSMS logs the SMS to database
func (s *Fast2SMSService) logSMS(log *models.SMSLog) {
	if s.LogRepo == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.LogRepo.Create(ctx, log)
	}()
}

// MockSMSService is a mock implementation for testing (prints OTP to console)
type MockSMSService struct {
	LogRepo SMSLogRepo
	Config  *SMSConfig
}

// NewMockSMSService creates a new mock SMS service
func NewMockSMSService() *MockSMSService {
	return &MockSMSService{
		Config: &SMSConfig{CostPerSMS: 0},
	}
}

// SetLogRepository sets the SMS log repository
func (s *MockSMSService) SetLogRepository(repo SMSLogRepo) {
	s.LogRepo = repo
}

// SetConfig sets the SMS configuration
func (s *MockSMSService) SetConfig(config *SMSConfig) {
	if config != nil {
		s.Config = config
	}
}

// SendOTP prints the OTP to console instead of sending SMS (for testing)
func (s *MockSMSService) SendOTP(phone, otp string) error {
	message := fmt.Sprintf("Your Cold Storage OTP is %s. Valid for 5 minutes.", otp)
	return s.SendSMS(phone, message, models.SMSTypeOTP, 0)
}

// SendSMS logs the SMS to console
func (s *MockSMSService) SendSMS(phone, message, messageType string, customerID int) error {
	fmt.Printf("\n========== MOCK SMS ==========\n")
	fmt.Printf("To: %s\n", phone)
	fmt.Printf("Type: %s\n", messageType)
	fmt.Printf("Message: %s\n", message)
	fmt.Printf("==============================\n\n")

	// Log to database
	if s.LogRepo != nil {
		smsLog := &models.SMSLog{
			CustomerID:  customerID,
			Phone:       phone,
			MessageType: messageType,
			Message:     message,
			Status:      models.SMSStatusSent,
			Cost:        0,
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			s.LogRepo.Create(ctx, smsLog)
		}()
	}

	return nil
}

// SendBulkSMS sends bulk SMS (mock)
func (s *MockSMSService) SendBulkSMS(phones []string, message string, customerIDs []int) (int, int, error) {
	for i, phone := range phones {
		customerID := 0
		if i < len(customerIDs) {
			customerID = customerIDs[i]
		}
		s.SendSMS(phone, message, models.SMSTypeBulk, customerID)
	}
	return len(phones), 0, nil
}
