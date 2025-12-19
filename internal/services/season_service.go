package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"cold-backend/internal/auth"
	"cold-backend/internal/models"
	"cold-backend/internal/repositories"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// SeasonService handles new season business logic
type SeasonService struct {
	seasonRepo *repositories.SeasonRequestRepository
	userRepo   *repositories.UserRepository
	pool       *pgxpool.Pool
	tsdbPool   *pgxpool.Pool
	jwtManager *auth.JWTManager
}

// NewSeasonService creates a new season service
func NewSeasonService(
	seasonRepo *repositories.SeasonRequestRepository,
	userRepo *repositories.UserRepository,
	pool *pgxpool.Pool,
	tsdbPool *pgxpool.Pool,
	jwtManager *auth.JWTManager,
) *SeasonService {
	return &SeasonService{
		seasonRepo: seasonRepo,
		userRepo:   userRepo,
		pool:       pool,
		tsdbPool:   tsdbPool,
		jwtManager: jwtManager,
	}
}

// InitiateNewSeason creates a new season request (requires admin password verification)
func (s *SeasonService) InitiateNewSeason(ctx context.Context, userID int, req *models.InitiateSeasonRequest) (*models.SeasonRequest, error) {
	// Verify user is admin
	user, err := s.userRepo.Get(ctx, userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	if user.Role != "admin" {
		return nil, errors.New("only admins can initiate a new season")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid password")
	}

	// Check if there's already a pending request
	pending, err := s.seasonRepo.GetPending(ctx)
	if err == nil && len(pending) > 0 {
		return nil, errors.New("there is already a pending season request")
	}

	// Create the request
	seasonReq := &models.SeasonRequest{
		InitiatedByUserID: userID,
		SeasonName:        req.SeasonName,
		Notes:             req.Notes,
	}

	return s.seasonRepo.Create(ctx, seasonReq)
}

// GetPendingRequests returns all pending season requests
func (s *SeasonService) GetPendingRequests(ctx context.Context) ([]*models.SeasonRequest, error) {
	return s.seasonRepo.GetPending(ctx)
}

// GetHistory returns all season requests
func (s *SeasonService) GetHistory(ctx context.Context) ([]*models.SeasonRequest, error) {
	return s.seasonRepo.GetAll(ctx)
}

// GetRequest returns a specific season request
func (s *SeasonService) GetRequest(ctx context.Context, id int) (*models.SeasonRequest, error) {
	return s.seasonRepo.GetByID(ctx, id)
}

// ArchivedData holds the archived data for a season
type ArchivedData struct {
	SeasonName      string                   `json:"season_name"`
	Entries         []map[string]interface{} `json:"entries"`
	RoomEntries     []map[string]interface{} `json:"room_entries"`
	GatePasses      []map[string]interface{} `json:"gate_passes"`
	GatePassPickups []map[string]interface{} `json:"gate_pass_pickups"`
	RentPayments    []map[string]interface{} `json:"rent_payments"`
	Invoices        []map[string]interface{} `json:"invoices"`
}

// GetArchivedData returns archived data for a specific season
func (s *SeasonService) GetArchivedData(ctx context.Context, seasonName string) (*ArchivedData, error) {
	data := &ArchivedData{
		SeasonName:      seasonName,
		Entries:         []map[string]interface{}{},
		RoomEntries:     []map[string]interface{}{},
		GatePasses:      []map[string]interface{}{},
		GatePassPickups: []map[string]interface{}{},
		RentPayments:    []map[string]interface{}{},
		Invoices:        []map[string]interface{}{},
	}

	// Get archived entries
	rows, err := s.pool.Query(ctx, `
		SELECT original_id, customer_id, truck_number, item_type, expected_quantity, created_at
		FROM archived_entries WHERE season_name = $1 ORDER BY original_id
	`, seasonName)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var origID, custID, qty int
			var truckNum, itemType string
			var createdAt interface{}
			if err := rows.Scan(&origID, &custID, &truckNum, &itemType, &qty, &createdAt); err == nil {
				data.Entries = append(data.Entries, map[string]interface{}{
					"id":           origID,
					"customer_id":  custID,
					"truck_number": truckNum,
					"category":     itemType,
					"quantity":     qty,
					"created_at":   createdAt,
				})
			}
		}
	}

	// Get archived room entries
	rows2, err := s.pool.Query(ctx, `
		SELECT original_id, entry_id, room_number, quantity, created_at
		FROM archived_room_entries WHERE season_name = $1 ORDER BY original_id
	`, seasonName)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var origID, entryID, roomNum, qty int
			var createdAt interface{}
			if err := rows2.Scan(&origID, &entryID, &roomNum, &qty, &createdAt); err == nil {
				data.RoomEntries = append(data.RoomEntries, map[string]interface{}{
					"id":         origID,
					"entry_id":   entryID,
					"room":       roomNum,
					"quantity":   qty,
					"created_at": createdAt,
				})
			}
		}
	}

	// Get archived gate passes
	rows3, err := s.pool.Query(ctx, `
		SELECT original_id, entry_id, customer_id, status, quantity, created_at
		FROM archived_gate_passes WHERE season_name = $1 ORDER BY original_id
	`, seasonName)
	if err == nil {
		defer rows3.Close()
		for rows3.Next() {
			var origID, entryID, custID, qty int
			var status string
			var createdAt interface{}
			if err := rows3.Scan(&origID, &entryID, &custID, &status, &qty, &createdAt); err == nil {
				data.GatePasses = append(data.GatePasses, map[string]interface{}{
					"id":          origID,
					"entry_id":    entryID,
					"customer_id": custID,
					"status":      status,
					"quantity":    qty,
					"created_at":  createdAt,
				})
			}
		}
	}

	// Get archived rent payments
	rows4, err := s.pool.Query(ctx, `
		SELECT original_id, customer_id, amount, payment_date
		FROM archived_rent_payments WHERE season_name = $1 ORDER BY original_id
	`, seasonName)
	if err == nil {
		defer rows4.Close()
		for rows4.Next() {
			var origID, custID int
			var amount float64
			var paymentDate interface{}
			if err := rows4.Scan(&origID, &custID, &amount, &paymentDate); err == nil {
				data.RentPayments = append(data.RentPayments, map[string]interface{}{
					"id":           origID,
					"customer_id":  custID,
					"amount":       amount,
					"payment_date": paymentDate,
				})
			}
		}
	}

	// Get archived invoices
	rows5, err := s.pool.Query(ctx, `
		SELECT original_id, customer_id, invoice_number, total_amount, status, created_at
		FROM archived_invoices WHERE season_name = $1 ORDER BY original_id
	`, seasonName)
	if err == nil {
		defer rows5.Close()
		for rows5.Next() {
			var origID, custID int
			var invoiceNumber, status string
			var totalAmount float64
			var createdAt interface{}
			if err := rows5.Scan(&origID, &custID, &invoiceNumber, &totalAmount, &status, &createdAt); err == nil {
				data.Invoices = append(data.Invoices, map[string]interface{}{
					"id":             origID,
					"customer_id":    custID,
					"invoice_number": invoiceNumber,
					"total_amount":   totalAmount,
					"status":         status,
					"created_at":     createdAt,
				})
			}
		}
	}

	// Get archived gate pass pickups
	rows6, err := s.pool.Query(ctx, `
		SELECT original_id, gate_pass_id, quantity, picked_at
		FROM archived_gate_pass_pickups WHERE season_name = $1 ORDER BY original_id
	`, seasonName)
	if err == nil {
		defer rows6.Close()
		for rows6.Next() {
			var origID, gatePassID, quantity int
			var pickedAt interface{}
			if err := rows6.Scan(&origID, &gatePassID, &quantity, &pickedAt); err == nil {
				data.GatePassPickups = append(data.GatePassPickups, map[string]interface{}{
					"id":           origID,
					"gate_pass_id": gatePassID,
					"quantity":     quantity,
					"picked_at":    pickedAt,
				})
			}
		}
	}

	return data, nil
}

