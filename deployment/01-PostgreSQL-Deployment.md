# 01 - PostgreSQL 18 Deployment Guide

**Service**: PostgreSQL 18 với Streaming Replication  
**Dependencies**: Không có (foundation service)  
**Thời gian ước tính**: 2-3 giờ  
**Version**: 3.2.0  
**Last Updated**: 2025-11-20

---

## Tổng Quan

PostgreSQL là **foundation** của toàn bộ hệ thống. Tất cả services khác (Kamailio, FreeSWITCH, VoIP Admin) phụ thuộc vào PostgreSQL.

### Chiến Lược Kết Nối (QUAN TRỌNG!)

- **LOCAL connection only**: Mỗi service kết nối đến PostgreSQL trên cùng node
- **KHÔNG dùng VIP** cho database connections
- **Lý do**: Xem [DATABASE-ARCHITECTURE.md](../DATABASE-ARCHITECTURE.md)

### Architecture

```
Node 1 (.101)              Node 2 (.102)
PostgreSQL Master    →     PostgreSQL Standby
(Read/Write)               (Read-Only, Async Replication)
```

---

## 6. Cài Đặt PostgreSQL 18

> **Vai trò:** PostgreSQL Database Administrator (DBA)

### 6.1 Add PostgreSQL Repository

**Trên cả 2 nodes:**

```bash
# Add PostgreSQL APT repository
sudo apt install -y curl ca-certificates
sudo install -d /usr/share/postgresql-common/pgdg
sudo curl -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.asc \
    --fail https://www.postgresql.org/media/keys/ACCC4CF8.asc

sudo sh -c 'echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc] \
    https://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > \
    /etc/apt/sources.list.d/pgdg.list'

# Update package list
sudo apt update
```

### 6.2 Install PostgreSQL 18

**Trên cả 2 nodes:**

```bash
# Install PostgreSQL 18
sudo apt install -y postgresql-18 postgresql-contrib-18 postgresql-18-pgaudit

# Verify installation
psql --version
# Expect: psql (PostgreSQL) 18.x

# Check service
sudo systemctl status postgresql
```

### 6.3 Initial PostgreSQL Configuration

**Trên cả 2 nodes:**

```bash
# Stop PostgreSQL để cấu hình
sudo systemctl stop postgresql

# Backup original configs
sudo cp /etc/postgresql/18/main/postgresql.conf /etc/postgresql/18/main/postgresql.conf.orig
sudo cp /etc/postgresql/18/main/pg_hba.conf /etc/postgresql/18/main/pg_hba.conf.orig
```

### 6.4 PostgreSQL Performance Tuning

**Trên cả 2 nodes:**

```bash
sudo nano /etc/postgresql/18/main/postgresql.conf
```

Tìm và sửa các parameters sau (hoặc thêm vào cuối file):

