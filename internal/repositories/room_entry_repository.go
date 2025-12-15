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
		`INSERT INTO room_entries(entry_id, truck_number, room_no, floor, gate_no, remark, quantity, created_by_user_id)
         VALUES($1, $2, $3, $4, $5, $6, $7, $8)
         RETURNING id, created_at, updated_at`,
		re.EntryID, re.TruckNumber, re.RoomNo, re.Floor, re.GateNo, re.Remark, re.Quantity, re.CreatedByUserID,
	).Scan(&re.ID, &re.CreatedAt, &re.UpdatedAt)
}

func (r *RoomEntryRepository) Get(ctx context.Context, id int) (*models.RoomEntry, error) {
	row := r.DB.QueryRow(ctx,
		`SELECT id, entry_id, truck_number, room_no, floor, gate_no, remark, quantity, created_by_user_id, created_at, updated_at
         FROM room_entries WHERE id=$1`, id)

	var re models.RoomEntry
	err := row.Scan(&re.ID, &re.EntryID, &re.TruckNumber, &re.RoomNo, &re.Floor,
		&re.GateNo, &re.Remark, &re.Quantity, &re.CreatedByUserID, &re.CreatedAt, &re.UpdatedAt)
	return &re, err
}

func (r *RoomEntryRepository) List(ctx context.Context) ([]*models.RoomEntry, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT id, entry_id, truck_number, room_no, floor, gate_no, remark, quantity, created_by_user_id, created_at, updated_at
         FROM room_entries ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roomEntries []*models.RoomEntry
	for rows.Next() {
		var re models.RoomEntry
		err := rows.Scan(&re.ID, &re.EntryID, &re.TruckNumber, &re.RoomNo, &re.Floor,
			&re.GateNo, &re.Remark, &re.Quantity, &re.CreatedByUserID, &re.CreatedAt, &re.UpdatedAt)
		if err != nil {
			return nil, err
		}
		roomEntries = append(roomEntries, &re)
	}
	return roomEntries, nil
}

func (r *RoomEntryRepository) GetByEntryID(ctx context.Context, entryID int) (*models.RoomEntry, error) {
	row := r.DB.QueryRow(ctx,
		`SELECT id, entry_id, truck_number, room_no, floor, gate_no, remark, quantity, created_by_user_id, created_at, updated_at
         FROM room_entries WHERE entry_id=$1`, entryID)

	var re models.RoomEntry
	err := row.Scan(&re.ID, &re.EntryID, &re.TruckNumber, &re.RoomNo, &re.Floor,
		&re.GateNo, &re.Remark, &re.Quantity, &re.CreatedByUserID, &re.CreatedAt, &re.UpdatedAt)
	return &re, err
}

func (r *RoomEntryRepository) Update(ctx context.Context, id int, re *models.RoomEntry) error {
	return r.DB.QueryRow(ctx,
		`UPDATE room_entries
         SET room_no=$1, floor=$2, gate_no=$3, remark=$4, quantity=$5, updated_at=NOW()
         WHERE id=$6
         RETURNING updated_at`,
		re.RoomNo, re.Floor, re.GateNo, re.Remark, re.Quantity, id,
	).Scan(&re.UpdatedAt)
}

// ReduceQuantity reduces the quantity in a room entry (for gate pass pickups)
func (r *RoomEntryRepository) ReduceQuantity(ctx context.Context, truckNumber, roomNo, floor string, quantity int) error {
	query := `
		UPDATE room_entries
		SET quantity = quantity - $1, updated_at = NOW()
		WHERE truck_number = $2 AND room_no = $3 AND floor = $4 AND quantity >= $1
	`

	result, err := r.DB.Exec(ctx, query, quantity, truckNumber, roomNo, floor)
	if err != nil {
		return err
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return nil // Silently ignore if no matching room entry or insufficient quantity
	}

	return nil
}

// GetTotalQuantityByTruckNumber returns the current total inventory for a truck
func (r *RoomEntryRepository) GetTotalQuantityByTruckNumber(ctx context.Context, truckNumber string) (int, error) {
	var totalQuantity int
	query := `SELECT COALESCE(SUM(quantity), 0) FROM room_entries WHERE truck_number = $1`

	err := r.DB.QueryRow(ctx, query, truckNumber).Scan(&totalQuantity)
	return totalQuantity, err
}

// ListByTruckNumber returns all room entries for a specific truck
func (r *RoomEntryRepository) ListByTruckNumber(ctx context.Context, truckNumber string) ([]*models.RoomEntry, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT id, entry_id, truck_number, room_no, floor, gate_no, remark, quantity, created_by_user_id, created_at, updated_at
         FROM room_entries WHERE truck_number=$1 ORDER BY created_at DESC`, truckNumber)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roomEntries []*models.RoomEntry
	for rows.Next() {
		var re models.RoomEntry
		err := rows.Scan(&re.ID, &re.EntryID, &re.TruckNumber, &re.RoomNo, &re.Floor,
			&re.GateNo, &re.Remark, &re.Quantity, &re.CreatedByUserID, &re.CreatedAt, &re.UpdatedAt)
		if err != nil {
			return nil, err
		}
		roomEntries = append(roomEntries, &re)
	}
	return roomEntries, nil
}
