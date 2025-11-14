# 2-NODE VOIP ARCHITECTURE - OPTIMIZED DESIGN
## High-Availability VoIP System (600-800 CC) on 2 Nodes

**Version**: 2.0
**Date**: 2025-11-14
**Constraint**: ONLY 2 physical nodes available
**Approach**: Consolidated services with intelligent failover

---

## EXECUTIVE SUMMARY

### Challenge
Original design required 9 nodes (2 Kamailio + 2 FreeSWITCH + 2 PostgreSQL + 2 API Gateway + 1 Redis).
**Constraint**: Only 2 nodes available.

### Solution
**Consolidated architecture** running all services on 2 nodes with:
- Smart service placement and resource allocation
- Bash script + Keepalived failover (replacing repmgr)
- Hybrid approach combining best practices from both architectures
- Performance optimizations to maintain 600-800 CC capacity

### Architecture Decision
**Adopt hybrid approach**: Combine Project A's performance optimizations with Project B's database design and consolidated deployment model.

---

## 1. ARCHITECTURE OVERVIEW

### 1.1. Node Layout

```
┌─────────────────────────────────────────────────────────────────┐
│                    VIP: 192.168.1.100                           │
│              (Single entry point for all services)               │
└────────────────────────┬────────────────────────────────────────┘
                         │
         ┌───────────────┴───────────────┐
         │                               │
         ▼                               ▼
┌─────────────────────┐         ┌─────────────────────┐
│   NODE 1 (Master)   │         │   NODE 2 (Backup)   │
│   192.168.1.101     │◄───────►│   192.168.1.102     │
│                     │ Sync    │                     │
├─────────────────────┤         ├─────────────────────┤
│ Services:           │         │ Services:           │
│ • Kamailio         │         │ • Kamailio         │
│ • FreeSWITCH       │         │ • FreeSWITCH       │
│ • PostgreSQL       │         │ • PostgreSQL       │
│ • VoIP Admin Svc   │         │ • VoIP Admin Svc   │
│ • Redis            │         │ • Redis            │
│ • Keepalived       │         │ • Keepalived       │
│ • lsyncd           │         │ • lsyncd           │
└─────────────────────┘         └─────────────────────┘
```

### 1.2. Service States

| Service | Node 1 (Master) | Node 2 (Backup) | Failover Strategy |
|---------|----------------|-----------------|-------------------|
| **Kamailio** | Active | Standby | VIP-based (both running) |
| **FreeSWITCH** | Active | Standby | VIP-based (both running) |
| **PostgreSQL** | Primary | Standby (Streaming Replication) | Bash script promotion |
| **VoIP Admin Service** | Active | Active | Load balanced via VIP |
| **Redis** | Master | Slave (Replication) | Bash script promotion |
| **Keepalived** | MASTER | BACKUP | VRRP protocol |
| **lsyncd** | Sync to Node2 | Sync to Node1 | Bidirectional |

### 1.3. VIP Strategy (Single VIP)

**Single VIP**: `192.168.1.100`
- **SIP**: Port 5060 → Kamailio (active node)
- **PostgreSQL**: Port 5432 → PostgreSQL (primary)
- **API**: Port 8080 → VoIP Admin Service (both nodes)
- **Redis**: Port 6379 → Redis (master)

**Benefits**:
- ✅ Simplified DNS/configuration (one IP for everything)
- ✅ Fewer keepalived instances to manage
- ✅ Easier troubleshooting
- ✅ Lower complexity

---

## 2. HARDWARE REQUIREMENTS (PER NODE)

### 2.1. Recommended Specifications

Given all services run on same nodes, higher specs needed:

| Resource | Minimum | Recommended | Optimal |
|----------|---------|-------------|---------|
| **CPU** | 16 cores | 24 cores | 32 cores |
| **RAM** | 64 GB | 96 GB | 128 GB |
| **Storage (OS + DB)** | 500 GB NVMe SSD | 1 TB NVMe SSD | 2 TB NVMe SSD |
| **Storage (Recordings)** | 3 TB HDD | 5 TB HDD | 10 TB HDD (RAID) |
| **Network** | 1 Gbps | 10 Gbps | 10 Gbps bonded |
| **tmpfs (RAM disk)** | 20 GB | 30 GB | 40 GB |

### 2.2. Resource Allocation (Per Node)

```
Total: 96 GB RAM, 24 CPU cores

PostgreSQL:           24 GB RAM,  8 cores
FreeSWITCH:          16 GB RAM,  6 cores
Kamailio:             8 GB RAM,  4 cores
VoIP Admin Service:   8 GB RAM,  2 cores
Redis:                4 GB RAM,  1 core
OS + buffers:        36 GB RAM,  3 cores
─────────────────────────────────────────
Total:               96 GB RAM, 24 cores
```

### 2.3. Storage Layout

```
/dev/nvme0n1        (1 TB SSD - System + DB)
├── /               100 GB  (OS)
├── /var/lib/postgresql  400 GB  (PostgreSQL data)
├── /var/lib/redis       50 GB   (Redis persistence)
└── /tmp_recordings      30 GB   (tmpfs - mounted from RAM)

/dev/sda            (5 TB HDD - Recordings)
└── /storage/recordings  5 TB    (Persistent recordings)
```

---

## 3. SERVICES ARCHITECTURE

### 3.1. PostgreSQL (Streaming Replication with Bash Failover)

#### Why Not repmgr?
- **Simpler**: Bash scripts easier to understand and customize
- **Less dependencies**: No repmgr daemon to manage
- **More control**: Custom logic for your specific needs
- **Lighter**: Less resource overhead

#### Streaming Replication Setup

