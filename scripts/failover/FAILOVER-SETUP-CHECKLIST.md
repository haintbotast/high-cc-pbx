# PostgreSQL Failover Setup Checklist

**Mục đích:** Verify prerequisites cho pg_rewind fast path
**Phiên bản:** 1.0
**Ngày:** 2025-11-21

---

## Tổng Quan

Checklist này đảm bảo tất cả prerequisites cho pg_rewind được cấu hình đúng trên cả 2 nodes. pg_rewind yêu cầu:

1. ✅ **wal_log_hints = on** trong postgresql.conf (hoặc data checksums enabled)
2. ✅ **Passwordless PostgreSQL connection** từ standby → master (.pgpass)
3. ✅ **Passwordless SSH** giữa 2 nodes (optional nhưng recommended)
4. ✅ **Replication slots** configured trên master
5. ✅ **Network connectivity** giữa 2 nodes

---

## PART 1: Verify PostgreSQL Configuration

### 1.1 Check wal_log_hints (CẢ 2 NODES)

```bash
# Trên cả Node 1 và Node 2
sudo -u postgres psql -c "SHOW wal_log_hints;"
```

**Expected output:**
```
 wal_log_hints
---------------
 on
(1 row)
```

❌ **Nếu OFF:** Edit `/etc/postgresql/18/main/postgresql.conf`:
```bash
sudo sed -i 's/^#wal_log_hints = off/wal_log_hints = on/' /etc/postgresql/18/main/postgresql.conf
sudo systemctl restart postgresql
```

---

### 1.2 Check Replication Slots (CHỈ TRÊN MASTER)

```bash
# Trên node đang là MASTER
sudo -u postgres psql -c "SELECT slot_name, slot_type, active FROM pg_replication_slots;"
```

**Expected output:**
```
    slot_name     | slot_type | active
------------------+-----------+--------
 standby_slot_101 | physical  | t
 standby_slot_102 | physical  | t
(2 rows)
```

❌ **Nếu thiếu slots:** Tạo lại slots:
```bash
# Trên master
sudo -u postgres psql <<EOF
SELECT pg_create_physical_replication_slot('standby_slot_101');
SELECT pg_create_physical_replication_slot('standby_slot_102');
EOF
```

---

### 1.3 Check pg_hba.conf (CẢ 2 NODES)

```bash
# Trên cả 2 nodes
sudo grep replicator /etc/postgresql/18/main/pg_hba.conf
```

**Expected lines:**
```
host    replication     replicator      172.16.91.101/32        scram-sha-256
host    replication     replicator      172.16.91.102/32        scram-sha-256
host    all             replicator      172.16.91.101/32        scram-sha-256
host    all             replicator      172.16.91.102/32        scram-sha-256
```

❌ **Nếu thiếu:** Add các dòng trên và reload:
```bash
sudo systemctl reload postgresql
```

---

## PART 2: Setup .pgpass File

### 2.1 Create .pgpass on Node 1

```bash
# SSH to Node 1 (172.16.91.101)
ssh root@172.16.91.101

# Create .pgpass file
sudo -u postgres bash -c 'cat > /var/lib/postgresql/.pgpass <<EOF
172.16.91.102:5432:*:replicator:YOUR_REPL_PASSWORD_HERE
172.16.91.102:5432:*:postgres:YOUR_REPL_PASSWORD_HERE
172.16.91.102:5432:replication:replicator:YOUR_REPL_PASSWORD_HERE
EOF'

# Set correct permissions (CRITICAL!)
sudo -u postgres chmod 0600 /var/lib/postgresql/.pgpass
sudo chown postgres:postgres /var/lib/postgresql/.pgpass
```

**Replace placeholder password:**
```bash
# Get actual replicator password
REPL_PASS=$(grep replicator_password /root/.voip_credentials | cut -d'=' -f2)

# Update .pgpass with actual password
sudo sed -i "s/YOUR_REPL_PASSWORD_HERE/$REPL_PASS/g" /var/lib/postgresql/.pgpass
```

---

### 2.2 Create .pgpass on Node 2

