package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"time"

	"cold-backend/internal/models"
	"cold-backend/internal/repositories"

	razorpay "github.com/razorpay/razorpay-go"
)

type RazorpayService struct {
	transactionRepo   *repositories.OnlineTransactionRepository
	rentPaymentRepo   *repositories.RentPaymentRepository
	ledgerRepo        *repositories.LedgerRepository
	customerRepo      *repositories.CustomerRepository
	systemSettingRepo *repositories.SystemSettingRepository
	// Fallback credentials from environment (used if DB credentials not set)
	envKeyID         string
	envKeySecret     string
	envWebhookSecret string
}

func NewRazorpayService(
	keyID, keySecret, webhookSecret string,
	transactionRepo *repositories.OnlineTransactionRepository,
	rentPaymentRepo *repositories.RentPaymentRepository,
	ledgerRepo *repositories.LedgerRepository,
	customerRepo *repositories.CustomerRepository,
	systemSettingRepo *repositories.SystemSettingRepository,
) *RazorpayService {
	return &RazorpayService{
		transactionRepo:   transactionRepo,
		rentPaymentRepo:   rentPaymentRepo,
		ledgerRepo:        ledgerRepo,
		customerRepo:      customerRepo,
		systemSettingRepo: systemSettingRepo,
		envKeyID:          keyID,
		envKeySecret:      keySecret,
		envWebhookSecret:  webhookSecret,
	}
}

// getCredentials returns the Razorpay credentials (from DB first, then env fallback)
func (s *RazorpayService) getCredentials(ctx context.Context) (keyID, keySecret, webhookSecret string) {
	// Try to get from database first
	if setting, err := s.systemSettingRepo.Get(ctx, "razorpay_key_id"); err == nil && setting != nil && setting.SettingValue != "" {
		keyID = setting.SettingValue
	}
	if setting, err := s.systemSettingRepo.Get(ctx, "razorpay_key_secret"); err == nil && setting != nil && setting.SettingValue != "" {
		keySecret = setting.SettingValue
	}
	if setting, err := s.systemSettingRepo.Get(ctx, "razorpay_webhook_secret"); err == nil && setting != nil && setting.SettingValue != "" {
		webhookSecret = setting.SettingValue
	}

	// Fallback to environment variables
	if keyID == "" {
		keyID = s.envKeyID
	}
	if keySecret == "" {
		keySecret = s.envKeySecret
	}
	if webhookSecret == "" {
		webhookSecret = s.envWebhookSecret
	}

	return keyID, keySecret, webhookSecret
}

// getClient returns a Razorpay client with current credentials
func (s *RazorpayService) getClient(ctx context.Context) *razorpay.Client {
	keyID, keySecret, _ := s.getCredentials(ctx)
	if keyID == "" || keySecret == "" {
		return nil
	}
	return razorpay.NewClient(keyID, keySecret)
}

// getKeyID returns the current key ID for frontend
func (s *RazorpayService) getKeyID(ctx context.Context) string {
	keyID, _, _ := s.getCredentials(ctx)
	return keyID
}

// getKeySecret returns the current key secret for signature verification
func (s *RazorpayService) getKeySecret(ctx context.Context) string {
	_, keySecret, _ := s.getCredentials(ctx)
	return keySecret
}

// getWebhookSecret returns the current webhook secret
func (s *RazorpayService) getWebhookSecret(ctx context.Context) string {
	_, _, webhookSecret := s.getCredentials(ctx)
	return webhookSecret
}

// IsEnabled checks if online payments are enabled in system settings
func (s *RazorpayService) IsEnabled(ctx context.Context) bool {
	// Only check the toggle setting - credentials are checked when actually creating payment
	setting, err := s.systemSettingRepo.Get(ctx, "online_payment_enabled")
	if err != nil || setting == nil {
		return false
	}

	return setting.SettingValue == "true"
}

// GetFeePercent returns the configured fee percentage
func (s *RazorpayService) GetFeePercent(ctx context.Context) float64 {
	setting, err := s.systemSettingRepo.Get(ctx, "online_payment_fee_percent")
	if err != nil || setting == nil {
		return 2.5 // Default 2.5%
	}

	fee, err := strconv.ParseFloat(setting.SettingValue, 64)
	if err != nil {
		return 2.5
	}

	return fee
}

// CalculateFee calculates the transaction fee for a given amount
func (s *RazorpayService) CalculateFee(amount float64, feePercent float64) float64 {
	return float64(int((amount*feePercent/100)*100+0.5)) / 100 // Round to 2 decimal places
}

