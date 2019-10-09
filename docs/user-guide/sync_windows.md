# Sync Windows

Sync windows are configurable windows of time where syncs will either be blocked or allowed. These are defined
by a kind, which can be either `allow` or `deny`, a start time in cron format and a duration along with one or 
more of either applications, namespaces and clusters. Wildcards are supported. These windows affect the running 
of both manual and automated syncs but allow an override for manual syncs which is useful if you are only interested
in preventing automated syncs or if you need to temporarily override a window to perform a sync.

If an application has matching allow windows then it will only be able to sync during those windows. If is has 
has matching deny windows then they will override any matching allows if the windows are active at the same time.

Windows can be created using the CLI:

```bash
argocd proj windows add PROJECT \
    --kind allow \
    --schedule "0 22 * * *" \
    --duration 1h \
    --applications "*"
```

Alternatively, they can be created directly in the `AppProject` manifest:
 
```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: default
spec:
  syncWindows:
  - kind: allow
    schedule: '10 1 * * *'
    duration: 1h
    applications:
    - '*-prod'
    manualSync: true
  - kind: deny
    schedule: '0 22 * * *'
    duration: 1h
    namespaces:
    - default
   - kind: allow
     schedule: '0 23 * * *'
     duration: 1h
     clusters:
     - in-cluster
     - cluster1
```

In order to perform a sync when syncs are being prevented by a window, you can configure the window to allow manual syncs
using the CLI, UI or directly in the `AppProject` manifest:

```bash
argocd proj windows enable-manual-sync PROJECT ID
```

To disable

```bash
argocd proj windows disable-manual-sync PROJECT ID
```

Windows can be listed using the CLI or viewed in the UI:

```bash
argocd proj windows list PROJECT
```

```bash
ID  STATUS    KIND   SCHEDULE    DURATION  APPLICATIONS  NAMESPACES  CLUSTERS  MANUALSYNC
0   Active    allow  * * * * *   1h        -             -           prod1     Disabled
1   Inactive  deny   * * * * 1   3h        -             default     -         Disabled
2   Inactive  allow  1 2 * * *   1h        prod-*        -           -         Enabled
3   Active    deny   * * * * *   1h        -             default     -         Disabled
```

All fields of a window can be updated using either the CLI or UI. The `applications`, `namespaces` and `clusters` fields
require the update to contain all of the required values. For example if updating the `namespaces` field and it already
contains default and kube-system then the new value would have to include those in the list. 

```bash
argocd proj windows update PROJECT ID --namespaces default,kube-system,prod1
```