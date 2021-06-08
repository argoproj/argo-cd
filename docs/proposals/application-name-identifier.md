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

This is a proposal to introduce using a hash value as application identifier
in the application instance label. This will allow application names longer
than 63 characters. As an additional goal, we propose to introduce a GUID as
installation ID that will allow multiple Argo CD instances manage resources
on the same cluster, without the need for manual reconfiguration.

## Open Questions [optional]

The major open questions are:

* Which hashing algorithm to use? The authors think that `SHA-1` and `FNV-1a`
  are the best options on the table, with `FNV-1a` being the more performant
  one and `SHA-1` being the one most resilient against collisions with a
  compromise for speed. There is a pretty good comparison of non cryptographic
  hash algorithms on Stack Exchange that suggests that `FNV-1a` might be more
  than sufficient for our use-cases:
  https://softwareengineering.stackexchange.com/a/145633

* The proposal defines a stretch goal for allowing multiple Argo CD instances
  to manage applications on the same cluster, without the need for changing
  the _application instance label key_ of each installation. This change may
  sound intrusive, but the authors think the benefits would outweigh the
  disadvantages. 

## Summary

Argo CD identifies resources it manages by setting the _application instance
label_ to the name of the managing `Application` on all resources that are
managed (i.e. reconciled from Git). The default label used is the well-known
label `app.kubernetes.io/instance`.

This proposal suggests to change the _value_ of this label to not use the
literal application name, but instead use a value from a stable and collision
free hash algorithm.

## Motivation

The main motivation behind this change is the Kubernetes restriction for the
maximum allowed length of label values, which no more than 63 characters.

In large scale installations, in order to build up an easy to understand and
well-formed naming schemes for applications managed by Argo CD, people often
hit the 63 character limit and need to define the naming scheme around this
unnecessary limit.

Furthermore, proposed changes such as described in
https://github.com/argoproj/argo-cd/pull/6409
would require the application names to include more implicit information
(such as the application's source namespace), which will even add more
characters to the application names.

An additional motivation - while we're at touching at application instance
label - is to improve the way how multiple Argo CD instances could manage
applications on the same cluster, without requiring the user to actually
perform instance specific configuration.

### Goals

* Allow application names of more than 63 characters

* Keep using a label to properly _select_ resources managed by an Application
  using Kubernetes label selectors

* Keep having a human-readable way to identify resources that belong to a
  given Argo CD application

* As a stretch-goal, allow multiple Argo CD instances to manage resources on
  the same cluster without the need for configuring application instance label
  key (usually `app.kubernetes.io/instance`)

### Non-Goals

* Change the default name of the application instance label

## Proposal

We propose to move from a _human-readable_ value to identify the resources of
an application to a _machine-readable_ value of fixed length. For this purpose
we propose to use a one-way hashing algorithm. The chosen algorithm should be
_collision-free_ to a certain extent, but does not require to be _secure_ in a
cryptographic evaluation.

We further propose to add an _annotation_ to the managed resources, which will
contain the plain text value of the application's name so that humans and other
consumers of the resources will still be able to identify the application that
is managing the resource. Annotations in Kubernetes don't suffer from the same
length restrictions as labels, and can easily store about every possible
application name that users might come up with.

Furthermore, we may want to consider encoding a unique, persistant identifier
of the Argo CD installation into the final hash value, e.g.

```
value = hashFunc(installationIdentifier + "." + applicationName)
```

This way, we could support multiple Argo CD instances managing applications on
the same cluster _without_ the need for those installations to have their
instance label name changed to a unique value in order to prevent a "split
brain" scenario where multiple Argo CD instances claim ownership to given
resources, which could potentially result in severe data loss (unintentinoal
resource pruning).

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

#### Use a hash value for the application name

* The application controller needs to be adapted to use the computed hash value
  for determining whether the inspected resource is being managed by the
  current application.

* The used hash algorithm is _not required_ to be cryptographically secure, but
  should be _reasonable resilient_ against collisions. It should also be
  _reasonable fast_ in computing the hash.

* The _application instance label_ on a managed resource should be set to the
  hexa-decimal representation of the computed hash value for the application
  name. What becomes the source for the hash's computation has yet to be
  decided upon (see above).

* Additionally, a new annotation should be introduced to managed resources.
  This annotation will hold the _human readable_ representation of the name
  of the application the resource belongs to. This annotation only serves an
  informational purpose for humans, or other tools that need to determine
  the name of the application that manages a given resource. This may seem
  redundant at first, but we believe is a requirement for many users.

For example, a managed resource could look like the following after this
change has been implemented using `SHA-1` algorithm when the application's
name is `some-application` and noted `installationIdentifier` isn't taken
into account yet):

```yaml
apiVersion: v1
Kind: Secret
metadata:
  name: some-secret
  namespace: some-namespace
  labels:
    app.kubernetes.io/instance: 3fbe5782d494cc956140bc58386c971bf0e96fad
  annotations:
    argo-cd.argoproj.io/application-name: some-application
```

The hash-value in the above `app.kubernetes.io/instance` label is simply the
hexa-decimal representation of the SHA-1 value for `some-application`:

