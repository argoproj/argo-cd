PACKAGE=github.com/argoproj/argo-cd/common
CURRENT_DIR=$(shell pwd)
DIST_DIR=${CURRENT_DIR}/dist
CLI_NAME=argocd

VERSION=$(shell cat ${CURRENT_DIR}/VERSION)
BUILD_DATE=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT=$(shell git rev-parse HEAD)
GIT_TAG=$(shell if [ -z "`git status --porcelain`" ]; then git describe --exact-match --tags HEAD 2>/dev/null; fi)
GIT_TREE_STATE=$(shell if [ -z "`git status --porcelain`" ]; then echo "clean" ; else echo "dirty"; fi)
PACKR_CMD=$(shell if [ "`which packr`" ]; then echo "packr"; else echo "go run vendor/github.com/gobuffalo/packr/packr/main.go"; fi)
VOLUME_MOUNT=$(shell if [[ selinuxenabled -eq 0 ]]; then echo ":Z"; elif [[ $(go env GOOS)=="darwin" ]]; then echo ":delegated"; else echo ""; fi)

define run-in-dev-tool
    docker run --rm -it -u $(shell id -u) -e HOME=/home/user -v ${CURRENT_DIR}:/go/src/github.com/argoproj/argo-cd${VOLUME_MOUNT} -w /go/src/github.com/argoproj/argo-cd argocd-dev-tools bash -c "GOPATH=/go $(1)"
endef

PATH:=$(PATH):$(PWD)/hack

# docker image publishing options
DOCKER_PUSH?=false
IMAGE_NAMESPACE?=
# perform static compilation
STATIC_BUILD?=true
# build development images
DEV_IMAGE?=false

override LDFLAGS += \
  -X ${PACKAGE}.version=${VERSION} \
  -X ${PACKAGE}.buildDate=${BUILD_DATE} \
  -X ${PACKAGE}.gitCommit=${GIT_COMMIT} \
  -X ${PACKAGE}.gitTreeState=${GIT_TREE_STATE}

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
all: cli image argocd-util

.PHONY: protogen
protogen:
	./hack/generate-proto.sh

.PHONY: openapigen
openapigen:
	./hack/update-openapi.sh

.PHONY: clientgen
clientgen:
	./hack/update-codegen.sh

.PHONY: codegen-local
codegen-local: protogen clientgen openapigen manifests-local

.PHONY: codegen
codegen: dev-tools-image
	$(call run-in-dev-tool,make codegen-local)

.PHONY: cli
cli: clean-debug
	CGO_ENABLED=0 ${PACKR_CMD} build -v -i -ldflags '${LDFLAGS}' -o ${DIST_DIR}/${CLI_NAME} ./cmd/argocd

.PHONY: release-cli
release-cli: clean-debug image
	docker create --name tmp-argocd-linux $(IMAGE_PREFIX)argocd:$(IMAGE_TAG)
	docker cp tmp-argocd-linux:/usr/local/bin/argocd ${DIST_DIR}/argocd-linux-amd64
	docker cp tmp-argocd-linux:/usr/local/bin/argocd-darwin-amd64 ${DIST_DIR}/argocd-darwin-amd64
	docker cp tmp-argocd-linux:/usr/local/bin/argocd-windows-amd64.exe ${DIST_DIR}/argocd-windows-amd64.exe
	docker rm tmp-argocd-linux

.PHONY: argocd-util
argocd-util: clean-debug
	# Build argocd-util as a statically linked binary, so it could run within the alpine-based dex container (argoproj/argo-cd#844)
	CGO_ENABLED=0 ${PACKR_CMD} build -v -i -ldflags '${LDFLAGS}' -o ${DIST_DIR}/argocd-util ./cmd/argocd-util

.PHONY: dev-tools-image
dev-tools-image:
	cd hack && docker build -t argocd-dev-tools . -f Dockerfile.dev-tools

.PHONY: manifests-local
manifests-local:
	./hack/update-manifests.sh

.PHONY: manifests
manifests: dev-tools-image
	$(call run-in-dev-tool,make manifests-local IMAGE_TAG='${IMAGE_TAG}')


# NOTE: we use packr to do the build instead of go, since we embed swagger files and policy.csv
# files into the go binary
.PHONY: server
server: clean-debug
	CGO_ENABLED=0 ${PACKR_CMD} build -v -i -ldflags '${LDFLAGS}' -o ${DIST_DIR}/argocd-server ./cmd/argocd-server

.PHONY: repo-server
repo-server:
	CGO_ENABLED=0 ${PACKR_CMD} build -v -i -ldflags '${LDFLAGS}' -o ${DIST_DIR}/argocd-repo-server ./cmd/argocd-repo-server

.PHONY: controller
controller:
	CGO_ENABLED=0 ${PACKR_CMD} build -v -i -ldflags '${LDFLAGS}' -o ${DIST_DIR}/argocd-application-controller ./cmd/argocd-application-controller

.PHONY: packr
packr:
	go build -o ${DIST_DIR}/packr ./vendor/github.com/gobuffalo/packr/packr/

