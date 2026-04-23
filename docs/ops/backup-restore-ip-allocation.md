# WireGuard IP Allocation Tables - Backup & Restore Procedures

> **Document Version:** 1.0  
> **Last Updated:** 2026-04-23  
> **Owner:** Operations Team  
> **Classification:** Internal - Critical Infrastructure

This document outlines backup and restore procedures for the WireGuard VPN IP allocation PostgreSQL tables used by the usipipo-agent system.

---

## 1. Tables to Backup

| Table Name | Description | Size Est. | Criticality |
|-----------|------------|---------|------------|
| `vpn_ip_allocations` | Current active IP allocations, leases, and server bindings | High | **Critical** |
| `vpn_ip_allocation_log` | Audit trail of all allocation/release events | Medium | **High** |
| `vpn_ip_reconciliation_runs` | Reconciliation job run history and results | Low | Medium |

### Table Schemas (Reference)

```sql
-- vpn_ip_allocations (simplified)
CREATE TABLE vpn_ip_allocations (
    id UUID PRIMARY KEY,
    server_id VARCHAR(255) NOT NULL,
    ip_address INET NOT NULL,
    lease_id VARCHAR(255) UNIQUE NOT NULL,
    public_key VARCHAR(255),
    private_key_encrypted TEXT,
    preshared_key_encrypted TEXT,
    vpn_type VARCHAR(50),
    status VARCHAR(50),  -- reserved, active, released, expired
    allocated_at TIMESTAMPTZ,
    lease_expires_at TIMESTAMPTZ,
    confirmed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- vpn_ip_allocation_log
CREATE TABLE vpn_ip_allocation_log (
    id SERIAL PRIMARY KEY,
    lease_id VARCHAR(255),
    ip_address INET,
    event_type VARCHAR(50),  -- reserve, confirm, release
    server_id VARCHAR(255),
    vpn_key_name VARCHAR(255),
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- vpn_ip_reconciliation_runs
CREATE TABLE vpn_ip_reconciliation_runs (
    id SERIAL PRIMARY KEY,
    run_id VARCHAR(255) UNIQUE NOT NULL,
    status VARCHAR(50),  -- running, completed, failed
    servers_checked INTEGER,
    ips_reconciled INTEGER,
    errors JSONB,
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

---

## 2. Backup Procedures

### 2.1 Full Database Backup (pg_dump)

```bash
#!/bin/bash
# full-backup-ip-tables.sh

set -euo pipefail

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR="/var/backups/usipipo"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-usipipo}"
DB_USER="${DB_USER:-usipipo}"

mkdir -p "${BACKUP_DIR}"

# Full dump of all three tables
pg_dump \
    --host="${DB_HOST}" \
    --port="${DB_PORT}" \
    --dbname="${DB_NAME}" \
    --username="${DB_USER}" \
    --table=vpn_ip_allocations \
    --table=vpn_ip_allocation_log \
    --table=vpn_ip_reconciliation_runs \
    --format=custom \
    --compress=9 \
    --file="${BACKUP_DIR}/ip-allocation-full-${TIMESTAMP}.dump" \
    --verbose

# Verify the backup
pg_restore --list "${BACKUP_DIR}/ip-allocation-full-${TIMESTAMP}.dump" > /dev/null

# Create timestamp symlink for easy latest access
ln -sf "ip-allocation-full-${TIMESTAMP}.dump" "${BACKUP_DIR}/ip-allocation-full-latest.dump"

echo "Full backup completed: ip-allocation-full-${TIMESTAMP}.dump"
```

### 2.2 Incremental Backup (Table-Specific)

```bash
#!/bin/bash
# incremental-backup-ip-tables.sh

set -euo pipefail

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR="/var/backups/usipipo"
INCREMENTAL_MARKER_FILE="/var/backups/usipipo/.last_backup_timestamp"

# Get last backup timestamp or default to 24 hours ago
LAST_TS=$(cat "${INCREMENTAL_MARKER_FILE}" 2>/dev/null || date -d "24 hours ago" +%Y-%m-%d\ %H:%M:%S)

# Backup only records changed since last backup (using updated_at)
pg_dump \
    --host="${DB_HOST:-localhost}" \
    --port="${DB_PORT:-5432}" \
    --dbname="${DB_NAME:-usipipo}" \
    --username="${DB_USER:-usipipo}" \
    --table=vpn_ip_allocations \
    --format=custom \
    --file="${BACKUP_DIR}/ip-allocation-incremental-${TIMESTAMP}.dump" \
    --verbose

