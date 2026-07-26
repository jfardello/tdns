# TDNS

**A DNS over TLS forwarder with caching, black hole, and runtime reconfiguration features.**

![tdns logo](gopher-sm.png)

TDNS focuses on privacy by acting as a DNS over TLS proxy and DNS sinkhole, preventing data gathering by carriers and
trackers. **It can also route DNS requests to stub servers for local internal networks** in changing environments
like public Wi-Fi, VPNs, or 5G services.

## Features

* Supports TLS and clear DNS upstreams.
* Web interface.
* Static host file responses.
* Routes DNS calls to specific domains via stub servers for internal services.
* Zen mode (disables social sites for a period of time).
* Caching.
* Network wide Black-hole service (responds with A records to `0.0.0.0` for blacklisted services and supports whitelisting).
* Scheduled black lists download.
* Single binary with server, web interface and CLI tool.
* REST management API.


## Download

Download from the project releases and add `tdns` to your `$PATH`.

## Quick start


```bash
sudo tdns config -o /etc/tdns -p /etc/tdns
sudo tdns serve -c /etc/tdns/tdns.yaml
```
     
> **[Note]:**  tdns will fail to bind to port 53 if it is already being used, check for network-manager/dnsmasq/etc listening to :53. 

The generated `tdns.yaml` contains the settings needed by both sides of the binary:

* The `server` section used by `tdns serve`.
* The `client` section used by `tdns adm ...` commands.

`tdns` is both the server and the CLI client for the management API.

## Installation bootstrap

Generate a starting configuration directory:

```bash
sudo tdns config -o /etc/tdns -p /etc/tdns
```

That command creates a bootstrap configuration with:

* a self-signed API certificate and private key
* a random HMAC signing key for JWT tokens
* a `tdns.yaml` file with both server and client sections
* a pre-issued client token embedded in `client.token`
* a sample `tdns.service` unit file

If you want to use systemd, review the generated unit file before installing it:

```bash
sudo mv /etc/tdns/tdns.service /etc/systemd/system/tdns.service
sudo systemctl daemon-reload
sudo systemctl enable --now tdns
```

If you prefer to start the server directly:

```bash
sudo tdns serve -c /etc/tdns/tdns.yaml
```

To refresh generated API clients and the embedded frontend before building
binaries:

```bash
./tools/generate_api.sh
./tools/generate_web.sh
go build ./...
```

The release workflow uses these same generation scripts before compiling
binaries. See [API Contract Maintenance](docs/api-contract-maintenance.md) for
source ownership, generated files, and the complete verification workflow.

## Usage

Show the command surface:

```bash
tdns help
tdns serve --help
tdns config --help
tdns adm help
```

Common server usage:

```bash
tdns serve -c /etc/tdns/tdns.yaml
tdns serve -c /etc/tdns/tdns.yaml -u tls://1.1.1.1:853#cloudflare-dns.com
tdns serve -c /etc/tdns/tdns.yaml -f /etc/hosts -b /etc/tdns/blacklist.hosts
```

For frontend development against a separately running TDNS server:

```bash
cd web
TDNS_API_URL=https://localhost:8443 npm run dev
```

If you do that, enable `cors.enabled` in `tdns.yaml` and set `cors.allowed_origins` to your Nuxt dev origin such as `http://localhost:3000`.

Common admin usage:

```bash
# Enable or disable middleware
tdns adm start blacklist
tdns adm stop blacklist
tdns adm start stub-resolver
tdns adm stop static-response
tdns adm start zen-mode

# Replace runtime data without changing the config file
tdns adm replace --stub 'corp.example,udp://10.0.0.53'
tdns adm replace --zen-domains facebook.com,x.com,instagram.com

# Query and label dnslog data
tdns adm top --since 24h --limit 20
tdns adm alias --hostname laptop --address 192.168.1.20

# Mint a new admin token from a config that has server.signing_key
tdns adm token --exp 365 --sub admin@tdns
```

## Test the server

```bash
dig TXT _status.tdns.local @127.0.0.1
```

If the service is started, point your local DNS configuration to the address on which TDNS listens.

