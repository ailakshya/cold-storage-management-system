package services

import (
	"context"
	"fmt"

	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
	"cold-backend/internal/sms"
)

// NotificationService handles sending transaction SMS notifications
type NotificationService struct {
	SMSService  sms.SMSProvider
	SettingRepo *repositories.SystemSettingRepository
}

// NewNotificationService creates a new notification service
func NewNotificationService(
	smsService sms.SMSProvider,
	settingRepo *repositories.SystemSettingRepository,
) *NotificationService {
	return &NotificationService{
		SMSService:  smsService,
		SettingRepo: settingRepo,
	}
}

// isEnabled checks if a notification type is enabled
func (s *NotificationService) isEnabled(ctx context.Context, settingKey string) bool {
	if s.SettingRepo == nil {
		return false
	}

	setting, err := s.SettingRepo.Get(ctx, settingKey)
	if err != nil || setting == nil {
		return false
	}

	return setting.SettingValue == "true"
}

// NotifyItemIn sends SMS when items are stored
func (s *NotificationService) NotifyItemIn(ctx context.Context, customer *models.Customer, thockNumber string, quantity int, totalStored int) error {
	if !s.isEnabled(ctx, models.SettingSMSItemIn) {
		return nil
	}

	if customer == nil || customer.Phone == "" {
		return nil
	}

	message := fmt.Sprintf(
		"Dear %s, %d items received at Cold Storage. Thock: %s. Total stored: %d. Thank you for choosing us!",
		customer.Name, quantity, thockNumber, totalStored,
	)

	return s.SMSService.SendSMS(customer.Phone, message, models.SMSTypeItemIn, customer.ID)
}

// NotifyItemOut sends SMS when items are picked up
func (s *NotificationService) NotifyItemOut(ctx context.Context, customer *models.Customer, thockNumber string, quantity int, remaining int, gatePassNo string) error {
	if !s.isEnabled(ctx, models.SettingSMSItemOut) {
		return nil
	}

	if customer == nil || customer.Phone == "" {
		return nil
	}

	message := fmt.Sprintf(
		"Dear %s, %d items picked up from Cold Storage. Gate Pass: %s. Remaining: %d items. Thank you!",
		customer.Name, quantity, gatePassNo, remaining,
	)

	return s.SMSService.SendSMS(customer.Phone, message, models.SMSTypeItemOut, customer.ID)
}

// NotifyPaymentReceived sends SMS when payment is received
func (s *NotificationService) NotifyPaymentReceived(ctx context.Context, customer *models.Customer, amount float64, remainingBalance float64) error {
	if !s.isEnabled(ctx, models.SettingSMSPaymentReceived) {
		return nil
	}

	if customer == nil || customer.Phone == "" {
		return nil
	}

	var message string
	if remainingBalance <= 0 {
		message = fmt.Sprintf(
			"Dear %s, payment of Rs.%.2f received. Your account is now clear. Thank you for your payment!",
			customer.Name, amount,
		)
	} else {
		message = fmt.Sprintf(
			"Dear %s, payment of Rs.%.2f received. Remaining balance: Rs.%.2f. Thank you!",
			customer.Name, amount, remainingBalance,
		)
	}

	return s.SMSService.SendSMS(customer.Phone, message, models.SMSTypePaymentReceived, customer.ID)
}
