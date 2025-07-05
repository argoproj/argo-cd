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
kubectl config set-context --current --namespace argocd
telepresence helm install --set agent.securityContext={} # Installs telepresence into your cluster
telepresence connect # Starts the connection to your cluster (bound to the current namespace)
telepresence intercept argocd-server --port 8080:http --env-file .envrc.remote # Starts the interception
```
* `--port` forwards traffic of remote port http to 8080 locally (use `--port 8080:https` if argocd-server terminates TLS)
* `--env-file` writes all the environment variables of the remote pod into a local file, the variables are also set on the subprocess of the `--run` command

With this, any traffic that hits your argocd-server service in the cluster (e.g. through a LB / ingress) will be forwarded to your laptop on port 8080. So that you can now start argocd-server locally to debug or test new code. If you launch argocd-server using the environment variables in `.envrc.remote`, it is able to fetch all the configmaps, secrets and so on from the cluster and transparently connect to the other microservices so that no further configuration should be necessary, and it behaves exactly the same as in the cluster.

List current status of Telepresence using:
```shell
telepresence status
```

Stop the intercept using:
```shell
telepresence leave argocd-server-argocd
```

And uninstall telepresence from your cluster:
```shell
telepresence helm uninstall
```

See [this quickstart](https://www.telepresence.io/docs/latest/quick-start/) for more information on how to intercept services using Telepresence.

### Connect (telepresence v1)
Use the following command instead:
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
In VSCode use the following launch configuration to run argocd-server:

```json
        {
            "name": "Launch argocd-server",
            "type": "go",
            "request": "launch",
            "mode": "auto",
            "program": "${workspaceFolder}/cmd/main.go",
            "envFile": [
                "${workspaceFolder}/.envrc.remote",
            ],
            "env": {
                "ARGOCD_BINARY_NAME": "argocd-server",
                "CGO_ENABLED": "0",
                "KUBECONFIG": "/path/to/kube/config"
            }
        }
```
### GoLand
In GoLand use the following launch configuration to run argocd-api-server:

1. Open **Run `>` Edit configurations...**
2. Click **+ icon** and select **Go build**
3. Add the following configuration:
```text
        Name: Run argocd-server
        Run kind: File
        File: <path-to-your-argo-cd-repo>/cmd/main.go
        Working directory: `path to your Argo CD repo root``
```
4. Set the environment variables:
    Refer to the [Procfile](https://github.com/argoproj/argo-cd/blob/master/Procfile) to configure the necessary environment variables for the component you are debugging.

    For example, for `api-server`(as given in Procfile)
```text
        api-server: [ "$BIN_MODE" = 'true' ] && COMMAND=./dist/argocd || COMMAND='go run ./cmd/main.go' && sh -c "GOCOVERDIR=${ARGOCD_COVERAGE_DIR:-/tmp/coverage/api-server} FORCE_LOG_COLORS=1 ARGOCD_FAKE_IN_CLUSTER=true ARGOCD_TLS_DATA_PATH=${ARGOCD_TLS_DATA_PATH:-/tmp/argocd-local/tls} ARGOCD_SSH_DATA_PATH=${ARGOCD_SSH_DATA_PATH:-/tmp/argocd-local/ssh} ARGOCD_BINARY_NAME=argocd-server 
        $COMMAND 
        --loglevel debug --redis localhost:${ARGOCD_E2E_REDIS_PORT:-6379} --disable-auth=${ARGOCD_E2E_DISABLE_AUTH:-'true'} --insecure --dex-server http://localhost:${ARGOCD_E2E_DEX_PORT:-5556} --repo-server localhost:${ARGOCD_E2E_REPOSERVER_PORT:-8081} --port ${ARGOCD_E2E_APISERVER_PORT:-8080} --otlp-address=${ARGOCD_OTLP_ADDRESS} --application-namespaces=${ARGOCD_APPLICATION_NAMESPACES:-''} --hydrator-enabled=${ARGOCD_HYDRATOR_ENABLED:='false'}"
```
```text
        ARGOCD_BINARY_NAME: argocd-server
        ARGOCD_FAKE_IN_CLUSTER=true
        FORCE_LOG_COLORS=1
        KUBECONFIG: (path to your .kubeconfig)
```

**Note**:When debugging Argo CD locally, you must run the api-server alongside your component (either by debugging it separately in GoLand or by including it in `ARGOCD_START`).
The local UI will fail to load data unless the api-server is running, as it handles the backend communication for the UI.

For example,
```text
make start-local ARGOCD_START="redis repo-server"
```

If you are using `.envrc.remote` from Telepresence you can use [Plugin](https://plugins.jetbrains.com/plugin/7861-envfile#) to load variables automatically.

5. Click Apply and Run or Debug.