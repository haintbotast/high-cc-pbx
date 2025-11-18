# Các Bản Sửa Lỗi Production Đã Áp Dụng

**Ngày:** 2025-01-18
**Phiên bản:** 1.0
**Trạng thái:** Critical fixes applied - Ready for testing

---

## Tóm Tắt Executive

Sau khi thực hiện comprehensive system review với vai trò các chuyên gia (PostgreSQL, Kamailio, FreeSWITCH, Go Developer, System Architect), đã phát hiện và sửa **3 lỗi CRITICAL** trong cấu hình hệ thống:

1. ✅ **Database name mismatch** trong Kamailio config
2. ✅ **Auth parameters** sai trong Kamailio auth_db module
3. ✅ **Missing xml_cdr.conf.xml** cho FreeSWITCH CDR posting

**⚠️ Lưu Ý:** Các bản sửa lỗi schema database (file `04-production-fixes.sql`) chưa được áp dụng và cần test trên staging trước.

---

## Chi Tiết Các Bản Sửa Lỗi

### 1. Kamailio Database Configuration (CRITICAL-10)

**File:** `configs/kamailio/kamailio.cfg`

**Vấn đề:**
- Database name trong Kamailio config là `kamailio`
- Nhưng database thực tế là `voipdb`
- Gây lỗi kết nối database khi Kamailio khởi động

**Sửa lỗi:**
```diff
- #!define DBURL "postgres://kamailio:PASSWORD@172.16.91.100/kamailio"
+ #!define DBURL "postgres://kamailio:PASSWORD@172.16.91.100/voipdb"
```

**Tác động:**
- Kamailio sẽ kết nối đúng database
- Tất cả modules sử dụng DBURL sẽ hoạt động (auth_db, usrloc, dispatcher, dialog, acc)

---

### 2. Kamailio Authentication Parameters (CRITICAL-11)

**File:** `configs/kamailio/kamailio.cfg`

**Vấn đề:**
- `calculate_ha1 = yes` - Kamailio sẽ tính HA1 từ plaintext password
- Nhưng database chỉ có HA1 hash, không có plaintext password
- `use_domain = 0` - Disable multi-domain support
- Thiếu `user_column`, `domain_column` mapping

**Sửa lỗi:**
```diff
  # ----- auth_db params -----
  modparam("auth_db", "db_url", DBURL)
- modparam("auth_db", "calculate_ha1", yes)
- modparam("auth_db", "password_column", "password")
- modparam("auth_db", "load_credentials", "")
- modparam("auth_db", "use_domain", 0)
+ modparam("auth_db", "calculate_ha1", 0)           # Use pre-calculated HA1
+ modparam("auth_db", "user_column", "username")
+ modparam("auth_db", "domain_column", "domain")
+ modparam("auth_db", "password_column", "ha1")     # Use HA1 hash
+ modparam("auth_db", "load_credentials", "ha1")
+ modparam("auth_db", "use_domain", 1)              # Multi-domain support
```

**Tác động:**
- Kamailio sẽ sử dụng HA1 hash từ view `kamailio.subscriber`
- Hỗ trợ xác thực multi-domain (nhiều khách hàng trên 1 hệ thống)
- SIP REGISTER authentication sẽ hoạt động đúng

**Chi tiết kỹ thuật:**
- View `kamailio.subscriber` join từ `voip.extensions` và `voip.domains`
- HA1 được tính tự động bởi trigger `voip.extensions_calc_ha1_trigger`
- Format: `MD5(username:domain:password)`

---

### 3. FreeSWITCH CDR Configuration (CRITICAL-12)

**File:** `configs/freeswitch/autoload_configs/xml_cdr.conf.xml` (MỚI TẠO)

**Vấn đề:**
- File config không tồn tại
- FreeSWITCH không thể POST CDR đến VoIP Admin
- CDR processing pipeline không hoạt động

**Nội dung file mới:**
```xml
<configuration name="xml_cdr.conf" description="XML CDR Configuration">
  <settings>
    <param name="url" value="http://172.16.91.100:8080/api/v1/cdr?uuid=${uuid}"/>
    <param name="cred" value="freeswitch:CHANGE_THIS_PASSWORD"/>
    <param name="timeout" value="5000"/>
    <param name="retries" value="2"/>
    <param name="delay" value="1000"/>
    <param name="encode" value="true"/>
    <param name="log-b-leg" value="false"/>
    <param name="prefix-a-leg" value="false"/>
    <param name="err-log-dir" value="/var/log/freeswitch"/>
    <param name="disable-on-error" value="false"/>
    <param name="enable-cdr" value="true"/>
  </settings>
</configuration>
```

