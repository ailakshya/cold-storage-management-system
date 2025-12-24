package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"cold-backend/internal/config"
	"cold-backend/internal/models"
	"cold-backend/internal/repositories"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// R2BackupScheduler handles automatic backups to R2
var (
	r2BackupTicker    *time.Ticker
	r2BackupStopChan  chan bool
	r2BackupMutex     sync.Mutex
	r2BackupInterval  = 1 * time.Minute // Backup every 1 minute for near-zero data loss
	r2BackupDBPool    *pgxpool.Pool     // Shared database pool from main app
	lastBackupTime    time.Time
	pendingChanges    int
	pendingChangesMux sync.Mutex
)

// StartR2BackupScheduler starts the automatic R2 backup scheduler
// Uses the provided database pool for backups (same connection as main app)
func StartR2BackupScheduler(pool *pgxpool.Pool) {
	r2BackupDBPool = pool
	r2BackupMutex.Lock()
	defer r2BackupMutex.Unlock()

	if r2BackupTicker != nil {
		return // Already running
	}

	r2BackupTicker = time.NewTicker(r2BackupInterval)
	r2BackupStopChan = make(chan bool)

	go func() {
		// Run first backup immediately
		log.Println("[R2 Backup] Starting automatic backup scheduler")
		runR2Backup()

		for {
			select {
			case <-r2BackupTicker.C:
				runR2Backup()
			case <-r2BackupStopChan:
				log.Println("[R2 Backup] Scheduler stopped")
				return
			}
		}
	}()

	log.Printf("[R2 Backup] Scheduler started (interval: %v)", r2BackupInterval)
}

// StopR2BackupScheduler stops the automatic backup scheduler
func StopR2BackupScheduler() {
	r2BackupMutex.Lock()
	defer r2BackupMutex.Unlock()

	if r2BackupTicker != nil {
		r2BackupTicker.Stop()
		r2BackupStopChan <- true
		r2BackupTicker = nil
	}
}

// runR2Backup performs a single backup to R2
func runR2Backup() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	log.Println("[R2 Backup] Starting backup...")

	// Create S3 client for R2
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			config.R2AccessKey,
			config.R2SecretKey,
			"",
		)),
		awsconfig.WithRegion(config.R2Region),
	)
	if err != nil {
		log.Printf("[R2 Backup] Failed to configure R2 client: %v", err)
		return
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(config.R2Endpoint)
	})

	// Create database backup
	backupData, err := createR2DatabaseBackup(ctx)
	if err != nil {
		log.Printf("[R2 Backup] Failed to create backup: %v", err)
		return
	}

	// Generate backup filename with IST timestamp
	ist, _ := time.LoadLocation("Asia/Kolkata")
	backupKey := fmt.Sprintf("base/cold_db_%s.sql", time.Now().In(ist).Format("20060102_150405"))

	// Upload to R2
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(config.R2BucketName),
		Key:         aws.String(backupKey),
		Body:        bytes.NewReader(backupData),
		ContentType: aws.String("application/sql"),
	})
	if err != nil {
		log.Printf("[R2 Backup] Failed to upload: %v", err)
		return
	}

	log.Printf("[R2 Backup] Success: %s (%s)", backupKey, formatBytes(int64(len(backupData))))

	// Cleanup old backups (older than 3 days)
	cleanupOldBackups(ctx, client)
}

// cleanupOldBackups deletes backups older than 3 days and failed backups (< 1KB)
func cleanupOldBackups(ctx context.Context, client *s3.Client) {
	maxAge := 3 * 24 * time.Hour
	cutoff := time.Now().Add(-maxAge)
	minValidSize := int64(1024) // 1KB minimum for valid backup

	// List all backups
	result, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(config.R2BucketName),
		Prefix: aws.String("base/"),
	})
	if err != nil {
		log.Printf("[R2 Cleanup] Failed to list backups: %v", err)
		return
	}

	deletedOld := 0
	deletedFailed := 0
	for _, obj := range result.Contents {
		shouldDelete := false
		reason := ""

		// Delete old backups (> 3 days)
		if obj.LastModified != nil && obj.LastModified.Before(cutoff) {
			shouldDelete = true
			reason = "older than 3 days"
		}

		// Delete failed/empty backups (< 1KB)
		if obj.Size != nil && *obj.Size < minValidSize {
			shouldDelete = true
			reason = fmt.Sprintf("failed backup (%d bytes)", *obj.Size)
		}

		if shouldDelete {
			_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(config.R2BucketName),
				Key:    obj.Key,
			})
			if err != nil {
				log.Printf("[R2 Cleanup] Failed to delete %s: %v", *obj.Key, err)
			} else {
				if obj.Size != nil && *obj.Size < minValidSize {
					deletedFailed++
				} else {
					deletedOld++
				}
				log.Printf("[R2 Cleanup] Deleted %s: %s", *obj.Key, reason)
			}
		}
	}

	if deletedOld > 0 || deletedFailed > 0 {
		log.Printf("[R2 Cleanup] Deleted %d old backups, %d failed backups", deletedOld, deletedFailed)
	}
}

