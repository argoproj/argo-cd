#! /usr/bin/env bash
set -x
set -o errexit
set -o nounset
set -o pipefail

KUSTOMIZE=kustomize

SRCROOT="$( CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P )"
AUTOGENMSG="# This is an auto-generated file. DO NOT EDIT"

cd ${SRCROOT}/manifests/ha/base/redis-ha && ./generate.sh

IMAGE_NAMESPACE="${IMAGE_NAMESPACE:-quay.io/argoproj}"
IMAGE_TAG="${IMAGE_TAG:-}"

# if the tag has not been declared, and we are on a release branch, use the VERSION file.
if [ "$IMAGE_TAG" = "" ]; then
  branch=$(git rev-parse --abbrev-ref HEAD)
  if [[ $branch = release-* ]]; then
    pwd
    IMAGE_TAG=v$(cat $SRCROOT/VERSION)
  fi
fi
# otherwise, use latest
if [ "$IMAGE_TAG" = "" ]; then
  IMAGE_TAG=latest
fi

# bundle_with_addons bundles given kustomize base with either stable or latest version of addons
function bundle_with_addons() {
  for addon in $(ls $SRCROOT/manifests/addons | grep -v README.md); do
    ADDON_BASE="latest"
    branch=$(git rev-parse --abbrev-ref HEAD)
    if [[ $branch = release-* ]]; then
        ADDON_BASE="stable"
    fi
    rm -rf $SRCROOT/manifests/_tmp-bundle && mkdir -p $SRCROOT/manifests/_tmp-bundle
    cat << EOF >> $SRCROOT/manifests/_tmp-bundle/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- ../$1
- ../addons/$addon/$ADDON_BASE
EOF
    echo "${AUTOGENMSG}" > $2
    $KUSTOMIZE build $SRCROOT/manifests/_tmp-bundle >> $2
  done
}

$KUSTOMIZE version

cd ${SRCROOT}/manifests/base && $KUSTOMIZE edit set image quay.io/argoproj/argocd=${IMAGE_NAMESPACE}/argocd:${IMAGE_TAG}
cd ${SRCROOT}/manifests/ha/base && $KUSTOMIZE edit set image quay.io/argoproj/argocd=${IMAGE_NAMESPACE}/argocd:${IMAGE_TAG}
cd ${SRCROOT}/manifests/core-install && $KUSTOMIZE edit set image quay.io/argoproj/argocd=${IMAGE_NAMESPACE}/argocd:${IMAGE_TAG}

bundle_with_addons "cluster-install" "${SRCROOT}/manifests/install.yaml"
bundle_with_addons "namespace-install" "${SRCROOT}/manifests/namespace-install.yaml"
bundle_with_addons "ha/cluster-install" "${SRCROOT}/manifests/ha/install.yaml"
bundle_with_addons "ha/namespace-install" "${SRCROOT}/manifests/ha/namespace-install.yaml"
bundle_with_addons "core-install" "${SRCROOT}/manifests/core-install.yaml"
