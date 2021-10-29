ARG BASE_IMAGE=docker.io/library/ubuntu:21.04
####################################################################################################
# Builder image
# Initial stage which pulls prepares build dependencies and CLI tooling we need for our final image
# Also used as the image in CI jobs so needs all dependencies
####################################################################################################
FROM docker.io/library/golang:1.16.5 as builder

RUN echo 'deb http://deb.debian.org/debian buster-backports main' >> /etc/apt/sources.list

RUN apt-get update && apt-get install -y \
    openssh-server \
    nginx \
    fcgiwrap \
    git \
    git-lfs \
    make \
    wget \
    gcc \
    sudo \
    zip && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

WORKDIR /tmp

ADD hack/install.sh .
ADD hack/installers installers
ADD hack/tool-versions.sh .

RUN ./install.sh ksonnet-linux
RUN ./install.sh helm2-linux
RUN ./install.sh helm-linux
RUN ./install.sh kustomize-linux

####################################################################################################
# Argo CD Base - used as the base for both the release and dev argocd images
####################################################################################################
FROM $BASE_IMAGE as argocd-base

USER root

ENV DEBIAN_FRONTEND=noninteractive

RUN groupadd -g 999 argocd && \
    useradd -r -u 999 -g argocd argocd && \
    mkdir -p /home/argocd && \
    chown argocd:0 /home/argocd && \
    chmod g=u /home/argocd && \
    apt-get update && \
    apt-get dist-upgrade -y && \
    apt-get install -y git git-lfs python3-pip tini gpg tzdata && \
    apt-get clean && \
    pip3 install awscli==1.18.80 && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

COPY hack/git-ask-pass.sh /usr/local/bin/git-ask-pass.sh
COPY hack/gpg-wrapper.sh /usr/local/bin/gpg-wrapper.sh
COPY hack/git-verify-wrapper.sh /usr/local/bin/git-verify-wrapper.sh
COPY --from=builder /usr/local/bin/ks /usr/local/bin/ks
COPY --from=builder /usr/local/bin/helm2 /usr/local/bin/helm2
COPY --from=builder /usr/local/bin/helm /usr/local/bin/helm
COPY --from=builder /usr/local/bin/kustomize /usr/local/bin/kustomize
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
# keep uid_entrypoint.sh for backward compatibility
RUN ln -s /usr/local/bin/entrypoint.sh /usr/local/bin/uid_entrypoint.sh

# support for mounting configuration from a configmap
RUN mkdir -p /app/config/ssh && \
    touch /app/config/ssh/ssh_known_hosts && \
    ln -s /app/config/ssh/ssh_known_hosts /etc/ssh/ssh_known_hosts 

RUN mkdir -p /app/config/tls
RUN mkdir -p /app/config/gpg/source && \
    mkdir -p /app/config/gpg/keys && \
    chown argocd /app/config/gpg/keys && \
    chmod 0700 /app/config/gpg/keys

# workaround ksonnet issue https://github.com/ksonnet/ksonnet/issues/298
ENV USER=argocd

USER 999
WORKDIR /home/argocd

####################################################################################################
# Argo CD UI stage
####################################################################################################
FROM docker.io/library/node:12.18.4 as argocd-ui

WORKDIR /src
ADD ["ui/package.json", "ui/yarn.lock", "./"]

RUN yarn install

ADD ["ui/", "."]

ARG ARGO_VERSION=latest
ENV ARGO_VERSION=$ARGO_VERSION
RUN NODE_ENV='production' NODE_ONLINE_ENV='online' yarn build

####################################################################################################
# Argo CD Build stage which performs the actual build of Argo CD binaries
####################################################################################################
FROM golang:1.16.5 as argocd-build

WORKDIR /go/src/github.com/argoproj/argo-cd

COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

# Perform the build
COPY . .
COPY --from=argocd-ui /src/dist/app /go/src/github.com/argoproj/argo-cd/ui/dist/app
RUN make argocd-all

ARG BUILD_ALL_CLIS=true
RUN if [ "$BUILD_ALL_CLIS" = "true" ] ; then \
    make BIN_NAME=argocd-darwin-amd64 GOOS=darwin argocd-all && \
    make BIN_NAME=argocd-windows-amd64.exe GOOS=windows argocd-all \
    ; fi

####################################################################################################
# Final image
####################################################################################################
FROM argocd-base
COPY --from=argocd-build /go/src/github.com/argoproj/argo-cd/dist/argocd* /usr/local/bin/

USER root
RUN ln -s /usr/local/bin/argocd /usr/local/bin/argocd-server
RUN ln -s /usr/local/bin/argocd /usr/local/bin/argocd-repo-server
RUN ln -s /usr/local/bin/argocd /usr/local/bin/argocd-application-controller
RUN ln -s /usr/local/bin/argocd /usr/local/bin/argocd-dex

USER 999
