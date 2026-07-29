# Native And Systemd Deployment

The supported native topology is one TDNS instance on a trusted home, LAN, or
VPN network. Run TDNS under a dedicated service account and expose only the
listeners required by trusted clients.

## Install The Binary And Account

Install a release binary and create a non-login service account:

```bash
sudo install -o root -g root -m 0755 ./tdns /usr/local/bin/tdns
sudo useradd --system --home-dir /var/lib/tdns --shell /usr/sbin/nologin tdns
sudo install -d -o root -g tdns -m 0750 /etc/tdns
sudo install -d -o tdns -g tdns -m 0750 /var/lib/tdns
```

Use the equivalent account-management command when `useradd` is not available.

## Bootstrap Configuration

Generate the configuration, certificate, private key, and service unit:

```bash
sudo tdns config \
  --output-dir /etc/tdns \
  --basepath /etc/tdns \
  --data-path /var/lib/tdns \
  --listendns 127.0.0.1:53 \
  --listenapi 127.0.0.1:8443 \
  --hosts localhost \
  --hosts 127.0.0.1
```

For LAN DNS service, replace the DNS listener with the host's specific trusted
LAN address, add every DNS client network to
`dns_access.allowed_client_cidrs`, and include every management hostname or IP
in `--hosts`. Avoid wildcard listeners. Private and link-local networks are
denied unless explicitly configured.

The generated YAML contains an active signing key, its identifier, and a
bootstrap client token. Restrict the configuration and API private key to root
and the service group:

```bash
sudo chown root:tdns /etc/tdns/tdns.yaml /etc/tdns/tdns_key.pem
sudo chmod 0640 /etc/tdns/tdns.yaml /etc/tdns/tdns_key.pem
sudo chown root:root /etc/tdns/tdns_cert.pem
sudo chmod 0644 /etc/tdns/tdns_cert.pem
sudo chown root:tdns /etc/tdns/hostsfile_list
sudo chmod 0640 /etc/tdns/hostsfile_list
```

Adjust the filenames when a different `--basename` is used. Existing
installations that copied signing keys or tokens from repository samples must
rotate them; removing a value from the current repository does not revoke old
credentials.

For production, move the generated `auth.active_key.value` into a separate
base64 key file owned by `root:tdns` with mode `0640`, set
`auth.active_key.file` to that path, and clear the inline value. A stricter
`0400` file owned by the service account is also supported. Environment loading
through `TDNS_AUTH_ACTIVE_KEY` has higher precedence when systemd credentials
or another secret injector supplies the value.

## Install The Service

Review and verify the generated unit:

```bash
sudo systemd-analyze verify /etc/tdns/tdns.service
sudo install -o root -g root -m 0644 \
  /etc/tdns/tdns.service /etc/systemd/system/tdns.service
sudo systemctl daemon-reload
sudo systemctl enable --now tdns
```

The generated service:

- runs as `tdns:tdns`
- uses `UMask=0077`
- grants only `CAP_NET_BIND_SERVICE` for native port 53
- gives write access only to `/var/lib/tdns`
- makes `/etc/tdns` read-only to the service
- enables systemd process, device, filesystem, kernel, and namespace isolation

If TDNS listens only on ports greater than 1023, remove both
`AmbientCapabilities` and `CapabilityBoundingSet` from the installed unit.

Inspect the effective sandbox:

```bash
systemd-analyze security tdns.service
systemctl status tdns
journalctl -u tdns
```

## Listener And Firewall Policy

Bind DNS to loopback or a specific trusted LAN/VPN address. Bind management
HTTPS to loopback unless remote administration is required. Keep
`server.pprof_addr` empty and `server.swagger_enabled` false in normal
production operation.

Enforce the same boundary in the host or network firewall:

- allow UDP DNS port 53 only from explicitly trusted client CIDRs
- allow management TCP port 8443 only from administrator addresses
- deny unsolicited Internet traffic to both listeners
- keep firewall rules aligned with `dns_access.allowed_client_cidrs`
- do not treat firewall rules as a replacement for the application DNS ACL

TDNS does not support reverse-proxy deployments and does not use forwarded
headers as a security boundary.

From an allowed client, verify that DNS succeeds. From a host outside every
configured prefix, verify that the query times out:

```bash
./tools/check-dns-exposure.sh allowed 192.168.1.53 53
./tools/check-dns-exposure.sh denied 192.168.1.53 53
```

## SQLite And Runtime Data

TDNS creates `/var/lib/tdns/tdns.sqlite` and applies migrations at startup. The
directory must remain owned by `tdns:tdns`. SQLite WAL and shared-memory files
are created beside the database, so back up and restore the complete directory.

Until an online backup command exists, stop the service for a consistent backup:

```bash
sudo systemctl stop tdns
sudo tar --numeric-owner -C /var/lib/tdns -czf tdns-data-backup.tar.gz .
sudo systemctl start tdns
```

Protect backups as sensitive DNS history and apply the configured retention and
disposal policy.

## Upgrade

Before replacing the binary:

1. Read the release notes and take a cold data backup.
2. Verify the downloaded release checksum.
3. Install the new binary as `root:root` with mode `0755`.
4. Run `systemctl restart tdns`.
5. Verify `tdns --version`, service logs, DNS resolution, HTTPS management, and
   persisted state.

Database migrations are automatic. Restore both the old binary and the
pre-upgrade data backup for a rollback.
