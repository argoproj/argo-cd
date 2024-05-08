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

!!! warning
    Directory-type applications only work for plain manifest files. If Argo CD encounters Kustomize, Helm, or Jsonnet files when directory: is set, it will fail to render the manifests.

## Including/Excluding Files

### Including Only Certain Files

To include only certain files/directories in a directory application, set the `include` option. The value is a glob
pattern.

For example, if you want to include only `.yaml` files, you can use this pattern:

```shell
argocd app set guestbook --directory-include "*.yaml"
```

!!! note
    It is important to quote `*.yaml` so that the shell does not expand the pattern before sending it to Argo CD.

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