```bash
# SSH to Node 2 (172.16.91.102)
ssh root@172.16.91.102

# Create .pgpass file
sudo -u postgres bash -c 'cat > /var/lib/postgresql/.pgpass <<EOF
172.16.91.101:5432:*:replicator:YOUR_REPL_PASSWORD_HERE
172.16.91.101:5432:*:postgres:YOUR_REPL_PASSWORD_HERE
172.16.91.101:5432:replication:replicator:YOUR_REPL_PASSWORD_HERE
EOF'

# Set correct permissions
sudo -u postgres chmod 0600 /var/lib/postgresql/.pgpass
sudo chown postgres:postgres /var/lib/postgresql/.pgpass

# Replace password
REPL_PASS=$(grep replicator_password /root/.voip_credentials | cut -d'=' -f2)
sudo sed -i "s/YOUR_REPL_PASSWORD_HERE/$REPL_PASS/g" /var/lib/postgresql/.pgpass
```

---

### 2.3 Verify .pgpass Permissions

```bash
# Trên CẢ 2 nodes
ls -la /var/lib/postgresql/.pgpass
```

**Expected output:**
```
-rw------- 1 postgres postgres 256 Nov 21 10:30 /var/lib/postgresql/.pgpass
```

✅ **Must be:**
- Owner: `postgres:postgres`
- Permissions: `600` (rw-------)
- ❌ **Nếu permissions > 0600**, PostgreSQL sẽ IGNORE file này!

---

### 2.4 Test Passwordless Connection

**Từ Node 1 → Node 2:**
```bash
# SSH to Node 1
ssh root@172.16.91.101

# Test connection as postgres user (should NOT prompt for password)
sudo -u postgres psql -h 172.16.91.102 -p 5432 -U replicator -d postgres -c "SELECT 1;"
```

**Expected output:**
```
 ?column?
----------
        1
(1 row)
```

❌ **Nếu vẫn hỏi password:** Check .pgpass permissions và format.

**Từ Node 2 → Node 1:**
```bash
# SSH to Node 2
ssh root@172.16.91.102

# Test connection
sudo -u postgres psql -h 172.16.91.101 -p 5432 -U replicator -d postgres -c "SELECT 1;"
```

---

## PART 3: Setup SSH Passwordless Authentication (Optional nhưng Recommended)

### 3.1 Generate SSH Keys (CẢ 2 NODES)

**Trên Node 1:**
```bash
ssh root@172.16.91.101

# Generate SSH key for postgres user (if not exists)
sudo -u postgres bash -c '
if [ ! -f ~/.ssh/id_rsa ]; then
    ssh-keygen -t rsa -b 4096 -N "" -f ~/.ssh/id_rsa
    echo "SSH key generated"
else
    echo "SSH key already exists"
fi
'
```

**Trên Node 2:**
```bash
ssh root@172.16.91.102

# Generate SSH key
sudo -u postgres bash -c '
if [ ! -f ~/.ssh/id_rsa ]; then
    ssh-keygen -t rsa -b 4096 -N "" -f ~/.ssh/id_rsa
    echo "SSH key generated"
else
    echo "SSH key already exists"
fi
'
```

---

### 3.2 Exchange SSH Keys

**Copy Node 1 key → Node 2:**
```bash
# Trên Node 1
sudo -u postgres ssh-copy-id -i /var/lib/postgresql/.ssh/id_rsa.pub postgres@172.16.91.102
```

**Copy Node 2 key → Node 1:**
```bash
# Trên Node 2
sudo -u postgres ssh-copy-id -i /var/lib/postgresql/.ssh/id_rsa.pub postgres@172.16.91.101
```

---

### 3.3 Test SSH Passwordless Login

**Từ Node 1 → Node 2:**
```bash
# Trên Node 1
sudo -u postgres ssh postgres@172.16.91.102 "hostname"
```

**Expected:** Hiển thị hostname của Node 2 KHÔNG hỏi password.

**Từ Node 2 → Node 1:**
```bash
# Trên Node 2
sudo -u postgres ssh postgres@172.16.91.101 "hostname"
```

**Expected:** Hiển thị hostname của Node 1 KHÔNG hỏi password.

---

## PART 4: Test pg_rewind Prerequisites

### 4.1 Test pg_rewind Dry Run (TRÊN STANDBY)