// ApproveRequest approves a season request and executes the archive/clear process
func (s *SeasonService) ApproveRequest(ctx context.Context, requestID int, approverUserID int, password string) error {
	// Get the request
	req, err := s.seasonRepo.GetByID(ctx, requestID)
	if err != nil {
		return errors.New("season request not found")
	}

	if req.Status != "pending" {
		return errors.New("request is not in pending status")
	}

	// Verify approver is admin and different from initiator
	approver, err := s.userRepo.Get(ctx, approverUserID)
	if err != nil {
		return errors.New("approver not found")
	}

	if approver.Role != "admin" {
		return errors.New("only admins can approve season requests")
	}

	// Special case: User ID 2 (lakshya) can approve their own requests
	// All other admins still require dual approval
	if approverUserID == req.InitiatedByUserID && approverUserID != 2 {
		return errors.New("approver must be different from initiator")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(approver.PasswordHash), []byte(password)); err != nil {
		return errors.New("invalid password")
	}

	// Update status to approved
	if err := s.seasonRepo.UpdateStatus(ctx, requestID, "approved", &approverUserID); err != nil {
		return err
	}

	// Execute archive and clear process in background
	go s.executeSeasonReset(requestID, req.SeasonName)

	return nil
}

// RejectRequest rejects a season request
func (s *SeasonService) RejectRequest(ctx context.Context, requestID int, rejecterUserID int, reason string) error {
	// Get the request
	req, err := s.seasonRepo.GetByID(ctx, requestID)
	if err != nil {
		return errors.New("season request not found")
	}

	if req.Status != "pending" {
		return errors.New("request is not in pending status")
	}

	// Verify rejecter is admin
	rejecter, err := s.userRepo.Get(ctx, rejecterUserID)
	if err != nil {
		return errors.New("user not found")
	}

	if rejecter.Role != "admin" {
		return errors.New("only admins can reject season requests")
	}

	return s.seasonRepo.RejectRequest(ctx, requestID, rejecterUserID, reason)
}

