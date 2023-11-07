# Sync Windows

Sync windows are configurable windows of time where syncs will either be blocked or allowed. These are defined
by a kind, which can be either `allow` or `deny`, a `schedule` in cron format and a duration along with one or 
more of either `applications`, `namespaces` and `clusters`. Wildcards are supported. These windows affect the running 
of both manual and automated syncs but allow an override for manual syncs which is useful if you are only interested
in preventing automated syncs or if you need to temporarily override a window to perform a sync.

The windows work in the following way. If there are no windows matching an application then all syncs are allowed. If there
are any `allow` windows matching an application then syncs will only be allowed when there is an active `allow` window. If there
are any `deny` windows matching an application then all syncs will be denied when the `deny` windows are active. If there is an
active matching `allow` and an active matching `deny` then syncs will be denied as `deny` windows override `allow` windows. The
UI and the CLI will both display the state of the sync windows. The UI has a panel which will display different colours depending
on the state. The colours are as follows. `Red: sync denied`, `Orange: manual allowed` and `Green: sync allowed`.

To display the sync state using the CLI:

```bash
argocd app get APP
```

Which will return the sync state and any matching windows.

```
Name:               guestbook
Project:            default
Server:             in-cluster
Namespace:          default
URL:                http://localhost:8080/applications/guestbook
Repo:               https://github.com/argoproj/argocd-example-apps.git
Target:
Path:               guestbook
SyncWindow:         Sync Denied
Assigned Windows:   deny:0 2 * * *:1h,allow:0 2 3 3 3:1h
Sync Policy:        Automated
Sync Status:        Synced to  (5c2d89b)
Health Status:      Healthy
```

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
    timeZone: "Europe/Amsterdam"
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
