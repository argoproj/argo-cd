ARG BASE_IMAGE=debian:9.5-slim
####################################################################################################
# Builder image
# Initial stage which pulls prepares build dependencies and CLI tooling we need for our final image
# Also used as the image in CI jobs so needs all dependencies
####################################################################################################
FROM golang:1.12.6 as builder

RUN echo 'deb http://deb.debian.org/debian stretch-backports main' >> /etc/apt/sources.list

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

# Install dep
ENV DEP_VERSION=0.5.0
RUN wget https://github.com/golang/dep/releases/download/v${DEP_VERSION}/dep-linux-amd64 -O /usr/local/bin/dep && \
    chmod +x /usr/local/bin/dep

# Install packr
ENV PACKR_VERSION=1.21.9
RUN wget https://github.com/gobuffalo/packr/releases/download/v${PACKR_VERSION}/packr_${PACKR_VERSION}_linux_amd64.tar.gz && \
    tar -vxf packr*.tar.gz -C /tmp/ && \
    mv /tmp/packr /usr/local/bin/packr

# Install kubectl
# NOTE: keep the version synced with https://storage.googleapis.com/kubernetes-release/release/stable.txt
ENV KUBECTL_VERSION=1.14.0
RUN curl -L -o /usr/local/bin/kubectl -LO https://storage.googleapis.com/kubernetes-release/release/v${KUBECTL_VERSION}/bin/linux/amd64/kubectl && \
    chmod +x /usr/local/bin/kubectl && \
    kubectl version --client

# Install ksonnet
ENV KSONNET_VERSION=0.13.1
RUN wget https://github.com/ksonnet/ksonnet/releases/download/v${KSONNET_VERSION}/ks_${KSONNET_VERSION}_linux_amd64.tar.gz && \
    tar -C /tmp/ -xf ks_${KSONNET_VERSION}_linux_amd64.tar.gz && \
    mv /tmp/ks_${KSONNET_VERSION}_linux_amd64/ks /usr/local/bin/ks && \
    ks version

# Install helm
ENV HELM_VERSION=2.12.1
RUN wget https://storage.googleapis.com/kubernetes-helm/helm-v${HELM_VERSION}-linux-amd64.tar.gz && \
    tar -C /tmp/ -xf helm-v${HELM_VERSION}-linux-amd64.tar.gz && \
    mv /tmp/linux-amd64/helm /usr/local/bin/helm && \
    helm version --client

ENV KUSTOMIZE_VERSION=3.1.0
RUN curl -L -o /usr/local/bin/kustomize https://github.com/kubernetes-sigs/kustomize/releases/download/v${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_linux_amd64 && \
    chmod +x /usr/local/bin/kustomize && \
    kustomize version

# Install AWS IAM Authenticator
ENV AWS_IAM_AUTHENTICATOR_VERSION=0.4.0-alpha.1
RUN curl -L -o /usr/local/bin/aws-iam-authenticator https://github.com/kubernetes-sigs/aws-iam-authenticator/releases/download/${AWS_IAM_AUTHENTICATOR_VERSION}/aws-iam-authenticator_${AWS_IAM_AUTHENTICATOR_VERSION}_linux_amd64 && \
    chmod +x /usr/local/bin/aws-iam-authenticator

####################################################################################################
# Argo CD Base - used as the base for both the release and dev argocd images
####################################################################################################
FROM $BASE_IMAGE as argocd-base

USER root

RUN echo 'deb http://deb.debian.org/debian stretch-backports main' >> /etc/apt/sources.list

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

