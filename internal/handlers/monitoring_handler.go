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
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"cold-backend/internal/config"
	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
	"cold-backend/internal/timeutil"

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

	// Generate backup filename with structured hourly folders
	// Format: {env}/base/YYYY/MM/DD/HH/cold_db_YYYYMMDD_HHMMSS.sql
	now := timeutil.Now()
	backupKey := fmt.Sprintf("%s/base/%s/%s/%s/%s/cold_db_%s.sql",
		config.GetR2BackupPrefix(),
		now.Format("2006"),           // Year
		now.Format("01"),             // Month
		now.Format("02"),             // Day
		now.Format("15"),             // Hour (24h)
		now.Format("20060102_150405")) // Full timestamp

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

	// Also backup JWT secret for disaster recovery
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret != "" {
		_, err = client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(config.R2BucketName),
			Key:         aws.String("config/jwt_secret.txt"),
			Body:        bytes.NewReader([]byte(jwtSecret)),
			ContentType: aws.String("text/plain"),
		})
		if err != nil {
			log.Printf("[R2 Backup] Warning: Failed to backup JWT secret: %v", err)
		} else {
			log.Printf("[R2 Backup] JWT secret backed up for disaster recovery")
		}
	}

	// Cleanup old backups (older than 1 day, keep 1 per hour)
	cleanupOldBackups(ctx, client)
}

// cleanupOldBackups: 3-tier retention policy for ~8GB limit
// - < 1 day: keep ALL backups (~1440, ~4.3GB)
// - 1-30 days: keep 1 per hour (~696, ~2.1GB)
// - > 30 days: keep 1 per day (unlimited, ~3MB/day)
func cleanupOldBackups(ctx context.Context, client *s3.Client) {
	now := time.Now().UTC()
	oneDayAgo := now.Add(-24 * time.Hour)
	thirtyDaysAgo := now.Add(-30 * 24 * time.Hour)
	minValidSize := int64(1024) // 1KB minimum for valid backup

	deletedFailed := 0
	deletedHourlyDuplicates := 0
	deletedDailyDuplicates := 0
	var continuationToken *string

	// Track latest backup per hour (for 1-30 day old backups)
	hourlyBackups := make(map[string]struct {
		key          string
		lastModified time.Time
	})

	// Track latest backup per day (for >30 day old backups)
	dailyBackups := make(map[string]struct {
		key          string
		lastModified time.Time
	})

	// First pass: collect all backups
	var allObjects []struct {
		key          string
		lastModified time.Time
		size         int64
	}

	for {
		result, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(config.R2BucketName),
			Prefix:            aws.String(config.GetR2BackupPrefix() + "/base/"),
			ContinuationToken: continuationToken,
		})
		if err != nil {
			log.Printf("[R2 Cleanup] Failed to list backups: %v", err)
			return
		}

		for _, obj := range result.Contents {
			if obj.Key == nil || obj.LastModified == nil {
				continue
			}
			size := int64(0)
			if obj.Size != nil {
				size = *obj.Size
			}
			allObjects = append(allObjects, struct {
				key          string
				lastModified time.Time
				size         int64
			}{*obj.Key, *obj.LastModified, size})
		}

		if result.IsTruncated == nil || !*result.IsTruncated {
			break
		}
		continuationToken = result.NextContinuationToken
	}

	// Build maps of latest backups per hour/day
	for _, obj := range allObjects {
		parts := strings.Split(obj.key, "/")
		if len(parts) < 5 {
			continue
		}

		// Skip recent backups (< 1 day) - keep all
		if obj.lastModified.After(oneDayAgo) {
			continue
		}

		if obj.lastModified.After(thirtyDaysAgo) {
			// 1-30 days old: track per hour
			hourFolder := strings.Join(parts[:5], "/") // base/YYYY/MM/DD/HH
			if existing, exists := hourlyBackups[hourFolder]; !exists || obj.lastModified.After(existing.lastModified) {
				hourlyBackups[hourFolder] = struct {
					key          string
					lastModified time.Time
				}{obj.key, obj.lastModified}
			}
		} else {
			// >30 days old: track per day
			dayFolder := strings.Join(parts[:4], "/") // base/YYYY/MM/DD
			if existing, exists := dailyBackups[dayFolder]; !exists || obj.lastModified.After(existing.lastModified) {
				dailyBackups[dayFolder] = struct {
					key          string
					lastModified time.Time
				}{obj.key, obj.lastModified}
			}
		}
	}

	// Second pass: delete failed and duplicate backups
	for _, obj := range allObjects {
		// Delete failed/empty backups (< 1KB) - any age
		if obj.size < minValidSize {
			_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(config.R2BucketName),
				Key:    aws.String(obj.key),
			})
			if err == nil {
				deletedFailed++
			}
			continue
		}

		// Skip recent backups (< 1 day) - keep all
		if obj.lastModified.After(oneDayAgo) {
			continue
		}

		parts := strings.Split(obj.key, "/")
		if len(parts) < 5 {
			continue
		}

		if obj.lastModified.After(thirtyDaysAgo) {
			// 1-30 days old: keep only 1 per hour
			hourFolder := strings.Join(parts[:5], "/")
			if latest, exists := hourlyBackups[hourFolder]; exists && obj.key != latest.key {
				_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
					Bucket: aws.String(config.R2BucketName),
					Key:    aws.String(obj.key),
				})
				if err == nil {
					deletedHourlyDuplicates++
				}
			}
		} else {
			// >30 days old: keep only 1 per day
			dayFolder := strings.Join(parts[:4], "/")
			if latest, exists := dailyBackups[dayFolder]; exists && obj.key != latest.key {
				_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
					Bucket: aws.String(config.R2BucketName),
					Key:    aws.String(obj.key),
				})
				if err == nil {
					deletedDailyDuplicates++
				}
			}
		}
	}

	if deletedFailed > 0 || deletedHourlyDuplicates > 0 || deletedDailyDuplicates > 0 {
		log.Printf("[R2 Cleanup] Deleted %d failed, %d hourly-dups (1-30d), %d daily-dups (>30d)", deletedFailed, deletedHourlyDuplicates, deletedDailyDuplicates)
	}
}

