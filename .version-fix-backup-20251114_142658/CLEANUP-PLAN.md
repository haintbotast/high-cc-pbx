# PROJECT CLEANUP & RESTRUCTURING PLAN

**Current Problem**: 11 markdown files (250 KB) with massive duplication, no actual code extracted
**Solution**: Clean structure with code in proper files, ONE README, clear action plan

---

## CLEANUP ACTIONS (Execute in order)

### Step 1: Archive Old Documents
```bash
mkdir -p archive/analysis
mv "Analysis architecture changes.md" archive/analysis/
mv "Voip production deployment optimized.md" archive/analysis/
mv "Architecture Comparison Analysis.md" archive/analysis/
mv "2-Node Architecture Design.md" archive/analysis/
mv "OVERALL PROJECT REVIEW.md" archive/analysis/
mv "ANALYSIS - Redis Removal & Optimization.md" archive/analysis/
mv "RESTRUCTURING-SUMMARY.md" archive/analysis/
mv "PROJECT-STRUCTURE.md" archive/analysis/
mv "README-NEW.md" archive/analysis/
```

### Step 2: Keep ONLY These Files
```
high-cc-pbx/
├── README.md              (ONE master README - rewrite)
├── claude.md              (Keep as-is - AI assistant guide)
└── IMPLEMENTATION-PLAN.md (New - actionable checklist)
```

### Step 3: Extract Code to Proper Files

**Database schemas**:
```bash
# Extract from old docs → database/schemas/01-voip-schema.sql
# Contains: all voip.* tables with proper SQL
```

**Configurations**:
```bash
# Extract from old docs → configs/postgresql/postgresql.conf
# Extract from old docs → configs/kamailio/kamailio.cfg
# Extract from old docs → configs/freeswitch/autoload_configs/*.xml
# Extract from old docs → configs/keepalived/keepalived.conf
# Extract from old docs → configs/lsyncd/lsyncd.conf.lua
```

**Scripts**:
```bash
# Extract from old docs → scripts/failover/postgres_failover.sh
# Extract from old docs → scripts/failover/failover_master.sh
# Extract from old docs → scripts/monitoring/system_health.sh
```

### Step 4: Create voip-admin Skeleton
```bash
# Create basic Go project structure
# NOT just documentation - actual go.mod, main.go
```

---

## FINAL PROJECT STRUCTURE (Clean)

```
high-cc-pbx/
├── README.md                    ⭐ ONE master document
├── claude.md                    🤖 AI assistant guide (existing)
├── IMPLEMENTATION-PLAN.md       📋 Phase-by-phase checklist
│
├── archive/                     📦 Old analysis docs (reference only)
│   └── analysis/
│       ├── Analysis architecture changes.md
│       ├── Voip production deployment optimized.md
│       ├── Architecture Comparison Analysis.md
│       ├── 2-Node Architecture Design.md
│       ├── OVERALL PROJECT REVIEW.md
│       ├── ANALYSIS - Redis Removal.md
│       ├── RESTRUCTURING-SUMMARY.md
│       └── PROJECT-STRUCTURE.md
│
├── database/                    💾 SQL files (ACTUAL code)
│   └── schemas/
│       ├── 01-voip-schema.sql
│       ├── 02-kamailio-schema.sql
│       └── 03-views.sql
│
├── configs/                     ⚙️ Config files (ACTUAL configs)
│   ├── postgresql/
│   │   ├── postgresql.conf
│   │   └── pg_hba.conf
│   ├── kamailio/
│   │   └── kamailio.cfg
│   ├── freeswitch/
│   │   └── autoload_configs/
│   │       ├── switch.conf.xml
│   │       ├── sofia.conf.xml
│   │       └── json_cdr.conf.xml
│   ├── keepalived/
│   │   ├── keepalived-node1.conf
│   │   └── keepalived-node2.conf
│   ├── lsyncd/
│   │   ├── lsyncd-node1.conf.lua
│   │   └── lsyncd-node2.conf.lua
│   └── voip-admin/
│       └── config.yaml
│
├── scripts/                     🔧 Bash scripts (ACTUAL scripts)
│   ├── failover/
│   │   ├── postgres_failover.sh
│   │   ├── failover_master.sh
│   │   ├── failover_backup.sh
│   │   └── failover_fault.sh
│   └── monitoring/
│       ├── system_health.sh
│       └── check_postgres.sh
│
└── voip-admin/                  💻 Go code (ACTUAL code)
    ├── go.mod
    ├── go.sum
    ├── Makefile
    ├── cmd/
    │   └── voipadmind/
    │       └── main.go
    └── internal/
        ├── config/
        │   └── config.go
        ├── database/
        │   └── postgres.go
        └── api/
            └── router.go
```

**Total**: 3 markdown docs + actual code in proper places

---

## NEXT ACTIONS (Do NOW)

1. ✅ **Execute cleanup** - Run Step 1-2 bash commands
2. ✅ **Extract database schema** - Create `database/schemas/01-voip-schema.sql` with REAL SQL
3. ✅ **Extract configs** - Create actual config files in `configs/`
4. ✅ **Extract scripts** - Create actual bash scripts in `scripts/`
5. ✅ **Create Go skeleton** - Basic `voip-admin/` with `go.mod` and `main.go`
6. ✅ **Write ONE README.md** - Clear, concise, actionable
7. ✅ **Write IMPLEMENTATION-PLAN.md** - Phase-by-phase with checkboxes

---

## WHAT EACH FILE SHOULD CONTAIN

### README.md (ONE file, ~5 KB max)
```markdown
# High-Availability VoIP System

Quick facts, architecture diagram, hardware requirements,
getting started (3 steps), and links to other files.

NO analysis, NO comparisons, NO lengthy explanations.
```

### IMPLEMENTATION-PLAN.md
```markdown
# Implementation Checklist

## Phase 1: Infrastructure (Week 1-2)
- [ ] Order hardware
- [ ] Install Debian 12
- [ ] Configure network
...

## Phase 2: Database (Week 3-4)
- [ ] Install PostgreSQL
- [ ] Apply schemas: psql -f database/schemas/01-voip-schema.sql
...
```

### database/schemas/01-voip-schema.sql
```sql
-- ACTUAL SQL CODE, NOT MARKDOWN
CREATE SCHEMA voip;

CREATE TABLE voip.domains (
    id SERIAL PRIMARY KEY,
    domain VARCHAR(255) UNIQUE NOT NULL,
    ...
);

-- All tables with proper syntax
```

### configs/postgresql/postgresql.conf
```ini
# ACTUAL CONFIG, NOT MARKDOWN
listen_addresses = '*'
max_connections = 300
shared_buffers = 12GB
...
```

### scripts/failover/postgres_failover.sh
```bash
#!/bin/bash
# ACTUAL BASH SCRIPT, NOT MARKDOWN
set -euo pipefail

promote_to_primary() {
    sudo -u postgres /usr/lib/postgresql/16/bin/pg_ctl promote -D /var/lib/postgresql/16/main
}
...
```

### voip-admin/cmd/voipadmind/main.go
```go
// ACTUAL GO CODE, NOT MARKDOWN
package main

import (
    "fmt"
    "net/http"
)

func main() {
    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "OK")
    })
    http.ListenAndServe(":8080", nil)
}
```

---

## EXECUTE THIS PLAN

Run this document as a script, then create the REAL files.

**Problem**: Too much analysis, not enough action.
**Solution**: Clean structure, actual code, clear checklist.

Let's DO IT.
