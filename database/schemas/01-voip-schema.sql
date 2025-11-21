-- =============================================================================
-- VoIP Platform Schema
-- Version: 2.0 (Optimized - No Redis)
-- Description: Business logic tables for VoIP system (600-800 CC)
-- =============================================================================

-- Create schema
CREATE SCHEMA IF NOT EXISTS voip;

-- =============================================================================
-- TENANCY & USERS
-- =============================================================================

-- Domains (multi-tenancy support)
CREATE TABLE voip.domains (
    id SERIAL PRIMARY KEY,
    domain VARCHAR(255) UNIQUE NOT NULL,
    tenant_name VARCHAR(255),
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_domains_active ON voip.domains(active);

-- Users (agents, supervisors, admins)
CREATE TABLE voip.users (
    id SERIAL PRIMARY KEY,
    domain_id INT REFERENCES voip.domains(id) ON DELETE CASCADE,
    username VARCHAR(100) NOT NULL,
    email VARCHAR(255),
    full_name VARCHAR(255),
    role VARCHAR(50), -- 'agent', 'supervisor', 'admin'
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(domain_id, username)
);

CREATE INDEX idx_users_domain ON voip.users(domain_id);
CREATE INDEX idx_users_active ON voip.users(active);

-- =============================================================================
-- UNIFIED EXTENSION MODEL (Brilliant from Project B)
-- =============================================================================

-- Extensions (users, queues, IVRs, trunks - all in ONE table)
CREATE TABLE voip.extensions (
    id SERIAL PRIMARY KEY,
    domain_id INT REFERENCES voip.domains(id) ON DELETE CASCADE,
    extension VARCHAR(50) NOT NULL,
    type VARCHAR(20) NOT NULL, -- 'user', 'queue', 'ivr', 'voicemail', 'trunk_out', 'conference'
    description VARCHAR(255),

    -- Polymorphic references
    user_id INT REFERENCES voip.users(id) ON DELETE SET NULL,
    queue_id INT,
    ivr_id INT,
    voicemail_box_id INT,
    trunk_id INT,
    conference_id INT,

    -- Metadata for routing
    service_ref JSONB,
    need_media BOOLEAN DEFAULT false, -- true if needs FreeSWITCH

    recording_policy_id INT,
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    UNIQUE(domain_id, extension)
);

CREATE INDEX idx_extensions_domain ON voip.extensions(domain_id);
CREATE INDEX idx_extensions_type ON voip.extensions(type);
CREATE INDEX idx_extensions_active ON voip.extensions(active);
CREATE INDEX idx_extensions_service_ref ON voip.extensions USING GIN(service_ref);

-- =============================================================================
-- QUEUES
-- =============================================================================

CREATE TABLE voip.queues (
    id SERIAL PRIMARY KEY,
    domain_id INT REFERENCES voip.domains(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    extension VARCHAR(50),
    strategy VARCHAR(50) DEFAULT 'ring-all', -- 'ring-all', 'round-robin', 'longest-idle'
    max_wait_time INT DEFAULT 300,
    max_wait_time_with_no_agent INT DEFAULT 120,
    tier_rules_apply BOOLEAN DEFAULT true,
    discard_abandoned_after INT DEFAULT 60,
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(domain_id, name)
);

CREATE TABLE voip.queue_members (
    id SERIAL PRIMARY KEY,
    queue_id INT REFERENCES voip.queues(id) ON DELETE CASCADE,
    user_id INT REFERENCES voip.users(id) ON DELETE CASCADE,
    tier INT DEFAULT 1,
    position INT DEFAULT 1,
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(queue_id, user_id)
);

-- =============================================================================
-- IVR
-- =============================================================================

CREATE TABLE voip.ivr_menus (
    id SERIAL PRIMARY KEY,
    domain_id INT REFERENCES voip.domains(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    extension VARCHAR(50),
    greeting_sound VARCHAR(500),
    invalid_sound VARCHAR(500),
    timeout_sound VARCHAR(500),
    max_failures INT DEFAULT 3,
    max_timeouts INT DEFAULT 3,
    timeout_seconds INT DEFAULT 5,
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE voip.ivr_entries (
    id SERIAL PRIMARY KEY,
    ivr_id INT REFERENCES voip.ivr_menus(id) ON DELETE CASCADE,
    digit VARCHAR(10) NOT NULL,
    action VARCHAR(50), -- 'transfer', 'queue', 'voicemail', 'sub-menu'
    action_data VARCHAR(255),
    order_num INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);

-- =============================================================================
-- TRUNKS
-- =============================================================================

CREATE TABLE voip.trunks (
    id SERIAL PRIMARY KEY,
    domain_id INT REFERENCES voip.domains(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    type VARCHAR(20), -- 'sip', 'pstn'
    host VARCHAR(255),
    port INT DEFAULT 5060,
    username VARCHAR(100),
    password VARCHAR(255),
    prefix VARCHAR(20),
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

-- =============================================================================
-- RECORDING POLICIES (Database-driven)
-- =============================================================================

CREATE TABLE voip.recording_policies (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    record_inbound BOOLEAN DEFAULT false,
    record_outbound BOOLEAN DEFAULT false,
    record_internal BOOLEAN DEFAULT false,
    record_queue BOOLEAN DEFAULT true,
    retention_days INT DEFAULT 90,
    storage_path VARCHAR(255) DEFAULT '/storage/recordings',
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

-- =============================================================================
-- CDR QUEUE (Replaces Redis)
-- =============================================================================

-- CDR Queue table for async processing
CREATE TABLE IF NOT EXISTS voip.cdr_queue (
    id BIGSERIAL PRIMARY KEY,
    payload JSONB NOT NULL,
    status VARCHAR(20) DEFAULT 'pending', -- 'pending', 'processing', 'completed', 'failed'
    attempts INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    processed_at TIMESTAMP
);

-- Ensure all columns exist (for idempotent scripts when table already exists)
DO $$
BEGIN
    -- Ensure status column exists
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'voip' AND table_name = 'cdr_queue' AND column_name = 'status'
    ) THEN
        ALTER TABLE voip.cdr_queue ADD COLUMN status VARCHAR(20) DEFAULT 'pending';
    END IF;

    -- Ensure created_at column exists
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'voip' AND table_name = 'cdr_queue' AND column_name = 'created_at'
    ) THEN
        ALTER TABLE voip.cdr_queue ADD COLUMN created_at TIMESTAMP DEFAULT NOW();
    END IF;

    -- Ensure processed_at column exists
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'voip' AND table_name = 'cdr_queue' AND column_name = 'processed_at'
    ) THEN
        ALTER TABLE voip.cdr_queue ADD COLUMN processed_at TIMESTAMP;
    END IF;

    -- Ensure attempts column exists
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'voip' AND table_name = 'cdr_queue' AND column_name = 'attempts'
    ) THEN
        ALTER TABLE voip.cdr_queue ADD COLUMN attempts INT DEFAULT 0;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_cdr_queue_status ON voip.cdr_queue(status) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_cdr_queue_created ON voip.cdr_queue(created_at) WHERE status = 'pending';

-- =============================================================================
-- CDR (Call Detail Records)
-- =============================================================================

CREATE TABLE IF NOT EXISTS voip.cdr (
    id BIGSERIAL PRIMARY KEY,
    call_uuid UUID NOT NULL,
    bleg_uuid UUID,
    domain_id INT REFERENCES voip.domains(id),

    direction VARCHAR(20), -- 'inbound', 'outbound', 'internal'
    caller_id_number VARCHAR(50),
    caller_id_name VARCHAR(100),
    destination_number VARCHAR(50),

    context VARCHAR(100),

    start_time TIMESTAMP,
    answer_time TIMESTAMP,
    end_time TIMESTAMP,
    duration INT, -- seconds
    billsec INT, -- billable seconds

    hangup_cause VARCHAR(50),

    queue_id INT REFERENCES voip.queues(id),
    agent_user_id INT REFERENCES voip.users(id),

    recording_id INT,

    sip_call_id VARCHAR(255),

    created_at TIMESTAMP DEFAULT NOW()
);

-- Ensure critical columns exist (for idempotent scripts)
DO $$
BEGIN
    -- Ensure call_uuid column exists
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'voip' AND table_name = 'cdr' AND column_name = 'call_uuid'
    ) THEN
        ALTER TABLE voip.cdr ADD COLUMN call_uuid UUID NOT NULL;
    END IF;

    -- Ensure created_at column exists
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'voip' AND table_name = 'cdr' AND column_name = 'created_at'
    ) THEN
        ALTER TABLE voip.cdr ADD COLUMN created_at TIMESTAMP DEFAULT NOW();
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_cdr_call_uuid ON voip.cdr(call_uuid);
CREATE INDEX IF NOT EXISTS idx_cdr_domain ON voip.cdr(domain_id);
CREATE INDEX IF NOT EXISTS idx_cdr_start_time ON voip.cdr(start_time);
CREATE INDEX IF NOT EXISTS idx_cdr_queue ON voip.cdr(queue_id);
CREATE INDEX IF NOT EXISTS idx_cdr_agent ON voip.cdr(agent_user_id);

-- =============================================================================
-- RECORDINGS
-- =============================================================================

CREATE TABLE voip.recordings (
    id BIGSERIAL PRIMARY KEY,
    call_uuid UUID NOT NULL,
    cdr_id BIGINT REFERENCES voip.cdr(id) ON DELETE SET NULL,

    file_path VARCHAR(500),
    file_size BIGINT,
    duration INT,
    format VARCHAR(20) DEFAULT 'wav',

    start_time TIMESTAMP,
    end_time TIMESTAMP,

    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_recordings_call_uuid ON voip.recordings(call_uuid);
CREATE INDEX idx_recordings_cdr ON voip.recordings(cdr_id);

-- =============================================================================
-- API KEYS
-- =============================================================================

CREATE TABLE voip.api_keys (
    id SERIAL PRIMARY KEY,
    key_name VARCHAR(100) NOT NULL,
    api_key VARCHAR(255) UNIQUE NOT NULL,
    permissions JSONB, -- {"cdr": "read", "recordings": "read"}
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP
);

CREATE INDEX idx_api_keys_active ON voip.api_keys(active);

-- =============================================================================
-- DEFAULT DATA
-- =============================================================================

-- Default domain
INSERT INTO voip.domains (domain, tenant_name)
VALUES ('default.local', 'Default Tenant');

-- Default recording policy
INSERT INTO voip.recording_policies (name, record_queue, retention_days)
VALUES ('Default Queue Recording', true, 90);

-- =============================================================================
-- VIEWS
-- =============================================================================

-- Unified extension view for routing
CREATE OR REPLACE VIEW voip.vw_extensions AS
SELECT
    e.id,
    e.domain_id,
    d.domain,
    e.extension,
    e.type,
    e.description,
    e.need_media,
    e.service_ref,
    e.active,

    -- User details
    u.username AS user_username,
    u.full_name AS user_full_name,

    -- Queue details
    q.name AS queue_name,
    q.strategy AS queue_strategy,

    -- Recording policy
    rp.name AS recording_policy
FROM voip.extensions e
LEFT JOIN voip.domains d ON e.domain_id = d.id
LEFT JOIN voip.users u ON e.user_id = u.id
LEFT JOIN voip.queues q ON e.queue_id = q.id
LEFT JOIN voip.recording_policies rp ON e.recording_policy_id = rp.id
WHERE e.active = true AND d.active = true;

-- =============================================================================
-- GRANTS (Adjust based on your security requirements)
-- =============================================================================

-- Grant to voip_admin user (create this user first)
-- GRANT USAGE ON SCHEMA voip TO voip_admin;
-- GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA voip TO voip_admin;
-- GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA voip TO voip_admin;
