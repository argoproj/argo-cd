ARG BASE_IMAGE=debian:9.5-slim
####################################################################################################
# Builder image
# Initial stage which pulls prepares build dependencies and CLI tooling we need for our final image
# Also used as the image in CI jobs so needs all dependencies
####################################################################################################
FROM argoproj/argo-cd-ci-builder:v1.0.0 as builder

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
