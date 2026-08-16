# TDNS Security

## Purpose

This is the living security document for TDNS. It records the current security
model, deployment assumptions, accepted risks, and decisions that constrain
implementation. Update it whenever a change affects authentication,
authorization, exposed listeners, configuration secrets, stored DNS data,
remote content, dependencies, or release artifacts.

This document is the authoritative record of the security posture and decisions
that are currently in force. Historical findings and completed remediation work
remain available through the repository and issue history.

## Maintenance Rules

Every security-relevant change must update the applicable section of this file
in the same pull request. Changes should record:

- the protected asset or trust boundary being changed
- the chosen behavior and secure default
- any compatibility or deployment consequence
- newly accepted residual risk and its intended review point
- related configuration, API contract, tests, and operational documentation

Do not copy credentials, production addresses, private keys, bearer tokens,
browser login codes, session identifiers, or CSRF tokens into this document.

## Protected Assets

TDNS protects:

- DNS queries and responses passing through the resolver
- management operations that alter resolver behavior or persisted overrides
- the server signing key and issued bearer credentials
- browser login codes, local administrator credentials, and browser sessions
- TLS private keys and trusted CA material
- the SQLite database containing DNS activity, aliases, tags, and overrides
- local and remotely downloaded hosts and blocklist data
- diagnostic data exposed through metrics, Swagger, pprof, and logs
- release artifacts and the dependency/generation toolchain used to build them

## Trust Boundaries

### DNS Listener

Clients send clear DNS traffic to the configured UDP listener. TDNS processes
that traffic through resolver middleware and forwards it to configured UDP,
TCP, or DNS-over-TLS upstreams. A wildcard listener is not evidence that every
source is trusted. Loopback is allowed implicitly; every other client must
belong to an explicit `dns_access.allowed_client_cidrs` prefix. Private,
link-local, container, and VPN networks are not trusted automatically.

Unauthorized and per-client rate-limited UDP requests are dropped before
tagging, cache access, DNS logging, or upstream work. A global response budget
limits emitted UDP replies. Stub and default upstream concurrency is bounded;
authorized requests rejected by saturation receive `SERVFAIL` only while the
response budget has capacity. Listener binding and firewall rules remain
defense in depth.

### Management HTTPS Server

The embedded web UI, management API, and optional Swagger endpoints share the
configured HTTPS listener. Metrics and pprof are excluded from this listener.
The server requires TLS 1.2 or newer and
sets request timeouts and baseline browser security headers. API query routes
accept read-only or read-write bearer tokens or browser sessions; mutation
routes require read-write scope.

### CLI and Public Go Client

`tdns adm` and the importable `apiclient` authenticate with an Authorization
Bearer JWT. Bearer authentication remains supported alongside browser cookie
authentication. Programmatic clients are not converted to cookie sessions and
are not subject to browser CSRF processing.

### Browser UI

The embedded UI is served from the management API origin and uses same-origin
browser session authentication. It does not store a bearer token, session
identifier, or CSRF token in browser storage. During startup it deletes the
legacy `tdns_jwt_token` local-storage entry, then restores authentication state
through the session endpoint. Session metadata and CSRF tokens exist only in
memory.

UI document responses enforce a Content Security Policy that defaults to
blocking all sources, permits same-origin scripts and API connections, and
allows generated inline Nuxt bootstrap scripts only by their SHA-256 hashes.
The policy does not allow `unsafe-eval` or general inline script execution.
Inline style attributes and runtime-generated style elements remain allowed for
Nuxt UI component rendering. The base style policy also includes an
`unsafe-inline` compatibility fallback for browsers that do not enforce the
more specific style directives. External stylesheet resources remain restricted
to same-origin URLs. Lucide icons are bundled into the static application, and
runtime icon API fallback is disabled.

The implemented browser authentication flows are:

- a CLI command generates a short-lived, purpose-bound login code
- the embedded login form presents password and browser-code modes as equal
  choices and leaves `Remember this browser` unchecked by default
- browser codes are redeemed once through the HTTPS API
- when a valid enabled local administrator exists, its username and password
  can instead be submitted to `POST /api/auth/login`
