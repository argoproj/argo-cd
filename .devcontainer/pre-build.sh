#!/usr/bin/env sh
set -eux

# install kubernetes
wget -q -O - https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash
k3d cluster get k3s-default || k3d cluster create --wait
k3d kubeconfig merge --kubeconfig-merge-default

# install kubectl
curl -LO https://dl.k8s.io/release/v1.26.0/bin/linux/amd64/kubectl
chmod +x ./kubectl
sudo mv ./kubectl /usr/local/bin/kubectl
kubectl cluster-info

# install kit
curl -q https://raw.githubusercontent.com/kitproj/kit/main/install.sh | sh

# download dependencies and do first-pass compile
CI=1 kit pre-up
