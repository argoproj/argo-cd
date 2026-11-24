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

## Webhook Configuration

By default, the ApplicationSet controller polls the OCI generator's registry every `requeueAfterSeconds`
interval (defaulting to every 3 minutes) to detect changes. To eliminate the polling delay, the
ApplicationSet webhook server can be configured to receive registry webhook events. The ApplicationSet
webhook currently supports registry webhook events from GitHub Container Registry (GHCR).

> [!NOTE]
> The ApplicationSet controller webhook does not use the same [API server webhook](../webhook.md). ApplicationSet
> exposes a webhook server as a service of type ClusterIP. An ApplicationSet specific Ingress resource needs to be
> created to expose this service to the webhook source.

When an artifact is pushed, the ApplicationSet controller refreshes every ApplicationSet with an OCI generator
whose `repoURL` and `revision` match the push. This includes OCI generators nested inside Matrix and Merge
generators.

- **`repoURL`** matches when it points at the pushed repository. Before comparing, the `oci://` prefix and any
  trailing `/` are removed and the URL is lowercased. For example, `oci://ghcr.io/my-org/platform` and
  `oci://GHCR.IO/My-Org/Platform/` both match a push to `ghcr.io/my-org/platform`.
- **`revision`** matches when it is the pushed tag, or a [semver constraint](https://github.com/Masterminds/semver#checking-version-constraints)
  that the pushed tag satisfies.

The `revision` decides whether a webhook is useful:

| `revision` | Webhook refreshes the ApplicationSet on push of                  |
|------------|------------------------------------------------------------------|
| `v1.2.3`   | Only `v1.2.3`, for example when the tag is overwritten           |
| `^1.2.0`   | `>=1.2.0` and `<2.0.0` (e.g., `1.2.1`, `1.9.0`)                  |
| `~1.2.0`   | `>=1.2.0` and `<1.3.0` (e.g., `1.2.1`, `1.2.9`)                  |
| `>=1.0.0`  | Any version `>=1.0.0`                                            |

For example, this ApplicationSet is refreshed whenever a new `1.x` tag of `ghcr.io/my-org/platform` is
published, and the new artifact is used to generate the Applications:

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
      revision: ^1.0.0
      directories:
      - path: platform/*
  template:
    metadata:
      name: '{{.path.basename}}'
    spec:
      project: "default"
      source:
        repoURL: oci://ghcr.io/my-org/platform
        targetRevision: ^1.0.0
        path: '{{.path.path}}'
      destination:
        server: https://kubernetes.default.svc
        namespace: '{{.path.basename}}'
```

> [!NOTE]
> The webhook refreshes the ApplicationSet, which regenerates its Applications. Applications that point at the
> same artifact, as in the template above, are refreshed by the [API server webhook](../webhook.md#3-webhook-configuration-for-oci-compliant-registries).
> To refresh both on the same push, configure a webhook for each.

### GitHub Container Registry (GHCR)

Webhooks cannot be registered directly on a GHCR image repository. Instead, `package` events are delivered from
the associated GitHub repository.

> [!NOTE]
> If your GHCR image repository is not yet linked to a GitHub repository, see [Connecting a repository to a package](https://docs.github.com/en/packages/learn-github-packages/connecting-a-repository-to-a-package).

#### 1. Create the webhook in GitHub

1. Go to your GitHub repository **Settings** → **Webhooks** → **Add webhook**
2. Set **Payload URL** to the `/api/webhook` endpoint of your ApplicationSet instance (e.g. `https://applicationset.example.com/api/webhook`)
3. Set **Content type** to `application/json`
4. Set **Secret** to a secure value
5. Under **Events**, select **Let me select individual events** and enable **Packages**

> [!NOTE]
> Only `published` events for `container` package types trigger a refresh. Other package types (npm, maven, etc.)
> and actions are ignored.

> [!WARNING]
> GitHub does not send `package` webhook events for artifacts with unknown media types. If your OCI artifact uses a
> custom or non-standard media type, the webhook will not be triggered. See [GitHub documentation on supported package types](https://docs.github.com/en/packages/learn-github-packages/about-permissions-for-github-packages).

#### 2. Configure ApplicationSet with the webhook secret (Optional)

GHCR webhooks use the same secret as GitHub Git webhooks (`webhook.github.secret`) in the `argocd-secret`
Kubernetes secret. When the secret is set, the ApplicationSet webhook rejects GHCR events without a valid
signature with `401 Unauthorized`.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: argocd-secret
  namespace: argocd
type: Opaque
stringData:
  webhook.github.secret: <your-webhook-secret>
```

After saving, restart the ApplicationSet controller for the changes to take effect.

The ApplicationSet webhook also rejects payloads larger than `webhook.maxPayloadSizeMB` in the `argocd-cm`
ConfigMap. See [Git generator webhook configuration](Generators-Git.md#1-create-the-webhook-in-the-git-provider) for details.

## Authentication

OCI generator uses the same authentication mechanisms as regular OCI-based Argo CD Applications. Configure repository credentials in the Argo CD UI or using repository secrets with `enableOCI: true`.

## Known Limitations

- **Artifact signing verification**: OCI artifacts may carry [cosign](https://github.com/sigstore/cosign)/sigstore signatures, but the OCI generator does not yet verify them. Source integrity enforcement (configured via `AppProject.spec.sourceIntegrity`) is therefore not yet supported for OCI generators.