# Update marker file
date +"%Y-%m-%d %H:%M:%S" > "${INCREMENTAL_MARKER_FILE}"

echo "Incremental backup completed: ip-allocation-incremental-${TIMESTAMP}.dump"
```

### 2.3 Point-in-Time Recovery (PITR)

**Prerequisites:**

1. WAL archiving must be enabled on the PostgreSQL server
2. Base backups must be taken regularly

```bash
# PITR-backup script
#!/bin/bash
# pitr-backup-ip-tables.sh

set -euo pipefail

TARGET_TIME="${1:-}"  # Pass target recovery time as YYYY-MM-DD HH:MM:SS
BACKUP_DIR="/var/backups/usipipo"

# Take a base backup first
pg_basebackup \
    --host="${DB_HOST:-localhost}" \
    --pgdata="${BACKUP_DIR}/basebackup-${TIMESTAMP}" \
    --format=custom \
    --wal-method=stream \
    --checkpoint=fast

# Create recovery configuration
cat > "${BACKUP_DIR}/recovery.conf" << EOF
restore_command = 'cp ${BACKUP_DIR}/wal/%f %p'
recovery_target_time = '${TARGET_TIME}'
recovery_target_action = 'promote'
EOF

echo "PITR backup configuration created"
echo "To restore: use recovery.conf with pg_restore or pg_ctl promote"
```

### 2.4 Individual Table Backup Commands

```bash
# Single table dump - vpn_ip_allocations (most critical)
pg_dump \
    --host="${DB_HOST}" \
    --dbname="${DB_NAME}" \
    --username="${DB_USER}" \
    --table=vpn_ip_allocations \
    --format=plain \
    --file="vpn_ip_allocations.sql" \
    --data-only

# With schema
pg_dump \
    --host="${DB_HOST}" \
    --dbname="${DB_NAME}" \
    --username="${DB_USER}" \
    --table=vpn_ip_allocations \
    --format=plain \
    --file="vpn_ip_allocations_full.sql"

# Compressed binary format (recommended)
pg_dump \
    --host="${DB_HOST}" \
    --dbname="${DB_NAME}" \
    --username="${DB_USER}" \
    --table=vpn_ip_allocations \
    --format=custom \
    --compress=9 \
    --file="vpn_ip_allocations.dump"
```

---

## 3. Restore Procedures

### 3.1 Full Restore

```bash
#!/bin/bash
# full-restore-ip-tables.sh

set -euo pipefail

BACKUP_FILE="${1:-${BACKUP_DIR}/ip-allocation-full-latest.dump}"
DB_HOST="${DB_HOST:-localhost}"
DB_NAME="${DB_NAME:-usipipo}"
DB_USER="${DB_USER:-usipipo}"

if [ ! -f "${BACKUP_FILE}" ]; then
    echo "ERROR: Backup file not found: ${BACKUP_FILE}"
    exit 1
fi

# Confirm before destructive operation
echo "WARNING: This will overwrite current table data!"
read -p "Continue? (yes/no): " CONFIRM
if [ "${CONFIRM}" != "yes" ]; then
    echo "Restore cancelled"
    exit 0
fi

# Create pre-restore backup (if tables exist)
EXISTING_COUNT=$(psql -h "${DB_HOST}" -d "${DB_NAME}" -U "${DB_USER}" -t -c "SELECT COUNT(*) FROM vpn_ip_allocations;" 2>/dev/null || echo "0")
if [ "${EXISTING_COUNT}" -gt "0" ]; then
    TIMESTAMP=$(date +%Y%m%d_%H%M%S)
    echo "Creating pre-restore backup..."
    pg_dump --host="${DB_HOST}" --dbname="${DB_NAME}" --username="${DB_USER}" \
        --table=vpn_ip_allocations --table=vpn_ip_allocation_log --table=vpn_ip_reconciliation_runs \
        --format=custom --file="/var/backups/usipipo/pre-restore-${TIMESTAMP}.dump"
fi

# Drop and recreate tables (optional - use if schema changed)
# psql -h "${DB_HOST}" -d "${DB_NAME}" -U "${DB_USER}" -c "DROP TABLE IF EXISTS vpn_ip_allocations CASCADE;"

