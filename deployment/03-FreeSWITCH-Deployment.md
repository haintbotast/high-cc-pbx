# 03 - FreeSWITCH 1.10 Deployment Guide

**Service**: FreeSWITCH 1.10 Media Server  
**Dependencies**: PostgreSQL 18 (voip schema), VoIP Admin (để post CDR)  
**Thời gian ước tính**: 2-3 giờ  
**Deploy sau**: [02-Kamailio-Deployment.md](02-Kamailio-Deployment.md)  
**Deploy trước**: [04-VoIP-Admin-Deployment.md](04-VoIP-Admin-Deployment.md)

---

## Tổng Quan

FreeSWITCH xử lý media và call control:
- Nhận SIP calls từ Kamailio (port 5080)
- Xử lý dialplan, IVR, voicemail
- Quản lý call queues
- Query directory/dialplan từ VoIP Admin API (mod_xml_curl)
- Gửi CDR đến VoIP Admin API (mod_xml_cdr)

### Chiến Lược Kết Nối

⚠️ **QUAN TRỌNG**: FreeSWITCH connect qua **VIP** (172.16.91.100:8080), KHÔNG phải localhost!

- FreeSWITCH (cả 2 nodes) → VoIP Admin API **VIP:8080** (172.16.91.100:8080)
- Dùng VIP vì:
  - FreeSWITCH query directory/dialplan **real-time cho MỖI CALL** (không chỉ CDR)
  - Nếu dùng localhost → khi failover, node đã down không query được
  - VIP đảm bảo luôn query active voip-admin (VIP follows master)
- VoIP Admin kết nối đến PostgreSQL LOCAL của chính nó
- 3 loại XML_CURL queries:
  1. **Directory**: Authentication, user settings (mod_xml_curl)
  2. **Dialplan**: Call routing logic (mod_xml_curl)
  3. **CDR**: Post call records (mod_xml_cdr)
- KHÔNG kết nối trực tiếp đến database (dùng API layer)

---

## Deployment Steps

> **Vai trò:** FreeSWITCH VoIP Expert

### 9.1 Add FreeSWITCH Repository

**Trên cả 2 nodes:**

```bash
# Add SignalWire repository for FreeSWITCH
wget -O - https://files.freeswitch.org/repo/deb/debian-release/fsstretch-archive-keyring.asc | sudo apt-key add -

echo "deb https://files.freeswitch.org/repo/deb/debian-release/ bookworm main" | \
    sudo tee /etc/apt/sources.list.d/freeswitch.list

sudo apt update
```

### 9.2 Install FreeSWITCH

**Trên cả 2 nodes:**

```bash
# Install FreeSWITCH and essential modules
sudo apt install -y \
    freeswitch \
    freeswitch-mod-commands \
    freeswitch-mod-console \
    freeswitch-mod-logfile \
    freeswitch-mod-sofia \
    freeswitch-mod-dialplan-xml \
    freeswitch-mod-dptools \
    freeswitch-mod-xml-curl \
    freeswitch-mod-xml-cdr \
    freeswitch-mod-event-socket \
    freeswitch-mod-db \
    freeswitch-mod-say-en \
    freeswitch-mod-tone-stream \
    freeswitch-mod-lua \
    freeswitch-mod-conference \
    freeswitch-mod-fifo \
    freeswitch-mod-hash

# Verify
freeswitch -version
```

### 9.3 Cấu Hình FreeSWITCH

**Trên cả 2 nodes:**

```bash
# Copy configs từ repo
cd /tmp/high-cc-pbx

sudo cp configs/freeswitch/autoload_configs/sofia.conf.xml \
        /etc/freeswitch/autoload_configs/

sudo cp configs/freeswitch/autoload_configs/xml_curl.conf.xml \
        /etc/freeswitch/autoload_configs/

sudo cp configs/freeswitch/autoload_configs/xml_cdr.conf.xml \
        /etc/freeswitch/autoload_configs/

sudo cp configs/freeswitch/autoload_configs/switch.conf.xml \
        /etc/freeswitch/autoload_configs/

sudo cp configs/freeswitch/autoload_configs/event_socket.conf.xml \
        /etc/freeswitch/autoload_configs/

sudo cp configs/freeswitch/autoload_configs/modules.conf.xml \
        /etc/freeswitch/autoload_configs/
```

**Replace passwords:**
```bash
# Get FreeSWITCH password from voip-admin config (same password)
FREESWITCH_PASS=$(grep freeswitch_password /root/.voip_credentials | cut -d'=' -f2)

sudo sed -i "s/CHANGE_THIS_PASSWORD/$FREESWITCH_PASS/g" \
    /etc/freeswitch/autoload_configs/xml_curl.conf.xml

sudo sed -i "s/CHANGE_THIS_PASSWORD/$FREESWITCH_PASS/g" \
    /etc/freeswitch/autoload_configs/xml_cdr.conf.xml
```