```shell
$ echo "some-application" | sha1sum
3fbe5782d494cc956140bc58386c971bf0e96fad  -
```

For humans or consumers that need the application name, the application name
is stored in clear text in an annotation. Suggestion for this annotation name
would be `argo-cd.argoproj.io/application-name`.

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
in some way in the resources managed by that Argo CD instance, and we do see
two main possibilities to do so:

* Encode the GUID in the application name's hash value, e.g. (as noted above)
  by concatenating the GUID with the application name before hashing it:

  ```yaml
  value = hashFunction(GUID + "." + applicationName)
  ```

  So, given a GUID of `61199294-412c-4e78-a237-3ebba6784fcd`, the previous
  example for `some-application` would become:

  ```yaml
  apiVersion: v1
  Kind: Secret
  metadata:
    name: some-secret
    namespace: some-namespace
    labels:
      app.kubernetes.io/instance: 1e2927872cdb73ec3c4010b8afeba2c4be1a92cb
    annotations:
      argo-cd.argoproj.io/application-name: some-application
  ```

  To generate the hash manually (e.g. for selection of resources by label),
  the user would have to take the GUID into account:

  ```shell
  $ echo "61199294-412c-4e78-a237-3ebba6784fcd.some-application" | sha1sum
  1e2927872cdb73ec3c4010b8afeba2c4be1a92cb  -
  ```

* Use a dedicated label to store the GUID instead of encoding it in the
  application name, and modify Argo CD so that it matches _both_, the app
  instance key and the GUID to determine whether a resource is managed by
  this Argo CD instance. Given above mentioned GUID, this may look like the
  following on a resource:

  ```yaml
  apiVersion: v1
  Kind: Secret
  metadata:
    name: some-secret
    namespace: some-namespace
    labels:
      app.kubernetes.io/instance: 3fbe5782d494cc956140bc58386c971bf0e96fad
      argo-cd.argoproj.io/installation-id: 61199294-412c-4e78-a237-3ebba6784fcd
    annotations:
      argo-cd.argoproj.io/application-name: some-application
  ```

Both methods have their advantages and disadvantages. We would prefer to use
the first approach and encode the GUID into the application name's hash, as
this would not introduce a new label. However, when manually selecting the
resources of any given application, the user would have to take this GUID
into account when constructing the hash to use for selecting resources.

### Detailed examples

### Security Considerations

We think this change will not have a direct impact on the security of Argo CD
or the applications it manages. 

One concern however is that when a user intentionally produces an application
name whose hash will collide with another, existing application's hash. That
may lead to the undesired deletion (pruning) of resources that do in fact
belong to another application outside the adversaries control. However, this
risk could be properly mitigated by permissions in the `AppProject`, since
Argo CD will never prune resources that are not allowed for any specific
Application.

### Risks and Mitigations

The main risks from the authors' points of view are collisions in the hashes
of application names. For example, if the strings `some-app` and `other-app`
produce the same hash, this could result in undesired behavior, especially
when unintentional. The mitigation for this is to chose a hashing algorithm
with a high resilience against collisions.

Another risk is that third party tools that need to map a managed resource
in the cluster back to the managing application rely on the clear text value
of the application instance label. These tools would have to be adapted to
read the proposed annotation instead of the application instance label.

We think that this may be a breaking change, however, the advantages outweigh
the risks.

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
    app.kubernetes.io/instance: 3fbe5782d494cc956140bc58386c971bf0e96fad
  annotations:
    argo-cd.argoproj.io/application-name: some-application
```

This would be very similar to _renaming_ an application, as in the managed
resources metadata would be modified in the cluster.

On a rollback to a previous Argo CD version, this change would be reverted
and the resource would look like the first shown example above.

## Drawbacks

We do see some drawbacks to this implementation:

* The already mentioned possible backwards incompatibility with existing tools
  that rely on reading the `app.kubernetes.io/instance` label to map back a
  resource to its managing application.

* This change would trigger a re-sync of each and every managed resource, which
  may result in unexpected heavy load on Argo CD and the cluster at upgrade
  time.

* People manually selecting resources from an application must create the
  value for the label selector instead of just using the application name, e.g.
  instead of

  ```shell
  kubectl get secrets -l app.kubernetes.io/instance=some-application --all-namespaces
  ```

  Something like the following needs to be constructed:

  ```shell
  kubectl get secrets -l app.kubernetes.io/instance=$(echo "some-application" | sha1sum | awk '{ print $1; }') --all-namespaces
  ```

* If we chose to also implement the GUID as application identifier, the GUID
  token becomes a viable part of the _state_ and needs to be backed up as
  part of any recovery procedures. Without this GUID, resources not anymore
  existing in Git will not get pruned upon removal, thus becoming stale in
  the cluster. A resource synced with a given GUID will only be ever removed
  again if the GUID of the pruning instance is the same as at sync time.

## Alternatives

* Enabling application names longer than 63 characters could also be done
  by having the _application instance label_ to become an annotation instead.
  This is a much simpler solution, however, this would disable the possibility
  to use label selectors for retrieving all resources managed by a given
  application.