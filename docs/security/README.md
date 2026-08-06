# Security

TDNS is designed for one instance on a trusted home, LAN, or VPN network. The
management interface and DNS listener are not intended for direct Internet
exposure.

## Secure Defaults

- DNS accepts loopback clients by default. Every other client network requires
  an explicit CIDR allowlist entry.
- The management interface uses HTTPS and scoped authentication.
- Browsers use an opaque Secure, HttpOnly, SameSite=Strict session cookie with
  CSRF protection.
- CLI and Go API clients continue to use scoped bearer credentials.
- Metrics and optional pprof use a separate diagnostics listener.
- SQLite, signing keys, TLS keys, configuration, and backups are sensitive
  runtime assets.
- Remote blocklists are downloaded through a bounded, validated GitHub-only
  pipeline before they replace active data.

## Before Network Exposure

Keep management and diagnostics on loopback unless a trusted LAN or VPN address
is required. Match firewall rules to `dns_access.allowed_client_cidrs`, keep
Swagger and pprof disabled in normal operation, and run TDNS as a dedicated
non-root identity.

The complete current posture, accepted risks, and decision history are in the
[living security document](../security.md). Deployment-specific controls are in
[Installing](../installing/) and the production checklist is in the
[configuration reference](../configuration.md#production-hardening-checklist).
