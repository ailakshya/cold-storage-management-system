package models

import (
	"time"
)

// APIRequestLog represents a single API request log entry
type APIRequestLog struct {
	Time         time.Time `json:"time" db:"time"`
	RequestID    string    `json:"request_id" db:"request_id"`
	Method       string    `json:"method" db:"method"`
	Path         string    `json:"path" db:"path"`
	StatusCode   int       `json:"status_code" db:"status_code"`
	DurationMs   float64   `json:"duration_ms" db:"duration_ms"`
	RequestSize  int       `json:"request_size" db:"request_size"`
	ResponseSize int       `json:"response_size" db:"response_size"`
	UserID       *int      `json:"user_id,omitempty" db:"user_id"`
	UserEmail    *string   `json:"user_email,omitempty" db:"user_email"`
	UserRole     *string   `json:"user_role,omitempty" db:"user_role"`
	IPAddress    string    `json:"ip_address" db:"ip_address"`
	UserAgent    string    `json:"user_agent" db:"user_agent"`
	ErrorMessage *string   `json:"error_message,omitempty" db:"error_message"`
}

// NodeMetrics represents K3s node resource metrics
type NodeMetrics struct {
	Time            time.Time `json:"time" db:"time"`
	NodeName        string    `json:"node_name" db:"node_name"`
	NodeIP          string    `json:"node_ip" db:"node_ip"`
	NodeRole        string    `json:"node_role" db:"node_role"`
	NodeStatus      string    `json:"node_status" db:"node_status"`
	CPUPercent      float64   `json:"cpu_percent" db:"cpu_percent"`
	CPUCores        int       `json:"cpu_cores" db:"cpu_cores"`
	MemoryUsedBytes int64     `json:"memory_used_bytes" db:"memory_used_bytes"`
	MemoryTotalBytes int64    `json:"memory_total_bytes" db:"memory_total_bytes"`
	MemoryPercent   float64   `json:"memory_percent" db:"memory_percent"`
	DiskUsedBytes   int64     `json:"disk_used_bytes" db:"disk_used_bytes"`
	DiskTotalBytes  int64     `json:"disk_total_bytes" db:"disk_total_bytes"`
	DiskPercent     float64   `json:"disk_percent" db:"disk_percent"`
	NetworkRxBytes  int64     `json:"network_rx_bytes" db:"network_rx_bytes"`
	NetworkTxBytes  int64     `json:"network_tx_bytes" db:"network_tx_bytes"`
	NetworkRxRate   int64     `json:"network_rx_rate" db:"network_rx_rate"`
	NetworkTxRate   int64     `json:"network_tx_rate" db:"network_tx_rate"`
	PodCount        int       `json:"pod_count" db:"pod_count"`
	LoadAverage1m   float64   `json:"load_average_1m" db:"load_average_1m"`
	LoadAverage5m   float64   `json:"load_average_5m" db:"load_average_5m"`
	LoadAverage15m  float64   `json:"load_average_15m" db:"load_average_15m"`
}

// PostgresMetrics represents PostgreSQL pod metrics
type PostgresMetrics struct {
	Time                  time.Time `json:"time" db:"time"`
	PodName               string    `json:"pod_name" db:"pod_name"`
	NodeName              string    `json:"node_name" db:"node_name"`
	Role                  string    `json:"role" db:"role"`
	Status                string    `json:"status" db:"status"`
	DatabaseSizeBytes     int64     `json:"database_size_bytes" db:"database_size_bytes"`
	ActiveConnections     int       `json:"active_connections" db:"active_connections"`
	IdleConnections       int       `json:"idle_connections" db:"idle_connections"`
	TotalConnections      int       `json:"total_connections" db:"total_connections"`
	MaxConnections        int       `json:"max_connections" db:"max_connections"`
	ReplicationLagSeconds float64   `json:"replication_lag_seconds" db:"replication_lag_seconds"`
	TransactionsCommitted int64     `json:"transactions_committed" db:"transactions_committed"`
	TransactionsRolledBack int64    `json:"transactions_rolled_back" db:"transactions_rolled_back"`
	BlocksRead            int64     `json:"blocks_read" db:"blocks_read"`
	BlocksHit             int64     `json:"blocks_hit" db:"blocks_hit"`
	CacheHitRatio         float64   `json:"cache_hit_ratio" db:"cache_hit_ratio"`
}

