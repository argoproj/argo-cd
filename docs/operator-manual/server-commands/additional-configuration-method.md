## Additional configuration methods

Additional configuration methods for configuring commands `argocd-server`, `argocd-repo-server` and `argocd-application-controller`.


### Synopsis

The commands can also be configured by setting the respective flag of the available options in `argocd-cmd-params-cm.yaml`. Each component has a specific prefix associated with it.

```
argocd-server                 --> server
argocd-repo-server            --> reposerver
argocd-application-controller --> controller
```

The flags that do not have a prefix are shared across multiple components. One such flag is `repo.server`
The list of flags that are available can be found in [argocd-cmd-params-cm.yaml](../argocd-cmd-params-cm.yaml) 


### Example

To set `logformat` of `argocd-application-controller`, add below entry to the config map `argocd-cmd-params-cm.yaml`.

```
data:
    controller.log.format: "text"
```

