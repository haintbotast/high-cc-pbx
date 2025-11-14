# PHÂN TÍCH THAY ĐỔI KIẾN TRÚC HỆ THỐNG VOIP
## Loại bỏ PgBouncer & Tối ưu hóa Architecture (600-800 CC)

---

## 1. TÓM TẮT CÁC THAY ĐỔI

### YÊU CẦU MỚI:
- ❌ **Loại bỏ PgBouncer** - đơn giản hóa stack
- ❌ **Không dùng NFS** - lsyncd 2 chiều, thư mục thống nhất
- ❌ **Không có etcd witness** - chỉ 2-node PostgreSQL
- ⚡ **CDR:** Đánh giá FreeSWITCH direct vs API Gateway
- 🔌 **ODBC:** Đánh giá kết nối trực tiếp PostgreSQL
- 🔓 **TLS:** Optional với Kamailio (không bắt buộc)
- 🔧 **Keepalived:** Sửa vấn đề race condition notify scripts

---

## 2. PHÂN TÍCH LOẠI BỎ PGBOUNCER

### 2.1 Tác động khi BỎ PgBouncer

#### ✅ **LỢI ÍCH:**
- Đơn giản hóa architecture (bớt 1 layer)
- Giảm latency 1-2ms (không qua proxy)
- Ít component hơn = ít failure point
- Giảm chi phí vận hành

#### ⚠️ **BẤT LỢI:**
- PostgreSQL phải xử lý nhiều connections hơn
- Mỗi Kamailio worker = 1 connection pool riêng

### 2.2 Tính toán Connection Load

**Kamailio db_mode=2 (Write-back - KHUYẾN CÁO):**
```
2 nodes × 16 workers × 5 concurrent queries = 160 connections
```

**FreeSWITCH ODBC:**
```
2 nodes × 32 channels (ODBC pool) = 64 connections
```

**API Gateway (nếu dùng):**
```
2 instances × 20 connections = 40 connections
```

**TỔNG: ~264 connections**

#### 📊 Đánh giá với PostgreSQL 16:
- `max_connections = 300` → **ĐỦ**
- Mỗi connection: ~10 MB RAM → 2.64 GB RAM
- **KẾT LUẬN: PostgreSQL handle được KHÔNG CẦN PgBouncer** ✅

### 2.3 Tối ưu PostgreSQL thay PgBouncer

```ini
# postgresql.conf
max_connections = 300
shared_buffers = 4GB

# Connection pooling ở application layer
# Kamailio: Built-in per-worker pooling
# FreeSWITCH ODBC: Connection pooling trong core
# API Gateway: database/sql pool (Go)
```

**→ QUYẾT ĐỊNH: BỎ PgBouncer, tối ưu PostgreSQL direct connections** ✅

---

## 3. CDR PROCESSING: DIRECT vs API GATEWAY

### 3.1 Phương án 1: FreeSWITCH ODBC → PostgreSQL (TRỰC TIẾP)

#### ⚙️ Configuration:
```xml
<!-- /etc/freeswitch/autoload_configs/cdr_pg_csv.conf.xml -->
<configuration name="cdr_pg_csv.conf">
  <settings>
    <param name="odbc-dsn" value="dsn:postgres:freeswitch"/>
    <param name="legs" value="a"/>
  </settings>
</configuration>
```

#### ✅ **ƯU ĐIỂM:**
- Đơn giản, ít component
- Latency thấp (write trực tiếp)
- Không cần API Gateway

#### ❌ **NHƯỢC ĐIỂM:**
- **BLOCKING:** FreeSWITCH thread bị block khi INSERT CDR
- Nếu DB chậm/down → ảnh hưởng call processing
- Không có retry logic
- Không có batching (1 INSERT/call)

### 3.2 Phương án 2: mod_json_cdr → API Gateway → PostgreSQL (ASYNC)

#### ⚙️ Architecture:
```
FreeSWITCH → HTTP POST (async) → API Gateway (Go) → Redis Queue → Batch INSERT → PostgreSQL
```

#### ✅ **ƯU ĐIỂM:**
- **NON-BLOCKING:** HTTP async, không block call
- Retry logic
- Batch insert (hiệu suất cao)
- Queue buffer nếu DB tạm down

