# Deployment and configuration

## Container Deployment

TDNS publishes a distroless container based on SUSE BCI Nano. The image runs as
UID and GID `65532`, contains no shell or package manager, and supports
`linux/amd64` and `linux/arm64`.

The supported container topology is one TDNS instance on a trusted home, LAN,
or VPN network. Do not expose the DNS or management listener directly to the
Internet.

### Image

The release image is:

```text
ghcr.io/jfardello/tdns:<version>
```

Stable releases also update `latest`. Production deployments should pin a
release tag or, preferably, the published image digest:

```bash
docker pull ghcr.io/jfardello/tdns:v0.2.0
docker buildx imagetools inspect ghcr.io/jfardello/tdns:v0.2.0
```

The image exposes `8053/udp` for DNS and `8443/tcp` for the management HTTPS
server. `EXPOSE` is metadata only; published host addresses determine which
networks can reach the service.

### Filesystem Layout

TDNS uses two mounts:

| Container path | Access | Contents |
| --- | --- | --- |
| `/etc/tdns` | Read-only while serving | YAML configuration, API certificate and private key, bootstrap client token, and static response files |
| `/var/lib/tdns` | Read-write | SQLite database, WAL files, downloaded blocklist, and other mutable state |

Create both host directories for the image's non-root identity:

```bash
sudo install -d -m 0750 -o 65532 -g 65532 ./tdns-config
sudo install -d -m 0750 -o 65532 -g 65532 ./tdns-data
```

On SELinux hosts, apply an appropriate container file label or add the runtime's
documented relabel option to the bind mounts.

### Bootstrap

Run the image once with the configuration directory writable:

```bash
docker run --rm \
  --user 65532:65532 \
  --cap-drop ALL \
  --security-opt no-new-privileges \
  --mount type=bind,src="$(pwd)/tdns-config",dst=/etc/tdns \
  ghcr.io/jfardello/tdns:<version> \
  config \
  --output-dir /etc/tdns \
  --basepath /etc/tdns \
  --data-path /var/lib/tdns \
  --systemd-unit=false \
  --listendns :8053 \
  --listenapi :8443 \
  --hosts localhost \
  --hosts 127.0.0.1
```

Add every hostname or IP address used to reach the management API through
additional `--hosts` values. Bootstrap creates:

- `/etc/tdns/tdns.yaml` with mode `0600`
- a self-signed API certificate
- an API private key with mode `0600`
- a random active signing key and key identifier stored in the restricted YAML
  file
- a 30-day bootstrap administration token in `client.token`
- a sample static hosts file

Review `tdns.yaml` before starting the server. The signing key and client token
are secrets. For production, move `auth.active_key.value` to a read-only secret
file mounted inside the container and set `auth.active_key.file` to that path,
or inject `TDNS_AUTH_ACTIVE_KEY` through the container runtime. The environment
source takes precedence over the file, and the file takes precedence over the
inline value.
The generated DNS ACL is loopback-only. Before publishing DNS outside the
container, add the actual client or trusted network prefixes to
`dns_access.allowed_client_cidrs`. Container bridge networks are not trusted
implicitly.

