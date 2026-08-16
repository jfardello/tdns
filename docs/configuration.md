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

Addresses such as `:53` and `:8443` mean "bind this port on every available
local address." They are valid server bind addresses, but clients cannot
connect to the empty host: use a concrete IP address or hostname covered by the
management certificate. On Linux, port 53 also requires root or
`CAP_NET_BIND_SERVICE`. The generated native systemd unit supplies that one
capability; a release binary run directly does not. The capability-free
container must instead use `:8053` internally and publish host UDP port 53 to
container UDP port 8053. Port 8443 is unprivileged; investigate an occupied
port, certificate/key access, firewalling, or missing port publishing if it is
unavailable.

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

The current `tdns config` command has no CIDR-selection flag. After bootstrap,
add the explicitly trusted prefixes to the restricted generated YAML before
starting TDNS. Do not add the server's interface prefix merely because it was
auto-detected: choose the smallest client prefixes required by the deployment.
The planned repeatable bootstrap option and its acceptance criteria are recorded
in [Bootstrap CIDR Selection Plan](security.md#bootstrap-cidr-selection-plan).

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
| `blacklist.external_file` | empty | Normalized relative path to a hosts file inside the configured GitHub repository. |
| `blacklist.external_repo` | empty | Exact HTTPS `github.com/<owner>/<repository>` URL used for blocklist refreshes. GitHub is the only supported remote provider. |
| `blacklist.external_repo_branch` | `master` | Valid Git branch read from the external GitHub repository. |
| `blacklist.external_pull_period` | empty | Cron expression for scheduled refreshes; empty disables scheduled pulls. |
| `blacklist.excludes` | `[]` | Configured domain or `label:<name>` exclusions. Persisted exclusions are merged with this list. |

Persisted blacklist hosts add entries to the configured blocklist. Persisted
exclusions add domain or label selectors to `blacklist.excludes`.

`tdns config` enables remote refreshes using the public
`StevenBlack/hosts` repository, the relative file
`alternates/gambling/hosts`, and a six-hour schedule. Remove both
`external_repo` and `external_file` and clear `external_pull_period` when a
deployment should use only a locally managed blocklist.

Remote refreshes resolve the branch and retrieve content through the GitHub API.
Public repositories need no credentials. For a private repository or a higher
GitHub API rate limit, set `GITHUB_TOKEN` in the TDNS process environment. TDNS
sends that token only to `api.github.com`, removes authorization before an
allowed cross-host redirect, and never writes the token to configuration or
logs.

Remote ingestion always enforces the following limits; they are not
configuration options:

| Resource | Limit |
| --- | --- |
| GitHub metadata response | 1 MiB |
| Compressed blocklist response | 64 MiB |
| Uncompressed blocklist content | 128 MiB |
| Hosts-file line | 64 KiB |
| Hosts entries | 2,000,000 |
| Redirects | 3 |
| Complete refresh, including validation | 2 minutes |

Redirects must remain on HTTPS and target the GitHub API or a supported GitHub
content host. A remote candidate must contain at least one valid hosts entry;
each data line starts with an IP address followed by one or more valid domain
names. Comments and blank lines are accepted. TDNS downloads to a mode `0600`
temporary file beside `blacklist.file`, validates and prepares the live lookup
tree, and only then atomically replaces the active file. Any network, size,
decompression, parsing, or installation failure leaves both the previous file
and in-memory blocklist active. The revision sidecar is also written atomically
with mode `0600` after successful installation.

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
| `dns_log.enabled` | `true` | Initial DNS query logging state. Runtime start/stop selections are persisted as configuration overrides. |
| `dns_log.purge` | `30d` | Mandatory retention age used by the scheduled DNS-log purge. It must be greater than zero and cannot exceed `180d`. |
| `dns_log.pseudonymization.domains` | `false` | Replaces canonical domain names with deterministic HMAC-SHA-256 tokens before queueing or logging. |
| `dns_log.pseudonymization.clients` | `false` | Replaces normalized client IP addresses with independently scoped HMAC-SHA-256 tokens before queueing or logging. Exact IP filters and aliases continue to work. |
| `dns_log.pseudonymization.key_environment` | `TDNS_DNS_LOG_PSEUDONYMIZATION_KEY` | Environment variable containing a base64-encoded key of at least 32 bytes. It takes precedence over `key_file`. |
| `dns_log.pseudonymization.key_file` | empty | File containing the base64-encoded key. It must be a regular file with no group write/execute or access for other users; `0600` or `0640` are suitable modes. |
| `tagger.enabled` | `true` | Enables address/host labels used by scoped middleware rules. |

TDNS creates the configured database and applies migrations during
`tdns serve` startup. SQLite uses WAL mode, so the database directory can also
contain `tdns.sqlite-wal` and `tdns.sqlite-shm`. Native and container
deployments must persist and back up the complete directory, not only the main
database file. The database also stores hashed browser-session credentials,
authorization context, and consumed browser-code identifiers. Treat it and its
backups as authentication data. The storage must provide reliable local file
locking.

The purge runs daily and cannot be disabled while DNS logging is enabled.
Prometheus reports purge attempts, duration, deleted rows, and the timestamp of
the last successful purge. Verify that the success timestamp advances and that
backups expire no later than the selected DNS-log retention period. Backups and
discarded database media must be disposed of so retained DNS activity cannot be
recovered through ordinary file access.

Domain and client pseudonymization can be enabled independently. Generate a
dedicated key, for example with `openssl rand -base64 32`, and do not reuse an
authentication or TLS key. HMAC tokens are deterministic so grouping, exact
client filters, and aliases remain useful, but they are pseudonyms rather than
anonymous data. Protect the key and its backups as sensitive data.

Changing the key or either pseudonymization mode makes existing DNS-log rows or
aliases incompatible. TDNS reports that the DNS-log data must be cleared and
pauses new DNS logging rather than mixing representations.

Administrators can control DNS logging through the management API or CLI:

```bash
tdns adm dnslog status
tdns adm dnslog stop
tdns adm dnslog clear
tdns adm dnslog start
```

Stop is a write barrier: every event accepted before it is flushed before the
operation returns, and later queries are not queued. Clear is accepted only
while logging is stopped and removes events, dashboard aggregates, aliases,
queued events, and sequence state. It also records the currently configured
pseudonymization mode and key, allowing logging to start without a restart.
The selected start/stop state survives restart through `config_overrides`.

### Diagnostics

Metrics and pprof use a separate, unauthenticated HTTP listener. The listener
accepts only a numeric, non-wildcard IP address; `0.0.0.0`, `[::]`, and hostnames
are rejected. Keep it on loopback unless an explicit LAN or VPN address is
protected by trusted network policy.

| Key | Fallback | Description |
| --- | --- | --- |
| `diagnostics.listen_addr` | `127.0.0.1:6060` | Trusted diagnostics HTTP listener. Wildcard addresses are rejected. |
| `diagnostics.metrics_enabled` | `true` | Serves Prometheus metrics at `/metrics` on the diagnostics listener. |
| `diagnostics.pprof_enabled` | `false` | Serves Go pprof endpoints under `/debug/pprof/` on the diagnostics listener. |

`server.pprof_addr` is no longer supported. Replace it with
`diagnostics.listen_addr` and set `diagnostics.pprof_enabled: true` only for a
bounded diagnostic session. Metrics and pprof are never served by management
HTTPS.

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
| `auth.browser.remember_days` | `10` | Absolute lifetime, in days, for explicitly remembered browser sessions. Must be from 1 through 30. |

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
redeemed only once through `POST /api/auth/exchange`.

Password login is disabled until a valid local credential has been stored in
SQLite. Set or replace the singleton administrator from an interactive terminal:

```bash
tdns -c /etc/tdns/tdns.yaml adm password set --username admin
```

The command prompts twice without echo and accepts no password argument or
password environment variable. The normalized username uses lowercase ASCII
letters, digits, dots, underscores, and hyphens, and the password must contain
12 through 72 UTF-8 bytes. Setting a password activates password login and
revokes every password-authenticated session. Disable it and revoke those
sessions with:

```bash
tdns -c /etc/tdns/tdns.yaml adm password disable
```

Browser-code login remains enabled independently of the local password. It is
the recovery path when the password is forgotten or disabled, provided the
operator still has local access to the active signing key. Use a browser code
to regain web access, then run the interactive `password set` command on a host
that can access the SQLite database to establish a new password.

The server exposes the following JSON browser-session endpoints:

| Endpoint | Purpose |
| --- | --- |
| `POST /api/auth/exchange` | Consume a browser code, set the session cookie, and return session metadata and a CSRF token. |
| `POST /api/auth/login` | Verify the local administrator password, set the session cookie, and return session metadata and a CSRF token. |
| `GET /api/auth/session` | Validate the session and issue a fresh, bounded CSRF token for a browser reload or tab. |
| `POST /api/auth/logout` | Validate browser request protections, revoke the session, and clear its cookie. |

The session cookie is named `__Host-tdns-session` and is set with `Path=/`,
`Secure`, `HttpOnly`, and `SameSite=Strict`. It contains an opaque identifier
rather than a JWT. Login requests with `remember` omitted or false receive the
existing non-persistent cookie and a 12-hour absolute server session. Requests
with `remember: true` receive `Expires` and `Max-Age` cookie attributes matching
the absolute server deadline configured by `auth.browser.remember_days`.
Remembered sessions do not slide or renew automatically.
Cookie-authenticated `POST`, `PUT`, `PATCH`, and `DELETE` requests
must send the CSRF token in `X-CSRF-Token` and pass same-origin validation.
Bearer-authenticated clients continue to use `Authorization` and do not use
browser CSRF processing. The embedded web UI uses these endpoints directly. It
stores session metadata and the CSRF token only in memory; the opaque session
identifier remains inaccessible in the HttpOnly cookie. A browser reload calls
`GET /api/auth/session` to restore state and issue a fresh bounded CSRF token.
The login page supports password and browser-code modes with one unchecked
`Remember this browser` option. Passwords and browser codes are cleared after
submission and are never written to Web Storage. The UI removes the legacy
`tdns_jwt_token` local-storage entry during startup.

The local administrator record, browser sessions, CSRF state, and consumed-code
history are stored only in SQLite; no password belongs in YAML. Back up and
restore the complete SQLite directory as one authentication boundary. A restore
also restores the snapshot's password hash and any session whose absolute
expiry is still in the future. After an untrusted or incident-recovery restore,
set a new password and reissue bearer credentials. Password rotation does not
revoke browser-code sessions, and TDNS does not currently provide a global
browser-session revocation command.

The browser UI and management API must share an origin. Production uses the
embedded UI. For Nuxt development, proxy `/api` through the development server:

```bash
cd web
TDNS_API_PROXY_TARGET=https://localhost:8443 npm run dev
```

The target certificate must be trusted by the development host. Cookie-based
development does not use `cors.enabled` or `cors.allowed_origins`.

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
| `server.signing_key` | empty | Deprecated inline active-key fallback for installations migrating from `v0.1.6` or older. Prefer `auth.active_key`. |
| `server.swagger_enabled` | `false` | Exposes Swagger UI and raw Swagger/OpenAPI documents under `/swagger/`. |

### CORS

| Key | Fallback | Description |
| --- | --- | --- |
| `cors.enabled` | `false` | Enables cross-origin management API requests for explicitly trusted bearer clients. Browser cookie authentication remains same-origin. |
| `cors.allowed_origins` | `[]` | Exact HTTP or HTTPS origins. Startup fails when CORS is enabled with an empty, malformed, or wildcard origin list. |

Credentialed CORS is not enabled. Cross-origin callers must use bearer
authentication. Each origin must contain only a scheme and host, with no path,
query, fragment, user information, surrounding whitespace, or wildcard.

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
- Keep `server.swagger_enabled` and `diagnostics.pprof_enabled` false during
  normal operation. Scrape metrics only through the trusted diagnostics
  listener.
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
