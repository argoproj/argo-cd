---
title: Allow Application resources to exist in any namespace
authors:
  - "@jannfis"
sponsors:
  - TBD
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2021-06-05
last-updated: 2021-06-05
---

# Allow Application resources to exist in any namespace

Improve Argo CDs multi-tenancy model to allow Application CRs to be created
and consumed from different namespaces than the control plane's namespace.

Related issues:
- https://github.com/argoproj/argo-cd/issues/3474

## Open Questions [optional]

* The major open question is, how to name `Application`s in a scenario where
  the K8s resource's name isn't unique anymore.

## Summary

The multi-tenancy model of Argo CD is currently of limited use in a purely
declarative setup when full autonomy of the tenants is desired.
This stems mainly from the fact that the multi-tenancy
model is built around the premise that only a full administrative party has
access to the Argo CD control plane namespace (usually `argocd`), and the
multi-tenancy is enforced through the Argo CD API instead of Kubernetes.

The Argo CD multi-tenancy model is centered around the `AppProject`, which
is used to impose certain rules and limitations to the `Application` that
is associated with the `AppProject`. These limitations e.g. include the
target clusters and namespaces where an `Application` is allowed to sync to,
what kind of resources are allowed to be synced by an `Application` and so
forth.

An `Application` is associated to an `AppProject` by referencing it in the
`.spec.project` field of the `Application` resource. Argo CD has an internal
RBAC model to control the `AppProject` that can be referenced from the
`Application`, but only when created or modified throught Argo CD's API
layer.

Whoever can create or modify `Application` resources in the control-plane
namespace, can effectively circumvent any restrictions that should be
imposed by the `AppProject`, simply by chosing another value for the
`.spec.project` field. So naturally, access to the `argocd` namespace
is considered equal to super user access within Argo CD RBAC model. This
prevents a fully-declarative way in which every party could autonomously
manage their applications using plain Kubernetes mechanisms (e.g. create,
modify and delete applications through K8s API control plane) and also
full GitOps-style management of `Application` resources in a multi-tenant
setup.

## Motivation

The main motivation behind this enhancement proposal is to allow organisations
who whish to set-up Argo CD for multi-tenancy can enable their tenants to fully
self-manage `Application` resources in a declarative way, including being
synced from Git (e.g. via an _app of apps_ pattern).

The owning party could also set-up a dedicated _app of apps_ pattern for their
tenants, e.g.

* Have one _management_ namespace per tenant

* Provide a Git repository to the tenant for managing Argo CD `Application`
  resources

* Create an appropriate `AppProject` per tenant that restricts source to above
  mentioned Git repository, restricts destination to above mentioned namespace
  and restricts allowed resources to be `Application`.

* Create an appropriate `Application` in Argo CD, which uses above mentionend
  Git repository as source, and above mentioned Namespace as destination

### Goals

* Allow reconciliation from Argo CD `Application` resources from any namespace
  in the cluster where the Argo CD control plane is installed to.

* Allow declarative self-service management of `Application` resources from
  users without access to the Argo CD control plane's namespace (e.g. `argocd`)

* Be as less intrusive as possible, while not introducing additional controllers
  or components.

### Non-Goals

* Allow reconciliation from Argo CD `Application` resources that are created in
  remote clusters. We believe this would be possible with some adaptions, but
  for this initial proposal we only consider the control plane's cluster as a
  source for `Application` resources.

* Allow users to create `AppProject` resources on their own. Since `AppProject`
  is used as central entity for enforcing governance and security, these should
  stay in the control of the cluster administrators.

* Replace or modify Argo CD internal RBAC model

## Proposal

We suggest to adapt the current mechanisms of reconciliation of `Application`
resources to include resources within the control plane's cluster, but outside
the control plane's namespace (e.g. `argocd`).

We think the following changes would need to be performed so that this will
become possible in a secure manner:

* Adapt the `AppProject` spec to include a new field, e.g. `sourceNamespaces`.
  The values of this new field will define which applications will be allowed
  to associate to the `AppProject`. Consider the following example:

  ```yaml
  apiVersion: argoproj.io/v1alpha1
  kind: AppProject
  metadata:
    name: some-project
    namespace: argocd
  spec:
    sourceNamespaces:
    - foo-ns
    - bar-ns
  ```

  ```yaml
  apiVersion: argoproj.io/v1alpha1
  kind: Application
  metadata:
    name: some-app
    namespace: bar-ns
  spec:
    project: some-project
  ```

  ```yaml
  apiVersion: argoproj.io/v1alpha1
  kind: Application
  metadata:
    name: other-app
    namespace: other-ns
  spec:
    project: some-project
  ```

  would allow `Application` resources that are created in either namespace
  `foo-ns` or `bar-ns` to specify `some-project` in their `.spec.project`
  field to associate themselves to the `AppProject` named `some-project`.
  In the above example, the Application `some-app` would be allowed to associate
  to the AppProject `some-project`, but the Application `other-app` would be
  invalid.

  This method would allow to delegate certain namespaces where users have
  Kubernetes RBAC access to create `Application` resources.

