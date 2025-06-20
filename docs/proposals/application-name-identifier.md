---
title: Change the way application resources are identified
authors:
  - "@jannfis"
sponsors:
  - TBD
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2021-06-07
last-updated: 2021-06-07
---

# Change the way application resources are identified

This is a proposal to introduce the tracking method settings that allows using
an annotation as the application identifier instead of the application instance label.
This will allow application names longer than 63 characters and solve issues caused by
copying `app.kubernetes.io/instance` label. As an additional goal, we propose to introduce an
installation ID that will allow multiple Argo CD instances to manage resources
on the same cluster.


## Summary

Argo CD identifies resources it manages by setting the _application instance
label_ to the name of the managing `Application` on all resources that are
managed (i.e. reconciled from Git). The default label used is the well-known
label `app.kubernetes.io/instance`.

This proposal suggests to introduce the `trackingMethod` setting that allows
controlling how application resources are identified and allows switching to
using the annotation instead of `app.kubernetes.io/instance` label.

## Motivation

The main motivation behind this change is to solve the following known issues:

* The Kubernetes label value cannot be longer than 63 characters. In large scale
  installations, in order to build up an easy to understand and
  well-formed naming schemes for applications managed by Argo CD, people often
  hit the 63 character limit and need to define the naming scheme around this
  unnecessary limit.

* Popular off-the-shelf Helm charts often add the `app.kubernetes.io/instance` label
  to the generated resource manifests. This label confuses Argo CD and makes it think the
  resource is managed by the application.

* Kubernetes operators often create additional resources without creating owner reference
  and copy the `app.kubernetes.io/instance` label from the application resource. This is
  also confusing Argo CD and makes it think the resource is managed by the application.

An additional motivation - while we're at touching at application instance
label - is to improve the way how multiple Argo CD instances could manage
applications on the same cluster, without requiring the user to actually
perform instance specific configuration.

### Goals

* Allow application names of more than 63 characters

* Prevent confusion caused by copied/generated `app.kubernetes.io/instance` label

* Keep having a human-readable way to identify resources that belong to a
  given Argo CD application

* As a stretch-goal, allow multiple Argo CD instances to manage resources on
  the same cluster without the need for configuring application instance label
  key (usually `app.kubernetes.io/instance`)

### Non-Goals

* Change the default name of the application instance label

## Proposal

We propose introducing a new setting `trackingMethod` that allows to control
how application resources are identified. The `trackingMethod` setting takes
one of the following values:

* `label` (default) - Argo CD keep using the `app.kubernetes.io/instance` label.
* `annotation+label` - Argo CD keep adding `app.kubernetes.io/instance` but only
  for informational purposes: label is not used for tracking, value is truncated if
  longer than 63 characters. The `app.kubernetes.io/instance` annotation is used
  to track application resources.
* `annotation` - Argo CD uses the `app.kubernetes.io/instance` annotation to track
  application resources.

The `app.kubernetes.io/instance` attribute values includes the application name,
resources identifier it is applied to, and optionally the Argo CD installation ID:

The application name allows to identify the application that manages the resource. The
resource identifier prevents confusion if an operation copies the
`app.kubernetes.io/instance` annotation to another resource. Finally optional
installation ID allows separate two Argo CD instances that manages resources in the same cluster.

The `trackingMethod` setting should be available at the system level and the application level to
allow the smooth transition from the old `app.kubernetes.io/instance` label to the new tracking method.
Using the app leverl settings users will be able to first switch applications one by one to the new tracking method
and prepare for the migration. Next system level setting can be changed to `annotation` or `annotation+label`
and not-migrated applications can be configured to use `labels` using application level setting.


### Use cases

Add a list of detailed use cases this enhancement intends to take care of.

#### Use case 1: Allow for more than 63 characters in application name

As a user, I would like to be able to give my applications names with arbitrary
length, because I want to include identifiers like target regions and possibly
availability zones, the environment and possibly other identifiers (e.g. a team
name) in the application names. The current restriction of 63 characters is not
sufficient for my naming requirements.

#### Use case 2: Allow for retrieving all resources using Kubernetes

As an administrator, I want to enable my users to use more than 63 characters
in their application names, but I still want to be able to retrieve all of the
resources managed by that particular application using Kubernetes mechanisms,
e.g. a label selector as in the following example:

```
kubectl get deployments -l app.kubernetes.io/instance=<application> --all-namespaces
```

#### Use case 3: Multiple Argo CD instances managing apps on same cluster

I also want to be able to see which application and Argo CD instance is the
one in charge of a given resource.

