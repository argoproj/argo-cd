#!/usr/bin/env bash

set -e
set -o pipefail

images=$(grep 'image: ' manifests/install.yaml manifests/namespace-install.yaml manifests/ha/install.yaml | sed 's/.*image: //' | sort | uniq)

failed=false
while IFS= read -r image; do
  extra_args=""
  if echo "$image" | grep "argocd"; then
    # Pass the file arg only for the Argo CD image. The file arg also gives us access to sarif output.
    extra_args="--file=Dockerfile --sarif-file-output=/tmp/argocd-image.sarif"
  fi

  set -x
  if ! snyk container test "$image" --org=argoproj --severity-threshold=high $extra_args; then
    failed=true
  fi
  set +x

done <<< "$images"

if [ "$failed" == "true" ]; then
  exit 1
fi
