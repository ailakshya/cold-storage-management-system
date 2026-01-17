package repositories

import (
	"context"
	"fmt"
	"time"

	"cold-backend/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MetricsRepository handles all metrics database operations
type MetricsRepository struct {
	pool *pgxpool.Pool
}

// NewMetricsRepository creates a new metrics repository
func NewMetricsRepository(pool *pgxpool.Pool) *MetricsRepository {
	return &MetricsRepository{pool: pool}
}

// ======================================
// API Request Logs
// ======================================

// InsertAPILog inserts a new API request log
func (r *MetricsRepository) InsertAPILog(ctx context.Context, log *models.APIRequestLog) error {
	query := `
		INSERT INTO api_request_logs (
			method, path, status_code, duration_ms, request_size, response_size,
			user_id, ip_address, user_agent, error_message
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := r.pool.Exec(ctx, query,
		log.Method, log.Path, log.StatusCode, log.DurationMs,
		log.RequestSize, log.ResponseSize, log.UserID,
		log.IPAddress, log.UserAgent, log.ErrorMessage)
	return err
}

// GetAPIAnalytics returns aggregated API statistics for a time range
func (r *MetricsRepository) GetAPIAnalytics(ctx context.Context, duration time.Duration) (*models.APIAnalytics, error) {
	query := `
		SELECT
			COUNT(*) AS total_requests,
			COUNT(*) FILTER (WHERE status_code < 400) AS success_requests,
			COUNT(*) FILTER (WHERE status_code >= 400) AS error_requests,
			COALESCE(AVG(duration_ms), 0) AS avg_duration,
			COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_ms), 0) AS p95_duration,
			COALESCE(MAX(duration_ms), 0) AS max_duration,
			COALESCE(SUM(request_size), 0) AS total_request_bytes,
			COALESCE(SUM(response_size), 0) AS total_response_bytes,
			COUNT(DISTINCT user_id) AS unique_users
		FROM api_request_logs
		WHERE created_at > NOW() - $1::INTERVAL`

	var analytics models.APIAnalytics
	analytics.TimeRange = duration.String()

	err := r.pool.QueryRow(ctx, query, duration.String()).Scan(
		&analytics.TotalRequests, &analytics.SuccessRequests, &analytics.ErrorRequests,
		&analytics.AvgDurationMs, &analytics.P95DurationMs, &analytics.MaxDurationMs,
		&analytics.TotalRequestBytes, &analytics.TotalResponseBytes, &analytics.UniqueUsers)
	if err != nil {
		return nil, err
	}

	if analytics.TotalRequests > 0 {
		analytics.ErrorRate = float64(analytics.ErrorRequests) / float64(analytics.TotalRequests) * 100
	}

	// Get top endpoints
	analytics.TopEndpoints, _ = r.GetTopEndpoints(ctx, duration, 10)

	// Get errors by status
	analytics.ErrorsByStatus, _ = r.GetErrorsByStatus(ctx, duration)

	return &analytics, nil
}

// GetTopEndpoints returns top N endpoints by request count
func (r *MetricsRepository) GetTopEndpoints(ctx context.Context, duration time.Duration, limit int) ([]models.EndpointStats, error) {
	query := `
		SELECT
			path,
			COUNT(*) AS total_requests,
			COUNT(*) FILTER (WHERE status_code >= 400) AS error_count,
			AVG(duration_ms) AS avg_duration,
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_ms) AS p95_duration,
			MAX(duration_ms) AS max_duration
		FROM api_request_logs
		WHERE created_at > NOW() - $1::INTERVAL
		GROUP BY path
		ORDER BY total_requests DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, query, duration.String(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var endpoints []models.EndpointStats
	for rows.Next() {
		var ep models.EndpointStats
		if err := rows.Scan(&ep.Path, &ep.TotalRequests, &ep.ErrorCount,
			&ep.AvgDurationMs, &ep.P95DurationMs, &ep.MaxDurationMs); err != nil {
			continue
		}
		endpoints = append(endpoints, ep)
	}
	return endpoints, nil
}

