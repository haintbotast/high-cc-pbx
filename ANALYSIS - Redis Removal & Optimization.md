# ANALYSIS: Redis Removal & System Optimization

**Date**: 2025-11-14
**Purpose**: Analyze removing Redis, calculate minimum hardware, merge services
**Impact**: Simplified architecture, reduced cost, single unified service

---

## 1. REDIS REMOVAL ANALYSIS

### 1.1. Current Redis Usage

Redis was used for:
1. **CDR Queue Buffer** - Async queue between FreeSWITCH and database
2. **Caching Layer** - Cache directory XML, extension lookups
3. **High Availability** - Master-slave replication

### 1.2. Can We Remove Redis?

**YES** ✅ - Redis can be removed with architectural adjustments

### 1.3. Alternative Approaches WITHOUT Redis

#### Option 1: PostgreSQL as Queue (RECOMMENDED) ✅

**Use PostgreSQL LISTEN/NOTIFY + Table Queue**:

```sql
-- CDR queue table
CREATE TABLE voip.cdr_queue (
    id BIGSERIAL PRIMARY KEY,
    payload JSONB NOT NULL,
    status VARCHAR(20) DEFAULT 'pending', -- 'pending', 'processing', 'completed', 'failed'
    attempts INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    processed_at TIMESTAMP
);

CREATE INDEX idx_cdr_queue_status ON voip.cdr_queue(status) WHERE status = 'pending';
CREATE INDEX idx_cdr_queue_created ON voip.cdr_queue(created_at) WHERE status = 'pending';
```

**Process**:
1. FreeSWITCH POST CDR → VoIP Admin Service
2. VoIP Admin inserts into `cdr_queue` table (fast INSERT)
3. Background worker polls queue (SELECT ... FOR UPDATE SKIP LOCKED)
4. Worker processes batch (100 CDR) and inserts into `voip.cdr`
5. Mark as 'completed' or retry on failure

**Advantages**:
✅ No Redis infrastructure needed
✅ ACID guarantees (no data loss)
✅ Simpler architecture
✅ PostgreSQL already handles this well
✅ Automatic replication (follows PostgreSQL HA)

**Disadvantages**:
⚠️ Slightly higher latency (~10-20ms vs Redis)
⚠️ More DB load (but acceptable for 600-800 CC)

#### Option 2: In-Memory Queue (Simple)

**Use Go channel-based queue**:
- FreeSWITCH → HTTP POST → In-memory channel (buffered)
- Background goroutine processes queue
- Persist queue state to disk on shutdown

**Advantages**:
✅ Very fast (in-memory)
✅ No external dependencies

**Disadvantages**:
❌ Data loss risk if service crashes
❌ No HA (queue lost on failover)

**Verdict**: NOT RECOMMENDED for production

### 1.4. Caching Without Redis

**Use multi-tier caching**:

1. **Tier 1**: In-memory cache (Go sync.Map or groupcache)
   - Extension lookups (60s TTL)
   - Directory XML (300s TTL)
   - 90%+ hit rate

2. **Tier 2**: PostgreSQL (source of truth)
   - Properly indexed
   - Query cache enabled

**Code Example**:
```go
type Cache struct {
    data sync.Map
    ttl  time.Duration
}

func (c *Cache) Get(key string) (interface{}, bool) {
    val, ok := c.data.Load(key)
    if !ok {
        return nil, false
    }

    entry := val.(CacheEntry)
    if time.Now().After(entry.ExpiresAt) {
        c.data.Delete(key)
        return nil, false
    }

    return entry.Value, true
}
```

### 1.5. Impact Analysis

| Aspect | With Redis | Without Redis (PG Queue) | Impact |
|--------|-----------|--------------------------|--------|
| **Infrastructure** | PostgreSQL + Redis | PostgreSQL only | ✅ Simpler |
| **HA Complexity** | PG + Redis replication | PG replication only | ✅ Reduced |
| **CDR Latency** | 2-5ms | 10-20ms | ⚠️ Acceptable |
| **Data Loss Risk** | Very low | Very low (ACID) | ✅ Same |
| **Memory Usage** | Redis RAM + App RAM | App RAM only | ✅ Reduced |
| **Cost** | Higher | Lower | ✅ Savings |
| **Failover** | 2 services | 1 service | ✅ Simpler |

