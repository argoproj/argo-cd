function(foo='foo', bar='bar')
    {
      apiVersion: 'v1',
      kind: 'ConfigMap',
      metadata: {
        name: 'my-map',
      },
      data: {
        foo: foo,
        bar: bar,
      }
    }
