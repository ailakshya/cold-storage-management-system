package models

import "time"

type CustomerActivityLog struct {
	ID           int        `json:"id"`
	CustomerID   int        `json:"customer_id"`
	CustomerName string     `json:"customer_name,omitempty"`
	Phone        string     `json:"phone,omitempty"`
	Action       string     `json:"action"`
	Details      string     `json:"details,omitempty"`
	IPAddress    string     `json:"ip_address,omitempty"`
	UserAgent    string     `json:"user_agent,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// Activity action types
const (
	ActionOTPRequested     = "otp_requested"
	ActionOTPVerified      = "otp_verified"
	ActionOTPFailed        = "otp_failed"
	ActionLogin            = "login"
	ActionLogout           = "logout"
	ActionDashboardView    = "dashboard_view"
	ActionGatePassRequest  = "gate_pass_request"
	ActionGatePassApproved = "gate_pass_approved"
	ActionGatePassRejected = "gate_pass_rejected"
	ActionProfileView      = "profile_view"
)
