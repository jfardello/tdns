#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
SWAG_VERSION=v1.16.6
SWAG_PACKAGE="github.com/swaggo/swag/cmd/swag@${SWAG_VERSION}"

cd "$ROOT_DIR"

go run "$SWAG_PACKAGE" fmt -d api
go run "$SWAG_PACKAGE" init \
	-g annotations.go \
	-d api \
	-o api/docs \
	--outputTypes go,json,yaml \
	--parseInternal
