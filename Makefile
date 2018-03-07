PACKAGE=github.com/argoproj/argo-cd
CURRENT_DIR=$(shell pwd)
DIST_DIR=${CURRENT_DIR}/dist

VERSION=$(shell cat ${CURRENT_DIR}/VERSION)
BUILD_DATE=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT=$(shell git rev-parse HEAD)
GIT_TAG=$(shell if [ -z "`git status --porcelain`" ]; then git describe --exact-match --tags HEAD 2>/dev/null; fi)
GIT_TREE_STATE=$(shell if [ -z "`git status --porcelain`" ]; then echo "clean" ; else echo "dirty"; fi)

override LDFLAGS += \
  -X ${PACKAGE}.version=${VERSION} \
  -X ${PACKAGE}.buildDate=${BUILD_DATE} \
  -X ${PACKAGE}.gitCommit=${GIT_COMMIT} \
  -X ${PACKAGE}.gitTreeState=${GIT_TREE_STATE}

# docker image publishing options
DOCKER_PUSH=false
IMAGE_TAG=latest
ifneq (${GIT_TAG},)
IMAGE_TAG=${GIT_TAG}
LDFLAGS += -X ${PACKAGE}.gitTag=${GIT_TAG}
endif
ifneq (${IMAGE_NAMESPACE},)
override LDFLAGS += -X ${PACKAGE}/cmd/argocd/commands.imageNamespace=${IMAGE_NAMESPACE}
endif
ifneq (${IMAGE_TAG},)
override LDFLAGS += -X ${PACKAGE}/cmd/argocd/commands.imageTag=${IMAGE_TAG}
endif

ifeq (${DOCKER_PUSH},true)
ifndef IMAGE_NAMESPACE
$(error IMAGE_NAMESPACE must be set to push images (e.g. IMAGE_NAMESPACE=argoproj))
endif
endif

ifdef IMAGE_NAMESPACE
IMAGE_PREFIX=${IMAGE_NAMESPACE}/
endif

.PHONY: all
all: cli server-image controller-image repo-server-image

.PHONY: protogen
protogen:
	./hack/generate-proto.sh

.PHONY: clientgen
clientgen:
	./hack/update-codegen.sh

.PHONY: codegen
codegen: protogen clientgen

# NOTE: we use packr to do the build instead of go, since we embed .yaml files into the go binary.
# This enables ease of maintenance of the yaml files.
.PHONY: cli
cli:
	CGO_ENABLED=0 go run vendor/github.com/gobuffalo/packr/packr/main.go build -v -i -ldflags '${LDFLAGS} -extldflags "-static"' -o ${DIST_DIR}/argocd ./cmd/argocd

.PHONY: server
server:
	CGO_ENABLED=0 go build -v -i -ldflags '${LDFLAGS}' -o ${DIST_DIR}/argocd-server ./cmd/argocd-server

.PHONY: server-image
server-image:
	docker build --build-arg BINARY=argocd-server --build-arg MAKE_TARGET=server -t $(IMAGE_PREFIX)argocd-server:$(IMAGE_TAG) -f Dockerfile-argocd .
	@if [ "$(DOCKER_PUSH)" = "true" ] ; then docker push $(IMAGE_PREFIX)argocd-server:$(IMAGE_TAG) ; fi

.PHONY: repo-server
repo-server:
	CGO_ENABLED=0 go build -v -i -ldflags '${LDFLAGS}' -o ${DIST_DIR}/argocd-repo-server ./cmd/argocd-repo-server

.PHONY: repo-server-image
repo-server-image:
	docker build --build-arg BINARY=argocd-repo-server --build-arg MAKE_TARGET=repo-server -t $(IMAGE_PREFIX)argocd-repo-server:$(IMAGE_TAG) -f Dockerfile-argocd .
	@if [ "$(DOCKER_PUSH)" = "true" ] ; then docker push $(IMAGE_PREFIX)argocd-server:$(IMAGE_TAG) ; fi

.PHONY: controller
controller:
	CGO_ENABLED=0 go build -v -i -ldflags '${LDFLAGS}' -o ${DIST_DIR}/argocd-application-controller ./cmd/argocd-application-controller

.PHONY: controller-image
controller-image:
	docker build --build-arg BINARY=argocd-application-controller --build-arg MAKE_TARGET=controller -t $(IMAGE_PREFIX)argocd-application-controller:$(IMAGE_TAG) -f Dockerfile-argocd .
	@if [ "$(DOCKER_PUSH)" = "true" ] ; then docker push $(IMAGE_PREFIX)argocd-application-controller:$(IMAGE_TAG) ; fi

.PHONY: builder-image
builder-image:
	docker build  -t $(IMAGE_PREFIX)argo-cd-ci-builder:$(IMAGE_TAG) -f Dockerfile-ci-builder .

.PHONY: lint
lint:
	gometalinter.v2 --config gometalinter.json ./...

.PHONY: test
test:
	go test ./...

.PHONY: clean
clean:
	-rm -rf ${CURRENT_DIR}/dist

.PHONY: precheckin
precheckin: test lint

