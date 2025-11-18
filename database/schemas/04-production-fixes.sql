-- =============================================================================
-- Production Fixes for VoIP Platform
-- Version: 1.0
-- Date: 2025-01-18
-- Description: Critical schema fixes aligned with VoIP Admin Go code
-- =============================================================================

-- =============================================================================
-- PART 1: Fix voip.cdr_queue Schema to Match Go Code
-- =============================================================================

-- Drop existing table (BACKUP FIRST if has data!)
DROP TABLE IF EXISTS voip.cdr_queue CASCADE;

-- Recreate with correct schema matching workers/cdr_processor.go
CREATE TABLE voip.cdr_queue (
    id BIGSERIAL PRIMARY KEY,
    uuid VARCHAR(255) NOT NULL UNIQUE,
    xml_data TEXT NOT NULL,
    received_at TIMESTAMP DEFAULT NOW() NOT NULL,
    processed_at TIMESTAMP,
    retry_count INT DEFAULT 0 NOT NULL,
    error_message TEXT
);

-- Optimized index for worker queries (FOR UPDATE SKIP LOCKED)
CREATE INDEX idx_cdr_queue_pending
ON voip.cdr_queue(received_at)
WHERE processed_at IS NULL AND retry_count < 3;

-- Index for cleanup operations
CREATE INDEX idx_cdr_queue_cleanup
ON voip.cdr_queue(processed_at)
WHERE processed_at IS NOT NULL;

COMMENT ON TABLE voip.cdr_queue IS 'CDR processing queue for async FreeSWITCH CDR ingestion';
COMMENT ON COLUMN voip.cdr_queue.uuid IS 'Call UUID from FreeSWITCH';
COMMENT ON COLUMN voip.cdr_queue.xml_data IS 'Raw XML CDR from FreeSWITCH mod_xml_cdr';
COMMENT ON COLUMN voip.cdr_queue.retry_count IS 'Number of processing attempts (max 3)';

-- =============================================================================
-- PART 2: Fix voip.cdr Schema to Match Go Models
-- =============================================================================

-- Add missing columns that Go code expects
ALTER TABLE voip.cdr
ADD COLUMN IF NOT EXISTS hangup_cause_q850 INT,
ADD COLUMN IF NOT EXISTS sip_hangup_disposition VARCHAR(100),
ADD COLUMN IF NOT EXISTS call_type VARCHAR(50),
ADD COLUMN IF NOT EXISTS context VARCHAR(100),
ADD COLUMN IF NOT EXISTS read_codec VARCHAR(50),
ADD COLUMN IF NOT EXISTS write_codec VARCHAR(50),
ADD COLUMN IF NOT EXISTS remote_media_ip VARCHAR(50),
ADD COLUMN IF NOT EXISTS rtp_audio_in_mos NUMERIC(4,2),
ADD COLUMN IF NOT EXISTS rtp_audio_in_packet_count INT,
ADD COLUMN IF NOT EXISTS rtp_audio_in_packet_loss INT,
ADD COLUMN IF NOT EXISTS rtp_audio_in_jitter_min INT,
ADD COLUMN IF NOT EXISTS rtp_audio_in_jitter_max INT,
ADD COLUMN IF NOT EXISTS sip_from_user VARCHAR(100),
ADD COLUMN IF NOT EXISTS sip_to_user VARCHAR(100),
ADD COLUMN IF NOT EXISTS sip_call_id VARCHAR(255),
ADD COLUMN IF NOT EXISTS user_agent VARCHAR(255),
ADD COLUMN IF NOT EXISTS record_file VARCHAR(500),
ADD COLUMN IF NOT EXISTS record_duration INT,
ADD COLUMN IF NOT EXISTS queue_wait_time INT,
ADD COLUMN IF NOT EXISTS agent_extension VARCHAR(50),
ADD COLUMN IF NOT EXISTS holdsec INT DEFAULT 0;

-- Rename call_uuid to uuid for consistency with Go code
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'voip'
        AND table_name = 'cdr'
        AND column_name = 'call_uuid'
    ) THEN
        ALTER TABLE voip.cdr RENAME COLUMN call_uuid TO uuid;
    END IF;
