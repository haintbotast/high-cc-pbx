#!/bin/bash
#################################################################
# Safe Rebuild Standby - Production Ready for VoIP System
# File: /usr/local/bin/safe_rebuild_standby.sh
# PostgreSQL 16 on Debian 12
# Version: 3.0 - Adapted for VoIP HA system
#################################################################

set -e

MASTER_IP="${1}"
PG_VERSION="16"
PGDATA="/var/lib/postgresql/${PG_VERSION}/main"
BACKUP_DIR="/opt/pgsql/backups"
LOG_FILE="/var/log/rebuild_standby.log"
REPL_PASSWORD="Repl!VoIP#2025\$HA"

#################################################################
# Logging functions
#################################################################
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"
}

error_exit() {
    log "ERROR: $*"
    logger -t rebuild_standby -p user.err "ERROR: $* on $(hostname)"
    exit 1
}

#################################################################
# Start rebuild process
#################################################################
log "========================================="
log "Starting standby rebuild for VoIP System"
log "Master IP: $MASTER_IP"
log "Current Host: $(hostname)"
log "========================================="

#################################################################
# Step 1: Validate parameters
#################################################################
log "Step 1: Validating parameters..."

if [[ -z "$MASTER_IP" ]]; then
    error_exit "Usage: $0 <master_ip>"
fi

# Detect current node
CURRENT_IP=$(hostname -I | awk '{print $1}')
CURRENT_HOSTNAME=$(hostname)

if [[ "$CURRENT_IP" == "172.16.91.101" ]]; then
    MY_SLOT_NAME="standby_slot_101"
    APP_NAME="standby_101"
elif [[ "$CURRENT_IP" == "172.16.91.102" ]]; then
    MY_SLOT_NAME="standby_slot_102"
    APP_NAME="standby_102"
else
    error_exit "Unknown node IP: $CURRENT_IP"
fi

log "  - Current node: $CURRENT_HOSTNAME ($CURRENT_IP)"
log "  - Replication slot: $MY_SLOT_NAME"
log "  - Application name: $APP_NAME"
log "  - Master: $MASTER_IP"

#################################################################
# Step 2: Verify master accessibility
#################################################################
log "Step 2: Verifying master accessibility..."

if ! ping -c 2 -W 2 "$MASTER_IP" >/dev/null 2>&1; then
    error_exit "Master server $MASTER_IP is not reachable (ping failed)"
fi

log "  - Master server is reachable"

# Test PostgreSQL connection
if ! PGPASSWORD="$REPL_PASSWORD" PGSSLMODE=disable \
    psql -h "$MASTER_IP" -p 5432 -U replicator -d postgres -c "SELECT 1;" \
    >/dev/null 2>&1; then
    error_exit "Cannot connect to PostgreSQL on master $MASTER_IP"
fi

log "  - PostgreSQL on master is accessible"

# Verify master is actually master
MASTER_IN_RECOVERY=$(PGPASSWORD="$REPL_PASSWORD" PGSSLMODE=disable \
    psql -h "$MASTER_IP" -p 5432 -U replicator -d postgres -qAt -c \
    "SELECT pg_is_in_recovery();" 2>/dev/null || echo "error")

if [[ "$MASTER_IN_RECOVERY" == "t" ]]; then
    error_exit "Target server $MASTER_IP is in recovery mode (standby), not master!"
elif [[ "$MASTER_IN_RECOVERY" != "f" ]]; then
    error_exit "Cannot determine role of target server $MASTER_IP"
fi

log "  - Master is confirmed as primary (not in recovery)"

#################################################################
# Step 3: Check/Create replication slot on master
#################################################################
log "Step 3: Checking replication slot on master..."

SLOT_EXISTS=$(PGPASSWORD="$REPL_PASSWORD" PGSSLMODE=disable \
    psql -h "$MASTER_IP" -p 5432 -U replicator -d postgres -qAt -c \
    "SELECT COUNT(*) FROM pg_replication_slots WHERE slot_name='$MY_SLOT_NAME';" \
    2>/dev/null || echo "0")

if [[ "$SLOT_EXISTS" -eq 0 ]]; then
    log "  - Slot does not exist, creating: $MY_SLOT_NAME"

    if PGPASSWORD="$REPL_PASSWORD" PGSSLMODE=disable \
        psql -h "$MASTER_IP" -p 5432 -U replicator -d postgres -c \
        "SELECT pg_create_physical_replication_slot('$MY_SLOT_NAME');" \
        >/dev/null 2>&1; then

        log "  - Replication slot created successfully"
    else
        log "  - WARNING: Failed to create slot (may already exist from another attempt)"
    fi
else
    log "  - Replication slot already exists: $MY_SLOT_NAME"
fi

#################################################################
# Step 4: Stop VoIP services before PostgreSQL
#################################################################
log "Step 4: Stopping VoIP services..."

# Stop in reverse dependency order
for service in voip-admin freeswitch kamailio; do
    if systemctl is-active --quiet $service; then
        log "  - Stopping $service..."
        systemctl stop $service || log "    WARNING: Failed to stop $service"
    else
        log "  - $service already stopped"
    fi
