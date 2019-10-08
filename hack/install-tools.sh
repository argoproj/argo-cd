#!/bin/bash
set -eux -o pipefail

mkdir -p /tmp/dl

install_kubectl() {
  set -eux -o pipefail
  [ -e /tmp/dl/kubectl ] || curl -sLf -C - -o /tmp/dl/kubectl https://storage.googleapis.com/kubernetes-release/release/v1.14.0/bin/linux/amd64/kubectl
  cp /tmp/dl/kubectl $BIN/kubectl
  chmod +x $BIN/kubectl
}

install_kubectx() {
  set -eux -o pipefail
  [ -e /tmp/dl/kubectx.zip ] || curl -sLf -C - -o /tmp/dl/kubectx.zip https://github.com/ahmetb/kubectx/archive/v0.6.3.zip
  unzip /tmp/dl/kubectx.zip kubectx-0.6.3/kubectx -d /tmp/dl
  unzip /tmp/dl/kubectx.zip kubectx-0.6.3/kubens -d /tmp/dl
  mv /tmp/dl/kubectx-0.6.3/kubectx $BIN/
  mv /tmp/dl/kubectx-0.6.3/kubens $BIN/
  chmod +x $BIN/kubectx
  chmod +x $BIN/kubens
}

install_dep() {
  set -eux -o pipefail
  [ -e /tmp/dl/dep ] || curl -sLf -C - -o /tmp/dl/dep https://github.com/golang/dep/releases/download/v0.5.3/dep-linux-amd64
  cp /tmp/dl/dep $BIN/dep
  chmod +x $BIN/dep
  dep version
}

install_ksonnet() {
  set -eux -o pipefail
  [ -e /tmp/dl/ks.tar.gz ] || curl -sLf -C - -o /tmp/dl/ks.tar.gz https://github.com/ksonnet/ksonnet/releases/download/v0.13.1/ks_0.13.1_linux_amd64.tar.gz
  tar -C /tmp -xf /tmp/dl/ks.tar.gz
  cp /tmp/ks_0.13.1_linux_amd64/ks $BIN/ks
  chmod +x $BIN/ks
  ks version
}

install_helm() {
  set -eux -o pipefail
  [ -e /tmp/dl/helm.tar.gz ] || curl -sLf -C - -o /tmp/dl/helm.tar.gz https://storage.googleapis.com/kubernetes-helm/helm-v2.13.1-linux-amd64.tar.gz
  tar -C /tmp/ -xf /tmp/dl/helm.tar.gz
  cp /tmp/linux-amd64/helm $BIN/helm
  helm version --client
  helm init --client-only
}

install_kustomize() {
  set -eux -o pipefail
  export VER=3.1.0
  [ -e /tmp/dl/kustomize_${VER} ] || curl -sLf -C - -o /tmp/dl/kustomize_${VER} https://github.com/kubernetes-sigs/kustomize/releases/download/v${VER}/kustomize_${VER}_linux_amd64
  cp /tmp/dl/kustomize_${VER} $BIN/kustomize
  chmod +x $BIN/kustomize
  kustomize version
}

install_kubectl
install_kubectx
install_dep
install_ksonnet
install_helm
install_kustomize
