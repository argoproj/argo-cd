local service = import 'vendor/nested/service.libsonnet';
local params = import 'params.libsonnet';

function(tlaString, tlaCode)
  [
    service.new(params),
    {
      apiVersion: 'apps/v1beta2',
      kind: 'Deployment',
      metadata: {
        name: params.name,
      },
      spec: {
        replicas: params.replicas,
        selector: {
          matchLabels: {
            app: params.name,
          },
        },
        template: {
          metadata: {
            labels: {
              app: params.name,
              tlaString: tlaString,
              tlaCode: tlaCode,
              extVarString: std.extVar('extVarString'),
              extVarCode: std.extVar('extVarCode'),
            },
          },
          spec: {
            containers: [
              {
                image: params.image,
                name: params.name,
                ports: [
                  {
                    containerPort: params.containerPort,
                  },
                ],
              },
            ],
          },
        },
      },
    },
    null,
  ]
