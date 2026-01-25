package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

type MonitoringServer struct {
	db         *pgxpool.Pool
	port       int
	alerts     []Alert
	alertsMux  sync.RWMutex
	clients    map[*websocket.Conn]bool
	clientsMux sync.Mutex
	broadcast  chan Alert
}

type Alert struct {
	ID        int       `json:"id"`
	Severity  string    `json:"severity"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Resolved  bool      `json:"resolved"`
}

type DashboardStats struct {
	DatabaseStatus    string       `json:"database_status"`
	ActiveConnections int          `json:"active_connections"`
	ResponseTime      int64        `json:"response_time_ms"`
	ActiveAlerts      int          `json:"active_alerts"`
	RequestRate       int          `json:"request_rate"`
	CPUPercent        float64      `json:"cpu_percent"`
	MemoryPercent     float64      `json:"memory_percent"`
	DiskPercent       float64      `json:"disk_percent"`
	TotalRequests     int64        `json:"total_requests"`
	SuccessRate       float64      `json:"success_rate"`
	AvgResponse       int64        `json:"avg_response"`
	ErrorRate         int          `json:"error_rate"`
	QueriesPerSec     int          `json:"queries_per_sec"`
	DBSize            string       `json:"db_size"`
	Uptime            string       `json:"uptime"`
	MemoryUsed        string       `json:"memory_used"`
	MemoryTotal       string       `json:"memory_total"`
	DiskUsed          string       `json:"disk_used"`
	DiskTotal         string       `json:"disk_total"`
	Nodes             []NodeStats  `json:"nodes"`
	ClusterTotals     ClusterStats `json:"cluster_totals"`
	DatabasePods      []DBPodStats `json:"database_pods"`
}

type NodeStats struct {
	Name          string  `json:"name"`
	Role          string  `json:"role"`
	Status        string  `json:"status"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsed    string  `json:"memory_used"`
	MemoryTotal   string  `json:"memory_total"`
	MemoryPercent float64 `json:"memory_percent"`
	DiskUsed      string  `json:"disk_used"`
	DiskTotal     string  `json:"disk_total"`
	DiskPercent   float64 `json:"disk_percent"`
	PodsRunning   int     `json:"pods_running"`
}

type ClusterStats struct {
	TotalNodes    int     `json:"total_nodes"`
	TotalCPUs     int     `json:"total_cpus"`
	AvgCPUPercent float64 `json:"avg_cpu_percent"`
	TotalMemory   string  `json:"total_memory"`
	UsedMemory    string  `json:"used_memory"`
	TotalDisk     string  `json:"total_disk"`
	UsedDisk      string  `json:"used_disk"`
	TotalPods     int     `json:"total_pods"`
}

