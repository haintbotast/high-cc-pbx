# 05 - Keepalived High Availability Deployment Guide

**Service**: Keepalived VRRP (VIP Failover)  
**Dependencies**: Tất cả services (PostgreSQL, Kamailio, FreeSWITCH, VoIP Admin)  
**Thời gian ước tính**: 1-2 giờ  
**Deploy sau**: [04-VoIP-Admin-Deployment.md](04-VoIP-Admin-Deployment.md)

---

## Tổng Quan

Keepalived quản lý VIP failover và health checks:
- VRRP protocol cho VIP management (172.16.91.100)
- Health check scripts kiểm tra PostgreSQL role (master/standby)
- Health check scripts kiểm tra Kamailio status
- Automatic failover khi detect failures
- Notify scripts để stop/start services đúng thứ tự

### Kiến Trúc HA

```
Normal Operation (Node 1 is MASTER):
- Node 1: VIP active, all services running
- Node 2: VIP standby, services stopped (except PostgreSQL standby)

After Failover (Node 2 becomes MASTER):
- Node 1: Services stopped, VIP removed
- Node 2: VIP active, services started
```

### Health Check Strategy

⚠️ **QUAN TRỌNG**: Health checks phải kiểm tra PostgreSQL ROLE:
- Check if PostgreSQL is MASTER (not in recovery)
- Nếu PostgreSQL standby → FAIL health check → trigger failover
- Không chỉ check process running, phải check ROLE

---

## Deployment Steps

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


---

## Troubleshooting

### VIP không move khi failover
```bash
# Check Keepalived status
systemctl status keepalived

# Check logs
journalctl -u keepalived -n 100 --no-pager
tail -100 /var/log/keepalived_voip_check.log

# Manual test health check script
sudo /etc/keepalived/check_voip_services.sh
echo $?
# Should return 0 if OK, 1 if FAIL
```

### Split-brain (cả 2 nodes có VIP)
```bash
# Check VIP on both nodes
# Node 1:
ip addr show ens33 | grep 172.16.91.100

# Node 2:
ip addr show ens33 | grep 172.16.91.100

# If both have VIP → split-brain
# Fix: Stop keepalived on one node, verify health checks
```

### Services không auto-start sau failover
```bash
# Check notify script
ls -la /etc/keepalived/keepalived_notify.sh
# Should be executable

# Test notify script manually
sudo /etc/keepalived/keepalived_notify.sh MASTER VoIP_HA 100

# Check logs
journalctl -xe | grep keepalived
```

### PostgreSQL health check fails
```bash
# Manual test PostgreSQL role check
sudo -u postgres psql -t -c "SELECT pg_is_in_recovery();"
# Should return: f (false = master), t (true = standby)

# Test full health check
sudo /etc/keepalived/check_voip_services.sh
```

---

## Verification Checklist

- [ ] Keepalived installed on both nodes
- [ ] Config files deployed to both nodes
- [ ] Node 1: priority=100 (MASTER)
- [ ] Node 2: priority=90 (BACKUP)
- [ ] Health check script executable
- [ ] Notify script executable
- [ ] VIP on Node 1 (normal state)
- [ ] VIP not on Node 2 (normal state)
- [ ] Can ping VIP (172.16.91.100)
- [ ] Failover test successful (stop Kamailio on Node 1 → VIP moves to Node 2)
- [ ] Failback successful (start Kamailio on Node 1 → VIP returns to Node 1)
- [ ] Services stop/start correctly during failover
- [ ] Health check logs clean

---

## Failover Testing Procedure

### Test 1: Service Failure
```bash
# Trên Node 1 (MASTER):
sudo systemctl stop kamailio

# Wait 10 seconds
# Verify VIP moved to Node 2:
# Trên Node 2:
ip addr show ens33 | grep 172.16.91.100
# Should show VIP

# Restart Kamailio on Node 1:
sudo systemctl start kamailio

# VIP should move back to Node 1
```

### Test 2: PostgreSQL Failover
```bash
# Simulate PostgreSQL failure on Node 1
sudo systemctl stop postgresql

# Wait for health check to detect
# VIP should move to Node 2
# PostgreSQL on Node 2 should be promoted to master

# This test requires safe_rebuild_standby.sh script
```

### Test 3: Complete Node Failure
```bash
# Power off Node 1 (or network disconnect)
# VIP should move to Node 2 within 10-15 seconds
# All services should start on Node 2
```

---

**Hoàn tất deployment**: Hệ thống HA đã sẵn sàng!  
**Tiếp theo**: Test tích hợp toàn bộ hệ thống và tạo dữ liệu mẫu  
**Quay lại**: [README.md](README.md) - Deployment Overview

**Version**: 3.2.0  
**Last Updated**: 2025-11-20
