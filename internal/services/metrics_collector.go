package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
)

// PodState tracks the state of a PostgreSQL pod for recovery detection
type PodState struct {
	Name            string
	Phase           string
	Ready           bool
	InitReady       bool
	FirstSeenUnhealthy *time.Time
	RecoveryAttempts   int
	LastRecoveryTime   *time.Time
}

// PodStateTracker tracks pod states for auto-recovery
type PodStateTracker struct {
	mu        sync.Mutex
	podStates map[string]*PodState
}

// NewPodStateTracker creates a new pod state tracker
func NewPodStateTracker() *PodStateTracker {
	return &PodStateTracker{
		podStates: make(map[string]*PodState),
	}
}

// MetricsCollector collects and stores infrastructure metrics
type MetricsCollector struct {
	repo           *repositories.MetricsRepository
	httpClient     *http.Client
	collectInterval time.Duration
	stopChan       chan struct{}
	wg             sync.WaitGroup

	// K3s nodes configuration
	nodes []NodeConfig

	// Previous network bytes for rate calculation
	prevNetworkRx map[string]int64
	prevNetworkTx map[string]int64
	prevTime      map[string]time.Time

	// Pod recovery tracking
	podTracker       *PodStateTracker
	recoveryEnabled  bool
}

// NodeConfig represents K3s node configuration
type NodeConfig struct {
	Name string
	IP   string
	Role string
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(repo *repositories.MetricsRepository) *MetricsCollector {
	return &MetricsCollector{
		repo:           repo,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
		collectInterval: 30 * time.Second,
		stopChan:       make(chan struct{}),
		nodes: []NodeConfig{
			{Name: "k3s-node1", IP: "192.168.15.110", Role: "control-plane"},
			{Name: "k3s-node2", IP: "192.168.15.111", Role: "control-plane"},
			{Name: "k3s-node3", IP: "192.168.15.112", Role: "control-plane"},
			{Name: "k3s-node4", IP: "192.168.15.113", Role: "worker"},
			{Name: "k3s-node5", IP: "192.168.15.114", Role: "worker"},
			{Name: "backup-server", IP: "192.168.15.195", Role: "backup"},
		},
		prevNetworkRx:   make(map[string]int64),
		prevNetworkTx:   make(map[string]int64),
		prevTime:        make(map[string]time.Time),
		podTracker:      NewPodStateTracker(),
		recoveryEnabled: true, // Auto-recovery enabled by default
	}
}

// Start begins the metrics collection loop
func (c *MetricsCollector) Start() {
	log.Println("[MetricsCollector] Starting metrics collector...")

	// Collect immediately on start
	c.collectAll()

	// Start collection loop
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ticker := time.NewTicker(c.collectInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				c.collectAll()
			case <-c.stopChan:
				log.Println("[MetricsCollector] Stopping metrics collector...")
				return
			}
		}
	}()
}

// Stop stops the metrics collection
func (c *MetricsCollector) Stop() {
	close(c.stopChan)
	c.wg.Wait()
}

// collectAll collects all metrics
func (c *MetricsCollector) collectAll() {
	ctx := context.Background()

	// Collect node metrics in parallel
	var wg sync.WaitGroup
	for _, node := range c.nodes {
		wg.Add(1)
		go func(n NodeConfig) {
			defer wg.Done()
			c.collectNodeMetrics(ctx, n)
		}(node)
	}

	// Collect PostgreSQL metrics
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.collectPostgresMetrics(ctx)
	}()

	// Collect VIP status
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.collectVIPStatus(ctx)
	}()

	wg.Wait()

	// Check for stuck pods and auto-recover if enabled
	if c.recoveryEnabled {
		c.checkAndRecoverStuckPods(ctx)
	}
}

