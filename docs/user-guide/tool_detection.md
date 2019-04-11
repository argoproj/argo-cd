# Tool Detection

The tool used to build an application is detected as follows:

If a specific tool is explicitly configured, then that tool is selected to create your application's manifests.

If not, then the tool is detected implicitly as follows:

* **Ksonnet** if there are two files, one named `app.yaml` and one named `components/params.libsonnet`.
* **Helm** if there's a file matching `Chart.yaml`. 
* **Kustomize** if there's a `kustomization.yaml`, `kustomization.yml`, or `Kustomization`

Otherwise it is assumed to be a plain **directory** application. 

## References

* [reposerver/repository/repository.go/GetAppSourceType](https://github.com/argoproj/argo-cd/blob/master/reposerver/repository/repository.go#L286)
* [server/repository/repository.go/listAppTypes](https://github.com/argoproj/argo-cd/blob/master/server/repository/repository.go#L97)
