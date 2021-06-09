# Plugins

Argo CD allows integrating more config management tools using config management plugins. Following changes are required to configure new plugin:

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
