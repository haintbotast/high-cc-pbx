# High-Availability VoIP System (Interactive Configuration)

## 🎯 Quick Start

This VoIP HA system uses **PostgreSQL 18** and an **interactive configuration approach** - no hardcoded IPs or passwords!

### 3-Step Setup:

```bash
# 1. Run interactive wizard to configure your environment
./scripts/setup/config_wizard.sh

# 2. Generate node-specific configurations
./scripts/setup/generate_configs.sh

# 3. Deploy to your nodes
# Follow instructions in generated-configs/DEPLOY.md
```

That's it! All configs will be customized for your network.

---

## 📚 Documentation

- **[INTERACTIVE-SETUP.md](INTERACTIVE-SETUP.md)** - Complete guide to the interactive system ⭐ START HERE
- **[PRODUCTION-READY-SUMMARY.md](PRODUCTION-READY-SUMMARY.md)** - 11-phase deployment checklist
- **[WORK-COMPLETED.md](WORK-COMPLETED.md)** - What's been built and production-ready status
- **[IMPLEMENTATION-PLAN.md](IMPLEMENTATION-PLAN.md)** - Original 8-phase implementation plan

---

## 🏗️ Architecture

- **2 Nodes**: Active-Passive High Availability
- **VIP**: Keepalived with VRRP (AH authentication)
- **Database**: PostgreSQL 18 with streaming replication and physical slots
- **SIP Proxy**: Kamailio 5.8
- **Media Server**: FreeSWITCH 1.10
- **Admin API**: Go-based voip-admin service
- **Capacity**: 600-800 concurrent calls

### Software Stack

| Component | Version | Purpose |
|-----------|---------|---------|
| PostgreSQL | **18** | Primary database with streaming replication |
| Kamailio | 5.8 | SIP proxy, load balancer |
| FreeSWITCH | 1.10 | Media server, IVR, voicemail |
| Keepalived | Latest | VRRP failover |
| lsyncd | Latest | Recording file synchronization |
| VoIP Admin | Custom (Go 1.23) | API gateway, XML_CURL provider |

---

## 💡 Key Features

### Interactive Configuration ✨
- **No hardcoded values** - Wizard asks for your specific IPs, passwords, interface names
- **Node-specific configs** - Automatically generates different configs for Node 1 and Node 2
- **Secure** - Passwords prompted interactively, API keys auto-generated
- **PostgreSQL 18** - Correctly configured (not 16!)

### Production-Grade Failover
- Based on proven PostgreSQL HA patterns
- **AH authentication** for Keepalived (more secure than PASS)
- **Split-brain detection** and automatic recovery
- **Health checks** verify PostgreSQL role (master/standby), not just process status
- **Unified notify script** handles MASTER/BACKUP/FAULT transitions

### Service-Aware
- Failover scripts know about VoIP service dependencies
- Stops services in correct order: voip-admin → freeswitch → kamailio → postgresql
- Starts services in correct order: postgresql → kamailio → freeswitch → voip-admin

---

## 📁 Project Structure

```
high-cc-pbx/
├── INTERACTIVE-SETUP.md           ⭐ Interactive system guide
├── README-INTERACTIVE.md           This file
│
├── scripts/
│   ├── setup/
│   │   ├── config_wizard.sh       ⭐ Run this first
│   │   └── generate_configs.sh    ⭐ Run this second
│   ├── monitoring/
│   │   └── check_voip_master.sh   Production health check
│   └── failover/
│       ├── keepalived_notify.sh   Unified notify script
│       └── safe_rebuild_standby.sh Auto-fix standby rebuild
│
├── configs/                        📝 Templates/examples only
│   ├── keepalived/                 (Use wizard to generate real configs)
│   ├── postgresql/
│   ├── freeswitch/
│   └── ...
│
├── generated-configs/              ✅ Created by generate_configs.sh
│   ├── node1/                      Your Node 1 configs
│   ├── node2/                      Your Node 2 configs
│   └── DEPLOY.md                   Deployment instructions with YOUR IPs
│
├── database/schemas/
│   ├── 01-voip-schema.sql          VoIP business logic schema
│   └── 02-kamailio-schema.sql      Kamailio SIP tables
│
└── voip-admin/                     Go service (skeleton)
```

---

## 🚀 Deployment Workflow