- the server creates an opaque server-side session in SQLite
- the browser receives only a host-bound Secure, HttpOnly, SameSite=Strict
  session cookie
- browser mutation requests use session-bound CSRF protection
- bearer JWT behavior remains unchanged for CLI and Go API clients

The cookie contains an opaque session identifier, not a JWT. This permits
immediate logout, expiration, and revocation without maintaining a JWT denylist.
The exchange, password-login, session-status, and logout routes return
`Cache-Control: no-store`; request-bearing authentication routes are JSON-only.
Code exchange has bounded source-address and global rate limits. Password login
adds a normalized-username budget. TDNS uses the direct peer address and does
not trust forwarding headers. Browser API requests include same-origin
credentials, and unsafe requests attach the current session-bound CSRF token. A
management API `401` clears in-memory authentication state and returns the user
to login.

### Remote Blocklist Content

Remote repositories, redirects, API responses, and downloaded content are
untrusted inputs. Remote blocklist ingestion supports exact HTTPS GitHub
repository URLs only. Repository components and GitHub API metadata are
validated, request and response work is time-bounded, redirects remain on
approved HTTPS GitHub hosts, and authorization is removed before cross-host
redirects. Metadata, compressed content, decompressed content, line length, and
entry count have mandatory fixed limits.

Generated bootstrap configuration opts into the public `StevenBlack/hosts`
repository and refreshes its gambling alternate every six hours. Operators may
remove the paired remote repository and file settings when they require a
locally managed blocklist without this outbound dependency.

Candidates are downloaded to restricted temporary files on the destination
filesystem and parsed into a replacement lookup tree before installation. A
validated candidate is atomically renamed over the active file and then swapped
into live memory. Any failure before installation leaves both the previous file
and lookup tree active. Logs and bounded-label metrics report refresh outcomes
without repository credentials, authorization headers, or remote response
bodies.

### Local Storage and Logs

The SQLite database and logs can reveal DNS activity and operational details.
They must be accessible only to the dedicated service account and authorized
operators. Retention, backup access, and secure disposal require deployment
policy. Secrets and authentication credentials must never be logged.

## Current Controls

### Transport and HTTP

- The management API is HTTPS-only.
- TLS 1.2 is the minimum protocol version.
- HTTP read-header, read, write, and idle timeouts are configured.
- Responses include anti-sniffing, anti-framing, referrer, and permissions
  headers.
- CORS is disabled by default.

### DNS Admission Controls

- IPv4 and IPv6 loopback clients are allowed by default.
- Every non-loopback client network requires an explicit CIDR allowlist entry.
- Per-client token buckets have bounded, idle-expiring state.
- A global token bucket limits UDP responses.
- Concurrent stub and default upstream work is bounded.
- ACL, rate, and saturation metrics use only fixed reason labels and never
  contain client addresses, CIDRs, or query names.

### JWT Authentication

- Management routes accept either a Bearer authorization header or the
  `__Host-tdns-session` cookie, never both.
- Only HS512 is accepted for API bearer tokens.
- Issuer, management audience, issued-at, not-before, expiration, token
  identifier, subject, scope, purpose, and signing-key identifier are required.
- Read-only and read-write scopes have distinct values; read-write credentials
  satisfy read-only routes, while read-only credentials receive `403` on
  mutation routes.
- Bearer tokens are purpose-bound and cannot be substituted for browser
  login codes or sessions.
- Browser login codes use a purpose-specific derived signing key, a distinct
  audience, a two-minute maximum lifetime, and zero validation leeway.
- Redeeming a validated browser code atomically records its hashed identifier
  and creates one opaque 12-hour session. Concurrent replay creates no
  additional session.
- SQLite stores only hashes of browser session identifiers, CSRF tokens, and
  consumed code identifiers. Raw credentials are returned only to the caller.
- The session cookie is non-persistent and uses `Path=/`, `Secure`, `HttpOnly`,
  and `SameSite=Strict` without a `Domain` attribute.
