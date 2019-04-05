# Application Source Types

Argo CD supports several different ways in which kubernetes manifests can be defined:

* [ksonnet](https://ksonnet.io) applications
* [kustomize](https://kustomize.io) applications
* [helm](https://helm.sh) charts
* Directory of YAML/json/jsonnet manifests
* Any custom config management tool configured as a config management plugin

Some additional considerations should be made when deploying apps of a particular type:

## Ksonnet

### Environments
Ksonnet has a first class concept of an "environment." To create an application from a ksonnet
app directory, an environment must be specified. For example, the following command creates the
"guestbook-default" app, which points to the `default` environment:

```bash
argocd app create guestbook-default --repo https://github.com/argoproj/argocd-example-apps.git --path guestbook --env default
```

### Parameters
Ksonnet parameters all belong to a component. For example, the following are the parameters
available in the guestbook app, all of which belong to the `guestbook-ui` component:

```bash
$ ks param list
COMPONENT    PARAM         VALUE
=========    =====         =====
guestbook-ui containerPort 80
guestbook-ui image         "gcr.io/heptio-images/ks-guestbook-demo:0.1"
guestbook-ui name          "guestbook-ui"
guestbook-ui replicas      1
guestbook-ui servicePort   80
guestbook-ui type          "LoadBalancer"
```

When overriding ksonnet parameters in Argo CD, the component name should also be specified in the
`argocd app set` command, in the form of `-p COMPONENT=PARAM=VALUE`. For example:

```bash
argocd app set guestbook-default -p guestbook-ui=image=gcr.io/heptio-images/ks-guestbook-demo:0.1
```

## Helm

### Values Files

Helm has the ability to use a different, or even multiple "values.yaml" files to derive its
parameters from. Alternate or multiple values file(s), can be specified using the `--values`
flag. The flag can be repeated to support multiple values files:

```bash
argocd app set helm-guestbook --values values-production.yaml
```

### Helm Parameters

Helm has the ability to set parameter values, which override any values in
a `values.yaml`. For example, `service.type` is a common parameter which is exposed in a Helm chart:

```bash
helm template . --set service.type=LoadBalancer
```

Similarly Argo CD can override values in the `values.yaml` parameters using `argo app set` command,
in the form of `-p PARAM=VALUE`. For example:

```bash
argocd app set helm-guestbook -p service.type=LoadBalancer
```

### Helm Hooks

Helm hooks are equivalent in concept to [Argo CD resource hooks](resource_hooks.md). In helm, a hook
is any normal kubernetes resource annotated with the `helm.sh/hook` annotation. When Argo CD deploys
helm application which contains helm hooks, all helm hook resources are currently ignored during
the `kubectl apply` of the manifests. There is an
[open issue](https://github.com/argoproj/argo-cd/issues/355) to map Helm hooks to Argo CD's concept
of Pre/Post/Sync hooks.

### Random Data

Helm templating has the ability to generate random data during chart rendering via the
`randAlphaNum` function. Many helm charts from the [charts repository](https://github.com/helm/charts)
make use of this feature. For example, the following is the secret for the
[redis helm chart](https://github.com/helm/charts/blob/master/stable/redis/templates/secrets.yaml):

```yaml
data:
  {{- if .Values.password }}
  redis-password: {{ .Values.password | b64enc | quote }}
  {{- else }}
  redis-password: {{ randAlphaNum 10 | b64enc | quote }}
  {{- end }}
```

The Argo CD application controller periodically compares Git state against the live state, running
the `helm template <CHART>` command to generate the helm manifests. Because the random value is
regenerated every time the comparison is made, any application which makes use of the `randAlphaNum`
function will always be in an `OutOfSync` state. This can be mitigated by explicitly setting a
value, in the values.yaml such that the value is stable between each comparison. For example:

```bash
argocd app set redis -p password=abc123
```

## Config Management Plugins

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
Commands have access to system environment variables and following additional variables:

`ARGOCD_APP_NAME` - name of application; `ARGOCD_APP_NAMESPACE` - destination application namespace

 * Create an application and specify required config management plugin name.

```bash
argocd app create <appName> --config-management-plugin <pluginName>
```

More config management plugin examples are available in [argocd-example-apps](https://github.com/argoproj/argocd-example-apps/tree/master/plugins).
