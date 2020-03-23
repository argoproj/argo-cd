FROM redis:5.0.3 as redis

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

COPY ./test/fixture/testrepos/id_rsa.pub /root/.ssh/authorized_keys

COPY ./test/fixture/testrepos/nginx.conf /etc/nginx/nginx.conf

COPY ./test/fixture/testrepos/sudoers.conf /etc/sudoers

COPY ./test/fixture/testrepos/ssh_host_*_key* /etc/ssh/
RUN chown root /etc/ssh/ssh_host_*_key*
RUN chmod 0600 /etc/ssh/ssh_host_*_key

COPY ./test/container/uid_entrypoint.sh /usr/local/bin

COPY --from=docker /usr/local/bin/docker /usr/local/bin/docker

RUN chmod -R 777 /usr/local/go

RUN mkdir -p /tmp/go-build-cache && chmod -R 777 /tmp/go-build-cache

ENTRYPOINT ["/usr/local/bin/uid_entrypoint.sh"]