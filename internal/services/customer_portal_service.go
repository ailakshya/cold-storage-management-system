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
	FamilyMemberName  string  `json:"family_member_name,omitempty"`
	ThockCategory     string  `json:"thock_category"`
	StoredQuantity    int     `json:"stored_quantity"`
	ExpectedQuantity  int     `json:"expected_quantity"`
	CurrentInventory  int     `json:"current_inventory"`
	TotalRent         float64 `json:"total_rent"`
	TotalPaid         float64 `json:"total_paid"`
	Balance           float64 `json:"balance"`
	CanTakeOut        int     `json:"can_take_out"`
	RentPerItem       float64 `json:"rent_per_item"`
}

// PaymentInfo represents a payment for the customer dashboard
type PaymentInfo struct {
	ID          int     `json:"id"`
	Amount      float64 `json:"amount"`
	PaymentDate string  `json:"payment_date"`
	ThockNumber string  `json:"thock_number,omitempty"`
	CreatedAt   string  `json:"created_at"`
}

// DashboardData represents the complete customer dashboard
type DashboardData struct {
	Customer     *models.Customer         `json:"customer"`
	Trucks       []ThockInfo              `json:"trucks"`
	GatePasses   []map[string]interface{} `json:"gate_passes"`
	Payments     []PaymentInfo            `json:"payments"`
	TotalRent    float64                  `json:"total_rent"`
	TotalPaid    float64                  `json:"total_paid"`
	TotalBalance float64                  `json:"total_balance"`
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
		// Get ORIGINAL entered quantity from room entries
		// room_entries.quantity stores what was originally entered (never reduced on pickup)
		originalEntered, err := s.RoomEntryRepo.GetTotalQuantityByThockNumber(ctx, entry.ThockNumber)
		if err != nil {
			// If no room entries, quantity is 0
			originalEntered = 0
		}

		// Get total picked up quantity
		var totalPickedUp int
		if s.GatePassPickupRepo != nil {
			pickups, pickupErr := s.GatePassPickupRepo.GetPickupsByThockNumber(ctx, entry.ThockNumber)
			if pickupErr == nil {
				for _, p := range pickups {
					totalPickedUp += p.PickupQuantity
				}
			}
		}

		// Calculate CURRENT inventory = original entered - picked up
		currentInventory := originalEntered - totalPickedUp
		if currentInventory < 0 {
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

		// Calculate rent based on ORIGINAL ENTERED quantity (never changes)
		// Use system rent_per_item setting
		rentPerItem := systemRentPerItem

		// Rent = original entered × rent per item (constant, doesn't change with pickups)
		entryTotalRent := float64(originalEntered) * rentPerItem

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
			ThockNumber:       entry.ThockNumber,
			EntryID:           entry.ID,
			FamilyMemberName:  entry.FamilyMemberName,
			ThockCategory:     entry.ThockCategory,
			StoredQuantity:    originalEntered,
			ExpectedQuantity:  entry.ExpectedQuantity,
			CurrentInventory:  currentInventory,
			TotalRent:         entryTotalRent,
			TotalPaid:         entryTotalPaid,
			Balance:           entryBalance,
			CanTakeOut:        canTakeOut,
			RentPerItem:       rentPerItem,
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

	// Get recent payments for this customer
	var payments []PaymentInfo
	if s.RentPaymentRepo != nil {
		rentPayments, err := s.RentPaymentRepo.GetByPhone(ctx, customer.Phone)
		if err == nil && rentPayments != nil {
			// Create a map of entry_id to thock_number for lookup
			entryThockMap := make(map[int]string)
			for _, truck := range trucks {
				// Find entry by thock number
				entry, _ := s.EntryRepo.GetByThockNumber(ctx, truck.ThockNumber)
				if entry != nil {
					entryThockMap[entry.ID] = truck.ThockNumber
				}
			}

			for _, rp := range rentPayments {
				thockNumber := ""
				if rp.EntryID > 0 {
					thockNumber = entryThockMap[rp.EntryID]
				}
				payments = append(payments, PaymentInfo{
					ID:          rp.ID,
					Amount:      rp.AmountPaid,
					PaymentDate: rp.PaymentDate.Format("2006-01-02"),
					ThockNumber: thockNumber,
					CreatedAt:   rp.CreatedAt.Format("2006-01-02T15:04:05Z"),
				})
			}
		}
	}

	return &DashboardData{
		Customer:     customer,
		Trucks:       trucks,
		GatePasses:   gatePasses,
		Payments:     payments,
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

	// Get ORIGINAL entered quantity from room entries
	originalEntered, err := s.RoomEntryRepo.GetTotalQuantityByThockNumber(ctx, request.ThockNumber)
	if err != nil {
		originalEntered = 0
	}

	// Get total picked up quantity
	var totalPickedUp int
	if s.GatePassPickupRepo != nil {
		pickups, pickupErr := s.GatePassPickupRepo.GetPickupsByThockNumber(ctx, request.ThockNumber)
		if pickupErr == nil {
			for _, p := range pickups {
				totalPickedUp += p.PickupQuantity
			}
		}
	}

	// Calculate CURRENT inventory = original - picked up
	currentInventory := originalEntered - totalPickedUp
	if currentInventory < 0 {
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
		request.FamilyMemberID,
		request.FamilyMemberName,
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
