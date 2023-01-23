# Custom Tooling

Argo CD bundles preferred versions of its supported templating tools (helm, kustomize, ks, jsonnet)
as part of its container images. Sometimes, it may be desired to use a specific version of a tool
other than what Argo CD bundles. Some reasons to do this might be:

* To upgrade/downgrade to a specific version of a tool due to bugs or bug fixes.
* To install additional dependencies to be used by kustomize's configmap/secret generators.
  (e.g. curl, vault, gpg, AWS CLI)
* To install a [config management plugin](config-management-plugins.md).

As the Argo CD repo-server is the single service responsible for generating Kubernetes manifests, it
can be customized to use alternative toolchain required by your environment.

## Adding Tools Via Volume Mounts

The first technique is to use an `init` container and a `volumeMount` to copy a different version of
a tool into the repo-server container. In the following example, an init container is overwriting
the helm binary with a different version than what is bundled in Argo CD:

```yaml
    spec:
      # 1. Define an emptyDir volume which will hold the custom binaries
      volumes:
      - name: custom-tools
        emptyDir: {}
      # 2. Use an init container to download/copy custom binaries into the emptyDir
      initContainers:
      - name: download-tools
        image: alpine:3.8
        command: [sh, -c]
        args:
        - wget -qO- https://storage.googleapis.com/kubernetes-helm/helm-v2.12.3-linux-amd64.tar.gz | tar -xvzf - &&
          mv linux-amd64/helm /custom-tools/
        volumeMounts:
        - mountPath: /custom-tools
          name: custom-tools
      # 3. Volume mount the custom binary to the bin directory (overriding the existing version)
      containers:
      - name: argocd-repo-server
        volumeMounts:
        - mountPath: /usr/local/bin/helm
          name: custom-tools
          subPath: helm
```

## BYOI (Build Your Own Image)

Sometimes replacing a binary isn't sufficient, and you need to install other dependencies. The
following example builds an entirely customized repo-server from a Dockerfile, installing extra
dependencies that may be needed for generating manifests.

```Dockerfile
FROM argoproj/argocd:v2.5.4 # Replace tag with the appropriate argo version

# Switch to root for the ability to perform install
USER root

# Install tools needed for your repo-server to retrieve & decrypt secrets, render manifests 
# (e.g. curl, awscli, gpg, sops)
RUN apt-get update && \
    apt-get install -y \
        curl \
        awscli \
        gpg && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/* && \
    curl -o /usr/local/bin/sops -L https://github.com/mozilla/sops/releases/download/3.2.0/sops-3.2.0.linux && \
    chmod +x /usr/local/bin/sops

# Switch back to non-root user
USER $ARGOCD_USER_ID
```