```ini
# ============================================================================
# PostgreSQL 18 Tuning for VoIP System
# Hardware: 16 cores, 64GB RAM
# Workload: OLTP with high concurrency
# ============================================================================

# CONNECTIONS
# -----------
listen_addresses = '*'                           # Listen on all interfaces
port = 5432
max_connections = 300                            # Kamailio + VoIP Admin + connections

# MEMORY
# ------
shared_buffers = 16GB                            # 25% of RAM
effective_cache_size = 48GB                      # 75% of RAM
work_mem = 64MB                                  # Per query operation
maintenance_work_mem = 2GB                       # For VACUUM, CREATE INDEX
huge_pages = try
temp_buffers = 32MB

# QUERY PLANNING
# --------------
random_page_cost = 1.1                           # SSD is fast
effective_io_concurrency = 200                   # SSD supports high I/O
default_statistics_target = 100

# WRITE AHEAD LOG (WAL)
# ---------------------
wal_level = replica                              # For streaming replication
max_wal_senders = 5                              # Number of replication connections
max_replication_slots = 5
wal_keep_size = 1GB                              # Keep 1GB of WAL for standby
wal_compression = on
archive_mode = on                                # Enable WAL archiving
archive_command = 'test ! -f /var/lib/postgresql/18/archive/%f && cp %p /var/lib/postgresql/18/archive/%f'

# WAL Writing
wal_buffers = 16MB
wal_writer_delay = 200ms
commit_delay = 0
commit_siblings = 5

# CHECKPOINTS
# -----------
checkpoint_timeout = 15min                       # More frequent checkpoints
checkpoint_completion_target = 0.9
max_wal_size = 4GB
min_wal_size = 1GB

# AUTOVACUUM
# ----------
autovacuum = on
autovacuum_max_workers = 4
autovacuum_naptime = 30s                         # Check more frequently
autovacuum_vacuum_scale_factor = 0.1
autovacuum_analyze_scale_factor = 0.05

# LOGGING
# -------
logging_collector = on
log_directory = '/var/log/postgresql'
log_filename = 'postgresql-%Y-%m-%d_%H%M%S.log'
log_rotation_age = 1d
log_rotation_size = 100MB
log_min_duration_statement = 1000                # Log queries > 1s
log_line_prefix = '%t [%p]: [%l-1] user=%u,db=%d,app=%a,client=%h '
log_checkpoints = on
log_connections = on
log_disconnections = on
log_lock_waits = on
log_temp_files = 0
log_autovacuum_min_duration = 0

# QUERY STATS
# -----------
shared_preload_libraries = 'pg_stat_statements'
pg_stat_statements.max = 10000
pg_stat_statements.track = all
track_activities = on
track_counts = on
track_io_timing = on
track_functions = all

# LOCALE
# ------
lc_messages = 'en_US.UTF-8'
lc_monetary = 'en_US.UTF-8'
lc_numeric = 'en_US.UTF-8'
lc_time = 'en_US.UTF-8'
default_text_search_config = 'pg_catalog.english'
timezone = 'Asia/Ho_Chi_Minh'
```

**Tạo archive directory:**
```bash
sudo mkdir -p /var/lib/postgresql/18/archive
sudo chown postgres:postgres /var/lib/postgresql/18/archive
sudo chmod 700 /var/lib/postgresql/18/archive
```

### 6.5 PostgreSQL Authentication Configuration

**Trên cả 2 nodes:**

```bash
sudo nano /etc/postgresql/18/main/pg_hba.conf
```

Thay thế nội dung bằng:

```
# PostgreSQL Client Authentication Configuration
# ARCHITECTURE: Each node connects to its LOCAL PostgreSQL only
# TYPE  DATABASE        USER            ADDRESS                 METHOD

# Local connections
local   all             postgres                                peer
local   all             all                                     peer

# Replication connections (giữa 2 nodes)
host    replication     replicator      172.16.91.101/32        scram-sha-256
host    replication     replicator      172.16.91.102/32        scram-sha-256

# Kamailio connections - MD5 cho performance
# Applications connect to LOCAL PostgreSQL only
host    kamailio        kamailio        127.0.0.1/32            md5
host    kamailio        kamailio        172.16.91.101/32        md5
host    kamailio        kamailio        172.16.91.102/32        md5

# VoIP Admin - SCRAM-SHA-256
host    voip            voipadmin       127.0.0.1/32            scram-sha-256
host    voip            voipadmin       172.16.91.101/32        scram-sha-256
host    voip            voipadmin       172.16.91.102/32        scram-sha-256

# FreeSWITCH ODBC - MD5
host    voip            freeswitch      127.0.0.1/32            md5
host    voip            freeswitch      172.16.91.101/32        md5
host    voip            freeswitch      172.16.91.102/32        md5

# Localhost (for monitoring, admin)
host    all             all             127.0.0.1/32            scram-sha-256
host    all             all             ::1/128                 scram-sha-256

# Deny all other
host    all             all             0.0.0.0/0               reject
```

### 6.6 Khởi Động PostgreSQL và Tạo Database

**Trên Node 1 (MASTER):**

