package g

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	DB *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{DB: db}
}

// Item operations

func (r *Repository) CreateItem(ctx context.Context, item *Item) error {
	query := `INSERT INTO items (name, sku, floor, current_qty, unit_cost)
		VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at, updated_at`
	return r.DB.QueryRow(ctx, query,
		item.Name, item.SKU, item.Floor, item.CurrentQty, item.UnitCost,
	).Scan(&item.ID, &item.CreatedAt, &item.UpdatedAt)
}

func (r *Repository) GetItem(ctx context.Context, id int) (*Item, error) {
	item := &Item{}
	query := `SELECT id, name, sku, floor, current_qty, unit_cost, created_at, updated_at
		FROM items WHERE id = $1`
	err := r.DB.QueryRow(ctx, query, id).Scan(
		&item.ID, &item.Name, &item.SKU, &item.Floor,
		&item.CurrentQty, &item.UnitCost, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *Repository) ListItems(ctx context.Context) ([]*Item, error) {
	query := `SELECT id, name, sku, floor, current_qty, unit_cost, created_at, updated_at
		FROM items ORDER BY floor, name`
	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*Item
	for rows.Next() {
		item := &Item{}
		if err := rows.Scan(&item.ID, &item.Name, &item.SKU, &item.Floor,
			&item.CurrentQty, &item.UnitCost, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *Repository) UpdateItem(ctx context.Context, id int, name, sku string, floor int, unitCost float64) error {
	query := `UPDATE items SET name = $2, sku = $3, floor = $4, unit_cost = $5, updated_at = NOW()
		WHERE id = $1`
	_, err := r.DB.Exec(ctx, query, id, name, sku, floor, unitCost)
	return err
}

func (r *Repository) UpdateItemQty(ctx context.Context, id int, qtyChange int) error {
	query := `UPDATE items SET current_qty = current_qty + $2, updated_at = NOW() WHERE id = $1`
	_, err := r.DB.Exec(ctx, query, id, qtyChange)
	return err
}

func (r *Repository) DeleteItem(ctx context.Context, id int) error {
	query := `DELETE FROM items WHERE id = $1`
	_, err := r.DB.Exec(ctx, query, id)
	return err
}

// Transaction operations

func (r *Repository) CreateTxn(ctx context.Context, txn *Txn) error {
	query := `INSERT INTO txns (item_id, type, qty, unit_price, total, reason)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at`
	return r.DB.QueryRow(ctx, query,
		txn.ItemID, txn.Type, txn.Qty, txn.UnitPrice, txn.Total, txn.Reason,
	).Scan(&txn.ID, &txn.CreatedAt)
}

func (r *Repository) ListTxns(ctx context.Context, limit int) ([]*Txn, error) {
	query := `SELECT t.id, t.item_id, t.type, t.qty, t.unit_price, t.total, t.reason, t.created_at, i.name
		FROM txns t JOIN items i ON t.item_id = i.id ORDER BY t.created_at DESC LIMIT $1`
	rows, err := r.DB.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txns []*Txn
	for rows.Next() {
		txn := &Txn{}
		if err := rows.Scan(&txn.ID, &txn.ItemID, &txn.Type, &txn.Qty, &txn.UnitPrice,
			&txn.Total, &txn.Reason, &txn.CreatedAt, &txn.ItemName); err != nil {
			return nil, err
		}
		txns = append(txns, txn)
	}
	return txns, nil
}

func (r *Repository) ListTxnsByItem(ctx context.Context, itemID int) ([]*Txn, error) {
	query := `SELECT t.id, t.item_id, t.type, t.qty, t.unit_price, t.total, t.reason, t.created_at, i.name
		FROM txns t JOIN items i ON t.item_id = i.id WHERE t.item_id = $1 ORDER BY t.created_at DESC`
	rows, err := r.DB.Query(ctx, query, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txns []*Txn
	for rows.Next() {
		txn := &Txn{}
		if err := rows.Scan(&txn.ID, &txn.ItemID, &txn.Type, &txn.Qty, &txn.UnitPrice,
			&txn.Total, &txn.Reason, &txn.CreatedAt, &txn.ItemName); err != nil {
			return nil, err
		}
		txns = append(txns, txn)
	}
	return txns, nil
}

// Config operations

func (r *Repository) GetConfig(ctx context.Context, key string) (string, error) {
	var value string
	query := `SELECT value FROM cfg WHERE key = $1`
	err := r.DB.QueryRow(ctx, query, key).Scan(&value)
	return value, err
}

func (r *Repository) SetConfig(ctx context.Context, key, value string) error {
	query := `INSERT INTO cfg (key, value) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`
	_, err := r.DB.Exec(ctx, query, key, value)
	return err
}

// Access log operations

func (r *Repository) LogAccess(ctx context.Context, deviceHash, ip string, success bool, failReason string) error {
	query := `INSERT INTO access_log (device_hash, ip_address, success, fail_reason) VALUES ($1, $2, $3, $4)`
	_, err := r.DB.Exec(ctx, query, deviceHash, ip, success, failReason)
	return err
}

func (r *Repository) GetFailedAttempts(ctx context.Context, deviceHash string, since time.Time) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM access_log WHERE device_hash = $1 AND success = false AND created_at > $2`
	err := r.DB.QueryRow(ctx, query, deviceHash, since).Scan(&count)
	return count, err
}

// Session operations

func (r *Repository) CreateSession(ctx context.Context, token, deviceHash string, expiresAt time.Time) error {
	query := `INSERT INTO sessions (token, device_hash, expires_at) VALUES ($1, $2, $3)`
	_, err := r.DB.Exec(ctx, query, token, deviceHash, expiresAt)
	return err
}

func (r *Repository) GetSession(ctx context.Context, token string) (*Session, error) {
	session := &Session{}
	query := `SELECT id, token, device_hash, expires_at, created_at FROM sessions WHERE token = $1`
	err := r.DB.QueryRow(ctx, query, token).Scan(
		&session.ID, &session.Token, &session.DeviceHash, &session.ExpiresAt, &session.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (r *Repository) DeleteSession(ctx context.Context, token string) error {
	query := `DELETE FROM sessions WHERE token = $1`
	_, err := r.DB.Exec(ctx, query, token)
	return err
}

func (r *Repository) DeleteExpiredSessions(ctx context.Context) error {
	query := `DELETE FROM sessions WHERE expires_at < NOW()`
	_, err := r.DB.Exec(ctx, query)
	return err
}

func (r *Repository) ExtendSession(ctx context.Context, token string, newExpiry time.Time) error {
	query := `UPDATE sessions SET expires_at = $2 WHERE token = $1`
	_, err := r.DB.Exec(ctx, query, token, newExpiry)
	return err
}

// Summary operations

func (r *Repository) GetSummary(ctx context.Context) (*Summary, error) {
	summary := &Summary{}

	// Get total items and qty
	err := r.DB.QueryRow(ctx, `SELECT COUNT(*), COALESCE(SUM(current_qty), 0) FROM items`).Scan(&summary.TotalItems, &summary.TotalQty)
	if err != nil {
		return nil, err
	}

	// Get current value (qty * unit_cost)
	err = r.DB.QueryRow(ctx, `SELECT COALESCE(SUM(current_qty * unit_cost), 0) FROM items`).Scan(&summary.CurrentValue)
	if err != nil {
		return nil, err
	}

	// Get total invested (sum of all 'in' transactions)
	err = r.DB.QueryRow(ctx, `SELECT COALESCE(SUM(total), 0) FROM txns WHERE type = 'in'`).Scan(&summary.TotalInvested)
	if err != nil {
		return nil, err
	}

	// Get total sold (sum of all 'out' transactions with sale price)
	err = r.DB.QueryRow(ctx, `SELECT COALESCE(SUM(total), 0) FROM txns WHERE type = 'out'`).Scan(&summary.TotalSold)
	if err != nil {
		return nil, err
	}

	summary.ProfitLoss = summary.TotalSold - summary.TotalInvested + summary.CurrentValue

	// Floor breakdown
	rows, err := r.DB.Query(ctx, `SELECT floor, COUNT(*), COALESCE(SUM(current_qty), 0), COALESCE(SUM(current_qty * unit_cost), 0)
		FROM items GROUP BY floor ORDER BY floor`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		fs := FloorSummary{}
		if err := rows.Scan(&fs.Floor, &fs.ItemCount, &fs.TotalQty, &fs.Value); err != nil {
			return nil, err
		}
		summary.FloorBreakdown = append(summary.FloorBreakdown, fs)
	}

	return summary, nil
}

// ============================================
// Full Main System Repository Methods
// ============================================

// Customer operations

func (r *Repository) CreateCustomer(ctx context.Context, c *Customer) error {
	return r.DB.QueryRow(ctx,
		`INSERT INTO customers(name, phone, so, village, address)
         VALUES($1, $2, $3, $4, $5)
         RETURNING id, created_at, updated_at`,
		c.Name, c.Phone, c.SO, c.Village, c.Address,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
}

func (r *Repository) GetCustomer(ctx context.Context, id int) (*Customer, error) {
	row := r.DB.QueryRow(ctx,
		`SELECT id, name, phone, COALESCE(so, '') as so, village, address, created_at, updated_at
         FROM customers WHERE id=$1`, id)

	var customer Customer
	err := row.Scan(&customer.ID, &customer.Name, &customer.Phone, &customer.SO, &customer.Village,
		&customer.Address, &customer.CreatedAt, &customer.UpdatedAt)
	return &customer, err
}

func (r *Repository) GetCustomerByPhone(ctx context.Context, phone string) (*Customer, error) {
	row := r.DB.QueryRow(ctx,
		`SELECT id, name, phone, COALESCE(so, '') as so, village, address, created_at, updated_at
         FROM customers WHERE phone=$1`, phone)

	var customer Customer
	err := row.Scan(&customer.ID, &customer.Name, &customer.Phone, &customer.SO, &customer.Village,
		&customer.Address, &customer.CreatedAt, &customer.UpdatedAt)
	return &customer, err
}

func (r *Repository) ListCustomers(ctx context.Context) ([]*Customer, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT id, name, phone, COALESCE(so, '') as so, village, address, created_at, updated_at
         FROM customers ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var customers []*Customer
	for rows.Next() {
		var customer Customer
		err := rows.Scan(&customer.ID, &customer.Name, &customer.Phone, &customer.SO, &customer.Village,
			&customer.Address, &customer.CreatedAt, &customer.UpdatedAt)
		if err != nil {
			return nil, err
		}
		customers = append(customers, &customer)
	}
	return customers, nil
}

func (r *Repository) UpdateCustomer(ctx context.Context, c *Customer) error {
	_, err := r.DB.Exec(ctx,
		`UPDATE customers SET name=$1, phone=$2, so=$3, village=$4, address=$5, updated_at=CURRENT_TIMESTAMP
         WHERE id=$6`,
		c.Name, c.Phone, c.SO, c.Village, c.Address, c.ID)
	return err
}

func (r *Repository) DeleteCustomer(ctx context.Context, id int) error {
	_, err := r.DB.Exec(ctx, `DELETE FROM customers WHERE id=$1`, id)
	return err
}

func (r *Repository) SearchCustomers(ctx context.Context, query string) ([]*Customer, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT id, name, phone, COALESCE(so, '') as so, village, address, created_at, updated_at
         FROM customers
         WHERE name ILIKE $1 OR phone ILIKE $1 OR village ILIKE $1
         ORDER BY created_at DESC LIMIT 50`, "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var customers []*Customer
	for rows.Next() {
		var customer Customer
		err := rows.Scan(&customer.ID, &customer.Name, &customer.Phone, &customer.SO, &customer.Village,
			&customer.Address, &customer.CreatedAt, &customer.UpdatedAt)
		if err != nil {
			return nil, err
		}
		customers = append(customers, &customer)
	}
	return customers, nil
}

// Entry operations

func (r *Repository) CreateEntry(ctx context.Context, e *Entry) error {
	// Use atomic INSERT with COUNT in a single query to prevent race conditions
	// This ensures counters auto-reset when entries are deleted (season reset)
	// The CTE (Common Table Expression) makes the count and insert atomic

	if e.ThockCategory != "seed" && e.ThockCategory != "sell" {
		return nil // Invalid category
	}

	// Determine the base offset for the category
	var baseOffset int
	if e.ThockCategory == "seed" {
		baseOffset = 1 // SEED starts at 1
	} else {
		baseOffset = 1501 // SELL starts at 1501
	}

	// Atomic query: COUNT and INSERT happen together in a single transaction
	// This prevents race conditions where two requests get the same count
	query := `
		WITH next_num AS (
			SELECT COALESCE(COUNT(*), 0) + $1 as num
			FROM entries
			WHERE thock_category = $2
		)
		INSERT INTO entries(customer_id, phone, name, village, so, expected_quantity, thock_category, thock_number, created_by_user_id)
		SELECT $3, $4, $5, $6, $7, $8, $9,
			CASE WHEN $9 = 'seed'
				THEN LPAD(num::text, 4, '0') || '/' || $8::text
				ELSE num::text || '/' || $8::text
			END,
			$10
		FROM next_num
		RETURNING id, thock_number, created_at, updated_at
	`

	return r.DB.QueryRow(ctx, query,
		baseOffset,           // $1
		e.ThockCategory,      // $2
		e.CustomerID,         // $3
		e.Phone,              // $4
		e.Name,               // $5
		e.Village,            // $6
		e.SO,                 // $7
		e.ExpectedQuantity,   // $8
		e.ThockCategory,      // $9
		e.CreatedByUserID,    // $10
	).Scan(&e.ID, &e.ThockNumber, &e.CreatedAt, &e.UpdatedAt)
}

func formatThockNumber(num, qty int, isSeed bool) string {
	if isSeed {
		return fmt.Sprintf("%04d/%d", num, qty)
	}
	return fmt.Sprintf("%d/%d", num, qty)
}

func (r *Repository) GetEntry(ctx context.Context, id int) (*Entry, error) {
	row := r.DB.QueryRow(ctx,
		`SELECT id, customer_id, phone, name, village, COALESCE(so, ''), expected_quantity, thock_category, thock_number, COALESCE(created_by_user_id, 0), created_at, updated_at
         FROM entries WHERE id=$1`, id)

	var entry Entry
	err := row.Scan(&entry.ID, &entry.CustomerID, &entry.Phone, &entry.Name, &entry.Village, &entry.SO,
		&entry.ExpectedQuantity, &entry.ThockCategory, &entry.ThockNumber, &entry.CreatedByUserID,
		&entry.CreatedAt, &entry.UpdatedAt)
	return &entry, err
}

func (r *Repository) GetEntryByThockNumber(ctx context.Context, thockNumber string) (*Entry, error) {
	row := r.DB.QueryRow(ctx,
		`SELECT id, customer_id, phone, name, village, COALESCE(so, ''), expected_quantity, thock_category, thock_number, COALESCE(created_by_user_id, 0), created_at, updated_at
         FROM entries WHERE thock_number=$1`, thockNumber)

	var entry Entry
	err := row.Scan(&entry.ID, &entry.CustomerID, &entry.Phone, &entry.Name, &entry.Village, &entry.SO,
		&entry.ExpectedQuantity, &entry.ThockCategory, &entry.ThockNumber, &entry.CreatedByUserID,
		&entry.CreatedAt, &entry.UpdatedAt)
	return &entry, err
}

func (r *Repository) ListEntries(ctx context.Context) ([]*Entry, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT id, customer_id, phone, name, village, COALESCE(so, ''), expected_quantity, thock_category, thock_number, COALESCE(created_by_user_id, 0), created_at, updated_at
         FROM entries ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*Entry
	for rows.Next() {
		var entry Entry
		err := rows.Scan(&entry.ID, &entry.CustomerID, &entry.Phone, &entry.Name, &entry.Village, &entry.SO,
			&entry.ExpectedQuantity, &entry.ThockCategory, &entry.ThockNumber, &entry.CreatedByUserID,
			&entry.CreatedAt, &entry.UpdatedAt)
		if err != nil {
			return nil, err
		}
		entries = append(entries, &entry)
	}
	return entries, nil
}

func (r *Repository) ListEntriesByCustomer(ctx context.Context, customerID int) ([]*Entry, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT id, customer_id, phone, name, village, COALESCE(so, ''), expected_quantity, thock_category, thock_number, COALESCE(created_by_user_id, 0), created_at, updated_at
         FROM entries WHERE customer_id=$1 ORDER BY created_at DESC`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*Entry
	for rows.Next() {
		var entry Entry
		err := rows.Scan(&entry.ID, &entry.CustomerID, &entry.Phone, &entry.Name, &entry.Village, &entry.SO,
			&entry.ExpectedQuantity, &entry.ThockCategory, &entry.ThockNumber, &entry.CreatedByUserID,
			&entry.CreatedAt, &entry.UpdatedAt)
		if err != nil {
			return nil, err
		}
		entries = append(entries, &entry)
	}
	return entries, nil
}

func (r *Repository) ListUnassignedEntries(ctx context.Context) ([]*Entry, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT e.id, e.customer_id, e.phone, e.name, e.village, COALESCE(e.so, ''), e.expected_quantity,
		        e.thock_category, e.thock_number, COALESCE(e.created_by_user_id, 0), e.created_at, e.updated_at
         FROM entries e
         LEFT JOIN room_entries re ON e.id = re.entry_id
         WHERE re.id IS NULL
         ORDER BY e.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*Entry
	for rows.Next() {
		var entry Entry
		err := rows.Scan(&entry.ID, &entry.CustomerID, &entry.Phone, &entry.Name, &entry.Village, &entry.SO,
			&entry.ExpectedQuantity, &entry.ThockCategory, &entry.ThockNumber, &entry.CreatedByUserID,
			&entry.CreatedAt, &entry.UpdatedAt)
		if err != nil {
			return nil, err
		}
		entries = append(entries, &entry)
	}
	return entries, nil
}

// Room Entry operations

func (r *Repository) CreateRoomEntry(ctx context.Context, re *RoomEntry) error {
	return r.DB.QueryRow(ctx,
		`INSERT INTO room_entries(entry_id, thock_number, room_no, floor, gate_no, remark, quantity, created_by_user_id)
         VALUES($1, $2, $3, $4, $5, $6, $7, $8)
         RETURNING id, created_at, updated_at`,
		re.EntryID, re.ThockNumber, re.RoomNo, re.Floor, re.GateNo, re.Remark, re.Quantity, re.CreatedByUserID,
	).Scan(&re.ID, &re.CreatedAt, &re.UpdatedAt)
}

func (r *Repository) GetRoomEntry(ctx context.Context, id int) (*RoomEntry, error) {
	row := r.DB.QueryRow(ctx,
		`SELECT id, entry_id, thock_number, room_no, floor, gate_no, COALESCE(remark, ''), quantity, COALESCE(created_by_user_id, 0), created_at, updated_at
         FROM room_entries WHERE id=$1`, id)

	var re RoomEntry
	err := row.Scan(&re.ID, &re.EntryID, &re.ThockNumber, &re.RoomNo, &re.Floor,
		&re.GateNo, &re.Remark, &re.Quantity, &re.CreatedByUserID, &re.CreatedAt, &re.UpdatedAt)
	return &re, err
}

func (r *Repository) ListRoomEntries(ctx context.Context) ([]*RoomEntry, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT id, entry_id, thock_number, room_no, floor, gate_no, COALESCE(remark, ''), quantity, COALESCE(created_by_user_id, 0), created_at, updated_at
         FROM room_entries ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roomEntries []*RoomEntry
	for rows.Next() {
		var re RoomEntry
		err := rows.Scan(&re.ID, &re.EntryID, &re.ThockNumber, &re.RoomNo, &re.Floor,
			&re.GateNo, &re.Remark, &re.Quantity, &re.CreatedByUserID, &re.CreatedAt, &re.UpdatedAt)
		if err != nil {
			return nil, err
		}
		roomEntries = append(roomEntries, &re)
	}
	return roomEntries, nil
}

func (r *Repository) ListRoomEntriesByThockNumber(ctx context.Context, thockNumber string) ([]*RoomEntry, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT id, entry_id, thock_number, room_no, floor, gate_no, COALESCE(remark, ''), quantity, COALESCE(created_by_user_id, 0), created_at, updated_at
         FROM room_entries WHERE thock_number=$1 ORDER BY created_at DESC`, thockNumber)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roomEntries []*RoomEntry
	for rows.Next() {
		var re RoomEntry
		err := rows.Scan(&re.ID, &re.EntryID, &re.ThockNumber, &re.RoomNo, &re.Floor,
			&re.GateNo, &re.Remark, &re.Quantity, &re.CreatedByUserID, &re.CreatedAt, &re.UpdatedAt)
		if err != nil {
			return nil, err
		}
		roomEntries = append(roomEntries, &re)
	}
	return roomEntries, nil
}

func (r *Repository) UpdateRoomEntry(ctx context.Context, id int, re *RoomEntry) error {
	return r.DB.QueryRow(ctx,
		`UPDATE room_entries
         SET room_no=$1, floor=$2, gate_no=$3, remark=$4, quantity=$5, updated_at=NOW()
         WHERE id=$6
         RETURNING updated_at`,
		re.RoomNo, re.Floor, re.GateNo, re.Remark, re.Quantity, id,
	).Scan(&re.UpdatedAt)
}

func (r *Repository) ReduceRoomEntryQuantity(ctx context.Context, thockNumber, roomNo, floor string, quantity int) error {
	query := `
		UPDATE room_entries
		SET quantity = quantity - $1, updated_at = NOW()
		WHERE thock_number = $2 AND room_no = $3 AND floor = $4 AND quantity >= $1
	`
	_, err := r.DB.Exec(ctx, query, quantity, thockNumber, roomNo, floor)
	return err
}

func (r *Repository) GetTotalQuantityByThockNumber(ctx context.Context, thockNumber string) (int, error) {
	var totalQuantity int
	query := `SELECT COALESCE(SUM(quantity), 0) FROM room_entries WHERE thock_number = $1`
	err := r.DB.QueryRow(ctx, query, thockNumber).Scan(&totalQuantity)
	return totalQuantity, err
}

// Gate Pass operations

func (r *Repository) CreateGatePass(ctx context.Context, gp *GatePass) error {
	query := `
		INSERT INTO gate_passes (
			customer_id, thock_number, entry_id, requested_quantity,
			payment_verified, payment_amount, issued_by_user_id, remarks,
			expires_at, request_source
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, CURRENT_TIMESTAMP + INTERVAL '30 hours', $9)
		RETURNING id, issued_at, expires_at, created_at, updated_at
	`

	return r.DB.QueryRow(ctx, query,
		gp.CustomerID, gp.ThockNumber, gp.EntryID,
		gp.RequestedQuantity, gp.PaymentVerified,
		gp.PaymentAmount, gp.IssuedByUserID, gp.Remarks, gp.RequestSource,
	).Scan(&gp.ID, &gp.IssuedAt, &gp.ExpiresAt, &gp.CreatedAt, &gp.UpdatedAt)
}

func (r *Repository) GetGatePass(ctx context.Context, id int) (*GatePass, error) {
	query := `
		SELECT id, customer_id, thock_number, entry_id, requested_quantity,
		       approved_quantity, final_approved_quantity, gate_no, status, payment_verified, payment_amount,
		       total_picked_up, issued_by_user_id, approved_by_user_id, created_by_customer_id,
		       COALESCE(request_source, 'employee'), issued_at, expires_at, approval_expires_at, completed_at,
		       remarks, created_at, updated_at
		FROM gate_passes
		WHERE id = $1
	`

	gp := &GatePass{}
	err := r.DB.QueryRow(ctx, query, id).Scan(
		&gp.ID, &gp.CustomerID, &gp.ThockNumber, &gp.EntryID,
		&gp.RequestedQuantity, &gp.ApprovedQuantity, &gp.FinalApprovedQuantity, &gp.GateNo,
		&gp.Status, &gp.PaymentVerified, &gp.PaymentAmount,
		&gp.TotalPickedUp, &gp.IssuedByUserID, &gp.ApprovedByUserID, &gp.CreatedByCustomerID,
		&gp.RequestSource, &gp.IssuedAt, &gp.ExpiresAt, &gp.ApprovalExpiresAt, &gp.CompletedAt,
		&gp.Remarks, &gp.CreatedAt, &gp.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return gp, nil
}

func (r *Repository) ListGatePasses(ctx context.Context) ([]map[string]interface{}, error) {
	query := `
		SELECT
			gp.id, gp.thock_number, gp.requested_quantity, gp.approved_quantity,
			gp.gate_no, gp.status, gp.payment_verified, gp.payment_amount,
			gp.issued_at, gp.expires_at, gp.completed_at, gp.remarks,
			gp.total_picked_up, gp.approval_expires_at, gp.final_approved_quantity,
			COALESCE(gp.request_source, 'employee') as request_source,
			c.id as customer_id, c.name as customer_name, c.phone as customer_phone,
			c.village as customer_village,
			e.id as entry_id, e.expected_quantity as entry_quantity
		FROM gate_passes gp
		JOIN customers c ON gp.customer_id = c.id
		LEFT JOIN entries e ON gp.entry_id = e.id
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
			requestedQty                                                                      int
			approvedQty, gateNo, remarks                                                      *string
			entryID, entryQty, finalApprovedQty                                               *int
			paymentVerified                                                                   bool
			paymentAmount                                                                     *float64
			issuedAt                                                                          interface{}
			expiresAt, approvalExpiresAt                                                      *interface{}
			completedAt                                                                       *interface{}
		)

		err := rows.Scan(
			&id, &thockNumber, &requestedQty, &approvedQty, &gateNo, &status,
			&paymentVerified, &paymentAmount, &issuedAt, &expiresAt, &completedAt, &remarks,
			&totalPickedUp, &approvalExpiresAt, &finalApprovedQty,
			&requestSource,
			&customerID, &customerName, &customerPhone, &customerVillage,
			&entryID, &entryQty,
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

		gatePasses = append(gatePasses, gatePass)
	}

	return gatePasses, rows.Err()
}

func (r *Repository) ListPendingGatePasses(ctx context.Context) ([]map[string]interface{}, error) {
	query := `
		SELECT
			gp.id, gp.thock_number, gp.requested_quantity, gp.gate_no,
			gp.payment_verified, gp.payment_amount, gp.issued_at, gp.expires_at, gp.remarks,
			(gp.expires_at IS NOT NULL AND CURRENT_TIMESTAMP > gp.expires_at) as is_expired,
			COALESCE(gp.request_source, 'employee') as request_source,
			c.id as customer_id, c.name as customer_name, c.phone as customer_phone,
			e.id as entry_id, e.expected_quantity as entry_quantity,
			re.room_no, re.floor, re.gate_no as gatar_no
		FROM gate_passes gp
		JOIN customers c ON gp.customer_id = c.id
		LEFT JOIN entries e ON gp.entry_id = e.id
		LEFT JOIN room_entries re ON gp.thock_number = re.thock_number
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
			id, customerID                                         int
			thockNumber, customerName, customerPhone, requestSource string
			requestedQty                                            int
			gateNo, remarks, roomNo, floor, gatarNo                 *string
			entryID, entryQty                                       *int
			paymentVerified, isExpired                              bool
			paymentAmount                                           *float64
			issuedAt, expiresAt                                     interface{}
		)

		err := rows.Scan(
			&id, &thockNumber, &requestedQty, &gateNo,
			&paymentVerified, &paymentAmount, &issuedAt, &expiresAt, &remarks, &isExpired,
			&requestSource,
			&customerID, &customerName, &customerPhone,
			&entryID, &entryQty,
			&roomNo, &floor, &gatarNo,
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
		if roomNo != nil {
			gatePass["room_no"] = *roomNo
		}
		if floor != nil {
			gatePass["floor"] = *floor
		}
		if gatarNo != nil {
			gatePass["gatar_no"] = *gatarNo
		}

		gatePasses = append(gatePasses, gatePass)
	}

	return gatePasses, rows.Err()
}

func (r *Repository) ListApprovedGatePasses(ctx context.Context) ([]map[string]interface{}, error) {
	query := `
		SELECT
			gp.id, gp.thock_number, gp.requested_quantity, gp.approved_quantity,
			gp.gate_no, gp.status, gp.payment_verified, gp.payment_amount,
			gp.issued_at, gp.approval_expires_at, gp.total_picked_up,
			COALESCE(gp.request_source, 'employee') as request_source,
			c.id as customer_id, c.name as customer_name, c.phone as customer_phone
		FROM gate_passes gp
		JOIN customers c ON gp.customer_id = c.id
		WHERE gp.status IN ('approved', 'partially_completed')
		ORDER BY gp.approval_expires_at ASC NULLS LAST
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
			id, customerID, requestedQty, totalPickedUp     int
			thockNumber, status, customerName, customerPhone, requestSource string
			approvedQty, gateNo                              *string
			paymentVerified                                  bool
			paymentAmount                                    *float64
			issuedAt                                          interface{}
			approvalExpiresAt                                 *interface{}
		)

		err := rows.Scan(
			&id, &thockNumber, &requestedQty, &approvedQty, &gateNo, &status,
			&paymentVerified, &paymentAmount, &issuedAt, &approvalExpiresAt, &totalPickedUp,
			&requestSource,
			&customerID, &customerName, &customerPhone,
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
		gatePass["customer_id"] = customerID
		gatePass["customer_name"] = customerName
		gatePass["customer_phone"] = customerPhone
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
		if approvalExpiresAt != nil {
			gatePass["approval_expires_at"] = *approvalExpiresAt
		}

		gatePasses = append(gatePasses, gatePass)
	}

	return gatePasses, rows.Err()
}

func (r *Repository) UpdateGatePass(ctx context.Context, id int, approvedQty int, gateNo, status, remarks string, approvedByUserID int) error {
	query := `
		UPDATE gate_passes
		SET approved_quantity = $1, gate_no = $2, status = $3::text, remarks = $4,
		    approved_by_user_id = $5,
		    approval_expires_at = CASE WHEN $3::text = 'approved' THEN CURRENT_TIMESTAMP + INTERVAL '15 hours' ELSE approval_expires_at END,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $6
	`

	_, err := r.DB.Exec(ctx, query, fmt.Sprintf("%d", approvedQty), gateNo, status, remarks, approvedByUserID, id)
	return err
}

func (r *Repository) UpdatePickupQuantity(ctx context.Context, gatePassID int, additionalQty int) error {
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

// Gate Pass Pickup operations

func (r *Repository) CreateGatePassPickup(ctx context.Context, pickup *GatePassPickup) error {
	query := `
		INSERT INTO gate_pass_pickups (gate_pass_id, quantity, room_no, floor, gatar_no, created_by_user_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`
	return r.DB.QueryRow(ctx, query,
		pickup.GatePassID, pickup.Quantity, pickup.RoomNo, pickup.Floor, pickup.GatarNo, pickup.CreatedByUserID,
	).Scan(&pickup.ID, &pickup.CreatedAt)
}

func (r *Repository) ListPickupsByGatePass(ctx context.Context, gatePassID int) ([]*GatePassPickup, error) {
	query := `
		SELECT id, gate_pass_id, quantity, room_no, floor, gatar_no, COALESCE(created_by_user_id, 0), created_at
		FROM gate_pass_pickups
		WHERE gate_pass_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.DB.Query(ctx, query, gatePassID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pickups []*GatePassPickup
	for rows.Next() {
		pickup := &GatePassPickup{}
		err := rows.Scan(&pickup.ID, &pickup.GatePassID, &pickup.Quantity, &pickup.RoomNo, &pickup.Floor, &pickup.GatarNo, &pickup.CreatedByUserID, &pickup.CreatedAt)
		if err != nil {
			return nil, err
		}
		pickups = append(pickups, pickup)
	}
	return pickups, nil
}

// Entry Event operations

func (r *Repository) CreateEntryEvent(ctx context.Context, event *EntryEvent) error {
	query := `
		INSERT INTO entry_events (entry_id, event_type, status, notes, created_by_user_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`
	return r.DB.QueryRow(ctx, query,
		event.EntryID, event.EventType, event.Status, event.Notes, event.CreatedByUserID,
	).Scan(&event.ID, &event.CreatedAt)
}

func (r *Repository) ListEntryEvents(ctx context.Context, limit int) ([]*EntryEvent, error) {
	query := `
		SELECT id, entry_id, event_type, status, COALESCE(notes, ''), COALESCE(created_by_user_id, 0), created_at
		FROM entry_events
		ORDER BY created_at DESC
		LIMIT $1
	`
	rows, err := r.DB.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*EntryEvent
	for rows.Next() {
		event := &EntryEvent{}
		err := rows.Scan(&event.ID, &event.EntryID, &event.EventType, &event.Status, &event.Notes, &event.CreatedByUserID, &event.CreatedAt)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func (r *Repository) ListEntryEventsByEntry(ctx context.Context, entryID int) ([]*EntryEvent, error) {
	query := `
		SELECT id, entry_id, event_type, status, COALESCE(notes, ''), COALESCE(created_by_user_id, 0), created_at
		FROM entry_events
		WHERE entry_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.DB.Query(ctx, query, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*EntryEvent
	for rows.Next() {
		event := &EntryEvent{}
		err := rows.Scan(&event.ID, &event.EntryID, &event.EventType, &event.Status, &event.Notes, &event.CreatedByUserID, &event.CreatedAt)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

// Rent Payment operations

func (r *Repository) GenerateReceiptNumber(ctx context.Context) (string, error) {
	var nextNum int
	err := r.DB.QueryRow(ctx, "SELECT nextval('g_receipt_number_sequence')").Scan(&nextNum)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("G-RCP-%06d", nextNum), nil
}

func (r *Repository) CreateRentPayment(ctx context.Context, payment *RentPayment) error {
	receiptNumber, err := r.GenerateReceiptNumber(ctx)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO rent_payments (receipt_number, entry_id, customer_name, customer_phone, total_rent, amount_paid, balance, processed_by_user_id, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, payment_date, created_at
	`

	err = r.DB.QueryRow(ctx, query,
		receiptNumber,
		payment.EntryID,
		payment.CustomerName,
		payment.CustomerPhone,
		payment.TotalRent,
		payment.AmountPaid,
		payment.Balance,
		payment.ProcessedByUserID,
		payment.Notes,
	).Scan(&payment.ID, &payment.PaymentDate, &payment.CreatedAt)

	if err != nil {
		return err
	}

	payment.ReceiptNumber = receiptNumber
	return nil
}

func (r *Repository) ListRentPayments(ctx context.Context) ([]*RentPayment, error) {
	query := `
		SELECT id, receipt_number, entry_id, customer_name, customer_phone, total_rent, amount_paid, balance,
		       payment_date, COALESCE(processed_by_user_id, 0), COALESCE(notes, ''), created_at
		FROM rent_payments
		ORDER BY payment_date DESC
	`

	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []*RentPayment
	for rows.Next() {
		payment := &RentPayment{}
		err := rows.Scan(
			&payment.ID,
			&payment.ReceiptNumber,
			&payment.EntryID,
			&payment.CustomerName,
			&payment.CustomerPhone,
			&payment.TotalRent,
			&payment.AmountPaid,
			&payment.Balance,
			&payment.PaymentDate,
			&payment.ProcessedByUserID,
			&payment.Notes,
			&payment.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		payments = append(payments, payment)
	}

	return payments, nil
}

func (r *Repository) GetRentPaymentByReceipt(ctx context.Context, receiptNumber string) (*RentPayment, error) {
	query := `
		SELECT id, receipt_number, entry_id, customer_name, customer_phone, total_rent, amount_paid, balance,
		       payment_date, COALESCE(processed_by_user_id, 0), COALESCE(notes, ''), created_at
		FROM rent_payments
		WHERE receipt_number = $1
	`

	payment := &RentPayment{}
	err := r.DB.QueryRow(ctx, query, receiptNumber).Scan(
		&payment.ID,
		&payment.ReceiptNumber,
		&payment.EntryID,
		&payment.CustomerName,
		&payment.CustomerPhone,
		&payment.TotalRent,
		&payment.AmountPaid,
		&payment.Balance,
		&payment.PaymentDate,
		&payment.ProcessedByUserID,
		&payment.Notes,
		&payment.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return payment, nil
}

// System Setting operations

func (r *Repository) GetSystemSetting(ctx context.Context, key string) (*SystemSetting, error) {
	setting := &SystemSetting{}
	query := `SELECT id, key, value, updated_at FROM system_settings WHERE key = $1`
	err := r.DB.QueryRow(ctx, query, key).Scan(&setting.ID, &setting.Key, &setting.Value, &setting.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return setting, nil
}

func (r *Repository) ListSystemSettings(ctx context.Context) ([]*SystemSetting, error) {
	query := `SELECT id, key, value, updated_at FROM system_settings ORDER BY key`
	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var settings []*SystemSetting
	for rows.Next() {
		setting := &SystemSetting{}
		err := rows.Scan(&setting.ID, &setting.Key, &setting.Value, &setting.UpdatedAt)
		if err != nil {
			return nil, err
		}
		settings = append(settings, setting)
	}
	return settings, nil
}

func (r *Repository) UpdateSystemSetting(ctx context.Context, key, value string) error {
	query := `UPDATE system_settings SET value = $2, updated_at = NOW() WHERE key = $1`
	_, err := r.DB.Exec(ctx, query, key, value)
	return err
}

// Dashboard Summary

func (r *Repository) GetDashboardSummary(ctx context.Context) (*DashboardSummary, error) {
	summary := &DashboardSummary{}

	// Total customers
	r.DB.QueryRow(ctx, `SELECT COUNT(*) FROM customers`).Scan(&summary.TotalCustomers)

	// Total entries
	r.DB.QueryRow(ctx, `SELECT COUNT(*) FROM entries`).Scan(&summary.TotalEntries)

	// Total quantity in room entries
	r.DB.QueryRow(ctx, `SELECT COALESCE(SUM(quantity), 0) FROM room_entries`).Scan(&summary.TotalQuantity)

	// Pending gate passes
	r.DB.QueryRow(ctx, `SELECT COUNT(*) FROM gate_passes WHERE status = 'pending'`).Scan(&summary.PendingGatePasses)

	// Today's entries
	r.DB.QueryRow(ctx, `SELECT COUNT(*) FROM entries WHERE DATE(created_at) = CURRENT_DATE`).Scan(&summary.TodayEntries)

	// Today's gate passes
	r.DB.QueryRow(ctx, `SELECT COUNT(*) FROM gate_passes WHERE DATE(created_at) = CURRENT_DATE`).Scan(&summary.TodayGatePasses)

	// Total rent collected
	r.DB.QueryRow(ctx, `SELECT COALESCE(SUM(amount_paid), 0) FROM rent_payments`).Scan(&summary.TotalRentCollected)

	// Room breakdown
	rows, err := r.DB.Query(ctx, `
		SELECT room_no, SUM(quantity), COUNT(*)
		FROM room_entries
		GROUP BY room_no
		ORDER BY room_no
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			rs := RoomSummary{}
			rows.Scan(&rs.RoomNo, &rs.TotalQty, &rs.EntryCount)
			summary.RoomBreakdown = append(summary.RoomBreakdown, rs)
		}
	}

	return summary, nil
}
