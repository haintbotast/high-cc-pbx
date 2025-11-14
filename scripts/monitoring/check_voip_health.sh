#!/bin/bash
# =============================================================================
# VoIP Health Check Script for Keepalived
# Description: Comprehensive health check for all critical services
# Exit 0 = healthy, Exit 1 = unhealthy
# =============================================================================

set -euo pipefail

LOGFILE="/var/log/voip-health-check.log"
VERBOSE=${VERBOSE:-0}  # Set VERBOSE=1 for debug logging

log() {
    if [ "$VERBOSE" -eq 1 ]; then
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] HEALTH: $1" >> "$LOGFILE"
    fi
}

check_failed() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] HEALTH CHECK FAILED: $1" >> "$LOGFILE"
    exit 1
}

# 1. Check PostgreSQL
log "Checking PostgreSQL..."
if ! systemctl is-active --quiet postgresql; then
    check_failed "PostgreSQL service not running"
fi

# Check if PostgreSQL can accept connections
if ! sudo -u postgres psql -c "SELECT 1" > /dev/null 2>&1; then
    check_failed "PostgreSQL not accepting connections"
fi

# 2. Check Kamailio
log "Checking Kamailio..."
if ! systemctl is-active --quiet kamailio; then
    check_failed "Kamailio service not running"
fi

# Check Kamailio is responding to kamcmd
if ! timeout 2 kamcmd core.uptime > /dev/null 2>&1; then
    check_failed "Kamailio not responding to kamcmd"
fi

# 3. Check FreeSWITCH
log "Checking FreeSWITCH..."
if ! systemctl is-active --quiet freeswitch; then
    check_failed "FreeSWITCH service not running"
fi

# Check FreeSWITCH is responding to fs_cli
if ! timeout 2 fs_cli -x "status" 2>/dev/null | grep -q "UP"; then
    check_failed "FreeSWITCH not responding or not UP"
fi

# 4. Check VoIP Admin Service
log "Checking VoIP Admin..."
if ! systemctl is-active --quiet voip-admin; then
    check_failed "VoIP Admin service not running"
fi

# Check HTTP health endpoint
if ! curl -sf --max-time 2 http://localhost:8080/health > /dev/null 2>&1; then
    check_failed "VoIP Admin health endpoint not responding"
fi

# 5. Check disk space (fail if > 90% full)
log "Checking disk space..."
DISK_USAGE=$(df / | awk 'NR==2 {print $5}' | sed 's/%//')
if [ "$DISK_USAGE" -gt 90 ]; then
    check_failed "Disk usage critical: ${DISK_USAGE}%"
fi

# 6. Check memory (fail if < 5% free)
log "Checking memory..."
MEM_FREE=$(free | awk '/Mem:/ {printf "%.0f", ($7/$2)*100}')
if [ "$MEM_FREE" -lt 5 ]; then
    check_failed "Memory critical: only ${MEM_FREE}% available"
fi

log "Health check PASSED"
exit 0