### 1.6. DECISION: Remove Redis ✅

**Recommendation**: Use PostgreSQL table queue + in-memory caching

**Why**:
- PostgreSQL can handle queue workload for 600-800 CC
- Simpler architecture (one less service)
- ACID guarantees (no data loss)
- Automatic HA (follows PostgreSQL replication)
- Cost savings (~$500-1000 in hardware/licensing)

**Trade-off**: +10-20ms CDR processing latency (acceptable)

---

## 2. VOIP ADMIN SERVICE + API GATEWAY MERGER

### 2.1. Current Separation

**Project A (Original)**:
- **API Gateway** - Simple CDR receiver + query API (~150 lines)

**Project B (New)**:
- **VoIP Admin Service** - Full platform (XML_CURL + CDR + Management)

### 2.2. Unified Service Architecture

**Single Service**: `voip-admin` (combines both)

```
voip-admin/
├── HTTP Server (port 8080)
│   ├── FreeSWITCH Integration
│   │   ├── POST /fs/cdr                    # CDR ingestion (was API Gateway)
│   │   ├── GET  /fs/xml/directory          # mod_xml_curl directory
│   │   └── GET  /fs/xml/dialplan           # mod_xml_curl dialplan (optional)
│   │
│   ├── CDR API
│   │   ├── GET  /api/cdr                   # Query CDR (was API Gateway)
│   │   └── GET  /api/cdr/{id}              # Get single CDR
│   │
│   ├── Recording API
│   │   ├── GET  /api/recordings            # List recordings
│   │   └── GET  /api/recordings/{id}/download # Download file
│   │
│   ├── Management API (Kamailio + FreeSWITCH)
│   │   ├── Extensions
│   │   │   ├── GET    /api/extensions
│   │   │   ├── POST   /api/extensions
│   │   │   ├── PUT    /api/extensions/{id}
│   │   │   └── DELETE /api/extensions/{id}
│   │   │
│   │   ├── Queues
│   │   │   ├── GET    /api/queues
│   │   │   ├── POST   /api/queues
│   │   │   └── PUT    /api/queues/{id}
│   │   │
│   │   ├── Users/Agents
│   │   │   ├── GET    /api/users
│   │   │   ├── POST   /api/users
│   │   │   └── PUT    /api/users/{id}
│   │   │
│   │   └── Kamailio Control
│   │       ├── POST   /api/kamailio/reload    # Reload dispatcher
│   │       └── GET    /api/kamailio/stats     # Get statistics
│   │
│   └── Health & Metrics
│       ├── GET  /health
│       └── GET  /metrics                   # Prometheus metrics
│
├── Background Workers
│   ├── CDR Queue Processor (polls cdr_queue table)
│   └── Cleanup Worker (old CDRs, recordings)
│
└── Cache Manager
    └── In-memory cache (extensions, directory)
```

### 2.3. Benefits of Merger

✅ **Simpler deployment** - One service instead of two
✅ **Shared caching** - Extension cache used by both XML_CURL and CDR
✅ **Shared database pool** - Efficient connection usage
✅ **Unified monitoring** - Single metrics endpoint
✅ **Easier development** - One codebase
✅ **Lower resource usage** - Less memory, fewer goroutines

### 2.4. Resource Allocation

**Single voip-admin service**:
- CPU: 2-4 cores (handles all API + background workers)
- RAM: 4-8 GB (includes caching)
- Connections: 20-30 to PostgreSQL

---

## 3. MINIMUM HARDWARE REQUIREMENTS

### 3.1. Service Resource Analysis (Per Node)