// GetPaymentStatus returns payment status info for frontend
func (s *RazorpayService) GetPaymentStatus(ctx context.Context) *models.PaymentStatusResponse {
	enabled := s.IsEnabled(ctx)
	feePercent := s.GetFeePercent(ctx)

	return &models.PaymentStatusResponse{
		Enabled:    enabled,
		FeePercent: feePercent,
		KeyID:      s.getKeyID(ctx),
	}
}

// CreateOrder creates a Razorpay order and stores transaction record
func (s *RazorpayService) CreateOrder(ctx context.Context, customer *models.Customer, req *models.CreateOnlinePaymentRequest) (*models.CreateOrderResponse, error) {
	if !s.IsEnabled(ctx) {
		return nil, fmt.Errorf("online payments are currently disabled")
	}

	client := s.getClient(ctx)
	if client == nil {
		return nil, fmt.Errorf("razorpay client not configured")
	}

	// Calculate fee
	feePercent := s.GetFeePercent(ctx)
	feeAmount := s.CalculateFee(req.Amount, feePercent)
	totalAmount := req.Amount + feeAmount

	// Convert to paise (Razorpay uses paise)
	amountPaise := int(totalAmount * 100)

	// Create Razorpay order
	orderData := map[string]interface{}{
		"amount":   amountPaise,
		"currency": "INR",
		"receipt":  fmt.Sprintf("rcpt_%d_%d", customer.ID, time.Now().Unix()),
		"notes": map[string]interface{}{
			"customer_id":    customer.ID,
			"customer_phone": customer.Phone,
			"payment_scope":  req.PaymentScope,
		},
	}

	order, err := client.Order.Create(orderData, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create razorpay order: %w", err)
	}

	orderID := order["id"].(string)

	// Store transaction record
	tx := &models.OnlineTransaction{
		RazorpayOrderID:  orderID,
		CustomerID:       customer.ID,
		CustomerPhone:    customer.Phone,
		CustomerName:     customer.Name,
		EntryID:          req.EntryID,
		FamilyMemberID:   req.FamilyMemberID,
		ThockNumber:      req.ThockNumber,
		FamilyMemberName: req.FamilyMemberName,
		PaymentScope:     req.PaymentScope,
		Amount:           req.Amount,
		FeeAmount:        feeAmount,
		TotalAmount:      totalAmount,
	}

	if err := s.transactionRepo.Create(ctx, tx); err != nil {
		return nil, fmt.Errorf("failed to store transaction: %w", err)
	}

	return &models.CreateOrderResponse{
		OrderID:       orderID,
		Amount:        int(req.Amount * 100),
		FeeAmount:     int(feeAmount * 100),
		TotalAmount:   amountPaise,
		Currency:      "INR",
		KeyID:         s.getKeyID(ctx),
		CustomerName:  customer.Name,
		CustomerPhone: customer.Phone,
		FeePercent:    feePercent,
	}, nil
}

// VerifyPayment verifies the payment signature and marks as success/failed
func (s *RazorpayService) VerifyPayment(ctx context.Context, req *models.VerifyPaymentRequest) (*models.OnlineTransaction, error) {
	// Verify signature
	if !s.verifySignature(ctx, req.RazorpayOrderID, req.RazorpayPaymentID, req.RazorpaySignature) {
		// Mark as failed
		_ = s.transactionRepo.UpdatePaymentFailed(ctx, req.RazorpayOrderID, "Invalid signature")
		return nil, fmt.Errorf("invalid payment signature")
	}

	// Get transaction
	tx, err := s.transactionRepo.GetByOrderID(ctx, req.RazorpayOrderID)
	if err != nil {
		return nil, fmt.Errorf("transaction not found: %w", err)
	}

	// Check if already processed
	if tx.Status == models.OnlineTxStatusSuccess {
		return tx, nil // Already processed, return existing
	}

	// Fetch payment details from Razorpay
	client := s.getClient(ctx)
	var payment map[string]interface{}
	if client != nil {
		payment, err = client.Payment.Fetch(req.RazorpayPaymentID, nil, nil)
		if err != nil {
			log.Printf("[Razorpay] Failed to fetch payment details: %v", err)
		}
	}

	// Extract payment details
	utr := ""
	method := ""
	bank := ""
	vpa := ""
	cardLast4 := ""
	cardNetwork := ""

	if payment != nil {
		if v, ok := payment["acquirer_data"].(map[string]interface{}); ok {
			if u, ok := v["upi_transaction_id"].(string); ok {
				utr = u
			}
			if u, ok := v["bank_transaction_id"].(string); ok && utr == "" {
				utr = u
			}
			if u, ok := v["rrn"].(string); ok && utr == "" {
				utr = u
			}
		}

		if m, ok := payment["method"].(string); ok {
			method = m
		}
		if b, ok := payment["bank"].(string); ok {
			bank = b
		}
		if v, ok := payment["vpa"].(string); ok {
			vpa = v
		}
		if card, ok := payment["card"].(map[string]interface{}); ok {
			if last4, ok := card["last4"].(string); ok {
				cardLast4 = last4
			}
			if network, ok := card["network"].(string); ok {
				cardNetwork = network
			}
		}
	}

	// Update transaction as successful
	err = s.transactionRepo.UpdatePaymentSuccess(
		ctx, req.RazorpayOrderID, req.RazorpayPaymentID, req.RazorpaySignature,
		utr, method, bank, vpa, cardLast4, cardNetwork,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update transaction: %w", err)
	}

	// Create rent payment and ledger entry
	err = s.createRentPaymentAndLedgerEntry(ctx, tx, utr)
	if err != nil {
		log.Printf("[Razorpay] Failed to create rent payment: %v", err)
		// Don't fail the verification, payment is still successful
	}

	// Fetch updated transaction
	tx, _ = s.transactionRepo.GetByOrderID(ctx, req.RazorpayOrderID)

	return tx, nil
}

