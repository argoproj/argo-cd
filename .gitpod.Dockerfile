FROM gitpod/workspace-full@sha256:599529108226a86194656e7ed3f284f1e53cfc1cc4d79210f94e1e60f98eca98

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
