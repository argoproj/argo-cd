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
```

The `generate` command must print a valid YAML stream to stdout. Both `init` and `generate` commands are executed inside the application source directory.

 * Create an application and specify required config management plugin name.

```bash
argocd app create <appName> --config-management-plugin <pluginName>
```

More config management plugin examples are available in [argocd-example-apps](https://github.com/argoproj/argocd-example-apps/tree/master/plugins).

## Environment

Commands have access to

(1) The system environment variables
(2) Argo CD  environment variables:

* `ARGOCD_APP_NAME` - name of application
* `ARGOCD_APP_NAMESPACE` - destination application namespace.

(3) Variables in the application spec:

> v1.2

```yaml
spec:
  source:
    plugin:
      env:
        - name: FOO
          value: bar
```
