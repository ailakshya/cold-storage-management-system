# POC Deployment Guide - Cold Storage Management System

## Overview

This guide documents the complete POC (Proof of Concept) deployment setup for the Cold Storage Management System with High Availability.

**POC Environment:**
- VM 230 (coldstore-prod1): 192.168.15.230 - Primary DB + K3s Master + App
- VM 231 (coldstore-prod2): 192.168.15.231 - Standby DB + K3s Agent + App

---

## Table of Contents

1. [Infrastructure Setup](#1-infrastructure-setup)
2. [PostgreSQL HA Configuration](#2-postgresql-ha-configuration)
3. [K3s Cluster Setup](#3-k3s-cluster-setup)
4. [Monitoring Stack](#4-monitoring-stack)
5. [Application Deployment](#5-application-deployment)
6. [R2 Backup Configuration](#6-r2-backup-configuration)
7. [Code Changes Made](#7-code-changes-made)
8. [Maintenance Commands](#8-maintenance-commands)

---

## 1. Infrastructure Setup

### 1.1 VM Creation (Proxmox)

Create two VMs with the following specs:

| VM | Name | IP | RAM | CPU | Disk |
|----|------|-----|-----|-----|------|
| 230 | coldstore-prod1 | 192.168.15.230 | 8 GB | 8 cores | 100 GB |
| 231 | coldstore-prod2 | 192.168.15.231 | 8 GB | 8 cores | 100 GB |

**Proxmox Commands:**
```bash
# Download Ubuntu 24.04 cloud image
cd /var/lib/vz/template/iso
wget https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img

# Create VM 230 (Primary)
qm create 230 --name coldstore-prod1 --memory 8192 --cores 8 --net0 virtio,bridge=vmbr0
qm importdisk 230 noble-server-cloudimg-amd64.img local-lvm
qm set 230 --scsihw virtio-scsi-pci --scsi0 local-lvm:vm-230-disk-0
qm set 230 --ide2 local-lvm:cloudinit
qm set 230 --boot c --bootdisk scsi0
qm set 230 --serial0 socket --vga serial0
qm set 230 --ipconfig0 ip=192.168.15.230/24,gw=192.168.15.1
qm set 230 --ciuser root --sshkeys /path/to/ssh_key.pub
qm resize 230 scsi0 100G
qm start 230

# Create VM 231 (Standby) - same process with different IP
qm create 231 --name coldstore-prod2 --memory 8192 --cores 8 --net0 virtio,bridge=vmbr0
qm importdisk 231 noble-server-cloudimg-amd64.img local-lvm
qm set 231 --scsihw virtio-scsi-pci --scsi0 local-lvm:vm-231-disk-0
qm set 231 --ide2 local-lvm:cloudinit
qm set 231 --boot c --bootdisk scsi0
qm set 231 --serial0 socket --vga serial0
qm set 231 --ipconfig0 ip=192.168.15.231/24,gw=192.168.15.1
qm set 231 --ciuser root --sshkeys /path/to/ssh_key.pub
qm resize 231 scsi0 100G
qm start 231
```

### 1.2 Base System Setup (Both VMs)

```bash
# Update system
apt-get update && apt-get upgrade -y

# Install essential packages
apt-get install -y curl wget gnupg2 lsb-release qemu-guest-agent

# Enable guest agent
systemctl enable --now qemu-guest-agent
```

---

## 2. PostgreSQL HA Configuration

### 2.1 Install PostgreSQL 16 + TimescaleDB (Both VMs)

```bash
# Install PostgreSQL 16
apt-get install -y postgresql postgresql-contrib

# Add TimescaleDB repository
echo "deb https://packagecloud.io/timescale/timescaledb/ubuntu/ $(lsb_release -cs) main" | tee /etc/apt/sources.list.d/timescaledb.list
curl -L https://packagecloud.io/timescale/timescaledb/gpgkey | apt-key add -
apt-get update

# Install TimescaleDB
apt-get install -y timescaledb-2-postgresql-16

# Run TimescaleDB tuner
timescaledb-tune --quiet --yes

# Restart PostgreSQL
systemctl restart postgresql
```

### 2.2 Configure Primary (VM 230)

**Edit `/etc/postgresql/16/main/postgresql.conf`:**
```conf
listen_addresses = '*'
wal_level = replica
max_wal_senders = 5
wal_keep_size = 1GB
hot_standby = on
shared_preload_libraries = 'timescaledb'
```

**Edit `/etc/postgresql/16/main/pg_hba.conf`:**
```conf
# Cold Storage App Connections
host    all             all             192.168.15.0/24         scram-sha-256
# Replication
host    replication     replicator      192.168.15.231/32       scram-sha-256
```

**Create users and database:**
```bash
sudo -u postgres psql << EOF
-- Create replication user
CREATE ROLE replicator WITH REPLICATION LOGIN PASSWORD 'ReplicaPass2026!';

-- Create application database and user
CREATE DATABASE cold_db;
CREATE USER cold_user WITH PASSWORD 'SecurePostgresPassword123';
GRANT ALL PRIVILEGES ON DATABASE cold_db TO cold_user;

-- Connect to cold_db
\c cold_db
GRANT ALL ON SCHEMA public TO cold_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO cold_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO cold_user;

-- Enable TimescaleDB
CREATE EXTENSION IF NOT EXISTS timescaledb;
EOF
```

**Restart PostgreSQL:**
```bash
systemctl restart postgresql
```

### 2.3 Configure Standby (VM 231)

```bash
# Stop PostgreSQL
systemctl stop postgresql

# Remove data directory
rm -rf /var/lib/postgresql/16/main/*

# Take base backup from primary
sudo -u postgres PGPASSWORD=ReplicaPass2026! pg_basebackup \
  -h 192.168.15.230 \
  -D /var/lib/postgresql/16/main \
  -U replicator \
  -Fp -Xs -P -R

# Start PostgreSQL (will start in recovery mode)
systemctl start postgresql
```

### 2.4 Verify Replication

**On Primary (230):**
```bash
sudo -u postgres psql -c "SELECT client_addr, state, sent_lsn, replay_lsn FROM pg_stat_replication;"
```

**On Standby (231):**
```bash
sudo -u postgres psql -c "SELECT pg_is_in_recovery();"
# Should return 't' (true)
```

---

## 3. K3s Cluster Setup

### 3.1 Install K3s Master (VM 230)

```bash
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="server --disable traefik --tls-san 192.168.15.230 --node-ip 192.168.15.230" sh -

# Get node token for agent
cat /var/lib/rancher/k3s/server/node-token
# Output: K10fe3ee39f1887d5c5949792585aa0dfe3c750f44acf833e206b1351e2afc83d5e::server:d080e241e15399397f88795ce482e7c3
```

### 3.2 Install K3s Agent (VM 231)

```bash
curl -sfL https://get.k3s.io | K3S_URL=https://192.168.15.230:6443 K3S_TOKEN=<TOKEN_FROM_MASTER> sh -
```

### 3.3 Verify Cluster

```bash
k3s kubectl get nodes
# NAME              STATUS   ROLES                  AGE   VERSION
# coldstore-prod1   Ready    control-plane,master   1h    v1.34.3+k3s1
# coldstore-prod2   Ready    <none>                 1h    v1.34.3+k3s1
```

---

## 4. Monitoring Stack

### 4.1 Install Node Exporter (Both VMs)

```bash
# Download Node Exporter
cd /tmp
wget https://github.com/prometheus/node_exporter/releases/download/v1.7.0/node_exporter-1.7.0.linux-amd64.tar.gz
tar xzf node_exporter-1.7.0.linux-amd64.tar.gz
mv node_exporter-1.7.0.linux-amd64/node_exporter /usr/local/bin/

# Create systemd service
cat > /etc/systemd/system/node_exporter.service << 'EOF'
[Unit]
Description=Node Exporter
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/node_exporter
Restart=always

[Install]
WantedBy=multi-user.target
EOF

# Enable and start
systemctl daemon-reload
systemctl enable --now node_exporter

# Verify (port 9100)
curl localhost:9100/metrics | head
```

### 4.2 Install Postgres Exporter (Both VMs)

```bash
# Download Postgres Exporter
cd /tmp
wget https://github.com/prometheus-community/postgres_exporter/releases/download/v0.15.0/postgres_exporter-0.15.0.linux-amd64.tar.gz
tar xzf postgres_exporter-0.15.0.linux-amd64.tar.gz
mv postgres_exporter-0.15.0.linux-amd64/postgres_exporter /usr/local/bin/

# Create systemd service
cat > /etc/systemd/system/postgres_exporter.service << 'EOF'
[Unit]
Description=Postgres Exporter
After=network.target postgresql.service

[Service]
Type=simple
Environment="DATA_SOURCE_NAME=postgresql://postgres:@localhost:5432/cold_db?sslmode=disable"
ExecStart=/usr/local/bin/postgres_exporter
Restart=always

[Install]
WantedBy=multi-user.target
EOF

# Enable and start
systemctl daemon-reload
systemctl enable --now postgres_exporter

# Verify (port 9187)
curl localhost:9187/metrics | head
```

### 4.3 Install Prometheus (VM 230 only)

```bash
# Download Prometheus
cd /tmp
wget https://github.com/prometheus/prometheus/releases/download/v2.48.0/prometheus-2.48.0.linux-amd64.tar.gz
tar xzf prometheus-2.48.0.linux-amd64.tar.gz
mv prometheus-2.48.0.linux-amd64 /opt/prometheus

# Create configuration
cat > /opt/prometheus/prometheus.yml << 'EOF'
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'node-exporter'
    static_configs:
      - targets: ['192.168.15.230:9100']
        labels:
          instance: 'coldstore-prod1'
      - targets: ['192.168.15.231:9100']
        labels:
          instance: 'coldstore-prod2'

  - job_name: 'postgres'
    static_configs:
      - targets: ['192.168.15.230:9187']
        labels:
          instance: 'coldstore-prod1'
      - targets: ['192.168.15.231:9187']
        labels:
          instance: 'coldstore-prod2'
EOF

# Create systemd service
cat > /etc/systemd/system/prometheus.service << 'EOF'
[Unit]
Description=Prometheus
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/prometheus
ExecStart=/opt/prometheus/prometheus --config.file=/opt/prometheus/prometheus.yml --storage.tsdb.path=/opt/prometheus/data
Restart=always

[Install]
WantedBy=multi-user.target
EOF

# Enable and start
systemctl daemon-reload
systemctl enable --now prometheus

# Verify (port 9090)
curl localhost:9090/api/v1/targets
```

---

## 5. Application Deployment

### 5.1 Build Application

```bash
# On development machine
cd /path/to/cold-storage-management-system

# Build for Linux
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o server ./cmd/server
```

### 5.2 Deploy to VMs

```bash
# Create application directory (on both VMs)
mkdir -p /opt/coldstore

# Copy binary
scp server root@192.168.15.230:/opt/coldstore/
scp server root@192.168.15.231:/opt/coldstore/

# Copy templates and static files
scp -r templates root@192.168.15.230:/opt/coldstore/
scp -r static root@192.168.15.231:/opt/coldstore/
```

### 5.3 Create Systemd Service (Both VMs)

```bash
cat > /etc/systemd/system/coldstore.service << 'EOF'
[Unit]
Description=Cold Storage Management System
After=network.target postgresql.service

[Service]
Type=simple
WorkingDirectory=/opt/coldstore
ExecStart=/opt/coldstore/server -mode employee
Restart=always
RestartSec=5
Environment="DB_HOST=localhost"
Environment="DB_PORT=5432"
Environment="DB_USER=cold_user"
Environment="DB_PASSWORD=SecurePostgresPassword123"
Environment="DB_NAME=cold_db"
Environment="JWT_SECRET=your-secret-key-here"

[Install]
WantedBy=multi-user.target
EOF

# Enable and start
systemctl daemon-reload
systemctl enable --now coldstore

# Check status
systemctl status coldstore
```

### 5.4 Restore Database from R2 Backup

```bash
# On VM 230 (Primary)
# Install AWS CLI
apt-get install -y awscli

# Configure R2 credentials
export AWS_ACCESS_KEY_ID="290bc63d7d6900dd2ca59751b7456899"
export AWS_SECRET_ACCESS_KEY="038697927a70289e79774479aa0156c3193e3d9253cf970fdb42b5c1a09a55f7"

# Download latest backup
aws s3 cp s3://cold-db-backups/production-beta/base/2026/01/11/00/cold_db_20260111_003639.sql /tmp/backup.sql \
  --endpoint-url https://8ac6054e727fbfd99ced86c9705a5893.r2.cloudflarestorage.com

# Restore to database
sudo -u postgres psql -d cold_db < /tmp/backup.sql
```

---

## 6. R2 Backup Configuration

### 6.1 Backup Settings (in code)

**File: `internal/config/r2_config.go`**
```go
const (
    R2Endpoint   = "https://8ac6054e727fbfd99ced86c9705a5893.r2.cloudflarestorage.com"
    R2AccessKey  = "290bc63d7d6900dd2ca59751b7456899"
    R2SecretKey  = "038697927a70289e79774479aa0156c3193e3d9253cf970fdb42b5c1a09a55f7"
    R2BucketName = "cold-db-backups"
)

// Backup prefix based on environment
var R2BackupPrefix = "poc"  // or "production-beta" for prod
```

### 6.2 Backup Schedule

- **Interval:** Every 1 minute
- **Path format:** `{env}/base/YYYY/MM/DD/HH/cold_db_YYYYMMDD_HHMMSS.sql`
- **Retention:**
  - < 1 day: Keep ALL backups
  - 1-30 days: Keep 1 per hour
  - > 30 days: Keep 1 per day

---

## 7. Code Changes Made

### 7.1 Database Configuration

**File: `internal/config/r2_config.go`**
- Added POC servers to DatabaseFallbacks
- Added R2BackupPrefix for environment-specific backups
- Added SetR2BackupPrefixFromDB() for auto-detection

### 7.2 Monitoring Handler

**File: `internal/handlers/monitoring_handler.go`**
- Added getPOCNodeMetrics() for Prometheus integration
- Added getK3sNodes() for K3s API integration
- Added getPOCAPIAnalytics() for API stats without TimescaleDB
- Updated GetLatestNodeMetrics() to work in POC mode

### 7.3 API Logging

**File: `cmd/server/main.go`**
- Added API logging middleware for POC environments
- Enabled logging to PostgreSQL when TimescaleDB unavailable

### 7.4 Metrics Repository

**File: `internal/repositories/metrics_repository.go`**
- Fixed column names for POC api_request_logs table
- Updated queries to use `created_at` instead of `time`

### 7.5 New Migrations

**File: `migrations/026_add_snapshot_tracking.sql`**
- Added snapshot_metadata table
- Added table_change_tracking for change-based backups
- Added triggers for change detection

**File: `migrations/027_add_trash_bin.sql`**
- Added trash_bin table for undo functionality
- Added move_to_trash() and restore_from_trash() functions

---

## 8. Maintenance Commands

### 8.1 Service Management

```bash
# Restart application
systemctl restart coldstore

# View logs
journalctl -u coldstore -f

# Check status
systemctl status coldstore
```

### 8.2 Database Commands

```bash
# Connect to database
sudo -u postgres psql -d cold_db

# Check replication status (on primary)
sudo -u postgres psql -c "SELECT * FROM pg_stat_replication;"

# Check if standby (on standby)
sudo -u postgres psql -c "SELECT pg_is_in_recovery();"

# Manual backup
sudo -u postgres pg_dump cold_db > /tmp/backup.sql
```

### 8.3 K3s Commands

```bash
# Check nodes
k3s kubectl get nodes

# Check pods
k3s kubectl get pods -A

# View logs
k3s kubectl logs -n <namespace> <pod-name>
```

### 8.4 Monitoring Commands

```bash
# Check Prometheus targets
curl http://localhost:9090/api/v1/targets

# Query node CPU
curl "http://localhost:9090/api/v1/query?query=node_cpu_seconds_total"

# Check Node Exporter
curl http://localhost:9100/metrics | grep node_cpu
```

---

## Access Information

| Service | URL | Credentials |
|---------|-----|-------------|
| App (230) | http://192.168.15.230:8080 | user@cold.in / 111111 |
| App (231) | http://192.168.15.231:8080 | user@cold.in / 111111 |
| Prometheus | http://192.168.15.230:9090 | - |
| Node Exporter 230 | http://192.168.15.230:9100 | - |
| Node Exporter 231 | http://192.168.15.231:9100 | - |

---

## Troubleshooting

### App won't start
```bash
# Check if port is in use
ss -tlnp | grep 8080
# Kill existing process
pkill -f coldstore
# Start fresh
systemctl start coldstore
```

### PostgreSQL replication broken
```bash
# On standby, re-sync from primary
systemctl stop postgresql
rm -rf /var/lib/postgresql/16/main/*
sudo -u postgres PGPASSWORD=ReplicaPass2026! pg_basebackup -h 192.168.15.230 -D /var/lib/postgresql/16/main -U replicator -Fp -Xs -P -R
systemctl start postgresql
```

### Prometheus not collecting metrics
```bash
# Check targets
curl http://localhost:9090/api/v1/targets
# Restart if needed
systemctl restart prometheus
```

---

*Document created: 2026-01-11*
*Last updated: 2026-01-11*
