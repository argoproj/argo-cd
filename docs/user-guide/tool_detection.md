# Tool Detection

The tool used to build an application is detected as follows:

If a specific tool is explicitly configured, then that tool is selected to create your application's manifests.

The tool can be explicitly specified in the Application custom resource like this:
```
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  ...
spec:
  ...
  source:
    ...
    
    # Tool -> plain directory
    directory:
      recurse: true
...
```

You also can select the tool in the Application creation wizard in the web user interface. The default is 'Directory'. Press the dropdown button beneath the tool name if you want to choose a different one.


If not, then the tool is detected implicitly as follows:

* **Helm** if there's a file matching `Chart.yaml`. 
* **Kustomize** if there's a `kustomization.yaml`, `kustomization.yml`, or `Kustomization`

Otherwise it is assumed to be a plain **directory** application. 

## Disable built-in tools

Optionally built-in config management tools might be disabled. In order to disable the tool add one of the following
keys to the `argocd-cm` ConfigMap: `kustomize.enable`, `helm.enable` or `jsonnet.enable`. Once the
tool is disabled Argo CD will assume the application target directory contains plain Kubernetes YAML manifests.

## References

* [reposerver/repository/repository.go/GetAppSourceType](https://github.com/argoproj/argo-cd/blob/master/reposerver/repository/repository.go#L286)
* [server/repository/repository.go/listAppTypes](https://github.com/argoproj/argo-cd/blob/master/server/repository/repository.go#L97)