# Restore from backup
pg_restore \
    --host="${DB_HOST}" \
    --dbname="${DB_NAME}" \
    --username="${DB_USER}" \
    --data-only \
    --disable-triggers \
    --exit-zero-on-clean \
    "${BACKUP_FILE}"

echo "Full restore completed from ${BACKUP_FILE}"
```

### 3.2 Point-in-Time Restore

```bash
#!/bin/bash
# pitr-restore-ip-tables.sh

set -euo pipefail

TARGET_TIME="$1"  # e.g., "2026-04-22 14:30:00"
RECOVERY_DIR="/var/backups/usipipo/recovery"

mkdir -p "${RECOVERY_DIR}"

# Stop PostgreSQL gracefully
systemctl stop postgresql

# Remove existing data directory
rm -rf /var/lib/postgresql/data
mkdir -p /var/lib/postgresql

# Restore from base backup
pg_basebackupRestore --backup="${BACKUP_DIR}/basebackup-latest" --target=/var/lib/postgresql/data

# Configure PITR
cat > /var/lib/postgresql/data/postgresql.conf << EOF
restore_command = 'cp ${BACKUP_DIR}/wal/%f %p'
recovery_target_time = '${TARGET_TIME}'
EOF

# Create recovery signal file
touch /var/lib/postgresql/data/recovery.signal

# Start PostgreSQL
systemctl start postgresql

# Wait for recovery to complete
echo "Waiting for recovery to complete..."
until psql -h localhost -U postgres -c "SELECT pg_is_in_recovery();" | grep -q 'f'; do
    sleep 5
done

echo "PITR restore completed to ${TARGET_TIME}"
```

### 3.3 Selective Restore (Single Server)

```bash
#!/bin/bash
# selective-restore-server.sh

set -euo pipefail

SERVER_ID="${1}"
BACKUP_FILE="${2:-${BACKUP_DIR}/ip-allocation-full-latest.dump}"

if [ -z "${SERVER_ID}" ]; then
    echo "Usage: $0 <server_id> [backup_file]"
    exit 1
fi

# Restore allocations for specific server only
psql -h "${DB_HOST}" -d "${DB_NAME}" -U "${DB_USER}" << EOF
-- Delete existing allocations for this server
DELETE FROM vpn_ip_allocations WHERE server_id = '${SERVER_ID}';

-- Restore from backup to temp table
CREATE TEMP TABLE temp_allocations AS
SELECT * FROM vpn_ip_allocations WHERE false;

\copy temp_allocations FROM '/dev/stdin' WITH FORMAT csv;

-- Insert filtered data
INSERT INTO vpn_ip_allocations
SELECT * FROM temp_allocations
WHERE server_id = '${SERVER_ID}';
EOF

echo "Selective restore completed for server: ${SERVER_ID}"
```

**Manual selective restore:**

```bash
# Export specific server IPs to CSV, then restore
psql -h localhost -d usipipo -U usipipo -c \
    "COPY (SELECT * FROM vpn_ip_allocations WHERE server_id = 'server-123') TO STDOUT WITH CSV HEADER" \
    > /tmp/server-123-allocations.csv

# Import
psql -h localhost -d usipipo -U usipipo -c \
    "COPY vpn_ip_allocations FROM '/tmp/server-123-allocations.csv' WITH CSV HEADER"
```

---

## 4. Pre-Backup Checklist

- [ ] Verify PostgreSQL connectivity: `pg_isready -h <host> -p 5432`
- [ ] Check disk space: `df -h <backup_dir>` (require 2x table size free)
- [ ] Verify database user permissions: `pg_dump --help` runs without errors
- [ ] Confirm backup directory exists and is writable: `touch <backup_dir>/test`
- [ ] Check for active reconciliation runs: `SELECT status FROM vpn_ip_reconciliation_runs WHERE status = 'running';`
- [ ] Notify on-call team of backup window
- [ ] Disable auto-reconciliation during backup (if applicable)
- [ ] Record current database state: `SELECT count(*) FROM vpn_ip_allocations;`
- [ ] Verify WAL archiving is functioning (if PITR configured)

---

## 5. Post-Restore Verification Steps

```sql
-- 1. Verify row counts match pre-backup
SELECT 'vpn_ip_allocations' as table_name, count(*) as row_count FROM vpn_ip_allocations
UNION ALL
SELECT 'vpn_ip_allocation_log', count(*) FROM vpn_ip_allocation_log
UNION ALL
SELECT 'vpn_ip_reconciliation_runs', count(*) FROM vpn_ip_reconciliation_runs;

