#!/bin/bash
#################################################################
# VoIP Master Health Check for Keepalived
# File: /usr/local/bin/check_voip_master.sh
# Based on production PostgreSQL health check
#
# Exit codes:
#   0 = All services healthy and node is MASTER
#   1 = Service unhealthy or node is STANDBY
#################################################################

set -euo pipefail

PSQL="psql"
PGPORT=5432
LOG_FILE="/var/log/keepalived_voip_check.log"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" >> "$LOG_FILE"
}

#################################################################
# 1. Check PostgreSQL Process
#################################################################
if ! pgrep -u postgres -f "postgres.*writer" > /dev/null; then
    log "FAIL: PostgreSQL process not running"
    exit 1
fi

#################################################################
# 2. Check PostgreSQL Port
#################################################################
if ! timeout 2 bash -c "echo > /dev/tcp/127.0.0.1/${PGPORT}" 2>/dev/null; then
    log "FAIL: PostgreSQL port $PGPORT not responding"
    exit 1
fi

#################################################################
# 3. Check PostgreSQL Role (MUST be master, not standby)
#################################################################
ROLE=$(sudo -u postgres $PSQL -p $PGPORT -qAt -c \
    "SELECT CASE WHEN pg_is_in_recovery() THEN 'standby' ELSE 'master' END;" \
    2>/dev/null || echo "error")

if [[ "$ROLE" != "master" ]]; then
    log "FAIL: Not master (role: ${ROLE})"
    exit 1
fi

#################################################################
# 4. PostgreSQL Write Test
#################################################################
if ! sudo -u postgres $PSQL -p $PGPORT -qAt -c \
    "BEGIN;
     CREATE TEMP TABLE health_check (id int);
     INSERT INTO health_check VALUES (1);
     ROLLBACK;" \
>/dev/null 2>&1; then
    log "FAIL: PostgreSQL write test failed"
    exit 1
fi

#################################################################
# 5. Check Kamailio
#################################################################
if ! systemctl is-active --quiet kamailio; then
    log "FAIL: Kamailio not running"
    exit 1
fi

# Check Kamailio is responding
if ! timeout 2 kamcmd core.uptime > /dev/null 2>&1; then
    log "FAIL: Kamailio not responding to kamcmd"
    exit 1
fi

#################################################################
# 6. Check FreeSWITCH
#################################################################
if ! systemctl is-active --quiet freeswitch; then
    log "FAIL: FreeSWITCH not running"
    exit 1
fi

# Check FreeSWITCH is responding
if ! timeout 2 fs_cli -x "status" 2>/dev/null | grep -q "UP"; then
    log "FAIL: FreeSWITCH not responding or not UP"
    exit 1
fi

#################################################################
# 7. Check VoIP Admin Service
#################################################################
if ! systemctl is-active --quiet voip-admin; then
    log "FAIL: VoIP Admin not running"
    exit 1
fi

# Check HTTP health endpoint
if ! curl -sf --max-time 2 http://localhost:8080/health > /dev/null 2>&1; then
    log "FAIL: VoIP Admin health endpoint not responding"
    exit 1
fi

#################################################################
# All checks passed
#################################################################
log "SUCCESS: All services healthy, node is MASTER"
exit 0
