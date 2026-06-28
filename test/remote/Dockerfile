ARG BASE_IMAGE=docker.io/library/ubuntu:26.04@sha256:f3d28607ddd78734bb7f71f117f3c6706c666b8b76cbff7c9ff6e5718d46ff64

FROM docker.io/library/golang:1.26.4@sha256:792443b89f65105abba56b9bd5e97f680a80074ac62fc844a584212f8c8102c3 AS go

RUN go install github.com/mattn/goreman@latest && \
    go install github.com/kisielk/godepgraph@latest

FROM $BASE_IMAGE

ENV DEBIAN_FRONTEND=noninteractive

# renovate: datasource=deb depName=git registryUrl=https://archive.ubuntu.com/ubuntu?suite=resolute&components=main,security&binaryArch=amd64
ARG GIT_APT_VERSION=1:2.53.0-1ubuntu1

RUN  apt-get update && apt-get install --no-install-recommends -y \
    ca-certificates \
    curl \
    openssh-server \
    nginx \
    fcgiwrap \
    git=${GIT_APT_VERSION} \
    git-lfs \
    gpg \
    make \
    wget \
    gcc \
    sudo \
    zip && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# These are required for running end-to-end tests
COPY ./test/fixture/testrepos/id_rsa.pub /root/.ssh/authorized_keys
COPY ./test/fixture/testrepos/nginx.conf /etc/nginx/nginx.conf
COPY ./test/fixture/testrepos/.htpasswd /etc/nginx/.htpasswd
COPY ./test/fixture/testrepos/sudoers.conf /etc/sudoers
COPY ./test/fixture/testrepos/ssh_host_*_key* /etc/ssh/
COPY ./test/fixture/certs/argocd-e2e-server.crt /etc/certs/argocd-test-server.crt
COPY ./test/fixture/certs/argocd-e2e-server.key /etc/certs/argocd-test-server.key
COPY ./test/fixture/certs/argocd-test-ca.crt /etc/certs/argocd-test-ca.crt

# Entrypoint is required for container's user management
COPY ./test/remote/entrypoint.sh /usr/local/bin
COPY ./test/remote/Procfile /Procfile

# We need goreman
COPY --from=go /go/bin/goreman /usr/local/bin/goreman

COPY ./test/e2e/testdata/ /app/config/testdata/

# Prepare user configuration & build environments
RUN useradd -l -u 1000 -d /home/user -s /bin/bash user && \
    echo "user ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/user && \
    mkdir -p /home/user && \
    HOME=/home/user git config --global user.name "ArgoCD Test User" && \
    HOME=/home/user git config --global user.email "noreply@example.com" && \
    mkdir -p /var/run/sshd && \
    mkdir -p /root/.ssh && \
    chown -R user /home/user && \
    chgrp -R user /home/user && \
    chown root /etc/ssh/ssh_host_*_key* && \
    chmod 0600 /etc/ssh/ssh_host_*_key  && \
    mkdir -p /tmp/argo-e2e/testdata.git && \
    cd /tmp/argo-e2e/testdata.git && \
    HOME=/home/user git init --bare && \
    cd /app/config/testdata && \
    HOME=/home/user git init && \
    HOME=/home/user git add . && \
    HOME=/home/user git commit -a -m "Initial commit" && \
    HOME=/home/user git remote add origin file:///tmp/argo-e2e/testdata.git && \
    HOME=/home/user git push origin master && \
    mkdir -p /tmp/argo-e2e/submodule.git && \
    cd /tmp/argo-e2e/submodule.git && \
    HOME=/home/user git init --bare && \
    mkdir -p /tmp/argo-e2e/submoduleParent.git && \
    cd /tmp/argo-e2e/submoduleParent.git && \
    HOME=/home/user git init --bare && \
    chown -R user /tmp/argo-e2e/testdata.git && \
    chown -R user /tmp/argo-e2e/submodule.git && \
    chown -R user /tmp/argo-e2e/submoduleParent.git && \
    ln -s /usr/libexec/git-core /usr/lib/git-core

RUN echo "[http]" >> /tmp/argo-e2e/testdata.git/config && \
    echo "  receivepack = true" >> /tmp/argo-e2e/testdata.git/config 
RUN echo "[http]" >> /tmp/argo-e2e/submodule.git/config && \
    echo "  receivepack = true" >> /tmp/argo-e2e/submodule.git/config 
RUN echo "[http]" >> /tmp/argo-e2e/submoduleParent.git/config && \
    echo "  receivepack = true" >> /tmp/argo-e2e/submoduleParent.git/config 

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
