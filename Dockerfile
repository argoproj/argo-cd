# Use sha256 hashes for reproducible builds
ARG BASE_IMAGE=docker.io/library/ubuntu:24.04@sha256:3f85b7caad41a95462cf5b787d8a04604c8262cdcdf9a472b8c52ef83375fe15
# ... (rest of the code for builder stage)

# Support for mounting configuration from a configmap
WORKDIR /app/config/ssh
RUN touch ssh_known_hosts && \
  ln -s /app/config/ssh/ssh_known_hosts /etc/ssh/ssh_known_hosts

WORKDIR /app/config
RUN mkdir -p tls && \
  mkdir -p gpg/source && \
  mkdir -p gpg/keys && \
  chown argocd gpg/keys && \
  chmod 0700 gpg/keys

ENV USER=argocd

USER $ARGOCD_USER_ID
WORKDIR /home/argocd

####################################################################################################
# Argo CD UI stage
####################################################################################################
FROM --platform=$BUILDPLATFORM docker.io/library/node:22.2.0@sha256:a8ba58f54e770a0f910ec36d25f8a4f1670e741a58c2e6358b2c30b575c84263 AS argocd-ui

WORKDIR /src
COPY ["ui/package.json", "ui/yarn.lock", "./"]

RUN yarn install --network-timeout 200000 && \
  yarn cache clean

COPY ["ui/", "."]

ARG ARGO_VERSION=latest
ENV ARGO_VERSION=$ARGO_VERSION
ARG TARGETARCH
RUN HOST_ARCH=$TARGETARCH NODE_ENV='production' NODE_ONLINE_ENV='online' NODE_OPTIONS=--max_old_space_size=8192 yarn build

####################################################################################################
# Argo CD Build stage which performs the actual build of Argo CD binaries
####################################################################################################
FROM --platform=$BUILDPLATFORM docker.io/library/golang:1.22.4@sha256:969349b8121a56d51c74f4c273ab974c15b3a8ae246a5cffc1df7d28b66cf978 AS argocd-build

WORKDIR /go/src/github.com/argoproj/argo-cd

COPY go.* ./
RUN go mod download

# Perform the build
COPY . .
COPY --from=argocd-ui /src/dist/app /go/src/github.com/argoproj/argo-cd/ui/dist/app

# Define build arguments outside ARG directive
ARG BUILD_DATE
ARG GIT_COMMIT
ARG GIT_TREE_STATE
ARG GIT_TAG
ARG TARGETOS
ARG TARGETARCH

RUN GIT_COMMIT=$GIT_COMMIT \
  GIT_TREE_STATE=$GIT_TREE_STATE \
  GIT_TAG=$GIT_TAG \
  BUILD_DATE=$BUILD_DATE \
  GOOS=$TARGETOS \
  GOARCH=$TARGETARCH \
  make argocd-all

####################################################################################################
# Final image
####################################################################################################
FROM argocd-base
COPY --from=argocd-build /go/src/github.com/argoproj/argo-cd/dist/argocd* /usr/local/bin/

USER root
RUN ln -s /usr/local/bin/argocd /usr/local/bin/argocd-server && \
  ln -s /usr/local/bin/argocd /usr/local/bin/argocd-repo-server && \
  ln -s /usr/local/bin/argocd /usr/local/bin/argocd-cmp-server && \
  ln -s /usr/local/bin/argocd /usr/local/bin/argocd-application-controller && \
  ln -s /usr/local/bin/argocd /usr/local/bin/argocd-dex && \
  ln -s /usr/local/bin/argocd /usr/local/bin/argocd-

