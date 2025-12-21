package services

import (
	"context"
	"errors"

	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
)

type EntryService struct {
	EntryRepo      *repositories.EntryRepository
	CustomerRepo   *repositories.CustomerRepository
	EntryEventRepo *repositories.EntryEventRepository
}

func NewEntryService(entryRepo *repositories.EntryRepository, customerRepo *repositories.CustomerRepository, entryEventRepo *repositories.EntryEventRepository) *EntryService {
	return &EntryService{
		EntryRepo:      entryRepo,
		CustomerRepo:   customerRepo,
		EntryEventRepo: entryEventRepo,
	}
}

func (s *EntryService) CreateEntry(ctx context.Context, req *models.CreateEntryRequest, userID int) (*models.Entry, error) {
	// Validate quantity
	if req.ExpectedQuantity < 1 {
		return nil, errors.New("expected quantity must be at least 1")
	}

	// Validate category
	if req.ThockCategory != "seed" && req.ThockCategory != "sell" {
		return nil, errors.New("thock category must be 'seed' or 'sell'")
	}

	// Validate phone number (must be exactly 10 digits)
	if len(req.Phone) != 10 {
		return nil, errors.New("phone number must be exactly 10 digits")
	}

	// Find or create customer
	var customer *models.Customer

	// Try to get customer by ID if provided
	if req.CustomerID > 0 {
		customer, _ = s.CustomerRepo.Get(ctx, req.CustomerID)
	}

	// If customer not found by ID, try to find by phone
	if customer == nil {
		var err error
		customer, err = s.CustomerRepo.GetByPhone(ctx, req.Phone)
		if err != nil {
			customer = nil  // Make sure it's nil on error
		}
	}

	// If still not found, create new customer
	if customer == nil {
		customer = &models.Customer{
			Name:    req.Name,
			Phone:   req.Phone,
			Village: req.Village,
			SO:      req.SO,
		}
		if err := s.CustomerRepo.Create(ctx, customer); err != nil {
			return nil, errors.New("failed to create customer: " + err.Error())
		}
	} else {
		// Update existing customer's S/O if provided and different
		if req.SO != "" && req.SO != customer.SO {
			customer.SO = req.SO
			customer.Name = req.Name
			customer.Village = req.Village
			s.CustomerRepo.Update(ctx, customer)
		}
	}

	// Create entry with denormalized customer data for historical record
	entry := &models.Entry{
		CustomerID:       customer.ID,
		Phone:            req.Phone,
		Name:             req.Name,
		Village:          req.Village,
		SO:               req.SO,
		ExpectedQuantity: req.ExpectedQuantity,
		ThockCategory:    req.ThockCategory,
		Remark:           req.Remark,
		CreatedByUserID:  userID,
	}

	if err := s.EntryRepo.Create(ctx, entry); err != nil {
		return nil, err
	}

	// Automatically create initial status event
	event := &models.EntryEvent{
		EntryID:         entry.ID,
		EventType:       models.EventTypeCreated,
		Status:          models.StatusPending,
		Notes:           "Entry created - awaiting storage",
		CreatedByUserID: userID,
	}

	if err := s.EntryEventRepo.Create(ctx, event); err != nil {
		// Log error but don't fail the entry creation
		// The entry was created successfully even if event creation failed
		return entry, nil
	}

	return entry, nil
}

func (s *EntryService) GetEntry(ctx context.Context, id int) (*models.Entry, error) {
	return s.EntryRepo.Get(ctx, id)
}

func (s *EntryService) ListEntries(ctx context.Context) ([]*models.Entry, error) {
	return s.EntryRepo.List(ctx)
}

func (s *EntryService) ListEntriesByCustomer(ctx context.Context, customerID int) ([]*models.Entry, error) {
	return s.EntryRepo.ListByCustomer(ctx, customerID)
}

func (s *EntryService) GetCountByCategory(ctx context.Context, category string) (int, error) {
	// Validate category
	if category != "seed" && category != "sell" {
		return 0, errors.New("category must be 'seed' or 'sell'")
	}
	return s.EntryRepo.GetCountByCategory(ctx, category)
}

func (s *EntryService) UpdateEntry(ctx context.Context, id int, req *models.UpdateEntryRequest) error {
	// Get existing entry
	entry, err := s.EntryRepo.Get(ctx, id)
	if err != nil {
		return errors.New("entry not found")
	}

	// Validate phone number
	if len(req.Phone) != 10 {
		return errors.New("phone number must be exactly 10 digits")
	}

	// Update fields
	entry.Name = req.Name
	entry.Phone = req.Phone
	entry.Village = req.Village
	entry.SO = req.SO
	entry.ExpectedQuantity = req.ExpectedQuantity
	entry.Remark = req.Remark

	return s.EntryRepo.Update(ctx, entry)
}
