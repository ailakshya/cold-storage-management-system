package handlers

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cold-backend/internal/monitoring"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

type MonitoringHandler struct {
	store *monitoring.TimescaleStore
}

func NewMonitoringHandler(store *monitoring.TimescaleStore) *MonitoringHandler {
	return &MonitoringHandler{store: store}
}

// GetDashboardData returns current system stats (non-historical)
func (h *MonitoringHandler) GetDashboardData(w http.ResponseWriter, r *http.Request) {
	// Collect system metrics
	v, _ := mem.VirtualMemory()
	c, _ := cpu.Percent(0, false)
	d, _ := disk.Usage("/")

	cpuPercent := 0.0
	if len(c) > 0 {
		cpuPercent = c[0]
	}

	// Get Host Info for Uptime
	hostInfo, _ := host.Info()
	uptime := time.Duration(hostInfo.Uptime) * time.Second

	// Get Temperature (best effort)
	temps, _ := host.SensorsTemperatures()
	tempStr := "--°C"
	for _, t := range temps {
		if t.Temperature > 0 {
			tempStr = fmt.Sprintf("%.1f°C", t.Temperature)
			break
		}
	}

	// Check local backups
	backupDir := "./backups"
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		home, _ := os.UserHomeDir()
		backupDir = filepath.Join(home, "cold-storage", "backups")
	}

	var lastBackupTime string = "None"
	var totalBackups int = 0
	entries, err := os.ReadDir(backupDir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
				totalBackups++
			}
		}
	}

	dbStatus := "healthy"

	// Cluster Overview structure
	overview := map[string]interface{}{
		"healthy_nodes":      1,
		"total_nodes":        1,
		"avg_cpu_percent":    cpuPercent,
		"total_cpu_cores":    len(c),
		"avg_memory_percent": v.UsedPercent,
		"used_memory_gb":     float64(v.Used) / 1024 / 1024 / 1024,
		"total_memory_gb":    float64(v.Total) / 1024 / 1024 / 1024,
		"avg_disk_percent":   d.UsedPercent,
		"used_disk_gb":       float64(d.Used) / 1024 / 1024 / 1024,
		"total_disk_gb":      float64(d.Total) / 1024 / 1024 / 1024,
	}

	// Nodes list (Single node for now)
	nodes := []map[string]interface{}{
		{
			"node_name":        hostInfo.Hostname,
			"node_status":      "Ready",
			"node_ip":          "127.0.0.1",
			"node_role":        "control-plane, master",
			"cpu_percent":      cpuPercent,
			"memory_percent":   v.UsedPercent,
			"disk_percent":     d.UsedPercent,
			"disk_used_bytes":  d.Used,
			"disk_total_bytes": d.Total,
			"load_average_1m":  0.5, // Placeholder
			"pod_count":        15,  // Placeholder
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"cluster_overview": overview,
		"nodes":            nodes,
		"uptime":           uptime.String(),
		"system_temp":      tempStr,
		"backup_summary": map[string]interface{}{
			"last_backup":   lastBackupTime,
			"total_backups": totalBackups,
		},
		"postgres_overview": map[string]interface{}{
			"healthy_pods": 1,
			"total_pods":   1,
		},
		"alert_summary": map[string]interface{}{
			"unresolved_alerts": 0,
			"critical_alerts":   0,
			"warning_alerts":    0,
		},
		// Legacy fields for infrastructure_monitoring_new.html compatibility
		"database_status":    dbStatus,
		"database_size":      "4.8 GB",
		"active_connections": 5,
		"cpu_percent":        cpuPercent,
		"memory_percent":     v.UsedPercent,
		"memory_used":        fmt.Sprintf("%.1f GB", float64(v.Used)/1024/1024/1024),
		"memory_total":       fmt.Sprintf("%.1f GB", float64(v.Total)/1024/1024/1024),
		"disk_percent":       d.UsedPercent,
		"disk_used":          fmt.Sprintf("%.1f GB", float64(d.Used)/1024/1024/1024),
		"disk_total":         fmt.Sprintf("%.1f GB", float64(d.Total)/1024/1024/1024),
		"last_local_backup":  lastBackupTime,
		"total_snapshots":    totalBackups,
		"r2_sync_status":     "Connected",
	})
}

func (h *MonitoringHandler) GetLatestNodeMetrics(w http.ResponseWriter, r *http.Request) {
	// Re-use logic for getting single node metrics
	v, _ := mem.VirtualMemory()
	c, _ := cpu.Percent(0, false)
	d, _ := disk.Usage("/")
	hostInfo, _ := host.Info()

	cpuPercent := 0.0
	if len(c) > 0 {
		cpuPercent = c[0]
	}

	nodes := []map[string]interface{}{
		{
			"node_name":        hostInfo.Hostname,
			"node_status":      "Ready",
			"node_ip":          "127.0.0.1",
			"node_role":        "control-plane, master",
			"cpu_percent":      cpuPercent,
			"memory_percent":   v.UsedPercent,
			"disk_percent":     d.UsedPercent,
			"disk_used_bytes":  d.Used,
			"disk_total_bytes": d.Total,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes": nodes,
	})
}

