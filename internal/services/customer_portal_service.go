package services

import (
	"context"
	"fmt"
	"strconv"

	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
)

type CustomerPortalService struct {
	CustomerRepo       *repositories.CustomerRepository
	EntryRepo          *repositories.EntryRepository
	RoomEntryRepo      *repositories.RoomEntryRepository
	GatePassRepo       *repositories.GatePassRepository
	RentPaymentRepo    *repositories.RentPaymentRepository
	SystemSettingRepo  *repositories.SystemSettingRepository
	GatePassPickupRepo *repositories.GatePassPickupRepository
}

func NewCustomerPortalService(
	customerRepo *repositories.CustomerRepository,
	entryRepo *repositories.EntryRepository,
	roomEntryRepo *repositories.RoomEntryRepository,
	gatePassRepo *repositories.GatePassRepository,
	rentPaymentRepo *repositories.RentPaymentRepository,
	systemSettingRepo *repositories.SystemSettingRepository,
	gatePassPickupRepo *repositories.GatePassPickupRepository,
) *CustomerPortalService {
	return &CustomerPortalService{
		CustomerRepo:       customerRepo,
		EntryRepo:          entryRepo,
		RoomEntryRepo:      roomEntryRepo,
		GatePassRepo:       gatePassRepo,
		RentPaymentRepo:    rentPaymentRepo,
		SystemSettingRepo:  systemSettingRepo,
		GatePassPickupRepo: gatePassPickupRepo,
	}
}

// ThockInfo represents dashboard data for a single truck
type ThockInfo struct {
	ThockNumber       string  `json:"thock_number"`
	EntryID           int     `json:"entry_id"`
	ExpectedQuantity  int     `json:"expected_quantity"`
	CurrentInventory  int     `json:"current_inventory"`
	TotalRent         float64 `json:"total_rent"`
	TotalPaid         float64 `json:"total_paid"`
	Balance           float64 `json:"balance"`
	CanTakeOut        int     `json:"can_take_out"`
	RentPerItem       float64 `json:"rent_per_item"`
}

// DashboardData represents the complete customer dashboard
type DashboardData struct {
	Customer    *models.Customer         `json:"customer"`
	Trucks      []ThockInfo              `json:"trucks"`
	GatePasses  []map[string]interface{} `json:"gate_passes"`
	TotalRent   float64                  `json:"total_rent"`
	TotalPaid   float64                  `json:"total_paid"`
	TotalBalance float64                 `json:"total_balance"`
}

// GetDashboardData returns all dashboard data for a customer
func (s *CustomerPortalService) GetDashboardData(ctx context.Context, customerID int) (*DashboardData, error) {
	// Get customer details
	customer, err := s.CustomerRepo.Get(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get customer: %w", err)
	}

	// Get rent_per_item setting from system settings
	var systemRentPerItem float64 = 0.0 // No hardcoded fallback - use API only
	if s.SystemSettingRepo != nil {
		rentSetting, err := s.SystemSettingRepo.Get(ctx, "rent_per_item")
		if err == nil && rentSetting != nil {
			if parsed, parseErr := strconv.ParseFloat(rentSetting.SettingValue, 64); parseErr == nil {
				systemRentPerItem = parsed
			}
		}
	}

	// Get all entries for this customer
	entries, err := s.EntryRepo.ListByCustomer(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get entries: %w", err)
	}

	var trucks []ThockInfo
	var totalRent, totalPaid, totalBalance float64

	// For each entry, calculate truck info
	for _, entry := range entries {
		// Get current inventory from room entries
		currentInventory, err := s.RoomEntryRepo.GetTotalQuantityByThockNumber(ctx, entry.ThockNumber)
		if err != nil {
			// If no room entries, inventory is 0
			currentInventory = 0
		}

		// Get pending quantity from gate passes (approved/pending but not yet picked up)
		pendingQty, err := s.GatePassRepo.GetPendingQuantityForEntry(ctx, entry.ID)
		if err != nil {
			pendingQty = 0
		}

		// Get rent payments for this entry
		payments, err := s.RentPaymentRepo.GetByEntryID(ctx, entry.ID)
		if err != nil {
			// If no payments, continue with 0 paid
			payments = []*models.RentPayment{}
		}

		// Calculate total paid (sum of amount_paid from all payments)
		var entryTotalPaid float64
		for _, payment := range payments {
			entryTotalPaid += payment.AmountPaid
		}

		// Calculate rent based on ORIGINAL STORED quantity
		// Use system rent_per_item setting
		rentPerItem := systemRentPerItem

		// Calculate original stored quantity for rent
		// Original = current inventory + all items already picked up
		// This ensures rent is based on what was originally stored, not current inventory
		storedQuantityForRent := currentInventory
		if s.GatePassPickupRepo != nil {
			// Get total pickups by thock number to calculate original stored quantity
			pickups, pickupErr := s.GatePassPickupRepo.GetPickupsByThockNumber(ctx, entry.ThockNumber)
			if pickupErr == nil {
				for _, p := range pickups {
					storedQuantityForRent += p.PickupQuantity
				}
			}
		}

		// Calculate total rent based on ORIGINAL stored quantity
		entryTotalRent := float64(storedQuantityForRent) * rentPerItem

		// Calculate balance (rent - paid)
		entryBalance := entryTotalRent - entryTotalPaid
		if entryBalance < 0 {
			entryBalance = 0
		}

		// Calculate effective available inventory (current - already committed in pending gate passes)
		effectiveInventory := currentInventory - pendingQty
		if effectiveInventory < 0 {
			effectiveInventory = 0
		}

		// Calculate how much customer can take out (considering pending gate passes)
		// canTakeOut = MIN(effectiveInventory, FLOOR(totalPaid / rentPerItem))
		canTakeOut := effectiveInventory
		if rentPerItem > 0 {
			maxAllowedByPayment := int(entryTotalPaid / rentPerItem)
			if maxAllowedByPayment < canTakeOut {
				canTakeOut = maxAllowedByPayment
			}
		}
		if canTakeOut < 0 {
			canTakeOut = 0
		}

		trucks = append(trucks, ThockInfo{
			ThockNumber:      entry.ThockNumber,
			EntryID:          entry.ID,
			ExpectedQuantity: entry.ExpectedQuantity,
			CurrentInventory: currentInventory,
			TotalRent:        entryTotalRent,
			TotalPaid:        entryTotalPaid,
			Balance:          entryBalance,
			CanTakeOut:       canTakeOut,
			RentPerItem:      rentPerItem,
		})

		totalRent += entryTotalRent
		totalPaid += entryTotalPaid
		totalBalance += entryBalance
	}

	// Get gate pass history
	gatePasses, err := s.GatePassRepo.ListByCustomerID(ctx, customerID)
	if err != nil {
		// If error, return empty list
		gatePasses = []map[string]interface{}{}
	}

	return &DashboardData{
		Customer:     customer,
		Trucks:       trucks,
		GatePasses:   gatePasses,
		TotalRent:    totalRent,
		TotalPaid:    totalPaid,
		TotalBalance: totalBalance,
	}, nil
}

