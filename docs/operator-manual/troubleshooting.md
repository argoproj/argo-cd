# Troubleshooting Tools

The document describes how to use `argocd admin` subcommands to simplify Argo CD settings customizations and troubleshot
connectivity issues.

## Settings

Argo CD provides multiple ways to customize system behavior and has a lot of settings. It might be dangerous to modify
settings on Argo CD used in production by multiple users. Before applying settings you can use `argocd admin` subcommands to
make sure that settings are valid and Argo CD is working as expected.

The `argocd admin settings validate` command performs basic settings validation and print short summary
of each settings group.

**Diffing Customization**

[Diffing customization](../user-guide/diffing.md) allows excluding some resource fields from diffing process.
The diffing customizations are configured in `resource.customizations` field of `argocd-cm` ConfigMap.

The following `argocd admin` command prints information about fields excluded from diffing in the specified ConfigMap.

```bash
argocd admin settings resource-overrides ignore-differences ./deploy.yaml --argocd-cm-path ./argocd-cm.yaml
```

**Health Assessment**

Argo CD provides built-in [health assessment](./health.md) for several Kubernetes resources which can be further
customized by writing your own health checks in [Lua](https://www.lua.org/).
The health checks are configured in the `resource.customizations` field of `argocd-cm` ConfigMap.

The following `argocd admin` command assess resource health using Lua script configured in the specified ConfigMap.

```bash
argocd admin settings resource-overrides health ./deploy.yaml --argocd-cm-path ./argocd-cm.yaml
```

**Resource Actions**

Resource actions allows configuring named Lua script which performs resource modification.

The following `argocd admin` command executes action using Lua script configured in the specified ConfigMap and prints
applied modifications.

```bash
argocd admin settings resource-overrides run-action /tmp/deploy.yaml restart --argocd-cm-path /private/tmp/argocd-cm.yaml
```

The following `argocd admin` command lists actions available for a given resource using Lua script configured in the specified ConfigMap.

```bash
argocd admin settings resource-overrides list-actions /tmp/deploy.yaml --argocd-cm-path /private/tmp/argocd-cm.yaml
```

## Cluster credentials

The `argocd admin cluster kubeconfig` is useful if you manually created Secret with cluster credentials and trying need to
troubleshoot connectivity issues. In this case, it is suggested to use the following steps:

1 SSH into [argocd-application-controller] pod.

```
kubectl exec -n argocd -it \
  $(kubectl get pods -n argocd -l app.kubernetes.io/name=argocd-application-controller -o jsonpath='{.items[0].metadata.name}') bash
```

2 Use `argocd admin cluster kubeconfig` command to export kubeconfig file from the configured Secret:

```
argocd admin cluster kubeconfig https://<api-server-url> /tmp/kubeconfig --namespace argocd
```

3 Use `kubectl` to get more details about connection issues, fix them and apply changes back to secret:

```
export KUBECONFIG=/tmp/kubeconfig
kubectl get pods -v 9
```

## Kubernetes API Client Performance Tuning

Argo CD components communicate with Kubernetes API servers to manage applications and resources. If you experience slow performance, timeouts, or rate limiting when working with large clusters or many applications, you may need to tune the Kubernetes API client settings.

### Symptoms

- Slow application sync or refresh operations
- Timeout errors when connecting to Kubernetes API
- "context deadline exceeded" errors in logs
- Rate limiting errors (HTTP 429 responses)
- High memory or CPU usage in Argo CD components

### Configuration Options

The following environment variables control how Argo CD components interact with Kubernetes API servers. These can be set either as environment variables or via the `argocd-cmd-params-cm` ConfigMap.

| ConfigMap Key | Environment Variable | Default | Description |
|--------------|---------------------|---------|-------------|
| `k8s.client.qps` | `ARGOCD_K8S_CLIENT_QPS` | `50` | Maximum queries per second (QPS) to the K8s API server |
| `k8s.client.burst` | `ARGOCD_K8S_CLIENT_BURST` | `100` | Maximum burst for throttle (should be >= QPS) |
| `k8s.client.max.idle.connections` | `ARGOCD_K8S_CLIENT_MAX_IDLE_CONNECTIONS` | `500` | Maximum idle connections in the HTTP transport pool |
| `k8s.tcp.timeout` | `ARGOCD_K8S_TCP_TIMEOUT` | `30s` | TCP connection timeout for K8s API requests |
| `k8s.tcp.keepalive` | `ARGOCD_K8S_TCP_KEEPALIVE` | `30s` | TCP keep-alive probe interval |
| `k8s.tls.handshake.timeout` | `ARGOCD_K8S_TLS_HANDSHAKE_TIMEOUT` | `10s` | TLS handshake timeout |
| `k8s.tcp.idle.timeout` | `ARGOCD_K8S_TCP_IDLE_TIMEOUT` | `5m` | Idle connection timeout |

### Tuning Recommendations

**For large clusters (1000+ resources per application):**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cmd-params-cm
data:
  k8s.client.qps: "100"
  k8s.client.burst: "200"
```

**For slow or high-latency API servers:**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cmd-params-cm
data:
  k8s.tcp.timeout: "60s"
  k8s.tls.handshake.timeout: "20s"
```

**For many concurrent operations:**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cmd-params-cm
data:
  k8s.client.max.idle.connections: "1000"
  k8s.tcp.keepalive: "60s"
  k8s.tcp.idle.timeout: "10m"
```

### Monitoring

After adjusting these settings, monitor the following metrics:

- Application sync time
- API server request latency
- Error rates in component logs
- Memory usage (increasing connection pools will use more memory)

See the [argocd-cmd-params-cm documentation](argocd-cmd-params-cm-yaml.md) for complete configuration options.