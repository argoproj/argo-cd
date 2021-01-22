# High Availability

Argo CD is largely stateless, all data is persisted as Kubernetes objects, which in turn is stored in Kubernetes' etcd. Redis is only used as a throw-away cache and can be lost. When lost, it will be rebuilt without loss of service.

A set HA of manifests are provided for users who wish to run Argo CD in a highly available manner. This runs more containers, and run Redis in HA mode.

[Manifests ⧉](https://github.com/argoproj/argo-cd/tree/master/manifests) 

!!! note
    The HA installation will require at least three different nodes due to pod anti-affinity roles in the specs.

## Scaling Up

### argocd-repo-server

**settings:**

The `argocd-repo-server` is responsible for cloning Git repository, keeping it up to date and generating manifests using the appropriate tool.

* `argocd-repo-server` fork/exec config management tool to generate manifests. The fork can fail due to lack of memory and limit on the number of OS threads.
The `--parallelismlimit` flag controls how many manifests generations are running concurrently and allows avoiding OOM kills.

* the `argocd-repo-server` ensures that repository is in the clean state during the manifest generation using config management tools such as Kustomize, Helm
or custom plugin. As a result Git repositories with multiple applications might be affect repository server performance.
Read [Monorepo Scaling Considerations](#monorepo-scaling-considerations) for more information.

* `argocd-repo-server` clones repository into `/tmp` ( of path specified in `TMPDIR` env variable ). Pod might run out of disk space if have too many repository
or repositories has a lot of files. To avoid this problem mount persistent volume.

* `argocd-repo-server` `git ls-remote` to resolve ambiguous revision such as `HEAD`, branch or tag name. This operation is happening pretty frequently
and might fail. To avoid failed syncs use `ARGOCD_GIT_ATTEMPTS_COUNT` environment variable to retry failed requests.

* `argocd-repo-server` Every 3m (by default) Argo CD checks for changes to the app manifests. Argo CD assumes by default that manifests only change when the repo changes, so it caches generated manifests (for 24h by default). With Kustomize remote bases, or Helm patch releases, the manifests can change even though the repo has not changed. By reducing the cache time, you can get the changes without waiting for 24h. Use `--repo-cache-expiration duration`, and we'd suggest in low volume environments you try '1h'. Bear in mind this will negate the benefit of caching if set too low. 

* `argocd-repo-server` fork exec config management tools such as `helm` or `kustomize` and enforces 90 seconds timeout. The timeout can be increased using `ARGOCD_EXEC_TIMEOUT` env variable.

**metrics:**

* `argocd_git_request_total` - Number of git requests. The metric provides two tags: `repo` - Git repo URL; `request_type` - `ls-remote` or `fetch`.

* `ARGOCD_ENABLE_GRPC_TIME_HISTOGRAM` (v1.8+) - environment variable that enables collecting RPC performance metrics. Enable it if you need to troubleshoot performance issue. Note: metric is expensive to both query and store!

### argocd-application-controller

**settings:**

The `argocd-application-controller` uses `argocd-repo-server` to get generated manifests and Kubernetes API server to get actual cluster state.

* each controller replica uses two separate queues to process application reconciliation (milliseconds) and app syncing (seconds). Number of queue processors for each queue is controlled by
`--status-processors` (20 by default) and `--operation-processors` (10 by default) flags. Increase number of processors if your Argo CD instance manages too many applications.
For 1000 application we use 50 for `--status-processors` and 25 for `--operation-processors`

* The manifest generation typically takes the most time during reconciliation. The duration of manifest generation is limited to make sure controller refresh queue does not overflow.
The app reconciliation fails with `Context deadline exceeded` error if manifest generating taking too much time. As workaround increase value of `--repo-server-timeout-seconds` and
consider scaling up `argocd-repo-server` deployment.

* The controller uses `kubectl` fork/exec to push changes into the cluster and to convert resource from preferred version into user specified version
(e.g. Deployment `apps/v1` into `extensions/v1beta1`). Same as config management tool `kubectl` fork/exec might cause pod OOM kill. Use `--kubectl-parallelism-limit` flag to limit
number of allowed concurrent kubectl fork/execs.

* The controller uses Kubernetes watch APIs to maintain lightweight Kubernetes cluster cache. This allows to avoid querying Kubernetes during app reconciliation and significantly improve
performance. For performance reasons controller monitors and caches only preferred the version of a resource. During reconciliation, the controller might have to convert cached resource from
preferred version into a version of the resource stored in Git. If `kubectl convert` fails because conversion is not supported than controller fallback to Kubernetes API query which slows down
reconciliation. In this case advice user-preferred resource version in Git.

* The controller polls Git every 3m by default. You can increase this duration using `--app-resync seconds` to reduce polling.

* If the controller is managing too many clusters and uses too much memory then you can shard clusters across multiple
controller replicas. To enable sharding increase the number of replicas in `argocd-application-controller` `StatefulSet`
and repeat number of replicas in `ARGOCD_CONTROLLER_REPLICAS` environment variable. The strategic merge patch below
demonstrates changes required to configure two controller replicas.

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

* `ARGOCD_ENABLE_GRPC_TIME_HISTOGRAM`  (v1.8+)- environment variable that enables collecting RPC performance metrics. Enable it if you need to troubleshoot performance issue. Note: metric is expensive to both query and store!

**metrics**

* `argocd_app_reconcile` - reports application reconciliation duration. Can be used to build reconciliation duration heat map to get high-level reconciliation performance picture.
* `argocd_app_k8s_request_total` - number of k8s requests per application. The number of fallback Kubernetes API queries - useful to identify which application has a resource with
non-preferred version and causes performance issues.

### argocd-server

The `argocd-server` is stateless and probably least likely to cause issues. You might consider increasing number of replicas to 3 or more to ensure there is no downtime during upgrades.

**settings:**

* The `ARGOCD_GRPC_MAX_SIZE_MB` environment variable allows specifying the max size of the server response message in megabytes.
The default value is 200. You might need to increase for an Argo CD instance that manages 3000+ applications.    

### argocd-dex-server, argocd-redis

The `argocd-dex-server` uses an in-memory database, and two or more instances would have inconsistent data. `argocd-redis` is pre-configured with the understanding of only three total redis servers/sentinels.

## Monorepo Scaling Considerations

Argo CD repo server maintains one repository clone locally and use it for application manifest generation. If the manifest generation requires to change a file in the local repository clone then only one concurrent manifest generation per server instance is allowed. This limitation might significantly slowdown Argo CD if you have a mono repository with multiple applications (50+).

### Enable Concurrent Processing

Argo CD determines if manifest generation might change local files in the local repository clone based on config management tool and application settings.
If the manifest generation has no side effects then requests are processed in parallel without the performance penalty. Following are known cases that might cause slowness and workarounds:

  * **Multiple Helm based applications pointing to the same directory in one Git repository:** ensure that your Helm chart don't have conditional
[dependencies](https://helm.sh/docs/chart_best_practices/dependencies/#conditions-and-tags) and create `.argocd-allow-concurrency` file in chart directory.

  * **Multiple Custom plugin based applications:** avoid creating temporal files during manifest generation and and create `.argocd-allow-concurrency` file in app directory.

  * **Multiple Kustomize or Ksonnet applications in same repository with [parameter overrides](../user-guide/parameters.md):** sorry, no workaround for now.


### Webhook and Manifest Paths Annotation

Argo CD aggressively caches generated manifests and uses repository commit SHA as a cache key. A new commit to the Git repository invalidates cache for all applications configured in the repository
that again negatively affect mono repositories with multiple applications. You might use [webhooks ⧉](https://github.com/argoproj/argo-cd/tree/master/docs/operator-manual/webhook) and `argocd.argoproj.io/manifest-generate-paths` Application
CRD annotation to solve this problem and improve performance.

The `argocd.argoproj.io/manifest-generate-paths` contains a semicolon-separated list of paths within the Git repository that are used during manifest generation. The webhook compares paths specified in the annotation
with the changed files specified in the webhook payload. If non of the changed files are located in the paths then webhook don't trigger application reconciliation and re-uses previously generated manifests cache for a new commit.

Installations that use a different repo for each app are **not** subject to this behavior and will likely get no benefit from using these annotations.

!!! note
    Installations with a large number of apps should also set the `--app-resync` flag in the `argocd-application-controller` process to a larger value to reduce automatic refreshes based on git polling. The exact value is a trade-off between reduced work and app sync in case of a missed webhook event. For most cases `1800` (30m) or `3600` (1h) is a good trade-off.


!!! note
    Application manifest paths annotation support depends on the git provider used for the Application. It is currently only supported for GitHub, GitLab, and Gogs based repos

* **Relative path** The annotation might contains relative path. In this case the path is considered relative to the path specified in the application source:

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
* **Absolute path** The annotation value might be an absolute path started from '/'. In this case path is considered as an absolute path within the Git repository:

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
