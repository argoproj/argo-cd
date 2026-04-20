# OCI Generator

The OCI generator works similarly to the Git generator but uses OCI artifacts as the source instead of Git repositories. This is particularly useful when you want to distribute pre-rendered or hydrated Kubernetes manifests as versioned, immutable artifacts.

Like the Git generator, the OCI generator has two subtypes: the OCI directory generator and the OCI file generator.

## OCI Generator: Directories

The OCI directory generator generates parameters using the directory structure within an OCI artifact. This is useful when you want to bundle multiple applications together in a single versioned artifact.

Let's say you have the following directory structure that you want to deploy:

```
.
└── platform
    ├── cert-manager
    │   └── manifest.yaml
    └── prometheus
        └── manifest.yaml
```

You can push this entire structure as a single OCI artifact using a tool like `oras`:

```bash
oras push ghcr.io/my-org/platform:v1.2.3 .
```

Now you can use an ApplicationSet to automatically deploy all directories within this artifact:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: platform-apps
  namespace: argocd
spec:
  goTemplate: true
  goTemplateOptions: ["missingkey=error"]
  generators:
  - oci:
      repoURL: oci://ghcr.io/my-org/platform
      revision: v1.2.3
      directories:
      - path: platform/*
  template:
    metadata:
      name: '{{.path.basename}}'
    spec:
      project: "default"
      source:
        repoURL: oci://ghcr.io/my-org/platform
        targetRevision: v1.2.3
        path: '{{.path.path}}'
      destination:
        server: https://kubernetes.default.svc
        namespace: '{{.path.basename}}'
      syncPolicy:
        syncOptions:
        - CreateNamespace=true
```

The generator parameters work the same way as the Git generator:

- `{{.path.path}}`: The directory paths within the OCI artifact that match the `path` wildcard.
- `{{index .path.segments n}}`: The directory paths split into array elements (`n` - array index).
- `{{.path.basename}}`: The rightmost directory name (e.g., `platform/cert-manager` produces `cert-manager`).
- `{{.path.basenameNormalized}}`: Same as `path.basename` but with unsupported characters replaced with `-`.

**Note**: If the `pathParamPrefix` option is specified, all `path`-related parameter names will be prefixed with the specified value and a dot separator. This is useful when combining multiple generators in a Matrix generator.

With this setup, the ApplicationSet controller will create two Applications: one for `cert-manager` and one for `prometheus`. When you push a new version of the OCI artifact (e.g., `v1.2.4`), you just need to update the `revision` field in the ApplicationSet.

## OCI Generator: Files

The OCI file generator works by reading JSON/YAML files from an OCI artifact and using their contents to generate parameters. Each file can contain either a single object or an array of objects.

Suppose you have an OCI artifact with this structure:

```
.
└── clusters
    ├── staging.yaml
    └── production.yaml
```

Where `staging.yaml` contains:

```yaml
environment: staging
replicas: 2
region: us-east-1
```

And `production.yaml` contains:

```yaml
environment: production
replicas: 5
region: us-west-2
```

You can use the file generator to create Applications based on these files:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: cluster-configs
  namespace: argocd
spec:
  goTemplate: true
  goTemplateOptions: ["missingkey=error"]
  generators:
  - oci:
      repoURL: oci://ghcr.io/my-org/configs
      revision: v2.1.0
      files:
      - path: "clusters/*.yaml"
  template:
    metadata:
      name: '{{.environment}}-app'
    spec:
      project: default
      source:
        repoURL: https://github.com/my-org/app-manifests
        targetRevision: HEAD
        path: overlays/{{.environment}}
        helm:
          parameters:
          - name: replicas
            value: '{{.replicas}}'
          - name: region
            value: '{{.region}}'
      destination:
        server: https://kubernetes.default.svc
        namespace: '{{.environment}}'
```

This will create two Applications: one for staging with 2 replicas in us-east-1, and one for production with 5 replicas in us-west-2.

The file contents are parsed as YAML/JSON, and the top-level fields become available as parameters. Path information is also available:

- `{{.path.path}}`: Directory containing the file
- `{{.path.filename}}`: Name of the file
- `{{.path.basename}}`: Directory name
- `{{.path.basenameNormalized}}`: Directory name with special characters replaced

## Authentication

OCI generator uses the same authentication mechanisms as regular OCI-based Argo CD Applications. Configure repository credentials in the Argo CD UI or using repository secrets with `enableOCI: true`.