- Cookie-authenticated unsafe requests require a matching `X-CSRF-Token` and
  same-origin fetch metadata or strict HTTPS Origin/Referer validation.
- Invalid Authorization headers never fall back to a valid session cookie.
- Expired browser sessions and consumed-code records are purged in bounded
  batches at startup and every 15 minutes.
- The active key issues tokens. One explicitly bounded previous key may verify
  tokens until its configured RFC3339 cutoff.
- Signing keys are generated with cryptographic randomness.
- Temporary generated signing keys are not printed to logs.
- Authentication failure, authorization denial, token issuance, and management
  mutations produce structured audit events without credentials or key
  material.

### Secrets and Configuration

- Generated configuration files use mode `0600`.
- Generated API private keys use mode `0600`; public certificates use mode
  `0644`.
- Native bootstrap files created by root must be changed to `root:tdns` mode
  `0640` before starting the generated non-root service. A selected
  configuration that cannot be read fails startup instead of silently applying
  listener and authentication defaults.
- Generated DNS and management listeners default to loopback.
- Generated bootstrap administration tokens have a 30-day lifetime.
- The repository sample contains no signing key or bearer token.
- An empty persistent signing key causes a temporary runtime key, which makes
  independently generated tokens invalid after restart. Production deployments
  must configure a persistent secret outside source control.
- Earlier sample credentials must be treated as compromised because removal
  from the working tree does not remove them from Git history or deployments.

### Deployment Artifacts

- The generated systemd unit runs as `tdns:tdns`, uses `UMask=0077`, grants
  only `CAP_NET_BIND_SERVICE`, and restricts writable storage to
  the configured runtime data path.
- The production container runs as UID/GID `65532` on a pinned SUSE BCI Nano
  base and contains no shell or package manager.
- The container deployment uses read-only configuration, a read-only root
  filesystem, a dedicated writable data mount, no privileged mode, no host
  networking, and no Linux capabilities.
- The serving process applies umask `0077` before creating SQLite, WAL,
  blocklist, or other runtime files.
- Release packaging publishes one OCI manifest for `linux/amd64` and
  `linux/arm64`.
- Container and native deployment guides define bootstrap, listener, firewall,
  SQLite WAL, backup, restore, upgrade, and rollback controls.

### Listener Binding and Reachability

- `:53` and `:8443` are valid wildcard bind addresses for the DNS and
  management listeners. TDNS warns about these addresses in production but does
  not reject them. The diagnostics listener is the only listener that rejects a
  wildcard address.
- A wildcard address is not a destination address. Clients must connect to a
  concrete host address or name covered by the management TLS certificate and
  permitted by host firewall and routing policy.
- UDP port 53 is a privileged port on normal Linux installations. A native
  non-root process needs `CAP_NET_BIND_SERVICE`; the generated systemd unit
  grants only that capability. A release binary started directly does not carry
  the capability from the archive.
- The production container deliberately drops every capability. It must listen
  on unprivileged UDP port 8053 and publish host port 53 to container port 8053.
  Configuring `:53` inside that container is incompatible with the approved
  capability-free runtime.
- TCP port 8443 is unprivileged. A failure there is not caused by Linux
  privileged-port rules; check for another listener, certificate/key access,
  host firewall or port publishing, and use of a concrete certificate-covered
  client address.
- A fatal DNS bind error terminates the TDNS process and therefore also removes
  a management listener that may have started successfully. Listener errors
  must be diagnosed from the first bind failure in the service or container
  log, not only from a later connectivity test.

### Diagnostics

- Swagger is opt-in through `server.swagger_enabled`.
- Metrics and pprof use a separate diagnostics HTTP listener that defaults to
  `127.0.0.1:6060` and rejects wildcard addresses.
- Metrics are enabled by default on that trusted listener.
- pprof is disabled by default through `diagnostics.pprof_enabled`.
- Enabled Swagger endpoints are currently unauthenticated.
- Diagnostics are unauthenticated plain HTTP and must be restricted by binding
  and network policy.

### Build and Dependencies

- The module and CI use Go 1.26.5.
- The patched-toolchain `govulncheck` baseline reports no reachable
  vulnerabilities.