// GetSlowestEndpoints returns slowest endpoints by average duration
func (r *MetricsRepository) GetSlowestEndpoints(ctx context.Context, duration time.Duration, limit int) ([]models.EndpointStats, error) {
	query := `
		SELECT
			path,
			COUNT(*) AS total_requests,
			COUNT(*) FILTER (WHERE status_code >= 400) AS error_count,
			AVG(duration_ms) AS avg_duration,
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_ms) AS p95_duration,
			MAX(duration_ms) AS max_duration
		FROM api_request_logs
		WHERE created_at > NOW() - $1::INTERVAL
		GROUP BY path
		HAVING COUNT(*) >= 10
		ORDER BY avg_duration DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, query, duration.String(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var endpoints []models.EndpointStats
	for rows.Next() {
		var ep models.EndpointStats
		if err := rows.Scan(&ep.Path, &ep.TotalRequests, &ep.ErrorCount,
			&ep.AvgDurationMs, &ep.P95DurationMs, &ep.MaxDurationMs); err != nil {
			continue
		}
		endpoints = append(endpoints, ep)
	}
	return endpoints, nil
}

// GetErrorsByStatus returns error counts grouped by status code
func (r *MetricsRepository) GetErrorsByStatus(ctx context.Context, duration time.Duration) (map[int]int64, error) {
	query := `
		SELECT status_code, COUNT(*) AS count
		FROM api_request_logs
		WHERE created_at > NOW() - $1::INTERVAL AND status_code >= 400
		GROUP BY status_code
		ORDER BY count DESC`

	rows, err := r.pool.Query(ctx, query, duration.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int]int64)
	for rows.Next() {
		var statusCode int
		var count int64
		if err := rows.Scan(&statusCode, &count); err != nil {
			continue
		}
		result[statusCode] = count
	}
	return result, nil
}

// GetRecentAPILogs returns recent API request logs
func (r *MetricsRepository) GetRecentAPILogs(ctx context.Context, limit int, offset int) ([]models.APIRequestLog, error) {
	query := `
		SELECT created_at, request_id, method, path, status_code, duration_ms,
			request_size, response_size, user_id,
			ip_address, user_agent, error_message
		FROM api_request_logs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.APIRequestLog
	for rows.Next() {
		var log models.APIRequestLog
		if err := rows.Scan(&log.Time, &log.RequestID, &log.Method, &log.Path,
			&log.StatusCode, &log.DurationMs, &log.RequestSize, &log.ResponseSize,
			&log.UserID, &log.IPAddress, &log.UserAgent, &log.ErrorMessage); err != nil {
			continue
		}
		logs = append(logs, log)
	}
	return logs, nil
}

// ======================================
// Node Metrics
// ======================================

// InsertNodeMetrics inserts node metrics
func (r *MetricsRepository) InsertNodeMetrics(ctx context.Context, metrics *models.NodeMetrics) error {
	query := `
		INSERT INTO node_metrics (
			node_name, node_ip, node_role, node_status, cpu_percent, cpu_cores,
			memory_used_bytes, memory_total_bytes, memory_percent,
			disk_used_bytes, disk_total_bytes, disk_percent,
			network_rx_bytes, network_tx_bytes, network_rx_rate, network_tx_rate,
			pod_count, load_average_1m, load_average_5m, load_average_15m
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)`

	_, err := r.pool.Exec(ctx, query,
		metrics.NodeName, metrics.NodeIP, metrics.NodeRole, metrics.NodeStatus,
		metrics.CPUPercent, metrics.CPUCores, metrics.MemoryUsedBytes, metrics.MemoryTotalBytes,
		metrics.MemoryPercent, metrics.DiskUsedBytes, metrics.DiskTotalBytes, metrics.DiskPercent,
		metrics.NetworkRxBytes, metrics.NetworkTxBytes, metrics.NetworkRxRate, metrics.NetworkTxRate,
		metrics.PodCount, metrics.LoadAverage1m, metrics.LoadAverage5m, metrics.LoadAverage15m)
	return err
}

// GetLatestNodeMetrics returns the latest metrics for all nodes
func (r *MetricsRepository) GetLatestNodeMetrics(ctx context.Context) ([]models.NodeMetrics, error) {
	query := `
		SELECT DISTINCT ON (node_name)
			time, node_name, node_ip, node_role, node_status, cpu_percent, cpu_cores,
			memory_used_bytes, memory_total_bytes, memory_percent,
			disk_used_bytes, disk_total_bytes, disk_percent,
			network_rx_bytes, network_tx_bytes, network_rx_rate, network_tx_rate,
			pod_count, load_average_1m, load_average_5m, load_average_15m
		FROM node_metrics
		WHERE time > NOW() - INTERVAL '5 minutes'
		ORDER BY node_name, time DESC`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []models.NodeMetrics
	for rows.Next() {
		var m models.NodeMetrics
		if err := rows.Scan(&m.Time, &m.NodeName, &m.NodeIP, &m.NodeRole, &m.NodeStatus,
			&m.CPUPercent, &m.CPUCores, &m.MemoryUsedBytes, &m.MemoryTotalBytes, &m.MemoryPercent,
			&m.DiskUsedBytes, &m.DiskTotalBytes, &m.DiskPercent,
			&m.NetworkRxBytes, &m.NetworkTxBytes, &m.NetworkRxRate, &m.NetworkTxRate,
			&m.PodCount, &m.LoadAverage1m, &m.LoadAverage5m, &m.LoadAverage15m); err != nil {
			continue
		}
		metrics = append(metrics, m)
	}
	return metrics, nil
}

