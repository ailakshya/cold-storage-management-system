-- TimescaleDB Schema for Cold Storage Monitoring
-- Database: metrics_db
-- Creates hypertables for time-series data with automatic compression

-- Enable TimescaleDB extension
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- =====================================================
-- API Request Logs
-- =====================================================
CREATE TABLE IF NOT EXISTS api_request_logs (
    time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    request_id UUID DEFAULT gen_random_uuid(),
    method VARCHAR(10) NOT NULL,
    path VARCHAR(500) NOT NULL,
    status_code INTEGER NOT NULL,
    duration_ms DECIMAL(10,2) NOT NULL,
    request_size INTEGER DEFAULT 0,
    response_size INTEGER DEFAULT 0,
    user_id INTEGER,
    user_email VARCHAR(255),
    user_role VARCHAR(50),
    ip_address VARCHAR(45),
    user_agent TEXT,
    error_message TEXT
);

-- Convert to hypertable (partitioned by time)
SELECT create_hypertable('api_request_logs', 'time', if_not_exists => TRUE);

-- Compression policy (compress data older than 7 days)
ALTER TABLE api_request_logs SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'path'
);
SELECT add_compression_policy('api_request_logs', INTERVAL '7 days', if_not_exists => TRUE);

-- Retention policy (delete data older than 90 days)
SELECT add_retention_policy('api_request_logs', INTERVAL '90 days', if_not_exists => TRUE);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_api_logs_path ON api_request_logs (path, time DESC);
CREATE INDEX IF NOT EXISTS idx_api_logs_status ON api_request_logs (status_code, time DESC);
CREATE INDEX IF NOT EXISTS idx_api_logs_user ON api_request_logs (user_id, time DESC);

-- =====================================================
-- K3s Node Metrics
-- =====================================================
CREATE TABLE IF NOT EXISTS node_metrics (
    time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    node_name VARCHAR(100) NOT NULL,
    node_ip VARCHAR(45) NOT NULL,
    node_role VARCHAR(50),
    node_status VARCHAR(20) DEFAULT 'Ready',
    cpu_percent DECIMAL(5,2),
    cpu_cores INTEGER,
    memory_used_bytes BIGINT,
    memory_total_bytes BIGINT,
    memory_percent DECIMAL(5,2),
    disk_used_bytes BIGINT,
    disk_total_bytes BIGINT,
    disk_percent DECIMAL(5,2),
    network_rx_bytes BIGINT,
    network_tx_bytes BIGINT,
    network_rx_rate BIGINT,  -- bytes per second
    network_tx_rate BIGINT,  -- bytes per second
    pod_count INTEGER,
    load_average_1m DECIMAL(6,2),
    load_average_5m DECIMAL(6,2),
    load_average_15m DECIMAL(6,2)
);

SELECT create_hypertable('node_metrics', 'time', if_not_exists => TRUE);

ALTER TABLE node_metrics SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'node_name'
);
SELECT add_compression_policy('node_metrics', INTERVAL '7 days', if_not_exists => TRUE);
SELECT add_retention_policy('node_metrics', INTERVAL '30 days', if_not_exists => TRUE);

CREATE INDEX IF NOT EXISTS idx_node_metrics_node ON node_metrics (node_name, time DESC);

-- =====================================================
-- PostgreSQL Metrics
-- =====================================================
CREATE TABLE IF NOT EXISTS postgres_metrics (
    time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    pod_name VARCHAR(100) NOT NULL,
    node_name VARCHAR(100),
    role VARCHAR(20) NOT NULL,  -- 'Primary' or 'Replica'
    status VARCHAR(20) NOT NULL,
    database_size_bytes BIGINT,
    active_connections INTEGER,
    idle_connections INTEGER,
    total_connections INTEGER,
    max_connections INTEGER,
    replication_lag_seconds DECIMAL(10,3),
    transactions_committed BIGINT,
    transactions_rolled_back BIGINT,
    blocks_read BIGINT,
    blocks_hit BIGINT,
    cache_hit_ratio DECIMAL(5,2)
);

SELECT create_hypertable('postgres_metrics', 'time', if_not_exists => TRUE);

