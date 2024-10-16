ARG BASE_IMAGE=docker.io/library/ubuntu:22.04@sha256:0bced47fffa3361afa981854fcabcd4577cd43cebbb808cea2b1f33a3dd7f508
####################################################################################################
# Builder image
# Initial stage which pulls prepares build dependencies and CLI tooling we need for our final image
# Also used as the image in CI jobs so needs all dependencies
####################################################################################################

FROM docker.io/library/golang:1.21.0@sha256:ec457a2fcd235259273428a24e09900c496d0c52207266f96a330062a01e3622 AS builder
ARG SOPS_VERSION="3.7.6-alpine"

RUN echo 'deb http://deb.debian.org/debian buster-backports main' >> /etc/apt/sources.list

RUN apt-get update && apt-get install --no-install-recommends -y \
    openssh-server \
    nginx \
    unzip \
    fcgiwrap \
    git \
    #git-lfs \
    make \
    wget \
    gcc \
    sudo \
    zip && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

WORKDIR /tmp

COPY hack/install.sh hack/tool-versions.sh ./
COPY hack/installers installers

RUN ./install.sh helm-linux
RUN INSTALL_PATH=/usr/local/bin ./install.sh kustomize
RUN ./install.sh kubectl-linux

COPY --from=quay.io/getsops/sops:v3.8.1-alpine /usr/local/bin/sops /usr/local/bin/sops

####################################################################################################
# Argo CD Base - used as the base for both the release and dev argocd images
####################################################################################################
FROM $BASE_IMAGE AS argocd-base

LABEL org.opencontainers.image.source="https://github.com/argoproj/argo-cd"

USER root

ENV ARGOCD_USER_ID=999
ENV DEBIAN_FRONTEND=noninteractive

RUN groupadd -g $ARGOCD_USER_ID argocd && \
    useradd -r -u $ARGOCD_USER_ID -g argocd argocd && \
    mkdir -p /home/argocd && \
    chown argocd:0 /home/argocd && \
    chmod g=u /home/argocd && \
    apt-get update && \
    apt-get dist-upgrade -y && \
    apt-get install -y \
    git tini gpg tzdata && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

COPY hack/gpg-wrapper.sh /usr/local/bin/gpg-wrapper.sh
COPY hack/git-verify-wrapper.sh /usr/local/bin/git-verify-wrapper.sh
COPY --from=builder /usr/local/bin/helm /usr/local/bin/helm
COPY --from=builder /usr/local/bin/kubectl /usr/local/bin/kubectl
COPY --from=builder /usr/local/bin/kustomize /usr/local/bin/kustomize
COPY --from=builder /usr/local/bin/sops /usr/local/bin/sops
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
# keep uid_entrypoint.sh for backward compatibility
RUN ln -s /usr/local/bin/entrypoint.sh /usr/local/bin/uid_entrypoint.sh

# support for mounting configuration from a configmap
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
FROM --platform=$BUILDPLATFORM docker.io/library/node:20.5.0@sha256:32ec50b65ac9572eda92baa6004a04dbbfc8021ea806fa62d37336183cad04e6 AS argocd-ui

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
FROM --platform=$BUILDPLATFORM docker.io/library/golang:1.21.0@sha256:ec457a2fcd235259273428a24e09900c496d0c52207266f96a330062a01e3622 AS argocd-build

WORKDIR /go/src/github.com/argoproj/argo-cd

COPY go.* ./
RUN go mod download

# Perform the build
COPY . .
COPY --from=argocd-ui /src/dist/app /go/src/github.com/argoproj/argo-cd/ui/dist/app
ARG TARGETOS
ARG TARGETARCH
# These build args are optional; if not specified the defaults will be taken from the Makefile
ARG GIT_TAG
ARG BUILD_DATE
ARG GIT_TREE_STATE
ARG GIT_COMMIT
RUN GIT_COMMIT=$GIT_COMMIT \
    GIT_TREE_STATE=$GIT_TREE_STATE \
    GIT_TAG=$GIT_TAG \
    BUILD_DATE=$BUILD_DATE \
    GOOS=$TARGETOS \
    GOARCH=$TARGETARCH \
    make argocd-all

####################################################################################################
# argocd-final
####################################################################################################
FROM argocd-base AS argocd-final
COPY --from=argocd-build /go/src/github.com/argoproj/argo-cd/dist/argocd* /usr/local/bin/

USER root
RUN ln -s /usr/local/bin/argocd /usr/local/bin/argocd-server && \
    ln -s /usr/local/bin/argocd /usr/local/bin/argocd-repo-server && \
    ln -s /usr/local/bin/argocd /usr/local/bin/argocd-cmp-server && \
    ln -s /usr/local/bin/argocd /usr/local/bin/argocd-application-controller && \
    ln -s /usr/local/bin/argocd /usr/local/bin/argocd-dex && \
    ln -s /usr/local/bin/argocd /usr/local/bin/argocd-notifications && \
    ln -s /usr/local/bin/argocd /usr/local/bin/argocd-applicationset-controller && \
    ln -s /usr/local/bin/argocd /usr/local/bin/argocd-k8s-auth

