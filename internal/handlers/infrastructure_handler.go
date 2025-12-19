package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type InfrastructureHandler struct{}

func NewInfrastructureHandler() *InfrastructureHandler {
	return &InfrastructureHandler{}
}

// GetBackupStatus returns system metrics from the backup server
func (h *InfrastructureHandler) GetBackupStatus(w http.ResponseWriter, r *http.Request) {
	// Fetch metrics from backup server (192.168.15.195:9100)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("http://192.168.15.195:9100/metrics")
	if err != nil {
		http.Error(w, "Failed to connect to backup server", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Failed to get metrics from backup server", http.StatusInternalServerError)
		return
	}

	// Read and forward the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read metrics", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
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

// GetPostgreSQLStatus returns PostgreSQL cluster health
func (h *InfrastructureHandler) GetPostgreSQLStatus(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("kubectl", "get", "pods", "-l", "cnpg.io/cluster=cold-postgres", "-o", "json")
	output, err := cmd.Output()

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"healthy": false,
			"message": "Failed to get PostgreSQL status",
		})
		return
	}

	// Parse pod status
	var podData struct {
		Items []struct {
			Metadata struct {
				Name   string            `json:"name"`
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
			Status struct {
				Phase string `json:"phase"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(output, &podData); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"healthy": false,
			"message": "Failed to parse pod data",
		})
		return
	}

	// Find primary and count replicas (exclude Completed/Init pods)
	var primary string
	replicas := []string{}
	runningCount := 0
	activePods := 0

	for _, pod := range podData.Items {
		// Skip completed init/join jobs
		if pod.Status.Phase == "Succeeded" || strings.Contains(pod.Metadata.Name, "-initdb") || strings.Contains(pod.Metadata.Name, "-join") {
			continue
		}
		activePods++

		if pod.Status.Phase == "Running" {
			runningCount++
			if pod.Metadata.Labels["role"] == "primary" {
				primary = pod.Metadata.Name
			} else {
				replicas = append(replicas, pod.Metadata.Name)
			}
		}
	}

	totalPods := activePods
	healthy := runningCount == totalPods && totalPods > 0 && primary != ""

	message := fmt.Sprintf("Primary + %d replicas", len(replicas))
	if !healthy {
		message = fmt.Sprintf("%d/%d pods running", runningCount, totalPods)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"healthy":  healthy,
		"message":  message,
		"primary":  primary,
		"replicas": replicas,
		"total":    totalPods,
		"running":  runningCount,
	})
}

// GetPostgreSQLPods returns detailed PostgreSQL pod metrics
func (h *InfrastructureHandler) GetPostgreSQLPods(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("kubectl", "get", "pods", "-l", "cnpg.io/cluster=cold-postgres", "-o", "json")
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
				Phase string `json:"phase"`
			} `json:"status"`
			Spec struct {
				NodeName string `json:"nodeName"`
			} `json:"spec"`
		} `json:"items"`
	}

	if err := json.Unmarshal(output, &podData); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pods": []map[string]interface{}{},
		})
		return
	}

	// Build detailed pod list
	pods := []map[string]interface{}{}
	for _, pod := range podData.Items {
		// Skip completed init/join jobs
		if pod.Status.Phase == "Succeeded" || strings.Contains(pod.Metadata.Name, "-initdb") || strings.Contains(pod.Metadata.Name, "-join") {
			continue
		}

		role := "Replica"
		if pod.Metadata.Labels["role"] == "primary" {
			role = "Primary"
		}

		// Default values
		dbSize := "N/A"
		connections := "N/A"
		lag := "N/A"
		cacheHit := "N/A"

		// Only run kubectl exec if pod is Running
		if pod.Status.Phase == "Running" {
			// Get database size
			dbSizeCmd := exec.Command("kubectl", "exec", pod.Metadata.Name, "--", "psql", "-U", "postgres", "-d", "cold_db", "-t", "-c",
				"SELECT pg_size_pretty(pg_database_size('cold_db'));")
			if dbSizeOutput, err := dbSizeCmd.Output(); err == nil {
				if size := strings.TrimSpace(string(dbSizeOutput)); size != "" {
					dbSize = size
				}
			}

			// Get active connections
			connCmd := exec.Command("kubectl", "exec", pod.Metadata.Name, "--", "psql", "-U", "postgres", "-d", "cold_db", "-t", "-c",
				"SELECT count(*) FROM pg_stat_activity WHERE datname = 'cold_db' AND pid <> pg_backend_pid();")
			if connOutput, err := connCmd.Output(); err == nil {
				if conn := strings.TrimSpace(string(connOutput)); conn != "" {
					connections = conn
				}
			}

			// Get cache hit ratio
			cacheCmd := exec.Command("kubectl", "exec", pod.Metadata.Name, "--", "psql", "-U", "postgres", "-d", "cold_db", "-t", "-c",
				"SELECT ROUND(100.0 * sum(blks_hit) / NULLIF(sum(blks_hit) + sum(blks_read), 0), 1)::text || '%' FROM pg_stat_database WHERE datname = 'cold_db';")
			if cacheOutput, err := cacheCmd.Output(); err == nil {
				if c := strings.TrimSpace(string(cacheOutput)); c != "" && c != "%" {
					cacheHit = c
				}
			}

			// Get replication lag for replicas using WAL bytes difference (accurate even when idle)
			if role == "Replica" {
				lagCmd := exec.Command("kubectl", "exec", pod.Metadata.Name, "--", "psql", "-U", "postgres", "-d", "cold_db", "-t", "-c",
					`SELECT COALESCE(pg_wal_lsn_diff(pg_last_wal_receive_lsn(), pg_last_wal_replay_lsn()), 0)`)
				if lagOutput, err := lagCmd.Output(); err == nil {
					if l := strings.TrimSpace(string(lagOutput)); l != "" {
						// Convert bytes to human-readable format
						if bytes, err := strconv.ParseFloat(l, 64); err == nil {
							if bytes == 0 {
								lag = "0"
							} else if bytes < 1024 {
								lag = fmt.Sprintf("%.0f B", bytes)
							} else if bytes < 1024*1024 {
								lag = fmt.Sprintf("%.1f KB", bytes/1024)
							} else {
								lag = fmt.Sprintf("%.1f MB", bytes/(1024*1024))
							}
						}
					}
				}
			}
		}

		pods = append(pods, map[string]interface{}{
			"name":        pod.Metadata.Name,
			"role":        role,
			"status":      pod.Status.Phase,
			"node":        pod.Spec.NodeName,
			"disk_used":   dbSize,
			"connections": connections,
			"max_conn":    200,
			"repl_lag":    lag,
			"cache_hit":   cacheHit,
			"is_external": false,
		})
	}

	// Add external metrics database (192.168.15.195)
	metricsDBPod := h.getMetricsDBStatus()
	if metricsDBPod != nil {
		pods = append(pods, metricsDBPod)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"pods": pods,
	})
}

// getMetricsDBStatus returns status of the external streaming replica on 192.168.15.195
func (h *InfrastructureHandler) getMetricsDBStatus() map[string]interface{} {
	host := "192.168.15.195"
	port := "5434" // Streaming replica of K8s cluster

	// Default values
	dbSize := "N/A"
	connections := "N/A"
	cacheHit := "N/A"
	replLag := "N/A"
	status := "Running"
	role := "Unknown"

	// Connect directly to streaming replica via network
	connStr := fmt.Sprintf("host=%s port=%s user=postgres password=postgres dbname=cold_db sslmode=disable connect_timeout=5", host, port)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		status = "Error"
		return h.buildMetricsDBResponse(host, port, dbSize, connections, cacheHit, replLag, status, role)
	}
	defer db.Close()

	// Check if this is actually a replica or standalone
	var isInRecovery bool
	err = db.QueryRowContext(ctx, "SELECT pg_is_in_recovery()").Scan(&isInRecovery)
	if err != nil {
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
			} else if lag.Float64 < 1024 {
				replLag = fmt.Sprintf("%.0f B", lag.Float64)
			} else if lag.Float64 < 1024*1024 {
				replLag = fmt.Sprintf("%.1f KB", lag.Float64/1024)
			} else {
				replLag = fmt.Sprintf("%.1f MB", lag.Float64/(1024*1024))
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

	return h.buildMetricsDBResponse(host, port, dbSize, connections, cacheHit, replLag, status, role)
}

func (h *InfrastructureHandler) buildMetricsDBResponse(host, port, dbSize, connections, cacheHit, replLag, status, role string) map[string]interface{} {
	return map[string]interface{}{
		"name":        "streaming-replica (192.168.15.195)",
		"role":        role,
		"status":      status,
		"node":        host + ":" + port,
		"disk_used":   dbSize,
		"connections": connections,
		"max_conn":    200,
		"repl_lag":    replLag,
		"cache_hit":   cacheHit,
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

	// Execute failover by deleting current primary
	// CloudNativePG will automatically promote a replica
	cmd := exec.Command("kubectl", "delete", "pod", req.TargetPod)
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
