-- =============================================================================
-- Kamailio Database Schema
-- Description: PostgreSQL schema for Kamailio SIP proxy
-- Version: Kamailio 6.0.4 (compatible with 6.0.x series)
-- Note: Schema compatibility preserved from 5.8.x
-- =============================================================================

-- Create kamailio schema
CREATE SCHEMA IF NOT EXISTS kamailio;

SET search_path TO kamailio;

-- =============================================================================
-- CORE TABLES
-- =============================================================================

-- Version tracking
CREATE TABLE version (
    table_name VARCHAR(64) PRIMARY KEY,
    table_version INT NOT NULL
);

-- =============================================================================
-- LOCATION & REGISTRATION
-- =============================================================================

-- Active SIP registrations (in-memory, persistent for restarts)
CREATE TABLE location (
    id SERIAL PRIMARY KEY,
    ruid VARCHAR(64) NOT NULL,
    username VARCHAR(64) DEFAULT '' NOT NULL,
    domain VARCHAR(190) DEFAULT '' NOT NULL,
    contact VARCHAR(512) DEFAULT '' NOT NULL,
    received VARCHAR(255) DEFAULT NULL,
    path VARCHAR(512) DEFAULT NULL,
    expires TIMESTAMP WITHOUT TIME ZONE DEFAULT '2030-05-28 21:32:15' NOT NULL,
    q REAL DEFAULT 1.0 NOT NULL,
    callid VARCHAR(255) DEFAULT 'Default-Call-ID' NOT NULL,
    cseq INT DEFAULT 1 NOT NULL,
    last_modified TIMESTAMP WITHOUT TIME ZONE DEFAULT '2000-01-01 00:00:01' NOT NULL,
    flags INT DEFAULT 0 NOT NULL,
    cflags INT DEFAULT 0 NOT NULL,
    user_agent VARCHAR(255) DEFAULT '' NOT NULL,
    socket VARCHAR(128) DEFAULT NULL,
    methods INT DEFAULT NULL,
    instance VARCHAR(255) DEFAULT NULL,
    reg_id INT DEFAULT 0 NOT NULL,
    server_id INT DEFAULT 0 NOT NULL,
    connection_id INT DEFAULT 0 NOT NULL,
    keepalive INT DEFAULT 0 NOT NULL,
    partition INT DEFAULT 0 NOT NULL,

    CONSTRAINT ruid_idx UNIQUE (ruid),
    CONSTRAINT account_contact_idx UNIQUE (username, domain, contact, callid)
);

CREATE INDEX location_username_idx ON location (username);
CREATE INDEX location_expires_idx ON location (expires);
CREATE INDEX location_connection_id_idx ON location (connection_id);

-- =============================================================================
-- SUBSCRIBER & AUTHENTICATION
-- =============================================================================

-- Subscriber credentials (read from voip.extensions via view)
-- Note: This is a view that joins with voip.extensions
-- See 03-views.sql for the actual view definition

CREATE TABLE subscriber (
    id SERIAL PRIMARY KEY,
    username VARCHAR(64) DEFAULT '' NOT NULL,
    domain VARCHAR(190) DEFAULT '' NOT NULL,
    password VARCHAR(64) DEFAULT '' NOT NULL,
    email_address VARCHAR(128) DEFAULT '' NOT NULL,
    ha1 VARCHAR(128) DEFAULT '' NOT NULL,
    ha1b VARCHAR(128) DEFAULT '' NOT NULL,
    rpid VARCHAR(128) DEFAULT NULL,

    CONSTRAINT account_idx UNIQUE (username, domain)
);

CREATE INDEX subscriber_username_idx ON subscriber (username, domain);

-- =============================================================================
-- DIALOG TRACKING
-- =============================================================================

