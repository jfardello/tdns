#!/usr/bin/env sh

set -eu

if [ "$#" -lt 2 ] || [ "$#" -gt 3 ]; then
	echo "usage: $0 allowed|denied server [port]" >&2
	exit 2
fi

expected=$1
server=$2
port=${3:-53}

if ! command -v dig >/dev/null 2>&1; then
	echo "dig is required for the DNS exposure check" >&2
	exit 2
fi

case "$expected" in
allowed)
	if ! dig @"$server" -p "$port" example.com A +time=1 +tries=1 >/dev/null; then
		echo "expected an allowed DNS response from $server:$port" >&2
		exit 1
	fi
	;;
denied)
	if dig @"$server" -p "$port" example.com A +time=1 +tries=1 >/dev/null 2>&1; then
		echo "unexpected DNS response from denied source to $server:$port" >&2
		exit 1
	fi
	;;
*)
	echo "first argument must be allowed or denied" >&2
	exit 2
	;;
esac
