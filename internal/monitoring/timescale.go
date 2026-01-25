package monitoring

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TimescaleStore struct {
	pool    *pgxpool.Pool
	enabled bool
}

func NewTimescaleStore(pool *pgxpool.Pool) *TimescaleStore {
	store := &TimescaleStore{pool: pool}
	if err := store.Init(); err != nil {
		log.Printf("[Monitoring] Warning: TimescaleDB initialization failed: %v. Running in standard Postgres mode.", err)
		store.enabled = false
	} else {
		store.enabled = true
		log.Println("[Monitoring] TimescaleDB metrics storage initialized")
	}
	return store
}

func (ts *TimescaleStore) Init() error {
	ctx := context.Background()

	// Check if TimescaleDB extension exists
	var version string
	err := ts.pool.QueryRow(ctx, "SELECT default_version FROM pg_available_extensions WHERE name = 'timescaledb'").Scan(&version)
	if err != nil {
		return fmt.Errorf("timescaledb extension not found: %w", err)
	}

	// Create extension if not exists
	_, err = ts.pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE")
	if err != nil {
		// Might fail if not superuser, but check if key tables exist regardless
		log.Printf("[Monitoring] Could not create extension (might already exist or permission denied): %v", err)
	}

	// Create System Metrics Table
	_, err = ts.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS metrics_system (
			time        TIMESTAMPTZ NOT NULL,
			cpu_percent DOUBLE PRECISION,
			mem_used    BIGINT,
			mem_total   BIGINT,
			disk_used   BIGINT,
			disk_total  BIGINT,
			load_avg    DOUBLE PRECISION
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create metrics_system table: %w", err)
	}

	// Convert to hypertable (ignore error if already hypertable)
	ts.pool.Exec(ctx, "SELECT create_hypertable('metrics_system', 'time', if_not_exists => TRUE)")

	// Create API Metrics Table
	_, err = ts.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS metrics_api (
			time        TIMESTAMPTZ NOT NULL,
			method      TEXT,
			path        TEXT,
			status_code INTEGER,
			duration_ms DOUBLE PRECISION,
			ip_address  TEXT
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create metrics_api table: %w", err)
	}

	// Convert to hypertable
	ts.pool.Exec(ctx, "SELECT create_hypertable('metrics_api', 'time', if_not_exists => TRUE)")

	return nil
}

func (ts *TimescaleStore) RecordSystemMetrics(cpu float64, memUsed, memTotal, diskUsed, diskTotal uint64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := ts.pool.Exec(ctx, `
		INSERT INTO metrics_system (time, cpu_percent, mem_used, mem_total, disk_used, disk_total)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, time.Now(), cpu, memUsed, memTotal, diskUsed, diskTotal)

	return err
}

func (ts *TimescaleStore) RecordAPIMetric(method, path string, status int, duration time.Duration, ip string) {
	// Run in background to not block request
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_, err := ts.pool.Exec(ctx, `
			INSERT INTO metrics_api (time, method, path, status_code, duration_ms, ip_address)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, time.Now(), method, path, status, float64(duration.Milliseconds()), ip)

		if err != nil {
			log.Printf("[Monitoring] Failed to record API metric: %v", err)
		}
	}()
}

// Analytics Queries

type APISummary struct {
	TotalRequests int64   `json:"total_requests"`
	AvgDuration   float64 `json:"avg_duration"`
	ErrorRate     float64 `json:"error_rate"`
}

type TimePoint struct {
	Time  time.Time `json:"time"`
	Value float64   `json:"value"`
}

func (ts *TimescaleStore) GetAPISummary(duration time.Duration) (APISummary, error) {
	ctx := context.Background()
	var summary APISummary

	err := ts.pool.QueryRow(ctx, `
		SELECT 
			COUNT(*) as total,
			COALESCE(AVG(duration_ms), 0) as avg_lat,
			COALESCE(SUM(CASE WHEN status_code >= 500 THEN 1 ELSE 0 END)::float / NULLIF(COUNT(*), 0), 0) as err_rate
		FROM metrics_api
		WHERE time > NOW() - $1::interval
	`, duration.String()).Scan(&summary.TotalRequests, &summary.AvgDuration, &summary.ErrorRate)

	return summary, err
}

func (ts *TimescaleStore) GetCPUTrend(duration time.Duration) ([]TimePoint, error) {
	ctx := context.Background()
	rows, err := ts.pool.Query(ctx, `
		SELECT time_bucket('1 minute', time) as bucket, AVG(cpu_percent)
		FROM metrics_system
		WHERE time > NOW() - $1::interval
		GROUP BY bucket
		ORDER BY bucket
	`, duration.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []TimePoint
	for rows.Next() {
		var p TimePoint
		if err := rows.Scan(&p.Time, &p.Value); err != nil {
			continue
		}
		points = append(points, p)
	}
	return points, nil
}

// InsertAPILog inserts a detailed API log entry (compatible with models.APIRequestLog structure)
// We accept any interface that matches the structure to avoid circular imports if possible,
// or just specific fields. But for integration with existing middleware, let's keep it flexible.
// Since we can't easily import "cold-backend/internal/models" here without risk of cycles if models imports monitoring,
// we'll define a simpler method signature or use the one we have.
// However, to satisfy the middleware, we should probably stick to the simple RecordAPIMetric we already have
// and update the middleware to use it.