- API generators are version-pinned and generated-file drift is checked.
- The web application uses Nuxt 4.5.1 and `@nuxt/ui` 4.10.0. Version ranges
  have patched minimums instead of floating `latest` dependencies.
- The `npm` CLI is supplied by the build environment and is not an application
  dependency.
- CI audits production web dependencies and blocks critical findings, matching
  the approved security baseline gate.

The production audit currently reports 12 accepted transitive findings: 11
high findings are one `brace-expansion` denial-of-service advisory propagated
through Nitro's archive-generation toolchain, and one low finding affects the
Windows esbuild development server. TDNS ships only Nuxt's statically generated
files embedded in the Go binary; it does not ship or execute Nitro, archiver,
esbuild, Node.js, or these package sources at runtime. These findings are
therefore classified as build-only and do not provide a path through the
deployed TDNS service. The project maintainer owns this temporary exception and
will review it by 2026-08-31 or when the next patched Nuxt/Nitro release becomes
available, whichever comes first.

## Established Decisions

| Decision | Status | Rationale |
| --- | --- | --- |
| Preserve bearer JWT authentication for CLI and public Go clients | Implemented | Maintains programmatic API compatibility. |
| Replace browser JWT storage with an HttpOnly session | Implemented | Prevents JavaScript from extracting the reusable session credential. |
| Store opaque browser sessions server-side in SQLite | Implemented | Supports direct expiration, logout, and revocation. |
| Support short-lived CLI browser codes and one local bcrypt administrator | Implemented | Preserves recovery through browser codes while adding an optional familiar password login. |
| Make browser login codes purpose-bound and single-use | Implemented | Limits replay and clipboard exposure. |
| Keep browser cookie authentication same-origin by default | Implemented | Avoids unnecessary credentialed CORS exposure. |
| Retain generated API artifacts in version control | Implemented | Builds do not require generators and CI can detect contract drift. |
| Build releases with Go 1.26.5 or a later patched supported release | Implemented | Avoids known reachable standard-library vulnerabilities found with Go 1.26.1. |
| Default DNS access to loopback and require an explicit CIDR allowlist for other clients | Implemented | Prevents a default installation from becoming an open resolver; firewalling remains defense in depth. |
| Use a two-minute browser login code and a non-persistent 12-hour session by default | Implemented | Limits bootstrap-code replay and preserves the current browser-close behavior unless persistence is explicitly requested. |
| Allow absolute remembered sessions from 1 through 30 days, defaulting to 10 | Implemented | Provides explicit browser persistence without sliding or indefinite renewal. |
| Store one local read-write administrator with bcrypt cost 12 and a 12-72 byte password | Implemented | Keeps credential scope and password verification bounded while avoiding plaintext configuration secrets. |
| Put metrics and pprof on a separate trusted listener and keep Swagger opt-in | Implemented | Separates diagnostics from the management surface and makes their network trust boundary explicit. |
| Enforce strict bearer claims without legacy-token compatibility | Implemented | Upgrades from `v0.1.6` or older require credential regeneration instead of retaining weak validation. |
| Load signing keys from secret files or environment variables and support active/previous key IDs | Implemented | Keeps production secrets out of ordinary YAML and permits controlled key rotation. |
| Purge DNS logs after 30 days by default, with a 180-day maximum | Implemented | Minimizes retained browsing data and prevents indefinite retention. |
| Support a single TDNS instance on a trusted home, LAN, or VPN network | Implemented | Internet-facing management deployments are outside the supported security model. |
| Do not support reverse-proxy deployments | Implemented | TDNS does not trust or interpret forwarded identity or origin headers. |
| Treat explicitly allowlisted DNS client networks as trusted | Implemented | DNS ACLs define the client trust boundary; unauthorized sources remain untrusted. |
| Issue CLI bearer tokens for 30 days by default with a 180-day normal maximum | Implemented | Reduces the previous 500-day default while retaining an explicit administrative override. |
| Block CI only for critical security findings | Implemented | Lower-severity findings remain visible for triage without blocking every build. |
| Support native/systemd and container deployments | Implemented | Both installation forms must provide equivalent least-privilege and secret-protection controls. |

