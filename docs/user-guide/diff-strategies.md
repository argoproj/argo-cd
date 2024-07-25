# Diff Strategies

Argo CD calculates the diff between the desired state and the live
state in order to define if an Application is out-of-sync. This same
logic is also used in Argo CD UI to display the differences between
live and desired states for all resources belonging to an application.

Argo CD currently has 3 different strategies to calculate diffs:

- **Legacy**: This is the main diff strategy used by default. It
  applies a 3-way diff based on live state, desired state and
  last-applied-configuration (annotation).
- **Structured-Merge Diff**: Strategy automatically applied when
  enabling Server-Side Apply sync option. 
- **Server-Side Diff**: New strategy that invokes a Server-Side Apply
  in dryrun mode in order to generate the predicted live state.

## Structured-Merge Diff
*Current Status: [Beta][1] (Since v2.5.0)*

This is diff strategy is automatically used when Server-Side Apply
sync option is enabled. It uses the [structured-merge-diff][2] library
used by Kubernetes to calculate diffs based on fields ownership. There
are some challenges using this strategy to calculate diffs for CRDs
that define default values. After different issues were identified by
the community, this strategy is being discontinued in favour of
Server-Side Diff.

## Server-Side Diff
*Current Status: [Beta][1] (Since v2.10.0)*

This diff strategy will execute a Server-Side Apply in dryrun mode for
each resource of the application. The response of this operation is then
compared with the live state in order to provide the diff results. The
diff results are cached and new Server-Side Apply requests to Kube API
are only triggered when:

- An Application refresh or hard-refresh is requested.
- There is a new revision in the repo which the Argo CD Application is
  targeting.
- The Argo CD Application spec changed.

One advantage of Server-Side Diff is that Kubernetes Admission
Controllers will participate in the diff calculation. If for example
a validation webhook identifies a resource to be invalid, that will be
informed to Argo CD during the diff stage rather than during the sync 
stage.

### Enabling it

Server-Side Diff can be enabled at the Argo CD Controller level or per
Application.

**Enabling Server-Side Diff for all Applications**

Add the following entry in the argocd-cmd-params-cm configmap:

```
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cmd-params-cm
data:
  controller.diff.server.side: "true"
...
```

Note: It is necessary to restart the `argocd-application-controller`
after applying this configuration.

**Enabling Server-Side Diff for one application**

Add the following annotation in the Argo CD Application resource:

```
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd.argoproj.io/compare-options: ServerSideDiff=true
...
```

**Disabling Server-Side Diff for one application**

If Server-Side Diff is enabled globally in your Argo CD instance, it
is possible to disable it at the application level. In order to do so,
add the following annotation in the Application resource:

```
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd.argoproj.io/compare-options: ServerSideDiff=false
...
```

*Note: Please report any issues that forced you to disable the
Server-Side Diff feature*

### Mutation Webhooks

Server-Side Diff does not include changes made by mutation webhooks by
default. If you want to include mutation webhooks in Argo CD diffs add
the following annotation in the Argo CD Application resource:

```
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd.argoproj.io/compare-options: IncludeMutationWebhook=true
...
```

Note: This annotation is only effective when Server-Side Diff is
enabled. To enable both options for a given application add the
following annotation in the Argo CD Application resource:

```
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd.argoproj.io/compare-options: ServerSideDiff=true,IncludeMutationWebhook=true
...
```

[1]: https://github.com/argoproj/argoproj/blob/main/community/feature-status.md#beta
[2]: https://github.com/kubernetes-sigs/structured-merge-diff
