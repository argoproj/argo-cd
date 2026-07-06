# Manifest Compression

## Overview

By default, the application controller stores cached resource manifests as raw in-memory objects. For clusters with a large number of managed resources, this can consume significant memory.

When manifest compression is enabled, cached manifests are serialized and compressed before being stored in memory, significantly reducing memory usage of the application controller.

## Prerequisites

- ArgoCD v3.5+

## Enabling Manifest Compression

Add the following key to your `argocd-cm` ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
data:
  resource.manifest.compression.enabled: "true"
```

This setting supports hot-reload and changes take effect without restarting the controller. The cluster cache is automatically invalidated and re-synced with the new storage mode.

## Configuring Storage and Compression

The serialization format and compression algorithm are configured via environment variables on the `argocd-application-controller` deployment.

### Storage Format

| Environment Variable | Description |
|---------------------|-------------|
| `ARGOCD_CLUSTER_CACHE_MANIFEST_STORAGE` | Serialization format. Default: `json` |

Supported values: `json`, `jsoniter`, `msgpack`

### Compression Algorithm

| Environment Variable | Description |
|---------------------|-------------|
| `ARGOCD_CLUSTER_CACHE_MANIFEST_COMPRESSION` | Compression algorithm. Default: `gzip-bestspeed` |

Supported values: `gzip-bestspeed`, `gzip-default`, `s2-encode`, `s2-encodebetter`, `zlib`, `none`

### Example Deployment Patch

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: argocd-application-controller
spec:
  template:
    spec:
      containers:
      - name: argocd-application-controller
        env:
        - name: ARGOCD_CLUSTER_CACHE_MANIFEST_STORAGE
          value: "json"
        - name: ARGOCD_CLUSTER_CACHE_MANIFEST_COMPRESSION
          value: "gzip-bestspeed"
```
## Disabling

Set the configmap key to `"false"` or remove it:

```yaml
data:
  resource.manifest.compression.enabled: "false"
```

When disabled, manifests are stored as raw in-memory objects, identical to the default ArgoCD behavior. The transition is seamless; the cache re-syncs automatically.