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
	"regexp"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Security validation patterns
var (
	// Kubernetes pod name: lowercase alphanumeric, dashes, dots; start/end with alphanumeric
	validPodNameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{0,251}[a-z0-9]$|^[a-z0-9]$`)
	// Namespace: lowercase alphanumeric and dashes
	validNamespaceRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,61}[a-z0-9]$|^[a-z0-9]$`)
)

// validatePodName checks if pod name is valid Kubernetes format
func validatePodName(name string) error {
	if name == "" {
		return fmt.Errorf("pod name cannot be empty")
	}
	if len(name) > 253 {
		return fmt.Errorf("pod name too long (max 253 characters)")
	}
	if !validPodNameRegex.MatchString(name) {
		return fmt.Errorf("invalid pod name format: must be lowercase alphanumeric with dashes/dots")
	}
	return nil
}

// validateNamespace checks if namespace is valid Kubernetes format
func validateNamespace(ns string) error {
	if ns == "" {
		return nil // empty namespace is allowed (uses default)
	}
	if len(ns) > 63 {
		return fmt.Errorf("namespace too long (max 63 characters)")
	}
	if !validNamespaceRegex.MatchString(ns) {
		return fmt.Errorf("invalid namespace format")
	}
	return nil
}

type InfrastructureHandler struct {
	backupStatusCache     []byte
	backupStatusCacheTime time.Time
	backupStatusCacheTTL  time.Duration
}

func NewInfrastructureHandler() *InfrastructureHandler {
	return &InfrastructureHandler{
		backupStatusCacheTTL: 30 * time.Second, // Cache for 30 seconds
	}
}

// GetBackupStatus returns system metrics from the backup server (cached)
func (h *InfrastructureHandler) GetBackupStatus(w http.ResponseWriter, r *http.Request) {
	// Return cached response if fresh
	if h.backupStatusCache != nil && time.Since(h.backupStatusCacheTime) < h.backupStatusCacheTTL {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.Write(h.backupStatusCache)
		return
	}

	// Fetch metrics from backup server with shorter timeout
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("http://192.168.15.195:9100/metrics")
	if err != nil {
		log.Printf("[BackupStatus] Failed to connect to 192.168.15.195:9100: %v", err)
		// Return cached data if available, even if stale
		if h.backupStatusCache != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "STALE")
			w.Write(h.backupStatusCache)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to connect to backup server",
			"details": err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Backup server returned error",
		})
		return
	}

	// Read and cache the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Failed to read metrics",
		})
		return
	}

	// Update cache
	h.backupStatusCache = body
	h.backupStatusCacheTime = time.Now()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Write(body)
}

// TriggerBackup triggers an immediate backup to NAS
func (h *InfrastructureHandler) TriggerBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Trigger backup on backup server
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Post("http://192.168.15.195:9100/trigger-backup", "application/json", nil)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Failed to connect to backup server: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	// Read and forward the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Failed to read backup response",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