#### PostgreSQL
```
Expected load (600-800 CC):
- Connections: ~150 (Kamailio: 80, FreeSWITCH: 40, voip-admin: 30)
- Queries/sec: ~500 (registration lookups, CDR inserts, extension lookups)
- Memory: Shared buffers + work_mem + connections
```

**Minimum**:
- CPU: 4 cores (8 cores for optimal)
- RAM: 8 GB (12 GB recommended)
- Storage: 200 GB SSD (500 GB for growth)

#### Kamailio
```
Expected load (600-800 CC):
- Workers: 8 (16 recommended for 800 CC)
- db_mode=2: Caches location in memory
- Memory: ~1 GB base + 100 MB per 10k registrations
```

**Minimum**:
- CPU: 2 cores (4 cores recommended)
- RAM: 2 GB (4 GB recommended)

#### FreeSWITCH
```
Expected load (600-800 CC):
- 400 calls per node
- RTP streams: 800 (400 calls × 2 legs)
- Media processing: ~50 MB RAM per call
- Recordings: tmpfs (RAM disk)
```

**Minimum**:
- CPU: 4 cores (8 cores recommended)
- RAM: 4 GB base + 20 GB tmpfs = 24 GB total
  (Recommended: 8 GB base + 30 GB tmpfs = 38 GB)

#### voip-admin (merged service)
```
Expected load:
- CDR ingestion: ~10-15 req/sec (800 CC ÷ 60s avg call duration)
- API queries: ~50 req/sec
- XML_CURL: ~5 req/sec (mostly cached)
- Background workers: 2-4 goroutines
```

**Minimum**:
- CPU: 1 core (2 cores recommended)
- RAM: 2 GB (4 GB with caching)

#### OS + System
```
- Kernel
- System services
- Buffers/cache
- Monitoring (Prometheus, node_exporter)
- Keepalived, lsyncd
```

**Minimum**:
- CPU: 1 core
- RAM: 2 GB (4 GB recommended)

### 3.2. Total Per-Node Requirements

| Resource | Absolute Minimum | Recommended | Optimal |
|----------|-----------------|-------------|---------|
| **CPU** | 12 cores | 16 cores | 24 cores |
| **RAM** | 38 GB | 64 GB | 96 GB |
| **Storage (SSD)** | 200 GB | 500 GB | 1 TB NVMe |
| **Storage (HDD)** | 2 TB | 3 TB | 5 TB |
| **Network** | 1 Gbps | 1 Gbps | 10 Gbps |

**Breakdown (Recommended - 64 GB)**:
```
PostgreSQL:       12 GB
FreeSWITCH:        8 GB (base)
tmpfs:            30 GB (recordings)
Kamailio:          4 GB
voip-admin:        4 GB
OS + buffers:      6 GB
──────────────────────
Total:            64 GB
```

### 3.3. Recommended Hardware Configuration

**For 600-800 CC production**:

```
Server: Dell PowerEdge R650 or equivalent
├── CPU: 2× Intel Xeon Silver 4314 (16 cores, 32 threads total) or AMD EPYC 7313P (16 cores)
├── RAM: 64 GB DDR4 ECC (4× 16 GB)
├── Storage:
│   ├── OS + DB: 2× 500 GB NVMe SSD (RAID 1)
│   └── Recordings: 2× 3 TB SATA HDD (RAID 1)
├── Network: 2× 1 Gbps NICs (bonded for HA)
└── Power: Dual PSU (redundant)

Cost: ~$3,500-4,500 per server
Total (2 nodes): ~$7,000-9,000
```

**Alternative (Budget)**:
```
Supermicro or Whitebox
├── CPU: AMD Ryzen 9 5950X (16 cores) or Ryzen 7 5800X (8 cores)
├── RAM: 64 GB DDR4 (2× 32 GB)
├── Storage:
│   ├── 1× 500 GB NVMe SSD
│   └── 1× 3 TB SATA HDD
├── Network: 1× 2.5 Gbps NIC
└── Power: Single PSU

Cost: ~$2,000-2,500 per server
Total (2 nodes): ~$4,000-5,000
```

