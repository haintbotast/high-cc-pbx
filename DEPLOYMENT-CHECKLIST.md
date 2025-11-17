# Danh Sách Kiểm Tra Triển Khai VoIP HA

Hướng dẫn tham khảo nhanh để triển khai hệ thống VoIP HA.

**Để biết chi tiết đầy đủ, xem [README.md](README.md)**

---

## Trước Khi Triển Khai

### Chuẩn Bị Phần Cứng
- [ ] 2 server đã cài đặt Debian 12
- [ ] Mỗi server: 16 cores, 64 GB RAM, 500 GB SSD + 3 TB HDD
- [ ] Mạng đã cấu hình (IP đã gán, interface đã up)
- [ ] Các server có thể ping nhau
- [ ] Git repository đã clone trên máy triển khai

### Lập Kế Hoạch Mạng
- [ ] Quyết định địa chỉ IP:
  - IP Node 1: ____________
  - IP Node 2: ____________
  - VIP: ____________
- [ ] Biết tên giao diện mạng (ví dụ: ens33, eth0)
- [ ] Quy tắc firewall đã chuẩn bị (nếu cần)

---

## Cấu Hình (Bước 1-2)

### Bước 1: Chạy Config Wizard
```bash
cd high-cc-pbx
./scripts/setup/config_wizard.sh
```

**Wizard sẽ hỏi:**
- [ ] IP của các node và VIP
- [ ] Giao diện mạng
- [ ] Tên hostname
- [ ] Mật khẩu PostgreSQL (replication, kamailio, voipadmin, freeswitch)
- [ ] Cài đặt Keepalived VRRP
- [ ] Cổng và mật khẩu FreeSWITCH
- [ ] Cổng VoIP Admin

**Kết quả**: `/tmp/voip-ha-config.env`

### Bước 2: Tạo Config
```bash
./scripts/setup/generate_configs.sh
```

**Kết quả**: Thư mục `generated-configs/` với:
- [ ] Config cho `node1/`
- [ ] Config cho `node2/`
- [ ] `DEPLOY.md` với hướng dẫn triển khai

**Xem lại config đã tạo trước khi tiếp tục**

---

## Triển Khai (Bước 3-6)

### Bước 3: Copy Config Lên Các Node
```bash
# Làm theo lệnh chính xác trong generated-configs/DEPLOY.md
scp -r generated-configs/node1/* root@<NODE1_IP>:/tmp/voip-configs/
scp -r generated-configs/node2/* root@<NODE2_IP>:/tmp/voip-configs/
```

### Bước 4: Cài Đặt Gói Phần Mềm (trên cả hai node)
```bash
# PostgreSQL 18
apt install -y postgresql-18 postgresql-contrib-18

# Kamailio
apt install -y kamailio kamailio-postgres-modules

# FreeSWITCH
apt install -y freeswitch freeswitch-mod-commands freeswitch-mod-sofia

# Keepalived
apt install -y keepalived

# lsyncd
apt install -y lsyncd rsync
```

### Bước 5: Áp Dụng Config (trên cả hai node)
```bash
# Copy config vào thư mục hệ thống
# Làm theo đường dẫn chính xác trong generated-configs/DEPLOY.md

# Ví dụ (xác minh đường dẫn):
cp /tmp/voip-configs/keepalived/keepalived.conf /etc/keepalived/
cp /tmp/voip-configs/postgresql/pg_hba.conf /etc/postgresql/18/main/
cp /tmp/voip-configs/freeswitch/sofia.conf.xml /etc/freeswitch/autoload_configs/
cp /tmp/voip-configs/voip-admin/config.yaml /etc/voip-admin/
cp /tmp/voip-configs/scripts/* /usr/local/bin/
chmod +x /usr/local/bin/*.sh
```

### Bước 6: Thiết Lập Database (chỉ Node 1)
```bash
# Trên Node 1
sudo -u postgres createuser -s replicator
sudo -u postgres psql -c "ALTER USER replicator WITH PASSWORD '<MẬT_KHẨU_REPL_CỦA_BẠN>';"

# Tạo database
sudo -u postgres createdb voip
sudo -u postgres createdb kamailio

# Áp dụng schema
sudo -u postgres psql -d voip -f /path/to/01-voip-schema.sql
sudo -u postgres psql -d kamailio -f /path/to/02-kamailio-schema.sql

# Tạo user ứng dụng
sudo -u postgres psql <<EOF
CREATE USER kamailio WITH PASSWORD '<MẬT_KHẨU_KAMAILIO_CỦA_BẠN>';
CREATE USER voipadmin WITH PASSWORD '<MẬT_KHẨU_VOIPADMIN_CỦA_BẠN>';
CREATE USER freeswitch WITH PASSWORD '<MẬT_KHẨU_FREESWITCH_CỦA_BẠN>';
GRANT ALL ON DATABASE kamailio TO kamailio;
GRANT ALL ON DATABASE voip TO voipadmin;
GRANT ALL ON DATABASE voip TO freeswitch;
EOF
```

---

## Khởi Động Dịch Vụ (Bước 7-8)

### Bước 7: Cấu Hình Replication (Node 2)
```bash
# Trên Node 2, với user postgres
sudo -u postgres pg_basebackup -h <NODE1_IP> -U replicator -D /var/lib/postgresql/18/main -Fp -Xs -P -R

# Xác minh file standby.signal tồn tại
ls -la /var/lib/postgresql/18/main/standby.signal

# Khởi động PostgreSQL trên Node 2
systemctl start postgresql-18
systemctl enable postgresql-18
```

