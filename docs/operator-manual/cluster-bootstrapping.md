# Cluster Bootstrapping

This guide is for operators who have already installed Argo CD, and have a new cluster and are looking to install many apps in that cluster.

There's no one particular pattern to solve this problem, e.g. you could write a script to create your apps, or you could even manually create them.

Our recommendation is to look at [ApplicationSets](./applicationset/index.md) and more specifically the [cluster generator](./applicationset/Generators-Cluster.md) which can handle most typical scenarios.

## Application Sets and cluster labels (recommended)

Following the [Declaratively setup guide](declarative-setup.md) you can create a cluster and assign it several labels.

Example

```yaml
apiVersion: v1
data:
  [...snip..]
kind: Secret
metadata:
  annotations:
    managed-by: argocd.argoproj.io
  labels:
    argocd.argoproj.io/secret-type: cluster
    cloud: gcp
    department: billing
    env: qa
    region: eu
    type: workload
  name: cluster-qa-eu-example
  namespace: argocd
```

Then as soon as you add the cluster to Argo CD, any application set that uses these labels will deploy the respective applications.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: eu-only-appset
  namespace: argocd
spec:
  goTemplate: true
  goTemplateOptions: ["missingkey=error"]
  generators:
  - matrix:
      generators:
        - git:
            repoURL: <a git repo>
            revision: HEAD
            directories:
            - path: my-eu-apps/*
        - clusters:    
            selector:
              matchLabels:
                type: "workload"     
                region: "eu"                     
  template:      
    metadata:
      name: 'eu-only-{{index .path.segments 1}}-{{.name}}'     
    spec:
      project: default
      source:
        repoURL: <a git repo>
        targetRevision: HEAD
        path: '{{.path.path}}'
      destination:
        server: '{{.server}}'
        namespace: 'eu-only-{{index .path.segments 1}}'

      syncPolicy:
        syncOptions:
          - CreateNamespace=true  
        automated: 
          prune: true
          selfHeal: true 
```

If you use Application Sets you also have access to all [gotemplate functions](./applicationset/GoTemplate.md) as well as [Sprig methods](https://masterminds.github.io/sprig/). So no Helm templating is required.

For more information see also [Templating](./applicationset/Template.md).


## App Of Apps Pattern (Alternative)

 You can also use the **app of apps pattern**.

> [!WARNING]
> **App of Apps is an admin-only tool**
>
> The ability to create Applications in arbitrary [Projects](./declarative-setup.md#projects) 
> is an admin-level capability. Only admins should have push access to the parent Application's source repository. 
> Admins should review pull requests to that repository, paying particular attention to the `project` field in each 
> Application. Projects with access to the namespace in which Argo CD is installed effectively have admin-level 
> privileges.

[Declaratively](declarative-setup.md) specify one Argo CD app that consists only of other apps.

![Application of Applications](../assets/application-of-applications.png)

### Helm Example


This example shows how to use Helm to achieve this. While Helm is used in this example, the same pattern works with any configuration management tool supported by Argo CD (e.g. Kustomize, plain YAML, Jsonnet), or even a custom [Config Management Plugin](./config-management-plugins.md).

### Repository Layout

A typical Git repository layout for a Helm-based app of apps is as follows:

```
├── Chart.yaml
├── values.yaml
├── templates
│   ├── guestbook.yaml
│   ├── helm-dependency.yaml
│   ├── helm-guestbook.yaml
│   └── kustomize-guestbook.yaml
```

- `Chart.yaml` defines the Helm chart for the parent app.
- `values.yaml` contains the default values shared across all child applicaitons.
- `templates/` contains one Helm template per child Application.

### Child Application Template
Each file under `templates/` defines a child Argo CD Application. You can either:

- Define one file per child Application, or
- Use a single Helm template that loops over multiple applications defined in `values.yaml`.

#### Pattern 1: One Template per Child Application

This pattern defines one YAML file per child Application. It is easier to read and reason about, and works well when managing a small number of child Applications.

For example, `templates/guestbook.yaml` contains:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: guestbook
  namespace: argocd
  finalizers:
  - resources-finalizer.argocd.argoproj.io
spec:
  destination:
    namespace: argocd
    server: {{ .Values.spec.destination.server }}
  project: default
  source:
    path: guestbook
    repoURL: https://github.com/argoproj/argocd-example-apps
    targetRevision: HEAD
  syncPolicy:
    automated:
      prune: true
```

#### Pattern 2: Loop Over Multiple Child Applications

This pattern uses a single Helm template that loops over multiple child Applications defined in `values.yaml`. It is more compact and works well when managing a large number of child Applications.

For example, `templates/apps.yaml` contains:

```yaml
{{- range .Values.applications }}
{{- $config := $.Values.config -}}
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: {{ printf "example.%s" .name | quote }}
  namespace: argocd
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  destination:
    namespace: {{ .namespace | default .name | quote }}
    server: {{ $config.spec.destination.server | quote }}
  project: default
  source:
    path: {{ .path | default .name | quote }}
    repoURL: {{ $config.spec.source.repoURL }}
    targetRevision: {{ $config.spec.source.targetRevision }}
    {{- with .tool }}
    {{- . | toYaml | nindent 4 }}
    {{- end }}
  syncPolicy:
    syncOptions:
      - CreateNamespace=true
    automated:
      prune: true
      selfHeal: true
```
In the above pattern:

- `range .Values.applications` loops over each child Application defined in the `applications` list in `values.yaml`.
- `$.Values.config` accesses shared configuration values defined in `values.yaml`.
- The finalizer is applied only to child Applications to ensure proper cleanup.

### Fixing vs Floating Revisions
You may either:
- Pin to a specific commit SHA:
```yaml
    targetRevision: d4c3db5f5e2f4b6c8e8f7e6d5c4b3a2f1e0d9c8b
```
- Float to the latest on a branch:
```yaml
    targetRevision: HEAD
```
- Float to a specific branch or tag:
```yaml
    targetRevision: staging
```

Pinning ensures child Applications only change when the parent Application changes. Even if the child apps repo changes, the child app will only change when the parent app change that revision.

### `values.yaml`
The `values.yaml` file provides default values shared across all child Applications:
```yaml
spec:
  destination:
    server: https://kubernetes.default.svc
```

This allows you to deploy the same App-of-Apps to multiple clusters by overriding values at install time.

### Parent Application
The parent Application points to the Helm chart itself and is usually created once (manually or via CI). 

Generally, the sync policy for the parent app is set to automated with pruning enabled, so child Applications are deleted when removed from Git. No finalizer is required on the parent app.


### Creating the Parent Application
Create and sync the parent application using the Argo CD CLI:

```bash
argocd app create apps \
    --dest-namespace argocd \
    --dest-server https://kubernetes.default.svc \
    --repo https://github.com/argoproj/argocd-example-apps.git \
    --path apps  
argocd app sync apps  
```

After syncing, the parent application will appear as Synced. However, the child applications will initially appear as `OutOfSync`, since they have not yet been created in the cluster, hence they must be reconciled individually.

When a child Application is removed from Git, it will be deleted from the cluster automatically due to the parent's automated sync policy with pruning enabled. The finalizer on the child Applications ensures that all resources managed by the child Applications are fully removed before the child Application itself is deleted.

![New App Of Apps](../assets/new-app-of-apps.png)

> [!NOTE]
> You may want to modify this behavior to bootstrap your cluster in waves; see [the health assessment of Applications](./health.md#argocd-app) for information on changing this.

You can either sync via the UI, firstly filter by the correct label:

![Filter Apps](../assets/filter-apps.png)

Then select the "out of sync" apps and sync: 

![Sync Apps](../assets/sync-apps.png)

Or, via the CLI: 

```bash
argocd app sync -l app.kubernetes.io/instance=apps
```

View [the example on GitHub](https://github.com/argoproj/argocd-example-apps/tree/master/apps).



### Cascading deletion

If you want to ensure that child-apps and all of their resources are deleted when the parent-app is deleted make sure to add the appropriate [finalizer](../user-guide/app_deletion.md#about-the-deletion-finalizer) to your `Application` definition

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: guestbook
  namespace: argocd
  finalizers:
  - resources-finalizer.argocd.argoproj.io
spec:
 ...
```

### Deleting child applications

When working with the App of Apps pattern, you may need to delete individual child applications. Starting in 3.2, Argo CD provides consistent deletion behaviour whether you delete from the Applications List or from the parent application's Resource Tree.

For detailed information about deletion options and behaviour, including:
- Consistent deletion across UI views
- Non-cascading (orphan) deletion to preserve managed resources
- Child application detection and improved dialog messages
- Best practices and example scenarios

See [Deleting Applications in the UI](../user-guide/app_deletion.md#deleting-applications-in-the-ui).

### Ignoring differences in child applications

To allow changes in child apps without triggering an out-of-sync status, or modification for debugging etc, the app of apps pattern works with [diff customization](../user-guide/diffing.md). The example below shows how to ignore changes to syncPolicy and other common values.

```yaml
spec:
  ...
  syncPolicy:
    ...
    syncOptions:
      - RespectIgnoreDifferences=true
    ...
  ignoreDifferences:
    - group: "*"
      kind: "Application"
      jsonPointers:
        # Allow manually disabling auto sync for apps, useful for debugging.
        - /spec/syncPolicy/automated
        # These are automatically updated on a regular basis. Not ignoring last applied configuration since it's used for computing diffs after normalization.
        - /metadata/annotations/argocd.argoproj.io~1refresh
        - /operation
  ...
```