type DBPodStats struct {
	Name           string `json:"name"`
	Role           string `json:"role"`
	Status         string `json:"status"`
	Node           string `json:"node"`
	DiskUsed       string `json:"disk_used"`
	DiskTotal      string `json:"disk_total"`
	Connections    int    `json:"connections"`
	ReplicationLag string `json:"replication_lag"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func NewMonitoringServer(db *pgxpool.Pool, port int) *MonitoringServer {
	return &MonitoringServer{
		db:        db,
		port:      port,
		alerts:    make([]Alert, 0),
		clients:   make(map[*websocket.Conn]bool),
		broadcast: make(chan Alert),
	}
}

func (ms *MonitoringServer) Start() {
	r := mux.NewRouter()

	// Dashboard page
	r.HandleFunc("/", ms.dashboardPage).Methods("GET")

	// API endpoints
	r.HandleFunc("/api/stats", ms.getStats).Methods("GET")
	r.HandleFunc("/api/alerts", ms.getAlerts).Methods("GET")
	r.HandleFunc("/api/test-alert", ms.createTestAlert).Methods("POST")

	// WebSocket for real-time updates
	r.HandleFunc("/ws", ms.handleWebSocket)

	// Serve static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Start background alert broadcaster
	go ms.handleBroadcast()

	// Start background health checker
	go ms.monitorHealth()

	addr := fmt.Sprintf(":%d", ms.port)
	log.Printf("Monitoring dashboard running on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

func (ms *MonitoringServer) dashboardPage(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/monitoring_dashboard.html"))
	tmpl.Execute(w, nil)
}

func (ms *MonitoringServer) getStats(w http.ResponseWriter, r *http.Request) {
	stats := ms.collectStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (ms *MonitoringServer) collectStats() DashboardStats {
	// Check database
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	err := ms.db.Ping(ctx)
	responseTime := time.Since(start).Milliseconds()

	dbStatus := "healthy"
	if err != nil {
		dbStatus = "unhealthy"
	}

	// Get active connections
	var activeConns int
	ms.db.QueryRow(ctx, "SELECT count(*) FROM pg_stat_activity").Scan(&activeConns)

	// Get database size
	var dbSizeBytes int64
	ms.db.QueryRow(ctx, "SELECT pg_database_size(current_database())").Scan(&dbSizeBytes)
	dbSize := fmt.Sprintf("%.2f GB", float64(dbSizeBytes)/(1024*1024*1024))

	// Get database uptime
	var uptimeSec int
	ms.db.QueryRow(ctx, "SELECT EXTRACT(EPOCH FROM (NOW() - pg_postmaster_start_time()))::int").Scan(&uptimeSec)
	uptime := formatUptime(uptimeSec)

	// System metrics (current pod/node)
	cpuPercents, _ := cpu.Percent(time.Second, false)
	cpuPercent := 0.0
	if len(cpuPercents) > 0 {
		cpuPercent = cpuPercents[0]
	}

	memStats, _ := mem.VirtualMemory()
	memPercent := memStats.UsedPercent
	memUsed := formatBytes(memStats.Used)
	memTotal := formatBytes(memStats.Total)

	diskStats, _ := disk.Usage("/")
	diskPercent := diskStats.UsedPercent
	diskUsed := formatBytes(diskStats.Used)
	diskTotal := formatBytes(diskStats.Total)

	// Collect K3s node metrics
	nodes := ms.collectNodeMetrics()

	// Calculate cluster totals
	clusterTotals := ms.calculateClusterTotals(nodes)

	// Collect database pod metrics
	dbPods := ms.collectDatabasePods(ctx)

	// Count alerts
	ms.alertsMux.RLock()
	activeAlertCount := 0
	for _, alert := range ms.alerts {
		if !alert.Resolved {
			activeAlertCount++
		}
	}
	ms.alertsMux.RUnlock()

	return DashboardStats{
		DatabaseStatus:    dbStatus,
		ActiveConnections: activeConns,
		ResponseTime:      responseTime,
		ActiveAlerts:      activeAlertCount,
		RequestRate:       0,
		CPUPercent:        cpuPercent,
		MemoryPercent:     memPercent,
		DiskPercent:       diskPercent,
		TotalRequests:     0,
		SuccessRate:       99.9,
		AvgResponse:       responseTime,
		ErrorRate:         0,
		QueriesPerSec:     0,
		DBSize:            dbSize,
		Uptime:            uptime,
		MemoryUsed:        memUsed,
		MemoryTotal:       memTotal,
		DiskUsed:          diskUsed,
		DiskTotal:         diskTotal,
		Nodes:             nodes,
		ClusterTotals:     clusterTotals,
		DatabasePods:      dbPods,
	}
}

func formatBytes(bytes uint64) string {
	gb := float64(bytes) / (1024 * 1024 * 1024)
	if gb < 1 {
		mb := float64(bytes) / (1024 * 1024)
		return fmt.Sprintf("%.1f MB", mb)
	}
	return fmt.Sprintf("%.1f GB", gb)
}

func (ms *MonitoringServer) collectNodeMetrics() []NodeStats {
	// Single-node production setup
	// Query system metrics for the current node
	cpuPercents, _ := cpu.Percent(time.Second, false)
	cpuPercent := 0.0
	if len(cpuPercents) > 0 {
		cpuPercent = cpuPercents[0]
	}

	memStats, _ := mem.VirtualMemory()
	diskStats, _ := disk.Usage("/")

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "cold-production"
	}

	nodes := []NodeStats{
		{
			Name:          hostname,
			Role:          "Production Server",
			Status:        "Ready",
			CPUPercent:    cpuPercent,
			MemoryUsed:    formatBytes(memStats.Used),
			MemoryTotal:   formatBytes(memStats.Total),
			MemoryPercent: memStats.UsedPercent,
			DiskUsed:      formatBytes(diskStats.Used),
			DiskTotal:     formatBytes(diskStats.Total),
			DiskPercent:   diskStats.UsedPercent,
			PodsRunning:   4, // Approximate: Employee, Customer, Cloudflared pods
		},
	}

	return nodes
}

func (ms *MonitoringServer) calculateClusterTotals(nodes []NodeStats) ClusterStats {
	if len(nodes) == 0 {
		return ClusterStats{}
	}

	// For single node, use the node's actual metrics
	node := nodes[0]

	// Get CPU count
	cpuCounts, _ := cpu.Counts(true)
	if cpuCounts == 0 {
		cpuCounts = 1
	}

	return ClusterStats{
		TotalNodes:    1,
		TotalCPUs:     cpuCounts,
		AvgCPUPercent: node.CPUPercent,
		TotalMemory:   node.MemoryTotal,
		UsedMemory:    node.MemoryUsed,
		TotalDisk:     node.DiskTotal,
		UsedDisk:      node.DiskUsed,
		TotalPods:     node.PodsRunning,
	}
}

func (ms *MonitoringServer) collectDatabasePods(ctx context.Context) []DBPodStats {
	// Query actual database stats for single-node setup
	var connections int
	ms.db.QueryRow(ctx, "SELECT count(*) FROM pg_stat_activity").Scan(&connections)

	var dbSizeBytes int64
	ms.db.QueryRow(ctx, "SELECT pg_database_size(current_database())").Scan(&dbSizeBytes)

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "cold-production"
	}

	// Single PostgreSQL instance running on host machine
	pods := []DBPodStats{
		{
			Name:           "PostgreSQL-14 (Host)",
			Role:           "Primary",
			Status:         "Running",
			Node:           hostname,
			DiskUsed:       formatBytes(uint64(dbSizeBytes)),
			DiskTotal:      "100 GB", // Estimate based on disk allocation
			Connections:    connections,
			ReplicationLag: "N/A (Single Node)",
		},
	}

	return pods
}

func formatUptime(seconds int) string {
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func (ms *MonitoringServer) getAlerts(w http.ResponseWriter, r *http.Request) {
	ms.alertsMux.RLock()
	defer ms.alertsMux.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ms.alerts)
}

func (ms *MonitoringServer) createTestAlert(w http.ResponseWriter, r *http.Request) {
	var alert Alert
	if err := json.NewDecoder(r.Body).Decode(&alert); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ms.alertsMux.Lock()
	alert.ID = len(ms.alerts) + 1
	alert.Timestamp = time.Now()
	ms.alerts = append(ms.alerts, alert)
	ms.alertsMux.Unlock()

	// Broadcast to all WebSocket clients
	ms.broadcast <- alert

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(alert)
}

func (ms *MonitoringServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()

	ms.clientsMux.Lock()
	ms.clients[conn] = true
	ms.clientsMux.Unlock()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			ms.clientsMux.Lock()
			delete(ms.clients, conn)
			ms.clientsMux.Unlock()
			break
		}
	}
}

func (ms *MonitoringServer) handleBroadcast() {
	for alert := range ms.broadcast {
		ms.clientsMux.Lock()
		for client := range ms.clients {
			err := client.WriteJSON(alert)
			if err != nil {
				client.Close()
				delete(ms.clients, client)
			}
		}
		ms.clientsMux.Unlock()
	}
}

func (ms *MonitoringServer) monitorHealth() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stats := ms.collectStats()

		// Create alerts based on conditions
		if stats.DatabaseStatus == "unhealthy" {
			alert := Alert{
				Severity:  "critical",
				Type:      "database_down",
				Message:   "Database is unreachable",
				Timestamp: time.Now(),
				Resolved:  false,
			}

			ms.alertsMux.Lock()
			alert.ID = len(ms.alerts) + 1
			ms.alerts = append(ms.alerts, alert)
			ms.alertsMux.Unlock()

			ms.broadcast <- alert
		}

		if stats.ResponseTime > 1000 {
			alert := Alert{
				Severity:  "warning",
				Type:      "high_latency",
				Message:   fmt.Sprintf("Database response time: %dms", stats.ResponseTime),
				Timestamp: time.Now(),
				Resolved:  false,
			}

			ms.alertsMux.Lock()
			alert.ID = len(ms.alerts) + 1
			ms.alerts = append(ms.alerts, alert)
			ms.alertsMux.Unlock()

			ms.broadcast <- alert
		}
	}
}