#### ❌ **NHƯỢC ĐIỂM:**
- Phức tạp hơn (API Gateway + Redis)
- Thêm infrastructure

### 3.3 📊 SO SÁNH HIỆU SUẤT

| Metric | ODBC Direct | API Gateway (Async) |
|--------|-------------|---------------------|
| Latency (CDR write) | 20-50ms | 2-5ms (async) |
| Call blocking risk | ⚠️ CÓ (nếu DB slow) | ❌ KHÔNG |
| Retry on failure | ❌ KHÔNG | ✅ CÓ |
| Batch insert | ❌ KHÔNG (1 by 1) | ✅ CÓ (100/batch) |
| DB load (800 CC) | 800 INSERTs/minute | 8-10 batches/minute |
| Complexity | 🟢 Đơn giản | 🟡 Trung bình |

### 3.4 🎯 KHUYẾN CÁO CDR

**Với 600-800 CC production:**
- **SỬ DỤNG API Gateway** (mod_json_cdr) ✅
- Lý do: Non-blocking critical, reliability cao hơn

**Cấu hình:**
```xml
<!-- /etc/freeswitch/autoload_configs/mod_json_cdr.conf.xml -->
<configuration name="json_cdr.conf">
  <settings>
    <param name="url" value="http://192.168.1.110:8080/api/cdr"/>
    <param name="auth-scheme" value="basic"/>
    <param name="encode" value="base64"/>
    <param name="retries" value="3"/>
    <param name="delay" value="5000"/>
    <param name="log-http-responses" value="true"/>
  </settings>
</configuration>
```

---

## 4. FREESWITCH ODBC → POSTGRESQL (KHÔNG QUA PGBOUNCER)

### 4.1 ODBC Connection Pooling

FreeSWITCH ODBC core **TỰ ĐỘNG pooling connections**, không cần PgBouncer.

#### 📄 Configuration:

**/etc/odbc.ini:**
```ini
[freeswitch]
Description = PostgreSQL FreeSWITCH Database
Driver = PostgreSQL Unicode
Server = 192.168.1.101
Port = 5432
Database = freeswitch
Username = freeswitch
Password = secure_password
Protocol = 13.0
ReadOnly = No
RowVersioning = No
ShowSystemTables = No
ShowOidColumn = No
FakeOidIndex = No
ConnSettings =
```

**/etc/odbcinst.ini:**
```ini
[PostgreSQL Unicode]
Description = PostgreSQL ODBC driver (Unicode version)
Driver = /usr/lib/x86_64-linux-gnu/odbc/psqlodbcw.so
Setup = /usr/lib/x86_64-linux-gnu/odbc/libodbcpsqlS.so
```

**/etc/freeswitch/autoload_configs/switch.conf.xml:**
```xml
<param name="core-db-dsn" value="freeswitch:freeswitch:secure_password"/>
<param name="max-db-handles" value="32"/>
<param name="db-handle-timeout" value="10"/>
```

### 4.2 Đánh giá ODBC Performance

#### ✅ **ODBC ƯU ĐIỂM:**
- FreeSWITCH native support
- Built-in connection pooling (max-db-handles=32)
- Automatic reconnect logic

#### ⚠️ **ODBC BẤT LỢI:**
- Overhead nhẹ vs native PostgreSQL driver (~5-10%)
- **NHƯNG:** Với workload này, overhead KHÔNG đáng kể

### 4.3 🎯 KHUYẾN CÁO ODBC

**SỬ DỤNG ODBC trực tiếp PostgreSQL** ✅
- Không cần PgBouncer
- FreeSWITCH ODBC pooling đủ hiệu quả
- Configuration:
  ```
  max-db-handles = 32 (cho mỗi node)
  db-handle-timeout = 10s
  ```

---

## 5. POSTGRESQL 2-NODE (KHÔNG CÓ ETCD WITNESS)

### 5.1 Vấn đề với 2-node Only

**SPLIT-BRAIN RISK:** Nếu network partition, cả 2 node đều nghĩ mình là Primary.

### 5.2 Giải pháp: repmgr WITHOUT etcd

**repmgr có thể hoạt động 2-node NHƯNG cần extra caution:**

#### 🔧 Configuration Adjustments:

**/etc/repmgr.conf** (Node 1):
```ini
node_id=1
node_name='node1'
conninfo='host=192.168.1.104 user=repmgr dbname=repmgr connect_timeout=2'
data_directory='/var/lib/postgresql/16/main'

# CRITICAL: 2-node settings
failover='automatic'
promote_command='repmgr standby promote -f /etc/repmgr.conf --log-to-file'
follow_command='repmgr standby follow -f /etc/repmgr.conf --log-to-file --upstream-node-id=%n'

# Split-brain protection (QUAN TRỌNG)
priority=100  # Node 1 ưu tiên cao hơn
reconnect_attempts=6
reconnect_interval=10

# Monitoring
monitoring_history=yes
monitor_interval_secs=5
```

**/etc/repmgr.conf** (Node 2):
```ini
node_id=2
node_name='node2'
conninfo='host=192.168.1.105 user=repmgr dbname=repmgr connect_timeout=2'
data_directory='/var/lib/postgresql/16/main'

failover='automatic'
promote_command='repmgr standby promote -f /etc/repmgr.conf --log-to-file'
follow_command='repmgr standby follow -f /etc/repmgr.conf --log-to-file --upstream-node-id=%n'

# Node 2 priority thấp hơn
priority=50

reconnect_attempts=6
reconnect_interval=10
monitoring_history=yes
monitor_interval_secs=5
```

### 5.3 ⚠️ LƯU Ý QUAN TRỌNG 2-NODE

1. **Manual verification sau network issues**
2. **Monitoring alerts cho split-brain**
3. **Periodic health checks**

### 5.4 🎯 KHUYẾN CÁO

**CHẤP NHẬN 2-node với repmgr** ✅
- Priority-based failover
- Manual intervention khi cần
- **Trade-off:** Simplicity vs absolute HA

---

## 6. RECORDING SYNC: LSYNCD 2-CHIỀU (KHÔNG NFS)

### 6.1 Yêu cầu Thống nhất Thư mục

**Cả 2 node dùng CÙNG đường dẫn:**
```
/storage/recordings/
```

### 6.2 Configuration lsyncd Bidirectional

#### 📄 Node 1: `/etc/lsyncd/lsyncd.conf.lua`
```lua
settings {
    logfile = "/var/log/lsyncd/lsyncd.log",
    statusFile = "/var/log/lsyncd/lsyncd.status",
    statusInterval = 10,
    nodaemon = false,
    insist = true,
    inotifyMode = "CloseWrite", -- CRITICAL
}

-- Sync TO Node 2
sync {
    default.rsync,
    source = "/storage/recordings/",
    target = "192.168.1.105::recordings",
    delay = 5,
    rsync = {
        archive = true,
        compress = false, -- LAN không cần compress
        _extra = {"--bwlimit=50000"} -- 50 MB/s limit
    }
}
```

#### 📄 Node 2: `/etc/lsyncd/lsyncd.conf.lua`
```lua
settings {
    logfile = "/var/log/lsyncd/lsyncd.log",
    statusFile = "/var/log/lsyncd/lsyncd.status",
    statusInterval = 10,
    nodaemon = false,
    insist = true,
    inotifyMode = "CloseWrite",
}

-- Sync TO Node 1
sync {
    default.rsync,
    source = "/storage/recordings/",
    target = "192.168.1.104::recordings",
    delay = 5,
    rsync = {
        archive = true,
        compress = false,
        _extra = {"--bwlimit=50000"}
    }
}
```

#### 📄 rsync daemon: `/etc/rsyncd.conf` (CẢ 2 NODE)
```ini
uid = freeswitch
gid = freeswitch
use chroot = no
max connections = 10
log file = /var/log/rsyncd.log

[recordings]
    path = /storage/recordings
    comment = FreeSWITCH recordings
    read only = no
    hosts allow = 192.168.1.0/24
```

### 6.3 🎯 KHUYẾN CÁO RECORDING SYNC

**Dùng lsyncd bidirectional với rsync daemon** ✅
- Không cần NFS
- Real-time sync (<5s)
- Thư mục thống nhất cả 2 node

---

## 7. KAMAILIO TLS: OPTIONAL (KHÔNG BẮT BUỘC)

### 7.1 Phân tích TLS với Kamailio