// CreateGatePassRequest creates a gate pass request from customer portal
func (s *CustomerPortalService) CreateGatePassRequest(ctx context.Context, customerID int, request *models.CreateCustomerGatePassRequest) (*models.GatePass, error) {
	// Verify truck belongs to customer
	entry, err := s.EntryRepo.GetByThockNumber(ctx, request.ThockNumber)
	if err != nil {
		return nil, fmt.Errorf("truck not found")
	}

	if entry.CustomerID != customerID {
		return nil, fmt.Errorf("unauthorized: truck does not belong to customer")
	}

	// Check current inventory
	currentInventory, err := s.RoomEntryRepo.GetTotalQuantityByThockNumber(ctx, request.ThockNumber)
	if err != nil {
		currentInventory = 0
	}

	// Get pending quantity from existing gate passes
	pendingQty, err := s.GatePassRepo.GetPendingQuantityForEntry(ctx, entry.ID)
	if err != nil {
		pendingQty = 0
	}

	// Calculate effective available inventory
	effectiveInventory := currentInventory - pendingQty
	if effectiveInventory < 0 {
		effectiveInventory = 0
	}

	if effectiveInventory < request.RequestedQuantity {
		return nil, fmt.Errorf("insufficient inventory: requested %d, available %d (current: %d, pending in gate passes: %d)",
			request.RequestedQuantity, effectiveInventory, currentInventory, pendingQty)
	}

	// Get rent payments to check payment allowance
	payments, err := s.RentPaymentRepo.GetByEntryID(ctx, entry.ID)
	if err != nil {
		payments = []*models.RentPayment{}
	}

	// Calculate total paid
	var totalPaid float64
	var totalRent float64
	for _, payment := range payments {
		totalPaid += payment.AmountPaid
		if payment.TotalRent > 0 {
			totalRent = payment.TotalRent
		}
	}

	// Calculate rent per item
	var rentPerItem float64
	if entry.ExpectedQuantity > 0 && totalRent > 0 {
		rentPerItem = totalRent / float64(entry.ExpectedQuantity)
	}

	// Check if customer has paid enough
	if rentPerItem > 0 {
		maxAllowed := int(totalPaid / rentPerItem)
		if request.RequestedQuantity > maxAllowed {
			return nil, fmt.Errorf("payment insufficient: you can take out max %d items based on your payment (requested: %d)", maxAllowed, request.RequestedQuantity)
		}
	}

	// Create gate pass
	gatePass, err := s.GatePassRepo.CreateCustomerGatePass(
		ctx,
		customerID,
		request.ThockNumber,
		request.RequestedQuantity,
		request.Remarks,
		entry.ID,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create gate pass: %w", err)
	}

	return gatePass, nil
}

// GetTrucksByCustomerID returns list of truck numbers for a customer
func (s *CustomerPortalService) GetTrucksByCustomerID(ctx context.Context, customerID int) ([]string, error) {
	entries, err := s.EntryRepo.ListByCustomer(ctx, customerID)
	if err != nil {
		return nil, err
	}

	trucks := make([]string, 0, len(entries))
	for _, entry := range entries {
		trucks = append(trucks, entry.ThockNumber)
	}

	return trucks, nil
}
