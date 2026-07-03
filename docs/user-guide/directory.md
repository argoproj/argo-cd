# Directory

A directory-type application loads plain manifest files from `.yml`, `.yaml`, and `.json` files. A directory-type
application may be created from the UI, CLI, or declaratively. This is the declarative syntax:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: guestbook
spec:
  destination:
    namespace: default
    server: https://kubernetes.default.svc
  project: default
  source:
    path: guestbook
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    targetRevision: HEAD
```

It's unnecessary to explicitly add the `spec.source.directory` field except to add additional configuration options.
Argo CD will automatically detect that the source repository/path contains plain manifest files.

## Enabling Recursive Resource Detection

By default, directory applications will only include the files from the root of the configured repository/path.

To enable recursive resource detection, set the `recurse` option.

```bash
argocd app set guestbook --directory-recurse
```

To do the same thing declaratively, use this syntax:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
spec:
  source:
    directory:
      recurse: true
```

> [!WARNING]
> Directory-type applications only work for plain manifest files. If Argo CD encounters Kustomize, Helm, or Jsonnet files when directory: is set, it will fail to render the manifests.

## Including/Excluding Files

### Including Only Certain Files

To include only certain files/directories in a directory application, set the `include` option. The value is a glob
pattern.

For example, if you want to include only `.yaml` files, you can use this pattern:

```shell
argocd app set guestbook --directory-include "*.yaml"
```

> [!NOTE]
> It is important to quote `*.yaml` so that the shell does not expand the pattern before sending it to Argo CD.

It is also possible to include multiple patterns. Wrap the patterns with `{}` and separate them with commas. To include
`.yml` and `.yaml` files, use this pattern:

```shell
argocd app set guestbook --directory-include "{*.yml,*.yaml}"
```

To include only a certain directory, use a pattern like this:

```shell
argocd app set guestbook --directory-include "some-directory/*"
```

To accomplish the same thing declaratively, use this syntax:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
spec:
  source:
    directory:
      include: 'some-directory/*'
```

### Excluding Certain Files

It is possible to exclude files matching a pattern from directory applications. For example, in a repository containing
some manifests and also a non-manifest YAML file, you could exclude the config file like this:

```shell
argocd app set guestbook --directory-exclude "config.yaml"
```

It is possible to exclude more than one pattern. For example, a config file and an irrelevant directory:

```shell
argocd app set guestbook --directory-exclude "{config.yaml,env-use2/*}"
```

If both `include` and `exclude` are specified, then the Application will include all files which match the `include`
pattern and do not match the `exclude` pattern. For example, consider this source repository:

```
config.json
deployment.yaml
env-use2/
  configmap.yaml
env-usw2/
  configmap.yaml
```

To exclude `config.json` and the `env-usw2` directory, you could use this combination of patterns:

```shell
argocd app set guestbook --directory-include "*.yaml" --directory-exclude "{config.json,env-usw2/*}"
```

This would be the declarative syntax:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
spec:
  source:
    directory:
      exclude: '{config.json,env-usw2/*}'
      include: '*.yaml'
```

### Allowing Custom File Extensions

By default, a directory-type application only considers files with the built-in manifest extensions
`.yaml`, `.yml`, `.json`, and `.jsonnet`. Any other file is skipped *before* the `include`/`exclude`
patterns are ever evaluated. This means an `include` pattern like `*.yaml.sealed` has no effect on its
own since the file is filtered out by extension first, so the pattern never gets a chance to match.

This is a problem for workflows that store manifests under a non-standard extension by convention. A
common example is [Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets), where encrypted
resources are named `*.yaml.sealed` to signal that they are sealed rather than plain YAML.

To support these cases, set `allowCustomExtensions: true`. When enabled, the built-in extension filter is
**disabled entirely**, and the `include`/`exclude` glob patterns become the *only* mechanism that decides
which files are treated as manifests.

```shell
argocd app set guestbook --directory-allow-custom-extensions --directory-include "{*.yaml.sealed,*.yaml,*.yml,*.json}"
```

To accomplish the same thing declaratively, use this syntax:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
spec:
  source:
    directory:
      allowCustomExtensions: true
      include: '{*.yaml.sealed,*.yaml,*.yml,*.json}'
```

> [!IMPORTANT]
> When `allowCustomExtensions` is `true`, **you are fully responsible for filtering.** Because the
> built-in extension filter is turned off, the standard `.yaml`, `.yml`, `.json`, and `.jsonnet`
> extensions are **no longer matched automatically** so you must list every extension you want in the
> `include` pattern, including the standard ones. If your example above only used
> `include: '*.yaml.sealed'`, your regular `.yaml` and `.json` manifests would be silently ignored.

#### Why is this opt-in and defaulted to off?

The field defaults to `false` so that existing applications behave exactly as before as turning on custom
extensions is a deliberate choice, never a silent change. The built-in extension filter also acts as a
safety net: it keeps unrelated files (documentation, license files, images, lockfiles) out of manifest
generation without any configuration. Disabling it hands that responsibility to you, which is why it is
gated behind an explicit flag.

#### Filtering behavior with the flag enabled

The `include` and `exclude` patterns work exactly as described in the sections above. `exclude` is applied
first, then `include`. The only difference is that the extension pre-filter is skipped. So a file is
selected when it matches `include` (or `include` is empty) **and** does not match `exclude`.

If you set `allowCustomExtensions: true` **without** any `include` or `exclude`, then *every* file in the
directory is read and treated as a candidate manifest. Argo CD logs a warning in this case because it is
rarely what you want:

- Non-manifest files (that don't contain `apiVersion`, `kind`, and `metadata`) are still safely ignored,
  but every file is read from disk and its size counts toward the maximum combined manifest size limit
  (the `reposerver.max.combined.directory.manifests.size` setting, default `10M`) which is a large binary in the
  directory that can cause manifest generation to fail.
- A stray file that happens to be a valid Kubernetes manifest would be applied to your cluster
  unintentionally.

This "consider everything" behavior is intentional and can be useful for repositories that are structured
so that *every* file is a manifest regardless of extension. If that is your case, prefer scoping it with an
`exclude` pattern for the known non-manifest files rather than leaving both patterns empty:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
spec:
  source:
    directory:
      allowCustomExtensions: true
      exclude: '{*.md,LICENSE,*.png}'
```

### Skipping File Rendering

In some cases, repositories may contain YAML files that resemble Kubernetes manifests because they include fields like `apiVersion`, `kind`, and `metadata`, but are not intended to be rendered or applied as actual Kubernetes resources. Examples include Helm `values.yaml` files or configuration snippets used by CI/CD pipelines.

To prevent Argo CD from attempting to parse these files as manifests (which could result in errors), you can explicitly mark them to be skipped using a special comment directive:

```yaml
# +argocd:skip-file-rendering
```

When this comment is present anywhere in the file, Argo CD will ignore the file during manifest processing. This allows for safe coexistence of Kubernetes-like files that are not actual manifests.

#### Example

```yaml
# +argocd:skip-file-rendering
apiVersion: v1
kind: ConfigMap
metadata:
  name: example
data:
  not-actually: a-manifest
```

