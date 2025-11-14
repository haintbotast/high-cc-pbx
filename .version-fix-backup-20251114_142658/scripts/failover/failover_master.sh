#!/bin/bash
# =============================================================================
# Keepalived MASTER Transition Script
# Description: Executed when node becomes MASTER
# =============================================================================

set -euo pipefail

LOGFILE="/var/log/voip-failover.log"
LOCKFILE="/var/lock/failover-master.lock"
STATE_FILE="/var/run/voip-master.state"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] MASTER: $1" | tee -a "$LOGFILE"
}

# Acquire lock (prevent race conditions)
exec 200>"$LOCKFILE"
flock -n 200 || {
    log "Another failover_master running, exiting"
    exit 1
}

log "========================================="
log "TRANSITIONING TO MASTER STATE"
log "========================================="
echo "MASTER" > "$STATE_FILE"

# Wait for peer to realize BACKUP
sleep 3

# 1. Promote PostgreSQL if standby
log "Checking PostgreSQL status..."
if sudo -u postgres test -f /var/lib/postgresql/16/main/standby.signal; then
    log "PostgreSQL is standby, promoting..."
    /usr/local/bin/postgres_failover.sh promote || log "ERROR: PostgreSQL promotion failed"
else
    log "PostgreSQL already primary"
fi

# 2. Ensure services are running
log "Ensuring services are running..."
systemctl start kamailio || log "ERROR: Kamailio start failed"
systemctl start freeswitch || log "ERROR: FreeSWITCH start failed"
systemctl start voip-admin || log "ERROR: VoIP Admin start failed"

# Wait for services to initialize
sleep 5

# 3. Health checks
log "Performing health checks..."
kamcmd core.uptime > /dev/null 2>&1 && log "✓ Kamailio: OK" || log "✗ Kamailio: FAIL"
fs_cli -x "status" | grep -q "UP" && log "✓ FreeSWITCH: OK" || log "✗ FreeSWITCH: FAIL"
sudo -u postgres psql -c "SELECT 1" > /dev/null 2>&1 && log "✓ PostgreSQL: OK" || log "✗ PostgreSQL: FAIL"
curl -s http://localhost:8080/health > /dev/null && log "✓ VoIP Admin: OK" || log "✗ VoIP Admin: FAIL"

log "========================================="
log "MASTER TRANSITION COMPLETE"
log "========================================="

flock -u 200
exit 0
