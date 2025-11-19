# Database Architecture - Local PostgreSQL Connection Strategy

**Ngày tạo:** 2025-01-18
**Phiên bản:** 1.0
**Mục đích:** Giải thích rõ ràng kiến trúc kết nối database trong hệ thống VoIP HA

---

## 📌 Nguyên Tắc Quan Trọng

### ❌ KHÔNG ĐÚNG (Common Misconception)
```
┌─────────┐      ┌─────────┐
│ Node 1  │      │ Node 2  │
│         │      │         │
│ Apps    │      │ Apps    │
└────┬────┘      └────┬────┘
     │                │
     └────────┬───────┘
              ↓
     ┌────────────────┐
     │ VIP            │
     │ 172.16.91.100  │
     └────────┬───────┘
              ↓
     ┌────────────────┐
     │ PostgreSQL ???  │
     └────────────────┘
```

**Tại sao KHÔNG đúng:**
- VIP chỉ dùng cho SIP traffic từ bên ngoài
- Database connection qua VIP tạo single point of failure
- Latency cao hơn (thêm 1 hop network)
- Phức tạp khi failover

---

### ✅ ĐÚNG (Correct Architecture)
```
┌──────────────────────┐         ┌──────────────────────┐
│ Node 1               │         │ Node 2               │
│                      │         │                      │
│ ┌────────────────┐   │         │ ┌────────────────┐   │
│ │ Kamailio       │   │         │ │ Kamailio       │   │
│ │ FreeSWITCH     │───┼──┐      │ │ FreeSWITCH     │───┼──┐
│ │ VoIP Admin     │   │  │      │ │ VoIP Admin     │   │  │
│ └────────┬───────┘   │  │      │ └────────┬───────┘   │  │
│          │           │  │      │          │           │  │
│          │ LOCAL     │  │      │          │ LOCAL     │  │
│          ↓           │  │      │          ↓           │  │
│ ┌────────────────┐   │  │      │ ┌────────────────┐   │  │
│ │ PostgreSQL 18  │   │  │      │ │ PostgreSQL 18  │   │  │
│ │ 172.16.91.101  │   │  │      │ │ 172.16.91.102  │   │  │
│ │ (MASTER)       │◄──┼──┼──────┼─│ (STANDBY)      │   │  │
│ └────────────────┘   │  │      │ └────────────────┘   │  │
│                      │  │      │                      │  │
└──────────────────────┘  │      └──────────────────────┘  │
                          │                                │
                          │  ┌──────────────────────────┐  │
                          └─►│ VIP: 172.16.91.100       │◄─┘
                             │ (SIP TRAFFIC ONLY)       │
                             └──────────────────────────┘
                                      ▲
                                      │
                             External SIP Phones
```

**Tại sao ĐÚNG:**
- ✅ Mỗi node kết nối LOCAL database (latency thấp nhất)
- ✅ No single point of failure cho database access
- ✅ Đơn giản hóa failover logic
- ✅ PostgreSQL streaming replication đồng bộ data
- ✅ VIP chỉ dùng cho external SIP traffic

---

## 🔌 Chi Tiết Kết Nối Database Per Component

### 1. Kamailio (SIP Proxy)

**File config:** `/etc/kamailio/kamailio.cfg`

**Node 1:**
```cfg
#!define DBURL "postgres://kamailio:PASSWORD@172.16.91.101/voipdb"
```

**Node 2:**
```cfg
#!define DBURL "postgres://kamailio:PASSWORD@172.16.91.102/voipdb"
```

**Lý do:**
- Kamailio thực hiện hàng trăm queries/giây (authentication, registration, location lookup)
- Kết nối LOCAL giảm latency từ ~2-5ms xuống <1ms
- Mỗi node có instance Kamailio riêng, kết nối database riêng

---

### 2. VoIP Admin (Go Service)

**File config:** `/etc/voip-admin/config.yaml`

**Node 1:**
```yaml
database:
  host: "172.16.91.101"
  port: 5432
  user: "voipadmin"
  dbname: "voipdb"
```

**Node 2:**
```yaml
database:
  host: "172.16.91.102"  # ← Khác với Node 1
  port: 5432
  user: "voipadmin"
  dbname: "voipdb"
```

**Lý do:**
- VoIP Admin xử lý XML_CURL requests từ FreeSWITCH
- Cache + LOCAL database cho directory lookups <5ms
- CDR processing ghi vào LOCAL database
- Background workers đọc từ LOCAL database

---

### 3. FreeSWITCH (Media Server)

**QUAN TRỌNG:** FreeSWITCH KHÔNG kết nối trực tiếp đến PostgreSQL!

