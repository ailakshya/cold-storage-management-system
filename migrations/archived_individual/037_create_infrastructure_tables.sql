-- Infrastructure management tables for cluster nodes and configuration

-- Infrastructure configuration table (flexible key-value store)
CREATE TABLE IF NOT EXISTS infra_config (
    id SERIAL PRIMARY KEY,
    key VARCHAR(100) UNIQUE NOT NULL,
    value TEXT NOT NULL,
    description TEXT,
    is_secret BOOLEAN DEFAULT FALSE,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Cluster nodes table
CREATE TABLE IF NOT EXISTS cluster_nodes (
    id SERIAL PRIMARY KEY,
    ip_address VARCHAR(45) UNIQUE NOT NULL, -- Supports IPv6
    hostname VARCHAR(100),
    role VARCHAR(20) DEFAULT 'worker', -- control-plane, worker, backup
    status VARCHAR(30) DEFAULT 'pending', -- pending, connecting, installing, joining, ready, failed, removed
    ssh_user VARCHAR(50) DEFAULT 'root',
    ssh_port INTEGER DEFAULT 22,
    ssh_key_id INTEGER, -- Reference to stored key (if using key storage)
    k3s_version VARCHAR(20),
    os_info VARCHAR(100),
    cpu_cores INTEGER,
    memory_mb INTEGER,
    disk_gb INTEGER,
    last_seen_at TIMESTAMP,
    provisioned_at TIMESTAMP,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- SSH keys table (for secure key storage)
CREATE TABLE IF NOT EXISTS ssh_keys (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    public_key TEXT NOT NULL,
    private_key_path VARCHAR(255), -- Path on server, not the key itself
    fingerprint VARCHAR(100),
    is_default BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Node provisioning logs
CREATE TABLE IF NOT EXISTS node_provision_logs (
    id SERIAL PRIMARY KEY,
    node_id INTEGER REFERENCES cluster_nodes(id) ON DELETE CASCADE,
    step VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL, -- running, success, failed
    message TEXT,
    output TEXT,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    finished_at TIMESTAMP
);

-- Infrastructure action audit log
CREATE TABLE IF NOT EXISTS infra_action_logs (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    action VARCHAR(100) NOT NULL, -- add_node, remove_node, reboot_node, drain_node, etc.
    target_type VARCHAR(50), -- node, cluster, database
    target_id VARCHAR(100), -- IP or identifier
    details JSONB,
    status VARCHAR(20) DEFAULT 'success', -- success, failed
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_cluster_nodes_status ON cluster_nodes(status);
CREATE INDEX IF NOT EXISTS idx_cluster_nodes_role ON cluster_nodes(role);
CREATE INDEX IF NOT EXISTS idx_node_provision_logs_node ON node_provision_logs(node_id);
CREATE INDEX IF NOT EXISTS idx_infra_action_logs_user ON infra_action_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_infra_action_logs_action ON infra_action_logs(action);
CREATE INDEX IF NOT EXISTS idx_infra_config_key ON infra_config(key);

-- Insert default configuration values
INSERT INTO infra_config (key, value, description) VALUES
    ('cluster_vip', '', 'Cluster VIP address'),
    ('offsite_db_host', '', 'Offsite database IP address'),
    ('offsite_db_port', '5434', 'Offsite database port'),
    ('offsite_metrics_port', '9100', 'Node exporter port on offsite server'),
    ('nas_mount_path', '', 'NAS backup mount path'),
    ('k3s_server_url', '', 'K3s server URL (https://ip:6443)'),
    ('k3s_token', '', 'K3s cluster join token'),
    ('default_ssh_user', 'root', 'Default SSH user for new nodes'),
    ('node_exporter_version', '1.7.0', 'Node exporter version to install')
ON CONFLICT (key) DO NOTHING;
