PWD=$(shell pwd)
TEST_ROOT=$(shell realpath $(PWD)/../..)

IMAGE_NAMESPACE?=
IMAGE_NAME?=argocd-e2e-cluster
IMAGE_TAG?=latest
ifneq (${IMAGE_NAMESPACE},)
IMAGE_PREFIX=$(IMAGE_NAMESPACE)/
else
IMAGE_PREFIX=
endif

.PHONY: image
image:
	docker build -t $(IMAGE_PREFIX)$(IMAGE_NAME):$(IMAGE_TAG) -f Dockerfile $(TEST_ROOT)

.PHONY: image-push
image-push: image
	docker push $(IMAGE_PREFIX)$(IMAGE_NAME):$(IMAGE_TAG)

.PHONY: manifests
manifests:
	@cd manifests && \
		kustomize edit set image argocd-e2e-cluster=$(IMAGE_PREFIX)$(IMAGE_NAME):$(IMAGE_TAG) && \
		kustomize build .
