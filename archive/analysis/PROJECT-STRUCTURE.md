# PROJECT DIRECTORY STRUCTURE

**Last Updated**: 2025-11-14
**Architecture**: 2-Node VoIP System (600-800 CC)
**Services**: Kamailio + FreeSWITCH + PostgreSQL + voip-admin

---

## Directory Layout

```
high-cc-pbx/
├── README.md                          # Project overview & quick start
├── PROJECT-STRUCTURE.md              # This file - directory guide
│
├── docs/                             # All documentation
│   ├── 00-GETTING-STARTED.md         # Start here
│   ├── 01-Architecture-Overview.md   # System architecture
│   ├── 02-Database-Design.md         # Schema design
│   ├── 03-Deployment-Guide.md        # Step-by-step deployment
│   ├── 04-Failover-Procedures.md     # HA and failover
│   ├── 05-API-Reference.md           # voip-admin API docs
│   └── 06-Troubleshooting.md         # Common issues
│
├── database/                         # Database schemas & migrations
│   ├── schemas/
│   │   ├── 01-voip-schema.sql        # voip schema (business logic)
│   │   ├── 02-kamailio-schema.sql    # Kamailio tables
│   │   └── 03-views.sql              # Views (vw_extensions, etc.)
│   ├── migrations/
│   │   └── 001-initial-schema.sql
│   └── seeds/
│       └── dev-data.sql              # Development test data
│
├── configs/                          # All configuration files
│   ├── postgresql/
│   │   ├── postgresql.conf           # PostgreSQL config
│   │   ├── pg_hba.conf               # Auth config
│   │   └── recovery.conf             # Standby config
│   │
│   ├── kamailio/
│   │   ├── kamailio.cfg              # Main config
│   │   ├── kamailio-local.cfg        # Node-specific settings
│   │   └── dispatcher.list           # FreeSWITCH destinations
│   │
│   ├── freeswitch/
│   │   ├── autoload_configs/
│   │   │   ├── switch.conf.xml       # Core config
│   │   │   ├── sofia.conf.xml        # SIP profiles
│   │   │   └── json_cdr.conf.xml     # CDR posting
│   │   └── dialplan/
│   │       └── default.xml           # Dialplan
│   │
│   ├── keepalived/
│   │   ├── keepalived-node1.conf     # Node 1 config
│   │   └── keepalived-node2.conf     # Node 2 config
│   │
│   ├── lsyncd/
│   │   ├── lsyncd-node1.conf.lua     # Node 1 rsync config
│   │   ├── lsyncd-node2.conf.lua     # Node 2 rsync config
│   │   └── rsyncd.conf               # rsync daemon
│   │
│   └── voip-admin/
│       ├── config.yaml               # Main service config
│       └── config.example.yaml       # Example with comments
│
├── scripts/                          # Bash scripts
│   ├── failover/
│   │   ├── postgres_failover.sh      # PostgreSQL promotion
│   │   ├── failover_master.sh        # Keepalived MASTER handler
│   │   ├── failover_backup.sh        # Keepalived BACKUP handler
│   │   └── failover_fault.sh         # Keepalived FAULT handler
│   │
│   ├── monitoring/
│   │   ├── system_health.sh          # Overall health check
│   │   ├── check_postgres.sh         # PostgreSQL status
│   │   ├── check_kamailio.sh         # Kamailio status
│   │   └── check_freeswitch.sh       # FreeSWITCH status
│   │
│   ├── maintenance/
│   │   ├── backup_postgres.sh        # Database backup
│   │   ├── cleanup_cdr.sh            # CDR retention
│   │   └── cleanup_recordings.sh     # Recording cleanup
│   │
│   └── deployment/
│       ├── install_node.sh           # Full node installation
│       ├── setup_postgres.sh         # PostgreSQL setup
│       ├── setup_kamailio.sh         # Kamailio setup
│       └── setup_freeswitch.sh       # FreeSWITCH setup
│
├── voip-admin/                       # VoIP Admin Service (Go)
│   ├── cmd/
│   │   └── voipadmind/
│   │       └── main.go               # Service entry point
│   │
│   ├── internal/
│   │   ├── config/
│   │   │   ├── config.go             # Config structures
│   │   │   └── loader.go             # Config file loader
│   │   │
│   │   ├── database/
│   │   │   ├── postgres.go           # PostgreSQL connection
│   │   │   ├── queries.go            # SQL queries
│   │   │   └── models.go             # Database models
│   │   │
│   │   ├── cache/
│   │   │   ├── memory.go             # In-memory cache
│   │   │   └── cache.go              # Cache interface
│   │   │
│   │   ├── freeswitch/
│   │   │   ├── xml/
│   │   │   │   ├── directory.go      # Directory XML generator
│   │   │   │   ├── dialplan.go       # Dialplan XML generator
│   │   │   │   └── templates.go      # XML templates
│   │   │   └── cdr/
│   │   │       ├── handler.go        # HTTP handler
│   │   │       ├── processor.go      # Background worker
│   │   │       └── parser.go         # JSON parser
│   │   │
│   │   ├── api/
│   │   │   ├── handlers/
│   │   │   │   ├── cdr.go            # CDR API
│   │   │   │   ├── recordings.go     # Recording API
│   │   │   │   ├── extensions.go     # Extension management
│   │   │   │   ├── queues.go         # Queue management
│   │   │   │   ├── users.go          # User management
│   │   │   │   ├── kamailio.go       # Kamailio control
│   │   │   │   └── health.go         # Health check
│   │   │   │
│   │   │   ├── middleware/
│   │   │   │   ├── auth.go           # API key auth
│   │   │   │   ├── logging.go        # Request logging
│   │   │   │   ├── cors.go           # CORS handling
│   │   │   │   └── metrics.go        # Prometheus metrics
│   │   │   │
│   │   │   └── router.go             # HTTP router setup
│   │   │
│   │   └── domain/
│   │       ├── models/
│   │       │   ├── extension.go      # Extension model
│   │       │   ├── cdr.go            # CDR model
│   │       │   ├── queue.go          # Queue model
│   │       │   ├── user.go           # User model
│   │       │   └── recording.go      # Recording model
│   │       │
│   │       └── services/
│   │           ├── extension_service.go
│   │           ├── cdr_service.go
│   │           ├── queue_service.go
│   │           └── routing_service.go
│   │
│   ├── pkg/
│   │   └── utils/
│   │       ├── crypto.go             # Password hashing
│   │       ├── validator.go          # Input validation
│   │       └── logger.go             # Logging utilities
│   │
│   ├── go.mod                        # Go module definition
│   ├── go.sum                        # Go dependencies
│   ├── Makefile                      # Build automation
│   ├── Dockerfile                    # Container image
│   └── README.md                     # Service documentation
│
├── ansible/                          # Deployment automation (future)
│   ├── inventory/
│   │   ├── hosts.ini                 # Server inventory
│   │   └── group_vars/
│   │       └── all.yml               # Variables
│   │
│   ├── playbooks/
│   │   ├── deploy_all.yml            # Full deployment
│   │   ├── deploy_postgres.yml       # PostgreSQL only
│   │   ├── deploy_kamailio.yml       # Kamailio only
│   │   └── deploy_freeswitch.yml     # FreeSWITCH only
│   │
│   └── roles/
│       ├── common/                   # Common setup
│       ├── postgresql/               # PostgreSQL role
│       ├── kamailio/                 # Kamailio role
│       └── freeswitch/               # FreeSWITCH role
│
├── monitoring/                       # Monitoring configs (future)
│   ├── prometheus/
│   │   ├── prometheus.yml            # Prometheus config
│   │   └── alerts.yml                # Alert rules
│   │
│   └── grafana/
│       └── dashboards/
│           ├── system-overview.json
│           ├── call-metrics.json
│           └── database-metrics.json
│
├── tests/                            # Testing (future)
│   ├── sipp/                         # SIPp scenarios
│   │   ├── register.xml              # Registration test
│   │   ├── call.xml                  # Call test
│   │   └── load_test.xml             # Load test
│   │
│   └── integration/                  # Integration tests
│       └── api_test.go               # API tests
│
└── .gitignore                        # Git ignore rules
```

