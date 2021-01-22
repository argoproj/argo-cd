# Parameter Overrides

Argo CD provides a mechanism to override the parameters of Argo CD applications that leverages config management
tools. This provides flexibility in having most of the application manifests defined in Git, while leaving room
for *some* parts of the  k8s manifests determined dynamically, or outside of Git. It also serves as an alternative way of
redeploying an application by changing application parameters via Argo CD, instead of making the 
changes to the manifests in Git.

!!! tip
    Many consider this mode of operation as an anti-pattern to GitOps, since the source of
    truth becomes a union of the Git repository, and the application overrides. The Argo CD parameter
    overrides feature is provided mainly as a convenience to developers and is intended to be used in
    dev/test environments, vs. production environments.

To use parameter overrides, run the `argocd app set -p (COMPONENT=)PARAM=VALUE` command:

```bash
argocd app set guestbook -p image=example/guestbook:abcd123
argocd app sync guestbook
```

The `PARAM` is expected to be a normal YAML path

```bash
argocd app set guestbook -p ingress.enabled=true
argocd app set guestbook -p ingress.hosts[0]=guestbook.myclusterurl
```

The `argocd app set` [command](./commands/argocd_app_set.md) supports more tool-specific flags such as `--kustomize-image`, `--jsonnet-ext-var-str` etc
flags. You can also specify overrides directly in the source field on application spec. Read more about supported options in corresponded tool [documentation](./application_sources.md).

## When To Use Overrides?

The following are situations where parameter overrides would be useful:

1. A team maintains a "dev" environment, which needs to be continually updated with the latest
version of their guestbook application after every build in the tip of master. To address this use
case, the application would expose a parameter named `image`, whose value used in the `dev`
environment contains a placeholder value (e.g. `example/guestbook:replaceme`). The placeholder value
would be determined externally (outside of Git) such as a build system. Then, as part of the build
pipeline, the parameter value of the `image` would be continually updated to the freshly built image
(e.g. `argocd app set guestbook -p image=example/guestbook:abcd123`). A sync operation
would result in the application being redeployed with the new image.

2. A repository of Helm manifests is already publicly available (e.g. https://github.com/helm/charts).
Since commit access to the repository is unavailable, it is useful to be able to install charts from
the public repository and customize the deployment with different parameters, without resorting to
forking the repository to make the changes. For example, to install Redis from the Helm chart
repository and customize the the database password, you would run:

```bash
argocd app create redis --repo https://github.com/helm/charts.git --path stable/redis --dest-server https://kubernetes.default.svc --dest-namespace default -p password=abc123
```

## Store Overrides In Git

> The following is available from v1.8 or later

The config management tool specific overrides can be specified in `.argocd-source.yaml` file stored in the source application
directory in the Git repository.

!!! warn
    The `.argocd-source` is a beta feature and subject to change.

The `.argocd-source.yaml` file is used during manifest generation and overrides
application source fields, such as `kustomize`, `helm` etc.

Example:

```yaml
kustomize:
  images:
    - gcr.io/heptio-images/ks-guestbook-demo:0.2
```

The `.argocd-source` is trying to solve two following main use cases:

- Provide the unifed way to "override" application parameters in Git and enable the "write back" feature
for projects like [argocd-image-updater](https://github.com/argoproj-labs/argocd-image-updater).
- Support "discovering" applications in the Git repository by projects like [applicationset](https://github.com/argoproj-labs/applicationset)
(see [git files generator](https://github.com/argoproj-labs/applicationset/blob/master/examples/git-files-discovery.yaml))

> The following is available from v1.9 or later

You can also store parameter overrides in an application specific file, if you
are sourcing multiple applications from a single path in your repository.

The application specific file must be named `.argocd-source-<appname>.yaml`,
where `<appname>` is the name of the application the overrides are valid for.

If there exists an non-application specific `.argocd-source.yaml`, parameters
included in that file will be merged first, and then the application specific
parameters are merged, which can also contain overrides to the parameters
stored in the non-application specific file.