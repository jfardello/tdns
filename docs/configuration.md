# TDNS Configuration

TDNS uses one YAML document for the DNS server, management HTTPS server, and
`tdns adm` client. The repository's [`tdns.yaml`](../tdns.yaml) contains every
recognized YAML key and can be used as a complete example.

## Loading and Precedence

`tdns serve` searches for `tdns.yaml` in this order unless `-c` is provided:

1. `/etc/tdns/`
2. `$HOME/.config/tdns`
3. the current directory

Configuration values are resolved in this order, from highest to lowest
precedence:

1. Persisted SQLite overrides, for the limited fields listed below
2. CLI flags explicitly bound to configuration keys
3. Viper environment values
4. YAML values
5. Built-in fallbacks

Use `tdns serve -c /path/to/tdns.yaml` to select a specific file. Viper's
automatic environment support uses the `TDNS_` prefix, but this application
does not translate dots in nested keys to underscores. Prefer YAML or the
documented CLI flags for nested settings.

The fallback column below describes `tdns serve` when a key is omitted. The
complete example file sets explicit values for options where a useful local
sample differs from the zero-value fallback.

## Generated Configuration

`tdns config` generates a starting YAML file, API certificate and private key,
bootstrap client token, static hosts file, and, by default, a systemd unit.
Important deployment options are:

| Flag | Purpose |
| --- | --- |
| `--output-dir` | Host or container directory in which files are created. |
| `--basepath` | Runtime configuration path written into generated YAML and the systemd unit. |
| `--data-path` | Runtime directory written into `database.file` and mutable feature paths. |
| `--systemd-unit` | Generates `tdns.service`; set to `false` for container bootstrap. |
| `--listendns` | DNS listener written into `server.listen_addr`. |
| `--listenapi` | HTTPS listener written into `server.api_addr`. |
| `--hosts` | Certificate DNS names or IP subject alternative names. Repeat for every management address. |

The generated YAML and private key use mode `0600`. The YAML contains the
signing key and a 30-day bootstrap administration token, so it is a credential
file. Generated listeners default to loopback. Use an explicit trusted LAN or
VPN address when remote clients need access; do not replace it with a wildcard
unless equivalent network controls have been reviewed.

See [Native and Systemd Deployment](systemd-deployment.md) and
[Container Deployment](container-deployment.md) for ownership and mount
procedures.

## Top-Level Options

| Key | Fallback | Description |
| --- | --- | --- |
| `timeout` | `1000` | Overall DNS resolution timeout in milliseconds. |
| `upstream_timeout` | `300` | Timeout for each upstream DNS exchange in milliseconds. |
| `loglevel` | `INFO` | Logging level, such as `DEBUG`, `INFO`, or `ERROR`. |
| `upstreams` | Cloudflare `1.1.1.1` over TLS | Ordered default resolver URLs. |
| `verify_tls` | `false` | Compatibility field currently accepted but not consulted by resolver construction. TLS verification behavior comes from the upstream URL and TLS client. |
| `enable_api` | `false` | Reserved compatibility field. The HTTPS API currently starts whenever the server starts, so changing this value has no effect. |

Upstreams use `protocol://address:port#tls-name`. Supported protocols are
`udp`, `tcp`, and `tls`. The optional `tls-name` is checked against the server
certificate for TLS upstreams.

## DNS Client Access And Load Limits

DNS access is denied unless the socket peer address is loopback or belongs to
an explicitly configured prefix. Private, link-local, container bridge, and VPN
networks are not trusted automatically.

| Key | Fallback | Description |
| --- | --- | --- |
| `dns_access.allowed_client_cidrs` | `[]` | Additional trusted IPv4 or IPv6 client prefixes. Loopback is always allowed. |
| `dns_access.client_queries_per_second` | `100` | Sustained token-bucket query rate for each normalized client IP. |
| `dns_access.client_burst` | `200` | Maximum per-client query burst. |
| `dns_access.global_responses_per_second` | `1000` | Sustained UDP response rate across all clients. |
| `dns_access.global_response_burst` | `2000` | Maximum global UDP response burst. |
| `dns_access.max_concurrent_upstreams` | `128` | Maximum requests concurrently using stub or default upstream resolvers. |
| `dns_access.max_tracked_clients` | `4096` | Maximum client limiter entries retained in memory. Old entries are evicted. |
| `dns_access.client_idle_timeout` | `10m` | Idle period before a client limiter entry is removed. Maximum `24h`. |

