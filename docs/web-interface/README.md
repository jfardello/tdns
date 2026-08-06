# Web Interface

The administration interface is embedded in the TDNS binary and served from
the management HTTPS listener. No separate web server or Node.js runtime is
required in production.

## Open The Interface

With the local generated configuration, open:

```text
https://127.0.0.1:8443
```

The bootstrap certificate is self-signed. Trust it in the browser or replace it
with a certificate valid for the management hostname.

## Sign In

The login page supports two independent methods:

- **Password** uses the local administrator credential stored as a bcrypt hash
  in SQLite. Set or rotate it interactively with
  `tdns -c /path/to/tdns.yaml adm password set --username admin`.
- **Browser code** uses a short-lived, single-use code generated with
  `tdns -c /path/to/tdns.yaml adm browser-code`. It remains available for
  recovery when password login is disabled.

`Remember this browser` is off by default. Without it, the cookie is
non-persistent and the server session lasts at most 12 hours. When selected,
the absolute lifetime comes from `auth.browser.remember_days`, which defaults
to 10 days and accepts values from 1 through 30.

The browser stores no bearer token, session identifier, password, login code,
or CSRF token in Web Storage.

## Swagger

Set `server.swagger_enabled: true` to expose Swagger under `/swagger/` on the
management listener. Swagger is unauthenticated and disabled by default; enable
it only on a trusted network and turn it off when it is no longer needed.

For endpoint generation and client ownership, see
[API contract maintenance](../api-contract-maintenance.md). For cookie and
authentication details, see [Security](../security/).
