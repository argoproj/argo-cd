#!/usr/bin/env sh
set -eux

wget -q -O - https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash
k3d cluster get k3s-default || k3d cluster create --wait
k3d kubeconfig merge --kubeconfig-merge-default

curl -q https://raw.githubusercontent.com/alexec/kit/main/install.sh | sh
