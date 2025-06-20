Minio
=====

[Minio](https://minio.io) is a lightweight, AWS S3 compatible object storage server. It is best suited for storing unstructured data such as photos, videos, log files, backups, VM and container images. Size of an object can range from a few KBs to a maximum of 5TB. Minio server is light enough to be bundled with the application stack, similar to NodeJS, Redis and MySQL.

Minio supports [distributed mode](https://docs.minio.io/docs/distributed-minio-quickstart-guide). In distributed mode, you can pool multiple drives (even on different machines) into a single object storage server.

Introduction
------------

This chart bootstraps Minio deployment on a [Kubernetes](https://kubernetes.io) cluster using the [Helm](https://helm.sh) package manager.

Prerequisites
-------------

-	Kubernetes 1.4+ with Beta APIs enabled for default standalone mode.
-   Kubernetes 1.5+ with Beta APIs enabled to run Minio in [distributed mode](#distributed-minio).
-	PV provisioner support in the underlying infrastructure.

Installing the Chart
--------------------

Install this chart using:

```bash
$ helm install stable/minio
```

The command deploys Minio on the Kubernetes cluster in the default configuration. The [configuration](#configuration) section lists the parameters that can be configured during installation.

### Release name

An instance of a chart running in a Kubernetes cluster is called a release. Each release is identified by a unique name within the cluster. Helm automatically assigns a unique release name after installing the chart. You can also set your preferred name by:

```bash
$ helm install --name my-release stable/minio
```

### Access and Secret keys

By default a pre-generated access and secret key will be used. To override the default keys, pass the access and secret keys as arguments to helm install.

```bash
$ helm install --set accessKey=myaccesskey,secretKey=mysecretkey \
    stable/minio
```

### Updating Minio configuration via Helm

[ConfigMap](https://kubernetes.io/docs/user-guide/configmap/) allows injecting containers with configuration data even while a Helm release is deployed.

To update your Minio server configuration while it is deployed in a release, you need to

1. Check all the configurable values in the Minio chart using `helm inspect values stable/minio`.
2. Override the `minio_server_config` settings in a YAML formatted file, and then pass that file like this `helm upgrade -f config.yaml stable/minio`.
3. Restart the Minio server(s) for the changes to take effect.

You can also check the history of upgrades to a release using `helm history my-release`. Replace `my-release` with the actual release name.

Uninstalling the Chart
----------------------

Assuming your release is named as `my-release`, delete it using the command:

```bash
$ helm delete my-release
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

Configuration
-------------

The following table lists the configurable parameters of the Minio chart and their default values.

| Parameter                  | Description                         | Default                                                 |
|----------------------------|-------------------------------------|---------------------------------------------------------|
| `image.repository`         | Image repository                    | `minio/minio`                                           |
| `image.tag`                | Minio image tag. Possible values listed [here](https://hub.docker.com/r/minio/minio/tags/).| `RELEASE.2018-07-10T01-42-11Z`|
| `image.pullPolicy`         | Image pull policy                   | `IfNotPresent`                                          |
| `mcImage.repository`       | Client image repository             | `minio/mc`                                              |
| `mcImage.tag`              | mc image tag. Possible values listed [here](https://hub.docker.com/r/minio/mc/tags/).| `RELEASE.2018-06-09T02-18-09Z`|
| `mcImage.pullPolicy`       | mc Image pull policy                | `IfNotPresent`                                          |
| `ingress.enabled`          | Enables Ingress                     | `false`                                                 |
| `ingress.annotations`      | Ingress annotations                 | `{}`                                                    |
| `ingress.hosts`            | Ingress accepted hostnames          | `[]`                                                    |
| `ingress.tls`              | Ingress TLS configuration           | `[]`                                                    |
| `mode`                     | Minio server mode (`standalone` or `distributed`)| `standalone`                               |
| `replicas`                 | Number of nodes (applicable only for Minio distributed mode). Should be 4 <= x <= 32 | `4`    |
| `accessKey`                | Default access key (5 to 20 characters) | `AKIAIOSFODNN7EXAMPLE`                              |
| `secretKey`                | Default secret key (8 to 40 characters) | `wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY`          |
| `configPath`               | Default config file location        | `~/.minio`                                              |
| `mountPath`                | Default mount location for persistent drive| `/export`                                        |
| `service.type`             | Kubernetes service type             | `ClusterIP`                                             |
| `service.port`             | Kubernetes port where service is exposed| `9000`                                              |
| `service.annotations`      | Service annotations                 | `{}`                                                    |
| `persistence.enabled`      | Use persistent volume to store data | `true`                                                  |
| `persistence.size`         | Size of persistent volume claim     | `10Gi`                                                  |
| `persistence.existingClaim`| Use an existing PVC to persist data | `nil`                                                   |
| `persistence.storageClass` | Type of persistent volume claim     | `generic`                                               |
| `persistence.accessMode`   | ReadWriteOnce or ReadOnly           | `ReadWriteOnce`                                         |
| `persistence.subPath`      | Mount a sub directory of the persistent volume if set | `""`                                  |
| `resources`                | CPU/Memory resource requests/limits | Memory: `256Mi`, CPU: `100m`                            |
| `priorityClassName`        | Pod priority settings               | `""`                                                    |
| `nodeSelector`             | Node labels for pod assignment      | `{}`                                                    |
| `affinity`                 | Affinity settings for pod assignment | `{}`                                                   |
| `tolerations`              | Toleration labels for pod assignment | `[]`                                                   |
| `defaultBucket.enabled`    | If set to true, a bucket will be created after minio install | `false`                        |
| `defaultBucket.name`       | Bucket name                         | `bucket`                                                |
| `defaultBucket.policy`     | Bucket policy                       | `none`                                                  |
| `defaultBucket.purge`      | Purge the bucket if already exists  | `false`                                                 |
| `azuregateway.enabled`     | Use minio as an [azure gateway](https://docs.minio.io/docs/minio-gateway-for-azure)| `false`  |
| `gcsgateway.enabled`       | Use minio as a [Google Cloud Storage gateway](https://docs.minio.io/docs/minio-gateway-for-gcs)| `false` |
| `gcsgateway.gcsKeyJson`    | credential json file of service account key | `""` |
| `gcsgateway.projectId`     | Google cloud project id             | `""` |
| `nasgateway.enabled`       | Use minio as a [NAS gateway](https://docs.minio.io/docs/minio-gateway-for-nas)             | `false` |
| `nasgateway.replicas`      | Number of NAS gateway instances to be run in parallel on a PV            | `4` |

Some of the parameters above map to the env variables defined in the [Minio DockerHub image](https://hub.docker.com/r/minio/minio/).

You can specify each parameter using the `--set key=value[,key=value]` argument to `helm install`. For example,

```bash
$ helm install --name my-release \
  --set persistence.size=100Gi \
    stable/minio
```

The above command deploys Minio server with a 100Gi backing persistent volume.

Alternately, you can provide a YAML file that specifies parameter values while installing the chart. For example,

```bash
$ helm install --name my-release -f values.yaml stable/minio
```

> **Tip**: You can use the default [values.yaml](values.yaml)

Distributed Minio
-----------

This chart provisions a Minio server in standalone mode, by default. To provision Minio server in [distributed mode](https://docs.minio.io/docs/distributed-minio-quickstart-guide), set the `mode` field to `distributed`,

```bash
$ helm install --set mode=distributed stable/minio
```

This provisions Minio server in distributed mode with 4 nodes. To change the number of nodes in your distributed Minio server, set the `replicas` field,

```bash
$ helm install --set mode=distributed,replicas=8 stable/minio
```

This provisions Minio server in distributed mode with 8 nodes. Note that the `replicas` value should be an integer between 4 and 16 (inclusive).

### StatefulSet [limitations](https://kubernetes.io/docs/concepts/abstractions/controllers/statefulsets/#limitations) applicable to distributed Minio

1. StatefulSets need persistent storage, so the `persistence.enabled` flag is ignored when `mode` is set to `distributed`.
2. When uninstalling a distributed Minio release, you'll need to manually delete volumes associated with the StatefulSet.

NAS Gateway
-----------

### Prerequisites

Minio in [NAS gateway mode](https://docs.minio.io/docs/minio-gateway-for-nas) can be used to create multiple Minio instances backed by single PV in `ReadWriteMany` mode. Currently few [Kubernetes volume plugins](https://kubernetes.io/docs/user-guide/persistent-volumes/#access-modes) support `ReadWriteMany` mode. To deploy Minio NAS gateway with Helm chart you'll need to have a Persistent Volume running with one of the supported volume plugins. [This document](https://kubernetes.io/docs/user-guide/volumes/#nfs)
outlines steps to create a NFS PV in Kubernetes cluster.

### Provision NAS Gateway Minio instances

To provision Minio servers in [NAS gateway mode](https://docs.minio.io/docs/minio-gateway-for-nas), set the `nasgateway.enabled` field to `true`,

```bash
$ helm install --set nasgateway.enabled=true stable/minio
```

This provisions 4 Minio NAS gateway instances backed by single storage. To change the number of instances in your Minio deployment, set the `replicas` field,

```bash
$ helm install --set nasgateway.enabled=true,nasgateway.replicas=8 stable/minio
```

This provisions Minio NAS gateway with 8 instances.

Persistence
-----------

This chart provisions a PersistentVolumeClaim and mounts corresponding persistent volume to default location `/export`. You'll need physical storage available in the Kubernetes cluster for this to work. If you'd rather use `emptyDir`, disable PersistentVolumeClaim by:

```bash
$ helm install --set persistence.enabled=false stable/minio
```

> *"An emptyDir volume is first created when a Pod is assigned to a Node, and exists as long as that Pod is running on that node. When a Pod is removed from a node for any reason, the data in the emptyDir is deleted forever."*

Existing PersistentVolumeClaim
------------------------------

If a Persistent Volume Claim already exists, specify it during installation.

1. Create the PersistentVolume
1. Create the PersistentVolumeClaim
1. Install the chart

```bash
$ helm install --set persistence.existingClaim=PVC_NAME stable/minio
```

NetworkPolicy
-------------

To enable network policy for Minio,
install [a networking plugin that implements the Kubernetes
NetworkPolicy spec](https://kubernetes.io/docs/tasks/administer-cluster/declare-network-policy#before-you-begin),
and set `networkPolicy.enabled` to `true`.

For Kubernetes v1.5 & v1.6, you must also turn on NetworkPolicy by setting
the DefaultDeny namespace annotation. Note: this will enforce policy for _all_ pods in the namespace:

    kubectl annotate namespace default "net.beta.kubernetes.io/network-policy={\"ingress\":{\"isolation\":\"DefaultDeny\"}}"

With NetworkPolicy enabled, traffic will be limited to just port 9000.

For more precise policy, set `networkPolicy.allowExternal=true`. This will
only allow pods with the generated client label to connect to Minio.
This label will be displayed in the output of a successful install.
