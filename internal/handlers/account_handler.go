package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"cold-backend/internal/cache"
	"cold-backend/internal/middleware"
	"cold-backend/internal/models"
	"cold-backend/internal/repositories"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Cache configuration for account handler
const (
	accountCacheKey = "account:summary"
	accountCacheTTL = 10 * time.Minute
)

// AccountHandler handles account management endpoints
type AccountHandler struct {
	DB              *pgxpool.Pool
	EntryRepo       *repositories.EntryRepository
	RoomEntryRepo   *repositories.RoomEntryRepository
	RentPaymentRepo *repositories.RentPaymentRepository
	GatePassRepo    *repositories.GatePassRepository
	SettingsRepo    *repositories.SystemSettingRepository
}

// FamilyMemberAccount represents a family member's account within a customer
type FamilyMemberAccount struct {
	ID         int                   `json:"id,omitempty"`
	Name       string                `json:"name"`
	Relation   string                `json:"relation,omitempty"`
	Quantity   int                   `json:"quantity"`
	Outgoing   int                   `json:"outgoing"`
	Rent       float64               `json:"rent"`
	Paid       float64               `json:"paid"`
	Balance    float64               `json:"balance"`
	CanTakeOut int                   `json:"can_take_out"`
	Thocks     []ThockInfo           `json:"thocks"`
	Payments   []*models.RentPayment `json:"payments"`
}

// CustomerAccount represents a customer's complete account info
type CustomerAccount struct {
	CustomerID    int                   `json:"customer_id"`
	Name          string                `json:"name"`
	Phone         string                `json:"phone"`
	SO            string                `json:"so"`
	Village       string                `json:"village"`
	Thocks        []ThockInfo           `json:"thocks"`
	Payments      []*models.RentPayment `json:"payments"`
	FamilyMembers []FamilyMemberAccount `json:"family_members"`
	TotalQuantity int                   `json:"total_quantity"`
	TotalRent     float64               `json:"total_rent"`
	TotalPaid     float64               `json:"total_paid"`
	TotalOutgoing int                   `json:"total_outgoing"`
	OutgoingRent  float64               `json:"outgoing_rent"`
	Balance       float64               `json:"balance"`
}

// ThockInfo represents a thock entry with family member support
type ThockInfo struct {
	ID               int     `json:"id"`
	ThockNumber      string  `json:"thock_number"`
	FamilyMemberName string  `json:"family_member_name"`
	Quantity         int     `json:"quantity"`
	QtyDisplay       string  `json:"qty_display,omitempty"`
	Rent             float64 `json:"rent"`
	Date             string  `json:"date"`
	Type             string  `json:"type"` // "incoming" or "outgoing"
}

// CompletedGatePass represents a completed gate pass with customer info
type CompletedGatePass struct {
	ID            int
	ThockNumber   string
	CustomerPhone string
	TotalPickedUp int
	RequestedQty  int
	CompletedAt   time.Time
}

// AccountSummary is the complete response for account management
type AccountSummary struct {
	Customers        []CustomerAccount `json:"customers"`
	TotalCustomers   int               `json:"total_customers"`
	TotalThocks      int               `json:"total_thocks"`
	TotalQuantity    int               `json:"total_quantity"`
	TotalOutstanding float64           `json:"total_outstanding"`
	TotalCollected   float64           `json:"total_collected"`
	RentPerItem      float64           `json:"rent_per_item"`
	GeneratedAt      time.Time         `json:"generated_at"`
}

