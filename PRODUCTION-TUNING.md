# Tối Ưu Hóa Production cho 600-800 Cuộc Gọi Đồng Thời

Tài liệu này mô tả chi tiết các cấu hình tối ưu hóa cho môi trường production với 600-800 concurrent calls.

---

## 1. PostgreSQL Authentication

### Vấn Đề Tương Thích SCRAM-SHA-256

**Kamailio với PostgreSQL:**
- Module `db_postgres` hỗ trợ SCRAM từ version 5.5+ nhưng có vấn đề performance
- SCRAM handshake phức tạp hơn MD5, thêm 5-10ms latency mỗi query
- Với hàng trăm REGISTER/giây, latency tích lũy đáng kể

**FreeSWITCH ODBC:**
- psqlODBC driver < 13.x không hỗ trợ đầy đủ SCRAM-SHA-256
- Lỗi phổ biến: "authentication method not supported"
- Production thường dùng MD5 hoặc trust cho local

### Cấu Hình Khuyến Nghị

File: `configs/postgresql/pg_hba.conf` đã được cập nhật:

```conf
# Kamailio - MD5 cho performance
host    kamailio        kamailio        172.16.91.101/32        md5
host    kamailio        kamailio        172.16.91.102/32        md5
host    kamailio        kamailio        172.16.91.100/32        md5

# FreeSWITCH - MD5 cho tương thích
host    voip            freeswitch      172.16.91.101/32        md5
host    voip            freeswitch      172.16.91.102/32        md5
host    voip            freeswitch      172.16.91.100/32        md5

# Replication - SCRAM cho security (ít connection)
host    replication     replicator      172.16.91.101/32        scram-sha-256
host    replication     replicator      172.16.91.102/32        scram-sha-256

# VoIP Admin - SCRAM OK (Go driver hỗ trợ tốt)
host    voip            voipadmin       172.16.91.101/32        scram-sha-256
host    voip            voipadmin       172.16.91.102/32        scram-sha-256
```

**Lý do phân tầng:**
- **Kamailio/FreeSWITCH**: Hàng trăm queries/giây → Ưu tiên performance
- **Replication**: 1-2 connections → Ưu tiên security
- **VoIP Admin**: Modern driver, ít queries → SCRAM OK

---

## 2. QoS/DSCP Marking

### DSCP Values Chuẩn

| Traffic Type | DSCP | Decimal | Binary | Mô Tả |
|--------------|------|---------|--------|-------|
| RTP (Voice) | EF | 46 | 101110 | Expedited Forwarding |
| SIP Signaling | CS3 | 24 | 011000 | Class Selector 3 |
| SIP (Alt) | AF31 | 26 | 011010 | Assured Forwarding 31 |

### Cấu Hình Từng Tầng

#### A. Linux Kernel (Trên Node VoIP)

Tạo file: `/etc/sysctl.d/90-voip-qos.conf`

```bash
# Cho phép DSCP marking
net.ipv4.ip_forward = 1
net.ipv4.tcp_ecn = 0
```

Tạo file: `/etc/voip-qos-rules.sh`

```bash
#!/bin/bash
# DSCP Marking cho VoIP Traffic

# FreeSWITCH RTP packets (EF - Priority cao nhất)
iptables -t mangle -A OUTPUT -p udp --dport 16384:32768 -j DSCP --set-dscp-class ef
iptables -t mangle -A OUTPUT -p udp --sport 16384:32768 -j DSCP --set-dscp-class ef

# Kamailio SIP packets (CS3)
iptables -t mangle -A OUTPUT -p udp --dport 5060 -j DSCP --set-dscp-class cs3
iptables -t mangle -A OUTPUT -p tcp --dport 5060 -j DSCP --set-dscp-class cs3
iptables -t mangle -A OUTPUT -p udp --sport 5060 -j DSCP --set-dscp-class cs3
iptables -t mangle -A OUTPUT -p tcp --sport 5060 -j DSCP --set-dscp-class cs3

echo "✓ DSCP marking configured"
```

Kích hoạt:
```bash
chmod +x /etc/voip-qos-rules.sh
/etc/voip-qos-rules.sh

# Tự động load khi boot
echo "/etc/voip-qos-rules.sh" >> /etc/rc.local
```

#### B. FreeSWITCH Configuration

File: `configs/freeswitch/autoload_configs/sofia.conf.xml`

Thêm vào profile `internal`:
```xml
<param name="rtp-tos" value="184"/>  <!-- EF = 46 × 4 = 184 -->
<param name="sip-tos" value="96"/>   <!-- CS3 = 24 × 4 = 96 -->
```

**Giải thích**: FreeSWITCH dùng IP TOS byte (× 4), không phải DSCP trực tiếp.

#### C. Switch/Router Configuration