**File config:**
- `/etc/freeswitch/autoload_configs/xml_curl.conf.xml`
- `/etc/freeswitch/autoload_configs/xml_cdr.conf.xml`

**Cả Node 1 và Node 2:**
```xml
<!-- Directory lookup -->
<param name="gateway-url" value="http://172.16.91.100:8080/freeswitch/directory"/>

<!-- CDR posting -->
<param name="url" value="http://172.16.91.100:8080/api/v1/cdr"/>
```

**Giải thích:**
- FreeSWITCH kết nối đến VoIP Admin qua **HTTP API** (VIA VIP)
- VoIP Admin sau đó kết nối đến LOCAL PostgreSQL
- Kiến trúc này ĐÚNG vì:
  - VIP đảm bảo FreeSWITCH luôn gọi được VoIP Admin (failover automatic)
  - VoIP Admin là HTTP service, không phải database
  - VoIP Admin tự quản lý connection đến LOCAL database

**Luồng dữ liệu:**
```
FreeSWITCH → HTTP (VIP) → VoIP Admin → LOCAL PostgreSQL
```

---

## 🔄 PostgreSQL Replication

### Streaming Replication Architecture

**Node 1 (MASTER):**
```
┌────────────────────────────────────┐
│ PostgreSQL 18 (MASTER)             │
│ IP: 172.16.91.101                  │
│                                    │
│ ┌────────────────────────────────┐ │
│ │ Database: voipdb               │ │
│ │ - Schema: voip (extensions,    │ │
│ │           cdr, queues, etc.)   │ │
│ │ - Schema: kamailio             │ │
│ │ - Schema: public               │ │
│ └────────────────────────────────┘ │
│                                    │
│ WAL Writer                         │
│   ↓                                │
│ wal_level = replica                │
│ max_wal_senders = 5                │
└────────┬───────────────────────────┘
         │
         │ Streaming Replication
         │ (async, physical slot)
         ↓
┌────────────────────────────────────┐
│ PostgreSQL 18 (STANDBY)            │
│ IP: 172.16.91.102                  │
│                                    │
│ WAL Receiver → WAL Replay          │
│                                    │
│ ┌────────────────────────────────┐ │
│ │ Database: voipdb (REPLICA)     │ │
│ │ - Identical schema             │ │
│ │ - Read-only mode               │ │
│ │ - Hot standby enabled          │ │
│ └────────────────────────────────┘ │
└────────────────────────────────────┘
```

### Replication Details

**Node 1 (MASTER) postgresql.conf:**
```ini
wal_level = replica
max_wal_senders = 5
max_replication_slots = 5
wal_keep_size = 1GB
archive_mode = on
```

**Node 2 (STANDBY) postgresql.auto.conf:**
```ini
primary_conninfo = 'host=172.16.91.101 port=5432 user=replicator ...'
primary_slot_name = 'node2_slot'
hot_standby = on
hot_standby_feedback = on
```

**pg_hba.conf (trên cả 2 nodes):**
```
host    replication     replicator      172.16.91.101/32        scram-sha-256
host    replication     replicator      172.16.91.102/32        scram-sha-256
```

---

## 📊 Benefits của Kiến Trúc LOCAL Database

### 1. Performance
- **Latency giảm 60-80%**
  - Remote DB via VIP: ~2-5ms
  - Local DB: <1ms
  - Quan trọng cho Kamailio (hundreds of queries/sec)

### 2. Reliability
- **No single point of failure**
  - Nếu VIP fail → External SIP traffic fail, nhưng database vẫn hoạt động
  - Mỗi node độc lập với database của mình

### 3. Simplified Failover
- **Node 1 down → Node 2 takes over:**
  1. Keepalived moves VIP to Node 2
  2. External SIP traffic → Node 2
  3. Node 2 apps ALREADY connected to LOCAL database (172.16.91.102)
  4. PostgreSQL promote từ STANDBY → MASTER (if configured)
  5. Zero database connection changes needed!

### 4. Network Efficiency
- **Giảm cross-node traffic**
  - Kamailio queries: 100-500 queries/sec × local = minimal network
  - FreeSWITCH XML_CURL: via VIP (necessary for failover)
  - CDR processing: local writes only
  - Replication: 1 connection stream (async)

---

## 🛠️ Deployment Checklist

### Configuration Files to Customize Per Node

| File | Node 1 Value | Node 2 Value | Method |
|------|--------------|--------------|--------|
| `/etc/kamailio/kamailio.cfg` | `172.16.91.101` | `172.16.91.102` | sed replacement |
| `/etc/voip-admin/config.yaml` | `172.16.91.101` | `172.16.91.102` | sed replacement |
| `/etc/freeswitch/autoload_configs/xml_curl.conf.xml` | `172.16.91.100` (VIP) | `172.16.91.100` (VIP) | Same (HTTP API) |
| `/etc/freeswitch/autoload_configs/xml_cdr.conf.xml` | `172.16.91.100` (VIP) | `172.16.91.100` (VIP) | Same (HTTP API) |