ALTER TABLE postgres_metrics SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'pod_name'
);
SELECT add_compression_policy('postgres_metrics', INTERVAL '7 days', if_not_exists => TRUE);
SELECT add_retention_policy('postgres_metrics', INTERVAL '30 days', if_not_exists => TRUE);

CREATE INDEX IF NOT EXISTS idx_postgres_metrics_pod ON postgres_metrics (pod_name, time DESC);

-- =====================================================
-- System Alerts
-- =====================================================
CREATE TABLE IF NOT EXISTS monitoring_alerts (
    id SERIAL,
    time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    alert_type VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL CHECK (severity IN ('info', 'warning', 'critical')),
    source VARCHAR(100) NOT NULL,
    title VARCHAR(200) NOT NULL,
    message TEXT NOT NULL,
    metric_name VARCHAR(100),
    metric_value DECIMAL(15,4),
    threshold_value DECIMAL(15,4),
    node_name VARCHAR(100),
    acknowledged BOOLEAN DEFAULT FALSE,
    acknowledged_by VARCHAR(255),
    acknowledged_at TIMESTAMPTZ,
    resolved BOOLEAN DEFAULT FALSE,
    resolved_at TIMESTAMPTZ,
    PRIMARY KEY (id, time)
);

SELECT create_hypertable('monitoring_alerts', 'time', if_not_exists => TRUE);
SELECT add_retention_policy('monitoring_alerts', INTERVAL '180 days', if_not_exists => TRUE);

CREATE INDEX IF NOT EXISTS idx_alerts_severity ON monitoring_alerts (severity, time DESC);
CREATE INDEX IF NOT EXISTS idx_alerts_active ON monitoring_alerts (resolved, time DESC);

-- =====================================================
-- Alert Thresholds Configuration
-- =====================================================
CREATE TABLE IF NOT EXISTS alert_thresholds (
    id SERIAL PRIMARY KEY,
    metric_name VARCHAR(100) NOT NULL UNIQUE,
    display_name VARCHAR(200),
    warning_threshold DECIMAL(15,4),
    critical_threshold DECIMAL(15,4),
    comparison VARCHAR(10) DEFAULT 'gt',  -- 'gt' (greater than), 'lt' (less than)
    enabled BOOLEAN DEFAULT TRUE,
    cooldown_minutes INTEGER DEFAULT 5,
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Insert default thresholds
INSERT INTO alert_thresholds (metric_name, display_name, warning_threshold, critical_threshold, comparison, description) VALUES
    ('api_error_rate', 'API Error Rate (%)', 5.0, 10.0, 'gt', 'Percentage of API requests returning errors'),
    ('api_response_time_p95', 'API Response Time P95 (ms)', 500, 1000, 'gt', '95th percentile API response time'),
    ('api_requests_per_minute', 'API Requests/min', 2000, 5000, 'gt', 'API request rate threshold'),
    ('node_cpu_percent', 'Node CPU Usage (%)', 70, 90, 'gt', 'CPU utilization per node'),
    ('node_memory_percent', 'Node Memory Usage (%)', 75, 90, 'gt', 'Memory utilization per node'),
    ('node_disk_percent', 'Node Disk Usage (%)', 80, 95, 'gt', 'Disk utilization per node'),
    ('node_load_average', 'Node Load Average (1m)', 8.0, 16.0, 'gt', '1-minute load average'),
    ('postgres_connections', 'PostgreSQL Connections', 150, 180, 'gt', 'Active database connections'),
    ('postgres_replication_lag', 'Replication Lag (sec)', 10, 30, 'gt', 'PostgreSQL replication delay'),
    ('postgres_cache_hit_ratio', 'Cache Hit Ratio (%)', 90, 80, 'lt', 'Database cache efficiency')
ON CONFLICT (metric_name) DO NOTHING;

-- =====================================================
-- Backup Status History
-- =====================================================
CREATE TABLE IF NOT EXISTS backup_history (
    time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    backup_type VARCHAR(50) NOT NULL,  -- 'postgres', 'full', 'incremental'
    status VARCHAR(20) NOT NULL,  -- 'success', 'failed', 'in_progress'
    source VARCHAR(100),
    destination VARCHAR(255),
    size_bytes BIGINT,
    duration_seconds INTEGER,
    error_message TEXT,
    metadata JSONB
);

SELECT create_hypertable('backup_history', 'time', if_not_exists => TRUE);
SELECT add_retention_policy('backup_history', INTERVAL '365 days', if_not_exists => TRUE);

-- =====================================================
-- VIP Status History
-- =====================================================
CREATE TABLE IF NOT EXISTS vip_status (
    time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    vip_address VARCHAR(45) NOT NULL,
    is_healthy BOOLEAN NOT NULL,
    response_time_ms INTEGER,
    active_node VARCHAR(100),
    message TEXT
);

SELECT create_hypertable('vip_status', 'time', if_not_exists => TRUE);
SELECT add_retention_policy('vip_status', INTERVAL '30 days', if_not_exists => TRUE);

-- =====================================================
-- Continuous Aggregates for Fast Queries
-- =====================================================

-- API Stats per hour
CREATE MATERIALIZED VIEW IF NOT EXISTS api_stats_hourly
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', time) AS bucket,
    path,
    COUNT(*) AS total_requests,
    COUNT(*) FILTER (WHERE status_code >= 400) AS error_count,
    AVG(duration_ms) AS avg_duration,
    PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_ms) AS p95_duration,
    MAX(duration_ms) AS max_duration,
    SUM(request_size) AS total_request_bytes,
    SUM(response_size) AS total_response_bytes
