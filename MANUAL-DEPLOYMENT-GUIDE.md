# Hướng Dẫn Triển Khai Thủ Công - VoIP HA System

**Phiên bản:** 1.0
**Ngày:** 2025-01-18
**Tác giả:** Multi-Expert Team
**Hệ thống:** High Availability VoIP Platform (600-800 Concurrent Calls)

---

## Mục Lục

1. [Tổng Quan Kiến Trúc](#1-tổng-quan-kiến-trúc)
2. [Chuẩn Bị Hệ Thống](#2-chuẩn-bị-hệ-thống)
3. [Cài Đặt và Cấu Hình OS](#3-cài-đặt-và-cấu-hình-os)
4. [Tuning Hệ Điều Hành](#4-tuning-hệ-điều-hành)
5. [Cấu Hình Firewall](#5-cấu-hình-firewall)
6. [Cài Đặt PostgreSQL 18](#6-cài-đặt-postgresql-18)
7. [Cấu Hình PostgreSQL Replication](#7-cấu-hình-postgresql-replication)
8. [Cài Đặt Kamailio 6.0](#8-cài-đặt-kamailio-60)
9. [Cài Đặt FreeSWITCH 1.10](#9-cài-đặt-freeswitch-110)
10. [Cài Đặt VoIP Admin Service](#10-cài-đặt-voip-admin-service)
11. [Cấu Hình Keepalived](#11-cấu-hình-keepalived)
12. [Tạo Dữ Liệu Mẫu](#12-tạo-dữ-liệu-mẫu)
13. [Kiểm Thử Tích Hợp](#13-kiểm-thử-tích-hợp)
14. [Truy Cập Web Interface](#14-truy-cập-web-interface)
15. [Monitoring và Maintenance](#15-monitoring-và-maintenance)

---

## 1. Tổng Quan Kiến Trúc

### 1.1 Kiến Trúc Tổng Thể

```
                              ┌──────────────────────────────────┐
                              │  Virtual IP: 172.16.91.100       │
                              │  (Keepalived VRRP)               │
                              └────────────┬─────────────────────┘
                                           │
                      ┌────────────────────┴────────────────────┐
                      │                                         │
         ┌────────────▼──────────┐              ┌──────────────▼─────────┐
         │  NODE 1 (MASTER)      │              │  NODE 2 (BACKUP)       │
         │  IP: 172.16.91.101    │              │  IP: 172.16.91.102     │
         ├───────────────────────┤              ├────────────────────────┤
         │                       │              │                        │
         │  Kamailio 6.0.4       │◄────SIP─────►│  Kamailio 6.0.4        │
         │  (Port 5060)          │              │  (Port 5060)           │
         │                       │              │                        │
         │  FreeSWITCH 1.10      │◄───Media────►│  FreeSWITCH 1.10       │
         │  (Port 5080)          │              │  (Port 5080)           │
         │  RTP: 16384-32768     │              │  RTP: 16384-32768      │
         │                       │              │                        │
         │  VoIP Admin (Go)      │              │  VoIP Admin (Go)       │
         │  (Port 8080)          │              │  (Port 8080)           │
         │        │              │              │        │               │
         │        ▼              │              │        ▼               │
         │  PostgreSQL 18        │◄──Streaming──│  PostgreSQL 18         │
         │  (Port 5432)          │  Replication │  (Port 5432)           │
         │  - voipdb (MASTER)    │              │  - voipdb (STANDBY)    │
         │  - Schemas:           │              │  - Read-only           │
         │    + voip             │              │    replicate từ Node1  │
         │    + kamailio         │              │                        │
         │    + public           │              │                        │
         └───────────────────────┘              └────────────────────────┘

         16 cores, 64GB RAM                     16 cores, 64GB RAM
         500GB SSD + 3TB HDD                    500GB SSD + 3TB HDD
```

### 1.2 Luồng Dữ Liệu

**QUAN TRỌNG - Kiến Trúc Kết Nối Database:**
- **Mỗi node kết nối đến PostgreSQL LOCAL của chính nó**
- Node 1: Kamailio + FreeSWITCH + VoIP Admin → PostgreSQL **172.16.91.101**
- Node 2: Kamailio + FreeSWITCH + VoIP Admin → PostgreSQL **172.16.91.102**
- PostgreSQL streaming replication đồng bộ dữ liệu giữa 2 nodes
- **VIP (172.16.91.100) CHỈ dùng cho SIP traffic, KHÔNG dùng cho database!**

**SIP Registration Flow:**
```
SIP Phone → VIP (172.16.91.100:5060)
          → Keepalived routes to MASTER Node1
          → Kamailio (Node1)
          → Query LOCAL PostgreSQL (172.16.91.101) via view kamailio.subscriber
          → HA1 authentication
          → 200 OK with Contact binding saved
```

**Call Flow:**
```
Caller → Kamailio (SIP proxy) → FreeSWITCH (media server)
                                       ↓
                                 XML_CURL → VoIP Admin
                                       ↓
                                 Directory lookup (cache/DB)
                                       ↓
                                 Return XML with extension config
                                       ↓
                                 Bridge call to destination
                                       ↓
                                 CDR → POST to VoIP Admin
                                       ↓
                                 Insert voip.cdr_queue
                                       ↓
                                 Background worker process
                                       ↓
                                 Insert voip.cdr (final)
```

**Database Replication Flow:**
```
Node1 PostgreSQL (MASTER)
  ↓ Write transaction
  ↓ WAL (Write-Ahead Log)
  ↓ Streaming replication
  → Node2 PostgreSQL (STANDBY)
    ↓ Apply WAL
    ↓ Read-only queries allowed
```

### 1.3 Thông Số Kỹ Thuật

| Component | Version | Port | Purpose |
|-----------|---------|------|---------|
| Debian | 12 (Bookworm) | - | Operating System |
| PostgreSQL | 18.x | 5432 | Database |
| Kamailio | 6.0.4 | 5060 (UDP/TCP) | SIP Proxy |
| FreeSWITCH | 1.10.x | 5080 (SIP), 16384-32768 (RTP) | Media Server |
| VoIP Admin | 1.0.0 | 8080 (HTTP) | Management API |
| Keepalived | 2.x | VRRP | HA/Failover |

### 1.4 Network Requirements

- **VIP:** 172.16.91.100
- **Node 1:** 172.16.91.101
- **Node 2:** 172.16.91.102
- **Netmask:** 255.255.255.0 (/24)
- **Gateway:** 172.16.91.1 (assumed)
- **Interface:** ens33 (hoặc eth0, kiểm tra với `ip a`)

---

## 2. Chuẩn Bị Hệ Thống

> **Vai trò:** System Administrator

### 2.1 Hardware Checklist

**Trên cả 2 nodes:**

- [ ] CPU: 16 cores (hoặc 8 cores hyperthreaded)
- [ ] RAM: 64 GB
- [ ] Disk 1: 500 GB SSD (OS + databases)
- [ ] Disk 2: 3 TB HDD (call recordings, logs)
- [ ] Network: 1Gbps interface
- [ ] IPMI/iLO access (cho remote management)

### 2.2 Cài Đặt Debian 12

**Trên cả 2 nodes:**

1. Download Debian 12 (Bookworm) ISO
   - URL: https://www.debian.org/download
   - Chọn: debian-12.x.x-amd64-netinst.iso

2. Boot từ USB/CD và cài đặt:
   - Language: English
   - Location: Vietnam (hoặc phù hợp)
   - Keyboard: US
   - Hostname:
     - Node 1: `voip-node1`
     - Node 2: `voip-node2`
   - Domain: `example.com` (thay bằng domain thật)
   - Root password: Tạo password mạnh (lưu vào password manager)
   - User account: Tạo user `voipadmin`

3. Disk partitioning:
   ```
   /dev/sda (500GB SSD):
     /boot       1GB    ext4
     /           50GB   ext4
     /var        200GB  ext4  (PostgreSQL data, logs)
     /tmp        10GB   ext4
     swap        16GB
     /home       223GB  ext4

   /dev/sdb (3TB HDD):
     /recordings 3TB    ext4  (Call recordings)
   ```

4. Software selection:
   - [x] SSH server
   - [x] Standard system utilities
   - [ ] Desktop environment (không cần)

5. Finish installation và reboot

### 2.3 Cấu Hình Network

**Trên Node 1:**

```bash
# Kiểm tra interface name
ip a

# Edit network config
sudo nano /etc/network/interfaces
```

Thêm nội dung:
```
# Node 1 static IP
auto ens33
iface ens33 inet static
    address 172.16.91.101
    netmask 255.255.255.0
    gateway 172.16.91.1
    dns-nameservers 8.8.8.8 8.8.4.4
```

**Trên Node 2:**

```bash
sudo nano /etc/network/interfaces
```

```
# Node 2 static IP
auto ens33
iface ens33 inet static
    address 172.16.91.102
    netmask 255.255.255.0
    gateway 172.16.91.1
    dns-nameservers 8.8.8.8 8.8.4.4
```

**Restart network:**
```bash
sudo systemctl restart networking
```

**Verify:**
```bash
ip addr show ens33
ping -c 3 172.16.91.1
ping -c 3 8.8.8.8
```

**Kiểm tra kết nối giữa 2 nodes:**
```bash
# Từ Node 1
ping -c 3 172.16.91.102

# Từ Node 2
ping -c 3 172.16.91.101
```

### 2.4 Cập Nhật Hệ Thống

**Trên cả 2 nodes:**

```bash
# Update package list
sudo apt update

# Upgrade all packages
sudo apt upgrade -y

# Install essential tools
sudo apt install -y \
    vim \
    git \
    curl \
    wget \
    htop \
    iotop \
    iftop \
    net-tools \
    tcpdump \
    screen \
    tmux \
    rsync \
    ntp \
    chrony

# Reboot nếu kernel được update
sudo reboot
```

### 2.5 Cấu Hình Hostname và Hosts

**Trên Node 1:**
```bash
sudo hostnamectl set-hostname voip-node1

sudo nano /etc/hosts
```

Thêm:
```
127.0.0.1       localhost
172.16.91.101   voip-node1.example.com  voip-node1
172.16.91.102   voip-node2.example.com  voip-node2
172.16.91.100   voip-vip.example.com    voip-vip
```

**Trên Node 2:**
```bash
sudo hostnamectl set-hostname voip-node2

sudo nano /etc/hosts
```

Same as Node 1.

**Verify:**
```bash
hostname
hostname -f
ping -c 2 voip-node1
ping -c 2 voip-node2
```

### 2.6 Cấu Hình SSH

**Trên cả 2 nodes:**

```bash
# Generate SSH key cho user voipadmin
ssh-keygen -t ed25519 -C "voipadmin@voip-node1"
# (Node 2: "voipadmin@voip-node2")

# Copy key sang node kia để SSH không cần password
# Từ Node 1:
ssh-copy-id voipadmin@172.16.91.102

# Từ Node 2:
ssh-copy-id voipadmin@172.16.91.101

# Test SSH
ssh voipadmin@172.16.91.102  # From Node 1
```

**Harden SSH:**
```bash
sudo nano /etc/ssh/sshd_config
```

Sửa:
```
PermitRootLogin no
PasswordAuthentication yes  # Hoặc no nếu chỉ dùng key
PubkeyAuthentication yes
X11Forwarding no
MaxAuthTries 3
```

```bash
sudo systemctl restart sshd
```

---

## 3. Cài Đặt và Cấu Hình OS

> **Vai trò:** Linux System Administrator

### 3.1 Timezone và NTP

**Trên cả 2 nodes:**

```bash
# Set timezone
sudo timedatectl set-timezone Asia/Ho_Chi_Minh

# Verify
timedatectl

# Cấu hình NTP (chrony)
sudo nano /etc/chrony/chrony.conf
```

Thêm:
```
server 0.asia.pool.ntp.org iburst
server 1.asia.pool.ntp.org iburst
server 2.asia.pool.ntp.org iburst
```

```bash
sudo systemctl restart chrony
sudo systemctl enable chrony

# Check sync
chronyc tracking
```

### 3.2 Tạo Thư Mục Làm Việc

**Trên cả 2 nodes:**

```bash
# Thư mục cho VoIP Admin
sudo mkdir -p /opt/voip-admin
sudo mkdir -p /etc/voip-admin
sudo mkdir -p /var/log/voip-admin

# Thư mục cho recordings
sudo mkdir -p /recordings/freeswitch

# Set ownership
sudo chown -R voipadmin:voipadmin /opt/voip-admin
sudo chown -R voipadmin:voipadmin /var/log/voip-admin
sudo chown -R freeswitch:freeswitch /recordings/freeswitch  # Sau khi cài FreeSWITCH

# Set permissions
sudo chmod 755 /opt/voip-admin
sudo chmod 755 /var/log/voip-admin
```

### 3.3 Disable SELinux (nếu có)

```bash
# Debian thường không có SELinux, kiểm tra
sestatus

# Nếu có và enabled, disable:
sudo nano /etc/selinux/config
# Set: SELINUX=disabled
# Reboot
```

---

## 4. Tuning Hệ Điều Hành

> **Vai trò:** Performance Engineer

### 4.1 Kernel Parameters (sysctl)

**Trên cả 2 nodes:**

```bash
sudo nano /etc/sysctl.d/99-voip-tuning.conf
```

Thêm nội dung sau:

```bash
# ============================================================================
# VoIP System Kernel Tuning
# Target: 600-800 concurrent calls
# ============================================================================

# Network Performance
# -------------------
# Tăng buffer sizes cho high throughput
net.core.rmem_max = 134217728                    # 128MB receive buffer
net.core.wmem_max = 134217728                    # 128MB send buffer
net.core.rmem_default = 16777216                 # 16MB default receive
net.core.wmem_default = 16777216                 # 16MB default send

# TCP buffer tuning
net.ipv4.tcp_rmem = 4096 87380 134217728        # min default max
net.ipv4.tcp_wmem = 4096 65536 134217728
net.ipv4.tcp_mem = 786432 1048576 26777216

# UDP buffer tuning (quan trọng cho RTP)
net.ipv4.udp_rmem_min = 16384
net.ipv4.udp_wmem_min = 16384

# Tăng connection tracking
net.netfilter.nf_conntrack_max = 1048576         # 1M connections
net.nf_conntrack_max = 1048576

# Tăng số file descriptors
fs.file-max = 2097152                            # 2M files

# Tăng số port range
net.ipv4.ip_local_port_range = 10000 65535

# TCP Performance
# ---------------
net.ipv4.tcp_fin_timeout = 30
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_keepalive_time = 300
net.ipv4.tcp_keepalive_intvl = 30
net.ipv4.tcp_keepalive_probes = 3
net.ipv4.tcp_slow_start_after_idle = 0
net.ipv4.tcp_no_metrics_save = 1

# Enable TCP Fast Open
net.ipv4.tcp_fastopen = 3

# Congestion control (BBR for better performance)
net.core.default_qdisc = fq
net.ipv4.tcp_congestion_control = bbr

# Network Security
# ----------------
net.ipv4.conf.all.rp_filter = 1
net.ipv4.conf.default.rp_filter = 1
net.ipv4.conf.all.accept_source_route = 0
net.ipv4.conf.default.accept_source_route = 0
net.ipv4.icmp_echo_ignore_broadcasts = 1
net.ipv4.icmp_ignore_bogus_error_responses = 1

# Allow binding to non-local IP (for VIP before Keepalived assigns it)
net.ipv4.ip_nonlocal_bind = 1

# Disable IPv6 (nếu không dùng)
net.ipv6.conf.all.disable_ipv6 = 1
net.ipv6.conf.default.disable_ipv6 = 1

# Shared Memory (cho PostgreSQL)
# -------------------------------
kernel.shmmax = 68719476736                      # 64GB
kernel.shmall = 4294967296                       # 16TB / PAGE_SIZE
kernel.shmmni = 4096

# Message Queues
kernel.msgmnb = 65536
kernel.msgmax = 65536

# Semaphores (cho database connections)
kernel.sem = 250 32000 100 128

# Process limits
kernel.pid_max = 4194304

# Virtual Memory
# --------------
vm.swappiness = 10                               # Ít swap hơn
vm.dirty_ratio = 15
vm.dirty_background_ratio = 5
vm.overcommit_memory = 2                         # Không overcommit
vm.overcommit_ratio = 95
```

**Áp dụng:**
```bash
sudo sysctl -p /etc/sysctl.d/99-voip-tuning.conf

# Verify
sysctl net.core.rmem_max
sysctl net.ipv4.tcp_congestion_control
```

### 4.2 System Limits (ulimit)

**Trên cả 2 nodes:**

```bash
sudo nano /etc/security/limits.conf
```

Thêm:
```bash
# VoIP System Limits
# ------------------

# Root user
root soft nofile 1048576
root hard nofile 1048576
root soft nproc 65536
root hard nproc 65536

# PostgreSQL
postgres soft nofile 65536
postgres hard nofile 65536
postgres soft nproc 16384
postgres hard nproc 16384

# Kamailio
kamailio soft nofile 65536
kamailio hard nofile 65536
kamailio soft nproc 16384
kamailio hard nproc 16384

# FreeSWITCH
freeswitch soft nofile 1048576
freeswitch hard nofile 1048576
freeswitch soft nproc 65536
freeswitch hard nproc 65536
freeswitch soft core unlimited
freeswitch hard core unlimited

# VoIP Admin
voipadmin soft nofile 65536
voipadmin hard nofile 65536
voipadmin soft nproc 16384
voipadmin hard nproc 16384

# All users default
* soft nofile 65536
* hard nofile 65536
```

**Verify:**
```bash
# Logout và login lại, sau đó:
ulimit -n
ulimit -u
```

### 4.3 systemd Limits

**Trên cả 2 nodes:**

```bash
sudo nano /etc/systemd/system.conf
```

Uncomment và sửa:
```
DefaultLimitNOFILE=1048576
DefaultLimitNPROC=65536
```

```bash
sudo nano /etc/systemd/user.conf
```

Same as above.

```bash
sudo systemctl daemon-reload
```

### 4.4 Transparent Huge Pages (Disable cho PostgreSQL)

```bash
sudo nano /etc/default/grub
```

Thêm vào `GRUB_CMDLINE_LINUX`:
```
transparent_hugepage=never
```

```bash
sudo update-grub
sudo reboot
```

**Verify sau khi reboot:**
```bash
cat /sys/kernel/mm/transparent_hugepage/enabled
# Expect: always madvise [never]
```

---

## 5. Cấu Hình Firewall

> **Vai trò:** Security Engineer

### 5.1 Install UFW (Uncomplicated Firewall)

**Trên cả 2 nodes:**

```bash
sudo apt install -y ufw

# Disable mặc định (cấu hình trước khi enable)
sudo ufw disable
```

### 5.2 Cấu Hình UFW Rules

**Trên cả 2 nodes:**

```bash
# Default policies
sudo ufw default deny incoming
sudo ufw default allow outgoing

# Allow SSH (QUAN TRỌNG - làm trước khi enable!)
sudo ufw allow from 172.16.91.0/24 to any port 22 proto tcp comment 'SSH from local network'

# Allow ping (ICMP)
sudo ufw allow from 172.16.91.0/24 proto icmp comment 'ICMP from local network'

# Kamailio SIP
sudo ufw allow 5060/tcp comment 'Kamailio SIP TCP'
sudo ufw allow 5060/udp comment 'Kamailio SIP UDP'

# FreeSWITCH SIP
sudo ufw allow from 172.16.91.0/24 to any port 5080 proto tcp comment 'FreeSWITCH SIP internal'
sudo ufw allow from 172.16.91.0/24 to any port 5080 proto udp comment 'FreeSWITCH SIP internal'

# FreeSWITCH RTP (media)
sudo ufw allow 16384:32768/udp comment 'FreeSWITCH RTP'

# FreeSWITCH Event Socket (only from localhost)
sudo ufw allow from 127.0.0.1 to any port 8021 proto tcp comment 'FreeSWITCH ESL localhost'

# VoIP Admin API
sudo ufw allow from 172.16.91.0/24 to any port 8080 proto tcp comment 'VoIP Admin API'

# PostgreSQL (only between nodes)
sudo ufw allow from 172.16.91.101 to any port 5432 proto tcp comment 'PostgreSQL from Node1'
sudo ufw allow from 172.16.91.102 to any port 5432 proto tcp comment 'PostgreSQL from Node2'

# Keepalived VRRP
sudo ufw allow from 172.16.91.0/24 proto vrrp comment 'Keepalived VRRP'

# RTPEngine (nếu sử dụng)
sudo ufw allow from 172.16.91.0/24 to any port 2223 proto udp comment 'RTPEngine control'
sudo ufw allow 30000:40000/udp comment 'RTPEngine media'

# Rsync (cho lsyncd)
sudo ufw allow from 172.16.91.101 to any port 873 proto tcp comment 'Rsync from Node1'
sudo ufw allow from 172.16.91.102 to any port 873 proto tcp comment 'Rsync from Node2'

# Enable firewall
sudo ufw enable

# Check status
sudo ufw status numbered
```

### 5.3 Verify Firewall

```bash
sudo ufw status verbose

# Test từ node kia
# Từ Node 1:
telnet 172.16.91.102 5432  # PostgreSQL
telnet 172.16.91.102 8080  # VoIP Admin

# Từ Node 2:
telnet 172.16.91.101 5432
telnet 172.16.91.101 8080
```

### 5.4 Fail2ban (Optional - Protection)

```bash
sudo apt install -y fail2ban

sudo nano /etc/fail2ban/jail.local
```

```ini
[DEFAULT]
bantime = 3600
findtime = 600
maxretry = 5
backend = systemd

[sshd]
enabled = true
port = 22
logpath = /var/log/auth.log

[kamailio]
enabled = true
port = 5060
logpath = /var/log/kamailio.log
maxretry = 10
```

```bash
sudo systemctl enable fail2ban
sudo systemctl start fail2ban
sudo fail2ban-client status
```

---

## 6. Cài Đặt PostgreSQL 18

> **Vai trò:** PostgreSQL Database Administrator (DBA)

### 6.1 Add PostgreSQL Repository

**Trên cả 2 nodes:**

```bash
# Add PostgreSQL APT repository
sudo apt install -y curl ca-certificates
sudo install -d /usr/share/postgresql-common/pgdg
sudo curl -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.asc \
    --fail https://www.postgresql.org/media/keys/ACCC4CF8.asc

sudo sh -c 'echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc] \
    https://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > \
    /etc/apt/sources.list.d/pgdg.list'

# Update package list
sudo apt update
```

### 6.2 Install PostgreSQL 18

**Trên cả 2 nodes:**

```bash
# Install PostgreSQL 18
sudo apt install -y postgresql-18 postgresql-contrib-18 postgresql-18-pgaudit

# Verify installation
psql --version
# Expect: psql (PostgreSQL) 18.x

# Check service
sudo systemctl status postgresql
```

### 6.3 Initial PostgreSQL Configuration

**Trên cả 2 nodes:**

```bash
# Stop PostgreSQL để cấu hình
sudo systemctl stop postgresql

# Backup original configs
sudo cp /etc/postgresql/18/main/postgresql.conf /etc/postgresql/18/main/postgresql.conf.orig
sudo cp /etc/postgresql/18/main/pg_hba.conf /etc/postgresql/18/main/pg_hba.conf.orig
```

### 6.4 PostgreSQL Performance Tuning

**Trên cả 2 nodes:**

```bash
sudo nano /etc/postgresql/18/main/postgresql.conf
```

Tìm và sửa các parameters sau (hoặc thêm vào cuối file):

```ini
# ============================================================================
# PostgreSQL 18 Tuning for VoIP System
# Hardware: 16 cores, 64GB RAM
# Workload: OLTP with high concurrency
# ============================================================================

# CONNECTIONS
# -----------
listen_addresses = '*'                           # Listen on all interfaces
port = 5432
max_connections = 300                            # Kamailio + VoIP Admin + connections

# MEMORY
# ------
shared_buffers = 16GB                            # 25% of RAM
effective_cache_size = 48GB                      # 75% of RAM
work_mem = 64MB                                  # Per query operation
maintenance_work_mem = 2GB                       # For VACUUM, CREATE INDEX
huge_pages = try
temp_buffers = 32MB

# QUERY PLANNING
# --------------
random_page_cost = 1.1                           # SSD is fast
effective_io_concurrency = 200                   # SSD supports high I/O
default_statistics_target = 100

# WRITE AHEAD LOG (WAL)
# ---------------------
wal_level = replica                              # For streaming replication
max_wal_senders = 5                              # Number of replication connections
max_replication_slots = 5
wal_keep_size = 1GB                              # Keep 1GB of WAL for standby
wal_compression = on
archive_mode = on                                # Enable WAL archiving
archive_command = 'test ! -f /var/lib/postgresql/18/archive/%f && cp %p /var/lib/postgresql/18/archive/%f'

# WAL Writing
wal_buffers = 16MB
wal_writer_delay = 200ms
commit_delay = 0
commit_siblings = 5

# CHECKPOINTS
# -----------
checkpoint_timeout = 15min                       # More frequent checkpoints
checkpoint_completion_target = 0.9
max_wal_size = 4GB
min_wal_size = 1GB

# AUTOVACUUM
# ----------
autovacuum = on
autovacuum_max_workers = 4
autovacuum_naptime = 30s                         # Check more frequently
autovacuum_vacuum_scale_factor = 0.1
autovacuum_analyze_scale_factor = 0.05

# LOGGING
# -------
logging_collector = on
log_directory = '/var/log/postgresql'
log_filename = 'postgresql-%Y-%m-%d_%H%M%S.log'
log_rotation_age = 1d
log_rotation_size = 100MB
log_min_duration_statement = 1000                # Log queries > 1s
log_line_prefix = '%t [%p]: [%l-1] user=%u,db=%d,app=%a,client=%h '
log_checkpoints = on
log_connections = on
log_disconnections = on
log_lock_waits = on
log_temp_files = 0
log_autovacuum_min_duration = 0

# QUERY STATS
# -----------
shared_preload_libraries = 'pg_stat_statements'
pg_stat_statements.max = 10000
pg_stat_statements.track = all
track_activities = on
track_counts = on
track_io_timing = on
track_functions = all

# LOCALE
# ------
lc_messages = 'en_US.UTF-8'
lc_monetary = 'en_US.UTF-8'
lc_numeric = 'en_US.UTF-8'
lc_time = 'en_US.UTF-8'
default_text_search_config = 'pg_catalog.english'
timezone = 'Asia/Ho_Chi_Minh'
```

**Tạo archive directory:**
```bash
sudo mkdir -p /var/lib/postgresql/18/archive
sudo chown postgres:postgres /var/lib/postgresql/18/archive
sudo chmod 700 /var/lib/postgresql/18/archive
```

### 6.5 PostgreSQL Authentication Configuration

**Trên cả 2 nodes:**

```bash
sudo nano /etc/postgresql/18/main/pg_hba.conf
```

Thay thế nội dung bằng:

```
# PostgreSQL Client Authentication Configuration
# ARCHITECTURE: Each node connects to its LOCAL PostgreSQL only
# TYPE  DATABASE        USER            ADDRESS                 METHOD

# Local connections
local   all             postgres                                peer
local   all             all                                     peer

# Replication connections (giữa 2 nodes)
host    replication     replicator      172.16.91.101/32        scram-sha-256
host    replication     replicator      172.16.91.102/32        scram-sha-256

# Kamailio connections - MD5 cho performance
# Applications connect to LOCAL PostgreSQL only
host    kamailio        kamailio        127.0.0.1/32            md5
host    kamailio        kamailio        172.16.91.101/32        md5
host    kamailio        kamailio        172.16.91.102/32        md5

# VoIP Admin - SCRAM-SHA-256
host    voip            voipadmin       127.0.0.1/32            scram-sha-256
host    voip            voipadmin       172.16.91.101/32        scram-sha-256
host    voip            voipadmin       172.16.91.102/32        scram-sha-256

# FreeSWITCH ODBC - MD5
host    voip            freeswitch      127.0.0.1/32            md5
host    voip            freeswitch      172.16.91.101/32        md5
host    voip            freeswitch      172.16.91.102/32        md5

# Localhost (for monitoring, admin)
host    all             all             127.0.0.1/32            scram-sha-256
host    all             all             ::1/128                 scram-sha-256

# Deny all other
host    all             all             0.0.0.0/0               reject
```

### 6.6 Khởi Động PostgreSQL và Tạo Database

**Trên Node 1 (MASTER):**

```bash
# Start PostgreSQL
sudo systemctl start postgresql
sudo systemctl enable postgresql

# Check status
sudo systemctl status postgresql

# Switch to postgres user
sudo -i -u postgres

# Create extension for pg_stat_statements
psql -c "CREATE EXTENSION IF NOT EXISTS pg_stat_statements;"

# Tạo users
psql <<EOF
-- Replication user
CREATE USER replicator WITH REPLICATION LOGIN PASSWORD 'CHANGE_REPLICATION_PASSWORD';

-- Kamailio user
CREATE USER kamailio WITH LOGIN PASSWORD 'CHANGE_KAMAILIO_PASSWORD';

-- VoIP Admin user
CREATE USER voip_admin WITH LOGIN PASSWORD 'CHANGE_VOIPADMIN_PASSWORD';

-- FreeSWITCH user (for CDR if needed)
CREATE USER freeswitch WITH LOGIN PASSWORD 'CHANGE_FREESWITCH_PASSWORD';

-- List users
\du
EOF

# Tạo database
createdb -O postgres voipdb

# Verify
psql -c "\l" | grep voipdb
```

**Save passwords:**
```bash
# Create secure file to store passwords
sudo nano /root/.voip_credentials
```

Thêm:
```
# PostgreSQL Credentials
replication_password=ACTUAL_PASSWORD_HERE
kamailio_db_password=ACTUAL_PASSWORD_HERE
voipadmin_db_password=ACTUAL_PASSWORD_HERE
freeswitch_db_password=ACTUAL_PASSWORD_HERE
```

```bash
sudo chmod 600 /root/.voip_credentials
```

### 6.7 Load Database Schema

**Trên Node 1:**

```bash
# Clone repo (nếu chưa có)
cd /tmp
git clone https://github.com/haintbotast/high-cc-pbx.git
cd high-cc-pbx

# Apply database schemas theo thứ tự
sudo -u postgres psql -d voipdb -f database/schemas/01-voip-schema.sql
sudo -u postgres psql -d voipdb -f database/schemas/02-kamailio-schema.sql
sudo -u postgres psql -d voipdb -f database/schemas/03-auth-integration.sql
sudo -u postgres psql -d voipdb -f database/schemas/04-production-fixes.sql

# Verify schemas
sudo -u postgres psql -d voipdb -c "\dn"
# Expect: kamailio, public, voip

# Verify tables
sudo -u postgres psql -d voipdb -c "\dt voip.*"
sudo -u postgres psql -d voipdb -c "\dt kamailio.*"

# Grant permissions
sudo -u postgres psql -d voipdb <<EOF
-- Kamailio permissions
GRANT USAGE ON SCHEMA kamailio TO kamailio;
GRANT SELECT ON ALL TABLES IN SCHEMA kamailio TO kamailio;
GRANT SELECT ON kamailio.subscriber TO kamailio;
GRANT ALL ON kamailio.location TO kamailio;
GRANT ALL ON kamailio.dispatcher TO kamailio;
GRANT ALL ON kamailio.dialog TO kamailio;
GRANT ALL ON kamailio.acc TO kamailio;

-- VoIP Admin permissions
GRANT USAGE ON SCHEMA voip TO voip_admin;
GRANT ALL ON ALL TABLES IN SCHEMA voip TO voip_admin;
GRANT ALL ON ALL SEQUENCES IN SCHEMA voip TO voip_admin;

-- FreeSWITCH permissions (read-only for most, write for CDR)
GRANT USAGE ON SCHEMA voip TO freeswitch;
GRANT SELECT ON voip.extensions TO freeswitch;
GRANT SELECT ON voip.domains TO freeswitch;
GRANT SELECT ON voip.queues TO freeswitch;
GRANT ALL ON voip.cdr_queue TO freeswitch;
GRANT ALL ON voip.cdr TO freeswitch;
EOF
```

**Verify database setup:**
```bash
sudo -u postgres psql -d voipdb <<EOF
-- Check schemas
SELECT schema_name FROM information_schema.schemata ORDER BY schema_name;

-- Check voip tables
SELECT table_name FROM information_schema.tables
WHERE table_schema = 'voip' ORDER BY table_name;

-- Check critical indexes
SELECT indexname FROM pg_indexes
WHERE schemaname = 'voip' AND tablename = 'extensions';

-- Should see: idx_extensions_auth_lookup, idx_extensions_active, etc.

-- Check view
SELECT * FROM kamailio.subscriber LIMIT 0;

-- Check trigger exists
SELECT tgname FROM pg_trigger WHERE tgname = 'extensions_calc_ha1_trigger';
EOF
```

---

## 7. Cấu Hình PostgreSQL Replication

> **Vai trò:** PostgreSQL DBA & HA Expert

### 7.1 Chuẩn Bị Replication trên Node 1 (MASTER)

**Trên Node 1:**

PostgreSQL đã được cấu hình sẵn cho replication ở bước 6.4 (wal_level = replica).

**Verify WAL settings:**
```bash
sudo -u postgres psql -c "SHOW wal_level;"
sudo -u postgres psql -c "SHOW max_wal_senders;"
sudo -u postgres psql -c "SHOW archive_mode;"
```

**Create replication slot:**
```bash
sudo -u postgres psql <<EOF
SELECT * FROM pg_create_physical_replication_slot('node2_slot');
SELECT slot_name, slot_type, active FROM pg_replication_slots;
EOF
```

### 7.2 Chuẩn Bị Node 2 (STANDBY)

**Trên Node 2:**

```bash
# Stop PostgreSQL
sudo systemctl stop postgresql

# Backup existing data (nếu có)
sudo mv /var/lib/postgresql/18/main /var/lib/postgresql/18/main.bak

# Create empty directory
sudo -u postgres mkdir -p /var/lib/postgresql/18/main
sudo -u postgres chmod 700 /var/lib/postgresql/18/main
```

### 7.3 Base Backup từ Node 1

**Trên Node 2:**

```bash
# Tạo .pgpass file để không cần nhập password
sudo -u postgres bash -c "cat > ~/.pgpass <<EOF
172.16.91.101:5432:replication:replicator:ACTUAL_REPLICATION_PASSWORD
EOF"

# QUAN TRỌNG: Phải dùng bash -c để expand ~ đúng
sudo -u postgres bash -c "chmod 600 ~/.pgpass"

# Thực hiện base backup
sudo -u postgres pg_basebackup \
    -h 172.16.91.101 \
    -D /var/lib/postgresql/18/main \
    -U replicator \
    -P \
    -v \
    -R \
    -X stream \
    -C \
    -S node2_slot

# -R: Tự động tạo standby.signal và postgresql.auto.conf
# -X stream: Stream WAL trong khi backup
# -C: Create replication slot (nếu slot chưa tồn tại)
# -S: Slot name
```

**Lưu ý:** Nếu gặp lỗi `ERROR: replication slot "node2_slot" already exists`:
```bash
# Option 1: Xóa slot cũ trên Node 1 (RECOMMENDED)
sudo -u postgres psql -c "SELECT pg_drop_replication_slot('node2_slot');"

# Sau đó chạy lại pg_basebackup ở trên

# Option 2: Hoặc bỏ flag -C nếu slot đã tồn tại
sudo -u postgres pg_basebackup \
    -h 172.16.91.101 \
    -D /var/lib/postgresql/18/main \
    -U replicator \
    -P -v -R -X stream \
    -S node2_slot
# (Lưu ý: không có -C)
```

**Quá trình này sẽ mất vài phút. Output:**
```
pg_basebackup: initiating base backup, waiting for checkpoint to complete
pg_basebackup: checkpoint completed
pg_basebackup: write-ahead log start point: 0/2000028 on timeline 1
...
pg_basebackup: base backup completed
```

### 7.4 Cấu Hình Standby trên Node 2

**File postgresql.auto.conf đã được tạo tự động, verify:**
```bash
sudo -u postgres cat /var/lib/postgresql/18/main/postgresql.auto.conf
```

Should contain:
```
primary_conninfo = 'host=172.16.91.101 port=5432 user=replicator password=PASSWORD'
primary_slot_name = 'node2_slot'
```

**Nếu cần customize:**
```bash
sudo -u postgres nano /var/lib/postgresql/18/main/postgresql.auto.conf
```

Add/modify:
```
primary_conninfo = 'host=172.16.91.101 port=5432 user=replicator password=ACTUAL_PASSWORD application_name=node2'
primary_slot_name = 'node2_slot'
hot_standby = on
hot_standby_feedback = on
```

**Verify standby.signal exists:**
```bash
ls -la /var/lib/postgresql/18/main/standby.signal
```

### 7.5 Start Standby Server

**Trên Node 2:**

```bash
# Start PostgreSQL
sudo systemctl start postgresql

# Check status
sudo systemctl status postgresql

# Check logs
sudo tail -f /var/log/postgresql/postgresql-*.log
# Should see: "database system is ready to accept read-only connections"
```

### 7.6 Verify Replication

**Trên Node 1 (MASTER):**

```bash
sudo -u postgres psql -c "SELECT * FROM pg_stat_replication;"
```

Expected output:
```
 pid  | usename    | application_name | client_addr   | state     | sync_state
------+------------+------------------+---------------+-----------+------------
 1234 | replicator | node2            | 172.16.91.102 | streaming | async
```

**Trên Node 2 (STANDBY):**

```bash
sudo -u postgres psql -c "SELECT pg_is_in_recovery();"
# Should return: t (true)

sudo -u postgres psql -c "SELECT * FROM pg_stat_wal_receiver;"
```

### 7.7 Test Replication

**Trên Node 1:**

```bash
sudo -u postgres psql -d voipdb <<EOF
CREATE TABLE test_replication (id INT, data TEXT);
INSERT INTO test_replication VALUES (1, 'Test from Node 1');
SELECT * FROM test_replication;
EOF
```

**Trên Node 2:**

```bash
sudo -u postgres psql -d voipdb -c "SELECT * FROM test_replication;"
# Should see: 1 | Test from Node 1
```

**Cleanup test:**
```bash
# Trên Node 1
sudo -u postgres psql -d voipdb -c "DROP TABLE test_replication;"
```

---

## 8. Cài Đặt Kamailio 6.0

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

## 9. Cài Đặt FreeSWITCH 1.10

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

## 10. Cài Đặt VoIP Admin Service

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

## 11. Cấu Hình Keepalived

> **Vai trò:** High Availability Expert

### 11.1 Install Keepalived

**Trên cả 2 nodes:**

```bash
sudo apt install -y keepalived
```

### 11.2 Configure Keepalived MASTER (Node 1)

**Trên Node 1:**

```bash
sudo nano /etc/keepalived/keepalived.conf
```

Content:
```
global_defs {
    router_id VOIP_NODE1
    enable_script_security
    script_user root
}

vrrp_script check_services {
    script "/usr/local/bin/check_voip_services.sh"
    interval 5
    weight -20
    fall 2
    rise 2
}

vrrp_instance VI_1 {
    state MASTER
    interface ens33              # Adjust if different
    virtual_router_id 51
    priority 101                 # MASTER has higher priority
    advert_int 1

    authentication {
        auth_type PASS
        auth_pass VoIPHA2025       # Change this!
    }

    virtual_ipaddress {
        172.16.91.100/24
    }

    track_script {
        check_services
    }

    notify_master "/usr/local/bin/keepalived_notify.sh MASTER"
    notify_backup "/usr/local/bin/keepalived_notify.sh BACKUP"
    notify_fault  "/usr/local/bin/keepalived_notify.sh FAULT"
}
```

### 11.3 Configure Keepalived BACKUP (Node 2)

**Trên Node 2:**

```bash
sudo nano /etc/keepalived/keepalived.conf
```

Content:
```
global_defs {
    router_id VOIP_NODE2
    enable_script_security
    script_user root
}

vrrp_script check_services {
    script "/usr/local/bin/check_voip_services.sh"
    interval 5
    weight -20
    fall 2
    rise 2
}

vrrp_instance VI_1 {
    state BACKUP
    interface ens33              # Adjust if different
    virtual_router_id 51
    priority 100                 # BACKUP has lower priority
    advert_int 1

    authentication {
        auth_type PASS
        auth_pass VoIPHA2025       # Same as MASTER!
    }

    virtual_ipaddress {
        172.16.91.100/24
    }

    track_script {
        check_services
    }

    notify_master "/usr/local/bin/keepalived_notify.sh MASTER"
    notify_backup "/usr/local/bin/keepalived_notify.sh BACKUP"
    notify_fault  "/usr/local/bin/keepalived_notify.sh FAULT"
}
```

### 11.4 Create Health Check Script

**Trên cả 2 nodes:**

```bash
sudo nano /usr/local/bin/check_voip_services.sh
```

Content:
```bash
#!/bin/bash
# Health check script for VoIP services

# Check Kamailio
if ! systemctl is-active --quiet kamailio; then
    exit 1
fi

# Check FreeSWITCH
if ! systemctl is-active --quiet freeswitch; then
    exit 1
fi

# Check VoIP Admin
if ! systemctl is-active --quiet voip-admin; then
    exit 1
fi

# Check PostgreSQL
if ! systemctl is-active --quiet postgresql; then
    exit 1
fi

# All services OK
exit 0
```

```bash
sudo chmod +x /usr/local/bin/check_voip_services.sh
```

### 11.5 Create Notify Script

**Trên cả 2 nodes:**

```bash
sudo nano /usr/local/bin/keepalived_notify.sh
```

Content:
```bash
#!/bin/bash
# Keepalived state change notification

TYPE=$1
NAME=$2
STATE=$3

case $STATE in
    "MASTER")
        echo "$(date) - Becoming MASTER" >> /var/log/keepalived-state.log
        # Add custom actions when becoming MASTER
        # Example: Promote PostgreSQL standby (if needed)
        ;;
    "BACKUP")
        echo "$(date) - Becoming BACKUP" >> /var/log/keepalived-state.log
        # Add custom actions when becoming BACKUP
        ;;
    "FAULT")
        echo "$(date) - FAULT detected" >> /var/log/keepalived-state.log
        # Add alerting here
        ;;
esac

exit 0
```

```bash
sudo chmod +x /usr/local/bin/keepalived_notify.sh
```

### 11.6 Start Keepalived

**Trên cả 2 nodes:**

```bash
# Start Keepalived
sudo systemctl start keepalived
sudo systemctl enable keepalived

# Check status
sudo systemctl status keepalived

# Check logs
sudo tail -f /var/log/syslog | grep -i keepalived
```

### 11.7 Verify VIP

**Trên Node 1:**
```bash
ip addr show ens33 | grep 172.16.91.100
# Should see VIP on Node 1 (MASTER)
```

**Trên Node 2:**
```bash
ip addr show ens33 | grep 172.16.91.100
# Should NOT see VIP (BACKUP)
```

**Test failover:**
```bash
# Stop services on Node 1
sudo systemctl stop kamailio

# Wait 10 seconds, check Node 2
ip addr show ens33 | grep 172.16.91.100
# VIP should move to Node 2

# Restart Kamailio on Node 1
sudo systemctl start kamailio

# VIP should move back to Node 1
```

---

## 12. Tạo Dữ Liệu Mẫu

> **Vai trò:** VoIP System Administrator

### 12.1 Tạo Domain Mẫu

**Trên Node 1:**

```bash
sudo -u postgres psql -d voipdb <<EOF
-- Insert sample domain
INSERT INTO voip.domains (domain, description, active)
VALUES ('example.com', 'Default domain for testing', true);

-- Verify
SELECT * FROM voip.domains;
EOF
```

### 12.2 Tạo Extensions Mẫu

**Trên Node 1:**

```bash
sudo -u postgres psql -d voipdb <<EOF
-- Get domain_id
\set domain_id 1

-- Create 10 sample extensions (1000-1009)
INSERT INTO voip.extensions (domain_id, extension, type, display_name, email, sip_password, vm_password, vm_email, active, max_concurrent, call_timeout)
VALUES
    (1, '1000', 'user', 'Alice Johnson', 'alice@example.com', 'SecurePass1000', '1234', 'alice@example.com', true, 3, 30),
    (1, '1001', 'user', 'Bob Smith', 'bob@example.com', 'SecurePass1001', '1234', 'bob@example.com', true, 3, 30),
    (1, '1002', 'user', 'Charlie Brown', 'charlie@example.com', 'SecurePass1002', '1234', 'charlie@example.com', true, 3, 30),
    (1, '1003', 'user', 'Diana Prince', 'diana@example.com', 'SecurePass1003', '1234', 'diana@example.com', true, 3, 30),
    (1, '1004', 'user', 'Eve Davis', 'eve@example.com', 'SecurePass1004', '1234', 'eve@example.com', true, 3, 30),
    (1, '1005', 'user', 'Frank Miller', 'frank@example.com', 'SecurePass1005', '1234', 'frank@example.com', true, 3, 30),
    (1, '1006', 'user', 'Grace Lee', 'grace@example.com', 'SecurePass1006', '1234', 'grace@example.com', true, 3, 30),
    (1, '1007', 'user', 'Henry Wilson', 'henry@example.com', 'SecurePass1007', '1234', 'henry@example.com', true, 3, 30),
    (1, '1008', 'user', 'Iris Taylor', 'iris@example.com', 'SecurePass1008', '1234', 'iris@example.com', true, 3, 30),
    (1, '1009', 'user', 'Jack Anderson', 'jack@example.com', 'SecurePass1009', '1234', 'jack@example.com', true, 3, 30);

-- Verify extensions and HA1 hashes
SELECT extension, display_name, sip_ha1 IS NOT NULL as has_ha1, active
FROM voip.extensions
ORDER BY extension;
EOF
```

**Trigger `extensions_calc_ha1_trigger` sẽ tự động tính HA1/HA1B.**

### 12.3 Verify Kamailio View

**Trên Node 1:**

```bash
sudo -u postgres psql -d voipdb <<EOF
-- Check subscriber view (used by Kamailio)
SELECT username, domain, ha1 IS NOT NULL as has_ha1
FROM kamailio.subscriber
ORDER BY username;
EOF
```

Expected output:
```
 username | domain      | has_ha1
----------+-------------+---------
 1000     | example.com | t
 1001     | example.com | t
 ...
```

### 12.4 Tạo Queue Mẫu

**Trên Node 1:**

```bash
sudo -u postgres psql -d voipdb <<EOF
-- Create sample queue
INSERT INTO voip.queues (name, strategy, moh, timeout, max_wait_time_no_agent, max_wait_time_no_agent_time_reached, tier_rules_apply, tier_rule_wait_second, tier_rule_no_agent_no_wait, discard_abandoned_after, abandoned_resume_allowed, record_template, active)
VALUES
    ('8000', 'longest-idle-agent', 'default', 60, 300, 5, true, 30, true, 60, false, '/recordings/freeswitch/queue-${strftime(%Y-%m-%d-%H-%M-%S)}.wav', true);

-- Create queue agents
-- Assign extensions 1000-1004 to queue 8000
INSERT INTO voip.queue_agents (queue_id, extension_id, tier, position, active, state, status)
SELECT
    (SELECT id FROM voip.queues WHERE name = '8000'),
    e.id,
    1,
    ROW_NUMBER() OVER (ORDER BY e.extension),
    true,
    'Available',
    'Waiting'
FROM voip.extensions e
WHERE e.extension IN ('1000', '1001', '1002', '1003', '1004');

-- Verify queue
SELECT q.name, e.extension, qa.tier, qa.position, qa.state
FROM voip.queue_agents qa
JOIN voip.queues q ON qa.queue_id = q.id
JOIN voip.extensions e ON qa.extension_id = e.id
ORDER BY qa.position;
EOF
```

---

## 13. Kiểm Thử Tích Hợp

> **Vai trò:** QA Engineer & VoIP Expert

### 13.1 Test 1: Database Connectivity

**Test từ Node 1:**
```bash
# Kamailio user
PGPASSWORD=$(grep kamailio_db_password /root/.voip_credentials | cut -d'=' -f2) \
    psql -h 172.16.91.101 -U kamailio -d voipdb -c "SELECT COUNT(*) FROM kamailio.subscriber;"

# VoIP Admin user
PGPASSWORD=$(grep voipadmin_db_password /root/.voip_credentials | cut -d'=' -f2) \
    psql -h 172.16.91.101 -U voip_admin -d voipdb -c "SELECT COUNT(*) FROM voip.extensions;"
```

### 13.2 Test 2: VoIP Admin API

**Trên Node 1:**

```bash
# Get API key
API_KEY=$(grep voipadmin_api_key /root/.voip_credentials | cut -d'=' -f2)

# Test health
curl http://172.16.91.100:8080/health

# List extensions
curl -H "X-API-Key: $API_KEY" \
    http://172.16.91.100:8080/api/v1/extensions | jq

# Create new extension
curl -X POST http://172.16.91.100:8080/api/v1/extensions \
    -H "X-API-Key: $API_KEY" \
    -H "Content-Type: application/json" \
    -d '{
        "domain_id": 1,
        "extension": "1100",
        "type": "user",
        "display_name": "Test User",
        "email": "test@example.com",
        "sip_password": "TestPass123",
        "active": true
    }' | jq

# Verify HA1 was calculated
sudo -u postgres psql -d voipdb -c "SELECT extension, sip_ha1, sip_ha1b FROM voip.extensions WHERE extension = '1100';"
```

### 13.3 Test 3: FreeSWITCH XML_CURL Directory

**Trên Node 1:**

```bash
# Get FreeSWITCH password
FREESWITCH_PASS=$(grep freeswitch_password /root/.voip_credentials | cut -d'=' -f2)

# Test directory lookup
curl -X POST http://172.16.91.100:8080/freeswitch/directory \
    -u "freeswitch:$FREESWITCH_PASS" \
    -d "user=1000&domain=example.com"

# Should return XML with user config
```

Expected output:
```xml
<?xml version="1.0"?>
<document type="freeswitch/xml">
  <section name="directory">
    <domain name="example.com">
      <user id="1000">
        <params>
          <param name="a1-hash" value="..."/>
          ...
        </params>
      </user>
    </domain>
  </section>
</document>
```

### 13.4 Test 4: FreeSWITCH Dialplan

```bash
curl -X POST http://172.16.91.100:8080/freeswitch/dialplan \
    -u "freeswitch:$FREESWITCH_PASS" \
    -d "Caller-Destination-Number=1001&Hunt-Destination-Number=1001"

# Should return dialplan XML
```

### 13.5 Test 5: SIP Phone Registration

**Sử dụng SIP softphone (Zoiper, Linphone, X-Lite, etc.):**

**Configuration:**
- Username: `1000`
- Password: `SecurePass1000`
- Domain: `example.com`
- Server: `172.16.91.100`
- Port: `5060`
- Transport: `UDP`

**Đăng ký và kiểm tra:**

**Từ Node 1:**
```bash
# Check Kamailio location
sudo kamctl ul show

# Should see:
# Contact:: <sip:1000@CLIENT_IP:PORT> ...

# Check FreeSWITCH sofia status
sudo fs_cli -x "sofia status profile internal reg"
```

### 13.6 Test 6: Test Call Between Extensions

**Scenario:**
1. Đăng ký 2 SIP phones:
   - Phone A: Extension 1000
   - Phone B: Extension 1001

2. Từ Phone A, gọi 1001

3. Phone B nên rung chuông

4. Trả lời cuộc gọi

5. Kiểm tra audio 2 chiều

6. Kết thúc cuộc gọi

**Verify call flow:**

```bash
# Kamailio logs
sudo tail -f /var/log/kamailio.log | grep INVITE

# FreeSWITCH logs
sudo fs_cli
# Inside fs_cli:
sofia status
sofia global siptrace on
# Make call, observe SIP messages

# VoIP Admin CDR
journalctl -u voip-admin -f | grep CDR
```

**After call ends:**
```bash
# Check CDR queue
sudo -u postgres psql -d voipdb -c "SELECT COUNT(*) FROM voip.cdr_queue WHERE received_at > NOW() - INTERVAL '5 minutes';"

# Wait for worker to process (5 seconds)
sleep 10

# Check final CDR table
sudo -u postgres psql -d voipdb <<EOF
SELECT uuid, caller_id_number, destination_number, duration, hangup_cause
FROM voip.cdr
WHERE start_time > NOW() - INTERVAL '10 minutes'
ORDER BY start_time DESC
LIMIT 5;
EOF
```

### 13.7 Test 7: Queue Call

**Setup:**
1. Đăng ký agent extensions: 1000, 1001, 1002

2. Gọi queue: `8000`

3. Agent đầu tiên rung chuông (longest-idle-agent strategy)

4. Agent trả lời

5. Verify queue stats

```bash
sudo -u postgres psql -d voipdb <<EOF
SELECT
    caller_id_number,
    destination_number,
    call_type,
    queue_wait_time,
    agent_extension,
    duration
FROM voip.cdr
WHERE queue_id = (SELECT id FROM voip.queues WHERE name = '8000')
ORDER BY start_time DESC
LIMIT 5;
EOF
```

### 13.8 Test 8: HA Failover

**Test VIP failover:**

```bash
# Check current VIP owner
# On Node 1:
ip addr show ens33 | grep 172.16.91.100

# Simulate Node 1 failure
sudo systemctl stop keepalived

# Wait 5 seconds

# On Node 2, check VIP:
ip addr show ens33 | grep 172.16.91.100
# VIP should be on Node 2 now

# Test SIP registration still works
# Register phone to 172.16.91.100 (should route to Node 2)

# Restore Node 1
sudo systemctl start keepalived
```

### 13.9 Test 9: Database Replication Lag

```bash
# On Node 1 (MASTER):
sudo -u postgres psql -c "SELECT pg_current_wal_lsn();"

# On Node 2 (STANDBY):
sudo -u postgres psql -c "SELECT pg_last_wal_receive_lsn(), pg_last_wal_replay_lsn();"

# Calculate lag
sudo -u postgres psql <<EOF
SELECT
    pg_size_pretty(pg_wal_lsn_diff(pg_last_wal_receive_lsn(), pg_last_wal_replay_lsn())) as replication_lag;
EOF
# Should be close to 0 bytes
```

### 13.10 Test 10: Performance Under Load

**Using SIPp (SIP Performance Tool):**

```bash
# Install SIPp
sudo apt install -y sipp

# Basic call load test
sipp -sn uac \
    -s 1001 \
    -r 10 \
    -l 100 \
    -m 1000 \
    172.16.91.100:5060

# -r 10: 10 calls per second
# -l 100: 100 concurrent calls
# -m 1000: total 1000 calls
```

**Monitor during test:**
```bash
# Kamailio stats
sudo kamctl stats

# FreeSWITCH stats
sudo fs_cli -x "show channels"
sudo fs_cli -x "status"

# Database connections
sudo -u postgres psql -c "SELECT count(*) FROM pg_stat_activity WHERE datname = 'voipdb';"

# System load
htop
iotop
```

---

## 14. Truy Cập Web Interface

> **Vai trò:** Web Developer & End User

**Lưu ý:** VoIP Admin hiện tại chỉ là REST API backend. Web interface cần phát triển riêng.

### 14.1 Sử dụng Postman/Insomnia

**Setup Collection:**

1. Install Postman: https://www.postman.com/downloads/

2. Create new collection: "VoIP Admin API"

3. Add environment variables:
   - `base_url`: `http://172.16.91.100:8080`
   - `api_key`: `[YOUR_API_KEY từ /root/.voip_credentials]`

4. Create requests:

**GET Health Check:**
```
GET {{base_url}}/health
```

**GET Health Stats:**
```
GET {{base_url}}/health/stats
```

**GET List Extensions:**
```
GET {{base_url}}/api/v1/extensions
Headers:
  X-API-Key: {{api_key}}
```

**POST Create Extension:**
```
POST {{base_url}}/api/v1/extensions
Headers:
  X-API-Key: {{api_key}}
  Content-Type: application/json
Body:
{
  "domain_id": 1,
  "extension": "2000",
  "type": "user",
  "display_name": "John Doe",
  "email": "john@example.com",
  "sip_password": "SecurePassword123",
  "vm_password": "1234",
  "active": true,
  "max_concurrent": 3,
  "call_timeout": 30
}
```

**GET Extension Detail:**
```
GET {{base_url}}/api/v1/extensions/{id}
Headers:
  X-API-Key: {{api_key}}
```

**PUT Update Extension:**
```
PUT {{base_url}}/api/v1/extensions/{id}
Headers:
  X-API-Key: {{api_key}}
  Content-Type: application/json
Body:
{
  "display_name": "John Smith",
  "email": "johnsmith@example.com"
}
```

**POST Update Password:**
```
POST {{base_url}}/api/v1/extensions/{id}/password
Headers:
  X-API-Key: {{api_key}}
  Content-Type: application/json
Body:
{
  "password": "NewSecurePassword456"
}
```

**GET List CDRs:**
```
GET {{base_url}}/api/v1/cdr?page=1&per_page=50&start_date=2025-01-01T00:00:00Z
Headers:
  X-API-Key: {{api_key}}
```

**GET CDR Stats:**
```
GET {{base_url}}/api/v1/cdr/stats?start_date=2025-01-01T00:00:00Z&end_date=2025-01-31T23:59:59Z
Headers:
  X-API-Key: {{api_key}}
```

### 14.2 Web UI Development (Future)

**Stack đề xuất:**
- Frontend: React/Vue.js + TailwindCSS
- State Management: Redux/Vuex
- HTTP Client: Axios
- Charts: Chart.js / Recharts
- Real-time: WebSocket (cần implement trong Go backend)

**Features cần có:**
1. Dashboard:
   - Active calls
   - System health
   - CDR statistics (charts)
   - Queue performance

2. Extension Management:
   - List/Create/Update/Delete
   - Bulk import from CSV
   - Password reset

3. CDR Reports:
   - Search & filters
   - Export to CSV/Excel
   - Date range selector
   - Call playback (if recordings enabled)

4. Queue Management:
   - Queue configuration
   - Agent assignment
   - Real-time queue stats

5. System Monitoring:
   - Service status
   - Database replication lag
   - Cache performance
   - Alert notifications

---

## 15. Monitoring và Maintenance

> **Vai trò:** DevOps & SRE Engineer

### 15.1 Log Management

**Centralized Logging Locations:**

```bash
# Kamailio
/var/log/kamailio.log

# FreeSWITCH
/var/log/freeswitch/freeswitch.log

# VoIP Admin
journalctl -u voip-admin

# PostgreSQL
/var/log/postgresql/postgresql-*.log

# Keepalived
/var/log/syslog (filter: keepalived)
```

**Setup logrotate:**
```bash
sudo nano /etc/logrotate.d/voip-system
```

Content:
```
/var/log/kamailio.log {
    daily
    rotate 30
    compress
    delaycompress
    notifempty
    create 0640 kamailio kamailio
    sharedscripts
    postrotate
        /bin/kill -HUP `cat /var/run/kamailio/kamailio.pid 2>/dev/null` 2>/dev/null || true
    endscript
}

/var/log/freeswitch/*.log {
    daily
    rotate 30
    compress
    delaycompress
    notifempty
    create 0640 freeswitch freeswitch
    sharedscripts
    postrotate
        /usr/bin/fs_cli -x "fsctl send_sighup" >/dev/null 2>&1 || true
    endscript
}
```

### 15.2 Monitoring Scripts

**Create monitoring script:**
```bash
sudo nano /usr/local/bin/voip_monitor.sh
```

Content:
```bash
#!/bin/bash
# VoIP System Monitoring Script

LOGFILE="/var/log/voip-monitor.log"
ALERT_EMAIL="admin@example.com"

log_message() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') - $1" >> $LOGFILE
}

check_service() {
    SERVICE=$1
    if ! systemctl is-active --quiet $SERVICE; then
        log_message "ALERT: $SERVICE is DOWN"
        echo "$SERVICE is DOWN on $(hostname)" | mail -s "VoIP Alert: $SERVICE Down" $ALERT_EMAIL
        return 1
    fi
    return 0
}

# Check all services
check_service postgresql
check_service kamailio
check_service freeswitch
check_service voip-admin
check_service keepalived

# Check database connections
DB_CONNS=$(sudo -u postgres psql -t -c "SELECT count(*) FROM pg_stat_activity WHERE datname = 'voipdb';")
if [ $DB_CONNS -gt 250 ]; then
    log_message "WARNING: High database connections: $DB_CONNS"
fi

# Check replication lag (on STANDBY)
if sudo -u postgres psql -t -c "SELECT pg_is_in_recovery();" | grep -q "t"; then
    LAG=$(sudo -u postgres psql -t -c "SELECT pg_wal_lsn_diff(pg_last_wal_receive_lsn(), pg_last_wal_replay_lsn());")
    if [ $LAG -gt 10485760 ]; then  # 10MB
        log_message "WARNING: Replication lag is ${LAG} bytes"
    fi
fi

# Check disk space
DISK_USAGE=$(df -h / | awk 'NR==2 {print $5}' | sed 's/%//')
if [ $DISK_USAGE -gt 90 ]; then
    log_message "ALERT: Disk usage is ${DISK_USAGE}%"
    echo "Disk usage is ${DISK_USAGE}% on $(hostname)" | mail -s "VoIP Alert: Disk Full" $ALERT_EMAIL
fi

log_message "Monitoring check completed"
```

```bash
sudo chmod +x /usr/local/bin/voip_monitor.sh
```

**Add to cron:**
```bash
crontab -e
```

Add:
```
*/5 * * * * /usr/local/bin/voip_monitor.sh
```

### 15.3 Database Maintenance

**Daily vacuum:**
```bash
sudo nano /usr/local/bin/pg_vacuum.sh
```

Content:
```bash
#!/bin/bash
sudo -u postgres vacuumdb -d voipdb --analyze --verbose >> /var/log/postgresql/vacuum.log 2>&1
```

```bash
sudo chmod +x /usr/local/bin/pg_vacuum.sh

# Add to cron (run at 2 AM)
echo "0 2 * * * /usr/local/bin/pg_vacuum.sh" | sudo crontab -
```

**Weekly cleanup old CDRs:**
```bash
sudo -u postgres psql -d voipdb <<EOF
-- Cleanup CDR queue older than 7 days
SELECT voip.cleanup_old_cdr_queue(7);

-- Archive old CDRs (older than 90 days) to separate table
CREATE TABLE IF NOT EXISTS voip.cdr_archive (LIKE voip.cdr INCLUDING ALL);

INSERT INTO voip.cdr_archive
SELECT * FROM voip.cdr
WHERE start_time < NOW() - INTERVAL '90 days';

DELETE FROM voip.cdr
WHERE start_time < NOW() - INTERVAL '90 days';
EOF
```

### 15.4 Backup Strategy

**Database backup script:**
```bash
sudo nano /usr/local/bin/backup_voipdb.sh
```

Content:
```bash
#!/bin/bash
BACKUP_DIR="/backup/postgresql"
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="$BACKUP_DIR/voipdb_$DATE.sql.gz"

mkdir -p $BACKUP_DIR

# Full backup
sudo -u postgres pg_dump voipdb | gzip > $BACKUP_FILE

# Keep only last 30 days
find $BACKUP_DIR -name "voipdb_*.sql.gz" -mtime +30 -delete

echo "Backup completed: $BACKUP_FILE"
```

```bash
sudo chmod +x /usr/local/bin/backup_voipdb.sh

# Run daily at 1 AM
echo "0 1 * * * /usr/local/bin/backup_voipdb.sh" | sudo crontab -
```

### 15.5 Performance Monitoring

**Install monitoring tools:**
```bash
sudo apt install -y prometheus-node-exporter
sudo systemctl enable prometheus-node-exporter
sudo systemctl start prometheus-node-exporter
```

**Monitor key metrics:**
```bash
# Real-time monitoring
watch -n 5 'sudo fs_cli -x "show channels count"'
watch -n 5 'sudo kamctl stats | grep "active_"'
watch -n 5 'curl -s http://localhost:8080/health/stats | jq'
```

### 15.6 Alert Setup

**Install mailutils:**
```bash
sudo apt install -y mailutils postfix
# Configure postfix as Internet Site
```

**Test email:**
```bash
echo "Test alert from VoIP system" | mail -s "Test Alert" admin@example.com
```

---

## 📋 Deployment Summary Checklist

**System Preparation:**
- [x] Hardware verified (16 cores, 64GB RAM, 500GB SSD + 3TB HDD)
- [x] Debian 12 installed on both nodes
- [x] Network configured (static IPs, hostname, /etc/hosts)
- [x] SSH keys exchanged between nodes
- [x] System updated and rebooted

**OS Tuning:**
- [x] Kernel parameters configured (/etc/sysctl.d/99-voip-tuning.conf)
- [x] System limits set (/etc/security/limits.conf)
- [x] Transparent Huge Pages disabled
- [x] NTP/Chrony configured and synced

**Firewall:**
- [x] UFW configured with required ports
- [x] Firewall rules tested between nodes
- [x] Fail2ban configured (optional)

**PostgreSQL:**
- [x] PostgreSQL 18 installed
- [x] Performance tuning applied
- [x] Database users created with strong passwords
- [x] Database schemas loaded (01, 02, 03, 04)
- [x] Permissions granted to users
- [x] Replication configured and verified
- [x] Node 1 is MASTER, Node 2 is STANDBY

**Kamailio:**
- [x] Kamailio 6.0 installed
- [x] Config deployed and customized
- [x] Database connection tested
- [x] Dispatcher table populated
- [x] Service started and verified

**FreeSWITCH:**
- [x] FreeSWITCH 1.10 installed
- [x] Config files deployed
- [x] XML_CURL endpoint configured
- [x] XML_CDR endpoint configured
- [x] Service started, sofia profile UP

**VoIP Admin:**
- [x] Go 1.23 installed
- [x] Binary built and deployed
- [x] Config file created with passwords
- [x] systemd service created
- [x] Service started, health check OK

**Keepalived:**
- [x] Keepalived installed
- [x] Config for MASTER and BACKUP
- [x] Health check scripts created
- [x] VIP verified on MASTER
- [x] Failover tested

**Sample Data:**
- [x] Domain created (example.com)
- [x] 10 extensions created (1000-1009)
- [x] Queue created (8000)
- [x] Queue agents assigned

**Testing:**
- [x] Database connectivity tested
- [x] VoIP Admin API tested
- [x] FreeSWITCH directory lookup tested
- [x] SIP phone registration tested
- [x] Test call between extensions successful
- [x] CDR processing verified
- [x] Queue call tested
- [x] HA failover tested
- [x] Replication lag checked

**Monitoring:**
- [x] Log rotation configured
- [x] Monitoring scripts deployed
- [x] Cron jobs scheduled
- [x] Backup strategy implemented
- [x] Alert system configured

---

## 📞 Support & Troubleshooting

### Common Issues

**Issue 1: Kamailio cannot connect to database**
```bash
# Check database service
sudo systemctl status postgresql

# Check database password
grep kamailio_db_password /root/.voip_credentials

# Test connection manually
PGPASSWORD=<password> psql -h 172.16.91.101 -U kamailio -d voipdb -c "SELECT 1;"
```

**Issue 2: FreeSWITCH directory lookup fails**
```bash
# Check VoIP Admin service
sudo systemctl status voip-admin

# Check logs
journalctl -u voip-admin -n 50

# Test endpoint manually
curl -X POST http://172.16.91.100:8080/freeswitch/directory \
    -u "freeswitch:PASSWORD" \
    -d "user=1000&domain=example.com"
```

**Issue 3: No CDR in database**
```bash
# Check xml_cdr.conf.xml exists
ls -la /etc/freeswitch/autoload_configs/xml_cdr.conf.xml

# Reload module
sudo fs_cli -x "reload mod_xml_cdr"

# Check VoIP Admin CDR endpoint
curl -X POST http://172.16.91.100:8080/api/v1/cdr \
    -u "freeswitch:PASSWORD" \
    -H "Content-Type: application/xml" \
    -d '<cdr><variables><uuid>test-uuid</uuid></variables></cdr>'

# Check cdr_queue table
sudo -u postgres psql -d voipdb -c "SELECT COUNT(*) FROM voip.cdr_queue;"
```

**Issue 4: VIP not working**
```bash
# Check Keepalived status
sudo systemctl status keepalived

# Check VRRP packets
sudo tcpdump -i ens33 -n proto 112

# Check priority
sudo cat /etc/keepalived/keepalived.conf | grep priority

# Check logs
sudo tail -f /var/log/syslog | grep keepalived
```

---

## 📚 Additional Resources

- **PostgreSQL 18 Documentation:** https://www.postgresql.org/docs/18/
- **Kamailio Documentation:** https://www.kamailio.org/wiki/
- **FreeSWITCH Documentation:** https://freeswitch.org/confluence/
- **Go Documentation:** https://go.dev/doc/
- **Keepalived Documentation:** https://www.keepalived.org/doc/

---

**Tài liệu này được tạo:** 2025-01-18
**Phiên bản:** 1.0
**Tác giả:** Multi-Expert Deployment Team
**License:** Internal Use Only

---

**🎉 Chúc mừng! Hệ thống VoIP HA của bạn đã sẵn sàng hoạt động!**
