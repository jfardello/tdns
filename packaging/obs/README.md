# Updating the OBS package sources

`update-package.sh` prepares a local OBS package checkout from an immutable
TDNS GitHub release. It packages the verified GoReleaser binaries; it does not
compile TDNS or download dependencies inside an OBS worker.

## Requirements

Install `curl`, `dpkg-source` (from `dpkg-dev`), `file`, `git`, `go`, `gzip`,
`osc`, `python3`, `sha256sum`, and `tar`. Both the TDNS Git checkout and the OBS
package checkout must be clean. The requested tag must already exist locally
and as a published, non-draft GitHub release. Its local commit must match the
same tag in the public GitHub repository.

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
