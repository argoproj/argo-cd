# Plugins

Argo CD allows integrating more config management tools using config management plugins. 

## Configure plugins via Argo CD configmap
Following changes are required to configure new plugin:

* Make sure required binaries are available in `argocd-repo-server` pod. The binaries can be added via volume mounts or using custom image (see [custom_tools](../operator-manual/custom_tools.md)).
* Register a new plugin in `argocd-cm` ConfigMap:

```yaml
data:
  configManagementPlugins: |
    - name: pluginName
      init:                          # Optional command to initialize application source directory
        command: ["sample command"]
        args: ["sample args"]
      generate:                      # Command to generate manifests YAML
        command: ["sample command"]
        args: ["sample args"]
```

The `generate` command must print a valid YAML stream to stdout. Both `init` and `generate` commands are executed inside the application source directory.

 * Create an application and specify required config management plugin name.

```bash
argocd app create <appName> --config-management-plugin <pluginName>
```

More config management plugin examples are available in [argocd-example-apps](https://github.com/argoproj/argocd-example-apps/tree/master/plugins).

## Configure plugin via sidecar

As an effort to provide first-class support for additional plugin tools, we have enhanced the feature where an operator 
can configure additional plugin tool via sidecar to repo-server. Following changes are required to configure new plugin:

### Register plugin sidecar

To install a plugin, simply patch argocd-repo-server to run config management plugin container as a sidecar, with argocd-cmp-server as it’s entrypoint. 
You can use either off-the-shelf or custom built plugin image as sidecar image. For example:
 
```yaml
- command: [/var/run/argocd/argocd-cmp-server] # Entrypoint should be Argo CD lightweight API server i.e. argocd-cmp-server
  image: k8s.gcr.io/kustomize/kustomize:v4.0.5 # This can be off-the-shelf or custom built image
  name: kustomize
  securityContext:
    runAsNonRoot: true
    runAsUser: 999
  volumeMounts:
  - mountPath: /var/run/argocd
    name: var-files
  - mountPath: /home/argocd/cmp-server/plugins
    name: plugins
  - mountPath: /home/argocd/cmp-server/config/kustomize.yaml # Plugin config file can either be volume mapped or baked into image
    subPath: kustomize.yaml
    name: config-files
  - mountPath: /tmp
    name: tmp-dir
```
 
 * Make sure that entrypoint is Argo CD lightweight API server i.e. argocd-cmp-server
 * Make sure that sidecar container is running as user 999
 * Make sure that plugin configuration file is present at `/home/argocd/cmp-server/config/`. It can either be volume mapped via configmap or baked into image

### Plugin configuration file

Plugins will be configured via a ConfigManagementPlugin manifest located inside the plugin container, placed at a well-known location 
(e.g. /home/argocd/cmp-server/plugins/plugin.yaml). Argo CD is agnostic to the mechanism of how the plugin.yaml would be placed, 
but various options can be used on how to place this file, including: 
- Baking the file into the plugin image as part of docker build
- Volume mapping the file through a configmap.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ConfigManagementPlugin
metadata:
  name: kustomize
spec:
  version: v1.0
  init:
    command: [kustomize, version]
  generate:
    command: [kustomize, build]
  find:
    command: [sh, -c, find . -name kustomization.yaml]
    glob: "**/kustomization.yaml"
  check:
    command: [-f ./kustomization.yaml]
    glob: "**/kustomization.yaml"
  allowConcurrency: true
  lockRepo: false
```

Note that, while the ConfigManagementPlugin looks like a Kubernetes object, it is not actually a custom resource. 
It only follows kubernetes-style spec conventions.

The `generate` command must print a valid YAML stream to stdout. Both `init` and `generate` commands are executed inside the application source directory.

The `find` command is executed in order to determine whether application repository is supported by the plugin or not. The `find` command should returns
non-error response in case when application source type is supported. 

If your plugin makes use of `git` (e.g. `git crypt`), it is advised to set `lockRepo` to `true` so that your plugin will have exclusive access to the
repository at the time it is executed. Otherwise, two applications synced at the same time may result in a race condition and sync failure.

### Volume map plugin configuration file via configmap

If you are volume mapping the plugin configuration file through a configmap. Register a new plugin configuration file in `argocd-cmp-cm` configmap. 
For example:

```yaml
data:
  kustomization.yaml: |
    apiVersion: argoproj.io/v1alpha1
    kind: ConfigManagementPlugin
    metadata:
      name: kustomize
    spec:
      version: v1.0
      init:
        command: [kustomize, version]
      generate:
        command: [kustomize, build]
      find:
        command: [sh, -c, find . -name kustomization.yaml]
        glob: "**/kustomization.yaml"
      check:
        command: [-f ./kustomization.yaml]
        glob: "**/kustomization.yaml"
      allowConcurrency: true
      lockRepo: false
```

### Create application

Create an application and specify required config management plugin name. By default, Argo CD searches for registered plugins in `argocd-cm` for
manifests generation. If plugin is not found in `argocd-cm` configmap then Argo CD will look for registered sidecars in argocd-repo-server.

```bash
argocd app create <appName> --config-management-plugin <pluginName> --plugin-env key1=value1
```

## Environment

Commands have access to

1. The system environment variables
2. [Standard build environment](build-environment.md)
3. Variables in the application spec (References to system and build variables will get interpolated in the variables' values):

> v1.2

```yaml
spec:
  source:
    plugin:
      env:
        - name: FOO
          value: bar
        - name: REV
          value: test-$ARGOCD_APP_REVISION
```
