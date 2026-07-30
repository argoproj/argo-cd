#!/usr/bin/env bash
# Generate SPDX 2.3 JSON for ui/ using a pinned pnpm standalone that includes `pnpm sbom`.
# Supports Linux (amd64, arm64) and macOS (Intel, Apple Silicon). Used locally and in
# .github/workflows/release.yaml (see --write for CI output path).
#
# CLEANUP once ui/package.json `packageManager` is pnpm 11+ and ui/pnpm-lock.yaml is refreshed (`pnpm install` in ./ui):
#   - Delete this script; in .github/workflows/release.yaml use `pnpm sbom` (see comments there). The export of
#     COREPACK_ENABLE_STRICT=0 below goes away with this file.
# =============================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly PNPM_SBOM_RELEASE_TAG='v11.0.0-rc.0'
CACHE_ROOT="${ROOT}/hack/.cache/pnpm-sbom"

# ui/package.json pins pnpm 10.x; standalone pnpm 11 otherwise exec-switches to that version (no `sbom`).
export COREPACK_ENABLE_STRICT=0

write_out=""
sbom_args=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    -o|--write|--output)
      if [[ $# -lt 2 ]]; then
        echo "usage: $0 [--write PATH] [extra args for pnpm sbom...]" >&2
        exit 1
      fi
      write_out="$2"
      shift 2
      ;;
    -h|--help)
      echo "usage: $0 [--write PATH] [extra args for pnpm sbom...]" >&2
      echo "  Default: write SPDX JSON to stdout." >&2
      echo "  --write PATH: write SPDX JSON to PATH (for CI)." >&2
      exit 0
      ;;
    *)
      sbom_args+=("$1")
      shift
      ;;
  esac
done

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
tarball=""
expected_sha=""
case "${os}-${arch}" in
  darwin-arm64)
    tarball="pnpm-macos-arm64.tar.gz"
    # https://github.com/pnpm/pnpm/releases/download/v11.0.0-rc.0/pnpm-macos-arm64.tar.gz
    expected_sha='ff7e95be75a27c793fbcdf49c4f9e0c8adfc54e214c7aea4b1306445f31e5391'
    ;;
  darwin-x86_64)
    tarball="pnpm-macos-x64.tar.gz"
    # https://github.com/pnpm/pnpm/releases/download/v11.0.0-rc.0/pnpm-macos-x64.tar.gz
    expected_sha='13fa24a2e0e25af7837acf13d84515fd5a4daab582ac271b01c0b574388ce0bd'
    ;;
  linux-x86_64)
    tarball="pnpm-linux-x64.tar.gz"
    # https://github.com/pnpm/pnpm/releases/download/v11.0.0-rc.0/pnpm-linux-x64.tar.gz
    expected_sha='fe82b94125a6b743456b869e823611a8837b545f2535e4602578e4c9fdb5742a'
    ;;
  linux-aarch64|linux-arm64)
    tarball="pnpm-linux-arm64.tar.gz"
    # https://github.com/pnpm/pnpm/releases/download/v11.0.0-rc.0/pnpm-linux-arm64.tar.gz
    expected_sha='69ad2d528f4a2c00fd42541a80c13491c57e66b2765b2d9d89829aeb0f6482be'
    ;;
  *)
    echo "Unsupported OS/arch for pinned pnpm SBOM helper: ${os}-${arch}" >&2
    exit 1
    ;;
esac

sha256_file() {
  local f="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${f}" | awk '{print $1}'
  else
    shasum -a 256 "${f}" | awk '{print $1}'
  fi
}

CACHE="${CACHE_ROOT}/${PNPM_SBOM_RELEASE_TAG}"
mkdir -p "${CACHE}"
TAR_PATH="${CACHE}/${tarball}"

if [[ ! -f "${TAR_PATH}" ]]; then
  echo "Downloading ${PNPM_SBOM_RELEASE_TAG}/${tarball} ..." >&2
  curl -fsSL "https://github.com/pnpm/pnpm/releases/download/${PNPM_SBOM_RELEASE_TAG}/${tarball}" \
    -o "${TAR_PATH}.part"
  actual="$(sha256_file "${TAR_PATH}.part")"
  if [[ "${actual}" != "${expected_sha}" ]]; then
    echo "SHA256 mismatch for ${tarball}: expected ${expected_sha}, got ${actual}" >&2
    exit 1
  fi
  mv "${TAR_PATH}.part" "${TAR_PATH}"
fi

actual="$(sha256_file "${TAR_PATH}")"
if [[ "${actual}" != "${expected_sha}" ]]; then
  echo "SHA256 mismatch for cached ${tarball}: expected ${expected_sha}, got ${actual}" >&2
  exit 1
fi

if [[ ! -x "${CACHE}/pnpm" ]]; then
  tar -xzf "${TAR_PATH}" -C "${CACHE}"
  chmod +x "${CACHE}/pnpm"
fi

echo "Using ${CACHE}/pnpm ($("${CACHE}/pnpm" --version))" >&2

run_sbom() {
  # With `set -u`, expanding an empty "${sbom_args[@]}" is an error on some bash builds.
  if [[ ${#sbom_args[@]} -gt 0 ]]; then
    (cd "${ROOT}" && "${CACHE}/pnpm" --dir ./ui sbom --sbom-format spdx --prod "${sbom_args[@]}")
  else
    (cd "${ROOT}" && "${CACHE}/pnpm" --dir ./ui sbom --sbom-format spdx --prod)
  fi
}

if [[ -n "${write_out}" ]]; then
  run_sbom >"${write_out}"
  test -s "${write_out}"
else
  run_sbom
fi