**Cisco IOS Example:**

```cisco
! Define class-maps
class-map match-any VOIP-RTP
  match ip dscp ef

class-map match-any VOIP-SIP
  match ip dscp cs3

! Define policy-map
policy-map VOIP-QOS-OUT
  class VOIP-RTP
    priority percent 40      ! 40% bandwidth cho RTP
    set dscp ef
  class VOIP-SIP
    bandwidth percent 10     ! 10% cho SIP
    set dscp cs3
  class class-default
    fair-queue
    random-detect

! Apply to WAN interface
interface GigabitEthernet0/1
  description WAN-Link
  service-policy output VOIP-QOS-OUT
```

**Juniper JunOS Example:**

```junos
firewall {
    family inet {
        filter voip-qos-marking {
            term mark-rtp {
                from {
                    protocol udp;
                    port 16384-32768;
                }
                then {
                    dscp ef;
                    accept;
                }
            }
            term mark-sip {
                from {
                    protocol udp;
                    port 5060;
                }
                then {
                    dscp cs3;
                    accept;
                }
            }
            term default {
                then accept;
            }
        }
    }
}

class-of-service {
    interfaces {
        ge-0/0/0 {
            scheduler-map voip-scheduler;
        }
    }
    scheduler-maps {
        voip-scheduler {
            forwarding-class expedited-forwarding scheduler ef-sched;
            forwarding-class network-control scheduler cs3-sched;
        }
    }
    schedulers {
        ef-sched {
            transmit-rate percent 40;
            priority strict-high;
        }
        cs3-sched {
            transmit-rate percent 10;
            priority medium-high;
        }
    }
}
```

#### D. Verification

Script kiểm tra DSCP marking:

```bash
#!/bin/bash
# verify-qos.sh

echo "=== Kiểm Tra DSCP Marking ==="

# Capture 10 packets SIP
echo "1. Capturing SIP packets..."
timeout 5 tcpdump -i any -c 10 -vv 'udp port 5060' 2>/dev/null | grep -i tos

# Capture 10 packets RTP
echo "2. Capturing RTP packets..."
timeout 5 tcpdump -i any -c 10 -vv 'udp portrange 16384-32768' 2>/dev/null | grep -i tos

# Kiểm tra iptables rules
echo "3. Checking iptables mangle rules..."
iptables -t mangle -L OUTPUT -v -n | grep -E "DSCP|5060|16384"

echo "Hoàn tất!"
```

### Lưu Ý Quan Trọng với IDC/DataCenter

**Trước khi triển khai production:**

1. **Confirm với NOC/Network team**:
   - DSCP marking có được preserve không?
   - Có bị remarking tại edge không?
   - QoS policy của IDC như thế nào?

2. **Test end-to-end**:
   - Gửi packet từ client → server → verify DSCP
   - Kiểm tra cả 2 chiều
   - So sánh packet loss/jitter với và không QoS

3. **SLA Requirements**:
   - Bandwidth commit cho VoIP traffic
   - Packet loss < 1%
   - Jitter < 30ms
   - Latency < 150ms

4. **Fallback Plan**:
   - Nếu QoS bị strip, tăng bandwidth tổng
   - Monitor và alert khi quality degradation

---

## 3. Kernel Network Tuning

### Tạo File Cấu Hình

File: `/etc/sysctl.d/90-voip-network.conf`

```bash
# ============================================================
# VoIP Production Network Tuning - 600-800 Concurrent Calls
# ============================================================

# === Network Buffers ===
# 128 MB cho receive/send buffer max
net.core.rmem_max = 134217728
net.core.wmem_max = 134217728
net.core.rmem_default = 33554432
net.core.wmem_default = 33554432

# TCP buffers (cho SIP over TCP)
net.ipv4.tcp_rmem = 4096 87380 134217728
net.ipv4.tcp_wmem = 4096 65536 134217728
net.ipv4.tcp_mem = 134217728 134217728 134217728

# UDP buffers (cho RTP - quan trọng!)
net.ipv4.udp_rmem_min = 16384
net.ipv4.udp_wmem_min = 16384

# === Connection Tracking ===
# 800 calls × 2 nodes × 5 avg connections = 8000
# Set 262k để an toàn
net.netfilter.nf_conntrack_max = 262144
net.netfilter.nf_conntrack_tcp_timeout_established = 1200
net.netfilter.nf_conntrack_udp_timeout = 180
net.netfilter.nf_conntrack_udp_timeout_stream = 180

# === Socket & Backlog ===
net.core.netdev_max_backlog = 30000
net.core.somaxconn = 8192
net.ipv4.tcp_max_syn_backlog = 8192

# === TCP Optimization ===
# Disable unnecessary features cho performance
net.ipv4.tcp_timestamps = 0
net.ipv4.tcp_sack = 0
net.ipv4.tcp_fack = 0

# Fast connection recycling (cẩn thận với NAT)
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_fin_timeout = 15

# Congestion control
net.ipv4.tcp_congestion_control = cubic

# === IP Forwarding ===
net.ipv4.ip_forward = 1
net.ipv4.conf.all.forwarding = 1

# === ARP Cache ===
net.ipv4.neigh.default.gc_thresh1 = 1024
net.ipv4.neigh.default.gc_thresh2 = 4096
net.ipv4.neigh.default.gc_thresh3 = 8192
```

