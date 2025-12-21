package repositories

import (
	"context"

	"cold-backend/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RoomEntryRepository struct {
	DB *pgxpool.Pool
}

func NewRoomEntryRepository(db *pgxpool.Pool) *RoomEntryRepository {
	return &RoomEntryRepository{DB: db}
}

func (r *RoomEntryRepository) Create(ctx context.Context, re *models.RoomEntry) error {
	return r.DB.QueryRow(ctx,
		`INSERT INTO room_entries(entry_id, thock_number, room_no, floor, gate_no, remark, quantity, quantity_breakdown, created_by_user_id)
         VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9)
         RETURNING id, created_at, updated_at`,
		re.EntryID, re.ThockNumber, re.RoomNo, re.Floor, re.GateNo, re.Remark, re.Quantity, re.QuantityBreakdown, re.CreatedByUserID,
	).Scan(&re.ID, &re.CreatedAt, &re.UpdatedAt)
}

func (r *RoomEntryRepository) Get(ctx context.Context, id int) (*models.RoomEntry, error) {
	row := r.DB.QueryRow(ctx,
		`SELECT re.id, re.entry_id, re.thock_number, re.room_no, re.floor, re.gate_no, re.remark, re.quantity,
		        COALESCE(re.quantity_breakdown, ''), re.created_by_user_id, re.created_at, re.updated_at,
		        COALESCE(e.remark, '') as variety
         FROM room_entries re
         LEFT JOIN entries e ON re.entry_id = e.id
         WHERE re.id=$1`, id)

	var re models.RoomEntry
	err := row.Scan(&re.ID, &re.EntryID, &re.ThockNumber, &re.RoomNo, &re.Floor,
		&re.GateNo, &re.Remark, &re.Quantity, &re.QuantityBreakdown, &re.CreatedByUserID, &re.CreatedAt, &re.UpdatedAt, &re.Variety)
	return &re, err
}

func (r *RoomEntryRepository) List(ctx context.Context) ([]*models.RoomEntry, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT re.id, re.entry_id, re.thock_number, re.room_no, re.floor, re.gate_no, re.remark, re.quantity,
		        COALESCE(re.quantity_breakdown, ''), re.created_by_user_id, re.created_at, re.updated_at,
		        COALESCE(e.remark, '') as variety
         FROM room_entries re
         LEFT JOIN entries e ON re.entry_id = e.id
         ORDER BY re.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roomEntries []*models.RoomEntry
	for rows.Next() {
		var re models.RoomEntry
		err := rows.Scan(&re.ID, &re.EntryID, &re.ThockNumber, &re.RoomNo, &re.Floor,
			&re.GateNo, &re.Remark, &re.Quantity, &re.QuantityBreakdown, &re.CreatedByUserID, &re.CreatedAt, &re.UpdatedAt, &re.Variety)
		if err != nil {
			return nil, err
		}
		roomEntries = append(roomEntries, &re)
	}
	return roomEntries, nil
}

func (r *RoomEntryRepository) GetByEntryID(ctx context.Context, entryID int) (*models.RoomEntry, error) {
	row := r.DB.QueryRow(ctx,
		`SELECT re.id, re.entry_id, re.thock_number, re.room_no, re.floor, re.gate_no, re.remark, re.quantity,
		        COALESCE(re.quantity_breakdown, ''), re.created_by_user_id, re.created_at, re.updated_at,
		        COALESCE(e.remark, '') as variety
         FROM room_entries re
         LEFT JOIN entries e ON re.entry_id = e.id
         WHERE re.entry_id=$1`, entryID)

	var re models.RoomEntry
	err := row.Scan(&re.ID, &re.EntryID, &re.ThockNumber, &re.RoomNo, &re.Floor,
		&re.GateNo, &re.Remark, &re.Quantity, &re.QuantityBreakdown, &re.CreatedByUserID, &re.CreatedAt, &re.UpdatedAt, &re.Variety)
	return &re, err
}

func (r *RoomEntryRepository) Update(ctx context.Context, id int, re *models.RoomEntry) error {
	return r.DB.QueryRow(ctx,
		`UPDATE room_entries
         SET room_no=$1, floor=$2, gate_no=$3, remark=$4, quantity=$5, quantity_breakdown=$6, updated_at=NOW()
         WHERE id=$7
         RETURNING updated_at`,
		re.RoomNo, re.Floor, re.GateNo, re.Remark, re.Quantity, re.QuantityBreakdown, id,
	).Scan(&re.UpdatedAt)
}

// ReduceQuantity reduces the quantity in a room entry (for gate pass pickups)
func (r *RoomEntryRepository) ReduceQuantity(ctx context.Context, thockNumber, roomNo, floor string, quantity int) error {
	query := `
		UPDATE room_entries
		SET quantity = quantity - $1, updated_at = NOW()
		WHERE thock_number = $2 AND room_no = $3 AND floor = $4 AND quantity >= $1
	`

	result, err := r.DB.Exec(ctx, query, quantity, thockNumber, roomNo, floor)
	if err != nil {
		return err
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return nil // Silently ignore if no matching room entry or insufficient quantity
	}

	return nil
}

// GetTotalQuantityByThockNumber returns the current total inventory for a truck
func (r *RoomEntryRepository) GetTotalQuantityByThockNumber(ctx context.Context, thockNumber string) (int, error) {
	var totalQuantity int
	query := `SELECT COALESCE(SUM(quantity), 0) FROM room_entries WHERE thock_number = $1`

	err := r.DB.QueryRow(ctx, query, thockNumber).Scan(&totalQuantity)
	return totalQuantity, err
}

// ListByThockNumber returns all room entries for a specific truck
func (r *RoomEntryRepository) ListByThockNumber(ctx context.Context, thockNumber string) ([]*models.RoomEntry, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT re.id, re.entry_id, re.thock_number, re.room_no, re.floor, re.gate_no, re.remark, re.quantity,
		        COALESCE(re.quantity_breakdown, ''), re.created_by_user_id, re.created_at, re.updated_at,
		        COALESCE(e.remark, '') as variety
         FROM room_entries re
         LEFT JOIN entries e ON re.entry_id = e.id
         WHERE re.thock_number=$1 ORDER BY re.created_at DESC`, thockNumber)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roomEntries []*models.RoomEntry
	for rows.Next() {
		var re models.RoomEntry
		err := rows.Scan(&re.ID, &re.EntryID, &re.ThockNumber, &re.RoomNo, &re.Floor,
			&re.GateNo, &re.Remark, &re.Quantity, &re.QuantityBreakdown, &re.CreatedByUserID, &re.CreatedAt, &re.UpdatedAt, &re.Variety)
		if err != nil {
			return nil, err
		}
		roomEntries = append(roomEntries, &re)
	}
	return roomEntries, nil
}
