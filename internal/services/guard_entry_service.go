package services

import (
	"context"
	"errors"
	"regexp"

	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
)

type GuardEntryService struct {
	GuardEntryRepo *repositories.GuardEntryRepository
}

func NewGuardEntryService(repo *repositories.GuardEntryRepository) *GuardEntryService {
	return &GuardEntryService{GuardEntryRepo: repo}
}

// CreateGuardEntry creates a new guard entry with validation
func (s *GuardEntryService) CreateGuardEntry(ctx context.Context, req *models.CreateGuardEntryRequest, userID int) (*models.GuardEntry, error) {
	// Validate customer name
	if req.CustomerName == "" {
		return nil, errors.New("customer name is required")
	}

	// Validate village
	if req.Village == "" {
		return nil, errors.New("village is required")
	}

	// Validate mobile (10 digits)
	mobileRegex := regexp.MustCompile(`^[0-9]{10}$`)
	if !mobileRegex.MatchString(req.Mobile) {
		return nil, errors.New("mobile must be exactly 10 digits")
	}

	// Validate at least one quantity is provided
	if req.SeedQuantity <= 0 && req.SellQuantity <= 0 {
		return nil, errors.New("at least one quantity (seed or sell) must be greater than 0")
	}

	// Validate driver_no if provided (must be 10 digits or empty)
	if req.DriverNo != "" && !mobileRegex.MatchString(req.DriverNo) {
		return nil, errors.New("driver number must be exactly 10 digits")
	}

	entry := &models.GuardEntry{
		CustomerName:    req.CustomerName,
		SO:              req.SO,
		Village:         req.Village,
		Mobile:          req.Mobile,
		DriverNo:        req.DriverNo,
		SeedQuantity:    req.SeedQuantity,
		SellQuantity:    req.SellQuantity,
		Remarks:         req.Remarks,
		CreatedByUserID: userID,
	}

	if err := s.GuardEntryRepo.Create(ctx, entry); err != nil {
		return nil, err
	}

	return entry, nil
}

// GetGuardEntry retrieves a guard entry by ID
func (s *GuardEntryService) GetGuardEntry(ctx context.Context, id int) (*models.GuardEntry, error) {
	return s.GuardEntryRepo.Get(ctx, id)
}

// ListTodayByUser returns today's entries for a specific guard
func (s *GuardEntryService) ListTodayByUser(ctx context.Context, userID int) ([]*models.GuardEntry, error) {
	return s.GuardEntryRepo.ListTodayByUser(ctx, userID)
}

// ListPending returns all pending guard entries
func (s *GuardEntryService) ListPending(ctx context.Context) ([]*models.GuardEntry, error) {
	return s.GuardEntryRepo.ListPending(ctx)
}

// MarkAsProcessed marks a guard entry as processed
func (s *GuardEntryService) MarkAsProcessed(ctx context.Context, id int, processedByUserID int) error {
	// Verify entry exists and is pending
	entry, err := s.GuardEntryRepo.Get(ctx, id)
	if err != nil {
		return errors.New("guard entry not found")
	}

	if entry.Status != "pending" {
		return errors.New("guard entry is already processed")
	}

	return s.GuardEntryRepo.MarkAsProcessed(ctx, id, processedByUserID)
}

// GetTodayCountByUser returns today's entry count for a guard
func (s *GuardEntryService) GetTodayCountByUser(ctx context.Context, userID int) (int, int, error) {
	return s.GuardEntryRepo.GetTodayCountByUser(ctx, userID)
}
