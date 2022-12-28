FROM docker.io/library/redis:7.0.5 as redis

# There are libraries we will want to copy from here in the final stage of the
# build, but the COPY directive does not have a way to determine system
# architecture, so we create a symlink here to facilitate copying.
RUN ln -s /usr/lib/$(uname -m)-linux-gnu /usr/lib/linux-gnu 

FROM docker.io/library/node:12.18.4-buster as node

FROM docker.io/library/golang:1.18 as golang

FROM docker.io/library/registry:2.8 as registry

FROM docker.io/bitnami/kubectl:1.23.6 as kubectl

FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive
RUN  apt-get update && apt-get install --fix-missing -y \
        ca-certificates \
        curl \
        openssh-server \
        nginx \
        fcgiwrap \
        git \
        git-lfs \
        gpg \
        jq \
        make \
        wget \
        gcc \
        g++ \
        sudo \
        tini \
        zip && \
        apt-get clean && \
        rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

COPY --from=golang /usr/local/go /usr/local/go

COPY --from=kubectl /opt/bitnami/kubectl/bin/kubectl /usr/local/bin/kubectl

ENV PATH /dist:/go/bin:/usr/local/go/bin:/go/src/github.com/argoproj/argo-cd/dist:$PATH
ENV GOROOT /usr/local/go
ENV GOPATH /go

# Install build and test dependencies
COPY hack/install.sh hack/tool-versions.sh go.* ./
COPY hack/installers installers

RUN ./install.sh helm-linux && \
    ./install.sh kustomize && \
    ./install.sh codegen-tools && \
    ./install.sh codegen-go-tools && \
    ./install.sh lint-tools && \
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
COPY --from=redis /usr/lib/linux-gnu/libssl.so.1.1 /usr/lib/linux-gnu/
COPY --from=redis /usr/lib/linux-gnu/libcrypto.so.1.1 /usr/lib/linux-gnu/
RUN mv /usr/lib/linux-gnu/libssl.so.1.1 /usr/lib/$(uname -m)-linux-gnu/ && \
    mv /usr/lib/linux-gnu/libcrypto.so.1.1 /usr/lib/$(uname -m)-linux-gnu/ && \
    rm -rf /usr/lib/linux-gnu/

# Copy registry binaries to the image
COPY --from=registry /bin/registry /usr/local/bin/
COPY --from=registry /etc/docker/registry/config.yml /etc/docker/registry/config.yml

# Copy node binaries
COPY --from=node /usr/local/lib/node_modules /usr/local/lib/node_modules
COPY --from=node /usr/local/bin/node /usr/local/bin
COPY --from=node /opt/yarn-v1.22.4 /opt/yarn-v1.22.4

# Entrypoint is required for container's user management
COPY ./test/container/entrypoint.sh /usr/local/bin

ARG UID

# Prepare user configuration & build environments
RUN useradd -l -u ${UID} -d /home/user -s /bin/bash user && \
    echo "user ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/user && \
    mkdir -p /home/user/.kube && chmod 777 /home/user/.kube && \
    chown -R user /home/user && \
    chgrp -R user /home/user && \
    HOME=/home/user git config --global user.name "ArgoCD Test User" && \
    HOME=/home/user git config --global user.email "noreply@example.com" && \
    mkdir -p /go/pkg && chmod 777 /go/pkg && \
    mkdir -p /var/run/sshd && \
    mkdir -p /root/.ssh && \
    mkdir -p /go && \
    chmod -R 777 /go && \
    chown root /etc/ssh/ssh_host_*_key* && \
    chmod 0600 /etc/ssh/ssh_host_*_key && \
    mkdir -p /tmp/go-build-cache && chmod -R 777 /tmp/go-build-cache && \
    mkdir -p /home/user/.cache && chmod 777 /home/user/.cache && \
    chmod -R 777 /usr/local/go && \
    ln -s /usr/local/bin/node /usr/local/bin/nodejs && \
    ln -s /usr/local/lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm && \
    ln -s /usr/local/lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx && \
    ln -s /opt/yarn-v1.22.4/bin/yarn /usr/local/bin/yarn && \
    ln -s /opt/yarn-v1.22.4/bin/yarnpkg /usr/local/bin/yarnpkg && \
    mkdir -p /var/lib/registry && chmod -R 777 /var/lib/registry

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
