# Tilt Development

> Tilt provides a real-time web UI that offers better visibility into logs, health status, and dependencies, making debugging easier compared to relying solely on terminal outputs. With a single `tilt up` command, developers can spin up all required services without managing multiple processes manually, simplifying the local development workflow. Tilt also integrates seamlessly with Docker and Kubernetes, allowing for efficient container-based development. Unlike goreman, which lacks dynamic config reloading, Tilt can detect and apply changes to Kubernetes YAML and Helm charts without full restarts, making it more efficient for iterative development.

### Prerequisites
* kubernetes environment (kind, minikube, k3d, etc.)
* tilt (`brew install tilt`)
* kustomize
* kubectl

### Running
1. Spin up environment by running `tilt up` in the root directory of the repo
    * Resources will be deployed into the `argocd` namespace in the cluster that your `kubeconfig` is currently pointed to. 

2. Spin down and remove deployment manifests: `tilt down`

### Debugging

#### VS Code
Add a `.vscode/launch.json` file with these configurations to support attaching to running pods corresponding to the service. 

NOTE: VS Code supports attaching to multiple sessions at the same time

```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Connect to argocd-server",
            "type": "go",
            "request": "attach",
            "mode": "remote",
            "remotePath": "${workspaceFolder}",
            "port": 9345,
            "host": "127.0.0.1"
        },
        {
            "name": "Connect to argocd-repo-server",
            "type": "go",
            "request": "attach",
            "mode": "remote",
            "remotePath": "${workspaceFolder}",
            "port": 9346,
            "host": "127.0.0.1"
        },
        {
            "name": "Connect to argocd-applicationset-controller",
            "type": "go",
            "request": "attach",
            "mode": "remote",
            "remotePath": "${workspaceFolder}",
            "port": 9347,
            "host": "127.0.0.1"
        },
        {
            "name": "Connect to argocd-application-controller",
            "type": "go",
            "request": "attach",
            "mode": "remote",
            "remotePath": "${workspaceFolder}",
            "port": 9348,
            "host": "127.0.0.1"
        },
        {
            "name": "Connect to argocd-notifications-controller",
            "type": "go",
            "request": "attach",
            "mode": "remote",
            "remotePath": "${workspaceFolder}",
            "port": 9349,
            "host": "127.0.0.1"
        },
        {
            "name": "Connect to argocd-commit-server",
            "type": "go",
            "request": "attach",
            "mode": "remote",
            "remotePath": "${workspaceFolder}",
            "port": 9350,
            "host": "127.0.0.1"
        }
    ]
}
```