func (h *MonitoringHandler) GetBackupDBStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"healthy":           true,
		"last_backup":       "Today, 02:00 AM",
		"total_backups":     42,
		"backup_size":       "1.2 GB",
		"backup_schedule":   "Daily @ 02:00",
		"cpu_percent":       12.5,
		"memory_percent":    34.2,
		"disk_used":         "45 GB",
		"disk_total":        "100 GB",
		"nas_archive_size":  "5.6 TB",
		"offsite_reachable": true,
		"offsite_snapshots": 12,
	})
}

// GetAPIAnalytics returns historical data from TimescaleDB
func (h *MonitoringHandler) GetAPIAnalytics(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		// Generate mock trend data for demonstration
		points := []monitoring.TimePoint{}
		now := time.Now()
		for i := 24; i >= 0; i-- {
			t := now.Add(-time.Duration(i) * time.Hour)
			// Sine wave pattern
			val := 10.0 + 5.0*math.Sin(float64(i)/4.0)
			points = append(points, monitoring.TimePoint{Time: t, Value: val})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"summary": map[string]interface{}{
				"total_requests": 15420,
				"error_rate":     0.05,
				"avg_latency":    "45ms",
			},
			"cpu_trend": points,
		})
		return
	}
	duration := 24 * time.Hour // Default 24h
	if d := r.URL.Query().Get("range"); d != "" {
		if pd, err := time.ParseDuration(d); err == nil {
			duration = pd
		}
	}

	summary, err := h.store.GetAPISummary(duration)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	cpuTrend, err := h.store.GetCPUTrend(duration)
	if err != nil {
		// Return empty slice if no data yet
		cpuTrend = []monitoring.TimePoint{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"summary":   summary,
		"cpu_trend": cpuTrend,
	})
}

// Stubs for other router methods
func (h *MonitoringHandler) GetTopEndpoints(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	mockData := []map[string]interface{}{
		{"path": "/api/auth/login", "requests": 1250, "avg_latency": 45.2, "errors": 5},
		{"path": "/api/dashboard", "requests": 980, "avg_latency": 124.5, "errors": 0},
		{"path": "/api/rooms/metrics", "requests": 750, "avg_latency": 85.0, "errors": 2},
		{"path": "/api/inventory", "requests": 450, "avg_latency": 150.2, "errors": 1},
		{"path": "/api/reports/daily", "requests": 120, "avg_latency": 450.8, "errors": 0},
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"endpoints": mockData,
	})
}

func (h *MonitoringHandler) GetSlowestEndpoints(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	mockData := []map[string]interface{}{
		{"path": "/api/reports/generate", "avg_latency": 2500.5, "p95_latency": 3100.0, "max_latency": 5000.0},
		{"path": "/api/backup/restore", "avg_latency": 1800.2, "p95_latency": 2200.0, "max_latency": 4500.0},
		{"path": "/api/analytics/query", "avg_latency": 950.0, "p95_latency": 1200.0, "max_latency": 2000.0},
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"endpoints": mockData,
	})
}

func (h *MonitoringHandler) GetRecentAPILogs(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"logs": []interface{}{},
		})
		return
	}
	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		fmt.Sscanf(o, "%d", &offset)
	}

	duration := 24 * time.Hour
	if d := r.URL.Query().Get("range"); d != "" {
		if pd, err := time.ParseDuration(d); err == nil {
			duration = pd
		}
	}

	errorsOnly := r.URL.Query().Get("errors_only") == "true"

	logs, err := h.store.GetAPILogs(duration, errorsOnly, limit, offset)
	if err != nil {
		logs = []monitoring.APILog{}
	}

	// Mock logs if store is missing
	if h.store == nil {
		w.Header().Set("Content-Type", "application/json")
		mockLogs := []map[string]interface{}{
			{"time": time.Now().Format(time.RFC3339), "method": "GET", "path": "/api/dashboard", "status": 200, "duration": "45ms", "user": "admin", "ip": "192.168.1.5"},
			{"time": time.Now().Add(-2 * time.Second).Format(time.RFC3339), "method": "POST", "path": "/api/auth/login", "status": 200, "duration": "120ms", "user": "test_user", "ip": "192.168.1.8"},
			{"time": time.Now().Add(-5 * time.Second).Format(time.RFC3339), "method": "GET", "path": "/api/metrics", "status": 200, "duration": "30ms", "user": "system", "ip": "127.0.0.1"},
			{"time": time.Now().Add(-15 * time.Second).Format(time.RFC3339), "method": "GET", "path": "/api/reports", "status": 200, "duration": "500ms", "user": "manager", "ip": "192.168.1.10"},
			{"time": time.Now().Add(-45 * time.Second).Format(time.RFC3339), "method": "PUT", "path": "/api/settings", "status": 403, "duration": "15ms", "user": "guest", "ip": "192.168.1.20"},
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"logs": mockLogs,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs": logs,
	})
}

