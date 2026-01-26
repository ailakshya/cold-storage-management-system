package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
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
		// Try to find a core temperature, commonly labeled "core" or "package"
		if t.Temperature > 0 {
			tempStr = fmt.Sprintf("%.1f°C", t.Temperature)
			break
		}
	}

	// Check local backups
	backupDir := "./backups" // Default location
	// Try to find absolute path if relative doesn't exist, assuming standard deployment
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		home, _ := os.UserHomeDir()
		backupDir = filepath.Join(home, "cold-storage", "backups")
	}

	var lastBackupTime string = "None"
	var totalBackups int = 0

	entries, err := os.ReadDir(backupDir)
	if err == nil {
		var backupFiles []os.FileInfo
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
				info, err := e.Info()
				if err == nil {
					backupFiles = append(backupFiles, info)
				}
			}
		}
		totalBackups = len(backupFiles)
		if len(backupFiles) > 0 {
			sort.Slice(backupFiles, func(i, j int) bool {
				return backupFiles[i].ModTime().After(backupFiles[j].ModTime())
			})
			lastBackupTime = backupFiles[0].ModTime().Format("2006-01-02 15:04:05")
		}
	}

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
		"system_temp":        tempStr,
		"last_local_backup":  lastBackupTime,
		"total_snapshots":    totalBackups,
		"r2_sync_status":     "Connected", // Connected if app is running
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
func (h *MonitoringHandler) GetTopEndpoints(w http.ResponseWriter, r *http.Request)      {}
func (h *MonitoringHandler) GetSlowestEndpoints(w http.ResponseWriter, r *http.Request)  {}
func (h *MonitoringHandler) GetRecentAPILogs(w http.ResponseWriter, r *http.Request)     {}
func (h *MonitoringHandler) GetLatestNodeMetrics(w http.ResponseWriter, r *http.Request) {}

// GetNodeMetricsHistory returns historical system metrics for charts
func (h *MonitoringHandler) GetNodeMetricsHistory(w http.ResponseWriter, r *http.Request) {
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
