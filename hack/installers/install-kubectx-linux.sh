#!/bin/bash
set -eux -o pipefail

[ -e $DOWNLOADS/kubectx.zip ] || curl -sLf --retry 3 -o $DOWNLOADS/kubectx.zip https://github.com/ahmetb/kubectx/archive/v0.6.3.zip
unzip $DOWNLOADS/kubectx.zip kubectx-0.6.3/kubectx -d $DOWNLOADS
unzip $DOWNLOADS/kubectx.zip kubectx-0.6.3/kubens -d $DOWNLOADS
sudo mv $DOWNLOADS/kubectx-0.6.3/kubectx $BIN/
sudo mv $DOWNLOADS/kubectx-0.6.3/kubens $BIN/
sudo chmod +x $BIN/kubectx
sudo chmod +x $BIN/kubens
