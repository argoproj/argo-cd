#! /usr/bin/env bash
set -x
set -o errexit
set -o nounset
set -o pipefail

SRCROOT="$( CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P )"
AUTOGENMSG="# This is an auto-generated file. DO NOT EDIT"

KUSTOMIZE=kustomize
[ -f "$SRCROOT/dist/kustomize" ] && KUSTOMIZE="$SRCROOT/dist/kustomize"

cd "${SRCROOT}/manifests/ha/base/redis-ha" && ./generate.sh

# Empty defaults - to avoid errors for unset env. variables
IMAGE_REGISTRY="${IMAGE_REGISTRY:-}"
IMAGE_NAMESPACE="${IMAGE_NAMESPACE:-}"
IMAGE_TAG="${IMAGE_TAG:-}"
# Image repository configuration - can be overridden in forks
IMAGE_REPOSITORY="${IMAGE_REPOSITORY:-argocd}"

# Apply defaults if needed
if [[ -n $IMAGE_REGISTRY ]];then
    if [[ -z $IMAGE_NAMESPACE ]]; then
	echo "IMAGE_NAMESPACE must be set when IMAGE_REGISTRY is set (e.g. IMAGE_NAMESPACE=argoproj)" >&2
	exit 1
    fi
    # both registry and namespace set, nothing to do
else  # registry not set
    if [[ -z $IMAGE_NAMESPACE ]]; then
	# Neither namespace nor registry given - apply the default values
	IMAGE_REGISTRY="${IMAGE_REGISTRY:-quay.io}"
	IMAGE_NAMESPACE="${IMAGE_NAMESPACE:-argoproj}"
    fi
    # If namespace is set, then it's an image without registry or
    # registry is given as part of namespace (old convention)
fi

# Construct full image name
# Note: keeping same logic as in Makefile for docker images
FULL_IMAGE_NAME="${IMAGE_REPOSITORY}"
if [[ -n $IMAGE_NAMESPACE ]]; then
    FULL_IMAGE_NAME="${IMAGE_NAMESPACE}/${FULL_IMAGE_NAME}"
fi
if [[ -n $IMAGE_REGISTRY ]]; then
    FULL_IMAGE_NAME="${IMAGE_REGISTRY}/${FULL_IMAGE_NAME}"
fi

# Auto-detect current image in manifests for release workflows
detect_current_image() {
  local manifest_file="$1"
  if [ -f "$manifest_file" ]; then
    # Look for the current image name in kustomization.yaml images section
    awk '/^images:/,/^[a-zA-Z]/ { if (/- name:/ && /argocd/) { gsub(/.*name: */, ""); gsub(/ *$/, ""); print; exit } }' "$manifest_file"
  fi
}

# Determine source image (what to replace)
DETECTED_IMAGE=$(detect_current_image "${SRCROOT}/manifests/base/kustomization.yaml")
if [ -n "$DETECTED_IMAGE" ] && [ "$DETECTED_IMAGE" != "quay.io/argoproj/argocd" ]; then
  # Found a custom image in manifests (subsequent release scenario)
  SOURCE_IMAGE_NAME="$DETECTED_IMAGE"
  echo "Detected existing custom image in manifests: $SOURCE_IMAGE_NAME"
else
  # Use default source image (fresh fork or manual override)
  SOURCE_IMAGE_NAME="quay.io/argoproj/argocd"
  echo "Using default source image: $SOURCE_IMAGE_NAME"
fi

# if the tag has not been declared, and we are on a release branch, use the VERSION file.
if [ "$IMAGE_TAG" = "" ]; then
  branch=$(git rev-parse --abbrev-ref HEAD)
  # In GitHub Actions PRs, HEAD is detached; use GITHUB_BASE_REF (the target branch) instead
  if [ "$branch" = "HEAD" ] && [ -n "${GITHUB_BASE_REF:-}" ]; then
    branch="$GITHUB_BASE_REF"
  fi
  if [[ $branch = release-* ]]; then
    pwd
    IMAGE_TAG=v$(cat "$SRCROOT/VERSION")
  fi
fi
# otherwise, use latest
if [ "$IMAGE_TAG" = "" ]; then
  IMAGE_TAG=latest
fi

$KUSTOMIZE version
which "$KUSTOMIZE"

