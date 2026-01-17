# Cold Storage Management System - Architecture v2.0

**Document Version**: 2.0
**Date**: 2026-01-12
**Architecture**: Mac Mini M4 Primary + Existing Server Secondary

---

## Table of Contents

1. [Overview](#1-overview)
2. [Architecture Decision](#2-architecture-decision)
3. [Hardware Specifications](#3-hardware-specifications)
4. [Network Topology](#4-network-topology)
5. [Component Layout](#5-component-layout)
6. [Data Flow](#6-data-flow)
7. [Failover Strategy](#7-failover-strategy)
8. [Cost Analysis](#8-cost-analysis)
9. [Power Consumption](#9-power-consumption)
10. [Future Scalability](#10-future-scalability)

---

## 1. Overview

The Cold Storage Management System has been redesigned with a cost-effective, power-efficient architecture using:

- **Mac Mini M4** as the primary production server (always on, low power)
- **Existing high-performance server** as secondary/archive server (on-demand)

This architecture provides:
- **70% cost savings** compared to dual Dell server setup (₹49,900 vs ₹1,85,000)
- **90% power reduction** for primary server (~15W vs ~150W)
- **High availability** through PostgreSQL streaming replication
- **Robust backup strategy** with local archives and R2 cloud storage

---

## 2. Architecture Decision

### 2.1 Previous Architecture (Rejected)

**Two Identical Dell Servers:**
- Cost: ₹1,85,000 each × 2 = ₹3,70,000
- Power: ~150W each = 300W total
- Overkill for current load (~100 users)
- Expensive to maintain

### 2.2 New Architecture (Approved)

**Mac Mini M4 + Existing Server:**
- Cost: ₹49,900 (Mac Mini) + ₹0 (existing) = ₹49,900
- Power: ~15W (Mac Mini) + ~150W (existing, on-demand) = ~15W average
- Right-sized for current load
- Room to scale when needed
- Existing server handles heavy workloads (backups, analytics)

### 2.3 Why This Architecture?

| Factor | Previous | New | Advantage |
|--------|----------|-----|-----------|
| **Initial Cost** | ₹3,70,000 | ₹49,900 | 87% savings |
| **Power 24/7** | 300W | 15W | 95% reduction |
| **Monthly Power Cost** | ~₹2,160 | ~₹108 | ₹2,052/month saved |
| **Noise** | High | Silent | Better office environment |
| **Space** | 2 rack units | 1 desktop unit | Minimal footprint |
| **Heat Output** | High | Minimal | No cooling needed |

---

## 3. Hardware Specifications

### 3.1 Server 1: Mac Mini M4 (Primary)

**Role**: Primary production server

| Component | Specification |
|-----------|---------------|
| **Model** | Mac Mini M4 (2024) |
| **Processor** | Apple M4 chip (10-core CPU) |
| **Memory** | 16GB unified memory |
| **Storage** | 256GB SSD |
| **Network** | Gigabit Ethernet |
| **Power** | ~15W typical, 18W max |
| **Cost** | ₹49,900 (with student discount) |
| **OS** | macOS Sequoia 15.2 |
| **IP Address** | 192.168.15.240 (example) |

**Installed Services:**
- PostgreSQL 16 (PRIMARY database)
- Cold Storage App (port 8080)
- Node Exporter (metrics)
- Postgres Exporter (metrics)

**Why Mac Mini M4?**
- Excellent performance/watt ratio
- Silent operation (fanless or near-silent)
- Small footprint (7.7" × 7.7" × 2")
- macOS stability for 24/7 operation
- Low heat output (no cooling needed)
- Fast SSD storage (sufficient for database)
- 10GbE optional upgrade available

### 3.2 Server 2: Existing Server (Secondary/Archive)

**Role**: Secondary/Archive server

| Component | Specification |
|-----------|---------------|
| **Processor** | 44 cores (Intel Xeon or similar) |
| **Memory** | 64GB RAM |
| **Storage** | 3 × 4TB HDD + 4 × 1TB SSD |
| **Network** | Gigabit Ethernet (or 10GbE) |
| **Power** | ~150W typical |
| **Cost** | ₹0 (already owned) |
| **OS** | Ubuntu 24.04 LTS (or Proxmox) |
| **IP Address** | 192.168.15.241 (example) |

**Installed Services:**
- PostgreSQL 16 (READ REPLICA)
- Cold Storage App (backup, port 8080)
- Prometheus + Grafana
- Node Exporter
- Postgres Exporter
- R2 backup scheduler
- Local snapshot archive
- Data analytics workloads

**Why Existing Server as Secondary?**
- Already available (no new cost)
- High RAM for analytics queries
- Large storage for local backups
- Powerful CPU for batch processing
- Can be powered off during low-usage periods

---

## 4. Network Topology

### 4.1 Network Diagram

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
│  │ Max Connections: 100       │    │ Max Connections: 50     │   │
│  │ Replication: Streaming     │    │ Read-only mode          │   │
│  │                            │    │                         │   │
│  │ Cold Storage App           │    │ Cold Storage App        │   │
│  │ Port: 8080                 │    │ Port: 8080 (backup)     │   │
│  │ JWT Auth                   │    │ Same JWT secret         │   │
│  │                            │    │                         │   │
│  │ Node Exporter              │    │ Prometheus (Master)     │   │
│  │ Port: 9100                 │◄───│ Port: 9090              │   │
│  │                            │    │ Scrapes: 240, 241       │   │
│  │ Postgres Exporter          │    │                         │   │
│  │ Port: 9187                 │    │ Grafana (Optional)      │   │
│  │                            │    │ Port: 3000              │   │
│  │                            │    │                         │   │
│  │ launchd (service manager)  │    │ Node Exporter           │   │
│  │                            │    │ Port: 9100              │   │
│  │                            │    │                         │   │
│  │ Always On (24/7)           │    │ Postgres Exporter       │   │
│  │ Power: ~15W                │    │ Port: 9187              │   │
│  │                            │    │                         │   │
│  │                            │    │ R2 Backup Scheduler     │   │
│  │                            │    │ (Every 1 minute)        │   │
│  │                            │    │                         │   │
│  │                            │    │ Local Archive Storage   │   │
│  │                            │    │ /archive/ (4TB HDD)     │   │
│  │                            │    │                         │   │
│  │                            │    │ systemd (service mgr)   │   │
│  │                            │    │                         │   │
│  │                            │    │ On-demand (power off)   │   │
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
                    │   Backups:           │
                    │   Every 1 minute     │
                    │   Retention:         │
                    │   - All < 1 day      │
                    │   - Hourly < 30d     │
                    │   - Daily > 30d      │
                    │                      │
                    │   JWT Secrets        │
                    │   Config backups     │
                    └──────────────────────┘
```

### 4.2 IP Address Allocation

| Server | Hostname | IP Address | Role | Always On |
|--------|----------|------------|------|-----------|
| Mac Mini M4 | coldstore-primary | 192.168.15.240 | Primary | Yes |
| Existing Server | coldstore-archive | 192.168.15.241 | Secondary | On-demand |

### 4.3 Port Mapping

| Service | Mac Mini (240) | Existing Server (241) | Access |
|---------|----------------|----------------------|--------|
| Cold Storage App | 8080 | 8080 | Public |
| PostgreSQL | 5432 | 5432 | Internal |
| Prometheus | - | 9090 | Private |
| Grafana | - | 3000 | Private |
| Node Exporter | 9100 | 9100 | Internal |
| Postgres Exporter | 9187 | 9187 | Internal |

---

## 5. Component Layout

### 5.1 Mac Mini M4 (Primary) - Components

```
┌─────────────────────────────────────────────┐
│           Mac Mini M4 (240)                 │
│         macOS Sequoia 15.2                  │
├─────────────────────────────────────────────┤
│                                             │
│  ┌────────────────────────────────────┐    │
│  │  PostgreSQL 16 (PRIMARY)           │    │
│  │  - Database: cold_db               │    │
│  │  - User: cold_user                 │    │
│  │  - Replication user: replicator    │    │
│  │  - Streaming to 241                │    │
│  │  - Data: /usr/local/var/postgres   │    │
│  └────────────────────────────────────┘    │
│                                             │
│  ┌────────────────────────────────────┐    │
│  │  Cold Storage App                  │    │
│  │  - Binary: /opt/coldstore/server   │    │
│  │  - Port: 8080                      │    │
│  │  - Mode: employee                  │    │
│  │  - Service: launchd plist          │    │
│  │  - Logs: ~/Library/Logs/coldstore/ │    │
│  └────────────────────────────────────┘    │
│                                             │
│  ┌────────────────────────────────────┐    │
│  │  Monitoring Exporters              │    │
│  │  - node_exporter (9100)            │    │
│  │  - postgres_exporter (9187)        │    │
│  └────────────────────────────────────┘    │
│                                             │
│  ┌────────────────────────────────────┐    │
│  │  Service Management (launchd)      │    │
│  │  - coldstore.plist                 │    │
│  │  - postgres.plist                  │    │
│  │  - node_exporter.plist             │    │
│  │  - postgres_exporter.plist         │    │
│  └────────────────────────────────────┘    │
│                                             │
│  Power: ~15W | Always On: Yes               │
└─────────────────────────────────────────────┘
```

### 5.2 Existing Server (Secondary) - Components

```
┌─────────────────────────────────────────────┐
│      Existing Server (241)                  │
│        Ubuntu 24.04 LTS                     │
├─────────────────────────────────────────────┤
│                                             │
│  ┌────────────────────────────────────┐    │
│  │  PostgreSQL 16 (READ REPLICA)      │    │
│  │  - Database: cold_db (replica)     │    │
│  │  - Streaming from 240              │    │
│  │  - Read-only queries               │    │
│  │  - Failover ready                  │    │
│  │  - Data: /var/lib/postgresql/16/   │    │
│  └────────────────────────────────────┘    │
│                                             │
│  ┌────────────────────────────────────┐    │
│  │  Cold Storage App (Backup)         │    │
│  │  - Binary: /opt/coldstore/server   │    │
│  │  - Port: 8080                      │    │
│  │  - Service: systemd                │    │
│  │  - Auto-connects to replica DB     │    │
│  └────────────────────────────────────┘    │
│                                             │
│  ┌────────────────────────────────────┐    │
│  │  Prometheus + Grafana Stack        │    │
│  │  - Prometheus (9090)               │    │
│  │  - Grafana (3000)                  │    │
│  │  - Scrapes both 240 & 241          │    │
│  │  - Retention: 90 days              │    │
│  └────────────────────────────────────┘    │
│                                             │
│  ┌────────────────────────────────────┐    │
│  │  R2 Backup Service                 │    │
│  │  - Runs every 1 minute             │    │
│  │  - Uploads to Cloudflare R2        │    │
│  │  - JWT secret backup               │    │
│  │  - Change-based snapshots          │    │
│  └────────────────────────────────────┘    │
│                                             │
│  ┌────────────────────────────────────┐    │
│  │  Local Archive Storage             │    │
│  │  - Path: /archive/                 │    │
│  │  - Size: 4TB HDD                   │    │
│  │  - Daily snapshots (90 days)       │    │
│  │  - Monthly full backups            │    │
│  └────────────────────────────────────┘    │
│                                             │
│  ┌────────────────────────────────────┐    │
│  │  Monitoring Exporters              │    │
│  │  - node_exporter (9100)            │    │
│  │  - postgres_exporter (9187)        │    │
│  └────────────────────────────────────┘    │
│                                             │
│  Power: ~150W | Always On: No (on-demand)   │
└─────────────────────────────────────────────┘
```

---

## 6. Data Flow

### 6.1 Normal Operation (Mac Mini Primary)

```
┌──────────────┐
│   Client     │
│   Browser    │
└──────┬───────┘
       │ HTTP (port 8080)
       ▼
┌──────────────────────────┐
│  Mac Mini M4 (240)       │
│  Cold Storage App        │
└──────┬───────────────────┘
       │ SQL queries
       ▼
┌──────────────────────────┐
│  PostgreSQL PRIMARY      │
│  (240)                   │
└──────┬───────────────────┘
       │ Streaming replication
       ▼
┌──────────────────────────┐
│  PostgreSQL REPLICA      │
│  (241)                   │
└──────┬───────────────────┘
       │ pg_dump (every 1 min)
       ▼
┌──────────────────────────┐
│  Cloudflare R2 Bucket    │
│  cold-db-backups         │
└──────────────────────────┘
       │
       │ Local copy
       ▼
┌──────────────────────────┐
│  Local Archive (241)     │
│  /archive/ (4TB)         │
└──────────────────────────┘
```

### 6.2 Monitoring Data Flow

```
┌─────────────────┐         ┌─────────────────┐
│  Mac Mini (240) │         │  Server (241)   │
│                 │         │                 │
│  node_exporter  │◄────────┤  Prometheus     │
│  (9100)         │ scrape  │  (9090)         │
│                 │         │                 │
│  pg_exporter    │◄────────┤                 │
│  (9187)         │ scrape  │                 │
└─────────────────┘         └────────┬────────┘
                                     │
                            ┌────────▼────────┐
                            │   Grafana       │
                            │   (3000)        │
                            │   Dashboards    │
                            └─────────────────┘
```

### 6.3 Backup Data Flow

```
PostgreSQL PRIMARY (240)
      │
      ▼ streaming
PostgreSQL REPLICA (241)
      │
      ├─► pg_dump (every 1 minute)
      │   │
      │   ├─► R2 Bucket (cloud)
      │   │   └─► Retention Policy
      │   │       - All < 1 day
      │   │       - Hourly < 30 days
      │   │       - Daily > 30 days
      │   │
      │   └─► Local Archive (241)
      │       └─► /archive/
      │           - Daily snapshots (90 days)
      │           - Monthly full (unlimited)
      │
      └─► Change tracking
          └─► Snapshot metadata
              └─► Incremental backups
```

---

## 7. Failover Strategy

### 7.1 Failure Scenarios

#### Scenario 1: Mac Mini Primary Fails

**Impact**: Primary application down, database writes unavailable

**Recovery Steps**:

1. **Promote Replica to Primary** (on Server 241):
   ```bash
   sudo -u postgres pg_ctl promote -D /var/lib/postgresql/16/main
   ```

2. **Update DNS/Load Balancer**:
   - Point traffic to 192.168.15.241:8080

3. **Verify Application**:
   ```bash
   curl http://192.168.15.241:8080/health
   ```

**Recovery Time Objective (RTO)**: < 5 minutes
**Recovery Point Objective (RPO)**: < 1 second (streaming replication)

#### Scenario 2: Secondary Server Fails

**Impact**: No monitoring, no backups, no replica

**Action Required**:
- Primary continues normal operation
- Fix secondary server when convenient
- Manual backups from primary until secondary restored

**Recovery Time Objective (RTO)**: Not critical (primary still operational)
**Recovery Point Objective (RPO)**: N/A

#### Scenario 3: Network Partition

**Impact**: Replication breaks, backups may fail

**Action Required**:
- Primary continues serving requests
- Monitor replication lag
- Fix network connectivity
- Re-sync replica if lag too high

#### Scenario 4: Database Corruption

**Impact**: Data integrity compromised

**Recovery Steps**:

1. **Restore from R2 backup**:
   ```bash
   # List recent backups
   aws s3 ls s3://cold-db-backups/production/base/$(date +%Y/%m/%d)/ \
     --endpoint-url https://[R2_ENDPOINT]

   # Download latest backup
   aws s3 cp s3://cold-db-backups/production/base/YYYY/MM/DD/HH/backup.sql /tmp/

   # Restore to database
   sudo -u postgres psql cold_db < /tmp/backup.sql
   ```

**Recovery Time Objective (RTO)**: < 15 minutes
**Recovery Point Objective (RPO)**: < 1 minute (backup frequency)

### 7.2 Automatic Failover Configuration

**Using HAProxy (Optional)**:

```bash
# Install HAProxy on a third machine or router
apt-get install haproxy

# Configure /etc/haproxy/haproxy.cfg
frontend cold_storage_frontend
    bind *:80
    default_backend cold_storage_backend

backend cold_storage_backend
    balance roundrobin
    option httpchk GET /health
    server primary 192.168.15.240:8080 check
    server secondary 192.168.15.241:8080 check backup
```

### 7.3 Health Check Monitoring

**Automated Health Checks**:

```bash
#!/bin/bash
# /usr/local/bin/health_check.sh

PRIMARY="192.168.15.240"
SECONDARY="192.168.15.241"

# Check primary
if curl -sf http://$PRIMARY:8080/health > /dev/null; then
    echo "Primary healthy"
else
    echo "PRIMARY DOWN! Failing over to secondary..."
    # Send alert
    # Update DNS
    # Promote replica
fi
```

### 7.4 Failover Testing Schedule

- **Weekly**: Test application connectivity to replica
- **Monthly**: Perform planned failover drill
- **Quarterly**: Full disaster recovery test from R2 backups

---

## 8. Cost Analysis

### 8.1 Hardware Cost Comparison

| Component | Previous (2 Dell) | New (Mac Mini + Existing) | Savings |
|-----------|-------------------|---------------------------|---------|
| **Primary Server** | ₹1,85,000 | ₹49,900 (Mac Mini M4) | ₹1,35,100 |
| **Secondary Server** | ₹1,85,000 | ₹0 (existing) | ₹1,85,000 |
| **Total Hardware** | ₹3,70,000 | ₹49,900 | **₹3,20,100 (87%)** |

### 8.2 Operating Cost Comparison (Annual)

| Cost Item | Previous | New | Annual Savings |
|-----------|----------|-----|----------------|
| **Power (24/7)** | 300W × ₹7.2/kWh × 8760h = ₹18,921 | 15W × ₹7.2/kWh × 8760h = ₹946 | ₹17,975 |
| **Cooling** | ~₹5,000 | ₹0 (no cooling needed) | ₹5,000 |
| **Maintenance** | ₹10,000 | ₹2,000 | ₹8,000 |
| **Total Annual** | ₹33,921 | ₹2,946 | **₹30,975 (91%)** |

### 8.3 Total Cost of Ownership (3 Years)

| Item | Previous | New | Savings |
|------|----------|-----|---------|
| **Initial Hardware** | ₹3,70,000 | ₹49,900 | ₹3,20,100 |
| **Operating (3 years)** | ₹1,01,763 | ₹8,838 | ₹92,925 |
| **Total 3-Year TCO** | ₹4,71,763 | ₹58,738 | **₹4,13,025 (88%)** |

### 8.4 Power Cost Calculation

**Previous Architecture (2 Dell Servers)**:
- Power: 150W × 2 = 300W
- Daily: 300W × 24h = 7.2 kWh
- Monthly: 7.2 kWh × 30 = 216 kWh × ₹7.2 = ₹1,555
- Annual: ₹1,555 × 12 = ₹18,660

**New Architecture (Mac Mini + Existing)**:
- Mac Mini (24/7): 15W × 24h = 360 Wh = 0.36 kWh/day
- Monthly: 0.36 × 30 = 10.8 kWh × ₹7.2 = ₹78
- Annual: ₹78 × 12 = ₹936

- Existing Server (8h/day): 150W × 8h = 1.2 kWh/day
- Monthly: 1.2 × 30 = 36 kWh × ₹7.2 = ₹259
- Annual: ₹259 × 12 = ₹3,108

**Total New Architecture**: ₹936 + ₹3,108 = ₹4,044/year

**Savings**: ₹18,660 - ₹4,044 = **₹14,616/year (78% reduction)**

---

## 9. Power Consumption

### 9.1 Power Usage Breakdown

| Server | Idle | Typical | Peak | Always On? | Daily Usage |
|--------|------|---------|------|------------|-------------|
| **Mac Mini M4** | 10W | 15W | 18W | Yes | 360 Wh |
| **Existing Server** | 80W | 150W | 200W | No (8h/day) | 1,200 Wh |
| **Total Daily** | - | - | - | - | **1,560 Wh (1.56 kWh)** |

### 9.2 Annual Power Consumption

**Mac Mini M4 (24/7)**:
- 15W × 24h × 365 days = 131.4 kWh/year
- Cost: 131.4 kWh × ₹7.2 = ₹946/year

**Existing Server (8h/day)**:
- 150W × 8h × 365 days = 438 kWh/year
- Cost: 438 kWh × ₹7.2 = ₹3,154/year

**Total Annual**:
- Energy: 569.4 kWh/year
- Cost: ₹4,100/year

### 9.3 Environmental Impact

**Carbon Footprint Reduction**:
- Previous: 300W × 8760h = 2,628 kWh/year × 0.82 kg CO₂/kWh = **2,155 kg CO₂/year**
- New: 569.4 kWh/year × 0.82 kg CO₂/kWh = **467 kg CO₂/year**
- **Reduction: 1,688 kg CO₂/year (78%)**

Equivalent to:
- 169 trees planted per year
- 4,200 km driven by average car saved

---

## 10. Future Scalability

### 10.1 Scaling Strategy

**Current Load**: ~100 users, ~1,000 requests/day

**Phase 1: Mac Mini Primary (Current)**
- Capacity: Up to 500 users
- Database: Single Mac Mini sufficient
- No changes needed

**Phase 2: Upgrade Mac Mini (₹50K-₹80K)**
- When: > 500 users
- Action: Upgrade Mac Mini to M4 Pro
  - 32GB RAM (from 16GB)
  - 512GB SSD (from 256GB)
  - 14-core CPU (from 10-core)
- Cost: ~₹80,000

**Phase 3: Dual Mac Minis (₹100K)**
- When: > 1,000 users
- Action: Add second Mac Mini as read replica
- Load balance reads across both
- Cost: ₹49,900 (second Mac Mini)

**Phase 4: Database Sharding (₹200K+)**
- When: > 5,000 users
- Action: Shard database by customer ID
- Use existing server for heavy shards
- Add more Mac Minis for additional shards

### 10.2 Storage Scaling

**Current Storage**:
- Mac Mini: 256GB SSD (database)
- Existing Server: 4TB HDD (archives)

**Scaling Options**:
1. **External Thunderbolt SSD** (₹15K-₹30K)
   - Connect to Mac Mini
   - Up to 4TB fast storage

2. **Network Storage (NAS)** (₹50K-₹100K)
   - Synology or QNAP
   - 8-16TB capacity
   - Shared across servers

3. **R2 Storage Expansion** (₹500/month per TB)
   - Increase retention from 8GB to 100GB
   - Cost: ~₹6,000/year

### 10.3 Network Bandwidth

**Current**: 1 Gbps Ethernet

**Upgrade Path**:
1. **Mac Mini**: Can upgrade to 10 Gbps Ethernet option (₹20K)
2. **Switch**: Upgrade to 10 GbE switch (₹30K-₹50K)
3. **Existing Server**: Add 10 GbE card (₹10K)

**When needed**: Database size > 100GB (faster replication)

### 10.4 Load Balancer Addition

**Current**: Single Mac Mini handles all requests

**When to add**: > 500 concurrent users

**Options**:
1. **HAProxy on Raspberry Pi** (₹5K)
   - Simple HTTP load balancer
   - Health checks
   - Automatic failover

2. **Dedicated Load Balancer** (₹30K-₹50K)
   - Hardware appliance
   - Advanced features
   - SSL termination

3. **Cloud Load Balancer** (₹500-₹2,000/month)
   - Cloudflare Load Balancing
   - Global distribution
   - DDoS protection

---

## Appendix A: Architecture Decision Record (ADR)

### ADR-001: Choose Mac Mini M4 as Primary Server

**Status**: Accepted
**Date**: 2026-01-12

**Context**:
We need a primary server that can run 24/7 with minimal power consumption, noise, and cost for a cold storage management system with ~100 users.

**Decision**:
Use Mac Mini M4 as primary production server instead of Dell server.

**Consequences**:
- Positive: 87% cost savings, 95% power reduction, silent operation
- Positive: macOS stability, excellent performance/watt
- Negative: Limited expandability (16GB RAM, 256GB SSD)
- Negative: macOS-specific deployment (launchd instead of systemd)
- Mitigation: Existing server provides scalability headroom

**Alternatives Considered**:
1. Two identical Dell servers (₹3.7L) - Rejected: Too expensive
2. Single Dell server (₹1.85L) - Rejected: Expensive, high power
3. Intel NUC (₹40K-₹60K) - Rejected: Higher power than Mac Mini
4. Raspberry Pi (₹5K-₹10K) - Rejected: Insufficient performance

---

## Appendix B: Network Security

### B.1 Firewall Rules (macOS - Mac Mini)

```bash
# Enable macOS firewall
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --setglobalstate on

# Allow specific ports
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --add /opt/coldstore/server
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --add /usr/local/bin/postgres

# Block all incoming except:
# - Port 8080 (app)
# - Port 5432 (postgres, from 241 only)
# - Port 9100, 9187 (metrics, from 241 only)
```

### B.2 Firewall Rules (Ubuntu - Existing Server)

```bash
# Allow SSH
ufw allow 22/tcp

# Allow from Mac Mini only
ufw allow from 192.168.15.240 to any port 5432
ufw allow from 192.168.15.240 to any port 9090
ufw allow from 192.168.15.240 to any port 3000

# Enable firewall
ufw --force enable
```

---

**END OF ARCHITECTURE DOCUMENT**
