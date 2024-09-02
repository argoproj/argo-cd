# Applications in any namespace

!!! warning
    Please read this documentation carefully before you enable this feature. Misconfiguration could lead to potential security issues.

## Introduction

As of version 2.5, Argo CD supports managing `Application` resources in namespaces other than the control plane's namespace (which is usually `argocd`), but this feature has to be explicitly enabled and configured appropriately.

Argo CD administrators can define a certain set of namespaces where `Application` resources may be created, updated and reconciled in. However, applications in these additional namespaces will only be allowed to use certain `AppProjects`, as configured by the Argo CD administrators. This allows ordinary Argo CD users (e.g. application teams) to use patterns like declarative management of `Application` resources, implementing app-of-apps and others without the risk of a privilege escalation through usage of other `AppProjects` that would exceed the permissions granted to the application teams.

Some manual steps will need to be performed by the Argo CD administrator in order to enable this feature. 

One additional advantage of adopting applications in any namespace is to allow end-users to configure notifications for their Argo CD application in the namespace where Argo CD application is running in. See notifications [namespace based configuration](notifications/index.md#namespace-based-configuration) page for more information.

## Prerequisites

### Cluster-scoped Argo CD installation

This feature can only be enabled and used when your Argo CD is installed as a cluster-wide instance, so it has permissions to list and manipulate resources on a cluster scope. It will not work with an Argo CD installed in namespace-scoped mode.

### Switch resource tracking method

Also, while technically not necessary, it is strongly suggested that you switch the application tracking method from the default `label` setting to either `annotation` or `annotation+label`. The reasoning for this is, that application names will be a composite of the namespace's name and the name of the `Application`, and this can easily exceed the 63 characters length limit imposed on label values. Annotations have a notably greater length limit.

To enable annotation based resource tracking, refer to the documentation about [resource tracking methods](../../user-guide/resource_tracking/)

## Implementation details

### Overview

In order for an application to be managed and reconciled outside the Argo CD's control plane namespace, two prerequisites must match:

1. The `Application`'s namespace must be explicitly enabled using the `--application-namespaces` parameter for the `argocd-application-controller` and `argocd-server` workloads. This parameter controls the list of namespaces that Argo CD will be allowed to source `Application` resources from globally. Any namespace not configured here cannot be used from any `AppProject`.
1. The `AppProject` referenced by the `.spec.project` field of the `Application` must have the namespace listed in its `.spec.sourceNamespaces` field. This setting will determine whether an `Application` may use a certain `AppProject`. If an `Application` specifies an `AppProject` that is not allowed, Argo CD refuses to process this `Application`. As stated above, any namespace configured in the `.spec.sourceNamespaces` field must also be enabled globally.

`Applications` in different namespaces can be created and managed just like any other `Application` in the `argocd` namespace previously, either declaratively or through the Argo CD API (e.g. using the CLI, the web UI, the REST API, etc).

### Reconfigure Argo CD to allow certain namespaces

#### Change workload startup parameters

In order to enable this feature, the Argo CD administrator must reconfigure the `argocd-server` and `argocd-application-controller` workloads to add the `--application-namespaces` parameter to the container's startup command.

The `--application-namespaces` parameter takes a comma-separated list of namespaces where `Applications` are to be allowed in. Each entry of the list supports:

- shell-style wildcards such as `*`, so for example the entry `app-team-*` would match `app-team-one` and `app-team-two`. To enable all namespaces on the cluster where Argo CD is running on, you can just specify `*`, i.e. `--application-namespaces=*`.
- regex, requires wrapping the string in ```/```, example to allow all namespaces except a particular one: ```/^((?!not-allowed).)*$/```.
  
The startup parameters for both, the `argocd-server` and the `argocd-application-controller` can also be conveniently set up and kept in sync by specifying the `application.namespaces` settings in the `argocd-cmd-params-cm` ConfigMap _instead_ of changing the manifests for the respective workloads. For example:

```yaml
data:
  application.namespaces: app-team-one, app-team-two
```

would allow the `app-team-one` and `app-team-two` namespaces for managing `Application` resources. After a change to the `argocd-cmd-params-cm` namespace, the appropriate workloads need to be restarted:

```bash
kubectl rollout restart -n argocd deployment argocd-server
kubectl rollout restart -n argocd statefulset argocd-application-controller
```

#### Adapt Kubernetes RBAC

We decided to not extend the Kubernetes RBAC for the `argocd-server` workload by default for the time being. If you want `Applications` in other namespaces to be managed by the Argo CD API (i.e. the CLI and UI), you need to extend the Kubernetes permissions for the `argocd-server` ServiceAccount.

We supply a `ClusterRole` and `ClusterRoleBinding` suitable for this purpose in the `examples/k8s-rbac/argocd-server-applications` directory. For a default Argo CD installation (i.e. installed to the `argocd` namespace), you can just apply them as-is:

```shell
kubectl apply -k examples/k8s-rbac/argocd-server-applications/
```

`argocd-notifications-controller-rbac-clusterrole.yaml` and `argocd-notifications-controller-rbac-clusterrolebinding.yaml` are used to support notifications controller to notify apps in all namespaces.

!!! note
    At some later point in time, we may make this cluster role part of the default installation manifests.

### Allowing additional namespaces in an AppProject

Any user with Kubernetes access to the Argo CD control plane's namespace (`argocd`), especially those with permissions to create or update `Applications` in a declarative way, is to be considered an Argo CD admin.

This prevented unprivileged Argo CD users from declaratively creating or managing `Applications` in the past. Those users were constrained to using the API instead, subject to Argo CD RBAC which ensures only `Applications` in allowed `AppProjects` were created.

For an `Application` to be created outside the `argocd` namespace, the `AppProject` referred to in the `Application`'s `.spec.project` field must include the `Application`'s namespace in its `.spec.sourceNamespaces` field.

For example, consider the two following (incomplete) `AppProject` specs:

```yaml
kind: AppProject
apiVersion: argoproj.io/v1alpha1
metadata:
  name: project-one
  namespace: argocd
spec:
  sourceNamespaces:
  - namespace-one
```

and

```yaml
kind: AppProject
apiVersion: argoproj.io/v1alpha1
metadata:
  name: project-two
  namespace: argocd
spec:
  sourceNamespaces:
  - namespace-two
```

In order for an Application to set `.spec.project` to `project-one`, it would have to be created in either namespace `namespace-one` or `argocd`. Likewise, in order for an Application to set `.spec.project` to `project-two`, it would have to be created in either namespace `namespace-two` or `argocd`.

If an Application in `namespace-two` would set their `.spec.project` to `project-one` or an Application in `namespace-one` would set their `.spec.project` to `project-two`, Argo CD would consider this as a permission violation and refuse to reconcile the Application.

Also, the Argo CD API will enforce these constraints, regardless of the Argo CD RBAC permissions.

The `.spec.sourceNamespaces` field of the `AppProject` is a list that can contain an arbitrary amount of namespaces, and each entry supports shell-style wildcard, so that you can allow namespaces with patterns like `team-one-*`.

!!! warning
    Do not add user controlled namespaces in the `.spec.sourceNamespaces` field of any privileged AppProject like the `default` project. Always make sure that the AppProject follows the principle of granting least required privileges. Never grant access to the `argocd` namespace within the AppProject.

!!! note
    For backwards compatibility, Applications in the Argo CD control plane's namespace (`argocd`) are allowed to set their `.spec.project` field to reference any AppProject, regardless of the restrictions placed by the AppProject's `.spec.sourceNamespaces` field.
  
### Application names

For the CLI and UI, applications are now referred to and displayed as in the format `<namespace>/<name>`. 

For backwards compatibility, if the namespace of the Application is the control plane's namespace (i.e. `argocd`), the `<namespace>` can be omitted from the application name when referring to it. For example, the application names `argocd/someapp` and `someapp` are semantically the same and refer to the same application in the CLI and the UI.

### Application RBAC

The RBAC syntax for Application objects has been changed from `<project>/<application>` to `<project>/<namespace>/<application>` to accommodate the need to restrict access based on the source namespace of the Application to be managed.

For backwards compatibility, Applications in the `argocd` namespace can still be refered to as `<project>/<application>` in the RBAC policy rules.

Wildcards do not make any distinction between project and application namespaces yet. For example, the following RBAC rule would match any application belonging to project `foo`, regardless of the namespace it is created in:

```
p, somerole, applications, get, foo/*, allow
```

If you want to restrict access to be granted only to `Applications` in project `foo` within namespace `bar`, the rule would need to be adapted as follows:

```
p, somerole, applications, get, foo/bar/*, allow
```
  
## Managing applications in other namespaces

### Declaratively

For declarative management of Applications, just create the Application from a YAML or JSON manifest in the desired namespace. Make sure that the `.spec.project` field refers to an AppProject that allows this namespace. For example, the following (incomplete) Application manifest creates an Application in the namespace `some-namespace`:

```yaml
kind: Application
apiVersion: argoproj.io/v1alpha1
metadata:
  name: some-app
  namespace: some-namespace
spec:
  project: some-project
  # ...
```

The project `some-project` will then need to specify `some-namespace` in the list of allowed source namespaces, e.g.

```yaml
kind: AppProject
apiVersion: argoproj.io/v1alpha1
metadata:
    name: some-project
    namespace: argocd
spec:
    sourceNamespaces:
    - some-namespace
```

### Using the CLI

You can use all existing Argo CD CLI commands for managing applications in other namespaces, exactly as you would use the CLI to manage applications in the control plane's namespace.

For example, to retrieve the `Application` named `foo` in the namespace `bar`, you can use the following CLI command:

```shell
argocd app get foo/bar
```

Likewise, to manage this application, keep referring to it as `foo/bar`:

```bash
# Create an application
argocd app create foo/bar ...
# Sync the application
argocd app sync foo/bar
# Delete the application
argocd app delete foo/bar
# Retrieve application's manifest
argocd app manifests foo/bar
```

As stated previously, for applications in the Argo CD's control plane namespace, you can omit the namespace from the application name.

### Using the UI

Similar to the CLI, you can refer to the application in the UI as `foo/bar`.

For example, to create an application named `bar` in the namespace `foo` in the web UI, set the application name in the creation dialogue's _Application Name_ field to `foo/bar`. If the namespace is omitted, the control plane's namespace will be used.

### Using the REST API

If you are using the REST API, the namespace for `Application` cannot be specified as the application name, and resources need to be specified using the optional `appNamespace` query parameter. For example, to work with the `Application` resource named `foo` in the namespace `bar`, the request would look like follows:

```bash
GET /api/v1/applications/foo?appNamespace=bar
```

For other operations such as `POST` and `PUT`, the `appNamespace` parameter must be part of the request's payload.

For `Application` resources in the control plane namespace, this parameter can be omitted.