// GetNodeMetricsHistory returns historical metrics for a node
func (r *MetricsRepository) GetNodeMetricsHistory(ctx context.Context, nodeName string, duration time.Duration, interval string) ([]models.NodeMetrics, error) {
	query := fmt.Sprintf(`
		SELECT
			time_bucket('%s', time) AS bucket,
			node_name, node_ip, node_role, node_status,
			AVG(cpu_percent) AS cpu_percent, MAX(cpu_cores) AS cpu_cores,
			AVG(memory_used_bytes)::BIGINT AS memory_used_bytes,
			MAX(memory_total_bytes) AS memory_total_bytes,
			AVG(memory_percent) AS memory_percent,
			AVG(disk_used_bytes)::BIGINT AS disk_used_bytes,
			MAX(disk_total_bytes) AS disk_total_bytes,
			AVG(disk_percent) AS disk_percent,
			MAX(network_rx_bytes) AS network_rx_bytes,
			MAX(network_tx_bytes) AS network_tx_bytes,
			AVG(network_rx_rate)::BIGINT AS network_rx_rate,
			AVG(network_tx_rate)::BIGINT AS network_tx_rate,
			AVG(pod_count)::INT AS pod_count,
			AVG(load_average_1m) AS load_average_1m,
			AVG(load_average_5m) AS load_average_5m,
			AVG(load_average_15m) AS load_average_15m
		FROM node_metrics
		WHERE node_name = $1 AND time > NOW() - $2::INTERVAL
		GROUP BY bucket, node_name, node_ip, node_role, node_status
		ORDER BY bucket DESC`, interval)

	rows, err := r.pool.Query(ctx, query, nodeName, duration.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []models.NodeMetrics
	for rows.Next() {
		var m models.NodeMetrics
		if err := rows.Scan(&m.Time, &m.NodeName, &m.NodeIP, &m.NodeRole, &m.NodeStatus,
			&m.CPUPercent, &m.CPUCores, &m.MemoryUsedBytes, &m.MemoryTotalBytes, &m.MemoryPercent,
			&m.DiskUsedBytes, &m.DiskTotalBytes, &m.DiskPercent,
			&m.NetworkRxBytes, &m.NetworkTxBytes, &m.NetworkRxRate, &m.NetworkTxRate,
			&m.PodCount, &m.LoadAverage1m, &m.LoadAverage5m, &m.LoadAverage15m); err != nil {
			continue
		}
		metrics = append(metrics, m)
	}
	return metrics, nil
}

// GetClusterOverview returns aggregated cluster statistics
func (r *MetricsRepository) GetClusterOverview(ctx context.Context) (*models.ClusterOverview, error) {
	query := `
		WITH latest AS (
			SELECT DISTINCT ON (node_name) *
			FROM node_metrics
			WHERE time > NOW() - INTERVAL '5 minutes'
			ORDER BY node_name, time DESC
		)
		SELECT
			COUNT(*) AS total_nodes,
			COUNT(*) FILTER (WHERE node_status = 'Ready') AS healthy_nodes,
			COALESCE(SUM(cpu_cores), 0) AS total_cpu_cores,
			COALESCE(AVG(cpu_percent), 0) AS avg_cpu_percent,
			COALESCE(SUM(memory_total_bytes) / 1073741824.0, 0) AS total_memory_gb,
			COALESCE(SUM(memory_used_bytes) / 1073741824.0, 0) AS used_memory_gb,
			COALESCE(AVG(memory_percent), 0) AS avg_memory_percent,
			COALESCE(SUM(disk_total_bytes) / 1073741824.0, 0) AS total_disk_gb,
			COALESCE(SUM(disk_used_bytes) / 1073741824.0, 0) AS used_disk_gb,
			COALESCE(AVG(disk_percent), 0) AS avg_disk_percent,
			COALESCE(SUM(pod_count), 0) AS total_pods,
			COALESCE(AVG(load_average_1m), 0) AS avg_load_average
		FROM latest`

	var overview models.ClusterOverview
	err := r.pool.QueryRow(ctx, query).Scan(
		&overview.TotalNodes, &overview.HealthyNodes, &overview.TotalCPUCores,
		&overview.AvgCPUPercent, &overview.TotalMemoryGB, &overview.UsedMemoryGB,
		&overview.AvgMemoryPercent, &overview.TotalDiskGB, &overview.UsedDiskGB,
		&overview.AvgDiskPercent, &overview.TotalPods, &overview.AvgLoadAverage)
	if err != nil {
		if err == pgx.ErrNoRows {
			return &overview, nil
		}
		return nil, err
	}
	overview.LastUpdated = time.Now()
	return &overview, nil
}

// ======================================
// PostgreSQL Metrics
// ======================================

// InsertPostgresMetrics inserts PostgreSQL pod metrics
func (r *MetricsRepository) InsertPostgresMetrics(ctx context.Context, metrics *models.PostgresMetrics) error {
	query := `
		INSERT INTO postgres_metrics (
			pod_name, node_name, role, status, database_size_bytes,
			active_connections, idle_connections, total_connections, max_connections,
			replication_lag_seconds, transactions_committed, transactions_rolled_back,
			blocks_read, blocks_hit, cache_hit_ratio
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`

	_, err := r.pool.Exec(ctx, query,
		metrics.PodName, metrics.NodeName, metrics.Role, metrics.Status,
		metrics.DatabaseSizeBytes, metrics.ActiveConnections, metrics.IdleConnections,
		metrics.TotalConnections, metrics.MaxConnections, metrics.ReplicationLagSeconds,
		metrics.TransactionsCommitted, metrics.TransactionsRolledBack,
		metrics.BlocksRead, metrics.BlocksHit, metrics.CacheHitRatio)
	return err
}

// GetLatestPostgresMetrics returns the latest metrics for all PostgreSQL pods
func (r *MetricsRepository) GetLatestPostgresMetrics(ctx context.Context) ([]models.PostgresMetrics, error) {
	query := `
		SELECT DISTINCT ON (pod_name)
			time, pod_name, node_name, role, status, database_size_bytes,
			active_connections, idle_connections, total_connections, max_connections,
			replication_lag_seconds, transactions_committed, transactions_rolled_back,
			blocks_read, blocks_hit, cache_hit_ratio
		FROM postgres_metrics
		WHERE time > NOW() - INTERVAL '5 minutes'
		ORDER BY pod_name, time DESC`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []models.PostgresMetrics
	for rows.Next() {
		var m models.PostgresMetrics
		if err := rows.Scan(&m.Time, &m.PodName, &m.NodeName, &m.Role, &m.Status,
			&m.DatabaseSizeBytes, &m.ActiveConnections, &m.IdleConnections,
			&m.TotalConnections, &m.MaxConnections, &m.ReplicationLagSeconds,
			&m.TransactionsCommitted, &m.TransactionsRolledBack,
			&m.BlocksRead, &m.BlocksHit, &m.CacheHitRatio); err != nil {
			continue
		}
		metrics = append(metrics, m)
	}
	return metrics, nil
}

// GetPostgresOverview returns aggregated PostgreSQL cluster statistics
func (r *MetricsRepository) GetPostgresOverview(ctx context.Context) (*models.PostgresOverview, error) {
	query := `
		WITH latest AS (
			SELECT DISTINCT ON (pod_name) *
			FROM postgres_metrics
			WHERE time > NOW() - INTERVAL '5 minutes'
			ORDER BY pod_name, time DESC
		)
		SELECT
			COUNT(*) AS total_pods,
			COUNT(*) FILTER (WHERE status = 'Running') AS healthy_pods,
			COALESCE((SELECT pod_name FROM latest WHERE role = 'Primary' LIMIT 1), '') AS primary_pod,
			COALESCE(SUM(database_size_bytes) / 1073741824.0, 0) AS total_database_size_gb,
			COALESCE(SUM(active_connections), 0) AS total_connections,
			COALESCE(MAX(replication_lag_seconds), 0) AS max_replication_lag,
			COALESCE(AVG(cache_hit_ratio), 0) AS avg_cache_hit_ratio
		FROM latest`

	var overview models.PostgresOverview
	err := r.pool.QueryRow(ctx, query).Scan(
		&overview.TotalPods, &overview.HealthyPods, &overview.PrimaryPod,
		&overview.TotalDatabaseSizeGB, &overview.TotalConnections,
		&overview.MaxReplicationLag, &overview.AvgCacheHitRatio)
	if err != nil {
		if err == pgx.ErrNoRows {
			return &overview, nil
		}
		return nil, err
	}
	overview.LastUpdated = time.Now()
	return &overview, nil
}

// ======================================
// Alerts
// ======================================

// InsertAlert inserts a new monitoring alert
func (r *MetricsRepository) InsertAlert(ctx context.Context, alert *models.MonitoringAlert) error {
	query := `
		INSERT INTO monitoring_alerts (
			alert_type, severity, source, title, message,
			metric_name, metric_value, threshold_value, node_name
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`

	return r.pool.QueryRow(ctx, query,
		alert.AlertType, alert.Severity, alert.Source, alert.Title, alert.Message,
		alert.MetricName, alert.MetricValue, alert.ThresholdValue, alert.NodeName).Scan(&alert.ID)
}

// GetActiveAlerts returns unresolved alerts
func (r *MetricsRepository) GetActiveAlerts(ctx context.Context) ([]models.MonitoringAlert, error) {
	query := `
		SELECT id, time, alert_type, severity, source, title, message,
			metric_name, metric_value, threshold_value, node_name,
			acknowledged, acknowledged_by, acknowledged_at, resolved, resolved_at
		FROM monitoring_alerts
		WHERE resolved = FALSE
		ORDER BY
			CASE severity WHEN 'critical' THEN 1 WHEN 'warning' THEN 2 ELSE 3 END,
			time DESC`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []models.MonitoringAlert
	for rows.Next() {
		var a models.MonitoringAlert
		if err := rows.Scan(&a.ID, &a.Time, &a.AlertType, &a.Severity, &a.Source,
			&a.Title, &a.Message, &a.MetricName, &a.MetricValue, &a.ThresholdValue,
			&a.NodeName, &a.Acknowledged, &a.AcknowledgedBy, &a.AcknowledgedAt,
			&a.Resolved, &a.ResolvedAt); err != nil {
			continue
		}
		alerts = append(alerts, a)
	}
	return alerts, nil
}

// GetRecentAlerts returns recent alerts
func (r *MetricsRepository) GetRecentAlerts(ctx context.Context, limit int) ([]models.MonitoringAlert, error) {
	query := `
		SELECT id, time, alert_type, severity, source, title, message,
			metric_name, metric_value, threshold_value, node_name,
			acknowledged, acknowledged_by, acknowledged_at, resolved, resolved_at
		FROM monitoring_alerts
		ORDER BY time DESC
		LIMIT $1`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []models.MonitoringAlert
	for rows.Next() {
		var a models.MonitoringAlert
		if err := rows.Scan(&a.ID, &a.Time, &a.AlertType, &a.Severity, &a.Source,
			&a.Title, &a.Message, &a.MetricName, &a.MetricValue, &a.ThresholdValue,
			&a.NodeName, &a.Acknowledged, &a.AcknowledgedBy, &a.AcknowledgedAt,
			&a.Resolved, &a.ResolvedAt); err != nil {
			continue
		}
		alerts = append(alerts, a)
	}
	return alerts, nil
}

// AcknowledgeAlert marks an alert as acknowledged
func (r *MetricsRepository) AcknowledgeAlert(ctx context.Context, alertID int, acknowledgedBy string) error {
	query := `
		UPDATE monitoring_alerts
		SET acknowledged = TRUE, acknowledged_by = $2, acknowledged_at = NOW()
		WHERE id = $1`

	_, err := r.pool.Exec(ctx, query, alertID, acknowledgedBy)
	return err
}

// ResolveAlert marks an alert as resolved
func (r *MetricsRepository) ResolveAlert(ctx context.Context, alertID int) error {
	query := `
		UPDATE monitoring_alerts
		SET resolved = TRUE, resolved_at = NOW()
		WHERE id = $1`

	_, err := r.pool.Exec(ctx, query, alertID)
	return err
}

// GetAlertSummary returns alert statistics
func (r *MetricsRepository) GetAlertSummary(ctx context.Context) (*models.AlertSummary, error) {
	query := `
		SELECT
			COUNT(*) AS total_alerts,
			COUNT(*) FILTER (WHERE severity = 'critical' AND resolved = FALSE) AS critical_alerts,
			COUNT(*) FILTER (WHERE severity = 'warning' AND resolved = FALSE) AS warning_alerts,
			COUNT(*) FILTER (WHERE severity = 'info' AND resolved = FALSE) AS info_alerts,
			COUNT(*) FILTER (WHERE acknowledged = FALSE AND resolved = FALSE) AS unacknowledged_alerts,
			COUNT(*) FILTER (WHERE resolved = FALSE) AS unresolved_alerts
		FROM monitoring_alerts
		WHERE time > NOW() - INTERVAL '24 hours'`

	var summary models.AlertSummary
	err := r.pool.QueryRow(ctx, query).Scan(
		&summary.TotalAlerts, &summary.CriticalAlerts, &summary.WarningAlerts,
		&summary.InfoAlerts, &summary.UnacknowledgedAlerts, &summary.UnresolvedAlerts)
	if err != nil {
		return &summary, nil
	}
	return &summary, nil
}

// GetAlertThresholds returns all alert thresholds
func (r *MetricsRepository) GetAlertThresholds(ctx context.Context) ([]models.AlertThreshold, error) {
	query := `
		SELECT id, metric_name, display_name, warning_threshold, critical_threshold,
			comparison, enabled, cooldown_minutes, description, created_at, updated_at
		FROM alert_thresholds
		ORDER BY metric_name`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var thresholds []models.AlertThreshold
	for rows.Next() {
		var t models.AlertThreshold
		if err := rows.Scan(&t.ID, &t.MetricName, &t.DisplayName, &t.WarningThreshold,
			&t.CriticalThreshold, &t.Comparison, &t.Enabled, &t.CooldownMinutes,
			&t.Description, &t.CreatedAt, &t.UpdatedAt); err != nil {
			continue
		}
		thresholds = append(thresholds, t)
	}
	return thresholds, nil
}

// UpdateAlertThreshold updates an alert threshold
func (r *MetricsRepository) UpdateAlertThreshold(ctx context.Context, threshold *models.AlertThreshold) error {
	query := `
		UPDATE alert_thresholds
		SET warning_threshold = $2, critical_threshold = $3, enabled = $4,
			cooldown_minutes = $5, updated_at = NOW()
		WHERE id = $1`

	_, err := r.pool.Exec(ctx, query, threshold.ID, threshold.WarningThreshold,
		threshold.CriticalThreshold, threshold.Enabled, threshold.CooldownMinutes)
	return err
}

// ======================================
// VIP & Backup Status
// ======================================

// InsertVIPStatus inserts VIP health check result
func (r *MetricsRepository) InsertVIPStatus(ctx context.Context, status *models.VIPStatus) error {
	query := `
		INSERT INTO vip_status (vip_address, is_healthy, response_time_ms, active_node, message)
		VALUES ($1, $2, $3, $4, $5)`

	_, err := r.pool.Exec(ctx, query,
		status.VIPAddress, status.IsHealthy, status.ResponseTimeMs, status.ActiveNode, status.Message)
	return err
}

// InsertBackupHistory inserts backup operation record
func (r *MetricsRepository) InsertBackupHistory(ctx context.Context, backup *models.BackupHistory) error {
	query := `
		INSERT INTO backup_history (backup_type, status, source, destination, size_bytes, duration_seconds, error_message, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.pool.Exec(ctx, query,
		backup.BackupType, backup.Status, backup.Source, backup.Destination,
		backup.SizeBytes, backup.DurationSeconds, backup.ErrorMessage, backup.Metadata)
	return err
}

// GetRecentBackups returns recent backup history
func (r *MetricsRepository) GetRecentBackups(ctx context.Context, limit int) ([]models.BackupHistory, error) {
	query := `
		SELECT time, backup_type, status, source, destination, size_bytes, duration_seconds, error_message
		FROM backup_history
		ORDER BY time DESC
		LIMIT $1`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var backups []models.BackupHistory
	for rows.Next() {
		var b models.BackupHistory
		if err := rows.Scan(&b.Time, &b.BackupType, &b.Status, &b.Source,
			&b.Destination, &b.SizeBytes, &b.DurationSeconds, &b.ErrorMessage); err != nil {
			continue
		}
		backups = append(backups, b)
	}
	return backups, nil
}
