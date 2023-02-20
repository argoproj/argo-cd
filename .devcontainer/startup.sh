#!/usr/bin/env sh
set -eux
wget -q -O - https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash
k3d cluster create --wait