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

### Argocd App

The health assessment of `argoproj.io/Application` CRD has been removed in argocd 1.8 (see [#3781](https://github.com/argoproj/argo-cd/issues/3781) for more information).
You might need to restore it if you are using app-of-apps pattern and orchestrating synchronization using sync waves. Add the following resource customization in
`argocd-cm` ConfigMap:

```yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
  labels:
    app.kubernetes.io/name: argocd-cm
    app.kubernetes.io/part-of: argocd
data:
  resource.customizations.health.argoproj.io_Application: |
    hs = {}
    hs.status = "Progressing"
    hs.message = ""
    if obj.status ~= nil then
      if obj.status.health ~= nil then
        hs.status = obj.status.health.status
        if obj.status.health.message ~= nil then
          hs.message = obj.status.health.message
        end
      end
    end
    return hs
```

## Custom Health Checks

Argo CD supports custom health checks written in [Lua](https://www.lua.org/). This is useful if you:

* Are affected by known issues where your `Ingress` or `StatefulSet` resources are stuck in `Progressing` state because of bug in your resource controller.
* Have a custom resource for which Argo CD does not have a built-in health check.

There are two ways to configure a custom health check. The next two sections describe those ways.

### Way 1. Define a Custom Health Check in `argocd-cm` ConfigMap

Custom health checks can be defined in `resource.customizations.health.<group_kind>` field of `argocd-cm`. If you are using argocd-operator, this is overridden by [the argocd-operator resourceCustomizations](https://argocd-operator.readthedocs.io/en/latest/reference/argocd/#resource-customizations).

The following example demonstrates a health check for `cert-manager.io/Certificate`.

```yaml
data:
  resource.customizations.health.cert-manager.io_Certificate: |
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
The custom health check might return one of the following health statuses:

  * `Healthy` - the resource is healthy
  * `Progressing` - the resource is not healthy yet but still making progress and might be healthy soon
  * `Degraded` - the resource is degraded
  * `Suspended` - the resource is suspended and waiting for some external event to resume (e.g. suspended CronJob or paused Deployment)

By default health typically returns `Progressing` status.

NOTE: As a security measure, access to the standard Lua libraries will be disabled by default. Admins can control access by
setting `resource.customizations.useOpenLibs.<group_kind>`. In the following example, standard libraries are enabled for health check of `cert-manager.io/Certificate`.

```yaml
data:
  resource.customizations.useOpenLibs.cert-manager.io_Certificate: "true"
  resource.customizations.health.cert-manager.io_Certificate:
    -- Lua standard libraries are enabled for this script
```

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

To test the implemented custom health checks, run `go test -v ./util/lua/`.

The [PR#1139](https://github.com/argoproj/argo-cd/pull/1139) is an example of Cert Manager CRDs custom health check.