// createR2DatabaseBackup creates a SQL backup using the shared database pool
func createR2DatabaseBackup(ctx context.Context) ([]byte, error) {
	// Use the shared database pool (same connection as main app)
	if r2BackupDBPool == nil {
		return nil, fmt.Errorf("database pool not initialized")
	}

	var buffer bytes.Buffer
	buffer.WriteString("-- Cold Storage Database Backup (Full Database)\n")
	buffer.WriteString(fmt.Sprintf("-- Generated: %s\n\n", timeutil.Now().Format(time.RFC3339)))
	// Disable foreign key checks during restore (tables may be in any order)
	buffer.WriteString("SET session_replication_role = 'replica';\n\n")

	// Get ALL tables from database dynamically
	tableRows, err := r2BackupDBPool.Query(ctx, `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public'
		AND table_type = 'BASE TABLE'
		AND table_name != 'schema_migrations'
		ORDER BY table_name`)
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %v", err)
	}
	defer tableRows.Close()

	var tables []string
	for tableRows.Next() {
		var tableName string
		if err := tableRows.Scan(&tableName); err == nil {
			tables = append(tables, tableName)
		}
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
						case bool:
							buffer.WriteString(fmt.Sprintf("%t", val))
						case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
							buffer.WriteString(fmt.Sprintf("%v", val))
						case map[string]interface{}, []interface{}:
							// JSON fields - marshal and quote
							jsonBytes, _ := json.Marshal(val)
							buffer.WriteString(fmt.Sprintf("'%s'", strings.ReplaceAll(string(jsonBytes), "'", "''")))
						default:
							// Check for numeric types (pgtype.Numeric shows as struct)
							str := fmt.Sprintf("%v", val)
							// If it looks like pgtype internal representation, extract the value
							if strings.HasPrefix(str, "{") && strings.Contains(str, " ") {
								// pgtype.Numeric: {value exp negative finite nan}
								// Try to use the type's String() method if available
								if stringer, ok := val.(fmt.Stringer); ok {
									str = stringer.String()
								}
							}
							// If still looks like struct, try to convert to number
							if strings.HasPrefix(str, "{") {
								// Skip this value or use NULL
								buffer.WriteString("NULL")
							} else {
								buffer.WriteString(fmt.Sprintf("'%s'", strings.ReplaceAll(str, "'", "''")))
							}
						}
					}
				}
				buffer.WriteString(");\n")
			}
		}

		rows.Close()
		dataRows.Close()
	}

	// Re-enable foreign key checks
	buffer.WriteString("\n-- Re-enable foreign key checks\n")
	buffer.WriteString("SET session_replication_role = 'origin';\n")

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

// metricsUnavailable returns a JSON error response when TimescaleDB metrics are not available
func (h *MonitoringHandler) metricsUnavailable(w http.ResponseWriter) bool {
	if h.repo == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "TimescaleDB metrics not available",
			"message": "Time-series metrics require TimescaleDB. Core features (R2 backups, PostgreSQL status) are still available.",
		})
		return true
	}
	return false
}

// ======================================
// Dashboard Overview
// ======================================

// GetDashboardData returns all data for the monitoring dashboard
func (h *MonitoringHandler) GetDashboardData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// For POC or Mac Mini HA environment without TimescaleDB, provide basic monitoring data
	// In these environments, repo exists but uses main DB (not TimescaleDB), so check environment
	if config.IsPOCEnvironment() || config.IsMacMiniHAEnvironment() {
		envName := "POC"
		if config.IsMacMiniHAEnvironment() {
			envName = "Mac Mini HA"
		}
		log.Printf("[Dashboard] %s environment detected, using direct metrics", envName)
		response := map[string]interface{}{
			"cluster_overview":  h.getPOCClusterOverview(),
			"postgres_overview": h.getPOCPostgresOverview(),
			"api_analytics":     h.getPOCAPIAnalytics(),
			"alert_summary":     h.getPOCAlertSummary(),
			"recent_alerts":     []interface{}{},
			"nodes":             h.getPOCNodeMetrics(),
			"postgres_pods":     []interface{}{},
			"environment":       config.GetEnvironmentName(),
			"is_poc":            config.IsPOCEnvironment(),
			"is_mac_mini":       config.IsMacMiniHAEnvironment(),
			"last_updated":      timeutil.Now(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

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
		"environment":       config.GetEnvironmentName(),
		"is_poc":            config.IsPOCEnvironment(),
		"last_updated":      timeutil.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetEnvironmentInfo returns deployment environment information
func (h *MonitoringHandler) GetEnvironmentInfo(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"environment":  config.GetEnvironmentName(),
		"is_poc":       config.IsPOCEnvironment(),
		"is_mac_mini":  config.IsMacMiniHAEnvironment(),
		"r2_prefix":    config.GetR2BackupPrefix(),
	}

	// Add environment-specific info
	if config.IsMacMiniHAEnvironment() {
		response["primary_server"] = "192.168.15.240"
		response["secondary_server"] = "192.168.15.241"
		response["primary_hostname"] = "coldstore-primary"
		response["secondary_hostname"] = "coldstore-archive"
		response["deployment_type"] = "Mac Mini HA"
		response["primary_os"] = "macOS"
		response["secondary_os"] = "Linux"
	} else if config.IsPOCEnvironment() {
		response["primary_vm"] = "230"
		response["standby_vm"] = "231"
		response["deployment_type"] = "VM-based HA (POC)"
	} else if config.GetEnvironmentName() == "Production (K3s)" {
		response["deployment_type"] = "K3s Cluster"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ======================================
// API Analytics Endpoints
// ======================================

// GetAPIAnalytics returns API usage statistics
func (h *MonitoringHandler) GetAPIAnalytics(w http.ResponseWriter, r *http.Request) {
	// For POC environment without TimescaleDB, still serve API analytics from main database
	if h.repo == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "API analytics not available",
			"message": "Metrics repository not initialized. Please enable API logging in the configuration.",
		})
		return
	}

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
	// For POC environment without TimescaleDB, still serve from main database
	if h.repo == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "API analytics not available",
		})
		return
	}
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
	// For POC environment without TimescaleDB, still serve from main database
	if h.repo == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "API analytics not available",
		})
		return
	}
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
	// For POC environment without TimescaleDB, still serve from main database
	if h.repo == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "API analytics not available",
		})
		return
	}
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

	// For POC or Mac Mini HA environment without TimescaleDB, return real metrics
	if config.IsPOCEnvironment() || config.IsMacMiniHAEnvironment() {
		nodes := h.getPOCNodeMetrics()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"nodes":        nodes,
			"last_updated": timeutil.Now(),
		})
		return
	}

	// For production with TimescaleDB
	nodes, err := h.repo.GetLatestNodeMetrics(ctx)
	if err != nil {
		http.Error(w, "Failed to get node metrics", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes":        nodes,
		"last_updated": timeutil.Now(),
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

	// For POC or Mac Mini HA environment without TimescaleDB, query Prometheus directly
	if config.IsPOCEnvironment() || config.IsMacMiniHAEnvironment() {
		metrics := h.getPrometheusNodeHistory(ctx, nodeName, duration)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"node_name": nodeName,
			"metrics":   metrics,
			"range":     duration.String(),
		})
		return
	}

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

