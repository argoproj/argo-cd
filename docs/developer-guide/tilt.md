# Tilt Development

[Tilt](https://tilt.dev/) provides a real-time web UI that offers better visibility into logs, health status, and dependencies, making debugging easier compared to relying solely on terminal outputs. With a single `tilt up` command, developers can spin up all required services without managing multiple processes manually, simplifying the local development workflow. Tilt also integrates seamlessly with Docker and Kubernetes, allowing for efficient container-based development. Unlike goreman, which lacks dynamic config reloading, Tilt can detect and apply changes to Kubernetes YAML and Helm charts without full restarts, making it more efficient for iterative development.

### Prerequisites
* kubernetes environment (kind, minikube, k3d, etc.)
* tilt (`brew install tilt`)
* kustomize
* kubectl

### Running
1. Spin up environment by running `tilt up` in the root directory of the repo
    * Resources will be deployed into the `argocd` namespace in the cluster that your `kubeconfig` is currently pointed to. 

2. Use `ctrl+c` to close tilt which stops watching files for changes and closes port-forwards. Everything deployed to the local cluster will be left in tact and continue to run. Run `tilt up` again to start up another session and pick up where you left off.   

### Cleanup
To remove all deployed resources in your local cluster including CRDs, run `tilt down` from the root of the repo. 

### Port Forwarding
Port forwarding is automatically setup from the cluster to localhost host for the folling ports:

| Deployment | API | Metrics | Webhook | Debug |
|------------|-----|---------|---------|-------|
| argocd-server | 8080 | 8083 | | 9345 |
| argocd-repo-server | 8081 | 8084 | | 9346 |
| argocd-redis | 6379 | | | |
| argocd-applicationset-controller | | 8085 | 7000 | 9347 |
| argocd-application-controller | | 8086 | | 9348 |
| argocd-notifications-controller | | 8087 | | 9349 |
| argocd-commit-server | 8089 | 8088 | | 9350 |

### Debugging ArgoCD
Each deployed pod running ArgoCD components uses delve to expose a debug port. Tilt is configured to forward each of those ports locally to `localhost`. IDEs can attach to the corresponding application to set break points and debug code running inside the cluster. 

| Deployment | Debug Host Port |
|-----------|------------|
| argocd-server | localhost:9345 |
| argocd-repo-server | localhost:9346 |
| argocd-applicationset-controller | localhost:9347 |
| argocd-application-controller | localhost:9348 |
| argocd-notifications-controller | localhost:9349 |
| argocd-commit-server | localhost:9350 |


#### VS Code
Add a `.vscode/launch.json` file with these configurations to support attaching to running pods corresponding to the service. 


```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Connect to server",
            "type": "go",
            "request": "attach",
            "mode": "remote",
            "remotePath": "${workspaceFolder}",
            "port": 9345,
            "host": "127.0.0.1"
        },
        {
            "name": "Connect to repo-server",
            "type": "go",
            "request": "attach",
            "mode": "remote",
            "remotePath": "${workspaceFolder}",
            "port": 9346,
            "host": "127.0.0.1"
        },
        {
            "name": "Connect to applicationset-controller",
            "type": "go",
            "request": "attach",
            "mode": "remote",
            "remotePath": "${workspaceFolder}",
            "port": 9347,
            "host": "127.0.0.1"
        },
        {
            "name": "Connect to application-controller",
            "type": "go",
            "request": "attach",
            "mode": "remote",
            "remotePath": "${workspaceFolder}",
            "port": 9348,
            "host": "127.0.0.1"
        },
        {
            "name": "Connect to notifications-controller",
            "type": "go",
            "request": "attach",
            "mode": "remote",
            "remotePath": "${workspaceFolder}",
            "port": 9349,
            "host": "127.0.0.1"
        },
        {
            "name": "Connect to commit-server",
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

#### Goland
Add a `.run/remote-debugging.run.xml` file with these configurations to support attaching to running pods corresponding to the service. 

```xml
<component name="ProjectRunConfigurationManager">
    <configuration default="false" name="Connect to server" type="GoRemoteDebugConfigurationType" factoryName="Go Remote" focusToolWindowBeforeRun="true" port="9345">
        <option name="disconnectOption" value="LEAVE" />
        <disconnect value="LEAVE" />
        <method v="2" />
    </configuration>
    <configuration default="false" name="Connect to repo-server" type="GoRemoteDebugConfigurationType" factoryName="Go Remote" focusToolWindowBeforeRun="true" port="9346">
        <option name="disconnectOption" value="LEAVE" />
        <disconnect value="LEAVE" />
        <method v="2" />
    </configuration>
    <configuration default="false" name="Connect to applicationset-controller" type="GoRemoteDebugConfigurationType" factoryName="Go Remote" focusToolWindowBeforeRun="true" port="9347">
        <option name="disconnectOption" value="LEAVE" />
        <disconnect value="LEAVE" />
        <method v="2" />
    </configuration>
    <configuration default="false" name="Connect to application-controller" type="GoRemoteDebugConfigurationType" factoryName="Go Remote" focusToolWindowBeforeRun="true" port="9348">
        <option name="disconnectOption" value="LEAVE" />
        <disconnect value="LEAVE" />
        <method v="2" />
    </configuration>
    <configuration default="false" name="Connect to notifications-controller" type="GoRemoteDebugConfigurationType" factoryName="Go Remote" focusToolWindowBeforeRun="true" port="9349">
        <option name="disconnectOption" value="LEAVE" />
        <disconnect value="LEAVE" />
        <method v="2" />
    </configuration>
    <configuration default="false" name="Connect to commit-server" type="GoRemoteDebugConfigurationType" factoryName="Go Remote" focusToolWindowBeforeRun="true" port="9350">
        <option name="disconnectOption" value="LEAVE" />
        <disconnect value="LEAVE" />
        <method v="2" />
    </configuration>
</component>
```