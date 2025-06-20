local base = import "base.libsonnet";
// uncomment if you reference ksonnet-lib
// local k = import "k.libsonnet";

base + {
  // Insert user-specified overrides here. For example if a component is named \"nginx-deployment\", you might have something like:\n")
  // "nginx-deployment"+: k.deployment.mixin.metadata.labels({foo: "bar"})
}
