-- Migration 031: Create Node Metrics Table
-- Stores K3s node resource metrics for monitoring

CREATE TABLE IF NOT EXISTS node_metrics (
    id BIGSERIAL PRIMARY KEY,
    node_name VARCHAR(100) NOT NULL,
    node_ip VARCHAR(45) NOT NULL,
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
    pod_count INTEGER,
    node_status VARCHAR(20) DEFAULT 'Ready',
    node_role VARCHAR(50),
    load_average_1m DECIMAL(6,2),
    load_average_5m DECIMAL(6,2),
    load_average_15m DECIMAL(6,2),
    collected_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_node_metrics_node_time ON node_metrics(node_name, collected_at DESC);
CREATE INDEX IF NOT EXISTS idx_node_metrics_collected_at ON node_metrics(collected_at DESC);
CREATE INDEX IF NOT EXISTS idx_node_metrics_node_ip ON node_metrics(node_ip);