echo "=== Manifest Generation Configuration ==="
echo "Source image (to replace): ${SOURCE_IMAGE_NAME}"
echo "Target image (replace with): ${FULL_IMAGE_NAME}:${IMAGE_TAG}"
if [ "$DETECTED_IMAGE" != "quay.io/argoproj/argocd" ] && [ -n "$DETECTED_IMAGE" ]; then
  echo "Scenario: Subsequent release (updating existing custom image)"
else
  echo "Scenario: First release or local development"
fi
echo "========================================"

cd "${SRCROOT}/manifests/base" && $KUSTOMIZE edit set image "${SOURCE_IMAGE_NAME}=${FULL_IMAGE_NAME}:${IMAGE_TAG}"
cd "${SRCROOT}/manifests/ha/base" && $KUSTOMIZE edit set image "${SOURCE_IMAGE_NAME}=${FULL_IMAGE_NAME}:${IMAGE_TAG}"
cd "${SRCROOT}/manifests/core-install" && $KUSTOMIZE edit set image "${SOURCE_IMAGE_NAME}=${FULL_IMAGE_NAME}:${IMAGE_TAG}"

# Because commit-server is added as a resource outside the base, we have to explicitly set the image override here.
# If/when commit-server is added to the base, this can be removed.
cd "${SRCROOT}/manifests/base/commit-server" && $KUSTOMIZE edit set image "${SOURCE_IMAGE_NAME}=${FULL_IMAGE_NAME}:${IMAGE_TAG}"

echo "${AUTOGENMSG}" > "${SRCROOT}/manifests/install.yaml"
$KUSTOMIZE build "${SRCROOT}/manifests/cluster-install" >> "${SRCROOT}/manifests/install.yaml"

echo "${AUTOGENMSG}" > "${SRCROOT}/manifests/namespace-install.yaml"
$KUSTOMIZE build "${SRCROOT}/manifests/namespace-install" >> "${SRCROOT}/manifests/namespace-install.yaml"

echo "${AUTOGENMSG}" > "${SRCROOT}/manifests/ha/install.yaml"
$KUSTOMIZE build "${SRCROOT}/manifests/ha/cluster-install" >> "${SRCROOT}/manifests/ha/install.yaml"

echo "${AUTOGENMSG}" > "${SRCROOT}/manifests/ha/namespace-install.yaml"
$KUSTOMIZE build "${SRCROOT}/manifests/ha/namespace-install" >> "${SRCROOT}/manifests/ha/namespace-install.yaml"

echo "${AUTOGENMSG}" > "${SRCROOT}/manifests/core-install.yaml"
$KUSTOMIZE build "${SRCROOT}/manifests/core-install" >> "${SRCROOT}/manifests/core-install.yaml"

# Copies enabling manifest hydrator. These can be removed once the manifest hydrator is either removed or enabled by
# default.

echo "${AUTOGENMSG}" > "${SRCROOT}/manifests/install-with-hydrator.yaml"
$KUSTOMIZE build "${SRCROOT}/manifests/cluster-install-with-hydrator" >> "${SRCROOT}/manifests/install-with-hydrator.yaml"

echo "${AUTOGENMSG}" > "${SRCROOT}/manifests/namespace-install-with-hydrator.yaml"
$KUSTOMIZE build "${SRCROOT}/manifests/namespace-install-with-hydrator" >> "${SRCROOT}/manifests/namespace-install-with-hydrator.yaml"

echo "${AUTOGENMSG}" > "${SRCROOT}/manifests/ha/install-with-hydrator.yaml"
$KUSTOMIZE build "${SRCROOT}/manifests/ha/cluster-install-with-hydrator" >> "${SRCROOT}/manifests/ha/install-with-hydrator.yaml"

echo "${AUTOGENMSG}" > "${SRCROOT}/manifests/ha/namespace-install-with-hydrator.yaml"
$KUSTOMIZE build "${SRCROOT}/manifests/ha/namespace-install-with-hydrator" >> "${SRCROOT}/manifests/ha/namespace-install-with-hydrator.yaml"

echo "${AUTOGENMSG}" > "${SRCROOT}/manifests/core-install-with-hydrator.yaml"
$KUSTOMIZE build "${SRCROOT}/manifests/core-install-with-hydrator" >> "${SRCROOT}/manifests/core-install-with-hydrator.yaml"