func NewAccountHandler(
	db *pgxpool.Pool,
	entryRepo *repositories.EntryRepository,
	roomEntryRepo *repositories.RoomEntryRepository,
	rentPaymentRepo *repositories.RentPaymentRepository,
	gatePassRepo *repositories.GatePassRepository,
	settingsRepo *repositories.SystemSettingRepository,
) *AccountHandler {
	h := &AccountHandler{
		DB:              db,
		EntryRepo:       entryRepo,
		RoomEntryRepo:   roomEntryRepo,
		RentPaymentRepo: rentPaymentRepo,
		GatePassRepo:    gatePassRepo,
		SettingsRepo:    settingsRepo,
	}

	// Register pre-warm callback for account summary
	cache.RegisterPreWarm(accountCacheKey, func(ctx context.Context) ([]byte, error) {
		summary, err := h.generateAccountSummary(ctx)
		if err != nil {
			return nil, err
		}
		return json.Marshal(summary)
	})

	return h
}

// GetAccountSummary returns all data needed for account management in ONE request
// This eliminates 5+ sequential API calls from the frontend
// Uses Redis cache with 10 minute TTL for fast responses
func (h *AccountHandler) GetAccountSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Verify accountant access
	role, ok := middleware.GetRoleFromContext(ctx)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasAccountantAccess, _ := ctx.Value(middleware.HasAccountantAccessKey).(bool)
	if role != "admin" && role != "accountant" && !hasAccountantAccess {
		http.Error(w, "Forbidden - accountant access required", http.StatusForbidden)
		return
	}

	// Try cache first
	if data, ok := cache.GetCached(ctx, accountCacheKey); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.Write(data)
		return
	}

	// Cache miss - generate fresh data
	summary, err := h.generateAccountSummary(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Cache the response
	data, _ := json.Marshal(summary)
	cache.SetCached(ctx, accountCacheKey, data, accountCacheTTL)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Write(data)
}

// UsedDebtRequest represents a used debt request for credit tracking
type UsedDebtRequest struct {
	CustomerPhone     string
	RequestedQuantity int
}

