#!/usr/bin/env sh

# Usage: ./add-goreleaser-checksum.sh <release_tag>   
# use the desired release tag, like v2.14.3
# Writes hack/ci/checksums/goreleaser-<tag>-linux-x86_64.tar.gz.sha256 (like helm-v<ver>-linux-amd64…).
# Older version files are left in place; add a new file when bumping goreleaser_version.

set -e

tag=$1
tarball=goreleaser_Linux_x86_64.tar.gz
checksumfile="goreleaser-${tag}-linux-x86_64.tar.gz.sha256"

wget "https://github.com/goreleaser/goreleaser/releases/download/${tag}/checksums.txt"

# Match the tarball line only: grep -F "$tarball" also matches *.tar.gz.sbom.json (substring).
if ! awk -v t="$tarball" '$NF == t { print; n = 1; exit } END { exit !n }' checksums.txt >"$checksumfile"
then
	rm -f "$checksumfile" checksums.txt
	echo "No line for ${tarball} in checksums.txt" >&2
	exit 1
fi

rm checksums.txt

script_dir=$(cd "$(dirname "$0")" && pwd)
outname="${script_dir}/${checksumfile}"
mv "$checksumfile" "$outname"