END $$;

-- Ensure uuid column exists and is VARCHAR (not UUID type)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'voip'
        AND table_name = 'cdr'
        AND column_name = 'uuid'
    ) THEN
        ALTER TABLE voip.cdr ADD COLUMN uuid VARCHAR(255) NOT NULL UNIQUE;
    END IF;
END $$;

-- Add check constraint for call_type
ALTER TABLE voip.cdr
DROP CONSTRAINT IF EXISTS chk_cdr_call_type;

ALTER TABLE voip.cdr
ADD CONSTRAINT chk_cdr_call_type
CHECK (call_type IN ('queue', 'direct', 'ivr', 'conference', 'other') OR call_type IS NULL);

-- Add check constraint for direction
ALTER TABLE voip.cdr
DROP CONSTRAINT IF EXISTS chk_cdr_direction;

ALTER TABLE voip.cdr
ADD CONSTRAINT chk_cdr_direction
CHECK (direction IN ('inbound', 'outbound', 'internal') OR direction IS NULL);

-- =============================================================================
-- PART 3: Add Critical Missing Indexes
-- =============================================================================

-- CRITICAL: Composite index for directory lookups (FreeSWITCH auth)
-- This is the MOST IMPORTANT index for performance!
CREATE INDEX IF NOT EXISTS idx_extensions_auth_lookup
ON voip.extensions(extension, domain_id)
INCLUDE (sip_ha1, sip_ha1b, display_name, vm_password, vm_email, max_concurrent, call_timeout)
WHERE type = 'user' AND active = true;

-- Index for CDR queries with filters
CREATE INDEX IF NOT EXISTS idx_cdr_caller_pattern
ON voip.cdr(caller_id_number text_pattern_ops);

CREATE INDEX IF NOT EXISTS idx_cdr_dest_pattern
ON voip.cdr(destination_number text_pattern_ops);

CREATE INDEX IF NOT EXISTS idx_cdr_direction
ON voip.cdr(direction)
WHERE direction IS NOT NULL;

-- Composite index for common CDR queries
CREATE INDEX IF NOT EXISTS idx_cdr_time_direction
ON voip.cdr(start_time DESC, direction, queue_id)
WHERE start_time IS NOT NULL;

-- Index for queue member lookups
CREATE INDEX IF NOT EXISTS idx_queue_members_queue
ON voip.queue_members(queue_id, tier, position)
WHERE active = true;

CREATE INDEX IF NOT EXISTS idx_queue_members_user
ON voip.queue_members(user_id)
WHERE active = true;

-- =============================================================================
-- PART 4: Add Missing voip.extensions Columns
-- =============================================================================

-- Add columns that Go code expects but schema doesn't have
ALTER TABLE voip.extensions
ADD COLUMN IF NOT EXISTS display_name VARCHAR(255),
ADD COLUMN IF NOT EXISTS email VARCHAR(255),
ADD COLUMN IF NOT EXISTS vm_password VARCHAR(10),
ADD COLUMN IF NOT EXISTS vm_email VARCHAR(255),
ADD COLUMN IF NOT EXISTS max_concurrent INT DEFAULT 3,
ADD COLUMN IF NOT EXISTS call_timeout INT DEFAULT 30;

-- Add check constraints
ALTER TABLE voip.extensions
DROP CONSTRAINT IF EXISTS chk_extensions_type;

ALTER TABLE voip.extensions
ADD CONSTRAINT chk_extensions_type
CHECK (type IN ('user', 'queue', 'ivr', 'voicemail', 'trunk_out', 'conference'));

ALTER TABLE voip.extensions
DROP CONSTRAINT IF EXISTS chk_extensions_max_concurrent;

ALTER TABLE voip.extensions
ADD CONSTRAINT chk_extensions_max_concurrent
CHECK (max_concurrent >= 1 AND max_concurrent <= 100);

ALTER TABLE voip.extensions
DROP CONSTRAINT IF EXISTS chk_extensions_call_timeout;

ALTER TABLE voip.extensions
ADD CONSTRAINT chk_extensions_call_timeout
CHECK (call_timeout >= 10 AND call_timeout <= 300);

