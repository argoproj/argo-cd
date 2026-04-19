#!/bin/bash
# Release workflow only (linux/amd64). Version: hack/tool-versions.sh goreleaser_version.
# Bump version + run hack/ci/checksums/add-goreleaser-checksum.sh when upgrading.
set -eux -o pipefail

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
# shellcheck source=../tool-versions.sh
. "${SCRIPT_DIR}/../tool-versions.sh"

GORELEASER_VERSION="${GORELEASER_VERSION:-${goreleaser_version}}"

export TARGET_FILE=goreleaser_Linux_x86_64.tar.gz
export DOWNLOADS="${DOWNLOADS:-/tmp/goreleaser-dl}"
export CHKSUM_FILE="${SCRIPT_DIR}/checksums/goreleaser-${GORELEASER_VERSION}-linux-x86_64.tar.gz.sha256"

mkdir -p "${DOWNLOADS}"
[ -e "${DOWNLOADS}/${TARGET_FILE}" ] || curl -sLf --retry 3 -o "${DOWNLOADS}/${TARGET_FILE}" \
  "https://github.com/goreleaser/goreleaser/releases/download/${GORELEASER_VERSION}/${TARGET_FILE}"
"${SCRIPT_DIR}/../installers/compare-chksum.sh"
tmpdir=$(mktemp -d)
tar -C "${tmpdir}" -xf "${DOWNLOADS}/${TARGET_FILE}"
sudo install -m 0755 "${tmpdir}/goreleaser" "${BIN:-/usr/local/bin}/goreleaser"
goreleaser --version
