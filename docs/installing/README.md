# Installing

TDNS is distributed as a native Linux binary and as a multi-architecture
container image. Both include the DNS server, administration CLI, and embedded
web interface.

## Binary Release

Download the amd64 or arm64 archive from the
[release page](https://github.com/jfardello/tdns/releases), extract it,
and install the binary:

```bash
tar -xzf tdns_Linux_x86_64.tar.gz
sudo install -o root -g root -m 0755 tdns /usr/local/bin/tdns
tdns --version
```

Use `tdns_Linux_arm64.tar.gz` for 64-bit ARM.

## Local First Run

Use unprivileged loopback ports for an initial test:

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

The generated configuration contains a signing key and bootstrap bearer token.
Keep it private. It also configures a public blocklist download and creates
SQLite state under the selected data path.

Before using DNS port 53, check whether NetworkManager, `systemd-resolved`,
`dnsmasq`, or another resolver already owns it.

## Production Options

- [Deployment and configuration](../systemd-deployment.md) covers native,
  systemd, container, backup, and upgrade workflows.
- [Container reference](../container-deployment.md) provides the standalone
  Docker and OCI image guide.
- [Configuration reference](../configuration.md) lists every YAML option and
  override mechanism.

Review the [security section](../security/) before binding DNS or management to
a non-loopback address.
