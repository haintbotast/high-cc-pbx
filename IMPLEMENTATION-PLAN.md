# IMPLEMENTATION PLAN - ACTIONABLE CHECKLIST

**Project**: High-Availability VoIP System (600-800 CC)
**Duration**: 6 months (25 weeks)
**Hardware**: 2 nodes × (16 cores, 64 GB RAM)
**Budget**: $7,000 hardware + $224,000 development

---

## PHASE 1: INFRASTRUCTURE SETUP (Weeks 1-2)

### Hardware Procurement
- [ ] Order Node 1: 16 cores, 64 GB RAM, 500 GB SSD + 3 TB HDD ($3,500-4,500)
- [ ] Order Node 2: Same specs as Node 1
- [ ] Order network equipment: 2× switches, cables
- [ ] Setup datacenter rack or colocation

### OS Installation (Both Nodes)
- [ ] Install Debian 12 on Node 1 (192.168.1.101)
- [ ] Install Debian 12 on Node 2 (192.168.1.102)
- [ ] Configure network interfaces (static IP, bonding if needed)
- [ ] Configure NTP time synchronization
- [ ] Apply OS security hardening (firewall, SELinux/AppArmor)
- [ ] Create system users: `voip-admin`, `freeswitch`, `kamailio`

### PostgreSQL Installation
- [ ] Node 1: Install PostgreSQL 16
  ```bash
  apt update
  apt install -y postgresql-16 postgresql-contrib-16
  ```
- [ ] Node 2: Install PostgreSQL 16
- [ ] Node 1: Create databases
  ```bash
  sudo -u postgres createdb voip_platform
  ```
- [ ] Node 1: Apply schema
  ```bash
  psql -U postgres voip_platform < database/schemas/01-voip-schema.sql
  ```
- [ ] Node 1: Configure `postgresql.conf`
  ```bash
  cp configs/postgresql/postgresql.conf /etc/postgresql/16/main/
  # Edit: listen_addresses, max_connections=300, shared_buffers=12GB
  ```
- [ ] Node 1: Configure `pg_hba.conf`
  ```bash
  cp configs/postgresql/pg_hba.conf /etc/postgresql/16/main/
  ```
- [ ] Node 1: Create replication user
  ```sql
  CREATE USER replicator WITH REPLICATION ENCRYPTED PASSWORD 'repl_password';
  ```
- [ ] Node 1: Restart PostgreSQL
- [ ] Node 2: Setup as standby
  ```bash
  systemctl stop postgresql
  rm -rf /var/lib/postgresql/16/main/*
  pg_basebackup -h 192.168.1.101 -U replicator -D /var/lib/postgresql/16/main -P -R
  touch /var/lib/postgresql/16/main/standby.signal
  systemctl start postgresql
  ```
- [ ] Verify replication
  ```bash
  # Node 1:
  sudo -u postgres psql -c "SELECT * FROM pg_stat_replication;"
  # Should show Node 2 connected
  ```

### Monitoring Setup
- [ ] Install Prometheus on monitoring server
- [ ] Install node_exporter on both nodes
- [ ] Install postgres_exporter on both nodes
- [ ] Install Grafana
- [ ] Create basic dashboards

**Phase 1 Milestone**: PostgreSQL HA cluster working with replication ✅

---

## PHASE 2: VIP & FAILOVER (Weeks 3-4)

### Keepalived Installation
- [ ] Node 1: Install keepalived
  ```bash
  apt install -y keepalived
  ```
- [ ] Node 2: Install keepalived
- [ ] Node 1: Deploy config
  ```bash
  cp configs/keepalived/keepalived-node1.conf /etc/keepalived/keepalived.conf
  # Edit: Set priority=100, state=MASTER
  ```
- [ ] Node 2: Deploy config
  ```bash
  cp configs/keepalived/keepalived-node2.conf /etc/keepalived/keepalived.conf
  # Edit: Set priority=50, state=BACKUP
  ```

