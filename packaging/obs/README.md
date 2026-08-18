# Updating the OBS package sources

`update-package.sh` prepares a local OBS package checkout from an immutable
TDNS GitHub release or a local GoReleaser development snapshot. It packages
verified GoReleaser binaries; it does not compile TDNS or download dependencies
inside an OBS worker.

## Requirements

Install `dpkg-source` (from `dpkg-dev`), `file`, `git`, `go`, `gzip`, `osc`,
`python3`, `sha256sum`, and `tar`; release updates also require `curl`. Both the
TDNS Git checkout and the OBS package checkout must be clean. In release mode,
the requested tag must already exist locally and as a published, non-draft
GitHub release, and both tags must resolve to the same commit.

## Usage

```sh
./packaging/obs/update-package.sh v0.2.0 /path/to/home:jfardello:tdns/tdns
```

Packaging-only revisions can be selected explicitly:

```sh
./packaging/obs/update-package.sh --revision 2 v0.2.0 /path/to/obs/tdns
```

The command downloads the x86-64, ARM64, and ARMv7 release archives plus the
versioned checksum manifest. It verifies every archive before rendering RPM and
Debian metadata in a temporary directory. Only after the full RPM and Debian
source set has been constructed does it replace the managed files in the OBS
checkout and run `osc addremove`.

Review the result and submit it manually:

```sh
osc status /path/to/obs/tdns
osc diff /path/to/obs/tdns
osc commit /path/to/obs/tdns
```

The updater never invokes `osc commit` and never uses GitHub's `latest` release
URL. On missing assets, checksum failures, tag/version mismatches, dirty
checkouts, or source-package construction errors, it exits without changing
the OBS checkout.

## Building a development snapshot locally

GoReleaser snapshots use a version such as `0.2.1~dev.g3aab283`. The tilde
makes the package sort before the eventual `0.2.1` release in both RPM and
Debian version ordering. Build all static archives without publishing images:

```sh
git switch develop
git pull --ff-only
goreleaser release --snapshot --clean --skip=docker
```

Do not skip GoReleaser's `before` hooks: they regenerate the API and embedded
web application. Commit the development changes before preparing OBS sources;
the updater requires a clean checkout and archives `HEAD` by default. It reads
the snapshot version from `dist/metadata.json`; prepare a clean OBS package
checkout with:

```sh
./packaging/obs/update-package.sh \
  --local-dist dist \
  --source-ref HEAD \
  /path/to/obs/tdns
```

The local mode performs the same checksum, architecture, binary-version,
manual-page, RPM, and Debian source validation as release mode. It does not
contact GitHub and does not require a tag. Inspect the configured repository
names and build without committing to OBS:

```sh
cd /path/to/obs/tdns
osc repositories
osc build openSUSE_Tumbleweed x86_64 tdns.spec
osc build Fedora_Rawhide x86_64 tdns.spec
osc build Debian_13 x86_64 'tdns_0.2.1~dev.g3aab283-1.dsc'
```

Use the exact repository and architecture names reported by
`osc repositories`. Non-native ARM builds require a working QEMU/binfmt or
equivalent emulation setup because the recipes execute `tdns` while validating
the package and generating its manual pages.
