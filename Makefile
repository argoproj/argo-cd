PACKAGE=github.com/argoproj/argo-cd/v2/common
CURRENT_DIR=$(shell pwd)
DIST_DIR=${CURRENT_DIR}/dist
CLI_NAME=argocd
BIN_NAME=argocd
CGO_FLAG=0

GEN_RESOURCES_CLI_NAME=argocd-resources-gen

HOST_OS:=$(shell go env GOOS)
HOST_ARCH:=$(shell go env GOARCH)

TARGET_ARCH?=linux/amd64

VERSION=$(shell cat ${CURRENT_DIR}/VERSION)
BUILD_DATE:=$(if $(BUILD_DATE),$(BUILD_DATE),$(shell date -u +'%Y-%m-%dT%H:%M:%SZ'))
GIT_COMMIT:=$(if $(GIT_COMMIT),$(GIT_COMMIT),$(shell git rev-parse HEAD))
GIT_TAG:=$(if $(GIT_TAG),$(GIT_TAG),$(shell if [ -z "`git status --porcelain`" ]; then git describe --exact-match --tags HEAD 2>/dev/null; fi))
GIT_TREE_STATE:=$(if $(GIT_TREE_STATE),$(GIT_TREE_STATE),$(shell if [ -z "`git status --porcelain`" ]; then echo "clean" ; else echo "dirty"; fi))
VOLUME_MOUNT=$(shell if test "$(go env GOOS)" = "darwin"; then echo ":delegated"; elif test selinuxenabled; then echo ":delegated"; else echo ""; fi)
KUBECTL_VERSION=$(shell go list -m k8s.io/client-go | head -n 1 | rev | cut -d' ' -f1 | rev)

GOPATH?=$(shell if test -x `which go`; then go env GOPATH; else echo "$(HOME)/go"; fi)
GOCACHE?=$(HOME)/.cache/go-build

# Docker command to use
DOCKER?=docker
ifeq ($(DOCKER),podman)
PODMAN_ARGS=--userns keep-id
else
PODMAN_ARGS=
endif

DOCKER_SRCDIR?=$(GOPATH)/src
DOCKER_WORKDIR?=/go/src/github.com/argoproj/argo-cd

ARGOCD_PROCFILE?=Procfile

# pointing to python 3.7 to match https://github.com/argoproj/argo-cd/blob/master/.readthedocs.yml
MKDOCS_DOCKER_IMAGE?=python:3.7-alpine
MKDOCS_RUN_ARGS?=

# Configuration for building argocd-test-tools image
TEST_TOOLS_NAMESPACE?=
TEST_TOOLS_IMAGE=argocd-test-tools
TEST_TOOLS_TAG?=latest
ifdef TEST_TOOLS_NAMESPACE
TEST_TOOLS_PREFIX=${TEST_TOOLS_NAMESPACE}/
endif

# You can change the ports where ArgoCD components will be listening on by
# setting the appropriate environment variables before running make.
ARGOCD_E2E_APISERVER_PORT?=8080
ARGOCD_E2E_REPOSERVER_PORT?=8081
ARGOCD_E2E_REDIS_PORT?=6379
ARGOCD_E2E_DEX_PORT?=5556
ARGOCD_E2E_YARN_HOST?=localhost
ARGOCD_E2E_DISABLE_AUTH?=

ARGOCD_E2E_TEST_TIMEOUT?=90m

ARGOCD_IN_CI?=false
ARGOCD_TEST_E2E?=true
ARGOCD_BIN_MODE?=true

ARGOCD_LINT_GOGC?=20

# Depending on where we are (legacy or non-legacy pwd), we need to use
# different Docker volume mounts for our source tree
LEGACY_PATH=$(GOPATH)/src/github.com/argoproj/argo-cd
ifeq ("$(PWD)","$(LEGACY_PATH)")
DOCKER_SRC_MOUNT="$(DOCKER_SRCDIR):/go/src$(VOLUME_MOUNT)"
else
DOCKER_SRC_MOUNT="$(PWD):/go/src/github.com/argoproj/argo-cd$(VOLUME_MOUNT)"
endif

# User and group IDs to map to the test container
CONTAINER_UID=$(shell id -u)
CONTAINER_GID=$(shell id -g)

# Set SUDO to sudo to run privileged commands with sudo
SUDO?=

