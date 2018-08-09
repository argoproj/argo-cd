PACKAGE=github.com/argoproj/argo-cd
CURRENT_DIR=$(shell pwd)
DIST_DIR=${CURRENT_DIR}/dist
CLI_NAME=argocd

VERSION=$(shell cat ${CURRENT_DIR}/VERSION)
BUILD_DATE=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT=$(shell git rev-parse HEAD)
GIT_TAG=$(shell if [ -z "`git status --porcelain`" ]; then git describe --exact-match --tags HEAD 2>/dev/null; fi)
GIT_TREE_STATE=$(shell if [ -z "`git status --porcelain`" ]; then echo "clean" ; else echo "dirty"; fi)
PACKR_CMD=$(shell if [ "`which packr`" ]; then echo "packr"; else echo "go run vendor/github.com/gobuffalo/packr/packr/main.go"; fi)

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
override LDFLAGS += -X ${PACKAGE}/install.imageNamespace=${IMAGE_NAMESPACE}
endif
ifneq (${IMAGE_TAG},)
override LDFLAGS += -X ${PACKAGE}/install.imageTag=${IMAGE_TAG}
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
all: cli server-image controller-image repo-server-image argocd-util

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
cli: clean-debug
	${PACKR_CMD} build -v -i -ldflags '${LDFLAGS} -extldflags "-static"' -o ${DIST_DIR}/${CLI_NAME} ./cmd/argocd

.PHONY: cli-linux
cli-linux: clean-debug
	docker build --iidfile /tmp/argocd-linux-id --target builder --build-arg MAKE_TARGET="cli IMAGE_TAG=$(IMAGE_TAG) IMAGE_NAMESPACE=$(IMAGE_NAMESPACE) CLI_NAME=argocd-linux-amd64" -f Dockerfile-argocd .
	docker create --name tmp-argocd-linux `cat /tmp/argocd-linux-id`
	docker cp tmp-argocd-linux:/root/go/src/github.com/argoproj/argo-cd/dist/argocd-linux-amd64 dist/
	docker rm tmp-argocd-linux

.PHONY: cli-darwin
cli-darwin: clean-debug
	docker build --iidfile /tmp/argocd-darwin-id --target builder --build-arg MAKE_TARGET="cli GOOS=darwin IMAGE_TAG=$(IMAGE_TAG) IMAGE_NAMESPACE=$(IMAGE_NAMESPACE) CLI_NAME=argocd-darwin-amd64" -f Dockerfile-argocd .
	docker create --name tmp-argocd-darwin `cat /tmp/argocd-darwin-id`
	docker cp tmp-argocd-darwin:/root/go/src/github.com/argoproj/argo-cd/dist/argocd-darwin-amd64 dist/
	docker rm tmp-argocd-darwin

.PHONY: argocd-util
argocd-util: clean-debug
	CGO_ENABLED=0 go build -v -i -ldflags '${LDFLAGS} -extldflags "-static"' -o ${DIST_DIR}/argocd-util ./cmd/argocd-util

.PHONY: install-manifest
install-manifest:
	if [ "${IMAGE_NAMESPACE}" = "" ] ; then echo "IMAGE_NAMESPACE must be set to build install manifest" ; exit 1 ; fi
	./hack/update-manifests.sh

.PHONY: server
server: clean-debug
	CGO_ENABLED=0 ${PACKR_CMD} build -v -i -ldflags '${LDFLAGS}' -o ${DIST_DIR}/argocd-server ./cmd/argocd-server
	
.PHONY: server-image
server-image:
	docker build --build-arg BINARY=argocd-server -t $(IMAGE_PREFIX)argocd-server:$(IMAGE_TAG) -f Dockerfile-argocd .
	@if [ "$(DOCKER_PUSH)" = "true" ] ; then docker push $(IMAGE_PREFIX)argocd-server:$(IMAGE_TAG) ; fi

.PHONY: repo-server
repo-server:
	CGO_ENABLED=0 go build -v -i -ldflags '${LDFLAGS}' -o ${DIST_DIR}/argocd-repo-server ./cmd/argocd-repo-server

.PHONY: repo-server-image
repo-server-image:
	docker build --build-arg BINARY=argocd-repo-server -t $(IMAGE_PREFIX)argocd-repo-server:$(IMAGE_TAG) -f Dockerfile-argocd .
	@if [ "$(DOCKER_PUSH)" = "true" ] ; then docker push $(IMAGE_PREFIX)argocd-repo-server:$(IMAGE_TAG) ; fi

.PHONY: controller
controller:
	CGO_ENABLED=0 go build -v -i -ldflags '${LDFLAGS}' -o ${DIST_DIR}/argocd-application-controller ./cmd/argocd-application-controller

.PHONY: controller-image
controller-image:
	docker build --build-arg BINARY=argocd-application-controller -t $(IMAGE_PREFIX)argocd-application-controller:$(IMAGE_TAG) -f Dockerfile-argocd .
	@if [ "$(DOCKER_PUSH)" = "true" ] ; then docker push $(IMAGE_PREFIX)argocd-application-controller:$(IMAGE_TAG) ; fi

.PHONY: cli-image
cli-image:
	docker build --build-arg BINARY=argocd -t $(IMAGE_PREFIX)argocd-cli:$(IMAGE_TAG) -f Dockerfile-argocd .
	@if [ "$(DOCKER_PUSH)" = "true" ] ; then docker push $(IMAGE_PREFIX)argocd-cli:$(IMAGE_TAG) ; fi

.PHONY: builder-image
builder-image:
	docker build  -t $(IMAGE_PREFIX)argo-cd-ci-builder:$(IMAGE_TAG) -f Dockerfile-ci-builder .

.PHONY: lint
lint:
	gometalinter.v2 --config gometalinter.json ./...

.PHONY: test
test:
	go test -v `go list ./... | grep -v "github.com/argoproj/argo-cd/test/e2e"`

.PHONY: test-coverage
test-coverage:
	go test -v -covermode=count -coverprofile=coverage.out `go list ./... | grep -v "github.com/argoproj/argo-cd/test/e2e"`
	echo CTOKEN=...$(COVERALLS_TOKEN)...
	echo CTOKEN2=...$(COVERALLS_TOKEN2)...
	@if [ "$(COVERALLS_TOKEN)" != "" ] ; then goveralls -coverprofile=coverage.out -service=argo-ci -repotoken "$(COVERALLS_TOKEN)"; fi

.PHONY: test-e2e
test-e2e:
	go test -v -failfast -timeout 20m ./test/e2e

# Cleans VSCode debug.test files from sub-dirs to prevent them from being included in packr boxes
.PHONY: clean-debug
clean-debug:
	-find ${CURRENT_DIR} -name debug.test | xargs rm -f

.PHONY: clean
clean: clean-debug
	-rm -rf ${CURRENT_DIR}/dist

.PHONY: precheckin
precheckin: test lint

.PHONY: release-precheck
release-precheck: install-manifest
	@if [ "$(GIT_TREE_STATE)" != "clean" ]; then echo 'git tree state is $(GIT_TREE_STATE)' ; exit 1; fi
	@if [ -z "$(GIT_TAG)" ]; then echo 'commit must be tagged to perform release' ; exit 1; fi

.PHONY: release
release: release-precheck precheckin cli-darwin cli-linux server-image controller-image repo-server-image cli-image
