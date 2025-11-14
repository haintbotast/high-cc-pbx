##Interactive Configuration System

## Overview

Instead of hardcoding IP addresses, passwords, and other environment-specific values in configuration files, this system uses an **interactive wizard** to collect your specific values and generate customized configurations for each node.

## Why Interactive?

✅ **No hardcoded values** - Every deployment is different
✅ **Node-specific configs** - Automatically generates configs for Node 1 and Node 2
✅ **Secure password handling** - Passwords are prompted securely and not stored in git
✅ **Flexible** - Easy to regenerate configs if network changes
✅ **Production-ready** - Values are validated and properly escaped

---

## Quick Start

### Step 1: Run the Configuration Wizard

```bash
./scripts/setup/config_wizard.sh
```

This interactive script will ask you for:
- **Network**: IP addresses, VIP, network interface names
- **PostgreSQL**: Version (18), passwords for replication and application users
- **Keepalived**: VRRP settings, priorities
- **FreeSWITCH**: Ports, ESL password, RTP range
- **VoIP Admin**: HTTP port, API keys (auto-generated)

All values are saved to: `/tmp/voip-ha-config.env`

### Step 2: Generate Node-Specific Configurations

```bash
./scripts/setup/generate_configs.sh
```

This reads `/tmp/voip-ha-config.env` and generates:
```
generated-configs/
├── node1/
│   ├── keepalived/keepalived.conf        (with Node 1 IP, interface)
│   ├── postgresql/pg_hba.conf             (with your IPs)
│   ├── freeswitch/sofia.conf.xml          (with Node 1 IP - NOT VIP!)
│   ├── voip-admin/config.yaml             (with your passwords, API keys)
│   └── scripts/safe_rebuild_standby.sh    (pre-configured for Node 1)
├── node2/
│   ├── keepalived/keepalived.conf        (with Node 2 IP, interface)
│   ├── postgresql/pg_hba.conf             (with your IPs)
│   ├── freeswitch/sofia.conf.xml          (with Node 2 IP - NOT VIP!)
│   ├── voip-admin/config.yaml             (with your passwords, API keys)
│   └── scripts/safe_rebuild_standby.sh    (pre-configured for Node 2)
└── DEPLOY.md                              (deployment instructions with your IPs)
```

### Step 3: Deploy to Nodes

Follow the instructions in `generated-configs/DEPLOY.md` to copy configs to each node.

---

## Example Session

```bash
$ ./scripts/setup/config_wizard.sh
============================================================
  VoIP HA System - Configuration Wizard
============================================================

This wizard will help you configure your VoIP HA system.
All values will be saved to: /tmp/voip-ha-config.env

=== Network Configuration ===

Node 1 IP address [172.16.91.101]: 172.16.91.101
Node 2 IP address [172.16.91.102]: 172.16.91.102
Virtual IP (VIP) [172.16.91.100]: 172.16.91.100
Network mask (CIDR) [24]: 24

Node 1 network interface [ens33]: ens33
Node 2 network interface [ens33]: ens33

Node 1 hostname [voip-node1]: voip-node1
Node 2 hostname [voip-node2]: voip-node2

=== PostgreSQL Configuration ===

PostgreSQL version [18]: 18
PostgreSQL port [5432]: 5432
Node 1 PGDATA path [/var/lib/postgresql/18/main]:

PostgreSQL Passwords:
Replication password: ***********
Confirm password: ***********
Kamailio database password: ***********
...

=== Keepalived Configuration ===

VRRP Virtual Router ID [51]: 51
VRRP authentication password [Keepalv!VoIP#2025HA]:
...

✓ Configuration saved to: /tmp/voip-ha-config.env

Next steps:
  1. Review the configuration above
  2. Run: ./scripts/setup/generate_configs.sh
```

---

## Configuration File Format

The wizard saves to `/tmp/voip-ha-config.env` in shell variable format:

```bash
# Network
NODE1_IP="172.16.91.101"
NODE2_IP="172.16.91.102"
VIP="172.16.91.100"
NODE1_INTERFACE="ens33"
NODE2_INTERFACE="ens33"

# PostgreSQL
PG_VERSION="18"
PG_REPL_PASSWORD="your-secure-password"
PG_KAMAILIO_PASSWORD="another-secure-password"
...

# Auto-generated API keys
FS_API_KEY="randombase64string"
ADMIN_API_KEY="randombase64string"
```

This file is:
- ✅ **Not in git** (excluded via .gitignore)
- ✅ **Permissions 600** (owner read/write only)
- ✅ **Reusable** - Run wizard again to update values

---

## Template System

The `configs/` directory contains **template files** with placeholders:

```
configs/
├── keepalived/
│   ├── keepalived-node1.conf.template    ❌ OLD: Hardcoded IPs
│   └── keepalived-node2.conf.template    ❌ OLD: Hardcoded IPs
```

The **new system** generates configs from code using your values:

```bash
# In generate_configs.sh
cat > "$OUTPUT_DIR/node1/keepalived/keepalived.conf" << EOF
interface $NODE1_INTERFACE        # Your value from wizard
virtual_ipaddress {
    $VIP/$NETMASK dev $NODE1_INTERFACE
}
EOF
```

---

## PostgreSQL Version: 18 (Fixed)