-- Active dialogs (calls in progress)
CREATE TABLE dialog (
    id SERIAL PRIMARY KEY,
    hash_entry INT NOT NULL,
    hash_id INT NOT NULL,
    callid VARCHAR(255) NOT NULL,
    from_uri VARCHAR(255) NOT NULL,
    from_tag VARCHAR(128) NOT NULL,
    to_uri VARCHAR(255) NOT NULL,
    to_tag VARCHAR(128) NOT NULL,
    caller_cseq VARCHAR(20) NOT NULL,
    callee_cseq VARCHAR(20) NOT NULL,
    caller_route_set VARCHAR(512),
    callee_route_set VARCHAR(512),
    caller_contact VARCHAR(255) NOT NULL,
    callee_contact VARCHAR(255) NOT NULL,
    caller_sock VARCHAR(128) NOT NULL,
    callee_sock VARCHAR(128) NOT NULL,
    state INT NOT NULL,
    start_time INT NOT NULL,
    timeout INT DEFAULT 0 NOT NULL,
    sflags INT DEFAULT 0 NOT NULL,
    iflags INT DEFAULT 0 NOT NULL,
    toroute_name VARCHAR(32),
    req_uri VARCHAR(255) NOT NULL,
    xdata VARCHAR(512),

    CONSTRAINT dialog_hash_idx UNIQUE (hash_entry, hash_id)
);

CREATE INDEX dialog_callid_idx ON dialog (callid);

-- Dialog variables (custom per-dialog data)
CREATE TABLE dialog_vars (
    id SERIAL PRIMARY KEY,
    hash_entry INT NOT NULL,
    hash_id INT NOT NULL,
    dialog_key VARCHAR(128) NOT NULL,
    dialog_value VARCHAR(512) NOT NULL,

    CONSTRAINT dialogvars_hash_idx UNIQUE (hash_entry, hash_id, dialog_key)
);

-- =============================================================================
-- DISPATCHER (Load Balancing to FreeSWITCH)
-- =============================================================================

-- Dispatcher targets (FreeSWITCH media servers)
CREATE TABLE dispatcher (
    id SERIAL PRIMARY KEY,
    setid INT DEFAULT 0 NOT NULL,
    destination VARCHAR(192) DEFAULT '' NOT NULL,
    flags INT DEFAULT 0 NOT NULL,
    priority INT DEFAULT 0 NOT NULL,
    attrs VARCHAR(128) DEFAULT '' NOT NULL,
    description VARCHAR(64) DEFAULT '' NOT NULL,

    CONSTRAINT dispatcher_setid_dest_idx UNIQUE (setid, destination)
);

CREATE INDEX dispatcher_setid_idx ON dispatcher (setid);

INSERT INTO dispatcher (setid, destination, flags, priority, attrs, description) VALUES
(1, 'sip:172.16.91.101:5080', 0, 1, 'weight=50', 'FreeSWITCH Node 1'),
(1, 'sip:172.16.91.102:5080', 0, 1, 'weight=50', 'FreeSWITCH Node 2');

-- =============================================================================
-- ACCOUNTING (CDR generation - minimal, detailed CDR in voip.cdr)
-- =============================================================================

-- ACC transactions (real-time accounting)
CREATE TABLE acc (
    id SERIAL PRIMARY KEY,
    method VARCHAR(16) DEFAULT '' NOT NULL,
    from_tag VARCHAR(128) DEFAULT '' NOT NULL,
    to_tag VARCHAR(128) DEFAULT '' NOT NULL,
    callid VARCHAR(255) DEFAULT '' NOT NULL,
    sip_code VARCHAR(3) DEFAULT '' NOT NULL,
    sip_reason VARCHAR(128) DEFAULT '' NOT NULL,
    time TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    src_user VARCHAR(64) DEFAULT '' NOT NULL,
    src_domain VARCHAR(190) DEFAULT '' NOT NULL,
    src_ip VARCHAR(64) DEFAULT '' NOT NULL,
    dst_ouser VARCHAR(64) DEFAULT '' NOT NULL,
    dst_user VARCHAR(64) DEFAULT '' NOT NULL,
    dst_domain VARCHAR(190) DEFAULT '' NOT NULL,

    CONSTRAINT acc_callid_idx UNIQUE (callid, from_tag, to_tag)
);

CREATE INDEX acc_time_idx ON acc (time);