// GetAllNodesMetricsHistory returns historical metrics for all nodes
func (h *MonitoringHandler) GetAllNodesMetricsHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rangeParam := r.URL.Query().Get("range")
	duration := parseDuration(rangeParam, 1*time.Hour)

	// Get node names based on environment
	var nodes []string
	if config.IsMacMiniHAEnvironment() {
		nodes = []string{"coldstore-primary", "coldstore-archive"}
	} else {
		nodes = []string{"coldstore-prod1", "coldstore-prod2"}
	}

	// For POC/Mac Mini HA environment without TimescaleDB, query Prometheus for all nodes
	if config.IsPOCEnvironment() || config.IsMacMiniHAEnvironment() {
		result := make(map[string]interface{})

		for _, nodeName := range nodes {
			metrics := h.getPrometheusNodeHistory(ctx, nodeName, duration)
			result[nodeName] = metrics
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"nodes": result,
			"range": duration.String(),
		})
		return
	}

	// For production with TimescaleDB, query all nodes
	result := make(map[string]interface{})

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

	for _, nodeName := range nodes {
		metrics, err := h.repo.GetNodeMetricsHistory(ctx, nodeName, duration, interval)
		if err != nil {
			log.Printf("Failed to get metrics for node %s: %v", nodeName, err)
			continue
		}
		result[nodeName] = metrics
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes":    result,
		"range":    duration.String(),
		"interval": interval,
	})
}

// GetClusterOverview returns aggregated cluster statistics
func (h *MonitoringHandler) GetClusterOverview(w http.ResponseWriter, r *http.Request) {
	if h.metricsUnavailable(w) {
		return
	}
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
	if h.metricsUnavailable(w) {
		return
	}
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
		"last_updated": timeutil.Now(),
	})
}

// getMetricsDBMetrics queries the backup database on 192.168.15.195
func (h *MonitoringHandler) getMetricsDBMetrics() *models.PostgresMetrics {
	host := "192.168.15.195"
	port := "5432" // Backup database server

	// Use proper credentials
	connStr := fmt.Sprintf("host=%s port=%s user=cold_user password=SecurePostgresPassword123 dbname=cold_db sslmode=disable connect_timeout=5", host, port)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil
	}
	defer db.Close()

	metrics := &models.PostgresMetrics{
		Time:           timeutil.Now(),
		PodName:        "backup-server (192.168.15.195)",
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
	err = db.QueryRowContext(ctx, "SELECT pg_database_size('cold_db')").Scan(&sizeBytes)
	if err != nil {
		metrics.Status = "Error"
		return metrics
	}
	if sizeBytes.Valid {
		metrics.DatabaseSizeBytes = sizeBytes.Int64
	}

	// Get active connections
	var activeConn sql.NullInt64
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM pg_stat_activity WHERE datname = 'cold_db' AND state = 'active'").Scan(&activeConn)
	if err == nil && activeConn.Valid {
		metrics.ActiveConnections = int(activeConn.Int64)
	}

	// Get total connections
	var totalConn sql.NullInt64
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM pg_stat_activity WHERE datname = 'cold_db'").Scan(&totalConn)
	if err == nil && totalConn.Valid {
		metrics.TotalConnections = int(totalConn.Int64)
	}

	// Get cache hit ratio
	var cacheRatio sql.NullFloat64
	err = db.QueryRowContext(ctx, `
		SELECT COALESCE(
			100.0 * sum(blks_hit) / NULLIF(sum(blks_hit) + sum(blks_read), 0),
			100.0
		) FROM pg_stat_database WHERE datname = 'cold_db'
	`).Scan(&cacheRatio)
	if err == nil && cacheRatio.Valid {
		metrics.CacheHitRatio = cacheRatio.Float64
	}

	return metrics
}

