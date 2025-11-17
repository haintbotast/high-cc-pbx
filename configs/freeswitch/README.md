# FreeSWITCH Configuration Files

## Directory Structure

These configuration files should be deployed to `/etc/freeswitch/` on each node:

```
/etc/freeswitch/
├── autoload_configs/
│   ├── switch.conf.xml          # Core settings
│   ├── modules.conf.xml         # Modules to load
│   ├── xml_curl.conf.xml        # mod_xml_curl for dynamic configs
│   ├── sofia.conf.xml           # SIP profiles
│   ├── cdr_pg_csv.conf.xml      # CDR to PostgreSQL
│   └── event_socket.conf.xml    # ESL interface
```

## Node-Specific Changes

### sofia.conf.xml
Update IP addresses for each node:

**Node 1 (172.16.91.101)**:
```xml
<param name="sip-ip" value="172.16.91.101"/>
<param name="rtp-ip" value="172.16.91.101"/>
```

**Node 2 (172.16.91.102)**:
```xml
<param name="sip-ip" value="172.16.91.102"/>
<param name="rtp-ip" value="172.16.91.102"/>
```

### xml_curl.conf.xml
Replace `API_KEY_HERE` with actual API key from voip-admin service.

### switch.conf.xml and cdr_pg_csv.conf.xml
Replace `PASSWORD` with actual PostgreSQL password for `freeswitch` user.

## Testing

After deployment:

1. Start FreeSWITCH:
   ```bash
   systemctl start freeswitch
   ```

2. Connect to fs_cli:
   ```bash
   fs_cli
   ```

3. Test Sofia profile:
   ```
   sofia status profile internal
   ```

4. Test database connection:
   ```
   pgsql select 1
   ```

5. Test XML_CURL:
   ```
   xml_locate directory domain example.com
   ```
