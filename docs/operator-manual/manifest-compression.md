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

The serialization format and compression algorithm are configured in the `argocd-cm` ConfigMap. These settings support hot reload and changes take effect when the cluster cache is invalidated and re-synced.

### Storage Format

| ConfigMap key | Description |
|---------------------|-------------|
| `resource.manifest.storage` | Serialization format. Default: `json` |

Supported values: `json`, `msgpack`

### Compression Algorithm

| ConfigMap key | Description |
|---------------------|-------------|
| `resource.manifest.compression` | Compression algorithm. Default: `gzip-bestspeed` |

Supported values: `gzip-bestspeed`, `gzip-default`, `s2-encode`, `s2-encodebetter`, `zlib`, `none`

### Example ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
data:
  resource.manifest.storage: "msgpack"
  resource.manifest.compression: "gzip-bestspeed"
```
## Disabling

Set the configmap key to `"false"` or remove it:

```yaml
data:
  resource.manifest.compression.enabled: "false"
```

When disabled, manifests are stored as raw in-memory objects, identical to the default ArgoCD behavior. The transition is seamless; the cache re-syncs automatically.