**Tác động:**
- FreeSWITCH sẽ POST CDR sau mỗi cuộc gọi
- VoIP Admin nhận CDR và insert vào `voip.cdr_queue`
- Background worker xử lý queue và insert vào `voip.cdr` final table
- Hỗ trợ retry 2 lần nếu POST thất bại
- Không block voice threads (timeout 5s)

**CDR Flow:**
```
FreeSWITCH → POST XML → VoIP Admin /api/v1/cdr
                              ↓
                        INSERT voip.cdr_queue
                              ↓
                    Background Worker (5s interval)
                              ↓
                        Parse XML + Enrich
                              ↓
                        INSERT voip.cdr
```

---

### 4. FreeSWITCH XML_CURL Credentials (MINOR FIX)

**File:** `configs/freeswitch/autoload_configs/xml_curl.conf.xml`

**Thay đổi:**
- Đổi placeholder từ `API_KEY_HERE` → `CHANGE_THIS_PASSWORD`
- Nhất quán với format trong xml_cdr.conf.xml
- Dễ dàng tìm kiếm và replace khi deploy

**Locations changed (3 vị trí):**
- Line 11: Directory binding
- Line 20: Dialplan binding
- Line 29: Configuration binding

---

## Files Đã Thay Đổi

### Modified Files

1. **configs/kamailio/kamailio.cfg**
   - Line 19: Database name fix
   - Lines 149-156: Auth_db parameters fix
   - Commit ready: ✅

2. **configs/freeswitch/autoload_configs/xml_curl.conf.xml**
   - Lines 11, 20, 29: Password placeholder consistency
   - Commit ready: ✅

### New Files

3. **configs/freeswitch/autoload_configs/xml_cdr.conf.xml**
   - Completely new file
   - CDR POST configuration
   - Commit ready: ✅

### Prepared But Not Applied

4. **database/schemas/04-production-fixes.sql**
   - Schema alignment fixes
   - **⚠️ NOT APPLIED YET** - Requires staging testing first
   - Commit ready: ✅

---

## Các Bản Sửa Lỗi Database Schema (Chưa Áp Dụng)

**File:** `database/schemas/04-production-fixes.sql`

**⚠️ QUAN TRỌNG:** File này đã được tạo nhưng **CHƯA được chạy** trên database. Cần test trên staging environment trước khi áp dụng lên production.

### Nội dung chính:

#### Part 1: Fix voip.cdr_queue Schema
- DROP và recreate bảng với schema đúng
- Columns: id, uuid, xml_data, received_at, processed_at, retry_count, error_message
- Index: idx_cdr_queue_pending (WHERE processed_at IS NULL AND retry_count < 3)

#### Part 2: Fix voip.cdr Schema
- Thêm 15+ columns thiếu:
  - hangup_cause_q850 INT
  - sip_hangup_disposition VARCHAR(100)
  - call_type VARCHAR(50)
  - context VARCHAR(100)
  - read_codec, write_codec VARCHAR(50)
  - remote_media_ip VARCHAR(50)
  - RTP stats: rtp_audio_in_mos, packet_count, packet_loss, jitter
  - sip_from_user, sip_to_user, sip_call_id
  - user_agent VARCHAR(255)
  - record_file, record_duration
  - queue_wait_time, agent_extension
  - holdsec INT

- Rename: call_uuid → uuid (nếu tồn tại)
- Add constraints: chk_cdr_call_type, chk_cdr_direction

#### Part 3: Add Critical Missing Indexes
- **idx_extensions_auth_lookup** (MOST IMPORTANT!)
  ```sql
  CREATE INDEX idx_extensions_auth_lookup
  ON voip.extensions(extension, domain_id)
  INCLUDE (sip_ha1, sip_ha1b, display_name, vm_password, vm_email, max_concurrent, call_timeout)
  WHERE type = 'user' AND active = true;
  ```

- idx_cdr_caller_pattern, idx_cdr_dest_pattern
- idx_cdr_direction, idx_cdr_time_direction
- idx_queue_members_queue, idx_queue_members_user

#### Part 4-6: voip.extensions, voip.queues, voip.queue_agents
- Thêm missing columns
- Rename bảng queue_members → queue_agents
- Add check constraints

#### Part 7: Performance Functions
- cleanup_old_cdr_queue(days INT)
- get_extension_for_auth(extension, domain)

