# Deployment Prerequisites - Thông Tin Cần Chuẩn Bị

**Created:** 2025-01-19
**Purpose:** Danh sách đầy đủ thông tin cần chuẩn bị trước khi triển khai hệ thống VoIP HA

---

## 📋 Tổng Quan

Trước khi bắt đầu triển khai, bạn cần chuẩn bị các thông tin sau:
1. **Thông tin mạng** (IP addresses, network interface)
2. **Domain/Hostname** cho SIP routing
3. **Mật khẩu bảo mật** cho các dịch vụ
4. **Certificates** (nếu dùng TLS/SSL)

---

## 🌐 Thông Tin Mạng (Network Information)

### 1. Địa Chỉ IP

| Thông tin | Giá trị mặc định | Giá trị của bạn | Ghi chú |
|-----------|-----------------|-----------------|---------|
| **VIP (Virtual IP)** | `172.16.91.100` | _____________ | IP ảo do Keepalived quản lý |
| **Node 1 IP** | `172.16.91.101` | _____________ | IP thật của Node 1 |
| **Node 2 IP** | `172.16.91.102` | _____________ | IP thật của Node 2 |
| **Network Interface** | `ens33` | _____________ | Tên interface mạng (ens33, eth0, ens192, etc.) |
| **Network Subnet** | `172.16.91.0/24` | _____________ | Subnet của mạng triển khai |

**Lưu ý:**
- VIP phải cùng subnet với Node 1 và Node 2
- VIP không được đang được sử dụng bởi thiết bị khác
- Network interface phải giống nhau trên cả 2 node

### 2. Kiểm Tra Network Interface
```bash
# Liệt kê tất cả network interfaces
ip addr show

# Hoặc
ifconfig

# Tìm interface có IP chính của server
ip route | grep default
```

---

## 🔐 Domain/Hostname Configuration

### 1. SIP Domain (Kamailio Alias)

**Trong file:** `configs/kamailio/kamailio.cfg` (line 37)
```cfg
alias=voip.example.com
```

#### Ý Nghĩa:
- **Alias** là tên miền SIP mà Kamailio sẽ nhận diện làm "chính mình"
- Khi SIP client gửi request đến `sip:user@voip.example.com`, Kamailio sẽ xử lý thay vì forward
- Quan trọng cho **SIP routing** và **domain-based authentication**

#### Khi Nào Cần Cấu Hình:
✅ **CẦN thay đổi** nếu:
- Bạn có domain riêng (ví dụ: `pbx.mycompany.com`)
- SIP phones đăng ký với domain cụ thể (ví dụ: `sip:1001@pbx.mycompany.com`)
- Bạn muốn multi-domain SIP service

❌ **KHÔNG cần thay đổi** nếu:
- Bạn chỉ dùng IP addresses cho SIP (ví dụ: `sip:1001@172.16.91.100`)
- Đây là lab/testing environment
- Chưa có domain DNS setup

#### Cách Cấu Hình:

**Option 1: Sử dụng VIP làm alias (đơn giản nhất)**
```cfg
alias=172.16.91.100
```

**Option 2: Sử dụng domain name thật**
```cfg
alias=pbx.mycompany.com
alias=voip.mycompany.com
```
Lưu ý: Cần có DNS record pointing `pbx.mycompany.com` → VIP (`172.16.91.100`)

**Option 3: Multi-domain**
```cfg
alias=voip.example.com
alias=pbx.example.com
alias=172.16.91.100
```

#### Ảnh Hưởng Đến Triển Khai:

✅ **Không ảnh hưởng đến chức năng cốt lõi:**
- PostgreSQL replication vẫn hoạt động
- Keepalived failover vẫn hoạt động
- Load balancing vẫn hoạt động

⚠️ **Ảnh hưởng đến SIP routing:**
```
# Ví dụ SIP INVITE
INVITE sip:1001@voip.example.com SIP/2.0

# Kamailio check: alias == "voip.example.com"?
# Nếu match → xử lý local (tìm user 1001 trong database)
# Nếu không match → coi là external, forward đi
```

