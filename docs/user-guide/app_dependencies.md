# Application dependencies

Argo CD lets you natively model dependencies between Applications. This mechanism is intended to replace the prior way of modeling dependencies when using the app-of-apps pattern with sync waves on the Application custom resources.

Using the new dependency mechanism, users are not bound to the app-of-apps pattern anymore and can define dependencies between applications using application selectors to define a directed, acyclic graph for dependency modeling. This means that an Application which defines one or more dependencies do not only depend on its direct dependencies, but to the dependencies of its dependencies as well.

Application dependencies support both, manual and automatic sync policies.

## Sync behaviour

Defining dependencies for an Application will affect its sync behaviour. When a sync is started, Argo CD will evaluate the sync and health status of all other Applications in its dependency tree. Only if all of its dependent Applications have a sync status of `Synced` and a health status of `Healthy`, will the sync proceed. Until all conditions are met, the sync will stay in a progressing state and a message will indicate that the sync is waiting for dependencies. This behaviour is similar to sync waves, where a sync progresses only to the next wave when the current wave's resources are all synced and healthy.

As a fallback mechanism, when using manual sync with the force option, Argo CD will not perform dependency resolution and the sync will proceed regardless of the states of the Application's dependencies.

## Feature state and caveats

This feature is in [beta state](https://github.com/argoproj/argoproj/blob/master/community/feature-status.md#beta). It will be polished across the upcoming releases.

Bug reports and enhancement requests are most welcome.

## Limitations and known issues

A list of things that are good to know before getting started with Application dependencies:

* An Application can only depend on other Applications that use the same AppProject and are in the same namespace as the dependent Application.
* A circular dependency defined anywhere in the dependency chain is considered to be an error and will prevent all applications in the circular dependency loop from syncing.
* Syncing an application that is dependent on other apps will not trigger a sync of the dependencies. However, manual sync of a dependent application whose dependencies are not satisfied will block unless force sync is used.
* Dependencies currently can only be configured in a declarative manner; the UI and CLI do not yet have first class support for configuring dependencies.

## Configuring Application dependencies

The dependencies of an Application are configured using the field `dependsOn` in the Application's spec. The field `dependsOn` is a complex data type, which lets you define one or more `selectors` to match a set of other Applications that this Application will be dependent upon.

Each selector can match Applications according to one or more properties. If you specify more than one matcher in a selector, each matcher has to match in order for an Application to be considered a dependency. For example, if you specify a `labelSelector` and a `namePattern`, both will have to match in order for an Application to be selected.

Additionally, the behaviour of dependency resolution can be configured on a per Application basis.

## Dependency selectors

Currently, the available properties to match against are `labels` and the `name` of the Application. Each selector can yield zero or more applications.

### Matching on labels

It is possible and recommended to use a standard label selector to select the dependencies for your Application:

```yaml
spec:
  dependsOn:
    selectors:
    - labelSelector:
        matchLabels:
            name: some-app
```

The above YAML will have the Application depend on any other Application that matches the given label selector. In this case, any application carrying a label `name` with the value of `some-app`.

### Matching on name patterns

Dependencies can be matched on the name of Applications by specifying the `namePattern` selector. The `namePattern` selector takes a list of Application name patterns to match against the Applications' `.metadata.name` property. Specifying multiple patterns 

```yaml
spec:
  dependsOn:
    selectors:
    - namePattern:
      - some-app
      - other-app
```

The above YAML will have the Application depend on the Applications `some-app` and `other-app`. 

The `namePattern` matcher supports shell-style glob matching, so the following example would match all Applications whose name is starting with `stage1-`:

```yaml
spec:
  dependsOn:
    selectors:
    - namePattern:
      - stage1-*
```

Regular expressions are not supported.

## Configuring behaviour of dependency resolution

There are several options to configure the behaviour of dependency resolution. Please note that these settings are not inherited to dependencies, but must be configured for each dependent uniquely.

### Delayed start of automated sync

In certain scenarios, you may want to delay the automatic sync process when there are dependencies configured between applications that are created at the same time (i.e. synced through an app of apps pattern from Git). This may allow the synchronization of all applications to succeed before the dependency tree is resolved and the sync is actually started in the right order.

To configure the sync delay, specify the number of seconds to delay in the field `syncDelay` as follows:

```yaml
spec:
  dependsOn:
    syncDelay: 5
```

Please note that any delay you configure here will delay _any_ automatic sync, regardless of whether the Application has just been created or has existed for a while.

### Block on empty dependencies

It is possible to block syncing when dependency selectors are configured, but no dependency exists yet. This may be useful in scenarios where you want to prevent an Application from being synced unless another specific Application exists.

To configure blocked syncs on empty dependencies, use the field `blockOnEmpty` as follows:

```yaml
spec:
  dependsOn:
    blockOnEmpty: true
```

Note that when you set `blockOnEmpty` to true, and sync this app through the CLI, the `argocd app sync` command will only return when the application starts syncing, i.e. when it has finished waiting or times out. If you need the CLI to return immediately, use `argocd app sync --async`.

### Configure a wait timeout

You can configure the maximum time that Argo CD should wait for dependencies to be resolved and the sync to start. The timeout also applies to the wait spent in blocking for empty dependencies.

To configure a timeout, set the field `timeout` to the number of seconds Argo CD should wait at most.

```yaml
spec:
  dependsOn:
    timeout: 10
```

When the timeout is reached, the Application's operation state will transition from `Running` into `Failed` and the sync won't proceed.