### Deployment Commands

**Node 1:**
```bash
# Kamailio - already correct (172.16.91.101 in template)
grep "DBURL" /etc/kamailio/kamailio.cfg

# VoIP Admin - already correct (172.16.91.101 in template)
grep "host:" /etc/voip-admin/config.yaml | head -2
```

**Node 2:**
```bash
# Kamailio - MUST change to 172.16.91.102
sudo sed -i 's/172.16.91.101/172.16.91.102/g' /etc/kamailio/kamailio.cfg

# VoIP Admin - MUST change to 172.16.91.102
sudo sed -i 's/host: "172.16.91.101"/host: "172.16.91.102"/' /etc/voip-admin/config.yaml
```

### Verification Commands

**Verify database connections on each node:**
```bash
# Check Kamailio config
grep DBURL /etc/kamailio/kamailio.cfg

# Check VoIP Admin config
grep "host:" /etc/voip-admin/config.yaml | head -2

# Test Kamailio database connection
PGPASSWORD='...' psql -h 127.0.0.1 -U kamailio -d voipdb -c "SELECT COUNT(*) FROM kamailio.subscriber;"

# Test VoIP Admin database connection
PGPASSWORD='...' psql -h 127.0.0.1 -U voipadmin -d voipdb -c "SELECT COUNT(*) FROM voip.extensions;"
```

**Verify replication:**
```bash
# On Node 1 (MASTER)
sudo -u postgres psql -c "SELECT * FROM pg_stat_replication;"

# On Node 2 (STANDBY)
sudo -u postgres psql -c "SELECT pg_is_in_recovery();"  # Should return 't'
```

---

## ❓ FAQ

### Q1: Tại sao không dùng VIP cho database như các hệ thống khác?

**A:** VIP thường dùng cho **Active-Passive database cluster** (ví dụ: PgPool, Patroni) nơi:
- VIP luôn point đến MASTER database
- Applications connect đến VIP
- Khi MASTER fail → VIP moves → applications reconnect

**Hệ thống của chúng ta khác:**
- Mỗi node là **self-contained unit** (apps + database cùng máy)
- Replication đồng bộ data, không phải load balancing
- Failover ở application level (Keepalived VIP), không phải database level

### Q2: Nếu Node 2 database là STANDBY (read-only), làm sao VoIP Admin ghi được CDR?

**A:** Đây là câu hỏi hay! Có 2 giải pháp:

**Option 1 (Recommended - Current Implementation):**
- Node 2 VoIP Admin GHI vào local database (172.16.91.102)
- Vì Node 2 đang STANDBY (read-only) → ghi sẽ FAIL
- VoIP Admin cần xử lý lỗi này và KHÔNG crash
- Khi Node 2 becomes MASTER (failover) → writes work again

**Option 2 (Alternative - Not Implemented):**
- Node 2 VoIP Admin detect STANDBY mode
- Redirect writes to MASTER (172.16.91.101) via replication user
- Phức tạp hơn, cần thêm logic

**Current approach:** Applications on STANDBY node accept read-only state. Failover triggers promotion.

### Q3: Nếu replication lag cao, Node 1 và Node 2 có data khác nhau?

**A:** Đúng, replication lag có thể tạo ra **stale data** trên STANDBY:
- Lag thường <100ms trong điều kiện bình thường
- Monitoring script cần check lag: `pg_wal_lsn_diff(pg_last_wal_receive_lsn(), pg_last_wal_replay_lsn())`
- Alert nếu lag > 10MB hoặc > 5 seconds
- Applications trên STANDBY node có thể đọc được stale data (acceptable cho read-only operations)

### Q4: Tại sao pg_hba.conf allow connections từ cả 2 nodes nếu chỉ connect LOCAL?

**A:** Defensive programming:
- Primary use case: localhost/local IP connections
- Backup scenario: admin cần query từ node kia (troubleshooting)
- Replication: Node 2 cần connect đến Node 1 via network
- Không có security issue vì firewall giới hạn 172.16.91.0/24

---

## 📚 References

- PostgreSQL Replication: https://www.postgresql.org/docs/18/warm-standby.html
- Streaming Replication: https://www.postgresql.org/docs/18/streaming-replication.html
- High Availability Best Practices: https://www.postgresql.org/docs/18/high-availability.html

---

**Document Owner:** VoIP HA Project Team
**Last Updated:** 2025-01-18
**Version:** 1.0