#### Troubleshooting:

**Vấn đề:** SIP phones không đăng ký được
```
SIP client: sip:1001@pbx.company.com
Kamailio alias: voip.example.com  ← không match!
```

**Giải pháp:**
```cfg
# Thêm vào kamailio.cfg
alias=pbx.company.com
```

**Hoặc:**
```cfg
# Nếu muốn accept tất cả domains, comment alias:
# alias=voip.example.com
```

---

## 🔒 Mật Khẩu Bảo Mật (Security Credentials)

### 1. PostgreSQL Passwords

| Account | Purpose | Config File | Yêu cầu |
|---------|---------|-------------|---------|
| `postgres` (OS user) | PostgreSQL admin | System | Peer auth (không cần password) |
| `replicator` | Streaming replication | `postgresql.conf`, `pg_hba.conf` | Min 16 chars, SCRAM-SHA-256 |
| `kamailio` | Kamailio read-write user | `kamailio.cfg`, `kamctlrc` | Min 16 chars, MD5 |
| `kamailioro` | Kamailio read-only user | `kamctlrc` (for kamctl) | Min 16 chars, MD5 |
| `voipadmin` | VoIP Admin database user | `config.yaml` | Min 16 chars, SCRAM-SHA-256 |
| `freeswitch` | FreeSWITCH ODBC user | `odbc.ini` | Min 16 chars, MD5 |

**Tạo mật khẩu mạnh:**
```bash
# Tạo random password 32 chars
openssl rand -base64 32

# Hoặc dùng pwgen
pwgen -s 32 1
```

### 2. Application Passwords/Keys

| Service | Purpose | Config File | Format |
|---------|---------|-------------|--------|
| **FreeSWITCH → VoIP Admin** | XML_CURL auth | `xml_curl.conf.xml`, `config.yaml` | Basic Auth |
| **Admin API Key** | REST API access | `config.yaml` | Random string 64+ chars |
| **UAC Restore Password** | Kamailio UAC module | `kamailio.cfg` (line 207) | Any string |

**Example credentials trong các file:**

#### File: `configs/freeswitch/autoload_configs/xml_curl.conf.xml`
```xml
<param name="gateway-credentials" value="freeswitch:CHANGE_THIS_PASSWORD"/>
```

#### File: `configs/voip-admin/config.yaml`
```yaml
auth:
  freeswitch_user: "freeswitch"
  freeswitch_password: "CHANGE_ME_FREESWITCH_PASSWORD"

  api_keys:
    - "CHANGE_ME_ADMIN_API_KEY_1"
```

**⚠️ Lưu ý quan trọng:**
- `freeswitch_password` trong `xml_curl.conf.xml` PHẢI GIỐNG với `freeswitch_password` trong `config.yaml`
- Mật khẩu này dùng cho HTTP Basic Authentication

### 3. Keepalived VRRP Authentication

**File:** `configs/keepalived/keepalived.conf`
```
authentication {
    auth_type AH
    auth_pass CHANGE_THIS_VRRP_PASSWORD
}
```

**Yêu cầu:**
- Max 8 ký tự (giới hạn của VRRP protocol)
- Phải giống nhau trên cả 2 nodes
- Dùng `AH` (Authentication Header) cho bảo mật tốt hơn

---

## 📦 Thông Tin Cần Chuẩn Bị Cho Từng Application

### 1. PostgreSQL 18

**Cần chuẩn bị:**
- ✅ Mật khẩu `replicator` user (cho streaming replication)
- ✅ Mật khẩu `kamailio` user
- ✅ Mật khẩu `voipadmin` user
- ✅ Mật khẩu `freeswitch` user (nếu dùng ODBC)
- ✅ Xác định node nào là MASTER ban đầu (thường là Node 1)

