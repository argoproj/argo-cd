function(foo='bar')
    {
      apiVersion: 'v1',
      kind: 'ConfigMap',
      metadata: {
        name: 'my-map',
      },
      data: {
        foo: foo,
      }
    }
