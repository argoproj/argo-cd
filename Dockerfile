ARG BASE_IMAGE=debian:9.5-slim
####################################################################################################
# Builder image
# Initial stage which pulls prepares build dependencies and CLI tooling we need for our final image
# Also used as the image in CI jobs so needs all dependencies
####################################################################################################
FROM golang:1.11.4 as builder

RUN apt-get update && apt-get install -y \
    git \
    make \
    wget \
    gcc \
    zip && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

WORKDIR /tmp

# Install docker
ENV DOCKER_CHANNEL stable
ENV DOCKER_VERSION 18.09.1
RUN wget -O docker.tgz "https://download.docker.com/linux/static/${DOCKER_CHANNEL}/x86_64/docker-${DOCKER_VERSION}.tgz" && \
    tar --extract --file docker.tgz --strip-components 1 --directory /usr/local/bin/ && \
    rm docker.tgz

# Install dep
ENV DEP_VERSION=0.5.0
RUN wget https://github.com/golang/dep/releases/download/v${DEP_VERSION}/dep-linux-amd64 -O /usr/local/bin/dep && \
    chmod +x /usr/local/bin/dep

# Install gometalinter
ENV GOMETALINTER_VERSION=2.0.12
RUN curl -sLo- https://github.com/alecthomas/gometalinter/releases/download/v${GOMETALINTER_VERSION}/gometalinter-${GOMETALINTER_VERSION}-linux-amd64.tar.gz | \
    tar -xzC "$GOPATH/bin" --exclude COPYING --exclude README.md --strip-components 1 -f- && \
    ln -s $GOPATH/bin/gometalinter $GOPATH/bin/gometalinter.v2

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

# Install kustomize
ENV KUSTOMIZE1_VERSION=1.0.11
RUN curl -L -o /usr/local/bin/kustomize1 https://github.com/kubernetes-sigs/kustomize/releases/download/v${KUSTOMIZE1_VERSION}/kustomize_${KUSTOMIZE1_VERSION}_linux_amd64 && \
    chmod +x /usr/local/bin/kustomize1 && \
    kustomize1 version


ENV KUSTOMIZE_VERSION=2.0.3
RUN curl -L -o /usr/local/bin/kustomize https://github.com/kubernetes-sigs/kustomize/releases/download/v${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_linux_amd64 && \
    chmod +x /usr/local/bin/kustomize && \
    kustomize version

# Install AWS IAM Authenticator
ENV AWS_IAM_AUTHENTICATOR_VERSION=0.4.0-alpha.1
RUN curl -L -o /usr/local/bin/aws-iam-authenticator https://github.com/kubernetes-sigs/aws-iam-authenticator/releases/download/${AWS_IAM_AUTHENTICATOR_VERSION}/aws-iam-authenticator_${AWS_IAM_AUTHENTICATOR_VERSION}_linux_amd64 && \
    chmod +x /usr/local/bin/aws-iam-authenticator

# Install golangci-lint
RUN wget https://install.goreleaser.com/github.com/golangci/golangci-lint.sh  && \
    chmod +x ./golangci-lint.sh && \
    ./golangci-lint.sh -b $GOPATH/bin && \
    golangci-lint linters

COPY .golangci.yml ${GOPATH}/src/dummy/.golangci.yml

RUN cd ${GOPATH}/src/dummy && \
    touch dummy.go \
    golangci-lint run

####################################################################################################
# Argo CD Base - used as the base for both the release and dev argocd images
####################################################################################################
FROM $BASE_IMAGE as argocd-base

USER root

RUN groupadd -g 999 argocd && \
    useradd -r -u 999 -g argocd argocd && \
    mkdir -p /home/argocd && \
    chown argocd:argocd /home/argocd && \
    apt-get update && \
    apt-get install -y git && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

COPY hack/ssh_known_hosts /etc/ssh/ssh_known_hosts
COPY hack/git-ask-pass.sh /usr/local/bin/git-ask-pass.sh
COPY --from=builder /usr/local/bin/ks /usr/local/bin/ks
COPY --from=builder /usr/local/bin/helm /usr/local/bin/helm
COPY --from=builder /usr/local/bin/kubectl /usr/local/bin/kubectl
COPY --from=builder /usr/local/bin/kustomize1 /usr/local/bin/kustomize1
COPY --from=builder /usr/local/bin/kustomize /usr/local/bin/kustomize
COPY --from=builder /usr/local/bin/aws-iam-authenticator /usr/local/bin/aws-iam-authenticator

# workaround ksonnet issue https://github.com/ksonnet/ksonnet/issues/298
ENV USER=argocd

USER argocd
WORKDIR /home/argocd


####################################################################################################
# Argo CD Build stage which performs the actual build of Argo CD binaries
####################################################################################################
FROM golang:1.11.4 as argocd-build

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
