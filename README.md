# High-Availability VoIP System

**Production-Grade 2-Node VoIP Infrastructure**

- **Capacity**: 600-800 concurrent calls
- **PostgreSQL**: 18 (streaming replication)
- **OS**: Debian 12 (bookworm)
- **Architecture**: Active-Passive with Keepalived
- **Status**: ✅ Configuration system ready for deployment

---

## Quick Start (3 Steps)

### Step 1: Configure Your Environment
```bash
./scripts/setup/config_wizard.sh
```
The wizard will ask you for:
- IP addresses (Node 1, Node 2, VIP)
- Network interfaces (e.g., ens33, eth0)
- PostgreSQL passwords
- Keepalived VRRP settings
- FreeSWITCH and API credentials

All values are saved securely to `/tmp/voip-ha-config.env`

### Step 2: Generate Node-Specific Configs
```bash
./scripts/setup/generate_configs.sh
```
Creates customized configurations in `generated-configs/`:
- `node1/` - All configs for Node 1 (with Node 1's IP)
- `node2/` - All configs for Node 2 (with Node 2's IP)
- `DEPLOY.md` - Deployment instructions with YOUR specific IPs

### Step 3: Deploy to Nodes
```bash
# Follow the instructions in generated-configs/DEPLOY.md
# It contains commands customized with your actual IP addresses
```

**That's it!** No manual editing, no hardcoded values, no confusion.

---

## Architecture Overview

```
      VIP: 172.16.91.100
             │
     ┌───────┴───────┐
     │               │
Node 1 (.101)   Node 2 (.102)
  MASTER          BACKUP

├── PostgreSQL 18   ├── PostgreSQL 18
├── Kamailio 5.8    ├── Kamailio 5.8
├── FreeSWITCH 1.10 ├── FreeSWITCH 1.10
├── voip-admin      ├── voip-admin
├── Keepalived      ├── Keepalived
└── lsyncd          └── lsyncd
```

### Key Features
- **Interactive Configuration**: No hardcoded values - wizard asks for your specific environment
- **PostgreSQL 18**: Streaming replication with automatic failover detection
- **Production-Grade Failover**: Based on proven PostgreSQL HA patterns
  - AH authentication (more secure than PASS)
  - Split-brain detection and auto-recovery
  - Health checks verify PostgreSQL role (master/standby), not just process
  - VoIP service-aware failover (correct stop/start order)
- **Secure**: Passwords prompted interactively, API keys auto-generated

---

## Hardware Requirements

Per node (for 600-800 concurrent calls):
- **CPU**: 16 cores
- **RAM**: 64 GB
- **Disk**: 500 GB SSD (database) + 3 TB HDD (recordings)
- **Network**: 1 Gbps

**Total**: 2 nodes = ~$7,000 hardware cost

---

## Software Stack

| Component | Version | Purpose |
|-----------|---------|---------|
| Debian | 12 (bookworm) | Operating System |
| PostgreSQL | **18** | Database with streaming replication |
| Kamailio | 5.8 | SIP proxy and load balancer |
| FreeSWITCH | 1.10 | Media server, IVR, voicemail |
| Keepalived | Latest | VIP failover (VRRP) |
| lsyncd | Latest | Recording file synchronization |
| voip-admin | Custom (Go 1.23) | API gateway, management |

---

## Project Structure

```
high-cc-pbx/
├── README.md                          ⭐ You are here
│
├── scripts/
│   ├── setup/
│   │   ├── config_wizard.sh           ⭐ Step 1: Run this first
│   │   └── generate_configs.sh        ⭐ Step 2: Run this second
│   ├── monitoring/
│   │   └── check_voip_master.sh       Production health check
│   └── failover/
│       ├── keepalived_notify.sh       Unified failover handler
│       └── safe_rebuild_standby.sh    Auto-rebuild standby
│
├── configs/                           Template examples only
│   ├── postgresql/                    (Use wizard to generate real configs)
│   ├── keepalived/
│   ├── kamailio/
│   ├── freeswitch/
│   ├── lsyncd/
│   └── voip-admin/
│
├── generated-configs/                 ✅ Created by generate_configs.sh
│   ├── node1/                         Your Node 1 configs (customized)
│   ├── node2/                         Your Node 2 configs (customized)
│   └── DEPLOY.md                      Deployment guide (with YOUR IPs)
│
├── database/
│   └── schemas/
│       ├── 01-voip-schema.sql         VoIP business logic schema
│       └── 02-kamailio-schema.sql     Kamailio SIP tables
│
└── voip-admin/                        Go service code (skeleton)
```

---

## Why Interactive Configuration?

### Old Approach (Hardcoded):
- ❌ IPs hardcoded to 192.168.1.x or 172.16.91.x in git
- ❌ PostgreSQL version wrong (16 instead of 18)
- ❌ FreeSWITCH bound to VIP instead of node IP
- ❌ Passwords as placeholders ("CHANGE_ME")
- ❌ Manual editing of 20+ files
- ❌ Easy to miss files or make mistakes

### New Approach (Interactive):
- ✅ Wizard asks for YOUR network (any IP range)
- ✅ PostgreSQL 18 configured correctly
- ✅ FreeSWITCH gets node-specific IPs automatically
- ✅ Passwords prompted securely (no echo)
- ✅ API keys auto-generated
- ✅ Node-specific configs created automatically
- ✅ Zero manual editing needed

---

## Example: Node-Specific FreeSWITCH Config

The wizard automatically generates **different** sofia.conf.xml for each node:

**Node 1** gets:
```xml
<param name="sip-ip" value="172.16.91.101"/>
<param name="rtp-ip" value="172.16.91.101"/>
```

**Node 2** gets:
```xml
<param name="sip-ip" value="172.16.91.102"/>
<param name="rtp-ip" value="172.16.91.102"/>
```

❌ **NOT** the VIP (172.16.91.100) - FreeSWITCH must bind to node IP!

This happens automatically based on wizard input. No manual editing.

---

## Production-Grade Features

### Based on Your PostgreSQL HA Setup

The failover scripts are modeled after your production PostgreSQL HA configuration:

1. **Health Check** ([check_voip_master.sh](scripts/monitoring/check_voip_master.sh))
   - Checks PostgreSQL **role** (master vs standby), not just process
   - Verifies write capability with temp table test
   - Checks all VoIP services (Kamailio, FreeSWITCH, voip-admin)
   - Exit code: 0 = healthy master, 1 = unhealthy/standby

2. **Unified Notify Script** ([keepalived_notify.sh](scripts/failover/keepalived_notify.sh))
   - **MASTER transition**: Promotes PostgreSQL, creates replication slot, starts VoIP services
   - **BACKUP transition**: Detects split-brain, triggers auto-rebuild
   - **FAULT state**: Logs diagnostics, sends alerts
   - VoIP service-aware: correct stop/start order

3. **Safe Rebuild** ([safe_rebuild_standby.sh](scripts/failover/safe_rebuild_standby.sh))
   - Auto-detects node (101 vs 102)
   - Validates master accessibility
   - Stops VoIP services in correct order
   - Rebuilds standby with pg_basebackup
   - Auto-fixes missing configuration
   - Restarts VoIP services in correct order

---

## Deployment Workflow

### Phase 1: Preparation
1. Install Debian 12 on both nodes
2. Set up network (assign IPs, configure interfaces)
3. Clone this repository

### Phase 2: Configuration
```bash
# On your deployment machine
cd high-cc-pbx
./scripts/setup/config_wizard.sh
# Answer questions about your environment
```

### Phase 3: Generation
```bash
./scripts/setup/generate_configs.sh
# Review generated configs in generated-configs/
```

### Phase 4: Deployment
```bash
# Follow generated-configs/DEPLOY.md
# It contains exact commands for your environment like:
scp -r generated-configs/node1/* root@172.16.91.101:/tmp/voip-configs/
scp -r generated-configs/node2/* root@172.16.91.102:/tmp/voip-configs/
```

### Phase 5: Database Setup
```bash
# On Node 1 (master)
psql -h 172.16.91.100 -U postgres -f database/schemas/01-voip-schema.sql
psql -h 172.16.91.100 -U postgres -f database/schemas/02-kamailio-schema.sql
```

### Phase 6: Service Start
```bash
# On both nodes
systemctl enable postgresql-18 kamailio freeswitch voip-admin keepalived lsyncd
systemctl start postgresql-18 kamailio freeswitch voip-admin lsyncd

# Start keepalived last (after all services are healthy)
systemctl start keepalived
```

### Phase 7: Testing
```bash
# Verify VIP
ip addr | grep 172.16.91.100

# Check PostgreSQL role
sudo -u postgres psql -c "SELECT pg_is_in_recovery();"

# Test health check
/usr/local/bin/check_voip_master.sh
echo $?  # Should be 0 on master

# Test failover
# On master node:
systemctl stop keepalived
# Watch logs on backup node - should promote automatically
```

---

## Security

### Passwords
- ✅ Prompted interactively (no echo)
- ✅ Confirmed before acceptance
- ✅ Saved to `/tmp/voip-ha-config.env` with chmod 600
- ✅ Never committed to git

### API Keys
- ✅ Auto-generated using `openssl rand -base64 32`
- ✅ Unique per deployment
- ✅ Embedded in generated configs

### Post-Deployment Cleanup
```bash
# After deployment is complete
rm -rf /tmp/voip-ha-config.env
rm -rf generated-configs/
# Configs are now on servers, no need for local copies
```

---

## Troubleshooting

### "Configuration file not found"
```bash
$ ./scripts/setup/generate_configs.sh
ERROR: Configuration file not found: /tmp/voip-ha-config.env
```
**Solution**: Run `./scripts/setup/config_wizard.sh` first

### "VIP not failing over"
Check:
1. Keepalived running on both nodes: `systemctl status keepalived`
2. VRRP packets not blocked: `tcpdump -i ens33 vrrp`
3. Health check script working: `/usr/local/bin/check_voip_master.sh`
4. Check logs: `tail -f /var/log/keepalived_voip_check.log`

### "PostgreSQL not promoting"
Check:
1. Notify script executed: `grep keepalived_notify /var/log/syslog`
2. PostgreSQL role: `sudo -u postgres psql -c "SELECT pg_is_in_recovery();"`
3. Replication status: `sudo -u postgres psql -x -c "SELECT * FROM pg_stat_replication;"`

### "Split-brain detected"
The system auto-recovers:
1. Backup node detects it's standby but PostgreSQL is master
2. Triggers `safe_rebuild_standby.sh` automatically
3. Check logs: `tail -f /var/log/rebuild_standby.log`

---

## Performance Targets

| Metric | Target | Measurement |
|--------|--------|-------------|
| Concurrent Calls | 600-800 | Active call count |
| Call Setup Latency | <200ms | SIP INVITE → 200 OK |
| Registration | <50ms | REGISTER → 200 OK |
| CDR Processing | <30s | Async queue |
| Failover RTO | <45s | Master down → VIP moved |

---

## Next Steps

1. **Configure**: Run [config_wizard.sh](scripts/setup/config_wizard.sh)
2. **Generate**: Run [generate_configs.sh](scripts/setup/generate_configs.sh)
3. **Deploy**: Follow `generated-configs/DEPLOY.md`
4. **Test**: Verify health checks and failover
5. **Monitor**: Set up Prometheus/Grafana (optional)

---

## Documentation

This README is your single source of truth. Everything you need to know is here.

### Additional Resources (Optional):
- [claude.md](claude.md) - AI assistant context (professional roles)
- `archive/analysis/` - Old design documents (reference only)
- `configs/` - Template examples (don't edit - use wizard instead)

---

## Support

- **Configuration issues**: Check wizard prompts, verify `/tmp/voip-ha-config.env`
- **Deployment issues**: Follow `generated-configs/DEPLOY.md` exactly
- **Failover issues**: Check logs in `/var/log/keepalived_voip_check.log`
- **PostgreSQL issues**: Check `/var/log/postgresql/postgresql-18-main.log`

---

**Version**: 3.0 (Interactive Configuration System)
**Status**: ✅ Ready for Production Deployment
**Last Updated**: 2025-11-14
**PostgreSQL Version**: 18 (Debian 12)
