#!/bin/bash
#################################################################
# Keepalived Unified Notify Script - PRODUCTION READY
# File: /usr/local/bin/keepalived_notify.sh
# Based on production PostgreSQL HA notify script
# Handles: MASTER, BACKUP, FAULT, STOP transitions
#################################################################

exec >> /var/log/keepalived_notify.log 2>&1

TYPE="${1:-UNKNOWN}"
NAME="${2:-UNKNOWN}"
STATE="${3:-UNKNOWN}"
PRIORITY="${4:-0}"

PSQL="/usr/bin/psql"
PGDATA="/var/lib/postgresql/16/main"
PG_CTL="/usr/lib/postgresql/16/bin/pg_ctl"

echo "========================================="
echo "[$(date '+%Y-%m-%d %H:%M:%S')] Notify"
echo "TYPE: $TYPE | NAME: $NAME | STATE: $STATE | PRIORITY: $PRIORITY"
echo "Host: $(hostname) | IP: $(hostname -I | awk '{print $1}')"
echo "========================================="

# Get current and peer IPs
CURRENT_IP=$(hostname -I | awk '{print $1}')
if [[ "$CURRENT_IP" == "172.16.91.101" ]]; then
    PEER_IP="172.16.91.102"
    MY_SLOT="standby_slot_101"
    PEER_SLOT="standby_slot_102"
elif [[ "$CURRENT_IP" == "172.16.91.102" ]]; then
    PEER_IP="172.16.91.101"
    MY_SLOT="standby_slot_102"
    PEER_SLOT="standby_slot_101"
else
    echo "ERROR: Cannot determine peer IP for $CURRENT_IP"
    exit 1
fi

case "$STATE" in
    MASTER)
        echo "→ Transition to MASTER"

        # Wait for VIP to stabilize
        sleep 2

        # Check if PostgreSQL is running
        if ! pgrep -u postgres -f "postgres.*writer" >/dev/null 2>&1; then
            echo "ERROR: PostgreSQL not running!"
            logger -t keepalived -p user.crit "PostgreSQL not running on new MASTER $(hostname)"
            exit 1
        fi

        # Check current role
        ROLE=$(sudo -u postgres $PSQL -p 5432 -qAt -c \
            "SELECT CASE WHEN pg_is_in_recovery() THEN 'standby' ELSE 'master' END;" \
            2>/dev/null || echo "error")

        echo "Current PostgreSQL role: $ROLE"

        if [[ "$ROLE" == "standby" ]]; then
            echo "Promoting PostgreSQL to MASTER..."

            if sudo -u postgres $PG_CTL promote -D "$PGDATA" -t 30; then
                echo "Promotion command executed"

                # Verify promotion completed
                for i in {1..30}; do
                    sleep 1
                    NEW_ROLE=$(sudo -u postgres $PSQL -p 5432 -qAt -c \
                        "SELECT CASE WHEN pg_is_in_recovery() THEN 'standby' ELSE 'master' END;" \
                        2>/dev/null || echo "error")

                    if [[ "$NEW_ROLE" == "master" ]]; then
                        echo "✓ PostgreSQL promoted successfully (${i}s)"
                        logger -t keepalived "PostgreSQL promoted to MASTER on $(hostname)"
                        break
                    fi
                done

                if [[ "$NEW_ROLE" != "master" ]]; then
                    echo "✗ Promotion timeout!"
                    logger -t keepalived -p user.crit "PostgreSQL promotion timeout on $(hostname)"
                fi
            else
                echo "✗ Promotion failed!"
                logger -t keepalived -p user.crit "PostgreSQL promotion failed on $(hostname)"
            fi

        elif [[ "$ROLE" == "master" ]]; then
            echo "✓ Already MASTER"
        else
            echo "✗ Cannot determine role: $ROLE"
        fi

        echo "---"
        echo "Checking replication slot for peer node..."

        # Check if we are actually master now
        CURRENT_ROLE=$(sudo -u postgres $PSQL -p 5432 -qAt -c \
            "SELECT CASE WHEN pg_is_in_recovery() THEN 'standby' ELSE 'master' END;" \
            2>/dev/null || echo "error")

        if [[ "$CURRENT_ROLE" == "master" ]]; then
            # Check if slot exists
            SLOT_EXISTS=$(sudo -u postgres $PSQL -p 5432 -qAt -c \
                "SELECT COUNT(*) FROM pg_replication_slots WHERE slot_name='$PEER_SLOT';" \
                2>/dev/null || echo "0")

            if [[ "$SLOT_EXISTS" -eq 0 ]]; then
                echo "Creating replication slot: $PEER_SLOT"
                if sudo -u postgres $PSQL -p 5432 -c \
                    "SELECT pg_create_physical_replication_slot('$PEER_SLOT');" \
