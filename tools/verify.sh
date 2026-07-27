#!/usr/bin/env sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

cd "$ROOT_DIR"
set -- api/docs api/openapi.yaml apiclient/generated web/app/generated

./tools/generate_api.sh

drift=$(git status --porcelain -- "$@")
if [ -n "$drift" ]; then
	echo "generated API files are not up to date:" >&2
	printf '%s\n' "$drift" >&2
	git diff -- "$@"
	exit 1
fi

./tools/generate_web.sh
go test ./...
npm --prefix web test
npm --prefix web run typecheck