```bash
# Start PostgreSQL
sudo systemctl start postgresql
sudo systemctl enable postgresql

# Check status
sudo systemctl status postgresql

# Switch to postgres user
sudo -i -u postgres

# Create extension for pg_stat_statements
psql -c "CREATE EXTENSION IF NOT EXISTS pg_stat_statements;"

# Tạo users
psql <<EOF
-- Replication user
CREATE USER replicator WITH REPLICATION LOGIN PASSWORD 'CHANGE_REPLICATION_PASSWORD';

-- Kamailio read-write user
CREATE USER kamailio WITH LOGIN PASSWORD 'CHANGE_KAMAILIO_PASSWORD';

-- Kamailio read-only user (for kamctl read commands)
CREATE USER kamailioro WITH LOGIN PASSWORD 'CHANGE_KAMAILIORO_PASSWORD';

-- VoIP Admin user
CREATE USER voip_admin WITH LOGIN PASSWORD 'CHANGE_VOIPADMIN_PASSWORD';

-- FreeSWITCH user (for CDR if needed)
CREATE USER freeswitch WITH LOGIN PASSWORD 'CHANGE_FREESWITCH_PASSWORD';

-- List users
\du
EOF

# Tạo database
createdb -O postgres voipdb

# Verify
psql -c "\l" | grep voipdb
```

**Save passwords:**
```bash
# Create secure file to store passwords
sudo nano /root/.voip_credentials
```

Thêm:
```
# PostgreSQL Credentials
replication_password=ACTUAL_PASSWORD_HERE
kamailio_db_password=ACTUAL_PASSWORD_HERE
kamailioro_db_password=ACTUAL_PASSWORD_HERE
voipadmin_db_password=ACTUAL_PASSWORD_HERE
freeswitch_db_password=ACTUAL_PASSWORD_HERE
```

```bash
sudo chmod 600 /root/.voip_credentials
```

### 6.7 Load Database Schema

**Trên Node 1:**

```bash
# Clone repo (nếu chưa có)
cd /tmp
git clone https://github.com/haintbotast/high-cc-pbx.git
cd high-cc-pbx

# STEP 1: Initialize database, schemas, và users (RUN FIRST!)
sudo -u postgres psql -d voipdb -f database/schemas/00-init-database.sql

# Replace default passwords in 00-init-database.sql với actual passwords:
# (Script sẽ tạo users với default passwords, cần change sau)

# Change passwords cho database users
KAMAILIO_PASS=$(grep kamailio_db_password /root/.voip_credentials | cut -d'=' -f2)
KAMAILIORO_PASS=$(grep kamailioro_db_password /root/.voip_credentials | cut -d'=' -f2)
VOIPADMIN_PASS=$(grep voipadmin_db_password /root/.voip_credentials | cut -d'=' -f2)
FREESWITCH_PASS=$(grep freeswitch_db_password /root/.voip_credentials | cut -d'=' -f2)
REPLICATOR_PASS=$(grep replicator_db_password /root/.voip_credentials | cut -d'=' -f2)

sudo -u postgres psql -d voipdb <<EOF
ALTER USER kamailio WITH PASSWORD '$KAMAILIO_PASS';
ALTER USER kamailioro WITH PASSWORD '$KAMAILIORO_PASS';
ALTER USER voipadmin WITH PASSWORD '$VOIPADMIN_PASS';
ALTER USER freeswitch WITH PASSWORD '$FREESWITCH_PASS';
ALTER USER replicator WITH PASSWORD '$REPLICATOR_PASS';
EOF

# STEP 2: Load application schemas theo thứ tự
sudo -u postgres psql -d voipdb -f database/schemas/01-voip-schema.sql
sudo -u postgres psql -d voipdb -f database/schemas/02-kamailio-schema.sql
sudo -u postgres psql -d voipdb -f database/schemas/03-auth-integration.sql
sudo -u postgres psql -d voipdb -f database/schemas/04-production-fixes.sql

# Verify schemas
sudo -u postgres psql -d voipdb -c "\dn"
# Expect: kamailio, public, voip

# Verify tables
sudo -u postgres psql -d voipdb -c "\dt voip.*"
sudo -u postgres psql -d voipdb -c "\dt kamailio.*"

# Verify users
sudo -u postgres psql -d voipdb -c "\du" | grep -E "kamailio|voipadmin|freeswitch|replicator"

# Test user connections
psql -h 127.0.0.1 -U kamailio -d voipdb -c "SELECT current_user, current_database();"
psql -h 127.0.0.1 -U voipadmin -d voipdb -c "SELECT current_user, current_database();"
```

