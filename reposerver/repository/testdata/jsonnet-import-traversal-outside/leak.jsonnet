{
  apiVersion: 'v1',
  kind: 'ConfigMap',
  metadata: {
    name: 'leak',
  },
  data: {
    secret: importstr '../../../../../../../../../../../../../../../../dev/null',
  },
}
