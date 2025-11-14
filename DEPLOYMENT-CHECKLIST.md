# VoIP HA Deployment Checklist

Quick reference guide for deploying the VoIP HA system.

**For complete details, see [README.md](README.md)**

---

## Pre-Deployment

### Hardware Preparation
- [ ] 2 servers with Debian 12 installed
- [ ] Each server: 16 cores, 64 GB RAM, 500 GB SSD + 3 TB HDD
- [ ] Network configured (IPs assigned, interfaces up)
- [ ] Servers can ping each other
- [ ] Git repository cloned on deployment machine

### Network Planning
- [ ] Decide on IP addresses:
  - Node 1 IP: ____________
  - Node 2 IP: ____________
  - VIP: ____________
- [ ] Know network interface names (e.g., ens33, eth0)
- [ ] Firewall rules prepared (if needed)

---

## Configuration (Steps 1-2)

### Step 1: Run Config Wizard
```bash
cd high-cc-pbx
./scripts/setup/config_wizard.sh
```

**The wizard will ask for:**
- [ ] Node IPs and VIP
- [ ] Network interfaces
- [ ] Hostnames
- [ ] PostgreSQL passwords (replication, kamailio, voipadmin, freeswitch)
- [ ] Keepalived VRRP settings
- [ ] FreeSWITCH ports and passwords
- [ ] VoIP Admin port

**Output**: `/tmp/voip-ha-config.env`

### Step 2: Generate Configs
```bash
./scripts/setup/generate_configs.sh
```

**Output**: `generated-configs/` directory with:
- [ ] `node1/` configs
- [ ] `node2/` configs
- [ ] `DEPLOY.md` with deployment instructions

**Review generated configs before proceeding**

---

## Deployment (Steps 3-6)

### Step 3: Copy Configs to Nodes
```bash
# Follow exact commands in generated-configs/DEPLOY.md
scp -r generated-configs/node1/* root@<NODE1_IP>:/tmp/voip-configs/
scp -r generated-configs/node2/* root@<NODE2_IP>:/tmp/voip-configs/
```

### Step 4: Install Packages (on both nodes)
```bash
# PostgreSQL 18
apt install -y postgresql-18 postgresql-contrib-18

# Kamailio
apt install -y kamailio kamailio-postgres-modules

# FreeSWITCH
apt install -y freeswitch freeswitch-mod-commands freeswitch-mod-sofia

# Keepalived
apt install -y keepalived

# lsyncd
apt install -y lsyncd rsync
```

### Step 5: Apply Configs (on both nodes)
```bash
# Copy configs to system directories
# Follow exact paths in generated-configs/DEPLOY.md

# Example (verify paths):
cp /tmp/voip-configs/keepalived/keepalived.conf /etc/keepalived/
cp /tmp/voip-configs/postgresql/pg_hba.conf /etc/postgresql/18/main/
cp /tmp/voip-configs/freeswitch/sofia.conf.xml /etc/freeswitch/autoload_configs/
cp /tmp/voip-configs/voip-admin/config.yaml /etc/voip-admin/
cp /tmp/voip-configs/scripts/* /usr/local/bin/
chmod +x /usr/local/bin/*.sh
```

### Step 6: Database Setup (Node 1 only)
```bash
# On Node 1
sudo -u postgres createuser -s replicator
sudo -u postgres psql -c "ALTER USER replicator WITH PASSWORD '<YOUR_REPL_PASSWORD>';"

# Create databases
sudo -u postgres createdb voip
sudo -u postgres createdb kamailio

# Apply schemas
sudo -u postgres psql -d voip -f /path/to/01-voip-schema.sql
sudo -u postgres psql -d kamailio -f /path/to/02-kamailio-schema.sql

# Create application users
sudo -u postgres psql <<EOF
CREATE USER kamailio WITH PASSWORD '<YOUR_KAMAILIO_PASSWORD>';
CREATE USER voipadmin WITH PASSWORD '<YOUR_VOIPADMIN_PASSWORD>';
CREATE USER freeswitch WITH PASSWORD '<YOUR_FREESWITCH_PASSWORD>';
GRANT ALL ON DATABASE kamailio TO kamailio;
GRANT ALL ON DATABASE voip TO voipadmin;
GRANT ALL ON DATABASE voip TO freeswitch;
EOF
```

---

## Service Start (Steps 7-8)

### Step 7: Configure Replication (Node 2)
```bash
# On Node 2, as postgres user
sudo -u postgres pg_basebackup -h <NODE1_IP> -U replicator -D /var/lib/postgresql/18/main -Fp -Xs -P -R

# Verify standby.signal exists
ls -la /var/lib/postgresql/18/main/standby.signal

# Start PostgreSQL on Node 2
systemctl start postgresql-18
systemctl enable postgresql-18
```