// MonitoringAlert represents a system alert
type MonitoringAlert struct {
	ID             int        `json:"id" db:"id"`
	Time           time.Time  `json:"time" db:"time"`
	AlertType      string     `json:"alert_type" db:"alert_type"`
	Severity       string     `json:"severity" db:"severity"`
	Source         string     `json:"source" db:"source"`
	Title          string     `json:"title" db:"title"`
	Message        string     `json:"message" db:"message"`
	MetricName     *string    `json:"metric_name,omitempty" db:"metric_name"`
	MetricValue    *float64   `json:"metric_value,omitempty" db:"metric_value"`
	ThresholdValue *float64   `json:"threshold_value,omitempty" db:"threshold_value"`
	NodeName       *string    `json:"node_name,omitempty" db:"node_name"`
	Acknowledged   bool       `json:"acknowledged" db:"acknowledged"`
	AcknowledgedBy *string    `json:"acknowledged_by,omitempty" db:"acknowledged_by"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty" db:"acknowledged_at"`
	Resolved       bool       `json:"resolved" db:"resolved"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty" db:"resolved_at"`
}

// AlertThreshold represents configurable alert thresholds
type AlertThreshold struct {
	ID               int       `json:"id" db:"id"`
	MetricName       string    `json:"metric_name" db:"metric_name"`
	DisplayName      string    `json:"display_name" db:"display_name"`
	WarningThreshold float64   `json:"warning_threshold" db:"warning_threshold"`
	CriticalThreshold float64  `json:"critical_threshold" db:"critical_threshold"`
	Comparison       string    `json:"comparison" db:"comparison"`
	Enabled          bool      `json:"enabled" db:"enabled"`
	CooldownMinutes  int       `json:"cooldown_minutes" db:"cooldown_minutes"`
	Description      string    `json:"description" db:"description"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

// BackupHistory represents backup operation history
type BackupHistory struct {
	Time            time.Time          `json:"time" db:"time"`
	BackupType      string             `json:"backup_type" db:"backup_type"`
	Status          string             `json:"status" db:"status"`
	Source          string             `json:"source" db:"source"`
	Destination     string             `json:"destination" db:"destination"`
	SizeBytes       int64              `json:"size_bytes" db:"size_bytes"`
	DurationSeconds int                `json:"duration_seconds" db:"duration_seconds"`
	ErrorMessage    *string            `json:"error_message,omitempty" db:"error_message"`
	Metadata        map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
}

// VIPStatus represents VIP health check history
type VIPStatus struct {
	Time          time.Time `json:"time" db:"time"`
	VIPAddress    string    `json:"vip_address" db:"vip_address"`
	IsHealthy     bool      `json:"is_healthy" db:"is_healthy"`
	ResponseTimeMs int      `json:"response_time_ms" db:"response_time_ms"`
	ActiveNode    string    `json:"active_node" db:"active_node"`
	Message       string    `json:"message" db:"message"`
}

// ======================================
// Aggregated/Summary Types
// ======================================

// APIAnalytics represents aggregated API statistics
type APIAnalytics struct {
	TimeRange       string  `json:"time_range"`
	TotalRequests   int64   `json:"total_requests"`
	SuccessRequests int64   `json:"success_requests"`
	ErrorRequests   int64   `json:"error_requests"`
	ErrorRate       float64 `json:"error_rate"`
	AvgDurationMs   float64 `json:"avg_duration_ms"`
	P95DurationMs   float64 `json:"p95_duration_ms"`
	MaxDurationMs   float64 `json:"max_duration_ms"`
	TotalRequestBytes  int64 `json:"total_request_bytes"`
	TotalResponseBytes int64 `json:"total_response_bytes"`
	UniqueUsers     int     `json:"unique_users"`
	TopEndpoints    []EndpointStats `json:"top_endpoints"`
	ErrorsByStatus  map[int]int64   `json:"errors_by_status"`
}

// EndpointStats represents per-endpoint statistics
type EndpointStats struct {
	Path          string  `json:"path"`
	Method        string  `json:"method,omitempty"`
	TotalRequests int64   `json:"total_requests"`
	ErrorCount    int64   `json:"error_count"`
	AvgDurationMs float64 `json:"avg_duration_ms"`
	P95DurationMs float64 `json:"p95_duration_ms"`
	MaxDurationMs float64 `json:"max_duration_ms"`
}

// ClusterOverview represents aggregated cluster statistics
type ClusterOverview struct {
	TotalNodes      int     `json:"total_nodes"`
	HealthyNodes    int     `json:"healthy_nodes"`
	TotalCPUCores   int     `json:"total_cpu_cores"`
	AvgCPUPercent   float64 `json:"avg_cpu_percent"`
	TotalMemoryGB   float64 `json:"total_memory_gb"`
	UsedMemoryGB    float64 `json:"used_memory_gb"`
	AvgMemoryPercent float64 `json:"avg_memory_percent"`
	TotalDiskGB     float64 `json:"total_disk_gb"`
	UsedDiskGB      float64 `json:"used_disk_gb"`
	AvgDiskPercent  float64 `json:"avg_disk_percent"`
	TotalPods       int     `json:"total_pods"`
	AvgLoadAverage  float64 `json:"avg_load_average"`
	LastUpdated     time.Time `json:"last_updated"`
}

// PostgresOverview represents aggregated PostgreSQL cluster statistics
type PostgresOverview struct {
	TotalPods          int       `json:"total_pods"`
	HealthyPods        int       `json:"healthy_pods"`
	PrimaryPod         string    `json:"primary_pod"`
	TotalDatabaseSizeGB float64  `json:"total_database_size_gb"`
	TotalConnections   int       `json:"total_connections"`
	MaxReplicationLag  float64   `json:"max_replication_lag"`
	AvgCacheHitRatio   float64   `json:"avg_cache_hit_ratio"`
	LastUpdated        time.Time `json:"last_updated"`
}

// TimeSeriesPoint represents a single point in time series data
type TimeSeriesPoint struct {
	Time  time.Time `json:"time"`
	Value float64   `json:"value"`
}

// NodeTimeSeries represents time series data for a node
type NodeTimeSeries struct {
	NodeName string            `json:"node_name"`
	CPU      []TimeSeriesPoint `json:"cpu"`
	Memory   []TimeSeriesPoint `json:"memory"`
	Disk     []TimeSeriesPoint `json:"disk"`
	Load     []TimeSeriesPoint `json:"load"`
}

// AlertSummary represents alert statistics
type AlertSummary struct {
	TotalAlerts      int `json:"total_alerts"`
	CriticalAlerts   int `json:"critical_alerts"`
	WarningAlerts    int `json:"warning_alerts"`
	InfoAlerts       int `json:"info_alerts"`
	UnacknowledgedAlerts int `json:"unacknowledged_alerts"`
	UnresolvedAlerts int `json:"unresolved_alerts"`
}

// MonitoringDashboardData represents all data for the monitoring dashboard
type MonitoringDashboardData struct {
	ClusterOverview  ClusterOverview   `json:"cluster_overview"`
	PostgresOverview PostgresOverview  `json:"postgres_overview"`
	APIAnalytics     APIAnalytics      `json:"api_analytics"`
	AlertSummary     AlertSummary      `json:"alert_summary"`
	RecentAlerts     []MonitoringAlert `json:"recent_alerts"`
	Nodes            []NodeMetrics     `json:"nodes"`
	PostgresPods     []PostgresMetrics `json:"postgres_pods"`
	LastUpdated      time.Time         `json:"last_updated"`
}
