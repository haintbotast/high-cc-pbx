#!/bin/bash
#################################################################
# VoIP HA Configuration Wizard
# Interactive script to generate node-specific configurations
#################################################################

set -euo pipefail

CONFIG_FILE="/tmp/voip-ha-config.env"

echo "============================================================"
echo "  VoIP HA System - Configuration Wizard"
echo "============================================================"
echo ""
echo "This wizard will help you configure your VoIP HA system."
echo "All values will be saved to: $CONFIG_FILE"
echo ""

# Function to prompt for input with default
prompt() {
    local var_name="$1"
    local prompt_text="$2"
    local default_value="$3"
    local current_value="${!var_name:-$default_value}"
    
    read -p "$prompt_text [$current_value]: " input
    eval "$var_name=\"${input:-$current_value}\""
}

# Function to prompt for password
prompt_password() {
    local var_name="$1"
    local prompt_text="$2"
    local password=""
    
    while true; do
        read -s -p "$prompt_text: " password
        echo ""
        read -s -p "Confirm password: " password2
        echo ""
        
        if [[ "$password" == "$password2" ]]; then
            eval "$var_name=\"$password\""
            break
        else
            echo "Passwords don't match. Try again."
        fi
    done
}

# Load existing config if available
if [[ -f "$CONFIG_FILE" ]]; then
    echo "Loading existing configuration from $CONFIG_FILE"
    source "$CONFIG_FILE"
    echo ""
fi

#################################################################
# Network Configuration
#################################################################
echo "=== Network Configuration ==="
echo ""

prompt NODE1_IP "Node 1 IP address" "172.16.91.101"
prompt NODE2_IP "Node 2 IP address" "172.16.91.102"
prompt VIP "Virtual IP (VIP)" "172.16.91.100"
prompt NETMASK "Network mask (CIDR)" "24"

echo ""
prompt NODE1_INTERFACE "Node 1 network interface" "ens33"
prompt NODE2_INTERFACE "Node 2 network interface" "ens33"

echo ""
prompt NODE1_HOSTNAME "Node 1 hostname" "voip-node1"
prompt NODE2_HOSTNAME "Node 2 hostname" "voip-node2"

#################################################################
# PostgreSQL Configuration
#################################################################
echo ""
echo "=== PostgreSQL Configuration ==="
echo ""

prompt PG_VERSION "PostgreSQL version" "18"
prompt PG_PORT "PostgreSQL port" "5432"
prompt PGDATA_NODE1 "Node 1 PGDATA path" "/var/lib/pgsql/${PG_VERSION}/data"
prompt PGDATA_NODE2 "Node 2 PGDATA path" "/var/lib/pgsql/${PG_VERSION}/data"

echo ""
echo "PostgreSQL Passwords:"
prompt_password PG_REPL_PASSWORD "Replication password"
prompt_password PG_KAMAILIO_PASSWORD "Kamailio database password"
prompt_password PG_VOIPADMIN_PASSWORD "VoIP Admin database password"
prompt_password PG_FREESWITCH_PASSWORD "FreeSWITCH database password"

#################################################################
# Keepalived Configuration
#################################################################
echo ""
echo "=== Keepalived Configuration ==="
echo ""

prompt VRRP_ROUTER_ID "VRRP Virtual Router ID" "51"
prompt VRRP_PASSWORD "VRRP authentication password" "Keepalv!VoIP#2025HA"
prompt NODE1_PRIORITY "Node 1 priority" "100"
prompt NODE2_PRIORITY "Node 2 priority" "90"

#################################################################
# FreeSWITCH Configuration
#################################################################
echo ""
echo "=== FreeSWITCH Configuration ==="
echo ""

prompt FS_SIP_PORT "FreeSWITCH SIP port" "5080"
prompt FS_ESL_PORT "FreeSWITCH ESL port" "8021"
prompt_password FS_ESL_PASSWORD "FreeSWITCH ESL password"
prompt FS_RTP_START "RTP port range start" "16384"
prompt FS_RTP_END "RTP port range end" "32768"

