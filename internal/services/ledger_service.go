package services

import (
	"context"
	"fmt"

	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
)

type LedgerService struct {
	LedgerRepo *repositories.LedgerRepository
}

func NewLedgerService(ledgerRepo *repositories.LedgerRepository) *LedgerService {
	return &LedgerService{
		LedgerRepo: ledgerRepo,
	}
}

// CreateEntry creates a new ledger entry
func (s *LedgerService) CreateEntry(ctx context.Context, entry *models.CreateLedgerEntryRequest) (*models.LedgerEntry, error) {
	// Validate entry type
	switch entry.EntryType {
	case models.LedgerEntryTypeCharge, models.LedgerEntryTypePayment,
		models.LedgerEntryTypeOnlinePayment, models.LedgerEntryTypeCredit,
		models.LedgerEntryTypeRefund, models.LedgerEntryTypeDebtApproval:
		// Valid
	default:
		return nil, fmt.Errorf("invalid entry type: %s", entry.EntryType)
	}

	// Validate debit/credit based on entry type
	switch entry.EntryType {
	case models.LedgerEntryTypeCharge, models.LedgerEntryTypeRefund:
		// These should have debit (money owed)
		if entry.Debit <= 0 {
			return nil, fmt.Errorf("%s entry must have positive debit amount", entry.EntryType)
		}
		entry.Credit = 0
	case models.LedgerEntryTypePayment, models.LedgerEntryTypeOnlinePayment, models.LedgerEntryTypeCredit:
		// These should have credit (money paid/credited)
		if entry.Credit <= 0 {
			return nil, fmt.Errorf("%s entry must have positive credit amount", entry.EntryType)
		}
		entry.Debit = 0
	case models.LedgerEntryTypeDebtApproval:
		// This is just an audit entry, no money changes
		entry.Debit = 0
		entry.Credit = 0
	}

	return s.LedgerRepo.Create(ctx, entry)
}

// CreateChargeEntry creates a CHARGE ledger entry (rent charged)
func (s *LedgerService) CreateChargeEntry(ctx context.Context, customerPhone, customerName, customerSO, description string, amount float64, referenceID *int, referenceType string, userID int, notes string) (*models.LedgerEntry, error) {
	entry := &models.CreateLedgerEntryRequest{
		CustomerPhone:   customerPhone,
		CustomerName:    customerName,
		CustomerSO:      customerSO,
		EntryType:       models.LedgerEntryTypeCharge,
		Description:     description,
		Debit:           amount,
		Credit:          0,
		ReferenceID:     referenceID,
		ReferenceType:   referenceType,
		CreatedByUserID: userID,
		Notes:           notes,
	}
	return s.LedgerRepo.Create(ctx, entry)
}

// CreatePaymentEntry creates a PAYMENT ledger entry (customer payment received)
func (s *LedgerService) CreatePaymentEntry(ctx context.Context, customerPhone, customerName, customerSO, description string, amount float64, referenceID *int, referenceType string, userID int, notes string) (*models.LedgerEntry, error) {
	entry := &models.CreateLedgerEntryRequest{
		CustomerPhone:   customerPhone,
		CustomerName:    customerName,
		CustomerSO:      customerSO,
		EntryType:       models.LedgerEntryTypePayment,
		Description:     description,
		Debit:           0,
		Credit:          amount,
		ReferenceID:     referenceID,
		ReferenceType:   referenceType,
		CreatedByUserID: userID,
		Notes:           notes,
	}
	return s.LedgerRepo.Create(ctx, entry)
}

// CreateCreditEntry creates a CREDIT ledger entry (discount/adjustment)
func (s *LedgerService) CreateCreditEntry(ctx context.Context, customerPhone, customerName, customerSO, description string, amount float64, userID int, notes string) (*models.LedgerEntry, error) {
	entry := &models.CreateLedgerEntryRequest{
		CustomerPhone:   customerPhone,
		CustomerName:    customerName,
		CustomerSO:      customerSO,
		EntryType:       models.LedgerEntryTypeCredit,
		Description:     description,
		Debit:           0,
		Credit:          amount,
		CreatedByUserID: userID,
		Notes:           notes,
	}
	return s.LedgerRepo.Create(ctx, entry)
}

