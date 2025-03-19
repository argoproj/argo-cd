# Server-Side Apply Dynamic Field Managers

This proposal is a follow up on the [original Server-Side Apply proposal](./server-side-apply.md), and seeks to make Argo more flexible and give users the ability to apply changes to shared Kuberentes resources at a granular level. It also considers how to remove field-level changes when an Application is destroyed.

To quote the [Kubernetes docs][1]:

> Server-Side Apply helps users and controllers manage their resources through declarative configuration. Clients can create and modify objects declaratively by submitting their _fully specified intent_.

> A fully specified intent is a partial object that only includes the fields and values for which the user has an opinion. That intent either creates a new object (using default values for unspecified fields), or is combined, by the API server, with the existing object.

## Example

Consider the case where you're working on a new feature for your app. You'd like to review these changes without disturbing your staging environment, so you use an `ApplicationSet` with a pull request generator to create [review apps][3] dynamically. These review apps configure a central Consul `IngressGateway`. Each review app needs to add a service to the `IngressGateway` upon creation, and remove that service upon deletion.

```yaml
apiVersion: consul.hashicorp.com/v1alpha1
kind: IngressGateway
metadata:
  name: review-app-ingress-gateway
  namespace: default
spec:
  listeners:
  - port: 443
    protocol: http
    services:
    - name: review-app-1
      namespace: my-cool-app
    - name: review-app-2
      namespace: my-groovy-app
    - name: review-app-3
      namespace: my-incredible-app
```

---

## Open Questions

### [Q-1] What should the behavior be for a server-side applied resource upon Application deletion?

The current behavior is to delete all objects "owned" by the Application. A user could choose to leave the resource behind with `Prune=false`, but that would also leave behind any config added to the shared object. Should the default delete behavior for server-side applied resources be to remove any fields that match that Application's [field manager][2]?

### [Q-2] What sync status should the UI display on a shared resource?

If an Application has successfully applied its partial spec to a shared resource, should it display as "in sync"? Or should it show "out of sync" when there are other changes to the shared object?

## Summary

ArgoCD supports server-side apply, but it uses the same field manager, `argocd-controller`, no matter what Application is issuing the sync. Setting a unique field manager per application would enable users to:

- Manage only specific fields they care about on a shared resource
- Avoid deleting or overwriting fields that are managed by other Applications

## Motivation

There exist situations where disparate Applications need to add or remove configuration from a shared Kubernetes resource. Server-side apply supports this behavior when different field managers are used.

## Goals

All following goals should be achieve in order to conclude this proposal:

#### [G-1] Applications can apply changes to a shared resource without disturbing existing fields

A common use case of server-side apply is the ability to manage only particular fields on a share Kubernetes resource, while leaving everything else in tact. This requires a unique field manager for each identity that shares the resource.

#### [G-2] Applications that are destroyed only remove the fields they manage from shared resources

A delete operation should undo only the additions or updates it made to a shared resource, unless that resource has `Prune=true`.

#### [G-3] Resource tracking logic for labels and annotations support multiple Applications

Multiple Applications can now own a shared resource. The resource tracking annotations should be able to reflect this shared ownership.

## Non-Goals

1. We don't intend to give users control of this field manager. Letting users change the field manager could lead to situations where fields get left behind on shared objects.

2. We will only support this feature when using annotation resource tracking. Label's 63 character limit will quickly prove too small for multiple Applications.

## Proposal

1. Each Application uses a field manager that is unique to itself and chosen deterministicly. It never changes for the lifetime of the Application.