FROM api_request_logs
GROUP BY bucket, path
WITH NO DATA;

SELECT add_continuous_aggregate_policy('api_stats_hourly',
    start_offset => INTERVAL '3 hours',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour',
    if_not_exists => TRUE);

-- Node metrics per 5 minutes
CREATE MATERIALIZED VIEW IF NOT EXISTS node_metrics_5min
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('5 minutes', time) AS bucket,
    node_name,
    AVG(cpu_percent) AS avg_cpu,
    MAX(cpu_percent) AS max_cpu,
    AVG(memory_percent) AS avg_memory,
    MAX(memory_percent) AS max_memory,
    AVG(disk_percent) AS avg_disk,
    AVG(pod_count) AS avg_pods,
    AVG(load_average_1m) AS avg_load
FROM node_metrics
GROUP BY bucket, node_name
WITH NO DATA;

SELECT add_continuous_aggregate_policy('node_metrics_5min',
    start_offset => INTERVAL '15 minutes',
    end_offset => INTERVAL '5 minutes',
    schedule_interval => INTERVAL '5 minutes',
    if_not_exists => TRUE);

-- =====================================================
-- Helper Functions
-- =====================================================

-- Function to get API error rate for last N minutes
CREATE OR REPLACE FUNCTION get_api_error_rate(minutes INTEGER DEFAULT 5)
RETURNS DECIMAL AS $$
DECLARE
    total_count BIGINT;
    error_count BIGINT;
BEGIN
    SELECT COUNT(*), COUNT(*) FILTER (WHERE status_code >= 400)
    INTO total_count, error_count
    FROM api_request_logs
    WHERE time > NOW() - (minutes || ' minutes')::INTERVAL;

    IF total_count = 0 THEN
        RETURN 0;
    END IF;

    RETURN (error_count::DECIMAL / total_count::DECIMAL) * 100;
END;
$$ LANGUAGE plpgsql;

-- Function to get latest metrics for all nodes
CREATE OR REPLACE FUNCTION get_latest_node_metrics()
RETURNS TABLE (
    node_name VARCHAR(100),
    node_ip VARCHAR(45),
    node_status VARCHAR(20),
    cpu_percent DECIMAL(5,2),
    memory_percent DECIMAL(5,2),
    disk_percent DECIMAL(5,2),
    pod_count INTEGER,
    last_updated TIMESTAMPTZ
) AS $$
BEGIN
    RETURN QUERY
    SELECT DISTINCT ON (nm.node_name)
        nm.node_name,
        nm.node_ip,
        nm.node_status,
        nm.cpu_percent,
        nm.memory_percent,
        nm.disk_percent,
        nm.pod_count,
        nm.time AS last_updated
    FROM node_metrics nm
    WHERE nm.time > NOW() - INTERVAL '5 minutes'
    ORDER BY nm.node_name, nm.time DESC;
END;
$$ LANGUAGE plpgsql;

-- Grant permissions
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO metrics;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO metrics;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO metrics;
