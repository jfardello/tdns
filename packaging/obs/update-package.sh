#!/bin/sh

set -eu

readonly PROGRAM="${0##*/}"
readonly DEFAULT_API_BASE=https://api.github.com
readonly DEFAULT_GIT_URL=https://github.com/jfardello/tdns.git
readonly DEFAULT_RELEASE_BASE=https://github.com/jfardello/tdns/releases/download

usage() {
	cat <<EOF
Usage: $PROGRAM [--revision NUMBER] TAG OBS_PACKAGE_CHECKOUT

Prepare a clean OBS package checkout from an immutable TDNS GitHub release.
TAG must use the form vX.Y.Z. The package revision defaults to 1.
EOF
}

fail() {
	printf '%s: %s\n' "$PROGRAM" "$*" >&2
	exit 1
}

require_command() {
	command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

revision=1
if [ "${1:-}" = "--revision" ]; then
	[ "$#" -ge 2 ] || fail "--revision requires a value"
	revision=$2
	shift 2
fi

[ "$#" -eq 2 ] || {
	usage >&2
	exit 2
}

tag=$1
obs_dir=$2

case "$tag" in
	v[0-9]*.[0-9]*.[0-9]*) ;;
	*) fail "tag must use the form vX.Y.Z" ;;
esac
version=${tag#v}

case "$version" in
	*[!0-9.]*|*.*.*.*|.*|*.|*..*) fail "tag must use the form vX.Y.Z" ;;
esac
[ "$(printf '%s' "$version" | awk -F. '{ print NF }')" -eq 3 ] || fail "tag must use the form vX.Y.Z"

case "$revision" in
	''|*[!0-9]*) fail "revision must be a positive integer" ;;
esac
[ "$revision" -gt 0 ] || fail "revision must be a positive integer"

for command_name in curl dpkg-source file git go gzip osc python3 sha256sum tar; do
	require_command "$command_name"
done

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(git -C "$script_dir" rev-parse --show-toplevel 2>/dev/null) || fail "the updater must run from a Git checkout"

[ -z "$(git -C "$repo_root" status --porcelain --untracked-files=normal)" ] || fail "TDNS Git checkout is dirty"
git -C "$repo_root" rev-parse --verify --quiet "refs/tags/$tag^{commit}" >/dev/null || fail "local Git tag not found: $tag"

git_url=${TDNS_GITHUB_GIT_URL:-$DEFAULT_GIT_URL}
local_commit=$(git -C "$repo_root" rev-parse "refs/tags/$tag^{commit}")
remote_refs=$(git ls-remote "$git_url" "refs/tags/$tag" "refs/tags/$tag^{}") || fail "could not resolve GitHub tag $tag"
remote_commit=$(printf '%s\n' "$remote_refs" | awk '$2 ~ /\^\{\}$/ { print $1; found=1 } END { if (!found && NR == 1) print first } { if (NR == 1) first=$1 }')
[ -n "$remote_commit" ] || fail "GitHub tag not found: $tag"
[ "$local_commit" = "$remote_commit" ] || fail "local tag $tag does not match the GitHub tag commit"

[ -d "$obs_dir" ] || fail "OBS package checkout does not exist: $obs_dir"
[ -d "$obs_dir/.osc" ] || fail "destination is not an OBS package checkout: $obs_dir"
[ -z "$(osc status "$obs_dir")" ] || fail "OBS package checkout is dirty"

api_base=${TDNS_GITHUB_API_BASE:-$DEFAULT_API_BASE}
release_base=${TDNS_RELEASE_BASE_URL:-$DEFAULT_RELEASE_BASE}
case "$release_base" in
	*latest*) fail "release base URL must not contain latest" ;;
esac

work_dir=$(mktemp -d "${TMPDIR:-/tmp}/tdns-obs-update.XXXXXX")
trap 'rm -rf "$work_dir"' EXIT HUP INT TERM
download_dir=$work_dir/downloads
stage_dir=$work_dir/stage
source_dir=$work_dir/tdns-$version
mkdir -p "$download_dir" "$stage_dir" "$source_dir"

release_json=$work_dir/release.json
curl --fail --location --silent --show-error \
	--output "$release_json" \
	"$api_base/repos/jfardello/tdns/releases/tags/$tag"

checksums=tdns_${version}_checksums.txt
archive_x86_64=tdns_Linux_x86_64.tar.gz
archive_arm64=tdns_Linux_arm64.tar.gz
archive_armv7=tdns_Linux_armv7.tar.gz
archives="$archive_x86_64 $archive_arm64 $archive_armv7"

python3 - "$release_json" "$tag" "$checksums" "$archive_x86_64" "$archive_arm64" "$archive_armv7" <<'PY'
import json
import sys