**Node 1 (Primary)**:
```ini
# postgresql.conf
listen_addresses = '*'
port = 5432
max_connections = 300

# Replication
wal_level = replica
max_wal_senders = 5
max_replication_slots = 5
hot_standby = on
wal_keep_size = 2GB
synchronous_commit = off  # async for performance
```

**Node 2 (Standby)**:
```ini
# postgresql.conf
hot_standby = on
hot_standby_feedback = on

# standby.signal file present (PostgreSQL 16)
```

**Primary server setup**:
```bash
# On Node 1: Create replication user
CREATE USER replicator WITH REPLICATION ENCRYPTED PASSWORD 'repl_pass';
```

**Standby setup**:
```bash
# On Node 2: Initial clone
pg_basebackup -h 192.168.1.101 -U replicator -D /var/lib/postgresql/16/main -P -R --wal-method=stream

# Create standby.signal
touch /var/lib/postgresql/16/main/standby.signal
```

#### Bash Script Failover Logic

**`/usr/local/bin/postgres_failover.sh`**:
```bash
#!/bin/bash
# PostgreSQL Failover Script - Promotes standby to primary
set -euo pipefail

LOGFILE="/var/log/postgres-failover.log"
PGDATA="/var/lib/postgresql/16/main"
PGUSER="postgres"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOGFILE"
}

promote_to_primary() {
    log "============================================"
    log "PROMOTING STANDBY TO PRIMARY"
    log "============================================"

    # Check if already primary
    if ! sudo -u $PGUSER test -f "$PGDATA/standby.signal"; then
        log "ERROR: Already primary or standby.signal missing"
        return 1
    fi

    # Promote
    log "Executing pg_ctl promote..."
    sudo -u $PGUSER /usr/lib/postgresql/16/bin/pg_ctl promote -D "$PGDATA"

    # Wait for promotion
    for i in {1..30}; do
        if ! sudo -u $PGUSER test -f "$PGDATA/standby.signal"; then
            log "SUCCESS: Promoted to primary (${i}s)"
            return 0
        fi
        sleep 1
    done

    log "ERROR: Promotion timeout"
    return 1
}

# Health check: is PostgreSQL responding?
check_postgres_health() {
    sudo -u $PGUSER psql -c "SELECT 1" > /dev/null 2>&1
    return $?
}

# Main execution
case "${1:-}" in
    promote)
        promote_to_primary
        ;;
    check)
        if check_postgres_health; then
            log "PostgreSQL health check: OK"
            exit 0
        else
            log "PostgreSQL health check: FAILED"
            exit 1
        fi
        ;;
    *)
        echo "Usage: $0 {promote|check}"
        exit 1
        ;;
esac
```

**Permissions**:
```bash
chmod +x /usr/local/bin/postgres_failover.sh
chown root:root /usr/local/bin/postgres_failover.sh
```

### 3.2. Keepalived Configuration (Single VIP)

**`/etc/keepalived/keepalived.conf`** - Node 1:
```
global_defs {
    router_id NODE1_VOIP
    enable_script_security
    script_user root
}

# PostgreSQL health check
vrrp_script check_postgres {
    script "/usr/local/bin/postgres_failover.sh check"
    interval 5
    weight -50
    fall 2
    rise 2
}

# Kamailio health check
vrrp_script check_kamailio {
    script "/usr/bin/kamcmd core.uptime"
    interval 5
    weight -30
    fall 2
    rise 2
}

# FreeSWITCH health check
vrrp_script check_freeswitch {
    script "/usr/bin/fs_cli -x 'status' | grep -q UP"
    interval 5
    weight -30
    fall 2
    rise 2
}

# Single VIP instance
vrrp_instance VI_VOIP {
    state MASTER
    interface eth0
    virtual_router_id 100
    priority 100
    advert_int 1

    authentication {
        auth_type PASS
        auth_pass voip_secret_2024
    }

    virtual_ipaddress {
        192.168.1.100/24 dev eth0 label eth0:vip
    }

    track_script {
        check_postgres
        check_kamailio
        check_freeswitch
    }

    notify_master "/usr/local/bin/failover_master.sh"
    notify_backup "/usr/local/bin/failover_backup.sh"
    notify_fault "/usr/local/bin/failover_fault.sh"
}
```

**`/etc/keepalived/keepalived.conf`** - Node 2:
```
global_defs {
    router_id NODE2_VOIP
    enable_script_security
    script_user root
}

vrrp_script check_postgres {
    script "/usr/local/bin/postgres_failover.sh check"
    interval 5
    weight -50
    fall 2
    rise 2
}

vrrp_script check_kamailio {
    script "/usr/bin/kamcmd core.uptime"
    interval 5
    weight -30
    fall 2
    rise 2
}

vrrp_script check_freeswitch {
    script "/usr/bin/fs_cli -x 'status' | grep -q UP"
    interval 5
    weight -30
    fall 2
    rise 2
}

vrrp_instance VI_VOIP {
    state BACKUP
    interface eth0
    virtual_router_id 100
    priority 50
    advert_int 1

    authentication {
        auth_type PASS
        auth_pass voip_secret_2024
    }

    virtual_ipaddress {
        192.168.1.100/24 dev eth0 label eth0:vip
    }

    track_script {
        check_postgres
        check_kamailio
        check_freeswitch
    }

    notify_master "/usr/local/bin/failover_master.sh"
    notify_backup "/usr/local/bin/failover_backup.sh"
    notify_fault "/usr/local/bin/failover_fault.sh"
}
```

### 3.3. Failover Scripts (with flock)

