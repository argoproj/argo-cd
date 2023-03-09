FROM gitpod/workspace-full

ENV GOCACHE=/go-build-cache \
    ARGOCD_REDIS_LOCAL=true \
    KUBECONFIG=/tmp/kubeconfig

USER root

RUN curl https://dl.k8s.io/release/$(curl -sL https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl -o /usr/local/bin/kubectl && \
    chmod +x /usr/local/bin/kubectl && \
    curl -L https://github.com/kubernetes-sigs/kubebuilder/releases/download/v2.3.1/kubebuilder_2.3.1_$(go env GOOS)_$(go env GOARCH).tar.gz | tar -xz -C /tmp/ && \
    mv /tmp/kubebuilder_2.3.1_$(go env GOOS)_$(go env GOARCH) /usr/local/kubebuilder && \
    apt-get install -y redis-server && \
    go install github.com/mattn/goreman@latest && \
    rm -rf /var/lib/apt/lists/* /var/log/* "$(GOCACHE)"

USER gitpod
