# Infrastructure Documentation - Cold Storage Backend

**Date:** December 24, 2025
**Testing Domain:** ailakshya.in
**Production Domain:** gurukripacoldstore.in

---

## Table of Contents

1. [Infrastructure Overview](#infrastructure-overview)
2. [Proxmox Virtual Machines](#proxmox-virtual-machines)
3. [K3s Kubernetes Cluster](#k3s-kubernetes-cluster)
4. [Bare Metal PostgreSQL Cluster](#bare-metal-postgresql-cluster)
5. [Cloudflare Tunnel HA](#cloudflare-tunnel-ha)
6. [Access Information](#access-information)
7. [Maintenance Commands](#maintenance-commands)
8. [Disaster Recovery](#disaster-recovery)

---

## Infrastructure Overview

### Physical Server
- **Host:** Proxmox VE at 192.168.15.96
- **CPU:** 2× Intel Xeon Gold 6138 @ 2.00GHz (80 cores total)
- **RAM:** 125.50 GB
- **Storage:** 4 TB HDD (local storage pool: "hdd")
- **Network:** Bridge vmbr0, Gateway 192.168.15.1

### Architecture (Updated December 2025)

The infrastructure uses a **split architecture** with:
- **3 K3s VMs** for running the application (stateless)
- **3 Bare Metal VMs** for PostgreSQL databases (stateful)

```
┌─────────────────────────────────────────────────────────────────┐
│                    KUBERNETES CLUSTER (App Only)                 │
├─────────────────┬─────────────────┬─────────────────────────────┤
│   k3s-node1     │   k3s-node2     │   k3s-node3                 │
│  192.168.15.110 │  192.168.15.111 │  192.168.15.112             │
│   control+worker│   control+worker│   control+worker            │
│     12GB RAM    │     12GB RAM    │     12GB RAM                │
│   cloudflared   │   cloudflared   │   cloudflared               │
│   backend pods  │   backend pods  │   backend pods              │
└────────┬────────┴────────┬────────┴────────┬────────────────────┘
         │                 │                 │
         └────────────────VIP-APP: 192.168.15.200──────────────────┐
                                                                    │
                                                          Cloudflare Tunnel
                                                          (HA - 3 replicas)
                                                                    │
┌─────────────────────────────────────────────────────────────────┐
│                 BARE METAL DATABASES                             │
├─────────────────┬─────────────────┬─────────────────────────────┤
│    db-node1     │    db-node2     │    db-node3                 │
│  192.168.15.120 │  192.168.15.121 │  192.168.15.122             │
│                 │                 │                              │
│  PostgreSQL 17  │  PostgreSQL 17  │  PostgreSQL 17              │
│  (Primary)      │  (Replica)      │  (Replica)                  │
│                 │                 │                              │
│  keepalived     │  keepalived     │  keepalived                 │
│  (priority 100) │  (priority 90)  │  (priority 80)              │
│     8GB RAM     │     8GB RAM     │     8GB RAM                 │
└────────┬────────┴────────┬────────┴────────┬────────────────────┘
         │                 │                 │
         └────────────────VIP-DB: 192.168.15.210───────────────────┘
```

### Benefits of This Architecture
| Aspect | Previous (CNPG in K8s) | Current (Bare Metal DB) |
|--------|------------------------|-------------------------|
| **Database Reliability** | Longhorn PVC issues | Direct disk, no storage layer |
| **Backup/Recovery** | Complex CNPG snapshots | Simple pg_dump, rsync |
| **Performance** | Container/CSI overhead | Native performance |
| **Debugging** | kubectl exec, complex | Direct SSH access |
| **Failover** | CNPG automatic | Keepalived VIP + streaming replication |

---

## Proxmox Virtual Machines

### VM Configuration

#### Kubernetes Cluster VMs
| VM ID | Hostname   | IP Address      | CPU | RAM  | Disk  | Role                    |
|-------|------------|-----------------|-----|------|-------|-------------------------|
| 210   | k3s-node1  | 192.168.15.110  | 4   | 12GB | 100GB | Control Plane + Worker  |
| 211   | k3s-node2  | 192.168.15.111  | 4   | 12GB | 80GB  | Control Plane + Worker  |
| 212   | k3s-node3  | 192.168.15.112  | 4   | 12GB | 80GB  | Control Plane + Worker  |

#### Database VMs (Bare Metal PostgreSQL)
| VM ID | Hostname   | IP Address      | CPU | RAM  | Disk | Role               |
|-------|------------|-----------------|-----|------|------|--------------------|
| 220   | db-node1   | 192.168.15.120  | 4   | 8GB  | 50GB | PostgreSQL Primary |
| 221   | db-node2   | 192.168.15.121  | 2   | 8GB  | 50GB | PostgreSQL Replica |
| 222   | db-node3   | 192.168.15.122  | 2   | 8GB  | 50GB | PostgreSQL Replica |

**Total Resources Allocated:**
- CPU: 20 cores
- RAM: 60 GB (36GB K8s + 24GB Databases)
- Disk: 410 GB

### VM Creation Commands

```bash
# VM configuration variables
STORAGE="hdd"
BRIDGE="vmbr0"
GATEWAY="192.168.15.1"

# Create VMs (executed on Proxmox host)
# VM 210 - k3s-node1
qm create 210 --name k3s-node1 --cores 16 --memory 8192 \
  --net0 virtio,bridge=$BRIDGE --scsihw virtio-scsi-pci \
  --ostype l26 --cpu host --agent 1
qm set 210 --scsi0 $STORAGE:100
qm set 210 --ipconfig0 ip=192.168.15.110/24,gw=$GATEWAY
qm set 210 --nameserver 8.8.8.8
qm start 210

# Similar commands for VMs 211-214...
# (See VM Configuration table for specific parameters)
```

### Network Configuration
- **Subnet:** 192.168.15.0/24
- **Gateway:** 192.168.15.1
- **DNS:** 8.8.8.8
- **Static IPs:** 192.168.15.110-114

### VM Preparation (executed on all nodes)

```bash
# Update system
apt-get update -qq && apt-get upgrade -y -qq

# Install required packages
apt-get install -y -qq curl wget git htop iotop nfs-common open-iscsi

# Disable swap (required for Kubernetes)
swapoff -a
sed -i '/ swap / s/^/#/' /etc/fstab

# Enable IP forwarding
echo "net.ipv4.ip_forward = 1" >> /etc/sysctl.conf
echo "net.ipv6.conf.all.forwarding = 1" >> /etc/sysctl.conf
sysctl -p

# Enable br_netfilter for Kubernetes
modprobe br_netfilter
echo "br_netfilter" >> /etc/modules-load.d/k8s.conf
```

---

## K3s Kubernetes Cluster

### Cluster Information
- **K3s Version:** v1.33.6+k3s1
- **Control Plane Nodes:** 3 (HA setup with embedded etcd)
- **Worker Nodes:** 2
- **Container Runtime:** containerd 2.1.5-k3s1.33
- **Disabled Components:** Traefik (ingress), ServiceLB (load balancer)

### Installation Commands

#### Node 1 (Primary Control Plane)
```bash
curl -sfL https://get.k3s.io | sh -s - server \
  --cluster-init \
  --disable traefik \
  --disable servicelb \
  --write-kubeconfig-mode 644 \
  --tls-san 192.168.15.110
```

#### Get Join Token
```bash
cat /var/lib/rancher/k3s/server/node-token
# Token: K109aa2849f9907ed1931a7d6f91f6a95ec6cfdb706cbeba31744cf343704e5c86a::server:8c117784b0f9da6d0a1d533bd91d3850
```

#### Nodes 2-3 (Additional Control Planes)
```bash
curl -sfL https://get.k3s.io | sh -s - server \
  --server https://192.168.15.110:6443 \
  --token K109aa2849f9907ed1931a7d6f91f6a95ec6cfdb706cbeba31744cf343704e5c86a::server:8c117784b0f9da6d0a1d533bd91d3850 \
  --disable traefik \
  --disable servicelb \
  --write-kubeconfig-mode 644
```

#### Nodes 4-5 (Worker Nodes)
```bash
curl -sfL https://get.k3s.io | \
  K3S_URL=https://192.168.15.110:6443 \
  K3S_TOKEN=K109aa2849f9907ed1931a7d6f91f6a95ec6cfdb706cbeba31744cf343704e5c86a::server:8c117784b0f9da6d0a1d533bd91d3850 \
  sh -
```

### Cluster Access

#### From Control Plane Nodes
```bash
kubectl get nodes
kubectl get pods --all-namespaces
```

#### From Local Machine (copy kubeconfig)
```bash
# Copy from k3s-node1
scp root@192.168.15.110:/etc/rancher/k3s/k3s.yaml ~/.kube/config

# Edit the server address
sed -i 's/127.0.0.1/192.168.15.110/g' ~/.kube/config

# Test access
kubectl get nodes
```

---

## Cloudflare Tunnel HA

### Overview
Cloudflare Tunnel provides secure, zero-trust access to the application without exposing ports to the internet. The tunnel runs as a highly available deployment across all 3 K8s nodes.

### Configuration

| Setting | Value |
|---------|-------|
| Deployment | cloudflared (3 replicas) |
| Target | VIP-APP (192.168.15.200:8080) |
| Testing Domain | ailakshya.in |
| Production Domain | gurukripacoldstore.in |

### Tunnel Architecture
```
Internet → Cloudflare Edge → cloudflared pods (x3) → VIP-APP (192.168.15.200)
                                    ↓
                            Backend Pods (K8s)
                                    ↓
                            VIP-DB (192.168.15.210)
```

### Check Tunnel Status
```bash
# Check cloudflared pods
kubectl get pods -l app=cloudflared

# Check logs
kubectl logs -l app=cloudflared --tail=50

# Verify pods are distributed across nodes
kubectl get pods -l app=cloudflared -o wide
```

### Tunnel Failover
- Cloudflare automatically load-balances between healthy cloudflared instances
- If one node dies, traffic routes to remaining nodes
- No single point of failure

---

## Bare Metal PostgreSQL Cluster

### Overview
PostgreSQL 17 with streaming replication across 3 bare metal VMs, using keepalived for VIP failover.

### Cluster Configuration

| Node | IP Address | Role | Keepalived Priority |
|------|------------|------|---------------------|
| db-node1 | 192.168.15.120 | Primary | 100 |
| db-node2 | 192.168.15.121 | Streaming Replica | 90 |
| db-node3 | 192.168.15.122 | Streaming Replica | 80 |
| **VIP-DB** | **192.168.15.210** | Virtual IP | - |

### Connection Information

#### Application Connection (via VIP)
```bash
# Connection string (always use VIP)
postgresql://cold_user:SecurePostgresPassword123@192.168.15.210:5432/cold_db

# Test connection
psql -h 192.168.15.210 -U cold_user -d cold_db
```

#### Direct Node Connections (for maintenance)
```bash
# Primary
ssh root@192.168.15.120
sudo -u postgres psql -d cold_db

# Replicas
ssh root@192.168.15.121
ssh root@192.168.15.122
```

### Check Replication Status
```bash
# On primary (db-node1)
ssh root@192.168.15.120 'sudo -u postgres psql -c "SELECT client_addr, state, sent_lsn, replay_lsn FROM pg_stat_replication;"'

# Check if node is primary or replica
ssh root@192.168.15.120 'sudo -u postgres psql -c "SELECT pg_is_in_recovery();"'
# Returns: f (false) = Primary, t (true) = Replica
```

### Keepalived VIP Management
```bash
# Check which node has VIP
ssh root@192.168.15.120 'ip addr show eth0 | grep 192.168.15.210'
ssh root@192.168.15.121 'ip addr show eth0 | grep 192.168.15.210'
ssh root@192.168.15.122 'ip addr show eth0 | grep 192.168.15.210'

# Restart keepalived
ssh root@192.168.15.120 'systemctl restart keepalived'
```

### PostgreSQL Service Management
```bash
# Status
ssh root@192.168.15.120 'systemctl status postgresql@17-main'

# Restart
ssh root@192.168.15.120 'systemctl restart postgresql'

# Logs
ssh root@192.168.15.120 'journalctl -u postgresql@17-main --tail=50'
```

### Key Configuration Files
- `/etc/postgresql/17/main/postgresql.conf` - Main config
- `/etc/postgresql/17/main/pg_hba.conf` - Access control
- `/var/lib/postgresql/17/main/postgresql.auto.conf` - Replica connection info
- `/etc/keepalived/keepalived.conf` - VIP failover config

---

## TrueNAS NFS (Optional)

Available for future use if large file storage is needed:

| Setting | Value |
|---------|-------|
| Server | 192.168.15.52 |
| Share | coman |
| Capacity | 24 TB |
| Mount Point | /mnt/data/coman |

```bash
# Mount on any node
mount -t nfs 192.168.15.52:/mnt/pool/coman /mnt/data/coman
```

---

## Access Information

### SSH Access

#### Proxmox Host
```bash
ssh root@192.168.15.96
```

#### K3s Nodes (Application Cluster)
```bash
ssh root@192.168.15.110  # k3s-node1
ssh root@192.168.15.111  # k3s-node2
ssh root@192.168.15.112  # k3s-node3
```

#### Database Nodes (Bare Metal PostgreSQL)
```bash
ssh root@192.168.15.120  # db-node1 (Primary)
ssh root@192.168.15.121  # db-node2 (Replica)
ssh root@192.168.15.122  # db-node3 (Replica)
```

### Kubernetes Access

#### From K3s Control Plane Nodes
```bash
kubectl get nodes
kubectl get pods -A
```

#### Remote Access (copy kubeconfig)
```bash
scp root@192.168.15.110:/etc/rancher/k3s/k3s.yaml ~/.kube/config
sed -i 's/127.0.0.1/192.168.15.110/g' ~/.kube/config
```

### Database Access

#### Via VIP (Recommended)
```bash
# Connect via VIP - always points to primary
psql -h 192.168.15.210 -U cold_user -d cold_db
# Password: SecurePostgresPassword123
```

#### Direct to Primary
```bash
ssh root@192.168.15.120
sudo -u postgres psql -d cold_db
```

#### From Backend Pod
```bash
# The backend connects to VIP-DB automatically
kubectl exec -it deploy/cold-backend-employee -- env | grep DB
```

---

## Maintenance Commands

### K3s Cluster Management

#### Check Cluster Status
```bash
kubectl get nodes
kubectl get pods -A
kubectl cluster-info
```

#### Restart K3s Service
```bash
# On control plane nodes
systemctl restart k3s

# On worker nodes
systemctl restart k3s-agent
```

#### Uninstall K3s
```bash
# On control plane nodes
/usr/local/bin/k3s-uninstall.sh

# On worker nodes
/usr/local/bin/k3s-agent-uninstall.sh
```

### PostgreSQL Management (Bare Metal)

#### Check Cluster Status
```bash
# Check which node is primary
for node in 192.168.15.120 192.168.15.121 192.168.15.122; do
  echo "=== $node ==="
  ssh root@$node 'sudo -u postgres psql -c "SELECT pg_is_in_recovery();"'
done
```

#### Check Replication Status
```bash
ssh root@192.168.15.120 'sudo -u postgres psql -c "SELECT client_addr, state, sent_lsn, replay_lsn FROM pg_stat_replication;"'
```

#### Check VIP Location
```bash
for node in 192.168.15.120 192.168.15.121 192.168.15.122; do
  echo "=== $node ==="
  ssh root@$node 'ip addr show eth0 | grep 192.168.15.210 || echo "VIP not here"'
done
```

#### Backup Database
```bash
# Full backup from primary
ssh root@192.168.15.120 'sudo -u postgres pg_dump cold_db > /var/backups/cold_db_$(date +%Y%m%d).sql'

# Backup to Cloudflare R2 (automatic via backend)
curl -X POST http://192.168.15.200:8080/api/infrastructure/backup-now -H "Authorization: Bearer $TOKEN"
```

#### Restart PostgreSQL
```bash
ssh root@192.168.15.120 'systemctl restart postgresql'
```

#### Restart Keepalived (force VIP failover)
```bash
ssh root@192.168.15.120 'systemctl stop keepalived'
# VIP will move to next priority node
```

### Proxmox VM Management

#### Start/Stop VMs
```bash
# On Proxmox host
qm start 210   # Start VM 210
qm stop 210    # Stop VM 210
qm shutdown 210  # Graceful shutdown
qm list        # List all VMs
```

#### Delete VMs
```bash
qm destroy 210 --purge  # Delete VM and storage
```

#### VM Snapshots
```bash
qm snapshot 210 snapshot1  # Create snapshot
qm listsnapshot 210        # List snapshots
qm rollback 210 snapshot1  # Restore snapshot
```

---

## Completed Milestones

### ✓ Infrastructure Migration (December 2025)
- Migrated from CNPG (K8s-based PostgreSQL) to bare metal PostgreSQL
- Set up 3-node PostgreSQL cluster with streaming replication
- Configured keepalived VIP for automatic failover
- Deleted old k3s-node4 and k3s-node5

### ✓ Cloudflare Tunnel HA
- Deployed cloudflared as 3-replica deployment
- Tunnel distributed across all K8s nodes
- Zero-trust access configured for ailakshya.in

### ✓ Backend Deployment
- cold-backend running on K8s (employee + customer portals)
- Automatic database fallback (VIP-DB → Backup → Localhost)
- Cloudflare R2 backup integration

### ✓ Monitoring System (v1.5.0)
- TimescaleDB for metrics storage
- Node and PostgreSQL metrics collection
- Infrastructure dashboard at `/infrastructure`

---

## Future Improvements

### Optional: NFS Storage
- Configure TrueNAS NFS export for "coman" share
- Use for large file storage if needed

### Production Deployment
- Switch to gurukripacoldstore.in domain
- Final security audit
- Performance testing under load

---

## Troubleshooting

### K3s Issues

#### Node Not Ready
```bash
# Check node status
kubectl describe node k3s-node1

# Check kubelet logs
journalctl -u k3s -f  # Control plane
journalctl -u k3s-agent -f  # Worker
```

#### Pods Not Starting
```bash
kubectl describe pod <pod-name>
kubectl logs <pod-name>
kubectl get events --sort-by='.lastTimestamp'
```

### Longhorn Issues

#### Volume Not Provisioning
```bash
kubectl get pvc
kubectl describe pvc <pvc-name>
kubectl logs -n longhorn-system -l app=longhorn-manager
```

### PostgreSQL Issues (Bare Metal)

#### Cannot Connect to VIP
```bash
# Check which node has VIP
for node in 192.168.15.120 192.168.15.121 192.168.15.122; do
  ssh root@$node 'ip addr show eth0 | grep 192.168.15.210' 2>/dev/null && echo "VIP is on $node"
done

# Check keepalived status
ssh root@192.168.15.120 'systemctl status keepalived'

# Restart keepalived on all nodes
for node in 192.168.15.120 192.168.15.121 192.168.15.122; do
  ssh root@$node 'systemctl restart keepalived'
done
```

#### Replication Not Working
```bash
# Check replication status on primary
ssh root@192.168.15.120 'sudo -u postgres psql -c "SELECT * FROM pg_stat_replication;"'

# Check if replica is receiving WAL
ssh root@192.168.15.121 'sudo -u postgres psql -c "SELECT pg_last_wal_receive_lsn(), pg_last_wal_replay_lsn();"'

# Check PostgreSQL logs
ssh root@192.168.15.121 'journalctl -u postgresql@17-main --tail=100'
```

#### Connection from Backend Failing
```bash
# Check backend pod logs
kubectl logs -l app=cold-backend --tail=100

# Test connection from K8s node
psql -h 192.168.15.210 -U cold_user -d cold_db -c "SELECT 1;"

# Check backend database config
kubectl exec deploy/cold-backend-employee -- env | grep -i db
```

---

## Disaster Recovery

### Database Recovery Scenarios

#### Scenario 1: Single Replica Failure
```bash
# 1. Check which node failed
for node in 192.168.15.120 192.168.15.121 192.168.15.122; do
  ssh root@$node 'hostname' 2>/dev/null || echo "$node is DOWN"
done

# 2. If it's a replica, just restart it - replication will catch up
ssh root@192.168.15.121 'systemctl restart postgresql'
```

#### Scenario 2: Primary Failure (VIP Failover)
```bash
# 1. Keepalived automatically moves VIP to next priority replica
# 2. Application continues working (read-only until promotion)

# 3. To promote replica to primary:
ssh root@192.168.15.121 'sudo -u postgres pg_ctl promote -D /var/lib/postgresql/17/main'

# 4. Update other replicas to follow new primary
# Edit /var/lib/postgresql/17/main/postgresql.auto.conf
```

#### Scenario 3: Complete Database Loss
```bash
# 1. Backend falls back to backup server (192.168.15.195)
# 2. If that fails, falls back to localhost

# 3. Restore from Cloudflare R2:
# Backend has automatic recovery - will download latest backup
# Or manually:
aws s3 cp s3://cold-db-backups/latest.sql.gz - | gunzip | psql cold_db
```

### Backend Recovery
```bash
# Backend pods auto-restart on failure
kubectl get pods -l app=cold-backend

# Force restart all backend pods
kubectl rollout restart deployment/cold-backend-employee
kubectl rollout restart deployment/cold-backend-customer
```

### Full Cluster Recovery
1. **Proxmox VMs**: Restore from Proxmox Backup Server (if configured)
2. **PostgreSQL**: Restore from Cloudflare R2 backup
3. **K8s Deployments**: Apply manifests from `/home/lakshya/jupyter-/cold/cold-backend/k8s/`

---

## Important Notes

1. **Database Connection**: Application always connects to VIP (192.168.15.210), never directly to nodes.

2. **Backups**: Automatic backups to Cloudflare R2 (cold-db-backups bucket).

3. **High Availability**:
   - K8s: 3 control plane nodes, backend replicas spread across nodes
   - Database: 3-node PostgreSQL with streaming replication + keepalived VIP

4. **Monitoring**: Check `/infrastructure` dashboard for cluster health.

5. **Updates**:
   - K3s: `curl -sfL https://get.k3s.io | sh -` on each node
   - PostgreSQL: `apt update && apt upgrade postgresql-17`

---

## References

- K3s Documentation: https://docs.k3s.io/
- PostgreSQL Streaming Replication: https://www.postgresql.org/docs/17/warm-standby.html
- Keepalived: https://keepalived.readthedocs.io/
- Proxmox VE Documentation: https://pve.proxmox.com/wiki/
- Cloudflare Tunnel: https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/

---

**Last Updated:** December 24, 2025
**Version:** v1.5.172 (Monitoring Dashboard Fix)
**Maintained By:** Lakshya (M.Tech CSE AI&ML)