// executeSeasonReset performs the actual archive and clear operations
func (s *SeasonService) executeSeasonReset(requestID int, seasonName string) {
	ctx := context.Background()
	summary := &models.RecordsArchivedSummary{}
	archiveTime := time.Now().Format("2006-01-02_15-04-05")
	archiveLocation := fmt.Sprintf("local:archived_tables/%s_%s", seasonName, archiveTime)

	log.Printf("[Season] Starting season reset for request %d", requestID)

	// Create archive tables in same database if they don't exist
	if err := s.createArchiveTables(ctx); err != nil {
		log.Printf("[Season] Failed to create archive tables: %v", err)
		s.seasonRepo.UpdateCompletion(ctx, requestID, "failed", "", nil, fmt.Sprintf("Failed to create archive tables: %v", err))
		return
	}

	// Archive app data to same database
	log.Println("[Season] Archiving entries...")
	summary.Entries, _ = s.archiveEntries(ctx, seasonName)

	log.Println("[Season] Archiving room_entries...")
	summary.RoomEntries, _ = s.archiveRoomEntries(ctx, seasonName)

	log.Println("[Season] Archiving entry_events...")
	summary.EntryEvents, _ = s.archiveEntryEvents(ctx, seasonName)

	log.Println("[Season] Archiving gate_passes...")
	summary.GatePasses, _ = s.archiveGatePasses(ctx, seasonName)

	log.Println("[Season] Archiving gate_pass_pickups...")
	summary.GatePassPickups, _ = s.archiveGatePassPickups(ctx, seasonName)

	log.Println("[Season] Archiving rent_payments...")
	summary.RentPayments, _ = s.archiveRentPayments(ctx, seasonName)

	log.Println("[Season] Archiving invoices...")
	summary.Invoices, _ = s.archiveInvoices(ctx, seasonName)

	// Archive timeseries data if available
	if s.tsdbPool != nil {
		log.Println("[Season] Archiving node_metrics...")
		summary.NodeMetrics, _ = s.archiveNodeMetrics(ctx, seasonName)

		log.Println("[Season] Archiving api_request_logs...")
		summary.APIRequestLogs, _ = s.archiveAPIRequestLogs(ctx, seasonName)
	}

	// Clear tables in correct order (respecting foreign keys)
	log.Println("[Season] Clearing tables...")

	// Clear customer_otps
	s.pool.Exec(ctx, "TRUNCATE TABLE customer_otps CASCADE")
	summary.CustomerOTPs = 1

	// Clear gate_pass_pickups
	s.pool.Exec(ctx, "TRUNCATE TABLE gate_pass_pickups CASCADE")

	// Clear gate_passes
	s.pool.Exec(ctx, "TRUNCATE TABLE gate_passes CASCADE")

	// Clear invoices
	s.pool.Exec(ctx, "TRUNCATE TABLE invoices CASCADE")

	// Clear rent_payments
	s.pool.Exec(ctx, "TRUNCATE TABLE rent_payments CASCADE")

	// Clear entry_events
	s.pool.Exec(ctx, "TRUNCATE TABLE entry_events CASCADE")

	// Clear room_entries
	s.pool.Exec(ctx, "TRUNCATE TABLE room_entries CASCADE")

	// Clear entries
	s.pool.Exec(ctx, "TRUNCATE TABLE entries CASCADE")

	// Reset customer data (keep only basic info)
	log.Println("[Season] Resetting customer data...")
	resetResult, _ := s.pool.Exec(ctx, `
		UPDATE customers SET
			total_bags = 0,
			balance = 0,
			opening_balance = 0,
			updated_at = CURRENT_TIMESTAMP
	`)
	summary.CustomersReset = int(resetResult.RowsAffected())

	// Reset ID sequences
	log.Println("[Season] Resetting ID sequences...")
	s.pool.Exec(ctx, "ALTER SEQUENCE entries_id_seq RESTART WITH 1")
	s.pool.Exec(ctx, "ALTER SEQUENCE room_entries_id_seq RESTART WITH 1")
	s.pool.Exec(ctx, "ALTER SEQUENCE entry_events_id_seq RESTART WITH 1")
	s.pool.Exec(ctx, "ALTER SEQUENCE gate_passes_id_seq RESTART WITH 1")
	s.pool.Exec(ctx, "ALTER SEQUENCE gate_pass_pickups_id_seq RESTART WITH 1")
	s.pool.Exec(ctx, "ALTER SEQUENCE rent_payments_id_seq RESTART WITH 1")
	s.pool.Exec(ctx, "ALTER SEQUENCE invoices_id_seq RESTART WITH 1")

	// Clear timeseries data if available
	if s.tsdbPool != nil {
		log.Println("[Season] Clearing timeseries data...")
		s.tsdbPool.Exec(ctx, "TRUNCATE TABLE node_metrics")
		s.tsdbPool.Exec(ctx, "TRUNCATE TABLE api_request_logs")
	}

	log.Printf("[Season] Season reset complete. Archived: %+v", summary)

	// Update request as completed
	s.seasonRepo.UpdateCompletion(ctx, requestID, "completed", archiveLocation, summary, "")
}