**`/usr/local/bin/failover_master.sh`**:
```bash
#!/bin/bash
set -euo pipefail

LOGFILE="/var/log/voip-failover.log"
LOCKFILE="/var/lock/failover-master.lock"
STATE_FILE="/var/run/voip-master.state"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] MASTER: $1" | tee -a "$LOGFILE"
}

# Acquire lock
exec 200>"$LOCKFILE"
flock -n 200 || {
    log "Another failover_master running, exiting"
    exit 1
}

log "========================================="
log "TRANSITIONING TO MASTER STATE"
log "========================================="
echo "MASTER" > "$STATE_FILE"

# Wait for peer to realize BACKUP
sleep 3

# 1. Promote PostgreSQL if standby
log "Checking PostgreSQL status..."
if sudo -u postgres test -f /var/lib/postgresql/16/main/standby.signal; then
    log "PostgreSQL is standby, promoting..."
    /usr/local/bin/postgres_failover.sh promote || log "ERROR: PostgreSQL promotion failed"
else
    log "PostgreSQL already primary or not configured"
fi

# 2. Promote Redis if slave
log "Checking Redis status..."
if redis-cli ROLE | head -1 | grep -q slave; then
    log "Redis is slave, promoting..."
    redis-cli SLAVEOF NO ONE
    log "Redis promoted to master"
else
    log "Redis already master"
fi

# 3. Start/Restart services
log "Ensuring services are running..."
systemctl start kamailio || log "ERROR: Kamailio start failed"
systemctl start freeswitch || log "ERROR: FreeSWITCH start failed"
systemctl start voip-admin || log "ERROR: VoIP Admin start failed"

# Wait for services to initialize
sleep 5

# 4. Health checks
log "Performing health checks..."
kamcmd core.uptime > /dev/null 2>&1 && log "✓ Kamailio: OK" || log "✗ Kamailio: FAIL"
fs_cli -x "status" | grep -q "UP" && log "✓ FreeSWITCH: OK" || log "✗ FreeSWITCH: FAIL"
sudo -u postgres psql -c "SELECT 1" > /dev/null 2>&1 && log "✓ PostgreSQL: OK" || log "✗ PostgreSQL: FAIL"
redis-cli PING | grep -q PONG && log "✓ Redis: OK" || log "✗ Redis: FAIL"
curl -s http://localhost:8080/health > /dev/null && log "✓ VoIP Admin: OK" || log "✗ VoIP Admin: FAIL"

log "========================================="
log "MASTER TRANSITION COMPLETE"
log "========================================="

flock -u 200
exit 0
```

**`/usr/local/bin/failover_backup.sh`**:
```bash
#!/bin/bash
set -euo pipefail

LOGFILE="/var/log/voip-failover.log"
LOCKFILE="/var/lock/failover-backup.lock"
STATE_FILE="/var/run/voip-master.state"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] BACKUP: $1" | tee -a "$LOGFILE"
}

exec 200>"$LOCKFILE"
flock -n 200 || {
    log "Another failover_backup running, exiting"
    exit 1
}

log "========================================="
log "TRANSITIONING TO BACKUP STATE"
log "========================================="
echo "BACKUP" > "$STATE_FILE"

# Services remain running in standby mode
# Only log the state change
log "Node is now BACKUP - services running in standby"

# Verify we're actually standby for data services
log "Verifying standby state..."
if sudo -u postgres test -f /var/lib/postgresql/16/main/standby.signal; then
    log "✓ PostgreSQL: Standby mode confirmed"
else
    log "⚠ PostgreSQL: Not in standby mode (may need manual check)"
fi

if redis-cli ROLE | head -1 | grep -q slave; then
    log "✓ Redis: Slave mode confirmed"
else
    log "⚠ Redis: Not in slave mode (may need manual check)"
fi

log "========================================="
log "BACKUP TRANSITION COMPLETE"
log "========================================="

flock -u 200
exit 0
```

**`/usr/local/bin/failover_fault.sh`**:
```bash
#!/bin/bash
set -euo pipefail

LOGFILE="/var/log/voip-failover.log"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] FAULT: $1" | tee -a "$LOGFILE"
}

log "============================================="
log "FAULT DETECTED ON KEEPALIVED"
log "============================================="

# Diagnostic information
log "Service status:"
systemctl is-active kamailio && log "  Kamailio: active" || log "  Kamailio: INACTIVE"
systemctl is-active freeswitch && log "  FreeSWITCH: active" || log "  FreeSWITCH: INACTIVE"
systemctl is-active postgresql && log "  PostgreSQL: active" || log "  PostgreSQL: INACTIVE"
systemctl is-active redis && log "  Redis: active" || log "  Redis: INACTIVE"

# Send alert (email, Slack, PagerDuty, etc.)
# mail -s "VoIP Keepalived FAULT on $(hostname)" admin@example.com <<< "Check logs at $LOGFILE"

log "Services remain running despite FAULT state"
log "Manual intervention may be required"

exit 0
```

**Set permissions**:
```bash
chmod +x /usr/local/bin/failover_*.sh
chmod +x /usr/local/bin/postgres_failover.sh
chown root:root /usr/local/bin/failover_*.sh
chown root:root /usr/local/bin/postgres_failover.sh
```

### 3.4. Redis Master-Slave Replication

**Node 1 (Master)** - `/etc/redis/redis.conf`:
```conf
bind 0.0.0.0
port 6379
maxmemory 4gb
maxmemory-policy allkeys-lru

# Persistence
save 900 1
save 300 10
save 60 10000
appendonly yes
appendfsync everysec

# Replication
masterauth redis_password_2024
requirepass redis_password_2024
```

