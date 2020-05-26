# Resource Health

## Overview
Argo CD provides built-in health assessment for several standard Kubernetes types, which is then
surfaced to the overall Application health status as a whole. The following checks are made for
specific types of kubernetes resources:

### Deployment, ReplicaSet, StatefulSet DaemonSet
* Observed generation is equal to desired generation.
* Number of **updated** replicas equals the number of desired replicas.

### Service
* If service type is of type `LoadBalancer`, the `status.loadBalancer.ingress` list is non-empty,
with at least one value for `hostname` or `IP`.

### Ingress
* The `status.loadBalancer.ingress` list is non-empty, with at least one value for `hostname` or `IP`.

### PersistentVolumeClaim
* The `status.phase` is `Bound`

## Custom Health Checks

Argo CD supports custom health checks written in [Lua](https://www.lua.org/). This is useful if you:

* Are affected by known issues where your `Ingress` or `StatefulSet` resources are stuck in `Progressing` state because of bug in your resource controller.
* Have a custom resource for which Argo CD does not have a built-in health check.

There are two ways to configure a custom health check. The next two sections describe those ways.

### Way 1. Define a Custom Health Check in `argocd-cm` ConfigMap

Custom health checks can be defined in `resource.customizations` field of `argocd-cm`. Following example demonstrates a health check for `cert-manager.io/Certificate`.

```yaml
data:
  resource.customizations: |
    cert-manager.io/Certificate:
      health.lua: |
        hs = {}
        if obj.status ~= nil then
          if obj.status.conditions ~= nil then
            for i, condition in ipairs(obj.status.conditions) do
              if condition.type == "Ready" and condition.status == "False" then
                hs.status = "Degraded"
                hs.message = condition.message
                return hs
              end
              if condition.type == "Ready" and condition.status == "True" then
                hs.status = "Healthy"
                hs.message = condition.message
                return hs
              end
            end
          end
        end
        
        hs.status = "Progressing"
        hs.message = "Waiting for certificate"
        return hs
```
The `obj` is a global variable which contains the resource. The script must return an object with status and optional message field.

NOTE: as a security measure you don't have access to most of the standard Lua libraries.

### Way 2. Contribute a Custom Health Check

A health check can be bundled into Argo CD. Custom health check scripts are located in the `resource_customizations` directory of [https://github.com/argoproj/argo-cd](https://github.com/argoproj/argo-cd). This must have the following directory structure:

```
argo-cd
|-- resource_customizations
|    |-- your.crd.group.io               # CRD group
|    |    |-- MyKind                     # Resource kind
|    |    |    |-- health.lua            # Health check
|    |    |    |-- health_test.yaml      # Test inputs and expected results
|    |    |    +-- testdata              # Directory with test resource YAML definitions
```

Each health check must have tests defined in `health_test.yaml` file. The `health_test.yaml` is a YAML file with the following structure:

```yaml
tests:
- healthStatus:
    status: ExpectedStatus
    message: Expected message
  inputPath: testdata/test-resource-definition.yaml
```

The [PR#1139](https://github.com/argoproj/argo-cd/pull/1139) is an example of Cert Manager CRDs custom health check.
