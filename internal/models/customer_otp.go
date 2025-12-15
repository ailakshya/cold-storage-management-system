package models

import "time"

// CustomerOTP represents an OTP code for customer portal login
type CustomerOTP struct {
	ID        int       `json:"id" db:"id"`
	Phone     string    `json:"phone" db:"phone"`
	OTPCode   string    `json:"-" db:"otp_code"`        // Never expose OTP in JSON responses
	IPAddress *string   `json:"ip_address,omitempty" db:"ip_address"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	Verified  bool      `json:"verified" db:"verified"`
	Attempts  int       `json:"attempts" db:"attempts"`
}

// SendOTPRequest represents a request to send OTP
type SendOTPRequest struct {
	Phone        string `json:"phone" binding:"required"`
	CaptchaToken string `json:"captcha_token,omitempty"`
}

// VerifyOTPRequest represents a request to verify OTP
type VerifyOTPRequest struct {
	Phone      string `json:"phone" binding:"required"`
	OTP        string `json:"otp" binding:"required"`
	RememberMe bool   `json:"remember_me"`
}

// CustomerAuthResponse is returned after successful OTP verification
type CustomerAuthResponse struct {
	Success  bool      `json:"success"`
	Token    string    `json:"token"`
	Customer *Customer `json:"customer"`
	Trucks   []string  `json:"trucks"`
}
