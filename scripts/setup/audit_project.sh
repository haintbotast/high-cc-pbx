#!/bin/bash
#################################################################
# Project Audit Script
# Comprehensive check of configuration, versions, and structure
#################################################################

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$PROJECT_ROOT"

ERRORS=0
WARNINGS=0

echo "============================================================"
echo "  VoIP HA Project Audit"
echo "============================================================"
echo "Project root: $PROJECT_ROOT"
echo ""

#################################################################
# 1. PostgreSQL Version Check
#################################################################
echo "=== 1. PostgreSQL Version Check ==="
echo ""

echo "Checking for incorrect PostgreSQL 16 references..."
PG16_FILES=$(find . -type f \
    \( -name "*.sh" -o -name "*.md" -o -name "*.conf" -o -name "*.xml" -o -name "*.yaml" \) \
    -not -path "./archive/*" \
    -not -path "./.git/*" \
    -not -path "./.version-fix-backup*/*" \
    -not -name "*.bak" \
    -exec grep -l "postgresql-16\|postgresql/16\|pgsql-16\|pgsql/16" {} \; 2>/dev/null | \
    grep -v "fix_postgresql_version.sh" | \
    grep -v "audit_project.sh" || true)

if [[ -n "$PG16_FILES" ]]; then
    echo "❌ ERROR: Found PostgreSQL 16 references:"
    echo "$PG16_FILES" | sed 's/^/    /'
    ((ERRORS++))
else
    echo "✓ No PostgreSQL 16 references found"
fi

echo ""
echo "Checking for PostgreSQL 18 references..."
PG18_COUNT=$(grep -r "postgresql-18\|pgsql-18\|PostgreSQL 18" . \
    --exclude-dir=archive \
    --exclude-dir=.git \
    --exclude-dir=.version-fix-backup* \
    --include="*.sh" --include="*.md" \
    2>/dev/null | wc -l)

echo "✓ Found $PG18_COUNT PostgreSQL 18 references"

#################################################################
# 2. Path Consistency Check
#################################################################
echo ""
echo "=== 2. Path Consistency Check ==="
echo ""

# Check for common PostgreSQL paths
echo "Checking PostgreSQL binary paths..."
WRONG_PATHS=$(grep -r "/usr/lib/postgresql/18\|/etc/postgresql/18\|/var/lib/postgresql/18" . \
    --exclude-dir=archive \
    --exclude-dir=.git \
    --exclude-dir=.version-fix-backup* \
    --include="*.sh" --include="*.md" --include="*.conf" \
    2>/dev/null | grep -v "# Debian" || true)

if [[ -n "$WRONG_PATHS" ]]; then
    echo "⚠ WARNING: Found Debian-style PostgreSQL paths (should be RHEL-style):"
    echo "$WRONG_PATHS" | sed 's/^/    /' | head -5
    echo "    Note: RHEL/CentOS uses /var/lib/postgresql/18/ not /var/lib/postgresql/18/"
    ((WARNINGS++))
fi

# Check for correct RHEL paths
RHEL_PATHS=$(grep -r "/var/lib/postgresql/18\|/usr/lib/postgresql/18" . \
    --exclude-dir=archive \
    --exclude-dir=.git \
    --exclude-dir=.version-fix-backup* \
    --include="*.sh" \
    2>/dev/null | wc -l)

echo "✓ Found $RHEL_PATHS RHEL-style PostgreSQL paths"

#################################################################
# 3. IP Address Consistency
#################################################################
echo ""
echo "=== 3. IP Address Consistency Check ==="
echo ""

echo "Checking for 192.168.1.x addresses (old)..."
OLD_IPS=$(grep -r "192\.168\.1\." . \
    --exclude-dir=archive \
    --exclude-dir=.git \
    --exclude-dir=.version-fix-backup* \
    --include="*.sh" --include="*.md" --include="*.conf" --include="*.xml" \
    2>/dev/null | grep -v "update_ip_addresses.sh" || true)

if [[ -n "$OLD_IPS" ]]; then
    echo "⚠ WARNING: Found old 192.168.1.x IP addresses:"
    echo "$OLD_IPS" | sed 's/^/    /' | head -5
    ((WARNINGS++))
else
    echo "✓ No old 192.168.1.x addresses found"
fi

echo ""
echo "Checking for 172.16.91.x addresses (current)..."
CURRENT_IPS=$(grep -r "172\.16\.91\." . \
    --exclude-dir=archive \
    --exclude-dir=.git \
    --exclude-dir=.version-fix-backup* \
    --include="*.sh" --include="*.conf" --include="*.xml" \
    2>/dev/null | wc -l)

echo "✓ Found $CURRENT_IPS references to 172.16.91.x"

#################################################################
# 4. Script Permissions Check
#################################################################
echo ""
echo "=== 4. Script Permissions Check ==="
echo ""

echo "Checking for non-executable .sh files..."
NON_EXEC=$(find scripts/ -name "*.sh" ! -perm -111 2>/dev/null || true)

if [[ -n "$NON_EXEC" ]]; then
    echo "❌ ERROR: Found non-executable scripts:"
    echo "$NON_EXEC" | sed 's/^/    /'
    ((ERRORS++))
else
    echo "✓ All .sh files are executable"
fi

#################################################################
# 5. Documentation Structure Check
#################################################################
echo ""
echo "=== 5. Documentation Structure Check ==="
echo ""

# Count markdown files in root
MD_ROOT=$(find . -maxdepth 1 -name "*.md" | wc -l)
echo "Markdown files in root: $MD_ROOT"

