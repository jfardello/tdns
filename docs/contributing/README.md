# Contributing

TDNS contains a Go server and CLI plus a Nuxt web application embedded into the
binary. Generated API contracts and clients are committed to the repository.

## Contribution Flow

TDNS uses a lightweight integration-branch flow:

1. Fork the repository on GitHub.
2. Create a focused feature or fix branch from the upstream `develop` branch.
3. Push that branch to the contributor's fork and open a pull request against
   the upstream `develop` branch.
4. Keep commits reviewable and update the branch when review or CI finds a
   problem. CI runs for pull requests targeting `develop`, but ordinary direct
   pushes to `develop` do not start a workflow.
5. Maintainers periodically open a pull request from `develop` to `main`.
   Merging that promotion runs CI on `main` and publishes the documentation.
6. Create release tags only from commits contained in `main`. A `v*` tag starts
   the release workflow, which rejects a tag whose commit is not in `main`.

A maintainer may commit directly to `develop` when intentionally working
without per-push Actions, but should still run the local verification commands
below before pushing.

For an urgent production fix, branch from `main`, open the fix pull request
against `main`, release it, and then merge or cherry-pick the same fix back into
`develop` so the branches do not diverge.

## Toolchain

- Go 1.26.5
- Node.js 22
- npm
- GoReleaser for release validation

Install the locked web dependencies:

```bash
npm --prefix web ci
```

## Verify A Change

Run the repository verification entry point:

```bash
./tools/verify.sh
go vet ./...
```

The script regenerates Swagger and OpenAPI artifacts, checks generated Go and
TypeScript clients for drift, builds the embedded frontend, and runs Go and web
tests.

When changing an HTTP route, annotation, or DTO, follow
[API contract maintenance](../api-contract-maintenance.md). Do not edit generated
API files manually. Security-relevant changes must update the
[living security document](../security.md) in the same change.

## Preview The Documentation

Serve the `docs` directory with any static HTTP server. For example:

```bash
python3 -m http.server 3000 --directory docs
```

Then open `http://127.0.0.1:3000`. Docsify renders the Markdown in the browser;
there is no documentation build step.

## Test OBS Packages From Develop

Development packages use a GoReleaser snapshot and a local `osc` build. Start
from a clean, committed `develop` checkout because the OBS source archive is
created from `HEAD`:

```bash
git switch develop
git pull --ff-only
goreleaser release --snapshot --clean --skip=docker
```

Keep the GoReleaser `before` hooks enabled so the API and embedded web assets
are regenerated. Snapshot versions have the form
`0.2.1~dev.g3aab283`; `~dev` makes them sort before the corresponding final
release in RPM and Debian package managers.

The updater reads the version from `dist/metadata.json`. Prepare a clean OBS
package checkout with:

```bash
./packaging/obs/update-package.sh \
  --local-dist dist \
  --source-ref HEAD \
  /path/to/obs/tdns
```

The updater consumes only local GoReleaser artifacts in this mode, verifies
them, creates both RPM and Debian source inputs, and runs `osc addremove`. It
does not contact GitHub, require a release tag, or commit to OBS.

List the repositories configured by the OBS project and build the desired
targets locally:

```bash
cd /path/to/obs/tdns
osc repositories
osc build openSUSE_Tumbleweed x86_64 tdns.spec
osc build Fedora_Rawhide x86_64 tdns.spec
osc build Debian_13 x86_64 'tdns_0.2.1~dev.g3aab283-1.dsc'
```

Replace the example repository names and snapshot version with the values from
your checkout. Building ARM targets on a different host requires QEMU/binfmt or
equivalent emulation because package validation executes the target binary.
More updater details are in the
[OBS packaging guide](../../packaging/obs/README.md).