// GetPostgresOverview returns aggregated PostgreSQL cluster statistics
func (h *MonitoringHandler) GetPostgresOverview(w http.ResponseWriter, r *http.Request) {
	if h.metricsUnavailable(w) {
		return
	}
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
	if h.metricsUnavailable(w) {
		return
	}
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
	if h.metricsUnavailable(w) {
		return
	}
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
	if h.metricsUnavailable(w) {
		return
	}
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
	if h.metricsUnavailable(w) {
		return
	}
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
	if h.metricsUnavailable(w) {
		return
	}
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
	if h.metricsUnavailable(w) {
		return
	}
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
	if h.metricsUnavailable(w) {
		return
	}
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
	if h.metricsUnavailable(w) {
		return
	}
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
				"last_modified": timeutil.ToIST(*obj.LastModified).Format("2006-01-02 15:04:05"),
			})
		}
	}

	result["total_backups"] = totalCount
	result["total_size"] = formatBytes(totalSize)
	result["total_size_bytes"] = totalSize
	result["backups"] = backups

	if !latestTime.IsZero() {
		result["last_backup"] = timeutil.ToIST(latestTime).Format("2006-01-02 15:04:05")
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

	// Generate backup filename with structured hourly folders
	// Format: {env}/base/YYYY/MM/DD/HH/cold_db_YYYYMMDD_HHMMSS.sql
	now := timeutil.Now()
	backupKey := fmt.Sprintf("%s/base/%s/%s/%s/%s/cold_db_%s.sql",
		config.GetR2BackupPrefix(),
		now.Format("2006"),           // Year
		now.Format("01"),             // Month
		now.Format("02"),             // Day
		now.Format("15"),             // Hour (24h)
		now.Format("20060102_150405")) // Full timestamp

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
	buffer.WriteString("-- Cold Storage Database Backup (Full Database)\n")
	buffer.WriteString(fmt.Sprintf("-- Generated: %s\n\n", timeutil.Now().Format(time.RFC3339)))

	// Get ALL tables from database dynamically
	tableRows, err := db.QueryContext(ctx, `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public'
		AND table_type = 'BASE TABLE'
		AND table_name != 'schema_migrations'
		ORDER BY table_name`)
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %v", err)
	}
	defer tableRows.Close()

	var tables []string
	for tableRows.Next() {
		var tableName string
		if err := tableRows.Scan(&tableName); err == nil {
			tables = append(tables, tableName)
		}
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

// ======================================
// POC Environment Helper Functions
// ======================================

// getPOCNodeMetrics returns real node metrics from Prometheus
// Supports both POC (VMs 230/231) and Mac Mini HA (240/241) environments
func (h *MonitoringHandler) getPOCNodeMetrics() []map[string]interface{} {
	nodes := []map[string]interface{}{}

	// Determine environment and nodes
	var nodeConfigs []struct {
		ip       string
		hostname string
		role     string
	}

	if config.IsMacMiniHAEnvironment() {
		// Mac Mini HA environment
		nodeConfigs = []struct {
			ip       string
			hostname string
			role     string
		}{
			{"192.168.15.240", "coldstore-primary", "primary"},
			{"192.168.15.241", "coldstore-archive", "secondary"},
		}
		log.Printf("[Monitoring] Mac Mini HA environment - checking nodes 240, 241")
	} else {
		// POC environment (VMs 230/231)
		nodeConfigs = []struct {
			ip       string
			hostname string
			role     string
		}{
			{"192.168.15.230", "coldstore-prod1", "master"},
			{"192.168.15.231", "coldstore-prod2", "agent"},
		}
		log.Printf("[Monitoring] POC environment - checking nodes 230, 231")

		// Get K3s nodes status (only for POC with K3s)
		log.Printf("[Monitoring] Getting K3s nodes status...")
		k3sNodes := h.getK3sNodes()
		log.Printf("[Monitoring] Found %d K3s nodes", len(k3sNodes))

		for _, cfg := range nodeConfigs {
			log.Printf("[Monitoring] Querying Prometheus for %s...", cfg.hostname)
			if metrics := h.getPrometheusNodeMetrics(cfg.ip, cfg.hostname, cfg.role); metrics != nil {
				// Merge with K3s status
				for _, k3sNode := range k3sNodes {
					if nodeName, ok := k3sNode["name"].(string); ok && nodeName == cfg.hostname {
						metrics["k3s_status"] = k3sNode["status"]
						metrics["k3s_version"] = k3sNode["version"]
						metrics["k3s_role"] = k3sNode["role"]
						log.Printf("[Monitoring] Merged K3s status for %s: %s", nodeName, k3sNode["status"])
						break
					}
				}
				nodes = append(nodes, metrics)
				log.Printf("[Monitoring] Added %s metrics", cfg.hostname)
			} else {
				log.Printf("[Monitoring] Failed to get metrics for %s", cfg.hostname)
			}
		}
		return nodes
	}

	// For Mac Mini HA, query each node directly
	for _, cfg := range nodeConfigs {
		log.Printf("[Monitoring] Querying metrics for %s (%s)...", cfg.hostname, cfg.ip)
		if metrics := h.getNodeMetricsDirect(cfg.ip, cfg.hostname, cfg.role); metrics != nil {
			nodes = append(nodes, metrics)
			log.Printf("[Monitoring] Added %s metrics", cfg.hostname)
		} else {
			log.Printf("[Monitoring] Failed to get metrics for %s", cfg.hostname)
		}
	}

	log.Printf("[Monitoring] Returning %d node metrics", len(nodes))
	return nodes
}

// getNodeMetricsDirect queries node metrics directly via Node Exporter
func (h *MonitoringHandler) getNodeMetricsDirect(ip, hostname, role string) map[string]interface{} {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	metrics := map[string]interface{}{
		"node_name":              hostname,
		"node_ip":                ip,
		"node_role":              role,
		"node_status":            "Unknown",
		"cpu_percent":            0.0,
		"memory_percent":         0.0,
		"disk_percent":           0.0,
		"disk_used_bytes":        int64(0),
		"disk_total_bytes":       int64(0),
		"memory_used_bytes":      int64(0),
		"memory_total_bytes":     int64(0),
		"load_average_1m":        0.0,
		"network_rx_bytes_per_sec": 0.0,
		"network_tx_bytes_per_sec": 0.0,
	}

	// Try to query Node Exporter on port 9100
	client := &http.Client{Timeout: 5 * time.Second}
	nodeExporterURL := fmt.Sprintf("http://%s:9100/metrics", ip)

	req, err := http.NewRequestWithContext(ctx, "GET", nodeExporterURL, nil)
	if err != nil {
		log.Printf("[NodeMetrics] Failed to create request for %s: %v", ip, err)
		return metrics
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[NodeMetrics] Failed to reach Node Exporter on %s: %v", ip, err)
		return metrics
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		metrics["node_status"] = "Ready"
		// Parse Prometheus text format metrics
		body, _ := io.ReadAll(resp.Body)
		metricsText := string(body)

		// Extract key metrics from Prometheus text format
		// Note: CPU requires rate calculation, use load as proxy for now
		metrics["cpu_percent"] = h.parsePrometheusGauge(metricsText, "node_load1") * 10 // Rough estimate
		metrics["memory_percent"] = h.parseMemoryPercent(metricsText)
		metrics["disk_percent"] = h.parseDiskPercent(metricsText)
		metrics["load_average_1m"] = h.parsePrometheusGauge(metricsText, "node_load1")
	}

	return metrics
}

// parsePrometheusMetric parses a simple gauge value from Prometheus text format
func (h *MonitoringHandler) parsePrometheusGauge(metricsText, metricName string) float64 {
	lines := strings.Split(metricsText, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, metricName+" ") || strings.HasPrefix(line, metricName+"{") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				value, err := strconv.ParseFloat(parts[len(parts)-1], 64)
				if err == nil {
					return value
				}
			}
		}
	}
	return 0
}

