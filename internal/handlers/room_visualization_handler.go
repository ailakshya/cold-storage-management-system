package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RoomVisualizationHandler handles room visualization endpoints
type RoomVisualizationHandler struct {
	DB *pgxpool.Pool
}

// NewRoomVisualizationHandler creates a new room visualization handler
func NewRoomVisualizationHandler(db *pgxpool.Pool) *RoomVisualizationHandler {
	return &RoomVisualizationHandler{DB: db}
}

// FloorStats contains statistics for a single floor
type FloorStats struct {
	Floor          string `json:"floor"`
	OccupiedGatars int    `json:"occupied_gatars"`
	TotalGatars    int    `json:"total_gatars"`
	TotalQuantity  int    `json:"total_qty"`
	EntryCount     int    `json:"entry_count"`
}

// RoomStats contains statistics for a single room
type RoomStats struct {
	RoomNo string       `json:"room_no"`
	Floors []FloorStats `json:"floors"`
}

// VisualizationSummary contains overall summary
type VisualizationSummary struct {
	TotalQuantity    int `json:"total_qty"`
	OccupiedGatars   int `json:"occupied_gatars"`
	TotalGatars      int `json:"total_gatars"`
	TotalEntryCount  int `json:"total_entry_count"`
}

// RoomVisualizationResponse is the response for GetRoomStats
type RoomVisualizationResponse struct {
	Rooms   []RoomStats          `json:"rooms"`
	Summary VisualizationSummary `json:"summary"`
}

// GatarItem represents an item stored in a gatar
type GatarItem struct {
	ThockNumber string `json:"thock_number"`
	Quantity    int    `json:"quantity"`
	Variety     string `json:"variety"`
	EntryID     int    `json:"entry_id"`
}

// GatarInfo represents a single gatar's data
type GatarInfo struct {
	Gatar    string      `json:"gatar"`
	Occupied bool        `json:"occupied"`
	Items    []GatarItem `json:"items"`
	TotalQty int         `json:"total_qty"`
}

// GatarOccupancyResponse is the response for GetGatarOccupancy
type GatarOccupancyResponse struct {
	RoomNo  string      `json:"room_no"`
	Floor   string      `json:"floor"`
	Gatars  []GatarInfo `json:"gatars"`
}

// Gatar ranges for each room/floor (from docs/ROOM_LAYOUT.md)
var gatarRanges = map[string]map[string]struct{ Start, End, Total int }{
	"1": {
		"0": {1, 140, 140},
		"1": {141, 280, 140},
		"2": {281, 420, 140},
		"3": {421, 560, 140},
		"4": {561, 680, 120},
	},
	"2": {
		"0": {681, 820, 140},
		"1": {821, 960, 140},
		"2": {961, 1100, 140},
		"3": {1101, 1240, 140},
		"4": {1241, 1360, 120},
	},
	"3": {
		"0": {1361, 1500, 140},
		"1": {1501, 1640, 140},
		"2": {1641, 1780, 140},
		"3": {1781, 1920, 140},
		"4": {1921, 2040, 120},
	},
}

