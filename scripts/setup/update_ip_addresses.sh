#!/bin/bash
#################################################################
# Update IP Addresses Script
# Description: Update all 192.168.1.x addresses to 172.16.91.x
# Usage: ./update_ip_addresses.sh
#################################################################

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "==================================================================="
echo "IP Address Update Script"
echo "==================================================================="
echo "Project root: $PROJECT_ROOT"
echo ""
echo "This script will update IP addresses:"
echo "  192.168.1.100 -> 172.16.91.100 (VIP)"
echo "  192.168.1.101 -> 172.16.91.101 (Node 1)"
echo "  192.168.1.102 -> 172.16.91.102 (Node 2)"
echo ""
read -p "Continue? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Aborted."
    exit 1
fi

echo ""
echo "Updating configuration files..."

# Function to update IPs in a file
update_file() {
    local file="$1"
    if [[ ! -f "$file" ]]; then
        echo "  ⚠ Skip: $file (not found)"
        return
    fi

    # Create backup
    cp "$file" "${file}.bak"

    # Replace IPs
    sed -i 's/192\.168\.1\.100/172.16.91.100/g' "$file"
    sed -i 's/192\.168\.1\.101/172.16.91.101/g' "$file"
    sed -i 's/192\.168\.1\.102/172.16.91.102/g' "$file"

    echo "  ✓ Updated: $file"
}

# PostgreSQL configs
echo ""
echo "[1/9] PostgreSQL configurations..."
update_file "$PROJECT_ROOT/configs/postgresql/pg_hba.conf"
update_file "$PROJECT_ROOT/configs/postgresql/recovery.conf.template"
update_file "$PROJECT_ROOT/database/schemas/01-voip-schema.sql"
update_file "$PROJECT_ROOT/database/schemas/02-kamailio-schema.sql"

# Keepalived configs
echo ""
echo "[2/9] Keepalived configurations..."
update_file "$PROJECT_ROOT/configs/keepalived/keepalived-node1.conf"
update_file "$PROJECT_ROOT/configs/keepalived/keepalived-node2.conf"

# lsyncd configs
echo ""
echo "[3/9] lsyncd configurations..."
update_file "$PROJECT_ROOT/configs/lsyncd/lsyncd-node1.conf.lua"
update_file "$PROJECT_ROOT/configs/lsyncd/lsyncd-node2.conf.lua"

# Kamailio configs
echo ""
echo "[4/9] Kamailio configurations..."
update_file "$PROJECT_ROOT/configs/kamailio/kamailio.cfg"

# FreeSWITCH configs
echo ""
echo "[5/9] FreeSWITCH configurations..."
update_file "$PROJECT_ROOT/configs/freeswitch/autoload_configs/switch.conf.xml"
update_file "$PROJECT_ROOT/configs/freeswitch/autoload_configs/xml_curl.conf.xml"
update_file "$PROJECT_ROOT/configs/freeswitch/autoload_configs/sofia.conf.xml"
update_file "$PROJECT_ROOT/configs/freeswitch/autoload_configs/cdr_pg_csv.conf.xml"

# VoIP Admin config
echo ""
echo "[6/9] VoIP Admin configuration..."
update_file "$PROJECT_ROOT/configs/voip-admin/config.yaml"

# Go application
echo ""
echo "[7/9] Go application..."
update_file "$PROJECT_ROOT/voip-admin/cmd/voipadmind/main.go"

# Scripts
echo ""
echo "[8/9] Failover scripts..."
update_file "$PROJECT_ROOT/scripts/failover/failover_master.sh"
update_file "$PROJECT_ROOT/scripts/failover/failover_backup.sh"

# Documentation
echo ""
echo "[9/9] Documentation..."
update_file "$PROJECT_ROOT/README.md"
update_file "$PROJECT_ROOT/IMPLEMENTATION-PLAN.md"

echo ""
echo "==================================================================="
echo "✓ IP address update complete!"
echo "==================================================================="
echo ""
echo "Backup files created with .bak extension"
echo "To restore: for f in \$(find . -name '*.bak'); do mv \"\$f\" \"\${f%.bak}\"; done"
echo ""
