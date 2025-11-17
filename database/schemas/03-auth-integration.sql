-- =============================================================================
-- Kamailio 6.0.4 Authentication Integration with VoIP Schema
-- Description: Integrate kamailio.subscriber with voip.extensions
-- Version: 1.0
-- Date: 2025-01-17
-- Based on: Kamailio 6.0.4 official documentation
-- =============================================================================

-- =============================================================================
-- PART 1: Add SIP Authentication Columns to voip.extensions
-- =============================================================================

-- Add SIP auth columns (if not exist)
ALTER TABLE voip.extensions
ADD COLUMN IF NOT EXISTS sip_password VARCHAR(128),
ADD COLUMN IF NOT EXISTS sip_ha1 VARCHAR(128),
ADD COLUMN IF NOT EXISTS sip_ha1b VARCHAR(128);

-- Index for fast auth lookups (only for user extensions)
CREATE INDEX IF NOT EXISTS idx_extensions_sip_auth
ON voip.extensions(sip_ha1)
WHERE type = 'user' AND active = true;

COMMENT ON COLUMN voip.extensions.sip_password IS 'SIP plaintext password (encrypted at rest)';
COMMENT ON COLUMN voip.extensions.sip_ha1 IS 'MD5(extension:domain:password) for Kamailio auth';
COMMENT ON COLUMN voip.extensions.sip_ha1b IS 'MD5(extension@domain:domain:password) for Kamailio auth with domain';

-- =============================================================================
-- PART 2: Helper Function for HA1 Calculation
-- =============================================================================

-- Function to calculate HA1 values according to RFC 2617
CREATE OR REPLACE FUNCTION voip.calculate_sip_ha1(
    p_extension VARCHAR,
    p_domain VARCHAR,
    p_password VARCHAR
) RETURNS TABLE(ha1 VARCHAR, ha1b VARCHAR) AS $$
BEGIN
    -- ha1  = MD5(username:realm:password)
    -- ha1b = MD5(username@domain:realm:password)
    RETURN QUERY SELECT
        MD5(p_extension || ':' || p_domain || ':' || p_password)::VARCHAR AS ha1,
        MD5(p_extension || '@' || p_domain || ':' || p_domain || ':' || p_password)::VARCHAR AS ha1b;
END;
$$ LANGUAGE plpgsql IMMUTABLE SECURITY DEFINER;

COMMENT ON FUNCTION voip.calculate_sip_ha1 IS 'Calculate HA1 and HA1b digest hashes for SIP authentication (RFC 2617)';

-- =============================================================================
-- PART 3: Auto-update Trigger for HA1 Calculation
-- =============================================================================

-- Trigger function to auto-calculate HA1 when password changes
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
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Create trigger on INSERT and UPDATE
DROP TRIGGER IF EXISTS extensions_calc_ha1_trigger ON voip.extensions;
CREATE TRIGGER extensions_calc_ha1_trigger
    BEFORE INSERT OR UPDATE OF sip_password ON voip.extensions
    FOR EACH ROW
    EXECUTE FUNCTION voip.extensions_calc_ha1_trigger();

COMMENT ON TRIGGER extensions_calc_ha1_trigger ON voip.extensions IS 'Auto-calculate HA1 digests when SIP password changes';

-- =============================================================================
-- PART 4: Replace Kamailio subscriber TABLE with VIEW
-- =============================================================================

-- Drop existing subscriber table (backup first if needed!)
-- WARNING: Only run this if kamailio.subscriber table is empty or you have backup
DROP TABLE IF EXISTS kamailio.subscriber CASCADE;

-- Create VIEW for Kamailio authentication
-- This VIEW joins voip schema and presents data in format Kamailio expects
CREATE OR REPLACE VIEW kamailio.subscriber AS
SELECT
    e.id,
    e.extension AS username,
    d.domain AS domain,
    COALESCE(e.sip_password, '') AS password,
    COALESCE(u.email, '') AS email_address,
    COALESCE(e.sip_ha1, '') AS ha1,
    COALESCE(e.sip_ha1b, '') AS ha1b,
    NULL::VARCHAR AS rpid
