---
title: Expose API server warnings on Argo CD UI
authors:
  - "@toVersus"
sponsors:
  - TBD
reviewers:
  - TBD
approvers:
  - TBD

creation-date: 2026-07-26
last-updated: 2026-07-26
---

# Expose API server warnings on Argo CD UI

This is the proposal for exposing API server warnings (e.g., Validating Admission Webhook/Policy,
deprecated APIs) on Argo CD UI.

Related Issues:
* [Expose validation webhook warnings in Argo CD UI](https://github.com/argoproj/argo-cd/issues/9256)

## Open Questions

- **Should we add new `Warning` operation phase and the `SyncedWithWarning` result code?** A warning
  does not block the change. The resource is applied successfully and the API server merely returns
  an advisory warning alongside it. This is exactly how kubectl treats it. A warning is still a success.
  Both new states therefore describe a "succeeded, but with a warning" outcome, so a fair question is
  whether we need a new result code at all. We could instead keep the existing `Succeeded` phase and
  `Synced` result code and carry the warning text only in the per-resource message. Introducing the new
  states makes the warning a first-class, visible signal, but the application-level `Warning` phase
  in particular has a cost. An older `argocd` CLI does not recognize it and treats it as a failed sync
  (see the Upgrade / Downgrade Strategy section). Is the added visibility worth new states and the
  backward-compatibility break?
- **If we add the `Warning` operation phase, does it need a feature gate?** The phase is the only
  part of this proposal that breaks an older client, so one option is to gate it behind a controller
  setting that defaults to off and can flip to on in a later major version. See the Upgrade /
  Downgrade Strategy section for the details.
- **If we keep the result code, is `SyncedWithWarning` the right name?** It sits next to `Synced`,
  `SyncFailed`, `Pruned`, and `PruneSkipped`. Does it read clearly alongside those, and does it
  warrant a UI label and color that is clearly distinct from both `Synced` and `SyncFailed`?
- **Are the changes to gitops-engine acceptable, especially how the warning handler is injected?**
  This proposal touches a fair amount of gitops-engine. It changes the `ResourceOperations` layer,
  `runResourceCommand`, and the per-operation client wiring. The most delicate part is how the
  per-operation `WarningHandler` reaches the kubectl command path. We wrap the shared
  `RESTClientGetter` so that only `ToRESTConfig` carries the handler, while the discovery client and
  REST mapper stay shared (see
  [Delivering the handler to the real requests](#delivering-the-handler-to-the-real-requests)). Is
  this wiring acceptable to gitops-engine maintainers, or is there a cleaner, more idiomatic injection
  point? And since gitops-engine is a shared library, could these changes affect consumers other than
  Argo CD?

## Summary

When the Kubernetes API server returns a warning while Argo CD applies a resource (for example from a
Validating Admission Webhook or Policy, or a deprecated API), that warning is currently only written
to the application-controller log, so users never see it in the UI. This proposal surfaces those
warnings per resource. As the application-controller creates, updates, or deletes a resource, a
per-operation client-go `WarningHandler` records any API server warning and folds it into that
resource's sync message. A new `SyncedWithWarning` result code marks the resource so the UI can flag
it, and a new `Warning` operation phase aggregates this at the Application level. Both are treated as a
successful sync, because a warning does not block the change.

## Motivation

Kubernetes provides a mechanism for the API server to return warnings to clients. This lets users be
notified of settings that should be changed, or that are recommended to be changed. For details,
see [Warning: Helpful Warnings Ahead](https://kubernetes.io/blog/2020/09/03/warnings/).
Warnings are returned via the `Warning` HTTP response header and typically originate from:

- Deprecated APIs
- [Validating Admission Webhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/)
- [Validating Admission Policy](https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/)

kubectl, a Kubernetes client, prints any warnings it receives to stderr. Unlike errors, warnings do not
block the resource modification, so the change itself completes successfully. In Argo CD, however,
these warnings are only recorded in the Argo CD application-controller logs and cannot be seen
by users from the Argo CD UI.

### Goals

- When a user modifies a resource through the Argo CD UI or CLI and the API server returns a warning,
display that warning on a per-resource basis.

### Non-Goals

- Display warnings when showing a resource diff through the Argo CD UI or CLI.

## Proposal

When the Argo CD application-controller creates, updates, or deletes a resource, it captures API server
warnings using client-go's [WarningHandler](https://pkg.go.dev/k8s.io/client-go/rest#WarningHandler).
Each resource operation gets its own dedicated WarningHandler (per-resource basis) that writes the
warning to that operation command's stderr. client-go folds the captured warning into the message
with a `Warning:` prefix, and it is preserved, together with the normal output, as that resource's
sync message.

The Argo CD UI already displays sync results on a per-resource basis, so the warning message can be
surfaced there as part of the result. In addition, a new result code `SyncedWithWarning` is assigned
to messages containing `Warning:`, indicating that the sync itself succeeded even though warnings
were emitted. For application-level aggregation, a new operation phase `Warning` is added, which is
treated as a successful state (`Successful()` returns true). This lets users see at a glance whether
any warnings were emitted, directly from the UI.

### Use cases

Cluster administrators and platform teams often delegate Argo CD permissions to developers, letting
them deploy applications in a self-service manner. This feature is primarily aimed at such cluster
administrators and platform teams.

#### Use case 1:

Ahead of a Kubernetes cluster version upgrade, there is no longer any need to individually notify
developers to update manifests that use deprecated APIs. Developers can notice the deprecated API
warning messages shown in the Argo CD UI on their own and update their manifests accordingly.

#### Use case 2:

Cluster administrators and platform teams use Validating Admission Webhooks/Policies to enforce
guardrails for cluster users. These guardrails also encode best practices for improving the security
and stability of applications.

In particular, when rolling out a security guardrail via a Validating Admission Webhook/Policy, it is
common to first run it in a warn-only mode (returning warnings without rejecting requests) so that
existing deployments are not broken. Once the administrator has confirmed, for example through the
Kubernetes API server's audit logs, that no new violations (warnings) are being emitted, they switch
the rule to reject resource creation/update.

With this feature, developers can see the warnings raised during this warn-only period directly in
the Argo CD UI and address them on their own, before the rule is switched to enforcement.

### Implementation Details/Notes/Constraints

#### Capturing API server warnings per resource

Argo CD never talks to the API server directly when it changes a resource. Every create, update, and
delete goes through gitops-engine's `ResourceOperations` interface, implemented by
`kubectlResourceOperations`.

```go
// ResourceOperations provides methods to manage k8s resources
type ResourceOperations interface {
	ApplyResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, force, validate, serverSideApply bool, manager string) (string, error)
	ReplaceResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, force bool) (string, error)
	CreateResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, validate bool) (string, error)
	UpdateResource(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy) (*unstructured.Unstructured, error)
}
```

These operations fall into two distinct client paths:

- **Create / Apply / Replace** run kubectl's own command libraries
  (`k8s.io/kubectl/pkg/cmd/{apply,create,replace}`) in-process. Argo CD writes the target cluster's
  `rest.Config` to a temporary kubeconfig, builds a `cmdutil.Factory` from it (`kubeCmdFactory`),
  and runs the corresponding `*Options.Run(...)`.
- **Update / Delete / Prune** use the client-go dynamic client (`dynamic.NewForConfig`). Standalone
  deletion is handled by `KubectlCmd.DeleteResource`, and pruning during apply uses the dynamic
  client wired into `ApplyOptions`.

Admission warnings only come back on write requests (create, update, apply, replace, delete), in the
`Warning` HTTP response header. Both client paths above send write requests, so whatever captures
warnings has to cover both.

##### Why each operation needs its own handler

client-go lets you register a `WarningHandler` that is called for every warning header a client
receives. The natural first idea is to register one handler on the shared `ResourceOperations` and
let it collect everything. It's worth seeing why that doesn't work, because the reason shapes the
rest of the design.

During a sync, Argo CD applies many resources at the same time. `processCreateTasks` starts one
goroutine per resource, and all of them share the same `ResourceOperations`. Several applies are
running together, so a shared handler would hear from all of them at once.

The catch is that a `WarningHandler` has no way of knowing which apply a given warning belongs to.
The callback only receives the warning text:

```go
HandleWarningHeader(code int, agent string, text string)
```

There is a context-aware variant, `HandleWarningHeaderWithContext`, that would in principle let us
pass a per-resource key along with the request. But kubectl's apply code issues its requests with an
empty `context.TODO()`, so nothing we attach ever reaches the handler. A single shared handler ends
up with one interleaved stream of warnings from many concurrent applies and no way to tell them
apart.

So instead of sharing, each operation gets its own handler that writes into a small stderr buffer
owned by that one call. Because the handler and its buffer belong to a single apply, its warnings can
never be confused with another resource's, no matter how many run in parallel:

```go
func (k *kubectlResourceOperations) runResourceCommand(...) (string, error) {
	// ...
	// One handler per operation, writing into this command's own stderr buffer.
	stderrBuf := &bytes.Buffer{}
	var warningHandler rest.WarningHandler
	if k.outputMode == outputModeLog { // server-side diff never surfaces warnings, so it gets none
		warningHandler = rest.NewWarningWriter(stderrBuf, rest.WarningWriterOptions{Deduplicate: true})
	}
	// ...
}
```

##### Delivering the handler to the real requests

Creating a handler is only half the job. It still has to reach the client that actually sends the
request. client-go makes this simple. When you set `WarningHandler` on a `rest.Config` and build a
client from that config, the client copies the handler onto itself and applies it to every request it
sends. Putting the handler on the `rest.Config` a client is built from is therefore the supported way
to capture warnings. (It's also why a proposal to add a separate `WarningHandler` field to kubectl's
apply options was turned down, because the config already carries it, and a nil-defaulted field
would only risk erasing a handler that was already set. See
[kubernetes/kubernetes#126051](https://github.com/kubernetes/kubernetes/pull/126051#discussion_r1678696327).)

For the dynamic client (Update, Delete, Prune) this is direct. Copy the shared `rest.Config`, set the
handler on the copy, and build the dynamic client from the copy. Copying leaves the shared config
untouched so parallel operations don't interfere with one another.

The kubectl command path (Create, Apply, Replace) takes one more step, because it doesn't build
clients straight from a `rest.Config`. It goes through a `cmdutil.Factory`, and that factory also owns
expensive machinery we don't want to rebuild for every resource, namely the discovery client, the REST
mapper, and the parsed OpenAPI schema.

A `cmdutil.Factory` reaches the cluster through a `RESTClientGetter`. That small interface has a few
methods. `ToRESTConfig` returns the config clients are built from, while `ToRESTMapper` and
`ToDiscoveryClient` hand back the shared caches. We wrap the shared getter and override only
`ToRESTConfig`, so it returns a config copy that carries this operation's handler. Every other method
falls through to the shared getter, so the caches are reused rather than rebuilt:

```go
// warningRESTClientGetter wraps the shared getter so the configs it hands out
// carry a per-operation warning handler, while discovery and mapper lookups
// still come from the shared caches.
type warningRESTClientGetter struct {
	genericclioptions.RESTClientGetter // the shared getter; caches live here
	warningHandler rest.WarningHandler
}

func (g *warningRESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	config, err := g.RESTClientGetter.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	if g.warningHandler == nil {
		return config, nil
	}
	config = rest.CopyConfig(config) // copy so the shared config is never mutated
	config.WarningHandler = g.warningHandler
	return config, nil
}
```

`warningClients` ties the two paths together:

```go
func (k *kubectlResourceOperations) warningClients(warningHandler rest.WarningHandler) (cmdutil.Factory, *rest.Config) {
	if warningHandler == nil {
		return k.fact, k.config // server-side diff: no handler, reuse everything as-is
	}
	// Create / Apply / Replace: a factory that reuses the shared caches but injects the handler.
	getter := &warningRESTClientGetter{RESTClientGetter: k.fact, warningHandler: warningHandler}
	fact := cmdutil.NewFactory(getter)
	// Update / Delete / Prune: a config copy the dynamic client is built from.
	cfg := rest.CopyConfig(k.config)
	cfg.WarningHandler = warningHandler
	return fact, cfg
}
```

Because the wrapped factory keeps sharing the discovery client, REST mapper, and OpenAPI schema, the
only extra memory an operation allocates is a small stderr buffer and a shallow config copy. That
holds even when a single Application syncs thousands of resources, or when many Applications sync at
the same time.

#### Turning captured warnings into a result code

By the time an operation returns, client-go has folded any captured warnings into its message with a
`Warning:` prefix. `applyObject` uses that prefix to flag the resource with the new
`SyncedWithWarning` result code, while keeping the full message (normal output plus warnings) intact:

```go
func (sc *syncContext) applyObject(ctx context.Context, t *syncTask, dryRun, validate bool) (common.ResultCode, string) {
	// ...

	if strings.Contains(message, "Warning:") {
		return common.ResultCodeSyncedWithWarning, message
	}
	return common.ResultCodeSynced, message
}
```

This `SyncedWithWarning` result code is what the UI keys off to surface warnings per resource.

#### Excluding kubectl's own client-side warnings

Not every line that starts with `Warning:` comes from the API server. While applying a resource,
kubectl also emits its own **client-side** warnings, and those must be told apart from the API server
warnings this proposal is about. There are two distinct sources:

- **API server warnings** arrive over the wire, in the HTTP `Warning` response header, and are handed
  to the `WarningHandler` we install. These are the ones we want to surface (Validating Admission
  Webhooks/Policies, deprecated APIs).
- **kubectl client-side warnings** are printed by kubectl itself, straight to its stderr, and never
  reach the `WarningHandler`. The most common one shows up when Argo CD applies RBAC resources. It
  first runs `kubectl auth reconcile` (which creates the object without the
  `last-applied-configuration` annotation) and then `kubectl apply`, so apply prints
  `Warning: resource ... is missing the ... last-applied-configuration annotation ...`.

##### How a client-side warning caused a false `SyncedWithWarning`

Originally the resource operations layer folded kubectl's stdout **and** stderr into the per-resource
message, and the warning handler wrote into that same stderr buffer. API server warnings and
kubectl's client-side warnings therefore ended up mixed in one message. Because the result-code check
was a plain `strings.Contains(message, "Warning:")`, a client-side warning alone was enough to flag a
resource as `SyncedWithWarning` even though the API server never returned anything. The RBAC
create-then-apply above is exactly that case. The sync succeeds cleanly, yet the resource was reported
with a warning.

##### kubectl's stderr only carries warnings, never results

On a successful operation, kubectl writes only warnings to its stderr. The real result
(`configured`, `created`, `unchanged`, the object JSON) is always written to stdout,
never stderr. Everything kubectl sends to stderr is a warning about its own client-side apply
mechanics:

- the missing `last-applied-configuration` annotation (create-then-apply),
- client-side-apply to server-side-apply migration notices,
- "error calculating patch from the OpenAPI spec" fallbacks,
- a notice when apply would prune/delete an object.

None of these is feedback from the API server. So leaving stderr out of the message can only ever drop
a client-side warning. It can never drop a result or other useful output.

##### Separating the warning buffer from kubectl's stderr

Give the warning handler its own buffer, kept separate from kubectl's stderr. The per-resource message
is then built from stdout plus the API server warnings only, and kubectl's client-side stderr is left
out:

```go
func (k *kubectlResourceOperations) runResourceCommand(_ context.Context, obj *unstructured.Unstructured, executor commandExecutor) (string, error) {
	// ...

	warningBuf := &bytes.Buffer{} // API server warnings (via the Warning response header)
	stderrBuf := &bytes.Buffer{}  // kubectl's own client-side output
	// ...
	warningHandler = rest.NewWarningWriter(warningBuf, rest.WarningWriterOptions{Deduplicate: true})
	// ...
	return k.handleLogOutput(stdout, warnings) // message = stdout + API server warnings only
}
```

With the streams separated, the `strings.Contains(message, "Warning:")` check now only ever sees
genuine API server warnings, so a resource is flagged `SyncedWithWarning` only when the API server
actually returned one. The RBAC create-then-apply no longer trips it.

##### Only client-side warnings are dropped, and none is actionable today

This is deliberately narrow. The only thing removed from the message is kubectl's client-side
warnings, and as of today none of them is something Argo CD needs to surface. They are all about
kubectl's internal apply behavior, not the state of the resource. If a client-side warning that users
should see is ever introduced, this can be revisited (for example by logging it separately).

The failure path already behaved this way. When an operation fails, the message comes from the
returned error (`err.Error()`), not from stderr, so these client-side warnings were never part of a
`SyncFailed` message. This change only makes the success path consistent with that.

#### Capturing warnings for RBAC resources under client-side apply

RBAC resources take a slightly different path, so it is worth confirming that their warnings are still
captured.

RBAC objects such as `RoleBinding` and `ClusterRoleBinding` have an immutable `roleRef` field that a
plain `kubectl apply` cannot change on its own. To handle that, when server-side apply is disabled and
the resource is an `rbac.authorization.k8s.io/v1` type, Argo CD runs two commands in order:

1. `kubectl auth reconcile` reconciles the RBAC object, recreating it if the immutable `roleRef`
   has to change. This step deliberately runs **without** a warning handler.
2. `kubectl apply` performs the normal create/update. This step runs **with** the per-operation
   warning handler that `runResourceCommand` installs.

Admission webhooks and policies run when the resource is applied in step 2, and because the handler is
attached to that step, their warnings are captured for RBAC resources just like for any other type.
The `auth reconcile` in front of it only deals with the immutable field and does not change this.

### Detailed examples

### Security Considerations

### Risks and Mitigations

#### Warnings can disappear on repeated syncs (client-side apply)

Admission webhooks and policies only run when there is an actual create or update. With client-side
apply, `kubectl apply` sends nothing to the API server for a resource that is already up to date
(reported as `unchanged`), so there is nothing to admit and no warning comes back.

The practical effect is that warnings shown on the first sync (when the resources were actually
created or changed) do not reappear if you sync the same, already-Synced Application again. Argo CD
does not persist the warnings. They exist only at the moment a resource is written.

Server-side apply behaves differently. It sends the desired state to the API server on every sync, so
the webhooks and policies run each time and the warnings are reported consistently. If you need
warnings to show on every sync, enable server-side apply for the Application.

### Upgrade / Downgrade Strategy

#### Older CLIs treat a `Warning` phase as a failure

This proposal adds a new `Warning` operation phase. The server treats it as a successful state
(`Successful()` returns true), but an older `argocd` CLI does not know about the new phase. Its
`Successful()` check does not include `Warning`, so it reads the phase as a failure and exits with a
non-zero status.

Running `argocd app sync` from an older CLI against an updated server therefore prints the sync result
correctly, but then exits non-zero on the final line:

```text
GROUP  KIND        NAMESPACE  NAME          STATUS  HEALTH   HOOK  MESSAGE
       Service     default    guestbook-ui  Synced  Healthy        service/guestbook-ui created. Warning: policy warn-always-...: Warning for Service/guestbook-ui
apps   Deployment  default    guestbook-ui  Synced  Healthy        deployment.apps/guestbook-ui created. Warning: policy warn-always-...: Warning for Deployment/guestbook-ui

Operation:          Sync
Phase:              Warning
Message:            synced with warnings (all tasks run)
{"level":"fatal","msg":"Operation has completed with phase: Warning","time":"2026-07-22T15:39:32+09:00"}
```

To avoid this, upgrade the `argocd` CLI to a version that matches the server. Until then, a `Warning`
phase from a newer server will read as a failed sync on the older CLI, even though the sync itself
succeeded.

#### Gating the `Warning` phase behind a controller setting

To roll the new phase out without breaking existing clients, the application-controller can expose a
setting that gates whether an operation is allowed to report the `Warning` phase. It defaults to off,
and a later major version can flip the default to on once clients have had time to upgrade.

The setting only needs to gate the application-level `Warning` phase, because that is the sole part of
this proposal that breaks an older client. The per-resource warning message and the
`SyncedWithWarning` result code stay on regardless. An older CLI or UI shows them as an unknown status
string without failing, so users still see warnings per resource even while the gate is off. With the
gate off, an operation that has resource warnings still aggregates to `Succeeded` at the Application
level, which keeps an older CLI working.

A likely home for the setting is `argocd-cm`, for example `application.sync.warningStatus.enabled`
defaulting to `"false"`. A more conservative option is to gate the whole feature (the warning message
and result code included) behind the same setting, but that hides warnings entirely until the gate is
turned on, so the feature delivers no value by default.

## Drawbacks

## Alternatives

Two earlier approaches were tried before the design above. Both are recorded here with the reason we
moved on.

### A single shared warning handler

The first attempt ([argoproj/gitops-engine#493](https://github.com/argoproj/gitops-engine/pull/493))
registered one warning handler on the shared `ResourceOperations` and collected everything through
it. As explained in
[Why each operation needs its own handler](#why-each-operation-needs-its-own-handler), it can't tell
which resource a warning came from. Operations run concurrently on shared state, the handler callback
carries no request or resource identity, and kubectl's apply path drops the request context.

The design could in theory be rescued by keeping the shared handler and sorting each warning by the
goroutine it runs on, using a `map[goroutineID]buffer` behind a lock. It would even be correct,
because the handler runs synchronously on the same goroutine that made the request. The problem is
that Go does not expose goroutine IDs on purpose. Reading them means parsing runtime stack traces,
and the whole scheme falls apart the moment client-go handles a warning on a different goroutine.
That is far too fragile for a shared library, so we did not pursue it.

### A fresh factory rebuilt from the kubeconfig per operation

An earlier version of this proposal already gave each operation its own handler, but attached it
differently. Instead of wrapping the shared getter, it built a brand-new `cmdutil.Factory` for every
operation out of the temporary kubeconfig file on disk, and injected the handler through the
factory's `WrapConfigFn` hook.

That `WrapConfigFn` step existed only because of the trip through the kubeconfig file. A
`WarningHandler` is a function value and can't be written to a file, so it had to be re-attached in
memory after the factory reloaded the config. The bigger cost was rebuilding the whole factory per
operation. Each one recreated its own discovery client and REST mapper instead of reusing the caches
the shared factory already held. In a large sync that applies many resources of the same kind at once
these per-operation factories add up quickly, and the `--kubectl-parallelism-limit` semaphore does
not help here, because it is only held around the API call itself, not around this setup.

Wrapping the shared `RESTClientGetter` keeps the per-operation handler, reuses the shared caches, and
removes the `WrapConfigFn` workaround entirely, so it replaced this approach.
