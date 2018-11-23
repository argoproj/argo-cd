#!/bin/sh

set -e

SRCROOT="$( CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P )"
AUTOGENMSG="# This is an auto-generated file. DO NOT EDIT"

update_image () {
  if [ ! -z "${IMAGE_NAMESPACE}" ]; then
    sed -i -- 's| image: \(.*\)/\(argocd.*\)| image: '"${IMAGE_NAMESPACE}"'/\2|g' "${1}"
  fi
  if [ ! -z "${IMAGE_TAG}" ]; then
    sed -i -- 's|\( image: .*/argocd.*\)\:.*|\1:'"${IMAGE_TAG}"'|g' "${1}"
  fi
}

echo "${AUTOGENMSG}" > "${SRCROOT}/manifests/install.yaml"
kustomize build "${SRCROOT}/manifests/cluster-install" >> "${SRCROOT}/manifests/install.yaml"
update_image "${SRCROOT}/manifests/install.yaml"

echo "${AUTOGENMSG}" > "${SRCROOT}/manifests/namespace-install.yaml"
kustomize build "${SRCROOT}/manifests/namespace-install" >> "${SRCROOT}/manifests/namespace-install.yaml"
update_image "${SRCROOT}/manifests/namespace-install.yaml"