### Failover Scripts
- [ ] Deploy `postgres_failover.sh` to both nodes
  ```bash
  cp scripts/failover/postgres_failover.sh /usr/local/bin/
  chmod +x /usr/local/bin/postgres_failover.sh
  ```
- [ ] Deploy `failover_master.sh` to both nodes
  ```bash
  cp scripts/failover/failover_master.sh /usr/local/bin/
  chmod +x /usr/local/bin/failover_master.sh
  ```
- [ ] Deploy `failover_backup.sh` to both nodes
- [ ] Deploy `failover_fault.sh` to both nodes
- [ ] Test failover scripts manually
  ```bash
  # Node 2:
  /usr/local/bin/postgres_failover.sh promote
  ```

### VIP Testing
- [ ] Start Keepalived on Node 1
  ```bash
  systemctl enable keepalived
  systemctl start keepalived
  ```
- [ ] Start Keepalived on Node 2
- [ ] Verify VIP is on Node 1
  ```bash
  ip addr show | grep 192.168.1.100
  ```
- [ ] Test failover: Stop keepalived on Node 1
  ```bash
  systemctl stop keepalived
  ```
- [ ] Verify VIP moved to Node 2
- [ ] Check logs
  ```bash
  tail -f /var/log/voip-failover.log
  ```

**Phase 2 Milestone**: Automatic VIP failover working ✅

---

## PHASE 3: KAMAILIO (Weeks 5-6)

### Kamailio Installation
- [ ] Node 1: Install Kamailio 6.0.x
  ```bash
  apt install -y kamailio kamailio-postgres-modules kamailio-utils-modules
  ```
- [ ] Node 2: Install Kamailio
- [ ] Node 1: Initialize Kamailio DB
  ```bash
  kamdbctl create  # Creates kamailio schema
  ```
- [ ] Both nodes: Deploy kamailio.cfg
  ```bash
  cp configs/kamailio/kamailio.cfg /etc/kamailio/
  # Edit: Set IP addresses, DB connection to VIP 192.168.1.100
  ```
- [ ] Both nodes: Configure db_mode=2 (write-back)
  ```cfg
  # In kamailio.cfg:
  modparam("usrloc", "db_mode", 2)
  modparam("usrloc", "db_url", "postgres://kamailio:password@192.168.1.100/voip_platform")
  ```
- [ ] Node 1: Insert FreeSWITCH into dispatcher
  ```sql
  INSERT INTO kamailio.dispatcher (setid, destination, flags, description)
  VALUES (1, 'sip:192.168.1.101:5080', 0, 'FreeSWITCH Node 1'),
         (1, 'sip:192.168.1.102:5080', 0, 'FreeSWITCH Node 2');
  ```
- [ ] Both nodes: Start Kamailio
  ```bash
  systemctl enable kamailio
  systemctl start kamailio
  ```
- [ ] Check Kamailio status
  ```bash
  kamcmd core.uptime
  kamcmd dispatcher.list
  ```

### SIP Testing
- [ ] Register SIP phone to VIP (192.168.1.100:5060)
- [ ] Check registration
  ```bash
  kamcmd ul.dump
  ```
- [ ] Register second SIP phone
- [ ] Test internal call (1001 → 1002)

**Phase 3 Milestone**: Kamailio registering users, routing calls ✅

---

## PHASE 4: FREESWITCH (Weeks 7-8)

### FreeSWITCH Installation
- [ ] Node 1: Install FreeSWITCH 1.10.x
  ```bash
  apt install -y freeswitch-meta-all freeswitch-mod-json-cdr
  apt install -y unixodbc odbc-postgresql
  ```
- [ ] Node 2: Install FreeSWITCH
- [ ] Both nodes: Create recording directories
  ```bash
  mkdir -p /storage/recordings /var/lib/freeswitch/recordings
  chown -R freeswitch:freeswitch /storage /var/lib/freeswitch
  ```