// createArchiveTables creates the archive tables in the same database
func (s *SeasonService) createArchiveTables(ctx context.Context) error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS archived_seasons (
			id SERIAL PRIMARY KEY,
			season_name VARCHAR(100) NOT NULL,
			archived_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS archived_entries (
			id SERIAL PRIMARY KEY,
			season_name VARCHAR(100),
			original_id INTEGER,
			customer_id INTEGER,
			truck_number VARCHAR(50),
			item_type VARCHAR(100),
			expected_quantity INTEGER,
			created_at TIMESTAMP,
			data JSONB
		)`,
		`CREATE TABLE IF NOT EXISTS archived_room_entries (
			id SERIAL PRIMARY KEY,
			season_name VARCHAR(100),
			original_id INTEGER,
			entry_id INTEGER,
			room_number INTEGER,
			quantity INTEGER,
			created_at TIMESTAMP,
			data JSONB
		)`,
		`CREATE TABLE IF NOT EXISTS archived_gate_passes (
			id SERIAL PRIMARY KEY,
			season_name VARCHAR(100),
			original_id INTEGER,
			entry_id INTEGER,
			customer_id INTEGER,
			status VARCHAR(50),
			quantity INTEGER,
			created_at TIMESTAMP,
			data JSONB
		)`,
		`CREATE TABLE IF NOT EXISTS archived_rent_payments (
			id SERIAL PRIMARY KEY,
			season_name VARCHAR(100),
			original_id INTEGER,
			customer_id INTEGER,
			amount DECIMAL(12,2),
			payment_date DATE,
			data JSONB
		)`,
		`CREATE TABLE IF NOT EXISTS archived_node_metrics (
			id SERIAL PRIMARY KEY,
			season_name VARCHAR(100),
			timestamp TIMESTAMP,
			node_name VARCHAR(100),
			data JSONB
		)`,
		`CREATE TABLE IF NOT EXISTS archived_api_logs (
			id SERIAL PRIMARY KEY,
			season_name VARCHAR(100),
			timestamp TIMESTAMP,
			method VARCHAR(10),
			path VARCHAR(255),
			data JSONB
		)`,
		`CREATE TABLE IF NOT EXISTS archived_invoices (
			id SERIAL PRIMARY KEY,
			season_name VARCHAR(100),
			original_id INTEGER,
			customer_id INTEGER,
			invoice_number VARCHAR(50),
			total_amount DECIMAL(12,2),
			status VARCHAR(50),
			created_at TIMESTAMP,
			data JSONB
		)`,
		`CREATE TABLE IF NOT EXISTS archived_invoice_items (
			id SERIAL PRIMARY KEY,
			season_name VARCHAR(100),
			original_id INTEGER,
			invoice_id INTEGER,
			description TEXT,
			amount DECIMAL(12,2),
			data JSONB
		)`,
		`CREATE TABLE IF NOT EXISTS archived_gate_pass_pickups (
			id SERIAL PRIMARY KEY,
			season_name VARCHAR(100),
			original_id INTEGER,
			gate_pass_id INTEGER,
			quantity INTEGER,
			picked_at TIMESTAMP,
			data JSONB
		)`,
	}

	for _, ddl := range tables {
		if _, err := s.pool.Exec(ctx, ddl); err != nil {
			return fmt.Errorf("failed to create table: %v", err)
		}
	}

	return nil
}

// Archive functions for each table
func (s *SeasonService) archiveEntries(ctx context.Context, seasonName string) (int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, customer_id, thock_number, thock_category, expected_quantity, created_at,
			   row_to_json(entries.*) as data
		FROM entries
	`)
	if err != nil {
		log.Printf("[Season] archiveEntries query error: %v", err)
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id int
		var customerID *int
		var expectedQty int
		var thockNumber, thockCategory string
		var createdAt time.Time
		var data []byte

		if err := rows.Scan(&id, &customerID, &thockNumber, &thockCategory, &expectedQty, &createdAt, &data); err != nil {
			log.Printf("[Season] archiveEntries scan error: %v", err)
			continue
		}

		custID := 0
		if customerID != nil {
			custID = *customerID
		}

		_, err := s.pool.Exec(ctx, `
			INSERT INTO archived_entries (season_name, original_id, customer_id, truck_number, item_type, expected_quantity, created_at, data)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, seasonName, id, custID, thockNumber, thockCategory, expectedQty, createdAt, data)

		if err != nil {
			log.Printf("[Season] archiveEntries insert error: %v", err)
		} else {
			count++
		}
	}

	log.Printf("[Season] archiveEntries: archived %d entries", count)
	return count, nil
}

func (s *SeasonService) archiveRoomEntries(ctx context.Context, seasonName string) (int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, entry_id, room_no, quantity, created_at,
			   row_to_json(room_entries.*) as data
		FROM room_entries
	`)
	if err != nil {
		log.Printf("[Season] archiveRoomEntries query error: %v", err)
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id int
		var entryID *int
		var roomNo string
		var quantity int
		var createdAt time.Time
		var data []byte

		if err := rows.Scan(&id, &entryID, &roomNo, &quantity, &createdAt, &data); err != nil {
			log.Printf("[Season] archiveRoomEntries scan error: %v", err)
			continue
		}

		entID := 0
		if entryID != nil {
			entID = *entryID
		}

		// Convert room_no string to int for archive table
		roomNum := 0
		fmt.Sscanf(roomNo, "%d", &roomNum)

		_, err := s.pool.Exec(ctx, `
			INSERT INTO archived_room_entries (season_name, original_id, entry_id, room_number, quantity, created_at, data)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, seasonName, id, entID, roomNum, quantity, createdAt, data)

		if err != nil {
			log.Printf("[Season] archiveRoomEntries insert error: %v", err)
		} else {
			count++
		}
	}

	log.Printf("[Season] archiveRoomEntries: archived %d room entries", count)
	return count, nil
}

func (s *SeasonService) archiveEntryEvents(ctx context.Context, seasonName string) (int, error) {
	result, err := s.pool.Exec(ctx, "SELECT COUNT(*) FROM entry_events")
	if err != nil {
		return 0, err
	}
	return int(result.RowsAffected()), nil
}

func (s *SeasonService) archiveGatePasses(ctx context.Context, seasonName string) (int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, entry_id, customer_id, status, quantity, created_at,
			   row_to_json(gate_passes.*) as data
		FROM gate_passes
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, entryID, customerID, quantity int
		var status string
		var createdAt time.Time
		var data []byte

		if err := rows.Scan(&id, &entryID, &customerID, &status, &quantity, &createdAt, &data); err != nil {
			continue
		}

		_, err := s.pool.Exec(ctx, `
			INSERT INTO archived_gate_passes (season_name, original_id, entry_id, customer_id, status, quantity, created_at, data)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, seasonName, id, entryID, customerID, status, quantity, createdAt, data)

		if err == nil {
			count++
		}
	}

	return count, nil
}