### Phase 1: Configure
```bash
./scripts/setup/config_wizard.sh
```
Enter your:
- IP addresses (Node 1, Node 2, VIP)
- Network interfaces (ens33, eth0, etc.)
- PostgreSQL passwords
- Keepalived VRRP settings
- FreeSWITCH ports and passwords

Saves to: `/tmp/voip-ha-config.env`

### Phase 2: Generate
```bash
./scripts/setup/generate_configs.sh
```
Creates:
- `generated-configs/node1/` - All configs for Node 1
- `generated-configs/node2/` - All configs for Node 2
- `generated-configs/DEPLOY.md` - Deployment instructions

### Phase 3: Deploy
Follow `generated-configs/DEPLOY.md`:
1. Copy configs to each node
2. Apply PostgreSQL schemas
3. Create database users
4. Start services
5. Enable Keepalived
6. Test failover

---

## 🔧 Example: Node-Specific FreeSWITCH Config

The wizard generates **different** sofia.conf.xml for each node:

**Node 1** (172.16.91.101):
```xml
<param name="sip-ip" value="172.16.91.101"/>
<param name="rtp-ip" value="172.16.91.101"/>
```

**Node 2** (172.16.91.102):
```xml
<param name="sip-ip" value="172.16.91.102"/>
<param name="rtp-ip" value="172.16.91.102"/>
```

❌ **NOT** the VIP! FreeSWITCH must bind to the node IP.

---

## 🔒 Security

### Passwords
- Prompted interactively (not stored in git)
- Confirmed before acceptance
- Saved to `/tmp/voip-ha-config.env` with chmod 600

### API Keys
- Auto-generated using `openssl rand -base64 32`
- Unique per deployment
- Embedded in generated configs

### Best Practices
```bash
# After deployment, clean up:
rm -rf /tmp/voip-ha-config.env
rm -rf generated-configs/

# Configs are now on the servers, no need for local copies
```

---

## 📊 Hardware Requirements

Per node (based on 600-800 concurrent calls):
- **CPU**: 16 cores
- **RAM**: 64 GB
- **Disk**: 500 GB SSD (database + recordings)
- **Network**: 1 Gbps

Total system: **2 nodes** = $7,000 hardware cost

---

## 🧪 Testing

### Test Configuration Syntax
```bash
# Keepalived
keepalived -t -f generated-configs/node1/keepalived/keepalived.conf

# PostgreSQL
sudo -u postgres /usr/pgsql-18/bin/postgres -C data_directory

# Kamailio
kamailio -c -f generated-configs/node1/kamailio/kamailio.cfg
```

### Test Failover
```bash
# On Node 1 (master), stop Keepalived
systemctl stop keepalived

# On Node 2, verify:
# - VIP moved: ip addr | grep 172.16.91.100
# - PostgreSQL promoted: sudo -u postgres psql -c "SELECT pg_is_in_recovery();"

# Start Keepalived on Node 1 again
systemctl start keepalived

# Node 1 should detect split-brain and auto-rebuild as standby
tail -f /var/log/rebuild_standby.log
```

---

## 📞 Support & Feedback

For questions about:
- **Interactive configuration**: See [INTERACTIVE-SETUP.md](INTERACTIVE-SETUP.md)
- **Deployment process**: See [PRODUCTION-READY-SUMMARY.md](PRODUCTION-READY-SUMMARY.md)
- **Production setup**: See [WORK-COMPLETED.md](WORK-COMPLETED.md)

---

## ✅ What's Production-Ready

1. ✅ Interactive configuration wizard
2. ✅ PostgreSQL 18 (not 16!)
3. ✅ Node-specific config generation
4. ✅ Production-grade Keepalived (AH auth, split-brain detection)
5. ✅ Service-aware failover scripts
6. ✅ Secure password handling
7. ✅ Auto-generated API keys

---

## 🎯 Next Steps

1. Read [INTERACTIVE-SETUP.md](INTERACTIVE-SETUP.md)
2. Run `./scripts/setup/config_wizard.sh`
3. Review `/tmp/voip-ha-config.env`
4. Run `./scripts/setup/generate_configs.sh`
5. Follow `generated-configs/DEPLOY.md`

**No more hardcoded values. No more manual editing. Just configure and deploy!**