For example, a LAN deployment that serves only `192.168.50.0/24` and the VPN
prefix `fd00:50::/64` uses:

```yaml
dns_access:
  allowed_client_cidrs:
    - 192.168.50.0/24
    - fd00:50::/64
```

Malformed or duplicate prefixes, disabled limits, excessive values, and invalid
idle durations fail startup. ACL and per-client rate rejection occur before
tagging, cache access, DNS logging, or upstream resolution. Rejected UDP
requests are silently dropped. An authorized request rejected by upstream
saturation receives `SERVFAIL` only when global response budget remains.

## DNS Features

### Cache

| Key | Fallback | Description |
| --- | --- | --- |
| `cache.enabled` | `true` | Enables DNS cache reads and writes. Persistently overridable. |
| `cache.ttl` | `0` | Cache lifetime and maximum returned TTL in minutes. The generated example uses `5`. |
| `cache.excludes` | `[]` | Client selectors that bypass the cache. Persistently extensible through the API. |

Cache selectors accept `label:<name>`, `ip:<address>`, or `cidr:<network>`.
Bare IP addresses and CIDRs are also accepted and normalized.

### Blacklist

| Key | Fallback | Description |
| --- | --- | --- |
| `blacklist.enabled` | `false` | Enables blacklist filtering. |
| `blacklist.file` | empty | Local hosts-style file used as the active blocklist. Required when the middleware is configured. |
| `blacklist.external_file` | empty | Path inside the external Git repository copied into `blacklist.file`. |
| `blacklist.external_repo` | empty | Git repository used for scheduled blocklist refreshes. |
| `blacklist.external_repo_branch` | `master` | Branch read from the external repository. |
| `blacklist.external_pull_period` | empty | Cron expression for scheduled refreshes; empty disables scheduled pulls. |
| `blacklist.excludes` | `[]` | Configured domain or `label:<name>` exclusions. Persisted exclusions are merged with this list. |

Persisted blacklist hosts add entries to the configured blocklist. Persisted
exclusions add domain or label selectors to `blacklist.excludes`.

### Static Responses

| Key | Fallback | Description |
| --- | --- | --- |
| `static_response.enabled` | `false` | Enables static DNS responses. |
| `static_response.file` | empty | Hosts-style file containing configured static answers. Required when the middleware is configured. |
| `static_response.labels` | `[]` | Restricts static answers to clients carrying any listed tagger label. |

Persisted static hosts are merged over the configured file, so a persisted
entry wins when both sources contain the same domain.

### Zen Mode

| Key | Fallback | Description |
| --- | --- | --- |
| `zen_mode.enabled` | `false` | Compatibility field currently accepted but not consulted. A blocking period begins through the management API or CLI. |
| `zen_mode.file` | empty | Optional hosts-style source of blocked domains. |
| `zen_mode.domains` | `[]` | Domains blocked during zen mode. Persisted domains are added to this set. |
| `zen_mode.excludes` | `[]` | Domain or `label:<name>` exclusions. Persisted exclusions are added to this set. |
| `zen_mode.labels` | `[]` | Restricts zen-mode blocking to clients carrying any listed tagger label. |
| `zen_mode.time` | `20` | Duration of a zen-mode period in minutes. |

### Stub Resolver

| Key | Fallback | Description |
| --- | --- | --- |
| `stub_resolver.enabled` | `false` | Enables domain-specific resolver routing. |
| `stub_resolver.stubs` | `[]` | Entries in `domain,upstream[,upstream...]` format. |

Stub resolver entries can be replaced at runtime, but they are not persisted
as SQLite configuration overrides.

### Status

| Key | Fallback | Description |
| --- | --- | --- |
| `status.enabled` | `true` | Enables `_status.tdns.local` status responses. |
| `status.expose_stats` | `false` | Includes cache statistics in status TXT responses. |
| `status.expose_uptime` | `false` | Includes process uptime in status TXT responses. |

## Storage and Observability

| Key | Fallback | Description |
| --- | --- | --- |
| `database.file` | `/var/lib/tdns/tdns.sqlite` | Shared SQLite file for configuration overrides, DNS logs, aliases, and tagger data. |
| `dns_log.enabled` | `true` | Enables DNS query logging. |
| `dns_log.purge` | `180d` | Retention age used by the scheduled DNS-log purge. |
| `tagger.enabled` | `true` | Enables address/host labels used by scoped middleware rules. |

