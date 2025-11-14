-- =============================================================================
-- lsyncd Configuration for Node 1
-- Description: Bidirectional recording synchronization
-- Syncs: /var/lib/freeswitch/recordings/ to Node 2
-- =============================================================================

settings {
    logfile = "/var/log/lsyncd/lsyncd.log",
    statusFile = "/var/log/lsyncd/lsyncd.status",
    statusInterval = 20,
    nodaemon = false,
    insist = true,
    inotifyMode = "CloseWrite",
    maxDelays = 5,
    maxProcesses = 4
}

-- Sync recordings to Node 2 via rsync over SSH
sync {
    default.rsync,
    source = "/var/lib/freeswitch/recordings/",
    target = "root@192.168.1.102:/var/lib/freeswitch/recordings/",

    delay = 5,  -- Delay in seconds before syncing

    rsync = {
        binary = "/usr/bin/rsync",
        archive = true,
        compress = true,
        verbose = false,

        -- Rsync options
        _extra = {
            "--update",              -- Skip files that are newer on receiver
            "--timeout=60",          -- I/O timeout
            "--contimeout=60",       -- Connection timeout
            "--bwlimit=50000",       -- Limit bandwidth to 50 MB/s
            "--exclude=*.tmp",       -- Exclude temporary files
            "--exclude=*.partial"    -- Exclude partial files
        },

        -- SSH options
        rsh = "/usr/bin/ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 -i /root/.ssh/lsyncd_rsa"
    },

    -- Conflict resolution: keep newer files
    onAttrib = false,  -- Don't sync attribute changes
}

-- =============================================================================
-- SETUP INSTRUCTIONS
-- =============================================================================
--
-- 1. Install lsyncd on both nodes:
--    apt-get install lsyncd rsync
--
-- 2. Create SSH key for passwordless authentication:
--    ssh-keygen -t rsa -b 4096 -f /root/.ssh/lsyncd_rsa -N ""
--
-- 3. Copy public key to Node 2:
--    ssh-copy-id -i /root/.ssh/lsyncd_rsa.pub root@192.168.1.102
--
-- 4. Test SSH connection:
--    ssh -i /root/.ssh/lsyncd_rsa root@192.168.1.102 "echo OK"
--
-- 5. Create log directory:
--    mkdir -p /var/log/lsyncd
--
-- 6. Create recordings directory:
--    mkdir -p /var/lib/freeswitch/recordings
--    chown freeswitch:freeswitch /var/lib/freeswitch/recordings
--
-- 7. Copy this file to /etc/lsyncd/lsyncd.conf.lua
--
-- 8. Enable and start lsyncd:
--    systemctl enable lsyncd
--    systemctl start lsyncd
--
-- 9. Monitor sync status:
--    tail -f /var/log/lsyncd/lsyncd.log
--    cat /var/log/lsyncd/lsyncd.status
--