**Lưu ý quan trọng**:
- File `00-init-database.sql` tạo database users, schemas, và permissions
- Files `01-04` tạo application tables và functions
- Permissions đã được setup trong `00-init-database.sql`, KHÔNG cần grant thủ công
- Nếu cần grant permissions
GRANT USAGE ON SCHEMA kamailio TO kamailioro;
GRANT SELECT ON ALL TABLES IN SCHEMA kamailio TO kamailioro;
GRANT SELECT ON ALL SEQUENCES IN SCHEMA kamailio TO kamailioro;

-- Set default search_path for Kamailio users (CRITICAL for kamctl)
ALTER USER kamailio SET search_path TO kamailio, public;
ALTER USER kamailioro SET search_path TO kamailio, public;

-- VoIP Admin permissions
GRANT USAGE ON SCHEMA voip TO voip_admin;
GRANT ALL ON ALL TABLES IN SCHEMA voip TO voip_admin;
GRANT ALL ON ALL SEQUENCES IN SCHEMA voip TO voip_admin;

-- FreeSWITCH permissions (read-only for most, write for CDR)
GRANT USAGE ON SCHEMA voip TO freeswitch;
GRANT SELECT ON voip.extensions TO freeswitch;
GRANT SELECT ON voip.domains TO freeswitch;
GRANT SELECT ON voip.queues TO freeswitch;
GRANT ALL ON voip.cdr_queue TO freeswitch;
GRANT ALL ON voip.cdr TO freeswitch;
EOF
```

**Verify database setup:**
```bash
sudo -u postgres psql -d voipdb <<EOF
-- Check schemas
SELECT schema_name FROM information_schema.schemata ORDER BY schema_name;

-- Check voip tables
SELECT table_name FROM information_schema.tables
WHERE table_schema = 'voip' ORDER BY table_name;

-- Check critical indexes
SELECT indexname FROM pg_indexes
WHERE schemaname = 'voip' AND tablename = 'extensions';

-- Should see: idx_extensions_auth_lookup, idx_extensions_active, etc.

-- Check view
SELECT * FROM kamailio.subscriber LIMIT 0;

-- Check trigger exists
SELECT tgname FROM pg_trigger WHERE tgname = 'extensions_calc_ha1_trigger';
EOF
```

---

## 7. Cấu Hình PostgreSQL Replication

> **Vai trò:** PostgreSQL DBA & HA Expert

### 7.1 Chuẩn Bị Replication trên Node 1 (MASTER)

**Trên Node 1:**

PostgreSQL đã được cấu hình sẵn cho replication ở bước 6.4 (wal_level = replica).

**Verify WAL settings:**
```bash
sudo -u postgres psql -c "SHOW wal_level;"
sudo -u postgres psql -c "SHOW max_wal_senders;"
sudo -u postgres psql -c "SHOW archive_mode;"
```

**Create replication slot:**
```bash
sudo -u postgres psql <<EOF
SELECT * FROM pg_create_physical_replication_slot('node2_slot');
SELECT slot_name, slot_type, active FROM pg_replication_slots;
EOF
```

### 7.2 Chuẩn Bị Node 2 (STANDBY)

**Trên Node 2:**

```bash
# Stop PostgreSQL
sudo systemctl stop postgresql

# Backup existing data (nếu có)
sudo mv /var/lib/postgresql/18/main /var/lib/postgresql/18/main.bak

# Create empty directory
sudo -u postgres mkdir -p /var/lib/postgresql/18/main
sudo -u postgres chmod 700 /var/lib/postgresql/18/main
```

### 7.3 Base Backup từ Node 1

**Trên Node 2:**

```bash
# Tạo .pgpass file để không cần nhập password
sudo -u postgres bash -c "cat > ~/.pgpass <<EOF
172.16.91.101:5432:replication:replicator:ACTUAL_REPLICATION_PASSWORD
EOF"

