# Infrastructure Management Documentation

Complete guide for K8s cluster management, node provisioning, and deployment automation.

**Version:** 1.5.173  
**Last Updated:** 2026-01-17

---

## Overview

The system runs on a **5-node K3s cluster** with automated infrastructure management capabilities including node provisioning, deployment automation, backup management, and failover.

**Infrastructure Components:**
- K3s (Lightweight Kubernetes)
- CloudNative-PG (PostgreSQL operator)
- Longhorn (Distributed storage)
- MetalLB (Load balancer)
- Cloudflare R2 (Backup storage)

---

## K3s Cluster Architecture

### Cluster Nodes

| Node | IP | Role | Specs |
|------|-----|------|-------|
| k3s-master | 192.168.15.110 | Control Plane + Worker | 4 CPU, 8GB RAM |
| k3s-worker-1 | 192.168.15.111 | Worker | 4 CPU, 8GB RAM |
| k3s-worker-2 | 192.168.15.112 | Worker | 4 CPU, 8GB RAM |
| k3s-worker-3 | 192.168.15.113 | Worker | 4 CPU, 8GB RAM |
| k3s-worker-4 | 192.168.15.114 | Worker | 4 CPU, 8GB RAM |

### High Availability Setup

**PostgreSQL HA:**
- 3-replica PostgreSQL cluster (CloudNative-PG)
- Primary: postgres-1
- Replicas: postgres-2, postgres-3
- Automatic failover
- MetalLB VIP: 192.168.15.200

**Application HA:**
- 3 replicas of main application
- Load balanced via MetalLB
- Service VIP: 192.168.15.200:8080
- Customer Portal VIP: 192.168.15.200:8081

---

## Features

### 1. Infrastructure Status

**Endpoint:** `GET /api/infrastructure/status`

**Response:**
```json
{
  "cluster": {
    "total_nodes": 5,
    "ready_nodes": 5,
    "master_node": "k3s-master",
    "k3s_version": "v1.28.5+k3s1"
  },
  "database": {
    "status": "healthy",
    "primary": "postgres-1",
    "replicas": ["postgres-2", "postgres-3"],
    "replication_lag": 0,
    "vip_status": "active"
  },
  "storage": {
    "total_capacity": "500Gi",
    "used": "150Gi",
    "available": "350Gi"
  },
  "backups": {
    "last_backup": "2026-01-17T02:00:00Z",
    "backup_status": "success",
    "r2_connection": "healthy"
  }
}
```

### 2. Node Management

**List Nodes:**
```http
GET /api/infrastructure/nodes
```

**Response:**
```json
{
  "nodes": [
    {
      "name": "k3s-master",
      "status": "Ready",
      "role": "control-plane,master",
      "cpu_usage": 45.2,
      "memory_usage": 62.5,
      "disk_usage": 38.7,
      "pods": 25
    }
  ]
}
```

**Node Operations:**
- Drain node: `POST /api/infrastructure/nodes/:name/drain`
- Cordon node: `POST /api/infrastructure/nodes/:name/cordon`
- Uncordon: `POST /api/infrastructure/nodes/:name/uncordon`
- Reboot: `POST /api/infrastructure/nodes/:name/reboot`

### 3. PostgreSQL Management

**Database Health:**
```http
GET /api/infrastructure/postgres/health
```

**Response:**
```json
{
  "cluster_name": "cold-postgres",
  "status": "healthy",
  "instances": [
    {
      "name": "postgres-1",
      "role": "primary",
      "status": "running",
      "connections": 15,
      "replication_lag": 0
    },
    {
      "name": "postgres-2",
      "role": "replica",
      "status": "running",
      "replication_lag": 245
    }
  ],
  "vip": {
    "address": "192.168.15.200",
    "active": true,
    "current_primary": "postgres-1"
  }
}
```

**Failover Operations:**
```http
POST /api/infrastructure/postgres/switchover
{
  "target_primary": "postgres-2",
  "reason": "Maintenance on postgres-1"
}
```

### 4. Backup Management

**List Backups:**
```http
GET /api/infrastructure/backups
```

**Trigger Manual Backup:**
```http
POST /api/infrastructure/backups/trigger
{
  "type": "full",
  "description": "Pre-deployment backup"
}
```

