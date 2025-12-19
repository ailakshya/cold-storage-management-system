# K3s Infrastructure Documentation - Cold Storage Backend POC

**Date:** December 18, 2025
**Testing Domain:** ailakshya.in
**Production Domain:** gurukripacoldstore.in

---

## Table of Contents

1. [Infrastructure Overview](#infrastructure-overview)
2. [Proxmox Virtual Machines](#proxmox-virtual-machines)
3. [K3s Kubernetes Cluster](#k3s-kubernetes-cluster)
4. [Longhorn Distributed Storage](#longhorn-distributed-storage)
5. [CloudNativePG PostgreSQL Cluster](#cloudnativepg-postgresql-cluster)
6. [TrueNAS NFS Mount](#truenas-nfs-mount)
7. [Access Information](#access-information)
8. [Maintenance Commands](#maintenance-commands)
9. [Next Steps](#next-steps)

---

## Infrastructure Overview

### Physical Server
- **Host:** Proxmox VE at 192.168.15.96
- **CPU:** 2× Intel Xeon Gold 6138 @ 2.00GHz (80 cores total)
- **RAM:** 125.50 GB (36 GB free before VM creation)
- **Storage:** 4 TB HDD (local storage pool: "hdd")
- **Network:** Bridge vmbr0, Gateway 192.168.15.1

### TrueNAS Server
- **Address:** 192.168.15.52
- **Share Path:** smb://192.168.15.52/coman/
- **Mount Point (planned):** /mnt/data/coman
- **Storage:** 24 TB ZFS share
- **Note:** NFS needs to be enabled on TrueNAS for Kubernetes integration

### Architecture
```
┌─────────────────────────────────────────────────────────────┐
│  Proxmox Server (192.168.15.96)                             │
│                                                               │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  K3s Cluster (5 VMs)                                   │  │
│  │                                                         │  │
│  │  Control Plane Nodes:                                  │  │
│  │  ├─ k3s-node1 (192.168.15.110) - Primary control plane│  │
│  │  ├─ k3s-node2 (192.168.15.111) - Control plane        │  │
│  │  └─ k3s-node3 (192.168.15.112) - Control plane        │  │
│  │                                                         │  │
│  │  Worker Nodes:                                         │  │
│  │  ├─ k3s-node4 (192.168.15.113) - Worker               │  │
│  │  └─ k3s-node5 (192.168.15.114) - Worker               │  │
│  │                                                         │  │
│  │  Storage Layer:                                        │  │
│  │  ├─ Longhorn (distributed storage across all nodes)   │  │
│  │  └─ TrueNAS NFS (24TB external storage) [pending]     │  │
│  │                                                         │  │
│  │  Database Layer:                                       │  │
│  │  └─ CloudNativePG PostgreSQL (5 replicas)             │  │
│  │     ├─ cold-postgres-1 (primary)                      │  │
│  │     ├─ cold-postgres-2 (replica)                      │  │
│  │     ├─ cold-postgres-3 (replica)                      │  │
│  │     ├─ cold-postgres-4 (replica)                      │  │
│  │     └─ cold-postgres-5 (replica)                      │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

---

## Proxmox Virtual Machines

### VM Configuration

| VM ID | Hostname   | IP Address      | CPU Cores | RAM  | Disk | Role          |
|-------|------------|-----------------|-----------|------|------|---------------|
| 210   | k3s-node1  | 192.168.15.110  | 16        | 8GB  | 100GB| Control Plane |
| 211   | k3s-node2  | 192.168.15.111  | 12        | 6GB  | 80GB | Control Plane |
| 212   | k3s-node3  | 192.168.15.112  | 12        | 6GB  | 80GB | Control Plane |
| 213   | k3s-node4  | 192.168.15.113  | 10        | 8GB  | 60GB | Worker        |
| 214   | k3s-node5  | 192.168.15.114  | 10        | 8GB  | 60GB | Worker        |

**Total Resources Allocated:**
- CPU: 60 cores
- RAM: 36 GB
- Disk: 420 GB

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

## Longhorn Distributed Storage

### Overview
Longhorn provides distributed block storage for Kubernetes persistent volumes.

### Installation
```bash
kubectl apply -f https://raw.githubusercontent.com/longhorn/longhorn/v1.7.2/deploy/longhorn.yaml
```

### Storage Classes
- **longhorn** (default): Distributed storage with 3 replicas
- **longhorn-static**: Static provisioning
- **local-path** (K3s default): Local storage on single node

### Longhorn Components
- **Namespace:** longhorn-system
- **UI Service:** longhorn-frontend (access via NodePort or kubectl port-forward)
- **Manager DaemonSet:** Runs on all nodes (5 instances)
- **Engine Images:** Deployed on all nodes
- **Instance Managers:** Handle volume replicas

### Access Longhorn UI
```bash
# Port-forward to access UI
kubectl port-forward -n longhorn-system svc/longhorn-frontend 8080:80

# Access at: http://localhost:8080
```

### Verify Installation
```bash
# Check pods
kubectl get pods -n longhorn-system

# Check storage classes
kubectl get storageclass

# Check volumes
kubectl get pvc --all-namespaces
```

---

## CloudNativePG PostgreSQL Cluster

### Overview
CloudNativePG operator manages PostgreSQL clusters with high availability.

### Operator Installation
```bash
kubectl apply --server-side -f https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-1.23/releases/cnpg-1.23.6.yaml
```

### PostgreSQL Cluster Configuration

**File:** `/tmp/postgres-cluster.yaml`

```yaml
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: cold-postgres
  namespace: default
spec:
  instances: 5
  imageName: ghcr.io/cloudnative-pg/postgresql:17.2

  postgresql:
    parameters:
      max_connections: "200"
      shared_buffers: "256MB"
      effective_cache_size: "1GB"
      work_mem: "16MB"

  storage:
    storageClass: longhorn
    size: 20Gi

  walStorage:
    storageClass: longhorn
    size: 5Gi

  resources:
    requests:
      memory: "1Gi"
      cpu: "500m"
    limits:
      memory: "2Gi"
      cpu: "2000m"

  bootstrap:
    initdb:
      database: cold_db
      owner: cold_user
      secret:
        name: cold-postgres-superuser

  monitoring:
    enablePodMonitor: false
```

### Create Superuser Secret
```bash
kubectl create secret generic cold-postgres-superuser \
  --from-literal=username=postgres \
  --from-literal=password=SecurePostgresPassword123
```

### Deploy Cluster
```bash
kubectl apply -f /tmp/postgres-cluster.yaml
```

### Cluster Details
- **Name:** cold-postgres
- **Database:** cold_db
- **Owner:** cold_user
- **Instances:** 5 (1 primary + 4 replicas)
- **PostgreSQL Version:** 17.2
- **Storage:** 20Gi data + 5Gi WAL per instance (Longhorn)
- **Total Storage:** 125Gi (25Gi × 5 instances)

### Connection Information

#### Primary Connection (read-write)
```bash
# Service: cold-postgres-rw (read-write)
kubectl get service cold-postgres-rw

# Connection string
postgresql://cold_user:<password>@cold-postgres-rw.default.svc.cluster.local:5432/cold_db
```

#### Replica Connection (read-only)
```bash
# Service: cold-postgres-ro (read-only)
kubectl get service cold-postgres-ro

# Connection string
postgresql://cold_user:<password>@cold-postgres-ro.default.svc.cluster.local:5432/cold_db
```

#### Get Superuser Password
```bash
kubectl get secret cold-postgres-superuser -o jsonpath='{.data.password}' | base64 -d
```

### Monitoring
```bash
# Check cluster status
kubectl get cluster cold-postgres

# Check pods
kubectl get pods -l cnpg.io/cluster=cold-postgres

# Check primary instance
kubectl get cluster cold-postgres -o jsonpath='{.status.currentPrimary}'

# View logs
kubectl logs -l cnpg.io/cluster=cold-postgres --tail=100
```

---

## TrueNAS NFS Mount

### Configuration (Pending)

**TrueNAS Server:** 192.168.15.52
**Share Name:** coman
**SMB Path:** smb://192.168.15.52/coman/
**Planned Mount Point:** /mnt/data/coman
**Storage Capacity:** 24 TB

### Setup Required on TrueNAS

1. **Enable NFS Service**
   - Navigate to Services → NFS
   - Start NFS service
   - Enable "Start Automatically"

2. **Configure NFS Export**
   - Navigate to Sharing → Unix (NFS) Shares
   - Edit the "coman" dataset
   - Add NFS share with:
     - Path: /mnt/pool/coman
     - Networks: 192.168.15.0/24 (or specific IPs)
     - Maproot User: root
     - Maproot Group: wheel

3. **Apply Changes**
   - Click Save
   - Restart NFS service

### Verify NFS Export (from K3s node)
```bash
# Check available exports
showmount -e 192.168.15.52

# Expected output:
# Export list for 192.168.15.52:
# /mnt/pool/coman 192.168.15.0/24
```

### Mount on K3s Nodes
```bash
# Create mount point on all nodes
mkdir -p /mnt/data/coman

# Test mount
mount -t nfs 192.168.15.52:/mnt/pool/coman /mnt/data/coman

# Verify
df -h | grep coman

# Make permanent (add to /etc/fstab)
echo "192.168.15.52:/mnt/pool/coman /mnt/data/coman nfs defaults 0 0" >> /etc/fstab
```

### Kubernetes NFS StorageClass (after NFS is configured)

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: truenas-nfs
provisioner: nfs.csi.k8s.io
parameters:
  server: 192.168.15.52
  share: /mnt/pool/coman
  mountPermissions: "0755"
volumeBindingMode: Immediate
```

---

## Access Information

### SSH Access

#### Proxmox Host
```bash
ssh root@192.168.15.96
```

#### K3s Nodes
```bash
ssh root@192.168.15.110  # k3s-node1
ssh root@192.168.15.111  # k3s-node2
ssh root@192.168.15.112  # k3s-node3
ssh root@192.168.15.113  # k3s-node4
ssh root@192.168.15.114  # k3s-node5
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

#### Connect to PostgreSQL Primary
```bash
# Port-forward to local machine
kubectl port-forward svc/cold-postgres-rw 5432:5432

# Connect with psql
psql -h localhost -U cold_user -d cold_db
```

#### Connect from within cluster
```bash
# Exec into postgres pod
kubectl exec -it cold-postgres-1 -- psql -U postgres cold_db
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

### Longhorn Management

#### Check Storage Usage
```bash
kubectl get pv
kubectl get pvc -A
```

#### Uninstall Longhorn
```bash
kubectl delete -f https://raw.githubusercontent.com/longhorn/longhorn/v1.7.2/deploy/longhorn.yaml
```

### PostgreSQL Management

#### Scale Cluster (change replica count)
```bash
kubectl patch cluster cold-postgres --type='json' \
  -p='[{"op": "replace", "path": "/spec/instances", "value": 3}]'
```

#### Trigger Manual Failover
```bash
kubectl cnpg promote cold-postgres cold-postgres-2
```

#### Backup Database
```bash
kubectl cnpg backup cold-postgres
```

#### Check Replication Status
```bash
kubectl exec cold-postgres-1 -- psql -U postgres -c \
  "SELECT * FROM pg_stat_replication;"
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

## Next Steps

### 1. Enable NFS on TrueNAS
- Configure NFS export for "coman" share
- Mount NFS share on K3s nodes
- Create Kubernetes NFS StorageClass

### 2. Deploy MinIO (S3-compatible storage)
- Install MinIO operator
- Create MinIO tenant
- Configure PostgreSQL backups to MinIO

### 3. Build and Deploy cold-backend Application
- Build Go application as container
- Create Kubernetes Deployment
- Create Kubernetes Service
- Connect to PostgreSQL cluster

### 4. Configure Cloudflare Tunnel
- Install cloudflared
- Create tunnel for ailakshya.in
- Configure DNS records
- Set up ingress rules

### 5. Testing and Validation
- Test high availability failover
- Test PostgreSQL replication
- Test backup and restore
- Load testing
- Disaster recovery testing

### 6. Monitoring and Observability ✓ COMPLETED (v1.5.0)
**TimescaleDB Monitoring System** has been deployed with:

#### Components
- **TimescaleDB Pod**: Time-series database for metrics storage
  - Service: `timescaledb.default.svc.cluster.local:5432`
  - Database: `metrics_db`, User: `metrics`
  - Storage: 10Gi on Longhorn

- **Metrics Collector**: Collects every 30 seconds
  - K3s node metrics (CPU, memory, disk, network, pods)
  - PostgreSQL pod metrics (connections, replication lag)

- **API Request Logging**: Tracks all API requests
  - Duration, status codes, user info
  - Error rates and response sizes

- **Monitoring Dashboard**: `/monitoring` page
  - Real-time cluster overview
  - Historical charts (5m to 7d range)
  - API analytics and performance
  - Alert management

#### API Endpoints (Admin only)
```
GET /api/monitoring/dashboard     - Full dashboard data
GET /api/monitoring/nodes         - Node metrics
GET /api/monitoring/postgres      - PostgreSQL metrics
GET /api/monitoring/api/analytics - API performance stats
GET /api/monitoring/alerts        - System alerts
```

#### TimescaleDB Features
- Hypertables with automatic partitioning
- Data compression (after 7 days)
- Automatic retention policies (30-90 days)
- Continuous aggregates for fast queries

### 7. Production Deployment
- Switch to gurukripacoldstore.in domain
- Configure SSL certificates
- Set up automated backups
- Document runbooks

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

### PostgreSQL Issues

#### Cluster Not Starting
```bash
kubectl describe cluster cold-postgres
kubectl logs -n cnpg-system deployment/cnpg-controller-manager
kubectl logs -l cnpg.io/cluster=cold-postgres
```

#### Connection Issues
```bash
# Check services
kubectl get svc | grep cold-postgres

# Check endpoints
kubectl get endpoints cold-postgres-rw

# Test connection from pod
kubectl run -it --rm psql-test --image=postgres:17 -- \
  psql postgresql://cold_user:SecurePostgresPassword123@cold-postgres-rw/cold_db
```

---

## Important Notes

1. **Security:** All passwords shown here are examples. Change them in production.

2. **Backups:** Currently, PostgreSQL backups are not configured. Add MinIO for automated backups.

3. **High Availability:** The cluster is configured for HA but requires proper testing.

4. **Resource Monitoring:** Install monitoring tools to track CPU/memory/disk usage.

5. **Networking:** Ensure firewall rules allow traffic between nodes.

6. **Storage:** Monitor Longhorn storage usage and plan capacity accordingly.

7. **Updates:** Keep K3s, Longhorn, and PostgreSQL versions updated.

8. **Documentation:** Keep this document updated as infrastructure changes.

---

## References

- K3s Documentation: https://docs.k3s.io/
- Longhorn Documentation: https://longhorn.io/docs/
- CloudNativePG Documentation: https://cloudnative-pg.io/documentation/
- Proxmox VE Documentation: https://pve.proxmox.com/wiki/
- TrueNAS Scale Documentation: https://www.truenas.com/docs/scale/

---

**Last Updated:** December 18, 2025
**Version:** v1.5.0 (with TimescaleDB Monitoring)
**Maintained By:** Lakshya (M.Tech CSE AI&ML)