- [ ] Both nodes: Mount tmpfs for recordings
  ```bash
  echo "tmpfs /var/lib/freeswitch/recordings tmpfs defaults,size=30G,uid=freeswitch,gid=freeswitch 0 0" >> /etc/fstab
  mount -a
  ```

### FreeSWITCH Configuration
- [ ] Both nodes: Deploy switch.conf.xml
  ```bash
  cp configs/freeswitch/autoload_configs/switch.conf.xml /etc/freeswitch/autoload_configs/
  # Edit: ODBC connection to VIP
  ```
- [ ] Both nodes: Deploy sofia.conf.xml
  ```bash
  cp configs/freeswitch/autoload_configs/sofia.conf.xml /etc/freeswitch/autoload_configs/
  # Edit: Set sip-ip to node IP (Node1: .101, Node2: .102), port 5080
  ```
- [ ] Both nodes: Deploy json_cdr.conf.xml
  ```bash
  cp configs/freeswitch/autoload_configs/json_cdr.conf.xml /etc/freeswitch/autoload_configs/
  # Edit: url=http://192.168.1.100:8080/fs/cdr (VoIP Admin)
  ```
- [ ] Both nodes: Start FreeSWITCH
  ```bash
  systemctl enable freeswitch
  systemctl start freeswitch
  ```
- [ ] Check FreeSWITCH status
  ```bash
  fs_cli -x "status"
  ```

### lsyncd Setup (Recording Sync)
- [ ] Both nodes: Install lsyncd
  ```bash
  apt install -y lsyncd rsync
  ```
- [ ] Both nodes: Deploy rsyncd.conf
  ```bash
  cp configs/lsyncd/rsyncd.conf /etc/rsyncd.conf
  systemctl enable rsync
  systemctl start rsync
  ```
- [ ] Node 1: Deploy lsyncd config
  ```bash
  cp configs/lsyncd/lsyncd-node1.conf.lua /etc/lsyncd/lsyncd.conf.lua
  # Syncs to Node 2
  ```
- [ ] Node 2: Deploy lsyncd config
  ```bash
  cp configs/lsyncd/lsyncd-node2.conf.lua /etc/lsyncd/lsyncd.conf.lua
  # Syncs to Node 1
  ```
- [ ] Both nodes: Start lsyncd
  ```bash
  systemctl enable lsyncd
  systemctl start lsyncd
  ```
- [ ] Test recording sync (create test file, verify sync)

**Phase 4 Milestone**: FreeSWITCH processing calls, recordings syncing ✅

---

## PHASE 5: VOIP-ADMIN SERVICE (Weeks 9-16)

### Phase 5.1: Basic Structure (Weeks 9-10)
- [ ] Create Go project structure
  ```bash
  cd voip-admin
  go mod init voip-admin
  ```
- [ ] Implement config loader (`internal/config/config.go`)
- [ ] Implement PostgreSQL connection (`internal/database/postgres.go`)
- [ ] Implement HTTP server (`cmd/voipadmind/main.go`)
- [ ] Implement health endpoint (`GET /health`)
- [ ] Test locally
  ```bash
  go run cmd/voipadmind/main.go
  curl http://localhost:8080/health
  ```

### Phase 5.2: CDR Ingestion (Weeks 11-12)
- [ ] Implement CDR handler (`internal/freeswitch/cdr/handler.go`)
  ```go
  // POST /fs/cdr - receives JSON from FreeSWITCH
  // INSERT INTO voip.cdr_queue
  ```
- [ ] Implement CDR parser (`internal/freeswitch/cdr/parser.go`)
- [ ] Implement background worker (`internal/freeswitch/cdr/processor.go`)
  ```go
  // SELECT FROM voip.cdr_queue WHERE status='pending' LIMIT 100
  // Parse and batch INSERT INTO voip.cdr
  ```
- [ ] Test CDR ingestion
  ```bash
  # Make test call in FreeSWITCH
  # Check voip.cdr_queue and voip.cdr tables
  ```

