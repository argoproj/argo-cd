#!/bin/bash
set -eux -o pipefail

. $(dirname $0)/../tool-versions.sh

export TARGET_FILE=kubectx-${kubectx_version}.zip

[ -e $DOWNLOADS/${TARGET_FILE} ] || curl -sLf --retry 3 -o $DOWNLOADS/${TARGET_FILE} https://github.com/ahmetb/kubectx/archive/v${kubectx_version}.zip
$(dirname $0)/compare-chksum.sh
unzip $DOWNLOADS/${TARGET_FILE} kubectx-${kubectx_version}/kubectx -d $DOWNLOADS
unzip $DOWNLOADS/${TARGET_FILE} kubectx-${kubectx_version}/kubens -d $DOWNLOADS
sudo install -m 0755 $DOWNLOADS/kubectx-${kubectx_version}/kubectx $BIN/kubectx
sudo install -m 0755 $DOWNLOADS/kubectx-${kubectx_version}/kubens $BIN/kubens