-- =============================================================================
-- PERMISSIONS & ACCESS CONTROL
-- =============================================================================

-- Trusted IP addresses (for gateway authentication)
CREATE TABLE trusted (
    id SERIAL PRIMARY KEY,
    src_ip VARCHAR(50) NOT NULL,
    proto VARCHAR(4) NOT NULL,
    from_pattern VARCHAR(64) DEFAULT NULL,
    ruri_pattern VARCHAR(64) DEFAULT NULL,
    tag VARCHAR(64) DEFAULT NULL,
    priority INT DEFAULT 0 NOT NULL
);

CREATE INDEX trusted_src_ip_idx ON trusted (src_ip);

-- Address permissions (IP-based ACL)
CREATE TABLE address (
    id SERIAL PRIMARY KEY,
    grp INT DEFAULT 0 NOT NULL,
    ip_addr VARCHAR(50) NOT NULL,
    mask INT DEFAULT 32 NOT NULL,
    port INT DEFAULT 0 NOT NULL,
    tag VARCHAR(64) DEFAULT NULL
);

CREATE INDEX address_grp_idx ON address (grp);

-- =============================================================================
-- DOMAIN & MULTI-TENANCY
-- =============================================================================

-- Domain list (for multi-tenant)
CREATE TABLE domain (
    id SERIAL PRIMARY KEY,
    domain VARCHAR(190) DEFAULT '' NOT NULL,
    did VARCHAR(64) DEFAULT NULL,
    last_modified TIMESTAMP WITHOUT TIME ZONE DEFAULT '2000-01-01 00:00:01' NOT NULL,

    CONSTRAINT domain_domain_idx UNIQUE (domain)
);

-- Domain attributes
CREATE TABLE domain_attrs (
    id SERIAL PRIMARY KEY,
    did VARCHAR(64) NOT NULL,
    name VARCHAR(32) NOT NULL,
    type INT NOT NULL,
    value VARCHAR(255) NOT NULL,
    last_modified TIMESTAMP WITHOUT TIME ZONE DEFAULT '2000-01-01 00:00:01' NOT NULL,

    CONSTRAINT domain_attrs_domain_attrs_idx UNIQUE (did, name, value)
);

-- =============================================================================
-- USRLOC (User Location Cache)
-- =============================================================================

-- User location (similar to location but for usrloc module)
CREATE TABLE usrloc (
    id SERIAL PRIMARY KEY,
    username VARCHAR(64) DEFAULT '' NOT NULL,
    domain VARCHAR(190) DEFAULT '' NOT NULL,
    contact VARCHAR(512) DEFAULT '' NOT NULL,
    expires TIMESTAMP WITHOUT TIME ZONE DEFAULT '2030-05-28 21:32:15' NOT NULL,
    q REAL DEFAULT 1.0 NOT NULL,
    callid VARCHAR(255) DEFAULT 'Default-Call-ID' NOT NULL,
    cseq INT DEFAULT 1 NOT NULL,
    last_modified TIMESTAMP WITHOUT TIME ZONE DEFAULT '2000-01-01 00:00:01' NOT NULL,
    flags INT DEFAULT 0 NOT NULL,
    cflags INT DEFAULT 0 NOT NULL,
    user_agent VARCHAR(255) DEFAULT '' NOT NULL,
    socket VARCHAR(128) DEFAULT NULL,
    methods INT DEFAULT NULL,
    ruid VARCHAR(64) DEFAULT '' NOT NULL,
    instance VARCHAR(255) DEFAULT NULL,
    reg_id INT DEFAULT 0 NOT NULL,
    server_id INT DEFAULT 0 NOT NULL,
    connection_id INT DEFAULT 0 NOT NULL,
    keepalive INT DEFAULT 0 NOT NULL,
    partition INT DEFAULT 0 NOT NULL
);

CREATE INDEX usrloc_account_contact_idx ON usrloc (username, domain, contact);
CREATE INDEX usrloc_expires_idx ON usrloc (expires);

-- =============================================================================
-- PRESENCE (SIP PUBLISH/SUBSCRIBE)
-- =============================================================================

