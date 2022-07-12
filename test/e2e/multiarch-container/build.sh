#!/bin/sh
set -x
VERSIONS="0.1 0.2 0.3"
PLATFORMS="linux/amd64,linux/arm64,linux/s390x,linux/ppc64le"
for version in $VERSIONS; do
	docker buildx build \
		-t "quay.io/argoprojlabs/argocd-e2e-container:${version}" \
		--platform "${PLATFORMS}" \
		--push \
		.
done
