#! /usr/bin/env bash
set -x
set -o errexit
set -o nounset
set -o pipefail

SRCROOT="$( CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P )"
AUTOGENMSG="# This is an auto-generated file. DO NOT EDIT"

KUSTOMIZE=kustomize
[ -f "$SRCROOT/dist/kustomize" ] && KUSTOMIZE="$SRCROOT/dist/kustomize"
# Really need advanced Yaml tools to fix https://github.com/argoproj/argo-cd/issues/20532.
PYTHON=python3
NORMALIZER="$SRCROOT/hack/update-manifests-normalizer.py"
TEMP_FILE=/tmp/argocd-normalized-manifest.yaml

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

$KUSTOMIZE version
which $KUSTOMIZE

cd ${SRCROOT}/manifests/base && $KUSTOMIZE edit set image quay.io/argoproj/argocd=${IMAGE_NAMESPACE}/argocd:${IMAGE_TAG}
cd ${SRCROOT}/manifests/ha/base && $KUSTOMIZE edit set image quay.io/argoproj/argocd=${IMAGE_NAMESPACE}/argocd:${IMAGE_TAG}
cd ${SRCROOT}/manifests/core-install && $KUSTOMIZE edit set image quay.io/argoproj/argocd=${IMAGE_NAMESPACE}/argocd:${IMAGE_TAG}

OUTPUT_FILE="${SRCROOT}/manifests/install.yaml"
$KUSTOMIZE build "${SRCROOT}/manifests/cluster-install" > $OUTPUT_FILE
$PYTHON $NORMALIZER $OUTPUT_FILE $TEMP_FILE
# Avoid printing the whole file contents
set +x
echo -e "${AUTOGENMSG}\n$(cat $TEMP_FILE)" > $OUTPUT_FILE
set -x

OUTPUT_FILE="${SRCROOT}/manifests/namespace-install.yaml"
$KUSTOMIZE build "${SRCROOT}/manifests/namespace-install" > $OUTPUT_FILE
$PYTHON $NORMALIZER $OUTPUT_FILE $TEMP_FILE
set +x
echo -e "${AUTOGENMSG}\n$(cat $TEMP_FILE)" > $OUTPUT_FILE
set -x

OUTPUT_FILE="${SRCROOT}/manifests/ha/install.yaml"
$KUSTOMIZE build "${SRCROOT}/manifests/ha/cluster-install" > $OUTPUT_FILE
$PYTHON $NORMALIZER $OUTPUT_FILE $TEMP_FILE
set +x
echo -e "${AUTOGENMSG}\n$(cat $TEMP_FILE)" > $OUTPUT_FILE
set -x

OUTPUT_FILE="${SRCROOT}/manifests/ha/namespace-install.yaml"
$KUSTOMIZE build "${SRCROOT}/manifests/ha/namespace-install" > $OUTPUT_FILE
$PYTHON $NORMALIZER $OUTPUT_FILE $TEMP_FILE
set +x
echo -e "${AUTOGENMSG}\n$(cat $TEMP_FILE)" > $OUTPUT_FILE
set -x

OUTPUT_FILE="${SRCROOT}/manifests/core-install.yaml"
$KUSTOMIZE build "${SRCROOT}/manifests/core-install" > $OUTPUT_FILE
$PYTHON $NORMALIZER $OUTPUT_FILE $TEMP_FILE
set +x
echo -e "${AUTOGENMSG}\n$(cat $TEMP_FILE)" > $OUTPUT_FILE
set -x
