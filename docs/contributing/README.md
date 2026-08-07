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
