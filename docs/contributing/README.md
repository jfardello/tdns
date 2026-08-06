# Contributing

TDNS contains a Go server and CLI plus a Nuxt web application embedded into the
binary. Generated API contracts and clients are committed to the repository.

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
