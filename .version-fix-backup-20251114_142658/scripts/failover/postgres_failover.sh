#!/bin/bash
# =============================================================================
# PostgreSQL Failover Script
# Description: Promotes standby to primary
# Usage: /usr/local/bin/postgres_failover.sh {promote|check}
# =============================================================================

set -euo pipefail

LOGFILE="/var/log/postgres-failover.log"
PGDATA="/var/lib/postgresql/16/main"
PGUSER="postgres"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOGFILE"
}

promote_to_primary() {
    log "============================================"
    log "PROMOTING STANDBY TO PRIMARY"
    log "============================================"

    # Check if already primary
    if ! sudo -u $PGUSER test -f "$PGDATA/standby.signal"; then
        log "ERROR: Already primary or standby.signal missing"
        return 1
    fi

    # Promote
    log "Executing pg_ctl promote..."
    sudo -u $PGUSER /usr/lib/postgresql/16/bin/pg_ctl promote -D "$PGDATA"

    # Wait for promotion (max 30 seconds)
    for i in {1..30}; do
        if ! sudo -u $PGUSER test -f "$PGDATA/standby.signal"; then
            log "SUCCESS: Promoted to primary (${i}s)"
            return 0
        fi
        sleep 1
    done

    log "ERROR: Promotion timeout"
    return 1
}

check_postgres_health() {
    sudo -u $PGUSER psql -c "SELECT 1" > /dev/null 2>&1
    return $?
}

# Main execution
case "${1:-}" in
    promote)
        promote_to_primary
        ;;
    check)
        if check_postgres_health; then
            log "PostgreSQL health check: OK"
            exit 0
        else
            log "PostgreSQL health check: FAILED"
            exit 1
        fi
        ;;
    *)
        echo "Usage: $0 {promote|check}"
        exit 1
        ;;
esac