The sample blocklist uses a public GitHub repository and needs no credential.
For a private repository, inject `GITHUB_TOKEN` as a runtime-managed environment
secret rather than placing it in `tdns.yaml`. Permit outbound HTTPS only to the
GitHub API and supported GitHub content hosts described in the [configuration
reference](configuration.md#blacklist).

After bootstrap, do not mount `/etc/tdns` read-write into the serving container.

Bootstrap does not create a local browser password. After the serving container
has created and migrated SQLite, set the password through an interactive exec
session. The image accepts no plaintext-password argument or environment
variable:

```bash
docker exec -it tdns /usr/local/bin/tdns \
  -c /etc/tdns/tdns.yaml adm password set --username admin
```

Run the same command to rotate the password. Disable password login and revoke
password-authenticated sessions with:

```bash
docker exec -it tdns /usr/local/bin/tdns \
  -c /etc/tdns/tdns.yaml adm password disable
```

Both operations run as the container's UID `65532` and update the mounted
SQLite database. Browser-code login remains available independently for
recovery when the active signing key is accessible to the container command.

### Run With Docker

Replace the host listener addresses with addresses on the trusted network that
should receive DNS or management traffic:

```bash
docker run -d \
  --name tdns \
  --restart unless-stopped \
  --user 65532:65532 \
  --read-only \
  --tmpfs /tmp:rw,noexec,nosuid,nodev,size=16m,mode=1777 \
  --cap-drop ALL \
  --security-opt no-new-privileges \
  --mount type=bind,src="$(pwd)/tdns-config",dst=/etc/tdns,readonly \
  --mount type=bind,src="$(pwd)/tdns-data",dst=/var/lib/tdns \
  --publish 192.168.1.53:53:8053/udp \
  --publish 127.0.0.1:8443:8443/tcp \
  ghcr.io/jfardello/tdns:<version> \
  serve -c /etc/tdns/tdns.yaml
```

Do not use privileged mode or host networking. Mapping host port 53 to
unprivileged container port 8053 avoids `CAP_NET_BIND_SERVICE`. Keep the
management port on loopback unless remote administration is required from an
explicitly trusted network. Host-side port mapping and firewall rules do not
replace `dns_access.allowed_client_cidrs`.

The repository's [`compose.yaml`](../compose.yaml) provides the same restrictive
defaults and binds both listeners to loopback. Change only the host-side
addresses required by the deployment:

```bash
docker compose up -d
```

Podman can run the same image and equivalent options. Keep the numeric ownership
of both bind-mounted directories aligned with the UID visible inside the
container.

### SQLite Bootstrap And Persistence

`tdns serve` creates the configured SQLite database and applies all schema
migrations during startup. Do not create an empty `tdns.sqlite` file manually.
The mounted data directory only needs to exist and be writable by UID `65532`.

TDNS enables SQLite WAL mode. Mount the complete `/var/lib/tdns` directory, not
only `tdns.sqlite`, because `tdns.sqlite-wal` and `tdns.sqlite-shm` are created
beside the database. The database contains configuration overrides, DNS query
logs, aliases, tagger state, hashed browser-session and CSRF credentials,
authorization context, and consumed browser-code identifiers. It and its
backups must be treated as authentication data.
The serving process applies umask `0077`, so newly created runtime files are
restricted to the container identity.

Use local storage with reliable file locking. Network filesystems that do not
provide SQLite-compatible locking and durability are unsupported.

### Backup And Restore

Until TDNS provides an online SQLite backup command, take a cold backup:

```bash
docker stop tdns
sudo tar --numeric-owner -C ./tdns-data -czf tdns-data-backup.tar.gz .
docker start tdns
```

Protect and expire backups as sensitive DNS history and authentication data.
The snapshot includes the local password hash, browser sessions, CSRF state,
and consumed-code history. To restore:

1. Stop and remove the TDNS container.
2. Move the current data directory aside.
3. Restore the complete archived directory, including any WAL files.
4. Restore ownership to `65532:65532` and directory mode `0750`.
5. Start the same TDNS version and verify DNS and management API operation.

An older snapshot can restore a previous password hash and sessions whose
absolute expiration is still in the future. Rotate the password and bearer
credentials after an incident-related restore. Password rotation does not
revoke browser-code sessions.

## Upgrade And Rollback

Before an upgrade:

1. Read the release notes.
2. Take a cold data backup.
3. Pull the new immutable release tag or digest.
4. Recreate the container with the same mounts and listener mappings.
5. Verify the reported version, DNS resolution, HTTPS API, and retained state.

Database migrations run automatically. Password login remains disabled until a
credential exists, so an existing installation may continue with browser-code
login or explicitly provision the password after upgrade. Verify password and
browser-code login, remembered-session persistence, and logout after an
authentication-related upgrade. Rolling back the binary after a migration is
not assumed to be safe. Restore the pre-upgrade data backup when returning to an
older image and account for its restored authentication state.

### Verification And Troubleshooting

Confirm the container identity and effective restrictions:

```bash
docker inspect tdns --format '{{.Config.User}} {{.HostConfig.ReadonlyRootfs}}'
docker inspect tdns --format '{{json .HostConfig.CapDrop}}'
docker logs tdns
```

Test the listeners from outside the container:

```bash
dig @127.0.0.1 example.com
curl --cacert ./tdns-config/tdns_cert.pem https://127.0.0.1:8443/
```

The production image has no shell, `curl`, `ls`, or package manager. Diagnose it
through container logs, runtime inspection, external DNS/HTTPS probes, and
read-only host inspection of the mounted directories. Do not add troubleshooting
tools to the production image.

Swagger remains disabled by default. If enabled temporarily, restrict the
management listener to a trusted network and disable Swagger after use. The
diagnostics listener defaults to `127.0.0.1:6060` inside the container and is
therefore reachable only from the same network namespace. For external
scraping, use a collector sidecar sharing that namespace or assign a stable
address on an isolated container network and bind diagnostics to that exact
address. Publishing the port alone does not make a loopback-bound listener
reachable. Keep `diagnostics.pprof_enabled` false during normal operation.

## Publishing

Tag pushes matching `v*` run the GitHub Actions release workflow and publish
one OCI manifest for amd64 and arm64. The workflow uses the repository's
automatic `GITHUB_TOKEN` with `contents: write` and `packages: write`; no
additional release or registry secret is required. Repository or organization
policy must allow those token permissions.

GoReleaser creates architecture-specific images and the shared version
manifest. Prereleases do not update `latest`. GHCR packages are private on first
publication; change the package visibility to public in GitHub if deployments
must pull TDNS without authenticating.


## Native And Systemd Deployment

The supported native topology is one TDNS instance on a trusted home, LAN, or
VPN network. Run TDNS under a dedicated service account and expose only the
listeners required by trusted clients.

### Install The Binary And Account

Install a release binary and create a non-login service account:

```bash
sudo install -o root -g root -m 0755 ./tdns /usr/local/bin/tdns
sudo useradd --system --home-dir /var/lib/tdns --shell /usr/sbin/nologin tdns
sudo install -d -o root -g tdns -m 0750 /etc/tdns
sudo install -d -o tdns -g tdns -m 0750 /var/lib/tdns
```

Use the equivalent account-management command when `useradd` is not available.

### Bootstrap Configuration

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

### Install The Service

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

### Local Administrator Password

Bootstrap does not create a browser password. Password login becomes available
only after the service account writes a valid credential to SQLite. Run the
interactive command from a terminal; never place the password in shell
arguments, an environment variable, or an input redirect:

```bash
sudo -u tdns /usr/local/bin/tdns \
  -c /etc/tdns/tdns.yaml adm password set --username admin
```

Run the same command to rotate a forgotten or exposed password. Rotation
revokes all password-authenticated browser sessions. Disable password login
while retaining browser-code recovery with:

```bash
sudo -u tdns /usr/local/bin/tdns \
  -c /etc/tdns/tdns.yaml adm password disable
```

The commands open the configured SQLite database directly and therefore must
run as the same dedicated identity as the service. Browser-code login remains
available as long as the operator can read the active signing key and run
`tdns adm browser-code`.

### SQLite And Runtime Data

TDNS creates `/var/lib/tdns/tdns.sqlite` and applies migrations at startup. The
directory must remain owned by `tdns:tdns`. SQLite WAL and shared-memory files
are created beside the database, so back up and restore the complete directory.

Until an online backup command exists, stop the service for a consistent backup:

```bash
sudo systemctl stop tdns
sudo tar --numeric-owner -C /var/lib/tdns -czf tdns-data-backup.tar.gz .
sudo systemctl start tdns
```

Protect backups as sensitive DNS history and authentication data and apply the
configured retention and disposal policy. A restore also restores the local
password hash, consumed-code records, and unexpired browser sessions from the
snapshot. After an incident-related restore, rotate the local password and
bearer credentials; note that password rotation does not revoke browser-code
sessions.

The sample blocklist uses a public GitHub repository and needs no credential.
For a private repository, provide `GITHUB_TOKEN` through a root-owned systemd
environment file; do not place it in `tdns.yaml`. Restrict outbound HTTPS to the
GitHub API and supported GitHub content hosts documented in the [configuration
reference](configuration.md#blacklist).

### Upgrade

Before replacing the binary:

1. Read the release notes and take a cold data backup.
2. Verify the downloaded release checksum.
3. Install the new binary as `root:root` with mode `0755`.
4. Run `systemctl restart tdns`.
5. Verify `tdns --version`, service logs, DNS resolution, HTTPS management, and
   persisted state.

Database migrations are automatic. An upgraded database does not enable
password login until `tdns adm password set` stores a credential. Verify both
password and browser-code login after an authentication-related upgrade.
Restore both the old binary and the pre-upgrade data backup for a rollback,
accounting for the authentication state contained in that snapshot.
