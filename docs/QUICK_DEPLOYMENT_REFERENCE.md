# Quick Deployment Reference - Cold Storage Management System

**Fast reference for deploying the production POC environment**

Date: 2026-01-12
See: PRODUCTION_SETUP_GUIDE.md for complete documentation

---

## Environment Overview

| Component | VM 230 (Primary) | VM 231 (Standby) |
|-----------|------------------|------------------|
| **Hostname** | coldstore-prod1 | coldstore-prod2 |
| **IP Address** | 192.168.15.230 | 192.168.15.231 |
| **PostgreSQL** | Primary | Streaming Replica |
| **K3s** | Master | Agent |
| **Application** | Port 8080 | Port 8080 |
| **Prometheus** | Port 9090 | - |
| **Node Exporter** | Port 9100 | Port 9100 |
| **Postgres Exporter** | Port 9187 | Port 9187 |

---

## Quick Commands

### 1. Connect to Proxmox

```bash
ssh -i ~/.ssh/id_rsa_195 root@192.168.15.96
```

### 2. Create VMs on Proxmox

```bash
# Download Ubuntu 24.04 cloud image
cd /var/lib/vz/template/iso
wget https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img

# Create VM 230
qm create 230 --name coldstore-prod1 --memory 8192 --cores 8 --net0 virtio,bridge=vmbr0
qm importdisk 230 noble-server-cloudimg-amd64.img local-lvm
qm set 230 --scsihw virtio-scsi-pci --scsi0 local-lvm:vm-230-disk-0
qm set 230 --ide2 local-lvm:cloudinit --boot c --bootdisk scsi0
qm set 230 --serial0 socket --vga serial0
qm set 230 --ipconfig0 ip=192.168.15.230/24,gw=192.168.15.1
qm set 230 --ciuser root --sshkeys /path/to/ssh_key.pub
qm resize 230 scsi0 100G
qm start 230

# Create VM 231 (same process, different IP)
qm create 231 --name coldstore-prod2 --memory 8192 --cores 8 --net0 virtio,bridge=vmbr0
qm importdisk 231 noble-server-cloudimg-amd64.img local-lvm
qm set 231 --scsihw virtio-scsi-pci --scsi0 local-lvm:vm-231-disk-0
qm set 231 --ide2 local-lvm:cloudinit --boot c --bootdisk scsi0
qm set 231 --serial0 socket --vga serial0
qm set 231 --ipconfig0 ip=192.168.15.231/24,gw=192.168.15.1
qm set 231 --ciuser root --sshkeys /path/to/ssh_key.pub
qm resize 231 scsi0 100G
qm start 231
```

### 3. Install PostgreSQL 16 + TimescaleDB (Both VMs)

```bash
# SSH to each VM and run:
apt-get update && apt-get upgrade -y
apt-get install -y curl wget gnupg2 lsb-release qemu-guest-agent postgresql-16 postgresql-contrib-16

# Add TimescaleDB
echo "deb https://packagecloud.io/timescale/timescaledb/ubuntu/ $(lsb_release -cs) main" | tee /etc/apt/sources.list.d/timescaledb.list
wget --quiet -O - https://packagecloud.io/timescale/timescaledb/gpgkey | apt-key add -
apt-get update && apt-get install -y timescaledb-2-postgresql-16
timescaledb-tune --quiet --yes
systemctl restart postgresql
```

### 4. Configure Primary PostgreSQL (VM 230 ONLY)

```bash
# Edit /etc/postgresql/16/main/postgresql.conf
listen_addresses = '*'
wal_level = replica
max_wal_senders = 5
wal_keep_size = 1GB
hot_standby = on
shared_preload_libraries = 'timescaledb'

# Edit /etc/postgresql/16/main/pg_hba.conf (add these lines)
host    all             cold_user       192.168.15.0/24         scram-sha-256
host    replication     replicator      192.168.15.231/32       scram-sha-256

# Restart and create database
systemctl restart postgresql

sudo -u postgres psql << 'EOF'
CREATE ROLE replicator WITH REPLICATION LOGIN PASSWORD 'ReplicaPass2026!';
CREATE DATABASE cold_db;
CREATE USER cold_user WITH PASSWORD 'SecurePostgresPassword123';
GRANT ALL PRIVILEGES ON DATABASE cold_db TO cold_user;
\c cold_db
GRANT ALL ON SCHEMA public TO cold_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO cold_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO cold_user;
CREATE EXTENSION IF NOT EXISTS timescaledb;
\q
EOF
```

### 5. Configure Standby PostgreSQL (VM 231 ONLY)

```bash
systemctl stop postgresql
rm -rf /var/lib/postgresql/16/main/*

sudo -u postgres PGPASSWORD='ReplicaPass2026!' pg_basebackup \
  -h 192.168.15.230 -D /var/lib/postgresql/16/main \
  -U replicator -Fp -Xs -P -R

systemctl start postgresql

# Verify
sudo -u postgres psql -c "SELECT pg_is_in_recovery();"  # Should return 't'
```

### 6. Verify Replication (VM 230)

```bash
sudo -u postgres psql -c "SELECT client_addr, state, sent_lsn, replay_lsn FROM pg_stat_replication;"
```

### 7. Install K3s

```bash
# Master (VM 230)
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="server --disable traefik --tls-san 192.168.15.230 --node-ip 192.168.15.230 --cluster-init" sh -
cat /var/lib/rancher/k3s/server/node-token  # Save this token

# Agent (VM 231)
export K3S_TOKEN="<TOKEN_FROM_MASTER>"
curl -sfL https://get.k3s.io | K3S_URL=https://192.168.15.230:6443 K3S_TOKEN=$K3S_TOKEN sh -

# Verify (from VM 230)
k3s kubectl get nodes
```

