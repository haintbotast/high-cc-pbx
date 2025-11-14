#!/bin/bash
#################################################################
# Generate Node-Specific Configurations
# Uses values from config_wizard.sh output
#################################################################

set -euo pipefail

CONFIG_FILE="/tmp/voip-ha-config.env"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
OUTPUT_DIR="$PROJECT_ROOT/generated-configs"

if [[ ! -f "$CONFIG_FILE" ]]; then
    echo "ERROR: Configuration file not found: $CONFIG_FILE"
    echo "Please run: ./scripts/setup/config_wizard.sh first"
    exit 1
fi

echo "============================================================"
echo "  Generating Node-Specific Configurations"
echo "============================================================"
echo ""
echo "Loading configuration from: $CONFIG_FILE"
source "$CONFIG_FILE"

# Create output directories
mkdir -p "$OUTPUT_DIR"/{node1,node2}/{keepalived,postgresql,freeswitch,kamailio,voip-admin,scripts}

echo ""
echo "Generating configurations..."

#################################################################
# Node 1 Keepalived
#################################################################
cat > "$OUTPUT_DIR/node1/keepalived/keepalived.conf" << EOF
#=====================================================================
# Keepalived Configuration - MASTER Node
# Node: $NODE1_HOSTNAME ($NODE1_IP)
# VIP: $VIP
# Generated: $(date)
#=====================================================================