## Policy Decisions

This section gives implementation detail for approved policy choices. A policy
whose status is pending must be resolved before its related implementation
starts.

### DNS Client Policy

TDNS allows loopback clients by default. Every non-loopback client network
must be present in an explicit CIDR allowlist. Standard private networks are not
trusted automatically. Application ACLs are mandatory; listener binding and
firewall policy remain defense-in-depth controls.

Status: Implemented on 2026-07-21 by the socket-peer admission policy and its
IPv4, IPv6, normalization, rejection, and bounded-state tests.

### Bootstrap CIDR Selection Plan

The application ACL is implemented, but bootstrap currently writes an empty
allowlist and requires the operator to edit YAML. Bootstrap ergonomics will be
completed without weakening the loopback-only default:

1. Add a repeatable `tdns config --allow-cidr <prefix>` option that writes only
   explicitly supplied IPv4 or IPv6 prefixes to
   `dns_access.allowed_client_cidrs`. Do not infer trust from host interfaces,
   RFC1918 ranges, container bridges, or a wildcard listener.
2. Move prefix parsing and canonicalization into shared configuration code so
   bootstrap and serving apply the same maximum of 256 entries, IPv4-mapped IPv6
   normalization, network masking, and duplicate rejection before credential
   files are generated.
3. Print a credential-free bootstrap summary containing the DNS bind address,
   the selected CIDRs, and whether the result remains loopback-only. Warn when a
   wildcard DNS listener is combined with non-loopback CIDRs, while allowing the
   explicitly requested configuration.
4. Keep listener selection separate from trust selection. `--listendns` decides
   where the socket binds; `--allow-cidr` decides which socket peers receive DNS
   service; host firewall or container publishing decides which packets can
   reach the socket.
5. Add generation tests for no option, repeated IPv4 and IPv6 options,
   canonical duplicates, malformed values, the 256-entry limit, and ensuring a
   rejected request creates no partial configuration or secret artifacts.
6. Update native and container bootstrap examples and verify each deployment
   from one allowed and one denied client. Container verification must use the
   peer address TDNS actually observes after the selected runtime's port
   publishing path.

Changing the allowlist through the management API remains out of this bootstrap
plan. It would require an authenticated configuration contract, atomic
persistence, live policy replacement, audit events, rollback behavior, and a
guard against removing the administrator's only working DNS path.

Status: Planned on 2026-08-07. The serving ACL is implemented; the repeatable
bootstrap selection option is not.

### Browser Credential Lifetimes

Browser login codes expire after two minutes. Browser sessions have a 12-hour
absolute lifetime, use non-persistent cookies, and do not have a refresh token.
Closing the browser removes the cookie even if the absolute lifetime has not
elapsed. Idle expiration is not required in the initial implementation.

The persistence, server HTTP integration, and embedded web UI migration are
implemented.

Status: Implemented on 2026-07-30.

### Diagnostics and Metrics Exposure

Metrics and pprof use a separate listener that defaults to loopback and may bind
only to an explicit numeric address. Swagger remains opt-in on management
HTTPS. Production deployments must keep diagnostics behind trusted network
controls; wildcard diagnostics binds fail startup. The management and DNS
listeners emit a startup warning when configured with wildcard addresses.

Status: Implemented on 2026-08-02.

### Authorization Compatibility

Strict issuer, audience, time, token-identifier, subject, purpose, scope, and
key-identifier validation is mandatory. TDNS does not provide a legacy-token
compatibility mode. Operators upgrading from `v0.1.6` or older must configure
an identified active key and reissue every bearer credential before restarting
the upgraded service.

Status: Implemented on 2026-07-29.

### Signing-Key Storage and Rotation

Production signing keys can be loaded from a restricted secret file or an
environment variable. Inline YAML remains a compatibility input but is not the
recommended production source. Tokens identify their signing key. The server
accepts one active key and one previous key during a bounded rotation overlap;
new tokens are issued only with the active key. Missing, unreadable, duplicate,
or invalid key identifiers fail startup.

