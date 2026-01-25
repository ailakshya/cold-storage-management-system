package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
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

	// Simple DB check (can be improved with ping)
	dbStatus := "healthy"
	// if err := h.store.Ping(); err != nil { dbStatus = "unhealthy" }

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"database_status":    dbStatus,
		"active_connections": 5, // Placeholder until we query DB
		"cpu_percent":        cpuPercent,
		"memory_percent":     v.UsedPercent,
		"memory_used":        fmt.Sprintf("%.1f GB", float64(v.Used)/1024/1024/1024),
		"memory_total":       fmt.Sprintf("%.1f GB", float64(v.Total)/1024/1024/1024),
		"disk_percent":       d.UsedPercent,
		"disk_used":          fmt.Sprintf("%.1f GB", float64(d.Used)/1024/1024/1024),
		"disk_total":         fmt.Sprintf("%.1f GB", float64(d.Total)/1024/1024/1024),
		"uptime":             uptime.String(),
	})
}

// GetAPIAnalytics returns historical data from TimescaleDB
func (h *MonitoringHandler) GetAPIAnalytics(w http.ResponseWriter, r *http.Request) {
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
func (h *MonitoringHandler) GetTopEndpoints(w http.ResponseWriter, r *http.Request)          {}
func (h *MonitoringHandler) GetSlowestEndpoints(w http.ResponseWriter, r *http.Request)      {}
func (h *MonitoringHandler) GetRecentAPILogs(w http.ResponseWriter, r *http.Request)         {}
func (h *MonitoringHandler) GetLatestNodeMetrics(w http.ResponseWriter, r *http.Request)     {}
func (h *MonitoringHandler) GetNodeMetricsHistory(w http.ResponseWriter, r *http.Request)    {}
func (h *MonitoringHandler) GetClusterOverview(w http.ResponseWriter, r *http.Request)       {}
func (h *MonitoringHandler) GetPrometheusMetrics(w http.ResponseWriter, r *http.Request)     {}
func (h *MonitoringHandler) GetLatestPostgresMetrics(w http.ResponseWriter, r *http.Request) {}
func (h *MonitoringHandler) GetPostgresOverview(w http.ResponseWriter, r *http.Request)      {}

// Alert stubs
func (h *MonitoringHandler) GetActiveAlerts(w http.ResponseWriter, r *http.Request)      {}
func (h *MonitoringHandler) GetRecentAlerts(w http.ResponseWriter, r *http.Request)      {}
func (h *MonitoringHandler) AcknowledgeAlert(w http.ResponseWriter, r *http.Request)     {}
func (h *MonitoringHandler) ResolveAlert(w http.ResponseWriter, r *http.Request)         {}
func (h *MonitoringHandler) GetAlertSummary(w http.ResponseWriter, r *http.Request)      {}
func (h *MonitoringHandler) GetAlertThresholds(w http.ResponseWriter, r *http.Request)   {}
func (h *MonitoringHandler) UpdateAlertThreshold(w http.ResponseWriter, r *http.Request) {}

// Backup stubs
func (h *MonitoringHandler) GetRecentBackups(w http.ResponseWriter, r *http.Request)  {}
func (h *MonitoringHandler) GetBackupDBStatus(w http.ResponseWriter, r *http.Request) {}
func (h *MonitoringHandler) GetR2Status(w http.ResponseWriter, r *http.Request)       {}
func (h *MonitoringHandler) BackupToR2(w http.ResponseWriter, r *http.Request)        {}
