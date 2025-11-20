# Hệ Thống VoIP Khả Dụng Cao (High-Availability)

**Hạ Tầng VoIP Production-Grade 2 Node**

- **Công suất**: 600-800 cuộc gọi đồng thời
- **PostgreSQL**: 18 (streaming replication)
- **Hệ điều hành**: Debian 12 (bookworm)
- **Kiến trúc**: Active-Passive với Keepalived
- **Trạng thái**: ✅ Hệ thống cấu hình sẵn sàng triển khai

---

## Bắt Đầu Triển Khai

### 📋 Chuẩn Bị Trước Khi Triển Khai

1. **Đọc Prerequisites** - [DEPLOYMENT-PREREQUISITES.md](DEPLOYMENT-PREREQUISITES.md)
   - Thông tin network (IPs, VIP, gateway, DNS)
   - Passwords cho tất cả services
   - Credentials cho database users
   - API keys và authentication tokens

2. **Review Architecture** - [DATABASE-ARCHITECTURE.md](DATABASE-ARCHITECTURE.md)
   - Hiểu chiến lược LOCAL database connection
   - Tại sao KHÔNG dùng VIP cho database
   - Replication setup

3. **Kamailio 6.0 Compatibility** - [KAMAILIO-6-COMPATIBILITY.md](KAMAILIO-6-COMPATIBILITY.md)
   - Breaking changes từ Kamailio 5.x
   - Required fixes và configurations

### 🚀 Triển Khai Thủ Công (Production-Ready)

Làm theo **[MANUAL-DEPLOYMENT-GUIDE.md](MANUAL-DEPLOYMENT-GUIDE.md)** - Hướng dẫn từng bước chi tiết cho:
1. PostgreSQL 18 installation và replication
2. Kamailio 6.0 setup và configuration
3. FreeSWITCH installation
4. VoIP Admin deployment
5. Keepalived HA configuration
6. Testing và verification

**Đặc điểm của manual deployment:**
- ✅ Kiểm soát hoàn toàn từng bước
- ✅ Hiểu rõ từng component
- ✅ Troubleshooting dễ dàng
- ✅ Production-tested configurations
- ✅ Idempotent scripts (chạy lại an toàn)

---

## Tổng Quan Kiến Trúc

```
      VIP: 172.16.91.100
             │
     ┌───────┴───────┐
     │               │
Node 1 (.101)   Node 2 (.102)
  MASTER          BACKUP

├── PostgreSQL 18   ├── PostgreSQL 18
├── Kamailio 6.0    ├── Kamailio 6.0
├── FreeSWITCH 1.10 ├── FreeSWITCH 1.10
├── voip-admin      ├── voip-admin
├── Keepalived      ├── Keepalived
└── lsyncd          └── lsyncd
```

### Tính Năng Chính
- **Production-Ready**: Configuration templates đã tested thực tế, manual deployment với hướng dẫn chi tiết
- **PostgreSQL 18**: Streaming replication với phát hiện failover tự động
- **Failover Chuẩn Production**: Dựa trên các mẫu PostgreSQL HA đã được kiểm chứng
  - Xác thực AH (an toàn hơn PASS)
  - Phát hiện split-brain và tự động phục hồi
  - Health check kiểm tra vai trò PostgreSQL (master/standby), không chỉ process
  - Failover nhận biết dịch vụ VoIP (thứ tự stop/start đúng)
- **Bảo Mật**: Mật khẩu nhập tương tác, API key tự động tạo

---

## Yêu Cầu Phần Cứng

Mỗi node (cho 600-800 cuộc gọi đồng thời):
- **CPU**: 16 cores
- **RAM**: 64 GB
- **Ổ cứng**: 500 GB SSD (database) + 3 TB HDD (ghi âm)
- **Mạng**: 1 Gbps

**Tổng**: 2 nodes = ~$7,000 chi phí phần cứng

---

## Ngăn Xếp Phần Mềm

| Thành Phần | Phiên Bản | Mục Đích |
|-----------|---------|---------|
| Debian | 12 (bookworm) | Hệ điều hành |
| PostgreSQL | **18** | Database với streaming replication |
| Kamailio | **6.0** | SIP proxy và load balancer |
| FreeSWITCH | 1.10 | Media server, IVR, voicemail |
| Keepalived | Latest | VIP failover (VRRP) |
| lsyncd | Latest | Đồng bộ file ghi âm |
| voip-admin | Tùy chỉnh (Go 1.23) | API gateway, quản lý |

