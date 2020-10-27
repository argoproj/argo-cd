FROM redis:5.0.8 as redis

FROM node:12.18.4 as node

FROM golang:1.14.1 as golang

FROM debian:10-slim

RUN echo 'deb http://deb.debian.org/debian buster-backports main' >> /etc/apt/sources.list && \
    apt-get update && apt-get install -y \
        ca-certificates \
        curl \
        openssh-server \
        nginx \
        fcgiwrap \
        git \
        git-lfs \
        gpg \
        make \
        wget \
        gcc \
        sudo \
        zip && \
        apt-get clean && \
        rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

COPY --from=golang /usr/local/go /usr/local/go

ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOROOT /usr/local/go
ENV GOPATH /go

# Install build and test dependencies
ADD ./hack/tool-versions.sh .
ADD ./hack/install.sh .
ADD ./hack/installers installers

RUN ./install.sh dep-linux && \
    ./install.sh packr-linux && \
    ./install.sh kubectl-linux && \
    ./install.sh ksonnet-linux && \
    ./install.sh helm2-linux && \
    ./install.sh helm-linux && \
    ./install.sh kustomize-linux && \
    ./install.sh codegen-tools && \
    ./install.sh codegen-go-tools && \
    ./install.sh lint-tools && \
    go get github.com/mattn/goreman && \
    go get github.com/kisielk/godepgraph && \
    go get github.com/jstemmer/go-junit-report && \
    rm -rf /tmp/dl && \
    rm -rf /tmp/helm && \
    rm -rf /tmp/helm2 && \
    rm -rf /tmp/ks_* && \
    rm -rf /tmp/packr

# These are required for running end-to-end tests
COPY ./test/fixture/testrepos/id_rsa.pub /root/.ssh/authorized_keys
COPY ./test/fixture/testrepos/nginx.conf /etc/nginx/nginx.conf
COPY ./test/fixture/testrepos/sudoers.conf /etc/sudoers
COPY ./test/fixture/testrepos/ssh_host_*_key* /etc/ssh/

# Copy redis binaries to the image
COPY --from=redis /usr/local/bin/* /usr/local/bin/

# Copy node binaries
COPY --from=node /usr/local/lib/node_modules /usr/local/lib/node_modules
COPY --from=node /usr/local/bin/node /usr/local/bin
COPY --from=node /opt/yarn-v1.22.4 /opt/yarn-v1.22.4

# Entrypoint is required for container's user management
COPY ./test/container/uid_entrypoint.sh /usr/local/bin

ARG UID

# Prepare user configuration & build environments
RUN useradd -l -u ${UID} -d /home/user -s /bin/bash user && \
    echo "user ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/user && \
    HELM_HOME=/home/user/.helm helm2 init --client-only && \
    HOME=/home/user git config --global user.name "ArgoCD Test User" && \
    HOME=/home/user git config --global user.email "noreply@example.com" && \
    mkdir -p /go/pkg && chmod 777 /go/pkg && \
    mkdir -p /home/user/.kube && chmod 777 /home/user/.kube && \
    mkdir -p /var/run/sshd && \
    mkdir -p /root/.ssh && \
    mkdir -p /go && \
    chmod -R 777 /go && \
    chown root /etc/ssh/ssh_host_*_key* && \
    chmod 0600 /etc/ssh/ssh_host_*_key && \
    mkdir -p /tmp/go-build-cache && chmod -R 777 /tmp/go-build-cache && \
    mkdir -p /home/user/.cache && chmod 777 /home/user/.cache && \
    chown -R user /home/user && \
    chgrp -R user /home/user && \
    chmod -R 777 /usr/local/go && \
    ln -s /usr/local/bin/node /usr/local/bin/nodejs && \
    ln -s /usr/local/lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm && \
    ln -s /usr/local/lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx && \
    ln -s /opt/yarn-v1.22.4/bin/yarn /usr/local/bin/yarn && \
    ln -s /opt/yarn-v1.22.4/bin/yarnpkg /usr/local/bin/yarnpkg

ENTRYPOINT ["/usr/local/bin/uid_entrypoint.sh"]