**QUAN TRỌNG - Verify VIP addresses:**
```bash
# xml_curl.conf.xml should use VIP (172.16.91.100), NOT localhost or node IPs
grep "gateway-url" /etc/freeswitch/autoload_configs/xml_curl.conf.xml
# Should show: http://172.16.91.100:8080/freeswitch/directory
#              http://172.16.91.100:8080/freeswitch/dialplan
#              http://172.16.91.100:8080/freeswitch/configuration

# xml_cdr.conf.xml should also use VIP
grep "url" /etc/freeswitch/autoload_configs/xml_cdr.conf.xml
# Should show: http://172.16.91.100:8080/api/v1/cdr
```

**Lý do dùng VIP thay vì localhost:**
- FreeSWITCH query directory/dialplan cho MỖI CALL (authentication, routing)
- VIP đảm bảo FreeSWITCH luôn connect đến active voip-admin
- Khi failover, VIP moves sang node mới → FreeSWITCH tự động query node mới
- Localhost chỉ work khi cả voip-admin và FreeSWITCH cùng node active

### 9.4 Adjust FreeSWITCH for Node-Specific IP

**Trên Node 1:**
```bash
sudo sed -i 's/172.16.91.102/172.16.91.101/g' \
    /etc/freeswitch/autoload_configs/sofia.conf.xml
```

**Trên Node 2:**
```bash
# IP đã đúng sẵn (172.16.91.102)
```

### 9.5 Configure Recordings Directory

**Trên cả 2 nodes:**

```bash
# Create recordings directory
sudo mkdir -p /recordings/freeswitch
sudo chown -R freeswitch:freeswitch /recordings/freeswitch
sudo chmod 755 /recordings/freeswitch
```

### 9.6 Start FreeSWITCH

**Trên cả 2 nodes:**

```bash
# Start FreeSWITCH
sudo systemctl start freeswitch
sudo systemctl enable freeswitch

# Check status
sudo systemctl status freeswitch

# Connect to FreeSWITCH CLI
sudo fs_cli

# Inside fs_cli:
sofia status
# Should see: internal profile UP

# Check modules
module_exists mod_xml_curl
module_exists mod_xml_cdr

# Exit
/exit
```

---


---

## Troubleshooting

### FreeSWITCH không start được
```bash
# Check logs
tail -100 /var/log/freeswitch/freeswitch.log

# Test config syntax
freeswitch -nc -nonat
```

### XML_CURL không query được directory
```bash
# Check voip-admin API health (trên node đang có VIP)
curl http://172.16.91.100:8080/health

# Hoặc check local voip-admin
curl http://localhost:8080/health

# Verify VIP address trong config
grep "gateway-url" /etc/freeswitch/autoload_configs/xml_curl.conf.xml
# PHẢI là VIP (172.16.91.100), KHÔNG phải localhost hoặc .101/.102

# Test directory API endpoint
curl -X POST "http://172.16.91.100:8080/freeswitch/directory" \
  -u "freeswitch:YOUR_PASSWORD" \
  -d "section=directory&tag_name=domain&key_name=name&key_value=default.local&user=1000&domain=default.local"

# Check FreeSWITCH logs
tail -f /var/log/freeswitch/freeswitch.log | grep xml_curl
```

### CDR không được gửi
```bash
# Check voip-admin health (dùng VIP)
curl http://172.16.91.100:8080/health

# Check xml_cdr config
grep "url" /etc/freeswitch/autoload_configs/xml_cdr.conf.xml
# PHẢI là VIP: http://172.16.91.100:8080/api/v1/cdr

# Check FreeSWITCH XML CDR logs
tail -f /var/log/freeswitch/freeswitch.log | grep xml_cdr

# Test CDR endpoint manually
curl -X POST http://172.16.91.100:8080/api/v1/cdr \
  -H "Content-Type: application/xml" \
  -d '<?xml version="1.0"?><cdr><variables><uuid>test-123</uuid></variables></cdr>'
```

---

## Verification Checklist

- [ ] FreeSWITCH 1.10.x installed (`freeswitch -version`)
- [ ] All required modules installed
- [ ] Config files deployed from repo
- [ ] Passwords replaced in xml_curl.conf.xml và xml_cdr.conf.xml
- [ ] IP addresses adjusted per node (Node 1: .101, Node 2: .102)
- [ ] Recordings directory created (/recordings/freeswitch)
- [ ] Listening on port 5080 for SIP from Kamailio
- [ ] Sofia profile UP (`sofia status` in fs_cli)
- [ ] mod_xml_curl loaded (`module_exists mod_xml_curl`)
- [ ] mod_xml_cdr loaded (`module_exists mod_xml_cdr`)
- [ ] VoIP Admin API accessible (curl localhost:8080/health)
- [ ] Service enabled (`systemctl is-enabled freeswitch`)
- [ ] Can register extensions and make test calls

---

**Tiếp theo**: [04-VoIP-Admin-Deployment.md](04-VoIP-Admin-Deployment.md)  
**Quay lại**: [README.md](README.md) - Deployment Overview

**Version**: 3.2.0  
**Last Updated**: 2025-11-20