**Đã có trong tài liệu:**
- ✅ DATABASE-ARCHITECTURE.md - Giải thích kiến trúc kết nối LOCAL
- ✅ MANUAL-DEPLOYMENT-GUIDE.md Section 7 - Hướng dẫn setup replication
- ✅ `configs/postgresql/` - Tất cả file config mẫu

**Lưu ý đặc biệt:**
```
❗ Mỗi node PHẢI kết nối đến LOCAL PostgreSQL:
   Node 1: 172.16.91.101
   Node 2: 172.16.91.102

❗ KHÔNG dùng VIP cho database connections!
```

---

### 2. Kamailio 6.0

**Cần chuẩn bị:**
- ✅ SIP domain/alias (mặc định: `voip.example.com`)
- ✅ Database password trong `DBURL` (line 22)
- ✅ Listen addresses - PHẢI customize per node:
  - Node 1: VIP + 172.16.91.101
  - Node 2: VIP + 172.16.91.102

**Đã có trong tài liệu:**
- ✅ KAMAILIO-6-COMPATIBILITY.md - Tương thích Kamailio 6.0
- ✅ MANUAL-DEPLOYMENT-GUIDE.md Section 8 - Cài đặt và cấu hình
- ✅ `configs/kamailio/kamailio.cfg` - Config hoàn chỉnh

**Tham số quan trọng cần review:**

| Parameter | Line | Giá trị mặc định | Cần thay đổi? |
|-----------|------|-----------------|---------------|
| `DBURL` | 22 | `postgres://kamailio:PASSWORD@172.16.91.101/voipdb` | ✅ Thay PASSWORD và IP per node |
| `alias` | 37 | `voip.example.com` | ⚠️ Tùy môi trường |
| `listen` | 31-34 | VIP + Node IP | ✅ Customize IP per node |
| Memory (shm/pkg) | `/etc/default/kamailio` | 512MB/16MB | ⚠️ Tùy RAM server |

**Kiểm tra syntax:**
```bash
kamailio -c -f /etc/kamailio/kamailio.cfg
```

---

### 3. FreeSWITCH 1.10

**Cần chuẩn bị:**
- ✅ VIP address cho XML_CURL (kết nối đến VoIP Admin qua VIP)
- ✅ Basic Auth credentials (username: freeswitch, password: ...)
- ✅ External SIP IP (nếu có trunk provider)
- ✅ Codec preferences (G711, G729, etc.)

**Đã có trong tài liệu:**
- ✅ MANUAL-DEPLOYMENT-GUIDE.md Section 9 - Cài đặt FreeSWITCH
- ✅ `configs/freeswitch/` - Tất cả config files

**Tham số quan trọng:**

#### File: `xml_curl.conf.xml`
```xml
<!-- Directory lookup -->
<param name="gateway-url" value="http://172.16.91.100:8080/freeswitch/directory"/>
<param name="gateway-credentials" value="freeswitch:CHANGE_THIS_PASSWORD"/>
```

