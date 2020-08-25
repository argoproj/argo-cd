# Debugging a Remote ArgoCD Environment

In this guide, we will describe how to debug a remote ArgoCD environment with [Telepresence](https://telepresence.io/).

Telepresence allows you to connect & debug a service deployed in a remote environment and to "cherry-pick" one service to run locally, staying connected to the remote cluster. This will:

* Reduce resource footprint on the local machine
* Decrease the feedback loop time
* Result in more confidence about the delivered code.

To read more about it, refer to the official documentation at [telepresence.io](https://telepresence.io/) or [Medium](https://medium.com/containers-101/development-environment-using-telepresence-634bd7210c26).

## Install ArgoCD
First of all, install ArgoCD on your cluster
```shell
kubectl create ns argocd
curl -sSfL https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml | kubectl apply -n argocd -f -
```

## Connect
Connect to one of the services, for example, to debug the main ArgoCD server run:
```shell
telepresence --swap-deployment argocd-server --namespace argocd --env-file .envrc.remote --expose 8080:8080 --expose 8083:8083 --run bash
```
* `--swap-deployment` changes the argocd-server deployment
* `--expose` forwards traffic of remote ports 8080 and 8083 to the same ports locally
* `--env-file` writes all the environment variables of the remote pod into a local file, the variables are also set on the subprocess of the `--run` command
* `--run` defines which command to run once a connection is established, use `bash`, `zsh` or others


## Debug
Once a connection is established, use your favorite tools to start the server locally.

### Terminal
* Compile `make server`
* Run `./dist/argocd-server`

### VSCode
In VSCode use the integrated terminal to run the Telepresence command to connect. Then, to run argocd-server service use the following configuration.
Make sure to run `packr` before starting the debugging session to generate the assets. 
Update the configuration file to point to kubeconfig file: `KUBECONFIG=` (required)
```json
        {
            "name": "Launch",
            "type": "go",
            "request": "launch",
            "mode": "auto",
            "program": "${workspaceFolder}/cmd/argocd-server",
            "envFile": [
                "${workspaceFolder}/.envrc.remote",
            ],
            "env": {
                "CGO_ENABLED": "0",
                "KUBECONFIG": "/path/to/kube/config"
            }
        }
```