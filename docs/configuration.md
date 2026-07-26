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

## HTTP Server and CLI Client

### Server

| Key | Fallback | Description |
| --- | --- | --- |
| `server.listen_addr` | empty | UDP DNS listener, such as `:53`. |
| `server.api_addr` | empty | HTTPS management listener, such as `:8443`. |
| `server.api_cert_file` | empty | HTTPS certificate file. |
| `server.api_key_file` | empty | HTTPS private-key file. |
| `server.pprof_addr` | empty | Optional unauthenticated pprof HTTP listener; empty disables it. Restrict this listener to trusted interfaces. |
| `server.signing_key` | generated temporary key | Base64-encoded HMAC key used to validate and issue management JWTs. Configure a persistent secret for stable tokens. |
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
