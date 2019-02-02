# FAQ

## Why is my application still `OutOfSync` immediately after a successful Sync?

It is possible for an application to still be `OutOfSync` even immediately after a successful Sync
operation. Some reasons for this might be:
* There is a bug in the manifest itself, where it contains extra/unknown fields from the 
  actual K8s spec. These extra fields would get dropped when querying Kubernetes for the live state,
  resulting in an `OutOfSync` status indicating a missing field was detected.
* The sync was performed (with pruning disabled), and there are resources which need to be deleted.
* A controller or [mutating webhook](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#mutatingadmissionwebhook)
  is altering the object after it was submitted to Kubernetes in a manner which contradicts git.
* A helm chart is using a template function such as [`randAlphaNum`](https://github.com/helm/charts/blob/master/stable/redis/templates/secret.yaml#L16),
  which generates different data every time `helm template` is invoked.

To debug `OutOfSync` issues, run the `app diff` command to see the differences between git and live:
```
argocd app diff APPNAME
```