Status: Implemented on 2026-07-29.

### DNS-Log Retention

DNS logs are retained for 30 days by default. Administrators may configure a
shorter period or increase retention to a maximum of 180 days. Purge cannot be
disabled. Backups containing DNS-log data must have equivalent access controls
and must expire no later than the documented backup-retention policy. Database
and backup disposal must prevent ordinary recovery of retained DNS activity.

Status: Implemented on 2026-08-02. TDNS validates configured and manual purge
durations, reports bounded purge metrics, and protects SQLite artifacts with
owner-only modes.

### DNS-Log Pseudonymization

Operators may independently pseudonymize logged domains and client addresses.
TDNS canonicalizes each identifier and replaces it with a deterministic
HMAC-SHA-256 token before asynchronous queueing or debug logging. Domain and
client tokens use separate contexts, and the dedicated key is loaded from an
environment variable or a restricted file. Client aliases and exact filters
operate on the client token, so raw client addresses are not added to storage.

Pseudonymization is not anonymization: low-entropy identifiers can still be
tested by anyone who obtains the key. Key access must therefore be restricted.
TDNS records the active modes and a non-secret key fingerprint; if retained
data was produced with an incompatible mode or key, logging pauses and TDNS
reports that the DNS-log data must be cleared.

Read-only administrators may inspect DNS-log status. Start, stop, and complete
deletion require read-write authorization and emit mutation audit events. Stop
prevents new events from being accepted and synchronously flushes accepted
events before returning. Complete deletion is rejected while logging is
running; while stopped it removes queued events, stored events, aliases,
dashboard aggregates, and sequence state in one serialized operation.

Status: Implemented on 2026-08-16.

### Supported Deployment Topology

The supported production topology is one TDNS instance deployed inside a
trusted home, LAN, or VPN network. The DNS and management listeners are not
intended to be Internet-facing. High availability, multi-instance session
sharing, and public management exposure are outside the current security model.

Reverse-proxy deployments are not supported. TDNS must not use
`Forwarded`, `X-Forwarded-For`, `X-Forwarded-Host`, or similar headers to make
authentication, authorization, CSRF-origin, rate-limit, or client-address
decisions. Operators may place network infrastructure in front of TDNS, but it
must preserve direct connection semantics and is outside documented support.

Status: Implemented on 2026-08-07 through the single-instance deployment
artifacts and direct-peer authentication, origin, and rate-limit behavior.

### Threat Assumptions

The host operating system and authorized administrator are trusted. Root or
kernel compromise is out of scope. Remote blocklist sources, unauthenticated
management input, browser input, and sources outside the configured DNS CIDR
allowlist are untrusted.

DNS clients inside explicitly configured CIDRs are trusted by policy. ACLs
therefore define a security boundary, not only a routing convenience. Rate and
concurrency controls should still protect availability from accidental load,
misconfiguration, and compromised devices, but deliberate attacks from an
allowlisted client network are not part of the guaranteed threat model.

Status: Implemented on 2026-07-21 by the DNS admission boundary. The trusted
client assumption remains an explicit limit of the guaranteed threat model.

### CLI Bearer-Token Lifetime

CLI bearer tokens are valid for 30 days by default. The normal maximum is 180
days. Issuance beyond that maximum requires the explicit
`--allow-long-lived` administrative override. Tokens issued by `v0.1.6` or
older must be replaced because they do not satisfy strict validation.

Status: Implemented on 2026-07-29.

### Security Baseline Gate

Security scans record findings at every severity. CI blocks automatically only
for findings classified as critical. High, moderate, low, and unclassified
findings remain visible and require normal triage, but do not fail the security
gate solely because they exist. Release documentation must not describe a
non-blocking finding as remediated or absent.

Status: Implemented on 2026-08-07 by the CI production audit configured with
the critical failure threshold and the documented triage exception process.

### Supported Installation Forms

Native/systemd and container deployments are supported. Both forms must run
TDNS as a dedicated non-root identity, restrict configuration, signing-key,
certificate-key, database, and log access, and expose only required listeners.