// collectNodeMetrics collects metrics for a single node (K3s or standalone)
func (c *MetricsCollector) collectNodeMetrics(ctx context.Context, node NodeConfig) {
	metrics := &models.NodeMetrics{
		Time:       time.Now(),
		NodeName:   node.Name,
		NodeIP:     node.IP,
		NodeRole:   node.Role,
		NodeStatus: "Ready",
	}

	// For K3s nodes, use kubectl; for standalone nodes (backup), use node_exporter only
	isK3sNode := node.Role == "control-plane" || node.Role == "worker"

	if isK3sNode {
		// Get node status from kubectl
		nodeStatus := c.getK3sNodeStatus(node.Name)
		metrics.NodeStatus = nodeStatus

		// Get pod count for this node
		metrics.PodCount = c.getNodePodCount(node.Name)

		// Get resource metrics via kubectl top
		c.getNodeResourceMetrics(node, metrics)
	} else {
		// For non-K3s nodes, get metrics from node_exporter
		c.getStandaloneNodeMetrics(node, metrics)
	}

	// Calculate network rates
	c.calculateNetworkRates(node.Name, metrics)

	// Store metrics
	if err := c.repo.InsertNodeMetrics(ctx, metrics); err != nil {
		log.Printf("[MetricsCollector] Error storing node metrics for %s: %v", node.Name, err)
	}
}