**Restore from Backup:**
```http
POST /api/infrastructure/backups/:id/restore
{
  "target_database": "cold_db",
  "confirm": true
}
```

---

## Node Provisioning

### Automatic Node Setup

**Add New Node:**
```http
POST /api/infrastructure/nodes/provision
{
  "name": "k3s-worker-5",
  "ip_address": "192.168.15.115",
  "ssh_key_id": 1,
  "role": "worker"
}
```

**Provisioning Steps (Automatic):**
1. SSH connection verification
2. System updates
3. K3s installation
4. Join to cluster
5. Label configuration
6. Health verification

**Provision Logs:**
```http
GET /api/infrastructure/nodes/:name/provision-logs
```

Real-time streaming of provision progress.

### SSH Key Management

**Add SSH Key:**
```http
POST /api/infrastructure/ssh-keys
{
  "name": "infrastructure-key",
  "public_key": "ssh-rsa AAAAB3...",
  "private_key": "-----BEGIN RSA PRIVATE KEY-----..."
}
```

**Security:** Private keys encrypted at rest

---

## Deployment Management

### Application Deployment

**Deploy New Version:**
```http
POST /api/deployments
{
  "version": "v1.5.173",
  "environment": "production",
  "image": "cold-backend:v1.5.173"
}
```

**Deployment Process:**
1. Pull new image
2. Update deployment manifest
3. Rolling update (1 pod at a time)
4. Health check each pod
5. Complete when all pods ready
6. Rollback if failures

**Deployment Status:**
```http
GET /api/deployments/:id
```

**Response:**
```json
{
  "id": 45,
  "version": "v1.5.173",
  "status": "completed",
  "started_at": "2026-01-17T10:00:00Z",
  "completed_at": "2026-01-17T10:05:23Z",
  "duration_seconds": 323,
  "pods_updated": 3,
  "logs_url": "/api/deployments/45/logs"
}
```

### Deployment History

**List Deployments:**
```http
GET /api/deployments?limit=10
```

Shows recent deployments with status.

### Rollback

**Rollback to Previous Version:**
```http
POST /api/deployments/:id/rollback
{
  "reason": "Critical bug found"
}
```

Reverts to previous stable version.

---

## Monitoring & Alerts

### Infrastructure Monitoring

**Dashboard:** `GET /api/infrastructure/monitoring`

**Metrics Collected:**
- Node CPU/Memory/Disk usage
- Pod resource usage
- Network I/O
- Database performance
- Backup status
- VIP availability

### Alert Configuration

**Set Alert Thresholds:**
```http
PUT /api/infrastructure/alerts/config
{
  "cpu_warning": 70,
  "cpu_critical": 85,
  "memory_warning": 75,
  "memory_critical": 90,
  "disk_warning": 80,
  "disk_critical": 95
}
```

**Alert Notifications:**
- Admin email
- SMS (if configured)
- Dashboard alerts
- Logged to monitoring_alerts table

---

## Disaster Recovery

### Backup Strategy

**Automated Backups:**
- Full backup: Daily at 2 AM
- Incremental: Every 6 hours
- Point-in-time: Pre-deployment, pre-season
- Retention: 30 days

**Backup Locations:**
- Local:  Longhorn volumes (snapshots)
- Remote: Cloudflare R2

### Failover Procedures

**Database Failover:**
1. CloudNative-PG auto-detects primary failure
2. Promotes replica to primary (< 30 seconds)
3. Updates VIP to new primary
4. Application reconnects automatically

**Node Failure:**
1. K3s detects node down
2. Reschedules pods to healthy nodes
3. Longhorn replicates data
4. Services continue without interruption

**Complete Cluster Failure:**
1. Restore from R2 backup
2. Rebuild cluster on new hardware
3. Restore PostgreSQL data
4. Deploy application
5. Update DNS/VIP

---

## Maintenance Procedures

### Planned Maintenance

**Drain Node for Maintenance:**
```bash
# Via API
POST /api/infrastructure/nodes/k3s-worker-1/drain

# Or kubectl
kubectl drain k3s-worker-1 --ignore-daemonsets --delete-emptydir-data
```

**Upgrade K3s:**
```bash
# On each node
curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=v1.28.6+k3s1 sh -
```