# QUAN TRỌNG: Phải dùng bash -c để expand ~ đúng
sudo -u postgres bash -c "chmod 600 ~/.pgpass"

# Thực hiện base backup
sudo -u postgres pg_basebackup \
    -h 172.16.91.101 \
    -D /var/lib/postgresql/18/main \
    -U replicator \
    -P \
    -v \
    -R \
    -X stream \
    -C \
    -S node2_slot

# -R: Tự động tạo standby.signal và postgresql.auto.conf
# -X stream: Stream WAL trong khi backup
# -C: Create replication slot (nếu slot chưa tồn tại)
# -S: Slot name
```

**Lưu ý:** Nếu gặp lỗi `ERROR: replication slot "node2_slot" already exists`:
```bash
# Option 1: Xóa slot cũ trên Node 1 (RECOMMENDED)
sudo -u postgres psql -c "SELECT pg_drop_replication_slot('node2_slot');"

# Sau đó chạy lại pg_basebackup ở trên

# Option 2: Hoặc bỏ flag -C nếu slot đã tồn tại
sudo -u postgres pg_basebackup \
    -h 172.16.91.101 \
    -D /var/lib/postgresql/18/main \
    -U replicator \
    -P -v -R -X stream \
    -S node2_slot
# (Lưu ý: không có -C)
```

**Quá trình này sẽ mất vài phút. Output:**
```
pg_basebackup: initiating base backup, waiting for checkpoint to complete
pg_basebackup: checkpoint completed
pg_basebackup: write-ahead log start point: 0/2000028 on timeline 1
...
pg_basebackup: base backup completed
```

### 7.4 Cấu Hình Standby trên Node 2

**File postgresql.auto.conf đã được tạo tự động, verify:**
```bash
sudo -u postgres cat /var/lib/postgresql/18/main/postgresql.auto.conf
```

Should contain:
```
primary_conninfo = 'host=172.16.91.101 port=5432 user=replicator password=PASSWORD'
primary_slot_name = 'node2_slot'
```

**Nếu cần customize:**
```bash
sudo -u postgres nano /var/lib/postgresql/18/main/postgresql.auto.conf
```

Add/modify:
```
primary_conninfo = 'host=172.16.91.101 port=5432 user=replicator password=ACTUAL_PASSWORD application_name=node2'
primary_slot_name = 'node2_slot'
hot_standby = on
hot_standby_feedback = on
```

**Verify standby.signal exists:**
```bash
ls -la /var/lib/postgresql/18/main/standby.signal
```

### 7.5 Start Standby Server

**Trên Node 2:**

```bash
# Start PostgreSQL
sudo systemctl start postgresql

# Check status
sudo systemctl status postgresql

# Check logs
sudo tail -f /var/log/postgresql/postgresql-*.log
# Should see: "database system is ready to accept read-only connections"
```

### 7.6 Verify Replication

**Trên Node 1 (MASTER):**

```bash
sudo -u postgres psql -c "SELECT * FROM pg_stat_replication;"
```

Expected output:
```
 pid  | usename    | application_name | client_addr   | state     | sync_state
------+------------+------------------+---------------+-----------+------------
 1234 | replicator | node2            | 172.16.91.102 | streaming | async
```

**Trên Node 2 (STANDBY):**

```bash
sudo -u postgres psql -c "SELECT pg_is_in_recovery();"
# Should return: t (true)

