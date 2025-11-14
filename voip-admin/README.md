# VoIP Admin Service

## Overview

The VoIP Admin Service is a unified Go application that serves multiple roles:

1. **API Gateway**: Handles CDR ingestion from FreeSWITCH
2. **XML_CURL Provider**: Serves directory, dialplan, and configuration to FreeSWITCH via mod_xml_curl
3. **Admin API**: Provides REST API for managing extensions, queues, IVRs, trunks
4. **CDR Processor**: Background workers process CDR queue asynchronously
5. **Extension Cache**: In-memory cache for fast directory lookups

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│              VoIP Admin Service (Port 8080)             │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌───────────────┐  ┌──────────────┐  ┌─────────────┐ │
│  │  HTTP Server  │  │ In-Memory    │  │ Background  │ │
│  │  (Gorilla)    │  │ Cache        │  │ Workers     │ │
│  └───────┬───────┘  └──────┬───────┘  └──────┬──────┘ │
│          │                  │                  │        │
│  ┌───────▼──────────────────▼──────────────────▼─────┐ │
│  │         PostgreSQL Connection Pool (50 conns)     │ │
│  └──────────────────────────┬────────────────────────┘ │
│                             │                           │
└─────────────────────────────┼───────────────────────────┘
                              │
                    ┌─────────▼─────────┐
                    │   PostgreSQL      │
                    │   (voip schema)   │
                    └───────────────────┘
```

## Project Structure

```
voip-admin/
├── cmd/
│   └── voipadmind/
│       └── main.go          # Application entry point
├── internal/
│   ├── api/                 # HTTP handlers
│   ├── cache/               # In-memory cache
│   ├── db/                  # Database layer
│   ├── models/              # Data models
│   ├── workers/             # Background workers (CDR processing)
│   └── xmlcurl/             # FreeSWITCH XML_CURL logic
├── go.mod
├── go.sum
└── README.md
```

## Current Status

**Phase**: Skeleton implementation

**Completed**:
- Basic HTTP server with Gorilla mux
- Configuration loading from YAML
- Health check endpoint
- Placeholder routes for XML_CURL, CDR, Extensions

**TODO** (See IMPLEMENTATION-PLAN.md Phase 5):
- [ ] Database connection pool
- [ ] In-memory cache implementation
- [ ] XML_CURL directory handler (voip.extensions lookup)
- [ ] XML_CURL dialplan handler (call routing logic)
- [ ] CDR queue processor (background workers)
- [ ] Extension CRUD handlers
- [ ] Queue management handlers
- [ ] IVR management handlers
- [ ] Trunk management handlers
- [ ] Authentication middleware
- [ ] Logging middleware
- [ ] Metrics (Prometheus)

## Building

```bash
cd voip-admin
go mod download
go build -o bin/voipadmind ./cmd/voipadmind
```

## Running

```bash
./bin/voipadmind -config /etc/voip-admin/config.yaml
```

Or with systemd:

```bash
systemctl start voip-admin
```

## Testing

```bash
# Health check
curl http://localhost:8080/health

# XML_CURL test (from FreeSWITCH)
curl -X POST http://localhost:8080/freeswitch/directory \
  -u freeswitch:API_KEY_HERE \
  -d 'user=1001&domain=example.com'
```

## Dependencies

- `github.com/gorilla/mux`: HTTP router
- `github.com/lib/pq`: PostgreSQL driver
- `gopkg.in/yaml.v3`: YAML config parsing

## Configuration

See [configs/voip-admin/config.yaml](../../configs/voip-admin/config.yaml)

## API Documentation

(To be added with Swagger/OpenAPI)