#### Part 8: Updated Triggers
- extensions_calc_ha1_trigger - Support new columns

#### Part 9: Verification
- Kiểm tra schema sau khi apply

### Tác động khi áp dụng:
- ✅ Go code sẽ không bị lỗi "column does not exist"
- ✅ Directory lookup <1ms với composite index
- ✅ CDR có đầy đủ thông tin (RTP stats, queue info, SIP details)
- ✅ Hỗ trợ queue management từ Go API

### Rủi ro:
- ⚠️ DROP TABLE voip.cdr_queue → Mất data hiện tại (nếu có)
- ⚠️ Schema change có thể ảnh hưởng application đang chạy
- ⚠️ Index creation có thể tốn thời gian trên bảng lớn

### Khuyến nghị:
```bash
# 1. Test trên staging
psql -U postgres -d voipdb_staging -f database/schemas/04-production-fixes.sql

# 2. Kiểm tra kết quả
psql -U postgres -d voipdb_staging -c "\d voip.cdr_queue"
psql -U postgres -d voipdb_staging -c "\d voip.cdr"

# 3. Test VoIP Admin với schema mới
cd voip-admin
./bin/voipadmind -config /etc/voip-admin/config_staging.yaml

# 4. Nếu OK, backup production và apply
pg_dump -U postgres voipdb > /backup/voipdb_before_fixes_$(date +%Y%m%d).sql
psql -U postgres -d voipdb -f database/schemas/04-production-fixes.sql
```

---

## Kiểm Tra Sau Khi Áp Dụng Fixes

### Test 1: Kamailio Database Connection

```bash
# Restart Kamailio
sudo systemctl restart kamailio

# Check log
sudo tail -f /var/log/kamailio.log | grep -E "postgres|ERROR"

# Kỳ vọng: Không có lỗi kết nối database
# Should see: "INFO: db_postgres ... connected to database"
```

### Test 2: Kamailio Authentication

```bash
# Từ SIP phone, đăng ký extension 1000
# Username: 1000
# Password: (password đã set trong database)
# Domain: example.com
# Server: 172.16.91.100:5060

# Check Kamailio log
sudo tail -f /var/log/kamailio.log | grep "REGISTER\|auth"

# Kỳ vọng:
# - "auth_check: ... credentials matched"
# - "200 OK" response
```

### Test 3: FreeSWITCH CDR Posting

```bash
# Thực hiện 1 cuộc gọi test
# Gọi từ 1000 → 1001 (hoặc bất kỳ extension nào)

# Check VoIP Admin log
journalctl -u voip-admin -f | grep CDR

# Kỳ vọng: "CDR received from FreeSWITCH, UUID: ..."

# Check database
psql -U postgres -d voipdb -c "SELECT COUNT(*) FROM voip.cdr_queue WHERE received_at > NOW() - INTERVAL '5 minutes';"

# Kỳ vọng: >= 1 row

# Đợi 10 giây (cho worker xử lý)
sleep 10

# Check final CDR table
psql -U postgres -d voipdb -c "SELECT uuid, caller_id_number, destination_number, duration FROM voip.cdr WHERE start_time > NOW() - INTERVAL '5 minutes';"

# Kỳ vọng: CDR đã được process và insert vào voip.cdr
```

### Test 4: FreeSWITCH Directory Lookup

```bash
# Test directory endpoint
curl -X POST http://172.16.91.100:8080/freeswitch/directory \
  -u "freeswitch:YOUR_PASSWORD" \
  -d "user=1000&domain=example.com"

# Kỳ vọng: XML response với thông tin extension

# Check cache performance (call 2 lần)
time curl -X POST http://172.16.91.100:8080/freeswitch/directory \
  -u "freeswitch:YOUR_PASSWORD" \
  -d "user=1000&domain=example.com"

# Lần 1: ~10-20ms (query DB)
# Lần 2: <5ms (from cache)
```

---

## Checklist Triển Khai

### Trước Khi Triển Khai

- [ ] Backup toàn bộ database
  ```bash
  pg_dump -U postgres voipdb > /backup/voipdb_$(date +%Y%m%d_%H%M%S).sql
  ```

- [ ] Backup config files
  ```bash
  tar -czf /backup/configs_$(date +%Y%m%d_%H%M%S).tar.gz \
    /etc/kamailio/ \
    /etc/freeswitch/ \
    /etc/voip-admin/
  ```

- [ ] Đọc kỹ deployment checklist
  - [ ] DEPLOYMENT-CHECKLIST.md
  - [ ] README.md section "Triển Khai"

