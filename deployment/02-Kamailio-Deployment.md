# 02 - Kamailio 6.0 Deployment Guide

**Service**: Kamailio 6.0 SIP Server  
**Dependencies**: PostgreSQL 18 (database schemas, kamailio + kamailioro users)  
**Thời gian ước tính**: 1-2 giờ  
**Deploy sau**: [01-PostgreSQL-Deployment.md](01-PostgreSQL-Deployment.md)  
**Deploy trước**: [03-FreeSWITCH-Deployment.md](03-FreeSWITCH-Deployment.md)

---

## Tổng Quan

Kamailio đóng vai trò SIP Proxy/Registrar trong hệ thống:
- Nhận SIP registrations từ softphones/hardphones
- Route calls đến FreeSWITCH backends (dispatcher)
- Xác thực qua PostgreSQL (kamailio.subscriber view)
- Load balancing giữa FreeSWITCH nodes

### Kamailio 6.0 Breaking Changes

⚠️ **QUAN TRỌNG**: Kamailio 6.0 có nhiều breaking changes so với 5.x. Xem chi tiết:
- [KAMAILIO-6-COMPATIBILITY.md](../KAMAILIO-6-COMPATIBILITY.md)

Các changes chính đã được fix trong configs/kamailio/kamailio.cfg:
- `hash_size` phải là power of 2 (4096)
- `get_profile_size()` return về variable thay vì $var(result)
- `dlg_manage()` thay cho `setflag(4)`
- UAC module `append_fromtag=1` bắt buộc

### Chiến Lược Kết Nối Database

- Kamailio trên Node 1 → PostgreSQL **172.16.91.101** (LOCAL)
- Kamailio trên Node 2 → PostgreSQL **172.16.91.102** (LOCAL)
- Mỗi node kết nối đến PostgreSQL local của chính nó
- KHÔNG kết nối qua VIP

---

## Deployment Steps

> **Vai trò:** Kamailio SIP Expert

### 8.1 Add Kamailio Repository

**Trên cả 2 nodes:**

```bash
# Install prerequisites
sudo apt install -y gnupg curl

# Download and install Kamailio GPG key (modern method)
curl -fsSL https://deb.kamailio.org/kamailiodebkey.gpg | \
    sudo gpg --dearmor -o /usr/share/keyrings/kamailio-archive-keyring.gpg

# Add repository with signed-by
echo "deb [signed-by=/usr/share/keyrings/kamailio-archive-keyring.gpg] http://deb.kamailio.org/kamailio60 bookworm main" | \
    sudo tee /etc/apt/sources.list.d/kamailio.list

# Update
sudo apt update

# Verify Kamailio 6.0 is available
apt-cache policy kamailio | grep -A2 "Candidate"
# Should show version 6.0.x from deb.kamailio.org
```

### 8.2 Install Kamailio

**Trên cả 2 nodes:**

```bash
# Install Kamailio with PostgreSQL modules
sudo apt install -y \
    kamailio \
    kamailio-postgres-modules \
    kamailio-tls-modules \
    kamailio-websocket-modules \
    kamailio-json-modules \
    kamailio-utils-modules \
    kamailio-xml-modules

# Verify
kamailio -v
# Expect: version: kamailio 6.0.x
```

### 8.3 Cấu Hình Kamailio

**Trên cả 2 nodes:**

```bash
# Backup original config
sudo cp /etc/kamailio/kamailio.cfg /etc/kamailio/kamailio.cfg.orig

# Copy config từ repo
cd /tmp/high-cc-pbx
sudo cp configs/kamailio/kamailio.cfg /etc/kamailio/

# Replace PASSWORD trong config
KAMAILIO_PASS=$(grep kamailio_db_password /root/.voip_credentials | cut -d'=' -f2)
sudo sed -i "s/PASSWORD/$KAMAILIO_PASS/" /etc/kamailio/kamailio.cfg
```

**QUAN TRỌNG - Customize per Node:**

**Trên Node 1:**
```bash
# Database: đã đúng sẵn (172.16.91.101)
# Listen addresses: đã đúng sẵn (VIP + .101)
```

**Trên Node 2:**
```bash
# Fix database IP
sudo sed -i 's/172.16.91.101/172.16.91.102/g' /etc/kamailio/kamailio.cfg

# Fix listen addresses (thay .101 bằng .102, giữ VIP)
sudo sed -i 's/listen=udp:172.16.91.101/listen=udp:172.16.91.102/' /etc/kamailio/kamailio.cfg
sudo sed -i 's/listen=tcp:172.16.91.101/listen=tcp:172.16.91.102/' /etc/kamailio/kamailio.cfg
```

**Verify:**
```bash
# Check database URL
grep "DBURL" /etc/kamailio/kamailio.cfg
# Node 1: 172.16.91.101, Node 2: 172.16.91.102

# Check listen
grep "^listen=" /etc/kamailio/kamailio.cfg
# Node 1: VIP + .101, Node 2: VIP + .102

# Test syntax
sudo kamailio -c -f /etc/kamailio/kamailio.cfg
```

**Configure kamctl (Kamailio control tool):**

