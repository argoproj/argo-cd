local common = import '../../../include/common.libsonnet';

{
  apiVersion: 'v1',
  kind: 'Namespace',
  metadata: {
    name: common.name,
  }
}