>/dev/null 2>&1; then
                    echo "✓ Replication slot created: $PEER_SLOT"
                    logger -t keepalived "Created replication slot $PEER_SLOT on $(hostname)"
                else
                    echo "⚠ WARNING: Failed to create slot $PEER_SLOT"
                    logger -t keepalived -p user.warning "Failed to create slot $PEER_SLOT on $(hostname)"
                fi
            else
                echo "✓ Replication slot already exists: $PEER_SLOT"
            fi

            # Display current slots
            echo "Current replication slots:"
            sudo -u postgres $PSQL -p 5432 -c \
                "SELECT slot_name, slot_type, active, restart_lsn FROM pg_replication_slots;" \
                2>/dev/null || echo "  (unable to query slots)"
        else
            echo "⚠ Not creating slot - current role is: $CURRENT_ROLE"
        fi

        echo "---"
        echo "Ensuring VoIP services are running..."

        # Start services if not running
        systemctl is-active --quiet kamailio || systemctl start kamailio
        systemctl is-active --quiet freeswitch || systemctl start freeswitch
        systemctl is-active --quiet voip-admin || systemctl start voip-admin

        sleep 3

        # Verify services
        systemctl is-active --quiet kamailio && echo "✓ Kamailio: Running" || echo "✗ Kamailio: Failed"
        systemctl is-active --quiet freeswitch && echo "✓ FreeSWITCH: Running" || echo "✗ FreeSWITCH: Failed"
        systemctl is-active --quiet voip-admin && echo "✓ VoIP Admin: Running" || echo "✗ VoIP Admin: Failed"

        ;;

    BACKUP)
        echo "→ Transition to BACKUP"

        # Check if PostgreSQL is running
        if ! pgrep -u postgres -f "postgres.*writer" >/dev/null 2>&1; then
            echo "PostgreSQL not running, no action needed"
            exit 0
        fi

        # Check current role
        ROLE=$(sudo -u postgres $PSQL -p 5432 -qAt -c \
            "SELECT CASE WHEN pg_is_in_recovery() THEN 'standby' ELSE 'master' END;" \
            2>/dev/null || echo "error")

        echo "Current PostgreSQL role: $ROLE"

        if [[ "$ROLE" == "master" ]]; then
            echo "⚠ SPLIT-BRAIN DETECTED!"
            echo "⚠ PostgreSQL is MASTER but Keepalived is BACKUP"
            echo "⚠ Executing automatic rebuild as STANDBY..."
            logger -t keepalived -p user.crit "SPLIT-BRAIN: Auto-rebuilding as standby on $(hostname)"

            # Execute rebuild script
            /usr/local/bin/safe_rebuild_standby.sh "$PEER_IP" &

            # Script will run in background to avoid blocking keepalived
            echo "✓ Rebuild script started in background"

        elif [[ "$ROLE" == "standby" ]]; then
            echo "✓ Already STANDBY"

            # Verify replication is working
            REPLICATION_STATUS=$(sudo -u postgres $PSQL -p 5432 -qAt -c \
                "SELECT status FROM pg_stat_wal_receiver;" 2>/dev/null || echo "error")

            echo "Replication status: $REPLICATION_STATUS"

        else
            echo "✗ Cannot determine role: $ROLE"
        fi

        # Keep VoIP services running for graceful handling
        echo "VoIP services status:"
        systemctl is-active --quiet kamailio && echo "  Kamailio: Running" || echo "  Kamailio: Stopped"
        systemctl is-active --quiet freeswitch && echo "  FreeSWITCH: Running" || echo "  FreeSWITCH: Stopped"
        systemctl is-active --quiet voip-admin && echo "  VoIP Admin: Running" || echo "  VoIP Admin: Stopped"

        ;;

    FAULT)
        echo "→ FAULT state"
        logger -t keepalived -p user.crit "Keepalived FAULT on $(hostname)"

        # Log system status
        echo "System diagnostics:"
        uptime
        free -h
        df -h

        # Check service status
        echo "Service status:"
        systemctl is-active --quiet postgresql-16 && echo "  PostgreSQL: Running" || echo "  PostgreSQL: FAILED"
        systemctl is-active --quiet kamailio && echo "  Kamailio: Running" || echo "  Kamailio: FAILED"
        systemctl is-active --quiet freeswitch && echo "  FreeSWITCH: Running" || echo "  FreeSWITCH: FAILED"
        systemctl is-active --quiet voip-admin && echo "  VoIP Admin: Running" || echo "  VoIP Admin: FAILED"

        ;;

    STOP)
        echo "→ Keepalived stopping"
        ;;

    *)
        echo "✗ Unknown state: $STATE"
        exit 1
        ;;
esac

echo "========================================="
exit 0
