package repositories

import (
	"context"
	"cold-backend/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type GatePassPickupRepository struct {
	DB *pgxpool.Pool
}

func NewGatePassPickupRepository(db *pgxpool.Pool) *GatePassPickupRepository {
	return &GatePassPickupRepository{DB: db}
}

// CreatePickup creates a new pickup record
func (r *GatePassPickupRepository) CreatePickup(ctx context.Context, pickup *models.GatePassPickup) error {
	query := `
		INSERT INTO gate_pass_pickups (
			gate_pass_id, pickup_quantity, picked_up_by_user_id, room_no, floor, remarks
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, pickup_time, created_at
	`

	return r.DB.QueryRow(ctx, query,
		pickup.GatePassID, pickup.PickupQuantity, pickup.PickedUpByUserID,
		pickup.RoomNo, pickup.Floor, pickup.Remarks,
	).Scan(&pickup.ID, &pickup.PickupTime, &pickup.CreatedAt)
}

// GetPickupsByGatePassID retrieves all pickups for a gate pass
func (r *GatePassPickupRepository) GetPickupsByGatePassID(ctx context.Context, gatePassID int) ([]models.GatePassPickup, error) {
	query := `
		SELECT
			gpp.id, gpp.gate_pass_id, gpp.pickup_quantity, gpp.picked_up_by_user_id,
			gpp.pickup_time, gpp.room_no, gpp.floor, gpp.remarks, gpp.created_at,
			u.name as picked_up_by_user_name
		FROM gate_pass_pickups gpp
		LEFT JOIN users u ON gpp.picked_up_by_user_id = u.id
		WHERE gpp.gate_pass_id = $1
		ORDER BY gpp.pickup_time DESC
	`

	rows, err := r.DB.Query(ctx, query, gatePassID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pickups []models.GatePassPickup
	for rows.Next() {
		var pickup models.GatePassPickup
		err := rows.Scan(
			&pickup.ID, &pickup.GatePassID, &pickup.PickupQuantity, &pickup.PickedUpByUserID,
			&pickup.PickupTime, &pickup.RoomNo, &pickup.Floor, &pickup.Remarks, &pickup.CreatedAt,
			&pickup.PickedUpByUserName,
		)
		if err != nil {
			return nil, err
		}
		pickups = append(pickups, pickup)
	}

	return pickups, rows.Err()
}