### Step 8: Start Services (Both Nodes)
```bash
# On both nodes

# PostgreSQL (already started)
systemctl enable postgresql-18

# Kamailio
systemctl enable kamailio
systemctl start kamailio

# FreeSWITCH
systemctl enable freeswitch
systemctl start freeswitch

# VoIP Admin
systemctl enable voip-admin
systemctl start voip-admin

# lsyncd
systemctl enable lsyncd
systemctl start lsyncd

# Keepalived (START LAST!)
systemctl enable keepalived
systemctl start keepalived
```

---

## Verification (Step 9)

### Check VIP
```bash
# On Node 1 (should have VIP)
ip addr | grep <VIP>

# On Node 2 (should NOT have VIP)
ip addr | grep <VIP>
```

### Check PostgreSQL Roles
```bash
# On Node 1 (should be false = master)
sudo -u postgres psql -c "SELECT pg_is_in_recovery();"

# On Node 2 (should be true = standby)
sudo -u postgres psql -c "SELECT pg_is_in_recovery();"
```

### Check Replication
```bash
# On Node 1 (should show Node 2 connected)
sudo -u postgres psql -x -c "SELECT * FROM pg_stat_replication;"
```

### Check Health Script
```bash
# On Node 1 (should return 0)
/usr/local/bin/check_voip_master.sh
echo $?

# On Node 2 (should return 1)
/usr/local/bin/check_voip_master.sh
echo $?
```

### Check Service Status
```bash
# On both nodes
systemctl status postgresql-18 kamailio freeswitch voip-admin keepalived lsyncd
```

---

## Failover Testing (Step 10)

### Test 1: Graceful Failover
```bash
# On Node 1 (master)
systemctl stop keepalived

# Wait 30-45 seconds

# On Node 2, check:
# - VIP moved
ip addr | grep <VIP>

# - PostgreSQL promoted
sudo -u postgres psql -c "SELECT pg_is_in_recovery();"  # Should be false

# - Services running
systemctl status kamailio freeswitch voip-admin
```

### Test 2: Failback
```bash
# On Node 1, start keepalived again
systemctl start keepalived

# Node 1 should detect split-brain and auto-rebuild as standby
tail -f /var/log/rebuild_standby.log

# After rebuild completes:
# - Node 1 should be standby
sudo -u postgres psql -c "SELECT pg_is_in_recovery();"  # Should be true

# - Node 2 should still be master with VIP
```

### Test 3: Service Failure
```bash
# On master node, stop PostgreSQL
systemctl stop postgresql-18

# Health check should fail, triggering failover
tail -f /var/log/keepalived_voip_check.log
```

---

## Post-Deployment Cleanup

### On Deployment Machine
```bash
# Remove sensitive config file
rm -rf /tmp/voip-ha-config.env

# Remove or archive generated configs
rm -rf generated-configs/
# OR
mv generated-configs/ ~/backups/voip-configs-$(date +%Y%m%d)/
```

### On Nodes
```bash
# Remove temporary configs
rm -rf /tmp/voip-configs/
```

---

## Monitoring Setup (Optional)

### Check Logs
```bash
# Keepalived health checks
tail -f /var/log/keepalived_voip_check.log

# Keepalived transitions
grep keepalived /var/log/syslog

# PostgreSQL
tail -f /var/log/postgresql/postgresql-18-main.log

# Kamailio
journalctl -u kamailio -f

# FreeSWITCH
tail -f /usr/local/freeswitch/log/freeswitch.log
```

### Set Up Monitoring (Optional)
- [ ] Prometheus + Grafana
- [ ] Centralized logging (ELK, Loki)
- [ ] Alerting (email, Slack, PagerDuty)

---

## Troubleshooting Quick Reference

| Issue | Check | Solution |
|-------|-------|----------|
| VIP not moving | `systemctl status keepalived` | Check VRRP config, firewall |
| PostgreSQL not replicating | `pg_stat_replication` | Check pg_hba.conf, replication user |
| Health check failing | `/usr/local/bin/check_voip_master.sh` | Check script permissions, service status |
| Split-brain | Check logs in `/var/log/rebuild_standby.log` | Auto-recovery via safe_rebuild |
| Services not starting | `systemctl status <service>` | Check logs, config syntax |

**For detailed troubleshooting, see [README.md](README.md#troubleshooting)**

---

## Success Criteria

✅ **Deployment is successful when:**
- [ ] VIP responds on Node 1
- [ ] PostgreSQL replication active (Node 1 → Node 2)
- [ ] All services running on both nodes
- [ ] Health check returns 0 on master, 1 on standby
- [ ] Failover test succeeds (VIP moves, PostgreSQL promotes)
- [ ] Failback test succeeds (Node 1 auto-rebuilds as standby)
- [ ] No errors in logs
- [ ] SIP registrations working (if clients configured)
- [ ] Test call succeeds (if trunks configured)

---

**Next**: Read [README.md](README.md) for complete documentation.