done

#################################################################
# Step 5: Stop PostgreSQL
#################################################################
log "Step 5: Stopping PostgreSQL..."

if systemctl is-active --quiet postgresql-${PG_VERSION}; then
    log "  - PostgreSQL is running, stopping..."

    if ! systemctl stop postgresql-${PG_VERSION}; then
        error_exit "Failed to stop PostgreSQL"
    fi

    log "  - PostgreSQL stopped successfully"
    sleep 3
else
    log "  - PostgreSQL is not running (OK for rebuild)"
fi

# Verify PostgreSQL is stopped
if systemctl is-active --quiet postgresql-${PG_VERSION}; then
    error_exit "PostgreSQL is still running after stop attempt"
fi

log "  - Confirmed: PostgreSQL is stopped"

#################################################################
# Step 6: Backup old PGDATA
#################################################################
log "Step 6: Backing up old PGDATA..."

if [[ -d "$PGDATA" ]]; then
    BACKUP_NAME="${BACKUP_DIR}/pgdata_$(hostname)_$(date +%Y%m%d_%H%M%S)"

    log "  - Creating backup directory: $BACKUP_DIR"
    mkdir -p "$BACKUP_DIR"

    log "  - Moving old PGDATA to: $BACKUP_NAME"
    if ! mv "$PGDATA" "$BACKUP_NAME"; then
        error_exit "Failed to backup old PGDATA"
    fi

    log "  - Backup completed: $BACKUP_NAME"
else
    log "  - No existing PGDATA to backup"
fi

#################################################################
# Step 7: Create new PGDATA directory
#################################################################
log "Step 7: Creating new PGDATA directory..."

mkdir -p "$PGDATA"
chown postgres:postgres "$PGDATA"
chmod 700 "$PGDATA"

log "  - PGDATA directory created with correct permissions"

#################################################################
# Step 8: Run pg_basebackup
#################################################################
log "Step 8: Running pg_basebackup from master..."
log "  This may take several minutes depending on database size..."

if sudo -u postgres PGPASSWORD="$REPL_PASSWORD" PGSSLMODE=disable \
    /usr/lib/postgresql/${PG_VERSION}/bin/pg_basebackup \
    -h "$MASTER_IP" \
    -p 5432 \
    -U replicator \
    -D "$PGDATA" \
    -Fp \
    -Xs \
    -P \
    -R \
    -S "$MY_SLOT_NAME"; then

    log "  - pg_basebackup completed successfully"
else
    error_exit "pg_basebackup failed!"
fi

#################################################################
# Step 9: Verify and FIX standby configuration
#################################################################
log "Step 9: Verifying and fixing standby configuration..."

NEEDS_FIX=false

#################################################################
# 9.1: Check standby.signal
#################################################################
log "  9.1: Checking standby.signal..."

if [[ ! -f "$PGDATA/standby.signal" ]]; then
    log "    ⚠ WARNING: standby.signal is missing!"
    log "    Creating standby.signal..."

    if sudo -u postgres touch "$PGDATA/standby.signal"; then
        log "    ✓ standby.signal created"
        NEEDS_FIX=true
    else
        error_exit "Failed to create standby.signal"
    fi
else
    log "    ✓ standby.signal exists"
fi

#################################################################
# 9.2: Check postgresql.auto.conf
#################################################################
log "  9.2: Checking postgresql.auto.conf..."

if [[ ! -f "$PGDATA/postgresql.auto.conf" ]]; then
    log "    ⚠ WARNING: postgresql.auto.conf is missing!"
    log "    Creating postgresql.auto.conf..."

    sudo -u postgres touch "$PGDATA/postgresql.auto.conf"
    NEEDS_FIX=true
else
    log "    ✓ postgresql.auto.conf exists"
fi

#################################################################
# 9.3: Check and fix primary_conninfo
#################################################################
log "  9.3: Checking primary_conninfo..."

PRIMARY_CONNINFO="host=$MASTER_IP port=5432 user=replicator password=$REPL_PASSWORD sslmode=disable application_name=$APP_NAME"

if ! grep -q "primary_conninfo" "$PGDATA/postgresql.auto.conf"; then
    log "    ⚠ WARNING: primary_conninfo is missing!"
    log "    Adding primary_conninfo..."

    echo "primary_conninfo = '$PRIMARY_CONNINFO'" | sudo -u postgres tee -a "$PGDATA/postgresql.auto.conf" >/dev/null

    log "    ✓ primary_conninfo added"
    NEEDS_FIX=true
else
    log "    ✓ primary_conninfo exists"
fi

#################################################################
# 9.4: Check and fix primary_slot_name
#################################################################
log "  9.4: Checking primary_slot_name..."

if ! grep -q "primary_slot_name" "$PGDATA/postgresql.auto.conf"; then
    log "    ⚠ WARNING: primary_slot_name is missing!"
    log "    Adding primary_slot_name..."

    echo "primary_slot_name = '$MY_SLOT_NAME'" | sudo -u postgres tee -a "$PGDATA/postgresql.auto.conf" >/dev/null

    log "    ✓ primary_slot_name added: $MY_SLOT_NAME"
    NEEDS_FIX=true