### Implementation Details/Notes/Constraints [optional]

#### Include resource identifies in the `app.kubernetes.io/instance` annotation

The `app.kubernetes.io/instance` annotation might be accidently added or copied
same as label. To prevent Argo CD confusion the annotation value should include
the identifier of the resource annotation was applied to. The resource identifier
includes the group, kind, namespace and name of the resource. It is proposed to use `;`
to separate identifier from the application name.

```yaml
annotations:
    app.kubernetes.io/instance: <application-name>;<group>/<kind>/<namespace>/<name>
```

Example:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-deployment
  namespace: default
  annotations:
    app.kubernetes.io/instance: my-application;apps/Deployment/default/my-deployment
```

#### Allow multiple Argo CD instances manage applications on same cluster

As of today, to allow two or more Argo CD instances with a similar set of
permissions (e.g. cluster-wide read access to resources) manage applications
on the same cluster, users would have to configure the _application instance
label key_ in the Argo CD configuration to a unique value. Otherwise, if an
application with the same name exists in two different Argo CD installations,
both would claim ownership of the resources of that application.

We do see the need for preventing such scenarios out-of-the-box in Argo CD.
For this, we do suggest the introduction of an _installation ID_ in the
form of a standard _GUID_.

This GUID would be generated once by Argo CD upon startup, and is persisted in
the Argo CD configuration, e.g. by storing it as `installationID` in the
`argocd-cm` ConfigMap. The GUID of the installation would need to be encoded
in some way in the resources managed by that Argo CD instance.

We suggest using a dedicated annotation to store the GUID and modify Argo CD so that it matches _both_, the app
instance key and the GUID to determine whether a resource is managed by
this Argo CD instance. Given above mentioned GUID, this may look like the
following on a resource:

  ```yaml
  apiVersion: v1
  Kind: Secret
  metadata:
    name: some-secret
    namespace: some-namespace
    annotations:
      app.kubernetes.io/instance: my-application;/Secret/some-namespace/some-secret
      argo-cd.argoproj.io/installation-id: 61199294-412c-4e78-a237-3ebba6784fcd
  ```

The user should be able to opt-out of this feature by setting the `installationID` to an empty string.

### Security Considerations

We think this change will not have a direct impact on the security of Argo CD
or the applications it manages. 

### Risks and Mitigations

The proposal assumes that user can keep adding `app.kubernetes.io/instance` label
to be able to retrieve resources using `kubectl get -l app.kubernetes.io/instance=<application>` command.
However, Argo CD is going to truncate the value of the label if it is longer than 63 characters. There is
a small possibility that there are several applications with the same first 63 characters in the name. This
should be clearly stated in documentation.

### Upgrade / Downgrade Strategy

Upgrading to a version that implements this proposal should be seamless, as
previously injected labels will not be removed and additional annotations will
be applied to the resource. E.g. consider following resource in Git, that will
be synced as part of an application named `some-application`. In Git, the
resource looks like follows:

```yaml
apiVersion: v1
Kind: Secret
metadata:
  name: some-secret
  namespace: some-namespace
```

When synced with the current incarnation of Argo CD, Argo CD would inject the
application instance label and once the resource is applied in the cluster, it
would look like follows:

```yaml
apiVersion: v1
Kind: Secret
metadata:
  name: some-secret
  namespace: some-namespace
  labels:
    app.kubernetes.io/instance: some-application
```

Once Argo CD is updated to a version implementing this proposal, the resource
would be rewritten to look like the following:

```yaml
apiVersion: v1
Kind: Secret
metadata:
  name: some-secret
  namespace: some-namespace
  labels:
    app.kubernetes.io/instance: some-application
  annotations:
    app.kubernetes.io/instance: my-application;/Secret/some-namespace/some-secret
    argo-cd.argoproj.io/installation-id: 61199294-412c-4e78-a237-3ebba6784fcd
```

On a rollback to a previous Argo CD version, this change would be reverted
and the resource would look like the first shown example above.

## Drawbacks

We do see some drawbacks to this implementation:

* This change would trigger a re-sync of each and every managed resource, which
  may result in unexpected heavy load on Argo CD and the cluster at upgrade
  time. The workaround is an ability to opt-out of this as a default and enable it
  on application basis.

## Alternatives

* Enabling application names longer than 63 characters could also be done
  by using the hashed value of the application name and additional metadata as a label.
  The disadvantage of this approach is that hash value is not human friendly. In particular,
  it is difficult to retrieve application manifests using `kubectl get -l app.kubernetes.io/instance=<application>`.