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
	apiURL := "https://www.fast2sms.com/dev/bulkV2"

	message := fmt.Sprintf("Your Cold Storage OTP is %s. Valid for 5 minutes. Do not share this code with anyone.", otp)

	data := url.Values{}
	data.Set("authorization", s.APIKey)
	data.Set("message", message)
	data.Set("language", "english")
	data.Set("route", "q") // Quick route for OTP/transactional SMS
	data.Set("numbers", phone)

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create SMS request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send SMS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("SMS API error (status %d): %s", resp.StatusCode, string(body))
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