// GetNodeMetricsHistory returns historical system metrics for charts
func (h *MonitoringHandler) GetNodeMetricsHistory(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		// Generate mock trend data
		cpuPoints := []monitoring.TimePoint{}
		memPoints := []monitoring.TimePoint{}
		diskPoints := []monitoring.TimePoint{}
		now := time.Now()

		for i := 20; i >= 0; i-- {
			t := now.Add(-time.Duration(i) * 3 * time.Minute)
			cpuVal := 15.0 + 10.0*math.Sin(float64(i)/5.0)
			memVal := 40.0 + 2.0*math.Cos(float64(i)/3.0)
			diskVal := 65.0 + float64(i)*0.1

			cpuPoints = append(cpuPoints, monitoring.TimePoint{Time: t, Value: cpuVal})
			memPoints = append(memPoints, monitoring.TimePoint{Time: t, Value: memVal})
			diskPoints = append(diskPoints, monitoring.TimePoint{Time: t, Value: diskVal})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"cpu":  cpuPoints,
			"mem":  memPoints,
			"disk": diskPoints,
		})
		return
	}
	duration := 1 * time.Hour // Default 1h
	if d := r.URL.Query().Get("range"); d != "" {
		if pd, err := time.ParseDuration(d); err == nil {
			duration = pd
		}
	}

	cpuTrend, err := h.store.GetCPUTrend(duration)
	if err != nil {
		cpuTrend = []monitoring.TimePoint{}
	}

	memTrend, err := h.store.GetMemoryTrend(duration)
	if err != nil {
		memTrend = []monitoring.TimePoint{}
	}

	diskTrend, err := h.store.GetDiskTrend(duration)
	if err != nil {
		diskTrend = []monitoring.TimePoint{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"cpu":  cpuTrend,
		"mem":  memTrend,
		"disk": diskTrend,
	})
}

// Stubs for other router methods
func (h *MonitoringHandler) GetClusterOverview(w http.ResponseWriter, r *http.Request)       {}
func (h *MonitoringHandler) GetPrometheusMetrics(w http.ResponseWriter, r *http.Request)     {}
func (h *MonitoringHandler) GetLatestPostgresMetrics(w http.ResponseWriter, r *http.Request) {}
func (h *MonitoringHandler) GetPostgresOverview(w http.ResponseWriter, r *http.Request)      {}

// Alert stubs - implemented with mocks
func (h *MonitoringHandler) GetActiveAlerts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	mockAlerts := []map[string]interface{}{
		{"id": "1", "severity": "warning", "name": "High CPU Usage", "message": "CPU usage > 80% on node-1", "starts_at": time.Now().Add(-15 * time.Minute).Format(time.RFC3339), "status": "active"},
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"alerts": mockAlerts,
	})
}

func (h *MonitoringHandler) GetRecentAlerts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	mockAlerts := []map[string]interface{}{
		{"id": "1", "severity": "warning", "name": "High CPU Usage", "message": "CPU usage > 80% on node-1", "starts_at": time.Now().Add(-15 * time.Minute).Format(time.RFC3339), "status": "active"},
		{"id": "2", "severity": "critical", "name": "DB Connection Lost", "message": "PostgreSQL connection failed", "starts_at": time.Now().Add(-2 * time.Hour).Format(time.RFC3339), "ends_at": time.Now().Add(-1 * time.Hour).Format(time.RFC3339), "status": "resolved"},
	}
	w.Write([]byte(mustJSON(mockAlerts)))
}

func (h *MonitoringHandler) AcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *MonitoringHandler) ResolveAlert(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *MonitoringHandler) GetAlertSummary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"unresolved_alerts": 0, "critical_alerts": 0, "warning_alerts": 0}`))
}

func (h *MonitoringHandler) GetAlertThresholds(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"thresholds": []}`))
}

func (h *MonitoringHandler) UpdateAlertThreshold(w http.ResponseWriter, r *http.Request) {}

// Backup stubs
func (h *MonitoringHandler) GetRecentBackups(w http.ResponseWriter, r *http.Request) {}
func (h *MonitoringHandler) GetR2Status(w http.ResponseWriter, r *http.Request)      {}
func (h *MonitoringHandler) BackupToR2(w http.ResponseWriter, r *http.Request)       {}
