# Declarative Setup

Argo CD settings might be defined declaratively using Kubernetes manifests.

## Repositories

Repository credentials are stored in secret. Use following steps to configure a repo:

1. Create secret which contains repository credentials. Consider using [bitnami-labs/sealed-secrets](https://github.com/bitnami-labs/sealed-secrets) to store encrypted secret
definition as a Kubernetes manifest.

2. Register repository in `argocd-cm` config map. Each repository must have `url` field and `usernameSecret`, `passwordSecret` or `sshPrivateKeySecret`.

Example:

```yaml
apiVersion: v1
data:
  dex.config: |
    connectors:
    - type: github
      id: github
      name: GitHub
      config:
        clientID: e8f597564a82e99ba9aa
        clientSecret: e551007c6c6dbc666bdade281ff095caec150159
  repositories: |
    - passwordSecret:
        key: password
        name: my-secret
      url: https://github.com/argoproj/my-private-repository
      usernameSecret:
        key: username
        name: my-secret
  url: http://localhost:4000
kind: ConfigMap
metadata:
  name: argocd-cm
```

## Clusters

Cluster credentials are stored in secrets same as repository credentials but does not require entry in `argocd-cm` config map. Each secret must have label
`argocd.argoproj.io/secret-type: cluster` and name which is following convention: `<hostname>-<port>`.

The secret data must include following fields:
* `name` - cluster name
* `server` - cluster api server url
* `config` - JSON representation of following data structure:

```yaml
# Basic authentication settings
username: string
password: string
# Bearer authentication settings
bearerToken: string
# IAM authentication configuration
awsAuthConfig:
    clusterName: string
    roleARN: string
# Transport layer security configuration settings
tlsClientConfig:
    # PEM-encoded bytes (typically read from a client certificate file).
    caData: string
    # PEM-encoded bytes (typically read from a client certificate file).
    certData: string
    # Server should be accessed without verifying the TLS certificate
    insecure: boolean
    # PEM-encoded bytes (typically read from a client certificate key file).
    keyData: string
    # ServerName is passed to the server for SNI and is used in the client to check server
    # ceritificates against. If ServerName is empty, the hostname used to contact the
    # server is used.
    serverName: string

```


Cluster secret example:

```yaml
apiVersion: v1
stringData:
  config: |||
    {
        "bearerToken": "<authentication token>",
        "tlsClientConfig": {
            "insecure": false,
            "caData": "<base64 encoded certificate>"
        }
    }
  |||
  name: mycluster.com
  server: https://mycluster.com
kind: Secret
metadata:
  labels:
    argocd.argoproj.io/secret-type: cluster
  name: mycluster.com-443
type: Opaque
```

## SSO & RBAC

* SSO configuration details: [SSO](sso.md)
* RBAC configuration details: [RBAC](rbac.md)

## Manage Argo CD using Argo CD

Argo CD is able to manage itself since all settings are represented by Kubernetes manifests. The suggested way is to create [Kustomize](https://github.com/kubernetes-sigs/kustomize)
based application which uses base Argo CD manifests from https://github.com/argoproj/argo-cd and apply required changes on top.

Example of `kustomization.yaml`:

```yaml
bases:
- github.com/argoproj/argo-cd//manifests/cluster-install?ref=v0.10.0

# additional resources like ingress rules, cluster and repository secrets.
resources:
- clusters-secrets.yaml
- repos-secrets.yaml

# changes to config maps
patchesStrategicMerge:
- overlays/argo-cd-cm.yaml
```

The live example of self managed Argo CD config is available at https://cd.apps.argoproj.io and with configuration
stored at [argoproj/argoproj-deployments](https://github.com/argoproj/argoproj-deployments/tree/master/argocd).