**Database Maintenance:**
```http
POST /api/infrastructure/postgres/maintenance
{
  "operation": "vacuum",
  "database": "cold_db"
}
```

### Emergency Procedures

**Force PostgreSQL Failover:**
```http
POST /api/infrastructure/postgres/force-failover
{
  "new_primary": "postgres-2",
  "emergency": true
}
```

**Emergency Backup:**
```http
POST /api/infrastructure/backups/emergency
{
  "reason": "Before critical operation"
}
```

---

## Configuration

### Infrastructure Settings

**System Settings:**
```sql
INSERT INTO infra_config (key, value) VALUES
('auto_backup_enabled', 'true'),
('backup_schedule', '0 2 * * *'),
('backup_retention_days', '30'),
('r2_bucket', 'cold-storage-backups'),
('alert_email', 'admin@example.com');
```

### Environment Variables

```env
# R2 Backup
R2_ACCOUNT_ID=your_account_id
R2_ACCESS_KEY_ID=your_access_key
R2_SECRET_ACCESS_KEY=your_secret
R2_BUCKET_NAME=cold-storage-backups

# Database
POSTGRES_PRIMARY_HOST=192.168.15.200
POSTGRES_BACKUP_HOST=192.168.15.111

# K3s
K3S_MASTER_IP=192.168.15.110
K3S_TOKEN=your_k3s_token
```

---

## API Reference

### Infrastructure APIs

**GET** `/api/infrastructure/status` - Overall status  
**GET** `/api/infrastructure/nodes` - List nodes  
**POST** `/api/infrastructure/nodes/:name/drain` - Drain node  
**POST** `/api/infrastructure/nodes/:name/reboot` - Reboot node  
**GET** `/api/infrastructure/postgres/health` - DB health  
**POST** `/api/infrastructure/postgres/switchover` - Planned failover  
**GET** `/api/infrastructure/backups` - List backups  
**POST** `/api/infrastructure/backups/trigger` - Manual backup  
**POST** `/api/infrastructure/backups/:id/restore` - Restore  

### Deployment APIs

**POST** `/api/deployments` - Deploy new version  
**GET** `/api/deployments` - List deployments  
**GET** `/api/deployments/:id` - Deployment status  
**POST** `/api/deployments/:id/rollback` - Rollback  
**GET** `/api/deployments/:id/logs` - Deployment logs  

### Node Provisioning APIs

**POST** `/api/infrastructure/nodes/provision` - Provision node  
**GET** `/api/infrastructure/nodes/:name/provision-logs` - Logs  
**POST** `/api/infrastructure/ssh-keys` - Add SSH key  
**GET** `/api/infrastructure/ssh-keys` - List SSH keys  

---

## Troubleshooting

### Node Not Ready

**Check:**
```bash
kubectl get nodes
kubectl describe node k3s-worker-1
```

**Common Issues:**
- Disk pressure
- Network issues
- K3s service down

**Fix:**
```bash
# Restart K3s
systemctl restart k3s-agent

# Clear disk space
docker system prune -af
```

### Database Not Connecting

**Check:**
1. VIP status: `GET /api/infrastructure/postgres/health`
2. Primary pod running
3. Connection string correct
4. Firewall rules

**Fix:**
```bash
# Restart PostgreSQL pod
kubectl delete pod postgres-1

# Check logs
kubectl logs postgres-1
```

### Backup Failure

**Check:**
1. R2 credentials
2. Network connectivity
3. Disk  space
4. Permissions

**Manual Backup:**
```bash
pg_dump -h 192.168.15.200 -U postgres cold_db | gzip > backup.sql.gz
```

---

## Best Practices

### For Admins

**Daily:**
- Check infrastructure dashboard
- Review node status
- Verify backup completion

**Weekly:**
- Review deployment history
- Check disk usage trends
- Update K3s/components if needed

**Monthly:**
- Test disaster recovery
- Review and rotate SSH keys
- Performance optimization

### Security

- Rotate credentials regularly
- Use SSH keys (not passwords)
- Encrypt backups
- Regular security updates
- Audit infrastructure logs

---

**Support:** Contact infrastructure team for cluster management assistance  
**Emergency:** Call on-call engineer for critical infrastructure issues