global_defs {
    router_id VOIP_HA_${NODE1_IP##*.}
    enable_script_security
    script_user root
}

vrrp_script chk_voip_master {
    script "/usr/local/bin/check_voip_master.sh"
    interval 3
    timeout 5
    weight -30
    fall 2
    rise 2
}

vrrp_instance VI_VOIP {
    state MASTER
    interface $NODE1_INTERFACE
    virtual_router_id $VRRP_ROUTER_ID
    priority $NODE1_PRIORITY
    advert_int 1

    authentication {
        auth_type AH
        auth_pass $VRRP_PASSWORD
    }

    virtual_ipaddress {
        $VIP/$NETMASK dev $NODE1_INTERFACE label ${NODE1_INTERFACE}:vip
    }

    track_script {
        chk_voip_master
    }

    notify "/usr/local/bin/keepalived_notify.sh"
    nopreempt
}
EOF

#################################################################
# Node 2 Keepalived
#################################################################
cat > "$OUTPUT_DIR/node2/keepalived/keepalived.conf" << EOF
#=====================================================================
# Keepalived Configuration - BACKUP Node
# Node: $NODE2_HOSTNAME ($NODE2_IP)
# VIP: $VIP
# Generated: $(date)
#=====================================================================

global_defs {
    router_id VOIP_HA_${NODE2_IP##*.}
    enable_script_security
    script_user root
}

vrrp_script chk_voip_master {
    script "/usr/local/bin/check_voip_master.sh"
    interval 3
    timeout 5
    weight -30
    fall 2
    rise 2
}

vrrp_instance VI_VOIP {
    state BACKUP
    interface $NODE2_INTERFACE
    virtual_router_id $VRRP_ROUTER_ID
    priority $NODE2_PRIORITY
    advert_int 1

    authentication {
        auth_type AH
        auth_pass $VRRP_PASSWORD
    }

    virtual_ipaddress {
        $VIP/$NETMASK dev $NODE2_INTERFACE label ${NODE2_INTERFACE}:vip
    }

    track_script {
        chk_voip_master
    }

    notify "/usr/local/bin/keepalived_notify.sh"
    nopreempt
}
EOF

#################################################################
# PostgreSQL pg_hba.conf (same for both nodes)
#################################################################
for node in node1 node2; do
cat > "$OUTPUT_DIR/$node/postgresql/pg_hba.conf" << EOF
# PostgreSQL Client Authentication Configuration
# Generated: $(date)

# TYPE  DATABASE        USER            ADDRESS                 METHOD

# Local connections
local   all             postgres                                peer
local   all             all                                     peer

# Replication connections
host    replication     replicator      $NODE1_IP/32            scram-sha-256
host    replication     replicator      $NODE2_IP/32            scram-sha-256

# VoIP application connections
host    kamailio        kamailio        $NODE1_IP/32            scram-sha-256
host    kamailio        kamailio        $NODE2_IP/32            scram-sha-256
host    kamailio        kamailio        $VIP/32                 scram-sha-256  # VIP

host    voip            voipadmin       $NODE1_IP/32            scram-sha-256
host    voip            voipadmin       $NODE2_IP/32            scram-sha-256
host    voip            voipadmin       $VIP/32                 scram-sha-256  # VIP

host    voip            freeswitch      $NODE1_IP/32            scram-sha-256
host    voip            freeswitch      $NODE2_IP/32            scram-sha-256
host    voip            freeswitch      $VIP/32                 scram-sha-256  # VIP

# Localhost
host    all             all             127.0.0.1/32            scram-sha-256
host    all             all             ::1/128                 scram-sha-256

# Reject all others
host    all             all             0.0.0.0/0               reject
EOF
done

#################################################################
# FreeSWITCH sofia.conf.xml - Node 1
#################################################################
cat > "$OUTPUT_DIR/node1/freeswitch/sofia.conf.xml" << EOF
<configuration name="sofia.conf" description="Sofia SIP Configuration">
  <global_settings>
    <param name="log-level" value="0"/>
    <param name="auto-restart" value="true"/>
  </global_settings>

  <profiles>
    <profile name="internal">
      <settings>
        <param name="sip-ip" value="$NODE1_IP"/>
        <param name="sip-port" value="$FS_SIP_PORT"/>
        <param name="rtp-ip" value="$NODE1_IP"/>
        <param name="context" value="default"/>
        <param name="dialplan" value="XML"/>
        <param name="rtp-timeout-sec" value="300"/>
        <param name="rtp-hold-timeout-sec" value="1800"/>
        <param name="inbound-codec-prefs" value="OPUS,PCMU,PCMA,G729"/>
        <param name="outbound-codec-prefs" value="OPUS,PCMU,PCMA,G729"/>
        <param name="auth-calls" value="false"/>
        <param name="accept-blind-auth" value="true"/>
      </settings>
    </profile>
  </profiles>
</configuration>
EOF

#################################################################
# FreeSWITCH sofia.conf.xml - Node 2
#################################################################
cat > "$OUTPUT_DIR/node2/freeswitch/sofia.conf.xml" << EOF
<configuration name="sofia.conf" description="Sofia SIP Configuration">
  <global_settings>
    <param name="log-level" value="0"/>
    <param name="auto-restart" value="true"/>
  </global_settings>

  <profiles>
    <profile name="internal">
      <settings>
        <param name="sip-ip" value="$NODE2_IP"/>
        <param name="sip-port" value="$FS_SIP_PORT"/>
        <param name="rtp-ip" value="$NODE2_IP"/>
        <param name="context" value="default"/>
        <param name="dialplan" value="XML"/>
        <param name="rtp-timeout-sec" value="300"/>
        <param name="rtp-hold-timeout-sec" value="1800"/>
        <param name="inbound-codec-prefs" value="OPUS,PCMU,PCMA,G729"/>
        <param name="outbound-codec-prefs" value="OPUS,PCMU,PCMA,G729"/>
        <param name="auth-calls" value="false"/>
        <param name="accept-blind-auth" value="true"/>
      </settings>
    </profile>
  </profiles>
</configuration>
EOF

#################################################################
# VoIP Admin config.yaml (both nodes - connects to VIP)
#################################################################
for node in node1 node2; do
cat > "$OUTPUT_DIR/$node/voip-admin/config.yaml" << EOF
# VoIP Admin Service Configuration
# Generated: $(date)

server:
  host: "0.0.0.0"
  port: $VOIP_ADMIN_PORT

database:
  host: "$VIP"
  port: $PG_PORT
  user: "voipadmin"
  password: "$PG_VOIPADMIN_PASSWORD"
  dbname: "voip"
  max_open_conns: 50
  max_idle_conns: 25

freeswitch:
  esl:
    host: "127.0.0.1"
    port: $FS_ESL_PORT
    password: "$FS_ESL_PASSWORD"

  xml_curl:
    enabled: true
    auth_user: "freeswitch"
    auth_pass: "$FS_API_KEY"

api:
  keys:
    - name: "freeswitch"
      key: "$FS_API_KEY"
      permissions: ["xml_curl", "cdr"]
    - name: "admin"
      key: "$ADMIN_API_KEY"
      permissions: ["read", "write", "admin"]

metrics:
  enabled: true
  port: $VOIP_ADMIN_METRICS_PORT
EOF
done

#################################################################
# Generate helper scripts with embedded config
#################################################################
for node_num in 1 2; do
    if [[ $node_num -eq 1 ]]; then
        node="node1"
        node_ip="$NODE1_IP"
        peer_ip="$NODE2_IP"
        node_hostname="$NODE1_HOSTNAME"
        slot_name="standby_slot_${NODE1_IP##*.}"
        peer_slot="standby_slot_${NODE2_IP##*.}"
    else
        node="node2"
        node_ip="$NODE2_IP"
        peer_ip="$NODE1_IP"
        node_hostname="$NODE2_HOSTNAME"
        slot_name="standby_slot_${NODE2_IP##*.}"
        peer_slot="standby_slot_${NODE1_IP##*.}"
    fi

    # Create node-specific rebuild script
    cat > "$OUTPUT_DIR/$node/scripts/safe_rebuild_standby.sh" << 'SCRIPTEOF'
#!/bin/bash
# Safe Rebuild Standby - Auto-configured
set -e

# Auto-detected configuration
SCRIPTEOF

    cat >> "$OUTPUT_DIR/$node/scripts/safe_rebuild_standby.sh" << SCRIPTEOF
CURRENT_IP="$node_ip"
PEER_IP="$peer_ip"
MY_SLOT_NAME="$slot_name"
PG_VERSION="$PG_VERSION"
REPL_PASSWORD="$PG_REPL_PASSWORD"
SCRIPTEOF

    cat >> "$OUTPUT_DIR/$node/scripts/safe_rebuild_standby.sh" << 'SCRIPTEOF'
PGDATA="/var/lib/postgresql/${PG_VERSION}/main"
LOG_FILE="/var/log/rebuild_standby.log"

# Use first argument as master IP, or default to peer
MASTER_IP="${1:-$PEER_IP}"

echo "Rebuilding standby from master: $MASTER_IP"
echo "My slot: $MY_SLOT_NAME"

# Rest of rebuild logic here...
# (Full implementation from safe_rebuild_standby.sh)
SCRIPTEOF

    chmod +x "$OUTPUT_DIR/$node/scripts/safe_rebuild_standby.sh"
done

#################################################################
# Create deployment instructions
#################################################################
cat > "$OUTPUT_DIR/DEPLOY.md" << EOF
# Deployment Instructions

## Generated Configuration Summary

**VIP**: $VIP
**Node 1**: $NODE1_IP ($NODE1_HOSTNAME)
**Node 2**: $NODE2_IP ($NODE2_HOSTNAME)
**PostgreSQL Version**: $PG_VERSION

## Deploy to Node 1 ($NODE1_IP)

\`\`\`bash
# Copy configs
scp -r generated-configs/node1/* root@$NODE1_IP:/tmp/voip-configs/

# On Node 1:
cp /tmp/voip-configs/keepalived/keepalived.conf /etc/keepalived/
cp /tmp/voip-configs/postgresql/pg_hba.conf /var/lib/pgsql/$PG_VERSION/data/
cp /tmp/voip-configs/freeswitch/sofia.conf.xml /etc/freeswitch/autoload_configs/
cp /tmp/voip-configs/voip-admin/config.yaml /etc/voip-admin/
cp /tmp/voip-configs/scripts/* /usr/local/bin/
\`\`\`

## Deploy to Node 2 ($NODE2_IP)

\`\`\`bash
# Copy configs
scp -r generated-configs/node2/* root@$NODE2_IP:/tmp/voip-configs/

# On Node 2:
cp /tmp/voip-configs/keepalived/keepalived.conf /etc/keepalived/
cp /tmp/voip-configs/postgresql/pg_hba.conf /var/lib/pgsql/$PG_VERSION/data/
cp /tmp/voip-configs/freeswitch/sofia.conf.xml /etc/freeswitch/autoload_configs/
cp /tmp/voip-configs/voip-admin/config.yaml /etc/voip-admin/
cp /tmp/voip-configs/scripts/* /usr/local/bin/
\`\`\`

## Passwords

Save these securely:

- PostgreSQL Replication: \`$PG_REPL_PASSWORD\`
- Kamailio DB: \`$PG_KAMAILIO_PASSWORD\`
- VoIP Admin DB: \`$PG_VOIPADMIN_PASSWORD\`
- FreeSWITCH DB: \`$PG_FREESWITCH_PASSWORD\`
- FreeSWITCH ESL: \`$FS_ESL_PASSWORD\`
- FreeSWITCH API Key: \`$FS_API_KEY\`
- Admin API Key: \`$ADMIN_API_KEY\`
- VRRP Password: \`$VRRP_PASSWORD\`
EOF

echo ""
echo "============================================================"
echo "✓ Configuration generation complete!"
echo "============================================================"
echo ""
echo "Output directory: $OUTPUT_DIR"
echo ""
echo "Generated files:"
find "$OUTPUT_DIR" -type f | sed 's|^|  - |'
echo ""
echo "Next steps:"
echo "  1. Review generated configs in: $OUTPUT_DIR"
echo "  2. Follow deployment instructions in: $OUTPUT_DIR/DEPLOY.md"
echo "============================================================"
