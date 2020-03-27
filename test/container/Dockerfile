FROM redis:5.0.3 as redis

FROM node:11.15.0 as node

FROM golang:1.14.0 

RUN echo 'deb http://deb.debian.org/debian buster-backports main' >> /etc/apt/sources.list

RUN apt-get update && apt-get install -y \
    ca-certificates \
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

# Install build and test dependencies
ADD ./hack/install.sh .
ADD ./hack/installers installers

RUN ./install.sh dep-linux
RUN ./install.sh packr-linux
RUN ./install.sh kubectl-linux
RUN ./install.sh ksonnet-linux
RUN ./install.sh helm2-linux
RUN ./install.sh helm-linux
RUN ./install.sh kustomize-linux

RUN ./install.sh codegen-tools
RUN ./install.sh codegen-go-tools
RUN ./install.sh lint-tools

# Create test user's home directory and initialize Helm v2
RUN mkdir -p /home/user && chmod 777 /home/user
RUN HELM_HOME=/home/user/.helm helm2 init --client-only

# Configure git to work correctly
RUN HOME=/home/user git config --global user.name "ArgoCD Test User"
RUN HOME=/home/user git config --global user.email "noreply@example.com"

RUN mkdir -p /go/pkg && chmod 777 /go/pkg

RUN apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# Install test dependency
RUN go get github.com/mattn/goreman

# For keeping 
RUN mkdir -p /home/user/.kube && chmod 777 /home/user/.kube

# Copy redis binaries to the image
COPY --from=redis /usr/local/bin/* /usr/local/bin/

# State dir for sshd
RUN mkdir -p /var/run/sshd

# Create root's .ssh directory for storing authorized_keys
RUN mkdir -p /root/.ssh

# These are required for running end-to-end tests
COPY ./test/fixture/testrepos/id_rsa.pub /root/.ssh/authorized_keys
COPY ./test/fixture/testrepos/nginx.conf /etc/nginx/nginx.conf
COPY ./test/fixture/testrepos/sudoers.conf /etc/sudoers
COPY ./test/fixture/testrepos/ssh_host_*_key* /etc/ssh/
RUN chown root /etc/ssh/ssh_host_*_key*
RUN chmod 0600 /etc/ssh/ssh_host_*_key

# Entrypoint is required for container's user management
COPY ./test/container/uid_entrypoint.sh /usr/local/bin
RUN chmod +x /usr/local/bin/uid_entrypoint.sh

# We need write access to Go libraries (ENABLE_CGO=0)
RUN chmod -R 777 /usr/local/go

# For mounting build cache into our container
RUN mkdir -p /tmp/go-build-cache && chmod -R 777 /tmp/go-build-cache
RUN mkdir -p /home/user/.cache && chmod 777 /home/user/.cache

# Copy node binaries and libraries and create appropriate symlinks
COPY --from=node /usr/local/lib/node_modules /usr/local/lib/node_modules
COPY --from=node /usr/local/bin/node /usr/local/bin
COPY --from=node /opt/yarn-v1.15.2 /opt/yarn-v1.15.2
RUN ln -s /usr/local/bin/node /usr/local/bin/nodejs 
RUN ln -s /usr/local/lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm
RUN ln -s /usr/local/lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx
RUN ln -s /opt/yarn-v1.15.2/bin/yarn /usr/local/bin/yarn
RUN ln -s /opt/yarn-v1.15.2/bin/yarnpkg /usr/local/bin/yarnpkg

ENTRYPOINT ["/usr/local/bin/uid_entrypoint.sh"]