# Runs any command in the argocd-test-utils container in server mode
# Server mode container will start with uid 0 and drop privileges during runtime
define run-in-test-server
	$(SUDO) $(DOCKER) run --rm -it \
		--name argocd-test-server \
		-u $(CONTAINER_UID):$(CONTAINER_GID) \
		-e USER_ID=$(CONTAINER_UID) \
		-e HOME=/home/user \
		-e GOPATH=/go \
		-e GOCACHE=/tmp/go-build-cache \
		-e ARGOCD_IN_CI=$(ARGOCD_IN_CI) \
		-e ARGOCD_E2E_TEST=$(ARGOCD_E2E_TEST) \
		-e ARGOCD_E2E_YARN_HOST=$(ARGOCD_E2E_YARN_HOST) \
		-e ARGOCD_E2E_DISABLE_AUTH=$(ARGOCD_E2E_DISABLE_AUTH) \
		-e ARGOCD_TLS_DATA_PATH=${ARGOCD_TLS_DATA_PATH:-/tmp/argocd-local/tls} \
		-e ARGOCD_SSH_DATA_PATH=${ARGOCD_SSH_DATA_PATH:-/tmp/argocd-local/ssh} \
		-e ARGOCD_GPG_DATA_PATH=${ARGOCD_GPG_DATA_PATH:-/tmp/argocd-local/gpg/source} \
		-e ARGOCD_APPLICATION_NAMESPACES \
		-e GITHUB_TOKEN \
		-v ${DOCKER_SRC_MOUNT} \
		-v ${GOPATH}/pkg/mod:/go/pkg/mod${VOLUME_MOUNT} \
		-v ${GOCACHE}:/tmp/go-build-cache${VOLUME_MOUNT} \
		-v ${HOME}/.kube:/home/user/.kube${VOLUME_MOUNT} \
		-v /tmp:/tmp${VOLUME_MOUNT} \
		-w ${DOCKER_WORKDIR} \
		-p ${ARGOCD_E2E_APISERVER_PORT}:8080 \
		-p 4000:4000 \
		-p 5000:5000 \
		$(PODMAN_ARGS) \
		$(TEST_TOOLS_PREFIX)$(TEST_TOOLS_IMAGE):$(TEST_TOOLS_TAG) \
		bash -c "$(1)"
endef

# Runs any command in the argocd-test-utils container in client mode
define run-in-test-client
	$(SUDO) $(DOCKER) run --rm -it \
	  --name argocd-test-client \
		-u $(CONTAINER_UID):$(CONTAINER_GID) \
		-e HOME=/home/user \
		-e GOPATH=/go \
		-e ARGOCD_E2E_K3S=$(ARGOCD_E2E_K3S) \
		-e GITHUB_TOKEN \
		-e GOCACHE=/tmp/go-build-cache \
		-e ARGOCD_LINT_GOGC=$(ARGOCD_LINT_GOGC) \
		-v ${DOCKER_SRC_MOUNT} \
		-v ${GOPATH}/pkg/mod:/go/pkg/mod${VOLUME_MOUNT} \
		-v ${GOCACHE}:/tmp/go-build-cache${VOLUME_MOUNT} \
		-v ${HOME}/.kube:/home/user/.kube${VOLUME_MOUNT} \
		-v /tmp:/tmp${VOLUME_MOUNT} \
		-w ${DOCKER_WORKDIR} \
		$(PODMAN_ARGS) \
		$(TEST_TOOLS_PREFIX)$(TEST_TOOLS_IMAGE):$(TEST_TOOLS_TAG) \
		bash -c "$(1)"
endef

#
define exec-in-test-server
	$(SUDO) $(DOCKER) exec -it -u $(CONTAINER_UID):$(CONTAINER_GID) -e ARGOCD_E2E_RECORD=$(ARGOCD_E2E_RECORD) -e ARGOCD_E2E_K3S=$(ARGOCD_E2E_K3S) argocd-test-server $(1)
endef

PATH:=$(PATH):$(PWD)/hack

# docker image publishing options
DOCKER_PUSH?=false
IMAGE_NAMESPACE?=
# perform static compilation
STATIC_BUILD?=true
# build development images
DEV_IMAGE?=false
ARGOCD_GPG_ENABLED?=true
ARGOCD_E2E_APISERVER_PORT?=8080