⚠️ **QUAN TRỌNG:** Chỉ test trên node đang là STANDBY và đã STOP PostgreSQL.

**Xác định node nào là standby:**
```bash
# Trên mỗi node, check role
sudo -u postgres psql -c "SELECT pg_is_in_recovery();"
```
- `t` = STANDBY
- `f` = MASTER

**Test pg_rewind (trên STANDBY sau khi stop PostgreSQL):**
```bash
# Giả sử Node 2 là standby và Node 1 là master
ssh root@172.16.91.102

# Stop PostgreSQL
sudo systemctl stop postgresql

# Test pg_rewind với --dry-run
sudo -u postgres /usr/lib/postgresql/18/bin/pg_rewind \
    --target-pgdata=/var/lib/postgresql/18/main \
    --source-server="host=172.16.91.101 port=5432 user=replicator dbname=postgres" \
    --dry-run \
    --progress

# Start lại PostgreSQL
sudo systemctl start postgresql
```

**Expected output với --dry-run:**
```
pg_rewind: connected to server
pg_rewind: servers diverged at WAL location 0/XXXXXXXX on timeline 1
pg_rewind: rewinding from last common checkpoint at 0/YYYYYYYY on timeline 1
pg_rewind: reading source file list
pg_rewind: reading target file list
pg_rewind: Done!
```

✅ **Nếu thấy "Done!"** → pg_rewind sẽ work khi cần failover.

❌ **Nếu lỗi "could not connect"** → Check .pgpass hoặc pg_hba.conf.

❌ **Nếu lỗi "wal_log_hints is off"** → Enable wal_log_hints trong postgresql.conf.

---

## PART 5: Network Connectivity Tests

### 5.1 Test Port 5432 Reachability

**Từ Node 1 → Node 2:**
```bash
ssh root@172.16.91.101
nc -zv 172.16.91.102 5432
```

**Expected:** `Connection to 172.16.91.102 5432 port [tcp/postgresql] succeeded!`

**Từ Node 2 → Node 1:**
```bash
ssh root@172.16.91.102
nc -zv 172.16.91.101 5432
```

---

### 5.2 Test Ping

```bash
# Từ Node 1
ping -c 3 172.16.91.102

# Từ Node 2
ping -c 3 172.16.91.101
```

**Expected:** 0% packet loss.

---

## PART 6: Final Verification Summary

### ✅ Checklist Hoàn Chỉnh

Chạy script tự động verify tất cả prerequisites:

```bash
#!/bin/bash
# File: /tmp/verify_pg_rewind_prerequisites.sh

echo "========================================="
echo "pg_rewind Prerequisites Verification"
echo "========================================="
echo ""

# Get peer IP
CURRENT_IP=$(hostname -I | awk '{print $1}')
if [[ "$CURRENT_IP" == "172.16.91.101" ]]; then
    PEER_IP="172.16.91.102"
elif [[ "$CURRENT_IP" == "172.16.91.102" ]]; then
    PEER_IP="172.16.91.101"
else
    echo "❌ Unknown node IP: $CURRENT_IP"
    exit 1
fi

echo "Current node: $CURRENT_IP"
echo "Peer node: $PEER_IP"
echo ""

# Check 1: wal_log_hints
echo -n "1. Check wal_log_hints... "
WAL_HINTS=$(sudo -u postgres psql -qAt -c "SHOW wal_log_hints;")
if [[ "$WAL_HINTS" == "on" ]]; then
    echo "✅ ON"
else
    echo "❌ OFF (REQUIRED!)"
fi

# Check 2: .pgpass exists
echo -n "2. Check .pgpass exists... "
if [[ -f /var/lib/postgresql/.pgpass ]]; then
    echo "✅ EXISTS"
else
    echo "❌ MISSING"
fi

# Check 3: .pgpass permissions
echo -n "3. Check .pgpass permissions... "
PERMS=$(stat -c "%a" /var/lib/postgresql/.pgpass 2>/dev/null)
if [[ "$PERMS" == "600" ]]; then
    echo "✅ 600"
else
    echo "❌ $PERMS (must be 600)"
fi

# Check 4: PostgreSQL passwordless connection
echo -n "4. Test PostgreSQL connection to peer... "
if sudo -u postgres psql -h "$PEER_IP" -U replicator -d postgres -c "SELECT 1;" >/dev/null 2>&1; then
    echo "✅ OK"
else
    echo "❌ FAILED"
fi

# Check 5: SSH passwordless (optional)
echo -n "5. Test SSH to peer (optional)... "
if sudo -u postgres ssh -o BatchMode=yes -o ConnectTimeout=5 postgres@"$PEER_IP" "echo ok" >/dev/null 2>&1; then
    echo "✅ OK"
else
    echo "⚠ FAILED (not critical)"
fi

# Check 6: Network connectivity
echo -n "6. Test network connectivity (ping)... "
if ping -c 2 -W 2 "$PEER_IP" >/dev/null 2>&1; then
    echo "✅ OK"
else
    echo "❌ FAILED"
fi

# Check 7: Port 5432 reachable
echo -n "7. Test PostgreSQL port 5432... "
if nc -zw2 "$PEER_IP" 5432 >/dev/null 2>&1; then
    echo "✅ OK"
else
    echo "❌ FAILED"
fi

echo ""
echo "========================================="
echo "Verification complete!"
echo "========================================="
```

