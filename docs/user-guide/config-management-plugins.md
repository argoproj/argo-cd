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
      parameters:                          # Optional command to generate parameters
        command: ["sample command"]
        args: ["sample args"]
      generate:                      # Command to generate manifests YAML
        command: ["sample command"]
        args: ["sample args"]
```

The `generate` command must print a valid YAML stream to stdout. The commands are executed inside the application source directory.

 * Create an application and specify required config management plugin name.

```bash
argocd app create <appName> --config-management-plugin <pluginName>
```

More config management plugin examples are available in [argocd-example-apps](https://github.com/argoproj/argocd-example-apps/tree/master/plugins).

## Environment

Commands have access to

1. The system environment variables
2. [Standard build environment](build-environment.md)
3. Variables in the application spec:

> v1.2

```yaml
spec:
  source:
    plugin:
      env:
        - name: FOO
          value: bar
```

## Parameters

> v1.6

Some manifests templating tools, such as JK, do not allow you to use environment variables to configure the command, you must use parameters instead.

This feature allows to you have parameters specified for an app an generic name/value pairs (name must be unique) and have these converted into the appropriate parameters for the command:

```yaml
spec:
  source:
    plugin:
      parameters:
        foo: bar
        kubeVersion: "${KUBE_VERSION}"
```

Parameters are only valid if you have a plugin command set-up, you'll get an error if you do not.

The parameters command in invoked with the parameters set as environment variable (yes - it is odd that we'd do that, but it works just fine). The environment variables name is `params_${name}`, so the above is `params_foo=bar`. This prevents collisions with other environment variables. You can use standard build environment variables, as in the example above, and these are also passed as the environment.

## Worked Example: JK 
 
Configuration:

```
data:
  configManagementPlugins: |
    - name: jk
      parameters:  
        command: ["jk-params"]
      generate:
        command: ["jk generate"]
```

Parameters script:

```
$ cat jk-params
#!/bin/sh
echo -p appName=${ARGOCD_APP_NAME} -p foo=${params_foo} main.js
```

App spec:

```yaml
spec:
  source:
    plugin:
      parameters:
        foo: bar
```

This will result in the following:

```
jk generate -p appName=my-app -p foo=bar main.js
```