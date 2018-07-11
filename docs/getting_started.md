# ArgoCD Getting Started

An example Ksonnet guestbook application is provided to demonstrates how ArgoCD works.

## Requirements
* Installed [minikube](https://github.com/kubernetes/minikube#installation)
* Installed the [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) command-line tool
* Have a [kubeconfig](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/) file (default location is `~/.kube/config`).

## 1. Download ArgoCD

Download the latest ArgoCD version
```
curl -sSL -o /usr/local/bin/argocd https://github.com/argoproj/argo-cd/releases/download/v0.5.3/argocd-darwin-amd64
chmod +x /usr/local/bin/argocd
```


## 2. Install ArgoCD
```
kubectl create namespace argocd
kubectl apply -n argocd -f https://github.com/argoproj/argo-cd/blob/master/manifests/install.yaml
```
This will create a new namespace, `argocd`; where ArgoCD services and application resources will live.

## 3. Open access to ArgoCD API server

By default, the ArgoCD API server is not exposed with an external IP. To expose the API server,
change the service type to `LoadBalancer`:

```
kubectl patch svc argocd-server -n argocd -p '{"spec": {"type": "LoadBalancer"}}'
```

## 4. Login to the server from the CLI

```
argocd login $(minikube service argocd-server -n argocd --url | cut -d'/' -f 3) --name minikube
```

The ArgoCD cli is now configured to talk to API server and you can deploy your first application.

## 5. Register a cluster to deploy apps to

```
argocd cluster add minikube --in-cluster
```
The `argocd cluster add CONTEXTNAME` command installs an `argocd-manager` ServiceAccount and 
ClusterRole into the cluster associated with the supplied kubectl context. ArgoCD then uses the 
associated service account token to perform its management tasks (i.e. deploy/monitoring).

The `--in-cluster` option indicates that the cluster we are registering, is actually the same 
cluster that ArgoCD is running in. This allows ArgoCD to connect to the cluster using the internal
kubernetes hostname (kubernetes.default.svc). When registering an external cluster, you should omit
the `--in-cluster`.

## 6. Create the application from a git repository

### Creating apps via UI

Open a browser to the ArgoCD UI, and login using the admin credentials created during installation.

```
minikube service argocd-server -n argocd
```

Connect a git repository containing your apps. An example repository containing a sample 
guestbook application is available at https://github.com/argoproj/argocd-example-apps.git.

![connect repo](assets/connect_repo.png)

After connecting a git repository, select the guestbook application for creation:

![select repo](assets/select_repo.png)
![select app](assets/select_app.png)
![select env](assets/select_env.png)
![create app](assets/create_app.png)


### Creating apps via CLI

Applications can be also be created using the ArgoCD CLI:

```
argocd app create --name guestbook-default --repo https://github.com/argoproj/argocd-example-apps.git --path examples/guestbook --env minikube
```

## 7. Sync (deploy) the application

Once the guestbook application is created, you can now view its status:

From UI:
![create app](assets/guestbook-app.png)

From CLI:
```
$ argocd app get guestbook-default
Name:          guestbook-default
Server:        https://kubernetes.default.svc
Namespace:     default
URL:           https://192.168.64.36:31880/applications/argocd/guestbook-default
Environment:   default
Repo:          https://github.com/argoproj/argocd-example-apps.git
Path:          guestbook
Target:        HEAD

KIND        NAME          STATUS     HEALTH
Service     guestbook-ui  OutOfSync
Deployment  guestbook-ui  OutOfSync
```

The application status is initially in an `OutOfSync` state, since the application has yet to be
deployed, and no Kubernetes resources have been created. To sync (deploy) the application, run:

```
$ argocd app sync guestbook-default
Application:        guestbook-default
Operation:          Sync
Phase:              Succeeded
Message:            successfully synced

KIND        NAME          MESSAGE
Service     guestbook-ui  service "guestbook-ui" created
Deployment  guestbook-ui  deployment.apps "guestbook-ui" created
```

This command retrieves the manifests from the ksonnet app in the git repository and performs a 
`kubectl apply` of the manifests. The guestbook app is now running and you can now view its resource
components, logs, events, and assessed health:

![view app](assets/guestbook-tree.png)


## 8. Next Steps

ArgoCD supports additional features such as SSO, WebHooks, RBAC. See the following guides on setting
these up:
* [Configuring SSO](sso.md)
* [Configuring RBAC](rbac.md)
* [Configuring WebHooks](webhook.md)