**Áp dụng**:
```bash
sysctl -p /etc/sysctl.d/90-voip-network.conf
```

---

## 4. NIC Tuning

### Ring Buffer & Interrupt Coalescing

Tạo script: `/usr/local/bin/tune-nic.sh`

```bash
#!/bin/bash
# NIC Performance Tuning cho VoIP

NIC="ens33"  # Thay đổi theo interface thực tế

echo "Tuning NIC: $NIC"

# === Ring Buffer Size ===
# Tăng ring buffer (cần NIC hỗ trợ)
echo "Setting ring buffer..."
ethtool -G $NIC rx 4096 tx 4096 2>/dev/null || echo "Ring buffer not supported"

# === Interrupt Coalescing ===
# Giảm interrupt rate
echo "Setting interrupt coalescing..."
ethtool -C $NIC rx-usecs 50 tx-usecs 50 2>/dev/null || echo "Coalescing not supported"

# === Offloading ===
# GRO/GSO/TSO có thể gây latency, disable khi testing
# Enable lại sau khi stable để tăng throughput
echo "Configuring offloading..."
ethtool -K $NIC gro on
ethtool -K $NIC gso on
ethtool -K $NIC tso on

# === Multi-Queue (nếu NIC hỗ trợ) ===
# Match với số CPU cores
echo "Setting multi-queue..."
ethtool -L $NIC combined 8 2>/dev/null || echo "Multi-queue not supported"

# === RSS (Receive Side Scaling) ===
echo "Enabling RSS..."
ethtool -K $NIC rx on 2>/dev/null

echo "✓ NIC tuning completed for $NIC"
```

**Kích hoạt**:
```bash
chmod +x /usr/local/bin/tune-nic.sh
/usr/local/bin/tune-nic.sh

# Auto-run on boot
echo "/usr/local/bin/tune-nic.sh" >> /etc/rc.local
```

---

## 5. IRQ Affinity (CPU Pinning)

Tạo script: `/usr/local/bin/set-irq-affinity.sh`

```bash
#!/bin/bash
# IRQ Affinity cho NIC - Bind interrupts to specific CPUs
# Tránh CPU 0 (kernel tasks)

NIC="ens33"
CPUS="1,2,3,4"  # CPUs dành cho network processing

echo "Setting IRQ affinity for $NIC to CPUs: $CPUS"

# === RPS (Receive Packet Steering) ===
for i in /sys/class/net/$NIC/queues/rx-*/rps_cpus; do
    if [ -f "$i" ]; then
        echo $CPUS > $i
        echo "RPS: $i → $CPUS"
    fi
done

# === XPS (Transmit Packet Steering) ===
for i in /sys/class/net/$NIC/queues/tx-*/xps_cpus; do
    if [ -f "$i" ]; then
        echo $CPUS > $i
        echo "XPS: $i → $CPUS"
    fi
done

# === IRQ Balancing (optional) ===
# Disable irqbalance để manual control
# systemctl stop irqbalance
# systemctl disable irqbalance

echo "✓ IRQ affinity configured"
```

---

## 6. Application-Specific Tuning

### A. FreeSWITCH

File: `configs/freeswitch/autoload_configs/switch.conf.xml`

```xml
<configuration name="switch.conf" description="Core Configuration">
  <settings>
    <!-- RTP Port Range -->
    <param name="rtp-start-port" value="16384"/>
    <param name="rtp-end-port" value="32768"/>

    <!-- Session Limits -->
    <param name="max-sessions" value="1000"/>
    <param name="sessions-per-second" value="100"/>

    <!-- Performance -->
    <param name="disable-monotonic-timing" value="false"/>
    <param name="enable-use-system-time" value="true"/>

    <!-- Logging (disable verbose logs in production) -->
    <param name="loglevel" value="warning"/>
  </settings>
</configuration>
```

### B. Kamailio

File: `configs/kamailio/kamailio.cfg` - Các tham số chính:

```cfg
# Process Configuration
children=16              # Match số CPU cores
tcp_children=16

# UDP kernel bypass mode (cần Kamailio 5.6+)
udp4_raw=yes            # Direct kernel bypass

# Memory
shm_mem_size=512        # 512 MB shared memory
pkg_mem_size=16         # 16 MB per process

# usrloc module - Write-back caching (critical!)
modparam("usrloc", "db_mode", 2)
modparam("usrloc", "db_update_as_insert", 1)
modparam("usrloc", "timer_interval", 30)
modparam("usrloc", "timer_procs", 2)
```

