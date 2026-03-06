# Helm

## Declarative

You can install Helm charts through the UI, or in the declarative GitOps way.  
Helm is [only used to inflate charts with `helm template`](../faq.md#after-deploying-my-helm-application-with-argo-cd-i-cannot-see-it-with-helm-ls-and-other-helm-commands). The lifecycle of the application is handled by Argo CD instead of Helm.
Here is an example:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: sealed-secrets
  namespace: argocd
spec:
  project: default
  source:
    chart: sealed-secrets
    repoURL: https://bitnami-labs.github.io/sealed-secrets
    targetRevision: 1.16.1
    helm:
      releaseName: sealed-secrets
  destination:
    server: "https://kubernetes.default.svc"
    namespace: kubeseal
```

Another example using a public OCI helm chart:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: nginx
spec:
  project: default
  source:
    chart: nginx
    repoURL: registry-1.docker.io/bitnamicharts  # note: the oci:// syntax is not included.
    targetRevision: 15.9.0
  destination:
    name: "in-cluster"
    namespace: nginx
```

> [!NOTE]
> **When using Helm there are multiple ways to provide values**
>
> Order of precedence is `parameters > valuesObject > values > valueFiles > helm repository values.yaml`. [Value precedence](./helm.md#helm-value-precedence) has a more detailed example.

The [Declarative Setup section on Helm](../operator-manual/declarative-setup.md#helm) has more info about how to configure private Helm repositories and private OCI registries.

## Customizing Helm Charts in Argo CD

When deploying Helm charts with Argo CD, you have several options to customize chart values **declaratively** in your Application manifest:

1. **Helm Parameters** – Override specific chart values using `spec.source.helm.parameters` (recommended for simple overrides).
2. **Inline Values** – Embed complete YAML value objects directly in the Application manifest using `spec.source.helm.values` or `spec.source.helm.valuesObject`.
3. **Values Files** – Reference external values files stored in your Git repository using `spec.source.helm.valueFiles`.
4. **File Parameters** – Load file contents as parameters using `spec.source.helm.fileParameters`.

Choose the option that best fits your use case. For example:
- Use **parameters** for simple, environment-specific overrides (e.g., replica count, service type).
- Use **inline values** when you need structured YAML objects inline (easier to review in the manifest).
- Use **values files** to keep values organized in separate files (better for complex configurations).

Values are merged and applied in a specific order; see [Helm Value Precedence](#helm-value-precedence) for details.

## Values Files

Helm charts can accept external values files to customize behavior. Argo CD supports referencing values files via the `spec.source.helm.valueFiles` field in your Application manifest.

### Values Files in the Chart Repository

By default, values files are expected to be located in the chart repository, either at the root or within a relative path from the chart directory:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: nginx-app
spec:
  project: default
  source:
    repoURL: https://github.com/acme/helm-charts
    path: charts/nginx
    chart: nginx
    targetRevision: HEAD
    helm:
      valueFiles:
        - values-production.yaml
        - values-secrets.yaml
  destination:
    server: https://kubernetes.default.svc
    namespace: default
```

In this example, `values-production.yaml` and `values-secrets.yaml` should exist in the `charts/nginx` directory of the Git repository.

### Values Files from External Repositories

Argo CD v2.6 introduced support for `sources` (multiple sources for an Application). Using multiple sources you can reference value files that live in a separate Git repository from the chart (see [multiple_sources.md](./multiple_sources.md#helm-value-files-from-external-git-repository) for details). This is useful when you want to keep Helm charts and environment-specific values in different repositories:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: nginx-prod
spec:
  project: default
  sources:
    - repoURL: https://github.com/acme/helm-charts
      path: charts/nginx
      chart: nginx
      targetRevision: 1.0.0
      helm:
        valueFiles:
          - $values/nginx/prod/values.yaml
    - repoURL: https://github.com/acme/gitops-values
      path: nginx/prod
      targetRevision: HEAD
      ref: values
  destination:
    server: https://kubernetes.default.svc
    namespace: default
```

### Handling Missing Values Files

If a values file referenced in `valueFiles` does not exist, Helm will fail with an error. You can ignore missing files using the `ignoreMissingValueFiles` flag, which is useful for implementing optional override patterns:

```yaml
source:
  helm:
    valueFiles:
      - values-common.yaml          # Always required
      - values-optional-override.yaml  # May not exist in all cases
    ignoreMissingValueFiles: true
```

This pattern is particularly helpful with [ApplicationSets](./application-set.md) to implement default/override configurations (see [this discussion](https://github.com/argoproj/argo-cd/issues/7767#issue-1060611415) for details).

### Values File Precedence

When multiple values files are specified, **the last file listed has the highest precedence** and overrides values from earlier files:

```yaml
valueFiles:
  - values-defaults.yaml       # Lowest precedence
  - values-regional.yaml
  - values-environment.yaml    # Highest precedence
```

In this example, `values-environment.yaml` will override any matching keys in `values-regional.yaml` and `values-defaults.yaml`.

## Inline Values

Argo CD supports embedding Helm values directly in your Application manifest. This is useful when you have structured YAML configuration that you want to keep alongside your application definition.

### Using valuesObject (Recommended for Structured Data)

The `valuesObject` field allows you to define values as a YAML object structure:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: nginx-ingress-app
spec:
  project: default
  source:
    repoURL: https://kubernetes.github.io/ingress-nginx
    chart: ingress-nginx
    targetRevision: 4.8.0
    helm:
      releaseName: nginx-ingress
      valuesObject:
        controller:
          replicaCount: 2
          service:
            type: LoadBalancer
        ingress:
          enabled: true
          className: nginx
  destination:
    server: https://kubernetes.default.svc
    namespace: ingress-nginx
```

### Using values (String Format)

Alternatively, you can provide values as a YAML string using the `values` field:

```yaml
source:
  helm:
    values: |
      controller:
        replicaCount: 2
        service:
          type: LoadBalancer
      ingress:
        enabled: true
        className: nginx
```

Both `valuesObject` and `values` provide the same functionality; choose the format that is more readable or convenient for your use case.

> [!TIP]
> **Secrets caution:** Examples in this page may include placeholders like `${DB_PASSWORD}`. Do not store secrets in plaintext in Git. Use sealed secrets, external secret managers, or reference Kubernetes Secrets. See [SealedSecrets](https://github.com/bitnami-labs/sealed-secrets) or external secrets operators for recommended patterns.

## Helm Parameters

Helm parameters allow you to override individual chart values without needing a separate values file. This is useful for simple, environment-specific overrides like replica counts, service types, or feature flags.

### Using Parameters

Parameters are specified as `name=value` pairs in the `spec.source.helm.parameters` field:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: nginx-app
spec:
  project: default
  source:
    repoURL: https://bitnami-labs.github.io/sealed-secrets
    chart: sealed-secrets
    targetRevision: 1.16.1
    helm:
      parameters:
        - name: "replicaCount"
          value: "3"
        - name: "service.type"
          value: "LoadBalancer"
        - name: "ingress.enabled"
          value: "true"
  destination:
    server: https://kubernetes.default.svc
    namespace: default
```

### Multiple Instances of the Same Parameter

When the same parameter is specified multiple times, the **last occurrence wins**:

```yaml
helm:
  parameters:
    - name: "replicaCount"
      value: "2"
    - name: "replicaCount"
      value: "5"   # This value will be used
```

### Combining Parameters with Other Value Sources

Parameters have the highest precedence, so they will override values from `valuesObject`, `values`, and `valueFiles`. This makes parameters ideal for environment-specific overrides that should always take precedence.

## Helm Value Precedence

When you specify values through multiple sources (parameters, inline values, values files), Argo CD merges them in a specific order. **Higher precedence values override lower precedence values.**

### Precedence Order (Highest to Lowest)

```
1. parameters (highest precedence – always wins)
2. valuesObject
3. values
4. valueFiles
5. Helm chart's default values.yaml (lowest precedence)
```

**Example:**

If you define the same key in multiple sources:

```yaml
source:
  helm:
    valueFiles:
      - values-defaults.yaml    # Sets foo: "from-file"
    values: |                   # Sets foo: "from-string"
      foo: "from-string"
    valuesObject:               # Sets foo: "from-object"
      foo: "from-object"
    parameters:                 # Sets foo: "from-parameter"
      - name: "foo"
        value: "from-parameter"
```

The final value of `foo` will be `"from-parameter"` because **parameters have the highest precedence**.

### Precedence of Multiple valueFiles

When multiple files are specified in `valueFiles`, the **last file has the highest precedence**:

```yaml
valueFiles:
  - values-defaults.yaml    # Lowest precedence
  - values-regional.yaml
  - values-environment.yaml # Highest precedence (overrides above files)
```

### Precedence of Multiple Parameters

When the same parameter is specified multiple times, the **last occurrence wins**:

```yaml
parameters:
  - name: "replicaCount"
    value: "2"
  - name: "replicaCount"
    value: "5"  # This value is used
```

### Practical Implications

- Use **parameters** for values that should always take precedence (e.g., environment-specific overrides).
- Use **valuesObject** or **values** for moderate complexity.
- Use **valueFiles** for defaults and can be overridden by parameters or inline values.
- Do not rely on `valuesObject` and `values` together; use one or the other (or combine with parameters/valueFiles for different purposes).

## Helm --set-file support

The `--set-file` argument to helm can be used with the following syntax on
the cli:

```bash
argocd app set helm-guestbook --helm-set-file some.key=path/to/file.ext
```

or using the fileParameters for yaml:

```yaml
source:
  helm:
    fileParameters:
      - name: some.key
        path: path/to/file.ext
```

## Common Helm Configuration Patterns

### Example 1: Deploying a Chart with Default Values (No Customization)

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: redis-app
spec:
  project: default
  source:
    repoURL: https://charts.bitnami.com/bitnami
    chart: redis
    targetRevision: 18.1.0
  destination:
    server: https://kubernetes.default.svc
    namespace: default
```

### Example 2: Using Parameters for Simple Environment-Specific Overrides

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: nginx-prod
spec:
  project: default
  source:
    repoURL: https://kubernetes.github.io/ingress-nginx
    chart: ingress-nginx
    targetRevision: 4.8.0
    helm:
      parameters:
        - name: "controller.replicaCount"
          value: "5"
        - name: "controller.service.type"
          value: "LoadBalancer"
        - name: "controller.resources.limits.cpu"
          value: "500m"
  destination:
    server: https://kubernetes.default.svc
    namespace: ingress-nginx
```

### Example 3: Using Inline Values for Complex Configurations

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: postgres-db
spec:
  project: default
  source:
    repoURL: https://charts.bitnami.com/bitnami
    chart: postgresql
    targetRevision: 12.5.0
    helm:
      valuesObject:
        auth:
          username: dbadmin
          password: ${DB_PASSWORD}  # Can use environment variables
        primary:
          persistence:
            enabled: true
            size: 50Gi
            storageClassName: ebs-gp3
        metrics:
          enabled: true
          serviceMonitor:
            enabled: true
  destination:
    server: https://kubernetes.default.svc
    namespace: default
```

### Example 4: Using Values Files for Environment-Specific Deployments

Directory structure in your Git repo:
```
helm-apps/
  values-common.yaml
  values-development.yaml
  values-staging.yaml
  values-production.yaml
  application.yaml
```

Application for **production** environment:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: myapp-prod
spec:
  project: default
  source:
    repoURL: https://github.com/acme/helm-apps
    path: helm-apps
    chart: myapp
    targetRevision: HEAD
    helm:
      valueFiles:
        - values-common.yaml      # Common defaults
        - values-production.yaml  # Production-specific overrides
  destination:
    server: https://kubernetes.default.svc
    namespace: production
```

Application for **staging** environment:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: myapp-staging
spec:
  project: default
  source:
    repoURL: https://github.com/acme/helm-apps
    path: helm-apps
    chart: myapp
    targetRevision: HEAD
    helm:
      valueFiles:
        - values-common.yaml       # Common defaults
        - values-staging.yaml      # Staging-specific overrides
  destination:
    server: https://kubernetes.default.svc
    namespace: staging
```

### Example 5: Combining Values Files and Parameters for Fine-Grained Control

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: prometheus-operator
spec:
  project: default
  source:
    repoURL: https://prometheus-community.github.io/helm-charts
    chart: kube-prometheus-stack
    targetRevision: 54.0.0
    helm:
      valueFiles:
        - values-monitoring.yaml  # Base monitoring configuration
      parameters:
        - name: "prometheus.prometheusSpec.retention"
          value: "30d"
        - name: "grafana.adminPassword"
          value: ${GRAFANA_PASSWORD}  # From environment
  destination:
    server: https://kubernetes.default.svc
    namespace: monitoring
```

## Helm Release Name

By default, the Helm release name is equal to the Application name to which it belongs. Sometimes, especially on a centralised Argo CD,
you may want to override that  name, and it is possible with the `release-name` flag on the cli:

```bash
argocd app set helm-guestbook --release-name myRelease
```

 or using the releaseName for yaml:

```yaml
source:
    helm:
      releaseName: myRelease
```

> [!WARNING]
> **Important notice on overriding the release name**
>
> Please note that overriding the Helm release name might cause problems when the chart you are deploying is using the `app.kubernetes.io/instance` label. Argo CD injects this label with the value of the Application name for tracking purposes. So when overriding the release name, the Application name will stop being equal to the release name. Because Argo CD will overwrite the label with the Application name it might cause some selectors on the resources to stop working. In order to avoid this we can configure Argo CD to use another label for tracking in the [ArgoCD configmap argocd-cm.yaml](../operator-manual/argocd-cm.yaml) - check the lines describing `application.instanceLabelKey`.

## Helm Hooks

Helm hooks are similar to [Argo CD hooks](resource_hooks.md). In Helm, a hook
is any normal Kubernetes resource annotated with the `helm.sh/hook` annotation.

Argo CD supports many (most?) Helm hooks by mapping the Helm annotations onto Argo CD's own hook annotations:

| Helm Annotation                 | Notes                                                                                         |
| ------------------------------- |-----------------------------------------------------------------------------------------------|
| `helm.sh/hook: crd-install`     | Supported as equivalent to normal Argo CD CRD handling.                                |
| `helm.sh/hook: pre-delete`      | Supported as equivalent to `argocd.argoproj.io/hook: PreDelete`                               |
| `helm.sh/hook: pre-rollback`    | Not supported. Never used in Helm stable.                                                     |
| `helm.sh/hook: pre-install`     | Supported as equivalent to `argocd.argoproj.io/hook: PreSync`.                                |
| `helm.sh/hook: pre-upgrade`     | Supported as equivalent to `argocd.argoproj.io/hook: PreSync`.                                |
| `helm.sh/hook: post-upgrade`    | Supported as equivalent to `argocd.argoproj.io/hook: PostSync`.                               |
| `helm.sh/hook: post-install`    | Supported as equivalent to `argocd.argoproj.io/hook: PostSync`.                               |
| `helm.sh/hook: post-delete`     | Supported as equivalent to `argocd.argoproj.io/hook: PostDelete`.                             |
| `helm.sh/hook: post-rollback`   | Not supported. Never used in Helm stable.                                                     |
| `helm.sh/hook: test-success`    | Not supported. No equivalent in Argo CD.                                                      |
| `helm.sh/hook: test-failure`    | Not supported. No equivalent in Argo CD.                                                      |
| `helm.sh/hook-delete-policy`    | Supported. See also `argocd.argoproj.io/hook-delete-policy`).                                 |
| `helm.sh/hook-delete-timeout`   | Not supported. Never used in Helm stable                                                      |
| `helm.sh/hook-weight`           | Supported as equivalent to `argocd.argoproj.io/sync-wave`.                                    |
| `helm.sh/resource-policy: keep` | Supported as equivalent to `argocd.argoproj.io/sync-options: Delete=false`.                   |

Unsupported hooks are ignored. In Argo CD, hooks are created by using `kubectl apply`, rather than `kubectl create`. This means that if the hook is named and already exists, it will not change unless you have annotated it with `before-hook-creation`.

> [!WARNING]
> **Helm hooks + ArgoCD hooks**
>
> If you define any Argo CD hooks, _all_ Helm hooks will be ignored.   

> [!WARNING]
> **'install' vs 'upgrade' vs 'sync'**
>
> Argo CD cannot know if it is running a first-time "install" or an "upgrade" - every operation is a "sync'. This means that, by default, apps that have `pre-install` and `pre-upgrade` will have those hooks run at the same time.

### Hook Tips

* Make your hook idempotent.
* Annotate  `pre-install` and `post-install` with `hook-weight: "-1"`. This will make sure it runs to success before any upgrade hooks.
* Annotate `pre-upgrade` and `post-upgrade` with `hook-delete-policy: before-hook-creation` to make sure it runs on every sync.

Read more about [Argo hooks](resource_hooks.md) and [Helm hooks](https://helm.sh/docs/topics/charts_hooks/).

## Random Data

Helm templating has the ability to generate random data during chart rendering via the
`randAlphaNum` function. Many helm charts from the [charts repository](https://github.com/helm/charts)
make use of this feature. For example, the following is the secret for the
[redis helm chart](https://github.com/helm/charts/blob/master/stable/redis/templates/secret.yaml):

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
value in the values.yaml or using `argocd app set` command to override the value such that the value
is stable between each comparison. For example:

```bash
argocd app set redis -p password=abc123
```

## Build Environment

Helm apps have access to the [standard build environment](build-environment.md) via substitution as parameters.

E.g. via the CLI:

```bash
argocd app create APPNAME \
  --helm-set-string 'app=${ARGOCD_APP_NAME}'
```

Or via declarative syntax:

```yaml
  spec:
    source:
      helm:
        parameters:
        - name: app
          value: $ARGOCD_APP_NAME
```

It's also possible to use build environment variables for the Helm values file path:

```yaml
  spec:
    source:
      helm:
        valueFiles:
        - values.yaml
        - myprotocol://somepath/$ARGOCD_APP_NAME/$ARGOCD_APP_REVISION
```

## Helm plugins

Argo CD is un-opinionated on what cloud provider you use and what kind of Helm plugins you are using, that's why there are no plugins delivered with the ArgoCD image.

But sometimes you want to use a custom plugin. Perhaps you would like to use Google Cloud Storage or Amazon S3 storage to save the Helm charts, for example: https://github.com/hayorov/helm-gcs where you can use `gs://` protocol for Helm chart repository access.
There are two ways to install custom plugins; you can modify the ArgoCD container image, or you can use a Kubernetes `initContainer`.

### Modifying the ArgoCD container image
One way to use this plugin is to prepare your own ArgoCD image where it is included.

Example `Dockerfile`:

```dockerfile
FROM argoproj/argocd:v1.5.7

USER root
RUN apt-get update && \
    apt-get install -y \
        curl && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

USER argocd

ARG GCS_PLUGIN_VERSION="0.3.5"
ARG GCS_PLUGIN_REPO="https://github.com/hayorov/helm-gcs.git"

RUN helm plugin install ${GCS_PLUGIN_REPO} --version ${GCS_PLUGIN_VERSION}

ENV HELM_PLUGINS="/home/argocd/.local/share/helm/plugins/"
```

The `HELM_PLUGINS` environment property required for ArgoCD to locate plugins correctly.

Once built, use the custom image for ArgoCD installation.

### Using `initContainers`
Another option is to install Helm plugins via Kubernetes `initContainers`.
Some users find this pattern preferable to maintaining their own version of the ArgoCD container image.

Below is an example of how to add Helm plugins when installing ArgoCD with the [official ArgoCD helm chart](https://github.com/argoproj/argo-helm/tree/master/charts/argo-cd):

```yaml
repoServer:
  volumes:
    - name: gcp-credentials
      secret:
        secretName: my-gcp-credentials
  volumeMounts:
    - name: gcp-credentials
      mountPath: /gcp
  env:
    - name: HELM_CACHE_HOME
      value: /helm-working-dir
    - name: HELM_CONFIG_HOME
      value: /helm-working-dir
    - name: HELM_DATA_HOME
      value: /helm-working-dir
  initContainers:
    - name: helm-gcp-authentication
      image: alpine/helm:3.16.1
      volumeMounts:
        - name: helm-working-dir
          mountPath: /helm-working-dir
        - name: gcp-credentials
          mountPath: /gcp
      env:
        - name: HELM_CACHE_HOME
          value: /helm-working-dir
        - name: HELM_CONFIG_HOME
          value: /helm-working-dir
        - name: HELM_DATA_HOME
          value: /helm-working-dir
      command: [ "/bin/sh", "-c" ]
      args:
        - apk --no-cache add curl;
          helm plugin install https://github.com/hayorov/helm-gcs.git;
          helm repo add my-gcs-repo gs://my-private-helm-gcs-repository;
          chmod -R 777 $HELM_DATA_HOME;
```

## Helm Version

Argo CD will assume that the Helm chart is v3 (even if the apiVersion field in the chart is Helm v2), unless v2 is explicitly specified within the Argo CD Application (see below).

If needed, it is possible to specifically set the Helm version to template with by setting the `helm-version` flag on the cli (either v2 or v3):

```bash
argocd app set helm-guestbook --helm-version v3
```

Or using declarative syntax:

```yaml
spec:
  source:
    helm:
      version: v3
```

## Helm `--pass-credentials`

Helm, [starting with v3.6.1](https://github.com/helm/helm/releases/tag/v3.6.1),
prevents sending repository credentials to download charts that are being served
from a different domain than the repository.

If needed, it is possible to opt into passing credentials for all domains by setting the `helm-pass-credentials` flag on the cli:

```bash
argocd app set helm-guestbook --helm-pass-credentials
```

Or using declarative syntax:

```yaml
spec:
  source:
    helm:
      passCredentials: true
```

## Helm `--skip-crds`

Helm installs custom resource definitions in the `crds` folder by default if they are not existing. 
See the [CRD best practices](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/) for details.

If needed, it is possible to skip the CRD installation step with the `helm-skip-crds` flag on the cli:

```bash
argocd app set helm-guestbook --helm-skip-crds
```

Or using declarative syntax:

```yaml
spec:
  source:
    helm:
      skipCrds: true
```

## Helm `--skip-schema-validation`

Helm validates the values.yaml file using a values.schema.json file. See [Schema files](https://helm.sh/docs/topics/charts/#schema-files) for details.

If needed, it is possible to skip the schema validation step with the `helm-skip-schema-validation` flag on the cli:

```bash
argocd app set helm-guestbook --helm-skip-schema-validation
```

Or using declarative syntax:

```yaml
spec:
  source:
    helm:
      skipSchemaValidation: true
```


## Helm `--skip-tests`

By default, Helm includes test manifests when rendering templates. Argo CD currently skips manifests that include hooks not supported by Argo CD, including [Helm test hooks](https://helm.sh/docs/topics/chart_tests/). While this feature covers many testing use cases, it is not totally congruent with --skip-tests, so the --skip-tests option can be used.

If needed, it is possible to skip the test manifests installation step with the `helm-skip-tests` flag on the cli:

```bash
argocd app set helm-guestbook --helm-skip-tests
```

Or using declarative syntax:

```yaml
spec:
  source:
    helm:
      skipTests: true # or false
```
