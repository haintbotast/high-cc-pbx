#!/bin/bash
#################################################################
# Fix PostgreSQL Version References (16 → 18)
# Systematically updates all files in the project
#################################################################

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$PROJECT_ROOT"

echo "============================================================"
echo "  PostgreSQL Version Fix: 16 → 18"
echo "============================================================"
echo "Project root: $PROJECT_ROOT"
echo ""

# Create backup directory
BACKUP_DIR="$PROJECT_ROOT/.version-fix-backup-$(date +%Y%m%d_%H%M%S)"
mkdir -p "$BACKUP_DIR"

# Files to fix (excluding archives)
FILES=(
    "claude.md"
    "CLEANUP-PLAN.md"
    "IMPLEMENTATION-PLAN.md"
    "PRODUCTION-READY-SUMMARY.md"
    "README.md"
    "configs/freeswitch/autoload_configs/cdr_pg_csv.conf.xml"
    "configs/freeswitch/autoload_configs/switch.conf.xml"
    "configs/postgresql/postgresql.conf"
    "scripts/failover/failover_backup.sh"
    "scripts/failover/failover_master.sh"
    "scripts/failover/keepalived_notify.sh"
    "scripts/failover/postgres_failover.sh"
)

echo "Backing up files to: $BACKUP_DIR"
for file in "${FILES[@]}"; do
    if [[ -f "$file" ]]; then
        mkdir -p "$BACKUP_DIR/$(dirname "$file")"
        cp "$file" "$BACKUP_DIR/$file"
    fi
done

echo ""
echo "Fixing PostgreSQL version references..."
echo ""

# Fix each file
for file in "${FILES[@]}"; do
    if [[ ! -f "$file" ]]; then
        echo "  ⚠ Skip: $file (not found)"
        continue
    fi

    echo "  Processing: $file"

    # Create temporary file
    temp_file=$(mktemp)

    # Apply replacements
    sed -e 's|postgresql-16|postgresql-18|g' \
        -e 's|postgresql/16|postgresql/18|g' \
        -e 's|pgsql-16|pgsql-18|g' \
        -e 's|pgsql/16|pgsql/18|g' \
        -e 's|/var/lib/postgresql/16/|/var/lib/postgresql/18/|g' \
        -e 's|/etc/postgresql/16/|/etc/postgresql/18/|g' \
        -e 's|/usr/lib/postgresql/16/|/usr/lib/postgresql/18/|g' \
        -e 's|PostgreSQL 16|PostgreSQL 18|g' \
        "$file" > "$temp_file"

    # Check if file changed
    if ! diff -q "$file" "$temp_file" > /dev/null 2>&1; then
        mv "$temp_file" "$file"
        echo "    ✓ Updated"
    else
        rm "$temp_file"
        echo "    - No changes needed"
    fi
done

echo ""
echo "============================================================"
echo "✓ PostgreSQL version fix complete!"
echo "============================================================"
echo ""
echo "Backup created at: $BACKUP_DIR"
echo ""
echo "Summary of changes:"
echo "  postgresql-16 → postgresql-18"
echo "  /var/lib/postgresql/16/ → /var/lib/postgresql/18/"
echo "  /usr/lib/postgresql/16/ → /usr/lib/postgresql/18/"
echo ""
echo "To verify: grep -r 'postgresql.*16' . --exclude-dir=archive --exclude-dir=.git"
echo ""
