package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"cold-backend/internal/models"
	"cold-backend/internal/repositories"

	"github.com/gorilla/mux"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// MonitoringHandler handles monitoring API endpoints
type MonitoringHandler struct {
	repo *repositories.MetricsRepository
}

// NewMonitoringHandler creates a new monitoring handler
func NewMonitoringHandler(repo *repositories.MetricsRepository) *MonitoringHandler {
	return &MonitoringHandler{repo: repo}
}

// ======================================
// Dashboard Overview
// ======================================

// GetDashboardData returns all data for the monitoring dashboard
func (h *MonitoringHandler) GetDashboardData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get cluster overview
	clusterOverview, _ := h.repo.GetClusterOverview(ctx)

	// Get PostgreSQL overview
	postgresOverview, _ := h.repo.GetPostgresOverview(ctx)

	// Get API analytics (last hour)
	apiAnalytics, _ := h.repo.GetAPIAnalytics(ctx, 1*time.Hour)

	// Get alert summary
	alertSummary, _ := h.repo.GetAlertSummary(ctx)

	// Get recent alerts
	recentAlerts, _ := h.repo.GetRecentAlerts(ctx, 10)

	// Get latest node metrics
	nodes, _ := h.repo.GetLatestNodeMetrics(ctx)

	// Get latest PostgreSQL metrics
	postgresPods, _ := h.repo.GetLatestPostgresMetrics(ctx)

	response := map[string]interface{}{
		"cluster_overview":  clusterOverview,
		"postgres_overview": postgresOverview,
		"api_analytics":     apiAnalytics,
		"alert_summary":     alertSummary,
		"recent_alerts":     recentAlerts,
		"nodes":             nodes,
		"postgres_pods":     postgresPods,
		"last_updated":      time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ======================================
// API Analytics Endpoints
// ======================================

// GetAPIAnalytics returns API usage statistics
func (h *MonitoringHandler) GetAPIAnalytics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse time range from query params
	rangeParam := r.URL.Query().Get("range")
	duration := parseDuration(rangeParam, 1*time.Hour)

	analytics, err := h.repo.GetAPIAnalytics(ctx, duration)
	if err != nil {
		http.Error(w, "Failed to get API analytics", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(analytics)
}

// GetTopEndpoints returns top endpoints by request count
func (h *MonitoringHandler) GetTopEndpoints(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rangeParam := r.URL.Query().Get("range")
	duration := parseDuration(rangeParam, 1*time.Hour)

	limitParam := r.URL.Query().Get("limit")
	limit := 10
	if l, err := strconv.Atoi(limitParam); err == nil && l > 0 {
		limit = l
	}

	endpoints, err := h.repo.GetTopEndpoints(ctx, duration, limit)
	if err != nil {
		http.Error(w, "Failed to get top endpoints", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"endpoints": endpoints,
		"range":     duration.String(),
	})
}

// GetSlowestEndpoints returns slowest endpoints by average duration
func (h *MonitoringHandler) GetSlowestEndpoints(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rangeParam := r.URL.Query().Get("range")
	duration := parseDuration(rangeParam, 1*time.Hour)

	limitParam := r.URL.Query().Get("limit")
	limit := 10
	if l, err := strconv.Atoi(limitParam); err == nil && l > 0 {
		limit = l
	}

	endpoints, err := h.repo.GetSlowestEndpoints(ctx, duration, limit)
	if err != nil {
		http.Error(w, "Failed to get slowest endpoints", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"endpoints": endpoints,
		"range":     duration.String(),
	})
}

// GetRecentAPILogs returns recent API request logs
func (h *MonitoringHandler) GetRecentAPILogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limitParam := r.URL.Query().Get("limit")
	limit := 100
	if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 500 {
		limit = l
	}

	offsetParam := r.URL.Query().Get("offset")
	offset := 0
	if o, err := strconv.Atoi(offsetParam); err == nil && o >= 0 {
		offset = o
	}

	logs, err := h.repo.GetRecentAPILogs(ctx, limit, offset)
	if err != nil {
		http.Error(w, "Failed to get API logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs":   logs,
		"limit":  limit,
		"offset": offset,
	})
}

// ======================================
// Node Metrics Endpoints
// ======================================

// GetLatestNodeMetrics returns the latest metrics for all nodes
func (h *MonitoringHandler) GetLatestNodeMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	nodes, err := h.repo.GetLatestNodeMetrics(ctx)
	if err != nil {
		http.Error(w, "Failed to get node metrics", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes":        nodes,
		"last_updated": time.Now(),
	})
}

