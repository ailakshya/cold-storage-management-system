package services

import (
	"context"
	"encoding/json"
	"errors"

	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
)

// SkipRange represents a range of thock numbers to skip
type SkipRange struct {
	From int `json:"from"`
	To   int `json:"to"`
}

type EntryService struct {
	EntryRepo        *repositories.EntryRepository
	CustomerRepo     *repositories.CustomerRepository
	EntryEventRepo   *repositories.EntryEventRepository
	SettingRepo      *repositories.SystemSettingRepository
	FamilyMemberRepo *repositories.FamilyMemberRepository
}

func NewEntryService(entryRepo *repositories.EntryRepository, customerRepo *repositories.CustomerRepository, entryEventRepo *repositories.EntryEventRepository) *EntryService {
	return &EntryService{
		EntryRepo:      entryRepo,
		CustomerRepo:   customerRepo,
		EntryEventRepo: entryEventRepo,
	}
}

// SetFamilyMemberRepo sets the FamilyMemberRepository for family member handling
func (s *EntryService) SetFamilyMemberRepo(repo *repositories.FamilyMemberRepository) {
	s.FamilyMemberRepo = repo
}

// SetSettingRepo sets the SystemSettingRepository for skip range calculation
func (s *EntryService) SetSettingRepo(repo *repositories.SystemSettingRepository) {
	s.SettingRepo = repo
}

// getSkipRanges retrieves skip ranges from settings for a given category
func (s *EntryService) getSkipRanges(ctx context.Context, category string) []SkipRange {
	if s.SettingRepo == nil {
		return nil
	}

	key := "skip_thock_ranges_" + category
	setting, err := s.SettingRepo.Get(ctx, key)
	if err != nil || setting == nil || setting.SettingValue == "" {
		return nil
	}

	var ranges []SkipRange
	if err := json.Unmarshal([]byte(setting.SettingValue), &ranges); err != nil {
		return nil
	}
	return ranges
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

	// Get skip ranges for the category
	skipRanges := s.getSkipRanges(ctx, req.ThockCategory)

	// Handle family member assignment
	var familyMemberID *int
	var familyMemberName string

	if s.FamilyMemberRepo != nil {
		// If family_member_id is provided, use it
		if req.FamilyMemberID != nil && *req.FamilyMemberID > 0 {
			fm, err := s.FamilyMemberRepo.Get(ctx, *req.FamilyMemberID)
			if err == nil {
				familyMemberID = &fm.ID
				familyMemberName = fm.Name
			}
		}

		// If no family member specified, get or create one based on entry name
		if familyMemberID == nil {
			fm, err := s.FamilyMemberRepo.GetOrCreateByName(ctx, customer.ID, req.Name, customer.Name)
			if err == nil && fm != nil {
				familyMemberID = &fm.ID
				familyMemberName = fm.Name
			}
		}
	}

	// Create entry with denormalized customer data for historical record
	entry := &models.Entry{
		CustomerID:       customer.ID,
		FamilyMemberID:   familyMemberID,
		FamilyMemberName: familyMemberName,
		Phone:            req.Phone,
		Name:             req.Name,
		Village:          req.Village,
		SO:               req.SO,
		ExpectedQuantity: req.ExpectedQuantity,
		ThockCategory:    req.ThockCategory,
		Remark:           req.Remark,
		CreatedByUserID:  userID,
	}

	// Convert SkipRange to repositories.SkipRange
	var repoSkipRanges []repositories.SkipRange
	for _, r := range skipRanges {
		repoSkipRanges = append(repoSkipRanges, repositories.SkipRange{From: r.From, To: r.To})
	}

	if err := s.EntryRepo.CreateWithSkipRanges(ctx, entry, repoSkipRanges); err != nil {
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

// GetMaxThockNumber returns the highest thock number for a category
func (s *EntryService) GetMaxThockNumber(ctx context.Context, category string) (int, error) {
	if category != "seed" && category != "sell" {
		return 0, errors.New("category must be 'seed' or 'sell'")
	}
	return s.EntryRepo.GetMaxThockNumber(ctx, category)
}

// ReassignEntry reassigns an entry to a different customer (optionally to a specific family member)
func (s *EntryService) ReassignEntry(ctx context.Context, entryID int, newCustomerID int, familyMemberID *int, familyMemberName string) (*models.Entry, *models.Customer, error) {
	// Get the new customer
	newCustomer, err := s.CustomerRepo.Get(ctx, newCustomerID)
	if err != nil {
		return nil, nil, errors.New("new customer not found")
	}

	// If no family member specified, use customer name
	if familyMemberName == "" {
		familyMemberName = newCustomer.Name
	}

	// Update entry with new customer details
	err = s.EntryRepo.ReassignCustomer(ctx, entryID, newCustomerID, newCustomer.Name, newCustomer.Phone, newCustomer.Village, newCustomer.SO, familyMemberID, familyMemberName)
	if err != nil {
		return nil, nil, errors.New("failed to reassign entry: " + err.Error())
	}

	// Get updated entry
	entry, err := s.EntryRepo.Get(ctx, entryID)
	if err != nil {
		return nil, newCustomer, nil
	}

	return entry, newCustomer, nil
}

func (s *EntryService) UpdateEntry(ctx context.Context, id int, req *models.UpdateEntryRequest) error {
	// Get existing entry
	entry, err := s.EntryRepo.Get(ctx, id)
	if err != nil {
		return errors.New("entry not found")
	}

	// Save old values for thock number recalculation
	oldCategory := entry.ThockCategory
	oldQty := entry.ExpectedQuantity

	// Validate phone number
	if len(req.Phone) != 10 {
		return errors.New("phone number must be exactly 10 digits")
	}

	// Validate category if provided
	if req.ThockCategory != "" && req.ThockCategory != "seed" && req.ThockCategory != "sell" {
		return errors.New("thock category must be 'seed' or 'sell'")
	}

	// Update fields
	entry.Name = req.Name
	entry.Phone = req.Phone
	entry.Village = req.Village
	entry.SO = req.SO
	entry.ExpectedQuantity = req.ExpectedQuantity
	entry.Remark = req.Remark
	if req.ThockCategory != "" {
		entry.ThockCategory = req.ThockCategory
	}

	return s.EntryRepo.Update(ctx, entry, oldCategory, oldQty)
}
