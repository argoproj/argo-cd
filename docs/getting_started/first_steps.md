# Accessing Argo CD

After installing the [server component](install.md) and the [Argo CD CLI](install_cli.md) you are now ready to begin deploying applications.


## Exposing the API and web UI

To start using either the CLI or the Web interface you need to expose the `argocd-server` service first. By default, after the initial installation
this service is only available from within the cluster itself (as ClusterIP).

The process of exposing the [service](https://kubernetes.io/docs/concepts/services-networking/service/) is not specific to Argo CD. As Argo CD is a Kubernetes native application you can follow all familiar ways to expose its service to the outside worlds. If your organization has a well defined policy on how Kubernetes services are exposed, then we advise you to follow that policy first.

### Using an Ingress (Recommended)

For a production Argo CD installation you should use [a Kubernetes Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/) to expose the Argo CD service.

See the [Ingress guide](../operations/ingress/index.md) for more details.

After you setup your ingress, you should also commit all associated files into Git and manage them with Argo CD as well (therefore following GitOps for Argo CD itself).

### Using a Load balancer

A quick way to expose Argo CD to the whole world is to simply convert the existing ClusterIP service to a Loadbalancer one.

```bash
kubectl patch svc argocd-server -n argocd -p '{"spec": {"type": "LoadBalancer"}}'
```

Then the Argo CD service will get its own separate IP address and optionally a DNS name if you have this option setup.

If you use this method in a permanent manner, beware of the cost impact. A load balancer is usually priced separately on most cloud providers. Make sure you understand the cost implications of this method. An ingress is almost always a cost-effective method if you have multiple services in a single cluster.



!!! warning
    While this method is very easy to execute, we suggest you only use it for local clusters, product demos and proof-of-concept scenarios. It also opens your Argo CD installation to the whole world. Make sure that you  have a firewall and/or extra security constraints and know how users will access Argo CD.

### Using Port forwarding

Kubectl port-forwarding can also be used to connect to the API server without exposing the service.

```bash
kubectl port-forward svc/argocd-server -n argocd 8080:443
```

The API server can then be accessed using `https://localhost:8080`

This method is expected to be temporary while you debug something or experiment locally. It is **NOT** recommended for production Argo CD installations.

## Find the initial password

On a default installation, Argo CD has one user defined with the following credentials

* username = `admin`
* password = randomly generated

To find the password execute the following:

```bash
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d; echo
```

!!! warning
    You should delete the `argocd-initial-admin-secret` from the Argo CD
    namespace once you changed the password. The secret serves no other
    purpose than to store the initially generated password in clear and can
    safely be deleted at any time. It will be re-created on demand by Argo CD
    if a new admin password must be re-generated.

## Use the CLI to deploy an application

To start deploying resources you need to authenticate with the CLI and then create [Argo CD applications](../basics/apps/index.md).

### CLI Login

Using the username `admin` and the password from above, login to Argo CD's IP or hostname:

```bash
argocd login <ARGOCD_SERVER>
```

The value of `ARGOCD_SERVER` depends on the method you used in the previous section. It can be an IP address, a hostname such as `argocd.example.com` or even `localhost` if you use a local cluster.

!!! note
    The CLI environment must be able to communicate with the Argo CD API server. If it isn't directly accessible as described in the previous section, you can tell the CLI to access it using port forwarding through one of these mechanisms: 1) add `--port-forward-namespace argocd` flag to every CLI command; or 2) set `ARGOCD_OPTS` environment variable: `export ARGOCD_OPTS='--port-forward-namespace argocd'`.

You can change the admin password using the command:

```bash
argocd account update-password
```

### Create an application

Create the [example guestbook application](https://github.com/argoproj/argocd-example-apps/tree/master/guestbook) with the following command:

```bash
argocd app create guestbook --repo https://github.com/argoproj/argocd-example-apps.git --path guestbook --dest-server https://kubernetes.default.svc --dest-namespace default
```

### Sync the application

Once the guestbook application is created, you can now view its status:

```bash
$ argocd app get guestbook
Name:               guestbook
Server:             https://kubernetes.default.svc
Namespace:          default
URL:                https://10.97.164.88/applications/guestbook
Repo:               https://github.com/argoproj/argocd-example-apps.git
Target:
Path:               guestbook
Sync Policy:        <none>
Sync Status:        OutOfSync from  (1ff8a67)
Health Status:      Missing

GROUP  KIND        NAMESPACE  NAME          STATUS     HEALTH
apps   Deployment  default    guestbook-ui  OutOfSync  Missing
       Service     default    guestbook-ui  OutOfSync  Missing
```

The application status is initially in `OutOfSync` state since the application has yet to be
deployed, and no Kubernetes resources have been created. To [sync](../syncing/index.md) (deploy) the application, run:

```bash
argocd app sync guestbook
```

This command retrieves the manifests from the [repository](../basics/repos/index.md) and performs a `kubectl apply` of the
manifests. The guestbook app is now running and you can now view its resource components, logs,
events, and assessed health status.

## Use the Web UI to deploy an application

As an alternative method you can also use Argo CD is by the friendly Web interface.

### Web login

Open your browser and  login by visiting the IP/hostname that you exposed
in the first section. You will see the initial Login screen.

Enter as username `admin` and as password the auto-generated value you found
by reading the initial Kubernetes secret.

### Create an application

After logging in, click the **+ New App** button as shown below:

![+ new app button](../assets/new-app.png)

Give your app the name `guestbook`, use the [project](../basics/projects/index.md) `default`, and leave the [sync policy](../syncing/index.md) as `Manual`:

![app information](../assets/app-ui-information.png)

Connect the [https://github.com/argoproj/argocd-example-apps.git](https://github.com/argoproj/argocd-example-apps.git) [repository](../basics/repos/index.md) to Argo CD by setting repository url to the github repo url, leave revision as `HEAD`, and set the path to `guestbook`:

![connect repo](../assets/connect-repo.png)

For **Destination**, set cluster URL to `https://kubernetes.default.svc` (or `in-cluster` for cluster name) and namespace to `default`:

![destination](../assets/destination.png)

After filling out the information above, click **Create** at the top of the UI to create the `guestbook` application:

![destination](../assets/create-app.png)


### Sync the application

The application status is initially in `OutOfSync` state since the application has yet to be
deployed, and no Kubernetes resources have been created. To [sync](../syncing/index.md) (deploy) the application, click the Sync button and accept the default values.

After some time the application will be fully deployed (you will see the "Healthy" and "Synced" status).

![guestbook app](../assets/guestbook-app.png)

You can also click on the application entry and get a graphical overview of all the individual Kubernetes resources it contains.

![view app](../assets/guestbook-tree.png)

On the top right corner of the window you can find different controls that allow you to change the visualization method.


