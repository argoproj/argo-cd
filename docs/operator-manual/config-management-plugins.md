
# Config Management Plugins

Argo CD's "native" config management tools are Helm, Jsonnet, and Kustomize. If you want to use a different config
management tools, or if Argo CD's native tool support does not include a feature you need, you might need to turn to
a Config Management Plugin (CMP).

The Argo CD "repo server" component is in charge of building Kubernetes manifests based on some source files from a
Helm, OCI, or git repository. When a config management plugin is correctly configured, the repo server may delegate the
task of building manifests to the plugin.

The following sections will describe how to create, install, and use plugins. Check out the
[example plugins](https://github.com/argoproj/argo-cd/tree/master/examples/plugins) for additional guidance.

!!! warning
    Plugins are granted a level of trust in the Argo CD system, so it is important to implement plugins securely. Argo
    CD administrators should only install plugins from trusted sources, and they should audit plugins to weigh their
    particular risks and benefits.

## Installing a config management plugin

### Sidecar plugin

An operator can configure a plugin tool via a sidecar to repo-server. The following changes are required to configure a new plugin:

#### Write the plugin configuration file

Plugins will be configured via a ConfigManagementPlugin manifest located inside the plugin container.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ConfigManagementPlugin
metadata:
  # The name of the plugin must be unique within a given Argo CD instance.
  name: my-plugin
spec:
  # The version of your plugin. Optional. If specified, the Application's spec.source.plugin.name field
  # must be <plugin name>-<plugin version>.
  version: v1.0
  # The init command runs in the Application source directory at the beginning of each manifest generation. The init
  # command can output anything. A non-zero status code will fail manifest generation.
  init:
    # Init always happens immediately before generate, but its output is not treated as manifests.
    # This is a good place to, for example, download chart dependencies.
    command: [sh]
    args: [-c, 'echo "Initializing..."']
  # The generate command runs in the Application source directory each time manifests are generated. Standard output
  # must be ONLY valid Kubernetes Objects in either YAML or JSON. A non-zero exit code will fail manifest generation.
  # To write log messages from the command, write them to stderr, it will always be displayed.
  # Error output will be sent to the UI, so avoid printing sensitive information (such as secrets).
  generate:
    command: [sh, -c]
    args:
      - |
        echo "{\"kind\": \"ConfigMap\", \"apiVersion\": \"v1\", \"metadata\": { \"name\": \"$ARGOCD_APP_NAME\", \"namespace\": \"$ARGOCD_APP_NAMESPACE\", \"annotations\": {\"Foo\": \"$ARGOCD_ENV_FOO\", \"KubeVersion\": \"$KUBE_VERSION\", \"KubeApiVersion\": \"$KUBE_API_VERSIONS\",\"Bar\": \"baz\"}}}"
  # The discovery config is applied to a repository. If every configured discovery tool matches, then the plugin may be
  # used to generate manifests for Applications using the repository. If the discovery config is omitted then the plugin 
  # will not match any application but can still be invoked explicitly by specifying the plugin name in the app spec. 
  # Only one of fileName, find.glob, or find.command should be specified. If multiple are specified then only the 
  # first (in that order) is evaluated.
  discover:
    # fileName is a glob pattern (https://pkg.go.dev/path/filepath#Glob) that is applied to the Application's source 
    # directory. If there is a match, this plugin may be used for the Application.
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
        # to present a form to match that type. Default is "string". This field must be present for non-string types.
        # It will not be inferred from the presence of an `array` or `map` field.
        collectionType: ""
        # This field communicates the parameter's default value to the UI. Setting this field is optional.
        string: default-string-value
      # All the fields above besides "string" apply to both the array and map type parameter announcements.
      - name: array-param
        # This field communicates the parameter's default value to the UI. Setting this field is optional.
        array: [default, items]
        collectionType: array
      - name: map-param
        # This field communicates the parameter's default value to the UI. Setting this field is optional.
        map:
          some: value
        collectionType: map
    # Dynamic parameter announcements are announcements specific to an Application handled by this plugin. For example,
    # the values for a Helm chart's values.yaml file could be sent as parameter announcements.
    dynamic:
      # The command is run in an Application's source directory. Standard output must be JSON matching the schema of the
      # static parameter announcements list.
      command: [echo, '[{"name": "example-param", "string": "default-string-value"}]']

  # If set to `true` then the plugin receives repository files with original file mode. Dangerous since the repository
  # might have executable files. Set to true only if you trust the CMP plugin authors.
  preserveFileMode: false
```

!!! note
    While the ConfigManagementPlugin _looks like_ a Kubernetes object, it is not actually a custom resource. 
    It only follows kubernetes-style spec conventions.

The `generate` command must print a valid Kubernetes YAML or JSON object stream to stdout. Both `init` and `generate` commands are executed inside the application source directory.

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

#### Place the plugin configuration file in the sidecar

Argo CD expects the plugin configuration file to be located at `/home/argocd/cmp-server/config/plugin.yaml` in the sidecar.

If you use a custom image for the sidecar, you can add the file directly to that image.

```dockerfile
WORKDIR /home/argocd/cmp-server/config/
COPY plugin.yaml ./
```

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
      init:
        command: [sh, -c, 'echo "Initializing..."']
      generate:
        command: [sh, -c, 'echo "{\"kind\": \"ConfigMap\", \"apiVersion\": \"v1\", \"metadata\": { \"name\": \"$ARGOCD_APP_NAME\", \"namespace\": \"$ARGOCD_APP_NAMESPACE\", \"annotations\": {\"Foo\": \"$ARGOCD_ENV_FOO\", \"KubeVersion\": \"$KUBE_VERSION\", \"KubeApiVersion\": \"$KUBE_API_VERSIONS\",\"Bar\": \"baz\"}}}"']
      discover:
        fileName: "./subdir/s*.yaml"
```

#### Register the plugin sidecar

To install a plugin, patch argocd-repo-server to run the plugin container as a sidecar, with argocd-cmp-server as its 
entrypoint. You can use either off-the-shelf or custom-built plugin image as sidecar image. For example:

```yaml
containers:
- name: my-plugin
  command: [/var/run/argocd/argocd-cmp-server] # Entrypoint should be Argo CD lightweight CMP server i.e. argocd-cmp-server
  image: ubuntu # This can be off-the-shelf or custom-built image
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
    # Starting with v2.4, do NOT mount the same tmp volume as the repo-server container. The filesystem separation helps 
    # mitigate path traversal attacks.
    - mountPath: /tmp
      name: cmp-tmp
volumes:
- configMap:
    name: my-plugin-config
  name: my-plugin-config
- emptyDir: {}
  name: cmp-tmp
``` 

!!! important "Double-check these items"
    1. Make sure to use `/var/run/argocd/argocd-cmp-server` as an entrypoint. The `argocd-cmp-server` is a lightweight GRPC service that allows Argo CD to interact with the plugin.
    2. Make sure that sidecar container is running as user 999.
    3. Make sure that plugin configuration file is present at `/home/argocd/cmp-server/config/plugin.yaml`. It can either be volume mapped via configmap or baked into image.

### Using environment variables in your plugin

Plugin commands have access to

1. The system environment variables of the sidecar
2. [Standard build environment variables](../user-guide/build-environment.md)
3. Variables in the Application spec (References to system and build variables will get interpolated in the variables' values):

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
    
    Before reaching the `init.command`, `generate.command`, and `discover.find.command` commands, Argo CD prefixes all 
    user-supplied environment variables (#3 above) with `ARGOCD_ENV_`. This prevents users from directly setting 
    potentially-sensitive environment variables.

4. Parameters in the Application spec:

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
   
    The parameters are available as JSON in the `ARGOCD_APP_PARAMETERS` environment variable. The example above would
    produce this JSON:
   
        [{"name": "values-files", "array": ["values-dev.yaml"]}, {"name": "helm-parameters", "map": {"image.tag": "v1.2.3"}}]
   
    !!! note
        Parameter announcements, even if they specify defaults, are _not_ sent to the plugin in `ARGOCD_APP_PARAMETERS`.
        Only parameters explicitly set in the Application spec are sent to the plugin. It is up to the plugin to apply
        the same defaults as the ones announced to the UI.
   
    The same parameters are also available as individual environment variables. The names of the environment variables
    follows this convention:
   
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
   
!!! warning "Sanitize/escape user input" 
    As part of Argo CD's manifest generation system, config management plugins are treated with a level of trust. Be
    sure to escape user input in your plugin to prevent malicious input from causing unwanted behavior.

## Using a config management plugin with an Application

You may leave the `name` field
empty in the `plugin` section for the plugin to be automatically matched with the Application based on its discovery rules. If you do mention the name make sure 
it is either `<metadata.name>-<spec.version>` if version is mentioned in the `ConfigManagementPlugin` spec or else just `<metadata.name>`. When name is explicitly 
specified only that particular plugin will be used iff its discovery pattern/command matches the provided application repo.

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
      env:
        - name: FOO
          value: bar
```

If you don't need to set any environment variables, you can set an empty plugin section.

```yaml
    plugin: {}
```

!!! important
    If your CMP command runs too long, the command will be killed, and the UI will show an error. The CMP server
    respects the timeouts set by the `server.repo.server.timeout.seconds` and `controller.repo.server.timeout.seconds` 
    items in `argocd-cm`. Increase their values from the default of 60s.

    Each CMP command will also independently timeout on the `ARGOCD_EXEC_TIMEOUT` set for the CMP sidecar. The default
    is 90s. So if you increase the repo server timeout greater than 90s, be sure to set `ARGOCD_EXEC_TIMEOUT` on the
    sidecar.
    
!!! note
    Each Application can only have one config management plugin configured at a time. If you're converting an existing
    plugin configured through the `argocd-cm` ConfigMap to a sidecar, make sure to update the plugin name to either `<metadata.name>-<spec.version>` 
    if version was mentioned in the `ConfigManagementPlugin` spec or else just use `<metadata.name>`. You can also remove the name altogether 
    and let the automatic discovery to identify the plugin.
!!! note
    If a CMP renders blank manfiests, and `prune` is set to `true`, Argo CD will automatically remove resources. CMP plugin authors should ensure errors are part of the exit code. Commonly something like `kustomize build . | cat` won't pass errors because of the pipe. Consider setting `set -o pipefail` so anything piped will pass errors on failure.

## Debugging a CMP

If you are actively developing a sidecar-installed CMP, keep a few things in mind:

1. If you are mounting plugin.yaml from a ConfigMap, you will have to restart the repo-server Pod so the plugin will
   pick up the changes.
2. If you have baked plugin.yaml into your image, you will have to build, push, and force a re-pull of that image on the
   repo-server Pod so the plugin will pick up the changes. If you are using `:latest`, the Pod will always pull the new
   image. If you're using a different, static tag, set `imagePullPolicy: Always` on the CMP's sidecar container.
3. CMP errors are cached by the repo-server in Redis. Restarting the repo-server Pod will not clear the cache. Always
   do a "Hard Refresh" when actively developing a CMP so you have the latest output.
4. Verify your sidecar has started properly by viewing the Pod and seeing that two containers are running `kubectl get pod -l app.kubernetes.io/component=repo-server -n argocd`
5. Write log message to stderr and set the `--loglevel=info` flag in the sidecar. This will print everything written to stderr, even on successfull command execution.


### Other Common Errors
| Error Message | Cause |
| -- | -- |
| `no matches for kind "ConfigManagementPlugin" in version "argoproj.io/v1alpha1"` | The `ConfigManagementPlugin` CRD was deprecated in Argo CD 2.4 and removed in 2.8. This error means you've tried to put the configuration for your plugin directly into Kubernetes as a CRD. Refer to this [section of documentation](#write-the-plugin-configuration-file) for how to write the plugin configuration file and place it properly in the sidecar. |

## Plugin tar stream exclusions

In order to increase the speed of manifest generation, certain files and folders can be excluded from being sent to your
plugin. We recommend excluding your `.git` folder if it isn't necessary. Use Go's
[filepatch.Match](https://pkg.go.dev/path/filepath#Match) syntax. For example, `.git/*` to exclude `.git` folder.

You can set it one of three ways:

1. The `--plugin-tar-exclude` argument on the repo server.
2. The `reposerver.plugin.tar.exclusions` key if you are using `argocd-cmd-params-cm`
3. Directly setting `ARGOCD_REPO_SERVER_PLUGIN_TAR_EXCLUSIONS` environment variable on the repo server.

For option 1, the flag can be repeated multiple times. For option 2 and 3, you can specify multiple globs by separating
them with semicolons.

## Application manifests generation using argocd.argoproj.io/manifest-generate-paths

To enhance the application manifests generation process, you can enable the use of the `argocd.argoproj.io/manifest-generate-paths` annotation. When this flag is enabled, the resources specified by this annotation will be passed to the CMP server for generating application manifests, rather than sending the entire repository. This can be particularly useful for monorepos.

You can set it one of three ways:

1. The `--plugin-use-manifest-generate-paths` argument on the repo server.
2. The `reposerver.plugin.use.manifest.generate.paths` key if you are using `argocd-cmd-params-cm`
3. Directly setting `ARGOCD_REPO_SERVER_PLUGIN_USE_MANIFEST_GENERATE_PATHS` environment variable on the repo server to `true`.

## Migrating from argocd-cm plugins

Installing plugins by modifying the argocd-cm ConfigMap is deprecated as of v2.4 and has been completely removed starting in v2.8.

CMP plugins work by adding a sidecar to `argocd-repo-server` along with a configuration in that sidecar located at `/home/argocd/cmp-server/config/plugin.yaml`. A argocd-cm plugin can be easily converted with the following steps.

### Convert the ConfigMap entry into a config file

First, copy the plugin's configuration into its own YAML file. Take for example the following ConfigMap entry:

```yaml
data:
  configManagementPlugins: |
    - name: pluginName
      init:                          # Optional command to initialize application source directory
        command: ["sample command"]
        args: ["sample args"]
      generate:                      # Command to generate Kubernetes Objects in either YAML or JSON
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
  generate:                      # Command to generate Kubernetes Objects in either YAML or JSON
    command: ["sample command"]
    args: ["sample args"]
```

!!! note
    The `lockRepo` key is not relevant for sidecar plugins, because sidecar plugins do not share a single source repo
    directory when generating manifests.

Next, we need to decide how this yaml is going to be added to the sidecar. We can either bake the yaml directly into the image, or we can mount it from a ConfigMap. 

If using a ConfigMap, our example would look like this:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: pluginName
  namespace: argocd
data:
  pluginName.yaml: |
    apiVersion: argoproj.io/v1alpha1
    kind: ConfigManagementPlugin
    metadata:
      name: pluginName
    spec:
      init:                          # Optional command to initialize application source directory
        command: ["sample command"]
        args: ["sample args"]
      generate:                      # Command to generate Kubernetes Objects in either YAML or JSON
        command: ["sample command"]
        args: ["sample args"]
```

Then this would be mounted in our plugin sidecar.

### Write discovery rules for your plugin

Sidecar plugins can use either discovery rules or a plugin name to match Applications to plugins. If the discovery rule is omitted 
then you have to explicitly specify the plugin by name in the app spec or else that particular plugin will not match any app.

If you want to use discovery instead of the plugin name to match applications to your plugin, write rules applicable to 
your plugin [using the instructions above](#1-write-the-plugin-configuration-file) and add them to your configuration 
file.

To use the name instead of discovery, update the name in your application manifest to `<metadata.name>-<spec.version>` 
if version was mentioned in the `ConfigManagementPlugin` spec or else just use `<metadata.name>`. For example:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: guestbook
spec:
  source:
    plugin:
      name: pluginName  # Delete this for auto-discovery (and set `plugin: {}` if `name` was the only value) or use proper sidecar plugin name
```

### Make sure the plugin has access to the tools it needs

Plugins configured with argocd-cm ran on the Argo CD image. This gave it access to all the tools installed on that
image by default (see the [Dockerfile](https://github.com/argoproj/argo-cd/blob/master/Dockerfile) for base image and
installed tools).

You can either use a stock image (like ubuntu, busybox, or alpine/k8s) or design your own base image with the tools your plugin needs. For
security, avoid using images with more binaries installed than what your plugin actually needs.

### Test the plugin

After installing the plugin as a sidecar [according to the directions above](#installing-a-config-management-plugin),
test it out on a few Applications before migrating all of them to the sidecar plugin.

Once tests have checked out, remove the plugin entry from your argocd-cm ConfigMap.

### Additional Settings

#### Preserve repository files mode

By default, config management plugin receives source repository files with reset file mode. This is done for security
reasons. If you want to preserve original file mode, you can set `preserveFileMode` to `true` in the plugin spec:

!!! warning
    Make sure you trust the plugin you are using. If you set `preserveFileMode` to `true` then the plugin might receive
    files with executable permissions which can be a security risk.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ConfigManagementPlugin
metadata:
  name: pluginName
spec:
  init:
    command: ["sample command"]
    args: ["sample args"]
  generate:
    command: ["sample command"]
    args: ["sample args"]
  preserveFileMode: true
```
