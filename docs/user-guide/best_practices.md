# Best Practices

## Separating Config Vs. Source Code Repositories

Using a separate Git repository to hold your kubernetes manifests, keeping the config separate
from your application source code, is highly recommended for the following reasons:

1. It provides a clean separation of application code vs. application config. There will be times
   when you wish to modify just the manifests without triggering an entire CI build. For example,
   you likely do _not_ want to trigger a build if you simply wish to bump the number of replicas in
   a Deployment spec.

2. Cleaner audit log. For auditing purposes, a repo which only holds configuration will have a much
   cleaner Git history of what changes were made, without the noise coming from check-ins due to
   normal development activity.

3. Your application may be comprised of services built from multiple Git repositories, but is
   deployed as a single unit. Oftentimes, microservices applications are comprised of services
   with different versioning schemes, and release cycles (e.g. ELK, Kafka + Zookeeper). It may not
   make sense to store the manifests in one of the source code repositories of a single component.

4. Separation of access. The developers who are developing the application, may not necessarily be 
   the same people who can/should push to production environments, either intentionally or
   unintentionally. By having separate repos, commit access can be given to the source code repo,
   and not the application config repo.

5. If you are automating your CI pipeline, pushing manifest changes to the same Git repository can
   trigger an infinite loop of build jobs and Git commit triggers. Having a separate repo to push
   config changes to, prevents this from happening.


## Leaving Room For Imperativeness

It may be desired to leave room for some imperativeness/automation, and not have everything defined
in your Git manifests. For example, if you want the number of your deployment's replicas to be
managed by [Horizontal Pod Autoscaler](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/),
then you would not want to track `replicas` in Git.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
spec:
  # do not include replicas in the manifests if you want replicas to be controlled by HPA
  # replicas: 1
  template:
    spec:
      containers:
      - image: nginx:1.7.9
        name: nginx
        ports:
        - containerPort: 80
...
```

## Ensuring Manifests At Git Revisions Are Truly Immutable

When using templating tools like `helm` or `kustomize`, it is possible for manifests to change
their meaning from one day to the next. This is typically caused by changes made to an upstream helm
repository or kustomize base.

For example, consider the following kustomization.yaml

```yaml
bases:
- github.com/argoproj/argo-cd//manifests/cluster-install
```

The above kustomization has a remote base to the HEAD revision of the argo-cd repo. Since this
is not a stable target, the manifests for this kustomize application can suddenly change meaning, even without
any changes to your own Git repository.

A better version would be to use a Git tag or commit SHA. For example:

```yaml
bases:
- github.com/argoproj/argo-cd//manifests/cluster-install?ref=v0.11.1
```