FROM voip.extensions e
INNER JOIN voip.domains d ON e.domain_id = d.id
LEFT JOIN voip.users u ON e.user_id = u.id
WHERE e.type = 'user'
  AND e.active = true
  AND d.active = true;

COMMENT ON VIEW kamailio.subscriber IS 'Kamailio 6.0.4 subscriber view - integrates with voip.extensions';

-- Grant permissions
GRANT SELECT ON kamailio.subscriber TO kamailio;

-- =============================================================================
-- PART 5: INSTEAD OF Triggers for INSERT/UPDATE/DELETE via kamctl
-- =============================================================================

-- INSERT trigger: Allow kamctl to add users via subscriber view
CREATE OR REPLACE FUNCTION kamailio.subscriber_insert_trigger() RETURNS TRIGGER AS $$
DECLARE
    v_domain_id INT;
    v_user_id INT;
BEGIN
    -- Lookup domain_id
    SELECT id INTO v_domain_id
    FROM voip.domains
    WHERE domain = NEW.domain AND active = true
    LIMIT 1;

    IF v_domain_id IS NULL THEN
        RAISE EXCEPTION 'Domain "%" not found or inactive', NEW.domain;
    END IF;

    -- Check if extension already exists
    IF EXISTS (
        SELECT 1 FROM voip.extensions
        WHERE extension = NEW.username
        AND domain_id = v_domain_id
    ) THEN
        RAISE EXCEPTION 'Extension "%" already exists in domain "%"', NEW.username, NEW.domain;
    END IF;

    -- Insert into voip.extensions
    INSERT INTO voip.extensions (
        domain_id,
        extension,
        type,
        sip_password,
        sip_ha1,
        sip_ha1b,
        active,
        created_at
    ) VALUES (
        v_domain_id,
        NEW.username,
        'user',
        NEW.password,
        NEW.ha1,
        NEW.ha1b,
        true,
        NOW()
    );

    RETURN NEW;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

DROP TRIGGER IF EXISTS subscriber_insert_trigger ON kamailio.subscriber;
CREATE TRIGGER subscriber_insert_trigger
    INSTEAD OF INSERT ON kamailio.subscriber
    FOR EACH ROW
    EXECUTE FUNCTION kamailio.subscriber_insert_trigger();

-- UPDATE trigger: Allow kamctl to update passwords
CREATE OR REPLACE FUNCTION kamailio.subscriber_update_trigger() RETURNS TRIGGER AS $$
DECLARE
    v_domain_id INT;
BEGIN
    -- Lookup domain_id
    SELECT id INTO v_domain_id
    FROM voip.domains
    WHERE domain = OLD.domain
    LIMIT 1;

    IF v_domain_id IS NULL THEN
        RAISE EXCEPTION 'Domain "%" not found', OLD.domain;
    END IF;

    -- Update voip.extensions
    UPDATE voip.extensions
    SET
        sip_password = NEW.password,
        sip_ha1 = NEW.ha1,
        sip_ha1b = NEW.ha1b,
        updated_at = NOW()
    WHERE extension = OLD.username
      AND domain_id = v_domain_id
      AND type = 'user';

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Extension "%" not found in domain "%"', OLD.username, OLD.domain;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

DROP TRIGGER IF EXISTS subscriber_update_trigger ON kamailio.subscriber;
CREATE TRIGGER subscriber_update_trigger
    INSTEAD OF UPDATE ON kamailio.subscriber
    FOR EACH ROW
    EXECUTE FUNCTION kamailio.subscriber_update_trigger();

-- DELETE trigger: Soft delete (set active = false)
CREATE OR REPLACE FUNCTION kamailio.subscriber_delete_trigger() RETURNS TRIGGER AS $$
DECLARE
    v_domain_id INT;