-- =============================================================================
-- PART 5: Update voip.queues Schema
-- =============================================================================

-- Match queue schema with Go models
ALTER TABLE voip.queues
ADD COLUMN IF NOT EXISTS moh VARCHAR(255) DEFAULT 'default',
ADD COLUMN IF NOT EXISTS record_template VARCHAR(255),
ADD COLUMN IF NOT EXISTS time_base_score VARCHAR(20) DEFAULT 'queue',
ADD COLUMN IF NOT EXISTS tier_rule_wait_second INT DEFAULT 30,
ADD COLUMN IF NOT EXISTS abandoned_resume_allowed BOOLEAN DEFAULT false,
ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT NOW();

-- Rename columns for consistency
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'voip'
        AND table_name = 'queues'
        AND column_name = 'max_wait_time_with_no_agent'
    ) THEN
        ALTER TABLE voip.queues
        RENAME COLUMN max_wait_time_with_no_agent TO max_wait_time_no_agent;
    END IF;
END $$;

-- Add check constraint
ALTER TABLE voip.queues
DROP CONSTRAINT IF EXISTS chk_queues_strategy;

ALTER TABLE voip.queues
ADD CONSTRAINT chk_queues_strategy
CHECK (strategy IN (
    'ring-all',
    'longest-idle-agent',
    'round-robin',
    'top-down',
    'agent-with-least-talk-time',
    'agent-with-fewest-calls',
    'sequentially-by-agent-order',
    'random'
));

-- =============================================================================
-- PART 6: Update voip.queue_members Schema
-- =============================================================================

-- Match with Go models
ALTER TABLE voip.queue_members
RENAME TO queue_agents;

-- Add missing columns
ALTER TABLE voip.queue_agents
ADD COLUMN IF NOT EXISTS extension_id INT REFERENCES voip.extensions(id) ON DELETE CASCADE,
ADD COLUMN IF NOT EXISTS state VARCHAR(50) DEFAULT 'Available',
ADD COLUMN IF NOT EXISTS status VARCHAR(50) DEFAULT 'Waiting',
ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT NOW();

-- Remove old user_id if migrating to extension_id
-- ALTER TABLE voip.queue_agents DROP COLUMN IF EXISTS user_id;

-- Add check constraints
ALTER TABLE voip.queue_agents
DROP CONSTRAINT IF EXISTS chk_queue_agents_state;

ALTER TABLE voip.queue_agents
ADD CONSTRAINT chk_queue_agents_state
CHECK (state IN ('Available', 'On Break', 'Logged Out'));

ALTER TABLE voip.queue_agents
DROP CONSTRAINT IF EXISTS chk_queue_agents_status;

ALTER TABLE voip.queue_agents
ADD CONSTRAINT chk_queue_agents_status
CHECK (status IN ('Waiting', 'Receiving', 'In a queue call'));

ALTER TABLE voip.queue_agents
DROP CONSTRAINT IF EXISTS chk_queue_agents_tier;

ALTER TABLE voip.queue_agents
ADD CONSTRAINT chk_queue_agents_tier
CHECK (tier >= 1 AND tier <= 10);

-- =============================================================================
-- PART 7: Performance Optimization Functions
-- =============================================================================

-- Function for CDR queue cleanup (called by Go CleanupWorker)
CREATE OR REPLACE FUNCTION voip.cleanup_old_cdr_queue(days INT DEFAULT 7)
RETURNS INT
LANGUAGE plpgsql
AS $$
DECLARE
    deleted_count INT;
BEGIN
    DELETE FROM voip.cdr_queue
    WHERE processed_at IS NOT NULL
      AND processed_at < NOW() - (days || ' days')::INTERVAL;

    GET DIAGNOSTICS deleted_count = ROW_COUNT;

    RAISE NOTICE 'Cleaned up % old CDR queue entries', deleted_count;
    RETURN deleted_count;
END;
$$;

COMMENT ON FUNCTION voip.cleanup_old_cdr_queue IS 'Cleanup processed CDR queue entries older than specified days';

