# Changelog

Tất cả thay đổi quan trọng của dự án High-Availability VoIP System sẽ được ghi lại trong file này.

Format dựa trên [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
và dự án tuân theo [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [3.2.0] - 2025-11-20

### 🎯 Tập Trung Manual Deployment

Release này chuyển hướng từ automatic deployment sang **manual deployment** với tài liệu hoàn chỉnh và production-ready.

### Added
- ✅ **CHANGELOG.md** - Version history và tracking changes
- ✅ **Database schema idempotency** - Scripts an toàn chạy lại nhiều lần
- ✅ **kamctl configuration** - Complete kamctlrc với read-only user support
- ✅ **Kamailio logging** - Dedicated rsyslog config cho /var/log/kamailio.log
- ✅ **PostgreSQL search_path** - Auto-set cho kamailio/kamailioro users
- ✅ **kamailioro user** - Read-only database user cho kamctl read commands

### Fixed
- 🐛 **Database schemas** - Fixed 3 critical errors:
  - subscriber table/view conflict (02-kamailio-schema.sql)
  - DROP TABLE fails when subscriber is VIEW (03-auth-integration.sql)
  - cdr_queue status column not exist (01-voip-schema.sql)
- 🐛 **Kamailio 6.0 config** - 7 critical fixes:
  - hash_size must be power of 2 (4096)
  - get_profile_size() API change (return to variable)
  - UAC append_fromtag requirement
  - nathelper nat_bflag requirement
  - pg_hba.conf database names (voipdb)
  - kamctl DBENGINE case sensitivity (lowercase)
  - dialog tracking: dlg_manage() instead of setflag(4)

### Changed
- 📝 **MANUAL-DEPLOYMENT-GUIDE.md** - Thêm kamctl và rsyslog configuration
- 📝 **DEPLOYMENT-PREREQUISITES.md** - Thêm kamailioro user vào password table
- 📝 **KAMAILIO-6-COMPATIBILITY.md** - Document tất cả breaking changes từ 5.x
- 📝 **README.md** - Update version 3.2.0, dates, architecture

### Removed
- ❌ Loại bỏ focus vào automatic deployment
- ❌ Session notes archived (temporary working docs)

---

## [3.1] - 2025-01-19

### 🚀 Kamailio 6.0 Compatible

### Added
- ✅ Kamailio 6.0.x compatibility
- ✅ KAMAILIO-6-COMPATIBILITY.md documentation
- ✅ Kamailio 6.0 configuration với all required modules
- ✅ Kamailio repository setup cho version 6.0

### Changed
- 📝 Updated README.md - Kamailio version 6.0
- 📝 Configs updated for Kamailio 6.0 API changes

### Fixed
- 🐛 Dialog module API changes (dlg_manage)
- 🐛 Dispatcher AVP handling (internal variables)
- 🐛 Authentication auth_check() compatibility

---

## [3.0] - 2025-01-17

### 🗄️ PostgreSQL 18 Upgrade

### Added
- ✅ PostgreSQL 18 support
- ✅ Streaming replication configuration
- ✅ Database schemas: voip, kamailio, integration
- ✅ DATABASE-ARCHITECTURE.md - LOCAL connection strategy

### Changed
- 📝 PostgreSQL upgrade from 16 to 18
- 📝 Updated pg_hba.conf, postgresql.conf for PG18

### Fixed
- 🐛 PostgreSQL replication errors
- 🐛 Database connection issues

---

## [2.0] - 2025-01-15

### 🏗️ Project Restructure

### Added
- ✅ DEPLOYMENT-CHECKLIST.md
- ✅ DEPLOYMENT-PREREQUISITES.md
- ✅ Production-ready configurations
- ✅ Failover scripts (keepalived_notify.sh, safe_rebuild_standby.sh)

### Changed
- 📝 Việt hóa toàn bộ tài liệu
- 📝 Restructured project directories
- 📝 Cleaned up documentation (giảm từ 10+ files xuống 3 core files)

### Removed
- ❌ Old backup files (*.bak)
- ❌ Redundant documentation

---

## [1.0] - 2025-01-10

### 🎉 Initial Release

### Added
- ✅ High-Availability VoIP System architecture
- ✅ Kamailio 5.8 SIP proxy
- ✅ FreeSWITCH 1.10 media server
- ✅ PostgreSQL 16 database with replication
- ✅ Keepalived for VIP failover
- ✅ voip-admin Go application
- ✅ Basic configurations and scripts
- ✅ 600-800 concurrent calls capacity

---

## Release Notes

### Version Naming Convention
- **Major (X.0.0)**: Breaking changes, major architecture updates
- **Minor (x.X.0)**: New features, enhancements, compatible changes
- **Patch (x.x.X)**: Bug fixes, documentation updates

### Changelog Categories
- **Added**: New features, files, capabilities
- **Changed**: Changes to existing functionality
- **Deprecated**: Features marked for removal
- **Removed**: Removed features
- **Fixed**: Bug fixes
- **Security**: Security fixes

### Support
- **Latest version**: 3.2.0
- **Minimum supported**: 3.0 (PostgreSQL 18)
- **End-of-life**: < 3.0

---

**Duy trì bởi**: VoIP HA Project Team
**Repository**: https://github.com/haintbotast/high-cc-pbx
**Documentation**: [README.md](README.md)
