# Monitoring & Metrics Documentation

Complete guide for the monitoring stack using TimescaleDB, Prometheus, and Grafana.

**Version:** 1.5.173  
**Last Updated:** 2026-01-17

---

## Overview

The system uses a comprehensive monitoring stack to track performance, health, and usage metrics.

**Components:**
- **TimescaleDB** - Metrics storage database (hypertables for time-series data)
- **Prometheus** - Metrics collection and alerting
- **Grafana** - Visualization dashboards
- **Custom API** - Application-level metrics and analytics

---

## TimescaleDB Metrics Database

### Overview

Separate PostgreSQL database with TimescaleDB extension for high-performance time-series data.

**Connection:**
```env
TIMESCALE_HOST=localhost
TIMESCALE_PORT=5432
TIMESCALE_USER=postgres
TIMESCALE_PASSWORD=postgres
TIMESCALE_DB=metrics_db
```

### Tables (Hypertables)

#### 1. api_request_logs

Logs all API requests for analytics.

```sql
CREATE TABLE api_request_logs (
    time TIMESTAMPTZ NOT NULL,
    endpoint TEXT,
    method TEXT,
    status_code INTEGER,
    duration_ms INTEGER,
    user_id INTEGER,
    ip_address TEXT,
    user_agent TEXT
);

SELECT create_hypertable('api_request_logs', 'time');
```

**Retention:** 90 days (compressed after 7 days)

#### 2. node_metrics

K8s node performance metrics.

```sql
CREATE TABLE node_metrics (
    time TIMESTAMPTZ NOT NULL,
    node_name TEXT,
    cpu_usage NUMERIC,
    memory_usage NUMERIC,
    disk_usage NUMERIC,
    network_rx BIGINT,
    network_tx BIGINT
);

SELECT create_hypertable('node_metrics', 'time');
```

**Collection Frequency:** Every 30 seconds

#### 3. postgres_metrics

PostgreSQL database health metrics.

```sql
CREATE TABLE postgres_metrics (
    time TIMESTAMPTZ NOT NULL,
    database_name TEXT,
    connections INTEGER,
    transactions_committed INTEGER,
    transactions_rolled_back INTEGER,
    cache_hit_ratio NUMERIC,
    deadlocks INTEGER
);

SELECT create_hypertable('postgres_metrics', 'time');
```

**Collection Frequency:** Every minute

#### 4. monitoring_alerts

System alerts and notifications.

```sql
CREATE TABLE monitoring_alerts (
    time TIMESTAMPTZ NOT NULL,
    alert_type TEXT,
    severity TEXT, -- critical, warning, info
    message TEXT,
    resolved BOOLEAN DEFAULT false,
    resolved_at TIMESTAMPTZ
);

SELECT create_hypertable('monitoring_alerts', 'time');
```

**Alert Types:** high_cpu, low_disk, high_memory, database_down, backup_failed

---

## API Analytics

### Dashboard Metrics

**Endpoint:** `GET /api/monitoring/dashboard`

```json
{
  "api_stats": {
    "total_requests_today": 5420,
    "avg_response_time_ms": 45,
    "error_rate": 0.02,
    "top_endpoints": [
      {
        "endpoint": "/api/entries",
        "count": 1234,
        "avg_duration_ms": 32
      }
    ]
  },
  "system_health": {
    "cpu_usage": 35.5,
    "memory_usage": 62.3,
    "disk_usage": 45.8,
    "database_status": "healthy"
  },
  "active_alerts": []
}
```

### Top Endpoints

**Endpoint:** `GET /api/monitoring/top-endpoints?period=24h`

Returns most frequently called endpoints with performance stats.

### Slowest Queries

**Endpoint:** `GET /api/monitoring/slow-queries?min_duration=1000`

Returns API calls taking longer than threshold (ms).

### Error Analysis

**Endpoint:** `GET /api/monitoring/errors?period=7d`

Groups errors by type, endpoint, and frequency.

---

## Prometheus Integration

### Metrics Exposed

**Endpoint:** `GET /metrics` (requires admin auth)

**Metrics:**
```
# HTTP Requests
http_requests_total{method="GET",endpoint="/api/entries",status="200"} 1234
http_request_duration_seconds{method="GET",endpoint="/api/entries"} 0.032

# Database
db_connections_active 15
db_query_duration_seconds{query="SELECT"} 0.015

# Application
app_users_active 45
app_entries_total 15234
```

### Alert Rules

Configure in Prometheus `prometheus.yml`:

```yaml
groups:
  - name: cold_storage
    rules:
      - alert: HighCPU
        expr: node_cpu_usage > 80
        for: 5m
        annotations:
          summary: "High CPU usage on {{ $labels.node }}"

      - alert: DatabaseDown
        expr: up{job="postgres"} == 0
        for: 1m
        annotations:
          summary: "PostgreSQL database is down"

      - alert: SlowAPI
        expr: http_request_duration_seconds > 1
        for: 5m
        annotations:
          summary: "API response time high"
```

---

## Grafana Dashboards

### Pre-built Dashboards

#### 1. Application Overview
- Request rate (requests/second)
- Response times (p50, p95, p99)
- Error rates
- Active users

#### 2. Infrastructure Health
- Node CPU/Memory/Disk usage
- Network I/O
- Database connections
- Backup status

#### 3. Business Metrics
- Daily entries
- Payment processing
- Gate pass approvals
- Customer portal usage

### Dashboard Access

**URL:** `http://192.168.15.200:3000`  
**Default Credentials:**
- Username: `admin`
- Password: (set during deployment)

**Setup:**
1. Add Prometheus data source
2. Add TimescaleDB data source
3. Import dashboards from `/grafana/dashboards/`

---

## Alert Management

### Alert Thresholds

**Endpoint:** `GET /api/monitoring/alert-thresholds`

**Response:**
```json
{
  "thresholds": [
    {
      "metric_name": "cpu_usage",
      "warning_threshold": 70,
      "critical_threshold": 85
    },
    {
      "metric_name": "disk_usage",
      "warning_threshold": 75,
      "critical_threshold": 90
    }
  ]
}
```

**Update:** `PUT /api/monitoring/alert-thresholds/:id`

### Active Alerts

**Endpoint:** `GET /api/monitoring/alerts?resolved=false`

Returns current unresolved alerts.

### Resolve Alert

**Endpoint:** `POST /api/monitoring/alerts/:id/resolve`

---

## Backup Monitoring

### Backup History

**Endpoint:** `GET /api/monitoring/backups`

```json
{
  "backups": [
    {
      "id": 123,
      "backup_type": "full",
      "size_bytes": 2147483648,
      "duration_seconds": 45,
      "success": true,
      "created_at": "2026-01-17T02:00:00Z"
    }
  ],
  "last_successful_backup": "2026-01-17T02:00:00Z",
  "next_scheduled_backup": "2026-01-18T02:00:00Z"
}
```

### R2 Cloud Storage Status

**Endpoint:** `GET /api/monitoring/r2-status`

Shows Cloudflare R2 backup bucket status and usage.

---

## Node Metrics Collection

### Node Health

**Endpoint:** `GET /api/monitoring/nodes`

```json
{
  "nodes": [
    {
      "name": "k3s-master",
      "status": "Ready",
      "cpu_usage": 35.5,
      "memory_usage": 62.3,
      "disk_usage": 45.8,
      "uptime_hours": 720
    }
  ]
}
```

### Historical Data

**Endpoint:** `GET /api/monitoring/node-metrics/:node_name?period=24h`

Returns time-series data for specific node.

---

## PostgreSQL Monitoring

### Database Health

**Endpoint:** `GET /api/monitoring/postgres`

```json
{
  "status": "healthy",
  "vip_active": true,
  "connections": {
    "active": 15,
    "idle": 5,
    "max": 100
  },
  "performance": {
    "cache_hit_ratio": 0.98,
    "transactions_per_second": 125,
    "deadlocks": 0
  },
  "replication": {
    "primary": "postgres-1",
    "replicas": ["postgres-2", "postgres-3"],
    "lag_bytes": 0
  }
}
```

---

## Configuration

### Enable API Logging

```env
API_LOGGING_ENABLED=true
API_LOGGING_SAMPLE_RATE=1.0  # 100% of requests
```

### TimescaleDB Compression

Automatic compression after 7 days:

```sql
ALTER TABLE api_request_logs SET (
  timescaledb.compress,
  timescaledb.compress_segmentby = 'endpoint'
);

SELECT add_compression_policy('api_request_logs', INTERVAL '7 days');
```

### Data Retention

Automatic deletion after 90 days:

```sql
SELECT add_retention_policy('api_request_logs', INTERVAL '90 days');
```

---

## Troubleshooting

### TimescaleDB Connection Issues

- Verify credentials
- Check database exists
- Ensure TimescaleDB extension installed
- Review connection logs

### Metrics Not Appearing

- Check API logging enabled
- Verify TimescaleDB connection
- Review insert errors in logs
- Check hypertable configuration

### High Database Usage

- Review retention policies
- Check compression status
- Analyze query performance
- Consider increasing retention interval

---

**Support:** Contact infrastructure team
