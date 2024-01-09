FROM gitpod/workspace-full@sha256:bb84c4490170fbfdeb577535a6dec5aafd1eb8fdf433f9a344bd8f4a69af5707

USER root

RUN curl -o /usr/local/bin/kubectl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
    chmod +x /usr/local/bin/kubectl

RUN curl -L https://github.com/kubernetes-sigs/kubebuilder/releases/download/v2.3.1/kubebuilder_2.3.1_$(go env GOOS)_$(go env GOARCH).tar.gz | \
    tar -xz -C /tmp/ && mv /tmp/kubebuilder_2.3.1_$(go env GOOS)_$(go env GOARCH) /usr/local/kubebuilder

ENV GOCACHE=/go-build-cache

RUN apt-get install redis-server -y
RUN go install github.com/mattn/goreman@latest

RUN chown -R gitpod:gitpod /go-build-cache

USER gitpod

ENV ARGOCD_REDIS_LOCAL=true
ENV KUBECONFIG=/tmp/kubeconfig
