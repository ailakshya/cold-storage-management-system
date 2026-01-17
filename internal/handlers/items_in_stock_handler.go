package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"cold-backend/internal/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ItemsInStockHandler handles items in stock endpoints
type ItemsInStockHandler struct {
	DB *pgxpool.Pool
}

// NewItemsInStockHandler creates a new items in stock handler
func NewItemsInStockHandler(db *pgxpool.Pool) *ItemsInStockHandler {
	return &ItemsInStockHandler{
		DB: db,
	}
}

// ItemInStock represents a single item currently in storage
type ItemInStock struct {
	EntryID          int       `json:"entry_id"`
	ThockNumber      string    `json:"thock_number"`
	CustomerName     string    `json:"customer_name"`
	Phone            string    `json:"phone"`
	Village          string    `json:"village"`
	FamilyMemberName string    `json:"family_member_name"`
	CurrentQty       int       `json:"current_qty"`
	RoomNo           string    `json:"room_no"`
	Floor            string    `json:"floor"`
	GatarLocations   string    `json:"gatar_locations"`
	CreatedAt        time.Time `json:"created_at"`
	DaysStored       int       `json:"days_stored"`
}

// StockSummary contains summary statistics for items in stock
type StockSummary struct {
	TotalItems     int                 `json:"total_items"`
	TotalQuantity  int                 `json:"total_quantity"`
	TotalCustomers int                 `json:"total_customers"`
	CapacityUsed   float64             `json:"capacity_used"`
	ByRoom         map[string]RoomStat `json:"by_room"`
	ByFloor        map[string]int      `json:"by_floor"`
}

// RoomStat contains statistics for a specific room
type RoomStat struct {
	Items    int `json:"items"`
	Quantity int `json:"quantity"`
}

// GetItemsInStock returns all items currently in storage
func (h *ItemsInStockHandler) GetItemsInStock(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Verify authentication
	_, ok := middleware.GetRoleFromContext(ctx)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	items, err := h.queryItemsInStock(ctx)
	if err != nil {
		http.Error(w, "Failed to load items: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"items": items,
		"count": len(items),
	})
}

// GetSummary returns summary statistics for items in stock
func (h *ItemsInStockHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Verify authentication
	_, ok := middleware.GetRoleFromContext(ctx)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	summary, err := h.calculateSummary(ctx)
	if err != nil {
		http.Error(w, "Failed to calculate summary: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

// queryItemsInStock fetches all items currently in storage
func (h *ItemsInStockHandler) queryItemsInStock(ctx context.Context) ([]ItemInStock, error) {
	query := `
		SELECT 
			e.id as entry_id,
			e.thock_number,
			COALESCE(e.name, '') as customer_name,
			COALESCE(e.phone, '') as phone,
			COALESCE(e.village, '') as village,
			COALESCE(e.family_member_name, '') as family_member_name,
			SUM(re.quantity) as current_qty,
			re.room_no,
			re.floor,
			STRING_AGG(DISTINCT re.gate_no, ', ' ORDER BY re.gate_no) as gatar_locations,
			e.created_at,
			EXTRACT(DAY FROM NOW() - e.created_at) as days_stored
		FROM room_entries re
		JOIN entries e ON re.entry_id = e.id
		GROUP BY e.id, e.thock_number, e.name, e.phone, e.village, 
		         e.family_member_name, re.room_no, re.floor, e.created_at
		HAVING SUM(re.quantity) > 0
		ORDER BY re.room_no, re.floor, e.thock_number
	`

	rows, err := h.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ItemInStock
	for rows.Next() {
		var item ItemInStock
		var daysStored float64

		err := rows.Scan(
			&item.EntryID,
			&item.ThockNumber,
			&item.CustomerName,
			&item.Phone,
			&item.Village,
			&item.FamilyMemberName,
			&item.CurrentQty,
			&item.RoomNo,
			&item.Floor,
			&item.GatarLocations,
			&item.CreatedAt,
			&daysStored,
		)
		if err != nil {
			return nil, err
		}

		item.DaysStored = int(daysStored)
		items = append(items, item)
	}

	return items, nil
}

// calculateSummary calculates summary statistics
func (h *ItemsInStockHandler) calculateSummary(ctx context.Context) (*StockSummary, error) {
	items, err := h.queryItemsInStock(ctx)
	if err != nil {
		return nil, err
	}

	summary := &StockSummary{
		ByRoom:  make(map[string]RoomStat),
		ByFloor: make(map[string]int),
	}

	uniqueCustomers := make(map[string]bool)
	totalQty := 0

	for _, item := range items {
		summary.TotalItems++
		totalQty += item.CurrentQty
		uniqueCustomers[item.Phone] = true

		// By room
		roomKey := "Room " + item.RoomNo
		roomStat := summary.ByRoom[roomKey]
		roomStat.Items++
		roomStat.Quantity += item.CurrentQty
		summary.ByRoom[roomKey] = roomStat

		// By floor
		floorKey := "Floor " + item.Floor
		summary.ByFloor[floorKey] += item.CurrentQty
	}

	summary.TotalQuantity = totalQty
	summary.TotalCustomers = len(uniqueCustomers)

	// Calculate capacity used (total capacity = 140,000 bags)
	const totalCapacity = 140000
	summary.CapacityUsed = (float64(totalQty) / float64(totalCapacity)) * 100

	return summary, nil
}
