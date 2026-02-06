local new(params) = {
  apiVersion: 'v1',
  kind: 'Service',
  metadata: {
    name: params.name,
  },
  spec: {
    ports: [
      {
        port: params.servicePort,
        targetPort: params.containerPort,
      },
    ],
    selector: {
      app: params.name,
    },
    type: params.type,
  },
};

{
  new:: new,
}