if [[ $MD_ROOT -gt 10 ]]; then
    echo "⚠ WARNING: Too many markdown files in root ($MD_ROOT)"
    echo "    Consider consolidating or moving to docs/"
    ((WARNINGS++))
fi

# Check for essential docs
ESSENTIAL_DOCS=(
    "README.md"
    "INTERACTIVE-SETUP.md"
    "IMPLEMENTATION-PLAN.md"
)

for doc in "${ESSENTIAL_DOCS[@]}"; do
    if [[ -f "$doc" ]]; then
        echo "✓ $doc exists"
    else
        echo "❌ ERROR: Missing essential doc: $doc"
        ((ERRORS++))
    fi
done

#################################################################
# 6. Configuration File Structure Check
#################################################################
echo ""
echo "=== 6. Configuration File Structure Check ==="
echo ""

# Check for required config directories
CONFIG_DIRS=(
    "configs/postgresql"
    "configs/keepalived"
    "configs/kamailio"
    "configs/freeswitch"
    "configs/lsyncd"
    "configs/voip-admin"
)

for dir in "${CONFIG_DIRS[@]}"; do
    if [[ -d "$dir" ]]; then
        file_count=$(find "$dir" -type f | wc -l)
        echo "✓ $dir exists ($file_count files)"
    else
        echo "❌ ERROR: Missing config directory: $dir"
        ((ERRORS++))
    fi
done

#################################################################
# 7. Script Dependencies Check
#################################################################
echo ""
echo "=== 7. Required Script Files Check ==="
echo ""

REQUIRED_SCRIPTS=(
    "scripts/setup/config_wizard.sh"
    "scripts/setup/generate_configs.sh"
    "scripts/monitoring/check_voip_master.sh"
    "scripts/failover/keepalived_notify.sh"
    "scripts/failover/safe_rebuild_standby.sh"
)

for script in "${REQUIRED_SCRIPTS[@]}"; do
    if [[ -f "$script" ]] && [[ -x "$script" ]]; then
        echo "✓ $script exists and is executable"
    elif [[ -f "$script" ]]; then
        echo "⚠ WARNING: $script exists but not executable"
        ((WARNINGS++))
    else
        echo "❌ ERROR: Missing required script: $script"
        ((ERRORS++))
    fi
done

#################################################################
# 8. Hardcoded Values Check
#################################################################
echo ""
echo "=== 8. Hardcoded Values Check ==="
echo ""

echo "Checking for hardcoded passwords in non-template files..."
HARDCODED_PASS=$(grep -r "PASSWORD\|password.*=" configs/ scripts/ \
    --include="*.sh" --include="*.conf" \
    2>/dev/null | \
    grep -v "PASSWORD_HERE\|YOUR_PASSWORD\|CHANGE_ME\|\$PASSWORD\|password=\"\"\|password=''" | \
    grep -v "pg_hba.conf" || true)

if [[ -n "$HARDCODED_PASS" ]]; then
    echo "⚠ WARNING: Possible hardcoded passwords found:"
    echo "$HARDCODED_PASS" | sed 's/^/    /' | head -5
    ((WARNINGS++))
else
    echo "✓ No hardcoded passwords detected"
fi

#################################################################
# 9. Database Schema Files Check
#################################################################
echo ""
echo "=== 9. Database Schema Files Check ==="
echo ""

SCHEMA_FILES=(
    "database/schemas/01-voip-schema.sql"
    "database/schemas/02-kamailio-schema.sql"
)

for schema in "${SCHEMA_FILES[@]}"; do
    if [[ -f "$schema" ]]; then
        line_count=$(wc -l < "$schema")
        echo "✓ $schema exists ($line_count lines)"
    else
        echo "❌ ERROR: Missing schema: $schema"
        ((ERRORS++))
    fi
done

#################################################################
# 10. Application Version References
#################################################################
echo ""
echo "=== 10. Application Version References ==="
echo ""

echo "Checking application versions mentioned in docs..."

# PostgreSQL
PG_VER=$(grep -r "PostgreSQL [0-9]" . --include="*.md" | grep -o "PostgreSQL [0-9]*" | sort -u)
echo "PostgreSQL versions: $PG_VER"

# Kamailio
KAM_VER=$(grep -r "Kamailio [0-9]" . --include="*.md" | grep -o "Kamailio [0-9]*\.[0-9]*" | sort -u || echo "Not found")
echo "Kamailio versions: $KAM_VER"

# FreeSWITCH
FS_VER=$(grep -r "FreeSWITCH [0-9]" . --include="*.md" | grep -o "FreeSWITCH [0-9]*\.[0-9]*" | sort -u || echo "Not found")
echo "FreeSWITCH versions: $FS_VER"

#################################################################
# Summary
#################################################################
echo ""
echo "============================================================"
echo "  Audit Summary"
echo "============================================================"
echo ""
echo "Errors:   $ERRORS"
echo "Warnings: $WARNINGS"
echo ""

if [[ $ERRORS -eq 0 ]] && [[ $WARNINGS -eq 0 ]]; then
    echo "✓✓✓ Project is clean! ✓✓✓"
    exit 0
elif [[ $ERRORS -eq 0 ]]; then
    echo "✓ No critical errors, but $WARNINGS warning(s) found"
    echo "Review warnings above for potential improvements"
    exit 0
else
    echo "❌ Found $ERRORS error(s) that need attention"
    echo "Please fix errors and run audit again"
    exit 1
fi