**Node 2 (Slave)** - `/etc/redis/redis.conf`:
```conf
bind 0.0.0.0
port 6379
maxmemory 4gb
maxmemory-policy allkeys-lru

# Persistence
save 900 1
save 300 10
save 60 10000
appendonly yes
appendfsync everysec

# Replication
replicaof 192.168.1.101 6379
masterauth redis_password_2024
requirepass redis_password_2024
replica-read-only yes
```

**Failover**: Handled by `failover_master.sh` script (`redis-cli SLAVEOF NO ONE`)

---

## 4. DATABASE SCHEMA DESIGN (ENHANCED)

### 4.1. Multi-Schema Approach (ADOPTED from Project B)

```sql
-- Create main database
CREATE DATABASE voip_platform;

\c voip_platform

-- Schema for Kamailio
CREATE SCHEMA kamailio;

-- Schema for VoIP business logic
CREATE SCHEMA voip;

-- Schema for replication monitoring (optional, for future)
CREATE SCHEMA monitoring;
```

### 4.2. VoIP Schema (Business Logic)

#### Core Tables

**voip.domains** (Multi-tenancy support):
```sql
CREATE TABLE voip.domains (
    id SERIAL PRIMARY KEY,
    domain VARCHAR(255) UNIQUE NOT NULL,
    tenant_name VARCHAR(255),
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_domains_active ON voip.domains(active);

-- Example data
INSERT INTO voip.domains (domain, tenant_name)
VALUES ('default.local', 'Default Tenant');
```

**voip.users** (System users/agents):
```sql
CREATE TABLE voip.users (
    id SERIAL PRIMARY KEY,
    domain_id INT REFERENCES voip.domains(id) ON DELETE CASCADE,
    username VARCHAR(100) NOT NULL,
    email VARCHAR(255),
    full_name VARCHAR(255),
    role VARCHAR(50), -- 'agent', 'supervisor', 'admin'
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(domain_id, username)
);

CREATE INDEX idx_users_domain ON voip.users(domain_id);
CREATE INDEX idx_users_active ON voip.users(active);
```

**voip.extensions** (UNIFIED EXTENSION MODEL - BRILLIANT from Project B):
```sql
CREATE TABLE voip.extensions (
    id SERIAL PRIMARY KEY,
    domain_id INT REFERENCES voip.domains(id) ON DELETE CASCADE,
    extension VARCHAR(50) NOT NULL,
    type VARCHAR(20) NOT NULL, -- 'user', 'queue', 'ivr', 'voicemail', 'trunk_out', 'conference'
    description VARCHAR(255),

    -- Reference to actual entity (polymorphic)
    user_id INT REFERENCES voip.users(id) ON DELETE SET NULL,
    queue_id INT,
    ivr_id INT,
    voicemail_box_id INT,
    trunk_id INT,
    conference_id INT,

    -- Metadata
    service_ref JSONB, -- Additional metadata for routing
    need_media BOOLEAN DEFAULT false, -- true if needs FreeSWITCH
    recording_policy_id INT,

    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    UNIQUE(domain_id, extension)
);

CREATE INDEX idx_extensions_domain ON voip.extensions(domain_id);
CREATE INDEX idx_extensions_type ON voip.extensions(type);
CREATE INDEX idx_extensions_active ON voip.extensions(active);
CREATE INDEX idx_extensions_service_ref ON voip.extensions USING GIN(service_ref);

-- Example data
INSERT INTO voip.extensions (domain_id, extension, type, user_id, need_media, service_ref)
VALUES
    (1, '1001', 'user', 1, false, '{"sip_password":"hashed"}'),
    (1, '8001', 'queue', NULL, true, '{"queue_id":1,"queue_name":"Support_L1"}'),
    (1, '9001', 'ivr', NULL, true, '{"ivr_id":1,"ivr_name":"Main_Menu"}'),
    (1, '0XXXXXXXXX', 'trunk_out', NULL, true, '{"trunk_id":1,"prefix":"0"}');
```

**voip.queues**:
```sql
CREATE TABLE voip.queues (
    id SERIAL PRIMARY KEY,
    domain_id INT REFERENCES voip.domains(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    extension VARCHAR(50),
    strategy VARCHAR(50) DEFAULT 'ring-all', -- 'ring-all', 'round-robin', 'longest-idle'
    max_wait_time INT DEFAULT 300,
    max_wait_time_with_no_agent INT DEFAULT 120,
    tier_rules_apply BOOLEAN DEFAULT true,
    discard_abandoned_after INT DEFAULT 60,
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(domain_id, name)
);
```

**voip.queue_members**:
```sql
CREATE TABLE voip.queue_members (
    id SERIAL PRIMARY KEY,
    queue_id INT REFERENCES voip.queues(id) ON DELETE CASCADE,
    user_id INT REFERENCES voip.users(id) ON DELETE CASCADE,
    tier INT DEFAULT 1,
    position INT DEFAULT 1,
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(queue_id, user_id)
);
```

**voip.ivr_menus**:
```sql
CREATE TABLE voip.ivr_menus (
    id SERIAL PRIMARY KEY,
    domain_id INT REFERENCES voip.domains(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    extension VARCHAR(50),
    greeting_sound VARCHAR(500),
    invalid_sound VARCHAR(500),
    timeout_sound VARCHAR(500),
    max_failures INT DEFAULT 3,
    max_timeouts INT DEFAULT 3,
    timeout_seconds INT DEFAULT 5,
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);
```

**voip.ivr_entries**:
```sql
CREATE TABLE voip.ivr_entries (
    id SERIAL PRIMARY KEY,
    ivr_id INT REFERENCES voip.ivr_menus(id) ON DELETE CASCADE,
    digit VARCHAR(10) NOT NULL,
    action VARCHAR(50), -- 'transfer', 'queue', 'voicemail', 'sub-menu'
    action_data VARCHAR(255),
    order_num INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);
```

