# Logging

The ArgoCD CLI allows you to view live logs from pods within a Deployment, DaemonSet, or ReplicaSet. ArgoCD _must_ know about the deployment (or other) object in order to access these logs via the command line:

```bash
argocd app logs <appName> -f -n <namespace> -r <deployment name>
```

Other options are as follows:
```
Flags:
  -f, --follow                 Follow log entries. Interrupt to cancel
  -h, --help                   help for logs
  -n, --namespace string       Namespace the pod lives in
  -r, --resource-name string   Name of the resource to get logs for
  -l, --tail-lines int         Number of lines to show (default 100)
```
