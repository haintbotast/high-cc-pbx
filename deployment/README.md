# Hướng Dẫn Triển Khai Theo Service

Tài liệu này chỉ dẫn thứ tự triển khai từng service của hệ thống VoIP HA.

## 📋 Thứ Tự Triển Khai (QUAN TRỌNG)

Tuân theo thứ tự dưới đây vì có dependencies giữa các services:

1. **[01-PostgreSQL-Deployment.md](01-PostgreSQL-Deployment.md)** ⚠️ BẮT BUỘC TRƯỚC TIÊN
   - Cài đặt PostgreSQL 18 trên cả 2 nodes
   - Thiết lập streaming replication
   - Tạo database schemas
   - Tạo users và permissions
   - **Dependencies**: Không có (foundation)
   - **Thời gian ước tính**: 2-3 giờ

2. **[02-Kamailio-Deployment.md](02-Kamailio-Deployment.md)**
   - Cài đặt Kamailio 6.0 repository
   - Deploy Kamailio configs
   - Setup kamctl và logging
   - Test SIP registration
   - **Dependencies**: PostgreSQL (database schemas, users)
   - **Thời gian ước tính**: 1-2 giờ

3. **[03-FreeSWITCH-Deployment.md](03-FreeSWITCH-Deployment.md)**
   - Cài đặt FreeSWITCH 1.10
   - ODBC configuration
   - XML dialplan setup
   - CDR integration với voip-admin
   - **Dependencies**: PostgreSQL, VoIP Admin (để post CDR)
   - **Thời gian ước tính**: 2-3 giờ

4. **[04-VoIP-Admin-Deployment.md](04-VoIP-Admin-Deployment.md)**
   - Build Go application
   - Deploy systemd service
   - Configure database connection
   - Setup API endpoints
   - **Dependencies**: PostgreSQL (voip schema)
   - **Thời gian ước tính**: 1 giờ

5. **[05-Keepalived-HA-Deployment.md](05-Keepalived-HA-Deployment.md)**
   - Cài đặt Keepalived
   - VIP configuration
   - Health check scripts
   - Failover testing
   - **Dependencies**: Tất cả services ở trên
   - **Thời gian ước tính**: 1-2 giờ

---

## 🎯 Quy Trình Deploy Từng Node

### Node 1 (Master)
```bash
# Deploy theo thứ tự 1 → 2 → 3 → 4 → 5
# Test từng service trước khi chuyển sang service tiếp theo
```

### Node 2 (Backup)
```bash
# Sau khi Node 1 stable:
# Deploy theo cùng thứ tự 1 → 2 → 3 → 4 → 5
# Test replication và failover
```

---

## ✅ Checklist Tổng Quát

- [ ] **Node 1**: PostgreSQL Master running
- [ ] **Node 2**: PostgreSQL Standby replicating
- [ ] **Node 1**: Kamailio accepting registrations
- [ ] **Node 2**: Kamailio configured (chưa start)
- [ ] **Node 1**: FreeSWITCH routing calls
- [ ] **Node 2**: FreeSWITCH configured (chưa start)
- [ ] **Node 1**: VoIP Admin API responding
- [ ] **Node 2**: VoIP Admin configured (chưa start)
- [ ] **Both**: Keepalived VIP on Node 1
- [ ] **Test**: Failover Node 1 → Node 2
- [ ] **Test**: Failback Node 2 → Node 1

---

## 📚 Tài Liệu Liên Quan

### Trước Khi Deploy
- [DEPLOYMENT-PREREQUISITES.md](../DEPLOYMENT-PREREQUISITES.md) - Chuẩn bị passwords, IPs, credentials
- [DATABASE-ARCHITECTURE.md](../DATABASE-ARCHITECTURE.md) - Hiểu LOCAL connection strategy
- [KAMAILIO-6-COMPATIBILITY.md](../KAMAILIO-6-COMPATIBILITY.md) - Breaking changes Kamailio 6.0

### Troubleshooting
- [DEPLOYMENT-CHECKLIST.md](../DEPLOYMENT-CHECKLIST.md) - Detailed step-by-step checklist
- Logs:
  - PostgreSQL: `/var/log/postgresql/postgresql-18-main.log`
  - Kamailio: `/var/log/kamailio.log`
  - FreeSWITCH: `/var/log/freeswitch/freeswitch.log`
  - VoIP Admin: `journalctl -u voipadmind -f`
  - Keepalived: `/var/log/keepalived_voip_check.log`

---

## 🔧 Deployment Tips

1. **Test từng bước** - Không deploy tất cả cùng lúc
2. **Verify database** - Luôn check PostgreSQL replication trước khi tiếp tục
3. **Backup configs** - Copy configs cũ trước khi thay thế
4. **Check logs** - Tail logs real-time khi start services
5. **Idempotent scripts** - Database schemas an toàn chạy lại
6. **One service at a time** - Deploy và test một service trước khi chuyển sang service khác

---

**Version**: 3.2.0
**Last Updated**: 2025-11-20
**Deployment Method**: Manual, Production-Ready