### Áp Dụng Config Fixes

- [ ] Copy configs/kamailio/kamailio.cfg → /etc/kamailio/
- [ ] Replace PASSWORD placeholder với password thật
- [ ] Copy configs/freeswitch/autoload_configs/xml_cdr.conf.xml → /etc/freeswitch/autoload_configs/
- [ ] Copy configs/freeswitch/autoload_configs/xml_curl.conf.xml → /etc/freeswitch/autoload_configs/
- [ ] Replace CHANGE_THIS_PASSWORD với password thật (3 files)
- [ ] Verify config syntax
  ```bash
  kamailio -c -f /etc/kamailio/kamailio.cfg
  ```

### Restart Services

- [ ] Restart Kamailio
  ```bash
  sudo systemctl restart kamailio
  sudo systemctl status kamailio
  ```

- [ ] Reload FreeSWITCH modules
  ```bash
  fs_cli -x "reload mod_xml_curl"
  fs_cli -x "reload mod_xml_cdr"
  ```

- [ ] Restart VoIP Admin (nếu đang chạy)
  ```bash
  sudo systemctl restart voip-admin
  sudo systemctl status voip-admin
  ```

### Verify

- [ ] Run all tests in section "Kiểm Tra Sau Khi Áp Dụng Fixes"
- [ ] Check logs for errors
- [ ] Perform test call
- [ ] Verify CDR in database

### Nếu Database Fixes Được Test OK trên Staging

- [ ] Schedule maintenance window
- [ ] Announce downtime (nếu cần)
- [ ] Run 04-production-fixes.sql
- [ ] Verify schema changes
- [ ] Test VoIP Admin với schema mới
- [ ] Monitor for 24 hours

---

## Rollback Plan

Nếu gặp vấn đề sau khi apply fixes:

### Rollback Config

```bash
# Restore Kamailio config
sudo cp /backup/configs_TIMESTAMP/etc/kamailio/kamailio.cfg /etc/kamailio/
sudo systemctl restart kamailio

# Restore FreeSWITCH configs
sudo cp /backup/configs_TIMESTAMP/etc/freeswitch/autoload_configs/*.xml \
        /etc/freeswitch/autoload_configs/
fs_cli -x "reload mod_xml_curl"
fs_cli -x "reload mod_xml_cdr"
```

### Rollback Database (nếu đã chạy 04-production-fixes.sql)

```bash
# Stop all services
sudo systemctl stop kamailio freeswitch voip-admin

# Restore database
psql -U postgres -c "DROP DATABASE voipdb;"
psql -U postgres -c "CREATE DATABASE voipdb;"
psql -U postgres voipdb < /backup/voipdb_TIMESTAMP.sql

# Start services
sudo systemctl start postgresql kamailio freeswitch voip-admin
```

---

## Impact Assessment

### Hệ thống hiện tại (Trước khi fix)

❌ Kamailio không kết nối được database
❌ SIP authentication không hoạt động
❌ FreeSWITCH không gửi được CDR
❌ Directory lookup có thể chậm (thiếu index)
❌ Go code lỗi khi insert CDR (schema mismatch)

### Sau khi áp dụng config fixes

✅ Kamailio kết nối database thành công
✅ SIP REGISTER authentication hoạt động
✅ FreeSWITCH POST CDR đến VoIP Admin
✅ CDR được xử lý bất đồng bộ
⚠️ Vẫn cần apply database fixes để hoàn chỉnh

### Sau khi áp dụng cả database fixes

✅ Tất cả chức năng hoạt động đầy đủ
✅ Performance tối ưu với composite indexes
✅ CDR có đầy đủ thông tin (RTP, queue, SIP)
✅ Hỗ trợ 600-800 concurrent calls như thiết kế

---

## Contact & References

- **Main README:** [README.md](README.md)
- **Deployment Checklist:** [DEPLOYMENT-CHECKLIST.md](DEPLOYMENT-CHECKLIST.md)
- **Implementation Summary:** [.session-notes/voip-admin-implementation-complete.md](.session-notes/voip-admin-implementation-complete.md)
- **Comprehensive Review:** [.session-notes/comprehensive-system-review.md](.session-notes/comprehensive-system-review.md)
- **Database Fixes:** [database/schemas/04-production-fixes.sql](database/schemas/04-production-fixes.sql)

---

**Created:** 2025-01-18
**Version:** 1.0
**Status:** Configuration fixes applied, database fixes ready for testing