ifeq (${COVERAGE_ENABLED}, true)
# We use this in the cli-local target to enable code coverage for e2e tests.
COVERAGE_FLAG=-cover
else
COVERAGE_FLAG=
endif

override LDFLAGS += \
  -X ${PACKAGE}.version=${VERSION} \
  -X ${PACKAGE}.buildDate=${BUILD_DATE} \
  -X ${PACKAGE}.gitCommit=${GIT_COMMIT} \
  -X ${PACKAGE}.gitTreeState=${GIT_TREE_STATE}\
  -X ${PACKAGE}.kubectlVersion=${KUBECTL_VERSION}\
  -X "${PACKAGE}.extraBuildInfo=${EXTRA_BUILD_INFO}"

ifeq (${STATIC_BUILD}, true)
override LDFLAGS += -extldflags "-static"
endif

ifneq (${GIT_TAG},)
IMAGE_TAG=${GIT_TAG}
LDFLAGS += -X ${PACKAGE}.gitTag=${GIT_TAG}
else
IMAGE_TAG?=latest
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
all: cli image

.PHONY: mockgen
mockgen:
	./hack/generate-mock.sh

.PHONY: gogen
gogen:
	export GO111MODULE=off
	go generate ./...

.PHONY: protogen
protogen: mod-vendor-local protogen-fast

.PHONY: protogen-fast
protogen-fast:
	export GO111MODULE=off
	./hack/generate-proto.sh

.PHONY: openapigen
openapigen:
	export GO111MODULE=off
	./hack/update-openapi.sh

.PHONY: notification-catalog
notification-catalog:
	go run ./hack/gen-catalog catalog

.PHONY: notification-docs
notification-docs:
	go run ./hack/gen-docs
	go run ./hack/gen-catalog docs


.PHONY: clientgen
clientgen:
	export GO111MODULE=off
	./hack/update-codegen.sh

.PHONY: clidocsgen
clidocsgen:
	go run tools/cmd-docs/main.go

.PHONY: actionsdocsgen
actionsdocsgen:
	hack/generate-actions-list.sh

.PHONY: codegen-local
codegen-local: mod-vendor-local mockgen gogen protogen clientgen openapigen clidocsgen actionsdocsgen manifests-local notification-docs notification-catalog
	rm -rf vendor/

.PHONY: codegen-local-fast
codegen-local-fast: mockgen gogen protogen-fast clientgen openapigen clidocsgen manifests-local notification-docs notification-catalog

.PHONY: codegen
codegen: test-tools-image
	$(call run-in-test-client,make codegen-local)

.PHONY: cli
cli: test-tools-image
	$(call run-in-test-client, GOOS=${HOST_OS} GOARCH=${HOST_ARCH} make cli-local)

.PHONY: cli-local
cli-local: clean-debug
	CGO_ENABLED=${CGO_FLAG} GODEBUG="tarinsecurepath=0,zipinsecurepath=0" go build -gcflags="all=-N -l" $(COVERAGE_FLAG) -v -ldflags '${LDFLAGS}' -o ${DIST_DIR}/${CLI_NAME} ./cmd

.PHONY: gen-resources-cli-local
gen-resources-cli-local: clean-debug
	CGO_ENABLED=${CGO_FLAG} GODEBUG="tarinsecurepath=0,zipinsecurepath=0" go build -v -ldflags '${LDFLAGS}' -o ${DIST_DIR}/${GEN_RESOURCES_CLI_NAME} ./hack/gen-resources/cmd

.PHONY: release-cli
release-cli: clean-debug build-ui
	make BIN_NAME=argocd-darwin-amd64 GOOS=darwin argocd-all
	make BIN_NAME=argocd-darwin-arm64 GOOS=darwin GOARCH=arm64 argocd-all
	make BIN_NAME=argocd-linux-amd64 GOOS=linux argocd-all
	make BIN_NAME=argocd-linux-arm64 GOOS=linux GOARCH=arm64 argocd-all
	make BIN_NAME=argocd-linux-ppc64le GOOS=linux GOARCH=ppc64le argocd-all
	make BIN_NAME=argocd-linux-s390x GOOS=linux GOARCH=s390x argocd-all
	make BIN_NAME=argocd-windows-amd64.exe GOOS=windows argocd-all

