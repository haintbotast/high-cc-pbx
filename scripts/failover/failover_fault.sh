#!/bin/bash
# =============================================================================
# Keepalived FAULT State Script
# Description: Executed when node enters FAULT state (health check failed)
# =============================================================================

set -euo pipefail

LOGFILE="/var/log/voip-failover.log"
LOCKFILE="/var/lock/failover-fault.lock"
STATE_FILE="/var/run/voip-fault.state"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] FAULT: $1" | tee -a "$LOGFILE"
}

# Acquire lock (prevent race conditions)
exec 200>"$LOCKFILE"
flock -n 200 || {
    log "Another failover_fault running, exiting"
    exit 1
}

log "========================================="
log "ENTERING FAULT STATE - CRITICAL"
log "========================================="
echo "FAULT" > "$STATE_FILE"

# 1. Log system status
log "Gathering diagnostics..."
uptime >> "$LOGFILE"
free -h >> "$LOGFILE"
df -h >> "$LOGFILE"

# 2. Check service status
log "Checking service status..."
systemctl status kamailio > /dev/null 2>&1 && log "✓ Kamailio: Running" || log "✗ Kamailio: FAILED"
systemctl status freeswitch > /dev/null 2>&1 && log "✓ FreeSWITCH: Running" || log "✗ FreeSWITCH: FAILED"
systemctl status postgresql > /dev/null 2>&1 && log "✓ PostgreSQL: Running" || log "✗ PostgreSQL: FAILED"
systemctl status voip-admin > /dev/null 2>&1 && log "✓ VoIP Admin: Running" || log "✗ VoIP Admin: FAILED"

# 3. Attempt service recovery
log "Attempting automatic service recovery..."

if ! systemctl is-active --quiet kamailio; then
    log "Attempting to restart Kamailio..."
    systemctl restart kamailio && log "✓ Kamailio restarted" || log "✗ Kamailio restart failed"
fi

if ! systemctl is-active --quiet freeswitch; then
    log "Attempting to restart FreeSWITCH..."
    systemctl restart freeswitch && log "✓ FreeSWITCH restarted" || log "✗ FreeSWITCH restart failed"
fi

if ! systemctl is-active --quiet postgresql; then
    log "Attempting to restart PostgreSQL..."
    systemctl restart postgresql && log "✓ PostgreSQL restarted" || log "✗ PostgreSQL restart failed"
fi

if ! systemctl is-active --quiet voip-admin; then
    log "Attempting to restart VoIP Admin..."
    systemctl restart voip-admin && log "✓ VoIP Admin restarted" || log "✗ VoIP Admin restart failed"
fi

# 4. Send alert (optional - configure email/webhook)
# Uncomment and configure as needed:
# echo "VoIP Node entered FAULT state at $(date)" | mail -s "CRITICAL: VoIP Failover FAULT" admin@example.com

log "========================================="
log "FAULT STATE RECOVERY ATTEMPTED"
log "Manual intervention may be required"
log "========================================="

flock -u 200
exit 0
