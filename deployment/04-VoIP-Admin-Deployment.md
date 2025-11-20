# 04 - VoIP Admin Service Deployment Guide

**Service**: VoIP Admin API (Go 1.23)  
**Dependencies**: PostgreSQL 18 (voip schema)  
**Thời gian ước tính**: 1 giờ  
**Deploy sau**: [03-FreeSWITCH-Deployment.md](03-FreeSWITCH-Deployment.md)  
**Deploy trước**: [05-Keepalived-HA-Deployment.md](05-Keepalived-HA-Deployment.md)

---

## Tổng Quan

VoIP Admin là API gateway và quản lý trung tâm:
- RESTful API cho quản lý extensions, queues, domains
- Nhận CDR từ FreeSWITCH (HTTP POST endpoint)
- Xử lý CDR async và lưu vào database
- Health check endpoints cho monitoring
- WebSocket support cho real-time events

### Chiến Lược Kết Nối

- VoIP Admin trên Node 1 → PostgreSQL **172.16.91.101** (LOCAL)
- VoIP Admin trên Node 2 → PostgreSQL **172.16.91.102** (LOCAL)
- Mỗi node kết nối đến PostgreSQL local của chính nó
- FreeSWITCH post CDR đến localhost:8080
- KHÔNG kết nối qua VIP

### Lưu Ý Quan Trọng

⚠️ **Node 2 Database Connection**: Trên Node 2, VoIP Admin kết nối đến PostgreSQL standby (read-only in recovery mode). Điều này OK vì:
- CDR được gửi đến Node có VIP (active node)
- Node standby chỉ nhận CDR khi nó trở thành master sau failover
- Replication đồng bộ tất cả data giữa 2 nodes

---

## Deployment Steps

> **Vai trò:** Go Developer & DevOps Engineer

### 10.1 Install Go 1.23

**Trên cả 2 nodes:**

```bash
# Download Go 1.23
cd /tmp
wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz

# Remove old Go (if exists)
sudo rm -rf /usr/local/go

# Extract
sudo tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz

# Add to PATH
echo 'export PATH=$PATH:/usr/local/go/bin' | sudo tee -a /etc/profile
source /etc/profile

# Verify
go version
# Expect: go version go1.23.0 linux/amd64
```

### 10.2 Build VoIP Admin

**Trên Node 1 (build once, copy binary):**

```bash
# Clone repo (if not already)
cd /opt
sudo git clone https://github.com/haintbotast/high-cc-pbx.git
sudo chown -R voipadmin:voipadmin high-cc-pbx

cd high-cc-pbx/voip-admin

# Download dependencies
go mod download

# Build
go build -o bin/voipadmind ./cmd/voipadmind

# Verify binary
./bin/voipadmind -version
# Expect: VoIP Admin Service v1.0.0
```

**Copy binary to Node 2:**
```bash
scp bin/voipadmind voipadmin@172.16.91.102:/tmp/
```

**Trên Node 2:**
```bash
sudo mkdir -p /opt/high-cc-pbx/voip-admin/bin
sudo mv /tmp/voipadmind /opt/high-cc-pbx/voip-admin/bin/
sudo chown voipadmin:voipadmin /opt/high-cc-pbx/voip-admin/bin/voipadmind
sudo chmod +x /opt/high-cc-pbx/voip-admin/bin/voipadmind
```

### 10.3 Cấu Hình VoIP Admin

**Trên cả 2 nodes:**

```bash
# Copy config
cd /opt/high-cc-pbx
sudo cp configs/voip-admin/config.yaml /etc/voip-admin/config.yaml

# Replace passwords
VOIPADMIN_PASS=$(grep voipadmin_db_password /root/.voip_credentials | cut -d'=' -f2)
FREESWITCH_PASS=$(grep freeswitch_password /root/.voip_credentials | cut -d'=' -f2)
API_KEY=$(openssl rand -base64 32)

# Save API key
echo "voipadmin_api_key=$API_KEY" | sudo tee -a /root/.voip_credentials

# Replace in config
sudo sed -i "s/password: \".*\"/password: \"$VOIPADMIN_PASS\"/" /etc/voip-admin/config.yaml
sudo sed -i "s/freeswitch_password: \".*\"/freeswitch_password: \"$FREESWITCH_PASS\"/" /etc/voip-admin/config.yaml
sudo sed -i "s/YOUR_SECURE_API_KEY_1/$API_KEY/" /etc/voip-admin/config.yaml
```

**QUAN TRỌNG - Customize Database IP per Node:**

