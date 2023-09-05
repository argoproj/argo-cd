# Multiple Sources for an Application

!!! warning "Beta Feature"
    Specifying multiple sources for an application is a beta feature. The UI and CLI still generally behave as if only
    the first source is specified. Full UI/CLI support will be added in a future release.
    This feature is subject to change in backwards incompatible ways until it is marked stable.

Argo CD has the ability to specify multiple sources for a single Application. Argo CD compiles all the sources
and reconciles the combined resources.

You can provide multiple sources using the `sources` field. When you specify the `sources` field, Argo CD will ignore 
the `source` (singular) field.

See the below example for specifying multiple sources:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: guestbook
  namespace: argocd
spec:
  project: default
  destination:
    server: https://kubernetes.default.svc
    namespace: default
  sources:
    - chart: elasticsearch
      repoURL: https://helm.elastic.co
      targetRevision: 8.5.1
    - repoURL: https://github.com/argoproj/argocd-example-apps.git
      path: guestbook
      targetRevision: HEAD
```

The above example has two sources specified. Argo CD will generate the manifests for each source separately and combine 
the resulting manifests.

If multiple sources produce the same resource (same `group`, `kind`, `name`, and `namespace`), the last source to 
produce the resource will take precedence. Argo CD will produce a `RepeatedResourceWarning` in this case, but it will 
sync the resources. This provides a convenient way to override a resource from a chart with a resource from a Git repo.

## Helm value files from external Git repository

Helm sources can reference value files from git sources. This allows you to use a third-party Helm chart with custom,
git-hosted values.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
spec:
  sources:
  - repoURL: 'https://prometheus-community.github.io/helm-charts'
    chart: prometheus
    targetRevision: 15.7.1
    helm:
      valueFiles:
      - $values/charts/prometheus/values.yaml
  - repoURL: 'https://git.example.com/org/value-files.git'
    targetRevision: dev
    ref: values
```

In the above example, the `prometheus` chart will use the value file from `git.example.gom/org/value-files.git`. 
`$values` resolves to the root of the `value-files` repository. The `$values` variable may only be specified at the 
beginning of the value file path.

If the `path` field is set in the `$values` source, Argo CD will attempt to generate resources from the git repository
at that URL. If the `path` field is not set, Argo CD will use the repository solely as a source of value files.

!!! note
    Sources with the `ref` field set must not also specify the `chart` field. Argo CD does not currently support using  
    another Helm chart as a source for value files.

## Copying files across sources

In some cases the builtin helm, kustomize or directory processing cannot be used due to the utilization of Config Management Plugins (CMPs).
In these cases the `helm.valueFiles` referencing of different sources cannot be used, therefore files has to be made accessible using different
means. Currently this is being done by a `from:` section in the source.

The `from:` section is a list with the following elements:

```yaml
from:
  - sourcePath: $reporef/from/here
    destinationPath: to/here
    failOnOutOfBoundsSymlink: true
```

The meaning of the fields:

1. `ref`: (required) This is the referenced repository, the referenced name has to be prefix with a dollar(`$`) sign. This repository will be the source of the copy operation.
1. `sourcePath`: (required) The path relative to the referenced source's repository's root directory. The file or directory specified by this path will be copied.
1. `destinationPath`: (required) The path relative to the current application's repository's directory, this will be the destination of the copy
1. `failOnOutOfBoundsSymlink`: (optional) Marks the handling of out-of-bound symlinks. If `failOnOutOfBoundsSymlink` is `true` then processing will error out if any out-of-bound symlinks are encountered. If it's `false`, then these will be ignored, and a warning logged about them. Default is `false`.

For the git repositories, the `path` field is ignored for copy operations, the `sourcePath` and `destinationPath` is relative to the repository's root.

### Example: Copying a single file, with plugin processing

In this example there's a helm chart, and it's parameterized with a values file from another repository, to be processed by a plugin.

```yaml
spec:
  sources:
    - repoURL: http://my/repo.git
      targetRevision: master
      ref: valuesrepo
    - repoURL: http://helm.example/
      targetRevision: 42.0.1
      chart: example-chart
      plugin:
        env:
          - name: VALUES
            value: env-values.yaml
      from:
        - sourcePath: "$valuesrepo/dev/values.yaml"
          destinationPath: "env-values.yaml"
```

The `valuesrepo` has the following layout, relative to the repo root:

```
.
dev
dev/values.yaml
```

The result will be `$valuesrepo/dev/values.yaml` copied over to the helm repository's root as `env-values.yaml`.

The plugin section tells the plugin about the location and name of the extra values file in an environment variable, and using that the value file from the other source passed as `helm template (...) -f $ARGOCD_ENV_VALUES | our-faveorite-plugin`, and thus our manifest is generated in a multisource setup, processed through a plugin.


### Copying a directory structure

The following example copies a complete directory structure over.

Relevant part of the Application specification:

```yaml
spec:
  sources:
    - repoURL: http://my/repo.git
      targetRevision: master
      ref: valuesrepo
    - repoURL: http://helm.example/
      targetRevision: 42.0.1
      chart: example-chart
      from:
        - sourcePath: "$valuesrepo/copyfrom-oob"
          destinationPath: "dst-dir"
          failOnOutOfBoundsSymlink: false
```

The layout of the source repository is the following:
```
./copyfrom-oob
./copyfrom-oob/baz
./copyfrom-oob/baz/boo -> ../../oob/nice
./copyfrom-oob/foo
./copyfrom-oob/foo/prayer.txt
./copyfrom-oob/foo/bar
```

The structure at the destination will be the following:
```
./
./dst-dir
./dst-dir/baz
./dst-dir/foo
./dst-dir/foo/prayer.txt
./dst-dir/foo/bar
```

The directory marked at `sourcePath` will be copied over to the target repository under the name marked as `destinationPath`.

Please note that the `baz/boo` symlink is not present at the directory, because that was out-of-bound. If `failOnOutOfBoundsSymlink: true` is set, then the processing of this entry would have errored out instead.