### Bước 8: Khởi Động Dịch Vụ (Cả Hai Node)
```bash
# Trên cả hai node

# PostgreSQL (đã start rồi)
systemctl enable postgresql-18

# Kamailio
systemctl enable kamailio
systemctl start kamailio

# FreeSWITCH
systemctl enable freeswitch
systemctl start freeswitch

# VoIP Admin
systemctl enable voip-admin
systemctl start voip-admin

# lsyncd
systemctl enable lsyncd
systemctl start lsyncd

# Keepalived (KHỞI ĐỘNG CUỐI CÙNG!)
systemctl enable keepalived
systemctl start keepalived
```

---

## Xác Minh (Bước 9)

### Kiểm Tra VIP
```bash
# Trên Node 1 (phải có VIP)
ip addr | grep <VIP>

# Trên Node 2 (KHÔNG có VIP)
ip addr | grep <VIP>
```

### Kiểm Tra Vai Trò PostgreSQL
```bash
# Trên Node 1 (phải là false = master)
sudo -u postgres psql -c "SELECT pg_is_in_recovery();"

# Trên Node 2 (phải là true = standby)
sudo -u postgres psql -c "SELECT pg_is_in_recovery();"
```

### Kiểm Tra Replication
```bash
# Trên Node 1 (phải hiển thị Node 2 đã kết nối)
sudo -u postgres psql -x -c "SELECT * FROM pg_stat_replication;"
```

### Kiểm Tra Script Health
```bash
# Trên Node 1 (phải trả về 0)
/usr/local/bin/check_voip_master.sh
echo $?

# Trên Node 2 (phải trả về 1)
/usr/local/bin/check_voip_master.sh
echo $?
```

### Kiểm Tra Trạng Thái Dịch Vụ
```bash
# Trên cả hai node
systemctl status postgresql-18 kamailio freeswitch voip-admin keepalived lsyncd
```

---

## Kiểm Tra Failover (Bước 10)

### Test 1: Failover Nhẹ Nhàng
```bash
# Trên Node 1 (master)
systemctl stop keepalived

# Đợi 30-45 giây

# Trên Node 2, kiểm tra:
# - VIP đã chuyển
ip addr | grep <VIP>

# - PostgreSQL đã promote
sudo -u postgres psql -c "SELECT pg_is_in_recovery();"  # Phải là false

# - Dịch vụ đang chạy
systemctl status kamailio freeswitch voip-admin
```

### Test 2: Failback
```bash
# Trên Node 1, start keepalived lại
systemctl start keepalived

# Node 1 phải phát hiện split-brain và tự động rebuild thành standby
tail -f /var/log/rebuild_standby.log

# Sau khi rebuild hoàn tất:
# - Node 1 phải là standby
sudo -u postgres psql -c "SELECT pg_is_in_recovery();"  # Phải là true

# - Node 2 vẫn là master với VIP
```

### Test 3: Lỗi Dịch Vụ
```bash
# Trên node master, stop PostgreSQL
systemctl stop postgresql-18

# Health check phải fail, kích hoạt failover
tail -f /var/log/keepalived_voip_check.log
```

---

## Dọn Dẹp Sau Triển Khai

### Trên Máy Triển Khai
```bash
# Xóa file config nhạy cảm
rm -rf /tmp/voip-ha-config.env

# Xóa hoặc lưu trữ config đã tạo
rm -rf generated-configs/
# HOẶC
mv generated-configs/ ~/backups/voip-configs-$(date +%Y%m%d)/
```

### Trên Các Node
```bash
# Xóa config tạm
rm -rf /tmp/voip-configs/
```

---

## Thiết Lập Giám Sát (Tùy Chọn)

### Kiểm Tra Log
```bash
# Health check của Keepalived
tail -f /var/log/keepalived_voip_check.log

# Chuyển trạng thái Keepalived
grep keepalived /var/log/syslog

# PostgreSQL
tail -f /var/log/postgresql/postgresql-18-main.log

# Kamailio
journalctl -u kamailio -f

# FreeSWITCH
tail -f /usr/local/freeswitch/log/freeswitch.log
```

### Thiết Lập Giám Sát (Tùy Chọn)
- [ ] Prometheus + Grafana
- [ ] Logging tập trung (ELK, Loki)
- [ ] Cảnh báo (email, Slack, PagerDuty)

---

## Tham Khảo Nhanh Xử Lý Sự Cố

| Vấn Đề | Kiểm Tra | Giải Pháp |
|-------|-------|----------|
| VIP không chuyển | `systemctl status keepalived` | Kiểm tra config VRRP, firewall |
| PostgreSQL không replicate | `pg_stat_replication` | Kiểm tra pg_hba.conf, user replication |
| Health check fail | `/usr/local/bin/check_voip_master.sh` | Kiểm tra quyền script, trạng thái dịch vụ |
| Split-brain | Xem log trong `/var/log/rebuild_standby.log` | Tự động phục hồi qua safe_rebuild |
| Dịch vụ không start | `systemctl status <service>` | Xem log, kiểm tra cú pháp config |

**Để xử lý sự cố chi tiết, xem [README.md](README.md#xử-lý-sự-cố)**

---

## Tiêu Chí Thành Công

✅ **Triển khai thành công khi:**
- [ ] VIP phản hồi trên Node 1
- [ ] Replication PostgreSQL active (Node 1 → Node 2)
- [ ] Tất cả dịch vụ đang chạy trên cả hai node
- [ ] Health check trả về 0 trên master, 1 trên standby
- [ ] Test failover thành công (VIP chuyển, PostgreSQL promote)
- [ ] Test failback thành công (Node 1 tự động rebuild thành standby)
- [ ] Không có lỗi trong log
- [ ] Đăng ký SIP hoạt động (nếu đã cấu hình client)
- [ ] Cuộc gọi thử nghiệm thành công (nếu đã cấu hình trunk)

---

**Tiếp theo**: Đọc [README.md](README.md) để biết tài liệu đầy đủ.
