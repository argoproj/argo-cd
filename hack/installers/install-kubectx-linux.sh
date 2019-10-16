#!/bin/bash
set -eux -o pipefail

[ -e $DOWNLOADS/kubectx.zip ] || curl -sLf --retry 3 -o $DOWNLOADS/kubectx.zip https://github.com/ahmetb/kubectx/archive/v0.6.3.zip
unzip $DOWNLOADS/kubectx.zip kubectx-0.6.3/kubectx -d $DOWNLOADS
unzip $DOWNLOADS/kubectx.zip kubectx-0.6.3/kubens -d $DOWNLOADS
mv $DOWNLOADS/kubectx-0.6.3/kubectx $BIN/
mv $DOWNLOADS/kubectx-0.6.3/kubens $BIN/
chmod +x $BIN/kubectx
chmod +x $BIN/kubens