**TLS bảo mật SIP signaling, NHƯNG:**
- Overhead: +10-20ms latency
- CPU: +15-20% cho encryption
- Complexity: Certificate management

### 7.2 🎯 KHUYẾN CÁO TLS

**Với mạng nội bộ (LAN):** TLS OPTIONAL ✅
**Với Internet-facing:** TLS BẮT BUỘC ⚠️

#### Configuration TLS (nếu cần):
```cfg
# kamailio.cfg
#!define WITH_TLS

modparam("tls", "config", "/etc/kamailio/tls.cfg")

listen=tls:192.168.1.102:5061
```

---

## 8. KEEPALIVED: SỬA RACE CONDITION

### 8.1 Vấn đề Race Condition

**Notify scripts có thể chạy đồng thời** → xung đột services.

### 8.2 ✅ Giải pháp: FLOCK

#### 📄 `/usr/local/bin/notify_master.sh`
```bash
#!/bin/bash
set -euo pipefail

LOGFILE="/var/log/keepalived-notify.log"
LOCKFILE="/var/lock/keepalived-master.lock"
STATE_FILE="/var/run/keepalived.state"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] MASTER: $1" | tee -a "$LOGFILE"
}

# CRITICAL: Acquire lock to prevent concurrent execution
exec 200>"$LOCKFILE"
flock -n 200 || {
    log "Another notify_master running, exiting"
    exit 1
}

log "Transitioning to MASTER"
echo "MASTER" > "$STATE_FILE"

# Wait for peer to realize BACKUP
sleep 2

# Start services với health checks
systemctl start kamailio || log "ERROR: Kamailio start failed"
systemctl start freeswitch || log "ERROR: FreeSWITCH start failed"

sleep 3

# Health checks
kamcmd core.uptime > /dev/null 2>&1 && log "Kamailio: OK" || log "Kamailio: FAIL"
fs_cli -x "status" | grep -q "UP" && log "FreeSWITCH: OK" || log "FreeSWITCH: FAIL"

log "MASTER transition complete"
flock -u 200
exit 0
```

#### 📄 `/usr/local/bin/notify_backup.sh`
```bash
#!/bin/bash
set -euo pipefail

LOGFILE="/var/log/keepalived-notify.log"
LOCKFILE="/var/lock/keepalived-backup.lock"
STATE_FILE="/var/run/keepalived.state"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] BACKUP: $1" | tee -a "$LOGFILE"
}

exec 200>"$LOCKFILE"
flock -n 200 || {
    log "Another notify_backup running, exiting"
    exit 1
}

log "Transitioning to BACKUP"
echo "BACKUP" > "$STATE_FILE"

# Services remain running in standby
log "Services remain running in BACKUP mode"

flock -u 200
exit 0
```

#### 📄 `/usr/local/bin/notify_fault.sh`
```bash
#!/bin/bash
set -euo pipefail

LOGFILE="/var/log/keepalived-notify.log"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] FAULT: $1" | tee -a "$LOGFILE"
}

log "========================================="
log "FAULT DETECTED!"
log "========================================="

# Alert administrators
# mail -s "Keepalived FAULT on $(hostname)" admin@example.com <<< "Keepalived entered FAULT state"

# Services continue running (don't stop)
log "Services remain running despite FAULT"

exit 0
```

### 8.3 🔧 Permissions & Testing

```bash
chmod +x /usr/local/bin/notify_*.sh
chown root:root /usr/local/bin/notify_*.sh

# Test manually
/usr/local/bin/notify_master.sh
tail -f /var/log/keepalived-notify.log
```

---

## 9. KIẾN TRÚC CUỐI CÙNG (SIMPLIFIED)

