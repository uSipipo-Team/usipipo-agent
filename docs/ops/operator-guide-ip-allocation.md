# WireGuard IP Allocation Operator Guide

> **Version:** 1.0 | **Last Updated:** 2026-04-23 | **Team:** Network Infrastructure

This guide covers deployment, configuration, and day-2 operations for the WireGuard IP Allocation system.

---

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [Configuration Reference](#configuration-reference)
4. [Deployment](#deployment)
5. [Day-2 Operations](#day-2-operations)
6. [Health Check Endpoints](#health-check-endpoints)
7. [Integration](#integration)
8. [Troubleshooting](#troubleshooting)

---

## Overview

The WireGuard IP Allocation system provides centralized IP address management for WireGuard VPN peers:

```
┌─────────────────────────────────────────────────────────────────┐
│                    WireGuard IP Allocation                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────┐    ┌──────────────┐    ┌─────────────┐    ┌──────────┐ │
│  │  Agent  │───▶│  Reserve   │───▶│  Confirm  │───▶│ Allocate│ │
│  │  (Go)  │    │    IP     │    │    IP    │    │    IP   │ │
│  └──────────┘    └──────────────┘    └─────────────┘    └──────────┘ │
│                         │                   │                  │          │
│                         ▼                   ▼                  ▼          │
│                    ┌──────────────────────────────────────────┐          │
│                    │         PostgreSQL Database            │          │
│                    │    (vpn_ip_allocations table)    │          │
│                    └───────────────────────────────┬──────┘          │
│                                                  │              │
│                                         ┌────────▼────────┐        │
│                                         │  Reconciliation │        │
│                                         │   (every 30s)  │        │
│                                         └────────────────┘        │
└─────────────────────────────────────────────────────────────────┘
```

### System Components

| Component | Description | Location |
|-----------|-------------|----------|
| **Backend API** | FastAPI with IP allocation endpoints | `usipipo-backend` |
| **Database** | PostgreSQL with `vpn_ip_allocations` | Primary DB |
| **Agent** | Go service for WireGuard management | `usipipo-agent` |
| **Metrics** | Prometheus metrics | `/metrics` |

---

## Prerequisites

### Hardware Requirements

| Resource | Minimum | Recommended |
|----------|---------|------------|
| **CPU** | 1 core | 2 cores |
| **Memory** | 512 MB | 1 GB |
| **Storage** | 10 GB | 25 GB |
| **Network** | 10 Mbps | 100 Mbps |

### Software Requirements

| Component | Version | Notes |
|-----------|---------|-------|
| **Python** | 3.13+ | Backend runtime |
| **PostgreSQL** | 15+ | Database |
| **Go** | 1.21+ | Agent runtime |
| **WireGuard** | 1.0+ | VPN kernel module |
| **Docker** | 24+ | Container runtime (optional) |
| **Kubernetes** | 1.28+ | Orchestration (optional) |

### Network Requirements

| Port | Service | Description |
|------|---------|-----------|
| 5432 | PostgreSQL | Database connection |
| 8000 | HTTP API | Backend API |
| 51820 | WireGuard | VPN port |

---

## Configuration Reference

### Environment Variables (Backend)

#### Required Variables

```bash
# Database - REQUIRED
DATABASE_URL=postgresql+asyncpg://user:password@host:5432/dbname

# Security - REQUIRED
SECRET_KEY=your-secure-secret-key-min-32-chars
JWT_SECRET=your-jwt-secret-key
```

#### IP Allocation Settings

```bash
# WireGuard IP Pool Configuration
WG_IP_POOL_ENABLED=true
WG_IP_POOL_START=2          # First IP octet (default: 2)
WG_IP_POOL_END=254          # Last IP octet (default: 254)
WG_IP_POOL_CIDR=10.88.88.0/24

# Reservation settings
WG_IP_RESERVATION_TTL_SECONDS=300    # Lease duration (5 minutes)
WG_IP_ALLOCATION_TTL_DAYS=30     # Full allocation (30 days)
```

#### Optional Variables

```bash
# API Configuration
API_HOST=0.0.0.0
API_PORT=8000
API_PREFIX=/api/v1

# Rate Limiting
RATE_LIMIT_ENABLED=true
RATE_LIMIT_DEFAULT=60/minute
RATE_LIMIT_AUTH=5/minute

# Logging
LOG_LEVEL=INFO
ENABLE_METRICS=true
```

### Agent Environment Variables

```bash
# Backend Connection
BACKEND_API_URL=https://backend.usipipo.com/api/v1
BACKEND_API_KEY=agent_xxxxxxxxxxxxxxx

# Server Identity
SERVER_ID=550e8400-e29b-41d4-a716-446655440000

# WireGuard Settings
WG_INTERFACE=wg0
WG_CONFIG_PATH=/etc/wireguard/wg0.conf

# Lock Configuration
WG_IP_LOCK_TIMEOUT=10s

# Reconciliation
WG_RECONCILE_INTERVAL=30s
```

### Configuration Examples

#### Docker (docker-compose.yml)

```yaml
services:
  backend:
    image: usipipo/backend:latest
    ports:
      - "8000:8000"
    environment:
      - DATABASE_URL=postgresql+asyncpg://user:pass@postgres:5432/usipipo
      - SECRET_KEY=${SECRET_KEY}
      - JWT_SECRET=${JWT_SECRET}
      - WG_IP_POOL_ENABLED=true
      - WG_IP_POOL_CIDR=10.88.88.0/24
    depends_on:
      - postgres
    restart: unless-stopped

  postgres:
    image: postgres:15
    environment:
      - POSTGRES_USER=user
      - POSTGRES_PASSWORD=pass
      - POSTGRES_DB=usipipo
    volumes:
      - pgdata:/var/lib/postgresql/data
    restart: unless-stopped

volumes:
  pgdata:
```

#### Kubernetes (deployment.yaml)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: usipipo-backend
  labels:
    app: usipipo-backend
spec:
  replicas: 2
  selector:
    matchLabels:
      app: usipipo-backend
  template:
    metadata:
      labels:
        app: usipipo-backend
    spec:
      containers:
        - name: backend
          image: usipipo/backend:latest
          ports:
            - containerPort: 8000
          env:
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: usipipo-secrets
                  key: database-url
            - name: SECRET_KEY
              valueFrom:
                secretKeyRef:
                  name: usipipo-secrets
                  key: secret-key
          envFrom:
            - configMapRef:
                name: usipipo-config
          readinessProbe:
            httpGet:
              path: /health
              port: 8000
          livenessProbe:
            httpGet:
              path: /health
              port: 8000
            initialDelaySeconds: 30
            periodSeconds: 10
```

---

## Deployment

### Docker Deployment

#### 1. Create configuration file

```bash
# Create .env file
cat > .env << 'EOF'
DATABASE_URL=postgresql+asyncpg://usipipo:secure_password@postgres:5432/usipipo
SECRET_KEY=your-production-secret-key-here-min-32-chars
JWT_SECRET=your-jwt-secret-key-production
WG_IP_POOL_ENABLED=true
WG_IP_POOL_CIDR=10.88.88.0/24
LOG_LEVEL=INFO
EOF
```

#### 2. Start the stack

```bash
# Start all services
docker-compose up -d

# Verify running
docker-compose ps

# Check logs
docker-compose logs -f backend
```

#### 3. Verify deployment

```bash
# Health check
curl http://localhost:8000/health

# IP pool status (admin)
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8000/api/v1/health/v2
```

### Kubernetes Deployment

#### 1. Create namespaces

```bash
kubectl create namespace usipipo
kubectl label namespace usipipo istio-injection=enabled
```

#### 2. Apply secrets

```bash
kubectl create secret generic usipipo-secrets \
  --from-literal=database-url='postgresql+asyncpg://...' \
  --from-literal=secret-key='...' \
  --namespace=usipipo
```

#### 3. Apply configuration

```bash
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
```

#### 4. Verify deployment

```bash
kubectl rollout status deployment/usipipo-backend -n usipipo
kubectl get pods -n usipipo
kubectl logs -n usipipo -l app=usipipo-backend
```

#### 5. Apply ingress

```bash
kubectl apply -f k8s/ingress.yaml
```

---

## Day-2 Operations

### Monitoring

#### Key Metrics

| Metric | Type | Description |
|--------|------|------------|
| `wg_ip_pool_total` | Gauge | Total IPs in pool |
| `wg_ip_pool_available` | Gauge | Available IPs |
| `wg_ip_allocations_total` | Counter | Total allocations |
| `wg_ip_releases_total` | Counter | Total releases |
| `wg_reconciliation_runs_total` | Counter | Reconciliation runs |
| `wg_drift_detected_total` | Counter | Drifts detected |

#### Prometheus Queries

```promql
# Pool utilization
wg_ip_pool_available / wg_ip_pool_total

# Allocation rate (per hour)
rate(wg_ip_allocations_total[1h])

# Allocation latency (p95)
histogram_quantile(0.95, rate(wg_ip_allocation_duration_seconds_bucket[5m]))

# Drift rate
rate(wg_drift_detected_total[5m])
```

#### Grafana Dashboards

Import the dashboard from `/home/mowgli/usipipo/usipipo-agent/docs/grafana/wireguard-ip-allocation-dashboard.json`.

#### Alert Configuration

Alert rules are defined in `/home/mowgli/usipipo/usipipo-agent/docs/alerting/wireguard-ip-allocation-alerts.yaml`.

| Alert | Severity | Description |
|-------|----------|------------|
| `WGIPPoolExhausted` | Critical | < 5% IPs available |
| `WGIPPoolCritical` | Critical | 0 IPs available |
| `WGDriftDetected` | Warning | Drift detected |
| `WGReconciliationFailed` | Critical | Reconciliation failed |

### Maintenance Tasks

#### IP Pool Expansion

```bash
# Add new CIDR to pool (requires migration)
psql -h postgres -U usipipo -d usipipo -c "
  INSERT INTO vpn_ip_allocations (server_id, ip_address, ip_int, cidr, status)
  SELECT 
    'server-uuid'::uuid,
    (10 || '.' || 100 + generate_series || '.88.0')::inet,
    100 + generate_series,
    '10.100.88.0/24',
    'free'
  FROM generate_series(0, 253);
"
```

#### Stale IP Cleanup

```bash
# Release IPs older than 90 days
psql -h postgres -U usipipo -d usipipo -c "
  UPDATE vpn_ip_allocations 
  SET status = 'free', 
      released_at = NOW(),
      released_by = 'maintenance'
  WHERE status = 'allocated' 
    AND allocated_at < NOW() - INTERVAL '90 days'
  RETURNING ip_address;
"
```

#### Reconciliation Run

```bash
# Manual reconciliation
curl -X POST http://localhost:8000/api/v1/wireguard/reconcile \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### Database Maintenance

```bash
# Analyze table
psql -h postgres -U usipipo -d usipipo -c "ANALYZE vpn_ip_allocations;"

# Check table size
psql -h postgres -U usipipo -d usipipo -c "
  SELECT pg_size_pretty(pg_total_relation_size('vpn_ip_allocations'));
"

# View allocation logs
psql -h postgres -U usipipo -d usipipo -c "
  SELECT * FROM vpn_ip_allocation_log 
  ORDER BY created_at DESC 
  LIMIT 100;
"
```

---

## Health Check Endpoints

### Basic Health

```bash
curl http://localhost:8000/health

# Response
{"status": "healthy"}
```

### Enhanced Health (v2)

```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8000/api/v1/health/v2

# Response (healthy)
{
  "status": "healthy",
  "version": "1.0.0",
  "timestamp": "2026-04-23T13:39:52Z",
  "components": {
    "database": {"status": "healthy", "latency_ms": 12},
    "redis": {"status": "healthy", "latency_ms": 2},
    "ip_pool": {
      "status": "healthy",
      "total": 253,
      "free": 198,
      "reserved": 12,
      "allocated": 43,
      "exhausted": false
    }
  }
}
```

### IP Pool Status (Admin)

```bash
# Get pool stats for server
curl -H "Authorization: Bearer $ADMIN_TOKEN" \
  "http://localhost:8000/api/v1/wireguard/ip-pool-status/{server_id}"

# Response
{
  "total": 253,
  "free": 198,
  "reserved": 12,
  "allocated": 43,
  "exhausted": false
}
```

### Readiness and Liveness Probes

```yaml
readinessProbe:
  httpGet:
    path: /health
    port: 8000
  initialDelaySeconds: 5
  periodSeconds: 10
  
livenessProbe:
  httpGet:
    path: /health
    port: 8000
  initialDelaySeconds: 30
  periodSeconds: 10
```

---

## Integration

### uSipipo System Integration

The WireGuard IP Allocation system integrates with:

```
┌─────────────────────────────────────────────────────────────────┐
│                   uSipipo Ecosystem                  │
├──────────────────────────────────────────────────────────────────┤
│                                                           │
│  ┌─────────────┐    ┌─────────────────┐    ┌────────────┐ │
│  │ Telegram  │    │   Mini App    │    │  Agent   │ │
│  │    Bot    │────│    (Web)     │────│  (Go)   │ │
│  └─────────────┘    └──────────────┘    └─────┬────┘ │
│                                            │        │
│                           ┌─────────────────┴────────┘        │
│                           ▼                             │
│                    ┌──────────────────┐               │
│                    │  Backend API    │               │
│                    │  /wireguard/*  │               │
│                    └───────┬────────┘               │
│                            │                      │
│            ┌───────────────┼───────────────┐        │
│            ▼               ▼               ▼        │
│     ┌──────────┐  ┌──────────┐  ┌──────────┐ │
│     │  Postgres │  │  Redis  │  │   VPN   │ │
│     │ Database │  │        │  │ Servers │ │
│     └──────────┘  └──────────┘  └──────────┘ │
│                                                 │
└─────────────────────────────────────────────────┘
```

### API Endpoints Summary

| Endpoint | Method | Auth | Description |
|----------|--------|------|------------|
| `/wireguard/reserve-ip` | POST | X-API-Key | Reserve IP |
| `/wireguard/confirm-allocation` | POST | X-API-Key | Confirm allocation |
| `/wireguard/release-ip` | POST | X-API-Key | Release IP |
| `/wireguard/ip-pool-status/{server_id}` | GET | JWT | Pool stats |
| `/health/v2` | GET | JWT | Health with IP pool |

### Telegram Bot Integration

The Telegram bot uses the IP allocation API when users request WireGuard keys:

```
User: /newkey
Bot: Creating your VPN key...
  → POST /wireguard/reserve-ip
  → (generates WireGuard keys)
  → POST /wireguard/confirm-allocation
Bot: Here's your config!
```

### Agent Integration

The Go agent calls the backend API for all IP operations:

```go
// Reserve IP
allocation, err := client.ReserveIP(ctx, keyName)

// Confirm after kernel+config success
err := client.ConfirmIP(ctx, ipAddress, publicKey)

// Release on key deletion
err := client.ReleaseIP(ctx, ipAddress, "user_request")
```

---

## Troubleshooting

### Common Issues

#### IP Pool Exhausted

**Symptoms:**
- `429 No Available IPs` errors
- New key creation fails

**Diagnosis:**
```bash
# Check pool status
curl -s http://localhost:8000/api/v1/health/v2 | jq '.components.ip_pool'

# Check detailed pool
psql -h postgres -U usipipo -d usipipo -c "
  SELECT status, COUNT(*) 
  FROM vpn_ip_allocations 
  GROUP BY status;
"
```

**Resolution:**
1. Release stale allocations (see Maintenance)
2. Expand IP pool
3. Contact engineering

#### Database Connection Failed

**Symptoms:**
- `500 Internal Server Error`
- Health check shows unhealthy database

**Diagnosis:**
```bash
# Test database connection
pg_isready -h postgres -p 5432 -U usipipo

# Check logs
docker-compose logs backend | grep database
```

**Resolution:**
1. Verify DATABASE_URL
2. Check network connectivity
3. Verify database credentials

#### Agent API Key Invalid

**Symptoms:**
- `401 Unauthorized` errors
- Agent cannot authenticate

**Diagnosis:**
```bash
# Verify API key in database
psql -h postgres -U usipipo -d usipipo -c "
  SELECT id, name, api_key, api_key_revoked 
  FROM vpn_servers 
  WHERE api_key = 'agent_xxx';
"
```

**Resolution:**
1. Generate new API key
2. Update agent configuration

### Runbook Reference

For detailed troubleshooting procedures, see [Runbook](./runbooks/wireguard-ip-allocation-runbook.md).

### Emergency Contacts

| Role | Contact |
|------|--------|
| Network Infra On-Call | #network-infra (Slack) |
| Platform Lead | @platform-lead |
| Database Admin | @dba |

---

## See Also

- [API Specification](./api/wireguard-ip-allocation-api.md) - Endpoint reference
- [Implementation Guide](./ip-allocation-implementation-guide.md) - Agent integration
- [Runbook](./runbooks/wireguard-ip-allocation-runbook.md) - Troubleshooting
- [Alerting](./alerting/wireguard-ip-allocation-alerts.yaml) - Alert rules
- [Dashboard](./grafana/wireguard-ip-allocation-dashboard.json) - Grafana dashboard