## Upstream format

Configuration files use the upstream concept, which is just a URL with this format:

`proto://address:port#DNS-name`

**Proto**
Either `tcp`, `udp`, or `tls`.

**Address**
IP address of the DNS server.

**Port**
Server port.

**DNS name**
Optional hostname expected in the upstream TLS certificate. If set, TDNS verifies the certificate against this name.

## Configuration reference

TDNS reads server, middleware, storage, HTTPS, and management-client settings
from one YAML file. Selected `tdns serve` flags override YAML values, and a
small set of management API operations persist an additional overlay in the
SQLite database without rewriting the file.

See [TDNS Configuration](docs/configuration.md) for every key and fallback,
CLI precedence, selector formats, security notes, and the complete list of
persisted versus runtime-only overrides. The repository's
[tdns.yaml](tdns.yaml) includes every recognized YAML option.

For deployment hardening and the current findings, see the
[Security Review](docs/security-review.md). The ordered remediation work is in
the [Security Implementation Plan](docs/security-plan.md).

## Certificates and hostnames

The REST API is always HTTPS. The certificate used by the API server comes from `server.api_cert_file` and `server.api_key_file`.
The `tdns config` command generates a self-signed certificate, and the `-H/--hosts` values become certificate SANs.

If you want the server certificate to be valid for a specific hostname, generate the bootstrap with that hostname:

```bash
tdns config \
  -o /etc/tdns \
  -p /etc/tdns \
  -l 0.0.0.0:53 \
  -a 0.0.0.0:8443 \
  -H tdns.example.com \
  -H 203.0.113.10
```

That makes the generated certificate valid for both `tdns.example.com` and `203.0.113.10`.

Then make sure the generated config points the embedded client at the same hostname used in the certificate:

```yaml
client:
  server: https://tdns.example.com:8443
```

If you connect by IP address instead of hostname, include that IP in `-H` when generating the certificate.

## Remote client configuration

Because `tdns config` writes one YAML file for both roles, the easiest way to bootstrap a remote client is:

1. Run `tdns config` on the server host.
2. Keep the generated `server.*` section on the server.
3. Copy the certificate named in `client.ca_cert` to the remote machine.
4. Copy the `client` section, or create a minimal client-only config there.

A minimal remote client config looks like this:

```yaml
client:
  server: https://tdns.example.com:8443
  ca_cert: /etc/tdns/tdns_cert.pem
  token: <jwt token>
```

You can reuse the token generated by `tdns config`, or issue a new one on the server with:

```bash
tdns adm token --exp 365 --sub remote-admin@tdns
```

That token must be created from a config that contains the server `signing_key`.

## REST API

REST calls require:

* HTTPS to the address configured in `server.api_addr`
* a bearer token signed with `server.signing_key`
* the CA or self-signed certificate trusted by the client

The current OpenAPI description is available at [api/openapi.yaml](api/openapi.yaml).
Set `server.swagger_enabled: true` to expose Swagger UI at `/swagger/`; it is
disabled by default. Generation and client maintenance are documented in
[API Contract Maintenance](docs/api-contract-maintenance.md).

Go applications can import the management client from
`github.com/jfardello/tdns/apiclient`. DNS upstream transport is exposed
separately as `github.com/jfardello/tdns/resolver`.

## Getting black hole lists

TDNS uses plain hosts files, usually pointing to `0.0.0.0`. Various projects provide quality hosts files.
TDNS has been tested with data from `stevenblack/hosts`. TDNS ignores the IP in the hosts file and uses `0.0.0.0`
as the sinkhole answer.

## History

TDNS started as shell script that used to manage dnsmask over dbus in order to switch on demand DNS servers per domain as wel las the default one so that I could have multiple VPNs confined on Cgroups with isolated routing. Over the time it became bloated, default DNS pointed to an stubby instance for DNS over TLS, it had some python code for dbus scripting and managing black listing.

Too bloated for a shell script, but existing solutions lacked the dynamic reconfiguration so I re-wrote it as a go program. I've being using tdns for years now, lately I did some refactor in order to improve maintainability, documentation and added a web interface.