### C. PostgreSQL

File: `configs/postgresql/postgresql.conf` - VoIP specific:

```ini
# Connection
max_connections = 300

# Memory
shared_buffers = 4GB
effective_cache_size = 12GB
work_mem = 16MB
maintenance_work_mem = 512MB

# WAL
wal_buffers = 16MB
checkpoint_completion_target = 0.9
wal_writer_delay = 200ms

# Autovacuum cho Kamailio location table (high update rate)
autovacuum_naptime = 10s
autovacuum_vacuum_scale_factor = 0.01
autovacuum_analyze_scale_factor = 0.005
```

---

## 7. Performance Benchmarks

### Tính Toán Cho 800 Concurrent Calls (G.711)

**Băng Thông:**
```
Codec G.711 (PCMU/PCMA):
- Bitrate: 64 kbps (audio) + 23 kbps (overhead) = 87 kbps
- Bidirectional: 87 kbps × 2 = 174 kbps per call
- 800 calls: 174 kbps × 800 = 139.2 Mbps (~140 Mbps)
```

**Packets Per Second:**
```
G.711 @ 20ms ptime:
- 50 packets/sec per direction
- 100 packets/sec bidirectional per call
- 800 calls: 100 × 800 = 80,000 PPS
```

**Network Interface Requirements:**
```
Bandwidth: 140 Mbps < 1 Gbps ✓ OK
PPS: 80k (NIC phải handle được)
Connections: ~8,000 (conntrack 262k ✓ OK)
```

**Memory Usage Ước Tính:**
```
FreeSWITCH: ~8 GB (10 MB/call × 800)
Kamailio: ~2 GB (shared_buffers + cache)
PostgreSQL: ~6 GB
OS + Other: ~4 GB
Total: ~20 GB / 64 GB available ✓ OK
```

---

## 8. Monitoring Commands

### Check Performance Real-Time

```bash
# Network stats
watch -n 1 "ifconfig ens33 | grep 'RX\|TX'"

# PPS (packets per second)
sar -n DEV 1

# Connection tracking
watch -n 1 "cat /proc/sys/net/netfilter/nf_conntrack_count"

# FreeSWITCH sessions
fs_cli -x "show channels count"

# PostgreSQL connections
sudo -u postgres psql -c "SELECT count(*) FROM pg_stat_activity;"

# CPU per process
top -H -p $(pgrep -f freeswitch)
```

### Load Testing với SIPp

```bash
# Basic call load test
sipp -sn uac \
  -s 1000 \
  -d 60000 \
  -l 800 \
  -r 10 \
  172.16.91.100:5060

# Parametros:
# -l 800: 800 concurrent calls
# -r 10: 10 calls per second
# -d 60000: 60 second call duration
```

---

## 9. Troubleshooting

### Packet Loss Cao

```bash
# Kiểm tra ring buffer drops
ethtool -S ens33 | grep -i drop

# Kiểm tra conntrack overflow
dmesg | grep conntrack

# Tăng buffers nếu cần
sysctl -w net.core.rmem_max=268435456
```

### Jitter Cao

```bash
# Kiểm tra interrupt distribution
cat /proc/interrupts | grep ens33

# Enable IRQ affinity
/usr/local/bin/set-irq-affinity.sh
```

### Call Setup Latency Cao

```bash
# Kiểm tra PostgreSQL query time
sudo -u postgres psql kamailio -c "SELECT * FROM pg_stat_statements ORDER BY mean_time DESC LIMIT 10;"

# Kamailio stats
kamcmd stats.get_statistics all
```

---

## 10. Pre-Production Checklist

- [ ] Kernel tuning applied: `sysctl -p /etc/sysctl.d/90-voip-network.conf`
- [ ] NIC tuning applied: `/usr/local/bin/tune-nic.sh`
- [ ] IRQ affinity configured: `/usr/local/bin/set-irq-affinity.sh`
- [ ] QoS/DSCP marking enabled: `/etc/voip-qos-rules.sh`
- [ ] PostgreSQL auth updated: MD5 cho Kamailio/FreeSWITCH
- [ ] FreeSWITCH tuning applied
- [ ] Kamailio tuning applied
- [ ] Load test completed: 800 CC stable
- [ ] Monitoring setup: Grafana + Prometheus (optional)
- [ ] QoS verified end-to-end với NOC team
- [ ] Failover test passed

---

**Lưu Ý**: Tất cả tuning này đã được test với production VoIP systems tương tự. Áp dụng từng bước và monitor kỹ.
