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
      lockRepo: true                 # Defaults to false. See below.
```

The `generate` command must print a valid YAML stream to stdout. Both `init` and `generate` commands are executed inside the application source directory.

 * Create an application and specify required config management plugin name.

```bash
argocd app create <appName> --config-management-plugin <pluginName>
```

More config management plugin examples are available in [argocd-example-apps](https://github.com/argoproj/argocd-example-apps/tree/master/plugins).

!!!note "Repository locking"
    If your plugin makes use of `git` (e.g. `git crypt`), it is advised to set
    `lockRepo` to `true` so that your plugin will have exclusive access to the
    repository at the time it is executed. Otherwise, two applications synced
    at the same time may result in a race condition and sync failure.

## Configure plugin via sidecar

As an effort to provide first-class support for additional plugin tools, we have enhanced the feature where an operator 
can configure additional plugin tool via sidecar to repo-server. Following changes are required to configure new plugin:

### Register plugin sidecar

To install a plugin, simply patch argocd-repo-server to run config management plugin container as a sidecar, with argocd-cmp-server as it’s entrypoint. 
You can use either off-the-shelf or custom built plugin image as sidecar image. For example:

```yaml
containers:
- name: cmp
  command: [/var/run/argocd/argocd-cmp-server] # Entrypoint should be Argo CD lightweight CMP server i.e. argocd-cmp-server
  image: busybox # This can be off-the-shelf or custom built image
  securityContext:
    runAsNonRoot: true
    runAsUser: 999
  volumeMounts:
    - mountPath: /var/run/argocd
      name: var-files
    - mountPath: /home/argocd/cmp-server/plugins
      name: plugins
    - mountPath: /home/argocd/cmp-server/config/plugin.yaml # Plugin config file can either be volume mapped or baked into image
      subPath: plugin.yaml
      name: cmp-plugin
    - mountPath: /tmp
      name: tmp
  volumes:
    - configMap:
        name: cmp-plugin
      name: cmp-plugin
``` 
 
 * Make sure to use `/var/run/argocd/argocd-cmp-server` as an entrypoint. The `argocd-cmp-server` is a lightweight GRPC service that allows Argo CD to interact with the plugin.
 * Make sure that sidecar container is running as user 999
 * Make sure that plugin configuration file is present at `/home/argocd/cmp-server/config/pluging.yaml`. It can either be volume mapped via configmap or baked into image

### Plugin configuration file

Plugins will be configured via a ConfigManagementPlugin manifest located inside the plugin container, placed at `/home/argocd/cmp-server/config/plugin.yaml`. 
Argo CD is agnostic to the mechanism of how the configuration file would be placed, 
but various options can be used on how to place this file, including:

- Baking the file into the plugin image as part of docker build.
- Volume mapping the file through a configmap.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ConfigManagementPlugin
metadata:
  name: cmp-plugin
spec:
  version: v1.0
  generate:
    command: [sh, -c, 'echo "{\"kind\": \"ConfigMap\", \"apiVersion\": \"v1\", \"metadata\": { \"name\": \"$ARGOCD_APP_NAME\", \"namespace\": \"$ARGOCD_APP_NAMESPACE\", \"annotations\": {\"Foo\": \"$FOO\", \"KubeVersion\": \"$KUBE_VERSION\", \"KubeApiVersion\": \"$KUBE_API_VERSIONS\",\"Bar\": \"baz\"}}}"']
  discover:
    fileName: "./subdir/s*.yaml"
  allowConcurrency: true
  lockRepo: false
```

Note that, while the ConfigManagementPlugin looks like a Kubernetes object, it is not actually a custom resource. 
It only follows kubernetes-style spec conventions.

The `generate` command must print a valid YAML stream to stdout. Both `init` and `generate` commands are executed inside the application source directory.

The `discover.fileName` is used as matching pattern to determine whether application repository is supported by the plugin or not. 

```yaml
  discover:
    find:
      command: [sh, -c, find . -name env.yaml]
```
If `discover.fileName` is not provided, the `discover.find.command` is executed in order to determine whether application repository is supported by the plugin or not. The `find` command should returns
non-error response in case when application source type is supported. 

If your plugin makes use of `git` (e.g. `git crypt`), it is advised to set `lockRepo` to `true` so that your plugin will have exclusive access to the
repository at the time it is executed. Otherwise, two applications synced at the same time may result in a race condition and sync failure.

### Volume map plugin configuration file via configmap

The `plugin.yaml` file can be delivered using a configmap. Create a Kubernetes config map and with the `plugin.yaml`
key that holds the plugin configuration file:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cmp-plugin
data:
  plugin.yaml: |
    apiVersion: argoproj.io/v1alpha1
    kind: ConfigManagementPlugin
    metadata:
      name: cmp-plugin
    spec:
      version: v1.0
      generate:
        command: [sh, -c, 'echo "{\"kind\": \"ConfigMap\", \"apiVersion\": \"v1\", \"metadata\": { \"name\": \"$ARGOCD_APP_NAME\", \"namespace\": \"$ARGOCD_APP_NAMESPACE\", \"annotations\": {\"Foo\": \"$FOO\", \"KubeVersion\": \"$KUBE_VERSION\", \"KubeApiVersion\": \"$KUBE_API_VERSIONS\",\"Bar\": \"baz\"}}}"']
      discover:
        fileName: "./subdir/s*.yaml"
      allowConcurrency: true
      lockRepo: false
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