.PHONY: test-tools-image
test-tools-image:
ifndef SKIP_TEST_TOOLS_IMAGE
	$(SUDO) $(DOCKER) build --build-arg UID=$(CONTAINER_UID) -t $(TEST_TOOLS_PREFIX)$(TEST_TOOLS_IMAGE) -f test/container/Dockerfile .
	$(SUDO) $(DOCKER) tag $(TEST_TOOLS_PREFIX)$(TEST_TOOLS_IMAGE) $(TEST_TOOLS_PREFIX)$(TEST_TOOLS_IMAGE):$(TEST_TOOLS_TAG)
endif

.PHONY: manifests-local
manifests-local:
	./hack/update-manifests.sh

.PHONY: manifests
manifests: test-tools-image
	$(call run-in-test-client,make manifests-local IMAGE_NAMESPACE='${IMAGE_NAMESPACE}' IMAGE_TAG='${IMAGE_TAG}')

# consolidated binary for cli, util, server, repo-server, controller
.PHONY: argocd-all
argocd-all: clean-debug
	CGO_ENABLED=${CGO_FLAG} GOOS=${GOOS} GOARCH=${GOARCH} GODEBUG="tarinsecurepath=0,zipinsecurepath=0" go build -v -ldflags '${LDFLAGS}' -o ${DIST_DIR}/${BIN_NAME} ./cmd

.PHONY: server
server: clean-debug
	CGO_ENABLED=${CGO_FLAG} GODEBUG="tarinsecurepath=0,zipinsecurepath=0" go build -v -ldflags '${LDFLAGS}' -o ${DIST_DIR}/argocd-server ./cmd

.PHONY: repo-server
repo-server:
	CGO_ENABLED=${CGO_FLAG} GODEBUG="tarinsecurepath=0,zipinsecurepath=0" go build -v -ldflags '${LDFLAGS}' -o ${DIST_DIR}/argocd-repo-server ./cmd

.PHONY: controller
controller:
	CGO_ENABLED=${CGO_FLAG} GODEBUG="tarinsecurepath=0,zipinsecurepath=0" go build -v -ldflags '${LDFLAGS}' -o ${DIST_DIR}/argocd-application-controller ./cmd

.PHONY: build-ui
build-ui:
	DOCKER_BUILDKIT=1 $(DOCKER) build -t argocd-ui --platform=$(TARGET_ARCH) --target argocd-ui .
	find ./ui/dist -type f -not -name gitkeep -delete
	$(DOCKER) run -v ${CURRENT_DIR}/ui/dist/app:/tmp/app --rm -t argocd-ui sh -c 'cp -r ./dist/app/* /tmp/app/'

.PHONY: image
ifeq ($(DEV_IMAGE), true)
# The "dev" image builds the binaries from the users desktop environment (instead of in Docker)
# which speeds up builds. Dockerfile.dev needs to be copied into dist to perform the build, since
# the dist directory is under .dockerignore.
IMAGE_TAG="dev-$(shell git describe --always --dirty)"
image: build-ui
	DOCKER_BUILDKIT=1 $(DOCKER) build --platform=$(TARGET_ARCH) -t argocd-base --target argocd-base .
	CGO_ENABLED=${CGO_FLAG} GOOS=linux GOARCH=amd64 GODEBUG="tarinsecurepath=0,zipinsecurepath=0" go build -v -ldflags '${LDFLAGS}' -o ${DIST_DIR}/argocd ./cmd
	ln -sfn ${DIST_DIR}/argocd ${DIST_DIR}/argocd-server
	ln -sfn ${DIST_DIR}/argocd ${DIST_DIR}/argocd-application-controller
	ln -sfn ${DIST_DIR}/argocd ${DIST_DIR}/argocd-repo-server
	ln -sfn ${DIST_DIR}/argocd ${DIST_DIR}/argocd-cmp-server
	ln -sfn ${DIST_DIR}/argocd ${DIST_DIR}/argocd-dex
	cp Dockerfile.dev dist
	DOCKER_BUILDKIT=1 $(DOCKER) build --platform=$(TARGET_ARCH) -t $(IMAGE_PREFIX)argocd:$(IMAGE_TAG) -f dist/Dockerfile.dev dist
else
image:
	DOCKER_BUILDKIT=1 $(DOCKER) build -t $(IMAGE_PREFIX)argocd:$(IMAGE_TAG) --platform=$(TARGET_ARCH) .