// verifySignature verifies the Razorpay payment signature
func (s *RazorpayService) verifySignature(ctx context.Context, orderID, paymentID, signature string) bool {
	keySecret := s.getKeySecret(ctx)
	if keySecret == "" {
		return false
	}
	data := orderID + "|" + paymentID
	h := hmac.New(sha256.New, []byte(keySecret))
	h.Write([]byte(data))
	expectedSignature := hex.EncodeToString(h.Sum(nil))
	return hmac.Equal([]byte(expectedSignature), []byte(signature))
}

// VerifyWebhookSignature verifies the webhook signature
func (s *RazorpayService) VerifyWebhookSignature(ctx context.Context, body []byte, signature string) bool {
	webhookSecret := s.getWebhookSecret(ctx)
	if webhookSecret == "" {
		return true // Skip verification if not configured
	}

	h := hmac.New(sha256.New, []byte(webhookSecret))
	h.Write(body)
	expectedSignature := hex.EncodeToString(h.Sum(nil))
	return hmac.Equal([]byte(expectedSignature), []byte(signature))
}

// createRentPaymentAndLedgerEntry creates linked records after successful payment
func (s *RazorpayService) createRentPaymentAndLedgerEntry(ctx context.Context, tx *models.OnlineTransaction, utr string) error {
	// Determine entry_id for rent payment
	entryID := 0
	if tx.EntryID != nil {
		entryID = *tx.EntryID
	}

	// Create rent payment record
	rentPayment := &models.RentPayment{
		EntryID:       entryID,
		CustomerName:  tx.CustomerName,
		CustomerPhone: tx.CustomerPhone,
		TotalRent:     0, // Will be calculated based on entry
		AmountPaid:    tx.Amount,
		Balance:       0,
		Notes:         fmt.Sprintf("Online Payment via Razorpay | UTR: %s | Fee: ₹%.2f", utr, tx.FeeAmount),
	}

	if tx.FamilyMemberID != nil {
		rentPayment.FamilyMemberID = tx.FamilyMemberID
	}
	if tx.FamilyMemberName != "" {
		rentPayment.FamilyMemberName = tx.FamilyMemberName
	}

	err := s.rentPaymentRepo.Create(ctx, rentPayment)
	if err != nil {
		return fmt.Errorf("failed to create rent payment: %w", err)
	}

	// Get customer S/O for ledger entry
	customer, _ := s.customerRepo.Get(ctx, tx.CustomerID)
	customerSO := ""
	if customer != nil {
		customerSO = customer.SO
	}

	// Create ledger entry
	description := fmt.Sprintf("Online Payment | %s | UTR: %s", tx.PaymentScope, utr)
	if tx.ThockNumber != "" {
		description = fmt.Sprintf("Online Payment for Thock %s | UTR: %s", tx.ThockNumber, utr)
	} else if tx.FamilyMemberName != "" {
		description = fmt.Sprintf("Online Payment for %s | UTR: %s", tx.FamilyMemberName, utr)
	}

	ledgerEntry, err := s.ledgerRepo.Create(ctx, &models.CreateLedgerEntryRequest{
		CustomerPhone: tx.CustomerPhone,
		CustomerName:  tx.CustomerName,
		CustomerSO:    customerSO,
		EntryType:     models.LedgerEntryTypeOnlinePayment,
		Description:   description,
		Credit:        tx.Amount,
		ReferenceID:   &rentPayment.ID,
		ReferenceType: "online_transaction",
		Notes:         fmt.Sprintf("Razorpay Payment ID: %s, Fee: ₹%.2f", tx.RazorpayPaymentID, tx.FeeAmount),
	})
	if err != nil {
		log.Printf("[Razorpay] Failed to create ledger entry: %v", err)
	}

	// Link transaction to created records
	ledgerEntryID := 0
	if ledgerEntry != nil {
		ledgerEntryID = ledgerEntry.ID
	}
	_ = s.transactionRepo.LinkToRentPayment(ctx, tx.RazorpayOrderID, rentPayment.ID, ledgerEntryID)

	return nil
}

