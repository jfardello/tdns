#!/usr/bin/env sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

cd "$ROOT_DIR/web"

if [ ! -d node_modules ]; then
  npm ci
fi

npm run generate