.PHONY: image
ifeq ($(DEV_IMAGE), true)
# The "dev" image builds the binaries from the users desktop environment (instead of in Docker)
# which speeds up builds. Dockerfile.dev needs to be copied into dist to perform the build, since
# the dist directory is under .dockerignore.
IMAGE_TAG="dev-$(shell git describe --always --dirty)"
image: packr
	docker build -t argocd-base --target argocd-base .
	docker build -t argocd-ui --target argocd-ui .
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 dist/packr build -v -i -ldflags '${LDFLAGS}' -o ${DIST_DIR}/argocd-server ./cmd/argocd-server
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 dist/packr build -v -i -ldflags '${LDFLAGS}' -o ${DIST_DIR}/argocd-application-controller ./cmd/argocd-application-controller
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 dist/packr build -v -i -ldflags '${LDFLAGS}' -o ${DIST_DIR}/argocd-repo-server ./cmd/argocd-repo-server
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 dist/packr build -v -i -ldflags '${LDFLAGS}' -o ${DIST_DIR}/argocd-util ./cmd/argocd-util
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 dist/packr build -v -i -ldflags '${LDFLAGS}' -o ${DIST_DIR}/argocd ./cmd/argocd
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 dist/packr build -v -i -ldflags '${LDFLAGS}' -o ${DIST_DIR}/argocd-darwin-amd64 ./cmd/argocd
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 dist/packr build -v -i -ldflags '${LDFLAGS}' -o ${DIST_DIR}/argocd-windows-amd64.exe ./cmd/argocd
	cp Dockerfile.dev dist
	docker build -t $(IMAGE_PREFIX)argocd:$(IMAGE_TAG) -f dist/Dockerfile.dev dist
else
image:
	docker build -t $(IMAGE_PREFIX)argocd:$(IMAGE_TAG) .
endif
	@if [ "$(DOCKER_PUSH)" = "true" ] ; then docker push $(IMAGE_PREFIX)argocd:$(IMAGE_TAG) ; fi

.PHONY: builder-image
builder-image:
	docker build  -t $(IMAGE_PREFIX)argo-cd-ci-builder:$(IMAGE_TAG) --target builder .
	@if [ "$(DOCKER_PUSH)" = "true" ] ; then docker push $(IMAGE_PREFIX)argo-cd-ci-builder:$(IMAGE_TAG) ; fi

.PHONY: dep
dep:
	dep ensure -v

.PHONY: dep-ensure
dep-ensure:
	dep ensure -no-vendor

.PHONY: install-lint-tools
install-lint-tools:
	./hack/install.sh lint-tools

.PHONY: lint
lint:
	golangci-lint --version
	# NOTE: If you get a "Killed" OOM message, try reducing the value of GOGC
	# See https://github.com/golangci/golangci-lint#memory-usage-of-golangci-lint
	GOGC=100 golangci-lint run --fix --verbose

.PHONY: build
build:
	go build -v `go list ./... | grep -v 'resource_customizations\|test/e2e'`

.PHONY: test
test:
	./hack/test.sh -coverprofile=coverage.out `go list ./... | grep -v 'test/e2e'`

.PHONY: test-e2e
test-e2e:
	# NO_PROXY ensures all tests don't go out through a proxy if one is configured on the test system
	NO_PROXY=* ./hack/test.sh -timeout 15m -v ./test/e2e

.PHONY: start-e2e
start-e2e: cli
	killall goreman || true
	# check we can connect to Docker to start Redis
	docker version
	kubectl create ns argocd-e2e || true
	kubectl config set-context --current --namespace=argocd-e2e
	kustomize build test/manifests/base | kubectl apply -f -
	# set paths for locally managed ssh known hosts and tls certs data
	ARGOCD_SSH_DATA_PATH=/tmp/argo-e2e/app/config/ssh \
	ARGOCD_TLS_DATA_PATH=/tmp/argo-e2e/app/config/tls \
	ARGOCD_E2E_DISABLE_AUTH=false \
	ARGOCD_ZJWT_FEATURE_FLAG=always \
		goreman start

# Cleans VSCode debug.test files from sub-dirs to prevent them from being included in packr boxes
.PHONY: clean-debug
clean-debug:
	-find ${CURRENT_DIR} -name debug.test | xargs rm -f

.PHONY: clean
clean: clean-debug
	-rm -rf ${CURRENT_DIR}/dist

.PHONY: start
start:
	killall goreman || true
	# check we can connect to Docker to start Redis
	docker version
	kubectl create ns argocd || true
	kubens argocd
	ARGOCD_ZJWT_FEATURE_FLAG=always \
		goreman start

.PHONY: pre-commit
pre-commit: dep-ensure codegen build lint test

.PHONY: release-precheck
release-precheck: manifests
	@if [ "$(GIT_TREE_STATE)" != "clean" ]; then echo 'git tree state is $(GIT_TREE_STATE)' ; exit 1; fi
	@if [ -z "$(GIT_TAG)" ]; then echo 'commit must be tagged to perform release' ; exit 1; fi
	@if [ "$(GIT_TAG)" != "v`cat VERSION`" ]; then echo 'VERSION does not match git tag'; exit 1; fi

.PHONY: release
release: pre-commit release-precheck image release-cli

.PHONY: build-docs
build-docs:
	mkdocs build

.PHONY: serve-docs
serve-docs:
	mkdocs serve

.PHONY: lint-docs
lint-docs:
	#  https://github.com/dkhamsing/awesome_bot
	find docs -name '*.md' -exec grep -l http {} + | xargs docker run --rm -v $(PWD):/mnt:ro dkhamsing/awesome_bot -t 3 --allow-dupe --allow-redirect --white-list `cat white-list | grep -v "#" | tr "\n" ','` --skip-save-results --

.PHONY: publish-docs
publish-docs: lint-docs
	mkdocs gh-deploy
