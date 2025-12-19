-- Migration 033: Create Alert Thresholds Table
-- Configurable thresholds for monitoring alerts

CREATE TABLE IF NOT EXISTS alert_thresholds (
    id SERIAL PRIMARY KEY,
    metric_name VARCHAR(100) NOT NULL UNIQUE,
    warning_threshold DECIMAL(15,4),
    critical_threshold DECIMAL(15,4),
    enabled BOOLEAN DEFAULT TRUE,
    cooldown_minutes INTEGER DEFAULT 5,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert default thresholds
INSERT INTO alert_thresholds (metric_name, warning_threshold, critical_threshold, description) VALUES
    ('api_error_rate', 5.0, 10.0, 'API error rate percentage'),
    ('api_response_time_avg', 500, 1000, 'Average API response time in milliseconds'),
    ('api_requests_per_minute', 1000, 2000, 'API requests per minute threshold'),
    ('node_cpu_percent', 70, 90, 'Node CPU utilization percentage'),
    ('node_memory_percent', 75, 90, 'Node memory utilization percentage'),
    ('node_disk_percent', 80, 95, 'Node disk utilization percentage'),
    ('node_load_average', 4.0, 8.0, 'Node load average (1 minute)'),
    ('postgres_connections', 150, 180, 'PostgreSQL active connections'),
    ('postgres_replication_lag', 10, 30, 'PostgreSQL replication lag in seconds')
ON CONFLICT (metric_name) DO NOTHING;