endif
	@if [ "$(DOCKER_PUSH)" = "true" ] ; then $(DOCKER) push $(IMAGE_PREFIX)argocd:$(IMAGE_TAG) ; fi

.PHONY: armimage
armimage:
	$(DOCKER) build -t $(IMAGE_PREFIX)argocd:$(IMAGE_TAG)-arm .

.PHONY: builder-image
builder-image:
	$(DOCKER) build  -t $(IMAGE_PREFIX)argo-cd-ci-builder:$(IMAGE_TAG) --target builder .
	@if [ "$(DOCKER_PUSH)" = "true" ] ; then $(DOCKER) push $(IMAGE_PREFIX)argo-cd-ci-builder:$(IMAGE_TAG) ; fi

.PHONY: mod-download
mod-download: test-tools-image
	$(call run-in-test-client,go mod download)

.PHONY: mod-download-local
mod-download-local:
	go mod download && go mod tidy # go mod download changes go.sum https://github.com/golang/go/issues/42970

.PHONY: mod-vendor
mod-vendor: test-tools-image
	$(call run-in-test-client,go mod vendor)

.PHONY: mod-vendor-local
mod-vendor-local: mod-download-local
	go mod vendor

# Deprecated - replace by install-tools-local
.PHONY: install-lint-tools
install-lint-tools:
	./hack/install.sh lint-tools

# Run linter on the code
.PHONY: lint
lint: test-tools-image
	$(call run-in-test-client,make lint-local)

# Run linter on the code (local version)
.PHONY: lint-local
lint-local:
	golangci-lint --version
	# NOTE: If you get a "Killed" OOM message, try reducing the value of GOGC
	# See https://github.com/golangci/golangci-lint#memory-usage-of-golangci-lint
	GOGC=$(ARGOCD_LINT_GOGC) GOMAXPROCS=2 golangci-lint run --fix --verbose

.PHONY: lint-ui
lint-ui: test-tools-image
	$(call run-in-test-client,make lint-ui-local)

.PHONY: lint-ui-local
lint-ui-local:
	cd ui && yarn lint

# Build all Go code
.PHONY: build
build: test-tools-image
	mkdir -p $(GOCACHE)
	$(call run-in-test-client, make build-local)

# Build all Go code (local version)
.PHONY: build-local
build-local:
	GODEBUG="tarinsecurepath=0,zipinsecurepath=0" go build -v `go list ./... | grep -v 'resource_customizations\|test/e2e'`

# Run all unit tests
#
# If TEST_MODULE is set (to fully qualified module name), only this specific
# module will be tested.
.PHONY: test
test: test-tools-image
	mkdir -p $(GOCACHE)
	$(call run-in-test-client,make TEST_MODULE=$(TEST_MODULE) test-local)

# Run all unit tests (local version)
.PHONY: test-local
test-local:
	if test "$(TEST_MODULE)" = ""; then \
		DIST_DIR=${DIST_DIR} RERUN_FAILS=0 PACKAGES=`go list ./... | grep -v 'test/e2e'` ./hack/test.sh -args -test.gocoverdir="$(PWD)/test-results"; \
	else \
		DIST_DIR=${DIST_DIR} RERUN_FAILS=0 PACKAGES="$(TEST_MODULE)" ./hack/test.sh -args -test.gocoverdir="$(PWD)/test-results" "$(TEST_MODULE)"; \
	fi

.PHONY: test-race
test-race: test-tools-image
	mkdir -p $(GOCACHE)
	$(call run-in-test-client,make TEST_MODULE=$(TEST_MODULE) test-race-local)

# Run all unit tests, with data race detection, skipping known failures (local version)
.PHONY: test-race-local
test-race-local:
	if test "$(TEST_MODULE)" = ""; then \
		DIST_DIR=${DIST_DIR} RERUN_FAILS=0 PACKAGES=`go list ./... | grep -v 'test/e2e'` ./hack/test.sh -race -args -test.gocoverdir="$(PWD)/test-results"; \
	else \
		DIST_DIR=${DIST_DIR} RERUN_FAILS=0 PACKAGES="$(TEST_MODULE)" ./hack/test.sh -race -args -test.gocoverdir="$(PWD)/test-results"; \
	fi

