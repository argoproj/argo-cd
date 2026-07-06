#!/bin/bash
# Download helm/kustomize/git-lfs binaries for linux/arm64 into hack/installers/downloads/.
# Run this on your Mac (Terminal) BEFORE podman build — uses macOS certs, not Podman VM.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOWNLOADS="${SCRIPT_DIR}/installers/downloads"
. "${SCRIPT_DIR}/tool-versions.sh"

INSTALL_OS=linux
ARCHITECTURE=arm64

mkdir -p "${DOWNLOADS}"

download() {
  local url="$1"
  local file="$2"
  if [[ -f "${DOWNLOADS}/${file}" ]]; then
    echo "✓ already exists: ${file}"
    return 0
  fi
  echo "↓ downloading ${file}"
  curl -sLf --retry 3 -o "${DOWNLOADS}/${file}" "${url}"
  echo "✓ saved ${file}"
}

HELM_FILE="helm-v${HELM_VERSION}-${INSTALL_OS}-${ARCHITECTURE}.tar.gz"
KUSTOMIZE_FILE="kustomize_${KUSTOMIZE_VERSION}_${INSTALL_OS}_${ARCHITECTURE}.tar.gz"
GIT_LFS_FILE="git-lfs-${INSTALL_OS}-${ARCHITECTURE}-v${GIT_LFS_VERSION}.tar.gz"

download "https://get.helm.sh/helm-v${HELM_VERSION}-${INSTALL_OS}-${ARCHITECTURE}.tar.gz" "${HELM_FILE}"
download "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/v${KUSTOMIZE_VERSION}/kustomize_v${KUSTOMIZE_VERSION}_${INSTALL_OS}_${ARCHITECTURE}.tar.gz" "${KUSTOMIZE_FILE}"
download "https://github.com/git-lfs/git-lfs/releases/download/v${GIT_LFS_VERSION}/${GIT_LFS_FILE}" "${GIT_LFS_FILE}"

echo ""
echo "Done. Files in ${DOWNLOADS}:"
ls -lh "${DOWNLOADS}"
