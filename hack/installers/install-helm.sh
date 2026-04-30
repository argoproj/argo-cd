#!/bin/bash
set -eux -o pipefail

. "$(dirname "$0")"/../tool-versions.sh

# Determine which Helm version to install (default to v3)
HELM_VERSION=${HELM_VERSION:-3}

if [ "$HELM_VERSION" = "3" ]; then
	TARGET_FILE=helm-v${helm3_version}-${INSTALL_OS}-${ARCHITECTURE}.tar.gz
	DOWNLOAD_URL="https://get.helm.sh/helm-v${helm3_version}-$INSTALL_OS-$ARCHITECTURE.tar.gz"
elif [ "$HELM_VERSION" = "4" ]; then
	TARGET_FILE=helm-v${helm4_version}-${INSTALL_OS}-${ARCHITECTURE}.tar.gz
	DOWNLOAD_URL="https://get.helm.sh/helm-v${helm4_version}-$INSTALL_OS-$ARCHITECTURE.tar.gz"
else
	echo "Error: HELM_VERSION must be 3 or 4, got $HELM_VERSION"
	exit 1
fi

[ -e "$DOWNLOADS/${TARGET_FILE}" ] || curl -sLf --retry 3 -o "$DOWNLOADS/${TARGET_FILE}" "$DOWNLOAD_URL"
"$(dirname "$0")"/compare-chksum.sh
mkdir -p /tmp/helm && tar -C /tmp/helm -xf "$DOWNLOADS/${TARGET_FILE}"
sudo install -m 0755 "/tmp/helm/$INSTALL_OS-$ARCHITECTURE/helm" "$BIN/helm"
helm version

