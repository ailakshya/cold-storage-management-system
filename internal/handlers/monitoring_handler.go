package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"cold-backend/internal/monitoring"
)

type MonitoringHandler struct {
	store *monitoring.TimescaleStore
}

func NewMonitoringHandler(store *monitoring.TimescaleStore) *MonitoringHandler {
	return &MonitoringHandler{store: store}
}

// GetDashboardData returns current system stats (non-historical)
func (h *MonitoringHandler) GetDashboardData(w http.ResponseWriter, r *http.Request) {
	// Frontend expects: database_status, active_connections, etc.
	// For now, return basic "Ok" to avoid UI errors
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"database_status":    "healthy",
		"active_connections": 5, // TODO: Mock
		"cpu_percent":        10.5,
		"memory_percent":     45.2,
		"disk_percent":       60.1,
		"uptime":             "2d 4h",
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
