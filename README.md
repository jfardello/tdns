# TDNS

TDNS is a DNS forwarder with encrypted upstreams, caching, network-wide
blocklists, split DNS, a management API, and an embedded web interface. The
server, administration client, and web interface ship as one Go binary.

## Install A Release

Download the archive for your platform from the
[latest release](https://github.com/jfardello/tdns/releases/latest).
Linux releases are available for amd64 and arm64.

```bash
tar -xzf tdns_Linux_x86_64.tar.gz
sudo install -o root -g root -m 0755 tdns /usr/local/bin/tdns
tdns --version
```

Use `tdns_Linux_arm64.tar.gz` on 64-bit ARM systems.

## First Run

Generate a local configuration on unprivileged ports, then start TDNS:

```bash
tdns config \
  --output-dir "$PWD/tdns-config" \
  --basepath "$PWD/tdns-config" \
  --data-path "$PWD/tdns-data" \
  --systemd-unit=false \
  --listendns 127.0.0.1:8053 \
  --listenapi 127.0.0.1:8443

tdns serve -c "$PWD/tdns-config/tdns.yaml"
```

Open `https://127.0.0.1:8443` for the web interface. The generated certificate
is self-signed, so the browser must trust it before the page can load. Port 53
can be configured after checking that another local resolver does not already
use it.

## Documentation

- [Installing](docs/installing/README.md)
- [Security](docs/security/README.md)
- [Contributing](docs/contributing/README.md)
- [Web interface](docs/web-interface/README.md)
- [Complete configuration reference](docs/configuration.md)

The documentation can be viewed directly as Markdown or served as the Docsify
site from the `docs` directory.
