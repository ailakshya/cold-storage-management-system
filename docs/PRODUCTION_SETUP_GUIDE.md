# Cold Storage Management System - Production Setup Guide v2.0

**Complete Documentation for Production Deployment**

Version: 2.0
Date: 2026-01-12
Architecture: Mac Mini M4 Primary + Existing Server Secondary

---

## Table of Contents

1. [Overview](#1-overview)
2. [Architecture](#2-architecture)
3. [Infrastructure Setup](#3-infrastructure-setup)
4. [Mac Mini M4 Setup (Primary)](#4-mac-mini-m4-setup-primary)
5. [Existing Server Setup (Secondary)](#5-existing-server-setup-secondary)
6. [PostgreSQL 16 + Streaming Replication](#6-postgresql-16--streaming-replication)
7. [Application Deployment](#7-application-deployment)
8. [Monitoring Stack](#8-monitoring-stack)
9. [R2 Cloudflare Backup System](#9-r2-cloudflare-backup-system)
10. [Verification and Testing](#10-verification-and-testing)
11. [Maintenance Procedures](#11-maintenance-procedures)
12. [Troubleshooting Guide](#12-troubleshooting-guide)
13. [Security Considerations](#13-security-considerations)
14. [Deployment Checklist](#14-deployment-checklist)

---

## 1. Overview

### 1.1 System Summary

The Cold Storage Management System is deployed with a cost-effective, high-availability architecture:

- **Mac Mini M4** (Primary): Always-on production server (~15W power)
- **Existing Server** (Secondary): Archive/backup server with monitoring (~150W, on-demand)
- **PostgreSQL 16 Streaming Replication** (Primary → Replica)
- **Cloudflare R2 Automated Backups** (every 1 minute)
- **Prometheus + Grafana Monitoring**

### 1.2 Environment Details

| Server | Hostname | IP Address | Role | OS | Always On |
|--------|----------|------------|------|-----|-----------|
| Mac Mini M4 | coldstore-primary | 192.168.15.240 | Primary | macOS | Yes |
| Existing Server | coldstore-archive | 192.168.15.241 | Secondary | Ubuntu | On-demand |

### 1.3 Key Features

- **Cost Savings**: 87% cheaper than dual Dell server setup (₹49,900 vs ₹3,70,000)
- **Power Efficiency**: 95% power reduction (~15W vs 300W)
- **Zero Data Loss**: 1-minute backup intervals to R2
- **High Availability**: Automatic database replication
- **Monitoring**: Real-time metrics via Prometheus + Grafana
- **Disaster Recovery**: JWT secrets and database backups on R2

### 1.4 Why This Architecture?

**Previous Architecture (Rejected)**:
- 2× Dell Servers (₹1,85,000 each)
- Total cost: ₹3,70,000
- Power: 300W continuous
- Overkill for ~100 users

**New Architecture (Approved)**:
- Mac Mini M4 (₹49,900) + Existing Server (₹0)
- Total cost: ₹49,900 (87% savings)
- Power: ~15W continuous (95% reduction)
- Right-sized for current load with room to scale

---

## 2. Architecture

### 2.1 Network Diagram

```
┌────────────────────────────────────────────────────────────────────┐
│                         Local Network                              │
│                      192.168.15.0/24                               │
├────────────────────────────────────────────────────────────────────┤
│                                                                    │
│  ┌────────────────────────────┐    ┌─────────────────────────┐   │
│  │   Mac Mini M4 (Primary)    │    │  Existing Server (Sec)  │   │
│  │    192.168.15.240          │◄──►│   192.168.15.241        │   │
│  │    coldstore-primary       │    │   coldstore-archive     │   │
│  ├────────────────────────────┤    ├─────────────────────────┤   │
│  │                            │    │                         │   │
│  │ macOS Sequoia 15.2         │    │ Ubuntu 24.04 LTS        │   │
│  │                            │    │                         │   │
│  │ PostgreSQL 16 (PRIMARY)    │───►│ PostgreSQL 16 (REPLICA) │   │
│  │ Port: 5432                 │    │ Port: 5432              │   │
│  │ Replication: Streaming     │    │ Read-only mode          │   │
│  │                            │    │                         │   │
│  │ Cold Storage App           │    │ Cold Storage App        │   │
│  │ Port: 8080                 │    │ Port: 8080 (backup)     │   │
│  │                            │    │                         │   │
│  │ Node Exporter (9100)       │◄───│ Prometheus (9090)       │   │
│  │ Postgres Exporter (9187)   │    │ Grafana (3000)          │   │
│  │                            │    │ Node Exporter (9100)    │   │
│  │ launchd services           │    │ Postgres Exporter       │   │
│  │                            │    │                         │   │
│  │ Always On: Yes             │    │ R2 Backup Scheduler     │   │
│  │ Power: ~15W                │    │ Local Archive (4TB)     │   │
│  │                            │    │                         │   │
│  │                            │    │ systemd services        │   │
│  │                            │    │                         │   │
│  │                            │    │ Always On: No (on-demand)│  │
│  │                            │    │ Power: ~150W when on    │   │
│  │                            │    │                         │   │
│  └────────────────────────────┘    └─────────────────────────┘   │
│              │                                   │                │
│              └───────────────┬───────────────────┘                │
│                              │                                    │
└──────────────────────────────┼────────────────────────────────────┘
                               │
                               │ Internet
                               ▼
                    ┌──────────────────────┐
                    │   Cloudflare R2      │
                    │   Bucket:            │
                    │   cold-db-backups    │
                    │                      │
                    │   Every 1 minute     │
                    │   Retention:         │
                    │   - All < 1 day      │
                    │   - Hourly < 30d     │
                    │   - Daily > 30d      │
                    └──────────────────────┘
```

### 2.2 Component Versions

| Component | Version | Mac Mini | Existing Server | Purpose |
|-----------|---------|----------|-----------------|---------|
| **Operating System** | - | macOS Sequoia 15.2 | Ubuntu 24.04 LTS | Base OS |
| **PostgreSQL** | 16 | PRIMARY | READ REPLICA | Database |
| **Go Runtime** | 1.23+ | Yes | Yes | Application |
| **Prometheus** | 2.48.0 | No | Yes | Metrics collection |
| **Grafana** | 10.x | No | Yes (optional) | Dashboards |
| **Node Exporter** | 1.7.0 | Yes | Yes | System metrics |
| **Postgres Exporter** | 0.15.0 | Yes | Yes | Database metrics |

### 2.3 Port Mapping

| Service | Mac Mini (240) | Existing Server (241) | Access |
|---------|----------------|----------------------|--------|
| Cold Storage App | 8080 | 8080 | Public |
| PostgreSQL | 5432 | 5432 | Internal |
| Prometheus | - | 9090 | Private |
| Grafana | - | 3000 | Private |
| Node Exporter | 9100 | 9100 | Internal |
| Postgres Exporter | 9187 | 9187 | Internal |

### 2.4 Hardware Specifications

#### Mac Mini M4 (Primary)

| Component | Specification |
|-----------|---------------|
| Processor | Apple M4 (10-core CPU) |
| Memory | 16GB unified memory |
| Storage | 256GB SSD |
| Network | Gigabit Ethernet |
| Power | ~15W typical |
| Cost | ₹49,900 (student discount) |

#### Existing Server (Secondary)

| Component | Specification |
|-----------|---------------|
| Processor | 44 cores |
| Memory | 64GB RAM |
| Storage | 3×4TB HDD + 4×1TB SSD |
| Network | Gigabit Ethernet |
| Power | ~150W typical |
| Cost | ₹0 (already owned) |

---

## 3. Infrastructure Setup

### 3.1 Prerequisites

Before starting, ensure you have:

- [ ] Mac Mini M4 with macOS Sequoia 15.2 installed
- [ ] Existing server with Ubuntu 24.04 LTS installed
- [ ] Both servers connected to the same network
- [ ] Static IP addresses configured (240 and 241)
- [ ] Admin/sudo access on both servers
- [ ] Internet connectivity for package downloads

### 3.2 Network Configuration

**Mac Mini (192.168.15.240)**:

```bash
# Via System Preferences → Network
# Or via command line:
sudo networksetup -setmanual "Ethernet" 192.168.15.240 255.255.255.0 192.168.15.1
sudo networksetup -setdnsservers "Ethernet" 8.8.8.8 1.1.1.1
```

**Existing Server (192.168.15.241)**:

```bash
# Edit netplan configuration
sudo nano /etc/netplan/00-installer-config.yaml

# Add:
network:
  version: 2
  ethernets:
    eth0:
      addresses:
        - 192.168.15.241/24
      gateway4: 192.168.15.1
      nameservers:
        addresses:
          - 8.8.8.8
          - 1.1.1.1

# Apply
sudo netplan apply
```

### 3.3 Hostname Configuration

**Mac Mini**:

```bash
sudo scutil --set ComputerName "coldstore-primary"
sudo scutil --set LocalHostName "coldstore-primary"
sudo scutil --set HostName "coldstore-primary"
```

**Existing Server**:

```bash
sudo hostnamectl set-hostname coldstore-archive
```

### 3.4 Hosts File (Both Servers)

**Mac Mini** (`/etc/hosts`):

```bash
sudo nano /etc/hosts

# Add:
192.168.15.240  coldstore-primary
192.168.15.241  coldstore-archive
```

**Existing Server** (`/etc/hosts`):

```bash
sudo nano /etc/hosts

# Add:
192.168.15.240  coldstore-primary
192.168.15.241  coldstore-archive
```

### 3.5 Time Synchronization

**Mac Mini**:

```bash
# Enable automatic time
sudo systemsetup -setusingnetworktime on
sudo systemsetup -settimezone "Asia/Kolkata"
```

**Existing Server**:

```bash
# Install and enable NTP
sudo apt-get install -y ntp
sudo timedatectl set-timezone Asia/Kolkata
sudo timedatectl set-ntp true
```

---

## 4. Mac Mini M4 Setup (Primary)

For complete Mac Mini setup instructions, see: **[MAC_MINI_SETUP.md](./MAC_MINI_SETUP.md)**

### 4.1 Quick Setup Summary

```bash
# 1. Install Homebrew
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# 2. Install packages
brew install postgresql@16 node_exporter postgres_exporter wget git

# 3. Configure PostgreSQL for replication
brew services start postgresql@16

# 4. Create database and users
psql postgres -c "CREATE DATABASE cold_db;"
psql postgres -c "CREATE USER cold_user WITH PASSWORD 'SecurePostgresPassword123';"
psql postgres -c "CREATE ROLE replicator WITH REPLICATION LOGIN PASSWORD 'ReplicaPass2026!';"

# 5. Deploy application
mkdir -p /opt/coldstore
# Copy application files to /opt/coldstore/

# 6. Create launchd service
# See MAC_MINI_SETUP.md for complete launchd configuration

# 7. Start services
launchctl load ~/Library/LaunchAgents/com.coldstore.server.plist
brew services start node_exporter
brew services start postgres_exporter
```

**For detailed Mac Mini setup, refer to [MAC_MINI_SETUP.md](./MAC_MINI_SETUP.md)**

---

## 5. Existing Server Setup (Secondary)

### 5.1 Base System Configuration

```bash
# SSH to existing server
ssh user@192.168.15.241

# Update system
sudo apt-get update && sudo apt-get upgrade -y

# Install essential packages
sudo apt-get install -y \
  curl wget gnupg2 lsb-release \
  vim htop net-tools \
  postgresql-client \
  build-essential \
  awscli
```

### 5.2 Install PostgreSQL 16

```bash
# Add PostgreSQL repository
sudo sh -c 'echo "deb http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list'
wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | sudo apt-key add -

# Update and install
sudo apt-get update
sudo apt-get install -y postgresql-16 postgresql-contrib-16

# Verify installation
psql --version
```

### 5.3 Install Node Exporter

```bash
# Download Node Exporter
cd /tmp
wget https://github.com/prometheus/node_exporter/releases/download/v1.7.0/node_exporter-1.7.0.linux-amd64.tar.gz
tar xzf node_exporter-1.7.0.linux-amd64.tar.gz
sudo mv node_exporter-1.7.0.linux-amd64/node_exporter /usr/local/bin/
sudo chmod +x /usr/local/bin/node_exporter

# Create systemd service
sudo tee /etc/systemd/system/node_exporter.service > /dev/null << 'EOF'
[Unit]
Description=Node Exporter
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/node_exporter
Restart=always

[Install]
WantedBy=multi-user.target
EOF

# Start service
sudo systemctl daemon-reload
sudo systemctl enable node_exporter
sudo systemctl start node_exporter
```

### 5.4 Install Postgres Exporter

```bash
# Download Postgres Exporter
cd /tmp
wget https://github.com/prometheus-community/postgres_exporter/releases/download/v0.15.0/postgres_exporter-0.15.0.linux-amd64.tar.gz
tar xzf postgres_exporter-0.15.0.linux-amd64.tar.gz
sudo mv postgres_exporter-0.15.0.linux-amd64/postgres_exporter /usr/local/bin/
sudo chmod +x /usr/local/bin/postgres_exporter

# Create systemd service
sudo tee /etc/systemd/system/postgres_exporter.service > /dev/null << 'EOF'
[Unit]
Description=Postgres Exporter
After=network.target postgresql.service

[Service]
Type=simple
User=postgres
Environment="DATA_SOURCE_NAME=postgresql://postgres:@localhost:5432/cold_db?sslmode=disable"
ExecStart=/usr/local/bin/postgres_exporter
Restart=always

[Install]
WantedBy=multi-user.target
EOF

# Start service
sudo systemctl daemon-reload
sudo systemctl enable postgres_exporter
sudo systemctl start postgres_exporter
```

### 5.5 Install Prometheus

```bash
# Download Prometheus
cd /tmp
wget https://github.com/prometheus/prometheus/releases/download/v2.48.0/prometheus-2.48.0.linux-amd64.tar.gz
tar xzf prometheus-2.48.0.linux-amd64.tar.gz
sudo mv prometheus-2.48.0.linux-amd64 /opt/prometheus

# Create data directory
sudo mkdir -p /opt/prometheus/data
sudo chown -R $USER:$USER /opt/prometheus

# Create configuration
sudo tee /opt/prometheus/prometheus.yml > /dev/null << 'EOF'
global:
  scrape_interval: 15s
  evaluation_interval: 15s
  external_labels:
    cluster: 'cold-storage-prod'
    environment: 'production'

scrape_configs:
  # Mac Mini M4 (Primary)
  - job_name: 'primary-node'
    static_configs:
      - targets: ['192.168.15.240:9100']
        labels:
          instance: 'coldstore-primary'
          role: 'primary'

  - job_name: 'primary-postgres'
    static_configs:
      - targets: ['192.168.15.240:9187']
        labels:
          instance: 'coldstore-primary'
          role: 'primary'
          db: 'cold_db'

  # Existing Server (Secondary)
  - job_name: 'secondary-node'
    static_configs:
      - targets: ['192.168.15.241:9100']
        labels:
          instance: 'coldstore-archive'
          role: 'secondary'

  - job_name: 'secondary-postgres'
    static_configs:
      - targets: ['192.168.15.241:9187']
        labels:
          instance: 'coldstore-archive'
          role: 'secondary'
          db: 'cold_db'

  # Application metrics
  - job_name: 'app-primary'
    metrics_path: '/metrics'
    static_configs:
      - targets: ['192.168.15.240:8080']
        labels:
          instance: 'coldstore-primary'

  - job_name: 'app-secondary'
    metrics_path: '/metrics'
    static_configs:
      - targets: ['192.168.15.241:8080']
        labels:
          instance: 'coldstore-archive'
EOF

# Create systemd service
sudo tee /etc/systemd/system/prometheus.service > /dev/null << 'EOF'
[Unit]
Description=Prometheus
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/prometheus
ExecStart=/opt/prometheus/prometheus \
  --config.file=/opt/prometheus/prometheus.yml \
  --storage.tsdb.path=/opt/prometheus/data \
  --storage.tsdb.retention.time=90d \
  --web.listen-address=0.0.0.0:9090
Restart=always

[Install]
WantedBy=multi-user.target
EOF

# Start Prometheus
sudo systemctl daemon-reload
sudo systemctl enable prometheus
sudo systemctl start prometheus
```

### 5.6 Install Grafana (Optional)

```bash
# Add Grafana repository
sudo apt-get install -y software-properties-common
sudo add-apt-repository "deb https://packages.grafana.com/oss/deb stable main"
wget -q -O - https://packages.grafana.com/gpg.key | sudo apt-key add -

# Install Grafana
sudo apt-get update
sudo apt-get install -y grafana

# Start Grafana
sudo systemctl enable grafana-server
sudo systemctl start grafana-server

# Access Grafana at http://192.168.15.241:3000
# Default credentials: admin/admin
```

---

## 6. PostgreSQL 16 + Streaming Replication

### 6.1 Configure Primary (Mac Mini)

**Edit postgresql.conf** (`/usr/local/var/postgres/postgresql.conf`):

```bash
# Add these settings
listen_addresses = '*'
wal_level = replica
max_wal_senders = 5
max_replication_slots = 5
wal_keep_size = 1GB
hot_standby = on
hot_standby_feedback = on
```

**Edit pg_hba.conf** (`/usr/local/var/postgres/pg_hba.conf`):

```bash
# Add replication line
host    replication     replicator      192.168.15.241/32       scram-sha-256
```

**Restart PostgreSQL**:

```bash
brew services restart postgresql@16
```

### 6.2 Configure Replica (Existing Server)

```bash
# Stop PostgreSQL
sudo systemctl stop postgresql

# Remove existing data
sudo rm -rf /var/lib/postgresql/16/main/*
sudo mkdir -p /var/lib/postgresql/16/main
sudo chown postgres:postgres /var/lib/postgresql/16/main
sudo chmod 700 /var/lib/postgresql/16/main

# Take base backup from primary
sudo -u postgres PGPASSWORD='ReplicaPass2026!' pg_basebackup \
  -h 192.168.15.240 \
  -D /var/lib/postgresql/16/main \
  -U replicator \
  -Fp -Xs -P -R

# Start PostgreSQL (will start in recovery mode)
sudo systemctl start postgresql

# Verify replication
sudo -u postgres psql -c "SELECT pg_is_in_recovery();"
# Expected: t (true)
```

### 6.3 Verify Replication

**On Primary (Mac Mini)**:

```bash
psql postgres -c "SELECT client_addr, state, sent_lsn, replay_lsn FROM pg_stat_replication;"

# Expected output showing 192.168.15.241 in streaming state
```

**On Replica (Existing Server)**:

```bash
sudo -u postgres psql -c "SELECT status, receive_start_lsn, replay_lsn FROM pg_stat_wal_receiver;"

# Expected: status | streaming
```

---

## 7. Application Deployment

### 7.1 Build Application

**On Development Machine**:

```bash
cd /path/to/cold-storage-management-system

# Build for Mac Mini (Apple Silicon)
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-w -s" -o server-mac ./cmd/server

# Build for Existing Server (Linux)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o server-linux ./cmd/server

# Verify binaries
file server-mac    # Expected: Mach-O 64-bit executable arm64
file server-linux  # Expected: ELF 64-bit LSB executable, x86-64
```

### 7.2 Deploy to Mac Mini (Primary)

```bash
# Copy files to Mac Mini
scp server-mac user@192.168.15.240:/opt/coldstore/server
scp -r templates user@192.168.15.240:/opt/coldstore/
scp -r static user@192.168.15.240:/opt/coldstore/
scp configs/config.yaml user@192.168.15.240:/opt/coldstore/

# SSH to Mac Mini
ssh user@192.168.15.240

# Set permissions
chmod +x /opt/coldstore/server

# Run migrations
cd /opt/coldstore
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=cold_user
export DB_PASSWORD=SecurePostgresPassword123
export DB_NAME=cold_db
export JWT_SECRET="YourProductionJWTSecretHere-ChangeThis-MustBe32CharsOrMore!"

./server -migrate

# Create and load launchd service (see MAC_MINI_SETUP.md for complete plist)
launchctl load ~/Library/LaunchAgents/com.coldstore.server.plist
```

### 7.3 Deploy to Existing Server (Secondary)

```bash
# Copy files to Existing Server
scp server-linux user@192.168.15.241:/tmp/server
ssh user@192.168.15.241

# Install application
sudo mkdir -p /opt/coldstore
sudo mv /tmp/server /opt/coldstore/
sudo chmod +x /opt/coldstore/server
sudo chown -R $USER:$USER /opt/coldstore

# Copy templates and static files
scp -r templates user@192.168.15.241:/opt/coldstore/
scp -r static user@192.168.15.241:/opt/coldstore/

# Create systemd service
sudo tee /etc/systemd/system/coldstore.service > /dev/null << 'EOF'
[Unit]
Description=Cold Storage Management System
After=network.target postgresql.service

[Service]
Type=simple
User=root
WorkingDirectory=/opt/coldstore
ExecStart=/opt/coldstore/server -mode employee
Restart=always
StandardOutput=journal
StandardError=journal

Environment="DB_HOST=localhost"
Environment="DB_PORT=5432"
Environment="DB_USER=cold_user"
Environment="DB_PASSWORD=SecurePostgresPassword123"
Environment="DB_NAME=cold_db"
Environment="JWT_SECRET=YourProductionJWTSecretHere-ChangeThis-MustBe32CharsOrMore!"
Environment="R2_ENDPOINT=https://8ac6054e727fbfd99ced86c9705a5893.r2.cloudflarestorage.com"
Environment="R2_ACCESS_KEY=290bc63d7d6900dd2ca59751b7456899"
Environment="R2_SECRET_KEY=038697927a70289e79774479aa0156c3193e3d9253cf970fdb42b5c1a09a55f7"
Environment="R2_BUCKET_NAME=cold-db-backups"

[Install]
WantedBy=multi-user.target
EOF

# Start service
sudo systemctl daemon-reload
sudo systemctl enable coldstore
sudo systemctl start coldstore
```

---

## 8. Monitoring Stack

### 8.1 Access Prometheus

```bash
# Open in browser
http://192.168.15.241:9090

# Check targets
http://192.168.15.241:9090/targets

# All 6 targets should be "UP":
# - primary-node (240:9100)
# - primary-postgres (240:9187)
# - secondary-node (241:9100)
# - secondary-postgres (241:9187)
# - app-primary (240:8080)
# - app-secondary (241:8080)
```

### 8.2 Access Grafana (Optional)

```bash
# Open in browser
http://192.168.15.241:3000

# Default credentials: admin/admin

# Add Prometheus data source:
# - URL: http://localhost:9090
# - Access: Server (default)

# Import dashboards:
# - Node Exporter Full: Dashboard ID 1860
# - PostgreSQL Database: Dashboard ID 9628
```

### 8.3 Sample Prometheus Queries

```promql
# CPU usage on Mac Mini
100 - (avg by (instance) (rate(node_cpu_seconds_total{instance="coldstore-primary",mode="idle"}[5m])) * 100)

# Memory usage on Mac Mini
node_memory_MemAvailable_bytes{instance="coldstore-primary"} / node_memory_MemTotal_bytes{instance="coldstore-primary"} * 100

# PostgreSQL connections on primary
pg_stat_database_numbackends{instance="coldstore-primary",datname="cold_db"}

# Replication lag (bytes)
pg_stat_replication_pg_wal_lsn_diff{instance="coldstore-primary"}

# Application requests per second
rate(http_requests_total[5m])
```

---

## 9. R2 Cloudflare Backup System

### 9.1 R2 Configuration

The application uses hardcoded R2 credentials (in `internal/config/r2_config.go`):

```go
R2Endpoint   = "https://8ac6054e727fbfd99ced86c9705a5893.r2.cloudflarestorage.com"
R2AccessKey  = "290bc63d7d6900dd2ca59751b7456899"
R2SecretKey  = "038697927a70289e79774479aa0156c3193e3d9253cf970fdb42b5c1a09a55f7"
R2BucketName = "cold-db-backups"
R2BackupPrefix = "production"  // Change from "poc" to "production"
```

### 9.2 Backup Schedule

- **Frequency**: Every 1 minute
- **Path**: `production/base/YYYY/MM/DD/HH/cold_db_YYYYMMDD_HHMMSS.sql`
- **Retention**:
  - All backups < 1 day old
  - Hourly backups for 1-30 days
  - Daily backups > 30 days

### 9.3 Local Archive Storage (Existing Server)

```bash
# Create archive directory
sudo mkdir -p /archive/postgres
sudo chown postgres:postgres /archive/postgres

# Create backup script
sudo tee /usr/local/bin/backup_local.sh > /dev/null << 'EOF'
#!/bin/bash
BACKUP_DIR="/archive/postgres"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
pg_dump -U postgres cold_db | gzip > $BACKUP_DIR/cold_db_$TIMESTAMP.sql.gz

# Keep last 90 days only
find $BACKUP_DIR -name "*.sql.gz" -mtime +90 -delete
EOF

sudo chmod +x /usr/local/bin/backup_local.sh

# Add to cron (daily at 2 AM)
(crontab -l 2>/dev/null; echo "0 2 * * * /usr/local/bin/backup_local.sh") | crontab -
```

### 9.4 Verify Backups

```bash
# Check R2 backups
aws s3 ls s3://cold-db-backups/production/base/$(date +%Y/%m/%d)/ \
  --endpoint-url https://8ac6054e727fbfd99ced86c9705a5893.r2.cloudflarestorage.com

# Check local backups
ls -lh /archive/postgres/

# Check application logs
# Mac Mini:
tail -f /opt/coldstore/logs/stdout.log | grep "R2 Backup"

# Existing Server:
sudo journalctl -u coldstore | grep "R2 Backup"
```

---

## 10. Verification and Testing

### 10.1 Health Checks

```bash
# Mac Mini (Primary)
curl http://192.168.15.240:8080/health
# Expected: {"status":"healthy","database":"connected"}

# Existing Server (Secondary)
curl http://192.168.15.241:8080/health
# Expected: {"status":"healthy","database":"connected"}
```

### 10.2 Database Connectivity

```bash
# From Mac Mini
psql -U cold_user -d cold_db -h localhost -c "SELECT COUNT(*) FROM users;"

# From Existing Server
sudo -u postgres psql -d cold_db -c "SELECT COUNT(*) FROM users;"
```

### 10.3 Replication Test

```bash
# Insert data on primary (Mac Mini)
psql -U cold_user -d cold_db -h localhost << EOF
CREATE TABLE replication_test (
  id SERIAL PRIMARY KEY,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  message TEXT
);
INSERT INTO replication_test (message) VALUES ('Test from primary at $(date)');
EOF

# Verify on replica (Existing Server) - wait a few seconds
sudo -u postgres psql -d cold_db -c "SELECT * FROM replication_test;"

# Should show the inserted row
```

### 10.4 Monitoring Verification

```bash
# Check all Prometheus targets are up
curl -s http://192.168.15.241:9090/api/v1/targets | jq '.data.activeTargets[] | {job: .labels.job, instance: .labels.instance, health: .health}'

# Expected: All health: "up"
```

### 10.5 Application Login Test

```bash
# Test login page
curl http://192.168.15.240:8080/ | grep -i "login"

# Test API login
curl -X POST http://192.168.15.240:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@cold.in","password":"111111"}'
```

---

## 11. Maintenance Procedures

### 11.1 Daily Checks

```bash
# Check replication status
ssh user@192.168.15.240 "psql postgres -c 'SELECT client_addr, state, pg_wal_lsn_diff(sent_lsn, replay_lsn) AS lag_bytes FROM pg_stat_replication;'"

# Check service status (Mac Mini)
ssh user@192.168.15.240 "launchctl list | grep coldstore"

# Check service status (Existing Server)
ssh user@192.168.15.241 "systemctl status coldstore postgresql prometheus"

# Check R2 backups
aws s3 ls s3://cold-db-backups/production/base/$(date +%Y/%m/%d)/ \
  --endpoint-url https://8ac6054e727fbfd99ced86c9705a5893.r2.cloudflarestorage.com | tail -10
```

### 11.2 Weekly Maintenance

```bash
# Update macOS (Mac Mini)
ssh user@192.168.15.240 "sudo softwareupdate -ia"

# Update Ubuntu (Existing Server)
ssh user@192.168.15.241 "sudo apt-get update && sudo apt-get upgrade -y"

# Vacuum database (on primary)
ssh user@192.168.15.240 "psql -U cold_user -d cold_db -c 'VACUUM ANALYZE;'"

# Check database size
ssh user@192.168.15.240 "psql -U cold_user -d cold_db -c \"SELECT pg_size_pretty(pg_database_size('cold_db'));\""
```

### 11.3 Application Updates

```bash
# Build new version
cd /path/to/cold-storage-management-system
git pull
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o server-mac ./cmd/server
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server-linux ./cmd/server

# Update Mac Mini
ssh user@192.168.15.240 "launchctl stop com.coldstore.server"
scp server-mac user@192.168.15.240:/opt/coldstore/server
ssh user@192.168.15.240 "chmod +x /opt/coldstore/server && /opt/coldstore/server -migrate"
ssh user@192.168.15.240 "launchctl start com.coldstore.server"

# Update Existing Server
ssh user@192.168.15.241 "sudo systemctl stop coldstore"
scp server-linux user@192.168.15.241:/opt/coldstore/server
ssh user@192.168.15.241 "sudo chmod +x /opt/coldstore/server && sudo systemctl start coldstore"

# Verify
curl http://192.168.15.240:8080/health
curl http://192.168.15.241:8080/health
```

---

## 12. Troubleshooting Guide

### 12.1 Mac Mini Primary Down

**Symptom**: Application on 240 not responding

**Recovery**:

```bash
# 1. Promote replica to primary (on 241)
ssh user@192.168.15.241 "sudo -u postgres pg_ctl promote -D /var/lib/postgresql/16/main"

# 2. Update DNS/load balancer to point to 241

# 3. Verify application on 241
curl http://192.168.15.241:8080/health
```

### 12.2 Replication Broken

**Symptom**: Replica not receiving updates

**Fix**:

```bash
# On replica (241), re-sync from primary
ssh user@192.168.15.241

sudo systemctl stop postgresql
sudo rm -rf /var/lib/postgresql/16/main/*

sudo -u postgres PGPASSWORD='ReplicaPass2026!' pg_basebackup \
  -h 192.168.15.240 \
  -D /var/lib/postgresql/16/main \
  -U replicator \
  -Fp -Xs -P -R

sudo systemctl start postgresql

# Verify
sudo -u postgres psql -c "SELECT status FROM pg_stat_wal_receiver;"
```

### 12.3 High Replication Lag

**Symptom**: Replica significantly behind primary

**Diagnosis**:

```bash
# Check lag in bytes
ssh user@192.168.15.240 "psql postgres -c 'SELECT pg_wal_lsn_diff(sent_lsn, replay_lsn) AS lag_bytes FROM pg_stat_replication;'"
```

**Fix**:

```bash
# If lag > 1GB, consider re-syncing replica (see 12.2)
# Otherwise, check network connectivity
ping 192.168.15.240
ping 192.168.15.241
```

---

## 13. Security Considerations

### 13.1 Change Default Credentials

**CRITICAL**: Change these before production:

```bash
# PostgreSQL passwords
ssh user@192.168.15.240
psql postgres << 'EOF'
ALTER USER cold_user WITH PASSWORD 'YourNewSecurePassword123!';
ALTER USER replicator WITH PASSWORD 'YourNewReplicatorPassword123!';
\q
EOF

# Update application configurations on both servers
# Update launchd plist on Mac Mini
# Update systemd service on Existing Server
```

### 13.2 Generate JWT Secret

```bash
# Generate strong JWT secret
openssl rand -base64 48

# Update in launchd plist (Mac Mini) and systemd service (Existing Server)
```

### 13.3 Firewall Configuration

**Mac Mini (macOS)**:

```bash
# Enable firewall
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --setglobalstate on

# Allow application
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --add /opt/coldstore/server
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --unblockapp /opt/coldstore/server
```

**Existing Server (Ubuntu)**:

```bash
# Install UFW
sudo apt-get install -y ufw

# Configure rules
sudo ufw allow 22/tcp   # SSH
sudo ufw allow from 192.168.15.240 to any port 5432  # PostgreSQL from primary only
sudo ufw allow from 192.168.15.240 to any port 9090  # Prometheus
sudo ufw allow from 192.168.15.240 to any port 3000  # Grafana

# Enable firewall
sudo ufw --force enable
```

---

## 14. Deployment Checklist

### 14.1 Pre-Deployment

- [ ] Mac Mini M4 purchased and received
- [ ] Static IP addresses assigned (240, 241)
- [ ] Network connectivity verified
- [ ] Admin credentials prepared
- [ ] R2 bucket created and verified
- [ ] Backup of existing data (if migrating)

### 14.2 Mac Mini Setup

- [ ] macOS Sequoia 15.2 installed
- [ ] Static IP configured (192.168.15.240)
- [ ] Hostname set (coldstore-primary)
- [ ] Homebrew installed
- [ ] PostgreSQL 16 installed
- [ ] Node Exporter installed
- [ ] Postgres Exporter installed
- [ ] Application deployed
- [ ] launchd services configured
- [ ] Firewall enabled

### 14.3 Existing Server Setup

- [ ] Ubuntu 24.04 LTS installed
- [ ] Static IP configured (192.168.15.241)
- [ ] Hostname set (coldstore-archive)
- [ ] PostgreSQL 16 installed
- [ ] Replica synced from primary
- [ ] Replication verified
- [ ] Node Exporter installed
- [ ] Postgres Exporter installed
- [ ] Prometheus installed
- [ ] Grafana installed (optional)
- [ ] Application deployed
- [ ] systemd services configured
- [ ] Firewall enabled

### 14.4 Verification

- [ ] Health checks passing on both servers
- [ ] Database replication working
- [ ] Monitoring stack operational
- [ ] R2 backups running every minute
- [ ] Local backups configured
- [ ] Application login tested
- [ ] API endpoints responding
- [ ] Prometheus targets all "up"

### 14.5 Security

- [ ] Default passwords changed
- [ ] JWT secret generated and configured
- [ ] Firewalls enabled on both servers
- [ ] SSH key-based auth configured
- [ ] R2 credentials secured

### 14.6 Documentation

- [ ] IP addresses documented
- [ ] Credentials stored securely
- [ ] Architecture diagram reviewed
- [ ] Runbook created
- [ ] Team trained on maintenance

---

## 15. Cost Breakdown

### 15.1 Hardware Costs

| Item | Previous | New | Savings |
|------|----------|-----|---------|
| Primary Server | ₹1,85,000 (Dell) | ₹49,900 (Mac Mini M4) | ₹1,35,100 |
| Secondary Server | ₹1,85,000 (Dell) | ₹0 (existing) | ₹1,85,000 |
| **Total** | **₹3,70,000** | **₹49,900** | **₹3,20,100 (87%)** |

### 15.2 Operating Costs (Annual)

| Item | Previous | New | Savings |
|------|----------|-----|---------|
| Power (24/7) | ₹18,921 | ₹946 + ₹3,108* | ₹14,867 |
| Cooling | ₹5,000 | ₹0 | ₹5,000 |
| Maintenance | ₹10,000 | ₹2,000 | ₹8,000 |
| **Total Annual** | **₹33,921** | **₹6,054** | **₹27,867 (82%)** |

*Existing server used 8 hours/day for backups and maintenance

### 15.3 Total Cost of Ownership (3 Years)

| Item | Previous | New | Savings |
|------|----------|-----|---------|
| Initial Hardware | ₹3,70,000 | ₹49,900 | ₹3,20,100 |
| Operating (3 years) | ₹1,01,763 | ₹18,162 | ₹83,601 |
| **Total 3-Year TCO** | **₹4,71,763** | **₹68,062** | **₹4,03,701 (86%)** |

---

**END OF PRODUCTION SETUP GUIDE**

**Document Version**: 2.0
**Last Updated**: 2026-01-12
**Next Review**: 2026-02-12
