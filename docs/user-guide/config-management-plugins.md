# Plugins

Argo CD allows integrating more config management tools using config management plugins. The following sections will
describe how to create, install, and use plugins. Check out the 
[example plugins](https://github.com/argoproj/argo-cd/tree/master/examples/plugins) for additional guidance.

## Installing a config management plugin

An operator can configure a plugin by adding a sidecar to argocd-repo-server Deployment. The following changes are 
required to configure a new plugin:

### 1. Write the plugin configuration file

Plugins will be configured via a ConfigManagementPlugin manifest located inside the plugin container.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ConfigManagementPlugin
metadata:
  # The name of the plugin must be unique within a given Argo CD instance.
  name: my-plugin
spec:
  # The init command runs in the Application source directory at the beginning of each manifest generation. The init
  # command can output anything. A non-zero status code will fail manifest generation.
  init:
    command: []
    args: []
  # The generate command runs in the Application source directory each time manifests are generated. Standard output
  # must be ONLY valid JSON or YAML manifests. A non-zero exit code will fail manifest generation.
  # Error output will be sent to the UI, so avoid printing sensitive information (such as secrets).
  generate:
    command: [sh, -c]
    args:
      - |
        echo "{\"kind\": \"ConfigMap\", \"apiVersion\": \"v1\", \"metadata\": { \"name\": \"$ARGOCD_APP_NAME\", \"namespace\": \"$ARGOCD_APP_NAMESPACE\", \"annotations\": {\"Foo\": \"$FOO\", \"KubeVersion\": \"$KUBE_VERSION\", \"KubeApiVersion\": \"$KUBE_API_VERSIONS\",\"Bar\": \"baz\"}}}"
  # The discovery config is applied to a repository. If every configured discovery tool matches, then the plugin may be
  # used to generate manifests for Applications using the repository. 
  # Only one of fileName, find.glob, or find.command should be specified. If multiple are specified then only the 
  # first (in that order) is evaluated.
  discover:
    # fileName is a glob pattern (https://pkg.go.dev/path/filepath#Glob) that is applied to the repository's root 
    # directory (not the Application source directory). If there is a match, this plugin may be used for the repository.
    fileName: "./subdir/s*.yaml"
    find:
      # This does the same thing as fileName, but it supports double-start (nested directory) glob patterns.
      glob: "**/Chart.yaml"
      # The find command runs in the repository's root directory. To match, it must exit with status code 0 _and_ 
      # produce non-empty output to standard out.
      command: [sh, -c, find . -name env.yaml]
  # The parameters config describes what parameters the UI should display for an Application. It is up to the user to
  # actually set parameters in the Application manifest (in spec.source.plugin.parameters). The announcements _only_
  # inform the "Parameters" tab in the App Details page of the UI.
  parameters:
    # Static parameter announcements are sent to the UI for _all_ Applications handled by this plugin.
    # Think of the `string`, `array`, and `map` values set here as "defaults". It is up to the plugin author to make 
    # sure that these default values actually reflect the plugin's behavior if the user doesn't explicitly set different
    # values for those parameters.
    static:
      - name: string-param
        title: Description of the string param
        tooltip: Tooltip shown when the user hovers the
        # If this field is set, the UI will indicate to the user that they must set the value.
        required: false
        # itemType tells the UI how to present the parameter's value (or, for arrays and maps, values). Default is
        # "string". Examples of other types which may be supported in the future are "boolean" or "number".
        # Even if the itemType is not "string", the parameter value from the Application spec will be sent to the plugin
        # as a string. It's up to the plugin to do the appropriate conversion.
        itemType: ""
        # collectionType describes what type of value this parameter accepts (string, array, or map) and allows the UI
        # to present a form to match that type. Default is "string".
        collectionType: ""
        # This field communicates the parameter's default value to the UI. Setting this field is optional.
        string: default-string-value
      # All the fields above besides "string" apply to both the array and map type parameter announcements.
      - name: array-param
        # This field communicates the parameter's default value to the UI. Setting this field is optional.
        array: [default, items]
      - name: map-param
        # This field communicates the parameter's default value to the UI. Setting this field is optional.
        map:
          some: value
    # Dynamic parameter announcements are announcements specific to an Application handled by this plugin. For example,
    # the values for a Helm chart's values.yaml file could be sent as parameter announcements.
    dynamic:
      # The command is run in an Application's source directory. Standard output must be JSON matching the schema of the
      # static parameter announcements list.
      command: [echo, '[{"name": "example-param", "string": "default-string-value"}]']
```

!!! note
    While the ConfigManagementPlugin _looks like_ a Kubernetes object, it is not actually a custom resource. 
    It only follows kubernetes-style spec conventions.

### 2. Place the plugin configuration file in the sidecar

Argo CD expects the plugin configuration file to be located at `/home/argocd/cmp-server/config/plugin.yaml` in the sidecar.

If you use a custom image for the sidecar, you can add the file directly to that image.

If you use a stock image for the sidecar or would rather maintain the plugin configuration in a ConfigMap, just nest the
plugin config file in a ConfigMap under the `plugin.yaml` key and mount the ConfigMap in the sidecar (see next section).

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-plugin-config
data:
  plugin.yaml: |
    apiVersion: argoproj.io/v1alpha1
    kind: ConfigManagementPlugin
    metadata:
      name: my-plugin
    spec:
      version: v1.0
      generate:
        command: [sh, -c, 'echo "{\"kind\": \"ConfigMap\", \"apiVersion\": \"v1\", \"metadata\": { \"name\": \"$ARGOCD_APP_NAME\", \"namespace\": \"$ARGOCD_APP_NAMESPACE\", \"annotations\": {\"Foo\": \"$FOO\", \"KubeVersion\": \"$KUBE_VERSION\", \"KubeApiVersion\": \"$KUBE_API_VERSIONS\",\"Bar\": \"baz\"}}}"']
      discover:
        fileName: "./subdir/s*.yaml"
```

### 3. Register the plugin sidecar

To install a plugin, patch argocd-repo-server to run the plugin container as a sidecar, with argocd-cmp-server as its 
entrypoint. You can use either off-the-shelf or custom-built plugin image as sidecar image. For example:

```yaml
containers:
- name: my-plugin
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
      name: my-plugin-config
  volumes:
    - configMap:
        name: my-plugin-config
      name: my-plugin-config
``` 

!!! important "Double-check these items"
    1. Make sure to use `/var/run/argocd/argocd-cmp-server` as an entrypoint. The `argocd-cmp-server` is a lightweight GRPC service that allows Argo CD to interact with the plugin.
    2. Make sure that sidecar container is running as user 999.
    3. Make sure that plugin configuration file is present at `/home/argocd/cmp-server/config/plugin.yaml`. It can either be volume mapped via configmap or baked into image.

## Using environment variables in your plugin

Plugin commands have access to

1. The system environment variables
2. [Standard build environment variables](build-environment.md)
3. Variables in the Application spec (References to system and build variables will get interpolated in the variables' values):

   ```yaml
   apiVersion: argoproj.io/v1alpha1
   kind: Application
   spec:
     source:
       plugin:
         env:
           - name: FOO
             value: bar
           - name: REV
             value: test-$ARGOCD_APP_REVISION
   ```

4. (Starting in v2.4) Parameters in the Application spec:

   ```yaml
   apiVersion: argoproj.io/v1alpha1
   kind: Application
   spec:
     source:
       plugin:
         parameters:
           - name: values-files
             array: [values-dev.yaml]
           - name: helm-parameters
             map:
               image.tag: v1.2.3
   ```
   
   The parameters are available as JSON in the `ARGOCD_APP_PARAMETERS` environment variable. The example above would
   produce this JSON:

   ```json
   [{"name": "values-files", "array": ["values-dev.yaml"]}, {"name": "helm-parameters", "map": {"image.tag": "v1.2.3"}}]
   ```

   !!! note
       Parameter announcements, even if they specify defaults, are _not_ sent to the plugin in `ARGOCD_APP_PARAMETERS`.
       Only parameters explicitly set in the Application spec are sent to the plugin. It is up to the plugin to apply
       the same defaults as the ones announced to the UI.
   
   The same parameters are also available as individual environment variables. The names of the environment variables
   follows this convention:

   ```yaml
   - name: some-string-param
     string: some-string-value
   # PARAM_SOME_STRING_PARAM=some-string-value
   
   - name: some-array-param
     value: [item1, item2]
   # PARAM_SOME_ARRAY_PARAM_0=item1
   # PARAM_SOME_ARRAY_PARAM_1=item2
   
   - name: some-map-param
     map:
       image.tag: v1.2.3
   # PARAM_SOME_MAP_PARAM_IMAGE_TAG=v1.2.3
   ```

!!! warning Sanitize/escape user input
    As part of Argo CD's manifest generation system, config management plugins are treated with a level of trust. Be 
    sure to escape user input in your plugin to prevent malicious input from causing unwanted behavior.

## Using a config management plugin with an Application

Argo CD will automatically select the correct plugin for your Application based on the plugin's configured discovery 
rules.

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
    plugin: {}
```

!!! important
    If your plugin's `init` or `generate` command runs too long, the command will be killed, and the UI will show an 
    error. The plugin server respects the timeouts set by the `server.repo.server.timeout.seconds` and 
    `controller.repo.server.timeout.seconds` items in `argocd-cm`. Increase their values from the default of 60s.

    Each plugin command will also independently timeout on the `ARGOCD_EXEC_TIMEOUT` set for the plugin sidecar. The 
    default is 90s. So if you increase the repo server timeout greater than 90s, be sure to set `ARGOCD_EXEC_TIMEOUT` 
    on the sidecar.

## Migrating from argocd-cm plugins

Installing plugins by modifying the argocd-cm ConfigMap is deprecated as of v2.4. Support will be completely removed in
a future release.

The following will show how to convert an argocd-cm plugin to a sidecar plugin. 

### 1. Convert the ConfigMap entry into a config file

First, copy the plugin's configuration into its own YAML file. Take for example the following ConfigMap entry:

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

The `pluginName` item would be converted to a config file like this:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ConfigManagementPlugin
metadata:
  name: pluginName
spec:
  init:                          # Optional command to initialize application source directory
    command: ["sample command"]
    args: ["sample args"]
  generate:                      # Command to generate manifests YAML
    command: ["sample command"]
    args: ["sample args"]
```

!!!note
    The `lockRepo` key is not relevant for sidecar plugins, because sidecar plugins do not share a single source repo
    directory when generating manifests.

### 2. Write discovery rules for your plugin

Sidecar plugins use discovery rules instead of a plugin name to match Applications to plugins.

Write rules applicable to your plugin [using the instructions above](#1-write-the-plugin-configuration-file) and add
them to your configuration file.

!!!important
    After installing your sidecar plugin, you'll need to remove the `name` field from the plugin config in your
    Application specs. For example:

    ```yaml
    apiVersion: argoproj.io/v1alpha1
    kind: Application
    metadata:
      name: guestbook
    spec:
      source:
        plugin:
          name: pluginName  # Delete this (and set `plugin: {}` if `name` was the only value).
    ```

### 3. Make sure the plugin has access to the tools it needs

Plugins configured with argocd-cm ran on the Argo CD image. This gave it access to all the tools installed on that
image by default (see the [Dockerfile](https://github.com/argoproj/argo-cd/blob/master/Dockerfile) for base image and 
installed tools).

You can either use a stock image (like busybox) or design your own base image with the tools your plugin needs. For 
security, avoid using image with more binaries installed than what your plugin actually needs.

### 4. Test the plugin

After installing the plugin as a sidecar [according to the directions above](#installing-a-config-management-plugin),
test it out on a few Applications before migrating all of them to the sidecar plugin.

Once tests have checked out, remove the plugin entry from your argocd-cm ConfigMap.
