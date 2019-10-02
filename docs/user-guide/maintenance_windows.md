# Maintenance Windows

Maintenance windows are configurable windows of time where syncs will blocked. These are defined
by a start time in cron format and a duration along with one or more of either applications, namespaces 
and clusters. Wildcards are supported. These windows affect the running of all syncs, automated and manual.


Schedules can be create using the cli
```bash
argocd proj maintenance add-window projectName --schedule "* * * * *" --duration 1h --applications "*" 
```

Alternatively, they can be created directly in the AppProject manifest. 
```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: default
spec:
  maintenance:
    enabled: true
    windows:
    - schedule: '10 1 * * *'
      duration: 1h
      applications:
      - *-prod
    - schedule: '0 22 * * *'
      duration: 1h
      namespaces:
      - default   
     - schedule: '0 23 * * *'
       duration: 1h
       clusters:
       - in-cluster
       - cluster1  
```

In order to perform a sync during a maintenance window, maintenance will need to be disabled. This can be performed
using the cli, ui or directly in the AppProject manifest

 ```bash
 argocd proj maintenance disable projectName
 ```

Windows can be listed using the cli or viewed in the ui
 
```bash
 proj maintenance list-windows projectName
```
```
 SCHEDULE    DURATION  APPLICATIONS    NAMESPACES  CLUSTERS  STATUS
 10 1 * * *  1h        test            -                     Inactive
 1 10 * * *  2h        app1,app2,app3  -           -         Inactive
 * * * * *   1h                        default     -         Active
```