# Run the E2E test suite. E2E test servers (see start-e2e target) must be
# started before.
.PHONY: test-e2e
test-e2e:
	$(call exec-in-test-server,make test-e2e-local)

# Run the E2E test suite (local version)
.PHONY: test-e2e-local
test-e2e-local: cli-local
	# NO_PROXY ensures all tests don't go out through a proxy if one is configured on the test system
	export GO111MODULE=off
	DIST_DIR=${DIST_DIR} RERUN_FAILS=5 PACKAGES="./test/e2e" ARGOCD_E2E_RECORD=${ARGOCD_E2E_RECORD} ARGOCD_GPG_ENABLED=true NO_PROXY=* ./hack/test.sh -timeout $(ARGOCD_E2E_TEST_TIMEOUT) -v -args -test.gocoverdir="$(PWD)/test-results"

# Spawns a shell in the test server container for debugging purposes
debug-test-server: test-tools-image
	$(call run-in-test-server,/bin/bash)

# Spawns a shell in the test client container for debugging purposes
debug-test-client: test-tools-image
	$(call run-in-test-client,/bin/bash)

# Starts e2e server in a container
.PHONY: start-e2e
start-e2e: test-tools-image
	$(DOCKER) version
	mkdir -p ${GOCACHE}
	$(call run-in-test-server,make ARGOCD_PROCFILE=test/container/Procfile start-e2e-local)