### Phase 5.3: CDR Query API (Weeks 13-14)
- [ ] Implement CDR query handler (`internal/api/handlers/cdr.go`)
  ```go
  // GET /api/cdr?start_date=...&end_date=...
  ```
- [ ] Implement recording download (`internal/api/handlers/recordings.go`)
  ```go
  // GET /api/recordings/{id}/download
  ```
- [ ] Add API key authentication (`internal/api/middleware/auth.go`)
- [ ] Test API
  ```bash
  curl -H "X-API-Key: test_key" http://localhost:8080/api/cdr
  ```

### Phase 5.4: Management API (Weeks 15-16)
- [ ] Implement extensions API (`internal/api/handlers/extensions.go`)
  ```go
  // GET/POST/PUT/DELETE /api/extensions
  ```
- [ ] Implement queues API (`internal/api/handlers/queues.go`)
- [ ] Implement users API (`internal/api/handlers/users.go`)
- [ ] Implement in-memory cache (`internal/cache/memory.go`)
  ```go
  // Cache extension lookups (60s TTL)
  ```
- [ ] Add Prometheus metrics (`GET /metrics`)

### Deployment
- [ ] Build binary
  ```bash
  cd voip-admin
  make build
  ```
- [ ] Node 1: Deploy binary
  ```bash
  scp build/voipadmind node1:/opt/voip-admin/bin/
  ```
- [ ] Node 2: Deploy binary
- [ ] Both nodes: Deploy config
  ```bash
  cp configs/voip-admin/config.yaml /etc/voip-admin/
  # Edit: DB connection, ports
  ```
- [ ] Both nodes: Create systemd service
  ```bash
  cat > /etc/systemd/system/voip-admin.service <<EOF
  [Unit]
  Description=VoIP Admin Service
  After=postgresql.service

  [Service]
  Type=simple
  User=voip-admin
  ExecStart=/opt/voip-admin/bin/voipadmind -config /etc/voip-admin/config.yaml
  Restart=always

  [Install]
  WantedBy=multi-user.target
  EOF
  ```
- [ ] Both nodes: Start service
  ```bash
  systemctl daemon-reload
  systemctl enable voip-admin
  systemctl start voip-admin
  ```
- [ ] Test all endpoints

**Phase 5 Milestone**: voip-admin service fully functional ✅

---

## PHASE 6: ADVANCED FEATURES (Weeks 17-20)

### Call Queues
- [ ] Configure FreeSWITCH callcenter module
- [ ] Create queue in voip.queues table
  ```sql
  INSERT INTO voip.queues (domain_id, name, extension, strategy)
  VALUES (1, 'Support_L1', '8001', 'longest-idle');
  ```
- [ ] Add queue members
  ```sql
  INSERT INTO voip.queue_members (queue_id, user_id, tier)
  VALUES (1, 1, 1), (1, 2, 1);
  ```
- [ ] Create extension for queue
  ```sql
  INSERT INTO voip.extensions (domain_id, extension, type, queue_id, need_media)
  VALUES (1, '8001', 'queue', 1, true);
  ```
- [ ] Test queue call (dial 8001)

### IVR Menus
- [ ] Create IVR menu in voip.ivr_menus
- [ ] Create IVR entries (digit → action)
- [ ] Upload greeting audio files
- [ ] Create extension for IVR
- [ ] Test IVR (dial IVR extension)

### PSTN Trunking
- [ ] Create trunk in voip.trunks
- [ ] Create extension for outbound (prefix 0)
- [ ] Configure FreeSWITCH gateway
- [ ] Test outbound call

**Phase 6 Milestone**: Queues, IVR, and trunking working ✅

---

## PHASE 7: PRODUCTION HARDENING (Weeks 21-24)

### Security
- [ ] Move passwords to environment variables or secrets manager
- [ ] Enable TLS for PostgreSQL connections
- [ ] Enable TLS for SIP (if required)
- [ ] Configure firewall rules (iptables)
  ```bash
  # Allow: 5060 (SIP), 5080 (FS), 5432 (PG), 8080 (API), 16384-32768 (RTP)
  ```