---

## Directory Purpose

### `/docs/`
**Purpose**: Human-readable documentation
**Contents**: Architecture, deployment guides, API docs, troubleshooting
**Target**: System administrators, developers, operators

### `/database/`
**Purpose**: Database schemas and migrations
**Contents**: SQL files for creating voip schema, Kamailio schema, views
**Usage**: Apply schemas with `psql -f database/schemas/01-voip-schema.sql`

### `/configs/`
**Purpose**: Configuration files for all services
**Contents**: Kamailio, FreeSWITCH, PostgreSQL, Keepalived, lsyncd configs
**Deployment**: Copy to appropriate locations on nodes (`/etc/...`)

### `/scripts/`
**Purpose**: Operational bash scripts
**Contents**: Failover, monitoring, maintenance, deployment automation
**Usage**: Executed manually or by Keepalived/cron

### `/voip-admin/`
**Purpose**: VoIP Admin Service source code (Go)
**Contents**: Complete Go application (HTTP API + background workers)
**Build**: `cd voip-admin && make build`
**Deploy**: Binary to `/opt/voip-admin/bin/voipadmind`

### `/ansible/` (future)
**Purpose**: Infrastructure as Code for deployment automation
**Contents**: Ansible playbooks and roles
**Usage**: `ansible-playbook -i inventory/hosts.ini playbooks/deploy_all.yml`

