# How to Build and Run Argo CD Locally 

## Getting Started

### Install Go

- Install version 1.18 or newer (Verify version by running `go version`)

- Get current value of `GOPATH` env:
```console
go env | grep path
```
- Change directory into that path
```
cd <path>
```

### Clone the repo

```
mkdir -p src/github.com/argoproj/ &&
cd src/github.com/argoproj &&
git clone https://github.com/argoproj/argo-cd.git
```

### Install Docker

<https://docs.docker.com/engine/install/>

### Install or Upgrade `kind` (Optional - Should work with any local cluster)

<https://kind.sigs.k8s.io/docs/user/quick-start/>

### Setup Cluster

`kind create cluster`

### Install Argo CD

```console
kubectl create namespace argocd &&
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/master/manifests/install.yaml
```

Setup kubectl config to avoid setting namespace in every kubectl command. 
All following commands on 'Getting Started' assume the namespace is already set.
<!-- Commands like `make start-local` also assume the namespace is already set. -->
```
kubectl config set-context --current --namespace=argocd
```
<!-- Delete this? - Setting kubectl config with `--namespace` helps avoid setting namespace in each following kubectl command  -->

### Install `yarn`

<https://classic.yarnpkg.com/lang/en/docs/install/>

### Install goreman

<https://github.com/mattn/goreman#getting-started>

- Procfile in the root of the ArgoCD repo controls log level for goreman.

### Run Argo CD

```
cd argo-cd
make start-local ARGOCD_GPG_ENABLED=false
```

- Navigate to <localhost:4000> to the ArgoCD UI on browser
- It may take a few minutes for the UI to be responsive

## Making Changes

### Logging In Using CLI

```
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d; echo
```

### UI Changes

Modifying the User-Interface auto-reloads the changes on port 4000  

### API Server Changes

Modifying the API server requires restarting the current `make start-local` session to reflect the changes.

### CLI Changes

Modifying the CLI requires restarting the current `make start-local` session to reflect the changes.

To test the CLI, you need to login:
`argocd login localhost:8080`

---
Congrats on making it to the end of this runbook! 🚀

For more on Argo CD, find us in Slack - <https://slack.cncf.io/> [#argo-contributors](https://cloud-native.slack.com/archives/C020XM04CUW)
 