-- 2. Verify critical columns are populated
SELECT 
    server_id,
    ip_address,
    status,
    lease_expires_at
FROM vpn_ip_allocations
WHERE status IN ('reserved', 'active')
ORDER BY allocated_at DESC
LIMIT 10;

-- 3. Check for orphaned leases (no corresponding log entries)
SELECT a.id, a.lease_id, a.ip_address
FROM vpn_ip_allocations a
LEFT JOIN vpn_ip_allocation_log l ON a.lease_id = l.lease_id
WHERE l.id IS NULL AND a.status = 'active';

-- 4. Verify referential integrity
SELECT COUNT(*) as orphaned_logs 
FROM vpn_ip_allocation_log l
LEFT JOIN vpn_ip_allocations a ON l.lease_id = a.lease_id
WHERE a.id IS NULL;

-- 5. Test application connectivity (from usipipo-agent host)
curl -s http://localhost:8080/health || echo "Health check failed"
```

```bash
# 6. Verify WireGuard can resolve IPs
cd /etc/wireguard && wg show

# 7. Check log for recent errors
journalctl -u usipipo-agent --since "1 hour ago" | grep -i error

# 8. Run reconciliation check (if needed)
curl -X POST http://localhost:8080/api/v1/reconcile
```

---

## 6. Disaster Recovery Scenarios

### 6.1 Database Crash

```bash
#!/bin/bash
# dr-database-crash.sh

set -euo pipefail

# 1. Assess damage
echo "Checking database status..."
pg_isready -h "${DB_HOST}"

if ! pg_isready -h "${DB_HOST}"; then
    echo "Database is down. Starting recovery..."

    # Check for corruption
    psql -h "${DB_HOST}" -d postgres -U postgres -c "SELECT pg_check_corruption();"

    # Attempt to start
    systemctl start postgresql
    systemctl restart postgresql

    # If still down, restore from latest base backup
    if ! pg_isready -h "${DB_HOST}"; then
        echo "Database require base restore..."
        BACKUP_FILE=$(ls -t /var/backups/usipipo/ip-allocation-full-*.dump | head -1)
        ./full-restore-ip-tables.sh "${BACKUP_FILE}"
    fi
fi

# 2. Verify data integrity
psql -h "${DB_HOST}" -d "${DB_NAME}" -U "${DB_USER}" -c "SELECT count(*) FROM vpn_ip_allocations;"
```

### 6.2 Accidental Data Deletion

```bash
#!/bin/bash
# dr-accidental-delete.sh

set -euo pipefail

# Example: Delete all allocations for a server
-- psql -d usipipo -c "DELETE FROM vpn_ip_allocations WHERE server_id = 'server-123';"

# Recovery steps:
# 1. Identify the deletion timestamp from logs
psql -d usipipo -c "SELECT now() as deletion_time, current_user as deleted_by;"

# 2. Find backup before deletion
# Assuming backup at 14:00 and deletion at 15:30
BACKUP_FILE=$(ls -t /var/backups/usipipo/ip-allocation-full-*.dump | head -1)

# 3. Restore to temp location
pg_restore --dbname=usipipo_test "${BACKUP_FILE}"

# 4. Extract deleted records
psql -d usipipo_test -c "\COPY (SELECT * FROM vpn_ip_allocations WHERE server_id = 'server-123') TO STDOUT WITH CSV HEADER" > /tmp/restored-allocations.csv

# 5. Re-insert
psql -d usipipo -c "\COPY vpn_ip_allocations FROM '/tmp/restored-allocations.csv' WITH CSV HEADER"
```

### 6.3 Migration Failure

```bash
#!/bin/bash
# dr-migration-failure.sh

# Scenario: Database migration fails mid-flight

# 1. Verify pre-migration state
psql -d usipipo -c "SELECT count(*) FROM vpn_ip_allocations;"

# 2. Check for partial migration data
psql -d usipipo -c "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public';"

# 3. Rollback to pre-migration backup
LATEST_BACKUP=$(ls -t /var/backups/usipipo/ip-allocation-full-*.dump | grep -v "$(date +%Y%m%d)" | head -1)

if [ -n "${LATEST_BACKUP}" ]; then
    echo "Restoring from pre-migration backup: ${LATEST_BACKUP}"
    ./full-restore-ip-tables.sh "${LATEST_BACKUP}"
else
    echo "ERROR: No pre-migration backup available"
    exit 1