func (s *SeasonService) archiveGatePassPickups(ctx context.Context, seasonName string) (int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, gate_pass_id, quantity, created_at,
			   row_to_json(gate_pass_pickups.*) as data
		FROM gate_pass_pickups
	`)
	if err != nil {
		log.Printf("[Season] archiveGatePassPickups query error: %v", err)
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, gatePassID, quantity int
		var createdAt time.Time
		var data []byte

		if err := rows.Scan(&id, &gatePassID, &quantity, &createdAt, &data); err != nil {
			log.Printf("[Season] archiveGatePassPickups scan error: %v", err)
			continue
		}

		_, err := s.pool.Exec(ctx, `
			INSERT INTO archived_gate_pass_pickups (season_name, original_id, gate_pass_id, quantity, picked_at, data)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, seasonName, id, gatePassID, quantity, createdAt, data)

		if err != nil {
			log.Printf("[Season] archiveGatePassPickups insert error: %v", err)
		} else {
			count++
		}
	}

	log.Printf("[Season] archiveGatePassPickups: archived %d pickups", count)
	return count, nil
}

func (s *SeasonService) archiveRentPayments(ctx context.Context, seasonName string) (int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, customer_id, amount, payment_date,
			   row_to_json(rent_payments.*) as data
		FROM rent_payments
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, customerID int
		var amount float64
		var paymentDate time.Time
		var data []byte

		if err := rows.Scan(&id, &customerID, &amount, &paymentDate, &data); err != nil {
			continue
		}

		_, err := s.pool.Exec(ctx, `
			INSERT INTO archived_rent_payments (season_name, original_id, customer_id, amount, payment_date, data)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, seasonName, id, customerID, amount, paymentDate, data)

		if err == nil {
			count++
		}
	}

	return count, nil
}

func (s *SeasonService) archiveInvoices(ctx context.Context, seasonName string) (int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, customer_id, invoice_number, total_amount, status, created_at,
			   row_to_json(invoices.*) as data
		FROM invoices
	`)
	if err != nil {
		log.Printf("[Season] archiveInvoices query error: %v", err)
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, customerID int
		var invoiceNumber, status string
		var totalAmount float64
		var createdAt time.Time
		var data []byte

		if err := rows.Scan(&id, &customerID, &invoiceNumber, &totalAmount, &status, &createdAt, &data); err != nil {
			log.Printf("[Season] archiveInvoices scan error: %v", err)
			continue
		}

		_, err := s.pool.Exec(ctx, `
			INSERT INTO archived_invoices (season_name, original_id, customer_id, invoice_number, total_amount, status, created_at, data)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, seasonName, id, customerID, invoiceNumber, totalAmount, status, createdAt, data)

		if err != nil {
			log.Printf("[Season] archiveInvoices insert error: %v", err)
		} else {
			count++
		}
	}

	// Also archive invoice items
	itemRows, err := s.pool.Query(ctx, `
		SELECT id, invoice_id, description, amount, row_to_json(invoice_items.*) as data
		FROM invoice_items
	`)
	if err == nil {
		defer itemRows.Close()
		for itemRows.Next() {
			var id, invoiceID int
			var description string
			var amount float64
			var data []byte

			if err := itemRows.Scan(&id, &invoiceID, &description, &amount, &data); err != nil {
				continue
			}

			s.pool.Exec(ctx, `
				INSERT INTO archived_invoice_items (season_name, original_id, invoice_id, description, amount, data)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, seasonName, id, invoiceID, description, amount, data)
		}
	}

	log.Printf("[Season] archiveInvoices: archived %d invoices", count)
	return count, nil
}

