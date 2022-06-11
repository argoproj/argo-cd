# Redis

[Redis](https://redis.io/) is an advanced key-value cache and store. It is often referred to as a data structure server since keys can contain strings, hashes, lists, sets, sorted sets, bitmaps and hyperloglogs.

## TL;DR

```bash
# Testing configuration
$ helm install stable/redis
```

```bash
# Production configuration
$ helm install stable/redis --values values-production.yaml
```

## Introduction

This chart bootstraps a [Redis](https://github.com/bitnami/bitnami-docker-redis) deployment on a [Kubernetes](https://kubernetes.io) cluster using the [Helm](https://helm.sh) package manager.

## Prerequisites

- Kubernetes 1.8+
- PV provisioner support in the underlying infrastructure

## Installing the Chart

To install the chart with the release name `my-release`:

```bash
$ helm install --name my-release stable/redis
```

The command deploys Redis on the Kubernetes cluster in the default configuration. The [configuration](#configuration) section lists the parameters that can be configured during installation.

> **Tip**: List all releases using `helm list`

## Uninstalling the Chart

To uninstall/delete the `my-release` deployment:

```bash
$ helm delete my-release
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Configuration

The following table lists the configurable parameters of the Redis chart and their default values.

| Parameter                                  | Description                                                                                                    | Default                              |
|--------------------------------------------|----------------------------------------------------------------------------------------------------------------|--------------------------------------|
| `image.registry`                           | Redis Image registry                                                                                           | `docker.io`                                          |
| `image.repository`                         | Redis Image name                                                                                               | `bitnami/redis`                                      |
| `image.tag`                                | Redis Image tag                                                                                                | `{VERSION}`                                          |
| `image.pullPolicy`                         | Image pull policy                                                                                              | `Always`                                             |
| `image.pullSecrets`                        | Specify docker-registry secret names as an array                                                               | `nil`                                                |
| `cluster.enabled`                          | Use master-slave topology                                                                                      | `true`                                               |
| `cluster.slaveCount`                       | Number of slaves                                                                                               | 1                                                    |
| `existingSecret`                           | Name of existing secret object (for password authentication)                                                   | `nil`                                                |
| `usePassword`                              | Use password                                                                                                   | `true`                                               |
| `password`                                 | Redis password (ignored if existingSecret set)                                                                 | Randomly generated                                   |
| `networkPolicy.enabled`                    | Enable NetworkPolicy                                                                                           | `false`                                              |
| `networkPolicy.allowExternal`              | Don't require client label for connections                                                                     | `true`                                               |
| `serviceAccount.create`                    | Specifies whether a ServiceAccount should be created                                                           | `false`                                              |
| `serviceAccount.name`                      | The name of the ServiceAccount to create                                                                       | Generated using the fullname template                |
| `rbac.create`                              | Specifies whether RBAC resources should be created                                                             | `false`                                              |
| `rbac.role.rules`                          | Rules to create                                                                                                | `[]`                                                 |
| `metrics.enabled`                          | Start a side-car prometheus exporter                                                                           | `false`                                              |
| `metrics.image.registry`                   | Redis exporter image registry                                                                                  | `docker.io`                                          |
| `metrics.image.repository`                 | Redis exporter image name                                                                                      | `bitnami/redis`                                      |
| `metrics.image.tag`                        | Redis exporter image tag                                                                                       | `v0.20.2`                                            |
| `metrics.image.pullPolicy`                 | Image pull policy                                                                                              | `IfNotPresent`                                       |
| `metrics.image.pullSecrets`                | Specify docker-registry secret names as an array                                                               | `nil`                                                |
| `metrics.podLabels`                        | Additional labels for Metrics exporter pod                                                                     | {}                                                   |
| `metrics.podAnnotations`                   | Additional annotations for Metrics exporter pod                                                                | {}                                                   |
| `master.service.type`                      | Kubernetes Service type (redis metrics)                                                                        | `LoadBalancer`                                       |
| `metrics.service.annotations`              | Annotations for the services to monitor  (redis master and redis slave service)                                | {}                                                   |
| `metrics.service.loadBalancerIP`           | loadBalancerIP if redis metrics service type is `LoadBalancer`                                                 | `nil`                                                |
| `metrics.resources`                        | Exporter resource requests/limit                                                                               | Memory: `256Mi`, CPU: `100m`                         |
| `persistence.existingClaim`                | Provide an existing PersistentVolumeClaim                                                                      | `nil`                                                |
| `master.persistence.enabled`               | Use a PVC to persist data (master node)                                                                        | `true`                                               |
| `master.persistence.path`                  | Path to mount the volume at, to use other images                                                               | `/bitnami`                                           |
| `master.persistence.subPath`               | Subdirectory of the volume to mount at                                                                         | `""`                                                 |
| `master.persistence.storageClass`          | Storage class of backing PVC                                                                                   | `generic`                                            |
| `master.persistence.accessModes`           | Persistent Volume Access Modes                                                                                 | `[ReadWriteOnce]`                                    |
| `master.persistence.size`                  | Size of data volume                                                                                            | `8Gi`                                                |
| `master.statefulset.updateStrategy`        | Update strategy for StatefulSet                                                                                | onDelete                                             |
| `master.statefulset.rollingUpdatePartition`| Partition update strategy                                                                                      | `nil`                                                |
| `master.podLabels`                         | Additional labels for Redis master pod                                                                         | {}                                                   |
| `master.podAnnotations`                    | Additional annotations for Redis master pod                                                                    | {}                                                   |
| `master.port`                              | Redis master port                                                                                              | 6379                                                 |
| `master.args`                              | Redis master command-line args                                                                                 | []                                                   |
| `master.disableCommands`                   | Comma-separated list of Redis commands to disable (master)                                                     | `FLUSHDB,FLUSHALL`                                   |
| `master.extraFlags`                        | Redis master additional command line flags                                                                     | []                                                   |
| `master.nodeSelector`                      | Redis master Node labels for pod assignment                                                                    | {"beta.kubernetes.io/arch": "amd64"}                 |
| `master.tolerations`                       | Toleration labels for Redis master pod assignment                                                              | []                                                   |
| `master.affinity   `                       | Affinity settings for Redis master pod assignment                                                              | []                                                   |
| `master.schedulerName`                     | Name of an alternate scheduler                                                                                 | `nil`                                                |
| `master.service.type`                      | Kubernetes Service type (redis master)                                                                         | `ClusterIP`                                          |
| `master.service.annotations`               | annotations for redis master service                                                                           | {}                                                   |
| `master.service.loadBalancerIP`            | loadBalancerIP if redis master service type is `LoadBalancer`                                                  | `nil`                                                |
| `master.securityContext.enabled`           | Enable security context (redis master pod)                                                                     | `true`                                               |
| `master.securityContext.fsGroup`           | Group ID for the container (redis master pod)                                                                  | `1001`                                               |
| `master.securityContext.runAsUser`         | User ID for the container (redis master pod)                                                                   | `1001`                                               |
| `master.resources`                         | Redis master CPU/Memory resource requests/limits                                                               | Memory: `256Mi`, CPU: `100m`                         |
| `master.livenessProbe.enabled`             | Turn on and off liveness probe (redis master pod)                                                              | `true`                                               |
| `master.livenessProbe.initialDelaySeconds` | Delay before liveness probe is initiated (redis master pod)                                                    | `30`                                                 |
| `master.livenessProbe.periodSeconds`       | How often to perform the probe (redis master pod)                                                              | `30`                                                 |
| `master.livenessProbe.timeoutSeconds`      | When the probe times out (redis master pod)                                                                    | `5`                                                  |
| `master.livenessProbe.successThreshold`    | Minimum consecutive successes for the probe to be considered successful after having failed (redis master pod) | `1`                                                  |
| `master.livenessProbe.failureThreshold`    | Minimum consecutive failures for the probe to be considered failed after having succeeded.                     | `5`                                                  |
| `master.readinessProbe.enabled`            | Turn on and off readiness probe (redis master pod)                                                             | `true`                                               |
| `master.readinessProbe.initialDelaySeconds`| Delay before readiness probe is initiated (redis master pod)                                                   | `5`                                                  |
| `master.readinessProbe.periodSeconds`      | How often to perform the probe (redis master pod)                                                              | `10`                                                 |
| `master.readinessProbe.timeoutSeconds`     | When the probe times out (redis master pod)                                                                    | `1`                                                  |
| `master.readinessProbe.successThreshold`   | Minimum consecutive successes for the probe to be considered successful after having failed (redis master pod) | `1`                                                  |
| `master.readinessProbe.failureThreshold`   | Minimum consecutive failures for the probe to be considered failed after having succeeded.                     | `5`                                                  |
| `slave.serviceType`                        | Kubernetes Service type (redis slave)                                                                          | `LoadBalancer`                                       |
| `slave.service.annotations`                | annotations for redis slave service                                                                            | {}                                                   |
| `slave.service.loadBalancerIP`             | LoadBalancerIP if Redis slave service type is `LoadBalancer`                                                   | `nil`                                                |
| `slave.port`                               | Redis slave port                                                                                               | `master.port`                                        |
| `slave.args`                               | Redis slave command-line args                                                                                  | `master.args`                                        |
| `slave.disableCommands`                    | Comma-separated list of Redis commands to disable (slave)                                                      | `master.disableCommands`                             |
| `slave.extraFlags`                         | Redis slave additional command line flags                                                                      | `master.extraFlags`                                  |
| `slave.livenessProbe.enabled`              | Turn on and off liveness probe (redis slave pod)                                                               | `master.livenessProbe.enabled`                       |
| `slave.livenessProbe.initialDelaySeconds`  | Delay before liveness probe is initiated (redis slave pod)                                                     | `master.livenessProbe.initialDelaySeconds`           |
| `slave.livenessProbe.periodSeconds`        | How often to perform the probe (redis slave pod)                                                               | `master.livenessProbe.periodSeconds`                 |
| `slave.livenessProbe.timeoutSeconds`       | When the probe times out (redis slave pod)                                                                     | `master.livenessProbe.timeoutSeconds`                |
| `slave.livenessProbe.successThreshold`     | Minimum consecutive successes for the probe to be considered successful after having failed (redis slave pod)  | `master.livenessProbe.successThreshold`              |
| `slave.livenessProbe.failureThreshold`     | Minimum consecutive failures for the probe to be considered failed after having succeeded.                     | `master.livenessProbe.failureThreshold`              |
| `slave.readinessProbe.enabled`             | Turn on and off slave.readiness probe (redis slave pod)                                                        | `master.readinessProbe.enabled`                      |
| `slave.readinessProbe.initialDelaySeconds` | Delay before slave.readiness probe is initiated (redis slave pod)                                              | `master.readinessProbe.initialDelaySeconds`          |
| `slave.readinessProbe.periodSeconds`       | How often to perform the probe (redis slave pod)                                                               | `master.readinessProbe.periodSeconds`                |
| `slave.readinessProbe.timeoutSeconds`      | When the probe times out (redis slave pod)                                                                     | `master.readinessProbe.timeoutSeconds`               |
| `slave.readinessProbe.successThreshold`    | Minimum consecutive successes for the probe to be considered successful after having failed (redis slave pod)  | `master.readinessProbe.successThreshold`             |
| `slave.readinessProbe.failureThreshold`    | Minimum consecutive failures for the probe to be considered failed after having succeeded. (redis slave pod)   | `master.readinessProbe.failureThreshold`             |
| `slave.podLabels`                          | Additional labels for Redis slave pod                                                                          | `master.podLabels`                                   |
| `slave.podAnnotations`                     | Additional annotations for Redis slave pod                                                                     | `master.podAnnotations`                              |
| `slave.schedulerName`                      | Name of an alternate scheduler                                                                                 | `nil`                                                |
| `slave.securityContext.enabled`            | Enable security context (redis slave pod)                                                                      | `master.securityContext.enabled`                     |
| `slave.securityContext.fsGroup`            | Group ID for the container (redis slave pod)                                                                   | `master.securityContext.fsGroup`                     |
| `slave.securityContext.runAsUser`          | User ID for the container (redis slave pod)                                                                    | `master.securityContext.runAsUser`                   |
| `slave.resources`                          | Redis slave CPU/Memory resource requests/limits                                                                | `master.resources`                                   |
| `slave.affinity`                          | Enable node/pod affinity for slaves                                                                | {}                                   |

The above parameters map to the env variables defined in [bitnami/redis](https://github.com/bitnami/bitnami-docker-redis). For more information please refer to the [bitnami/redis](https://github.com/bitnami/bitnami-docker-redis) image documentation.

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`. For example,

```bash
$ helm install --name my-release \
  --set password=secretpassword \
    stable/redis
```

The above command sets the Redis server password to `secretpassword`.

Alternatively, a YAML file that specifies the values for the parameters can be provided while installing the chart. For example,

```bash
$ helm install --name my-release -f values.yaml stable/redis
```

> **Tip**: You can use the default [values.yaml](values.yaml)

> **Note for minikube users**: Current versions of minikube (v0.24.1 at the time of writing) provision `hostPath` persistent volumes that are only writable by root. Using chart defaults cause pod failure for the Redis pod as it attempts to write to the `/bitnami` directory. Consider installing Redis with `--set persistence.enabled=false`. See minikube issue [1990](https://github.com/kubernetes/minikube/issues/1990) for more information.

## NetworkPolicy

To enable network policy for Redis, install
[a networking plugin that implements the Kubernetes NetworkPolicy spec](https://kubernetes.io/docs/tasks/administer-cluster/declare-network-policy#before-you-begin),
and set `networkPolicy.enabled` to `true`.

For Kubernetes v1.5 & v1.6, you must also turn on NetworkPolicy by setting
the DefaultDeny namespace annotation. Note: this will enforce policy for _all_ pods in the namespace:

    kubectl annotate namespace default "net.beta.kubernetes.io/network-policy={\"ingress\":{\"isolation\":\"DefaultDeny\"}}"

With NetworkPolicy enabled, only pods with the generated client label will be
able to connect to Redis. This label will be displayed in the output
after a successful install.

## Persistence

The [Bitnami Redis](https://github.com/bitnami/bitnami-docker-redis) image stores the Redis data and configurations at the `/bitnami` path of the container.

By default, the chart mounts a [Persistent Volume](https://kubernetes.io/docs/user-guide/persistent-volumes/) at this location. The volume is created using dynamic volume provisioning. If a Persistent Volume Claim already exists, specify it during installation.

By default, the chart persists both data and configuration. If you wish to persist only the data directory set `persistence.path` to `/bitnami/redis/data` and `persistence.subPath` to `redis/data`.

### Existing PersistentVolumeClaim

1. Create the PersistentVolume
1. Create the PersistentVolumeClaim
1. Install the chart

```bash
$ helm install --set persistence.existingClaim=PVC_NAME stable/redis
```

## Metrics

The chart optionally can start a metrics exporter for [prometheus](https://prometheus.io). The metrics endpoint (port 9121) is exposed in the service. Metrics can be scraped from within the cluster using something similar as the described in the [example Prometheus scrape configuration](https://github.com/prometheus/prometheus/blob/master/documentation/examples/prometheus-kubernetes.yml). If metrics are to be scraped from outside the cluster, the Kubernetes API proxy can be utilized to access the endpoint.
