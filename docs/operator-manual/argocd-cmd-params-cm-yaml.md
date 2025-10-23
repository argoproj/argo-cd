# argocd-cmd-params-cm.yaml example

An example of an argocd-cmd-params-cm.yaml file:

```yaml
{ !docs/operator-manual/argocd-cmd-params-cm.yaml! }
```

Argo CD optionally exposes a profiling endpoint that can be used to profile the CPU and memory usage of the Argo CD
component.
The profiling endpoint is available on metrics port of each component. See [metrics](./metrics.md) for more information
about the port.
For security reasons the profiling endpoint is disabled by default. The endpoint can be enabled by setting the
`server.profile.enabled`
or `controller.profile.enabled` key of [argocd-cmd-params-cm](argocd-cmd-params-cm.yaml) ConfigMap to `true`.
Once the endpoint is enabled you can use go profile tool to collect the CPU and memory profiles. Example: