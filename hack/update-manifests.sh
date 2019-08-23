#! /usr/bin/env bash
set -x
set -o errexit
set -o nounset
set -o pipefail

SRCROOT="$( CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P )"
AUTOGENMSG="# This is an auto-generated file. DO NOT EDIT"

cd ${SRCROOT}/manifests/ha/base/redis-ha && ./generate.sh

IMAGE_NAMESPACE="${IMAGE_NAMESPACE:-argoproj}"
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

cd ${SRCROOT}/manifests/base && kustomize edit set image argoproj/argocd=${IMAGE_NAMESPACE}/argocd:${IMAGE_TAG} argoproj/argocd-ui=${IMAGE_NAMESPACE}/argocd-ui:${IMAGE_TAG}
cd ${SRCROOT}/manifests/ha/base && kustomize edit set image argoproj/argocd=${IMAGE_NAMESPACE}/argocd:${IMAGE_TAG} argoproj/argocd-ui=${IMAGE_NAMESPACE}/argocd-ui:${IMAGE_TAG}

echo "${AUTOGENMSG}" > "${SRCROOT}/manifests/install.yaml"
kustomize build "${SRCROOT}/manifests/cluster-install" >> "${SRCROOT}/manifests/install.yaml"

echo "${AUTOGENMSG}" > "${SRCROOT}/manifests/namespace-install.yaml"
kustomize build "${SRCROOT}/manifests/namespace-install" >> "${SRCROOT}/manifests/namespace-install.yaml"

echo "${AUTOGENMSG}" > "${SRCROOT}/manifests/ha/install.yaml"
kustomize build "${SRCROOT}/manifests/ha/cluster-install" >> "${SRCROOT}/manifests/ha/install.yaml"

echo "${AUTOGENMSG}" > "${SRCROOT}/manifests/ha/namespace-install.yaml"
kustomize build "${SRCROOT}/manifests/ha/namespace-install" >> "${SRCROOT}/manifests/ha/namespace-install.yaml"