The generated systemd guidance must use a restrictive umask and appropriate
service hardening. Container guidance must avoid privileged mode and host
networking by default, drop unnecessary capabilities, mount credentials
read-only, and prefer port mapping or the minimum required bind capability for
DNS port 53. Container images and native release artifacts follow the same
dependency, vulnerability, and version-verification policy.

Status: Implemented on 2026-08-07 by the hardened generated systemd unit,
capability-free container, multi-architecture release configuration, and the
native and container operating guides.

### Password Login And Remembered Sessions

TDNS supports one enabled local read-write administrator stored in SQLite.
The credential is managed through an interactive CLI that never accepts a
plaintext password in command arguments, environment variables, or YAML.
Passwords must contain from 12 through 72 UTF-8 bytes and are stored only as a
bcrypt cost-12 hash. A valid stored credential enables password login; browser
codes remain available as an independent authentication and recovery method.

Both password and browser-code login offer an unchecked remember option.
Without it, TDNS retains the implemented non-persistent cookie and absolute
12-hour server session. With it, the cookie and server-side session receive the
same absolute lifetime from `auth.browser.remember_days`, which defaults to 10
and accepts values from 1 through 30. Remembered sessions do not slide or renew
automatically. Password changes and account disablement revoke every session
created through password authentication.

Password verification uses generic failures, a dummy bcrypt comparison for
unknown or unusable credentials, and bounded source-address, username, and
global attempt limits without durable account lockout. Cookie identifiers
remain opaque, server-side, Secure, HttpOnly, SameSite=Strict, host-only, and
protected by the existing CSRF controls. Implementation and verification are
tracked by issues `#91` through `#95`.

The JSON-only `POST /api/auth/login` endpoint implements password login. It
rejects bearer or existing cookie credentials, requires a same-origin request,
bounds and strictly decodes the body, and returns the same unauthorized response
for wrong, unknown, disabled, or unusable credentials. Unknown and unusable
accounts execute a fixed cost-12 dummy bcrypt comparison. Before bcrypt work,
the endpoint applies independent direct-peer and normalized-username budgets of
five attempts with one attempt restored every 30 seconds, plus a global burst
of ten with one attempt restored per second. Peer and username state are each
limited to 1024 entries and expire after 15 minutes of inactivity. Usernames,
passwords, hashes, session identifiers, and CSRF tokens are excluded from audit
events and metric labels.

The SQLite credential record, cost and input validation, interactive
`tdns adm password set` and `tdns adm password disable` commands, session
authentication-method attribution, and password-session revocation are
implemented by issue `#91`. The hardened password endpoint and generated API
contracts are implemented by issue `#92`. Optional absolute remembered sessions,
bounded configuration, persistence attribution, and persistent cookie lifetime
are implemented by issue `#93`. The dual-mode embedded login, shared explicit
remember control, secret clearing, and generic browser errors are implemented by
issue `#94`.

The local credential and all browser authentication state are part of the
SQLite backup boundary. Restoring a database restores the password hash,
consumed-code history, and any sessions that were unexpired at the snapshot's
timestamps. Password rotation or disablement revokes password-authenticated
sessions but deliberately leaves browser-code recovery sessions active.

Status: Implemented and operationally verified on 2026-08-02.

## Known Temporary Risks

- Diagnostics and enabled Swagger are unauthenticated and require their
  documented trusted-network controls.
- Frontend production dependencies retain documented build-only audit
  exceptions through 2026-08-31.
- Backup access, expiration, restoration, and disposal remain deployment
  responsibilities; TDNS cannot enforce policy on copies outside its runtime
  database directory.
- Restoring an older SQLite snapshot can restore unexpired sessions and an old
  local password hash. TDNS has no global browser-session revocation command;
  operators must protect backups as credentials and account for restored
  authentication state during incident recovery.

## Required Verification

Security-relevant changes should run the applicable subset of:

```bash
./tools/verify.sh
go test ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
npm --prefix web audit --omit=dev
npm --prefix web test
npm --prefix web run typecheck
```

