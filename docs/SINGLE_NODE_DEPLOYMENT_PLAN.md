# Single Node Deployment Roadmap

## 1. Executive Summary
**Goal**: Deploy the Cold Storage Management System on a single robust physical server.
**Hardware**:
- **Compute**: Single Node
- **Fast Storage (SSD)**: 4x 1TB SSDs
- **Mass Storage (HDD)**: 3x 4TB HDDs
**Architecture**:
- **OS**: Linux (Ubuntu 24.04 LTS recommended) with ZFS.
- **Orchestration**: K3s (Lightweight Kubernetes).
- **Database**: PostgreSQL (CloudNativePG operator).
- **Filesystem Strategy**:
    - **Performance Pool (SSDs)**: RAID 10 (Striped Mirrors) for OS, Databases, and Application Containers.
    - **Capacity Pool (HDDs)**: RAIDZ1 for Backups, Logs, and Cold Storage.

---

## 2. Storage Architecture Strategy (ZFS)

We will use **ZFS** to manage the disks. This provides:
- **Redundancy**: Protection against drive failures.
- **Performance**: Striping across mirrors for high IOPS.
- **Integrity**: Bit-rot protection and self-healing.

### 2.1 Fast Pool (`fast-pool`) - 4x SSDs
**Topology**: Striped Mirrors (Equivalent to RAID 10).
**Capacity**: ~2 TB Usable.
**Use Case**:
- Host Operating System (`/`)
- K3s Data (`/var/lib/rancher/k3s`)
- Database Storage (Postgres, Redis)
- Active Application Logs

**Why?**
You asked for a "full mirror system". Using a striped mirror (2 vdevs of 2 mirror drives) provides the best balance of write speed (2x write speed of single drive) and redundancy (can survive 1-2 drive failures).

### 2.2 Storage Pool (`mass-pool`) - 3x HDDs
**Topology**: RAIDZ1 (Single Parity).
**Capacity**: ~8 TB Usable.
**Use Case**:
- Database Backups (WAL archives, nightly dumps)
- Long-term log retention
- Static Assets / Cold Data

**Why?**
With 3 drives, a 3-way mirror would only give 4TB usable. RAIDZ1 gives ~8TB while still allowing 1 drive to fail without data loss.

---

## 3. Implementation Steps

### Phase 1: OS & ZFS Setup (Pre-Kubernetes)

1.  **Install OS**: Install Ubuntu Server on the SSDs.
    - *Recommend*: Install OS on a small partition of the SSDs (mdadm RAID 1) OR install on ZFS Root directly (advanced).
    - *Easier Alternative*: Install OS on SSD 1 (or a separate small boot drive if available), and use the remaining capacity for the ZFS pool.
    - *Assuming standard install*: Let's assume you install OS on a ZFS Root using the Ubuntu Installer (select "Advanced" -> "Experimental ZFS").

2.  **Verify/Create Pools**:
    ```bash
    # (If OS is not on ZFS, or to configure the remaining disks)
    
    # 1. Create the Fast Pool (Performance)
    # Replaces RAID 10. Two mirrors striped together.
    zpool create -f fast-pool \
      mirror /dev/disk/by-id/ssd-1 /dev/disk/by-id/ssd-2 \
      mirror /dev/disk/by-id/ssd-3 /dev/disk/by-id/ssd-4

    # 2. Create the Mass Pool (Capacity)
    # RAIDZ1 for 3 drives.
    zpool create -f mass-pool raidz1 \
      /dev/disk/by-id/hdd-1 \
      /dev/disk/by-id/hdd-2 \
      /dev/disk/by-id/hdd-3

    # 3. Enable Compression (Free speed/space)
    zfs set compression=lz4 fast-pool
    zfs set compression=lz4 mass-pool
    ```

### Phase 2: K3s Installation

We will use K3s for its simplicity and single-binary architecture.

1.  **Install K3s**:
    ```bash
    curl -sfL https://get.k3s.io | sh -
    ```

2.  **Configure Storage Class**:
    K3s uses the `local-path-provisioner` by default. We need to point it to our fast storage.

    **Option A: Link Default Storage to Fast Pool** (Simplest)
    ```bash
    # Create directory on fast pool
    mkdir -p /fast-pool/k3s-storage
    
    # Edit the local-path config to point here
    kubectl edit configmap local-path-config -n kube-system
    ```
    *Inside the config JSON, change path from `/var/lib/rancher/k3s/storage` to `/fast-pool/k3s-storage`.*

### Phase 3: Application Deployment

1.  **Clone & Prepare**:
    ```bash
    git clone https://github.com/lakshyajaat/cold-storage-management-system.git
    cd cold-storage-management-system
    ```

2.  **Database Configuration**:
    Update `k8s/postgres-cluster.yaml` to ensure it fits the single node resources.
    - Keep `instances: 3`? For a single node, `instances: 1` saves resources, but `instances: 3` tests the operator. Since it's a single node, high availability (HA) inside the node doesn't protect against node failure.
    - **Recommended**: Set `instances: 1` for single-node efficiency unless testing failover logic.
    - Ensure `storageClass` matches your setup (e.g., `local-path` or `longhorn` if you install it).

3.  **Deploy**:
    ```bash
    # Apply standard manifests
    kubectl apply -f k8s/
    
    # Or use the convenient deploy script if available
    ```

---

## 4. Maintenance & Monitoring

### Disk Monitoring
- Run `zpool status -x` daily (cron job) to check for disk errors.
- Configure ZFS Event Daemon (ZED) to email you on drive failure.

### Backups
- Configure **Postgres WAL Archiving** to write to `/mass-pool/backups`.
- This ensures that if the SSD pool dies completely, your database history is safe on the HDDs.
