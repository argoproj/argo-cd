# Sync Windows

Sync windows are configurable windows of time where syncs will either be blocked or allowed. These are defined
by a kind, which can be either `allow` or `deny`, a `schedule` in cron format and a duration along with rules that 
can accept `application`, `namespace`, `cluster` or `label`. These windows affect the running of both manual and automated 
syncs but allow an override for manual syncs which is useful if you are only interestedin preventing automated syncs or 
if you need to temporarily override a window to perform a sync.

The windows work in the following way. If there are no windows matching an application then all syncs are allowed. If there
are any `allow` windows matching an application then syncs will only be allowed when there ia an active `allow` windows. If there
are any `deny` windows matching an application then all syncs will be denied when the `deny` windows are active. If there is an
active matching `allow` and an active matching `deny` then syncs will be denied as `deny` windows override `allow` windows. The
UI and the CLI will both display the state of the sync windows. The UI has a panel which will display different colours depending
on the state. The colours are as follows. `Red: sync denied`, `Orange: manual allowed` and `Green: sync allowed`.

Rules follow the same pattern as [kubernetes label selectors](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors).
The rule kind can be either `application`, `namespace`, `cluster` or `label`. The operators are `in`, `notIn` and `exists` but
`exists` can only be used with `label`. Wildcards are supported for all kinds except `label`. When creating rule conditions using the cli, 
if the kind is not one of `application`, `namespace` or `cluster` then it is assumed to be a `label key`. The following are examples 
of rules.

- `stateful in database` would match any applications that have the label stateful with database as the value
- `application in frontend,database` would match any applications called frontend or database
- `stateful exists` would match any applications that have the label stateful

Rules can also contain multiple conditions.

- `application in web-*` AND `namespace in default` would match applications called dev-* in the default namespace 

To display the sync state using the CLI:

```bash
argocd app get APP
```

Which will return the sync state and any matching windows.

```bash
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
    --condition "namespace in default"
```

Alternatively, they can be created directly in the `AppProject` manifest:
 
```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: default
spec:
  syncWindows:
  - duration: 1h
    kind: deny
    rules:
    - conditions:
      - kind: application
        operator: in
        values:
        - web
        - db
    - conditions:
      - kind: namespace
        operator: in
        values:
        - default
    - conditions:
      - kind: cluster
        operator: notIn
        values:
        - cluster1
    - conditions:
      - kind: application
        operator: in
        values:
        - web-*
      - kind: namespace
        operator: notIn
        values:
        - production
    schedule: '* * * * *'
```

In order to perform a sync when syncs are being prevented by a window, you can configure the window to allow manual syncs
using the CLI, UI or directly in the `AppProject` manifest:

```bash
argocd proj windows enable-manual-sync PROJECT WINDOW_ID
```

To disable usng the CLI:

```bash
argocd proj windows disable-manual-sync PROJECT WINDOW_ID
```

Windows can be listed using the CLI or viewed in the UI:

```bash
argocd proj windows list PROJECT
```

```
ID  STATUS    KIND   SCHEDULE   DURATION  RULES  MANUALSYNC
0   Active    deny   * * * * *  1h        4      Disabled
1   Inactive  allow  0 0 * * *  1h        1      Enabled
2   Inactive  deny   0 0 * * *  1h        1      Disabled
```

All fields of a window can be updated using either the CLI or UI. `kind`, `schedule` or `duration` can be updated in the 
CLI using:

```bash
argocd proj windows update PROJECT WINDOW_ID --kind allow --duration 2h
```

Rules changes in the CLI are handled by the rules sub-command. They can be listed using:

```bash
argocd-local proj  windows rules list PROJECT WINDOW_ID
```

```
ID  RULE
0   application in (web, db)
1   namespace in (default)
2   cluster notIn (cluster1)
3   application in (web-*) AND namespace notIn (production)
```

Added using:

```bash
argocd-local proj  windows rules add PROJECT WINDOW_ID --condition "application in web-*,frontend"
```

Deleted using:

```bash
argocd-local proj  windows rules delete PROJECT WINDOW_ID RULE_ID
```

Conditions can be added to existing rules using:

```bash
argocd-local proj  windows rules add-condition default WINDOW_ID RULE_ID --condition "namespace in prod"
```

And Delete using:

```bash
argocd-local proj  windows rules add-condition default PROJECT WINDOW_ID RULE_ID CONDITION_ID
```
