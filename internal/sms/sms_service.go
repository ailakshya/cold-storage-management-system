package sms

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// SMSProvider is an interface for sending SMS messages
type SMSProvider interface {
	SendOTP(phone, otp string) error
}

// Fast2SMSService implements SMSProvider for Fast2SMS (India)
type Fast2SMSService struct {
	APIKey string
}

// NewFast2SMSService creates a new Fast2SMS service
func NewFast2SMSService(apiKey string) *Fast2SMSService {
	return &Fast2SMSService{
		APIKey: apiKey,
	}
}

// SendOTP sends an OTP code via Fast2SMS
func (s *Fast2SMSService) SendOTP(phone, otp string) error {
	message := fmt.Sprintf("Your Cold Storage OTP is %s. Valid for 5 minutes. Do not share this code with anyone.", otp)

	// Build URL with query parameters
	apiURL := fmt.Sprintf(
		"https://www.fast2sms.com/dev/bulkV2?authorization=%s&route=q&message=%s&language=english&flash=0&numbers=%s",
		url.QueryEscape(s.APIKey),
		url.QueryEscape(message),
		url.QueryEscape(phone),
	)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create SMS request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send SMS: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("SMS API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Check for API-level errors in response body
	if strings.Contains(string(body), "\"return\":false") {
		return fmt.Errorf("SMS API error: %s", string(body))
	}

	return nil
}

// MockSMSService is a mock implementation for testing (prints OTP to console)
type MockSMSService struct{}

// NewMockSMSService creates a new mock SMS service
func NewMockSMSService() *MockSMSService {
	return &MockSMSService{}
}

// SendOTP prints the OTP to console instead of sending SMS (for testing)
func (s *MockSMSService) SendOTP(phone, otp string) error {
	fmt.Printf("\n========== MOCK SMS ==========\n")
	fmt.Printf("To: %s\n", phone)
	fmt.Printf("OTP: %s\n", otp)
	fmt.Printf("Message: Your Cold Storage OTP is %s. Valid for 5 minutes.\n", otp)
	fmt.Printf("==============================\n\n")
	return nil
}
