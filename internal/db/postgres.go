package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"cold-backend/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(cfg *config.Config) *pgxpool.Pool {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Name,
	)

	// PERFORMANCE FIX: Configure connection pool for optimal performance
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatalf("db config parse failed: %v", err)
	}

	// Configure pool settings for production workload
	poolConfig.MaxConns = 25                          // Maximum connections in pool
	poolConfig.MinConns = 5                           // Keep warm connections ready
	poolConfig.MaxConnLifetime = time.Hour            // Recycle connections hourly
	poolConfig.MaxConnIdleTime = 30 * time.Minute     // Close idle connections after 30min
	poolConfig.HealthCheckPeriod = time.Minute        // Check connection health every minute
	poolConfig.ConnConfig.ConnectTimeout = 5 * time.Second  // Fast timeout for new connections

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		log.Fatalf("db connect failed: %v", err)
	}

	log.Printf("✓ Database connection pool: %d max, %d min warm",
		poolConfig.MaxConns, poolConfig.MinConns)

	// Run migrations
	if err := RunMigrations(pool); err != nil {
		log.Printf("Warning: Migration failed: %v", err)
	}

	return pool
}

// RunMigrations creates necessary tables if they don't exist
func RunMigrations(pool *pgxpool.Pool) error {
	ctx := context.Background()

	// Create login_logs table
	loginLogsTable := `
		CREATE TABLE IF NOT EXISTS login_logs (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			login_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			logout_time TIMESTAMP,
			ip_address VARCHAR(45),
			user_agent TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_login_logs_user_id ON login_logs(user_id);
		CREATE INDEX IF NOT EXISTS idx_login_logs_login_time ON login_logs(login_time);
	`

	if _, err := pool.Exec(ctx, loginLogsTable); err != nil {
		return fmt.Errorf("failed to create login_logs table: %w", err)
	}

	// Create room_entry_edit_logs table
	editLogsTable := `
		CREATE TABLE IF NOT EXISTS room_entry_edit_logs (
			id SERIAL PRIMARY KEY,
			room_entry_id INTEGER NOT NULL REFERENCES room_entries(id) ON DELETE CASCADE,
			edited_by_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			old_room_no VARCHAR(10),
			new_room_no VARCHAR(10),
			old_floor VARCHAR(10),
			new_floor VARCHAR(10),
			old_gate_no VARCHAR(50),
			new_gate_no VARCHAR(50),
			old_quantity INTEGER,
			new_quantity INTEGER,
			old_remark TEXT,
			new_remark TEXT,
			edited_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_room_entry_edit_logs_room_entry_id ON room_entry_edit_logs(room_entry_id);
		CREATE INDEX IF NOT EXISTS idx_room_entry_edit_logs_edited_by ON room_entry_edit_logs(edited_by_user_id);
	`

	if _, err := pool.Exec(ctx, editLogsTable); err != nil {
		return fmt.Errorf("failed to create room_entry_edit_logs table: %w", err)
	}

	// Create admin_action_logs table
	adminActionLogsTable := `
		CREATE TABLE IF NOT EXISTS admin_action_logs (
			id SERIAL PRIMARY KEY,
			admin_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			action_type VARCHAR(50) NOT NULL,
			target_type VARCHAR(50) NOT NULL,
			target_id INTEGER,
			description TEXT NOT NULL,
			old_value TEXT,
			new_value TEXT,
			ip_address VARCHAR(45),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_admin_action_logs_admin_user_id ON admin_action_logs(admin_user_id);
		CREATE INDEX IF NOT EXISTS idx_admin_action_logs_created_at ON admin_action_logs(created_at);
		CREATE INDEX IF NOT EXISTS idx_admin_action_logs_action_type ON admin_action_logs(action_type);
	`

	if _, err := pool.Exec(ctx, adminActionLogsTable); err != nil {
		return fmt.Errorf("failed to create admin_action_logs table: %w", err)
	}

	// Create gate_passes table
	gatePassesTable := `
		CREATE TABLE IF NOT EXISTS gate_passes (
			id SERIAL PRIMARY KEY,
			customer_id INTEGER NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
			thock_number VARCHAR(20) NOT NULL,
			entry_id INTEGER REFERENCES entries(id) ON DELETE SET NULL,
			requested_quantity INTEGER NOT NULL,
			approved_quantity INTEGER,
			gate_no VARCHAR(50),
			status VARCHAR(20) DEFAULT 'pending',
			payment_verified BOOLEAN DEFAULT false,
			payment_amount DECIMAL(10,2),
			issued_by_user_id INTEGER REFERENCES users(id),
			approved_by_user_id INTEGER REFERENCES users(id),
			issued_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP,
			completed_at TIMESTAMP,
			remarks TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_gate_passes_customer_id ON gate_passes(customer_id);
		CREATE INDEX IF NOT EXISTS idx_gate_passes_entry_id ON gate_passes(entry_id);
		CREATE INDEX IF NOT EXISTS idx_gate_passes_status ON gate_passes(status);
		CREATE INDEX IF NOT EXISTS idx_gate_passes_issued_at ON gate_passes(issued_at);
	`

	if _, err := pool.Exec(ctx, gatePassesTable); err != nil {
		return fmt.Errorf("failed to create gate_passes table: %w", err)
	}

	// Add expires_at column if it doesn't exist (for existing databases)
	alterGatePassesTable := `
		ALTER TABLE gate_passes ADD COLUMN IF NOT EXISTS expires_at TIMESTAMP;
		CREATE INDEX IF NOT EXISTS idx_gate_passes_expires_at ON gate_passes(expires_at);
	`

	if _, err := pool.Exec(ctx, alterGatePassesTable); err != nil {
		return fmt.Errorf("failed to alter gate_passes table: %w", err)
	}

	// Backfill expires_at for existing gate passes
	backfillExpiresAt := `
		UPDATE gate_passes
		SET expires_at = issued_at + INTERVAL '30 hours'
		WHERE expires_at IS NULL;
	`

	if _, err := pool.Exec(ctx, backfillExpiresAt); err != nil {
		return fmt.Errorf("failed to backfill expires_at: %w", err)
	}

	// Add new columns to gate_passes for partial completion tracking
	alterGatePassesForPickup := `
		ALTER TABLE gate_passes
		ADD COLUMN IF NOT EXISTS total_picked_up INTEGER DEFAULT 0,
		ADD COLUMN IF NOT EXISTS approval_expires_at TIMESTAMP,
		ADD COLUMN IF NOT EXISTS final_approved_quantity INTEGER;
	`

	if _, err := pool.Exec(ctx, alterGatePassesForPickup); err != nil {
		return fmt.Errorf("failed to alter gate_passes for pickup tracking: %w", err)
	}

	// Create gate_pass_pickups table
	gatePassPickupsTable := `
		CREATE TABLE IF NOT EXISTS gate_pass_pickups (
			id SERIAL PRIMARY KEY,
			gate_pass_id INTEGER NOT NULL REFERENCES gate_passes(id) ON DELETE CASCADE,
			pickup_quantity INTEGER NOT NULL,
			picked_up_by_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			pickup_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			room_no VARCHAR(10),
			floor VARCHAR(10),
			remarks TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_gate_pass_pickups_gate_pass_id ON gate_pass_pickups(gate_pass_id);
		CREATE INDEX IF NOT EXISTS idx_gate_pass_pickups_pickup_time ON gate_pass_pickups(pickup_time);
	`

	if _, err := pool.Exec(ctx, gatePassPickupsTable); err != nil {
		return fmt.Errorf("failed to create gate_pass_pickups table: %w", err)
	}

	log.Println("Migrations completed successfully")
	return nil
}