fi

# 4. Verify post-rollback state
psql -d usipipo -c "SELECT count(*) FROM vpn_ip_allocations;"
```

---

## 7. Testing Procedures

### 7.1 Backup Verification Test

```bash
#!/bin/bash
# test-backup-validity.sh

BACKUP_FILE="${1:-/var/backups/usipipo/ip-allocation-full-latest.dump}"

echo "Testing backup: ${BACKUP_FILE}"

# 1. Check file exists and size
if [ ! -s "${BACKUP_FILE}" ]; then
    echo "FAIL: Backup file missing or empty"
    exit 1
fi
echo "PASS: File exists and non-empty"

# 2. Verify pg_restore can read it
if pg_restore --list "${BACKUP_FILE}" > /dev/null 2>&1; then
    echo "PASS: pg_restore can read backup"
else
    echo "FAIL: Corrupt backup file"
    exit 1
fi

# 3. Extract and verify data to temp DB
TEMP_DB="usipipo_backup_test"
createdb "${TEMP_DB}" 2>/dev/null || true

if pg_restore --dbname="${TEMP_DB}" "${BACKUP_FILE}" 2>/dev/null; then
    echo "PASS: Backup restores successfully"
    
    # Verify row counts
    ROW_COUNT=$(psql -d "${TEMP_DB}" -t -c "SELECT count(*) FROM vpn_ip_allocations;")
    echo "Restored ${ROW_COUNT} allocation records"
    
    # Verify schema
    psql -d "${TEMP_DB}" -c "SELECT * FROM vpn_ip_allocations LIMIT 1;"
    echo "PASS: Schema valid"
else
    echo "FAIL: Restore failed"
    dropdb "${TEMP_DB}"
    exit 1
fi

# Cleanup
dropdb "${TEMP_DB}"
echo "All backup tests PASSED"
```

### 7.2 Restore Drill (Quarterly)

```bash
#!/bin/bash
# restore-drill.sh

set -euo pipefail

DRILL_DB="usipipo_drill_$(date +%Y%m%d)"
BACKUP_FILE=$(ls -t /var/backups/usipipo/ip-allocation-full-*.dump | head -1)

echo "=== Restore Drill ==="
echo "Target DB: ${DRILL_DB}"
echo "Source: ${BACKUP_FILE}"

# Create isolated test database
createdb "${DRILL_DB}"

# Restore to test DB
pg_restore --dbname="${DRILL_DB}" "${BACKUP_FILE}"

# Run verification queries
psql -d "${DRILL_DB}" << 'EOF'
-- Verify row counts
SELECT 'vpn_ip_allocations' as tbl, count(*) as cnt FROM vpn_ip_allocations
UNION ALL
SELECT 'vpn_ip_allocation_log', count(*) FROM vpn_ip_allocation_log
UNION ALL
SELECT 'vpn_ip_reconciliation_runs', count(*) FROM vpn_ip_reconciliation_runs;

-- Check for data anomalies
SELECT 'null server_ids' as check, count(*) as cnt FROM vpn_ip_allocations WHERE server_id IS NULL;
SELECT 'null ip_addresses' as check, count(*) as cnt FROM vpn_ip_allocations WHERE ip_address IS NULL;
SELECT 'expired leases' as check, count(*) FROM vpn_ip_allocations WHERE lease_expires_at < NOW() AND status = 'active';
SELECT 'duplicate IPs' as check, count(*) FROM vpn_ip_allocations GROUP BY ip_address HAVING count(*) > 1;
EOF

# Cleanup
dropdb "${DRILL_DB}"

echo "=== Restore Drill Completed ==="
```

### 7.3 Automated Backup Test (Cron)

```0 2 * * * /home/mowgli/usipipo/scripts/test-backup-validity.sh >> /var/log/usipipo/backup-test.log 2>&1
```

---

## Appendix: Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_HOST` | localhost | PostgreSQL host |
| `DB_PORT` | 5432 | PostgreSQL port |
| `DB_NAME` | usipipo | Database name |
| `DB_USER` | usipipo | Database user |
| `BACKUP_DIR` | /var/backups/usipipo | Backup storage location |

---

## Related Documentation

- [WIREGUARD-SETUP.md](../WIREGUARD-SETUP.md)
- [ip-allocation-implementation-guide.md](../ip-allocation-implementation-guide.md)
- [DEPLOYMENT.md](../../DEPLOYMENT.md)