// UpdateBackupSchedule updates the cron schedule for automatic backups
func (h *InfrastructureHandler) UpdateBackupSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Schedule string `json:"schedule"` // "15min", "30min", "1hour", "3hours", "6hours", "12hours", "daily"
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Send schedule update to backup server
	jsonData, err := json.Marshal(req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Failed to encode request",
		})
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post("http://192.168.15.195:9100/update-schedule", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Failed to connect to backup server: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	// Read and forward the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Failed to read schedule update response",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

// GetK3sStatus returns K3s cluster health
func (h *InfrastructureHandler) GetK3sStatus(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("kubectl", "get", "nodes", "-o", "json")
	output, err := cmd.Output()

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"healthy": false,
			"message": "Failed to get K3s status",
		})
		return
	}

	// Parse node status
	var nodeData struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Status struct {
				Conditions []struct {
					Type   string `json:"type"`
					Status string `json:"status"`
				} `json:"conditions"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(output, &nodeData); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"healthy": false,
			"message": "Failed to parse node data",
		})
		return
	}

	// Count ready nodes
	readyCount := 0
	for _, node := range nodeData.Items {
		for _, cond := range node.Status.Conditions {
			if cond.Type == "Ready" && cond.Status == "True" {
				readyCount++
				break
			}
		}
	}

	totalNodes := len(nodeData.Items)
	healthy := readyCount == totalNodes && totalNodes > 0

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"healthy": healthy,
		"message": fmt.Sprintf("%d/%d nodes ready", readyCount, totalNodes),
		"nodes":   totalNodes,
		"ready":   readyCount,
	})
}

// Bare metal PostgreSQL node configuration
type dbNode struct {
	Name string
	Host string
	Port int
}

// getDBNodes returns the list of bare metal PostgreSQL nodes
func getDBNodes() []dbNode {
	return []dbNode{
		{"db-node1", "192.168.15.120", 5432},
		{"db-node2", "192.168.15.121", 5432},
		{"db-node3", "192.168.15.122", 5432},
	}
}

// getDBCredentials returns database credentials from config
func getDBCredentials() (user, password, dbname string) {
	// Use credentials from r2_config.go fallbacks
	return "cold_user", "SecurePostgresPassword123", "cold_db"
}

// GetPostgreSQLStatus returns PostgreSQL cluster health (bare metal nodes)
func (h *InfrastructureHandler) GetPostgreSQLStatus(w http.ResponseWriter, r *http.Request) {
	nodes := getDBNodes()
	user, password, dbname := getDBCredentials()

	var primary string
	replicas := []string{}
	runningCount := 0

	for _, node := range nodes {
		// Try to connect to each node
		connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable&connect_timeout=3",
			user, url.QueryEscape(password), node.Host, node.Port, dbname)

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		db, err := sql.Open("pgx", connStr)
		if err != nil {
			cancel()
			continue
		}

		if err := db.PingContext(ctx); err != nil {
			db.Close()
			cancel()
			continue
		}

		// Check if primary or replica
		var isInRecovery bool
		err = db.QueryRowContext(ctx, "SELECT pg_is_in_recovery()").Scan(&isInRecovery)
		db.Close()
		cancel()

		if err != nil {
			continue
		}

		runningCount++
		if !isInRecovery {
			primary = node.Name
		} else {
			replicas = append(replicas, node.Name)
		}
	}

	totalNodes := len(nodes)
	healthy := runningCount == totalNodes && primary != ""

	message := fmt.Sprintf("Primary + %d replicas", len(replicas))
	if !healthy {
		message = fmt.Sprintf("%d/%d nodes running", runningCount, totalNodes)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"healthy":  healthy,
		"message":  message,
		"primary":  primary,
		"replicas": replicas,
		"total":    totalNodes,
		"running":  runningCount,
	})
}

// GetPostgreSQLPods returns detailed PostgreSQL node metrics (bare metal)
func (h *InfrastructureHandler) GetPostgreSQLPods(w http.ResponseWriter, r *http.Request) {
	nodes := getDBNodes()
	user, password, dbname := getDBCredentials()

	pods := []map[string]interface{}{}

	for _, node := range nodes {
		// Default values
		dbSize := "N/A"
		connections := "N/A"
		lag := "N/A"
		cacheHit := "N/A"
		status := "Error"
		role := "Unknown"
		var syncPct float64 = -1

		// Connect to this node
		connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable&connect_timeout=3",
			user, url.QueryEscape(password), node.Host, node.Port, dbname)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		db, err := sql.Open("pgx", connStr)
		if err != nil {
			cancel()
			pods = append(pods, map[string]interface{}{
				"name":        node.Name,
				"role":        role,
				"status":      status,
				"node":        node.Host,
				"disk_used":   dbSize,
				"connections": connections,
				"max_conn":    200,
				"repl_lag":    lag,
				"cache_hit":   cacheHit,
				"sync_pct":    syncPct,
				"is_external": false,
			})
			continue
		}

		if err := db.PingContext(ctx); err != nil {
			db.Close()
			cancel()
			pods = append(pods, map[string]interface{}{
				"name":        node.Name,
				"role":        role,
				"status":      status,
				"node":        node.Host,
				"disk_used":   dbSize,
				"connections": connections,
				"max_conn":    200,
				"repl_lag":    lag,
				"cache_hit":   cacheHit,
				"sync_pct":    syncPct,
				"is_external": false,
			})
			continue
		}

		status = "Running"

		// Check if primary or replica
		var isInRecovery bool
		err = db.QueryRowContext(ctx, "SELECT pg_is_in_recovery()").Scan(&isInRecovery)
		if err == nil {
			if isInRecovery {
				role = "Replica"
			} else {
				role = "Primary"
				syncPct = -1 // Primary doesn't have sync percentage
			}
		}

		// Get database size
		var size sql.NullString
		err = db.QueryRowContext(ctx, "SELECT pg_size_pretty(pg_database_size('cold_db'))").Scan(&size)
		if err == nil && size.Valid {
			dbSize = size.String
		}

		// Get active connections
		var connCount sql.NullInt64
		err = db.QueryRowContext(ctx, "SELECT count(*) FROM pg_stat_activity WHERE datname = 'cold_db' AND pid <> pg_backend_pid()").Scan(&connCount)
		if err == nil && connCount.Valid {
			connections = fmt.Sprintf("%d", connCount.Int64)
		}

		// Get cache hit ratio
		var cache sql.NullString
		err = db.QueryRowContext(ctx, `
			SELECT COALESCE(
				ROUND(100.0 * sum(blks_hit) / NULLIF(sum(blks_hit) + sum(blks_read), 0), 1)::text || '%',
				'N/A'
			) FROM pg_stat_database WHERE datname = 'cold_db'
		`).Scan(&cache)
		if err == nil && cache.Valid && cache.String != "%" {
			cacheHit = cache.String
		}

		// Get replication lag for replicas
		if role == "Replica" {
			var lagBytes sql.NullFloat64
			err = db.QueryRowContext(ctx, `
				SELECT COALESCE(pg_wal_lsn_diff(pg_last_wal_receive_lsn(), pg_last_wal_replay_lsn()), 0)::float
			`).Scan(&lagBytes)
			if err == nil && lagBytes.Valid {
				if lagBytes.Float64 == 0 {
					lag = "0"
					syncPct = 100
				} else if lagBytes.Float64 < 1024 {
					lag = fmt.Sprintf("%.0f B", lagBytes.Float64)
				} else if lagBytes.Float64 < 1024*1024 {
					lag = fmt.Sprintf("%.1f KB", lagBytes.Float64/1024)
				} else {
					lag = fmt.Sprintf("%.1f MB", lagBytes.Float64/(1024*1024))
				}
			}

			// Get sync percentage if not already 100
			if syncPct != 100 {
				var pct sql.NullFloat64
				err = db.QueryRowContext(ctx, `
					SELECT CASE
						WHEN pg_last_wal_receive_lsn() IS NULL THEN 0
						WHEN pg_last_wal_receive_lsn() = pg_last_wal_replay_lsn() THEN 100
						ELSE ROUND((100.0 - (pg_wal_lsn_diff(pg_last_wal_receive_lsn(), pg_last_wal_replay_lsn())::numeric /
							GREATEST(pg_wal_lsn_diff(pg_last_wal_receive_lsn(), '0/0')::numeric, 1) * 100))::numeric, 1)
					END
				`).Scan(&pct)
				if err == nil && pct.Valid {
					syncPct = pct.Float64
				}
			}
		}

		db.Close()
		cancel()

		pods = append(pods, map[string]interface{}{
			"name":        node.Name,
			"role":        role,
			"status":      status,
			"node":        node.Host,
			"disk_used":   dbSize,
			"connections": connections,
			"max_conn":    200,
			"repl_lag":    lag,
			"cache_hit":   cacheHit,
			"sync_pct":    syncPct,
			"is_external": false,
		})
	}

	// Add external backup database (192.168.15.195)
	metricsDBPod := h.getMetricsDBStatus()
	if metricsDBPod != nil {
		pods = append(pods, metricsDBPod)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"pods": pods,
	})
}

// getMetricsDBStatus returns status of the backup database on 192.168.15.195
func (h *InfrastructureHandler) getMetricsDBStatus() map[string]interface{} {
	host := "192.168.15.195"
	port := "5432" // Backup database server

	// Default values
	dbSize := "N/A"
	connections := "N/A"
	cacheHit := "N/A"
	replLag := "N/A"
	status := "Running"
	role := "Unknown"
	var syncPct float64 = -1 // -1 means N/A

	// Use backup database credentials (streaming replica of primary)
	// The backup server is a streaming replica, so it uses the same credentials as primary
	dbUser := os.Getenv("BACKUP_DB_USER")
	if dbUser == "" {
		dbUser = "cold_user"
	}
	dbPassword := os.Getenv("BACKUP_DB_PASSWORD")
	if dbPassword == "" {
		dbPassword = "SecurePostgresPassword123"
	}
	dbName := os.Getenv("BACKUP_DB_NAME")
	if dbName == "" {
		dbName = "cold_db"
	}

	// Connect directly to streaming replica via network
	// Use URL-style connection string for proper password escaping
	escapedPassword := url.QueryEscape(dbPassword)
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&connect_timeout=5", dbUser, escapedPassword, host, port, dbName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		log.Printf("[MetricsDB] sql.Open error: %v", err)
		status = "Error"
		return h.buildMetricsDBResponse(host, port, dbSize, connections, cacheHit, replLag, status, role, syncPct)
	}
	defer db.Close()

	// Test the connection
	if err := db.PingContext(ctx); err != nil {
		log.Printf("[MetricsDB] Ping error: %v", err)
		status = "Error"
		return h.buildMetricsDBResponse(host, port, dbSize, connections, cacheHit, replLag, status, role, syncPct)
	}

	// Check if this is actually a replica or standalone
	var isInRecovery bool
	err = db.QueryRowContext(ctx, "SELECT pg_is_in_recovery()").Scan(&isInRecovery)
	if err != nil {
		log.Printf("[MetricsDB] pg_is_in_recovery error: %v", err)
		status = "Error"
		role = "Unknown"
	} else if isInRecovery {
		role = "Replica"
		// Get replication lag using WAL bytes difference (accurate even when idle)
		var lag sql.NullFloat64
		err = db.QueryRowContext(ctx, `
			SELECT COALESCE(pg_wal_lsn_diff(pg_last_wal_receive_lsn(), pg_last_wal_replay_lsn()), 0)::float
		`).Scan(&lag)
		if err == nil && lag.Valid {
			// Format bytes lag
			if lag.Float64 == 0 {
				replLag = "0"
				syncPct = 100
			} else if lag.Float64 < 1024 {
				replLag = fmt.Sprintf("%.0f B", lag.Float64)
			} else if lag.Float64 < 1024*1024 {
				replLag = fmt.Sprintf("%.1f KB", lag.Float64/1024)
			} else {
				replLag = fmt.Sprintf("%.1f MB", lag.Float64/(1024*1024))
			}
		}
		// Get sync percentage
		if syncPct != 100 {
			var pct sql.NullFloat64
			err = db.QueryRowContext(ctx, `
				SELECT CASE
					WHEN pg_last_wal_receive_lsn() IS NULL THEN 0
					WHEN pg_last_wal_receive_lsn() = pg_last_wal_replay_lsn() THEN 100
					ELSE ROUND((100.0 - (pg_wal_lsn_diff(pg_last_wal_receive_lsn(), pg_last_wal_replay_lsn())::numeric /
						GREATEST(pg_wal_lsn_diff(pg_last_wal_receive_lsn(), '0/0')::numeric, 1) * 100))::numeric, 1)
				END
			`).Scan(&pct)
			if err == nil && pct.Valid {
				syncPct = pct.Float64
			}
		}
	} else {
		role = "Standalone"
		replLag = "N/A"
	}

	// Get database size
	var size sql.NullString
	err = db.QueryRowContext(ctx, "SELECT pg_size_pretty(pg_database_size('cold_db'))").Scan(&size)
	if err != nil {
		status = "Error"
	} else if size.Valid {
		dbSize = size.String
	}

	// Get active connections
	var connCount sql.NullInt64
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM pg_stat_activity WHERE datname = 'cold_db'").Scan(&connCount)
	if err == nil && connCount.Valid {
		connections = fmt.Sprintf("%d", connCount.Int64)
	}

	// Get cache hit ratio
	var cache sql.NullString
	err = db.QueryRowContext(ctx, `
		SELECT COALESCE(
			ROUND(100.0 * sum(blks_hit) / NULLIF(sum(blks_hit) + sum(blks_read), 0), 1)::text || '%',
			'N/A'
		) FROM pg_stat_database WHERE datname = 'cold_db'
	`).Scan(&cache)
	if err == nil && cache.Valid && cache.String != "%" {
		cacheHit = cache.String
	}

	return h.buildMetricsDBResponse(host, port, dbSize, connections, cacheHit, replLag, status, role, syncPct)
}

func (h *InfrastructureHandler) buildMetricsDBResponse(host, port, dbSize, connections, cacheHit, replLag, status, role string, syncPct float64) map[string]interface{} {
	return map[string]interface{}{
		"name":        "backup-server (192.168.15.195)",
		"role":        role,
		"status":      status,
		"node":        host + ":" + port,
		"disk_used":   dbSize,
		"connections": connections,
		"max_conn":    200,
		"repl_lag":    replLag,
		"cache_hit":   cacheHit,
		"sync_pct":    syncPct,
		"is_external": true,
	}
}

// GetVIPStatus checks if VIP is reachable via HTTP
func (h *InfrastructureHandler) GetVIPStatus(w http.ResponseWriter, r *http.Request) {
	vip := "192.168.15.200"

	// Use HTTP check instead of ping (ping doesn't work from inside pod to cluster VIP)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s:8080/health", vip))

	healthy := err == nil && resp != nil && resp.StatusCode == http.StatusOK
	if resp != nil {
		resp.Body.Close()
	}

	message := fmt.Sprintf("%s active", vip)
	if !healthy {
		message = fmt.Sprintf("%s unreachable", vip)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"healthy": healthy,
		"message": message,
		"vip":     vip,
	})
}

// GetBackendPods returns backend pod status
func (h *InfrastructureHandler) GetBackendPods(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("kubectl", "get", "pods", "-l", "app=cold-backend", "-o", "json")
	output, err := cmd.Output()

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pods": []map[string]interface{}{},
		})
		return
	}

	// Parse pod data
	var podData struct {
		Items []struct {
			Metadata struct {
				Name   string            `json:"name"`
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
			Status struct {
				Phase             string `json:"phase"`
				ContainerStatuses []struct {
					Ready bool `json:"ready"`
				} `json:"containerStatuses"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(output, &podData); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pods": []map[string]interface{}{},
		})
		return
	}

	// Build pod list
	pods := []map[string]interface{}{}
	for _, pod := range podData.Items {
		ready := "0/1"
		if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].Ready {
			ready = "1/1"
		}

		mode := pod.Metadata.Labels["mode"]
		if mode == "" {
			mode = "unknown"
		}

		pods = append(pods, map[string]interface{}{
			"name":   pod.Metadata.Name,
			"ready":  ready,
			"status": pod.Status.Phase,
			"mode":   mode,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"pods": pods,
	})
}

