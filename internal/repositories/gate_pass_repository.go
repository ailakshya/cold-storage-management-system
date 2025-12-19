package repositories

import (
	"context"
	"fmt"

	"cold-backend/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type GatePassRepository struct {
	DB *pgxpool.Pool
}

func NewGatePassRepository(db *pgxpool.Pool) *GatePassRepository {
	return &GatePassRepository{DB: db}
}

// CheckDuplicateGatePass checks if a similar gate pass was created within the last 10 seconds
// Returns true if a duplicate is found
func (r *GatePassRepository) CheckDuplicateGatePass(ctx context.Context, customerID int, thockNumber string, requestedQty int) (bool, error) {
	query := `
		SELECT COUNT(*) FROM gate_passes
		WHERE customer_id = $1
		AND thock_number = $2
		AND requested_quantity = $3
		AND created_at > NOW() - INTERVAL '10 seconds'
	`
	var count int
	err := r.DB.QueryRow(ctx, query, customerID, thockNumber, requestedQty).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// CreateGatePass creates a new gate pass with 30-hour expiration
func (r *GatePassRepository) CreateGatePass(ctx context.Context, gatePass *models.GatePass) error {
	// Check for duplicate gate pass (same customer, same thock, same quantity within 10 seconds)
	isDuplicate, err := r.CheckDuplicateGatePass(ctx, gatePass.CustomerID, gatePass.ThockNumber, gatePass.RequestedQuantity)
	if err != nil {
		return fmt.Errorf("failed to check for duplicate gate pass: %w", err)
	}
	if isDuplicate {
		return fmt.Errorf("duplicate gate pass detected: a gate pass for %s with %d items was already created within the last 10 seconds", gatePass.ThockNumber, gatePass.RequestedQuantity)
	}

	query := `
		INSERT INTO gate_passes (
			customer_id, thock_number, entry_id, requested_quantity,
			payment_verified, payment_amount, issued_by_user_id, remarks,
			expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, CURRENT_TIMESTAMP + INTERVAL '30 hours')
		RETURNING id, issued_at, expires_at, created_at, updated_at
	`

	return r.DB.QueryRow(ctx, query,
		gatePass.CustomerID, gatePass.ThockNumber, gatePass.EntryID,
		gatePass.RequestedQuantity, gatePass.PaymentVerified,
		gatePass.PaymentAmount, gatePass.IssuedByUserID, gatePass.Remarks,
	).Scan(&gatePass.ID, &gatePass.IssuedAt, &gatePass.ExpiresAt, &gatePass.CreatedAt, &gatePass.UpdatedAt)
}

// GetGatePass retrieves a gate pass by ID
func (r *GatePassRepository) GetGatePass(ctx context.Context, id int) (*models.GatePass, error) {
	query := `
		SELECT id, customer_id, thock_number, entry_id, requested_quantity,
		       approved_quantity, gate_no, status, payment_verified, payment_amount,
		       issued_by_user_id, approved_by_user_id, issued_at, expires_at, completed_at,
		       remarks, created_at, updated_at, total_picked_up, approval_expires_at, final_approved_quantity
		FROM gate_passes
		WHERE id = $1
	`

	gatePass := &models.GatePass{}
	err := r.DB.QueryRow(ctx, query, id).Scan(
		&gatePass.ID, &gatePass.CustomerID, &gatePass.ThockNumber, &gatePass.EntryID,
		&gatePass.RequestedQuantity, &gatePass.ApprovedQuantity, &gatePass.GateNo,
		&gatePass.Status, &gatePass.PaymentVerified, &gatePass.PaymentAmount,
		&gatePass.IssuedByUserID, &gatePass.ApprovedByUserID, &gatePass.IssuedAt,
		&gatePass.ExpiresAt, &gatePass.CompletedAt, &gatePass.Remarks, &gatePass.CreatedAt, &gatePass.UpdatedAt,
		&gatePass.TotalPickedUp, &gatePass.ApprovalExpiresAt, &gatePass.FinalApprovedQuantity,
	)

	if err != nil {
		return nil, err
	}

	return gatePass, nil
}

// ListAllGatePasses retrieves all gate passes with customer and user details
func (r *GatePassRepository) ListAllGatePasses(ctx context.Context) ([]map[string]interface{}, error) {
	query := `
		SELECT
			gp.id, gp.thock_number, gp.requested_quantity, gp.approved_quantity,
			gp.gate_no, gp.status, gp.payment_verified, gp.payment_amount,
			gp.issued_at, gp.expires_at, gp.completed_at, gp.remarks,
			gp.total_picked_up, gp.approval_expires_at, gp.final_approved_quantity,
			COALESCE(gp.request_source, 'employee') as request_source,
			gp.created_by_customer_id,
			c.id as customer_id, c.name as customer_name, c.phone as customer_phone,
			c.village as customer_village,
			e.id as entry_id, e.expected_quantity as entry_quantity,
			iu.id as issued_by_id, iu.name as issued_by_name,
			au.id as approved_by_id, au.name as approved_by_name
		FROM gate_passes gp
		JOIN customers c ON gp.customer_id = c.id
		LEFT JOIN entries e ON gp.entry_id = e.id
		LEFT JOIN users iu ON gp.issued_by_user_id = iu.id
		LEFT JOIN users au ON gp.approved_by_user_id = au.id
		ORDER BY gp.issued_at DESC
	`

	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var gatePasses []map[string]interface{}
	for rows.Next() {
		var gatePass map[string]interface{} = make(map[string]interface{})

		var (
			id, customerID, totalPickedUp int
			thockNumber, status, customerName, customerPhone, customerVillage, requestSource string
			requestedQty int
			approvedQty, gateNo, remarks, approvedByName, issuedByName *string
			entryID, approvedByID, entryQty, finalApprovedQty, createdByCustomerID, issuedByID *int
			paymentVerified bool
			paymentAmount *float64
			issuedAt interface{}
			expiresAt, approvalExpiresAt *interface{}
			completedAt *interface{}
		)

		err := rows.Scan(
			&id, &thockNumber, &requestedQty, &approvedQty, &gateNo, &status,
			&paymentVerified, &paymentAmount, &issuedAt, &expiresAt, &completedAt, &remarks,
			&totalPickedUp, &approvalExpiresAt, &finalApprovedQty,
			&requestSource, &createdByCustomerID,
			&customerID, &customerName, &customerPhone, &customerVillage,
			&entryID, &entryQty,
			&issuedByID, &issuedByName,
			&approvedByID, &approvedByName,
		)
		if err != nil {
			return nil, err
		}

		gatePass["id"] = id
		gatePass["thock_number"] = thockNumber
		gatePass["requested_quantity"] = requestedQty
		gatePass["status"] = status
		gatePass["payment_verified"] = paymentVerified
		gatePass["issued_at"] = issuedAt
		gatePass["customer_id"] = customerID
		gatePass["customer_name"] = customerName
		gatePass["customer_phone"] = customerPhone
		gatePass["customer_village"] = customerVillage
		gatePass["total_picked_up"] = totalPickedUp
		gatePass["request_source"] = requestSource

		if issuedByID != nil {
			gatePass["issued_by_id"] = *issuedByID
		}
		if issuedByName != nil {
			gatePass["issued_by_name"] = *issuedByName
		}
		if createdByCustomerID != nil {
			gatePass["created_by_customer_id"] = *createdByCustomerID
		}

		if approvedQty != nil {
			gatePass["approved_quantity"] = *approvedQty
		}
		if gateNo != nil {
			gatePass["gate_no"] = *gateNo
		}
		if paymentAmount != nil {
			gatePass["payment_amount"] = *paymentAmount
		}
		if expiresAt != nil {
			gatePass["expires_at"] = *expiresAt
		}
		if approvalExpiresAt != nil {
			gatePass["approval_expires_at"] = *approvalExpiresAt
		}
		if completedAt != nil {
			gatePass["completed_at"] = *completedAt
		}
		if remarks != nil {
			gatePass["remarks"] = *remarks
		}
		if entryID != nil {
			gatePass["entry_id"] = *entryID
		}
		if entryQty != nil {
			gatePass["entry_quantity"] = *entryQty
		}
		if finalApprovedQty != nil {
			gatePass["final_approved_quantity"] = *finalApprovedQty
		}
		if approvedByID != nil && approvedByName != nil {
			gatePass["approved_by_id"] = *approvedByID
			gatePass["approved_by_name"] = *approvedByName
		}

		gatePasses = append(gatePasses, gatePass)
	}

	return gatePasses, rows.Err()
}

// ListPendingGatePasses retrieves gate passes with status 'pending' and calculates expiration
func (r *GatePassRepository) ListPendingGatePasses(ctx context.Context) ([]map[string]interface{}, error) {
	query := `
		SELECT
			gp.id, gp.thock_number, gp.requested_quantity, gp.gate_no,
			gp.payment_verified, gp.payment_amount, gp.issued_at, gp.expires_at, gp.remarks,
			(gp.expires_at IS NOT NULL AND CURRENT_TIMESTAMP > gp.expires_at) as is_expired,
			COALESCE(gp.request_source, 'employee') as request_source,
			gp.created_by_customer_id,
			c.id as customer_id, c.name as customer_name, c.phone as customer_phone,
			c.village as customer_village,
			e.id as entry_id, e.expected_quantity as entry_quantity,
			iu.name as issued_by_name,
			(SELECT string_agg(DISTINCT room_no, ', ' ORDER BY room_no) FROM room_entries WHERE thock_number = gp.thock_number) as rooms,
			(SELECT string_agg(DISTINCT floor, ', ' ORDER BY floor) FROM room_entries WHERE thock_number = gp.thock_number) as floors,
			(SELECT string_agg(DISTINCT gate_no, ', ') FROM room_entries WHERE thock_number = gp.thock_number) as gatars,
			(SELECT COALESCE(SUM(quantity), 0) FROM room_entries WHERE thock_number = gp.thock_number) as total_qty,
			(SELECT string_agg(quantity_breakdown, ', ') FROM room_entries WHERE thock_number = gp.thock_number) as qty_breakdown,
			(SELECT string_agg(DISTINCT NULLIF(remark, ''), ', ') FROM room_entries WHERE thock_number = gp.thock_number) as remark
		FROM gate_passes gp
		JOIN customers c ON gp.customer_id = c.id
		LEFT JOIN entries e ON gp.entry_id = e.id
		LEFT JOIN users iu ON gp.issued_by_user_id = iu.id
		WHERE gp.status = 'pending'
		ORDER BY gp.issued_at ASC
	`

	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var gatePasses []map[string]interface{}
	for rows.Next() {
		var gatePass map[string]interface{} = make(map[string]interface{})

		var (
			id, customerID int
			thockNumber, customerName, customerPhone, requestSource string
			requestedQty int
			gateNo, remarks, issuedByName, customerVillage, rooms, floors, gatars, qtyBreakdown, remark *string
			entryID, entryQty, createdByCustomerID, totalQty *int
			paymentVerified, isExpired bool
			paymentAmount *float64
			issuedAt, expiresAt interface{}
		)

		err := rows.Scan(
			&id, &thockNumber, &requestedQty, &gateNo,
			&paymentVerified, &paymentAmount, &issuedAt, &expiresAt, &remarks, &isExpired,
			&requestSource, &createdByCustomerID,
			&customerID, &customerName, &customerPhone, &customerVillage,
			&entryID, &entryQty, &issuedByName,
			&rooms, &floors, &gatars, &totalQty, &qtyBreakdown, &remark,
		)
		if err != nil {
			return nil, err
		}

		gatePass["id"] = id
		gatePass["thock_number"] = thockNumber
		gatePass["requested_quantity"] = requestedQty
		gatePass["payment_verified"] = paymentVerified
		gatePass["issued_at"] = issuedAt
		gatePass["expires_at"] = expiresAt
		gatePass["is_expired"] = isExpired
		gatePass["customer_id"] = customerID
		gatePass["customer_name"] = customerName
		gatePass["customer_phone"] = customerPhone
		gatePass["request_source"] = requestSource

		if issuedByName != nil {
			gatePass["issued_by_name"] = *issuedByName
		}
		if createdByCustomerID != nil {
			gatePass["created_by_customer_id"] = *createdByCustomerID
		}
		if customerVillage != nil {
			gatePass["customer_village"] = *customerVillage
		}

		if gateNo != nil {
			gatePass["gate_no"] = *gateNo
		}
		if paymentAmount != nil {
			gatePass["payment_amount"] = *paymentAmount
		}
		if remarks != nil {
			gatePass["remarks"] = *remarks
		}
		if entryID != nil {
			gatePass["entry_id"] = *entryID
		}
		if entryQty != nil {
			gatePass["entry_quantity"] = *entryQty
		}
		if rooms != nil {
			gatePass["rooms"] = *rooms
		}
		if floors != nil {
			gatePass["floors"] = *floors
		}
		if gatars != nil {
			gatePass["gatars"] = *gatars
		}
		if totalQty != nil {
			gatePass["total_qty"] = *totalQty
		}
		if qtyBreakdown != nil {
			gatePass["qty_breakdown"] = *qtyBreakdown
		}
		if remark != nil {
			gatePass["remark"] = *remark
		}

		gatePasses = append(gatePasses, gatePass)
	}

	return gatePasses, rows.Err()
}

// UpdateGatePass updates gate pass details and sets 15-hour expiration when approved
func (r *GatePassRepository) UpdateGatePass(ctx context.Context, id int, approvedQty int, gateNo, status, remarks string, approvedByUserID int) error {
	query := `
		UPDATE gate_passes
		SET approved_quantity = $1, gate_no = $2, status = $3::text, remarks = $4,
		    approved_by_user_id = $5,
		    approval_expires_at = CASE WHEN $3::text = 'approved' THEN CURRENT_TIMESTAMP + INTERVAL '15 hours' ELSE approval_expires_at END,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $6
	`

	_, err := r.DB.Exec(ctx, query, approvedQty, gateNo, status, remarks, approvedByUserID, id)
	return err
}

// UpdateGatePassWithSource updates gate pass details including request_source
func (r *GatePassRepository) UpdateGatePassWithSource(ctx context.Context, id int, approvedQty int, gateNo, status, requestSource, remarks string, approvedByUserID int) error {
	query := `
		UPDATE gate_passes
		SET approved_quantity = $1, gate_no = $2, status = $3::text, request_source = $4, remarks = $5,
		    approved_by_user_id = $6,
		    approval_expires_at = CASE WHEN $3::text = 'approved' THEN CURRENT_TIMESTAMP + INTERVAL '15 hours' ELSE approval_expires_at END,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $7
	`

	_, err := r.DB.Exec(ctx, query, approvedQty, gateNo, status, requestSource, remarks, approvedByUserID, id)
	return err
}

// UpdatePickupQuantity updates the total picked up quantity
func (r *GatePassRepository) UpdatePickupQuantity(ctx context.Context, gatePassID int, additionalQty int) error {
	query := `
		UPDATE gate_passes
		SET total_picked_up = total_picked_up + $1,
		    status = CASE
		        WHEN total_picked_up + $1 >= requested_quantity THEN 'completed'
		        WHEN total_picked_up + $1 > 0 THEN 'partially_completed'
		        ELSE status
		    END,
		    completed_at = CASE
		        WHEN total_picked_up + $1 >= requested_quantity THEN CURRENT_TIMESTAMP
		        ELSE completed_at
		    END,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`

	_, err := r.DB.Exec(ctx, query, additionalQty, gatePassID)
	return err
}

// ExpireGatePasses marks gate passes as expired if approval period has passed
func (r *GatePassRepository) ExpireGatePasses(ctx context.Context) error {
	query := `
		UPDATE gate_passes
		SET status = 'expired',
		    final_approved_quantity = total_picked_up,
		    updated_at = CURRENT_TIMESTAMP
		WHERE approval_expires_at IS NOT NULL
		  AND approval_expires_at < CURRENT_TIMESTAMP
		  AND status IN ('approved', 'partially_completed')
	`

	_, err := r.DB.Exec(ctx, query)
	return err
}

// GetExpiredGatePasses retrieves recently expired gate passes for admin logs
func (r *GatePassRepository) GetExpiredGatePasses(ctx context.Context) ([]map[string]interface{}, error) {
	query := `
		SELECT
			gp.id, gp.thock_number, gp.requested_quantity, gp.total_picked_up,
			gp.final_approved_quantity, gp.approval_expires_at, gp.updated_at,
			c.id as customer_id, c.name as customer_name, c.phone as customer_phone,
			(gp.requested_quantity - gp.total_picked_up) as remaining_quantity
		FROM gate_passes gp
		JOIN customers c ON gp.customer_id = c.id
		WHERE gp.status = 'expired'
		  AND gp.updated_at > CURRENT_TIMESTAMP - INTERVAL '7 days'
		ORDER BY gp.updated_at DESC
	`

	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var expiredPasses []map[string]interface{}
	for rows.Next() {
		var expiredPass map[string]interface{} = make(map[string]interface{})

		var (
			id, customerID, requestedQty, totalPickedUp, remainingQty int
			finalApprovedQty                                          *int
			thockNumber, customerName, customerPhone                  string
			approvalExpiresAt, updatedAt                              interface{}
		)

		err := rows.Scan(
			&id, &thockNumber, &requestedQty, &totalPickedUp,
			&finalApprovedQty, &approvalExpiresAt, &updatedAt,
			&customerID, &customerName, &customerPhone,
			&remainingQty,
		)
		if err != nil {
			return nil, err
		}

		expiredPass["id"] = id
		expiredPass["thock_number"] = thockNumber
		expiredPass["requested_quantity"] = requestedQty
		expiredPass["total_picked_up"] = totalPickedUp
		expiredPass["remaining_quantity"] = remainingQty
		expiredPass["customer_id"] = customerID
		expiredPass["customer_name"] = customerName
		expiredPass["customer_phone"] = customerPhone
		expiredPass["approval_expires_at"] = approvalExpiresAt
		expiredPass["updated_at"] = updatedAt

		if finalApprovedQty != nil {
			expiredPass["final_approved_quantity"] = *finalApprovedQty
		}

		expiredPasses = append(expiredPasses, expiredPass)
	}

	return expiredPasses, rows.Err()
}

// CompleteGatePass marks gate pass as completed
func (r *GatePassRepository) CompleteGatePass(ctx context.Context, id int) error {
	query := `
		UPDATE gate_passes
		SET status = 'completed', completed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`

	_, err := r.DB.Exec(ctx, query, id)
	return err
}

// CreateCustomerGatePass creates a gate pass from customer portal (status = pending, no expiration)
func (r *GatePassRepository) CreateCustomerGatePass(ctx context.Context, customerID int, thockNumber string, requestedQuantity int, remarks string, entryID int) (*models.GatePass, error) {
	// Check for duplicate gate pass (same customer, same thock, same quantity within 10 seconds)
	isDuplicate, err := r.CheckDuplicateGatePass(ctx, customerID, thockNumber, requestedQuantity)
	if err != nil {
		return nil, fmt.Errorf("failed to check for duplicate gate pass: %w", err)
	}
	if isDuplicate {
		return nil, fmt.Errorf("duplicate request detected: a gate pass for %s with %d items was already created within the last 10 seconds", thockNumber, requestedQuantity)
	}

	query := `
		INSERT INTO gate_passes (
			customer_id, thock_number, entry_id, requested_quantity,
			payment_verified, status, created_by_customer_id, request_source, remarks
		) VALUES ($1, $2, $3, $4, false, 'pending', $5, 'customer_portal', $6)
		RETURNING id, issued_at, created_at, updated_at
	`

	gatePass := &models.GatePass{
		CustomerID:          customerID,
		ThockNumber:         thockNumber,
		RequestedQuantity:   requestedQuantity,
		Status:              "pending",
		PaymentVerified:     false,
		CreatedByCustomerID: &customerID,
		RequestSource:       "customer_portal",
	}

	if remarks != "" {
		gatePass.Remarks = &remarks
	}

	err = r.DB.QueryRow(ctx, query, customerID, thockNumber, entryID, requestedQuantity, customerID, remarks).Scan(
		&gatePass.ID, &gatePass.IssuedAt, &gatePass.CreatedAt, &gatePass.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return gatePass, nil
}

// ListByCustomerID retrieves all gate passes for a customer
func (r *GatePassRepository) ListByCustomerID(ctx context.Context, customerID int) ([]map[string]interface{}, error) {
	query := `
		SELECT
			gp.id, gp.thock_number, gp.requested_quantity, gp.approved_quantity,
			gp.gate_no, gp.status, gp.payment_verified, gp.payment_amount,
			gp.issued_at, gp.expires_at, gp.completed_at, gp.remarks,
			gp.total_picked_up, gp.approval_expires_at, gp.final_approved_quantity,
			gp.request_source,
			e.id as entry_id, e.expected_quantity as entry_quantity,
			au.name as approved_by_name
		FROM gate_passes gp
		LEFT JOIN entries e ON gp.entry_id = e.id
		LEFT JOIN users au ON gp.approved_by_user_id = au.id
		WHERE gp.customer_id = $1
		ORDER BY gp.issued_at DESC
	`

	rows, err := r.DB.Query(ctx, query, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var gatePasses []map[string]interface{}
	for rows.Next() {
		var gatePass map[string]interface{} = make(map[string]interface{})

		var (
			id, requestedQty, totalPickedUp int
			thockNumber, status, requestSource string
			approvedQty, gateNo, remarks, approvedByName *string
			entryID, entryQty, finalApprovedQty *int
			paymentVerified bool
			paymentAmount *float64
			issuedAt interface{}
			expiresAt, approvalExpiresAt, completedAt *interface{}
		)

		err := rows.Scan(
			&id, &thockNumber, &requestedQty, &approvedQty, &gateNo, &status,
			&paymentVerified, &paymentAmount, &issuedAt, &expiresAt, &completedAt, &remarks,
			&totalPickedUp, &approvalExpiresAt, &finalApprovedQty, &requestSource,
			&entryID, &entryQty, &approvedByName,
		)
		if err != nil {
			return nil, err
		}

		gatePass["id"] = id
		gatePass["thock_number"] = thockNumber
		gatePass["requested_quantity"] = requestedQty
		gatePass["status"] = status
		gatePass["payment_verified"] = paymentVerified
		gatePass["issued_at"] = issuedAt
		gatePass["total_picked_up"] = totalPickedUp
		gatePass["request_source"] = requestSource

		if approvedQty != nil {
			gatePass["approved_quantity"] = *approvedQty
		}
		if gateNo != nil {
			gatePass["gate_no"] = *gateNo
		}
		if paymentAmount != nil {
			gatePass["payment_amount"] = *paymentAmount
		}
		if expiresAt != nil {
			gatePass["expires_at"] = *expiresAt
		}
		if approvalExpiresAt != nil {
			gatePass["approval_expires_at"] = *approvalExpiresAt
		}
		if completedAt != nil {
			gatePass["completed_at"] = *completedAt
		}
		if remarks != nil {
			gatePass["remarks"] = *remarks
		}
		if entryID != nil {
			gatePass["entry_id"] = *entryID
		}
		if entryQty != nil {
			gatePass["entry_quantity"] = *entryQty
		}
		if finalApprovedQty != nil {
			gatePass["final_approved_quantity"] = *finalApprovedQty
		}
		if approvedByName != nil {
			gatePass["approved_by_name"] = *approvedByName
		}

		gatePasses = append(gatePasses, gatePass)
	}

	return gatePasses, rows.Err()
}

// GetTotalApprovedQuantityForEntry calculates the total approved quantity
// across all completed and approved gate passes for a specific entry
// This is used to prevent overselling - customer can't request more than available stock
func (r *GatePassRepository) GetTotalApprovedQuantityForEntry(ctx context.Context, entryID int) (int, error) {
	query := `
		SELECT COALESCE(SUM(
			CASE
				WHEN approved_quantity IS NOT NULL THEN approved_quantity
				ELSE requested_quantity
			END
		), 0)
		FROM gate_passes
		WHERE entry_id = $1
		AND status IN ('approved', 'completed', 'partially_completed')
	`

	var totalApproved int
	err := r.DB.QueryRow(ctx, query, entryID).Scan(&totalApproved)
	if err != nil {
		return 0, err
	}

	return totalApproved, nil
}

// GetPendingQuantityForEntry calculates the remaining quantity yet to be picked up
// from pending, approved, and partially_completed gate passes
// This is used to show accurate "Can Take Out" values
func (r *GatePassRepository) GetPendingQuantityForEntry(ctx context.Context, entryID int) (int, error) {
	query := `
		SELECT COALESCE(SUM(
			COALESCE(approved_quantity, requested_quantity) - total_picked_up
		), 0)
		FROM gate_passes
		WHERE entry_id = $1
		AND status IN ('pending', 'approved', 'partially_completed')
	`

	var pendingQty int
	err := r.DB.QueryRow(ctx, query, entryID).Scan(&pendingQty)
	if err != nil {
		return 0, err
	}

	return pendingQty, nil
}