### `/monitoring/` (future)
**Purpose**: Monitoring and alerting configurations
**Contents**: Prometheus rules, Grafana dashboards
**Usage**: Deployed to monitoring infrastructure

### `/tests/` (future)
**Purpose**: Testing scenarios and test code
**Contents**: SIPp XML scenarios, integration tests
**Usage**: Load testing, regression testing

---

## File Naming Conventions

### Configuration Files
- **Service configs**: `{service}.conf` or `{service}.cfg`
- **Node-specific**: `{service}-node1.conf`, `{service}-node2.conf`
- **Example configs**: `{file}.example.{ext}`

### Scripts
- **Bash scripts**: `{purpose}_{action}.sh` (e.g., `check_postgres.sh`)
- **Executable**: All scripts chmod +x
- **Location**: Scripts deployed to `/usr/local/bin/` on nodes

### SQL Files
- **Numbered**: `01-schema-name.sql`, `02-next-schema.sql`
- **Migrations**: `{timestamp}-description.sql`

### Go Code
- **Packages**: lowercase, no underscores (e.g., `database`, `freeswitch`)
- **Files**: lowercase with underscores (e.g., `cdr_service.go`)
- **Tests**: `{file}_test.go`

---

## Deployment Mapping

| Source File | Destination (on nodes) | Service |
|-------------|----------------------|---------|
| `configs/postgresql/postgresql.conf` | `/etc/postgresql/16/main/postgresql.conf` | PostgreSQL |
| `configs/kamailio/kamailio.cfg` | `/etc/kamailio/kamailio.cfg` | Kamailio |
| `configs/freeswitch/autoload_configs/*.xml` | `/etc/freeswitch/autoload_configs/` | FreeSWITCH |
| `configs/keepalived/keepalived-node1.conf` | `/etc/keepalived/keepalived.conf` (Node 1) | Keepalived |
| `configs/lsyncd/lsyncd-node1.conf.lua` | `/etc/lsyncd/lsyncd.conf.lua` (Node 1) | lsyncd |
| `configs/voip-admin/config.yaml` | `/etc/voip-admin/config.yaml` | voip-admin |
| `scripts/failover/*.sh` | `/usr/local/bin/` | Keepalived |
| `scripts/monitoring/*.sh` | `/usr/local/bin/` | Cron/manual |
| `voip-admin/build/voipadmind` | `/opt/voip-admin/bin/voipadmind` | voip-admin |
| `database/schemas/*.sql` | Applied to PostgreSQL | PostgreSQL |

---

## Version Control

### What to Commit (.git)
✅ All configuration files (configs/)
✅ All scripts (scripts/)
✅ All documentation (docs/)
✅ All SQL schemas (database/)
✅ All Go source code (voip-admin/)
✅ Ansible playbooks (ansible/)
✅ README.md, PROJECT-STRUCTURE.md