The system now correctly uses **PostgreSQL 18** (matching your production setup), not 16.

All generated paths use:
- `/var/lib/postgresql/18/main` (PGDATA)
- `/usr/lib/postgresql/18/bin/` (binaries)
- `postgresql-18` (systemd service)

This matches your reference PostgreSQL HA configuration exactly.

---

## Key Design Decisions

### 1. No Hardcoded Values in Git
**Problem**: Your network uses 172.16.91.x, someone else uses 10.0.0.x
**Solution**: Wizard asks for IPs, generates node-specific configs

### 2. Node-Specific IPs in FreeSWITCH
**Problem**: FreeSWITCH sofia profile must bind to node IP, not VIP
**Solution**: `generate_configs.sh` creates:
- `node1/freeswitch/sofia.conf.xml` with `sip-ip="172.16.91.101"`
- `node2/freeswitch/sofia.conf.xml` with `sip-ip="172.16.91.102"`

### 3. Secure Password Handling
**Problem**: Passwords in git are a security risk
**Solution**:
- Wizard prompts with `read -s` (hidden input)
- Confirms password
- Saves to `/tmp/voip-ha-config.env` (not in git, 600 permissions)
- Generated configs contain actual passwords

### 4. Auto-Generated API Keys
**Problem**: Manually creating random API keys is error-prone
**Solution**: `openssl rand -base64 32` in wizard

### 5. Deployment Instructions with Your Values
**Problem**: Generic docs say "replace YOUR_IP with..."
**Solution**: `DEPLOY.md` contains actual commands with your IPs:
```bash
# Copy configs to Node 1 (172.16.91.101)
scp -r generated-configs/node1/* root@172.16.91.101:/tmp/voip-configs/
```

---

## Updating Configuration

If you need to change IPs, passwords, or any value:

```bash
# Run wizard again (it will load existing values as defaults)
./scripts/setup/config_wizard.sh

# Regenerate configs
./scripts/setup/generate_configs.sh

# Redeploy to nodes
# (follow new DEPLOY.md)
```

---

## Integration with Existing Scripts

The generated `safe_rebuild_standby.sh` has values embedded:

```bash
#!/bin/bash
# Auto-configured for Node 1

CURRENT_IP="172.16.91.101"
PEER_IP="172.16.91.102"
MY_SLOT_NAME="standby_slot_101"
PG_VERSION="18"
REPL_PASSWORD="your-actual-password"

# Rest of script logic...
```

This means:
- ✅ No manual editing needed
- ✅ Works immediately after deployment
- ✅ Node-aware (knows if it's 101 or 102)

---

## Security Best Practices

### ✅ What's Secure:
1. `/tmp/voip-ha-config.env` is chmod 600 (owner only)
2. Passwords prompted with `read -s` (no echo)
3. Generated configs can be deployed with secure copy
4. API keys use cryptographic random (openssl rand)

### ⚠️ What to Do:
1. **After deployment**: Delete `/tmp/voip-ha-config.env` on deployment machine
2. **Secure generated-configs/**: This directory contains passwords!
   ```bash
   chmod 700 generated-configs/
   rm -rf generated-configs/  # After deployment
   ```
3. **Rotate passwords**: Re-run wizard with new passwords, regenerate, redeploy

---

## Comparison: Old vs New

| Aspect | Old (Hardcoded) | New (Interactive) |
|--------|-----------------|-------------------|
| **IP Addresses** | 192.168.1.x in git | Asked by wizard |
| **PostgreSQL Version** | 16 (wrong!) | 18 (correct!) |
| **Passwords** | Placeholders in git | Securely prompted |
| **Network Interface** | `eth0` hardcoded | Asked by wizard |
| **FreeSWITCH IPs** | VIP (wrong!) | Node IPs (correct!) |
| **Node-Specific** | Manual editing | Auto-generated |
| **Deployment Docs** | Generic | Custom with your IPs |

---

## Files in This System

### User-Facing:
- `scripts/setup/config_wizard.sh` - Interactive wizard (you run this)
- `scripts/setup/generate_configs.sh` - Config generator (you run this)
- `generated-configs/DEPLOY.md` - Deployment instructions (auto-generated)

### Templates (Used by generate_configs.sh):
- None! Configs are generated programmatically from code

### Configuration Storage:
- `/tmp/voip-ha-config.env` - Your values (excluded from git)

---

## Troubleshooting

### "Configuration file not found"
```bash
$ ./scripts/setup/generate_configs.sh
ERROR: Configuration file not found: /tmp/voip-ha-config.env
```
**Solution**: Run `./scripts/setup/config_wizard.sh` first

### "Passwords don't match"
If you mistype during password confirmation, the wizard will ask again.

### "Permission denied" on /tmp/voip-ha-config.env
The file is chmod 600. This is correct. Run commands as the same user who ran the wizard.

---

## Next Steps

1. ✅ Run `./scripts/setup/config_wizard.sh`
2. ✅ Review `/tmp/voip-ha-config.env`
3. ✅ Run `./scripts/setup/generate_configs.sh`
4. ✅ Review `generated-configs/`
5. ✅ Follow `generated-configs/DEPLOY.md`

**The old hardcoded configs in `configs/` are now templates/examples only.**
**Use the interactive wizard for production deployments.**