### 8. Install Monitoring Exporters (Both VMs)

```bash
# Node Exporter
wget https://github.com/prometheus/node_exporter/releases/download/v1.7.0/node_exporter-1.7.0.linux-amd64.tar.gz
tar xzf node_exporter-1.7.0.linux-amd64.tar.gz
mv node_exporter-1.7.0.linux-amd64/node_exporter /usr/local/bin/

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

systemctl daemon-reload && systemctl enable --now node_exporter

# Postgres Exporter
wget https://github.com/prometheus-community/postgres_exporter/releases/download/v0.15.0/postgres_exporter-0.15.0.linux-amd64.tar.gz
tar xzf postgres_exporter-0.15.0.linux-amd64.tar.gz
mv postgres_exporter-0.15.0.linux-amd64/postgres_exporter /usr/local/bin/

cat > /etc/systemd/system/postgres_exporter.service << 'EOF'
[Unit]
Description=Postgres Exporter
After=postgresql.service
[Service]
Type=simple
User=postgres
Environment="DATA_SOURCE_NAME=postgresql://postgres:@localhost:5432/cold_db?sslmode=disable"
ExecStart=/usr/local/bin/postgres_exporter
Restart=always
[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload && systemctl enable --now postgres_exporter
```

### 9. Install Prometheus (VM 230 ONLY)

```bash
wget https://github.com/prometheus/prometheus/releases/download/v2.48.0/prometheus-2.48.0.linux-amd64.tar.gz
tar xzf prometheus-2.48.0.linux-amd64.tar.gz
mv prometheus-2.48.0.linux-amd64 /opt/prometheus

cat > /opt/prometheus/prometheus.yml << 'EOF'
global:
  scrape_interval: 15s
scrape_configs:
  - job_name: 'node-exporter'
    static_configs:
      - targets: ['192.168.15.230:9100', '192.168.15.231:9100']
  - job_name: 'postgres'
    static_configs:
      - targets: ['192.168.15.230:9187', '192.168.15.231:9187']
EOF

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

systemctl daemon-reload && systemctl enable --now prometheus
```

### 10. Deploy Application (Both VMs)

```bash
# Build on dev machine
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o server ./cmd/server

# Copy to VMs
scp server root@192.168.15.230:/opt/coldstore/
scp server root@192.168.15.231:/opt/coldstore/
scp -r templates static root@192.168.15.230:/opt/coldstore/
scp -r templates static root@192.168.15.231:/opt/coldstore/

# Create systemd service (both VMs)
cat > /etc/systemd/system/coldstore.service << 'EOF'
[Unit]
Description=Cold Storage Management System
After=network.target postgresql.service
[Service]
Type=simple
WorkingDirectory=/opt/coldstore
ExecStart=/opt/coldstore/server -mode employee
Restart=always
Environment="DB_HOST=localhost"
Environment="DB_PORT=5432"
Environment="DB_USER=cold_user"
Environment="DB_PASSWORD=SecurePostgresPassword123"
Environment="DB_NAME=cold_db"
Environment="JWT_SECRET=YourProductionJWTSecretHere"
[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload && systemctl enable --now coldstore
```

### 11. Run Migrations (VM 230 ONLY)

```bash
cd /opt/coldstore
./server -migrate
# Or manually: for f in migrations/*.sql; do sudo -u postgres psql -d cold_db -f $f; done
```

---

## Verification Commands

```bash
# Check VMs
qm list | grep coldstore

# Check PostgreSQL replication
sudo -u postgres psql -c "SELECT * FROM pg_stat_replication;"

# Check K3s cluster
k3s kubectl get nodes

# Check services
systemctl status coldstore postgresql k3s node_exporter postgres_exporter prometheus

# Check application
curl http://192.168.15.230:8080/health
curl http://192.168.15.231:8080/health

# Check Prometheus targets
curl http://192.168.15.230:9090/api/v1/targets | jq .

# Check R2 backups
journalctl -u coldstore | grep "R2 Backup" | tail -10
```

---

## Credentials (CHANGE IN PRODUCTION)

| Service | Username | Password |
|---------|----------|----------|
| PostgreSQL (cold_user) | cold_user | SecurePostgresPassword123 |
| PostgreSQL (replicator) | replicator | ReplicaPass2026! |
| Application | user@cold.in | 111111 |

---

## Troubleshooting Quick Fixes

### Application won't start
```bash
journalctl -u coldstore -n 50
ss -tlnp | grep 8080  # Check if port in use
systemctl restart coldstore
```

### Replication broken
```bash
# On standby (231)
systemctl stop postgresql
rm -rf /var/lib/postgresql/16/main/*
sudo -u postgres PGPASSWORD='ReplicaPass2026!' pg_basebackup -h 192.168.15.230 -D /var/lib/postgresql/16/main -U replicator -Fp -Xs -P -R
systemctl start postgresql
```

### Prometheus not collecting
```bash
systemctl restart node_exporter postgres_exporter prometheus
curl http://localhost:9100/metrics | head
```

### R2 backups failing
```bash
journalctl -u coldstore | grep "R2 Backup" | tail -20
systemctl restart coldstore
```

---

## Important URLs

- App Primary: http://192.168.15.230:8080
- App Standby: http://192.168.15.231:8080
- Prometheus: http://192.168.15.230:9090
- Node Exporter 230: http://192.168.15.230:9100/metrics
- Node Exporter 231: http://192.168.15.231:9100/metrics

---

## Next Steps After Deployment

1. Change all default credentials
2. Configure firewall (UFW)
3. Set up SSL/TLS certificates
4. Configure backups beyond R2
5. Set up monitoring alerts
6. Test failover procedure
7. Document any custom changes

---

**For complete documentation, see: PRODUCTION_SETUP_GUIDE.md**