// ExecuteFailover promotes a PostgreSQL replica to primary
func (h *InfrastructureHandler) ExecuteFailover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		TargetPod string `json:"target_pod"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.TargetPod == "" {
		http.Error(w, "target_pod is required", http.StatusBadRequest)
		return
	}

	// Validate pod name to prevent command injection
	if err := validatePodName(req.TargetPod); err != nil {
		http.Error(w, "Invalid pod name: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Execute failover by deleting current primary
	// CloudNativePG will automatically promote a replica
	cmd := exec.Command("kubectl", "delete", "pod", req.TargetPod, "-n", "default")
	output, err := cmd.CombinedOutput()

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Failover failed: " + err.Error(),
			"output":  string(output),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Failover initiated. Pod %s deleted. CloudNativePG will promote a replica.", req.TargetPod),
		"output":  strings.TrimSpace(string(output)),
	})
}

// RecoverStuckPods detects and recovers stuck PostgreSQL pods
func (h *InfrastructureHandler) RecoverStuckPods(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DryRun bool `json:"dry_run"` // If true, only report stuck pods without deleting
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Default to dry run if no body
		req.DryRun = true
	}

	// Get detailed pod status
	cmd := exec.Command("kubectl", "get", "pods", "-l", "cnpg.io/cluster=cold-postgres", "-o", "json")
	output, err := cmd.Output()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Failed to get pod status: " + err.Error(),
		})
		return
	}

	var podData struct {
		Items []struct {
			Metadata struct {
				Name              string            `json:"name"`
				Labels            map[string]string `json:"labels"`
				CreationTimestamp string            `json:"creationTimestamp"`
			} `json:"metadata"`
			Status struct {
				Phase                 string `json:"phase"`
				StartTime             string `json:"startTime"`
				InitContainerStatuses []struct {
					Name  string `json:"name"`
					Ready bool   `json:"ready"`
					State struct {
						Running    *struct{} `json:"running"`
						Waiting    *struct{} `json:"waiting"`
						Terminated *struct{} `json:"terminated"`
					} `json:"state"`
				} `json:"initContainerStatuses"`
				ContainerStatuses []struct {
					Name  string `json:"name"`
					Ready bool   `json:"ready"`
					State struct {
						Running    *struct{} `json:"running"`
						Waiting    *struct{} `json:"waiting"`
						Terminated *struct{} `json:"terminated"`
					} `json:"state"`
				} `json:"containerStatuses"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(output, &podData); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Failed to parse pod data: " + err.Error(),
		})
		return
	}

	// Find stuck pods and healthy pods
	stuckPods := []map[string]interface{}{}
	healthyPods := []string{}
	var primaryPod string

	for _, pod := range podData.Items {
		// Skip completed jobs
		if pod.Status.Phase == "Succeeded" || strings.Contains(pod.Metadata.Name, "-initdb") || strings.Contains(pod.Metadata.Name, "-join") {
			continue
		}

		isPrimary := pod.Metadata.Labels["role"] == "primary"
		if isPrimary {
			primaryPod = pod.Metadata.Name
		}

		// Check if pod is healthy
		isHealthy := pod.Status.Phase == "Running"
		if len(pod.Status.ContainerStatuses) > 0 {
			isHealthy = isHealthy && pod.Status.ContainerStatuses[0].Ready
		}

		if isHealthy {
			healthyPods = append(healthyPods, pod.Metadata.Name)
			continue
		}

		// Calculate how long pod has been in current state
		var creationTime time.Time
		if pod.Metadata.CreationTimestamp != "" {
			creationTime, _ = time.Parse(time.RFC3339, pod.Metadata.CreationTimestamp)
		}
		stuckDuration := time.Since(creationTime)

		// Determine stuck reason
		stuckReason := "Unknown"
		if pod.Status.Phase == "Pending" {
			stuckReason = "Pending"
		} else if pod.Status.Phase == "Failed" {
			stuckReason = "Failed"
		} else if len(pod.Status.InitContainerStatuses) > 0 && !pod.Status.InitContainerStatuses[0].Ready {
			stuckReason = "Init:0/1"
		} else if len(pod.Status.ContainerStatuses) > 0 && !pod.Status.ContainerStatuses[0].Ready {
			stuckReason = "0/1 Running"
		}

		// Apply thresholds
		thresholdMet := false
		threshold := 5 * time.Minute
		if pod.Status.Phase == "Pending" {
			threshold = 10 * time.Minute
		}
		if pod.Status.Phase == "Failed" {
			threshold = 1 * time.Minute // Immediate for failed
		}
		thresholdMet = stuckDuration > threshold

		if thresholdMet {
			stuckPods = append(stuckPods, map[string]interface{}{
				"name":           pod.Metadata.Name,
				"reason":         stuckReason,
				"stuck_duration": stuckDuration.Round(time.Second).String(),
				"is_primary":     isPrimary,
				"can_recover":    !isPrimary && len(healthyPods) > 0,
			})
		}
	}

	// If dry run, just return the list
	if req.DryRun {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":      true,
			"dry_run":      true,
			"stuck_pods":   stuckPods,
			"healthy_pods": healthyPods,
			"primary":      primaryPod,
			"message":      fmt.Sprintf("Found %d stuck pods, %d healthy pods", len(stuckPods), len(healthyPods)),
		})
		return
	}

	// Execute recovery
	recoveredPods := []string{}
	skippedPods := []string{}

	for _, stuckPod := range stuckPods {
		podName := stuckPod["name"].(string)
		isPrimary := stuckPod["is_primary"].(bool)
		canRecover := stuckPod["can_recover"].(bool)

		// Safety checks
		if isPrimary {
			skippedPods = append(skippedPods, podName+" (primary)")
			continue
		}
		if !canRecover {
			skippedPods = append(skippedPods, podName+" (no healthy pods)")
			continue
		}

		// Delete the stuck pod
		deleteCmd := exec.Command("kubectl", "delete", "pod", podName, "--force", "--grace-period=0")
		if _, err := deleteCmd.Output(); err == nil {
			recoveredPods = append(recoveredPods, podName)
		} else {
			skippedPods = append(skippedPods, podName+" (delete failed)")
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":        true,
		"dry_run":        false,
		"recovered_pods": recoveredPods,
		"skipped_pods":   skippedPods,
		"healthy_pods":   healthyPods,
		"message":        fmt.Sprintf("Recovered %d pods, skipped %d", len(recoveredPods), len(skippedPods)),
	})
}