####################################################################################################
# Final Image
####################################################################################################

FROM argocd-final AS argocd

FROM amazon/aws-cli:2.11.19 AS awscli

FROM registry1.dso.mil/ironbank/opensource/alpinelinux/alpine:3.20

ARG HELM_SECRETS_VERSION="4.4.2"

ARG ARGOCD_USER_ID=999

ENV HOME=/home/argocd \
    USER=argocd \
    HELM_SECRETS_BACKEND="sops" \
    HELM_SECRETS_HELM_PATH=/usr/local/bin/helm \
    HELM_PLUGINS="/home/argocd/.local/share/helm/plugins/" \
    HELM_SECRETS_VALUES_ALLOW_SYMLINKS=false \
    HELM_SECRETS_VALUES_ALLOW_ABSOLUTE_PATH=false \
    HELM_SECRETS_VALUES_ALLOW_PATH_TRAVERSAL=false \
    HELM_SECRETS_WRAPPER_ENABLED=false \
    LIBGCRYPT_FORCE_FIPS_MODE=true

RUN addgroup -g $ARGOCD_USER_ID argocd && \
    adduser -D -u $ARGOCD_USER_ID -s /sbin/nologin -G argocd argocd && \
    chown argocd:argocd ${HOME} && \
    chmod g=u ${HOME} && \
    apk update && \
    apk upgrade && \
    apk upgrade musl && \
    apk add git git-lfs nss_wrapper openssl openssh-keysign tini gpg gpg-agent gcompat

COPY --from=argocd --chown=root:root /usr/local/bin/argocd /usr/local/bin/
COPY --from=argocd --chown=root:root /usr/local/bin/helm* /usr/local/bin/
COPY --from=argocd --chown=root:root /usr/local/bin/sops /usr/local/bin/
COPY --from=argocd --chown=root:root /usr/local/bin/kubectl /usr/local/bin/kubectl
COPY --from=argocd --chown=root:root /usr/local/bin/kustomize /usr/local/bin/kustomize
COPY --from=awscli --chown=root:root /usr/local/aws-cli /usr/local/aws-cli
COPY scripts/* /usr/local/bin/

RUN mkdir -p /app/config/ssh /app/config/tls && \
    mkdir -p /app/config/gpg/source /app/config/gpg/keys && \
    #mkdir -p /home/argocd/.kube && \
    #chown argocd /home/argocd/.kube && \
    #touch /home/argocd/.kube/config && \
    chown argocd:0 /app/config/gpg/keys && \
    chmod 0700 /app/config/gpg/keys && \
    chmod 0755 /usr/local/bin/*.sh && \
    touch /app/config/ssh/ssh_known_hosts && \
    ln -s /app/config/ssh/ssh_known_hosts /etc/ssh/ssh_known_hosts && \
    ln -s /usr/local/aws-cli/v2/current/bin/aws /usr/local/bin/aws && \
    ln -s /usr/local/aws-cli/v2/current/bin/aws_completer /usr/local/bin/aws_completer && \
    ln -s /usr/local/bin/argocd /usr/local/bin/argocd-server && \
    ln -s /usr/local/bin/argocd /usr/local/bin/argocd-repo-server && \
    ln -s /usr/local/bin/argocd /usr/local/bin/argocd-application-controller && \
    ln -s /usr/local/bin/argocd /usr/local/bin/argocd-dex && \
    ln -s /usr/local/bin/argocd /usr/local/bin/argocd-cmp-server && \
    ln -s /usr/local/bin/argocd /usr/local/bin/argocd-notifications && \
    ln -s /usr/local/bin/argocd /usr/local/bin/argocd-applicationset-controller && \
    ln -s /usr/local/bin/entrypoint.sh /usr/local/bin/uid_entrypoint.sh && \
    chmod -s /usr/lib/ssh/ssh-keysign

RUN chmod 750 -R /home/argocd

USER $ARGOCD_USER_ID

RUN helm plugin install --version ${HELM_SECRETS_VERSION} https://github.com/jkroepke/helm-secrets

USER root

RUN mkdir /usr/local/sbin && \
    chmod 0775 /usr/local/sbin

RUN ln -sf "$(helm env HELM_PLUGINS)/helm-secrets/scripts/wrapper/helm.sh" /usr/local/sbin/helm

USER $ARGOCD_USER_ID

WORKDIR ${HOME}

HEALTHCHECK --start-period=3s \
    CMD curl -f http://localhost:8080/healthz || exit 1

ENTRYPOINT ["entrypoint.sh"]
CMD ["argocd-server"]