path, expected_tag, *required_assets = sys.argv[1:]
with open(path, encoding="utf-8") as release_file:
    release = json.load(release_file)

if release.get("tag_name") != expected_tag:
    raise SystemExit("GitHub release tag does not match the requested tag")
if release.get("draft"):
    raise SystemExit("GitHub release is still a draft")

assets = {asset.get("name") for asset in release.get("assets", [])}
missing = [name for name in required_assets if name not in assets]
if missing:
    raise SystemExit("GitHub release is missing assets: " + ", ".join(missing))
PY

for asset in $checksums $archives; do
	curl --fail --location --silent --show-error \
		--output "$download_dir/$asset" \
		"$release_base/$tag/$asset"
done

for archive in $archives; do
	entry_count=$(awk -v archive="$archive" '$2 == archive { count++ } END { print count + 0 }' "$download_dir/$checksums")
	[ "$entry_count" -eq 1 ] || fail "$checksums must contain exactly one entry for $archive"
done

(
	cd "$download_dir"
	sha256sum --check "$checksums"
)

validate_archive() {
	archive=$1
	component=$2
	expected_arch=$3
	expected_arm=${4:-}
	extract_dir=$work_dir/extract-$component
	mkdir "$extract_dir"
	tar -xzf "$download_dir/$archive" -C "$extract_dir"
	for required_file in tdns LICENSE README.md; do
		[ -f "$extract_dir/$required_file" ] || fail "$archive does not contain $required_file at its root"
	done
	[ -x "$extract_dir/tdns" ] || fail "$archive contains a non-executable tdns binary"
	file -b "$extract_dir/tdns" | grep -Eq 'statically linked.*stripped' || fail "$archive does not contain a static, stripped binary"
	go version -m "$extract_dir/tdns" | grep -Fq "GOARCH=$expected_arch" || fail "$archive has the wrong GOARCH"
	if [ -n "$expected_arm" ]; then
		go version -m "$extract_dir/tdns" | grep -Fq "GOARM=$expected_arm" || fail "$archive has the wrong GOARM"
	fi
}

validate_archive tdns_Linux_x86_64.tar.gz amd64 amd64
validate_archive tdns_Linux_arm64.tar.gz arm64 arm64
validate_archive tdns_Linux_armv7.tar.gz armhf arm 7

amd64_binary=$work_dir/extract-amd64/tdns
"$amd64_binary" --version | grep -Fq "tdns version $version" || fail "release binary version does not match $tag"
man_dir=$work_dir/man
mkdir "$man_dir"
"$amd64_binary" man --output-dir "$man_dir"
find "$man_dir" -type f -name 'tdns*.1' -print -quit | grep -q . || fail "release binary did not generate manual pages"

cp "$download_dir/$checksums" "$stage_dir/"
for archive in $archives; do
	cp "$download_dir/$archive" "$stage_dir/"
done
for asset in tdns.service tdns.sysusers tdns.tmpfiles README.packaging; do
	cp "$repo_root/packaging/common/$asset" "$stage_dir/"
done
cp "$repo_root/packaging/rpm/tdns-rpmlintrc" "$stage_dir/"

python3 - "$repo_root/packaging/rpm/tdns.spec" "$stage_dir/tdns.spec" "$version" "$revision" <<'PY'
import re
import sys

source, destination, version, revision = sys.argv[1:]
with open(source, encoding="utf-8") as source_file:
    contents = source_file.read()

contents, version_count = re.subn(r"^Version:\s+.*$", f"Version:        {version}", contents, count=1, flags=re.MULTILINE)
contents, release_count = re.subn(r"^Release:\s+.*$", f"Release:        {revision}%{{?dist}}", contents, count=1, flags=re.MULTILINE)
if version_count != 1 or release_count != 1:
    raise SystemExit("could not update RPM Version and Release fields")

with open(destination, "w", encoding="utf-8") as destination_file:
    destination_file.write(contents)
PY

timestamp=${SOURCE_DATE_EPOCH:-$(date +%s)}
rpm_date=$(date -u -d "@$timestamp" '+%a %b %d %Y')
suse_date=$(date -u -d "@$timestamp" '+%a %b %e %H:%M:%S UTC %Y')
deb_date=$(date -u -d "@$timestamp" --rfc-email)
maintainer='Jose Fardello <jmfardello@gmail.com>'

python3 - "$stage_dir/tdns.spec" "$rpm_date" "$maintainer" "$version" "$revision" <<'PY'
import sys

path, date, maintainer, version, revision = sys.argv[1:]
with open(path, encoding="utf-8") as spec_file:
    contents = spec_file.read()