// getK3sNodeStatus gets the status of a K3s node
func (c *MetricsCollector) getK3sNodeStatus(nodeName string) string {
	cmd := exec.Command("kubectl", "get", "node", nodeName, "-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
	output, err := cmd.Output()
	if err != nil {
		return "Unknown"
	}

	status := strings.TrimSpace(string(output))
	if status == "True" {
		return "Ready"
	}
	return "NotReady"
}

// getNodePodCount gets the number of pods running on a node
func (c *MetricsCollector) getNodePodCount(nodeName string) int {
	cmd := exec.Command("kubectl", "get", "pods", "--all-namespaces", "--field-selector", fmt.Sprintf("spec.nodeName=%s", nodeName), "-o", "json")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	var podData struct {
		Items []interface{} `json:"items"`
	}
	if err := json.Unmarshal(output, &podData); err != nil {
		return 0
	}

	return len(podData.Items)
}

// getNodeResourceMetrics gets CPU, memory, disk metrics for a node
func (c *MetricsCollector) getNodeResourceMetrics(node NodeConfig, metrics *models.NodeMetrics) {
	// Try kubectl top node first
	cmd := exec.Command("kubectl", "top", "node", node.Name, "--no-headers")
	output, err := cmd.Output()
	if err == nil {
		// Parse: NAME CPU(cores) CPU% MEMORY(bytes) MEMORY%
		fields := strings.Fields(string(output))
		if len(fields) >= 5 {
			// Parse CPU percentage
			cpuStr := strings.TrimSuffix(fields[2], "%")
			if cpu, err := strconv.ParseFloat(cpuStr, 64); err == nil {
				metrics.CPUPercent = cpu
			}

			// Parse Memory percentage
			memStr := strings.TrimSuffix(fields[4], "%")
			if mem, err := strconv.ParseFloat(memStr, 64); err == nil {
				metrics.MemoryPercent = mem
			}
		}
	}

	// Get detailed node info from kubectl describe
	cmd = exec.Command("kubectl", "get", "node", node.Name, "-o", "json")
	output, err = cmd.Output()
	if err == nil {
		var nodeData struct {
			Status struct {
				Capacity struct {
					CPU    string `json:"cpu"`
					Memory string `json:"memory"`
				} `json:"capacity"`
				Allocatable struct {
					CPU    string `json:"cpu"`
					Memory string `json:"memory"`
				} `json:"allocatable"`
			} `json:"status"`
		}

		if err := json.Unmarshal(output, &nodeData); err == nil {
			// Parse CPU cores
			metrics.CPUCores = parseCPU(nodeData.Status.Capacity.CPU)

			// Parse memory
			metrics.MemoryTotalBytes = parseMemory(nodeData.Status.Capacity.Memory)
			if metrics.MemoryPercent > 0 {
				metrics.MemoryUsedBytes = int64(float64(metrics.MemoryTotalBytes) * metrics.MemoryPercent / 100)
			}
		}
	}

	// Get disk usage via node_exporter
	c.getNodeDiskUsage(node, metrics)

	// Get load average via node_exporter
	c.getNodeLoadAverage(node, metrics)

	// Get network bytes via node_exporter
	c.getNodeNetworkBytes(node, metrics)
}

// getNodeDiskUsage gets disk usage for a node via node_exporter
func (c *MetricsCollector) getNodeDiskUsage(node NodeConfig, metrics *models.NodeMetrics) {
	exporterMetrics := c.fetchNodeExporterMetrics(node.IP)
	if exporterMetrics == "" {
		return
	}

	var sizeBytes, availBytes float64

	// Parse node_filesystem metrics for root mountpoint
	for _, line := range strings.Split(exporterMetrics, "\n") {
		// Only look at root filesystem
		if !strings.Contains(line, `mountpoint="/"`) {
			continue
		}
		// Skip special filesystems
		if strings.Contains(line, `fstype="tmpfs"`) || strings.Contains(line, `fstype="overlay"`) {
			continue
		}

		if strings.HasPrefix(line, "node_filesystem_size_bytes{") {
			if val := parsePrometheusValue(line); val > 0 {
				sizeBytes = val
			}
		}
		if strings.HasPrefix(line, "node_filesystem_avail_bytes{") {
			if val := parsePrometheusValue(line); val > 0 {
				availBytes = val
			}
		}
	}

	if sizeBytes > 0 {
		metrics.DiskTotalBytes = int64(sizeBytes)
		metrics.DiskUsedBytes = int64(sizeBytes - availBytes)
		metrics.DiskPercent = (sizeBytes - availBytes) / sizeBytes * 100
	}
}

// getNodeLoadAverage gets load average for a node via node_exporter
func (c *MetricsCollector) getNodeLoadAverage(node NodeConfig, metrics *models.NodeMetrics) {
	exporterMetrics := c.fetchNodeExporterMetrics(node.IP)
	if exporterMetrics == "" {
		return
	}

	// Parse load average metrics
	for _, line := range strings.Split(exporterMetrics, "\n") {
		if strings.HasPrefix(line, "node_load1 ") {
			if val := parsePrometheusValue(line); val >= 0 {
				metrics.LoadAverage1m = val
			}
		}
		if strings.HasPrefix(line, "node_load5 ") {
			if val := parsePrometheusValue(line); val >= 0 {
				metrics.LoadAverage5m = val
			}
		}
		if strings.HasPrefix(line, "node_load15 ") {
			if val := parsePrometheusValue(line); val >= 0 {
				metrics.LoadAverage15m = val
			}
		}
	}
}

// BackupServerMetrics represents the JSON metrics from backup-server
type BackupServerMetrics struct {
	Healthy       bool    `json:"healthy"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
	DiskRoot      struct {
		Total   string  `json:"total"`
		Used    string  `json:"used"`
		Free    string  `json:"free"`
		Percent float64 `json:"percent"`
	} `json:"disk_root"`
}

// getStandaloneNodeMetrics gets all metrics for non-K3s nodes via node_exporter or JSON endpoint
func (c *MetricsCollector) getStandaloneNodeMetrics(node NodeConfig, metrics *models.NodeMetrics) {
	// For backup-server, try JSON endpoint first
	if node.Role == "backup" {
		if c.getBackupServerJSONMetrics(node, metrics) {
			return
		}
	}

	// Fall back to Prometheus-format node_exporter
	exporterMetrics := c.fetchNodeExporterMetrics(node.IP)
	if exporterMetrics == "" {
		metrics.NodeStatus = "NotReady"
		return
	}

	metrics.NodeStatus = "Ready"
	metrics.PodCount = 0 // Not a K3s node

	var cpuIdle, cpuTotal float64
	var memTotal, memAvailable float64

	for _, line := range strings.Split(exporterMetrics, "\n") {
		// CPU metrics (calculate from idle time)
		if strings.HasPrefix(line, "node_cpu_seconds_total{") && strings.Contains(line, `mode="idle"`) {
			if val := parsePrometheusValue(line); val > 0 {
				cpuIdle += val
			}
		}
		if strings.HasPrefix(line, "node_cpu_seconds_total{") {
			if val := parsePrometheusValue(line); val > 0 {
				cpuTotal += val
			}
		}

		// Memory metrics
		if strings.HasPrefix(line, "node_memory_MemTotal_bytes ") {
			if val := parsePrometheusValue(line); val > 0 {
				memTotal = val
			}
		}
		if strings.HasPrefix(line, "node_memory_MemAvailable_bytes ") {
			if val := parsePrometheusValue(line); val > 0 {
				memAvailable = val
			}
		}
	}

	// Calculate CPU percentage (approximate - not perfect but useful)
	if cpuTotal > 0 && cpuIdle > 0 {
		metrics.CPUPercent = 100.0 - (cpuIdle / cpuTotal * 100.0)
		if metrics.CPUPercent < 0 {
			metrics.CPUPercent = 0
		}
		if metrics.CPUPercent > 100 {
			metrics.CPUPercent = 100
		}
	}

	// Memory
	if memTotal > 0 {
		metrics.MemoryTotalBytes = int64(memTotal)
		metrics.MemoryUsedBytes = int64(memTotal - memAvailable)
		if memAvailable > 0 {
			metrics.MemoryPercent = (memTotal - memAvailable) / memTotal * 100.0
		}
	}

	// Get disk, load, and network
	c.getNodeDiskUsage(node, metrics)
	c.getNodeLoadAverage(node, metrics)
	c.getNodeNetworkBytes(node, metrics)
}

// getBackupServerJSONMetrics fetches metrics from the backup-server JSON endpoint
func (c *MetricsCollector) getBackupServerJSONMetrics(node NodeConfig, metrics *models.NodeMetrics) bool {
	url := fmt.Sprintf("http://%s:9100/metrics", node.IP)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	var bsMetrics BackupServerMetrics
	if err := json.Unmarshal(body, &bsMetrics); err != nil {
		return false
	}

	metrics.NodeStatus = "Ready"
	if !bsMetrics.Healthy {
		metrics.NodeStatus = "NotReady"
	}
	metrics.PodCount = 0

	metrics.CPUPercent = bsMetrics.CPUPercent
	metrics.MemoryPercent = bsMetrics.MemoryPercent
	metrics.DiskPercent = bsMetrics.DiskRoot.Percent

	// Parse disk values from strings (e.g., "1.8T", "361G")
	metrics.DiskTotalBytes = parseSizeString(bsMetrics.DiskRoot.Total)
	metrics.DiskUsedBytes = parseSizeString(bsMetrics.DiskRoot.Used)

	return true
}

// parseSizeString converts human-readable sizes like "1.8T", "361G" to bytes
func parseSizeString(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "N/A" {
		return 0
	}

	multiplier := int64(1)
	if strings.HasSuffix(s, "T") {
		multiplier = 1024 * 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "T")
	} else if strings.HasSuffix(s, "G") {
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "G")
	} else if strings.HasSuffix(s, "M") {
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(s, "M")
	} else if strings.HasSuffix(s, "K") {
		multiplier = 1024
		s = strings.TrimSuffix(s, "K")
	}

	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return int64(val * float64(multiplier))
}

// getNodeNetworkBytes gets network bytes for a node via node_exporter
func (c *MetricsCollector) getNodeNetworkBytes(node NodeConfig, metrics *models.NodeMetrics) {
	exporterMetrics := c.fetchNodeExporterMetrics(node.IP)
	if exporterMetrics == "" {
		return
	}

	// Parse network metrics - sum all physical interfaces (exclude lo, docker, veth, cni)
	var totalRx, totalTx float64
	for _, line := range strings.Split(exporterMetrics, "\n") {
		// Skip virtual interfaces
		if strings.Contains(line, `device="lo"`) || strings.Contains(line, `device="docker"`) ||
			strings.Contains(line, `device="veth"`) || strings.Contains(line, `device="cni"`) ||
			strings.Contains(line, `device="flannel"`) || strings.Contains(line, `device="cali"`) {
			continue
		}
		if strings.HasPrefix(line, "node_network_receive_bytes_total{") {
			if val := parsePrometheusValue(line); val > 0 {
				totalRx += val
			}
		}
		if strings.HasPrefix(line, "node_network_transmit_bytes_total{") {
			if val := parsePrometheusValue(line); val > 0 {
				totalTx += val
			}
		}
	}
	metrics.NetworkRxBytes = int64(totalRx)
	metrics.NetworkTxBytes = int64(totalTx)
}

// nodeExporterCache caches node_exporter responses to avoid duplicate fetches
var nodeExporterCache = make(map[string]string)
var nodeExporterCacheMu sync.Mutex
var nodeExporterCacheTime = make(map[string]time.Time)

// fetchNodeExporterMetrics fetches metrics from node_exporter (with caching per collection cycle)
func (c *MetricsCollector) fetchNodeExporterMetrics(nodeIP string) string {
	nodeExporterCacheMu.Lock()
	defer nodeExporterCacheMu.Unlock()

	// Check cache (valid for 25 seconds)
	if cached, ok := nodeExporterCache[nodeIP]; ok {
		if time.Since(nodeExporterCacheTime[nodeIP]) < 25*time.Second {
			return cached
		}
	}

	// Fetch from node_exporter
	resp, err := c.httpClient.Get(fmt.Sprintf("http://%s:9100/metrics", nodeIP))
	if err != nil {
		log.Printf("[MetricsCollector] Error fetching node_exporter metrics from %s: %v", nodeIP, err)
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	result := string(body)
	nodeExporterCache[nodeIP] = result
	nodeExporterCacheTime[nodeIP] = time.Now()
	return result
}

// parsePrometheusValue extracts the numeric value from a Prometheus metric line
func parsePrometheusValue(line string) float64 {
	// Format: metric_name{labels} value or metric_name value
	parts := strings.Fields(line)
	if len(parts) >= 2 {
		// Get the last part which is the value
		valStr := parts[len(parts)-1]
		if val, err := strconv.ParseFloat(valStr, 64); err == nil {
			return val
		}
	}
	return -1
}

// calculateNetworkRates calculates network rates from total bytes
func (c *MetricsCollector) calculateNetworkRates(nodeName string, metrics *models.NodeMetrics) {
	prevRx, hasRx := c.prevNetworkRx[nodeName]
	prevTx, hasTx := c.prevNetworkTx[nodeName]
	prevT, hasT := c.prevTime[nodeName]

	if hasRx && hasTx && hasT {
		elapsed := time.Since(prevT).Seconds()
		if elapsed > 0 {
			metrics.NetworkRxRate = int64(float64(metrics.NetworkRxBytes-prevRx) / elapsed)
			metrics.NetworkTxRate = int64(float64(metrics.NetworkTxBytes-prevTx) / elapsed)
		}
	}

	// Store current values for next calculation
	c.prevNetworkRx[nodeName] = metrics.NetworkRxBytes
	c.prevNetworkTx[nodeName] = metrics.NetworkTxBytes
	c.prevTime[nodeName] = time.Now()
}

// collectPostgresMetrics collects metrics from PostgreSQL pods
func (c *MetricsCollector) collectPostgresMetrics(ctx context.Context) {
	// Get PostgreSQL pods
	cmd := exec.Command("kubectl", "get", "pods", "-l", "cnpg.io/cluster=cold-postgres", "-o", "json")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("[MetricsCollector] Error getting PostgreSQL pods: %v", err)
		return
	}

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
		return
	}

	for _, pod := range podData.Items {
		// Skip init/join jobs
		if pod.Status.Phase == "Succeeded" || strings.Contains(pod.Metadata.Name, "-initdb") || strings.Contains(pod.Metadata.Name, "-join") {
			continue
		}

		metrics := &models.PostgresMetrics{
			Time:     time.Now(),
			PodName:  pod.Metadata.Name,
			NodeName: pod.Spec.NodeName,
			Status:   pod.Status.Phase,
			Role:     "Replica",
		}

		if pod.Metadata.Labels["role"] == "primary" {
			metrics.Role = "Primary"
		}

		// Get database metrics
		c.getPostgresDatabaseMetrics(pod.Metadata.Name, metrics)

		// Store metrics
		if err := c.repo.InsertPostgresMetrics(ctx, metrics); err != nil {
			log.Printf("[MetricsCollector] Error storing PostgreSQL metrics for %s: %v", pod.Metadata.Name, err)
		}
	}
}

// getPostgresDatabaseMetrics gets detailed PostgreSQL metrics
func (c *MetricsCollector) getPostgresDatabaseMetrics(podName string, metrics *models.PostgresMetrics) {
	// Get database size
	cmd := exec.Command("kubectl", "exec", podName, "--", "psql", "-U", "postgres", "-d", "cold_db", "-t", "-c",
		"SELECT pg_database_size('cold_db')")
	output, _ := cmd.Output()
	if size, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64); err == nil {
		metrics.DatabaseSizeBytes = size
	}

	// Get connections
	cmd = exec.Command("kubectl", "exec", podName, "--", "psql", "-U", "postgres", "-d", "cold_db", "-t", "-c",
		"SELECT count(*) FILTER (WHERE state = 'active'), count(*) FILTER (WHERE state = 'idle'), count(*) FROM pg_stat_activity WHERE datname = 'cold_db'")
	output, _ = cmd.Output()
	fields := strings.Split(strings.TrimSpace(string(output)), "|")
	if len(fields) >= 3 {
		if active, err := strconv.Atoi(strings.TrimSpace(fields[0])); err == nil {
			metrics.ActiveConnections = active
		}
		if idle, err := strconv.Atoi(strings.TrimSpace(fields[1])); err == nil {
			metrics.IdleConnections = idle
		}
		if total, err := strconv.Atoi(strings.TrimSpace(fields[2])); err == nil {
			metrics.TotalConnections = total
		}
	}

	// Get max connections
	cmd = exec.Command("kubectl", "exec", podName, "--", "psql", "-U", "postgres", "-d", "cold_db", "-t", "-c",
		"SHOW max_connections")
	output, _ = cmd.Output()
	if max, err := strconv.Atoi(strings.TrimSpace(string(output))); err == nil {
		metrics.MaxConnections = max
	}

	// Get replication lag (for replicas) using WAL bytes difference (accurate even when idle)
	if metrics.Role == "Replica" {
		cmd = exec.Command("kubectl", "exec", podName, "--", "psql", "-U", "postgres", "-d", "cold_db", "-t", "-c",
			`SELECT COALESCE(pg_wal_lsn_diff(pg_last_wal_receive_lsn(), pg_last_wal_replay_lsn()), 0)`)
		output, _ = cmd.Output()
		if lag, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64); err == nil {
			metrics.ReplicationLagSeconds = lag // Note: Now stores bytes, not seconds
		}
	}

	// Get cache hit ratio
	cmd = exec.Command("kubectl", "exec", podName, "--", "psql", "-U", "postgres", "-d", "cold_db", "-t", "-c",
		"SELECT COALESCE(ROUND(100.0 * sum(blks_hit) / NULLIF(sum(blks_hit) + sum(blks_read), 0), 2), 0) FROM pg_stat_database WHERE datname = 'cold_db'")
	output, _ = cmd.Output()
	if ratio, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64); err == nil {
		metrics.CacheHitRatio = ratio
	}

	// Get transaction stats
	cmd = exec.Command("kubectl", "exec", podName, "--", "psql", "-U", "postgres", "-d", "cold_db", "-t", "-c",
		"SELECT xact_commit, xact_rollback, blks_read, blks_hit FROM pg_stat_database WHERE datname = 'cold_db'")
	output, _ = cmd.Output()
	fields = strings.Split(strings.TrimSpace(string(output)), "|")
	if len(fields) >= 4 {
		if commits, err := strconv.ParseInt(strings.TrimSpace(fields[0]), 10, 64); err == nil {
			metrics.TransactionsCommitted = commits
		}
		if rollbacks, err := strconv.ParseInt(strings.TrimSpace(fields[1]), 10, 64); err == nil {
			metrics.TransactionsRolledBack = rollbacks
		}
		if reads, err := strconv.ParseInt(strings.TrimSpace(fields[2]), 10, 64); err == nil {
			metrics.BlocksRead = reads
		}
		if hits, err := strconv.ParseInt(strings.TrimSpace(fields[3]), 10, 64); err == nil {
			metrics.BlocksHit = hits
		}
	}
}

// collectVIPStatus checks VIP health and stores result
func (c *MetricsCollector) collectVIPStatus(ctx context.Context) {
	vip := "192.168.15.200"
	status := &models.VIPStatus{
		Time:       time.Now(),
		VIPAddress: vip,
	}

	start := time.Now()
	resp, err := c.httpClient.Get(fmt.Sprintf("http://%s:8080/health", vip))
	elapsed := time.Since(start)

	status.ResponseTimeMs = int(elapsed.Milliseconds())

	if err != nil {
		status.IsHealthy = false
		status.Message = fmt.Sprintf("Connection failed: %v", err)
	} else {
		defer resp.Body.Close()
		status.IsHealthy = resp.StatusCode == http.StatusOK
		if status.IsHealthy {
			status.Message = "VIP is healthy"
		} else {
			body, _ := io.ReadAll(resp.Body)
			status.Message = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
		}
	}

	// Store status
	if err := c.repo.InsertVIPStatus(ctx, status); err != nil {
		log.Printf("[MetricsCollector] Error storing VIP status: %v", err)
	}
}

// Helper functions

func parseCPU(cpu string) int {
	// Parse CPU string like "16" or "16000m"
	cpu = strings.TrimSpace(cpu)
	if strings.HasSuffix(cpu, "m") {
		if val, err := strconv.Atoi(strings.TrimSuffix(cpu, "m")); err == nil {
			return val / 1000
		}
	}
	if val, err := strconv.Atoi(cpu); err == nil {
		return val
	}
	return 0
}

func parseMemory(mem string) int64 {
	// Parse memory string like "8Gi", "8192Mi", "8589934592"
	mem = strings.TrimSpace(mem)

	multipliers := map[string]int64{
		"Ki": 1024,
		"Mi": 1024 * 1024,
		"Gi": 1024 * 1024 * 1024,
		"Ti": 1024 * 1024 * 1024 * 1024,
		"K":  1000,
		"M":  1000 * 1000,
		"G":  1000 * 1000 * 1000,
		"T":  1000 * 1000 * 1000 * 1000,
	}

	for suffix, mult := range multipliers {
		if strings.HasSuffix(mem, suffix) {
			if val, err := strconv.ParseFloat(strings.TrimSuffix(mem, suffix), 64); err == nil {
				return int64(val * float64(mult))
			}
		}
	}

	// Plain bytes
	if val, err := strconv.ParseInt(mem, 10, 64); err == nil {
		return val
	}

	return 0
}

// checkAndRecoverStuckPods checks for stuck PostgreSQL pods and auto-recovers them
func (c *MetricsCollector) checkAndRecoverStuckPods(ctx context.Context) {
	// Get detailed pod status including container statuses
	cmd := exec.Command("kubectl", "get", "pods", "-l", "cnpg.io/cluster=cold-postgres", "-o", "json")
	output, err := cmd.Output()
	if err != nil {
		return // K8s API not available, skip recovery check
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
				ContainerStatuses     []struct {
					Ready bool `json:"ready"`
				} `json:"containerStatuses"`
				InitContainerStatuses []struct {
					Ready bool `json:"ready"`
				} `json:"initContainerStatuses"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(output, &podData); err != nil {
		return
	}

	// Count healthy pods and find primary
	var healthyCount int
	var primaryPod string
	now := time.Now()

	c.podTracker.mu.Lock()
	defer c.podTracker.mu.Unlock()

	for _, pod := range podData.Items {
		// Skip init/join jobs
		if pod.Status.Phase == "Succeeded" || strings.Contains(pod.Metadata.Name, "-initdb") || strings.Contains(pod.Metadata.Name, "-join") {
			continue
		}

		// Check if primary
		if pod.Metadata.Labels["role"] == "primary" {
			primaryPod = pod.Metadata.Name
		}

		// Check if pod is healthy (Running + container ready)
		isHealthy := pod.Status.Phase == "Running" &&
			len(pod.Status.ContainerStatuses) > 0 &&
			pod.Status.ContainerStatuses[0].Ready

		// Check init containers (if present and not all ready, pod is stuck in init)
		initReady := true
		if len(pod.Status.InitContainerStatuses) > 0 {
			for _, initStatus := range pod.Status.InitContainerStatuses {
				if !initStatus.Ready {
					initReady = false
					break
				}
			}
		}

		isHealthy = isHealthy && initReady

		if isHealthy {
			healthyCount++
			// Clear any unhealthy tracking for this pod
			if state, exists := c.podTracker.podStates[pod.Metadata.Name]; exists {
				state.FirstSeenUnhealthy = nil
			}
			continue
		}

		// Pod is unhealthy - track it
		state, exists := c.podTracker.podStates[pod.Metadata.Name]
		if !exists {
			state = &PodState{
				Name: pod.Metadata.Name,
			}
			c.podTracker.podStates[pod.Metadata.Name] = state
		}

		state.Phase = pod.Status.Phase
		state.Ready = isHealthy
		state.InitReady = initReady

		// First time seeing this pod unhealthy?
		if state.FirstSeenUnhealthy == nil {
			state.FirstSeenUnhealthy = &now
			log.Printf("[AutoRecovery] Pod %s detected unhealthy (phase: %s, init: %v)",
				pod.Metadata.Name, pod.Status.Phase, initReady)
			continue
		}

		// Calculate how long pod has been unhealthy
		unhealthyDuration := now.Sub(*state.FirstSeenUnhealthy)

		// Determine threshold based on condition
		var threshold time.Duration
		if !initReady {
			threshold = 5 * time.Minute // Init:0/1 for > 5 min
		} else if pod.Status.Phase == "Pending" {
			threshold = 10 * time.Minute // Pending for > 10 min
		} else if pod.Status.Phase == "Failed" {
			threshold = 30 * time.Second // Failed - recover quickly
		} else {
			threshold = 5 * time.Minute // 0/1 Running for > 5 min
		}

		// Check if we should recover
		if unhealthyDuration < threshold {
			continue
		}

		// Check recovery backoff (5 min between attempts)
		if state.LastRecoveryTime != nil && now.Sub(*state.LastRecoveryTime) < 5*time.Minute {
			continue
		}

		// Max 3 recovery attempts per pod
		if state.RecoveryAttempts >= 3 {
			log.Printf("[AutoRecovery] Pod %s exceeded max recovery attempts (3), skipping", pod.Metadata.Name)
			continue
		}

		// Never delete primary
		if pod.Metadata.Name == primaryPod {
			log.Printf("[AutoRecovery] Skipping primary pod %s (would cause failover)", pod.Metadata.Name)
			continue
		}

		// Ensure at least one healthy pod remains
		if healthyCount < 1 {
			log.Printf("[AutoRecovery] No healthy pods, skipping recovery of %s to prevent data loss", pod.Metadata.Name)
			continue
		}

		// RECOVER: Delete the stuck pod
		log.Printf("[AutoRecovery] Recovering stuck pod %s (unhealthy for %v)",
			pod.Metadata.Name, unhealthyDuration.Round(time.Second))

		deleteCmd := exec.Command("kubectl", "delete", "pod", pod.Metadata.Name, "--force", "--grace-period=0")
		if err := deleteCmd.Run(); err != nil {
			log.Printf("[AutoRecovery] Failed to delete pod %s: %v", pod.Metadata.Name, err)
		} else {
			log.Printf("[AutoRecovery] Successfully deleted pod %s, CNPG will recreate it", pod.Metadata.Name)
			state.RecoveryAttempts++
			state.LastRecoveryTime = &now
			state.FirstSeenUnhealthy = nil
		}
	}
}

// SetRecoveryEnabled enables or disables auto-recovery
func (c *MetricsCollector) SetRecoveryEnabled(enabled bool) {
	c.recoveryEnabled = enabled
	log.Printf("[MetricsCollector] Auto-recovery %s", map[bool]string{true: "enabled", false: "disabled"}[enabled])
}

// IsRecoveryEnabled returns whether auto-recovery is enabled
func (c *MetricsCollector) IsRecoveryEnabled() bool {
	return c.recoveryEnabled
}
