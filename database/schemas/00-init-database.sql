-- =============================================================================
-- Database Initialization Script
-- Description: Create database, schemas, users, and grant permissions
-- Version: 1.0
-- Date: 2025-11-21
-- Run as: postgres superuser
-- =============================================================================

-- =============================================================================
-- PART 1: Create Database and Schemas
-- =============================================================================

-- Create database (idempotent - safe if already exists)
-- Note: This must be run separately before other scripts:
--   CREATE DATABASE voipdb;

-- Connect to voipdb
\c voipdb

-- Create schemas
CREATE SCHEMA IF NOT EXISTS voip;
CREATE SCHEMA IF NOT EXISTS kamailio;

COMMENT ON SCHEMA voip IS 'VoIP system tables: extensions, queues, CDR, recordings';
COMMENT ON SCHEMA kamailio IS 'Kamailio SIP proxy tables: location, dialog, dispatcher, etc.';

-- =============================================================================
-- PART 2: Create Database Users
-- =============================================================================

-- Create users (if not exists)
DO $$
BEGIN
    -- Kamailio read-write user
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'kamailio') THEN
        CREATE USER kamailio WITH PASSWORD 'CHANGE_ME_KAMAILIO_PASSWORD';
    END IF;

    -- Kamailio read-only user (for kamctl queries)
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'kamailioro') THEN
        CREATE USER kamailioro WITH PASSWORD 'CHANGE_ME_KAMAILIORO_PASSWORD';
    END IF;

    -- VoIP Admin user
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'voipadmin') THEN
        CREATE USER voipadmin WITH PASSWORD 'CHANGE_ME_VOIPADMIN_PASSWORD';
    END IF;

    -- FreeSWITCH user (for future ODBC if needed)
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'freeswitch') THEN
        CREATE USER freeswitch WITH PASSWORD 'CHANGE_ME_FREESWITCH_PASSWORD';
    END IF;

    -- Replicator user (for streaming replication)
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'replicator') THEN
        CREATE USER replicator WITH REPLICATION PASSWORD 'CHANGE_ME_REPLICATOR_PASSWORD';
    END IF;
END $$;

-- =============================================================================
-- PART 3: Grant Permissions
-- =============================================================================

-- Grant CONNECT on database
GRANT CONNECT ON DATABASE voipdb TO kamailio, kamailioro, voipadmin, freeswitch;

-- Grant USAGE on schemas
GRANT USAGE ON SCHEMA voip TO kamailio, kamailioro, voipadmin, freeswitch;
GRANT USAGE ON SCHEMA kamailio TO kamailio, kamailioro;

-- ==== Kamailio permissions ====
-- Full access to kamailio schema
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA kamailio TO kamailio;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA kamailio TO kamailio;
ALTER DEFAULT PRIVILEGES IN SCHEMA kamailio GRANT ALL ON TABLES TO kamailio;
ALTER DEFAULT PRIVILEGES IN SCHEMA kamailio GRANT ALL ON SEQUENCES TO kamailio;

-- Set search_path for kamailio user (CRITICAL for kamctl)
ALTER USER kamailio SET search_path TO kamailio, public;

-- Read-only access for kamailioro
GRANT SELECT ON ALL TABLES IN SCHEMA kamailio TO kamailioro;
ALTER DEFAULT PRIVILEGES IN SCHEMA kamailio GRANT SELECT ON TABLES TO kamailioro;
ALTER USER kamailioro SET search_path TO kamailio, public;

-- Read access to voip.extensions and voip.domains (for subscriber view)
GRANT SELECT ON voip.extensions TO kamailio, kamailioro;
GRANT SELECT ON voip.domains TO kamailio, kamailioro;

-- ==== VoIP Admin permissions ====
-- Full access to voip schema
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA voip TO voipadmin;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA voip TO voipadmin;
ALTER DEFAULT PRIVILEGES IN SCHEMA voip GRANT ALL ON TABLES TO voipadmin;
ALTER DEFAULT PRIVILEGES IN SCHEMA voip GRANT ALL ON SEQUENCES TO voipadmin;

-- Read access to kamailio schema (for monitoring, statistics)
GRANT SELECT ON ALL TABLES IN SCHEMA kamailio TO voipadmin;
ALTER DEFAULT PRIVILEGES IN SCHEMA kamailio GRANT SELECT ON TABLES TO voipadmin;

-- ==== FreeSWITCH permissions ====
-- Read access to voip schema (for directory/dialplan queries if using ODBC)
-- Note: Current architecture uses voip-admin API, not direct ODBC
GRANT SELECT ON voip.extensions TO freeswitch;
GRANT SELECT ON voip.domains TO freeswitch;
GRANT SELECT ON voip.queues TO freeswitch;

-- =============================================================================
-- PART 4: Verification
-- =============================================================================

-- List all users
SELECT rolname, rolsuper, rolcanlogin, rolreplication
FROM pg_roles
WHERE rolname IN ('kamailio', 'kamailioro', 'voipadmin', 'freeswitch', 'replicator')
ORDER BY rolname;

-- List all schemas
SELECT schema_name, schema_owner
FROM information_schema.schemata
WHERE schema_name IN ('voip', 'kamailio')
ORDER BY schema_name;

-- =============================================================================
-- SECURITY NOTES
-- =============================================================================

-- IMPORTANT: Change all passwords after running this script!
-- 1. ALTER USER kamailio WITH PASSWORD 'YourSecurePassword1';
-- 2. ALTER USER kamailioro WITH PASSWORD 'YourSecurePassword2';
-- 3. ALTER USER voipadmin WITH PASSWORD 'YourSecurePassword3';
-- 4. ALTER USER freeswitch WITH PASSWORD 'YourSecurePassword4';
-- 5. ALTER USER replicator WITH PASSWORD 'YourSecurePassword5';
--
-- Save passwords in /root/.voip_credentials (mode 600)

-- =============================================================================
-- END OF INITIALIZATION SCRIPT
-- =============================================================================

-- Changelog:
-- v1.0 (2025-11-21): Initial script for database, schemas, users, and permissions