// GetRoomStats returns aggregated statistics for all rooms and floors
func (h *RoomVisualizationHandler) GetRoomStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Query to get stats grouped by room and floor
	query := `
		SELECT
			room_no,
			floor,
			COUNT(DISTINCT gate_no) as occupied_gatars,
			SUM(quantity) as total_qty,
			COUNT(DISTINCT entry_id) as entry_count
		FROM room_entries
		WHERE deleted_at IS NULL
		GROUP BY room_no, floor
		ORDER BY room_no, floor
	`

	rows, err := h.DB.Query(ctx, query)
	if err != nil {
		http.Error(w, "Failed to query room stats: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Build a map to store stats
	statsMap := make(map[string]map[string]FloorStats)

	for rows.Next() {
		var roomNo, floor string
		var occupiedGatars, totalQty, entryCount int

		if err := rows.Scan(&roomNo, &floor, &occupiedGatars, &totalQty, &entryCount); err != nil {
			http.Error(w, "Failed to scan row: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if statsMap[roomNo] == nil {
			statsMap[roomNo] = make(map[string]FloorStats)
		}

		// Get total gatars for this room/floor
		totalGatars := 0
		if ranges, ok := gatarRanges[roomNo]; ok {
			if floorRange, ok := ranges[floor]; ok {
				totalGatars = floorRange.Total
			}
		}

		statsMap[roomNo][floor] = FloorStats{
			Floor:          floor,
			OccupiedGatars: occupiedGatars,
			TotalGatars:    totalGatars,
			TotalQuantity:  totalQty,
			EntryCount:     entryCount,
		}
	}

	// Build response
	var rooms []RoomStats
	var totalQty, totalOccupied, totalGatars, totalEntries int

	// Process rooms in order
	for _, roomNo := range []string{"1", "2", "3"} {
		var floors []FloorStats

		// Process floors in order (0-4)
		for floor := 0; floor <= 4; floor++ {
			floorStr := strconv.Itoa(floor)

			// Get total gatars for this floor
			floorTotalGatars := 0
			if ranges, ok := gatarRanges[roomNo]; ok {
				if floorRange, ok := ranges[floorStr]; ok {
					floorTotalGatars = floorRange.Total
				}
			}

			if stats, ok := statsMap[roomNo][floorStr]; ok {
				stats.TotalGatars = floorTotalGatars
				floors = append(floors, stats)
				totalQty += stats.TotalQuantity
				totalOccupied += stats.OccupiedGatars
				totalEntries += stats.EntryCount
			} else {
				// Floor has no entries yet
				floors = append(floors, FloorStats{
					Floor:          floorStr,
					OccupiedGatars: 0,
					TotalGatars:    floorTotalGatars,
					TotalQuantity:  0,
					EntryCount:     0,
				})
			}
			totalGatars += floorTotalGatars
		}

		rooms = append(rooms, RoomStats{
			RoomNo: roomNo,
			Floors: floors,
		})
	}

	response := RoomVisualizationResponse{
		Rooms: rooms,
		Summary: VisualizationSummary{
			TotalQuantity:   totalQty,
			OccupiedGatars:  totalOccupied,
			TotalGatars:     totalGatars,
			TotalEntryCount: totalEntries,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetGatarOccupancy returns detailed gatar-level data for a specific room/floor
func (h *RoomVisualizationHandler) GetGatarOccupancy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	roomNo := r.URL.Query().Get("room")
	floor := r.URL.Query().Get("floor")

	if roomNo == "" || floor == "" {
		http.Error(w, "Missing room or floor parameter", http.StatusBadRequest)
		return
	}

	// Validate room and floor
	if _, ok := gatarRanges[roomNo]; !ok {
		http.Error(w, "Invalid room number", http.StatusBadRequest)
		return
	}
	floorRange, ok := gatarRanges[roomNo][floor]
	if !ok {
		http.Error(w, "Invalid floor number", http.StatusBadRequest)
		return
	}

	// Query to get items in each gatar
	query := `
		SELECT
			re.gate_no,
			re.thock_number,
			re.quantity,
			COALESCE(e.remark, '') as variety,
			re.entry_id
		FROM room_entries re
		LEFT JOIN entries e ON re.entry_id = e.id
		WHERE re.room_no = $1
		  AND re.floor = $2
		  AND re.deleted_at IS NULL
		ORDER BY re.gate_no, re.created_at DESC
	`

	rows, err := h.DB.Query(ctx, query, roomNo, floor)
	if err != nil {
		http.Error(w, "Failed to query gatar data: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Build a map of gatar -> items
	gatarItems := make(map[string][]GatarItem)
	gatarTotals := make(map[string]int)

	for rows.Next() {
		var gateNo, thockNumber, variety string
		var quantity, entryID int

		if err := rows.Scan(&gateNo, &thockNumber, &quantity, &variety, &entryID); err != nil {
			http.Error(w, "Failed to scan row: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Handle comma-separated gate numbers (e.g., "1, 2, 3")
		gateNos := strings.Split(gateNo, ",")
		for _, g := range gateNos {
			g = strings.TrimSpace(g)
			if g == "" {
				continue
			}

			item := GatarItem{
				ThockNumber: thockNumber,
				Quantity:    quantity,
				Variety:     variety,
				EntryID:     entryID,
			}

			gatarItems[g] = append(gatarItems[g], item)
			gatarTotals[g] += quantity
		}
	}

	// Build response with all gatars in range
	var gatars []GatarInfo
	for g := floorRange.Start; g <= floorRange.End; g++ {
		gStr := strconv.Itoa(g)
		items := gatarItems[gStr]
		gatars = append(gatars, GatarInfo{
			Gatar:    gStr,
			Occupied: len(items) > 0,
			Items:    items,
			TotalQty: gatarTotals[gStr],
		})
	}

	response := GatarOccupancyResponse{
		RoomNo: roomNo,
		Floor:  floor,
		Gatars: gatars,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetGatarDetails returns details for a specific gatar
func (h *RoomVisualizationHandler) GetGatarDetails(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	gatar := r.URL.Query().Get("gatar")
	if gatar == "" {
		http.Error(w, "Missing gatar parameter", http.StatusBadRequest)
		return
	}

	// Query to get all items in this gatar (handle comma-separated gate_no)
	query := `
		SELECT
			re.id,
			re.thock_number,
			re.room_no,
			re.floor,
			re.gate_no,
			re.quantity,
			re.remark,
			COALESCE(e.remark, '') as variety,
			COALESCE(c.name, '') as customer_name,
			COALESCE(c.phone, '') as customer_phone,
			re.created_at
		FROM room_entries re
		LEFT JOIN entries e ON re.entry_id = e.id
		LEFT JOIN customers c ON e.customer_id = c.id
		WHERE re.deleted_at IS NULL
		  AND (re.gate_no = $1 OR re.gate_no LIKE '%' || $1 || '%')
		ORDER BY re.created_at DESC
	`

	rows, err := h.DB.Query(ctx, query, gatar)
	if err != nil {
		http.Error(w, "Failed to query gatar details: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type GatarDetail struct {
		ID            int    `json:"id"`
		ThockNumber   string `json:"thock_number"`
		RoomNo        string `json:"room_no"`
		Floor         string `json:"floor"`
		GateNo        string `json:"gate_no"`
		Quantity      int    `json:"quantity"`
		Remark        string `json:"remark"`
		Variety       string `json:"variety"`
		CustomerName  string `json:"customer_name"`
		CustomerPhone string `json:"customer_phone"`
		CreatedAt     string `json:"created_at"`
	}

	var details []GatarDetail
	for rows.Next() {
		var d GatarDetail
		var createdAt interface{}

		if err := rows.Scan(
			&d.ID, &d.ThockNumber, &d.RoomNo, &d.Floor, &d.GateNo,
			&d.Quantity, &d.Remark, &d.Variety, &d.CustomerName,
			&d.CustomerPhone, &createdAt,
		); err != nil {
			http.Error(w, "Failed to scan row: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Format created_at
		if t, ok := createdAt.(interface{ Format(string) string }); ok {
			d.CreatedAt = t.Format("02/01/2006 15:04")
		}

		// Check if this gatar is actually in the gate_no (handle comma-separated)
		gateNos := strings.Split(d.GateNo, ",")
		for _, g := range gateNos {
			if strings.TrimSpace(g) == gatar {
				details = append(details, d)
				break
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"gatar":   gatar,
		"items":   details,
		"count":   len(details),
	})
}

// Helper function to get room/floor from gatar number
func getRoomFloorFromGatar(gatarNum int) (string, string) {
	for roomNo, floors := range gatarRanges {
		for floor, r := range floors {
			if gatarNum >= r.Start && gatarNum <= r.End {
				return roomNo, floor
			}
		}
	}
	return "", ""
}