// ProcessWebhook processes Razorpay webhook events
func (s *RazorpayService) ProcessWebhook(ctx context.Context, event string, paymentData map[string]interface{}) error {
	switch event {
	case "payment.captured":
		return s.handlePaymentCaptured(ctx, paymentData)
	case "payment.failed":
		return s.handlePaymentFailed(ctx, paymentData)
	default:
		log.Printf("[Razorpay] Unhandled webhook event: %s", event)
		return nil
	}
}

func (s *RazorpayService) handlePaymentCaptured(ctx context.Context, paymentData map[string]interface{}) error {
	paymentEntity, ok := paymentData["payment"].(map[string]interface{})
	if !ok {
		paymentEntity = paymentData
	}
	entity, ok := paymentEntity["entity"].(map[string]interface{})
	if !ok {
		entity = paymentEntity
	}

	orderID, _ := entity["order_id"].(string)
	paymentID, _ := entity["id"].(string)

	if orderID == "" {
		return fmt.Errorf("missing order_id in webhook")
	}

	// Check if already processed
	processed, _ := s.transactionRepo.IsPaymentProcessed(ctx, orderID)
	if processed {
		log.Printf("[Razorpay] Payment already processed: %s", orderID)
		return nil
	}

	// Get transaction
	tx, err := s.transactionRepo.GetByOrderID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("transaction not found: %w", err)
	}

	// Extract payment details
	utr := ""
	method := ""
	bank := ""
	vpa := ""

	if acquirerData, ok := entity["acquirer_data"].(map[string]interface{}); ok {
		if u, ok := acquirerData["upi_transaction_id"].(string); ok {
			utr = u
		}
		if u, ok := acquirerData["bank_transaction_id"].(string); ok && utr == "" {
			utr = u
		}
	}

	if m, ok := entity["method"].(string); ok {
		method = m
	}
	if b, ok := entity["bank"].(string); ok {
		bank = b
	}
	if v, ok := entity["vpa"].(string); ok {
		vpa = v
	}

	// Update transaction
	err = s.transactionRepo.UpdatePaymentSuccess(ctx, orderID, paymentID, "", utr, method, bank, vpa, "", "")
	if err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	// Create rent payment and ledger entry
	return s.createRentPaymentAndLedgerEntry(ctx, tx, utr)
}

func (s *RazorpayService) handlePaymentFailed(ctx context.Context, paymentData map[string]interface{}) error {
	paymentEntity, ok := paymentData["payment"].(map[string]interface{})
	if !ok {
		paymentEntity = paymentData
	}
	entity, ok := paymentEntity["entity"].(map[string]interface{})
	if !ok {
		entity = paymentEntity
	}

	orderID, _ := entity["order_id"].(string)
	reason := "Payment failed"

	if errorData, ok := entity["error_description"].(string); ok {
		reason = errorData
	}

	if orderID != "" {
		return s.transactionRepo.UpdatePaymentFailed(ctx, orderID, reason)
	}

	return nil
}

// GetTransactionHistory returns transaction history for a customer
func (s *RazorpayService) GetTransactionHistory(ctx context.Context, customerID int, limit, offset int) ([]*models.OnlineTransaction, error) {
	return s.transactionRepo.GetByCustomer(ctx, customerID, limit, offset)
}

// GetAllTransactions returns all transactions with filters (admin)
func (s *RazorpayService) GetAllTransactions(ctx context.Context, filter *models.OnlineTransactionFilter) ([]*models.OnlineTransaction, int, error) {
	return s.transactionRepo.GetAll(ctx, filter)
}

// GetSummary returns payment summary for reports
func (s *RazorpayService) GetSummary(ctx context.Context, startDate, endDate *time.Time) (*models.OnlinePaymentSummary, error) {
	return s.transactionRepo.GetSummary(ctx, startDate, endDate)
}