// GetNodeMetricsHistory returns historical metrics for a node
func (h *MonitoringHandler) GetNodeMetricsHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	nodeName := vars["name"]

	if nodeName == "" {
		http.Error(w, "Node name is required", http.StatusBadRequest)
		return
	}

	rangeParam := r.URL.Query().Get("range")
	duration := parseDuration(rangeParam, 1*time.Hour)

	// Determine interval based on duration
	interval := "1 minute"
	if duration > 6*time.Hour {
		interval = "5 minutes"
	}
	if duration > 24*time.Hour {
		interval = "15 minutes"
	}
	if duration > 7*24*time.Hour {
		interval = "1 hour"
	}

	metrics, err := h.repo.GetNodeMetricsHistory(ctx, nodeName, duration, interval)
	if err != nil {
		http.Error(w, "Failed to get node history", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"node_name": nodeName,
		"metrics":   metrics,
		"range":     duration.String(),
		"interval":  interval,
	})
}

// GetClusterOverview returns aggregated cluster statistics
func (h *MonitoringHandler) GetClusterOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	overview, err := h.repo.GetClusterOverview(ctx)
	if err != nil {
		http.Error(w, "Failed to get cluster overview", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(overview)
}

// ======================================
// PostgreSQL Metrics Endpoints
// ======================================

// GetLatestPostgresMetrics returns the latest metrics for all PostgreSQL pods
func (h *MonitoringHandler) GetLatestPostgresMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	pods, err := h.repo.GetLatestPostgresMetrics(ctx)
	if err != nil {
		http.Error(w, "Failed to get PostgreSQL metrics", http.StatusInternalServerError)
		return
	}

	// Append streaming-replica (external replica on 192.168.15.195)
	if metricsDBPod := h.getMetricsDBMetrics(); metricsDBPod != nil {
		pods = append(pods, *metricsDBPod)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"pods":         pods,
		"last_updated": time.Now(),
	})
}

// getMetricsDBMetrics queries the external metrics database on 192.168.15.195
func (h *MonitoringHandler) getMetricsDBMetrics() *models.PostgresMetrics {
	host := "192.168.15.195"
	port := "5434" // Streaming replica of K8s cluster

	// Connect to streaming replica
	connStr := fmt.Sprintf("host=%s port=%s user=postgres password=postgres dbname=cold_db sslmode=disable connect_timeout=5", host, port)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil
	}
	defer db.Close()

	metrics := &models.PostgresMetrics{
		Time:           time.Now(),
		PodName:        "streaming-replica (192.168.15.195)",
		NodeName:       host,
		Role:           "Unknown",
		Status:         "Running",
		MaxConnections: 200,
	}

	// Check if this is actually a replica or standalone
	var isInRecovery bool
	err = db.QueryRowContext(ctx, "SELECT pg_is_in_recovery()").Scan(&isInRecovery)
	if err != nil {
		metrics.Status = "Error"
		metrics.Role = "Unknown"
		return metrics
	}

	if isInRecovery {
		metrics.Role = "Replica"
		// Get replication lag using WAL bytes difference (accurate even when idle)
		var replLag sql.NullFloat64
		err = db.QueryRowContext(ctx, `
			SELECT COALESCE(pg_wal_lsn_diff(pg_last_wal_receive_lsn(), pg_last_wal_replay_lsn()), 0)::float
		`).Scan(&replLag)
		if err == nil && replLag.Valid && replLag.Float64 >= 0 {
			metrics.ReplicationLagSeconds = replLag.Float64 // Note: Now stores bytes, not seconds
		}
	} else {
		metrics.Role = "Standalone"
		metrics.ReplicationLagSeconds = -1 // Indicates N/A for standalone
	}

	// Get database size
	var sizeBytes sql.NullInt64
	err = db.QueryRowContext(ctx, "SELECT pg_database_size('metrics_db')").Scan(&sizeBytes)
	if err != nil {
		metrics.Status = "Error"
		return metrics
	}
	if sizeBytes.Valid {
		metrics.DatabaseSizeBytes = sizeBytes.Int64
	}

	// Get active connections
	var activeConn sql.NullInt64
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM pg_stat_activity WHERE datname = 'metrics_db' AND state = 'active'").Scan(&activeConn)
	if err == nil && activeConn.Valid {
		metrics.ActiveConnections = int(activeConn.Int64)
	}

	// Get total connections
	var totalConn sql.NullInt64
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM pg_stat_activity WHERE datname = 'metrics_db'").Scan(&totalConn)
	if err == nil && totalConn.Valid {
		metrics.TotalConnections = int(totalConn.Int64)
	}

	// Get cache hit ratio
	var cacheRatio sql.NullFloat64
	err = db.QueryRowContext(ctx, `
		SELECT COALESCE(
			100.0 * sum(blks_hit) / NULLIF(sum(blks_hit) + sum(blks_read), 0),
			100.0
		) FROM pg_stat_database WHERE datname = 'metrics_db'
	`).Scan(&cacheRatio)
	if err == nil && cacheRatio.Valid {
		metrics.CacheHitRatio = cacheRatio.Float64
	}

	return metrics
}

