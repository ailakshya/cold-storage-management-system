# Mac Mini M4 Setup Guide - Cold Storage Management System

**Document Version**: 1.0
**Date**: 2026-01-12
**Target**: Mac Mini M4 with macOS Sequoia 15.2

---

## Table of Contents

1. [Overview](#1-overview)
2. [Initial macOS Configuration](#2-initial-macos-configuration)
3. [Homebrew Installation](#3-homebrew-installation)
4. [PostgreSQL 16 Setup](#4-postgresql-16-setup)
5. [Application Deployment](#5-application-deployment)
6. [Monitoring Setup](#6-monitoring-setup)
7. [launchd Service Configuration](#7-launchd-service-configuration)
8. [Network Configuration](#8-network-configuration)
9. [Security Hardening](#9-security-hardening)
10. [Maintenance](#10-maintenance)

---

## 1. Overview

This guide covers the complete setup of Mac Mini M4 as the primary production server for the Cold Storage Management System.

### 1.1 Mac Mini Specifications

| Component | Specification |
|-----------|---------------|
| **Model** | Mac Mini M4 (2024) |
| **Processor** | Apple M4 (10-core CPU, 8P+2E) |
| **GPU** | 10-core GPU |
| **Memory** | 16GB unified memory |
| **Storage** | 256GB SSD |
| **Network** | Gigabit Ethernet |
| **Power** | ~15W typical |
| **OS** | macOS Sequoia 15.2 |

### 1.2 Services to Install

- PostgreSQL 16 (PRIMARY database)
- Cold Storage Management App
- Node Exporter (system metrics)
- Postgres Exporter (database metrics)

### 1.3 Prerequisites

- Mac Mini M4 with macOS Sequoia 15.2 installed
- Internet connection
- Admin account access
- Static IP configured (192.168.15.240)

---

## 2. Initial macOS Configuration

### 2.1 System Preferences

```bash
# Disable sleep (keep always on)
sudo pmset -a sleep 0
sudo pmset -a disksleep 0
sudo pmset -a displaysleep 0

# Disable screensaver
defaults -currentHost write com.apple.screensaver idleTime 0

# Set computer name
sudo scutil --set ComputerName "coldstore-primary"
sudo scutil --set LocalHostName "coldstore-primary"
sudo scutil --set HostName "coldstore-primary"

# Restart to apply changes
sudo reboot
```

### 2.2 Enable Remote Access

```bash
# Enable SSH (Remote Login)
sudo systemsetup -setremotelogin on

# Enable Screen Sharing (optional)
sudo launchctl load -w /System/Library/LaunchDaemons/com.apple.screensharing.plist

# Verify SSH is enabled
sudo systemsetup -getremotelogin
# Expected: Remote Login: On
```

### 2.3 Install Xcode Command Line Tools

```bash
# Install command line tools (required for Homebrew)
xcode-select --install

# Verify installation
xcode-select -p
# Expected: /Library/Developer/CommandLineTools

# Accept license
sudo xcodebuild -license accept
```

### 2.4 Create Application Directory

```bash
# Create directory for application
sudo mkdir -p /opt/coldstore
sudo chown $USER:staff /opt/coldstore
```

---

## 3. Homebrew Installation

### 3.1 Install Homebrew

```bash
# Install Homebrew
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# Add Homebrew to PATH (for Apple Silicon)
echo 'eval "$(/opt/homebrew/bin/brew shellenv)"' >> ~/.zprofile
eval "$(/opt/homebrew/bin/brew shellenv)"

# Verify installation
brew --version
# Expected: Homebrew 4.x.x

# Update Homebrew
brew update
```

### 3.2 Install Essential Packages

```bash
# Install essential packages
brew install \
  wget \
  curl \
  git \
  htop \
  jq \
  tree \
  postgresql@16 \
  node_exporter \
  postgres_exporter

# Verify installations
wget --version
git --version
psql --version
```

---

## 4. PostgreSQL 16 Setup

### 4.1 Install PostgreSQL 16

```bash
# Install PostgreSQL 16
brew install postgresql@16

# Add PostgreSQL to PATH
echo 'export PATH="/opt/homebrew/opt/postgresql@16/bin:$PATH"' >> ~/.zprofile
source ~/.zprofile

# Verify installation
psql --version
# Expected: psql (PostgreSQL) 16.x
```

### 4.2 Initialize Database Cluster

```bash
# Initialize database cluster
initdb /usr/local/var/postgres

# Start PostgreSQL
brew services start postgresql@16

# Wait for startup
sleep 5

# Verify PostgreSQL is running
brew services list | grep postgresql@16
# Expected: postgresql@16 started

# Test connection
psql postgres -c "SELECT version();"
```

### 4.3 Configure PostgreSQL for Replication

```bash
# Backup original config
cp /usr/local/var/postgres/postgresql.conf /usr/local/var/postgres/postgresql.conf.backup

# Edit postgresql.conf
cat >> /usr/local/var/postgres/postgresql.conf << 'EOF'

# Replication settings
wal_level = replica
max_wal_senders = 5
max_replication_slots = 5
wal_keep_size = 1GB
hot_standby = on
hot_standby_feedback = on

# Network configuration
listen_addresses = '*'

# Performance tuning (16GB RAM Mac Mini)
shared_buffers = 2GB
effective_cache_size = 8GB
maintenance_work_mem = 512MB
checkpoint_completion_target = 0.9
wal_buffers = 16MB
default_statistics_target = 100
random_page_cost = 1.1
effective_io_concurrency = 200
work_mem = 20MB
min_wal_size = 1GB
max_wal_size = 4GB
max_connections = 100

EOF

# Restart PostgreSQL
brew services restart postgresql@16
sleep 5
```

### 4.4 Configure Authentication (pg_hba.conf)

```bash
# Backup original pg_hba.conf
cp /usr/local/var/postgres/pg_hba.conf /usr/local/var/postgres/pg_hba.conf.backup

# Edit pg_hba.conf
cat > /usr/local/var/postgres/pg_hba.conf << 'EOF'
# TYPE  DATABASE        USER            ADDRESS                 METHOD

# Local connections
local   all             all                                     trust
host    all             all             127.0.0.1/32            trust
host    all             all             ::1/128                 trust

# Application connections (local network)
host    all             cold_user       192.168.15.0/24         scram-sha-256

# Replication connections (from secondary server)
host    replication     replicator      192.168.15.241/32       scram-sha-256
host    replication     replicator      192.168.15.0/24         scram-sha-256

EOF

# Reload PostgreSQL
psql postgres -c "SELECT pg_reload_conf();"
```

### 4.5 Create Database and Users

```bash
# Create database and users
psql postgres << 'EOF'

-- Create replication user
CREATE ROLE replicator WITH REPLICATION LOGIN PASSWORD 'ReplicaPass2026!';

-- Create application database
CREATE DATABASE cold_db;

-- Create application user
CREATE USER cold_user WITH PASSWORD 'SecurePostgresPassword123';

-- Grant privileges
GRANT ALL PRIVILEGES ON DATABASE cold_db TO cold_user;

-- Connect to cold_db
\c cold_db

-- Grant schema privileges
GRANT ALL ON SCHEMA public TO cold_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO cold_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO cold_user;

-- Verify
SELECT datname FROM pg_database WHERE datname = 'cold_db';
\du

\q
EOF

echo "✓ Database and users created"
```

### 4.6 Test Database Connection

```bash
# Test local connection
psql -U cold_user -d cold_db -h localhost -c "SELECT version();"

# Test remote connection (from another machine)
# psql -U cold_user -d cold_db -h 192.168.15.240 -c "SELECT 1;"

# Check replication slots
psql postgres -c "SELECT * FROM pg_replication_slots;"
```

---

## 5. Application Deployment

### 5.1 Build Application (on development machine)

```bash
# On your development Mac or Linux machine
cd /path/to/cold-storage-management-system

# Build for Mac (Apple Silicon)
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-w -s" -o server ./cmd/server

# Or build for Mac (Intel, if needed)
# CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-w -s" -o server ./cmd/server

# Verify binary
file server
# Expected: server: Mach-O 64-bit executable arm64

# Check size
ls -lh server
```

### 5.2 Copy Application Files to Mac Mini

```bash
# From development machine, copy to Mac Mini
scp server user@192.168.15.240:/opt/coldstore/
scp -r templates user@192.168.15.240:/opt/coldstore/
scp -r static user@192.168.15.240:/opt/coldstore/
scp configs/config.yaml user@192.168.15.240:/opt/coldstore/

# SSH to Mac Mini
ssh user@192.168.15.240

# Verify files
ls -la /opt/coldstore/
chmod +x /opt/coldstore/server
```

### 5.3 Run Database Migrations

```bash
# SSH to Mac Mini
ssh user@192.168.15.240

# Set environment variables
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=cold_user
export DB_PASSWORD=SecurePostgresPassword123
export DB_NAME=cold_db
export JWT_SECRET="YourProductionJWTSecretHere-ChangeThis-MustBe32CharsOrMore!"

# Run migrations
cd /opt/coldstore
./server -migrate

# Verify migrations
psql -U cold_user -d cold_db -c "\dt"
psql -U cold_user -d cold_db -c "SELECT COUNT(*) FROM schema_migrations;"
```

### 5.4 Test Application Manually

```bash
# Test application startup
cd /opt/coldstore
./server -mode employee

# In another terminal, test HTTP endpoint
curl http://localhost:8080/health

# Expected output:
# {"status":"healthy","database":"connected","version":"1.x.x"}

# Stop application (Ctrl+C)
```

---

## 6. Monitoring Setup

### 6.1 Install Node Exporter

```bash
# Node Exporter is already installed via Homebrew (step 3.2)

# Start Node Exporter
brew services start node_exporter

# Verify
curl http://localhost:9100/metrics | head -20

# Check service status
brew services list | grep node_exporter
```

### 6.2 Install Postgres Exporter

```bash
# Postgres Exporter is already installed via Homebrew

# Create config for Postgres Exporter
mkdir -p ~/.config/postgres_exporter

cat > ~/.config/postgres_exporter/queries.yaml << 'EOF'
# Custom queries can be added here
EOF

# Set environment variable for Postgres Exporter
echo 'export DATA_SOURCE_NAME="postgresql://cold_user:SecurePostgresPassword123@localhost:5432/cold_db?sslmode=disable"' >> ~/.zprofile
source ~/.zprofile

# Start Postgres Exporter
brew services start postgres_exporter

# Verify
curl http://localhost:9187/metrics | head -20

# Check service status
brew services list | grep postgres_exporter
```

### 6.3 Verify Monitoring Stack

```bash
# Check all exporters are running
brew services list

# Expected output:
# Name              Status  User   File
# node_exporter     started [user] ~/Library/LaunchAgents/...
# postgres_exporter started [user] ~/Library/LaunchAgents/...
# postgresql@16     started [user] ~/Library/LaunchAgents/...

# Test metrics endpoints
curl -s http://localhost:9100/metrics | grep "node_cpu_seconds_total" | head -5
curl -s http://localhost:9187/metrics | grep "pg_up"
```

---

## 7. launchd Service Configuration

### 7.1 Create launchd Plist for Cold Storage App

```bash
# Create launchd plist
cat > ~/Library/LaunchAgents/com.coldstore.server.plist << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.coldstore.server</string>

    <key>ProgramArguments</key>
    <array>
        <string>/opt/coldstore/server</string>
        <string>-mode</string>
        <string>employee</string>
    </array>

    <key>WorkingDirectory</key>
    <string>/opt/coldstore</string>

    <key>EnvironmentVariables</key>
    <dict>
        <key>DB_HOST</key>
        <string>localhost</string>
        <key>DB_PORT</key>
        <string>5432</string>
        <key>DB_USER</key>
        <string>cold_user</string>
        <key>DB_PASSWORD</key>
        <string>SecurePostgresPassword123</string>
        <key>DB_NAME</key>
        <string>cold_db</string>
        <key>JWT_SECRET</key>
        <string>YourProductionJWTSecretHere-ChangeThis-MustBe32CharsOrMore!</string>
        <key>R2_ENDPOINT</key>
        <string>https://8ac6054e727fbfd99ced86c9705a5893.r2.cloudflarestorage.com</string>
        <key>R2_ACCESS_KEY</key>
        <string>290bc63d7d6900dd2ca59751b7456899</string>
        <key>R2_SECRET_KEY</key>
        <string>038697927a70289e79774479aa0156c3193e3d9253cf970fdb42b5c1a09a55f7</string>
        <key>R2_BUCKET_NAME</key>
        <string>cold-db-backups</string>
    </dict>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <dict>
        <key>SuccessfulExit</key>
        <false/>
    </dict>

    <key>StandardOutPath</key>
    <string>/opt/coldstore/logs/stdout.log</string>

    <key>StandardErrorPath</key>
    <string>/opt/coldstore/logs/stderr.log</string>

    <key>ProcessType</key>
    <string>Interactive</string>

    <key>Nice</key>
    <integer>0</integer>

    <key>SoftResourceLimits</key>
    <dict>
        <key>NumberOfFiles</key>
        <integer>65536</integer>
    </dict>

    <key>HardResourceLimits</key>
    <dict>
        <key>NumberOfFiles</key>
        <integer>65536</integer>
    </dict>
</dict>
</plist>
EOF

# Create logs directory
mkdir -p /opt/coldstore/logs

# Load the service
launchctl load ~/Library/LaunchAgents/com.coldstore.server.plist

# Verify service is running
launchctl list | grep coldstore

# Check logs
tail -f /opt/coldstore/logs/stdout.log
```

### 7.2 Manage launchd Service

```bash
# Start service
launchctl start com.coldstore.server

# Stop service
launchctl stop com.coldstore.server

# Restart service
launchctl stop com.coldstore.server
launchctl start com.coldstore.server

# Unload service (disable)
launchctl unload ~/Library/LaunchAgents/com.coldstore.server.plist

# Reload service (reload config)
launchctl unload ~/Library/LaunchAgents/com.coldstore.server.plist
launchctl load ~/Library/LaunchAgents/com.coldstore.server.plist

# Check service status
launchctl list | grep coldstore

# View logs
tail -f /opt/coldstore/logs/stdout.log
tail -f /opt/coldstore/logs/stderr.log
```

### 7.3 Auto-start on Boot

```bash
# The service will automatically start on boot due to:
# <key>RunAtLoad</key>
# <true/>

# To disable auto-start, edit plist and change to:
# <key>RunAtLoad</key>
# <false/>

# Then reload:
launchctl unload ~/Library/LaunchAgents/com.coldstore.server.plist
launchctl load ~/Library/LaunchAgents/com.coldstore.server.plist
```

---

## 8. Network Configuration

### 8.1 Set Static IP Address

**Via System Preferences (GUI)**:

1. Open System Preferences → Network
2. Select Ethernet (or Wi-Fi)
3. Click "Advanced"
4. Go to TCP/IP tab
5. Configure IPv4: Manually
6. IP Address: 192.168.15.240
7. Subnet Mask: 255.255.255.0
8. Router: 192.168.15.1
9. Click OK → Apply

**Via Command Line**:

```bash
# List network services
networksetup -listallnetworkservices

# Set manual IP (replace "Ethernet" with your service name)
sudo networksetup -setmanual "Ethernet" 192.168.15.240 255.255.255.0 192.168.15.1

# Set DNS servers
sudo networksetup -setdnsservers "Ethernet" 8.8.8.8 1.1.1.1

# Verify
ifconfig en0 | grep "inet "
# Expected: inet 192.168.15.240 netmask 0xffffff00 broadcast 192.168.15.255
```

### 8.2 Configure Hostname Resolution

```bash
# Edit /etc/hosts
sudo nano /etc/hosts

# Add entries
192.168.15.240  coldstore-primary
192.168.15.241  coldstore-archive

# Test
ping coldstore-archive
```

### 8.3 Enable Firewall

```bash
# Enable macOS firewall
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --setglobalstate on

# Allow signed applications
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --setallowsigned on

# Allow built-in applications
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --setallowsignedapp on

# Add Cold Storage App to allowed list
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --add /opt/coldstore/server
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --unblockapp /opt/coldstore/server

# Add PostgreSQL
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --add /opt/homebrew/opt/postgresql@16/bin/postgres
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --unblockapp /opt/homebrew/opt/postgresql@16/bin/postgres

# Restart firewall
sudo pkill -HUP socketfilterfw

# Verify firewall status
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --getglobalstate
```

### 8.4 Advanced Firewall Rules (Optional - pf)

```bash
# Create pf.conf
sudo nano /etc/pf.conf

# Add rules
# Block all incoming by default, allow specific ports
block in all
pass in proto tcp to any port 22 # SSH
pass in proto tcp to any port 8080 # Cold Storage App
pass in proto tcp from 192.168.15.241 to any port 5432 # PostgreSQL from secondary only
pass in proto tcp from 192.168.15.241 to any port 9100 # Node Exporter from secondary only
pass in proto tcp from 192.168.15.241 to any port 9187 # Postgres Exporter from secondary only
pass out all # Allow all outgoing

# Enable pf
sudo pfctl -e

# Load rules
sudo pfctl -f /etc/pf.conf

# Verify rules
sudo pfctl -sr
```

---

## 9. Security Hardening

### 9.1 Change Default Passwords

```bash
# Change PostgreSQL passwords
psql postgres << 'EOF'
ALTER USER cold_user WITH PASSWORD 'YourNewSecurePassword123!';
ALTER USER replicator WITH PASSWORD 'YourNewReplicatorPassword123!';
\q
EOF

# Update launchd plist with new password
nano ~/Library/LaunchAgents/com.coldstore.server.plist
# Change DB_PASSWORD value

# Reload service
launchctl unload ~/Library/LaunchAgents/com.coldstore.server.plist
launchctl load ~/Library/LaunchAgents/com.coldstore.server.plist
```

### 9.2 Generate Strong JWT Secret

```bash
# Generate JWT secret
openssl rand -base64 48

# Copy output and update launchd plist
nano ~/Library/LaunchAgents/com.coldstore.server.plist
# Update JWT_SECRET value

# Reload service
launchctl unload ~/Library/LaunchAgents/com.coldstore.server.plist
launchctl load ~/Library/LaunchAgents/com.coldstore.server.plist
```

### 9.3 Disable Root Login

```bash
# Edit SSH config
sudo nano /etc/ssh/sshd_config

# Add or modify:
PermitRootLogin no
PasswordAuthentication no
PubkeyAuthentication yes

# Restart SSH
sudo launchctl stop com.openssh.sshd
sudo launchctl start com.openssh.sshd
```

### 9.4 Setup Automatic Security Updates

```bash
# Enable automatic updates
sudo softwareupdate --schedule on

# Check for updates
softwareupdate -l

# Install updates
sudo softwareupdate -ia
```

### 9.5 File Permissions

```bash
# Secure application directory
sudo chown -R $USER:staff /opt/coldstore
chmod 755 /opt/coldstore
chmod 755 /opt/coldstore/server
chmod 644 /opt/coldstore/config.yaml

# Secure PostgreSQL data directory
chmod 700 /usr/local/var/postgres

# Secure launchd plist
chmod 644 ~/Library/LaunchAgents/com.coldstore.server.plist
```

---

## 10. Maintenance

### 10.1 Application Updates

```bash
# Stop application
launchctl stop com.coldstore.server

# Backup old binary
cp /opt/coldstore/server /opt/coldstore/server.backup.$(date +%Y%m%d)

# Copy new binary (from dev machine)
scp server user@192.168.15.240:/opt/coldstore/

# Set permissions
chmod +x /opt/coldstore/server

# Run migrations (if any)
cd /opt/coldstore
./server -migrate

# Start application
launchctl start com.coldstore.server

# Verify
curl http://localhost:8080/health
tail -f /opt/coldstore/logs/stdout.log
```

### 10.2 Database Maintenance

```bash
# Vacuum database
psql -U cold_user -d cold_db -c "VACUUM ANALYZE;"

# Check database size
psql -U cold_user -d cold_db -c "SELECT pg_size_pretty(pg_database_size('cold_db'));"

# Check replication status
psql postgres -c "SELECT client_addr, state, sent_lsn, replay_lsn FROM pg_stat_replication;"

# Check for bloat
psql -U cold_user -d cold_db -c "
SELECT schemaname, tablename,
  pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC
LIMIT 10;"
```

### 10.3 Log Rotation

```bash
# Create log rotation script
cat > /opt/coldstore/rotate_logs.sh << 'EOF'
#!/bin/bash

LOG_DIR="/opt/coldstore/logs"
MAX_SIZE=10485760  # 10MB

# Rotate if log files exceed 10MB
for log in stdout.log stderr.log; do
  if [ -f "$LOG_DIR/$log" ]; then
    SIZE=$(stat -f%z "$LOG_DIR/$log")
    if [ $SIZE -gt $MAX_SIZE ]; then
      mv "$LOG_DIR/$log" "$LOG_DIR/$log.$(date +%Y%m%d-%H%M%S)"
      touch "$LOG_DIR/$log"
      echo "Rotated $log"
    fi
  fi
done

# Keep only last 10 rotated logs
cd $LOG_DIR
ls -t stdout.log.* | tail -n +11 | xargs rm -f
ls -t stderr.log.* | tail -n +11 | xargs rm -f

EOF

chmod +x /opt/coldstore/rotate_logs.sh

# Create launchd plist for log rotation (daily)
cat > ~/Library/LaunchAgents/com.coldstore.logrotate.plist << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.coldstore.logrotate</string>
    <key>ProgramArguments</key>
    <array>
        <string>/opt/coldstore/rotate_logs.sh</string>
    </array>
    <key>StartCalendarInterval</key>
    <dict>
        <key>Hour</key>
        <integer>2</integer>
        <key>Minute</key>
        <integer>0</integer>
    </dict>
</dict>
</plist>
EOF

# Load log rotation service
launchctl load ~/Library/LaunchAgents/com.coldstore.logrotate.plist
```

### 10.4 System Monitoring

```bash
# Check system resources
htop

# Check disk usage
df -h

# Check memory usage
vm_stat

# Check CPU temperature (requires additional tools)
# brew install osx-cpu-temp
# osx-cpu-temp

# Check network connections
netstat -an | grep LISTEN

# Check PostgreSQL connections
psql postgres -c "SELECT count(*) FROM pg_stat_activity;"
```

### 10.5 Backup Verification

```bash
# Check R2 backups (requires AWS CLI)
brew install awscli

# Configure AWS CLI for R2
export AWS_ACCESS_KEY_ID="290bc63d7d6900dd2ca59751b7456899"
export AWS_SECRET_ACCESS_KEY="038697927a70289e79774479aa0156c3193e3d9253cf970fdb42b5c1a09a55f7"

# List recent backups
aws s3 ls s3://cold-db-backups/production/base/$(date +%Y/%m/%d)/ \
  --endpoint-url https://8ac6054e727fbfd99ced86c9705a5893.r2.cloudflarestorage.com

# Check application logs for backup status
grep "R2 Backup" /opt/coldstore/logs/stdout.log | tail -20
```

---

## Appendix A: Troubleshooting

### A.1 Application Won't Start

```bash
# Check logs
tail -n 100 /opt/coldstore/logs/stderr.log

# Check if port 8080 is in use
lsof -i :8080

# Manually test application
cd /opt/coldstore
./server -mode employee

# Check launchd service
launchctl list | grep coldstore
launchctl error com.coldstore.server
```

### A.2 Database Connection Issues

```bash
# Check PostgreSQL is running
brew services list | grep postgresql

# Test connection
psql -U cold_user -d cold_db -h localhost

# Check PostgreSQL logs
tail -f /usr/local/var/postgres/server.log

# Check pg_hba.conf
cat /usr/local/var/postgres/pg_hba.conf

# Reload PostgreSQL config
psql postgres -c "SELECT pg_reload_conf();"
```

### A.3 Replication Not Working

```bash
# Check replication status on primary
psql postgres -c "SELECT * FROM pg_stat_replication;"

# Check pg_hba.conf allows replication user
cat /usr/local/var/postgres/pg_hba.conf | grep replication

# Test replication user connection
PGPASSWORD='ReplicaPass2026!' psql -U replicator -h localhost -d postgres -c "SELECT 1;"

# Check PostgreSQL logs
tail -f /usr/local/var/postgres/server.log
```

### A.4 High Memory Usage

```bash
# Check memory usage
top -l 1 | grep PhysMem

# Check PostgreSQL memory
ps aux | grep postgres

# Reduce PostgreSQL shared_buffers if needed
nano /usr/local/var/postgres/postgresql.conf
# Change: shared_buffers = 1GB (from 2GB)

# Restart PostgreSQL
brew services restart postgresql@16
```

---

## Appendix B: Quick Reference

### B.1 Service Management Commands

```bash
# Cold Storage App
launchctl start com.coldstore.server
launchctl stop com.coldstore.server
launchctl list | grep coldstore

# PostgreSQL
brew services start postgresql@16
brew services stop postgresql@16
brew services restart postgresql@16

# Monitoring
brew services start node_exporter
brew services start postgres_exporter
```

### B.2 Important Paths

```bash
# Application
/opt/coldstore/server                           # Application binary
/opt/coldstore/logs/                            # Application logs
~/Library/LaunchAgents/com.coldstore.server.plist  # Service config

# PostgreSQL
/usr/local/var/postgres/                        # Data directory
/usr/local/var/postgres/postgresql.conf         # Config file
/usr/local/var/postgres/pg_hba.conf             # Auth config
/usr/local/var/postgres/server.log              # PostgreSQL logs

# Homebrew
/opt/homebrew/                                  # Homebrew root
/opt/homebrew/var/log/                          # Homebrew logs
```

### B.3 Useful Commands

```bash
# Check all services
launchctl list | grep -E "(coldstore|postgres|node_exporter)"

# Tail all logs
tail -f /opt/coldstore/logs/stdout.log /opt/coldstore/logs/stderr.log /usr/local/var/postgres/server.log

# Check application status
curl http://localhost:8080/health

# Check metrics
curl http://localhost:9100/metrics | head -20
curl http://localhost:9187/metrics | head -20

# Database query
psql -U cold_user -d cold_db -c "SELECT COUNT(*) FROM users;"
```

---

**END OF MAC MINI SETUP GUIDE**