// ConnectG connects to the gallery database
func ConnectG(cfg *config.Config) *pgxpool.Pool {
	if !cfg.G.Enabled {
		return nil
	}

	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s",
		cfg.G.DB.User,
		cfg.G.DB.Password,
		cfg.G.DB.Host,
		cfg.G.DB.Port,
		cfg.G.DB.Name,
	)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Printf("G db config parse failed: %v", err)
		return nil
	}

	poolConfig.MaxConns = 5
	poolConfig.MinConns = 1
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 15 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		log.Printf("G db connect failed: %v", err)
		return nil
	}

	return pool
}

// TryConnectWithFallback attempts to connect to databases in order of fallback priority.
// Tries all common passwords for each host to handle password sync issues.
// Returns the pool if successful, or nil if all connections fail.
func TryConnectWithFallback() (*pgxpool.Pool, string) {
	// Try each fallback database
	for _, dbConfig := range config.DatabaseFallbacks {
		log.Printf("[DB] Trying to connect to %s (%s:%d)...", dbConfig.Name, dbConfig.Host, dbConfig.Port)

		// Try each password for this host
		for i, password := range config.CommonPasswords {
			dsn := dbConfig.ConnectionStringWithPassword(password)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			poolConfig, err := pgxpool.ParseConfig(dsn)
			if err != nil {
				log.Printf("[DB]   Password %d: parse error: %v", i+1, err)
				cancel()
				continue
			}

			// Configure pool settings
			poolConfig.MaxConns = 25
			poolConfig.MinConns = 5
			poolConfig.MaxConnLifetime = time.Hour
			poolConfig.MaxConnIdleTime = 30 * time.Minute
			poolConfig.HealthCheckPeriod = time.Minute
			poolConfig.ConnConfig.ConnectTimeout = 5 * time.Second

			pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
			if err != nil {
				log.Printf("[DB]   Password %d: connect error: %v", i+1, err)
				cancel()
				continue
			}

			// Test the connection
			if err := pool.Ping(ctx); err != nil {
				log.Printf("[DB]   Password %d: ping error: %v", i+1, err)
				cancel()
				pool.Close()
				continue
			}

			cancel()
			log.Printf("[DB] Successfully connected to %s", dbConfig.Name)
			return pool, dbConfig.Name
		}
		log.Printf("[DB] Failed to connect to %s (tried all passwords)", dbConfig.Name)
	}

	log.Println("[DB] All database connections failed - entering setup mode")
	return nil, ""
}
