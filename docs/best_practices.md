# Best Practices

## Separating config vs. source code repositories

Using a separate git repository to hold your kubernetes manifests, keeping the config separate
from your application source code, is highly recommended for the following reasons:

1. It provides a clean separation of application code vs. application config. There will be times
   when you wish to change over and not other. For example, you likely do _not_ want to trigger
   a build if you are updating an annotation in a spec.

2. Cleaner audit log. For auditing purposes, a repo which only holds configuration will have a much
   cleaner git history of what changes were made, without the noise stemming from check-ins of
    normal development activity.

2. Your application may be comprised of services built from multiple git repositories, but is
   deployed as a single unit. Often times, microservices applications are comprised of services
   with different versioning schemes, and release cycles (e.g. ELK, Kafka + Zookeeper). It may not
   make sense to store the manifests in one of the source code repositories of a single component.

3. Separate repositories enables separation of access. The person who is developing the app, may
   not necessarily be the same person who can/should affect production environment, either
   intentionally or unintentionally.

4. If you are automating your CI pipeline, pushing manifest changes to the same git repository will
   likely trigger an infinite loop of build jobs and git commit triggers. Pushing config changes to
   a separate repo prevent this from happening.


## Leaving room for imperativeness 

It may be desired to leave room for some imperativeness/automation, and not have everything defined
in your git manifests. For example, if you want the number of your deployment's replicas to be
managed by [Horizontal Pod Autoscaler](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/),
then you would not want to track `replicas` in git.

```
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