* `Applications` created in the control-plane's namespace (e.g. `argocd`) are
  allowed to associate with any project when created declaratively, as they
  are considered created by a super-user. So the following example would be
  allowed to associate itself to project `some-project`, even with the
  `argocd` namespace not being in the list of allowed `sourceNamespaces`:

  ```yaml
  apiVersion: argoproj.io/v1alpha1
  kind: Application
  metadata:
    name: superuser-app
    namespace: argocd
  spec:
    project: some-project
  ```

* `Applications` created imperatively (e.g. through Argo CD API via UI or
  CLI) will keep being created in the control plane namespace, and Argo CD
  RBAC will still be applied to determine whether a user is allowed to
  create `Application` for given `AppProject`.

If no `.spec.sourceNamespaces` is set to an `AppProject`, a default of the
control plane's namespace (e.g. `argocd`) will be assumed, and the current
behaviour would not be changed (only `Application` resources created in the
`argocd` namespace would be allowed to associate with the `AppProject`).

When the `argocd-application-controllers` discovers an `Application` to
consider for reconciliation, it would make sure that the `Application` is
valid by:

* Looking up the `AppProject` from the value of `.spec.project` field in the
 `Application` resource

* Matching the value of `.metadata.namespace` in the `Application` to a
  namespace found in `.spec.sourceNamespaces` in the referenced `AppProject`
  resource.

* If there is a match, reconciliation would continue as normal with the
  association desired in the `Application`'s spec.

* If there is no match, reconciliation would be aborted with a permission
  error.

### Use cases

Add a list of detailed use cases this enhancement intends to take care of.

#### Use case 1: Autonomous self-service of declarative configuration

As a developer, I want to be able to create & manage an Argo CD `Application`
in a declarative way, without sending a pull-request to the cluster admin's
repository and possibly wait for their review, approval and merge. I want
this process to be in full control of my DevOps team.

As a cluster admin, I want to allow my users the self management of Argo CD
`Application` resources without getting involved in the creation process,
e.g. by reviewing & approving PRs into our admin namespace. I still want
to be sure that users cannot circumvent any restrictions that our organisation
wants to impose on these applications capabilities.

#### Use case 2: App-of-apps pattern for my own applications

As a developer, I want to have the ability to use app-of-apps for my own
`Application` resources. For this, I will have to create `Application`
manifests, but I'm currently not allowed to write to the `argocd` namespace.

#### Use case 3: Easy onboarding of new applications and tenants

As an administrator, I want to provide my tenants with a very easy way to
create their applications from a simple commit to Git, without losing my
ability to govern and restrict what goes into my cluster.  I want to set up
an Argo CD application that reconciles my tenant's `Application` manifests to
a fixed location (namespace) in my cluster, so that the tenant can just put
their manifests into the Git repository and Argo CD will pick it up from
there, without having to use complex tools such as Open Policy Agent to
enforce what AppProjects my tenants are allowed to use.

### Implementation Details/Notes/Constraints [optional]

#### Application names

One major challenge to solve is the _uniqueness_ of `Application` names. As of
now this is enforced by Kubernetes, since the `Application` name is also the
name of the Kubernetes resource (e.g. value of `.metadata.name`). Because all
`Application` resources must currently exist in the same namespace, Kubernetes
already enforces a unique name for each `Application`.

This imposes a major challenge when `Application` resources may exist in other
namespaces, because it would be possible that two resources will have the same
name.

One way to mitigate this would be to decouple the application's name from the
name of the `Application` resource by considering the name of the resource's
namespace as part of the applicatio name.

E.g. consider the two following examples:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: some-app
  namespace: foons
spec:
  project: some-project
```

and

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: some-app
  namespace: barns
spec:
  project: some-project
```

The _name_ of the first application would become `foons/some-app` and the _name_
of the second application would become `barns/some-app`. This would have the
advantage to instantly have a clue about _where_ the application is coming from,
without having to resort to the Kubernetes API to inspect `.spec.namespace`
field of the app. The notation may also use a hyphen (`-`) or underscore (`_`)
instead of a slash (`/`) as a divider between the _source_ and the _name_, this
is implementation detail.

