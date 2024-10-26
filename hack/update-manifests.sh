#! /usr/bin/env bash
set -x
set -o errexit
set -o nounset
set -o pipefail

SRCROOT="$( CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P )"

KUSTOMIZE=kustomize
[ -f "$SRCROOT/dist/kustomize" ] && KUSTOMIZE="$SRCROOT/dist/kustomize"

HACK_IMAGEPULLPOLICY="${SRCROOT}/hack/update-manifests-imagepullpolicy"

cd ${SRCROOT}/manifests/ha/base/redis-ha && ./generate.sh

IMAGE_NAMESPACE="${IMAGE_NAMESPACE:-quay.io/argoproj}"
IMAGE_TAG="${IMAGE_TAG:-}"
IMAGE_PULL_POLICY="${IMAGE_PULL_POLICY:-Always}"

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

$KUSTOMIZE version
which $KUSTOMIZE

cd ${SRCROOT}/manifests/base && $KUSTOMIZE edit set image quay.io/argoproj/argocd=${IMAGE_NAMESPACE}/argocd:${IMAGE_TAG}
cd ${SRCROOT}/manifests/ha/base && $KUSTOMIZE edit set image quay.io/argoproj/argocd=${IMAGE_NAMESPACE}/argocd:${IMAGE_TAG}
cd ${SRCROOT}/manifests/core-install && $KUSTOMIZE edit set image quay.io/argoproj/argocd=${IMAGE_NAMESPACE}/argocd:${IMAGE_TAG}

echo -n "" > "${SRCROOT}/manifests/install.yaml"
$KUSTOMIZE build "${SRCROOT}/manifests/cluster-install" >> "${SRCROOT}/manifests/install.yaml"
go run $HACK_IMAGEPULLPOLICY -manifest=${SRCROOT}/manifests/install.yaml -image=${IMAGE_NAMESPACE}/argocd:${IMAGE_TAG} -image-pull-policy=$IMAGE_PULL_POLICY

echo -n "" > "${SRCROOT}/manifests/namespace-install.yaml"
$KUSTOMIZE build "${SRCROOT}/manifests/namespace-install" >> "${SRCROOT}/manifests/namespace-install.yaml"
go run $HACK_IMAGEPULLPOLICY -manifest=${SRCROOT}/manifests/namespace-install.yaml -image=${IMAGE_NAMESPACE}/argocd:${IMAGE_TAG} -image-pull-policy=$IMAGE_PULL_POLICY

echo -n "" > "${SRCROOT}/manifests/ha/install.yaml"
$KUSTOMIZE build "${SRCROOT}/manifests/ha/cluster-install" >> "${SRCROOT}/manifests/ha/install.yaml"
go run $HACK_IMAGEPULLPOLICY -manifest=${SRCROOT}/manifests/ha/install.yaml -image=${IMAGE_NAMESPACE}/argocd:${IMAGE_TAG} -image-pull-policy=$IMAGE_PULL_POLICY

echo -n "" > "${SRCROOT}/manifests/ha/namespace-install.yaml"
$KUSTOMIZE build "${SRCROOT}/manifests/ha/namespace-install" >> "${SRCROOT}/manifests/ha/namespace-install.yaml"
go run $HACK_IMAGEPULLPOLICY -manifest=${SRCROOT}/manifests/ha/namespace-install.yaml -image=${IMAGE_NAMESPACE}/argocd:${IMAGE_TAG} -image-pull-policy=$IMAGE_PULL_POLICY

echo -n "" > "${SRCROOT}/manifests/core-install.yaml"
$KUSTOMIZE build "${SRCROOT}/manifests/core-install" >> "${SRCROOT}/manifests/core-install.yaml"
go run $HACK_IMAGEPULLPOLICY -manifest=${SRCROOT}/manifests/core-install.yaml -image=${IMAGE_NAMESPACE}/argocd:${IMAGE_TAG} -image-pull-policy=$IMAGE_PULL_POLICY