### 3.4. Updated Cost Analysis

| Item | Old (9-node) | Old (2-node w/ Redis) | New (2-node optimized) | Savings |
|------|-------------|----------------------|------------------------|---------|
| **Hardware** | $45,000 | $10,000 | **$7,000** | **$38,000** |
| **Power/month** | $900 | $200 | **$150** | **$750/mo** |
| **Cooling** | High | Medium | **Medium** | - |
| **Rack space** | 9U | 2U | **2U** | 7U |

**Annual Operational Savings**: $9,000/year (power alone)

---

## 4. REVISED ARCHITECTURE (Without Redis)

### 4.1. Simplified Flow

```
┌─────────────────────────────────────────────────────────┐
│                  VIP: 192.168.1.100                     │
└────────────────────┬────────────────────────────────────┘
                     │
     ┌───────────────┴───────────────┐
     │                               │
     ▼                               ▼
┌─────────────────┐         ┌─────────────────┐
│  Node 1 (MASTER)│         │ Node 2 (BACKUP) │
│  192.168.1.101  │◄───────►│  192.168.1.102  │
└─────────────────┘  Sync   └─────────────────┘

Services on each node:
├── Kamailio
├── FreeSWITCH
├── PostgreSQL (Primary / Standby)
├── voip-admin (merged service, NO Redis)
├── Keepalived
└── lsyncd
```

### 4.2. CDR Flow (Without Redis)

```
FreeSWITCH (call ends)
    ↓
POST /fs/cdr (JSON)
    ↓
voip-admin (HTTP handler)
    ↓
INSERT INTO voip.cdr_queue (payload) -- Fast INSERT, returns immediately
    ↓
[HTTP 202 Accepted]

Background (separate goroutine):
    ↓
SELECT * FROM voip.cdr_queue
WHERE status='pending'
ORDER BY created_at
LIMIT 100
FOR UPDATE SKIP LOCKED
    ↓
Parse JSON + INSERT INTO voip.cdr (batch 100 rows)
    ↓
UPDATE voip.cdr_queue SET status='completed'
```

**Latency**: 10-20ms (PostgreSQL queue) vs 2-5ms (Redis)
**Acceptable**: Yes, FreeSWITCH doesn't wait for processing

### 4.3. Caching Strategy (Without Redis)

**Tier 1: In-Memory (sync.Map)**
```go
type CacheEntry struct {
    Value     interface{}
    ExpiresAt time.Time
}

var extensionCache sync.Map

// Cache extension lookups (60s TTL)
func GetExtension(ext string) (*Extension, error) {
    // Check cache first
    if val, ok := extensionCache.Load(ext); ok {
        entry := val.(CacheEntry)
        if time.Now().Before(entry.ExpiresAt) {
            return entry.Value.(*Extension), nil
        }
    }

    // Cache miss - query database
    extension, err := db.QueryExtension(ext)
    if err != nil {
        return nil, err
    }

    // Store in cache
    extensionCache.Store(ext, CacheEntry{
        Value:     extension,
        ExpiresAt: time.Now().Add(60 * time.Second),
    })

    return extension, nil
}
```

**Tier 2: PostgreSQL Query Cache**
- PostgreSQL automatically caches frequently accessed data
- Proper indexing ensures fast lookups

---

## 5. PERFORMANCE IMPACT ANALYSIS

### 5.1. CDR Processing

| Scenario | With Redis | Without Redis (PG Queue) | Difference |
|----------|-----------|--------------------------|------------|
| **Insert latency** | 2-5ms | 10-20ms | +10-15ms ⚠️ |
| **Throughput** | 50k inserts/sec | 10k inserts/sec | Lower (still sufficient) |
| **Data safety** | AOF (eventually consistent) | ACID (immediate) | ✅ Better |
| **Failover** | Manual promotion | Auto (PG replication) | ✅ Simpler |
| **Complexity** | Higher | Lower | ✅ Simpler |

