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
	LedgerRepo         *repositories.LedgerRepository
}

func NewCustomerPortalService(
	customerRepo *repositories.CustomerRepository,
	entryRepo *repositories.EntryRepository,
	roomEntryRepo *repositories.RoomEntryRepository,
	gatePassRepo *repositories.GatePassRepository,
	rentPaymentRepo *repositories.RentPaymentRepository,
	systemSettingRepo *repositories.SystemSettingRepository,
	gatePassPickupRepo *repositories.GatePassPickupRepository,
	ledgerRepo *repositories.LedgerRepository,
) *CustomerPortalService {
	return &CustomerPortalService{
		CustomerRepo:       customerRepo,
		EntryRepo:          entryRepo,
		RoomEntryRepo:      roomEntryRepo,
		GatePassRepo:       gatePassRepo,
		RentPaymentRepo:    rentPaymentRepo,
		SystemSettingRepo:  systemSettingRepo,
		GatePassPickupRepo: gatePassPickupRepo,
		LedgerRepo:         ledgerRepo,
	}
}

// ThockInfo represents dashboard data for a single truck
type ThockInfo struct {
	ThockNumber      string  `json:"thock_number"`
	EntryID          int     `json:"entry_id"`
	FamilyMemberID   *int    `json:"family_member_id,omitempty"`
	FamilyMemberName string  `json:"family_member_name,omitempty"`
	ThockCategory    string  `json:"thock_category"`
	StoredQuantity   int     `json:"stored_quantity"`
	ExpectedQuantity int     `json:"expected_quantity"`
	CurrentInventory int     `json:"current_inventory"`
	TotalRent        float64 `json:"total_rent"`
	CanTakeOut       int     `json:"can_take_out"`
	RentPerItem      float64 `json:"rent_per_item"`
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

	// Get all payments for this customer to calculate per-family-member totals
	allPayments, _ := s.RentPaymentRepo.GetByPhone(ctx, customer.Phone)

	// Calculate total paid per family member (use 0 for entries with no family member)
	familyMemberPaid := make(map[int]float64)
	for _, payment := range allPayments {
		fmID := 0
		if payment.FamilyMemberID != nil {
			fmID = *payment.FamilyMemberID
		}
		familyMemberPaid[fmID] += payment.AmountPaid
	}

	// Add online payments from ledger (attributed to family_member_id = 0 since online payments don't track family members)
	if s.LedgerRepo != nil {
		ledgerPayments, err := s.LedgerRepo.GetPaymentHistory(ctx, customer.Phone, 1000)
		if err == nil {
			for _, lp := range ledgerPayments {
				// Only add ONLINE_PAYMENT entries to avoid double counting with rent_payments
				if lp.EntryType == "ONLINE_PAYMENT" {
					familyMemberPaid[0] += lp.Amount // 0 = customer level (no family member)
				}
			}
		}
	}

	// Calculate total picked up per family member
	familyMemberPickedUp := make(map[int]int)
	for _, entry := range entries {
		fmID := 0
		if entry.FamilyMemberID != nil {
			fmID = *entry.FamilyMemberID
		}

		if s.GatePassPickupRepo != nil {
			pickups, pickupErr := s.GatePassPickupRepo.GetPickupsByThockNumber(ctx, entry.ThockNumber)
			if pickupErr == nil {
				for _, p := range pickups {
					familyMemberPickedUp[fmID] += p.PickupQuantity
				}
			}
		}
	}

	var trucks []ThockInfo
	var totalRent, totalPaid, totalBalance float64

	// For each entry, calculate truck info
	rentPerItem := systemRentPerItem

	for _, entry := range entries {
		// Get family member ID (0 if not assigned)
		fmID := 0
		if entry.FamilyMemberID != nil {
			fmID = *entry.FamilyMemberID
		}

		// Get ORIGINAL entered quantity from room entries
		originalEntered, err := s.RoomEntryRepo.GetTotalQuantityByThockNumber(ctx, entry.ThockNumber)
		if err != nil {
			originalEntered = 0
		}

		// Get total picked up quantity for THIS thock
		var thockPickedUp int
		if s.GatePassPickupRepo != nil {
			pickups, pickupErr := s.GatePassPickupRepo.GetPickupsByThockNumber(ctx, entry.ThockNumber)
			if pickupErr == nil {
				for _, p := range pickups {
					thockPickedUp += p.PickupQuantity
				}
			}
		}

		// Calculate CURRENT inventory = original entered - picked up
		currentInventory := originalEntered - thockPickedUp
		if currentInventory < 0 {
			currentInventory = 0
		}

		// Get pending quantity from gate passes
		pendingQty, _ := s.GatePassRepo.GetPendingQuantityForEntry(ctx, entry.ID)

		// Calculate rent for this entry
		entryTotalRent := float64(originalEntered) * rentPerItem

		// Calculate effective available inventory
		effectiveInventory := currentInventory - pendingQty
		if effectiveInventory < 0 {
			effectiveInventory = 0
		}

		// Calculate canTakeOut based on FAMILY MEMBER's total paid and picked up
		fmTotalPaid := familyMemberPaid[fmID]
		// Also include general/online payments (fmID=0) if this is a specific family member
		// Online payments don't track family members, so they apply to entire account
		if fmID != 0 {
			fmTotalPaid += familyMemberPaid[0]
		}
		fmTotalPickedUp := familyMemberPickedUp[fmID]
		// Also include pickups from general account
		if fmID != 0 {
			fmTotalPickedUp += familyMemberPickedUp[0]
		}

		canTakeOut := effectiveInventory
		if rentPerItem > 0 {
			// Items paid for by this family member (including general/online payments)
			itemsPaidFor := int(fmTotalPaid / rentPerItem)
			// Remaining allowance = items paid for - already picked up by family member
			remainingAllowance := itemsPaidFor - fmTotalPickedUp
			if remainingAllowance < 0 {
				remainingAllowance = 0
			}
			if remainingAllowance < canTakeOut {
				canTakeOut = remainingAllowance
			}
		}
		if canTakeOut < 0 {
			canTakeOut = 0
		}

		trucks = append(trucks, ThockInfo{
			ThockNumber:      entry.ThockNumber,
			EntryID:          entry.ID,
			FamilyMemberID:   entry.FamilyMemberID,
			FamilyMemberName: entry.FamilyMemberName,
			ThockCategory:    entry.ThockCategory,
			StoredQuantity:   originalEntered,
			ExpectedQuantity: entry.ExpectedQuantity,
			CurrentInventory: currentInventory,
			TotalRent:        entryTotalRent,
			CanTakeOut:       canTakeOut,
			RentPerItem:      rentPerItem,
		})

		totalRent += entryTotalRent
	}

	// Calculate total paid from ledger entries (includes both manual + online payments)
	if s.LedgerRepo != nil {
		ledgerPaid, err := s.LedgerRepo.GetTotalCredit(ctx, customer.Phone)
		if err == nil {
			totalPaid = ledgerPaid
		}
	} else {
		// Fallback to rent payments if ledger not available
		for _, paid := range familyMemberPaid {
			totalPaid += paid
		}
	}
	totalBalance = totalRent - totalPaid
	if totalBalance < 0 {
		totalBalance = 0
	}

	// Get gate pass history
	gatePasses, err := s.GatePassRepo.ListByCustomerID(ctx, customerID)
	if err != nil {
		// If error, return empty list
		gatePasses = []map[string]interface{}{}
	}

	// Get recent payments from ledger (includes both manual + online payments)
	var payments []PaymentInfo
	if s.LedgerRepo != nil {
		ledgerPayments, err := s.LedgerRepo.GetPaymentHistory(ctx, customer.Phone, 20)
		if err == nil {
			for _, lp := range ledgerPayments {
				payments = append(payments, PaymentInfo{
					ID:          lp.ID,
					Amount:      lp.Amount,
					PaymentDate: lp.CreatedAt.Format("2006-01-02"),
					ThockNumber: lp.Description, // Description contains thock info
					CreatedAt:   lp.CreatedAt.Format("2006-01-02T15:04:05Z"),
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

	// Get customer details for phone lookup
	customer, err := s.CustomerRepo.Get(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("customer not found")
	}

	// Get rent_per_item setting
	var rentPerItem float64 = 0.0
	if s.SystemSettingRepo != nil {
		rentSetting, err := s.SystemSettingRepo.Get(ctx, "rent_per_item")
		if err == nil && rentSetting != nil {
			if parsed, parseErr := strconv.ParseFloat(rentSetting.SettingValue, 64); parseErr == nil {
				rentPerItem = parsed
			}
		}
	}

	// Get ORIGINAL entered quantity from room entries
	originalEntered, err := s.RoomEntryRepo.GetTotalQuantityByThockNumber(ctx, request.ThockNumber)
	if err != nil {
		originalEntered = 0
	}

	// Get total picked up quantity for this thock
	var thockPickedUp int
	if s.GatePassPickupRepo != nil {
		pickups, pickupErr := s.GatePassPickupRepo.GetPickupsByThockNumber(ctx, request.ThockNumber)
		if pickupErr == nil {
			for _, p := range pickups {
				thockPickedUp += p.PickupQuantity
			}
		}
	}

	// Calculate CURRENT inventory = original - picked up
	currentInventory := originalEntered - thockPickedUp
	if currentInventory < 0 {
		currentInventory = 0
	}

	// Get pending quantity from existing gate passes
	pendingQty, _ := s.GatePassRepo.GetPendingQuantityForEntry(ctx, entry.ID)

	// Calculate effective available inventory
	effectiveInventory := currentInventory - pendingQty
	if effectiveInventory < 0 {
		effectiveInventory = 0
	}

	if effectiveInventory < request.RequestedQuantity {
		return nil, fmt.Errorf("insufficient inventory: requested %d, available %d (current: %d, pending in gate passes: %d)",
			request.RequestedQuantity, effectiveInventory, currentInventory, pendingQty)
	}

	// Calculate family member's payment allowance
	fmID := 0
	if entry.FamilyMemberID != nil {
		fmID = *entry.FamilyMemberID
	}

	// Get all payments for this customer
	allPayments, _ := s.RentPaymentRepo.GetByPhone(ctx, customer.Phone)

	// Calculate total paid by this family member
	var fmTotalPaid float64
	var generalPaid float64 // Online payments (fmID=0)
	for _, payment := range allPayments {
		paymentFmID := 0
		if payment.FamilyMemberID != nil {
			paymentFmID = *payment.FamilyMemberID
		}
		if paymentFmID == fmID {
			fmTotalPaid += payment.AmountPaid
		}
		if paymentFmID == 0 {
			generalPaid += payment.AmountPaid
		}
	}

	// Add online payments from ledger (they don't have family member info, so they're general)
	if s.LedgerRepo != nil {
		ledgerPayments, err := s.LedgerRepo.GetPaymentHistory(ctx, customer.Phone, 1000)
		if err == nil {
			for _, lp := range ledgerPayments {
				if lp.EntryType == "ONLINE_PAYMENT" {
					generalPaid += lp.Amount
				}
			}
		}
	}

	// Include general/online payments for family members
	if fmID != 0 {
		fmTotalPaid += generalPaid
	} else {
		// For entries without family member, generalPaid is already in fmTotalPaid
		// but we need to add online payments separately
		if s.LedgerRepo != nil {
			ledgerPayments, err := s.LedgerRepo.GetPaymentHistory(ctx, customer.Phone, 1000)
			if err == nil {
				for _, lp := range ledgerPayments {
					if lp.EntryType == "ONLINE_PAYMENT" {
						fmTotalPaid += lp.Amount
					}
				}
			}
		}
	}

	// Calculate total picked up by this family member (across all their thocks)
	entries, _ := s.EntryRepo.ListByCustomer(ctx, customerID)
	var fmTotalPickedUp int
	var generalPickedUp int
	for _, e := range entries {
		entryFmID := 0
		if e.FamilyMemberID != nil {
			entryFmID = *e.FamilyMemberID
		}
		if s.GatePassPickupRepo != nil {
			pickups, pickupErr := s.GatePassPickupRepo.GetPickupsByThockNumber(ctx, e.ThockNumber)
			if pickupErr == nil {
				for _, p := range pickups {
					if entryFmID == fmID {
						fmTotalPickedUp += p.PickupQuantity
					}
					if entryFmID == 0 {
						generalPickedUp += p.PickupQuantity
					}
				}
			}
		}
	}
	// Include general pickups for family members (since general payments cover all)
	if fmID != 0 {
		fmTotalPickedUp += generalPickedUp
	}

	// Check if family member has paid enough
	if rentPerItem > 0 {
		itemsPaidFor := int(fmTotalPaid / rentPerItem)
		remainingAllowance := itemsPaidFor - fmTotalPickedUp
		if remainingAllowance < 0 {
			remainingAllowance = 0
		}
		if request.RequestedQuantity > remainingAllowance {
			return nil, fmt.Errorf("payment insufficient: you can take out max %d items based on your payment (requested: %d)", remainingAllowance, request.RequestedQuantity)
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
