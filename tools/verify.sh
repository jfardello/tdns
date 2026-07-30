#!/usr/bin/env sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

cd "$ROOT_DIR"
set -- api/docs api/openapi.yaml apiclient/generated web/app/generated

before=$(mktemp)
after=$(mktemp)
trap 'rm -f "$before" "$after"' EXIT
git diff --binary HEAD -- "$@" >"$before"

./tools/generate_api.sh

git diff --binary HEAD -- "$@" >"$after"
if ! cmp -s "$before" "$after"; then
	echo "generated API files are not up to date:" >&2
	diff -u "$before" "$after" || true
	exit 1
fi

./tools/generate_web.sh
go test ./...
npm --prefix web test
npm --prefix web run typecheck