```bash
# Copy kamctlrc
sudo cp configs/kamailio/kamctlrc /etc/kamailio/kamctlrc

# Replace passwords
KAMAILIO_PASS=$(grep kamailio_db_password /root/.voip_credentials | cut -d'=' -f2)
KAMAILIORO_PASS=$(grep kamailioro_db_password /root/.voip_credentials | cut -d'=' -f2)
sudo sed -i "s/CHANGE_ME_KAMAILIO_PASSWORD/$KAMAILIO_PASS/" /etc/kamailio/kamctlrc
sudo sed -i "s/CHANGE_ME_KAMAILIORO_PASSWORD/$KAMAILIORO_PASS/" /etc/kamailio/kamctlrc

# Customize per node (database host)
# Node 2 only:
# sudo sed -i 's/172.16.91.101/172.16.91.102/' /etc/kamailio/kamctlrc

# Secure the file
sudo chmod 600 /etc/kamailio/kamctlrc
sudo chown kamailio:kamailio /etc/kamailio/kamctlrc

# Test kamctl
kamctl dispatcher show
```

**Configure Kamailio logging (rsyslog):**

```bash
# Copy rsyslog config
sudo cp configs/kamailio/kamailio-rsyslog.conf /etc/rsyslog.d/kamailio.conf

# Create log file
sudo touch /var/log/kamailio.log
sudo chmod 640 /var/log/kamailio.log
sudo chown syslog:adm /var/log/kamailio.log

# Restart rsyslog
sudo systemctl restart rsyslog
```

### 8.4 Populate Kamailio Tables

**Trên Node 1 ONLY (database đã có rồi):**

Kamailio tables đã được tạo ở bước 6.7 (database/schemas/02-kamailio-schema.sql), chỉ cần verify:

```bash
sudo -u postgres psql -d voipdb -c "\dt kamailio.*"
```

### 8.5 Configure Kamailio Default Settings

**Trên cả 2 nodes:**

```bash
sudo nano /etc/default/kamailio
```

Set:
```bash
RUN_KAMAILIO=yes
USER=kamailio
GROUP=kamailio
SHM_MEMORY=512
PKG_MEMORY=16
CFGFILE=/etc/kamailio/kamailio.cfg
```

### 8.6 Populate Dispatcher Table

**Trên Node 1:**

```bash
sudo -u postgres psql -d voipdb <<EOF
-- Add FreeSWITCH servers to dispatcher
-- setid=1 (như định nghĩa trong kamailio.cfg: DS_SETID 1)

INSERT INTO kamailio.dispatcher (setid, destination, flags, priority, attrs, description)
VALUES
    (1, 'sip:172.16.91.101:5080', 0, 0, '', 'FreeSWITCH Node 1'),
    (1, 'sip:172.16.91.102:5080', 0, 0, '', 'FreeSWITCH Node 2');

-- Verify
SELECT * FROM kamailio.dispatcher;
EOF
```

**Replication sẽ tự động sync sang Node 2.**

### 8.7 Start Kamailio

**Trên cả 2 nodes:**

```bash
# Start Kamailio
sudo systemctl start kamailio
sudo systemctl enable kamailio

# Check status
sudo systemctl status kamailio

# Check logs
sudo tail -f /var/log/kamailio.log

# Test listening
sudo netstat -tulpn | grep kamailio
# Should see ports 5060 TCP/UDP
```

### 8.8 Test Kamailio

**Trên Node 1:**

```bash
# kamctl tool
sudo kamctl db show

# Dispatcher list
sudo kamctl dispatcher show

# Monitor
sudo kamctl monitor
```

---


---

## Troubleshooting

### Kamailio không start được
```bash
# Check syntax
sudo kamailio -c -f /etc/kamailio/kamailio.cfg

# Check logs
sudo tail -100 /var/log/kamailio.log
journalctl -u kamailio -n 100
```

### Database connection errors
```bash
# Test connection
sudo -u kamailio psql -h 127.0.0.1 -U kamailio -d voipdb -c "SELECT 1;"

# Check kamctlrc
grep DBURL /etc/kamailio/kamctlrc
```

### kamctl không hoạt động
```bash
# Verify kamctlrc permissions
ls -la /etc/kamailio/kamctlrc
# Should be: -rw------- kamailio:kamailio

# Test database access
kamctl db show
```

---

## Verification Checklist

- [ ] Kamailio 6.0.x installed (verify with `kamailio -v`)
- [ ] Config syntax valid (`kamailio -c`)
- [ ] Database connection working (kamctl db show)
- [ ] Listening on correct IPs (VIP + local IP)
- [ ] Dispatcher table populated with FreeSWITCH destinations
- [ ] rsyslog configured, /var/log/kamailio.log receiving logs
- [ ] Service enabled (`systemctl is-enabled kamailio`)
- [ ] kamctl commands working

---

**Tiếp theo**: [03-FreeSWITCH-Deployment.md](03-FreeSWITCH-Deployment.md)  
**Quay lại**: [README.md](README.md) - Deployment Overview

**Version**: 3.2.0  
**Last Updated**: 2025-11-20
