package repositories

import (
	"context"

	"cold-backend/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RoomEntryGatarRepository struct {
	DB *pgxpool.Pool
}

func NewRoomEntryGatarRepository(db *pgxpool.Pool) *RoomEntryGatarRepository {
	return &RoomEntryGatarRepository{DB: db}
}

// CreateBatch inserts multiple gatar entries for a room entry
func (r *RoomEntryGatarRepository) CreateBatch(ctx context.Context, roomEntryID int, gatars []models.GatarInput) error {
	for _, g := range gatars {
		_, err := r.DB.Exec(ctx,
			`INSERT INTO room_entry_gatars(room_entry_id, gatar_no, quantity, quality, remark)
             VALUES($1, $2, $3, $4, $5)`,
			roomEntryID, g.GatarNo, g.Quantity, g.Quality, g.Remark)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetByRoomEntryID returns all gatar entries for a room entry
func (r *RoomEntryGatarRepository) GetByRoomEntryID(ctx context.Context, roomEntryID int) ([]models.RoomEntryGatar, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT id, room_entry_id, gatar_no, quantity, COALESCE(quality, ''), COALESCE(remark, ''), created_at, updated_at
         FROM room_entry_gatars
         WHERE room_entry_id = $1
         ORDER BY gatar_no`, roomEntryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var gatars []models.RoomEntryGatar
	for rows.Next() {
		var g models.RoomEntryGatar
		err := rows.Scan(&g.ID, &g.RoomEntryID, &g.GatarNo, &g.Quantity, &g.Quality, &g.Remark, &g.CreatedAt, &g.UpdatedAt)
		if err != nil {
			return nil, err
		}
		gatars = append(gatars, g)
	}
	return gatars, nil
}

// GetByRoomFloorGatar returns gatar entries for a specific room/floor/gatar combination
func (r *RoomEntryGatarRepository) GetByRoomFloorGatar(ctx context.Context, roomNo string, floor string, gatarNo int) ([]models.RoomEntryGatar, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT reg.id, reg.room_entry_id, reg.gatar_no, reg.quantity, COALESCE(reg.quality, ''), COALESCE(reg.remark, ''), reg.created_at, reg.updated_at
         FROM room_entry_gatars reg
         JOIN room_entries re ON reg.room_entry_id = re.id
         LEFT JOIN entries e ON re.entry_id = e.id
         WHERE re.room_no = $1 AND re.floor = $2 AND reg.gatar_no = $3
           AND COALESCE(e.status, 'active') != 'deleted'
           AND reg.quantity > 0
         ORDER BY reg.created_at`, roomNo, floor, gatarNo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var gatars []models.RoomEntryGatar
	for rows.Next() {
		var g models.RoomEntryGatar
		err := rows.Scan(&g.ID, &g.RoomEntryID, &g.GatarNo, &g.Quantity, &g.Quality, &g.Remark, &g.CreatedAt, &g.UpdatedAt)
		if err != nil {
			return nil, err
		}
		gatars = append(gatars, g)
	}
	return gatars, nil
}

// GatarStock represents stock information for a single gatar
type GatarStock struct {
	GatarNo     int    `json:"gatar_no"`
	Quantity    int    `json:"quantity"`
	Variety     string `json:"variety"`
	ThockNumber string `json:"thock_number"`
	Quality     string `json:"quality"`
	RoomEntryID int    `json:"room_entry_id"`
}

// GetStockByRoomFloor returns per-gatar stock counts for visualization
func (r *RoomEntryGatarRepository) GetStockByRoomFloor(ctx context.Context, roomNo string, floor string) ([]GatarStock, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT reg.gatar_no, SUM(reg.quantity) as total_qty,
		        STRING_AGG(DISTINCT COALESCE(e.remark, ''), ', ') as variety,
		        STRING_AGG(DISTINCT re.thock_number, ', ') as thock_numbers,
		        COALESCE(MAX(reg.quality), '') as quality,
		        MAX(reg.room_entry_id) as room_entry_id
         FROM room_entry_gatars reg
         JOIN room_entries re ON reg.room_entry_id = re.id
         LEFT JOIN entries e ON re.entry_id = e.id
         WHERE re.room_no = $1 AND re.floor = $2
           AND COALESCE(e.status, 'active') != 'deleted'
           AND reg.quantity > 0
         GROUP BY reg.gatar_no
         ORDER BY reg.gatar_no`, roomNo, floor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stocks []GatarStock
	for rows.Next() {
		var s GatarStock
		err := rows.Scan(&s.GatarNo, &s.Quantity, &s.Variety, &s.ThockNumber, &s.Quality, &s.RoomEntryID)
		if err != nil {
			return nil, err
		}
		stocks = append(stocks, s)
	}
	return stocks, nil
}

// ReduceQuantity reduces the quantity in a specific gatar entry
func (r *RoomEntryGatarRepository) ReduceQuantity(ctx context.Context, id int, amount int) error {
	_, err := r.DB.Exec(ctx,
		`UPDATE room_entry_gatars
         SET quantity = quantity - $1, updated_at = NOW()
         WHERE id = $2 AND quantity >= $1`, amount, id)
	return err
}

// DeleteByRoomEntryID removes all gatar entries for a room entry
func (r *RoomEntryGatarRepository) DeleteByRoomEntryID(ctx context.Context, roomEntryID int) error {
	_, err := r.DB.Exec(ctx, `DELETE FROM room_entry_gatars WHERE room_entry_id = $1`, roomEntryID)
	return err
}

// UpdateBatch replaces all gatar entries for a room entry
func (r *RoomEntryGatarRepository) UpdateBatch(ctx context.Context, roomEntryID int, gatars []models.GatarInput) error {
	// Delete existing and insert new
	if err := r.DeleteByRoomEntryID(ctx, roomEntryID); err != nil {
		return err
	}
	return r.CreateBatch(ctx, roomEntryID, gatars)
}

// GatarSearchResult represents a search result with gatar-level details
type GatarSearchResult struct {
	RoomEntryID int    `json:"room_entry_id"`
	EntryID     int    `json:"entry_id"`
	ThockNumber string `json:"thock_number"`
	RoomNo      string `json:"room_no"`
	Floor       string `json:"floor"`
	GatarNo     int    `json:"gatar_no"`
	Quantity    int    `json:"quantity"`
	Quality     string `json:"quality"`
	Variety     string `json:"variety"`
	CustomerID  int    `json:"customer_id"`
}

// SearchByGatar finds all entries in a specific gatar location
func (r *RoomEntryGatarRepository) SearchByGatar(ctx context.Context, roomNo string, floor string, gatarNo int) ([]GatarSearchResult, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT reg.room_entry_id, re.entry_id, re.thock_number, re.room_no, re.floor,
		        reg.gatar_no, reg.quantity, COALESCE(reg.quality, ''),
		        COALESCE(e.remark, '') as variety, COALESCE(e.customer_id, 0)
         FROM room_entry_gatars reg
         JOIN room_entries re ON reg.room_entry_id = re.id
         LEFT JOIN entries e ON re.entry_id = e.id
         WHERE re.room_no = $1 AND re.floor = $2 AND reg.gatar_no = $3
           AND COALESCE(e.status, 'active') != 'deleted'
           AND reg.quantity > 0
         ORDER BY reg.created_at DESC`, roomNo, floor, gatarNo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []GatarSearchResult
	for rows.Next() {
		var r GatarSearchResult
		err := rows.Scan(&r.RoomEntryID, &r.EntryID, &r.ThockNumber, &r.RoomNo, &r.Floor,
			&r.GatarNo, &r.Quantity, &r.Quality, &r.Variety, &r.CustomerID)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}