// createR2DatabaseBackup creates a SQL backup using the shared database pool
func createR2DatabaseBackup(ctx context.Context) ([]byte, error) {
	// Use the shared database pool (same connection as main app)
	if r2BackupDBPool == nil {
		return nil, fmt.Errorf("database pool not initialized")
	}

	var buffer bytes.Buffer
	buffer.WriteString("-- Cold Storage Database Backup\n")
	buffer.WriteString(fmt.Sprintf("-- Generated: %s\n\n", time.Now().Format(time.RFC3339)))

	tables := []string{
		"users", "customers", "entries", "entry_events", "room_entries",
		"system_settings", "rent_payments", "invoices", "gate_passes",
		"gate_pass_pickups", "guard_entries", "token_colors", "season_requests",
	}

	tablesProcessed := 0
	for _, table := range tables {
		rows, err := r2BackupDBPool.Query(ctx, fmt.Sprintf(`
			SELECT column_name FROM information_schema.columns
			WHERE table_name = '%s' ORDER BY ordinal_position`, table))
		if err != nil {
			log.Printf("[R2 Backup] Warning: failed to get columns for %s: %v", table, err)
			continue
		}

		buffer.WriteString(fmt.Sprintf("\n-- Table: %s\n", table))
		tablesProcessed++

		dataRows, err := r2BackupDBPool.Query(ctx, fmt.Sprintf("SELECT * FROM %s", table))
		if err != nil {
			log.Printf("[R2 Backup] Warning: failed to query %s: %v", table, err)
			rows.Close()
			continue
		}

		// Get column names from field descriptions (pgx v5 API)
		fields := dataRows.FieldDescriptions()
		cols := make([]string, len(fields))
		for i, f := range fields {
			cols[i] = string(f.Name)
		}

		if len(cols) > 0 {
			for dataRows.Next() {
				values, err := dataRows.Values()
				if err != nil {
					continue
				}
				buffer.WriteString(fmt.Sprintf("INSERT INTO %s (%s) VALUES (", table, strings.Join(cols, ", ")))
				for i, v := range values {
					if i > 0 {
						buffer.WriteString(", ")
					}
					if v == nil {
						buffer.WriteString("NULL")
					} else {
						switch val := v.(type) {
						case []byte:
							buffer.WriteString(fmt.Sprintf("'%s'", strings.ReplaceAll(string(val), "'", "''")))
						case string:
							buffer.WriteString(fmt.Sprintf("'%s'", strings.ReplaceAll(val, "'", "''")))
						case time.Time:
							buffer.WriteString(fmt.Sprintf("'%s'", val.Format("2006-01-02 15:04:05")))
						default:
							buffer.WriteString(fmt.Sprintf("%v", val))
						}
					}
				}
				buffer.WriteString(");\n")
			}
		}

		rows.Close()
		dataRows.Close()
	}

	log.Printf("[R2 Backup] Processed %d/%d tables, backup size: %s", tablesProcessed, len(tables), formatBytes(int64(buffer.Len())))
	return buffer.Bytes(), nil
}

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
// R2 Cloud Storage Status
// ======================================

// GetR2Status returns Cloudflare R2 storage status and backup information
func (h *MonitoringHandler) GetR2Status(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	response := map[string]interface{}{
		"connected":     false,
		"endpoint":      "Cloudflare R2",
		"bucket":        "cold-db-backups",
		"total_backups": 0,
		"total_size":    "0 B",
		"last_backup":   "Never",
		"backups":       []interface{}{},
		"error":         "",
	}

	// Get R2 status from setup handler (reuse the same S3 client logic)
	r2Status := getR2StorageStatus(ctx)
	if r2Status != nil {
		for k, v := range r2Status {
			response[k] = v
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

// getR2StorageStatus fetches R2 storage status and backup list
func getR2StorageStatus(ctx context.Context) map[string]interface{} {
	result := make(map[string]interface{})

	// Create S3 client for R2
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			config.R2AccessKey,
			config.R2SecretKey,
			"",
		)),
		awsconfig.WithRegion(config.R2Region),
	)
	if err != nil {
		result["connected"] = false
		result["error"] = "Failed to configure R2 client: " + err.Error()
		return result
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(config.R2Endpoint)
	})

	// Use paginator to handle >1000 objects
	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: aws.String(config.R2BucketName),
		Prefix: aws.String("base/"),
	})

	result["connected"] = true
	result["error"] = ""

	// Calculate total size and find latest backup
	var totalSize int64
	var totalCount int
	var latestTime time.Time
	var latestKey string
	backups := []map[string]interface{}{}
	ist, _ := time.LoadLocation("Asia/Kolkata")

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			result["connected"] = false
			result["error"] = "Failed to list R2 bucket: " + err.Error()
			return result
		}

		for _, obj := range page.Contents {
			totalCount++
			if obj.Size != nil {
				totalSize += *obj.Size
			}
			if obj.LastModified != nil && obj.LastModified.After(latestTime) {
				latestTime = *obj.LastModified
				if obj.Key != nil {
					latestKey = *obj.Key
				}
			}
			backups = append(backups, map[string]interface{}{
				"key":           *obj.Key,
				"size":          formatBytes(*obj.Size),
				"size_bytes":    *obj.Size,
				"last_modified": obj.LastModified.In(ist).Format("2006-01-02 15:04:05"),
			})
		}
	}

	result["total_backups"] = totalCount
	result["total_size"] = formatBytes(totalSize)
	result["total_size_bytes"] = totalSize
	result["backups"] = backups

	if !latestTime.IsZero() {
		// Convert to IST (Indian Standard Time = UTC+5:30)
		ist, _ := time.LoadLocation("Asia/Kolkata")
		result["last_backup"] = latestTime.In(ist).Format("2006-01-02 15:04:05")
		result["last_backup_key"] = latestKey
		result["last_backup_age"] = time.Since(latestTime).Round(time.Minute).String()
	} else {
		result["last_backup"] = "Never"
		result["last_backup_key"] = ""
		result["last_backup_age"] = "N/A"
	}

	return result
}