Release validation must also inspect the built binary's Go version and exercise
the enabled HTTPS, Swagger, authentication, and shutdown paths.

## Decision Log

| Date | Decision | Consequence |
| --- | --- | --- |
| 2026-07-20 | Keep bearer authentication for programmatic clients and use opaque HttpOnly browser sessions. | Browser and API clients use separate authentication transports behind one authorization principal. |
| 2026-07-20 | Bootstrap browser sessions with short-lived, single-use CLI-generated codes. | TDNS does not need to add browser passwords, but code redemption requires replay protection. |
| 2026-07-20 | Use same-origin browser cookies by default. | Frontend development should proxy the API instead of enabling broad credentialed CORS. |
| 2026-07-30 | Reject ambiguous credentials and require CSRF plus same-origin evidence for cookie-authenticated mutations. | Invalid bearer credentials cannot fall back to cookies, while bearer clients remain outside browser CSRF processing. |
| 2026-07-30 | Keep browser session state and CSRF tokens in memory and restore them through the session endpoint. | Browser storage contains no reusable TDNS credential, and reloads obtain a fresh bounded CSRF token. |
| 2026-07-20 | Require Go 1.26.5. | CI and release builds use the patched toolchain baseline. |
| 2026-07-21 | Default DNS access to loopback with explicit CIDRs for every other client network. | Application ACLs become mandatory and private networks are not implicitly trusted. |
| 2026-07-21 | Use two-minute browser login codes and non-persistent 12-hour sessions without refresh tokens. | Browser restart ends the session and expired sessions require a new CLI-generated code. |
| 2026-07-21 | Move metrics and pprof to a separate trusted listener while keeping Swagger opt-in. | Diagnostics have a distinct network trust boundary and cannot default to a wildcard bind. |
| 2026-07-29 | Require strict bearer claims without a legacy-token compatibility mode. | Installations upgrading from `v0.1.6` or older regenerate signing keys and bearer credentials. |
| 2026-07-21 | Support file and environment signing-key sources with active and previous key identifiers. | Production secrets can avoid YAML and keys can rotate without an immediate full-token outage. |
| 2026-07-21 | Default DNS-log retention to 30 days, cap it at 180 days, and prohibit disabling purge. | TDNS cannot retain query history indefinitely through configuration. |
| 2026-07-26 | Support one TDNS instance on a trusted home, LAN, or VPN network and do not support reverse proxies. | Public management, forwarded-header trust, and multi-instance operation are outside the supported model. |
| 2026-07-26 | Treat DNS clients in explicitly allowlisted CIDRs as trusted. | Availability controls remain useful, but malicious allowlisted clients are outside the guaranteed threat model. |
| 2026-07-26 | Default CLI bearer tokens to 30 days and cap them at 180 days. | Automation receives a bounded lifetime substantially shorter than the previous default. |
| 2026-07-26 | Fail the CI security gate only for critical findings. | Other findings remain reported and triaged without automatically blocking builds. |
| 2026-07-26 | Support both native/systemd and container installations. | Both deployment forms require equivalent non-root, minimal-capability, restricted-secret, and listener controls. |
| 2026-08-01 | Add one CLI-managed bcrypt administrator and optional absolute remembered sessions while retaining browser-code login. | Password login is read-write, bcrypt uses cost 12 with 12-72 byte passwords, the default cookie remains non-persistent for 12 hours, and explicit persistence defaults to 10 days with a 30-day cap. |
| 2026-08-07 | Keep wildcard DNS and management binds available, but treat them only as socket selection and preserve explicit CIDR and network controls. | Port 53 requires the systemd bind capability or container host-port mapping; wildcard destinations and certificate coverage remain operator concerns. |
| 2026-08-07 | Plan explicit repeatable CIDR selection during bootstrap without network auto-discovery. | Generated configurations remain loopback-only unless the operator supplies each trusted prefix. |

## Related Documents

- [Configuration reference](configuration.md)
- [Container deployment](container-deployment.md)
- [Native and systemd deployment](systemd-deployment.md)
- [API contract maintenance](api-contract-maintenance.md)
