# Custom Tooling

Argo CD bundles preferred versions of its supported templating tools (helm, kustomize, ks, jsonnet)
as part of its container images. Sometimes, it may be desired to use a specific version of a tool
other than what Argo CD bundles. Some reasons to do this might be:

* To upgrade/downgrade to a specific version of a tool due to bugs or bug fixes.
* To install additional dependencies which to be used by kustomize's configmap/secret generators
  (e.g. curl, vault)

As the Argo CD repo-server is the single service responsible for generating Kubernetes manifests, it
can be customized to use alternative toolchain required by your environment.

The following example describes how the repo-server manifest can be customized to use a different
version of helm than what is bundled in Argo CD:

```yaml
    spec:
      # 1. Define an emptyDir volume which will hold the custom binaries
      volumes:
      - name: custom-tools
        emptyDir: {}
      # 2. Use an init container to download and/or copy the custom binaries into the emptyDir
      initContainers:
      - name: download-tools
        image: lachlanevenson/k8s-helm:v2.10.0
        command: [cp, /usr/local/bin/helm, /custom-tools]
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