**Chạy script trên CẢ 2 nodes:**
```bash
# Copy script
cat > /tmp/verify_pg_rewind_prerequisites.sh <<'EOF'
[paste script above]
EOF

chmod +x /tmp/verify_pg_rewind_prerequisites.sh

# Run on both nodes
ssh root@172.16.91.101 "bash /tmp/verify_pg_rewind_prerequisites.sh"
ssh root@172.16.91.102 "bash /tmp/verify_pg_rewind_prerequisites.sh"
```

---

## Troubleshooting

### Lỗi: .pgpass permissions too open

```
WARNING: password file "/var/lib/postgresql/.pgpass" has group or world access
```

**Fix:**
```bash
sudo -u postgres chmod 0600 /var/lib/postgresql/.pgpass
```

---

### Lỗi: pg_rewind "could not connect to server"

**Nguyên nhân:** .pgpass format sai hoặc password sai.

**Fix:**
```bash
# Verify .pgpass content
sudo -u postgres cat /var/lib/postgresql/.pgpass

# Format must be: hostname:port:database:username:password
# Example: 172.16.91.102:5432:*:replicator:MySecretPassword123

# Test connection manually with password
PGPASSWORD='actual_password' sudo -u postgres psql -h 172.16.91.102 -U replicator -d postgres -c "SELECT 1;"
```

---

### Lỗi: pg_rewind "wal_log_hints is off"

**Fix:**
```bash
# Edit postgresql.conf
sudo sed -i 's/^#wal_log_hints = off/wal_log_hints = on/' /etc/postgresql/18/main/postgresql.conf

# Restart PostgreSQL
sudo systemctl restart postgresql
```

---

### Lỗi: SSH asks for password

**Nguyên nhân:** SSH keys chưa được exchange hoặc permissions sai.

**Fix:**
```bash
# Check SSH directory permissions
sudo -u postgres ls -la /var/lib/postgresql/.ssh/

# Should be:
# drwx------ (700) for .ssh/
# -rw------- (600) for id_rsa
# -rw-r--r-- (644) for id_rsa.pub
# -rw------- (600) for authorized_keys

# Fix permissions
sudo -u postgres chmod 700 /var/lib/postgresql/.ssh
sudo -u postgres chmod 600 /var/lib/postgresql/.ssh/id_rsa
sudo -u postgres chmod 644 /var/lib/postgresql/.ssh/id_rsa.pub
sudo -u postgres chmod 600 /var/lib/postgresql/.ssh/authorized_keys
```

---

## Summary

Khi tất cả checks PASS:
- ✅ wal_log_hints = on
- ✅ .pgpass exists với permissions 600
- ✅ Passwordless PostgreSQL connection works
- ✅ Network connectivity OK
- ✅ Port 5432 reachable

→ **pg_rewind sẽ work!** Recovery time: **< 1 minute** thay vì 10-30 minutes.

---

**Version:** 1.0
**Last Updated:** 2025-11-21
**Author:** PostgreSQL HA Expert
