-- External PostgreSQL servers and deployment automation

-- External PostgreSQL servers table (for managing standalone PostgreSQL instances)
CREATE TABLE IF NOT EXISTS external_databases (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    ip_address VARCHAR(45) NOT NULL,
    port INTEGER DEFAULT 5432,
    db_name VARCHAR(100) DEFAULT 'cold_db',
    db_user VARCHAR(100) DEFAULT 'postgres',
    role VARCHAR(30) DEFAULT 'replica', -- primary, replica, standby, backup
    status VARCHAR(30) DEFAULT 'unknown', -- healthy, degraded, failed, unknown
    replication_source_id INTEGER REFERENCES external_databases(id), -- For replicas
    ssh_user VARCHAR(50) DEFAULT 'root',
    ssh_port INTEGER DEFAULT 22,
    pg_version VARCHAR(20),
    connection_count INTEGER,
    replication_lag_seconds FLOAT,
    disk_usage_percent INTEGER,
    last_backup_at TIMESTAMP,
    last_checked_at TIMESTAMP,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(ip_address, port)
);

-- Deployment configuration table
CREATE TABLE IF NOT EXISTS deployment_config (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    image_repo VARCHAR(255) NOT NULL, -- e.g., lakshyajaat/cold-backend
    current_version VARCHAR(50),
    deployment_name VARCHAR(100), -- K8s deployment name
    namespace VARCHAR(100) DEFAULT 'default',
    replicas INTEGER DEFAULT 2,
    build_command TEXT, -- e.g., CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server
    build_context VARCHAR(255), -- Path to build context
    docker_file VARCHAR(255) DEFAULT 'Dockerfile',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Deployment history table
CREATE TABLE IF NOT EXISTS deployment_history (
    id SERIAL PRIMARY KEY,
    deployment_id INTEGER REFERENCES deployment_config(id),
    version VARCHAR(50) NOT NULL,
    previous_version VARCHAR(50),
    deployed_by INTEGER REFERENCES users(id),
    status VARCHAR(30) DEFAULT 'pending', -- pending, building, deploying, success, failed, rolledback
    build_output TEXT,
    deploy_output TEXT,
    error_message TEXT,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_external_databases_status ON external_databases(status);
CREATE INDEX IF NOT EXISTS idx_external_databases_role ON external_databases(role);
CREATE INDEX IF NOT EXISTS idx_deployment_history_deployment ON deployment_history(deployment_id);
CREATE INDEX IF NOT EXISTS idx_deployment_history_status ON deployment_history(status);

-- Insert default deployment configurations
INSERT INTO deployment_config (name, image_repo, deployment_name, build_command, build_context) VALUES
    ('cold-backend-employee', 'lakshyajaat/cold-backend', 'cold-backend-employee',
     'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server ./cmd/server',
     '/home/lakshya/jupyter-/cold/cold-backend'),
    ('cold-backend-customer', 'lakshyajaat/cold-backend', 'cold-backend-customer',
     'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server ./cmd/server',
     '/home/lakshya/jupyter-/cold/cold-backend')
ON CONFLICT (name) DO NOTHING;

-- Add external database config keys
INSERT INTO infra_config (key, value, description) VALUES
    ('docker_registry', 'lakshyajaat', 'Docker Hub registry/username'),
    ('docker_push_enabled', 'false', 'Push to Docker Hub after build'),
    ('deploy_nodes', '192.168.15.110,192.168.15.111,192.168.15.112,192.168.15.113,192.168.15.114', 'K3s nodes for image distribution')
ON CONFLICT (key) DO NOTHING;