// GetRecoveryStatus returns auto-recovery settings and recent recovery actions
func (h *InfrastructureHandler) GetRecoveryStatus(w http.ResponseWriter, r *http.Request) {
	// This will be populated from the metrics collector's recovery status
	// For now, return basic status based on pod health
	cmd := exec.Command("kubectl", "get", "pods", "-l", "cnpg.io/cluster=cold-postgres", "-o", "json")
	output, err := cmd.Output()

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"auto_recovery_enabled": true,
			"cluster_healthy":       false,
			"message":               "Cannot get pod status",
		})
		return
	}

	var podData struct {
		Items []struct {
			Status struct {
				Phase             string `json:"phase"`
				ContainerStatuses []struct {
					Ready bool `json:"ready"`
				} `json:"containerStatuses"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(output, &podData); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"auto_recovery_enabled": true,
			"cluster_healthy":       false,
			"message":               "Cannot parse pod data",
		})
		return
	}

	// Check cluster health
	healthyCount := 0
	totalCount := 0
	for _, pod := range podData.Items {
		if pod.Status.Phase == "Running" {
			if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].Ready {
				healthyCount++
			}
		}
		totalCount++
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"auto_recovery_enabled": true,
		"cluster_healthy":       healthyCount == totalCount && totalCount > 0,
		"healthy_pods":          healthyCount,
		"total_pods":            totalCount,
		"message":               fmt.Sprintf("%d/%d pods healthy", healthyCount, totalCount),
	})
}

// DownloadDatabase creates a pg_dump and streams it as a downloadable file
func (h *InfrastructureHandler) DownloadDatabase(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("cold_db_backup_%s.sql", timestamp)

	// Set headers for file download
	w.Header().Set("Content-Type", "application/sql")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	// Try Docker first (local backup server)
	cmd := exec.Command("docker", "exec", "cold-storage-postgres", "pg_dump", "-U", "postgres", "cold_db")
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		w.Write(output)
		return
	}

	// Fallback: Try CNPG primary pod
	// Find the primary pod
	findPrimaryCmd := exec.Command("kubectl", "get", "pods", "-n", "default",
		"-l", "cnpg.io/cluster=cold-postgres",
		"-o", "jsonpath={.items[?(@.metadata.labels.role=='primary')].metadata.name}")
	primaryPodBytes, err := findPrimaryCmd.Output()
	if err != nil {
		http.Error(w, "Failed to find database: "+err.Error(), http.StatusInternalServerError)
		return
	}

	primaryPod := strings.TrimSpace(string(primaryPodBytes))
	if primaryPod == "" {
		// Try to find any running postgres pod
		findAnyCmd := exec.Command("kubectl", "get", "pods", "-n", "default",
			"-l", "cnpg.io/cluster=cold-postgres",
			"-o", "jsonpath={.items[0].metadata.name}")
		anyPodBytes, err := findAnyCmd.Output()
		if err != nil || len(anyPodBytes) == 0 {
			http.Error(w, "No PostgreSQL pod found", http.StatusInternalServerError)
			return
		}
		primaryPod = strings.TrimSpace(string(anyPodBytes))
	}

	// Run pg_dump on the pod
	dumpCmd := exec.Command("kubectl", "exec", primaryPod, "-n", "default", "-c", "postgres",
		"--", "pg_dump", "-U", "postgres", "cold_db")
	dumpOutput, err := dumpCmd.Output()
	if err != nil {
		http.Error(w, "Failed to dump database: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(dumpOutput)
}