# Starts e2e server locally (or within a container)
.PHONY: start-e2e-local
start-e2e-local: mod-vendor-local dep-ui-local cli-local
	kubectl create ns argocd-e2e || true
	kubectl create ns argocd-e2e-external || true
	kubectl create ns argocd-e2e-external-2 || true
	kubectl config set-context --current --namespace=argocd-e2e
	kustomize build test/manifests/base | kubectl apply -f -
	kubectl apply -f https://raw.githubusercontent.com/open-cluster-management/api/a6845f2ebcb186ec26b832f60c988537a58f3859/cluster/v1alpha1/0000_04_clusters.open-cluster-management.io_placementdecisions.crd.yaml
	# Create GPG keys and source directories
	if test -d /tmp/argo-e2e/app/config/gpg; then rm -rf /tmp/argo-e2e/app/config/gpg/*; fi
	mkdir -p /tmp/argo-e2e/app/config/gpg/keys && chmod 0700 /tmp/argo-e2e/app/config/gpg/keys
	mkdir -p /tmp/argo-e2e/app/config/gpg/source && chmod 0700 /tmp/argo-e2e/app/config/gpg/source
	mkdir -p /tmp/argo-e2e/app/config/plugin && chmod 0700 /tmp/argo-e2e/app/config/plugin
	# create folders to hold go coverage results for each component
	mkdir -p /tmp/coverage/app-controller
	mkdir -p /tmp/coverage/api-server
	mkdir -p /tmp/coverage/repo-server
	mkdir -p /tmp/coverage/applicationset-controller
	mkdir -p /tmp/coverage/notification
	# set paths for locally managed ssh known hosts and tls certs data
	ARGOCD_SSH_DATA_PATH=/tmp/argo-e2e/app/config/ssh \
	ARGOCD_TLS_DATA_PATH=/tmp/argo-e2e/app/config/tls \
	ARGOCD_GPG_DATA_PATH=/tmp/argo-e2e/app/config/gpg/source \
	ARGOCD_GNUPGHOME=/tmp/argo-e2e/app/config/gpg/keys \
	ARGOCD_GPG_ENABLED=$(ARGOCD_GPG_ENABLED) \
	ARGOCD_PLUGINCONFIGFILEPATH=/tmp/argo-e2e/app/config/plugin \
	ARGOCD_PLUGINSOCKFILEPATH=/tmp/argo-e2e/app/config/plugin \
	ARGOCD_E2E_DISABLE_AUTH=false \
	ARGOCD_ZJWT_FEATURE_FLAG=always \
	ARGOCD_IN_CI=$(ARGOCD_IN_CI) \
	BIN_MODE=$(ARGOCD_BIN_MODE) \
	ARGOCD_APPLICATION_NAMESPACES=argocd-e2e-external,argocd-e2e-external-2 \
	ARGOCD_APPLICATIONSET_CONTROLLER_NAMESPACES=argocd-e2e-external,argocd-e2e-external-2 \
	ARGOCD_APPLICATIONSET_CONTROLLER_ALLOWED_SCM_PROVIDERS=http://127.0.0.1:8341,http://127.0.0.1:8342,http://127.0.0.1:8343,http://127.0.0.1:8344 \
	ARGOCD_E2E_TEST=true \
		goreman -f $(ARGOCD_PROCFILE) start ${ARGOCD_START}
	ls -lrt /tmp/coverage

# Cleans VSCode debug.test files from sub-dirs to prevent them from being included in by golang embed
.PHONY: clean-debug
clean-debug:
	-find ${CURRENT_DIR} -name debug.test -exec rm -f {} +

.PHONY: clean
clean: clean-debug
	-rm -rf ${CURRENT_DIR}/dist

.PHONY: start
start: test-tools-image
	$(DOCKER) version
	$(call run-in-test-server,make ARGOCD_PROCFILE=test/container/Procfile start-local ARGOCD_START=${ARGOCD_START})

# Starts a local instance of ArgoCD
.PHONY: start-local
start-local: mod-vendor-local dep-ui-local cli-local
	# check we can connect to Docker to start Redis
	killall goreman || true
	kubectl create ns argocd || true
	rm -rf /tmp/argocd-local
	mkdir -p /tmp/argocd-local
	mkdir -p /tmp/argocd-local/gpg/keys && chmod 0700 /tmp/argocd-local/gpg/keys
	mkdir -p /tmp/argocd-local/gpg/source
	REDIS_PASSWORD=$(shell kubectl get secret argocd-redis -o jsonpath='{.data.auth}' | base64 -d) \
	ARGOCD_ZJWT_FEATURE_FLAG=always \
	ARGOCD_IN_CI=false \
	ARGOCD_GPG_ENABLED=$(ARGOCD_GPG_ENABLED) \
	BIN_MODE=$(ARGOCD_BIN_MODE) \
	ARGOCD_E2E_TEST=false \
	ARGOCD_APPLICATION_NAMESPACES=$(ARGOCD_APPLICATION_NAMESPACES) \
		goreman -f $(ARGOCD_PROCFILE) start ${ARGOCD_START}

# Run goreman start with exclude option , provide exclude env variable with list of services
.PHONY: run
run:
	bash ./hack/goreman-start.sh


# Runs pre-commit validation with the virtualized toolchain
.PHONY: pre-commit
pre-commit: codegen build lint test

# Runs pre-commit validation with the local toolchain
.PHONY: pre-commit-local
pre-commit-local: codegen-local build-local lint-local test-local

.PHONY: release-precheck
release-precheck: manifests
	@if [ "$(GIT_TREE_STATE)" != "clean" ]; then echo 'git tree state is $(GIT_TREE_STATE)' ; exit 1; fi
	@if [ -z "$(GIT_TAG)" ]; then echo 'commit must be tagged to perform release' ; exit 1; fi
	@if [ "$(GIT_TAG)" != "v`cat VERSION`" ]; then echo 'VERSION does not match git tag'; exit 1; fi

.PHONY: release
release: pre-commit release-precheck image release-cli

.PHONY: build-docs-local
build-docs-local:
	mkdocs build

.PHONY: build-docs
build-docs:
	$(DOCKER) run ${MKDOCS_RUN_ARGS} --rm -it -v ${CURRENT_DIR}:/docs -w /docs --entrypoint "" ${MKDOCS_DOCKER_IMAGE} sh -c 'pip install mkdocs; pip install $$(mkdocs get-deps); mkdocs build'

.PHONY: serve-docs-local
serve-docs-local:
	mkdocs serve

.PHONY: serve-docs
serve-docs:
	$(DOCKER) run ${MKDOCS_RUN_ARGS} --rm -it -p 8000:8000 -v ${CURRENT_DIR}:/docs -w /docs --entrypoint "" ${MKDOCS_DOCKER_IMAGE} sh -c 'pip install mkdocs; pip install $$(mkdocs get-deps); mkdocs serve -a $$(ip route get 1 | awk '\''{print $$7}'\''):8000'

# Verify that kubectl can connect to your K8s cluster from Docker
.PHONY: verify-kube-connect
verify-kube-connect: test-tools-image
	$(call run-in-test-client,kubectl version)

# Show the Go version of local and virtualized environments
.PHONY: show-go-version
show-go-version: test-tools-image
	@echo -n "Local Go version: "
	@go version
	@echo -n "Docker Go version: "
	$(call run-in-test-client,go version)

# Installs all tools required to build and test ArgoCD locally
.PHONY: install-tools-local
install-tools-local: install-test-tools-local install-codegen-tools-local install-go-tools-local

# Installs all tools required for running unit & end-to-end tests (Linux packages)
.PHONY: install-test-tools-local
install-test-tools-local:
	./hack/install.sh kustomize
	./hack/install.sh helm
	./hack/install.sh gotestsum

# Installs all tools required for running codegen (Linux packages)
.PHONY: install-codegen-tools-local
install-codegen-tools-local:
	./hack/install.sh codegen-tools

# Installs all tools required for running codegen (Go packages)
.PHONY: install-go-tools-local
install-go-tools-local:
	./hack/install.sh codegen-go-tools

.PHONY: dep-ui
dep-ui: test-tools-image
	$(call run-in-test-client,make dep-ui-local)

dep-ui-local:
	cd ui && yarn install

start-test-k8s:
	go run ./hack/k8s

.PHONY: list
list:
	@LC_ALL=C $(MAKE) -pRrq -f $(lastword $(MAKEFILE_LIST)) : 2>/dev/null | awk -v RS= -F: '/^# File/,/^# Finished Make data base/ {if ($$1 !~ "^[#.]") {print $$1}}' | sort | egrep -v -e '^[^[:alnum:]]' -e '^$@$$'

.PHONY: applicationset-controller
applicationset-controller:
	GODEBUG="tarinsecurepath=0,zipinsecurepath=0" CGO_ENABLED=${CGO_FLAG} go build -v -ldflags '${LDFLAGS}' -o ${DIST_DIR}/argocd-applicationset-controller ./cmd

.PHONY: checksums
checksums:
	sha256sum ./dist/$(BIN_NAME)-* | awk -F './dist/' '{print $$1 $$2}' > ./dist/$(BIN_NAME)-$(TARGET_VERSION)-checksums.txt

.PHONY: snyk-container-tests
snyk-container-tests:
	./hack/snyk-container-tests.sh

.PHONY: snyk-non-container-tests
snyk-non-container-tests:
	./hack/snyk-non-container-tests.sh

.PHONY: snyk-report
snyk-report:
	./hack/snyk-report.sh $(target_branch)

.PHONY: help
help:
	@echo 'Note: Generally an item w/ (-local) will run inside docker unless you use the -local variant'
	@echo
	@echo 'Common targets'
	@echo
	@echo 'all -- make cli and image'
	@echo
	@echo 'components:'
	@echo '  applicationset-controller -- applicationset controller'
	@echo '  cli(-local)               -- argocd cli program'
	@echo '  controller                -- controller (orchestrator)'
	@echo '  repo-server               -- repo server (manage repository instances)'
	@echo '  server                    -- argocd web application'
	@echo
	@echo 'build:'
	@echo '  image                     -- make image of the following items'
	@echo '  build(-local)             -- compile go'
	@echo '  build-docs(-local)        -- build docs'
	@echo '  build-ui                  -- compile typescript'
	@echo
	@echo 'run:'
	@echo '  run                       -- run the components locally'
	@echo '  serve-docs(-local)        -- expose the documents for viewing in a browser'
	@echo
	@echo 'release:'
	@echo '  release-cli'
	@echo '  release-precheck'
	@echo '  checksums'
	@echo
	@echo 'docs:'
	@echo '  build-docs(-local)'
	@echo '  serve-docs(-local)'
	@echo '  notification-docs'
	@echo '  clidocsgen'
	@echo
	@echo 'testing:'
	@echo '  test(-local)'
	@echo '  start-e2e(-local)'
	@echo '  test-e2e(-local)'
	@echo '  test-race(-local)'
	@echo
	@echo 'debug:'
	@echo '  list -- list all make targets'
	@echo '  install-tools-local -- install all the tools below'
	@echo '  install-lint-tools(-local)'
	@echo
	@echo 'codegen:'
	@echo '  codegen(-local) -- if using -local, run the following targets first'
	@echo '  install-codegen-tools-local -- run this to install the codegen tools'
	@echo '  install-go-tools-local -- run this to install go libraries for codegen'