-- Function to get extension with domain lookup (optimized)
CREATE OR REPLACE FUNCTION voip.get_extension_for_auth(
    p_extension VARCHAR,
    p_domain VARCHAR
) RETURNS TABLE (
    id INT,
    extension VARCHAR,
    sip_ha1 VARCHAR,
    sip_ha1b VARCHAR,
    display_name VARCHAR,
    vm_password VARCHAR,
    vm_email VARCHAR,
    max_concurrent INT,
    call_timeout INT
)
LANGUAGE plpgsql STABLE
AS $$
BEGIN
    RETURN QUERY
    SELECT
        e.id,
        e.extension,
        e.sip_ha1,
        e.sip_ha1b,
        e.display_name,
        e.vm_password,
        e.vm_email,
        e.max_concurrent,
        e.call_timeout
    FROM voip.extensions e
    INNER JOIN voip.domains d ON e.domain_id = d.id
    WHERE e.extension = p_extension
      AND d.domain = p_domain
      AND e.type = 'user'
      AND e.active = true
      AND d.active = true;
END;
$$;

COMMENT ON FUNCTION voip.get_extension_for_auth IS 'Optimized extension lookup for FreeSWITCH directory authentication';

-- =============================================================================
-- PART 8: Update Triggers for New Columns
-- =============================================================================

-- Update HA1 trigger to handle new columns
CREATE OR REPLACE FUNCTION voip.extensions_calc_ha1_trigger() RETURNS TRIGGER AS $$
DECLARE
    v_domain VARCHAR;
    v_ha1_result RECORD;
BEGIN
    -- Only process for user extensions with password
    IF NEW.type = 'user' AND NEW.sip_password IS NOT NULL AND NEW.sip_password != '' THEN
        -- Get domain name
        SELECT domain INTO v_domain
        FROM voip.domains
        WHERE id = NEW.domain_id;

        IF v_domain IS NULL THEN
            RAISE EXCEPTION 'Domain not found for domain_id: %', NEW.domain_id;
        END IF;

        -- Calculate HA1 values
        SELECT * INTO v_ha1_result
        FROM voip.calculate_sip_ha1(NEW.extension, v_domain, NEW.sip_password);

        -- Update HA1 columns
        NEW.sip_ha1 := v_ha1_result.ha1;
        NEW.sip_ha1b := v_ha1_result.ha1b;

        -- Set updated_at
        NEW.updated_at := NOW();
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- =============================================================================
-- PART 9: Verify Schema
-- =============================================================================

-- Verification queries (run after applying fixes)
DO $$
BEGIN
    RAISE NOTICE '=== Schema Verification ===';

    -- Check cdr_queue structure
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'voip'
        AND table_name = 'cdr_queue'
        AND column_name = 'uuid'
    ) THEN
        RAISE NOTICE '✅ voip.cdr_queue has uuid column';
    ELSE
        RAISE WARNING '❌ voip.cdr_queue missing uuid column';
    END IF;

    -- Check cdr structure
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'voip'
        AND table_name = 'cdr'
        AND column_name = 'call_type'
    ) THEN
        RAISE NOTICE '✅ voip.cdr has call_type column';
    ELSE
        RAISE WARNING '❌ voip.cdr missing call_type column';
    END IF;

    -- Check critical index
    IF EXISTS (
        SELECT 1 FROM pg_indexes
        WHERE schemaname = 'voip'
        AND tablename = 'extensions'
        AND indexname = 'idx_extensions_auth_lookup'
    ) THEN
        RAISE NOTICE '✅ Critical auth lookup index exists';
    ELSE
        RAISE WARNING '❌ Critical auth lookup index missing';
    END IF;

    RAISE NOTICE '=== Verification Complete ===';
END $$;

-- =============================================================================
-- END OF PRODUCTION FIXES
-- =============================================================================

-- Changelog:
-- v1.0 (2025-01-18): Initial production fixes
--                    - Aligned voip.cdr_queue with Go code
--                    - Aligned voip.cdr with Go models
--                    - Added critical missing indexes
--                    - Added missing columns to extensions/queues
--                    - Added performance optimization functions