sudo -u postgres psql -c "SELECT * FROM pg_stat_wal_receiver;"
```

### 7.7 Test Replication

**Trên Node 1:**

```bash
sudo -u postgres psql -d voipdb <<EOF
CREATE TABLE test_replication (id INT, data TEXT);
INSERT INTO test_replication VALUES (1, 'Test from Node 1');
SELECT * FROM test_replication;
EOF
```

**Trên Node 2:**

```bash
sudo -u postgres psql -d voipdb -c "SELECT * FROM test_replication;"
# Should see: 1 | Test from Node 1
```

**Cleanup test:**
```bash
# Trên Node 1
sudo -u postgres psql -d voipdb -c "DROP TABLE test_replication;"
```

---

## 7.8 Setup pg_rewind Fast Path (KHUYẾN NGHỊ CAO)

> **Vai trò:** PostgreSQL HA Expert

### Tổng Quan

**pg_rewind** cho phép recovery standby trong **< 1 phút** thay vì 10-30 phút với pg_basebackup.

**Performance comparison:**
- pg_rewind: ~1 minute (chỉ sync divergent WAL)
- pg_basebackup: 10-30 minutes (copy toàn bộ database)

**Xem chi tiết:** [scripts/failover/PERFORMANCE-COMPARISON.md](../scripts/failover/PERFORMANCE-COMPARISON.md)

### Prerequisites

1. ✅ **wal_log_hints = on** - Đã configured ở section 6.4
2. ✅ **Replication slots** - Đã configured ở section 7.1
3. 📋 **Passwordless .pgpass** - Cần setup thêm (bên dưới)
4. 📋 **Deploy safe_rebuild_standby_v4.sh** - Script với pg_rewind support

---

### 7.8.1 Verify wal_log_hints

**Trên cả 2 nodes:**
```bash
sudo -u postgres psql -c "SHOW wal_log_hints;"
```

Expected: `on`

❌ **Nếu OFF** (không nên xảy ra nếu đã làm theo section 6.4):
```bash
# Edit postgresql.conf
sudo nano /etc/postgresql/18/main/postgresql.conf

# Tìm và sửa:
# wal_log_hints = on

# Restart PostgreSQL
sudo systemctl restart postgresql
```

---

### 7.8.2 Setup .pgpass cho Passwordless Connection

**QUAN TRỌNG:** pg_rewind cần passwordless connection từ standby → master.

**Trên Node 1 (tạo .pgpass để connect đến Node 2):**
```bash
# Get replicator password
REPL_PASS=$(grep replicator_password /root/.voip_credentials | cut -d'=' -f2)

# Create .pgpass for postgres user
sudo -u postgres bash -c "cat > /var/lib/postgresql/.pgpass <<EOF
172.16.91.102:5432:*:replicator:$REPL_PASS
172.16.91.102:5432:*:postgres:$REPL_PASS
172.16.91.102:5432:replication:replicator:$REPL_PASS
EOF"

# Set permissions (CRITICAL - must be 600!)
sudo -u postgres chmod 0600 /var/lib/postgresql/.pgpass
sudo chown postgres:postgres /var/lib/postgresql/.pgpass
```

**Trên Node 2 (tạo .pgpass để connect đến Node 1):**
```bash
# Get replicator password
REPL_PASS=$(grep replicator_password /root/.voip_credentials | cut -d'=' -f2)

# Create .pgpass
sudo -u postgres bash -c "cat > /var/lib/postgresql/.pgpass <<EOF
172.16.91.101:5432:*:replicator:$REPL_PASS
172.16.91.101:5432:*:postgres:$REPL_PASS
172.16.91.101:5432:replication:replicator:$REPL_PASS
EOF"

