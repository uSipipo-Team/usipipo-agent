# WireGuard IP Allocation Operator Runbook

**Document ID:** WG-IP-ALLOC-RUNBOOK  
**Version:** 1.0  
**Last Updated:** 2026-04-23  
**Team:** Network Infrastructure  
**On-Call:** Network Infra Team (Primary), Platform Team (Secondary)

---

## Table of Contents

1. [Alert Reference](#alert-reference)
2. [Common Scenarios](#common-scenarios)
3. [Emergency Procedures](#emergency-procedures)
4. [Contacts](#contacts)

---

## Alert Reference

### WGIPPoolExhausted

| Field | Value |
|-------|-------|
| **Severity** | Critical |
| **Definition** | Pool available < 5% of total |
| **For** | 2 minutes |

#### Symptoms

- New peer provisioning fails
- Error: "no available IPs in pool"
- IP allocation latency spikes
- Grafana dashboard shows < 5% available

#### Diagnosis

```bash
# Check current pool status
curl -s http://localhost:9090/api/v1/query?query=wg_ip_pool_available | jq
curl -s http://localhost:9090/api/v1/query?query=wg_ip_pool_total | jq

# Calculate utilization
curl -s http://localhost:9090/api/v1/query?query=wg:ip_pool_utilization | jq

# Check recent allocations (possible leak source)
curl -s http://localhost:9090/api/v1/query?query=rate(wg_ip_allocated_total[1h]) | jq

# Review allocation logs
kubectl logs -n wireguard deploy/allocator --since=1h | grep -i "allocat"
```

#### Resolution

1. **Immediate:** Expand IP pool (if configured)
   ```bash
   # Example: Add new CIDR to pool
   wg-allocator pool add --name main --cidr 10.0.100.0/24
   ```

2. **Short-term:** Release orphaned IPs
   ```bash
   # List IPs not associated with active peers
   wg-allocator orphan list --pool=main
   
   # Release orphaned IPs
   wg-allocator orphan release --pool=main --dry-run=false
   ```

3. **Investigate:** Check for IP leaks
   ```bash
   # Review audit logs for unexpected allocations
   kubectl logs -n wireguard deploy/allocator --since=24h | grep -i leak
   ```

#### Escalation

- **5 minutes:** Slack: #network-infra
- **15 minutes:** Page Network Infra on-call
- **30 minutes:** Page Platform Lead

---

### WGIPPoolCritical

| Field | Value |
|-------|-------|
| **Severity** | Critical |
| **Definition** | Pool has 0 available IPs |
| **For** | 1 minute |

#### Symptoms

- All peer creation fails immediately
- Complete service outage for new provisioning
- No IPs available in pool

#### Diagnosis

```bash
# Verify pool is truly exhausted
curl -s http://localhost:9090/api/v1/query?query=wg_ip_pool_available | jq '.data.result[0].value[1]'

# Check allocated vs total
curl -s http://localhost:9090/api/v1/query?query=wg_ip_pool_allocated | jq
curl -s http://localhost:9090/api/v1/query?query=wg_ip_pool_total | jq

# List all active allocations
wg-allocator alloc list --pool=main
```

#### Resolution

1. **Emergency:** Expand pool immediately
   ```bash
   # Add new CIDR block
   wg-allocator pool expand --pool=main --cidr 10.0.200.0/24
   ```

2. **Release IPs:** Check for unreleased IPs
   ```bash
   # Find IPs allocated to deleted peers
   wg-allocator orphan list --pool=main --include-recently-deleted
   
   # Force release orphaned IPs
   wg-allocator orphan release --pool=main --grace-period=5m
   ```

3. **Database repair:** If inconsistencies found
   ```bash
   # Reconcile database state
   wg-allocator reconcile --pool=main --fix
   ```

#### Escalation

- **Immediately:** Page on-call (this is P0)
- **5 minutes:** Notify infrastructure lead
- **15 minutes:** Initiate emergency pool expansion

---

### WGDriftDetected

| Field | Value |
|-------|-------|
| **Severity** | Warning |
| **Definition** | IP drift > 10/hour |
| **For** | 5 minutes |

#### Symptoms

- Allocation rate significantly exceeds release rate
- Possible IP leak
- Pool depleting faster than expected
- Unauthorized allocations

#### Diagnosis

```bash
# Check current drift rate
curl -s http://localhost:9090/api/v1/query?query=wg:drift_rate | jq

# Compare allocation vs release rates
curl -s http://localhost:9090/api/v1/query?query=rate(wg_ip_allocated_total[1h]) | jq
curl -s http://localhost:9090/api/v1/query?query=rate(wg_ip_released_total[1h]) | jq

# Review recent allocations
wg-allocator alloc list --pool=main --sort=created --limit=100

# Check for unauthorized allocations (compare audit log)
kubectl logs -n wireguard deploy/allocator --since=6h | grep -E "(allocat|failed)" | tail -100
```

#### Resolution

1. **Identify leak source**
   ```bash
   # List all allocations in last hour
   wg-allocator audit list --pool=main --since=1h --verbose
   
   # Check for deleted peers with IPs not released
   wg-allocator orphan list --pool=main --deleted-since=1h
   ```

2. **Fix leak**
   ```bash
   # If peer deleted without IP release
   wg-allocator orphan release --pool=main --force
   
   # If configuration error, fix and redeploy
   ```

3. **Prevent recurrence**
   - Implement cleanup webhook for peer deletion
   - Add allocation audit logging
   - Review access controls

#### Escalation

- **15 minutes:** Slack: #network-infra
- **30 minutes:** Create incident ticket
- **1 hour:** Investigate root cause

---

### WGLockContentionHigh

| Field | Value |
|-------|-------|
| **Severity** | Warning |
| **Definition** | Lock wait p99 > 1 second |
| **For** | 5 minutes |

#### Symptoms

- Elevated latency for peer operations
- Allocation requests queued
- High lock wait times
- Timeout errors in logs

#### Diagnosis

```bash
# Check contention metrics
curl -s http://localhost:9090/api/v1/query?query=histogram_quantile(0.99,\ rate(wg_lock_wait_seconds_bucket[5m])) | jq

# Check active allocators
kubectl get pods -n wireguard -l app=wg-allocator -o wide

# Review lock wait distribution
curl -s http://localhost:9090/api/v1/query?query=rate(wg_lock_wait_seconds_bucket[5m]) | jq

# Check recent timeouts
kubectl logs -n wireguard deploy/allocator --since=10m | grep -i "timeout\|wait"
```

#### Resolution

1. **Reduce contention**
   ```bash
   # Scale allocator instances (if using leader lock)
   kubectl scale deployment wg-allocator --replicas=1 -n wireguard
   
   # Or increase lock timeout
   ```

2. **Check database**
   ```bash
   # Check for long-running queries
   # Database connection pool may be saturated
   ```

3. **Scale horizontally** (if applicable)
   ```bash
   # Implement sharded pools
   # Distribute load across multiple pools
   ```

#### Escalation

- **10 minutes:** Check database connection pool
- **20 minutes:** Page database team if related
- **30 minutes:** Scale infrastructure

---

### WGReconciliationError

| Field | Value |
|-------|-------|
| **Severity** | Warning |
| **Definition** | Reconciliation errors detected |
| **For** | 2 minutes |

#### Symptoms

- Reconciliation runs failing
- IP state inconsistent with database
- Duplicate IP risk increased

#### Diagnosis

```bash
# Check reconciliation status
curl -s http://localhost:9090/api/v1/query?query=wg_last_reconciliation_timestamp | jq

# Review reconciliation errors
kubectl logs -n wireguard deploy/allocator --since=10m | grep -i "reconcile.*error"

# Check error count
curl -s http://localhost:9090/api/v1/query?query=wg_reconciliation_errors_total | jq

# Manual reconciliation check
wg-allocator reconcile --pool=main --dry-run --verbose
```

#### Resolution

1. **Check errors**
   ```bash
   # Review detailed error logs
   kubectl logs -n wireguard deploy/allocator --since=30m | grep -E "ERROR|FATAL"
   ```

2. **Run manual reconciliation**
   ```bash
   # Dry run first
   wg-allocator reconcile --pool=main --dry-run
   
   # If healthy, run with fixes
   wg-allocator reconcile --pool=main --fix
   ```

3. **Database repair** (if needed)
   ```bash
   # Manual DB intervention may be required
   # Contact DBA
   ```

#### Escalation

- **10 minutes:** Review error logs
- **30 minutes:** Page if unresolved
- **1 hour:** Database team consultation

---

### WGAllocationFailureRate

| Field | Value |
|-------|-------|
| **Severity** | Warning |
| **Definition** | Failure rate > 5% |
| **For** | 5 minutes |

#### Symptoms

- Peer provisioning failures
- Error rate elevated
- User complaints about peer creation

#### Diagnosis

```bash
# Check failure rate
curl -s http://localhost:9090/api/v1/query?query=wg:allocation_failure_rate | jq

# Check error breakdown
curl -s http://localhost:9090/api/v1/query?query=rate(wg_allocation_failures_total[5m]) by (error_type) | jq

# Review failure logs
kubectl logs -n wireguard deploy/allocator --since=10m | grep -i "fail"
```

#### Resolution

1. **Determine cause**
   ```bash
   # Check specific error types
   wg-allocator alloc stats --pool=main --errors
   ```

2. **Fix based on error**
   - Pool exhaustion: Expand pool
   - Database error: Check DB health
   - Lock contention: See WGLockContentionHigh

3. **Monitor recovery**
   ```bash
   # Watch for resolution
   watch -n 5 'curl -s http://localhost:9090/api/v1/query?query=wg:allocation_failure_rate'
   ```

#### Escalation

- **10 minutes:** Identify error type
- **20 minutes:** Create ticket
- **30 minutes:** Page if critical

---

### WGReconciliationNotRunning

| Field | Value |
|-------|-------|
| **Severity** | Critical |
| **Definition** | No runs for 10+ minutes |
| **For** | 5 minutes |

#### Symptoms

- No reconciliation detected
- Orphaned IPs not recovered
- State drift not detected

#### Diagnosis

```bash
# Verify reconciliation status
curl -s http://localhost:9090/api/v1/query?query=wg_last_reconciliation_timestamp | jq

# Check reconciler pod
kubectl get pods -n wireguard -l component=reconciler

# Check scheduler
kubectl get cronjobs -n wireguard

# Review scheduler logs
kubectl logs -n wireguard job/reconciler-$(date +%Y%m%d) 2>&1 || true
```

#### Resolution

1. **Check pod status**
   ```bash
   # Get reconciler status
   kubectl get pods -n wireguard -l component=reconciler -o wide
   
   # Check pod logs
   kubectl logs -n wireguard deploy/wg-reconciler --since=1h
   ```

2. **Restart reconciler**
   ```bash
   # Restart reconciler deployment
   kubectl rollout restart deployment/wg-reconciler -n wireguard
   
   # Or run manually
   wg-allocator reconcile --pool=main --fix
   ```

3. **Fix scheduler** (if cronjob)
   ```bash
   # Check cronjob status
   kubectl get cronjobs -n wireguard -o yaml
   
   # Suspend and recreate if needed
   kubectl patch cronjob wg-reconciler -n wireguard -p '{"spec":{"suspend":false}}'
   ```

#### Escalation

- **2 minutes:** Restart reconciler
- **5 minutes:** Page on-call
- **10 minutes:** Emergency procedure

---

## Common Scenarios

### IP Pool Exhausted

**Problem:** No IPs available for new peer creation

**Steps:**

1. **Verify exhaustion**
   ```bash
   curl -s http://localhost:9090/api/v1/query?query=wg_ip_pool_available
   ```

2. **Immediate mitigation**
   - Option A: Expand pool (`wg-allocator pool expand`)
   - Option B: Release orphaned IPs (`wg-allocator orphan release`)

3. **Long-term fix**
   - Add new CIDR block
   - Implement proactive expansion alerts
   - Review pool sizing

### Drift Detected

**Problem:** IPs allocated faster than released (possible leak)

**Steps:**

1. **Identify leak**
   ```bash
   wg-allocator audit list --pool=main --since=1h --verbose
   ```

2. **Fix leak**
   - Recover orphaned IPs
   - Fix peer deletion handler

3. **Prevent**
   - Add deletion webhook
   - Implement audit logging
   - Review access patterns

### Lock Contention

**Problem:** High lock wait times

**Steps:**

1. **Identify contention source**
   ```bash
   kubectl top pods -n wireguard
   # or
   kubectl exec -it deploy/wg-allocator -n wireguard -- /allocator metrics
   ```

2. **Mitigation**
   - Scale workers
   - Reduce concurrent requests
   - Implement request queuing

3. **Scale solution**
   - Implement pool sharding
   - Use distributed lock (Consul/etcd)

### Duplicate IP Detection

**Problem:** Duplicate IP assigned to multiple peers

**Steps:**

1. **Verify duplicate**
   ```bash
   wg-allocator duplicate list --pool=main
   ```

2. **Identify conflict**
   ```bash
   # Show affected peers
   wg-allocator duplicate resolve --ip=10.0.1.50 --dry-run
   ```

3. **Resolve**
   ```bash
   # Reassign IPs
   wg-allocator duplicate resolve --ip=10.0.1.50 --reassign
   ```

4. **Investigate cause**
   - Check reconciliation logs
   - Review database state

---

## Emergency Procedures

### Complete Pool Failure

**Trigger:** All IPs exhausted, cannot expand pool

**Steps:**

1. **Acknowledge alert**
   - Page on-call immediately
   - Notify Slack: #network-incidents

2. **Emergency expansion**
   ```bash
   # If standard expansion fails
   wg-allocator pool emergency-add --cidr=<new-cidr>
   ```

3. **Database intervention** (last resort)
   ```bash
   # Connect to database
   kubectl exec -it statefulset/wg-allocator-db -n wireguard -- psql
   
   # Manually check IP state
   SELECT * FROM ip_allocations WHERE pool_id = '<pool-id>';
   ```

4. **Escalate**
   - **Immediate:** Page infrastructure lead
   - **5 minutes:** Page CTO

### Reconciliation Service Down

**Trigger:** WGReconciliationNotRunning > 10 minutes

**Steps:**

1. **Verify service down**
   ```bash
   kubectl get pods -n wireguard -l component=reconciler
   ```

2. **Force run**
   ```bash
   # Manual run
   kubectl run wg-reconciler-manual --image=wg-allocator:latest -n wireguard --restart=Never \
     --command -- /allocator reconcile --pool=main --fix
   ```

3. **Fix scheduler**
   ```bash
   # Check cronjob
   kubectl get cronjob wg-reconciler -n wireguard -o yaml
   
   # Recreate if needed
   kubectl delete cronjob wg-reconciler -n wireguard
   # Recreate from manifest
   ```

4. **Monitor**
   ```bash
   # Watch for next reconciliation
   watch -n 30 'curl -s http://localhost:9090/api/v1/query?query=wg_last_reconciliation_timestamp'
   ```

### Database Corruption

**Trigger:** Reconciliation cannot fix state

**Steps:**

1. **Confirm corruption**
   ```bash
   # Run integrity check
   wg-allocator db check --pool=main
   ```

2. **Backup**
   ```bash
   # Create full backup
   wg-allocator db backup --pool=main --label=pre-repair
   ```

3. **Repair**
   ```bash
   # Repair duplicates
   wg-allocator db repair --pool=main --duplicates
   
   # Reset inconsistent state
   wg-allocator db repair --pool=main --sync
   ```

4. **Verify**
   ```bash
   # Verify integrity
   wg-allocator db check --pool=main
   ```

5. **Escalate**
   - Contact DBA
   - Create incident report

---

## Contacts

| Role | Name | Phone | Email | Slack |
|------|------|-------|------|-------|
| **Network Infra Lead** | J. Smith | +1-555-0101 | netinfra-lead@example.com | @jsmith |
| **Network Infra On-Call** | — | +1-555-0199 | netinfra-oncall@example.com | @netinfra-oncall |
| **Platform Lead** | A. Chen | +1-555-0102 | platform-lead@example.com | @achen |
| **Platform On-Call** | — | +1-555-0198 | platform-oncall@example.com | @platform-oncall |
| **DBA Team** | — | +1-555-0110 | dba@example.com | @dba-team |
| **Security Team** | — | +1-555-0120 | security@example.com | @security |

### Escalation Path

```
< 5 minutes   → Slack: #network-infra
< 15 minutes  → Network Infra On-Call
< 30 minutes  → Platform Lead
< 1 hour     → Infrastructure Lead
Immediate   → CTO (for P0 incidents)
```

### External Contacts

| Service | Contact | Phone |
|---------|---------|-------|
| **Cloud Provider** | Support | +1-888-555-0100 |
| **Database Provider** | Support | +1-800-555-0200 |

---

## Quick Reference Commands

```bash
# Check pool status
wg-allocator pool status --pool=main

# List allocations
wg-allocator alloc list --pool=main

# Run reconciliation
wg-allocator reconcile --pool=main --fix

# Expand pool
wg-allocator pool expand --pool=main --cidr=10.0.100.0/24

# Release orphaned IPs
wg-allocator orphan release --pool=main

# Manual allocation
wg-allocator alloc create --pool=main --peer=<peer-id> --ip=<ip>

# Release IP
wg-allocator alloc release --pool=main --ip=<ip>
```

---

**Document History**

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2026-04-23 | Network Infra Team | Initial creation |

*This runbook is maintained by the Network Infrastructure team. Update this document for any topology or procedure changes.*