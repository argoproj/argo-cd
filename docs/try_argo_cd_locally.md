# Try Argo CD Locally

> [!TIP]
> This guide assumes you have a grounding in the tools that Argo CD is based on. Please read [understanding the basics](understand_the_basics.md) to learn about these tools.


Follow these steps to install `Kind` for local development and set it up with Argo CD.

!!! note "Windows users"
    When running Argo CD locally on Windows:

    - Ensure Docker Desktop is running with WSL 2 backend enabled.
    - Ensure WSL 2 is enabled and set as the default version.

    Local Kubernetes clusters such as kind, Minikube, or Docker Desktop require WSL 2 to run Linux containers.


To run an Argo CD development environment review the [developer guide for running locally](./developer-guide/running-locally.md).

## Install Kind

Install Kind Following Instructions [here](https://kind.sigs.k8s.io/docs/user/quick-start#installation).

##  Create a Kind Cluster
Once Kind is installed, create a new Kubernetes cluster with:
```bash
kind create cluster --name argocd-cluster
```
This will create a local Kubernetes cluster named `argocd-cluster`.

## Set Up kubectl to Use the Kind Cluster
After creating the cluster, set `kubectl` to use your new `kind` cluster:
```bash
kubectl cluster-info --context kind-argocd-cluster
```
This command verifies that `kubectl` is pointed to the right cluster.

## Install ArgoCD on the Cluster
You can now install Argo CD on your `kind` cluster. First, apply the Argo CD manifest to create the necessary resources:
```bash
kubectl create namespace argocd
kubectl apply -n argocd --server-side --force-conflicts -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
```

> [!NOTE]
> The `--server-side --force-conflicts` flags are required because some Argo CD CRDs exceed the size limit for client-side apply. See the [getting started guide](getting_started.md) for more details.

## Expose ArgoCD API Server
By default, Argo CD's API server is not exposed outside the cluster. You need to expose it to access the UI locally. For development purposes, you can use Kubectl 'port-forward'.
```bash
kubectl port-forward svc/argocd-server -n argocd 8080:443
```
This will forward port 8080 on your local machine to the ArgoCD API server’s port 443 inside the Kubernetes cluster.

## Access ArgoCD UI
Now, you can open your browser and navigate to http://localhost:8080 to access the ArgoCD UI.

### Log in to ArgoCD
To log in to the ArgoCD UI, you'll need the default admin password. You can retrieve it from the Kubernetes cluster:
```bash
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d
```
!!! note "PowerShell users"
    The `base64 -d` command is not available in Windows PowerShell.

    To decode the password manually, use:

    ```powershell
    [System.Text.Encoding]::UTF8.GetString(
      [System.Convert]::FromBase64String("<BASE64_VALUE>")
    )
    ```

Use the admin username and the retrieved password to log in.

You can now move on to step #2 in the [Getting Started Guide](getting_started.md).