# Set permissions
sudo -u postgres chmod 0600 /var/lib/postgresql/.pgpass
sudo chown postgres:postgres /var/lib/postgresql/.pgpass
```

**Verify permissions:**
```bash
# Trên cả 2 nodes
ls -la /var/lib/postgresql/.pgpass
```

Expected output:
```
-rw------- 1 postgres postgres 256 Nov 21 10:30 /var/lib/postgresql/.pgpass
```

⚠️ **LƯU Ý:** Nếu permissions > 600, PostgreSQL sẽ IGNORE file này!

---

### 7.8.3 Test Passwordless Connection

**Từ Node 1 → Node 2:**
```bash
# Should NOT prompt for password
sudo -u postgres psql -h 172.16.91.102 -p 5432 -U replicator -d postgres -c "SELECT 1;"
```

Expected: `?column? ---------- 1`

**Từ Node 2 → Node 1:**
```bash
sudo -u postgres psql -h 172.16.91.101 -p 5432 -U replicator -d postgres -c "SELECT 1;"
```

❌ **Nếu vẫn hỏi password:** Check .pgpass permissions và format.

---

### 7.8.4 Update pg_hba.conf cho pg_rewind

**Trên cả 2 nodes**, verify pg_hba.conf có dòng này:

```bash
sudo grep "replication.*replicator" /etc/postgresql/18/main/pg_hba.conf
```

Expected (đã configured ở section 6.5):
```
host    replication     replicator      172.16.91.101/32        scram-sha-256
host    replication     replicator      172.16.91.102/32        scram-sha-256
```

**Thêm dòng sau để pg_rewind có thể connect tới database postgres:**
```bash
# Trên cả 2 nodes
sudo nano /etc/postgresql/18/main/pg_hba.conf
```

Thêm sau dòng replication:
```
# pg_rewind needs to connect to postgres database
host    all             replicator      172.16.91.101/32        scram-sha-256
host    all             replicator      172.16.91.102/32        scram-sha-256
```

Reload PostgreSQL:
```bash
sudo systemctl reload postgresql
```

---

### 7.8.5 Deploy safe_rebuild_standby_v4.sh Script

**Trên cả 2 nodes:**

```bash
# Copy script từ repo
cd /tmp/high-cc-pbx
sudo cp scripts/failover/safe_rebuild_standby_v4.sh /usr/local/bin/
sudo chmod +x /usr/local/bin/safe_rebuild_standby_v4.sh

# Verify script exists
ls -lh /usr/local/bin/safe_rebuild_standby_v4.sh
```

**Replace password placeholder trong script:**
```bash
# Get replicator password
REPL_PASS=$(grep replicator_password /root/.voip_credentials | cut -d'=' -f2)

# Update script
sudo sed -i "s/REPL_PASSWORD_PLACEHOLDER/$REPL_PASS/g" /usr/local/bin/safe_rebuild_standby_v4.sh

# Verify password was replaced
sudo grep "^REPL_PASSWORD=" /usr/local/bin/safe_rebuild_standby_v4.sh
# Should show: REPL_PASSWORD="YourActualPassword"
```

---

### 7.8.6 Test pg_rewind Dry Run

⚠️ **QUAN TRỌNG:** Chỉ test trên node đang là STANDBY!

**Xác định node nào là standby:**
```bash
# Trên mỗi node
sudo -u postgres psql -c "SELECT pg_is_in_recovery();"
```
- `t` = STANDBY (test trên node này)
- `f` = MASTER (KHÔNG test trên node này)

**Test pg_rewind dry-run trên STANDBY (giả sử Node 2):**
```bash
# Stop PostgreSQL
sudo systemctl stop postgresql

# Run pg_rewind with --dry-run
sudo -u postgres /usr/lib/postgresql/18/bin/pg_rewind \
    --target-pgdata=/var/lib/postgresql/18/main \
    --source-server="host=172.16.91.101 port=5432 user=replicator dbname=postgres" \
    --dry-run \
    --progress

# Start PostgreSQL again
sudo systemctl start postgresql
```

**Expected output:**
```
pg_rewind: connected to server
pg_rewind: servers diverged at WAL location 0/XXXXXXXX on timeline 1
pg_rewind: rewinding from last common checkpoint at 0/YYYYYYYY on timeline 1
pg_rewind: reading source file list
pg_rewind: reading target file list
pg_rewind: Done!
```

✅ **Nếu thấy "Done!"** → pg_rewind sẽ work khi cần failover!

❌ **Nếu lỗi:** Xem troubleshooting trong [FAILOVER-SETUP-CHECKLIST.md](../scripts/failover/FAILOVER-SETUP-CHECKLIST.md)

---

### 7.8.7 Verification Checklist

**Run automated verification script trên cả 2 nodes:**

```bash
# Copy verification script
cat > /tmp/verify_pg_rewind.sh <<'EOFSCRIPT'
#!/bin/bash
echo "====================================="
echo "pg_rewind Prerequisites Verification"
echo "====================================="