**Trên Node 1:**
```bash
# Node 1 connects to LOCAL PostgreSQL (172.16.91.101)
# Config đã đúng sẵn (host: "172.16.91.101")
# Verify:
grep "host:" /etc/voip-admin/config.yaml | head -2
# Should show: host: "172.16.91.101"
```

**Trên Node 2:**
```bash
# Node 2 MUST connect to its LOCAL PostgreSQL (172.16.91.102)
sudo sed -i 's/host: "172.16.91.101"/host: "172.16.91.102"/' /etc/voip-admin/config.yaml

# Verify:
grep "host:" /etc/voip-admin/config.yaml | head -2
# Should show: host: "172.16.91.102"
```

**Lưu ý Kiến Trúc:**
- Mỗi node kết nối đến LOCAL PostgreSQL của chính nó
- VoIP Admin trên Node 1 → PostgreSQL **172.16.91.101** (MASTER)
- VoIP Admin trên Node 2 → PostgreSQL **172.16.91.102** (STANDBY in recovery mode)
- Replication đồng bộ data tự động giữa 2 nodes

### 10.4 Create systemd Service

**Trên cả 2 nodes:**

```bash
sudo nano /etc/systemd/system/voip-admin.service
```

Content:
```ini
[Unit]
Description=VoIP Admin Service
Documentation=https://github.com/haintbotast/high-cc-pbx
After=network.target postgresql.service

[Service]
Type=simple
User=voipadmin
Group=voipadmin
WorkingDirectory=/opt/high-cc-pbx/voip-admin
ExecStart=/opt/high-cc-pbx/voip-admin/bin/voipadmind -config /etc/voip-admin/config.yaml
Restart=on-failure
RestartSec=5s

# Resource limits
LimitNOFILE=65536
LimitNPROC=16384

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=voip-admin

[Install]
WantedBy=multi-user.target
```

### 10.5 Start VoIP Admin

**Trên cả 2 nodes:**

```bash
# Reload systemd
sudo systemctl daemon-reload

# Start service
sudo systemctl start voip-admin
sudo systemctl enable voip-admin

# Check status
sudo systemctl status voip-admin

# Check logs
journalctl -u voip-admin -f

# Test health endpoint
curl http://localhost:8080/health
# Expect: {"status":"ok","timestamp":"..."}

curl http://localhost:8080/health/stats
# Expect: JSON with database, cache stats
```


---

## Troubleshooting

### VoIP Admin không start được
```bash
# Check logs
journalctl -u voip-admin -n 100 --no-pager

# Check config
cat /etc/voip-admin/config.yaml | grep -v password

# Test binary
/opt/high-cc-pbx/voip-admin/bin/voipadmind -version
```

### Database connection errors
```bash
# Test database connection from voipadmin user
sudo -u voipadmin psql -h 172.16.91.101 -U voipadmin -d voipdb -c "SELECT 1;"

# Check config database host (must be LOCAL IP)
grep "host:" /etc/voip-admin/config.yaml

# Verify .pgpass (if used)
ls -la ~/.pgpass
```

### API không response
```bash
# Check service status
systemctl status voip-admin

# Check listening ports
sudo netstat -tlpn | grep voipadmind
# Should see :8080

# Test health endpoint
curl http://localhost:8080/health
curl http://localhost:8080/health/stats
```

### CDR không được lưu
```bash
# Check CDR queue table
sudo -u postgres psql -d voipdb -c "SELECT COUNT(*) FROM voip.cdr_queue WHERE status='pending';"

# Check CDR processing logs
journalctl -u voip-admin | grep -i cdr

# Manual test CDR endpoint
curl -X POST http://localhost:8080/api/v1/cdr \
  -H "Content-Type: application/json" \
  -H "X-API-Key: YOUR_API_KEY" \
  -d '{"uuid":"test-123","caller":"1000","destination":"1001"}'
```

---

## Verification Checklist

- [ ] Go 1.23 installed (`go version`)
- [ ] Binary built and executable
- [ ] Config file created with correct passwords
- [ ] Database host set to LOCAL IP (not VIP)
- [ ] Node 1: host="172.16.91.101"
- [ ] Node 2: host="172.16.91.102"
- [ ] systemd service created and enabled
- [ ] Service running (`systemctl status voip-admin`)
- [ ] Health endpoint responding (HTTP 200)
- [ ] Listening on port 8080
- [ ] Can receive test CDR posts

---

**Tiếp theo**: [05-Keepalived-HA-Deployment.md](05-Keepalived-HA-Deployment.md)  
**Quay lại**: [README.md](README.md) - Deployment Overview

**Version**: 3.2.0  
**Last Updated**: 2025-11-20
