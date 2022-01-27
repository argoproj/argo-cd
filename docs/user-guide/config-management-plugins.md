# Plugins

Argo CD allows integrating more config management tools using config management plugins.

## Installing a CMP

There are two ways to install a Config Management Plugin (CMP):
1. Add the plugin config to the Argo CD ConfigMap. The repo-server container will run your plugin's commands.

   This is a good option for a simple plugin that requires only a few lines of code that fit nicely in the Argo CD ConfigMap.
2. Add the plugin as a sidecar to the repo-server Pod.

   This is a good option for a more complex plugin that would clutter the Argo CD ConfigMap.

### Option 1: Configure plugins via Argo CD configmap

The following changes are required to configure a new plugin:

1. Make sure required binaries are available in `argocd-repo-server` pod. The binaries can be added via volume mounts or 
   using a custom image (see [custom_tools](../operator-manual/custom_tools.md) for examples of both).
2. Register a new plugin in `argocd-cm` ConfigMap:

        :::yaml
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
    
    The `generate` command must print a valid YAML or JSON stream to stdout. Both `init` and `generate` commands are executed inside the application source directory.

3. [Create an Application which uses your new CMP](#using-a-cmp).

More CMP examples are available in [argocd-example-apps](https://github.com/argoproj/argocd-example-apps/tree/master/plugins).

!!!note "Repository locking"
    If your plugin makes use of `git` (e.g. `git crypt`), it is advised to set
    `lockRepo` to `true` so that your plugin will have exclusive access to the
    repository at the time it is executed. Otherwise, two applications synced
    at the same time may result in a race condition and sync failure.

### Option 2: Configure plugin via sidecar

An operator can configure a plugin tool via a sidecar to repo-server. The following changes are required to configure a new plugin:

#### 1. Write the plugin configuration file

Plugins will be configured via a ConfigManagementPlugin manifest located inside the plugin container.

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

!!! note
    While the ConfigManagementPlugin _looks like_ a Kubernetes object, it is not actually a custom resource. 
    It only follows kubernetes-style spec conventions.

The `generate` command must print a valid YAML stream to stdout. Both `init` and `generate` commands are executed inside the application source directory.

The `discover.fileName` is used as [glob](https://pkg.go.dev/path/filepath#Glob) pattern to determine whether an
application repository is supported by the plugin or not. 

```yaml
  discover:
    find:
      command: [sh, -c, find . -name env.yaml]
```

If `discover.fileName` is not provided, the `discover.find.command` is executed in order to determine whether an 
application repository is supported by the plugin or not. The `find` command should return a non-error exit code when 
the application source type is supported. 

If your plugin makes use of `git` (e.g. `git crypt`), it is advised to set `lockRepo` to `true` so that your plugin will have exclusive access to the
repository at the time it is executed. Otherwise, two applications synced at the same time may result in a race condition and sync failure.

#### 2. Place the plugin configuration file in the sidecar

Argo CD expects the plugin configuration file to be located at `/home/argocd/cmp-server/config/plugin.yaml` in the sidecar.

If you use a custom image for the sidecar, you can add the file directly to that image.

If you use a stock image for the sidecar or would rather maintain the plugin configuration in a ConfigMap, just nest the
plugin config file in a ConfigMap under the `plugin.yaml` key.

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

#### 3. Register the plugin sidecar

To install a plugin, patch argocd-repo-server to run the CMP container as a sidecar, with argocd-cmp-server as its 
entrypoint. You can use either off-the-shelf or custom-built plugin image as sidecar image. For example:

```yaml
containers:
- name: cmp
  command: [/var/run/argocd/argocd-cmp-server] # Entrypoint should be Argo CD lightweight CMP server i.e. argocd-cmp-server
  image: busybox # This can be off-the-shelf or custom-built image
  securityContext:
    runAsNonRoot: true
    runAsUser: 999
  volumeMounts:
    - mountPath: /var/run/argocd
      name: var-files
    - mountPath: /home/argocd/cmp-server/plugins
      name: plugins
    - mountPath: /tmp
      name: tmp
    # Remove this volumeMount if you've chosen to bake the config file into the sidecar image.
    - mountPath: /home/argocd/cmp-server/config/plugin.yaml
      subPath: plugin.yaml
      name: cmp-plugin
  volumes:
    - configMap:
        name: cmp-plugin
      name: cmp-plugin
``` 

!!! important "Double-check these items"
    1. Make sure to use `/var/run/argocd/argocd-cmp-server` as an entrypoint. The `argocd-cmp-server` is a lightweight GRPC service that allows Argo CD to interact with the plugin.
    2. Make sure that sidecar container is running as user 999.
    3. Make sure that plugin configuration file is present at `/home/argocd/cmp-server/config/plugin.yaml`. It can either be volume mapped via configmap or baked into image.

### Environment

CMP commands have access to

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

## Using a CMP

If your CMP is defined in the `argocd-cm` ConfigMap, you can create a new Application using the CLI. Replace 
`<pluginName>` with the name configured in `argocd-cm`.

```bash
argocd app create <appName> --config-management-plugin <pluginName>
```

If your CMP is defined as a sidecar, you must manually define the Application manifest. Do not configure a `name` field
in the `plugin` section. The plugin will be automatically matched with the Application based on its discovery rules.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: guestbook
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    targetRevision: HEAD
    path: guestbook
    plugin:
      # For either argocd-cm- or sidecar-installed CMPs, you can pass environment variables to the CMP.
      env:
        - name: FOO
          value: bar
```

If you don't need to set any environment variables, you can set an empty plugin section.

```yaml
    plugin: {}
```

!!! important
    If your sidecar CMP command runs too long, the command will be killed, and the UI will show an error. The CMP server
    respects the timeouts set by the `server.repo.server.timeout.seconds` and `controller.repo.server.timeout.seconds` 
    items in `argocd-cm`. Increase their values from the default of 60s.

    Each CMP command will also independently timeout on the `ARGOCD_EXEC_TIMEOUT` set for the CMP sidecar. The default
    is 90s. So if you increase the repo server timeout greater than 90s, be sure to set `ARGOCD_EXEC_TIMEOUT` on the
    sidecar.