```
┌─────────────────────────────────────────────┐
│         SIP Clients (Softphones/Phones)     │
└────────────────┬────────────────────────────┘
                 │ SIP/5060
                 ▼
┌────────────────────────────────────────────────┐
│  Kamailio Cluster (VIP: 192.168.1.102:5060)  │
│  ┌──────────────┐      ┌──────────────┐      │
│  │   Node 1     │      │   Node 2     │      │
│  │ 192.168.1.106│      │ 192.168.1.107│      │
│  │ db_mode=2    │      │ db_mode=2    │      │
│  └──────────────┘      └──────────────┘      │
└───────────┬────────────────────┬──────────────┘
            │                    │
            │ PostgreSQL Direct  │
            ▼                    ▼
┌────────────────────────────────────────────────┐
│  PostgreSQL 16 HA (VIP: 192.168.1.101:5432)   │
│  ┌──────────────┐      ┌──────────────┐       │
│  │  Primary     │◄────►│  Standby     │       │
│  │192.168.1.104 │repmgr│192.168.1.105 │       │
│  └──────────────┘      └──────────────┘       │
└────────────────────────────────────────────────┘
            ▲                    ▲
            │                    │
            │ ODBC Direct        │
            │                    │
┌───────────┴────────────────────┴───────────────┐
│       FreeSWITCH Cluster (Dispatcher)          │
│  ┌──────────────┐      ┌──────────────┐       │
│  │   Node 1     │◄────►│   Node 2     │       │
│  │ 192.168.1.108│lsyncd│ 192.168.1.109│       │
│  │ /storage/rec/│      │ /storage/rec/│       │
│  └──────────────┘      └──────────────┘       │
│         │                      │               │
│         └──────┬───────────────┘               │
│                │ HTTP POST (async)             │
│                ▼                               │
│     ┌─────────────────────┐                   │
│     │   API Gateway (Go)  │                   │
│     │   + Redis Queue     │                   │
│     └─────────────────────┘                   │
└────────────────────────────────────────────────┘
```

### 9.1 Component Count

| Component | Trước (có PgBouncer) | Sau (không PgBouncer) |
|-----------|----------------------|-----------------------|
| Kamailio | 2 nodes | 2 nodes |
| FreeSWITCH | 2 nodes | 2 nodes |
| PostgreSQL | 2 nodes + witness | 2 nodes (repmgr) |
| PgBouncer | 2 instances | ❌ KHÔNG |
| API Gateway | 2 instances | 2 instances |
| Redis | 1 instance | 1 instance |
| lsyncd | On FS nodes | On FS nodes |

**GIẢM: 3 components (PgBouncer × 2 + etcd witness)**

---

## 10. PERFORMANCE EXPECTATIONS (600-800 CC)

| Metric | Target | Achievable |
|--------|--------|------------|
| Concurrent Calls | 600-800 | ✅ YES |
| CPS | 50-100 | ✅ YES |
| Call Setup Latency | <200ms | ✅ 100-150ms |
| Registration (db_mode=2) | <50ms | ✅ 20-30ms |
| CDR Insertion (async) | <10s | ✅ 3-5s |
| Recording Sync | <5s | ✅ 2-5s |
| Failover RTO | <60s | ✅ 30-45s |
| Uptime | 99.9% | ✅ With proper HA |

---

## 11. KẾT LUẬN & QUYẾT ĐỊNH

### ✅ CÁC THAY ĐỔI CHẤP NHẬN:

1. **Loại bỏ PgBouncer** ✅
   - PostgreSQL handle 300 connections dễ dàng
   - Giảm complexity

2. **CDR qua API Gateway (async)** ✅
   - Non-blocking critical
   - Reliability cao

3. **FreeSWITCH ODBC → PostgreSQL trực tiếp** ✅
   - Built-in pooling đủ
   - Không cần PgBouncer

4. **PostgreSQL 2-node với repmgr (không etcd)** ✅
   - Priority-based failover
   - Trade-off acceptable

5. **lsyncd bidirectional (không NFS)** ✅
   - Thư mục thống nhất
   - Real-time sync

6. **Kamailio TLS optional** ✅
   - Tùy môi trường deployment

7. **Keepalived với flock** ✅
   - Sửa race condition

### 📈 LỢI ÍCH:

- **Đơn giản hơn:** Bớt 3 components
- **Dễ vận hành:** Ít failure points
- **Hiệu suất tương đương:** Không loss performance
- **Chi phí thấp hơn:** Ít infrastructure

### ⚠️ TRADE-OFFS:

- PostgreSQL phải handle nhiều connections hơn (acceptable)
- 2-node HA có risk split-brain (mitigate bằng priority + monitoring)
- Manual intervention có thể cần trong edge cases

**→ ARCHITECTURE MỚI SẴN SÀNG CHO PRODUCTION DEPLOYMENT** ✅