TDNS creates the configured database and applies migrations during
`tdns serve` startup. SQLite uses WAL mode, so the database directory can also
contain `tdns.sqlite-wal` and `tdns.sqlite-shm`. Native and container
deployments must persist and back up the complete directory, not only the main
database file. The database also stores hashed browser-session credentials,
authorization context, and consumed browser-code identifiers. Treat it and its
backups as authentication data. The storage must provide reliable local file
locking.

## HTTP Server and CLI Client

### Authentication

| Key | Fallback | Description |
| --- | --- | --- |
| `auth.issuer` | `tdns` | Required `iss` claim for management bearer tokens. |
| `auth.bearer_audience` | `tdns-management-api` | Required management bearer-token audience. |
| `auth.active_key.id` | `ephemeral` when no key is configured | Identifier written to the JWT `kid` header. A persistent key requires an explicit identifier. |
| `auth.active_key.environment` | `TDNS_AUTH_ACTIVE_KEY` | Environment variable containing the base64-encoded active HS512 key. |
| `auth.active_key.file` | empty | Restricted regular file containing the base64-encoded active key. |
| `auth.active_key.value` | empty | Inline base64-encoded active key. Retained for compatibility, but not recommended for production. |
| `auth.previous_key.id` | empty | Identifier of the previous verification-only key. |
| `auth.previous_key.environment` | `TDNS_AUTH_PREVIOUS_KEY` | Environment variable containing the previous base64-encoded key. |
| `auth.previous_key.file` | empty | Restricted regular file containing the previous key. |
| `auth.previous_key.value` | empty | Inline previous key. |
| `auth.previous_key_accept_until` | empty | Required RFC3339 cutoff when a previous key is configured. |

For each key slot, TDNS uses the first available source in this order:
environment variable, key file, inline `value`. The deprecated
`server.signing_key` value is the final active-key fallback. Key files must be
regular files that permit at most owner access and optional group read access;
every decoded HS512 key must contain at least 64 bytes.

TDNS validates the complete key set before opening listeners. Active and
previous identifiers must be different and contain at most 64 letters, digits,
dots, underscores, or hyphens. The previous key is rejected at and after
`auth.previous_key_accept_until`. It can validate existing tokens before that
instant, but only the active key issues new tokens.

When no persistent active key is configured, `tdns serve` creates a temporary
process key for local startup. Tokens cannot survive restart, and offline
`tdns adm token` and `tdns adm browser-code` issuance is refused.

Generate a browser login code from a host that has access to the persistent
active signing key:

```bash
tdns --config /etc/tdns/tdns.yaml adm browser-code \
  --sub admin@tdns \
  --scope read-write
```

The command prints only the code to standard output. Codes expire after two
minutes by default, can be shortened with `--ttl`, and cannot be extended past
two minutes. Each code is purpose-bound to browser session exchange and can be
redeemed only once. The browser exchange endpoint is documented separately
when that HTTP workflow is enabled.

To rotate a persistent key without immediately invalidating every current
strict-format token:

1. Move the current active identifier and key source to `previous_key`.
2. Set `previous_key_accept_until` to the absolute RFC3339 end of the overlap.
3. Install a newly generated key under a new `active_key.id`.
4. Restart TDNS and reissue credentials. New tokens use only the new active key.
5. Remove the previous key configuration after the cutoff.

The cutoff must be chosen deliberately. It should be no later than the
expiration of the credentials being migrated, and the old key file must remain
protected until it is removed.

### Server

| Key | Fallback | Description |
| --- | --- | --- |
| `server.listen_addr` | empty | UDP DNS listener, such as `:53`. |
| `server.api_addr` | empty | HTTPS management listener, such as `:8443`. |
| `server.api_cert_file` | empty | HTTPS certificate file. |
| `server.api_key_file` | empty | HTTPS private-key file. |
| `server.pprof_addr` | empty | Optional unauthenticated pprof HTTP listener; empty disables it. Restrict this listener to trusted interfaces. |
| `server.signing_key` | empty | Deprecated inline active-key fallback for installations migrating from `v0.1.6` or older. Prefer `auth.active_key`. |
| `server.swagger_enabled` | `false` | Exposes Swagger UI and raw Swagger/OpenAPI documents under `/swagger/`. |

### CORS

| Key | Fallback | Description |
| --- | --- | --- |
| `cors.enabled` | `false` | Enables cross-origin management API requests. |
| `cors.allowed_origins` | `[]` | Explicit allowed origins. When CORS is enabled and this list is empty, all origins are accepted. |

### Client

