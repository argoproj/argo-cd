#!/bin/bash
set -eux -o pipefail

. $(dirname $0)/../tool-versions.sh

[ -e $DOWNLOADS/kubectx.zip ] || curl -sLf --retry 3 -o $DOWNLOADS/kubectx.zip https://github.com/ahmetb/kubectx/archive/v${kubectx_version}.zip
unzip $DOWNLOADS/kubectx.zip kubectx-${kubectx_version}/kubectx -d $DOWNLOADS
unzip $DOWNLOADS/kubectx.zip kubectx-${kubectx_version}/kubens -d $DOWNLOADS
mv $DOWNLOADS/kubectx-${kubectx_version}/kubectx $BIN/
mv $DOWNLOADS/kubectx-${kubectx_version}/kubens $BIN/
chmod +x $BIN/kubectx
chmod +x $BIN/kubens