**voip.trunks**:
```sql
CREATE TABLE voip.trunks (
    id SERIAL PRIMARY KEY,
    domain_id INT REFERENCES voip.domains(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    type VARCHAR(20), -- 'sip', 'pstn'
    host VARCHAR(255),
    port INT DEFAULT 5060,
    username VARCHAR(100),
    password VARCHAR(255),
    prefix VARCHAR(20),
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);
```

**voip.recording_policies** (DATABASE-DRIVEN - from Project B):
```sql
CREATE TABLE voip.recording_policies (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    record_inbound BOOLEAN DEFAULT false,
    record_outbound BOOLEAN DEFAULT false,
    record_internal BOOLEAN DEFAULT false,
    record_queue BOOLEAN DEFAULT true,
    retention_days INT DEFAULT 90,
    storage_path VARCHAR(255) DEFAULT '/storage/recordings',
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Default policy
INSERT INTO voip.recording_policies (name, record_queue, retention_days)
VALUES ('Default Queue Recording', true, 90);
```

**voip.cdr** (Call Detail Records):
```sql
CREATE TABLE voip.cdr (
    id BIGSERIAL PRIMARY KEY,
    call_uuid UUID NOT NULL,
    bleg_uuid UUID,
    domain_id INT REFERENCES voip.domains(id),

    direction VARCHAR(20), -- 'inbound', 'outbound', 'internal'
    caller_id_number VARCHAR(50),
    caller_id_name VARCHAR(100),
    destination_number VARCHAR(50),

    context VARCHAR(100),

    start_time TIMESTAMP,
    answer_time TIMESTAMP,
    end_time TIMESTAMP,
    duration INT, -- Total duration in seconds
    billsec INT, -- Billed duration (answer to hangup)

    hangup_cause VARCHAR(50),

    queue_id INT REFERENCES voip.queues(id),
    agent_user_id INT REFERENCES voip.users(id),

    recording_id INT,

    sip_call_id VARCHAR(255),

    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_cdr_call_uuid ON voip.cdr(call_uuid);
CREATE INDEX idx_cdr_domain ON voip.cdr(domain_id);
CREATE INDEX idx_cdr_start_time ON voip.cdr(start_time);
CREATE INDEX idx_cdr_queue ON voip.cdr(queue_id);
CREATE INDEX idx_cdr_agent ON voip.cdr(agent_user_id);

-- Partitioning by month (for performance)
-- Example: CREATE TABLE voip.cdr_2024_11 PARTITION OF voip.cdr FOR VALUES FROM ('2024-11-01') TO ('2024-12-01');
```

**voip.recordings**:
```sql
CREATE TABLE voip.recordings (
    id BIGSERIAL PRIMARY KEY,
    call_uuid UUID NOT NULL,
    cdr_id BIGINT REFERENCES voip.cdr(id) ON DELETE SET NULL,

    file_path VARCHAR(500),
    file_size BIGINT,
    duration INT,
    format VARCHAR(20) DEFAULT 'wav',

    start_time TIMESTAMP,
    end_time TIMESTAMP,

    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_recordings_call_uuid ON voip.recordings(call_uuid);
CREATE INDEX idx_recordings_cdr ON voip.recordings(cdr_id);
```

**voip.api_keys** (API Security):
```sql
CREATE TABLE voip.api_keys (
    id SERIAL PRIMARY KEY,
    key_name VARCHAR(100) NOT NULL,
    api_key VARCHAR(255) UNIQUE NOT NULL,
    permissions JSONB, -- {"cdr": "read", "recordings": "read"}
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP
);

CREATE INDEX idx_api_keys_active ON voip.api_keys(active);
```

### 4.3. Views for Routing

**voip.vw_extensions** (Unified view for Kamailio routing):
```sql
CREATE OR REPLACE VIEW voip.vw_extensions AS
SELECT
    e.id,
    e.domain_id,
    d.domain,
    e.extension,
    e.type,
    e.description,
    e.need_media,
    e.service_ref,
    e.active,

    -- User details if type='user'
    u.username AS user_username,
    u.full_name AS user_full_name,

    -- Queue details if type='queue'
    q.name AS queue_name,
    q.strategy AS queue_strategy,

    -- Recording policy
    rp.name AS recording_policy
FROM voip.extensions e
LEFT JOIN voip.domains d ON e.domain_id = d.id
LEFT JOIN voip.users u ON e.user_id = u.id
LEFT JOIN voip.queues q ON e.queue_id = q.id
LEFT JOIN voip.recording_policies rp ON e.recording_policy_id = rp.id
WHERE e.active = true AND d.active = true;
```

---

## 5. VOIP ADMIN SERVICE (ENHANCED)

### 5.1. Service Architecture

**Expanded from simple API Gateway to full VoIP Admin Service**:

```
VoIP Admin Service (Go)
├── HTTP Server (port 8080)
│   ├── FreeSWITCH Integration
│   │   ├── GET  /fs/xml/directory  (mod_xml_curl - user auth)
│   │   ├── GET  /fs/xml/dialplan   (mod_xml_curl - call routing)
│   │   └── POST /fs/cdr            (mod_json_cdr - CDR ingestion)
│   │
│   ├── API Endpoints
│   │   ├── GET  /api/cdr           (Query CDR)
│   │   ├── GET  /api/recordings    (List recordings)
│   │   ├── GET  /api/recordings/{id} (Get recording file)
│   │   ├── POST /api/extensions    (Create extension)
│   │   ├── PUT  /api/extensions/{id} (Update extension)
│   │   └── DELETE /api/extensions/{id} (Delete extension)
│   │
│   └── Management Endpoints
│       ├── GET  /api/queues
│       ├── POST /api/queues
│       ├── GET  /api/users
│       └── GET  /health
│
├── CDR Processor (background worker)
│   ├── Redis Queue Consumer
│   ├── JSON Parser
│   └── Batch Inserter (100 CDR/batch)
│
└── Cache Layer
    ├── In-memory cache (extension lookup)
    └── Redis cache (directory XML)
```