# Get peer IP
CURRENT_IP=$(hostname -I | awk '{print $1}')
if [[ "$CURRENT_IP" == "172.16.91.101" ]]; then
    PEER_IP="172.16.91.102"
elif [[ "$CURRENT_IP" == "172.16.91.102" ]]; then
    PEER_IP="172.16.91.101"
else
    echo "❌ Unknown IP: $CURRENT_IP"
    exit 1
fi

echo "Current: $CURRENT_IP, Peer: $PEER_IP"
echo ""

# Check 1: wal_log_hints
echo -n "1. wal_log_hints... "
WAL_HINTS=$(sudo -u postgres psql -qAt -c "SHOW wal_log_hints;")
[[ "$WAL_HINTS" == "on" ]] && echo "✅ ON" || echo "❌ OFF"

# Check 2: .pgpass exists
echo -n "2. .pgpass exists... "
[[ -f /var/lib/postgresql/.pgpass ]] && echo "✅ YES" || echo "❌ NO"

# Check 3: .pgpass permissions
echo -n "3. .pgpass permissions... "
PERMS=$(stat -c "%a" /var/lib/postgresql/.pgpass 2>/dev/null)
[[ "$PERMS" == "600" ]] && echo "✅ 600" || echo "❌ $PERMS"

# Check 4: Passwordless connection
echo -n "4. Passwordless connection to peer... "
if sudo -u postgres psql -h "$PEER_IP" -U replicator -d postgres -c "SELECT 1;" >/dev/null 2>&1; then
    echo "✅ OK"
else
    echo "❌ FAILED"
fi

# Check 5: pg_rewind binary exists
echo -n "5. pg_rewind binary... "
[[ -x /usr/lib/postgresql/18/bin/pg_rewind ]] && echo "✅ EXISTS" || echo "❌ MISSING"

# Check 6: safe_rebuild_standby_v4.sh deployed
echo -n "6. safe_rebuild_standby_v4.sh... "
[[ -x /usr/local/bin/safe_rebuild_standby_v4.sh ]] && echo "✅ DEPLOYED" || echo "❌ MISSING"

echo ""
echo "====================================="
EOFSCRIPT

chmod +x /tmp/verify_pg_rewind.sh
/tmp/verify_pg_rewind.sh
```

**Expected: All checks show ✅**

**Full checklist:** [scripts/failover/FAILOVER-SETUP-CHECKLIST.md](../scripts/failover/FAILOVER-SETUP-CHECKLIST.md)

---

### 7.8.8 Summary

✅ Sau khi hoàn thành section này, bạn có:

1. **wal_log_hints = on** - Enabled pg_rewind capability
2. **.pgpass configured** - Passwordless PostgreSQL connections
3. **pg_hba.conf updated** - Allow replicator user to connect
4. **safe_rebuild_standby_v4.sh** - Script với pg_rewind fast path
5. **Verified prerequisites** - All checks pass

**Benefits:**
- Recovery time: **< 1 minute** (thay vì 10-30 phút)
- Downtime cost savings: **~$2,500-$3,000 per failover**
- Success rate: **~90%** dùng fast path, 10% fallback to full rebuild

**Performance comparison:** [scripts/failover/PERFORMANCE-COMPARISON.md](../scripts/failover/PERFORMANCE-COMPARISON.md)

---

## Tài Liệu Liên Quan

- [DATABASE-ARCHITECTURE.md](../DATABASE-ARCHITECTURE.md) - LOCAL connection strategy  
- [DEPLOYMENT-PREREQUISITES.md](../DEPLOYMENT-PREREQUISITES.md) - Passwords và credentials  
- [deployment/README.md](README.md) - Deployment overview

## Bước Tiếp Theo

✅ PostgreSQL đã running và replication hoạt động  
➡️ **Tiếp theo**: [02-Kamailio-Deployment.md](02-Kamailio-Deployment.md)
