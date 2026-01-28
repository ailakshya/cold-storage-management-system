package monitoring

import (
	"context"
	"fmt"
	"log"
	"sort"
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
	TotalRequests   int64   `json:"total_requests"`
	SuccessRequests int64   `json:"success_requests"`
	AvgDurationMs   float64 `json:"avg_duration_ms"`
	P95DurationMs   float64 `json:"p95_duration_ms"`
	ErrorRate       float64 `json:"error_rate"`
}

type EndpointStat struct {
	Path          string  `json:"path"`
	TotalRequests int64   `json:"total_requests"`
	AvgDurationMs float64 `json:"avg_duration_ms"`
	P95DurationMs float64 `json:"p95_duration_ms"`
	MaxDurationMs float64 `json:"max_duration_ms"`
	ErrorCount    int64   `json:"error_count"`
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
		var durations []float64
		threshold := time.Now().Add(-duration)

		for _, l := range ts.logBuffer {
			if l.Time.After(threshold) {
				total++
				totalDur += l.Duration
				durations = append(durations, l.Duration)
				if l.StatusCode >= 500 {
					errors++
				}
			}
		}

		summary := APISummary{TotalRequests: total, SuccessRequests: total - errors}
		if total > 0 {
			summary.AvgDurationMs = totalDur / float64(total)
			summary.ErrorRate = float64(errors) / float64(total)
			// P95
			sort.Float64s(durations)
			p95Idx := int(float64(len(durations)) * 0.95)
			if p95Idx >= len(durations) {
				p95Idx = len(durations) - 1
			}
			summary.P95DurationMs = durations[p95Idx]
		}
		return summary, nil
	}

	ctx := context.Background()
	var summary APISummary

	err := ts.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE status_code < 500) as success,
			COALESCE(AVG(duration_ms), 0) as avg_dur,
			COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms), 0) as p95_dur,
			COALESCE(SUM(CASE WHEN status_code >= 500 THEN 1 ELSE 0 END)::float / NULLIF(COUNT(*), 0), 0) as err_rate
		FROM metrics_api
		WHERE time > NOW() - $1::interval
	`, duration.String()).Scan(&summary.TotalRequests, &summary.SuccessRequests, &summary.AvgDurationMs, &summary.P95DurationMs, &summary.ErrorRate)

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

func (ts *TimescaleStore) GetTopEndpoints(duration time.Duration, limit int) ([]EndpointStat, error) {
	if !ts.enabled || ts.pool == nil {
		return ts.getEndpointStatsFromBuffer(duration, limit, func(a, b EndpointStat) bool {
			return a.TotalRequests > b.TotalRequests
		}), nil
	}

	ctx := context.Background()
	rows, err := ts.pool.Query(ctx, `
		SELECT path, COUNT(*) as total,
			AVG(duration_ms) as avg_dur,
			COUNT(*) FILTER (WHERE status_code >= 400) as errors
		FROM metrics_api
		WHERE time > NOW() - $1::interval
		GROUP BY path
		ORDER BY total DESC
		LIMIT $2
	`, duration.String(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []EndpointStat
	for rows.Next() {
		var s EndpointStat
		if err := rows.Scan(&s.Path, &s.TotalRequests, &s.AvgDurationMs, &s.ErrorCount); err != nil {
			continue
		}
		stats = append(stats, s)
	}
	return stats, nil
}

func (ts *TimescaleStore) GetSlowestEndpoints(duration time.Duration, limit int) ([]EndpointStat, error) {
	if !ts.enabled || ts.pool == nil {
		return ts.getEndpointStatsFromBuffer(duration, limit, func(a, b EndpointStat) bool {
			return a.AvgDurationMs > b.AvgDurationMs
		}), nil
	}

	ctx := context.Background()
	rows, err := ts.pool.Query(ctx, `
		SELECT path, COUNT(*) as total,
			AVG(duration_ms) as avg_dur,
			COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms), 0) as p95_dur,
			MAX(duration_ms) as max_dur
		FROM metrics_api
		WHERE time > NOW() - $1::interval
		GROUP BY path
		ORDER BY avg_dur DESC
		LIMIT $2
	`, duration.String(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []EndpointStat
	for rows.Next() {
		var s EndpointStat
		if err := rows.Scan(&s.Path, &s.TotalRequests, &s.AvgDurationMs, &s.P95DurationMs, &s.MaxDurationMs); err != nil {
			continue
		}
		stats = append(stats, s)
	}
	return stats, nil
}

// getEndpointStatsFromBuffer aggregates endpoint stats from the in-memory log buffer.
func (ts *TimescaleStore) getEndpointStatsFromBuffer(duration time.Duration, limit int, less func(a, b EndpointStat) bool) []EndpointStat {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	threshold := time.Now().Add(-duration)
	type pathAgg struct {
		total     int64
		errors    int64
		totalDur  float64
		maxDur    float64
		durations []float64
	}
	agg := make(map[string]*pathAgg)

	for _, l := range ts.logBuffer {
		if l.Time.Before(threshold) {
			continue
		}
		p, ok := agg[l.Path]
		if !ok {
			p = &pathAgg{}
			agg[l.Path] = p
		}
		p.total++
		p.totalDur += l.Duration
		p.durations = append(p.durations, l.Duration)
		if l.Duration > p.maxDur {
			p.maxDur = l.Duration
		}
		if l.StatusCode >= 400 {
			p.errors++
		}
	}

	stats := make([]EndpointStat, 0, len(agg))
	for path, p := range agg {
		s := EndpointStat{
			Path:          path,
			TotalRequests: p.total,
			ErrorCount:    p.errors,
			MaxDurationMs: p.maxDur,
		}
		if p.total > 0 {
			s.AvgDurationMs = p.totalDur / float64(p.total)
			sort.Float64s(p.durations)
			p95Idx := int(float64(len(p.durations)) * 0.95)
			if p95Idx >= len(p.durations) {
				p95Idx = len(p.durations) - 1
			}
			s.P95DurationMs = p.durations[p95Idx]
		}
		stats = append(stats, s)
	}

	sort.Slice(stats, func(i, j int) bool {
		return less(stats[i], stats[j])
	})

	if len(stats) > limit {
		stats = stats[:limit]
	}
	return stats
}
