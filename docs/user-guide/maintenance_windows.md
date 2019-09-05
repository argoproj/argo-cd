# Maintenance Windows

Maintenance windows are configurable windows of time where syncs will blocked. These are defined
by a start time in cron format and a duration. These windows affect the running of a manual
and automated syncs.

In order to use maintenance windows a sync-policy will need to be created. These can be either manual or automated.
```bash
argocd app set --sync-policy manual
```

Maintenance then needs to be enabled.

```bash
argocd app set --maintenance enabled
```

And the schedules need to be created.
```bash
argocd app set --maintenance-windows "0 11 * * *:1h,0 23 * * *:1h"
```

Alternatively, they can be created directly in the application manifest. 
```yaml
spec:
  syncPolicy:
    enabled: true
    windows:
      - schedule: 0 11 * * *
        duration: 1h
      - schedule: 0 23 * * *
        duration: 1h
```

In order to perform a sync during a maintenance window, maintenance will need to be disabled. This can be done
using the cli or the ui.

 ```bash
 argocd app set --maintenance disable
 ```
 
 