- [ ] Install fail2ban for brute-force protection
- [ ] Disable root SSH login
- [ ] Setup SSH keys only

### Backup
- [ ] Create PostgreSQL backup script
  ```bash
  #!/bin/bash
  pg_basebackup -h 192.168.1.100 -U postgres -D /backup/pg-$(date +%Y%m%d) -P
  ```
- [ ] Setup WAL archiving
- [ ] Create recording backup script (rsync to off-site)
- [ ] Schedule backups (cron)
  ```bash
  # Daily at 2 AM
  0 2 * * * /usr/local/bin/backup_postgres.sh
  ```
- [ ] Test restore from backup

### Load Testing
- [ ] Install SIPp on test machine
- [ ] Create SIPp scenarios
  ```bash
  sipp -sf scenarios/call.xml -s 1001 192.168.1.100 -r 10 -l 100
  ```
- [ ] Test 100 CC
- [ ] Test 400 CC
- [ ] Test 600 CC
- [ ] Test 800 CC
- [ ] Record metrics (CPU, RAM, latency)

### Monitoring & Alerts
- [ ] Create Grafana dashboards
  - System overview (CPU, RAM, disk)
  - Call metrics (CC, CPS, latency)
  - Database metrics (connections, queries, replication lag)
- [ ] Configure alert rules
  - CPU >80%
  - RAM >90%
  - Replication lag >10s
  - Failover events
- [ ] Setup notifications (email, Slack, PagerDuty)

### Documentation
- [ ] Document final architecture (as-built)
- [ ] Create operations runbook
- [ ] Document troubleshooting procedures
- [ ] Create disaster recovery plan

**Phase 7 Milestone**: Production-ready system ✅

---

## PHASE 8: GO-LIVE (Week 25)

### Pre-production Checklist
- [ ] All services running and healthy
- [ ] Monitoring active and alerting
- [ ] Backups tested and working
- [ ] Load test passed (800 CC)
- [ ] Failover test passed
- [ ] Documentation complete
- [ ] Team trained

### Cutover (if from old system)
- [ ] Schedule maintenance window
- [ ] Backup old system
- [ ] Update DNS/routing to VIP 192.168.1.100
- [ ] Migrate users/extensions data
- [ ] Test sample calls
- [ ] Monitor for 24 hours

### Post-Go-Live
- [ ] 24-hour monitoring watch
- [ ] Performance validation
- [ ] Address any issues
- [ ] Collect feedback
- [ ] Optimize based on real traffic

**Phase 8 Milestone**: PRODUCTION LIVE ✅✅✅

---

## SUCCESS CRITERIA

### Technical
- [ ] 600-800 concurrent calls sustained
- [ ] Call setup latency <150ms
- [ ] Registration latency <50ms
- [ ] Failover RTO <45s
- [ ] 99.9% uptime over 30 days

### Operational
- [ ] Backups running daily and tested
- [ ] Monitoring dashboards operational
- [ ] Alerts configured and working
- [ ] Team can perform common operations

### Business
- [ ] Project delivered within 6 months
- [ ] Budget within ±10% ($300k total)
- [ ] Hardware savings $38,000 achieved
- [ ] Stakeholders satisfied

---

## RESOURCE TRACKER

### Team
- [ ] Solutions Architect assigned
- [ ] Database Administrator assigned
- [ ] VoIP Engineer assigned
- [ ] Backend Developer (Go) assigned
- [ ] DevOps Engineer assigned
- [ ] QA Engineer assigned

### Budget
- [ ] Hardware: $7,000-9,000 approved
- [ ] Development: $224,000 approved
- [ ] Contingency: $50,000 approved
- [ ] **Total: ~$300,000**

### Timeline
- Start Date: __________
- Expected Go-Live: __________ (25 weeks from start)
- Actual Go-Live: __________

---

**Use this document as your master checklist. Check off items as completed.**
**Update regularly in team meetings. Adjust timelines as needed.**