// GetPostgresOverview returns aggregated PostgreSQL cluster statistics
func (h *MonitoringHandler) GetPostgresOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	overview, err := h.repo.GetPostgresOverview(ctx)
	if err != nil {
		http.Error(w, "Failed to get PostgreSQL overview", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(overview)
}

// ======================================
// Alert Endpoints
// ======================================

// GetActiveAlerts returns unresolved alerts
func (h *MonitoringHandler) GetActiveAlerts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	alerts, err := h.repo.GetActiveAlerts(ctx)
	if err != nil {
		http.Error(w, "Failed to get active alerts", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"alerts": alerts,
		"count":  len(alerts),
	})
}

// GetRecentAlerts returns recent alerts
func (h *MonitoringHandler) GetRecentAlerts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limitParam := r.URL.Query().Get("limit")
	limit := 50
	if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 200 {
		limit = l
	}

	alerts, err := h.repo.GetRecentAlerts(ctx, limit)
	if err != nil {
		http.Error(w, "Failed to get alerts", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"alerts": alerts,
		"limit":  limit,
	})
}

// AcknowledgeAlert marks an alert as acknowledged
func (h *MonitoringHandler) AcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)

	alertID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid alert ID", http.StatusBadRequest)
		return
	}

	// Get user email from context
	acknowledgedBy := "admin"
	if claims, ok := r.Context().Value("claims").(map[string]interface{}); ok {
		if email, ok := claims["email"].(string); ok {
			acknowledgedBy = email
		}
	}

	if err := h.repo.AcknowledgeAlert(ctx, alertID, acknowledgedBy); err != nil {
		http.Error(w, "Failed to acknowledge alert", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Alert acknowledged",
	})
}

// ResolveAlert marks an alert as resolved
func (h *MonitoringHandler) ResolveAlert(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)

	alertID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid alert ID", http.StatusBadRequest)
		return
	}

	if err := h.repo.ResolveAlert(ctx, alertID); err != nil {
		http.Error(w, "Failed to resolve alert", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Alert resolved",
	})
}