// generateAccountSummary generates the account summary data (used by cache and pre-warm)
func (h *AccountHandler) generateAccountSummary(ctx context.Context) (*AccountSummary, error) {
	// Parallel data fetching using goroutines
	var (
		entries             []*models.Entry
		roomEntries         []*models.RoomEntry
		payments            []*models.RentPayment
		completedGatePasses []CompletedGatePass
		usedDebtRequests    []UsedDebtRequest
		familyMemberMap     map[int]map[string]string // customer_id -> name -> relation
		rentPerItem         float64
		wg                  sync.WaitGroup
		entriesErr          error
		roomErr             error
		paymentsErr         error
		gatePassErr         error
		debtErr             error
		settingsErr error
	)

	wg.Add(7)

	// Fetch entries
	go func() {
		defer wg.Done()
		entries, entriesErr = h.EntryRepo.List(ctx)
	}()

	// Fetch room entries
	go func() {
		defer wg.Done()
		roomEntries, roomErr = h.RoomEntryRepo.List(ctx)
	}()

	// Fetch payments (with employee names already joined)
	go func() {
		defer wg.Done()
		payments, paymentsErr = h.RentPaymentRepo.List(ctx)
	}()

	// Fetch completed gate passes with customer phone
	go func() {
		defer wg.Done()
		completedGatePasses, gatePassErr = h.getCompletedGatePasses(ctx)
	}()

	// Fetch used debt requests (items taken on credit)
	go func() {
		defer wg.Done()
		usedDebtRequests, debtErr = h.getUsedDebtRequests(ctx)
	}()

	// Fetch rent per item setting
	go func() {
		defer wg.Done()
		setting, err := h.SettingsRepo.Get(ctx, "rent_per_item")
		if err != nil {
			settingsErr = err
			return
		}
		if setting != nil {
			// Try to parse as number - the value might be stored as a string like "10" or as JSON
			var val float64
			if err := json.Unmarshal([]byte(setting.SettingValue), &val); err != nil {
				// Try parsing as plain string number
				fmt.Sscanf(setting.SettingValue, "%f", &val)
			}
			rentPerItem = val
		}
	}()

	// Fetch all family members to get relations
	go func() {
		defer wg.Done()
		familyMemberMap = make(map[int]map[string]string)
		rows, err := h.DB.Query(ctx,
			`SELECT customer_id, name, relation FROM family_members`)
		if err != nil {
			// Non-fatal error, just skip family member relations
			return
		}
		defer rows.Close()

		for rows.Next() {
			var customerID int
			var name, relation string
			if err := rows.Scan(&customerID, &name, &relation); err != nil {
				continue
			}
			if familyMemberMap[customerID] == nil {
				familyMemberMap[customerID] = make(map[string]string)
			}
			familyMemberMap[customerID][name] = relation
		}
	}()

	wg.Wait()

	// Check for errors
	if entriesErr != nil {
		return nil, fmt.Errorf("failed to load entries: %w", entriesErr)
	}
	if roomErr != nil {
		return nil, fmt.Errorf("failed to load room entries: %w", roomErr)
	}
	if paymentsErr != nil {
		return nil, fmt.Errorf("failed to load payments: %w", paymentsErr)
	}
	// Gate pass, debt, and settings errors are non-fatal
	if settingsErr != nil {
		rentPerItem = 0
	}
	if gatePassErr != nil {
		completedGatePasses = []CompletedGatePass{}
	}
	if debtErr != nil {
		usedDebtRequests = []UsedDebtRequest{}
	}

	// Build credit map from used debt requests (items taken on credit that are still owed)
	creditByPhone := make(map[string]float64)
	for _, dr := range usedDebtRequests {
		creditByPhone[dr.CustomerPhone] += float64(dr.RequestedQuantity) * rentPerItem
	}

	// Build thock stored quantity map from room entries
	thockStoredQty := make(map[string]int)
	for _, re := range roomEntries {
		thockStoredQty[re.ThockNumber] += re.Quantity
	}

	// Build customer map
	customerMap := make(map[string]*CustomerAccount)

	// Process entries
	for _, entry := range entries {
		phone := entry.Phone
		if _, exists := customerMap[phone]; !exists {
			customerMap[phone] = &CustomerAccount{
				CustomerID: entry.CustomerID,
				Name:       entry.Name,
				Phone:      entry.Phone,
				SO:         entry.SO,
				Village:    entry.Village,
				Thocks:     make([]ThockInfo, 0),
				Payments:   make([]*models.RentPayment, 0),
			}
		}

		customer := customerMap[phone]
		storedQty := thockStoredQty[entry.ThockNumber]
		expectedQty := entry.ExpectedQuantity
		rent := float64(storedQty) * rentPerItem

		qtyDisplay := ""
		if expectedQty != storedQty {
			qtyDisplay = fmt.Sprintf("%d → %d", expectedQty, storedQty)
		}

		customer.Thocks = append(customer.Thocks, ThockInfo{
			ID:               entry.ID,
			ThockNumber:      entry.ThockNumber,
			FamilyMemberName: entry.FamilyMemberName,
			Quantity:         storedQty,
			QtyDisplay:       qtyDisplay,
			Rent:             rent,
			Date:             entry.CreatedAt.Format("02/01/2006"),
			Type:             "incoming",
		})

		customer.TotalQuantity += storedQty
		customer.TotalRent += rent
	}

	// Process completed gate passes (outgoing)
	for _, gp := range completedGatePasses {
		customer, exists := customerMap[gp.CustomerPhone]
		if !exists {
			continue
		}

		quantity := gp.TotalPickedUp
		if quantity == 0 {
			quantity = gp.RequestedQty
		}
		rent := float64(quantity) * rentPerItem

		customer.Thocks = append(customer.Thocks, ThockInfo{
			ID:          gp.ID,
			ThockNumber: gp.ThockNumber,
			Quantity:    -quantity, // Negative for outgoing
			Rent:        -rent,
			Date:        gp.CompletedAt.Format("02/01/2006"),
			Type:        "outgoing",
		})

		customer.TotalOutgoing += quantity
		customer.OutgoingRent += rent
	}

	// Assign payments to customers
	for _, payment := range payments {
		customer, exists := customerMap[payment.CustomerPhone]
		if exists {
			customer.Payments = append(customer.Payments, payment)
			customer.TotalPaid += payment.AmountPaid
		}
	}

	// Calculate balances and build result
	customers := make([]CustomerAccount, 0, len(customerMap))
	var totalOutstanding, totalCollected float64
	var totalThocks, totalQty int

	for _, customer := range customerMap {
		// Group thocks by family member
		familyMemberThocks := make(map[string][]ThockInfo)
		familyMemberQty := make(map[string]int)
		familyMemberRent := make(map[string]float64)

		for _, thock := range customer.Thocks {
			fmName := thock.FamilyMemberName
			if fmName == "" {
				fmName = customer.Name // Default to customer name if no family member
			}
			familyMemberThocks[fmName] = append(familyMemberThocks[fmName], thock)
			if thock.Type == "incoming" {
				familyMemberQty[fmName] += thock.Quantity
				familyMemberRent[fmName] += thock.Rent
			}
		}

		// Group payments by family member
		familyMemberPayments := make(map[string][]*models.RentPayment)
		familyMemberPaid := make(map[string]float64)

		for _, payment := range customer.Payments {
			fmName := payment.FamilyMemberName
			if fmName == "" {
				fmName = customer.Name // Default to customer name if no family member
			}
			familyMemberPayments[fmName] = append(familyMemberPayments[fmName], payment)
			familyMemberPaid[fmName] += payment.AmountPaid
		}

		// Build family member accounts
		familyMembers := make([]FamilyMemberAccount, 0)
		processedFamilyMembers := make(map[string]bool)

		// Get family member relations for this customer
		customerFamilyRelations := familyMemberMap[customer.CustomerID]

		// Process family members from thocks
		for fmName := range familyMemberThocks {
			processedFamilyMembers[fmName] = true
			fmRent := familyMemberRent[fmName]
			fmPaid := familyMemberPaid[fmName]
			fmBalance := fmRent - fmPaid

			// Calculate outgoing (items picked up) for this family member
			fmOutgoing := 0
			for _, thock := range familyMemberThocks[fmName] {
				if thock.Type == "outgoing" {
					fmOutgoing += thock.Quantity
				}
			}

			// Calculate canTakeOut: items paid for minus items already picked up
			fmCanTakeOut := 0
			if rentPerItem > 0 {
				itemsPaidFor := int(fmPaid / rentPerItem)
				fmCanTakeOut = itemsPaidFor - fmOutgoing
				if fmCanTakeOut < 0 {
					fmCanTakeOut = 0
				}
			}

			// Get relation for this family member
			fmRelation := ""
			if customerFamilyRelations != nil {
				fmRelation = customerFamilyRelations[fmName]
			}

			familyMembers = append(familyMembers, FamilyMemberAccount{
				Name:       fmName,
				Relation:   fmRelation,
				Quantity:   familyMemberQty[fmName],
				Outgoing:   fmOutgoing,
				Rent:       fmRent,
				Paid:       fmPaid,
				Balance:    fmBalance,
				CanTakeOut: fmCanTakeOut,
				Thocks:     familyMemberThocks[fmName],
				Payments:   familyMemberPayments[fmName],
			})
		}

		// Add family members who only have payments (no thocks)
		for fmName, payments := range familyMemberPayments {
			if !processedFamilyMembers[fmName] {
				fmPaid := familyMemberPaid[fmName]

				// Calculate canTakeOut for family members with only payments
				fmCanTakeOut := 0
				if rentPerItem > 0 {
					itemsPaidFor := int(fmPaid / rentPerItem)
					fmCanTakeOut = itemsPaidFor // No outgoing since no thocks
				}

				// Get relation for this family member
				fmRelation := ""
				if customerFamilyRelations != nil {
					fmRelation = customerFamilyRelations[fmName]
				}

				familyMembers = append(familyMembers, FamilyMemberAccount{
					Name:       fmName,
					Relation:   fmRelation,
					Quantity:   0,
					Outgoing:   0,
					Rent:       0,
					Paid:       fmPaid,
					Balance:    -fmPaid, // Overpaid
					CanTakeOut: fmCanTakeOut,
					Thocks:     []ThockInfo{},
					Payments:   payments,
				})
			}
		}

		// Sort family members: those with balance first, then alphabetically
		sort.Slice(familyMembers, func(i, j int) bool {
			iHasDue := familyMembers[i].Balance > 0
			jHasDue := familyMembers[j].Balance > 0
			if iHasDue != jHasDue {
				return iHasDue
			}
			return strings.ToLower(familyMembers[i].Name) < strings.ToLower(familyMembers[j].Name)
		})

		customer.FamilyMembers = familyMembers

		// Balance = TotalRent + CreditValue - TotalPaid
		// TotalRent = rent for items currently in storage
		// CreditValue = rent for items taken out on credit (admin-approved debt)
		creditValue := creditByPhone[customer.Phone]
		customer.Balance = customer.TotalRent + creditValue - customer.TotalPaid
		customers = append(customers, *customer)
		totalOutstanding += customer.Balance
		totalCollected += customer.TotalPaid
		totalThocks += len(customer.Thocks)
		totalQty += customer.TotalQuantity
	}

	// Sort: customers with dues first (alphabetically), then no-due customers (alphabetically)
	sort.Slice(customers, func(i, j int) bool {
		iHasDue := customers[i].Balance > 0
		jHasDue := customers[j].Balance > 0

		if iHasDue != jHasDue {
			return iHasDue // Customers with dues come first
		}
		// Within same category, sort alphabetically by name
		return strings.ToLower(customers[i].Name) < strings.ToLower(customers[j].Name)
	})

	return &AccountSummary{
		Customers:        customers,
		TotalCustomers:   len(customers),
		TotalThocks:      totalThocks,
		TotalQuantity:    totalQty,
		TotalOutstanding: totalOutstanding,
		TotalCollected:   totalCollected,
		RentPerItem:      rentPerItem,
		GeneratedAt:      time.Now(),
	}, nil
}

