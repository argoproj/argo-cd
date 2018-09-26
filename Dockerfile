####################################################################################################
# Builder image
# Initial stage which pulls prepares build dependencies and CLI tooling we need for our final image
# Also used as the image in CI jobs so needs all dependencies
####################################################################################################
FROM golang:1.10.3 as builder

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
ENV DOCKER_VERSION=18.06.0
RUN curl -O https://download.docker.com/linux/static/stable/x86_64/docker-${DOCKER_VERSION}-ce.tgz && \
  tar -xzf docker-${DOCKER_VERSION}-ce.tgz && \
  mv docker/docker /usr/local/bin/docker && \
  rm -rf ./docker

# Install dep
ENV DEP_VERSION=0.5.0
RUN wget https://github.com/golang/dep/releases/download/v${DEP_VERSION}/dep-linux-amd64 -O /usr/local/bin/dep && \
    chmod +x /usr/local/bin/dep

# Install gometalinter
RUN curl -sLo- https://github.com/alecthomas/gometalinter/releases/download/v2.0.5/gometalinter-2.0.5-linux-amd64.tar.gz | \
    tar -xzC "$GOPATH/bin" --exclude COPYING --exclude README.md --strip-components 1 -f- && \
    ln -s $GOPATH/bin/gometalinter $GOPATH/bin/gometalinter.v2

# Install packr
ENV PACKR_VERSION=1.13.2
RUN wget https://github.com/gobuffalo/packr/releases/download/v${PACKR_VERSION}/packr_${PACKR_VERSION}_linux_amd64.tar.gz && \
    tar -vxf packr*.tar.gz -C /tmp/ && \
    mv /tmp/packr /usr/local/bin/packr

# Install kubectl
RUN curl -L -o /usr/local/bin/kubectl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl && \
    chmod +x /usr/local/bin/kubectl

# Install ksonnet
# NOTE: we frequently switch between tip of master ksonnet vs. official builds. Comment/uncomment
# the corresponding section to switch between the two options:
# Option 1: build ksonnet ourselves
#RUN go get -v -u github.com/ksonnet/ksonnet && mv ${GOPATH}/bin/ksonnet /usr/local/bin/ks
# Option 2: use official tagged ksonnet release
ENV KSONNET_VERSION=0.11.0
RUN wget https://github.com/ksonnet/ksonnet/releases/download/v${KSONNET_VERSION}/ks_${KSONNET_VERSION}_linux_amd64.tar.gz && \
    tar -C /tmp/ -xf ks_${KSONNET_VERSION}_linux_amd64.tar.gz && \
    mv /tmp/ks_${KSONNET_VERSION}_linux_amd64/ks /usr/local/bin/ks

# Install helm
ENV HELM_VERSION=2.9.1
RUN wget https://storage.googleapis.com/kubernetes-helm/helm-v${HELM_VERSION}-linux-amd64.tar.gz && \
    tar -C /tmp/ -xf helm-v${HELM_VERSION}-linux-amd64.tar.gz && \
    mv /tmp/linux-amd64/helm /usr/local/bin/helm

# Install kustomize
ENV KUSTOMIZE_VERSION=1.0.8
RUN curl -L -o /usr/local/bin/kustomize https://github.com/kubernetes-sigs/kustomize/releases/download/v${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_linux_amd64 && \
    chmod +x /usr/local/bin/kustomize

ENV AWS_IAM_AUTHENTICATOR_VERSION=0.3.0
RUN curl -L -o /usr/local/bin/aws-iam-authenticator https://github.com/kubernetes-sigs/aws-iam-authenticator/releases/download/v0.3.0/heptio-authenticator-aws_${AWS_IAM_AUTHENTICATOR_VERSION}_linux_amd64 && \
    chmod +x /usr/local/bin/aws-iam-authenticator


####################################################################################################
# ArgoCD Build stage which performs the actual build of ArgoCD binaries
####################################################################################################
FROM golang:1.10.3 as argocd-build

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
ARG MAKE_TARGET="cli server controller repo-server argocd-util"
RUN make ${MAKE_TARGET}


####################################################################################################
# Final image
####################################################################################################
FROM debian:9.5-slim

RUN groupadd -g 999 argocd && \
    useradd -r -u 999 -g argocd argocd && \
    mkdir -p /home/argocd && \
    chown argocd:argocd /home/argocd && \
    apt-get update && \
    apt-get install -y git && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

COPY --from=builder /usr/local/bin/ks /usr/local/bin/ks
COPY --from=builder /usr/local/bin/helm /usr/local/bin/helm
COPY --from=builder /usr/local/bin/kubectl /usr/local/bin/kubectl
COPY --from=builder /usr/local/bin/kustomize /usr/local/bin/kustomize
COPY --from=builder /usr/local/bin/aws-iam-authenticator /usr/local/bin/aws-iam-authenticator

# workaround ksonnet issue https://github.com/ksonnet/ksonnet/issues/298
ENV USER=argocd

COPY --from=argocd-build /go/src/github.com/argoproj/argo-cd/dist/* /usr/local/bin/

# Symlink argocd binaries under root for backwards compatibility that expect it under /
RUN ln -s /usr/local/bin/argocd /argocd && \
    ln -s /usr/local/bin/argocd-server /argocd-server && \
    ln -s /usr/local/bin/argocd-util /argocd-util && \
    ln -s /usr/local/bin/argocd-application-controller /argocd-application-controller && \
    ln -s /usr/local/bin/argocd-repo-server /argocd-repo-server

USER argocd

RUN helm init --client-only

WORKDIR /home/argocd
ARG BINARY
CMD ${BINARY}