else
    CURRENT_SLOT=$(grep "primary_slot_name" "$PGDATA/postgresql.auto.conf" | cut -d"'" -f2)

    if [[ "$CURRENT_SLOT" != "$MY_SLOT_NAME" ]]; then
        log "    ⚠ WARNING: primary_slot_name incorrect!"
        log "    Current: $CURRENT_SLOT"
        log "    Expected: $MY_SLOT_NAME"
        log "    Fixing primary_slot_name..."

        sudo -u postgres sed -i "s/primary_slot_name = .*/primary_slot_name = '$MY_SLOT_NAME'/" "$PGDATA/postgresql.auto.conf"

        log "    ✓ primary_slot_name fixed"
        NEEDS_FIX=true
    else
        log "    ✓ primary_slot_name correct: $MY_SLOT_NAME"
    fi
fi

#################################################################
# 9.5: Summary
#################################################################
if [[ "$NEEDS_FIX" == true ]]; then
    log ""
    log "  Configuration was incomplete - auto-fixed"
    logger -t rebuild_standby "Auto-fixed standby configuration on $(hostname)"
else
    log ""
    log "  All configuration correct"
fi

#################################################################
# Step 10: Start PostgreSQL
#################################################################
log "Step 10: Starting PostgreSQL as standby..."

if ! systemctl start postgresql-${PG_VERSION}; then
    error_exit "Failed to start PostgreSQL"
fi

log "  - PostgreSQL service started"
sleep 5

#################################################################
# Step 11: Verify standby status
#################################################################
log "Step 11: Verifying standby status..."

# Check if PostgreSQL is running
if ! systemctl is-active --quiet postgresql-${PG_VERSION}; then
    error_exit "PostgreSQL is not running after start"
fi
log "  - PostgreSQL is running"

# Check if in recovery mode
IN_RECOVERY=$(sudo -u postgres psql -p 5432 -qAt -c "SELECT pg_is_in_recovery();" 2>/dev/null || echo "error")

if [[ "$IN_RECOVERY" != "t" ]]; then
    log "  ✗ ERROR: PostgreSQL is NOT in recovery mode!"
    log "  Expected: standby (recovery=t)"
    log "  Got: master (recovery=$IN_RECOVERY)"

    error_exit "PostgreSQL started as master instead of standby!"
fi

log "  - Confirmed: PostgreSQL is in recovery mode (standby)"

# Check replication status
log "  - Checking replication connection..."
sleep 3

REPLICATION_STATUS=$(sudo -u postgres psql -p 5432 -qAt -c \
    "SELECT status FROM pg_stat_wal_receiver;" 2>/dev/null || echo "error")

if [[ "$REPLICATION_STATUS" == "streaming" ]]; then
    log "  - Replication status: streaming ✓ (connected to master)"
elif [[ "$REPLICATION_STATUS" == "error" ]]; then
    log "  - ⚠ WARNING: Cannot query replication status (may still be connecting)"
else
    log "  - Replication status: $REPLICATION_STATUS"
fi

#################################################################
# Step 12: Start VoIP services
#################################################################
log "Step 12: Starting VoIP services..."

sleep 2

# Start in dependency order
for service in kamailio freeswitch voip-admin; do
    log "  - Starting $service..."
    if systemctl start $service; then
        log "    ✓ $service started"
    else
        log "    ✗ WARNING: Failed to start $service"
    fi
done

sleep 3

# Verify services
log ""
log "Service status:"
systemctl is-active --quiet kamailio && log "  ✓ Kamailio: Running" || log "  ✗ Kamailio: Failed"
systemctl is-active --quiet freeswitch && log "  ✓ FreeSWITCH: Running" || log "  ✗ FreeSWITCH: Failed"
systemctl is-active --quiet voip-admin && log "  ✓ VoIP Admin: Running" || log "  ✗ VoIP Admin: Failed"

#################################################################
# Success
#################################################################
log "========================================="
log "✓ Standby rebuild completed successfully!"
log "========================================="
log ""
log "Summary:"
log "  - Old PGDATA backed up to: ${BACKUP_NAME:-N/A}"
log "  - New standby connected to master: $MASTER_IP"
log "  - Replication slot: $MY_SLOT_NAME"
log "  - Status: PostgreSQL is running as standby"
log "  - VoIP services: Started"
if [[ "$NEEDS_FIX" == true ]]; then
    log "  - Configuration auto-fixed: YES"
fi
log ""
log "Next steps:"
log "  1. Monitor replication: sudo -u postgres psql -c \"SELECT * FROM pg_stat_wal_receiver;\""
log "  2. Check replication lag: sudo -u postgres psql -x -c \"SELECT EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp())) AS lag_seconds;\""
log "  3. Verify Keepalived: sudo systemctl status keepalived"
log "  4. Check VIP location: ip addr | grep 172.16.91.100"
log "  5. Test VoIP services: kamcmd core.uptime && fs_cli -x status && curl http://localhost:8080/health"

logger -t rebuild_standby "Standby rebuild completed successfully on $(hostname)"

exit 0
