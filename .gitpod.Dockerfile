FROM gitpod/workspace-full@sha256:dc72512ec88e7263d7cdbcb374bd33ccc4f9e7bddb46c0f65501131d5a2d6a76

USER root

RUN curl -o /usr/local/bin/kubectl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
    chmod +x /usr/local/bin/kubectl

RUN curl -L https://github.com/kubernetes-sigs/kubebuilder/releases/download/v2.3.1/kubebuilder_2.3.1_$(go env GOOS)_$(go env GOARCH).tar.gz | \
    tar -xz -C /tmp/ && mv /tmp/kubebuilder_2.3.1_$(go env GOOS)_$(go env GOARCH) /usr/local/kubebuilder

ENV GOCACHE=/go-build-cache

RUN apt-get install redis-server -y
RUN go install github.com/mattn/goreman@latest

USER gitpod

ENV ARGOCD_REDIS_LOCAL=true
ENV KUBECONFIG=/tmp/kubeconfig