It is suggested to _not_ apply this naming convention to `Application` resources
created in the control plane's namespace, as this may break backwards compat
with existing RBAC rules. So, when considering the following example:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: some-app
  namespace: argocd
spec:
  project: some-project
```

The name of the application would still be `some-app`, instead of
`argocd/some-app`. This would allow us introducing the change without breaking
the RBAC rules of existing installations, and it would be not as intrusive.
Since for all other `Application` resources in external namespaces the name
will be prefixed, collisions in names should not happen.

We also considered the name of the `AppProject` instead the name of the
`Application`'s namespace. However, since in this proposal an `AppProject`
could allow resources from _multiple_ namespaces, and in each namespace an
`Application` with the same name could co-exist, this wouldn't solve the
uniqueness requirement.

#### Length of application names

When implementing above naming conventions, we may easily hit the length limit
enforced by Kubernetes on label values. Since we currently rely on setting the
`Application` name as a label on resources managed through the `Application`,
we should have an alternative implemented first.

There are already a few thoughts and suggestions on how to move away from
storing the `Application` name as a value in the identifying label, but this
would be a prerequisite before moving forward. As the demand for having more
lengthy `Application` names than what is currently possible (due to the limit
imposed by label's values), we feel that this should be implemented
independently anyway.

List of issues on GitHub tracker regarding the length of the application name:

* https://github.com/argoproj/argo-cd/issues/5595

### Detailed examples

### Security Considerations

We think that the security model of this proposal fits nicely within existing
mechanisms of Argo CD (e.g. RBAC, `AppProject` constraints).

However, on a bug in the implementation - e.g. the unwanted possibility for
non-admin users to associate their `Application` resources to arbitrary or
non-allowed `AppProject` resources - privilege escalation could occur.

Good unit- and end-to-end tests need to be in place for this functionality
to ensure we don't accidentally introduce a change that would allow any form
of uncontrolled association between an `Application` and an `AppProject`.

### Risks and Mitigations

#### Uncontrolled creation of a huge amount of Application resources

A rogue party or process (e.g. malfunctioning CI) could create a tremenduous
amount of unwanted `Application` resources in an allowed source namespace,
with a potential performance impact on the Argo CD installation. However, this
is also true for a rogue party using the Argo CLI with appropriate Argo CD RBAC
permissions to create applications or even with ApplicationSet.

A possible mitigation to this would be to enforce an (optional) quota to the
number of `Application` resources allowed to associate with any given
`AppProject` resource.

#### Third-party tools

Most third-party tools will look for `Application` resources in a single
namespace (the control plane's namespace - `argocd`) right now. These would
have to adapted to scan for `Application` resources in the complete cluster,
and might need updated RBAC permissions for this purpose.

### Upgrade / Downgrade Strategy

Upgrading to a version implementing this proposal should be frictionless and
wouldn't require administrators to perform any changes in the configuration
to keep the current behaviour. `AppProject` resources without the new field
`.spec.sourceNamespaces` being set will keep their behaviour, since they
will allow `Application` resources in the control plane's namespace to be
associated with the `AppProject`. Also, these `Applications` wouldn't be
subject to a name change (e.g. the proposed `<namespace>-<appname>` name).

Downgrading would not be easily possible once users start to make use of the
feature and create `Applications` in other namespaces than the control plane's
one. These applications would be simply ignored in a downgrade scenario, and
effectively become unmanaged. The mitigation to this would be to take these
`Application` resources and migrate them back to the control plane's namespace,
but possibly would have to adapt application names for uniqueness as well,
with consequences to RBAC rules.

## Drawbacks

* Application names have to change considerably to stay _unique_

* The Application CRs still need to reside on the control plane's cluster

* Downgrade/rollback would not be easily possible

## Alternatives

### ApplicationSet

One alternative is `ApplicationSet`, which automates the creation of
`Application` resources using generators with user-controlled input.
However, this does not solve creation of one-off `Applications` and is
considerably more complex to setup and maintain.

We do think that the proposed solution will nicely play together with the
`ApplicationSet` mechanism, and also could be used in conjuction with it
to provide better isolation of `Application` resources generated by
`ApplicationSet`.

### AppSource CRD and controller

Another recent proposal is the `AppSource` CRD and controller. We suggest this
proposal as a direct alternative to the `AppSource` CRD, with the drawback
that `Application` resources require to reside in the control plane's cluster
instead of the ability being created on remote clusters as well.

The `AppSource` proposal can be found here:
https://github.com/argoproj/argo-cd/issues/6405