// GetAlertSummary returns alert statistics
func (h *MonitoringHandler) GetAlertSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	summary, err := h.repo.GetAlertSummary(ctx)
	if err != nil {
		http.Error(w, "Failed to get alert summary", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

// GetAlertThresholds returns all alert thresholds
func (h *MonitoringHandler) GetAlertThresholds(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	thresholds, err := h.repo.GetAlertThresholds(ctx)
	if err != nil {
		http.Error(w, "Failed to get alert thresholds", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"thresholds": thresholds,
	})
}

// UpdateAlertThreshold updates an alert threshold
func (h *MonitoringHandler) UpdateAlertThreshold(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)

	thresholdID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid threshold ID", http.StatusBadRequest)
		return
	}

	var req struct {
		WarningThreshold  float64 `json:"warning_threshold"`
		CriticalThreshold float64 `json:"critical_threshold"`
		Enabled           bool    `json:"enabled"`
		CooldownMinutes   int     `json:"cooldown_minutes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Update the threshold
	threshold := &models.AlertThreshold{
		ID:                thresholdID,
		WarningThreshold:  req.WarningThreshold,
		CriticalThreshold: req.CriticalThreshold,
		Enabled:           req.Enabled,
		CooldownMinutes:   req.CooldownMinutes,
	}
	if err := h.repo.UpdateAlertThreshold(ctx, threshold); err != nil {
		http.Error(w, "Failed to update threshold", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Threshold updated",
	})
}

// ======================================
// Backup History Endpoints
// ======================================

// GetRecentBackups returns recent backup history
func (h *MonitoringHandler) GetRecentBackups(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limitParam := r.URL.Query().Get("limit")
	limit := 20
	if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
		limit = l
	}

	backups, err := h.repo.GetRecentBackups(ctx, limit)
	if err != nil {
		http.Error(w, "Failed to get backup history", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"backups": backups,
		"limit":   limit,
	})
}

// GetBackupDBStatus returns the status of the backup database container
func (h *MonitoringHandler) GetBackupDBStatus(w http.ResponseWriter, r *http.Request) {
	// Fetch status from backup server metrics endpoint
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://192.168.15.195:9100/metrics")

	response := map[string]interface{}{
		"host":             "192.168.15.195",
		"container":        "cold-storage-postgres",
		"healthy":          false,
		"last_backup":      "N/A",
		"total_backups":    0,
		"backup_size":      "N/A",
		"backup_schedule":  "N/A",
		"cpu_percent":      0.0,
		"memory_percent":   0.0,
		"disk_percent":     0,
		"disk_total":       "N/A",
		"disk_used":        "N/A",
		"nas_archive_size": "N/A",
		"nas_last_backup":  "N/A",
	}

	if err == nil {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		var metricsData map[string]interface{}
		if err := json.Unmarshal(body, &metricsData); err == nil {
			// Parse the response
			if healthy, ok := metricsData["healthy"].(bool); ok {
				response["healthy"] = healthy
			}
			if cpu, ok := metricsData["cpu_percent"].(float64); ok {
				response["cpu_percent"] = cpu
			}
			if mem, ok := metricsData["memory_percent"].(float64); ok {
				response["memory_percent"] = mem
			}
			if lastBackup, ok := metricsData["last_backup"].(string); ok {
				response["last_backup"] = lastBackup
			}
			if totalBackups, ok := metricsData["total_backups"].(float64); ok {
				response["total_backups"] = int(totalBackups)
			}
			if totalSize, ok := metricsData["total_size"].(string); ok {
				response["backup_size"] = totalSize
			}
			if schedule, ok := metricsData["backup_schedule"].(string); ok {
				response["backup_schedule"] = schedule
			}
			if nasSize, ok := metricsData["nas_archive_size"].(string); ok {
				response["nas_archive_size"] = nasSize
			}
			if nasLastBackup, ok := metricsData["nas_last_backup"].(string); ok {
				response["nas_last_backup"] = nasLastBackup
			}

			// Parse disk_root
			if diskRoot, ok := metricsData["disk_root"].(map[string]interface{}); ok {
				if percent, ok := diskRoot["percent"].(float64); ok {
					response["disk_percent"] = int(percent)
				}
				if total, ok := diskRoot["total"].(string); ok {
					response["disk_total"] = total
				}
				if used, ok := diskRoot["used"].(string); ok {
					response["disk_used"] = used
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ======================================
// Helper Functions
// ======================================

// parseDuration parses a duration string like "1h", "24h", "7d"
func parseDuration(s string, defaultDuration time.Duration) time.Duration {
	if s == "" {
		return defaultDuration
	}

	// Handle special cases
	switch s {
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "30m":
		return 30 * time.Minute
	case "1h":
		return 1 * time.Hour
	case "3h":
		return 3 * time.Hour
	case "6h":
		return 6 * time.Hour
	case "12h":
		return 12 * time.Hour
	case "24h", "1d":
		return 24 * time.Hour
	case "3d":
		return 3 * 24 * time.Hour
	case "7d", "1w":
		return 7 * 24 * time.Hour
	case "30d":
		return 30 * 24 * time.Hour
	}

	// Try to parse as Go duration
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}

	return defaultDuration
}
