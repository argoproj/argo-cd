# UI Extensions: React 19 Upgrade

Starting with Argo CD 3.5, extensions must externalize `react/jsx-runtime`
in addition to `react` and `react-dom`. The host application exposes the
JSX runtime as `window.ReactJSXRuntime`. Extensions that bundle their own
copy of the runtime will fail to load.

## Detection

Extensions built against an older Argo CD UI fail to load with a
`TypeError`. The host UI surfaces it as:

```
Extension <name>.js failed to load: TypeError: Cannot read properties of undefined (reading '<prop>')
```

The property name in the message and the function names in the stack
trace vary depending on whether the extension's bundle is minified, so
they are not reliable to match against. The two signals that do hold in
every build are:

- the failure is a `TypeError` at load time, before the extension
  renders, and
- the extension's bundler config does not list `react/jsx-runtime` in
  its `externals` map.

Dependencies such as `antd` import `react/jsx-runtime` directly. If
`react/jsx-runtime` is not externalized, the bundler includes its own
copy of the runtime, which reaches into a React internals object that
React 19 removed, and crashes against the host's React 19 instance.

## Remediation

Add `react/jsx-runtime` to the `externals` map in your bundler config so
all JSX runtime imports resolve to the host's runtime at
`window.ReactJSXRuntime`:

```js
// webpack.config.js
externals: {
  react: 'React',
  'react-dom': 'ReactDOM',
  'react/jsx-runtime': 'ReactJSXRuntime',
  moment: 'Moment',
}
```

After rebuilding, the extension shares the host application's React 19
instance and loads successfully.

## Incompatible dependencies

Externalizing `react/jsx-runtime` resolves the most common failure mode.
Extensions that still fail to load after this change are likely depending
on a library version that is itself incompatible with React 19. In most
cases this is resolved by bumping the package to a version that supports
React 19. Most actively maintained libraries already have one.

## Reference

The following pull requests apply this fix and are useful templates for
other extension repositories:

- [argoproj-labs/argocd-ephemeral-access#141](https://github.com/argoproj-labs/argocd-ephemeral-access/pull/141) — Ephemeral Access extension.
- [argoproj-labs/rollout-extension#104](https://github.com/argoproj-labs/rollout-extension/pull/104) — Argo Rollouts extension.

## Globals exposed by the Argo CD UI

The host UI currently exposes the following modules on `window` for
extensions to consume via `externals`:

| Module              | Global            |
| ------------------- | ----------------- |
| `react`             | `React`           |
| `react-dom`         | `ReactDOM`        |
| `react/jsx-runtime` | `ReactJSXRuntime` |
| `moment`            | `Moment`          |
