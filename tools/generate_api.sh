#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
SWAG_VERSION=v1.16.6
SWAG_PACKAGE="github.com/swaggo/swag/cmd/swag@${SWAG_VERSION}"
OAPI_CODEGEN_VERSION=v2.8.0
OAPI_CODEGEN_PACKAGE="github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@${OAPI_CODEGEN_VERSION}"
NODE_BIN="$ROOT_DIR/web/node_modules/.bin"

cd "$ROOT_DIR"

go run "$SWAG_PACKAGE" fmt -d api
go run "$SWAG_PACKAGE" fmt -d internal/httpapi
go run "$SWAG_PACKAGE" init \
	-g annotations.go \
	-d api,internal/httpapi \
	-o api/docs \
	--outputTypes go,json,yaml \
	--parseInternal

if [ ! -x "$NODE_BIN/swagger2openapi" ] || [ ! -x "$NODE_BIN/redocly" ] || [ ! -x "$NODE_BIN/openapi-typescript" ]; then
	npm --prefix web ci
fi

"$NODE_BIN/redocly" lint api/docs/swagger.yaml --config redocly.yaml
"$NODE_BIN/swagger2openapi" api/docs/swagger.yaml \
	--outfile api/openapi.yaml \
	--yaml
"$NODE_BIN/redocly" lint api/openapi.yaml --config redocly.yaml

"$NODE_BIN/openapi-typescript" api/openapi.yaml \
	--output web/app/generated/api.d.ts

go run "$OAPI_CODEGEN_PACKAGE" \
	--config apiclient/oapi-codegen.yaml \
	api/openapi.yaml
