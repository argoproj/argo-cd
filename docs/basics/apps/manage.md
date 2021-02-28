# Managing Applications

You can manage *Applications* via the argocd CLI, the web UI or the Kubernetes
API. For managing *Applications* using either Argo CD's CLI or UI, 
[RBAC permissions](../rbac.md).
must be set-up for your user to allow manipulation of `applications` objects.

The default `admin` user already has appropriate permissions to manipulate all
existing *Applications* in your Argo CD installation.

## Using the argocd CLI

### List existing applications

To list all applications that you have authorization for, use the
`argocd app list` command. This will also give you the most important details
 about the applications:

```bash
$ argocd app list
NAME            CLUSTER                         NAMESPACE       PROJECT  STATUS  HEALTH   SYNCPOLICY  CONDITIONS  REPO                                             PATH            TARGET
helm-guestbook  https://kubernetes.default.svc  helm-guestbook  default  Synced  Healthy  <none>      <none>      https://github.com/argoproj/argocd-example-apps  helm-guestbook  HEAD
```

### Get details about an application

To get more detailed information about a specific *Application*, you can use the
`argocd app get` command. This will also display the resources that are managed
by the application:

```bash
$ argocd app get helm-guestbook
Name:               helm-guestbook
Project:            default
Server:             https://kubernetes.default.svc
Namespace:          helm-guestbook
URL:                http://127.0.0.1:8088/applications/helm-guestbook
Repo:               https://github.com/argoproj/argocd-example-apps
Target:             HEAD
Path:               helm-guestbook
SyncWindow:         Sync Allowed
Sync Policy:        <none>
Sync Status:        Synced to HEAD (0d3eec0)
Health Status:      Healthy

GROUP  KIND        NAMESPACE       NAME            STATUS     HEALTH   HOOK     MESSAGE
       Endpoints   helm-guestbook  helm-guestbook  Succeeded           PreSync  helm-guestbook created
       Service     helm-guestbook  helm-guestbook  Synced     Healthy           service/helm-guestbook unchanged
apps   Deployment  helm-guestbook  helm-guestbook  Synced     Healthy           deployment.apps/helm-guestbook configured
```

### Create new applications

To create a new *Application*, use the `argocd app create` command. The
following example creates an application `kustomize-guestbook` from the Argo
CD example applications repository, with a sync option to auto create the
target namespace:

```bash
$ argocd app create kustomize-guestbook \
  --project default \
  --repo https://github.com/argoproj/argocd-example-apps \
  --path kustomize-guestbook \
  --dest-namespace kustomize-guestbook \
  --dest-server https://kubernetes.default.svc \
  --sync-option CreateNamespace=true
application 'kustomize-guestbook' created
```

## Using the web UI

## Using the Kubernetes API
