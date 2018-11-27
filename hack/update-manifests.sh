#!/bin/sh

set -e

SRCROOT="$( CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P )"
AUTOGENMSG="# This is an auto-generated file. DO NOT EDIT"

update_image () {
  if [ ! -z "${IMAGE_NAMESPACE}" ]; then
    sed 's| image: \(.*\)/\(argocd.*\)| image: '"${IMAGE_NAMESPACE}"'/\2|g' "${1}" > "${1}.bak"
    mv "${1}.bak" "${1}"
  fi
}

if [ ! -z "${IMAGE_TAG}" ]; then
  (cd ${SRCROOT}/manifests/base && kustomize edit set imagetag argoproj/argocd:${IMAGE_TAG} argoproj/argocd-ui:${IMAGE_TAG})
fi

echo "${AUTOGENMSG}" > "${SRCROOT}/manifests/install.yaml"
kustomize build "${SRCROOT}/manifests/cluster-install" >> "${SRCROOT}/manifests/install.yaml"
update_image "${SRCROOT}/manifests/install.yaml"

echo "${AUTOGENMSG}" > "${SRCROOT}/manifests/namespace-install.yaml"
kustomize build "${SRCROOT}/manifests/namespace-install" >> "${SRCROOT}/manifests/namespace-install.yaml"
update_image "${SRCROOT}/manifests/namespace-install.yaml"