1. Change the removal behavior for shared resources. When a resource with a custom field manager is "removed", it instead removes only the fields managed by its field manager from the shared resource by sending an empty "fully specified intent" using server-side apply. You can fully delete a shared resource by setting `Prune=true` at the resource level. [Demo of this behavior](#removal-demo).

1. Add documentation suggesting that users might want to consider changing the permissions on the ArgoCD role to disallow the `delete` and `update` verbs on shared resources. Server-side apply will always use `patch`, and removing `delete` and `update` helps prevent users from errantly wiping out changes made from other Applications.

1. Add documentation that references the Kubernetes docs to show users how to properly define their Custom Resource Definition so that patches merge with expected results. For example, should a particular array on the resource be replaced or added to when patching?

### Use cases

The following use cases should be implemented:

#### [UC-1]: As a user, I would like to manage specific fields on a Kubernetes object shared by other ArgoCD Applications.

Change the Server-Side Apply field manager to be unique and deterministic for each Application.

#### [UC-2]: As a user, I want the transition to unqiue field managers to be transparent when upgrading ArgoCD

Users shouldn't notice the switch to unique field managers in their Kubernetes manifests.

#### [UC-3]: As a user running an Application with a shared resource, I only want to remove the fields my Application owns from the shared resource when I delete my Application.

Change the delete behavior to server-side apply an empty "fully specified intent" instead of deleting a shared resource.

### Security Considerations

TBD

### Risks and Mitigations

#### [R-1] Field manager names are limited to 128 characters

We should trim every field manager to 128 characters.

### Upgrade / Downgrade

- Because ArgoCD uses `--force-conflicts` when issuing a server-side apply, the new field manager will transparently assume ownership of the fields from the existing field manager `argocd-controller`.

## Drawbacks

TBD

## Demos

### Removal Demo

```bash
#!/usr/bin/env bash

# Create a temporary cluster
# https://github.com/kubernetes-sigs/kind/
kind create cluster

# Create the shared resource
cat <<EOF > /tmp/base-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  annotations:
    foo: bar
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:latest
        ports:
        - containerPort: 80
EOF

kubectl --context kind-kind apply -f /tmp/base-deployment.yaml --server-side --field-manager base
kubectl --context kind-kind get deployment nginx -oyaml

# Another app adds an annotation
cat <<EOF > /tmp/app1-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  annotations:
    asdf: qwerty
EOF

kubectl --context kind-kind apply -f /tmp/app1-deployment.yaml --server-side --field-manager app1
AFTER_APP1_APPLY=$(kubectl --context kind-kind get deployment nginx -oyaml)

# The same app removes only its anntoation 
cat <<EOF > /tmp/app1-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
EOF

kubectl --context kind-kind apply -f /tmp/app1-deployment.yaml --server-side --field-manager app1
AFTER_APP1_REMOVAL=$(kubectl --context kind-kind get deployment nginx -oyaml)

# Diff before and after removal of a field on a shared resource
# https://github.com/dandavison/delta
delta -s <(echo "$AFTER_APP1_APPLY") <(echo "$AFTER_APP1_REMOVAL")

kind delete cluster
```

### Swapping Field Manager Demo

```bash
#!/usr/bin/env bash

# Create kind cluster
kind create cluster

# Create service in App1
cat <<EOF > /tmp/app1-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: nginx-service
spec:
  selector:
    app: nginx
  ports:
    - protocol: TCP
      port: 80
      targetPort: 8080
EOF

# Create service in App2
cat <<EOF > /tmp/app2-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: nginx-service
spec:
  ports:
    - protocol: TCP
      name: foo
      port: 1000
      targetPort: 8080
EOF

# Sync App1 with argocd-controller
kubectl --context kind-kind apply -f /tmp/app1-service.yaml --force-conflicts --server-side --field-manager argocd-controller

# Sync App2 with another-manager
kubectl --context kind-kind apply -f /tmp/app2-service.yaml --force-conflicts --server-side --field-manager another-manager

# Modify App2 service
cat <<EOF > /tmp/app2-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: nginx-service
spec:
  ports:
    - protocol: TCP
      name: bar
      port: 2000
      targetPort: 8080
EOF

# Sync App2 with argocd-controller
kubectl --context kind-kind apply -f /tmp/app2-service.yaml --force-conflicts --server-side --field-manager argocd-controller

# Sync App2 with argocd-controller
kubectl --context kind-kind apply -f /tmp/app2-service.yaml --force-conflicts --server-side --field-manager argocd-controller

# Modify App2 service
cat <<EOF > /tmp/app2-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: nginx-service
spec:
  type: LoadBalancer
  ports:
    - protocol: TCP
      name: buzz
      port: 3000
      targetPort: 8080
EOF

# Sync App2 with another-manager
kubectl --context kind-kind apply -f /tmp/app2-service.yaml --force-conflicts --server-side --field-manager another-manager

# Get the service. Notice that the selector is missing.
kubectl --context kind-kind get service nginx-service -oyaml

# Destroy kind cluster
kind delete cluster
```

[1]: https://kubernetes.io/docs/reference/using-api/server-side-apply/
[2]: https://kubernetes.io/docs/reference/using-api/server-side-apply/#managers
[3]: https://docs.gitlab.com/ee/ci/review_apps/