-- Presentity (published presence info)
CREATE TABLE presentity (
    id SERIAL PRIMARY KEY,
    username VARCHAR(64) NOT NULL,
    domain VARCHAR(190) NOT NULL,
    event VARCHAR(64) NOT NULL,
    etag VARCHAR(128) NOT NULL,
    expires INT NOT NULL,
    received_time INT NOT NULL,
    body BYTEA NOT NULL,
    sender VARCHAR(255) NOT NULL,
    priority INT DEFAULT 0 NOT NULL,
    ruid VARCHAR(64),

    CONSTRAINT presentity_presentity_idx UNIQUE (username, domain, event, etag)
);

CREATE INDEX presentity_account_idx ON presentity (username, domain, event);
CREATE INDEX presentity_expires_idx ON presentity (expires);

-- Active watchers (subscriptions)
CREATE TABLE active_watchers (
    id SERIAL PRIMARY KEY,
    presentity_uri VARCHAR(255) NOT NULL,
    watcher_username VARCHAR(64) NOT NULL,
    watcher_domain VARCHAR(190) NOT NULL,
    to_user VARCHAR(64) NOT NULL,
    to_domain VARCHAR(190) NOT NULL,
    event VARCHAR(64) DEFAULT 'presence' NOT NULL,
    event_id VARCHAR(255),
    to_tag VARCHAR(128) NOT NULL,
    from_tag VARCHAR(128) NOT NULL,
    callid VARCHAR(255) NOT NULL,
    local_cseq INT NOT NULL,
    remote_cseq INT NOT NULL,
    contact VARCHAR(255) NOT NULL,
    record_route TEXT,
    expires INT NOT NULL,
    status INT DEFAULT 2 NOT NULL,
    reason VARCHAR(64),
    version INT DEFAULT 0 NOT NULL,
    socket_info VARCHAR(128) NOT NULL,
    local_contact VARCHAR(255) NOT NULL,
    from_user VARCHAR(64) NOT NULL,
    from_domain VARCHAR(190) NOT NULL,
    updated INT NOT NULL,
    updated_winfo INT NOT NULL,
    flags INT DEFAULT 0 NOT NULL,
    user_agent VARCHAR(255) DEFAULT '' NOT NULL,

    CONSTRAINT active_watchers_active_watchers_idx UNIQUE (callid, to_tag, from_tag)
);

CREATE INDEX active_watchers_presentity_uri_idx ON active_watchers (presentity_uri);
CREATE INDEX active_watchers_expires_idx ON active_watchers (expires);

-- =============================================================================
-- RLS (Resource List Server)
-- =============================================================================

-- RLS watchers
CREATE TABLE rls_watchers (
    id SERIAL PRIMARY KEY,
    presentity_uri VARCHAR(255) NOT NULL,
    to_user VARCHAR(64) NOT NULL,
    to_domain VARCHAR(190) NOT NULL,
    watcher_username VARCHAR(64) NOT NULL,
    watcher_domain VARCHAR(190) NOT NULL,
    event VARCHAR(64) DEFAULT 'presence' NOT NULL,
    event_id VARCHAR(255),
    to_tag VARCHAR(128) NOT NULL,
    from_tag VARCHAR(128) NOT NULL,
    callid VARCHAR(255) NOT NULL,
    local_cseq INT NOT NULL,
    remote_cseq INT NOT NULL,
    contact VARCHAR(255) NOT NULL,
    record_route TEXT,
    expires INT NOT NULL,
    status INT DEFAULT 1 NOT NULL,
    reason VARCHAR(64),
    version INT DEFAULT 0 NOT NULL,
    socket_info VARCHAR(128) NOT NULL,
    local_contact VARCHAR(255) NOT NULL,
    from_user VARCHAR(64) NOT NULL,
    from_domain VARCHAR(190) NOT NULL,
    updated INT NOT NULL,

    CONSTRAINT rls_watchers_rls_watcher_idx UNIQUE (callid, to_tag, from_tag)
);

