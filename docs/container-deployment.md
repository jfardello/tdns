# Container Deployment

TDNS publishes a distroless container based on SUSE BCI Nano. The image runs as
UID and GID `65532`, contains no shell or package manager, and supports
`linux/amd64` and `linux/arm64`.

The supported container topology is one TDNS instance on a trusted home, LAN,
or VPN network. Do not expose the DNS or management listener directly to the
Internet.

## Image

The release image is:

```text
git.kubewire.net/jfardello/tdns:<version>
```

Stable releases also update `latest`. Production deployments should pin a
release tag or, preferably, the published image digest:

```bash
docker pull git.kubewire.net/jfardello/tdns:v0.2.0
docker buildx imagetools inspect git.kubewire.net/jfardello/tdns:v0.2.0
```

The image exposes `8053/udp` for DNS and `8443/tcp` for the management HTTPS
server. `EXPOSE` is metadata only; published host addresses determine which
networks can reach the service.

## Filesystem Layout

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

## Bootstrap

Run the image once with the configuration directory writable:

```bash
docker run --rm \
  --user 65532:65532 \
  --cap-drop ALL \
  --security-opt no-new-privileges \
  --mount type=bind,src="$(pwd)/tdns-config",dst=/etc/tdns \
  git.kubewire.net/jfardello/tdns:<version> \
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

After bootstrap, do not mount `/etc/tdns` read-write into the serving container.

## Run With Docker

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
  git.kubewire.net/jfardello/tdns:<version> \
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

## SQLite Bootstrap And Persistence

`tdns serve` creates the configured SQLite database and applies all schema
migrations during startup. Do not create an empty `tdns.sqlite` file manually.
The mounted data directory only needs to exist and be writable by UID `65532`.

TDNS enables SQLite WAL mode. Mount the complete `/var/lib/tdns` directory, not
only `tdns.sqlite`, because `tdns.sqlite-wal` and `tdns.sqlite-shm` are created
beside the database. The database contains configuration overrides, DNS query
logs, aliases, and tagger state and must be treated as sensitive.
The serving process applies umask `0077`, so newly created runtime files are
restricted to the container identity.

Use local storage with reliable file locking. Network filesystems that do not
provide SQLite-compatible locking and durability are unsupported.

## Backup And Restore

Until TDNS provides an online SQLite backup command, take a cold backup:

```bash
docker stop tdns
sudo tar --numeric-owner -C ./tdns-data -czf tdns-data-backup.tar.gz .
docker start tdns
```

Protect and expire backups as sensitive DNS history. To restore:

1. Stop and remove the TDNS container.
2. Move the current data directory aside.
3. Restore the complete archived directory, including any WAL files.
4. Restore ownership to `65532:65532` and directory mode `0750`.
5. Start the same TDNS version and verify DNS and management API operation.

## Upgrade And Rollback

Before an upgrade:

1. Read the release notes.
2. Take a cold data backup.
3. Pull the new immutable release tag or digest.
4. Recreate the container with the same mounts and listener mappings.
5. Verify the reported version, DNS resolution, HTTPS API, and retained state.

Database migrations run automatically. Rolling back the binary after a migration
is not assumed to be safe. Restore the pre-upgrade data backup when returning to
an older image.

## Verification And Troubleshooting

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

Run the DNS check from both an explicitly allowed client and a client outside
the allowlist. The allowed query must receive a response and the denied query
must time out:

```bash
./tools/check-dns-exposure.sh allowed 192.168.1.53 53
./tools/check-dns-exposure.sh denied 192.168.1.53 53
```

The production image has no shell, `curl`, `ls`, or package manager. Diagnose it
through container logs, runtime inspection, external DNS/HTTPS probes, and
read-only host inspection of the mounted directories. Do not add troubleshooting
tools to the production image.

Swagger remains disabled by default. If enabled temporarily, restrict the
management listener to a trusted network and disable Swagger after use. Keep
`server.pprof_addr` empty until the separate trusted diagnostics listener
tracked by issue `#88` is implemented.

## Publishing

Tag pushes matching `v*` run the release workflow and publish one OCI manifest
for amd64 and arm64. The Gitea repository must define:

- `GITEA_TOKEN` for creating the release
- `REGISTRY_USERNAME` for registry login
- `REGISTRY_TOKEN` with permission to publish the container package

GoReleaser creates architecture-specific images and the shared version manifest.
Prereleases do not update `latest`.