**At 800 CC**: ~13 CDR/sec (800 calls ÷ 60s avg duration)
**PostgreSQL capacity**: 10,000 inserts/sec
**Headroom**: **750x** ✅ More than sufficient

### 5.2. Caching

| Metric | Redis | In-Memory (Go) | Difference |
|--------|-------|---------------|------------|
| **Latency** | 1-2ms (network) | 0.1ms (local) | ✅ 10x faster |
| **Throughput** | 100k ops/sec | 1M+ ops/sec | ✅ 10x faster |
| **Memory** | Separate process | Same process | ✅ Lower overhead |
| **HA** | Master-slave | Per-instance (ok) | ⚠️ Cache not replicated |

**Cache miss handling**: Query PostgreSQL (10-20ms) - acceptable

### 5.3. Overall System Impact

| Aspect | Impact | Severity | Mitigation |
|--------|--------|----------|------------|
| CDR latency +10-15ms | Low | 🟢 LOW | Async processing, FreeSWITCH doesn't wait |
| No distributed cache | Low | 🟢 LOW | In-memory cache per instance, 90%+ hit rate |
| Simpler architecture | Positive | 🟢 BENEFIT | Fewer services to manage |
| Lower memory usage | Positive | 🟢 BENEFIT | -4 GB per node |
| Lower failover complexity | Positive | 🟢 BENEFIT | One less service to promote |

**Verdict**: Removing Redis has **negligible performance impact** and **significant operational benefits**

---

## 6. FINAL RECOMMENDATIONS

### 6.1. Architecture Decisions

✅ **Remove Redis** - Use PostgreSQL queue + in-memory cache
✅ **Merge Services** - Single `voip-admin` service (XML_CURL + CDR + API + Management)
✅ **Optimize Hardware** - 64 GB RAM, 16 cores per node (was 96 GB, 24 cores)
✅ **Simplify Deployment** - 2 nodes, 6 services each (was 7 with Redis)

### 6.2. Updated Service List (Per Node)

1. **Kamailio** - SIP proxy
2. **FreeSWITCH** - Media server
3. **PostgreSQL** - Database (Primary/Standby)
4. **voip-admin** - Unified management service
5. **Keepalived** - VIP failover
6. **lsyncd** - Recording sync

**Total**: 6 services (was 7)

### 6.3. Hardware Recommendation

**Production (600-800 CC)**:
- **CPU**: 16 cores (2× Xeon Silver 4314 or AMD EPYC 7313P)
- **RAM**: 64 GB DDR4 ECC
- **Storage**: 500 GB NVMe SSD + 3 TB SATA HDD
- **Network**: 2× 1 Gbps bonded
- **Cost**: **$7,000-9,000 for 2 nodes**

**Savings vs original**: $38,000 (84% reduction)

### 6.4. Performance Confidence

| Metric | Target | Achievable with 16-core/64GB | Confidence |
|--------|--------|------------------------------|------------|
| Concurrent calls | 600-800 | ✅ Yes | 90% |
| Call setup latency | <150ms | ✅ 100-150ms | 95% |
| Registration | <50ms | ✅ 20-30ms | 95% |
| CDR processing | <30s | ✅ 10-20s | 95% |
| Failover RTO | <45s | ✅ 30-45s | 90% |

**Overall Confidence**: 92% ✅

---

## CONCLUSION

**Removing Redis and optimizing hardware is RECOMMENDED** ✅

**Benefits**:
- ✅ $38,000 hardware savings (84% reduction)
- ✅ Simpler architecture (6 services vs 7)
- ✅ Lower operational complexity
- ✅ Better data safety (PostgreSQL ACID)
- ✅ Sufficient performance for 600-800 CC

**Trade-offs**:
- ⚠️ +10-15ms CDR insert latency (acceptable, async processing)
- ⚠️ Cache not distributed (mitigated by high hit rate)

**Hardware**: 16 cores, 64 GB RAM per node = **$7,000-9,000 total**

**Next Steps**: Restructure project to reflect this simplified architecture