// CreateRefundEntry creates a REFUND ledger entry (money returned to customer)
func (s *LedgerService) CreateRefundEntry(ctx context.Context, customerPhone, customerName, customerSO, description string, amount float64, userID int, notes string) (*models.LedgerEntry, error) {
	entry := &models.CreateLedgerEntryRequest{
		CustomerPhone:   customerPhone,
		CustomerName:    customerName,
		CustomerSO:      customerSO,
		EntryType:       models.LedgerEntryTypeRefund,
		Description:     description,
		Debit:           amount,
		Credit:          0,
		CreatedByUserID: userID,
		Notes:           notes,
	}
	return s.LedgerRepo.Create(ctx, entry)
}

// CreateDebtApprovalEntry creates a DEBT_APPROVAL ledger entry (audit record)
func (s *LedgerService) CreateDebtApprovalEntry(ctx context.Context, customerPhone, customerName, customerSO, description string, referenceID *int, userID int, notes string) (*models.LedgerEntry, error) {
	entry := &models.CreateLedgerEntryRequest{
		CustomerPhone:   customerPhone,
		CustomerName:    customerName,
		CustomerSO:      customerSO,
		EntryType:       models.LedgerEntryTypeDebtApproval,
		Description:     description,
		Debit:           0,
		Credit:          0,
		ReferenceID:     referenceID,
		ReferenceType:   "debt_request",
		CreatedByUserID: userID,
		Notes:           notes,
	}
	return s.LedgerRepo.Create(ctx, entry)
}

// GetBalance returns the current balance for a customer
func (s *LedgerService) GetBalance(ctx context.Context, customerPhone string) (float64, error) {
	return s.LedgerRepo.GetBalance(ctx, customerPhone)
}

// GetCustomerLedger returns all ledger entries for a customer
func (s *LedgerService) GetCustomerLedger(ctx context.Context, customerPhone string, limit, offset int) ([]models.LedgerEntry, error) {
	return s.LedgerRepo.GetByCustomer(ctx, customerPhone, limit, offset)
}

// GetAllEntries returns all ledger entries with optional filters (for audit)
func (s *LedgerService) GetAllEntries(ctx context.Context, filter *models.LedgerFilter) ([]models.LedgerEntry, error) {
	return s.LedgerRepo.GetAll(ctx, filter)
}

// GetCustomerSummary returns balance summary for a customer
func (s *LedgerService) GetCustomerSummary(ctx context.Context, customerPhone string) (*models.LedgerSummary, error) {
	return s.LedgerRepo.GetSummaryByCustomer(ctx, customerPhone)
}

// GetAllCustomerBalances returns balance summaries for all customers
func (s *LedgerService) GetAllCustomerBalances(ctx context.Context) ([]models.LedgerSummary, error) {
	return s.LedgerRepo.GetAllCustomerBalances(ctx)
}

// GetDebtors returns customers with positive balance (they owe money)
func (s *LedgerService) GetDebtors(ctx context.Context) ([]models.LedgerSummary, error) {
	return s.LedgerRepo.GetDebtors(ctx)
}

// GetTotalsByType returns sum of amounts by entry type
func (s *LedgerService) GetTotalsByType(ctx context.Context) (map[models.LedgerEntryType]float64, error) {
	return s.LedgerRepo.GetTotalsByType(ctx)
}

// HasOutstandingBalance checks if customer has outstanding balance
func (s *LedgerService) HasOutstandingBalance(ctx context.Context, customerPhone string) (bool, float64, error) {
	balance, err := s.LedgerRepo.GetBalance(ctx, customerPhone)
	if err != nil {
		return false, 0, err
	}
	return balance > 0, balance, nil
}
