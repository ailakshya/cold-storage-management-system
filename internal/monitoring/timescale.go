package monitoring

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TimescaleStore struct {
	pool      *pgxpool.Pool
	enabled   bool
	logBuffer []APILog
	mu        sync.RWMutex
}

func NewTimescaleStore(pool *pgxpool.Pool) *TimescaleStore {
	store := &TimescaleStore{
		pool:      pool,
		logBuffer: make([]APILog, 0, 1000),
	}

	if pool == nil {
		store.enabled = false
		log.Println("[Monitoring] TimescaleDB pool is nil. Running in in-memory mode for logs.")
		return store
	}

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
	if ts.pool == nil {
		return fmt.Errorf("pool is nil")
	}
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
		log.Printf("[Monitoring] Could not create extension: %v", err)
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

	ts.pool.Exec(ctx, "SELECT create_hypertable('metrics_api', 'time', if_not_exists => TRUE)")

	return nil
}

func (ts *TimescaleStore) RecordSystemMetrics(cpu float64, memUsed, memTotal, diskUsed, diskTotal uint64) error {
	if !ts.enabled || ts.pool == nil {
		return nil // Skip system metrics explicitly in fallback mode (or store in memory if needed)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := ts.pool.Exec(ctx, `
		INSERT INTO metrics_system (time, cpu_percent, mem_used, mem_total, disk_used, disk_total)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, time.Now(), cpu, memUsed, memTotal, diskUsed, diskTotal)

	return err
}

func (ts *TimescaleStore) RecordAPIMetric(method, path string, status int, duration time.Duration, ip string) {
	// Always record to memory buffer
	ts.mu.Lock()
	ts.logBuffer = append(ts.logBuffer, APILog{
		Time:       time.Now(),
		Method:     method,
		Path:       path,
		StatusCode: status,
		Duration:   float64(duration.Milliseconds()),
		IPAddress:  ip,
	})
	// Keep only last 1000 logs
	if len(ts.logBuffer) > 1000 {
		ts.logBuffer = ts.logBuffer[len(ts.logBuffer)-1000:]
	}
	ts.mu.Unlock()

	// If DB enabled, record there too
	if ts.enabled && ts.pool != nil {
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
	if !ts.enabled || ts.pool == nil {
		// Use in-memory buffer
		ts.mu.RLock()
		defer ts.mu.RUnlock()

		var total int64
		var totalDur float64
		var errors int64
		threshold := time.Now().Add(-duration)

		for _, l := range ts.logBuffer {
			if l.Time.After(threshold) {
				total++
				totalDur += l.Duration
				if l.StatusCode >= 500 {
					errors++
				}
			}
		}

		summary := APISummary{TotalRequests: total}
		if total > 0 {
			summary.AvgDuration = totalDur / float64(total)
			summary.ErrorRate = float64(errors) / float64(total)
		}
		return summary, nil
	}

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
	if !ts.enabled || ts.pool == nil {
		return []TimePoint{}, nil // No memory fallback for trends yet
	}
	return ts.getResourceTrend("cpu_percent", duration)
}

func (ts *TimescaleStore) GetMemoryTrend(duration time.Duration) ([]TimePoint, error) {
	if !ts.enabled || ts.pool == nil {
		return []TimePoint{}, nil
	}
	ctx := context.Background()
	rows, err := ts.pool.Query(ctx, `
		SELECT time_bucket('1 minute', time) as bucket, AVG(mem_used::float / NULLIF(mem_total, 0) * 100)
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

func (ts *TimescaleStore) GetDiskTrend(duration time.Duration) ([]TimePoint, error) {
	if !ts.enabled || ts.pool == nil {
		return []TimePoint{}, nil
	}
	ctx := context.Background()
	rows, err := ts.pool.Query(ctx, `
		SELECT time_bucket('1 minute', time) as bucket, AVG(disk_used::float / NULLIF(disk_total, 0) * 100)
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

func (ts *TimescaleStore) getResourceTrend(column string, duration time.Duration) ([]TimePoint, error) {
	ctx := context.Background()
	rows, err := ts.pool.Query(ctx, fmt.Sprintf(`
		SELECT time_bucket('1 minute', time) as bucket, AVG(%s)
		FROM metrics_system
		WHERE time > NOW() - $1::interval
		GROUP BY bucket
		ORDER BY bucket
	`, column), duration.String())
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

type APILog struct {
	Time       time.Time `json:"time"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	StatusCode int       `json:"status_code"`
	Duration   float64   `json:"duration_ms"`
	IPAddress  string    `json:"ip_address"`
}

func (ts *TimescaleStore) GetAPILogs(duration time.Duration, errorsOnly bool, limit, offset int) ([]APILog, error) {
	if !ts.enabled || ts.pool == nil {
		// Serve from memory with filtering and pagination
		ts.mu.RLock()
		defer ts.mu.RUnlock()

		var filtered []APILog
		threshold := time.Now().Add(-duration)

		// Buffer is oldest to newest, we want newest first for logs
		for i := len(ts.logBuffer) - 1; i >= 0; i-- {
			l := ts.logBuffer[i]
			if l.Time.Before(threshold) {
				continue
			}
			if errorsOnly && l.StatusCode < 400 {
				continue
			}
			filtered = append(filtered, l)
		}

		// Pagination
		if offset >= len(filtered) {
			return []APILog{}, nil
		}
		end := offset + limit
		if end > len(filtered) {
			end = len(filtered)
		}

		return filtered[offset:end], nil
	}

	ctx := context.Background()

	query := `
		SELECT time, method, path, status_code, duration_ms, ip_address
		FROM metrics_api
		WHERE time > NOW() - $1::interval
	`
	args := []interface{}{duration.String()}
	argCounter := 2

	if errorsOnly {
		query += " AND status_code >= 400"
	}

	query += fmt.Sprintf(" ORDER BY time DESC LIMIT $%d OFFSET $%d", argCounter, argCounter+1)
	args = append(args, limit, offset)

	rows, err := ts.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []APILog
	for rows.Next() {
		var l APILog
		if err := rows.Scan(&l.Time, &l.Method, &l.Path, &l.StatusCode, &l.Duration, &l.IPAddress); err != nil {
			continue
		}
		logs = append(logs, l)
	}
	return logs, nil
}