// parseMemoryPercent calculates memory percentage from Node Exporter metrics
func (h *MonitoringHandler) parseMemoryPercent(metricsText string) float64 {
	var memTotal, memAvail float64
	lines := strings.Split(metricsText, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "node_memory_MemTotal_bytes ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				memTotal, _ = strconv.ParseFloat(parts[1], 64)
			}
		}
		if strings.HasPrefix(line, "node_memory_MemAvailable_bytes ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				memAvail, _ = strconv.ParseFloat(parts[1], 64)
			}
		}
	}
	if memTotal > 0 {
		return 100.0 * (1.0 - memAvail/memTotal)
	}
	return 0
}

// parseDiskPercent calculates disk percentage from Node Exporter metrics
func (h *MonitoringHandler) parseDiskPercent(metricsText string) float64 {
	var diskTotal, diskAvail float64
	lines := strings.Split(metricsText, "\n")
	for _, line := range lines {
		// Look for root filesystem
		if strings.Contains(line, "node_filesystem_size_bytes") && strings.Contains(line, `mountpoint="/"`) {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				diskTotal, _ = strconv.ParseFloat(parts[len(parts)-1], 64)
			}
		}
		if strings.Contains(line, "node_filesystem_avail_bytes") && strings.Contains(line, `mountpoint="/"`) {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				diskAvail, _ = strconv.ParseFloat(parts[len(parts)-1], 64)
			}
		}
	}
	if diskTotal > 0 {
		return 100.0 * (1.0 - diskAvail/diskTotal)
	}
	return 0
}

