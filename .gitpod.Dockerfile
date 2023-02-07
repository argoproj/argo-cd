FROM gitpod/workspace-full

USER root

os=$(go env GOOS)
arch=$(go env GOARCH)
version=3.9.0

RUN curl -o /usr/local/bin/kubectl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
    chmod +x /usr/local/bin/kubectl

RUN curl -L https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${version}/kubebuilder_${os}_${arch} -o /usr/local/bin/kubebuilder

ENV GOCACHE=/go-build-cache

RUN apt-get install redis-server -y
RUN go install github.com/mattn/goreman@latest

USER gitpod

ENV ARGOCD_REDIS_LOCAL=true
ENV KUBECONFIG=/tmp/kubeconfig