**⚠️ Lưu ý:**
- FreeSWITCH kết nối đến VoIP Admin **QUA VIP** (http://172.16.91.100:8080)
- KHÔNG customize per node (cùng config cho cả 2 nodes)
- Keepalived tự động failover VIP, FreeSWITCH không cần biết node nào đang active

---

### 4. VoIP Admin (Go Service)

**Cần chuẩn bị:**
- ✅ Database host - PHẢI customize per node:
  - Node 1: `172.16.91.101`
  - Node 2: `172.16.91.102`
- ✅ Database password
- ✅ FreeSWITCH Basic Auth password (GIỐNG với xml_curl.conf.xml)
- ✅ Admin API keys (generate random 64+ chars)

**Đã có trong tài liệu:**
- ✅ DATABASE-ARCHITECTURE.md - Giải thích LOCAL database connection
- ✅ MANUAL-DEPLOYMENT-GUIDE.md Section 10 - Build và deploy VoIP Admin
- ✅ `configs/voip-admin/config.yaml` - Config mẫu đầy đủ

**Tham số quan trọng:**

```yaml
database:
  host: "172.16.91.101"      # Node 1
  # host: "172.16.91.102"    # Node 2 - PHẢI THAY ĐỔI!
  password: "CHANGE_ME_STRONG_PASSWORD"

auth:
  freeswitch_password: "CHANGE_ME_FREESWITCH_PASSWORD"  # GIỐNG xml_curl
  api_keys:
    - "CHANGE_ME_ADMIN_API_KEY_1"  # Generate: openssl rand -hex 32
```

**Testing endpoints:**
```bash
# Health check
curl http://172.16.91.100:8080/health

# Stats (requires API key)
curl -H "X-API-Key: YOUR_KEY" http://172.16.91.100:8080/health/stats
```

---

### 5. Keepalived (VRRP Failover)

**Cần chuẩn bị:**
- ✅ VIP address
- ✅ Network interface name
- ✅ VRRP password (max 8 chars)
- ✅ VRRP Router ID (unique, ví dụ: 51)
- ✅ Priority (Node 1: 150, Node 2: 100)

**Đã có trong tài liệu:**
- ✅ MANUAL-DEPLOYMENT-GUIDE.md Section 5 - Cài đặt Keepalived
- ✅ `configs/keepalived/` - Config cho cả 2 nodes

**Tham số quan trọng:**

| Parameter | Node 1 | Node 2 | Ghi chú |
|-----------|--------|--------|---------|
| `state` | MASTER | BACKUP | Initial state |
| `priority` | 150 | 100 | Node 1 cao hơn |
| `virtual_router_id` | 51 | 51 | Phải giống nhau |
| `auth_pass` | SAME_PASSWORD | SAME_PASSWORD | Max 8 chars |
| `virtual_ipaddress` | VIP | VIP | Phải giống nhau |

**⚠️ Kernel parameter bắt buộc:**
```bash
# /etc/sysctl.conf
net.ipv4.ip_nonlocal_bind = 1
```

**Lý do:** Cho phép Kamailio/FreeSWITCH listen trên VIP trước khi Keepalived assign VIP

---

## 📝 Checklist Trước Khi Triển Khai

### Giai Đoạn 1: Chuẩn Bị Thông Tin

- [ ] Xác định IP addresses (VIP, Node 1, Node 2)
- [ ] Xác định network interface name (ens33, eth0, etc.)
- [ ] Chọn SIP domain/alias (hoặc dùng IP)
- [ ] Tạo tất cả passwords (PostgreSQL, services, VRRP)
- [ ] Generate API keys cho VoIP Admin

### Giai Đoạn 2: Review Configurations

- [ ] **Kamailio:**
  - [ ] Thay `PASSWORD` trong `DBURL` (line 22)
  - [ ] Customize `listen` addresses per node (lines 31-34)
  - [ ] Review `alias` (line 37) - đổi hoặc giữ nguyên
  - [ ] Check `/etc/default/kamailio` memory settings

- [ ] **VoIP Admin:**
  - [ ] Customize `database.host` per node (Node 1: .101, Node 2: .102)
  - [ ] Thay `database.password`
  - [ ] Thay `auth.freeswitch_password` (GIỐNG với xml_curl)
  - [ ] Thay `auth.api_keys`

- [ ] **FreeSWITCH:**
  - [ ] Thay `gateway-credentials` trong `xml_curl.conf.xml` (GIỐNG với config.yaml)
  - [ ] Review codec settings trong `vars.xml`

- [ ] **PostgreSQL:**
  - [ ] Chuẩn bị passwords cho: replicator, kamailio, voipadmin, freeswitch
  - [ ] Xác định node nào làm MASTER ban đầu

- [ ] **Keepalived:**
  - [ ] Customize VIP, interface, auth_pass
  - [ ] Node 1: priority 150, state MASTER
  - [ ] Node 2: priority 100, state BACKUP

### Giai Đoạn 3: Kiểm Tra Hệ Thống

- [ ] Cả 2 nodes có Debian 12 (bookworm)
- [ ] Cả 2 nodes có hardware đủ (16 cores, 64GB RAM)
- [ ] Network connectivity giữa 2 nodes (ping test)
- [ ] VIP chưa được sử dụng bởi thiết bị khác
- [ ] Firewall cho phép traffic:
  - [ ] PostgreSQL: 5432
  - [ ] Kamailio: 5060 UDP/TCP
  - [ ] FreeSWITCH: 5080, 8021, 16384-32768 (RTP)
  - [ ] VoIP Admin: 8080
  - [ ] Keepalived: VRRP (protocol 112)

---

## 🎯 Quick Reference: Các File Cần Thay Đổi Per Node

| File | Node 1 Value | Node 2 Value | Parameter |
|------|--------------|--------------|-----------|
| `kamailio.cfg` | 172.16.91.101 | 172.16.91.102 | `DBURL`, `listen` (lines 22, 33-34) |
| `config.yaml` | 172.16.91.101 | 172.16.91.102 | `database.host` (line 18) |
| `keepalived.conf` | MASTER, 150 | BACKUP, 100 | `state`, `priority` |

**Các file KHÔNG thay đổi giữa các nodes:**
- `xml_curl.conf.xml` (FreeSWITCH) - dùng VIP
- `pg_hba.conf` (PostgreSQL) - allow cả 2 IPs
- Tất cả schema files trong `database/schema/`

---

## 📚 Tài Liệu Liên Quan

| Tài liệu | Mục đích |
|----------|---------|
| [README.md](README.md) | Tổng quan dự án |
| [DATABASE-ARCHITECTURE.md](DATABASE-ARCHITECTURE.md) | Giải thích LOCAL database strategy |
| [KAMAILIO-6-COMPATIBILITY.md](KAMAILIO-6-COMPATIBILITY.md) | Tương thích Kamailio 6.0 |
| [MANUAL-DEPLOYMENT-GUIDE.md](MANUAL-DEPLOYMENT-GUIDE.md) | Hướng dẫn triển khai chi tiết từng bước |
| [DEPLOYMENT-CHECKLIST.md](DEPLOYMENT-CHECKLIST.md) | Checklist đánh dấu tiến độ triển khai |

---

## ❓ FAQ

### Q1: Tôi có thể bỏ qua việc đổi alias không?

**A:** Có, nếu:
- Bạn chỉ dùng IP addresses cho SIP endpoints
- Đây là môi trường lab/test
- Chưa có domain DNS

Bạn có thể:
1. Giữ nguyên `alias=voip.example.com` (không ảnh hưởng nếu không dùng)
2. Hoặc đổi thành `alias=172.16.91.100` (rõ ràng hơn)

### Q2: Password nào quan trọng nhất cần thay đổi?

**A:** Theo thứ tự ưu tiên:
1. **PostgreSQL `replicator`** - nếu lộ, attacker có thể replicate database
2. **VoIP Admin API keys** - nếu lộ, attacker có full control
3. **PostgreSQL application users** (kamailio, voipadmin) - nếu lộ, data breach
4. **FreeSWITCH auth** - nếu lộ, free calls/toll fraud
5. **Keepalived VRRP** - nếu lộ, rogue VRRP packets

### Q3: Tôi có thể dùng cùng password cho nhiều services không?

**A:** ❌ KHÔNG NÊN vì:
- Nếu 1 service bị compromise → tất cả services bị ảnh hưởng
- Best practice: mỗi service 1 password riêng
- Dùng password manager để quản lý

### Q4: Tôi đã deploy rồi, có thể đổi password sau không?

**A:** Có, nhưng phức tạp:
- PostgreSQL: `ALTER USER ... PASSWORD '...'` + restart applications
- VoIP Admin: Đổi trong config.yaml + restart service
- Kamailio: Đổi trong kamailio.cfg + `kamctl fifo reload`

**Đơn giản hơn:** Chuẩn bị đúng passwords ngay từ đầu!

---

**Maintained by:** VoIP HA Project Team
**Last updated:** 2025-01-19
**Version:** 1.0
