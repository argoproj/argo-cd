# High Availability

Argo CD is largely stateless. All data is persisted as Kubernetes objects, which in turn is stored in Kubernetes' etcd. Redis is only used as a throw-away cache and can be lost. When lost, it will be rebuilt without loss of service.

A set of [HA manifests](https://github.com/argoproj/argo-cd/tree/master/manifests/ha) are provided for users who wish to run Argo CD in a highly available manner. This runs more containers, and runs Redis in HA mode.

> **NOTE:** The HA installation will require at least three different nodes due to pod anti-affinity roles in the
> specs. Additionally, IPv6 only clusters are not supported.

## Scaling Up

### argocd-repo-server

**settings:**

The `argocd-repo-server` is responsible for cloning Git repository, keeping it up to date and generating manifests using the appropriate tool.

* `argocd-repo-server` fork/exec config management tool to generate manifests. The fork can fail due to lack of memory or limit on the number of OS threads.
The `--parallelismlimit` flag controls how many manifests generations are running concurrently and helps avoid OOM kills.

* the `argocd-repo-server` ensures that repository is in the clean state during the manifest generation using config management tools such as Kustomize, Helm
or custom plugin. As a result Git repositories with multiple applications might affect repository server performance.
Read [Monorepo Scaling Considerations](#monorepo-scaling-considerations) for more information.

* `argocd-repo-server` clones the repository into `/tmp` (or the path specified in the `TMPDIR` env variable). The Pod might run out of disk space if it has too many repositories
or if the repositories have a lot of files. To avoid this problem mount a persistent volume.

* `argocd-repo-server` uses `git ls-remote` to resolve ambiguous revisions such as `HEAD`, a branch or a tag name. This operation happens frequently
and might fail. To avoid failed syncs use the `ARGOCD_GIT_ATTEMPTS_COUNT` environment variable to retry failed requests.

* `argocd-repo-server` Every 3m (by default) Argo CD checks for changes to the app manifests. Argo CD assumes by default that manifests only change when the repo changes, so it caches the generated manifests (for 24h by default). With Kustomize remote bases, or in case a Helm chart gets changed without bumping its version number, the expected manifests can change even though the repo has not changed. By reducing the cache time, you can get the changes without waiting for 24h. Use `--repo-cache-expiration duration`, and we'd suggest in low volume environments you try '1h'. Bear in mind that this will negate the benefits of caching if set too low.

* `argocd-repo-server` executes config management tools such as `helm` or `kustomize` and enforces a 90 second timeout. This timeout can be changed by using the `ARGOCD_EXEC_TIMEOUT` env variable. The value should be in the Go time duration string format, for example, `2m30s`.

**metrics:**

* `argocd_git_request_total` - Number of git requests. This metric provides two tags: `repo` - Git repo URL; `request_type` - `ls-remote` or `fetch`.

* `ARGOCD_ENABLE_GRPC_TIME_HISTOGRAM` - Is an environment variable that enables collecting RPC performance metrics. Enable it if you need to troubleshoot performance issues. Note: This metric is expensive to both query and store!

### argocd-application-controller

**settings:**

The `argocd-application-controller` uses `argocd-repo-server` to get generated manifests and Kubernetes API server to get the actual cluster state.

* each controller replica uses two separate queues to process application reconciliation (milliseconds) and app syncing (seconds). The number of queue processors for each queue is controlled by
`--status-processors` (20 by default) and `--operation-processors` (10 by default) flags. Increase the number of processors if your Argo CD instance manages too many applications.
For 1000 application we use 50 for `--status-processors` and 25 for `--operation-processors`

* The manifest generation typically takes the most time during reconciliation. The duration of manifest generation is limited to make sure the controller refresh queue does not overflow.
The app reconciliation fails with `Context deadline exceeded` error if the manifest generation is taking too much time. As a workaround increase the value of `--repo-server-timeout-seconds` and
consider scaling up the `argocd-repo-server` deployment.

* The controller uses Kubernetes watch APIs to maintain a lightweight Kubernetes cluster cache. This allows avoiding querying Kubernetes during app reconciliation and significantly improves
performance. For performance reasons the controller monitors and caches only the preferred versions of a resource. During reconciliation, the controller might have to convert cached resources from the
preferred version into a version of the resource stored in Git. If `kubectl convert` fails because the conversion is not supported then the controller falls back to Kubernetes API query which slows down
reconciliation. In this case, we advise to use the preferred resource version in Git.

* The controller polls Git every 3m by default. You can change this duration using the `timeout.reconciliation` and `timeout.reconciliation.jitter` setting in the `argocd-cm` ConfigMap. The value of the fields is a duration string e.g `60s`, `1m`, `1h` or `1d`.

* If the controller is managing too many clusters and uses too much memory then you can shard clusters across multiple
controller replicas. To enable sharding, increase the number of replicas in `argocd-application-controller` `StatefulSet`
and repeat the number of replicas in the `ARGOCD_CONTROLLER_REPLICAS` environment variable. The strategic merge patch below demonstrates changes required to configure two controller replicas.

* By default, the controller will update the cluster information every 10 seconds. If there is a problem with your cluster network environment that is causing the update time to take a long time, you can try modifying the environment variable `ARGO_CD_UPDATE_CLUSTER_INFO_TIMEOUT` to increase the timeout (the unit is seconds).

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: argocd-application-controller
spec:
  replicas: 2
  template:
    spec:
      containers:
      - name: argocd-application-controller
        env:
        - name: ARGOCD_CONTROLLER_REPLICAS
          value: "2"
```
* In order to manually set the cluster's shard number, specify the optional `shard` property when creating a cluster. If not specified, it will be calculated on the fly by the application controller.

* The shard distribution algorithm of the `argocd-application-controller` can be set by using the `--sharding-method` parameter. Supported sharding methods are : [legacy (default), round-robin]. `legacy` mode uses an `uid` based distribution (non-uniform). `round-robin` uses an equal distribution across all shards. The `--sharding-method` parameter can also be overriden by setting the key `controller.sharding.algorithm` in the `argocd-cmd-params-cm` `configMap` (preferably) or by setting the `ARGOCD_CONTROLLER_SHARDING_ALGORITHM` environment variable and by specifiying the same possible values.

!!! warning "Alpha Feature"
    The `round-robin` shard distribution algorithm  is an experimental feature. Reshuffling is known to occur in certain scenarios with cluster removal. If the cluster at rank-0 is removed, reshuffling all clusters across shards will occur and may temporarily have negative performance impacts.

* A cluster can be manually assigned and forced to a `shard` by patching the `shard` field in the cluster secret to contain the shard number, e.g.
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: mycluster-secret
  labels:
    argocd.argoproj.io/secret-type: cluster
type: Opaque
stringData:
  shard: 1
  name: mycluster.example.com
  server: https://mycluster.example.com
  config: |
    {
      "bearerToken": "<authentication token>",
      "tlsClientConfig": {
        "insecure": false,
        "caData": "<base64 encoded certificate>"
      }
    }
```

* `ARGOCD_ENABLE_GRPC_TIME_HISTOGRAM` - environment variable that enables collecting RPC performance metrics. Enable it if you need to troubleshoot performance issues. Note: This metric is expensive to both query and store!

* `ARGOCD_CLUSTER_CACHE_LIST_PAGE_BUFFER_SIZE` - environment variable controlling the number of pages the controller
  buffers in memory when performing a list operation against the K8s api server while syncing the cluster cache. This
  is useful when the cluster contains a large number of resources and cluster sync times exceed the default etcd
  compaction interval timeout. In this scenario, when attempting to sync the cluster cache, the application controller
  may throw an error that the `continue parameter is too old to display a consistent list result`. Setting a higher
  value for this environment variable configures the controller with a larger buffer in which to store pre-fetched
  pages which are processed asynchronously, increasing the likelihood that all pages have been pulled before the etcd
  compaction interval timeout expires. In the most extreme case, operators can set this value such that
  `ARGOCD_CLUSTER_CACHE_LIST_PAGE_SIZE * ARGOCD_CLUSTER_CACHE_LIST_PAGE_BUFFER_SIZE` exceeds the largest resource
  count (grouped by k8s api version, the granule of parallelism for list operations). In this case, all resources will
  be buffered in memory -- no api server request will be blocked by processing.

**metrics**

* `argocd_app_reconcile` - reports application reconciliation duration. Can be used to build reconciliation duration heat map to get a high-level reconciliation performance picture.
* `argocd_app_k8s_request_total` - number of k8s requests per application. The number of fallback Kubernetes API queries - useful to identify which application has a resource with
non-preferred version and causes performance issues.

### argocd-server

The `argocd-server` is stateless and probably the least likely to cause issues. To ensure there is no downtime during upgrades, consider increasing the number of replicas to `3` or more and repeat the number in the `ARGOCD_API_SERVER_REPLICAS` environment variable. The strategic merge patch below
demonstrates this.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: argocd-server
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: argocd-server
        env:
        - name: ARGOCD_API_SERVER_REPLICAS
          value: "3"
```

**settings:**

* The `ARGOCD_API_SERVER_REPLICAS` environment variable is used to divide [the limit of concurrent login requests (`ARGOCD_MAX_CONCURRENT_LOGIN_REQUESTS_COUNT`)](./user-management/index.md#failed-logins-rate-limiting) between each replica.
* The `ARGOCD_GRPC_MAX_SIZE_MB` environment variable allows specifying the max size of the server response message in megabytes.
The default value is 200. You might need to increase this for an Argo CD instance that manages 3000+ applications.

### argocd-dex-server, argocd-redis

The `argocd-dex-server` uses an in-memory database, and two or more instances would have inconsistent data. `argocd-redis` is pre-configured with the understanding of only three total redis servers/sentinels.

## Monorepo Scaling Considerations

Argo CD repo server maintains one repository clone locally and uses it for application manifest generation. If the manifest generation requires to change a file in the local repository clone then only one concurrent manifest generation per server instance is allowed. This limitation might significantly slowdown Argo CD if you have a mono repository with multiple applications (50+).

### Enable Concurrent Processing

Argo CD determines if manifest generation might change local files in the local repository clone based on the config management tool and application settings.
If the manifest generation has no side effects then requests are processed in parallel without a performance penalty. The following are known cases that might cause slowness and their workarounds:

  * **Multiple Helm based applications pointing to the same directory in one Git repository:** ensure that your Helm chart doesn't have conditional
[dependencies](https://helm.sh/docs/chart_best_practices/dependencies/#conditions-and-tags) and create `.argocd-allow-concurrency` file in the chart directory.

  * **Multiple Custom plugin based applications:** avoid creating temporal files during manifest generation and create `.argocd-allow-concurrency` file in the app directory, or use the sidecar plugin option, which processes each application using a temporary copy of the repository.

  * **Multiple Kustomize applications in same repository with [parameter overrides](../user-guide/parameters.md):** sorry, no workaround for now.


### Webhook and Manifest Paths Annotation

Argo CD aggressively caches generated manifests and uses the repository commit SHA as a cache key. A new commit to the Git repository invalidates the cache for all applications configured in the repository.
This can negatively affect repositories with multiple applications. You can use [webhooks](https://github.com/argoproj/argo-cd/blob/master/docs/operator-manual/webhook.md) and the `argocd.argoproj.io/manifest-generate-paths` Application CRD annotation to solve this problem and improve performance.

The `argocd.argoproj.io/manifest-generate-paths` annotation contains a semicolon-separated list of paths within the Git repository that are used during manifest generation. The webhook compares paths specified in the annotation with the changed files specified in the webhook payload. If no modified files match the paths specified in `argocd.argoproj.io/manifest-generate-paths`, then the webhook will not trigger application reconciliation and the existing cache will be considered valid for the new commit.

Installations that use a different repository for each application are **not** subject to this behavior and will likely get no benefit from using these annotations.

!!! note
    Application manifest paths annotation support depends on the git provider used for the Application. It is currently only supported for GitHub, GitLab, and Gogs based repos.

* **Relative path** The annotation might contain a relative path. In this case the path is considered relative to the path specified in the application source:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: guestbook
  namespace: argocd
  annotations:
    # resolves to the 'guestbook' directory
    argocd.argoproj.io/manifest-generate-paths: .
spec:
  source:
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    targetRevision: HEAD
    path: guestbook
# ...
```

* **Absolute path** The annotation value might be an absolute path starting with '/'. In this case path is considered as an absolute path within the Git repository:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: guestbook
  annotations:
    argocd.argoproj.io/manifest-generate-paths: /guestbook
spec:
  source:
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    targetRevision: HEAD
    path: guestbook
# ...
```

* **Multiple paths** It is possible to put multiple paths into the annotation. Paths must be separated with a semicolon (`;`):

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: guestbook
  annotations:
    # resolves to 'my-application' and 'shared'
    argocd.argoproj.io/manifest-generate-paths: .;../shared
spec:
  source:
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    targetRevision: HEAD
    path: my-application
# ...
```

### Application Sync Timeout & Jitter

Argo CD has a timeout for application syncs. It will trigger a refresh for each application periodically when the timeout expires.
With a large number of applications, this will cause a spike in the refresh queue and can cause a spike to the repo-server component. To avoid this, you can set a jitter to the sync timeout which will spread out the refreshes and give time to the repo-server to catch up.

The jitter is the maximum duration that can be added to the sync timeout, so if the sync timeout is 5 minutes and the jitter is 1 minute, then the actual timeout will be between 5 and 6 minutes.

To configure the jitter you can set the following environment variables:

* `ARGOCD_RECONCILIATION_JITTER` - The jitter to apply to the sync timeout. Disabled when value is 0. Defaults to 0.

## Rate Limiting Application Reconciliations

To prevent high controller resource usage or sync loops caused either due to misbehaving apps or other environment specific factors,
we can configure rate limits on the workqueues used by the application controller. There are two types of rate limits that can be configured:

  * Global rate limits
  * Per item rate limits

The final rate limiter uses a combination of both and calculates the final backoff as `max(globalBackoff, perItemBackoff)`.

### Global rate limits

  This is enabled by default, it is a simple bucket based rate limiter that limits the number of items that can be queued per second.
This is useful to prevent a large number of apps from being queued at the same time.

To configure the bucket limiter you can set the following environment variables:

  * `WORKQUEUE_BUCKET_SIZE` - The number of items that can be queued in a single burst. Defaults to 500.
  * `WORKQUEUE_BUCKET_QPS` - The number of items that can be queued per second. Defaults to 50.

### Per item rate limits

  This by default returns a fixed base delay/backoff value but can be configured to return exponential values.
Per item rate limiter limits the number of times a particular item can be queued. This is based on exponential backoff where the backoff time for an item keeps increasing exponentially
if it is queued multiple times in a short period, but the backoff is reset automatically if a configured `cool down` period has elapsed since the last time the item was queued.

To configure the per item limiter you can set the following environment variables:

  * `WORKQUEUE_FAILURE_COOLDOWN_NS` : The cool down period in nanoseconds, once period has elapsed for an item the backoff is reset. Exponential backoff is disabled if set to 0(default), eg. values : 10 * 10^9 (=10s)
  * `WORKQUEUE_BASE_DELAY_NS` : The base delay in nanoseconds, this is the initial backoff used in the exponential backoff formula. Defaults to 1000 (=1μs)
  * `WORKQUEUE_MAX_DELAY_NS` : The max delay in nanoseconds, this is the max backoff limit. Defaults to 3 * 10^9 (=3s)
  * `WORKQUEUE_BACKOFF_FACTOR` : The backoff factor, this is the factor by which the backoff is increased for each retry. Defaults to 1.5

The formula used to calculate the backoff time for an item, where `numRequeue` is the number of times the item has been queued
and `lastRequeueTime` is the time at which the item was last queued:

- When `WORKQUEUE_FAILURE_COOLDOWN_NS` != 0 :

```
backoff = time.Since(lastRequeueTime) >= WORKQUEUE_FAILURE_COOLDOWN_NS ?
          WORKQUEUE_BASE_DELAY_NS :
          min(
              WORKQUEUE_MAX_DELAY_NS,
              WORKQUEUE_BASE_DELAY_NS * WORKQUEUE_BACKOFF_FACTOR ^ (numRequeue)
              )
```

- When `WORKQUEUE_FAILURE_COOLDOWN_NS` = 0 :

```
backoff = WORKQUEUE_BASE_DELAY_NS
```

## HTTP Request Retry Strategy

In scenarios where network instability or transient server errors occur, the retry strategy ensures the robustness of HTTP communication by automatically resending failed requests. It uses a combination of maximum retries and backoff intervals to prevent overwhelming the server or thrashing the network.

### Configuring Retries

The retry logic can be fine-tuned with the following environment variables:

* `ARGOCD_K8SCLIENT_RETRY_MAX` - The maximum number of retries for each request. The request will be dropped after this count is reached. Defaults to 0 (no retries).
* `ARGOCD_K8SCLIENT_RETRY_BASE_BACKOFF` - The initial backoff delay on the first retry attempt in ms. Subsequent retries will double this backoff time up to a maximum threshold. Defaults to 100ms.

### Backoff Strategy

The backoff strategy employed is a simple exponential backoff without jitter. The backoff time increases exponentially with each retry attempt until a maximum backoff duration is reached.

The formula for calculating the backoff time is:

```
backoff = min(retryWaitMax, baseRetryBackoff * (2 ^ retryAttempt))
```
Where `retryAttempt` starts at 0 and increments by 1 for each subsequent retry.

### Maximum Wait Time

There is a cap on the backoff time to prevent excessive wait times between retries. This cap is defined by:

`retryWaitMax` - The maximum duration to wait before retrying. This ensures that retries happen within a reasonable timeframe. Defaults to 10 seconds.

### Non-Retriable Conditions

Not all HTTP responses are eligible for retries. The following conditions will not trigger a retry:

* Responses with a status code indicating client errors (4xx) except for 429 Too Many Requests.
* Responses with the status code 501 Not Implemented.
