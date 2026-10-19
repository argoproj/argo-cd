{
  apiVersion: 'v1',
  kind: 'ConfigMap',
  metadata: {
    name: 'leak',
  },
  data: {
    content: importstr '/dev/zero',
  },
}
