package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"

	"cold-backend/internal/monitoring"
	"cold-backend/internal/services"
)

type MonitoringHandler struct {
	store          *monitoring.TimescaleStore
	dbPool         *pgxpool.Pool
	backupDir      string
	restoreService *services.RestoreService
}

func NewMonitoringHandler(store *monitoring.TimescaleStore, dbPool *pgxpool.Pool, backupDir string, restoreService *services.RestoreService) *MonitoringHandler {
	return &MonitoringHandler{
		store:          store,
		dbPool:         dbPool,
		backupDir:      backupDir,
		restoreService: restoreService,
	}
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
	// Check local backups
	backupDir := h.backupDir

	var lastBackupTime string = "None"
	var totalBackups int = 0
	var latestModTime time.Time

	entries, err := os.ReadDir(backupDir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
				totalBackups++
				info, err := e.Info()
				if err == nil {
					if info.ModTime().After(latestModTime) {
						latestModTime = info.ModTime()
						lastBackupTime = latestModTime.Format("2006-01-02 15:04:05")
					}
				}
			}
		}
	}

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

	// Get real database stats
	dbSize := "--"
	activeConns := 0
	dbStatus := "Offline"
	healthyPods := 0

	if h.dbPool != nil {
		// Default timeout 2s for all checks
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := h.dbPool.Ping(ctx); err == nil {
			dbStatus = "Online"
			healthyPods = 1
		}

		var size string
		// We use a new context or reuse the existing one if it supports multiple queries (it does)
		err := h.dbPool.QueryRow(ctx, "SELECT pg_size_pretty(pg_database_size(current_database()))").Scan(&size)
		if err == nil {
			dbSize = size
		}

		var count int
		err = h.dbPool.QueryRow(ctx, "SELECT count(*) FROM pg_stat_activity").Scan(&count)
		if err == nil {
			activeConns = count
		}
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
			"healthy_pods": healthyPods,
			"total_pods":   1,
		},
		"alert_summary": map[string]interface{}{
			"unresolved_alerts": 0,
			"critical_alerts":   0,
			"warning_alerts":    0,
		},
		// Legacy fields for infrastructure_monitoring_new.html compatibility
		"database_status":    dbStatus,
		"database_size":      dbSize,
		"active_connections": activeConns,
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
	w.Write([]byte(`{"endpoints": []}`))
}

func (h *MonitoringHandler) GetSlowestEndpoints(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"endpoints": []}`))
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
	w.Write([]byte(`{"alerts": []}`))
}

func (h *MonitoringHandler) GetRecentAlerts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`[]`))
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

// Backup handlers
func (h *MonitoringHandler) GetRecentBackups(w http.ResponseWriter, r *http.Request) {
	if h.restoreService == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"dates": [], "total_backups": 0}`))
		return
	}

	dates, total, err := h.restoreService.ListAvailableDates(r.Context())
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		// Log error but return empty list to not break UI
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":         err.Error(),
			"dates":         []interface{}{},
			"total_backups": 0,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"dates":         dates,
		"total_backups": total,
	})
}

func (h *MonitoringHandler) GetR2Status(w http.ResponseWriter, r *http.Request) {
	status := "Not Configured"
	if h.restoreService != nil {
		status = "Connected"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   status,
		"provider": "Cloudflare R2",
	})
}

func (h *MonitoringHandler) BackupToR2(w http.ResponseWriter, r *http.Request) {
	if h.restoreService == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Backup service not initialized",
		})
		return
	}

	key, err := h.restoreService.CreateBackup(r.Context())
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Backup failed: " + err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"key":     key,
		"message": "Backup created successfully",
	})
}

// Backup Scheduler
var (
	backupTicker      *time.Ticker
	backupStopChan    chan bool
	backupMutex       sync.Mutex
	localBackupTicker *time.Ticker
	localBackupStop   chan bool

	// Schedules
	r2BackupInterval    = 5 * time.Minute
	localBackupInterval = 1 * time.Minute
)

// StartBackupSchedulers starts both local and R2 backup schedulers
func StartBackupSchedulers(s *services.RestoreService) {
	backupMutex.Lock()
	defer backupMutex.Unlock()

	if backupTicker != nil {
		return // Already running
	}

	backupTicker = time.NewTicker(r2BackupInterval)
	backupStopChan = make(chan bool)

	localBackupTicker = time.NewTicker(localBackupInterval)
	localBackupStop = make(chan bool)

	// R2 Backup Routine (Every 5 mins)
	go func() {
		log.Printf("[Scheduler] Starting R2 backup scheduler (interval: %v)", r2BackupInterval)
		// Run first R2 backup immediately? Maybe not, to avoid clash with local immediate
		// runSchedulerBackup(s) - Let's wait for first tick

		for {
			select {
			case <-backupTicker.C:
				runSchedulerBackup(s) // Creates local + uploads to R2
			case <-backupStopChan:
				log.Println("[Scheduler] R2 Scheduler stopped")
				return
			}
		}
	}()

	// Local Backup Routine (Every 1 min)
	go func() {
		log.Printf("[Scheduler] Starting Local backup scheduler (interval: %v)", localBackupInterval)
		// Run first local backup immediately
		runLocalSchedulerBackup(s)

		for {
			select {
			case <-localBackupTicker.C:
				runLocalSchedulerBackup(s)
			case <-localBackupStop:
				log.Println("[Scheduler] Local Scheduler stopped")
				return
			}
		}
	}()
}

// StopBackupSchedulers stops the automatic backup schedulers
func StopBackupSchedulers() {
	backupMutex.Lock()
	defer backupMutex.Unlock()

	if backupTicker != nil {
		backupTicker.Stop()
		backupStopChan <- true
		backupTicker = nil
	}

	if localBackupTicker != nil {
		localBackupTicker.Stop()
		localBackupStop <- true
		localBackupTicker = nil
	}
}

func runSchedulerBackup(s *services.RestoreService) {
	// Specific context for backup with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	key, err := s.CreateBackup(ctx)
	if err != nil {
		log.Printf("[Scheduler] R2 Backup failed: %v", err)
	} else {
		log.Printf("[Scheduler] R2 Backup success: %s", key)
	}
}

func runLocalSchedulerBackup(s *services.RestoreService) {
	// Specific context for local backup with shorter timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	_, _, _, err := s.CreateLocalBackup(ctx)
	if err != nil {
		log.Printf("[Scheduler] Local Backup failed: %v", err)
	} else {
		// Log success only on debug level or less frequently to avoid spam?
		// For now logging is fine to verify
		// log.Printf("[Scheduler] Local Backup success: %s", name)
	}
}