BEGIN
    -- Lookup domain_id
    SELECT id INTO v_domain_id
    FROM voip.domains
    WHERE domain = OLD.domain
    LIMIT 1;

    IF v_domain_id IS NULL THEN
        RAISE EXCEPTION 'Domain "%" not found', OLD.domain;
    END IF;

    -- Soft delete: set active = false (preserves audit trail)
    UPDATE voip.extensions
    SET
        active = false,
        updated_at = NOW()
    WHERE extension = OLD.username
      AND domain_id = v_domain_id
      AND type = 'user';

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Extension "%" not found in domain "%"', OLD.username, OLD.domain;
    END IF;

    RETURN OLD;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

DROP TRIGGER IF EXISTS subscriber_delete_trigger ON kamailio.subscriber;
CREATE TRIGGER subscriber_delete_trigger
    INSTEAD OF DELETE ON kamailio.subscriber
    FOR EACH ROW
    EXECUTE FUNCTION kamailio.subscriber_delete_trigger();

-- =============================================================================
-- PART 6: Update voip.cdr_queue for FreeSWITCH Integration
-- =============================================================================

-- Add columns for async CDR processing
ALTER TABLE voip.cdr_queue
ADD COLUMN IF NOT EXISTS raw_xml TEXT,
ADD COLUMN IF NOT EXISTS status VARCHAR(20) DEFAULT 'pending',
ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT NOW();

-- Index for worker efficiency (only pending records)
CREATE INDEX IF NOT EXISTS idx_cdr_queue_status
ON voip.cdr_queue(status, created_at)
WHERE status = 'pending';

-- Index for cleanup (processed/failed records)
CREATE INDEX IF NOT EXISTS idx_cdr_queue_cleanup
ON voip.cdr_queue(status, created_at)
WHERE status IN ('processed', 'failed');

COMMENT ON COLUMN voip.cdr_queue.raw_xml IS 'Raw XML CDR from FreeSWITCH mod_xml_cdr';
COMMENT ON COLUMN voip.cdr_queue.status IS 'Processing status: pending, processed, failed';

-- =============================================================================
-- PART 7: Sample Data for Testing
-- =============================================================================

-- Create test user (optional - comment out for production)
/*
DO $$
DECLARE
    v_domain_id INT;
    v_test_password VARCHAR := 'Test123!';
    v_ha1_result RECORD;
BEGIN
    -- Get default domain
    SELECT id INTO v_domain_id FROM voip.domains WHERE domain = 'default.local' LIMIT 1;

    IF v_domain_id IS NOT NULL THEN
        -- Calculate HA1 for test user
        SELECT * INTO v_ha1_result FROM voip.calculate_sip_ha1('1000', 'default.local', v_test_password);

        -- Insert test extension
        INSERT INTO voip.extensions (
            domain_id, extension, type, description,
            sip_password, sip_ha1, sip_ha1b, active
        ) VALUES (
            v_domain_id, '1000', 'user', 'Test User 1000',
            v_test_password, v_ha1_result.ha1, v_ha1_result.ha1b, true
        ) ON CONFLICT (domain_id, extension) DO NOTHING;

        RAISE NOTICE 'Test user created: 1000@default.local (password: %)', v_test_password;
    END IF;
END $$;
*/

-- =============================================================================
-- PART 8: Verification Queries
-- =============================================================================

-- Verify subscriber view
-- SELECT * FROM kamailio.subscriber LIMIT 5;

-- Test HA1 calculation
-- SELECT * FROM voip.calculate_sip_ha1('1000', 'default.local', 'Test123!');

-- Check if triggers work
-- INSERT INTO kamailio.subscriber (username, domain, password) VALUES ('1001', 'default.local', 'Pass456!');
-- SELECT * FROM voip.extensions WHERE extension = '1001';

-- =============================================================================
-- END OF INTEGRATION SCHEMA
-- =============================================================================

-- Changelog:
-- v1.0 (2025-01-17): Initial schema for Kamailio 6.0.4 integration
--                    Based on official Kamailio 6.0.4 documentation
--                    Supports VIEW-based subscriber table
--                    Auto HA1 calculation triggers
--                    INSTEAD OF triggers for kamctl compatibility
