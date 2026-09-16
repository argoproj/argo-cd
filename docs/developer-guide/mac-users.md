# MacOS users
Below are known issues specific to macOS

## Port 5000

You may get an error listening on port 5000 on macOS:

```text
docker: Error response from daemon: Ports are not available: exposing port TCP 0.0.0.0:5000 -> 0.0.0.0:0: listen tcp 0.0.0.0:5000: bind: address already in use.
```

This shouldn't happen anymore: `make start-e2e-local` and `make test-e2e-local` now default the Helm/OCI registry used by e2e tests to port `5050` instead of `5000`, to avoid this exact conflict. If you still hit a port conflict (e.g. something else is using `5050`), override it with `ARGOCD_E2E_HELM_REGISTRY_PORT` (see [Configuration of E2E Tests execution](test-e2e.md#configuration-of-e2e-tests-execution)), or disable "AirPlay Receiver" in macOS System Preferences.

## Firewall dialogs
If you get firewall dialogs, you can click "Deny", since no access from outside your computer is typically desired.

## Exec format error for virtualized toolchain make targets
If you get `/go/src/github.com/argoproj/argo-cd/dist/mockery: cannot execute binary file: Exec format error`, this typically means that you ran a virtualized `make` target after you ran the a local `make` target.   
To fix this and continue with the virtualized toolchain, delete the contents of `argo-cd/dist` folder.   
If later on you wish to run `make` targets of the local toolchain again, run `make install-tools-local` to re-populate the contents of the `argo-cd/dist` folder.