// getK3sNodes queries K3s API for node information
func (h *MonitoringHandler) getK3sNodes() []map[string]interface{} {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	nodes := []map[string]interface{}{}

	// SSH to master and get kubectl nodes
	// Using SSH via Proxmox to access VMs
	client := &http.Client{Timeout: 5 * time.Second}

	// For POC, we'll parse kubectl output
	// In production, we'd use the K8s client-go library
	cmd := "k3s kubectl get nodes -o json"

	// Execute via local SSH (since the app is running on the VM)
	result, err := h.executeLocalCommand(ctx, cmd)
	if err != nil {
		log.Printf("[K3s] Failed to get nodes: %v", err)
		log.Printf("[K3s] Command output: %s", result)
		return nodes
	}

	// Parse JSON output
	var nodesData struct {
		Items []struct {
			Metadata struct {
				Name   string            `json:"name"`
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
			Status struct {
				Conditions []struct {
					Type   string `json:"type"`
					Status string `json:"status"`
				} `json:"conditions"`
				NodeInfo struct {
					KubeletVersion string `json:"kubeletVersion"`
				} `json:"nodeInfo"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal([]byte(result), &nodesData); err != nil {
		log.Printf("[K3s] Failed to parse nodes JSON: %v", err)
		log.Printf("[K3s] Raw JSON: %s", result)
		return nodes
	}

	log.Printf("[K3s] Found %d nodes in cluster", len(nodesData.Items))

	for _, node := range nodesData.Items {
		status := "NotReady"
		for _, condition := range node.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				status = "Ready"
				break
			}
		}

		role := "worker"
		if _, ok := node.Metadata.Labels["node-role.kubernetes.io/master"]; ok {
			role = "master"
		} else if _, ok := node.Metadata.Labels["node-role.kubernetes.io/control-plane"]; ok {
			role = "master"
		}

		nodeInfo := map[string]interface{}{
			"name":    node.Metadata.Name,
			"status":  status,
			"version": node.Status.NodeInfo.KubeletVersion,
			"role":    role,
		}
		nodes = append(nodes, nodeInfo)
		log.Printf("[K3s] Node: %s, Status: %s, Role: %s", node.Metadata.Name, status, role)
	}

	_ = client // Prevent unused warning

	return nodes
}

// getPrometheusNodeMetrics queries Prometheus for real system metrics
func (h *MonitoringHandler) getPrometheusNodeMetrics(ip, instance, role string) map[string]interface{} {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	metrics := map[string]interface{}{
		"node_name":              instance,
		"node_ip":                ip,
		"node_role":              role,
		"node_status":            "Unknown",
		"cpu_percent":            0.0,
		"memory_percent":         0.0,
		"disk_percent":           0.0,
		"disk_used_bytes":        int64(0),
		"disk_total_bytes":       int64(0),
		"memory_used_bytes":      int64(0),
		"memory_total_bytes":     int64(0),
		"load_average_1m":        0.0,
		"pod_count":              0,
		"network_rx_bytes_per_sec": 0.0,
		"network_tx_bytes_per_sec": 0.0,
	}

	// Query Prometheus for metrics - use environment-specific URL
	promURL := h.getPrometheusURL()

	// CPU usage query (using instance name from Prometheus config)
	cpuQuery := fmt.Sprintf(`100 - (avg by (instance) (rate(node_cpu_seconds_total{mode="idle",instance="%s"}[5m])) * 100)`, instance)
	log.Printf("[Prometheus] Querying CPU for %s", instance)
	if cpuValue := h.queryPrometheus(ctx, promURL, cpuQuery); cpuValue >= 0 {
		metrics["cpu_percent"] = cpuValue
		metrics["node_status"] = "Ready"
		log.Printf("[Prometheus] CPU for %s: %.2f%%", instance, cpuValue)
	} else {
		log.Printf("[Prometheus] Failed to get CPU for %s", instance)
	}

	// Memory usage query (using instance name)
	memQuery := fmt.Sprintf(`100 * (1 - (node_memory_MemAvailable_bytes{instance="%s"} / node_memory_MemTotal_bytes{instance="%s"}))`, instance, instance)
	if memValue := h.queryPrometheus(ctx, promURL, memQuery); memValue >= 0 {
		metrics["memory_percent"] = memValue
	}

	// Disk usage query (using instance name)
	diskQuery := fmt.Sprintf(`100 - ((node_filesystem_avail_bytes{instance="%s",mountpoint="/",fstype!="rootfs"} * 100) / node_filesystem_size_bytes{instance="%s",mountpoint="/",fstype!="rootfs"})`, instance, instance)
	if diskValue := h.queryPrometheus(ctx, promURL, diskQuery); diskValue >= 0 {
		metrics["disk_percent"] = diskValue
	}

	// Total memory (using instance name)
	memTotalQuery := fmt.Sprintf(`node_memory_MemTotal_bytes{instance="%s"}`, instance)
	if memTotal := h.queryPrometheus(ctx, promURL, memTotalQuery); memTotal > 0 {
		metrics["memory_total_bytes"] = int64(memTotal)
		metrics["memory_used_bytes"] = int64(memTotal * metrics["memory_percent"].(float64) / 100.0)
	}

	// Total disk (using instance name)
	diskTotalQuery := fmt.Sprintf(`node_filesystem_size_bytes{instance="%s",mountpoint="/",fstype!="rootfs"}`, instance)
	if diskTotal := h.queryPrometheus(ctx, promURL, diskTotalQuery); diskTotal > 0 {
		metrics["disk_total_bytes"] = int64(diskTotal)
		metrics["disk_used_bytes"] = int64(diskTotal * metrics["disk_percent"].(float64) / 100.0)
	}

	// Load average (using instance name)
	loadQuery := fmt.Sprintf(`node_load1{instance="%s"}`, instance)
	if load := h.queryPrometheus(ctx, promURL, loadQuery); load >= 0 {
		metrics["load_average_1m"] = load
	}

	// Network I/O (using instance name)
	netRxQuery := fmt.Sprintf(`rate(node_network_receive_bytes_total{instance="%s",device!~"lo|veth.*"}[5m])`, instance)
	if netRx := h.queryPrometheus(ctx, promURL, netRxQuery); netRx >= 0 {
		metrics["network_rx_bytes_per_sec"] = netRx
	}

	netTxQuery := fmt.Sprintf(`rate(node_network_transmit_bytes_total{instance="%s",device!~"lo|veth.*"}[5m])`, instance)
	if netTx := h.queryPrometheus(ctx, promURL, netTxQuery); netTx >= 0 {
		metrics["network_tx_bytes_per_sec"] = netTx
	}

	// Get pod count from K3s (if available)
	podCountQuery := fmt.Sprintf(`count(kube_pod_info{node=~".*%s.*"})`, instance)
	if podCount := h.queryPrometheus(ctx, promURL, podCountQuery); podCount >= 0 {
		metrics["pod_count"] = int(podCount)
	}

	return metrics
}

// queryPrometheus executes a PromQL query and returns the result value
func (h *MonitoringHandler) queryPrometheus(ctx context.Context, promURL, query string) float64 {
	client := &http.Client{Timeout: 5 * time.Second}

	queryURL := fmt.Sprintf("%s/api/v1/query?query=%s", promURL, url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		log.Printf("[Prometheus] Failed to create request: %v", err)
		return -1
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[Prometheus] HTTP request failed: %v", err)
		return -1
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[Prometheus] HTTP status %d for query: %s", resp.StatusCode, query)
		return -1
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[Prometheus] Failed to read response: %v", err)
		return -1
	}

	var result struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Value []interface{} `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("[Prometheus] Failed to parse JSON: %v", err)
		return -1
	}

	if result.Status != "success" {
		log.Printf("[Prometheus] Query status: %s", result.Status)
		return -1
	}

	if len(result.Data.Result) == 0 {
		log.Printf("[Prometheus] No results for query: %s", query)
		return -1
	}

	if len(result.Data.Result[0].Value) < 2 {
		log.Printf("[Prometheus] Invalid result format")
		return -1
	}

	valueStr, ok := result.Data.Result[0].Value[1].(string)
	if !ok {
		log.Printf("[Prometheus] Value is not a string")
		return -1
	}

	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		log.Printf("[Prometheus] Failed to parse float: %v", err)
		return -1
	}

	return value
}

// queryPrometheusRange executes a PromQL range query and returns time-series data
func (h *MonitoringHandler) queryPrometheusRange(ctx context.Context, promURL, query string, start, end time.Time, step string) []map[string]interface{} {
	client := &http.Client{Timeout: 10 * time.Second}

	queryURL := fmt.Sprintf("%s/api/v1/query_range?query=%s&start=%d&end=%d&step=%s",
		promURL,
		url.QueryEscape(query),
		start.Unix(),
		end.Unix(),
		step)

	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		log.Printf("[Prometheus] Failed to create request: %v", err)
		return nil
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[Prometheus] Failed to execute query: %v", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[Prometheus] Query failed with status %d", resp.StatusCode)
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[Prometheus] Failed to read response: %v", err)
		return nil
	}

	var result struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Metric map[string]string `json:"metric"`
				Values [][]interface{}   `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("[Prometheus] Failed to parse response: %v", err)
		return nil
	}

	if result.Status != "success" || len(result.Data.Result) == 0 {
		log.Printf("[Prometheus] Query returned no data")
		return nil
	}

	// Convert to time-series format
	points := []map[string]interface{}{}
	for _, value := range result.Data.Result[0].Values {
		if len(value) < 2 {
			continue
		}

		timestamp, ok := value[0].(float64)
		if !ok {
			continue
		}

		valueStr, ok := value[1].(string)
		if !ok {
			continue
		}

		floatValue, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			continue
		}

		points = append(points, map[string]interface{}{
			"time":  time.Unix(int64(timestamp), 0),
			"value": floatValue,
		})
	}

	return points
}

// getPrometheusURL returns the Prometheus server URL based on environment
func (h *MonitoringHandler) getPrometheusURL() string {
	if config.IsMacMiniHAEnvironment() {
		// In Mac Mini HA, Prometheus runs on secondary server (Linux)
		return "http://192.168.15.241:9090"
	}
	// POC environment - Prometheus on primary VM
	return "http://192.168.15.230:9090"
}

// getPrometheusNodeHistory fetches historical metrics from Prometheus
func (h *MonitoringHandler) getPrometheusNodeHistory(ctx context.Context, nodeName string, duration time.Duration) []map[string]interface{} {
	promURL := h.getPrometheusURL()
	end := timeutil.Now()
	start := end.Add(-duration)

	// Determine step based on duration
	step := "1m"
	if duration > 6*time.Hour {
		step = "5m"
	}
	if duration > 24*time.Hour {
		step = "15m"
	}
	if duration > 7*24*time.Hour {
		step = "1h"
	}

	// Validate node name based on environment
	validNames := map[string]bool{}
	if config.IsMacMiniHAEnvironment() {
		validNames["coldstore-primary"] = true
		validNames["coldstore-archive"] = true
	} else {
		validNames["coldstore-prod1"] = true
		validNames["coldstore-prod2"] = true
	}

	if !validNames[nodeName] {
		log.Printf("[Prometheus] Unknown node name for environment: %s", nodeName)
		return []map[string]interface{}{}
	}

	// Query CPU usage (using instance name, not IP)
	cpuQuery := fmt.Sprintf(`100 - (avg by (instance) (rate(node_cpu_seconds_total{mode="idle",instance="%s"}[5m])) * 100)`, nodeName)
	cpuData := h.queryPrometheusRange(ctx, promURL, cpuQuery, start, end, step)

	// Query Memory usage (using instance name)
	memQuery := fmt.Sprintf(`100 * (1 - (node_memory_MemAvailable_bytes{instance="%s"} / node_memory_MemTotal_bytes{instance="%s"}))`, nodeName, nodeName)
	memData := h.queryPrometheusRange(ctx, promURL, memQuery, start, end, step)

	// Query Disk usage (using instance name)
	diskQuery := fmt.Sprintf(`100 - ((node_filesystem_avail_bytes{instance="%s",mountpoint="/",fstype!="rootfs"} * 100) / node_filesystem_size_bytes{instance="%s",mountpoint="/",fstype!="rootfs"})`, nodeName, nodeName)
	diskData := h.queryPrometheusRange(ctx, promURL, diskQuery, start, end, step)

	// Merge all metrics into a single time series
	metrics := []map[string]interface{}{}

	// Use CPU data as the base timeline
	if cpuData != nil {
		for i, cpuPoint := range cpuData {
			metric := map[string]interface{}{
				"time":        cpuPoint["time"],
				"cpu_percent": cpuPoint["value"],
			}

			// Add memory data if available
			if memData != nil && i < len(memData) {
				metric["memory_percent"] = memData[i]["value"]
			}

			// Add disk data if available
			if diskData != nil && i < len(diskData) {
				metric["disk_percent"] = diskData[i]["value"]
			}

			metrics = append(metrics, metric)
		}
	}

	return metrics
}

// executeLocalCommand executes a command on the local system
func (h *MonitoringHandler) executeLocalCommand(ctx context.Context, command string) (string, error) {
	// Execute command using bash
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command failed: %w, output: %s", err, string(output))
	}
	return string(output), nil
}

// getVMMetrics queries a VM's PostgreSQL server for system metrics
func (h *MonitoringHandler) getVMMetrics(host, name string) map[string]interface{} {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	connStr := fmt.Sprintf("host=%s port=5432 user=cold_user password=SecurePostgresPassword123 dbname=cold_db sslmode=disable connect_timeout=3", host)
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return map[string]interface{}{
			"node_name":      name,
			"node_status":    "Unreachable",
			"cpu_percent":    0.0,
			"memory_percent": 0.0,
			"disk_percent":   0.0,
		}
	}
	defer db.Close()

	// Test connection
	if err := db.PingContext(ctx); err != nil {
		return map[string]interface{}{
			"node_name":      name,
			"node_status":    "Unreachable",
			"cpu_percent":    0.0,
			"memory_percent": 0.0,
			"disk_percent":   0.0,
		}
	}

	// Get database size as a proxy for disk usage
	var dbSize int64
	err = db.QueryRowContext(ctx, "SELECT pg_database_size('cold_db')").Scan(&dbSize)
	if err != nil {
		dbSize = 0
	}

	// Get connection count as a proxy for CPU usage
	var connCount int
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM pg_stat_activity WHERE datname = 'cold_db'").Scan(&connCount)
	if err != nil {
		connCount = 0
	}

	// Simulate metrics (in POC we don't have real system metrics)
	cpuPercent := float64(connCount) * 5.0 // Rough estimate
	if cpuPercent > 100 {
		cpuPercent = 100
	}

	memoryPercent := 30.0 + float64(connCount)*2.0 // Base usage + connections
	if memoryPercent > 100 {
		memoryPercent = 100
	}

	diskPercent := float64(dbSize) / (20 * 1024 * 1024 * 1024) * 100 // Assuming 20GB disk
	if diskPercent > 100 {
		diskPercent = 100
	}

	return map[string]interface{}{
		"node_name":         name,
		"node_status":       "Ready",
		"node_ip":           host,
		"node_role":         "worker",
		"cpu_percent":       cpuPercent,
		"memory_percent":    memoryPercent,
		"disk_percent":      diskPercent,
		"disk_used_bytes":   dbSize,
		"disk_total_bytes":  int64(20 * 1024 * 1024 * 1024),
		"load_average_1m":   cpuPercent / 100.0,
		"pod_count":         1,
	}
}

// getPOCClusterOverview returns cluster overview for POC environment
func (h *MonitoringHandler) getPOCClusterOverview() map[string]interface{} {
	log.Printf("[Cluster] Getting POC cluster overview...")
	nodes := h.getPOCNodeMetrics()

	totalNodes := len(nodes)
	healthyNodes := 0
	totalCPU := 0.0
	totalMemory := 0.0
	totalDisk := 0.0

	for _, node := range nodes {
		if status, ok := node["node_status"].(string); ok && status == "Ready" {
			healthyNodes++
			log.Printf("[Cluster] Node %v is Ready", node["node_name"])
		}
		if cpu, ok := node["cpu_percent"].(float64); ok {
			totalCPU += cpu
			log.Printf("[Cluster] Node %v CPU: %.2f%%", node["node_name"], cpu)
		}
		if mem, ok := node["memory_percent"].(float64); ok {
			totalMemory += mem
			log.Printf("[Cluster] Node %v Memory: %.2f%%", node["node_name"], mem)
		}
		if disk, ok := node["disk_percent"].(float64); ok {
			totalDisk += disk
			log.Printf("[Cluster] Node %v Disk: %.2f%%", node["node_name"], disk)
		}
	}

	avgCPU := 0.0
	avgMemory := 0.0
	avgDisk := 0.0
	if totalNodes > 0 {
		avgCPU = totalCPU / float64(totalNodes)
		avgMemory = totalMemory / float64(totalNodes)
		avgDisk = totalDisk / float64(totalNodes)
	}

	overview := map[string]interface{}{
		"total_nodes":        totalNodes,
		"healthy_nodes":      healthyNodes,
		"total_cpu_cores":    totalNodes * 4, // Assume 4 cores per VM
		"avg_cpu_percent":    avgCPU,
		"total_memory_gb":    float64(totalNodes * 8), // Assume 8GB per VM
		"used_memory_gb":     float64(totalNodes*8) * avgMemory / 100.0,
		"avg_memory_percent": avgMemory,
		"total_disk_gb":      float64(totalNodes * 20), // Assume 20GB per VM
		"used_disk_gb":       float64(totalNodes*20) * avgDisk / 100.0,
		"avg_disk_percent":   avgDisk,
	}

	log.Printf("[Cluster] Overview: %d/%d nodes healthy, CPU: %.1f%%, Memory: %.1f%%, Disk: %.1f%%",
		healthyNodes, totalNodes, avgCPU, avgMemory, avgDisk)

	return overview
}

// getPOCPostgresOverview returns PostgreSQL overview for POC/Mac Mini HA environment
func (h *MonitoringHandler) getPOCPostgresOverview() map[string]interface{} {
	totalPods := 2 // Primary + Standby/Archive
	healthyPods := 0

	// Determine server IPs based on environment
	var primaryIP, secondaryIP string
	if config.IsMacMiniHAEnvironment() {
		primaryIP = "192.168.15.240"
		secondaryIP = "192.168.15.241"
	} else {
		primaryIP = "192.168.15.230"
		secondaryIP = "192.168.15.231"
	}

	// Check both PostgreSQL instances
	if h.checkPostgresHealth(primaryIP) {
		healthyPods++
	}
	if h.checkPostgresHealth(secondaryIP) {
		healthyPods++
	}

	return map[string]interface{}{
		"total_pods":   totalPods,
		"healthy_pods": healthyPods,
		"primary_pods": 1,
		"replica_pods": 1,
	}
}

// checkPostgresHealth checks if a PostgreSQL instance is healthy
func (h *MonitoringHandler) checkPostgresHealth(host string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	connStr := fmt.Sprintf("host=%s port=5432 user=cold_user password=SecurePostgresPassword123 dbname=cold_db sslmode=disable connect_timeout=2", host)
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return false
	}
	defer db.Close()

	return db.PingContext(ctx) == nil
}

// getPOCAPIAnalytics returns real API analytics from main database
func (h *MonitoringHandler) getPOCAPIAnalytics() map[string]interface{} {
	// For POC environment, query the main database (api_request_logs table exists there)
	if h.repo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		analytics, err := h.repo.GetAPIAnalytics(ctx, 1*time.Hour)
		if err != nil {
			log.Printf("[POC API Analytics] Failed to get analytics: %v", err)
			return map[string]interface{}{
				"total_requests":   0,
				"success_requests": 0,
				"error_requests":   0,
				"error_rate":       0.0,
				"avg_duration_ms":  0.0,
				"p95_duration_ms":  0.0,
				"p99_duration_ms":  0.0,
			}
		}

		return map[string]interface{}{
			"total_requests":   analytics.TotalRequests,
			"success_requests": analytics.SuccessRequests,
			"error_requests":   analytics.ErrorRequests,
			"error_rate":       analytics.ErrorRate,
			"avg_duration_ms":  analytics.AvgDurationMs,
			"p95_duration_ms":  analytics.P95DurationMs,
			"p99_duration_ms":  analytics.MaxDurationMs,
		}
	}

	// Fallback if repo is not initialized
	return map[string]interface{}{
		"total_requests":   0,
		"success_requests": 0,
		"error_requests":   0,
		"error_rate":       0.0,
		"avg_duration_ms":  0.0,
		"p95_duration_ms":  0.0,
		"p99_duration_ms":  0.0,
	}
}

// getPOCAlertSummary returns simulated alert summary
func (h *MonitoringHandler) getPOCAlertSummary() map[string]interface{} {
	return map[string]interface{}{
		"unresolved_alerts": 0,
		"critical_alerts":   0,
		"warning_alerts":    0,
		"info_alerts":       0,
	}
}