| Key | Fallback | Description |
| --- | --- | --- |
| `client.server` | empty | Base HTTPS URL used by `tdns adm`, such as `https://tdns.example.com:8443`. |
| `client.ca_cert` | empty | CA or self-signed certificate trusted by the management client. |
| `client.token` | empty | Bearer token sent by `tdns adm`. Treat it as a secret. |

`tdns adm token` issues read-write tokens by default. Use `--scope read-only`
for reporting and inspection clients. Tokens default to 30 days and are limited
to 180 days unless the administrator explicitly supplies `--allow-long-lived`.

## CLI Overrides

Only these `tdns serve` flags are bound to configuration keys:

| Flag | Configuration key |
| --- | --- |
| `-u`, `--upstream` | `upstreams` |
| `-s`, `--stub` | `stub_resolver.stubs` |
| `-f`, `--hosts` | `static_response.file` |
| `-b`, `--blacklist` | `blacklist.file` |
| `-z`, `--zenfile` | `zen_mode.file` |
| `-T`, `--zentime` | `zen_mode.time` |
| `-t`, `--timeout` | `timeout` |
| `-U`, `--upstreamtimeout` | `upstream_timeout` |

`-c/--configfile` selects the file and `-v/--verbose` changes logging; neither
maps to a YAML key.

## Persisted SQLite Overrides

Management API operations can persist a limited overlay in the
`config_overrides` table inside `database.file`. TDNS applies this overlay
after Viper has resolved defaults, YAML, environment values, and CLI flags,
then initializes middleware. API writes also update the corresponding running
middleware immediately.

Only the settings in the following table support SQLite overrides. Every other
configuration key must be changed through YAML, a bound CLI flag, or its
runtime-only API operation where one exists.

| Effective setting | API operation | Startup behavior |
| --- | --- | --- |
| `cache.enabled` | `POST /api/cache/{start\|stop}` | Replaces the resolved boolean value. |
| `cache.excludes` | `POST /api/cache/excludes` | Persisted selectors are appended to configured selectors. |
| Static hosts | `POST /api/static-response/persisted` | Merged over `static_response.file`; persisted addresses win by domain. |
| Zen domains | `POST /api/zen-mode/persisted/domains` | Added to `zen_mode.file` and `zen_mode.domains`. |
| Zen exclusions | `POST /api/zen-mode/persisted/excludes` | Added to `zen_mode.excludes`. |
| Blacklist hosts | `POST /api/blacklist/persisted/hosts` | Added to the configured blacklist. |
| Blacklist exclusions | `POST /api/blacklist/persisted/excludes` | Added to `blacklist.excludes`. |

Each replacement endpoint replaces the persisted set for its category; it
does not rewrite `tdns.yaml`. An empty submitted list clears that persisted
category. Domain values are trimmed, lowercased, and stored without a trailing
dot. Selector values are normalized before storage.

The following API changes are runtime-only and disappear after restart:

- stub resolver entry replacement and enable/disable state
- zen runtime-domain replacement and active timer state
- static runtime-host replacement and enable/disable state
- blacklist runtime whitelist and enable/disable state
- cache contents (clearing the cache does not change configuration)

Do not edit `config_overrides` manually. Use the management API so validation,
normalization, in-memory state, and persistent state remain synchronized. A
configured and writable `database.file` is required for persisted operations.

## Production Hardening Checklist

Before considering an installation production-ready:

- Run TDNS as the dedicated `tdns` account or container UID/GID `65532`; never
  run the serving process as root.
- Keep the YAML configuration and API private key readable only by root and the
  service group, or only by the container runtime identity.
- Treat authentication key files, inline key values, environment keys, and
  `client.token` as credentials. Rotate values copied from old samples or
  installations.
- Mount `/etc/tdns` read-only while serving and keep `/var/lib/tdns` as the only
  persistent writable container mount.
- Bind DNS and management HTTPS only to loopback or explicit trusted LAN/VPN
  addresses and enforce matching firewall rules.
- Do not use container privileged mode or host networking. Drop all
  capabilities and map host port 53 to container port 8053.
- Keep `server.swagger_enabled` false and `server.pprof_addr` empty during
  normal operation.
- Keep CORS disabled. Frontend development should use only explicitly listed
  local origins.
- Protect SQLite data and backups as sensitive DNS history. Use cold backups
  until an online backup operation is implemented, bound backup retention, and
  test restoration.
- Pin native releases by checksum and container releases by immutable digest.
- Verify logs, DNS resolution, management HTTPS, database migrations, and
  persisted state after every upgrade.

The current supported topology and accepted security boundaries are defined in
[Security](security.md).