### 5.2. Configuration File

**`/etc/voip-admin/config.yaml`**:
```yaml
server:
  host: 0.0.0.0
  port: 8080
  read_timeout: 30s
  write_timeout: 30s

database:
  host: 192.168.1.100  # VIP
  port: 5432
  database: voip_platform
  user: voip_admin
  password: admin_password
  max_open_conns: 20
  max_idle_conns: 10
  conn_max_lifetime: 1h

redis:
  host: 192.168.1.100  # VIP
  port: 6379
  password: redis_password_2024
  db: 0
  pool_size: 10

cdr:
  queue_name: cdr_queue
  batch_size: 100
  batch_timeout: 5s
  workers: 4

cache:
  directory_ttl: 300s     # 5 minutes
  extension_ttl: 600s     # 10 minutes
  enable_local_cache: true

logging:
  level: info
  format: json
  output: /var/log/voip-admin/service.log
```

### 5.3. Code Structure (Enhanced)

```
voip-admin/
├── cmd/
│   └── voipadmind/
│       └── main.go
├── internal/
│   ├── config/
│   │   ├── config.go
│   │   └── loader.go
│   ├── database/
│   │   ├── postgres.go
│   │   ├── queries.go
│   │   └── models.go
│   ├── cache/
│   │   ├── redis.go
│   │   └── local.go
│   ├── freeswitch/
│   │   ├── xml/
│   │   │   ├── directory.go      # Generate directory XML
│   │   │   ├── dialplan.go       # Generate dialplan XML
│   │   │   └── templates.go      # XML templates
│   │   └── cdr/
│   │       ├── handler.go        # HTTP handler for CDR
│   │       ├── processor.go      # Background processor
│   │       └── parser.go         # JSON parser
│   ├── api/
│   │   ├── handlers/
│   │   │   ├── cdr.go
│   │   │   ├── recordings.go
│   │   │   ├── extensions.go
│   │   │   ├── queues.go
│   │   │   └── health.go
│   │   └── middleware/
│   │       ├── auth.go
│   │       ├── logging.go
│   │       └── cors.go
│   └── domain/
│       ├── models/
│       │   ├── extension.go
│       │   ├── cdr.go
│       │   ├── queue.go
│       │   └── user.go
│       └── services/
│           ├── extension_service.go
│           ├── cdr_service.go
│           └── routing_service.go
├── pkg/
│   └── utils/
│       ├── crypto.go
│       └── validator.go
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

### 5.4. Key Implementation Files

**`internal/freeswitch/xml/directory.go`** (Example):
```go
package xml

import (
    "encoding/xml"
    "fmt"
)

type DirectoryDocument struct {
    XMLName xml.Name `xml:"document"`
    Type    string   `xml:"type,attr"`
    Section Section  `xml:"section"`
}

type Section struct {
    Name   string  `xml:"name,attr"`
    Domain *Domain `xml:"domain,omitempty"`
}

type Domain struct {
    Name string `xml:"name,attr"`
    User *User  `xml:"user,omitempty"`
}

type User struct {
    ID     string     `xml:"id,attr"`
    Params []Param    `xml:"params>param"`
    Variables []Variable `xml:"variables>variable"`
}

type Param struct {
    Name  string `xml:"name,attr"`
    Value string `xml:"value,attr"`
}

type Variable struct {
    Name  string `xml:"name,attr"`
    Value string `xml:"value,attr"`
}

func GenerateDirectoryXML(user, domain, password string) ([]byte, error) {
    doc := DirectoryDocument{
        Type: "freeswitch/xml",
        Section: Section{
            Name: "directory",
            Domain: &Domain{
                Name: domain,
                User: &User{
                    ID: user,
                    Params: []Param{
                        {Name: "password", Value: password},
                        {Name: "dial-string", Value: "{...}user/"},
                    },
                    Variables: []Variable{
                        {Name: "user_context", Value: "default"},
                        {Name: "effective_caller_id_name", Value: user},
                    },
                },
            },
        },
    }

    return xml.MarshalIndent(doc, "", "  ")
}
```

### 5.5. Systemd Service

**`/etc/systemd/system/voip-admin.service`**:
```ini
[Unit]
Description=VoIP Admin Service
After=network.target postgresql.service redis.service
Wants=postgresql.service redis.service

[Service]
Type=simple
User=voip-admin
Group=voip-admin
WorkingDirectory=/opt/voip-admin
ExecStart=/opt/voip-admin/bin/voipadmind -config /etc/voip-admin/config.yaml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

# Resource limits
LimitNOFILE=65536
MemoryLimit=8G
CPUQuota=200%

