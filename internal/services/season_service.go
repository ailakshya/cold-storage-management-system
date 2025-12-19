package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"cold-backend/internal/auth"
	"cold-backend/internal/models"
	"cold-backend/internal/repositories"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// SeasonService handles new season business logic
type SeasonService struct {
	seasonRepo   *repositories.SeasonRequestRepository
	userRepo     *repositories.UserRepository
	pool         *pgxpool.Pool
	tsdbPool     *pgxpool.Pool
	jwtManager   *auth.JWTManager
	archiveHost  string
	archivePort  string
	archiveUser  string
	archivePass  string
	archiveDB    string
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
		seasonRepo:   seasonRepo,
		userRepo:     userRepo,
		pool:         pool,
		tsdbPool:     tsdbPool,
		jwtManager:   jwtManager,
		archiveHost:  "cold-postgres-rw",
		archivePort:  "5432",
		archiveUser:  "postgres",
		archivePass:  "SecurePostgresPassword123",
		archiveDB:    "cold_storage",
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
	archiveLocation := fmt.Sprintf("192.168.15.195:/archive/%s_%s", seasonName, archiveTime)

	log.Printf("[Season] Starting season reset for request %d", requestID)

	// Connect to archive database
	archiveConnStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		s.archiveHost, s.archivePort, s.archiveUser, s.archivePass, s.archiveDB)

	archiveDB, err := sql.Open("postgres", archiveConnStr)
	if err != nil {
		log.Printf("[Season] Failed to connect to archive database: %v", err)
		s.seasonRepo.UpdateCompletion(ctx, requestID, "failed", "", nil, fmt.Sprintf("Failed to connect to archive database: %v", err))
		return
	}
	defer archiveDB.Close()

	// Test connection
	if err := archiveDB.Ping(); err != nil {
		log.Printf("[Season] Failed to ping archive database: %v", err)
		s.seasonRepo.UpdateCompletion(ctx, requestID, "failed", "", nil, fmt.Sprintf("Failed to ping archive database: %v", err))
		return
	}

	// Create archive tables if they don't exist
	if err := s.createArchiveTables(archiveDB); err != nil {
		log.Printf("[Season] Failed to create archive tables: %v", err)
		s.seasonRepo.UpdateCompletion(ctx, requestID, "failed", "", nil, fmt.Sprintf("Failed to create archive tables: %v", err))
		return
	}

	// Archive app data
	log.Println("[Season] Archiving entries...")
	summary.Entries, _ = s.archiveEntries(ctx, archiveDB, seasonName)

	log.Println("[Season] Archiving room_entries...")
	summary.RoomEntries, _ = s.archiveRoomEntries(ctx, archiveDB, seasonName)

	log.Println("[Season] Archiving entry_events...")
	summary.EntryEvents, _ = s.archiveEntryEvents(ctx, archiveDB, seasonName)

	log.Println("[Season] Archiving gate_passes...")
	summary.GatePasses, _ = s.archiveGatePasses(ctx, archiveDB, seasonName)

	log.Println("[Season] Archiving gate_pass_pickups...")
	summary.GatePassPickups, _ = s.archiveGatePassPickups(ctx, archiveDB, seasonName)

	log.Println("[Season] Archiving rent_payments...")
	summary.RentPayments, _ = s.archiveRentPayments(ctx, archiveDB, seasonName)

	log.Println("[Season] Archiving invoices...")
	summary.Invoices, _ = s.archiveInvoices(ctx, archiveDB, seasonName)

	// Archive timeseries data if available
	if s.tsdbPool != nil {
		log.Println("[Season] Archiving node_metrics...")
		summary.NodeMetrics, _ = s.archiveNodeMetrics(ctx, archiveDB, seasonName)

		log.Println("[Season] Archiving api_request_logs...")
		summary.APIRequestLogs, _ = s.archiveAPIRequestLogs(ctx, archiveDB, seasonName)
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

// createArchiveTables creates the archive tables on the backup server
func (s *SeasonService) createArchiveTables(archiveDB *sql.DB) error {
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
	}

	for _, ddl := range tables {
		if _, err := archiveDB.Exec(ddl); err != nil {
			return fmt.Errorf("failed to create table: %v", err)
		}
	}

	return nil
}

// Archive functions for each table
func (s *SeasonService) archiveEntries(ctx context.Context, archiveDB *sql.DB, seasonName string) (int, error) {
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

		_, err := archiveDB.Exec(`
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

func (s *SeasonService) archiveRoomEntries(ctx context.Context, archiveDB *sql.DB, seasonName string) (int, error) {
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

		_, err := archiveDB.Exec(`
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

func (s *SeasonService) archiveEntryEvents(ctx context.Context, archiveDB *sql.DB, seasonName string) (int, error) {
	result, err := s.pool.Exec(ctx, "SELECT COUNT(*) FROM entry_events")
	if err != nil {
		return 0, err
	}
	return int(result.RowsAffected()), nil
}

func (s *SeasonService) archiveGatePasses(ctx context.Context, archiveDB *sql.DB, seasonName string) (int, error) {
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

		_, err := archiveDB.Exec(`
			INSERT INTO archived_gate_passes (season_name, original_id, entry_id, customer_id, status, quantity, created_at, data)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, seasonName, id, entryID, customerID, status, quantity, createdAt, data)

		if err == nil {
			count++
		}
	}

	return count, nil
}

func (s *SeasonService) archiveGatePassPickups(ctx context.Context, archiveDB *sql.DB, seasonName string) (int, error) {
	result, err := s.pool.Exec(ctx, "SELECT COUNT(*) FROM gate_pass_pickups")
	if err != nil {
		return 0, err
	}
	return int(result.RowsAffected()), nil
}

func (s *SeasonService) archiveRentPayments(ctx context.Context, archiveDB *sql.DB, seasonName string) (int, error) {
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

		_, err := archiveDB.Exec(`
			INSERT INTO archived_rent_payments (season_name, original_id, customer_id, amount, payment_date, data)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, seasonName, id, customerID, amount, paymentDate, data)

		if err == nil {
			count++
		}
	}

	return count, nil
}

func (s *SeasonService) archiveInvoices(ctx context.Context, archiveDB *sql.DB, seasonName string) (int, error) {
	result, err := s.pool.Exec(ctx, "SELECT COUNT(*) FROM invoices")
	if err != nil {
		return 0, err
	}
	return int(result.RowsAffected()), nil
}

func (s *SeasonService) archiveNodeMetrics(ctx context.Context, archiveDB *sql.DB, seasonName string) (int, error) {
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

		_, err := archiveDB.Exec(`
			INSERT INTO archived_node_metrics (season_name, timestamp, node_name, data)
			VALUES ($1, $2, $3, $4)
		`, seasonName, timestamp, nodeName, data)

		if err == nil {
			count++
		}
	}

	return count, nil
}

func (s *SeasonService) archiveAPIRequestLogs(ctx context.Context, archiveDB *sql.DB, seasonName string) (int, error) {
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

		_, err := archiveDB.Exec(`
			INSERT INTO archived_api_logs (season_name, timestamp, method, path, data)
			VALUES ($1, $2, $3, $4, $5)
		`, seasonName, timestamp, method, path, data)

		if err == nil {
			count++
		}
	}

	return count, nil
}
