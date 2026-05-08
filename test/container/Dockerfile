FROM docker.io/library/redis:8.6.3@sha256:25dbb04fc4d6d190eda327dab551631500b0f9ac8f9808e8e63b7fda1ddff196 AS redis

# There are libraries we will want to copy from here in the final stage of the
# build, but the COPY directive does not have a way to determine system
# architecture, so we create a symlink here to facilitate copying.
RUN ln -s /usr/lib/$(uname -m)-linux-gnu /usr/lib/linux-gnu

# Please make sure to also check the contained yarn version and update the references below when upgrading this image's version
FROM docker.io/library/node:22.9.0@sha256:8398ea18b8b72817c84af283f72daed9629af2958c4f618fe6db4f453c5c9328 AS node

FROM docker.io/library/golang:1.26.3@sha256:efaccb5b497e90df3ebe5216cc25cd9f98e73874e2d638b56e38d4a3f098c41c AS golang

FROM docker.io/library/registry:3.1@sha256:85347ed2ecde64161c7a4788a4d7d3dcc9d6f86f7be95834022e3c6a423a945a AS registry

FROM docker.io/bitnamilegacy/kubectl:1.32@sha256:9524faf8e3cefb47fa28244a5d15f95ec21a73d963273798e593e61f80712333 AS kubectl

FROM docker.io/library/ubuntu:26.04@sha256:f3d28607ddd78734bb7f71f117f3c6706c666b8b76cbff7c9ff6e5718d46ff64

ENV DEBIAN_FRONTEND=noninteractive

# NOTE: binutils-gold contains the gold linker, which was recently removed
#       from binutils, but is still nesessary for building lint-tools on arm64 only
#       until this Golang issue is fixed: https://github.com/golang/go/issues/22040
RUN  apt-get update && apt-get install --fix-missing --no-install-recommends -y \
    ca-certificates \
    curl \
    openssh-server \
    nginx \
    fcgiwrap \
    git \
    gpg \
    gpg-agent \
    jq \
    make \
    wget \
    gcc \
    g++ \
    sudo \
    tini \
    unzip \
    zip && \
    if [ "$(uname -m)" = "aarch64" ]; then \
       apt-get install --fix-missing -y binutils-gold; \
    fi && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

COPY --from=golang /usr/local/go /usr/local/go

COPY --from=kubectl /opt/bitnami/kubectl/bin/kubectl /usr/local/bin/kubectl

ENV PATH=/dist:/go/bin:/usr/local/go/bin:/go/src/github.com/argoproj/argo-cd/dist:$PATH
ENV GOROOT=/usr/local/go
ENV GOPATH=/go

# Disable gRPC service config lookups via DNS TXT records to prevent excessive
# DNS queries for _grpc_config.<hostname> which can cause timeouts in dual-stack
# environments. This can be overridden via argocd-cmd-params-cm ConfigMap.
# See https://github.com/argoproj/argo-cd/issues/24991
ENV GRPC_ENABLE_TXT_SERVICE_CONFIG=false

# Install build and test dependencies
COPY hack/install.sh hack/tool-versions.sh go.* ./
COPY hack/installers installers
COPY gitops-engine/go.* ./gitops-engine/

RUN ./install.sh helm && \
    ./install.sh kustomize && \
    ./install.sh codegen-tools && \
    ./install.sh codegen-go-tools && \
    ./install.sh lint-tools && \
    ./install.sh gotestsum && \
    ./install.sh git-lfs && \
    go install github.com/mattn/goreman@latest && \
    go install github.com/kisielk/godepgraph@latest && \
    go install github.com/jstemmer/go-junit-report@latest && \
    rm -rf /tmp/dl && \
    rm -rf /tmp/helm && \
    rm -rf /tmp/ks_*

# These are required for running end-to-end tests
COPY ./test/fixture/testrepos/id_rsa.pub /root/.ssh/authorized_keys
COPY ./test/fixture/testrepos/nginx.conf /etc/nginx/nginx.conf
COPY ./test/fixture/testrepos/sudoers.conf /etc/sudoers
COPY ./test/fixture/testrepos/ssh_host_*_key* /etc/ssh/

# Copy redis binaries to the image
COPY --from=redis /usr/local/bin/* /usr/local/bin/

# Copy redis dependencies/shared libraries
# Ubuntu 22.04+ has moved to OpenSSL3 and no longer provides these libraries
COPY --from=redis /usr/lib/linux-gnu/libssl.so.3 /usr/lib/linux-gnu/
COPY --from=redis /usr/lib/linux-gnu/libcrypto.so.3 /usr/lib/linux-gnu/
RUN mv /usr/lib/linux-gnu/libssl.so.3 /usr/lib/$(uname -m)-linux-gnu/ && \
    mv /usr/lib/linux-gnu/libcrypto.so.3 /usr/lib/$(uname -m)-linux-gnu/ && \
    rm -rf /usr/lib/linux-gnu/

# Copy registry binaries to the image
COPY --from=registry /bin/registry /usr/local/bin/
COPY --from=registry /etc/distribution/config.yml /etc/docker/registry/config.yml

# Copy node binaries
COPY --from=node /usr/local/lib/node_modules /usr/local/lib/node_modules
COPY --from=node /usr/local/bin/node /usr/local/bin

# Entrypoint is required for container's user management
COPY ./test/container/entrypoint.sh /usr/local/bin

ARG UID

# Prepare user configuration & build environments
RUN userdel -r ubuntu && \
    useradd -l -u ${UID} -d /home/user -s /bin/bash user && \
    echo "user ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/user && \
    mkdir -p /home/user/.kube && \
    mkdir -p /home/user/.cache && \
    chown -R user /home/user && \
    chgrp -R user /home/user && \
    mkdir -p /go/src/github.com/gogo && \
    mkdir -p /go/src/k8s.io && \
    chown -R user /go && \
    HOME=/home/user git config --global user.name "ArgoCD Test User" && \
    HOME=/home/user git config --global user.email "noreply@example.com" && \
    HOME=/home/user git config --global --add safe.directory '*' && \
    mkdir -p /go/src && \
    mkdir -p /go/pkg && \
    chown -R user:user /go && \
    mkdir -p /var/run/sshd && \
    mkdir -p /root/.ssh && \
    chown root /etc/ssh/ssh_host_*_key* && \
    chmod 0600 /etc/ssh/ssh_host_*_key && \
    mkdir -p /tmp/go-build-cache && \
    chown -R user:user /tmp/go-build-cache && \
    ln -s /usr/local/bin/node /usr/local/bin/nodejs && \
    ln -s /usr/local/lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm && \
    ln -s /usr/local/lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx && \
    npm install -g corepack@0.34.6 && \
    corepack enable && \
    mkdir -p /var/lib/registry

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
