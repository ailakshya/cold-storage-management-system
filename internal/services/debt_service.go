package services

import (
	"context"
	"fmt"

	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
)

type DebtService struct {
	DebtRepo      *repositories.DebtRequestRepository
	LedgerService *LedgerService
}

func NewDebtService(debtRepo *repositories.DebtRequestRepository, ledgerService *LedgerService) *DebtService {
	return &DebtService{
		DebtRepo:      debtRepo,
		LedgerService: ledgerService,
	}
}

// CreateRequest creates a new debt request
func (s *DebtService) CreateRequest(ctx context.Context, req *models.CreateDebtRequestRequest, requestedByUserID int, requestedByName string) (*models.DebtRequest, error) {
	// Validate that customer actually has outstanding balance
	hasBalance, balance, err := s.LedgerService.HasOutstandingBalance(ctx, req.CustomerPhone)
	if err != nil {
		// If ledger has no entries, check if balance was provided
		if req.CurrentBalance <= 0 {
			return nil, fmt.Errorf("customer has no outstanding balance")
		}
	} else if !hasBalance {
		return nil, fmt.Errorf("customer has no outstanding balance")
	} else {
		// Update the current balance from ledger
		req.CurrentBalance = balance
	}

	return s.DebtRepo.Create(ctx, req, requestedByUserID, requestedByName)
}

// GetByID returns a debt request by ID
func (s *DebtService) GetByID(ctx context.Context, id int) (*models.DebtRequest, error) {
	return s.DebtRepo.GetByID(ctx, id)
}

// GetPending returns all pending debt requests
func (s *DebtService) GetPending(ctx context.Context) ([]models.DebtRequest, error) {
	return s.DebtRepo.GetPending(ctx)
}

// GetByCustomer returns debt requests for a customer
func (s *DebtService) GetByCustomer(ctx context.Context, customerPhone string) ([]models.DebtRequest, error) {
	return s.DebtRepo.GetByCustomer(ctx, customerPhone)
}

// GetAll returns all debt requests with optional filters
func (s *DebtService) GetAll(ctx context.Context, filter *models.DebtRequestFilter) ([]models.DebtRequest, error) {
	return s.DebtRepo.GetAll(ctx, filter)
}

// Approve approves a debt request
func (s *DebtService) Approve(ctx context.Context, id int, approvedByUserID int, approvedByName string) error {
	// Get the request first to validate
	request, err := s.DebtRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get debt request: %w", err)
	}
	if request == nil {
		return fmt.Errorf("debt request not found")
	}
	if request.Status != models.DebtRequestStatusPending {
		return fmt.Errorf("debt request is not in pending status")
	}

	// Approve the request
	err = s.DebtRepo.Approve(ctx, id, approvedByUserID, approvedByName)
	if err != nil {
		return err
	}

	return nil
}

// Reject rejects a debt request
func (s *DebtService) Reject(ctx context.Context, id int, approvedByUserID int, approvedByName, reason string) error {
	// Get the request first to validate
	request, err := s.DebtRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get debt request: %w", err)
	}
	if request == nil {
		return fmt.Errorf("debt request not found")
	}
	if request.Status != models.DebtRequestStatusPending {
		return fmt.Errorf("debt request is not in pending status")
	}

	return s.DebtRepo.Reject(ctx, id, approvedByUserID, approvedByName, reason)
}

// UseApproval marks a debt approval as used and creates audit entry
func (s *DebtService) UseApproval(ctx context.Context, debtRequestID int, gatePassID int, userID int) error {
	// Get the request first
	request, err := s.DebtRepo.GetByID(ctx, debtRequestID)
	if err != nil {
		return fmt.Errorf("failed to get debt request: %w", err)
	}
	if request == nil {
		return fmt.Errorf("debt request not found")
	}
	if request.Status != models.DebtRequestStatusApproved {
		return fmt.Errorf("debt request is not in approved status")
	}

	// Mark as used
	err = s.DebtRepo.MarkAsUsed(ctx, debtRequestID, gatePassID)
	if err != nil {
		return err
	}

	// Create audit ledger entry
	description := fmt.Sprintf("Item out approved on credit - Gate Pass #%d, Thock: %s, Qty: %d",
		gatePassID, request.ThockNumber, request.RequestedQuantity)
	refID := debtRequestID
	_, err = s.LedgerService.CreateDebtApprovalEntry(ctx,
		request.CustomerPhone,
		request.CustomerName,
		request.CustomerSO,
		description,
		&refID,
		userID,
		fmt.Sprintf("Approved by: %s", request.ApprovedByName),
	)
	if err != nil {
		// Log but don't fail - the approval was already marked as used
		fmt.Printf("Warning: failed to create debt approval ledger entry: %v\n", err)
	}

	return nil
}

// GetApprovedForCustomerAndThock checks if there's an approved (unused) debt request for customer+thock
func (s *DebtService) GetApprovedForCustomerAndThock(ctx context.Context, customerPhone, thockNumber string) (*models.DebtRequest, error) {
	return s.DebtRepo.GetApprovedForCustomerAndThock(ctx, customerPhone, thockNumber)
}

// CanCreateGatePass checks if a gate pass can be created (either no balance or has debt approval)
func (s *DebtService) CanCreateGatePass(ctx context.Context, customerPhone, thockNumber string) (bool, *models.DebtRequest, float64, error) {
	// Check if customer has outstanding balance
	hasBalance, balance, err := s.LedgerService.HasOutstandingBalance(ctx, customerPhone)
	if err != nil {
		// No ledger entries = no balance
		return true, nil, 0, nil
	}

	if !hasBalance {
		// No outstanding balance, can create gate pass
		return true, nil, 0, nil
	}

	// Has balance - check for approved debt request
	debtReq, err := s.GetApprovedForCustomerAndThock(ctx, customerPhone, thockNumber)
	if err != nil {
		return false, nil, balance, err
	}

	if debtReq != nil {
		// Has approved debt request
		return true, debtReq, balance, nil
	}

	// Has balance but no approved debt request
	return false, nil, balance, nil
}

// GetPendingSummary returns summary of pending requests for dashboard
func (s *DebtService) GetPendingSummary(ctx context.Context) (*models.PendingDebtRequestSummary, error) {
	return s.DebtRepo.GetPendingSummary(ctx)
}

// ExpireOldRequests expires pending requests older than 24 hours
func (s *DebtService) ExpireOldRequests(ctx context.Context) error {
	return s.DebtRepo.ExpireOldRequests(ctx)
}