marker = "%changelog\n"
if marker not in contents:
    raise SystemExit("RPM spec does not contain %changelog")
entry = f"* {date} {maintainer} - {version}-{revision}\n- Package upstream release {version} for OBS.\n"
contents = contents.replace(marker, marker + entry, 1)
with open(path, "w", encoding="utf-8") as spec_file:
    spec_file.write(contents)
PY

{
	printf '%s\n' '-------------------------------------------------------------------'
	printf '%s - %s\n\n' "$suse_date" "$maintainer"
	printf '%s\n\n' "- Package upstream release $version for OBS."
	cat "$repo_root/packaging/rpm/tdns.changes"
} > "$stage_dir/tdns.changes"

git -C "$repo_root" archive --format=tar --prefix="tdns-$version/" "$tag" | gzip -n > "$stage_dir/tdns_${version}.orig.tar.gz"
git -C "$repo_root" archive --format=tar "$tag" | tar -x -C "$source_dir"
cp -R "$repo_root/packaging/debian" "$source_dir/debian"

python3 - "$source_dir/debian/changelog" "$version" "$revision" "$maintainer" "$deb_date" <<'PY'
import sys

path, version, revision, maintainer, date = sys.argv[1:]
with open(path, encoding="utf-8") as changelog_file:
    previous = changelog_file.read()
entry = (
    f"tdns ({version}-{revision}) unstable; urgency=medium\n\n"
    f"  * Package upstream release {version} for OBS.\n\n"
    f" -- {maintainer}  {date}\n\n"
)
with open(path, "w", encoding="utf-8") as changelog_file:
    changelog_file.write(entry + previous)
PY

cp "$download_dir/tdns_Linux_x86_64.tar.gz" "$stage_dir/tdns_${version}.orig-amd64.tar.gz"
cp "$download_dir/tdns_Linux_arm64.tar.gz" "$stage_dir/tdns_${version}.orig-arm64.tar.gz"
cp "$download_dir/tdns_Linux_armv7.tar.gz" "$stage_dir/tdns_${version}.orig-armhf.tar.gz"

for source_archive in \
	"tdns_${version}.orig.tar.gz" \
	"tdns_${version}.orig-amd64.tar.gz" \
	"tdns_${version}.orig-arm64.tar.gz" \
	"tdns_${version}.orig-armhf.tar.gz"; do
	cp "$stage_dir/$source_archive" "$work_dir/$source_archive"
done

(
	cd "$work_dir"
	dpkg-source --build "tdns-$version"
)
cp "$work_dir/tdns_${version}-${revision}.dsc" "$stage_dir/"
cp "$work_dir/tdns_${version}-${revision}.debian.tar."* "$stage_dir/"

managed_patterns='tdns.spec tdns.changes tdns-rpmlintrc tdns.service tdns.sysusers tdns.tmpfiles README.packaging tdns_Linux_*.tar.gz tdns_*_checksums.txt tdns_*.orig.tar.gz tdns_*.orig-*.tar.gz tdns_*.dsc tdns_*.debian.tar.*'
backup_dir=$work_dir/backup
mkdir "$backup_dir"

for pattern in $managed_patterns; do
	for existing in "$obs_dir"/$pattern; do
		[ -e "$existing" ] || continue
		cp -p "$existing" "$backup_dir/"
	done
done

restore_destination() {
	for pattern in $managed_patterns; do
		for generated in "$obs_dir"/$pattern; do
			[ -e "$generated" ] || continue
			rm -f "$generated"
		done
	done
	for existing in "$backup_dir"/*; do
		[ -e "$existing" ] || continue
		cp -p "$existing" "$obs_dir/"
	done
	osc addremove "$obs_dir" >/dev/null 2>&1 || true
}

destination_changed=1
rollback_on_exit() {
	status=$?
	if [ "$destination_changed" -eq 1 ]; then
		restore_destination
	fi
	rm -rf "$work_dir"
	exit "$status"
}
trap rollback_on_exit EXIT
trap 'exit 1' HUP INT TERM

for pattern in $managed_patterns; do
	for existing in "$obs_dir"/$pattern; do
		[ -e "$existing" ] || continue
		rm -f "$existing"
	done
done

if ! cp -p "$stage_dir"/* "$obs_dir/"; then
	restore_destination
	destination_changed=0
	fail "could not copy prepared sources into the OBS checkout"
fi
if ! osc addremove "$obs_dir"; then
	restore_destination
	destination_changed=0
	fail "osc addremove failed; the previous checkout contents were restored"
fi

printf 'Prepared TDNS %s-%s OBS sources in %s\n' "$version" "$revision" "$obs_dir"
osc status "$obs_dir"
destination_changed=0
