# Plugins

Argo CD allows integrating more config management tools using config management plugins.

!!! warning
    Plugins are granted a level of trust in the Argo CD system, so it is important to implement plugins securely. Argo 
    CD administrators should only install plugins from trusted sources, and they should audit plugins to weigh their 
    particular risks and benefits.

## Installing a CMP

There are two ways to install a Config Management Plugin (CMP):

1. Add the plugin config to the Argo CD ConfigMap. The repo-server container will run your plugin's commands.
   This is a good option for a simple plugin that requires only a few lines of code that fit nicely in the Argo CD ConfigMap.
2. Add the plugin as a sidecar to the repo-server Pod.
   This is a good option for a more complex plugin that would clutter the Argo CD ConfigMap. A copy of the repository is sent to the sidecar container as a tarball and processed individually per application, which makes it a good option for [concurrent processing of monorepos](../operator-manual/high_availability.md#enable-concurrent-processing).

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
    
    The `generate` command must print a valid YAML or JSON stream to stdout. Both `init` and `generate` commands are executed inside the application source directory or in `path` when specified for the app.

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
  init:
    # Init always happens immediately before generate, but its output is not treated as manifests.
    # This is a good place to, for example, download chart dependencies.
    command: [sh, -c, 'echo "Initializing..."']
  generate:
    command: [sh, -c, 'echo "{\"kind\": \"ConfigMap\", \"apiVersion\": \"v1\", \"metadata\": { \"name\": \"$ARGOCD_APP_NAME\", \"namespace\": \"$ARGOCD_APP_NAMESPACE\", \"annotations\": {\"Foo\": \"$FOO\", \"KubeVersion\": \"$KUBE_VERSION\", \"KubeApiVersion\": \"$KUBE_API_VERSIONS\",\"Bar\": \"baz\"}}}"']
  discover:
    fileName: "./subdir/s*.yaml"
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
application repository is supported by the plugin or not. The `find` command should return a non-error exit code
and produce output to stdout when the application source type is supported.

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
      init:
        command: [sh, -c, 'echo "Initializing..."']
      generate:
        command: [sh, -c, 'echo "{\"kind\": \"ConfigMap\", \"apiVersion\": \"v1\", \"metadata\": { \"name\": \"$ARGOCD_APP_NAME\", \"namespace\": \"$ARGOCD_APP_NAMESPACE\", \"annotations\": {\"Foo\": \"$FOO\", \"KubeVersion\": \"$KUBE_VERSION\", \"KubeApiVersion\": \"$KUBE_API_VERSIONS\",\"Bar\": \"baz\"}}}"']
      discover:
        fileName: "./subdir/s*.yaml"
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
    # Remove this volumeMount if you've chosen to bake the config file into the sidecar image.
    - mountPath: /home/argocd/cmp-server/config/plugin.yaml
      subPath: plugin.yaml
      name: cmp-plugin
    # Starting with v2.4, do NOT mount the same tmp volume as the repo-server container. The filesystem separation helps 
    # mitigate path traversal attacks.
    - mountPath: /tmp
      name: cmp-tmp
volumes:
  - configMap:
      name: cmp-plugin
    name: cmp-plugin
  - emptyDir: {}
    name: cmp-tmp
``` 

!!! important "Double-check these items"
    1. Make sure to use `/var/run/argocd/argocd-cmp-server` as an entrypoint. The `argocd-cmp-server` is a lightweight GRPC service that allows Argo CD to interact with the plugin.
    2. Make sure that sidecar container is running as user 999.
    3. Make sure that plugin configuration file is present at `/home/argocd/cmp-server/config/plugin.yaml`. It can either be volume mapped via configmap or baked into image.

### Environment

CMP commands have access to

1. The system environment variables (of the repo-server container for argocd-cm plugins or of the sidecar for sidecar plugins)
2. [Standard build environment](build-environment.md)
3. Variables in the application spec (References to system and build variables will get interpolated in the variables' values):

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

!!! note
    The `discover.command` command only has access to the above environment starting with v2.4.

Before reaching the `init.command`, `generate.command`, and `discover.command` commands, Argo CD prefixes all 
user-supplied environment variables (#3 above) with `ARGOCD_ENV_`. This prevents users from directly setting 
potentially-sensitive environment variables.

If your plugin was written before 2.4 and depends on user-supplied environment variables, then you will need to update
your plugin's behavior to work with 2.4. If you use a third-party plugin, make sure they explicitly advertise support
for 2.4.

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

!!! note
    Each Application can only have one config management plugin configured at a time. If you're converting an existing
    plugin configured through the `argocd-cm` ConfigMap to a sidecar, make sure the discovery mechanism only returns
    true for Applications that have had their `name` field in the `plugin` section of their spec removed.

## Plugin tar stream exclusions

In order to increase the speed of manifest generation, certain files and folders can be excluded from being sent to your
plugin. We recommend excluding your `.git` folder if it isn't necessary. Use Go's 
[filepatch.Match](https://pkg.go.dev/path/filepath#Match) syntax.

You can set it one of three ways:

1. The `--plugin-tar-exclude` argument on the repo server.
2. The `reposerver.plugin.tar.exclusions` key if you are using `argocd-cmd-params-cm`
3. Directly setting `ARGOCD_REPO_SERVER_PLUGIN_TAR_EXCLUSIONS` environment variable on the repo server.

For option 1, the flag can be repeated multiple times. For option 2 and 3, you can specify multiple globs by separating
them with semicolons.