### What to Ignore (.gitignore)
❌ Binaries (voip-admin/build/*)
❌ Secrets (*.key, *.pem, passwords)
❌ Node-specific data (/var/lib/*, /var/log/*)
❌ Go build artifacts (vendor/, *.test)
❌ IDE files (.vscode/, .idea/)

---

## Build & Deployment Workflow

### Development
```bash
# 1. Clone repository
git clone <repo-url> high-cc-pbx
cd high-cc-pbx

# 2. Build voip-admin service
cd voip-admin
go mod download
make build
# Binary: voip-admin/build/voipadmind

# 3. Run tests
make test

# 4. Create configuration from examples
cp configs/voip-admin/config.example.yaml configs/voip-admin/config.yaml
# Edit config.yaml with your settings
```

### Deployment (Manual)
```bash
# Node 1
# 1. Copy configs
scp -r configs/postgresql/* node1:/etc/postgresql/16/main/
scp -r configs/kamailio/* node1:/etc/kamailio/
scp -r configs/freeswitch/* node1:/etc/freeswitch/
scp configs/keepalived/keepalived-node1.conf node1:/etc/keepalived/keepalived.conf
scp configs/voip-admin/config.yaml node1:/etc/voip-admin/

# 2. Copy scripts
scp scripts/failover/*.sh node1:/usr/local/bin/
scp scripts/monitoring/*.sh node1:/usr/local/bin/
chmod +x node1:/usr/local/bin/*.sh

# 3. Deploy voip-admin
scp voip-admin/build/voipadmind node1:/opt/voip-admin/bin/

# 4. Apply database schemas (once, on primary)
psql -h node1 -U postgres -f database/schemas/01-voip-schema.sql
psql -h node1 -U postgres -f database/schemas/02-kamailio-schema.sql
psql -h node1 -U postgres -f database/schemas/03-views.sql

# Repeat for Node 2 (with node2 configs)
```

### Deployment (Ansible - future)
```bash
# Deploy everything
ansible-playbook -i ansible/inventory/hosts.ini ansible/playbooks/deploy_all.yml

# Deploy specific service
ansible-playbook -i ansible/inventory/hosts.ini ansible/playbooks/deploy_kamailio.yml
```

---

## Configuration Management

### Node-Specific vs Shared

**Shared (same on both nodes)**:
- `configs/postgresql/postgresql.conf` (base config)
- `configs/kamailio/kamailio.cfg` (main routing logic)
- `configs/freeswitch/dialplan/*.xml`
- `configs/voip-admin/config.yaml` (with VIP references)

**Node-Specific (different per node)**:
- `configs/keepalived/keepalived-node1.conf` vs `node2.conf` (priority, state)
- `configs/lsyncd/lsyncd-node1.conf.lua` vs `node2.conf.lua` (target IPs)
- IP addresses in configs (192.168.1.101 vs .102)

**How to Handle**:
1. Use templates with variables (e.g., `{{ NODE_IP }}`)
2. Ansible will substitute variables during deployment
3. Manual deployment: Edit configs before copying

---

## Service Dependencies

```
Startup Order:
1. PostgreSQL       (must be first)
2. voip-admin       (depends on PostgreSQL)
3. Kamailio         (depends on PostgreSQL, voip-admin optional)
4. FreeSWITCH       (depends on PostgreSQL, voip-admin for XML_CURL)
5. Keepalived       (depends on all above for health checks)
6. lsyncd           (depends on FreeSWITCH for recordings)
```

**Systemd Dependencies**: Configured in service files
```ini
[Unit]
After=postgresql.service
Wants=postgresql.service
```

---

## Development Guidelines

### Adding New Features

**1. Database changes**:
- Create migration in `database/migrations/{timestamp}-{description}.sql`
- Update schema in `database/schemas/`
- Update models in `voip-admin/internal/database/models.go`

**2. API endpoints**:
- Add handler in `voip-admin/internal/api/handlers/`
- Add route in `voip-admin/internal/api/router.go`
- Update `docs/05-API-Reference.md`

**3. Configuration changes**:
- Update example config `configs/voip-admin/config.example.yaml`
- Update loader in `voip-admin/internal/config/loader.go`
- Document in README

### Code Style

**Go**:
- Follow standard Go conventions
- Use `gofmt` for formatting
- Run `go vet` before commit
- Keep functions small (<50 lines)
- Comment exported functions

**SQL**:
- Uppercase keywords (SELECT, FROM, WHERE)
- Indent for readability
- Add indexes for all foreign keys
- Comment complex queries

**Bash**:
- Use `#!/bin/bash` shebang
- Add `set -euo pipefail`
- Quote variables: `"$VAR"`
- Add description comments

---

## Quick Reference

### Common Commands

```bash
# Build voip-admin
cd voip-admin && make build

# Apply database schema
psql -h 192.168.1.100 -U postgres -f database/schemas/01-voip-schema.sql

# Check system health
/usr/local/bin/system_health.sh

# Manual failover test
systemctl stop keepalived  # on current master

# View logs
journalctl -u voip-admin -f
tail -f /var/log/voip-failover.log

# Restart services
systemctl restart kamailio freeswitch voip-admin
```

### Important Paths (on nodes)

```
Configs:
- PostgreSQL: /etc/postgresql/16/main/
- Kamailio:   /etc/kamailio/
- FreeSWITCH: /etc/freeswitch/
- voip-admin: /etc/voip-admin/

Logs:
- PostgreSQL: /var/log/postgresql/
- Kamailio:   /var/log/kamailio.log
- FreeSWITCH: /var/log/freeswitch/
- voip-admin: /var/log/voip-admin/ (or journalctl)
- Failover:   /var/log/voip-failover.log

Data:
- PostgreSQL:  /var/lib/postgresql/16/main/
- Recordings:  /storage/recordings/
- CDR queue:   voip.cdr_queue table
```

---

## Next Steps

1. **Extract code from documentation** - Move configs, scripts to proper locations
2. **Create voip-admin service** - Implement Go application
3. **Write deployment scripts** - Automate installation
4. **Create Ansible playbooks** - Infrastructure as Code
5. **Setup CI/CD** - Automated testing and deployment

See [docs/00-GETTING-STARTED.md](docs/00-GETTING-STARTED.md) for detailed steps.