---

## Cấu Trúc Dự Án

```
high-cc-pbx/
├── README.md                          ⭐ Bạn đang ở đây
│
├── scripts/
│   ├── setup/
│   │   ├── config_wizard.sh           ⭐ Bước 1: Chạy cái này trước
│   │   └── generate_configs.sh        ⭐ Bước 2: Chạy cái này sau
│   ├── monitoring/
│   │   └── check_voip_master.sh       Kiểm tra sức khỏe production
│   └── failover/
│       ├── keepalived_notify.sh       Xử lý failover thống nhất
│       └── safe_rebuild_standby.sh    Tự động rebuild standby
│
├── configs/                           Chỉ là template mẫu
│   ├── postgresql/                    (Dùng wizard để tạo config thật)
│   ├── keepalived/
│   ├── kamailio/
│   ├── freeswitch/
│   ├── lsyncd/
│   └── voip-admin/
│
├── generated-configs/                 ✅ Được tạo bởi generate_configs.sh
│   ├── node1/                         Config Node 1 của bạn (đã tùy chỉnh)
│   ├── node2/                         Config Node 2 của bạn (đã tùy chỉnh)
│   └── DEPLOY.md                      Hướng dẫn triển khai (với IP CỦA BẠN)
│
├── database/
│   └── schemas/
│       ├── 01-voip-schema.sql         Schema logic nghiệp vụ VoIP
│       └── 02-kamailio-schema.sql     Bảng SIP của Kamailio
│
└── voip-admin/                        Code Go service (khung sườn)
```

---

## Tại Sao Cấu Hình Tương Tác?

### Cách Cũ (Hardcode):
- ❌ IP hardcode thành 192.168.1.x hoặc 172.16.91.x trong git
- ❌ Phiên bản PostgreSQL sai (16 thay vì 18)
- ❌ FreeSWITCH bind vào VIP thay vì IP của node
- ❌ Mật khẩu là placeholder ("CHANGE_ME")
- ❌ Phải chỉnh sửa thủ công 20+ file
- ❌ Dễ bỏ sót file hoặc sai sót

### Cách Mới (Tương Tác):
- ✅ Wizard hỏi về mạng CỦA BẠN (bất kỳ dải IP nào)
- ✅ PostgreSQL 18 được cấu hình đúng
- ✅ FreeSWITCH nhận IP riêng của node tự động
- ✅ Mật khẩu nhập an toàn (không hiển thị)
- ✅ API key tự động tạo
- ✅ Config riêng cho từng node tự động tạo
- ✅ Không cần chỉnh sửa thủ công

---

## Ví Dụ: Config FreeSWITCH Riêng Cho Từng Node

Wizard tự động tạo file sofia.conf.xml **KHÁC NHAU** cho mỗi node:

**Node 1** nhận:
```xml
<param name="sip-ip" value="172.16.91.101"/>
<param name="rtp-ip" value="172.16.91.101"/>
```

**Node 2** nhận:
```xml
<param name="sip-ip" value="172.16.91.102"/>
<param name="rtp-ip" value="172.16.91.102"/>
```

❌ **KHÔNG PHẢI** VIP (172.16.91.100) - FreeSWITCH phải bind vào IP của node!

Điều này xảy ra tự động dựa trên input từ wizard. Không cần chỉnh sửa thủ công.

---

## Tính Năng Chuẩn Production

### Dựa Trên Cấu Hình PostgreSQL HA Của Bạn

Các script failover được mô phỏng theo cấu hình PostgreSQL HA production của bạn:

1. **Kiểm Tra Sức Khỏe** ([check_voip_master.sh](scripts/monitoring/check_voip_master.sh))
   - Kiểm tra **vai trò** PostgreSQL (master vs standby), không chỉ process
   - Xác minh khả năng ghi với temp table test
   - Kiểm tra tất cả dịch vụ VoIP (Kamailio, FreeSWITCH, voip-admin)
   - Exit code: 0 = master khỏe mạnh, 1 = không khỏe/standby

