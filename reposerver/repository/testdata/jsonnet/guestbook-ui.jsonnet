local service = import 'nested/service.libsonnet';
local params = import 'params.libsonnet';

function(tlaString, tlaStringFile, tlaCode, tlaCodeFile)
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
              tlaStringFile: tlaStringFile,
              tlaCode: tlaCode,
              tlaCodeFile: tlaCodeFile,
              extVarString: std.extVar('extVarString'),
              extVarStringFile: std.extVar('extVarStringFile'),
              extVarCode: std.extVar('extVarCode'),
              extVarCodeFile: std.extVar('extVarCodeFile'),
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
  ]
