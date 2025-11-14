#!/bin/bash
# =============================================================================
# Keepalived BACKUP Transition Script
# Description: Executed when node becomes BACKUP
# =============================================================================

set -euo pipefail

LOGFILE="/var/log/voip-failover.log"
LOCKFILE="/var/lock/failover-backup.lock"
STATE_FILE="/var/run/voip-backup.state"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] BACKUP: $1" | tee -a "$LOGFILE"
}

# Acquire lock (prevent race conditions)
exec 200>"$LOCKFILE"
flock -n 200 || {
    log "Another failover_backup running, exiting"
    exit 1
}

log "========================================="
log "TRANSITIONING TO BACKUP STATE"
log "========================================="
echo "BACKUP" > "$STATE_FILE"

# Wait for peer to become MASTER
sleep 3

# 1. Ensure PostgreSQL is in standby mode
log "Checking PostgreSQL status..."
if sudo -u postgres test -f /var/lib/postgresql/16/main/standby.signal; then
    log "PostgreSQL is already standby"
else
    log "WARNING: PostgreSQL is primary but node is BACKUP"
    log "This should not happen - manual intervention may be required"
fi

# 2. Keep services running (for graceful existing connections)
log "Keeping services running for graceful degradation..."
systemctl status kamailio > /dev/null 2>&1 && log "✓ Kamailio: Running" || log "✗ Kamailio: Stopped"
systemctl status freeswitch > /dev/null 2>&1 && log "✓ FreeSWITCH: Running" || log "✗ FreeSWITCH: Stopped"
systemctl status voip-admin > /dev/null 2>&1 && log "✓ VoIP Admin: Running" || log "✗ VoIP Admin: Stopped"

# Note: We keep services running in BACKUP mode to allow:
# - Existing SIP registrations to complete
# - Active calls to finish gracefully
# - Database replication to continue

log "========================================="
log "BACKUP TRANSITION COMPLETE"
log "========================================="

flock -u 200
exit 0