// BackupToR2 triggers an immediate backup to Cloudflare R2
func (h *MonitoringHandler) BackupToR2(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Create S3 client for R2
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			config.R2AccessKey,
			config.R2SecretKey,
			"",
		)),
		awsconfig.WithRegion(config.R2Region),
	)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to configure R2 client: " + err.Error(),
		})
		return
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(config.R2Endpoint)
	})

	// Get database backup using pg_dump equivalent
	backupData, err := h.createDatabaseBackup(ctx)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to create backup: " + err.Error(),
		})
		return
	}

	// Generate backup filename with IST timestamp
	ist, _ := time.LoadLocation("Asia/Kolkata")
	backupKey := fmt.Sprintf("base/cold_db_%s.sql", time.Now().In(ist).Format("20060102_150405"))

	// Upload to R2
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(config.R2BucketName),
		Key:         aws.String(backupKey),
		Body:        bytes.NewReader(backupData),
		ContentType: aws.String("application/sql"),
	})
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to upload to R2: " + err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"message":     "Backup uploaded to R2 successfully",
		"backup_key":  backupKey,
		"backup_size": formatBytes(int64(len(backupData))),
	})
}

// createDatabaseBackup creates a SQL backup of the database
func (h *MonitoringHandler) createDatabaseBackup(ctx context.Context) ([]byte, error) {
	// Connect to the database
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		"192.168.15.200", 5432, "postgres", "SecurePostgresPassword123", "cold_db")

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer db.Close()

	var buffer bytes.Buffer
	buffer.WriteString("-- Cold Storage Database Backup\n")
	buffer.WriteString(fmt.Sprintf("-- Generated: %s\n\n", time.Now().Format(time.RFC3339)))

	// Get all tables
	tables := []string{
		"users", "customers", "entries", "entry_events", "room_entries",
		"system_settings", "rent_payments", "invoices", "gate_passes",
		"gate_pass_pickups", "guard_entries", "token_colors", "season_requests",
	}

	for _, table := range tables {
		// Get table schema
		rows, err := db.QueryContext(ctx, fmt.Sprintf(`
			SELECT column_name, data_type, is_nullable, column_default
			FROM information_schema.columns
			WHERE table_name = '%s'
			ORDER BY ordinal_position`, table))
		if err != nil {
			continue
		}

		buffer.WriteString(fmt.Sprintf("\n-- Table: %s\n", table))

		// Get data
		dataRows, err := db.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s", table))
		if err != nil {
			log.Printf("[R2 Backup] Warning: failed to query %s: %v", table, err)
			rows.Close()
			continue
		}

		cols, _ := dataRows.Columns()
		if len(cols) > 0 {
			values := make([]interface{}, len(cols))
			valuePtrs := make([]interface{}, len(cols))
			for i := range values {
				valuePtrs[i] = &values[i]
			}

			for dataRows.Next() {
				dataRows.Scan(valuePtrs...)
				buffer.WriteString(fmt.Sprintf("INSERT INTO %s (%s) VALUES (", table, strings.Join(cols, ", ")))
				for i, v := range values {
					if i > 0 {
						buffer.WriteString(", ")
					}
					if v == nil {
						buffer.WriteString("NULL")
					} else {
						switch val := v.(type) {
						case []byte:
							buffer.WriteString(fmt.Sprintf("'%s'", strings.ReplaceAll(string(val), "'", "''")))
						case string:
							buffer.WriteString(fmt.Sprintf("'%s'", strings.ReplaceAll(val, "'", "''")))
						case time.Time:
							buffer.WriteString(fmt.Sprintf("'%s'", val.Format("2006-01-02 15:04:05")))
						default:
							buffer.WriteString(fmt.Sprintf("%v", val))
						}
					}
				}
				buffer.WriteString(");\n")
			}
		}

		rows.Close()
		dataRows.Close()
	}

	return buffer.Bytes(), nil
}

// formatBytes formats bytes to human readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
