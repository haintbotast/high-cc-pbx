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
- Gửi CDR đến VoIP Admin API
- ODBC connection đến PostgreSQL cho directory/dialplan

### Chiến Lược Kết Nối

- FreeSWITCH trên Node 1 → PostgreSQL **172.16.91.101** (LOCAL via ODBC)
- FreeSWITCH trên Node 2 → PostgreSQL **172.16.91.102** (LOCAL via ODBC)
- CDR posting → VoIP Admin API qua HTTP (localhost:8080)
- KHÔNG kết nối qua VIP

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

### ODBC connection errors
```bash
# Test ODBC connection
echo "SELECT 1;" | isql -v voipdb-odbc

# Check odbc.ini
cat /etc/odbc.ini

# Verify FreeSWITCH ODBC DSN
fs_cli -x "odbc query voipdb-odbc SELECT 1"
```

### CDR không được gửi
```bash
# Check voip-admin health
curl http://localhost:8080/health

# Check FreeSWITCH XML CDR logs
tail -f /var/log/freeswitch/freeswitch.log | grep xml_cdr

# Test CDR endpoint manually
curl -X POST http://localhost:8080/api/v1/cdr \
  -H "Content-Type: application/json" \
  -d '{"test": "data"}'
```

---

## Verification Checklist

- [ ] FreeSWITCH 1.10.x installed
- [ ] ODBC configured and tested (`isql -v voipdb-odbc`)
- [ ] FreeSWITCH ODBC connection working (`odbc query`)
- [ ] Listening on port 5080 for SIP from Kamailio
- [ ] XML dialplan loaded
- [ ] mod_xml_cdr configured to post to VoIP Admin
- [ ] Service enabled (`systemctl is-enabled freeswitch`)
- [ ] Can make test calls between extensions

---

**Tiếp theo**: [04-VoIP-Admin-Deployment.md](04-VoIP-Admin-Deployment.md)  
**Quay lại**: [README.md](README.md) - Deployment Overview

**Version**: 3.2.0  
**Last Updated**: 2025-11-20
