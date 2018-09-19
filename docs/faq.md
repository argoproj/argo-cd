# FAQ

## Why is my application still `OutOfSync` immediately after a successful Sync?

It is possible for an application to still be `OutOfSync` even immediately after a successful Sync
operation. Some reasons for this might be:
* There may be problems in manifests themselves, which may contain extra/unknown fields from the 
  actual K8s spec. These extra fields would get dropped when querying Kubernetes for the live state,
  resulting in an `OutOfSync` status indicating a missing field was detected.
* The sync was performed (with pruning disabled), and there are resources which need to be deleted.
* A mutating webhook altered the manifest after it was submitted to Kubernetes

To debug `OutOfSync` issues, run the `app diff` command to see the differences between git and live:
```
argocd app diff APPNAME
```