2. **Script Notify Thống Nhất** ([keepalived_notify.sh](scripts/failover/keepalived_notify.sh))
   - **Chuyển sang MASTER**: Promote PostgreSQL, tạo replication slot, start dịch vụ VoIP
   - **Chuyển sang BACKUP**: Phát hiện split-brain, kích hoạt auto-rebuild
   - **Trạng thái FAULT**: Ghi log chẩn đoán, gửi cảnh báo
   - Nhận biết dịch vụ VoIP: thứ tự stop/start đúng

3. **Rebuild An Toàn** ([safe_rebuild_standby.sh](scripts/failover/safe_rebuild_standby.sh))
   - Tự động phát hiện node (101 vs 102)
   - Kiểm tra master có thể truy cập
   - Stop dịch vụ VoIP theo thứ tự đúng
   - Rebuild standby với pg_basebackup
   - Tự động sửa cấu hình thiếu
   - Restart dịch vụ VoIP theo thứ tự đúng

---

## Quy Trình Triển Khai

### Giai Đoạn 1: Chuẩn Bị
1. Cài đặt Debian 12 trên cả hai node
2. Thiết lập mạng (gán IP, cấu hình interface)
3. Clone repository này

### Giai Đoạn 2: Cấu Hình
```bash
# Trên máy triển khai
cd high-cc-pbx
./scripts/setup/config_wizard.sh
# Trả lời các câu hỏi về môi trường của bạn
```

### Giai Đoạn 3: Tạo Config
```bash
./scripts/setup/generate_configs.sh
# Xem lại config đã tạo trong generated-configs/
```

### Giai Đoạn 4: Triển Khai
```bash
# Làm theo generated-configs/DEPLOY.md
# Nó chứa các lệnh chính xác cho môi trường của bạn như:
scp -r generated-configs/node1/* root@172.16.91.101:/tmp/voip-configs/
scp -r generated-configs/node2/* root@172.16.91.102:/tmp/voip-configs/
```

### Giai Đoạn 5: Thiết Lập Database
```bash
# Trên Node 1 (master)
psql -h 172.16.91.100 -U postgres -f database/schemas/01-voip-schema.sql
psql -h 172.16.91.100 -U postgres -f database/schemas/02-kamailio-schema.sql
```

### Giai Đoạn 6: Khởi Động Dịch Vụ
```bash
# Trên cả hai node
systemctl enable postgresql-18 kamailio freeswitch voip-admin keepalived lsyncd
systemctl start postgresql-18 kamailio freeswitch voip-admin lsyncd

# Start keepalived cuối cùng (sau khi tất cả dịch vụ đã khỏe)
systemctl start keepalived
```

### Giai Đoạn 7: Kiểm Tra
```bash
# Xác minh VIP
ip addr | grep 172.16.91.100

# Kiểm tra vai trò PostgreSQL
sudo -u postgres psql -c "SELECT pg_is_in_recovery();"

# Test health check
/usr/local/bin/check_voip_master.sh
echo $?  # Phải là 0 trên master

# Test failover
# Trên node master:
systemctl stop keepalived
# Xem log trên node backup - sẽ tự động promote
```

---

## Bảo Mật

### Mật Khẩu
- ✅ Nhập tương tác (không hiển thị)
- ✅ Xác nhận trước khi chấp nhận
- ✅ Lưu vào `/tmp/voip-ha-config.env` với chmod 600
- ✅ Không bao giờ commit vào git

### API Keys
- ✅ Tự động tạo bằng `openssl rand -base64 32`
- ✅ Duy nhất cho mỗi lần triển khai
- ✅ Nhúng trong config đã tạo

### Dọn Dẹp Sau Triển Khai
```bash
# Sau khi triển khai xong
rm -rf /tmp/voip-ha-config.env
rm -rf generated-configs/
# Config đã ở trên server, không cần bản local
```

---

## Xử Lý Sự Cố

### "Không tìm thấy file cấu hình"
```bash
$ ./scripts/setup/generate_configs.sh
ERROR: Configuration file not found: /tmp/voip-ha-config.env
```
**Giải pháp**: Chạy `./scripts/setup/config_wizard.sh` trước

### "VIP không chuyển"
Kiểm tra:
1. Keepalived chạy trên cả hai node: `systemctl status keepalived`
2. Gói VRRP không bị chặn: `tcpdump -i ens33 vrrp`
3. Script health check hoạt động: `/usr/local/bin/check_voip_master.sh`
4. Xem log: `tail -f /var/log/keepalived_voip_check.log`

