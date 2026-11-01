# Ignoring Differences: Common Cases and Pitfalls

[Diff Customization](diffing.md) explains the syntax of `ignoreDifferences` rules and where to configure them. The
[`RespectIgnoreDifferences`](sync-options.md#respect-ignore-differences-configs) sync option explains what the rules
do at sync time. This page collects the shapes users run into most often, the rule that handles each one, and the
mistakes that make a rule silently do nothing or do too much.

Every case below is covered by a test in the Argo CD repository
(`controller/sync_respect_ignore_semantics_test.go`), so the described behavior is what the controller does.

> [!NOTE]
> Without `RespectIgnoreDifferences=true`, a rule only hides the difference. The sync still applies the manifest as
> written, and the ignored field takes git's value again on every sync. With the option on, the sync keeps the live
> value of every ignored field. Most of the recipes below assume the option is on.

> [!WARNING]
> Releases before the fix for [#30674](https://github.com/argoproj/argo-cd/issues/30674) could drop or replace whole
> lists when an ignored field sat inside a list item. If one of the list recipes below does not behave as described,
> check your release notes.

## Fields another controller changes

### Replicas managed by an HPA

The manifest declares `replicas`, the HorizontalPodAutoscaler changes it.

```yaml
spec:
  ignoreDifferences:
    - group: apps
      kind: Deployment
      jsonPointers:
        - /spec/replicas
  syncPolicy:
    syncOptions:
      - RespectIgnoreDifferences=true
```

The sync applies the live replica count instead of git's, so the HPA's decision survives the sync. The same works
for Argo Rollouts (`group: argoproj.io`, `kind: Rollout`).

If the cluster uses server-side apply, `managedFieldsManagers` is the alternative that needs no path:

```yaml
spec:
  ignoreDifferences:
    - group: apps
      kind: Deployment
      managedFieldsManagers:
        - kube-controller-manager
```

This form only covers fields the manifest still declares. A field that exists only in the live resource is not
covered by it; use a pointer or jq rule for that.

### Container image or pull policy changed by an image updater

```yaml
spec:
  ignoreDifferences:
    - group: argoproj.io
      kind: Rollout
      jqPathExpressions:
        - .spec.template.spec.containers[].image
        - .spec.template.spec.containers[].imagePullPolicy
```

Containers are matched by `name`, so the rule follows each container even when git and the cluster list the
containers in a different order. Changes to other container fields in git, such as resource requests, are still
applied.

### Resource requests set by a Vertical Pod Autoscaler

```yaml
spec:
  ignoreDifferences:
    - group: apps
      kind: Deployment
      jqPathExpressions:
        - .spec.template.spec.containers[].resources
```

This works whether or not the manifest declares `resources`. When it does not, the sync copies the live
`resources` of each container into the apply so they are not removed.

### A pod template annotation added at runtime

`kubectl rollout restart` sets `kubectl.kubernetes.io/restartedAt`; cloud providers add their own annotations.

```yaml
spec:
  ignoreDifferences:
    - group: apps
      kind: Deployment
      jsonPointers:
        - /spec/template/metadata/annotations/kubectl.kubernetes.io~1restartedAt
```

A slash inside a JSON pointer segment is written `~1`. The same rule as a jq expression is
`.spec.template.metadata.annotations["kubectl.kubernetes.io/restartedAt"]`. If some manifests have no
`annotations` at all, `.spec.template.metadata.annotations?.["kubectl.kubernetes.io/restartedAt"]` avoids an
error on those.

### Webhook caBundle injected by cert-manager

```yaml
data:
  resource.customizations.ignoreDifferences.admissionregistration.k8s.io_MutatingWebhookConfiguration: |
    jqPathExpressions:
      - '.webhooks[]?.clientConfig.caBundle'
```

Webhooks are matched by `name`. A change to another field of the same webhook in git, for example
`failurePolicy`, is still applied while the injected bundle is kept.

### Service fields the API server allocates

```yaml
spec:
  ignoreDifferences:
    - kind: Service
      jsonPointers:
        - /spec/clusterIP
        - /spec/clusterIPs
      jqPathExpressions:
        - .spec.ports[].nodePort
```

Ports are matched by `port` and `protocol`. A manifest that omits `protocol` still matches the live port, because
the schema default `TCP` is used for the missing key.

### Defaults inside StatefulSet volumeClaimTemplates

The API server fills `apiVersion`, `kind`, `metadata.creationTimestamp` and `status` into every template.

```yaml
spec:
  ignoreDifferences:
    - group: apps
      kind: StatefulSet
      jqPathExpressions:
        - .spec.volumeClaimTemplates[].apiVersion
        - .spec.volumeClaimTemplates[].kind
        - .spec.volumeClaimTemplates[].metadata.creationTimestamp
        - .spec.volumeClaimTemplates[].status
```

### Secret keys written by an operator or generated by a chart

An operator rotates one key, or a Helm chart renders a random password on every `helm template`
([Cilium documents this case](https://docs.cilium.io/en/stable/configuration/argocd-issues/)).

```yaml
spec:
  ignoreDifferences:
    - kind: Secret
      jqPathExpressions:
        - .data["password"]
```

The manifest may use `stringData` and the rule still applies: Argo CD folds `stringData` into `data` before the
rules run, as the API server does on write. A rule written as `/stringData/password` protects the same key. Other
keys in the Secret are still applied from git.

To protect every key, ignore `/data` as a whole. Then nothing in the Secret's data is ever updated from git, which
is usually what is wanted for a fully generated Secret and usually not for a Secret with some fixed keys.

### Tokens created in the UI inside an AppProject

```yaml
spec:
  ignoreDifferences:
    - group: argoproj.io
      kind: AppProject
      jqPathExpressions:
        - .spec.roles[].jwtTokens
```

Roles are matched by `name`. A policy change in git is applied while the tokens stay.

### A child Application whose targetRevision is updated by another tool

For a multi-source Application, select the source by content, not by index:

```yaml
spec:
  ignoreDifferences:
    - group: argoproj.io
      kind: Application
      jqPathExpressions:
        - .spec.sources[] | select(.repoURL == "https://git.example.com/python-app-helm-values.git") | .targetRevision
```

`spec.sources` has no merge key, so items are matched by position when the sync merges the live value in. Keep the
order of sources in git the same as in the cluster.

### A list owned entirely by another tool

An ApplicationSet whose `generators`, or an Application whose `sources`, are maintained outside git:

```yaml
spec:
  ignoreDifferences:
    - group: argoproj.io
      kind: Application
      jsonPointers:
        - /spec/sources
```

The live list replaces the whole list from git on sync. Whatever the manifest holds under that path is never
applied while the rule is in place.

## Items another controller injects that git does not declare

### An injected init container or sidecar

Select the container by name. The rule works whether the manifest has its own init containers or none.

```yaml
spec:
  ignoreDifferences:
    - group: apps
      kind: Deployment
      jqPathExpressions:
        - .spec.template.spec.initContainers[] | select(.name == "injected-init-container")
```

Ignoring only a field of the injected container, such as `| .image`, keeps the whole container too. The sync
needs the container's identity to place the field, and a keyed list item cannot be applied as an image alone.

### An injected toleration

```yaml
spec:
  ignoreDifferences:
    - group: apps
      kind: Deployment
      jqPathExpressions:
        - .spec.template.spec.tolerations[] | select(.key == "node.example.com/spot")
```

`tolerations` is an atomic list: the API server replaces the whole list on every apply. The sync therefore carries
the injected toleration into the apply next to the ones git declares, so a change to one of git's tolerations is
still applied and the injected one is not removed. Ignoring only `.value` of the injected toleration carries the
whole toleration for the same reason.

### A nodeSelector key set by another controller

`nodeSelector` is an atomic map. If the manifest declares a `nodeSelector`, a rule on one key works as expected:

```yaml
spec:
  ignoreDifferences:
    - group: apps
      kind: Deployment
      jqPathExpressions:
        - .spec.template.spec.nodeSelector["topology.kubernetes.io/zone"]
```

If the manifest declares no `nodeSelector` at all, the sync does not build one from the single ignored key. That
would make the API server replace the whole map with that one key and drop the rest. To preserve a `nodeSelector`
the manifest does not declare, ignore `.spec.template.spec.nodeSelector` as a whole.

### An Ingress path managed by Argo Rollouts

Rollouts inserts a header route in front of the stable route during a canary.

```yaml
spec:
  ignoreDifferences:
    - group: networking.k8s.io
      kind: Ingress
      jqPathExpressions:
        - .spec.rules[].http.paths[] | select(.backend.service.name == "canary-header-route")
```

`paths` has no merge key. The live-only path is inserted at the position it has in the cluster, so it stays in
front of the stable route.

## Pitfalls

### An iterator over a field some items lack disables the rule

```yaml
jqPathExpressions:
  - .spec.template.spec.containers[].env[] | select(.name == "B") | .value
```

If any container has no `env`, jq fails with `cannot iterate over null` and the whole rule is skipped, at diff time
and at sync time. Other rules on the same resource still work. Write `.env[]?` to skip containers without `env`.

### Indexes name positions, not items

`/spec/template/spec/containers/0/image` means whichever container is first. Git and the cluster may order the list
differently, and a controller may insert an item in front. Select items by content instead:
`.spec.template.spec.containers[] | select(.name == "web") | .image`.

For a list with merge keys, a sync with `RespectIgnoreDifferences=true` refuses such a pointer outright when index 0
is one container in git and another in the cluster, and names the rule in the operation's message. While the orders
agree the pointer works, so the refusal appears only when it would have misapplied.

An index rule written for an item a webhook injects first, `/spec/template/spec/tolerations/0`, removes git's own
first item wherever the webhook has not injected yet.

When two index rules name the same list, both indexes refer to the original list. `/initContainers/0` and
`/initContainers/1` ignore the first two items, not the first item and then the first item of what remains.

### A rule on a field does not keep the item

```yaml
jqPathExpressions:
  - .spec.template.spec.containers[].image
```

This rule is about images, not about containers. With `RespectIgnoreDifferences=true`, a container that exists only
in the cluster, whether injected or deleted from git, is not kept by it: the sync applies git's list of containers
and the apply removes the extra one (an admission webhook re-injects its container on the same apply). The same
holds for a toleration whose `value` is ignored, or an env var whose `value` is ignored. To keep an injected item,
ignore the item itself: `.spec.template.spec.containers[] | select(.name == "istio-proxy")`.

The upside is that deletions in git work: removing a container from the manifest removes it from the cluster, even
with `[].image` ignored.

### A whole-item rule keeps the old item across a rename

`.spec.template.spec.tolerations[] | select(.key == "spot")` keeps the `spot` toleration wherever it is in the
cluster. Rename it to `preemptible` in git and the cluster ends up with both: the old one is ignored and carried, the
new one is in git and applied. Delete the old item by hand, or drop the rule for one sync.

### A conditional rule does not stop git from winning

```yaml
jqPathExpressions:
  - .data.setting | select(. == "live")
```

This hides the difference while the live value is `live`. The sync still applies git's value, because the rule
removed nothing from the manifest. Rules decide what is ignored; they do not express "never set this field".

### Ignoring every item is not the same as ignoring the list

`.entries[]` ignores each item, and when every item is ignored the apply still asserts `entries: []`, which is also
what the diff compares. To leave a list alone entirely, ignore the list: `.entries` or `/entries`.

### Lists on custom resources may be atomic

Argo CD reads `x-kubernetes-list-type`, `x-kubernetes-list-map-keys` and `x-kubernetes-map-type` from the
cluster's schema. A CRD that publishes none of them is handled as if every list were atomic, so an ignored item
inside such a list is carried next to git's items and the whole list is applied as one value. Check the CRD if
list-item rules on a custom resource surprise you.

### Never trust Argo CD's own manager

`managedFieldsManagers: [argocd-controller]` would ignore every field Argo CD itself applied, which is everything
git has ever declared: no change from git would apply again, and the diff would report Synced regardless. A sync
with `RespectIgnoreDifferences=true` refuses the rule. Name the other manager, such as `kube-controller-manager`
or `vpa-recommender`. For child Applications, `argocd-application-controller` is a different manager and is fine.

### Never ignore a merge key

`.spec.template.spec.containers[].env[].name` ignores the field that identifies env entries. Items cannot be matched
without it, so the sync refuses the rule. There is no sensible version of it.

### Removing a rule under client-side apply deletes the field it protected

With `RespectIgnoreDifferences=true` and client-side apply, the live value of an ignored field is part of what the
sync applies, and so of kubectl's last-applied-configuration annotation. Remove the rule, and the next sync treats
the field as something git used to declare and no longer does: the three-way merge deletes it. Declare the field in
git before removing the rule, or use `ServerSideApply=true`, where the other manager keeps ownership of its field.

### `managedFieldsManagers` covers only fields git declares

The rule ignores the fields of the named manager that differ from the manifest. A field that is only in the live
resource is not part of the comparison and is not protected by this rule form.

### Argo CD co-owns ignored fields under server-side apply

With `RespectIgnoreDifferences`, the sync applies the live value of an ignored field, so `argocd-controller`
appears as a manager of that field next to the controller that set it. That is expected and has always been the
case. Under server-side apply it is also the only way an ignored field survives when Argo CD is its sole owner.

### What is never carried from the live resource

`status`, and the metadata fields the API server populates (`uid`, `resourceVersion`, `generation`,
`creationTimestamp`, `managedFields`, `deletionTimestamp`), are never copied into the apply, even when a rule
ignores `/status` or all of `/metadata`. Labels and annotations under an ignored `/metadata` are carried, except
Argo CD's own tracking annotation and label, which are always applied from the manifest: ignoring all annotations
cannot hand a shared resource back to the application that synced it last. The resource's `apiVersion`, `kind`,
`metadata.name` and `metadata.namespace` are always the manifest's as well; the cluster may serve the live object
at another API version, and that version must not be put on the manifest's body.

### A rule that is not a path does nothing at diff time and fails the sync

`jqPathExpressions` must evaluate to paths. `.data | keys[] | select(startswith("x"))`, `to_entries`, `map(...)`
and `with_entries(...)` are not path expressions. The diff skips such a rule without an error, so the field keeps
showing as different; a sync with `RespectIgnoreDifferences=true` fails with the rule and the resource named in
the operation's message, rather than apply the desired value over the one the rule was meant to preserve. Name
the field instead, for example `.data["x-key"]`.

A rule that is a path but does not fit a resource, such as `.spec.template.spec.containers[].env[]` on a
container with no `env` ("cannot iterate over null"), contributes nothing for that resource at diff and sync
alike. Write `env[]?` when the field may be absent.

### Finalizers added by a controller

```yaml
spec:
  ignoreDifferences:
    - group: argoproj.io
      kind: Application
      jqPathExpressions:
        - .metadata.finalizers[] | select(startswith("pre-delete-finalizer.argocd.argoproj.io"))
```

`metadata.finalizers` is a set: items are matched by value. The selected finalizer is carried next to the ones git
declares, also when git declares none. A `managedFieldsManagers` rule does not cover it, because a finalizer that
only the cluster has is not part of the comparison (see above).

### The option only acts on resources that already exist

When the resource is being created there is no live state to keep, and the manifest is applied as written.
