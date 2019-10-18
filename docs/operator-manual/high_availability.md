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

* one instance of `argocd-repo-server` executes only one operation on one Git repo concurrently. Increase the number of `argocd-repo-server` replica count if you have a lot of
applications in the same repository.

* `argocd-repo-server` clones repository into `/tmp` ( of path specified in `TMPDIR` env variable ). Pod might run out of disk space if have too many repository
or repositories has a lot of files. To avoid this problem mount persistent volume.

* `argocd-repo-server` `git ls-remote` to resolve ambiguous revision such as `HEAD`, branch or tag name. This operation is happening pretty frequently
and might fail. To avoid failed syncs use `ARGOCD_GIT_ATTEMPTS_COUNT` environment variable to retry failed requests.

* `argocd-repo-server` Every 3m (by default) Argo CD checks for changes to the app manifests. Argo CD assumes by default that manifests only change when the repo changes, so it caches generated manifests (for 24h by default). With Kustomize remote bases, or Helm patch releases, the manifests can change even though the repo has not changed. By reducing the cache time, you can get the changes without waiting for 24h. Use `--repo-cache-expiration duration`, and we'd suggest in low volume environments you try '1h'. Bear in mind this will negate the benefit of caching if set too low. 

**metrics:**

* `argocd_git_request_total` - Number of git requests. The metric provides two tags: `repo` - Git repo URL; `request_type` - `ls-remote` or `fetch`.

### argocd-application-controller

**settings:**

The `argocd-application-controller` uses `argocd-repo-server` to get generated manifests and Kubernetes API server to get actual cluster state.

* controller uses two separate queues to process application reconciliation (milliseconds) and app syncing (seconds). Number of queue processors for each queue is controlled by
`--status-processors` (20 by default) and `--operation-processors` (10 by default) flags. Increase number of processors if your Argo CD instance manages too many applications.
For 1000 application we use 50 for `--status-processors` and 25 for `--operation-processors`

* The manifest generation typically takes the most time during reconciliation. The duration of manifest generation is limited to make sure controller refresh queue does not overflow.
The app reconciliation fails with `Context deadline exceeded` error if manifest generating taking too much time. As workaround increase value of `--repo-server-timeout-seconds` and
consider scaling up `argocd-repo-server` deployment.

* controller uses `kubectl` fork/exec to push changes into the cluster and to convert resource from preferred version into user specified version
(e.g. Deployment `apps/v1` into `extensions/v1beta1`). Same as config management tool `kubectl` fork/exec might cause pod OOM kill. Use `--kubectl-parallelism-limit` flag to limit
number of allowed concurrent kubectl fork/execs.

* controller uses Kubernetes watch APIs to maintain lightweight Kubernetes cluster cache. This allows to avoid querying Kubernetes during app reconciliation and significantly improve
performance. For performance reasons controller monitors and caches only preferred the version of a resource. During reconciliation, the controller might have to convert cached resource from
preferred version into a version of the resource stored in Git. If `kubectl convert` fails because conversion is not supported than controller fallback to Kubernetes API query which slows down
reconciliation. In this case advice user-preferred resource version in Git.

* The controller polls Git every 3m by default. You can increase this duration using `--app-resync seconds` to reduce polling.

**metrics**

* `argocd_app_reconcile` - reports application reconciliation duration. Can be used to build reconciliation duration heat map to get high-level reconciliation performance picture.
* `argocd_app_k8s_request_total` - number of k8s requests per application. The number of fallback Kubernetes API queries - useful to identify which application has a resource with
non-preferred version and causes performance issues.

### argocd-server

The `argocd-server` is stateless and probably least likely to cause issues. You might consider increasing number of replicas to 3 or more to ensure there is no downtime during upgrades.

### argocd-dex-server, argocd-redis

The `argocd-dex-server` uses an in-memory database, and two or more instances would have inconsistent data. `argocd-redis` is pre-configured with the understanding of only three total redis servers/sentinels.