#################################################################
# Kamailio Configuration
#################################################################
echo ""
echo "=== Kamailio Configuration ==="
echo ""

prompt KAM_SIP_PORT "Kamailio SIP port" "5060"

#################################################################
# VoIP Admin Service Configuration
#################################################################
echo ""
echo "=== VoIP Admin Service Configuration ==="
echo ""

prompt VOIP_ADMIN_PORT "VoIP Admin HTTP port" "8080"
prompt VOIP_ADMIN_METRICS_PORT "Prometheus metrics port" "9090"

# Generate API keys
echo ""
echo "Generating API keys..."
FS_API_KEY=$(openssl rand -base64 32 | tr -d '/+=' | cut -c1-32)
ADMIN_API_KEY=$(openssl rand -base64 32 | tr -d '/+=' | cut -c1-32)
echo "  FreeSWITCH API Key: $FS_API_KEY"
echo "  Admin API Key: $ADMIN_API_KEY"

#################################################################
# Save Configuration
#################################################################
echo ""
echo "=== Saving Configuration ==="
echo ""

cat > "$CONFIG_FILE" << ENVEOF
# VoIP HA System Configuration
# Generated: $(date)

# Network
NODE1_IP="$NODE1_IP"
NODE2_IP="$NODE2_IP"
VIP="$VIP"
NETMASK="$NETMASK"
NODE1_INTERFACE="$NODE1_INTERFACE"
NODE2_INTERFACE="$NODE2_INTERFACE"
NODE1_HOSTNAME="$NODE1_HOSTNAME"
NODE2_HOSTNAME="$NODE2_HOSTNAME"

# PostgreSQL
PG_VERSION="$PG_VERSION"
PG_PORT="$PG_PORT"
PGDATA_NODE1="$PGDATA_NODE1"
PGDATA_NODE2="$PGDATA_NODE2"
PG_REPL_PASSWORD="$PG_REPL_PASSWORD"
PG_KAMAILIO_PASSWORD="$PG_KAMAILIO_PASSWORD"
PG_VOIPADMIN_PASSWORD="$PG_VOIPADMIN_PASSWORD"
PG_FREESWITCH_PASSWORD="$PG_FREESWITCH_PASSWORD"

# Keepalived
VRRP_ROUTER_ID="$VRRP_ROUTER_ID"
VRRP_PASSWORD="$VRRP_PASSWORD"
NODE1_PRIORITY="$NODE1_PRIORITY"
NODE2_PRIORITY="$NODE2_PRIORITY"

# FreeSWITCH
FS_SIP_PORT="$FS_SIP_PORT"
FS_ESL_PORT="$FS_ESL_PORT"
FS_ESL_PASSWORD="$FS_ESL_PASSWORD"
FS_RTP_START="$FS_RTP_START"
FS_RTP_END="$FS_RTP_END"
FS_API_KEY="$FS_API_KEY"

# Kamailio
KAM_SIP_PORT="$KAM_SIP_PORT"

# VoIP Admin
VOIP_ADMIN_PORT="$VOIP_ADMIN_PORT"
VOIP_ADMIN_METRICS_PORT="$VOIP_ADMIN_METRICS_PORT"
ADMIN_API_KEY="$ADMIN_API_KEY"
ENVEOF

chmod 600 "$CONFIG_FILE"

echo "✓ Configuration saved to: $CONFIG_FILE"
echo ""
echo "=== Configuration Summary ==="
cat "$CONFIG_FILE"
echo ""
echo "============================================================"
echo "Next steps:"
echo "  1. Review the configuration above"
echo "  2. Run: ./scripts/setup/generate_configs.sh"
echo "     This will generate node-specific configs using these values"
echo "  3. Deploy to nodes using: ./scripts/setup/deploy_node.sh"
echo "============================================================"