CREATE INDEX rls_watchers_presentity_uri_idx ON rls_watchers (presentity_uri);
CREATE INDEX rls_watchers_expires_idx ON rls_watchers (expires);

-- =============================================================================
-- HTABLE (Shared Memory Hash Tables - persistent)
-- =============================================================================

-- Htable storage for persistent key-value data
CREATE TABLE htable (
    id SERIAL PRIMARY KEY,
    key_name VARCHAR(64) DEFAULT '' NOT NULL,
    key_type INT DEFAULT 0 NOT NULL,
    value_type INT DEFAULT 0 NOT NULL,
    key_value VARCHAR(128) DEFAULT '' NOT NULL,
    expires INT DEFAULT 0 NOT NULL,

    CONSTRAINT htable_key_name_key_value_idx UNIQUE (key_name, key_value)
);

-- =============================================================================
-- UAC (User Agent Client - for outbound calls)
-- =============================================================================

-- UAC registration (for trunks)
CREATE TABLE uacreg (
    id SERIAL PRIMARY KEY,
    l_uuid VARCHAR(64) DEFAULT '' NOT NULL,
    l_username VARCHAR(64) DEFAULT '' NOT NULL,
    l_domain VARCHAR(190) DEFAULT '' NOT NULL,
    r_username VARCHAR(64) DEFAULT '' NOT NULL,
    r_domain VARCHAR(190) DEFAULT '' NOT NULL,
    realm VARCHAR(190) DEFAULT '' NOT NULL,
    auth_username VARCHAR(64) DEFAULT '' NOT NULL,
    auth_password VARCHAR(64) DEFAULT '' NOT NULL,
    auth_ha1 VARCHAR(128) DEFAULT '' NOT NULL,
    auth_proxy VARCHAR(255) DEFAULT '' NOT NULL,
    expires INT DEFAULT 0 NOT NULL,
    flags INT DEFAULT 0 NOT NULL,
    reg_delay INT DEFAULT 0 NOT NULL,
    socket VARCHAR(128) DEFAULT NULL,

    CONSTRAINT uacreg_l_uuid_idx UNIQUE (l_uuid)
);

CREATE INDEX uacreg_l_username_idx ON uacreg (l_username);

-- =============================================================================
-- STATISTICS & MONITORING
-- =============================================================================

-- Statistics tracking
CREATE TABLE stats_mem (
    id SERIAL PRIMARY KEY,
    module VARCHAR(64) NOT NULL,
    time TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    used_size BIGINT NOT NULL,
    real_size BIGINT NOT NULL,
    max_size BIGINT NOT NULL,
    free_size BIGINT NOT NULL
);

CREATE INDEX stats_mem_time_idx ON stats_mem (time);

-- =============================================================================
-- VERSION TRACKING
-- =============================================================================

INSERT INTO version (table_name, table_version) VALUES
('location', 9),
('subscriber', 7),
('dialog', 7),
('dialog_vars', 1),
('dispatcher', 4),
('acc', 5),
('trusted', 6),
('address', 6),
('domain', 2),
('domain_attrs', 1),
('usrloc', 9),
('presentity', 5),
('active_watchers', 12),
('rls_watchers', 3),
('htable', 2),
('uacreg', 3),
('stats_mem', 1);

-- =============================================================================
-- GRANTS
-- =============================================================================

GRANT USAGE ON SCHEMA kamailio TO kamailio;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA kamailio TO kamailio;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA kamailio TO kamailio;

-- =============================================================================
-- NOTES
-- =============================================================================
--
-- This schema includes the core Kamailio tables needed for:
-- - SIP registration (location, subscriber)
-- - Active call tracking (dialog)
-- - Load balancing to FreeSWITCH (dispatcher)
-- - Basic accounting (acc - detailed CDR in voip.cdr)
-- - IP-based authentication (trusted, address)
-- - Presence/BLF (presentity, active_watchers, rls_watchers)
-- - Trunk registration (uacreg)
--
-- The subscriber table will be replaced with a VIEW joining voip.extensions
-- See 03-views.sql for view definitions
--