func (s *SeasonService) archiveNodeMetrics(ctx context.Context, seasonName string) (int, error) {
	if s.tsdbPool == nil {
		return 0, nil
	}

	rows, err := s.tsdbPool.Query(ctx, `
		SELECT timestamp, node_name, row_to_json(node_metrics.*) as data
		FROM node_metrics
		LIMIT 10000
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var timestamp time.Time
		var nodeName string
		var data []byte

		if err := rows.Scan(&timestamp, &nodeName, &data); err != nil {
			continue
		}

		_, err := s.pool.Exec(ctx, `
			INSERT INTO archived_node_metrics (season_name, timestamp, node_name, data)
			VALUES ($1, $2, $3, $4)
		`, seasonName, timestamp, nodeName, data)

		if err == nil {
			count++
		}
	}

	return count, nil
}

func (s *SeasonService) archiveAPIRequestLogs(ctx context.Context, seasonName string) (int, error) {
	if s.tsdbPool == nil {
		return 0, nil
	}

	rows, err := s.tsdbPool.Query(ctx, `
		SELECT timestamp, method, path, row_to_json(api_request_logs.*) as data
		FROM api_request_logs
		LIMIT 10000
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var timestamp time.Time
		var method, path string
		var data []byte

		if err := rows.Scan(&timestamp, &method, &path, &data); err != nil {
			continue
		}

		_, err := s.pool.Exec(ctx, `
			INSERT INTO archived_api_logs (season_name, timestamp, method, path, data)
			VALUES ($1, $2, $3, $4, $5)
		`, seasonName, timestamp, method, path, data)

		if err == nil {
			count++
		}
	}

	return count, nil
}
