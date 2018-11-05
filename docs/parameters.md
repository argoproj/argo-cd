# Parameter Overrides

Argo CD provides a mechanism to override the parameters of a ksonnet/helm app. This gives some extra
flexibility in having most of the application manifests defined in git, while leaving room for
*some* parts of the k8s manifests determined dynamically, or outside of git. It also serves as an
alternative way of redeploying an application by changing application parameters via Argo CD, instead
of making the changes to the manifests in git.

**NOTE:** many consider this mode of operation as an anti-pattern to GitOps, since the source of
truth becomes a union of the git repository, and the application overrides. The Argo CD parameter
overrides feature is provided mainly convenience to developers and is intended to be used more for
dev/test environments, vs. production environments.

To use parameter overrides, run the `argocd app set -p (COMPONENT=)PARAM=VALUE` command:
```
argocd app set guestbook -p guestbook=image=example/guestbook:abcd123
argocd app sync guestbook
```

The following are situations where parameter overrides would be useful:

1. A team maintains a "dev" environment, which needs to be continually updated with the latest
version of their guestbook application after every build in the tip of master. To address this use
case, the application would expose an parameter named `image`, whose value used in the `dev`
environment contains a placeholder value (e.g. `example/guestbook:replaceme`). The placeholder value
would be determined externally (outside of git) such as a build systems. Then, as part of the build
pipeline, the parameter value of the `image` would be continually updated to the freshly built image
(e.g. `argocd app set guestbook -p guestbook=image=example/guestbook:abcd123`). A sync operation
would result in the application being redeployed with the new image.

2. A repository of helm manifests is already publicly available (e.g. https://github.com/helm/charts).
Since commit access to the repository is unavailable, it is useful to be able to install charts from
the public repository, customizing the deployment with different parameters, without resorting to
forking the repository to make the changes. For example, to install redis from the helm chart
repository and customize the the database password, you would run:

```
argocd app create redis --repo https://github.com/helm/charts.git --path stable/redis --dest-server https://kubernetes.default.svc --dest-namespace default -p password=abc123
```