### "PostgreSQL không promote"
Kiểm tra:
1. Script notify đã chạy: `grep keepalived_notify /var/log/syslog`
2. Vai trò PostgreSQL: `sudo -u postgres psql -c "SELECT pg_is_in_recovery();"`
3. Trạng thái replication: `sudo -u postgres psql -x -c "SELECT * FROM pg_stat_replication;"`

### "Phát hiện split-brain"
Hệ thống tự phục hồi:
1. Node backup phát hiện nó là standby nhưng PostgreSQL là master
2. Kích hoạt `safe_rebuild_standby.sh` tự động
3. Xem log: `tail -f /var/log/rebuild_standby.log`

---

## Mục Tiêu Hiệu Năng

| Chỉ Số | Mục Tiêu | Đo Lường |
|--------|--------|-------------|
| Cuộc gọi đồng thời | 600-800 | Số cuộc gọi active |
| Độ trễ thiết lập cuộc gọi | <200ms | SIP INVITE → 200 OK |
| Đăng ký | <50ms | REGISTER → 200 OK |
| Xử lý CDR | <30s | Hàng đợi async |
| RTO Failover | <45s | Master down → VIP chuyển |

---

## Các Bước Tiếp Theo

1. **Cấu hình**: Chạy [config_wizard.sh](scripts/setup/config_wizard.sh)
2. **Tạo config**: Chạy [generate_configs.sh](scripts/setup/generate_configs.sh)
3. **Triển khai**: Làm theo `generated-configs/DEPLOY.md`
4. **Kiểm tra**: Xác minh health check và failover
5. **Giám sát**: Thiết lập Prometheus/Grafana (tùy chọn)

---

## Tài Liệu

README này là nguồn sự thật duy nhất. Mọi thứ bạn cần biết đều ở đây.

### Tài Liệu Kỹ Thuật Chi Tiết:
- [DEPLOYMENT-PREREQUISITES.md](DEPLOYMENT-PREREQUISITES.md) - ⭐ Danh sách đầy đủ thông tin cần chuẩn bị trước khi triển khai
- [DATABASE-ARCHITECTURE.md](DATABASE-ARCHITECTURE.md) - Giải thích kiến trúc LOCAL database connection
- [KAMAILIO-6-COMPATIBILITY.md](KAMAILIO-6-COMPATIBILITY.md) - Phân tích tương thích Kamailio 6.0
- [DEPLOYMENT-CHECKLIST.md](DEPLOYMENT-CHECKLIST.md) - Checklist đánh dấu từng bước triển khai
- [MANUAL-DEPLOYMENT-GUIDE.md](MANUAL-DEPLOYMENT-GUIDE.md) - Hướng dẫn triển khai thủ công chi tiết

### Tài Nguyên Bổ Sung:
- [CHANGELOG.md](CHANGELOG.md) - Version history và release notes
- [claude.md](claude.md) - Context cho AI assistant (các vai trò chuyên môn)
- `configs/` - Configuration templates (customize theo môi trường của bạn)
- `database/schemas/` - PostgreSQL schemas (idempotent, an toàn chạy lại)

---

## Hỗ Trợ

- **Vấn đề cấu hình**: Kiểm tra [DEPLOYMENT-PREREQUISITES.md](DEPLOYMENT-PREREQUISITES.md)
- **Vấn đề triển khai**: Làm theo [MANUAL-DEPLOYMENT-GUIDE.md](MANUAL-DEPLOYMENT-GUIDE.md) từng bước
- **Vấn đề Kamailio**: Xem `/var/log/kamailio.log` và [KAMAILIO-6-COMPATIBILITY.md](KAMAILIO-6-COMPATIBILITY.md)
- **Vấn đề failover**: Xem log `/var/log/keepalived_voip_check.log`
- **Vấn đề PostgreSQL**: Xem `/var/log/postgresql/postgresql-18-main.log`
- **Database schemas**: Tất cả scripts idempotent - an toàn chạy lại nhiều lần

---

**Phiên bản**: 3.2.0 (Manual Deployment Ready)
**Trạng thái**: ✅ Production-Ready, Tested
**Ngày phát hành**: 2025-11-20
**Software Stack**: PostgreSQL 18, Kamailio 6.0, FreeSWITCH 1.10 (Debian 12)
**Changelog**: [CHANGELOG.md](CHANGELOG.md)
