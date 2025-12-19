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

// GetAllPickups retrieves all pickups with customer and gate pass info for activity log
func (r *GatePassPickupRepository) GetAllPickups(ctx context.Context) ([]map[string]interface{}, error) {
	query := `
		SELECT
			gpp.id, gpp.gate_pass_id, gpp.pickup_quantity, gpp.picked_up_by_user_id,
			gpp.pickup_time, gpp.room_no, gpp.floor, gpp.remarks, gpp.created_at,
			COALESCE(u.name, '') as picked_up_by_user_name,
			gp.thock_number,
			COALESCE(c.name, '') as customer_name,
			COALESCE(c.phone, '') as customer_phone,
			COALESCE(c.village, '') as customer_village
		FROM gate_pass_pickups gpp
		LEFT JOIN users u ON gpp.picked_up_by_user_id = u.id
		LEFT JOIN gate_passes gp ON gpp.gate_pass_id = gp.id
		LEFT JOIN customers c ON gp.customer_id = c.id
		ORDER BY gpp.pickup_time DESC
	`

	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pickups []map[string]interface{}
	for rows.Next() {
		var id, gatePassID, pickupQuantity, pickedUpByUserID int
		var pickupTime, createdAt interface{}
		var roomNo, floor, remarks, pickedUpByUserName, thockNumber, customerName, customerPhone, customerVillage *string

		err := rows.Scan(
			&id, &gatePassID, &pickupQuantity, &pickedUpByUserID,
			&pickupTime, &roomNo, &floor, &remarks, &createdAt,
			&pickedUpByUserName, &thockNumber, &customerName, &customerPhone, &customerVillage,
		)
		if err != nil {
			return nil, err
		}

		pickup := map[string]interface{}{
			"id":                    id,
			"gate_pass_id":          gatePassID,
			"pickup_quantity":       pickupQuantity,
			"picked_up_by_user_id":  pickedUpByUserID,
			"pickup_time":           pickupTime,
			"created_at":            createdAt,
			"type":                  "out",
		}

		if roomNo != nil {
			pickup["room_no"] = *roomNo
		}
		if floor != nil {
			pickup["floor"] = *floor
		}
		if remarks != nil {
			pickup["remarks"] = *remarks
		}
		if pickedUpByUserName != nil {
			pickup["picked_up_by_user_name"] = *pickedUpByUserName
		}
		if thockNumber != nil {
			pickup["thock_number"] = *thockNumber
		}
		if customerName != nil {
			pickup["customer_name"] = *customerName
		}
		if customerPhone != nil {
			pickup["customer_phone"] = *customerPhone
		}
		if customerVillage != nil {
			pickup["customer_village"] = *customerVillage
		}

		pickups = append(pickups, pickup)
	}

	return pickups, rows.Err()
}

// GetPickupsByThockNumber retrieves all pickups for a thock number (across all gate passes)
func (r *GatePassPickupRepository) GetPickupsByThockNumber(ctx context.Context, thockNumber string) ([]models.GatePassPickup, error) {
	query := `
		SELECT
			gpp.id, gpp.gate_pass_id, gpp.pickup_quantity, gpp.picked_up_by_user_id,
			gpp.pickup_time, gpp.room_no, gpp.floor, gpp.remarks, gpp.created_at,
			u.name as picked_up_by_user_name
		FROM gate_pass_pickups gpp
		LEFT JOIN users u ON gpp.picked_up_by_user_id = u.id
		LEFT JOIN gate_passes gp ON gpp.gate_pass_id = gp.id
		WHERE gp.thock_number = $1
		ORDER BY gpp.pickup_time DESC
	`

	rows, err := r.DB.Query(ctx, query, thockNumber)
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

	// Fetch gatar breakdown for each pickup
	for i := range pickups {
		gatars, err := r.GetGatarBreakdownByPickupID(ctx, pickups[i].ID)
		if err == nil {
			pickups[i].GatarBreakdown = gatars
		}
	}

	return pickups, rows.Err()
}

// CreateGatarBreakdown inserts gatar breakdown records for a pickup
func (r *GatePassPickupRepository) CreateGatarBreakdown(ctx context.Context, pickupID int, gatars []models.GatarBreakdown) error {
	if len(gatars) == 0 {
		return nil
	}

	query := `
		INSERT INTO gate_pass_pickup_gatars (pickup_id, gatar_no, quantity)
		VALUES ($1, $2, $3)
	`

	for _, g := range gatars {
		_, err := r.DB.Exec(ctx, query, pickupID, g.GatarNo, g.Quantity)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetGatarBreakdownByPickupID retrieves gatar breakdown for a pickup
func (r *GatePassPickupRepository) GetGatarBreakdownByPickupID(ctx context.Context, pickupID int) ([]models.GatarBreakdown, error) {
	query := `
		SELECT id, pickup_id, gatar_no, quantity, created_at
		FROM gate_pass_pickup_gatars
		WHERE pickup_id = $1
		ORDER BY gatar_no
	`

	rows, err := r.DB.Query(ctx, query, pickupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var gatars []models.GatarBreakdown
	for rows.Next() {
		var g models.GatarBreakdown
		err := rows.Scan(&g.ID, &g.PickupID, &g.GatarNo, &g.Quantity, &g.CreatedAt)
		if err != nil {
			return nil, err
		}
		gatars = append(gatars, g)
	}

	return gatars, rows.Err()
}
