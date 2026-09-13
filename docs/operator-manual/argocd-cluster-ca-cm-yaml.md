# argocd-cluster-ca-cm.yaml example

An example of an `argocd-cluster-ca-cm.yaml` file providing a default CA bundle used to verify the Kubernetes API
server of managed clusters that do not define their own `tlsClientConfig.caData`. See
[Default CA bundle for cluster connections](declarative-setup.md#default-ca-bundle-for-cluster-connections) for the
precedence rules.

```yaml
{!docs/operator-manual/argocd-cluster-ca-cm.yaml!}
```
