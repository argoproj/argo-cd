{
  apiVersion: 'v1',
  kind: 'ConfigMap',
  metadata: {
    name: 'my-map',
  },
  data: {
    foo: std.extVar('foo'),
    bar: std.extVar('bar'),
  }
}