[Install]
WantedBy=multi-user.target
```

---

## 6. WHICH SECTIONS TO ADOPT FROM PROJECT B

### ✅ ADOPT (Better Design)

1. **Multi-schema Database**
   - **Why**: Better organization, allows cross-schema JOIN
   - **How**: Single `voip_platform` DB with `kamailio`, `voip`, `monitoring` schemas
   - **Impact**: LOW (just database structure change)

2. **Unified Extension Model**
   - **Why**: Brilliant design, simplifies routing logic
   - **How**: `voip.extensions` table with type field
   - **Impact**: MEDIUM (requires Kamailio routing changes)

3. **Database-Driven Recording Policies**
   - **Why**: Flexible, no config file changes needed
   - **How**: `voip.recording_policies` table
   - **Impact**: LOW (enhancement, doesn't break existing)

4. **VoIP Admin Service Concept**
   - **Why**: Centralized management, API-first
   - **How**: Expand API Gateway to full platform backend
   - **Impact**: HIGH (significant development effort)

5. **API Keys Table**
   - **Why**: Better security than basic auth
   - **How**: `voip.api_keys` table with permissions
   - **Impact**: LOW (easy to implement)

### ⚠️ CONSIDER (Depends on Requirements)

6. **mod_xml_curl for Directory**
   - **Why**: Dynamic user configuration
   - **When**: If need multi-tenancy or frequent user changes
   - **Impact**: MEDIUM (adds 20-50ms latency)
   - **Decision**: Use ONLY for directory, NOT dialplan

7. **Multi-tenancy (voip.domains)**
   - **Why**: Support multiple customers on same infrastructure
   - **When**: If building SaaS platform
   - **Impact**: MEDIUM (requires tenant isolation logic)
   - **Decision**: Add tables now, implement later if needed

### ❌ DON'T ADOPT (Not Suitable for 600-800 CC)

8. **mod_xml_curl for Dialplan**
   - **Why**: Too slow for high-volume
   - **Latency**: +20-50ms per call (unacceptable at 600-800 CC)
   - **Decision**: Keep static XML dialplan

9. **Direct CDR Insert (No Queue)**
   - **Why**: Project B does sync insert, we need async
   - **Decision**: Keep Redis queue + batch processing

### 📊 Adoption Summary

| Feature | Project A (Original) | Project B (New) | 2-Node Decision |
|---------|---------------------|-----------------|-----------------|
| Database | Separate DBs | Multi-schema | ✅ ADOPT multi-schema |
| Extensions | usrloc only | Unified model | ✅ ADOPT unified model |
| Recording | Config file | DB-driven | ✅ ADOPT DB-driven |
| Admin Service | Simple API GW | Full platform | ✅ ADOPT (enhanced) |
| Failover | repmgr | Not specified | ✅ Bash scripts (custom) |
| CDR | Async (Redis) | Sync (direct) | ❌ KEEP async |
| FS Directory | ODBC | mod_xml_curl | ⚠️ CONSIDER (optional) |
| FS Dialplan | Static XML | mod_xml_curl | ❌ KEEP static |

---

## 7. PERFORMANCE OPTIMIZATIONS FOR 2-NODE

### 7.1. Resource Contention Mitigation

**Problem**: All services on same nodes compete for resources

**Solutions**:

1. **CPU Pinning** (systemd):
```ini
# /etc/systemd/system/freeswitch.service
[Service]
CPUAffinity=0-5     # Cores 0-5 for FreeSWITCH
```

```ini
# /etc/systemd/system/postgresql.service
[Service]
CPUAffinity=6-13    # Cores 6-13 for PostgreSQL
```

2. **I/O Priority**:
```ini
# PostgreSQL
IOSchedulingClass=realtime
IOSchedulingPriority=0

# FreeSWITCH (media priority)
IOSchedulingClass=realtime
IOSchedulingPriority=1
```

3. **Network Traffic Shaping** (tc):
```bash
# Prioritize SIP/RTP traffic
tc qdisc add dev eth0 root handle 1: htb default 12
tc class add dev eth0 parent 1: classid 1:1 htb rate 1gbit
tc class add dev eth0 parent 1:1 classid 1:10 htb rate 800mbit prio 1  # RTP
tc class add dev eth0 parent 1:1 classid 1:11 htb rate 100mbit prio 2  # SIP
tc class add dev eth0 parent 1:1 classid 1:12 htb rate 100mbit prio 3  # Other
```

### 7.2. Kernel Tuning

**`/etc/sysctl.conf`**:
```conf
# Network performance
net.core.rmem_max = 134217728
net.core.wmem_max = 134217728
net.ipv4.tcp_rmem = 4096 87380 67108864
net.ipv4.tcp_wmem = 4096 65536 67108864
net.core.netdev_max_backlog = 300000
net.ipv4.tcp_max_syn_backlog = 8192

# File descriptors
fs.file-max = 2097152
fs.nr_open = 2097152

# Shared memory (PostgreSQL)
kernel.shmmax = 68719476736
kernel.shmall = 4294967296

# Ephemeral ports (RTP)
net.ipv4.ip_local_port_range = 16384 65535
```

### 7.3. Caching Strategy

**3-tier caching**:
```
Request
   ↓
1. Application cache (in-memory, 60s TTL) → 90% hit rate
   ↓
2. Redis cache (300s TTL) → 9% hit rate
   ↓
3. PostgreSQL (source of truth) → 1% hit rate
```

---

## 8. MONITORING & HEALTH CHECKS

### 8.1. Health Check Script

**`/usr/local/bin/system_health.sh`**:
```bash
#!/bin/bash

check_service() {
    local service=$1
    systemctl is-active --quiet $service && echo "✓ $service" || echo "✗ $service FAILED"
}

check_port() {
    local port=$1
    local name=$2
    nc -zv localhost $port > /dev/null 2>&1 && echo "✓ $name (port $port)" || echo "✗ $name FAILED (port $port)"
}

echo "=== VoIP System Health Check ==="
echo "Node: $(hostname)"
echo "Time: $(date)"
echo ""

echo "Services:"
check_service postgresql
check_service redis-server
check_service kamailio
check_service freeswitch
check_service voip-admin
check_service keepalived
check_service lsyncd

echo ""
echo "Ports:"
check_port 5432 "PostgreSQL"
check_port 6379 "Redis"
check_port 5060 "Kamailio"
check_port 5080 "FreeSWITCH"
check_port 8080 "VoIP Admin"

echo ""
echo "PostgreSQL Role:"
sudo -u postgres psql -tAc "SELECT CASE WHEN pg_is_in_recovery() THEN 'STANDBY' ELSE 'PRIMARY' END"

echo ""
echo "Redis Role:"
redis-cli --no-auth-warning -a redis_password_2024 ROLE | head -1

echo ""
echo "Keepalived State:"
cat /var/run/voip-master.state 2>/dev/null || echo "Not set"

echo ""
echo "VIP Status:"
ip addr show eth0 | grep 192.168.1.100 && echo "✓ VIP active" || echo "✗ VIP not active"
```

### 8.2. Prometheus Exporters

Install exporters on both nodes:
```bash
# PostgreSQL
apt install prometheus-postgres-exporter

# Node metrics
apt install prometheus-node-exporter

# Redis
apt install prometheus-redis-exporter
```

**Metrics endpoints**:
- PostgreSQL: `http://localhost:9187/metrics`
- Redis: `http://localhost:9121/metrics`
- Node: `http://localhost:9100/metrics`
- VoIP Admin: `http://localhost:8080/metrics` (custom)

---

## 9. FAILOVER TESTING PROCEDURES

### 9.1. Planned Failover Test

```bash
# On Node 1 (current master)

# 1. Check current state
/usr/local/bin/system_health.sh

# 2. Stop keepalived (triggers failover)
systemctl stop keepalived

# 3. Wait 30 seconds

# 4. Check Node 2 status
# (should be MASTER now)

# 5. Verify calls work
# (make test call)

# 6. Restart Node 1 keepalived
systemctl start keepalived

# 7. Verify Node 1 is BACKUP
/usr/local/bin/system_health.sh
```

### 9.2. Disaster Recovery Test

```bash
# Simulate Node 1 complete failure

# On Node 1:
systemctl stop postgresql freeswitch kamailio keepalived voip-admin redis

# On Node 2:
# (should automatically become MASTER)
/usr/local/bin/system_health.sh

# Verify:
# - VIP moved to Node 2
# - PostgreSQL promoted
# - Redis promoted
# - Calls working
```

---

## 10. DEPLOYMENT CHECKLIST

### 10.1. Pre-Deployment

- [ ] Hardware meets specs (96 GB RAM, 24 cores minimum)
- [ ] Network configured (192.168.1.0/24)
- [ ] Debian 12 installed on both nodes
- [ ] NTP synchronized
- [ ] SSH keys configured
- [ ] Passwords generated (PostgreSQL, Redis, Kamailio)

### 10.2. Node 1 Setup

- [ ] Install PostgreSQL 16 (primary)
- [ ] Install Redis (master)
- [ ] Install Kamailio 6.0.x
- [ ] Install FreeSWITCH 1.10.x
- [ ] Install Keepalived
- [ ] Install lsyncd
- [ ] Deploy VoIP Admin Service
- [ ] Configure all services
- [ ] Create database schemas
- [ ] Test services individually

### 10.3. Node 2 Setup

- [ ] Install PostgreSQL 16 (standby)
- [ ] Setup streaming replication from Node 1
- [ ] Install Redis (slave)
- [ ] Install Kamailio 6.0.x
- [ ] Install FreeSWITCH 1.10.x
- [ ] Install Keepalived
- [ ] Install lsyncd
- [ ] Deploy VoIP Admin Service
- [ ] Configure all services
- [ ] Test replication

### 10.4. Integration Testing

- [ ] Test VIP failover (keepalived)
- [ ] Test PostgreSQL failover
- [ ] Test Redis failover
- [ ] Test SIP registration
- [ ] Test internal calls
- [ ] Test queue calls
- [ ] Test CDR recording
- [ ] Test file synchronization (lsyncd)
- [ ] Load test with SIPp (100 CC)
- [ ] Load test with SIPp (400 CC)
- [ ] Load test with SIPp (600 CC)
- [ ] Disaster recovery test

---

## 11. COST & RESOURCE COMPARISON

| Aspect | Original (9 nodes) | 2-Node Consolidated |
|--------|-------------------|---------------------|
| **Hardware cost** | ~$45,000 | ~$10,000 |
| **Power consumption** | ~9 kW | ~2 kW |
| **Rack space** | 9U | 2U |
| **Network ports** | 18 ports | 4 ports (2 bonded) |
| **Complexity** | High | Medium |
| **Performance** | Optimal | Good (with tuning) |
| **Scalability** | Easy | Moderate |

**Savings**: ~$35,000 upfront, ~$500/month operational

---

## CONCLUSION

This 2-node architecture balances:
- ✅ **Cost efficiency**: 78% hardware cost reduction
- ✅ **Performance**: Can handle 600-800 CC with proper tuning
- ✅ **Simplicity**: Fewer nodes to manage
- ✅ **Flexibility**: Adopts best database design from Project B
- ⚠️ **Trade-off**: Less isolation between services

**Key Success Factors**:
1. High-spec hardware (96+ GB RAM, 24+ cores per node)
2. Proper resource allocation (CPU pinning, I/O priority)
3. Aggressive caching (3-tier cache strategy)
4. Monitoring and alerting (Prometheus + Grafana)
5. Regular failover testing

**Next Steps**:
1. Review and approve this architecture
2. Proceed with detailed implementation guides
3. Create Ansible playbooks for automation
4. Setup monitoring infrastructure