// getCompletedGatePasses fetches completed gate passes with customer phone
func (h *AccountHandler) getCompletedGatePasses(ctx context.Context) ([]CompletedGatePass, error) {
	query := `
		SELECT gp.id, gp.thock_number, c.phone, gp.total_picked_up, gp.requested_quantity,
		       COALESCE(gp.completed_at, gp.updated_at) as completed_at
		FROM gate_passes gp
		JOIN customers c ON gp.customer_id = c.id
		WHERE gp.status = 'completed'
		ORDER BY gp.completed_at DESC
	`

	rows, err := h.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []CompletedGatePass
	for rows.Next() {
		var gp CompletedGatePass
		if err := rows.Scan(&gp.ID, &gp.ThockNumber, &gp.CustomerPhone, &gp.TotalPickedUp, &gp.RequestedQty, &gp.CompletedAt); err != nil {
			return nil, err
		}
		results = append(results, gp)
	}

	return results, nil
}

// getUsedDebtRequests fetches used debt requests (items taken on credit)
func (h *AccountHandler) getUsedDebtRequests(ctx context.Context) ([]UsedDebtRequest, error) {
	query := `
		SELECT customer_phone, requested_quantity
		FROM debt_requests
		WHERE status = 'used'
	`

	rows, err := h.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []UsedDebtRequest
	for rows.Next() {
		var dr UsedDebtRequest
		if err := rows.Scan(&dr.CustomerPhone, &dr.RequestedQuantity); err != nil {
			return nil, err
		}
		results = append(results, dr)
	}

	return results, nil
}
