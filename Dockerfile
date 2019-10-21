ARG BASE_IMAGE=debian:10-slim
####################################################################################################
# Builder image
# Initial stage which pulls prepares build dependencies and CLI tooling we need for our final image
# Also used as the image in CI jobs so needs all dependencies
####################################################################################################
FROM golang:1.12.6 as builder

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
    zip && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

WORKDIR /tmp

ADD hack/install.sh .
ADD hack/installers installers

RUN ./install.sh dep-linux
RUN ./install.sh packr-linux
RUN ./install.sh kubectl-linux
RUN ./install.sh ksonnet-linux
RUN ./install.sh helm-linux
RUN ./install.sh kustomize-linux
RUN ./install.sh aws-iam-authenticator-linux

####################################################################################################
# Argo CD Base - used as the base for both the release and dev argocd images
####################################################################################################
FROM $BASE_IMAGE as argocd-base

USER root

RUN echo 'deb http://deb.debian.org/debian buster-backports main' >> /etc/apt/sources.list

RUN groupadd -g 999 argocd && \
    useradd -r -u 999 -g argocd argocd && \
    mkdir -p /home/argocd && \
    chown argocd:0 /home/argocd && \
    chmod g=u /home/argocd && \
    chmod g=u /etc/passwd && \
    apt-get update && \
    apt-get install -y git git-lfs && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

COPY hack/git-ask-pass.sh /usr/local/bin/git-ask-pass.sh
COPY --from=builder /usr/local/bin/ks /usr/local/bin/ks
COPY --from=builder /usr/local/bin/helm /usr/local/bin/helm
COPY --from=builder /usr/local/bin/kubectl /usr/local/bin/kubectl
COPY --from=builder /usr/local/bin/kustomize /usr/local/bin/kustomize
COPY --from=builder /usr/local/bin/aws-iam-authenticator /usr/local/bin/aws-iam-authenticator
# script to add current (possibly arbitrary) user to /etc/passwd at runtime
# (if it's not already there, to be openshift friendly)
COPY uid_entrypoint.sh /usr/local/bin/uid_entrypoint.sh

# support for mounting configuration from a configmap
RUN mkdir -p /app/config/ssh && \
    touch /app/config/ssh/ssh_known_hosts && \
    ln -s /app/config/ssh/ssh_known_hosts /etc/ssh/ssh_known_hosts 

RUN mkdir -p /app/config/tls

# workaround ksonnet issue https://github.com/ksonnet/ksonnet/issues/298
ENV USER=argocd

USER argocd
WORKDIR /home/argocd

####################################################################################################
# Argo CD UI stage
####################################################################################################
FROM node:11.15.0 as argocd-ui

WORKDIR /src
ADD ["ui/package.json", "ui/yarn.lock", "./"]

RUN yarn install

ADD ["ui/", "."]

ARG ARGO_VERSION=latest
ENV ARGO_VERSION=$ARGO_VERSION
RUN NODE_ENV='production' yarn build

####################################################################################################
# Argo CD Build stage which performs the actual build of Argo CD binaries
####################################################################################################
FROM golang:1.12.6 as argocd-build

COPY --from=builder /usr/local/bin/dep /usr/local/bin/dep
COPY --from=builder /usr/local/bin/packr /usr/local/bin/packr

# A dummy directory is created under $GOPATH/src/dummy so we are able to use dep
# to install all the packages of our dep lock file
COPY Gopkg.toml ${GOPATH}/src/dummy/Gopkg.toml
COPY Gopkg.lock ${GOPATH}/src/dummy/Gopkg.lock

RUN cd ${GOPATH}/src/dummy && \
    dep ensure -vendor-only && \
    mv vendor/* ${GOPATH}/src/ && \
    rmdir vendor

# Perform the build
WORKDIR /go/src/github.com/argoproj/argo-cd
COPY . .
RUN make cli server controller repo-server argocd-util && \
    make CLI_NAME=argocd-darwin-amd64 GOOS=darwin cli


####################################################################################################
# Final image
####################################################################################################
FROM argocd-base
COPY --from=argocd-build /go/src/github.com/argoproj/argo-cd/dist/argocd* /usr/local/bin/
COPY --from=argocd-ui ./src/dist/app /shared/app

