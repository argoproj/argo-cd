export const testData = {
  nodes: [
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'knative-serving',
      name: 'config-observability',
      uid: '99c80e92-2173-4df0-90e9-a85ef733d3b5',
      resourceVersion: '18045',
      createdAt: '2020-10-02T14:33:05Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'Role',
      namespace: 'kubeflow',
      name: 'metadata-ui',
      uid: '4d846f2e-6724-4c56-9f09-3ab2792e0380',
      resourceVersion: '18882',
      createdAt: '2020-10-02T14:34:18Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'kubeflow',
      name: 'metadata-db-parameters',
      uid: '9fa7fe6a-130d-4296-9959-3be52fd3abbb',
      resourceVersion: '18051',
      createdAt: '2020-10-02T14:33:05Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'minio',
      uid: 'a5432ea0-0c43-4c32-9b11-53ce4441f59a',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'minio',
          uid: 'fb146934-e0f4-4a5a-8afc-d16a77c6bebc'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645311',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'minio-6b88d6499f',
      uid: '491e0881-0693-4142-8469-db871b5195e8',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'minio',
          uid: 'a5432ea0-0c43-4c32-9b11-53ce4441f59a'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645310',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'minio-6b88d6499f-cl27t',
      uid: '0aaf797a-f749-4b88-a47b-908936853bba',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'minio-6b88d6499f',
          uid: '491e0881-0693-4142-8469-db871b5195e8'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'minio',
          'app.kubernetes.io/component': 'minio',
          'app.kubernetes.io/instance': 'minio-0.2.5',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'minio',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.5',
          'pod-template-hash': '6b88d6499f'
        }
      },
      resourceVersion: '645309',
      images: [
        'minio/minio:RELEASE.2018-02-09T22-40-05Z'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:58Z'
    },
    {
      version: 'v1',
      kind: 'PersistentVolumeClaim',
      namespace: 'kubeflow',
      name: 'minio-pv-claim',
      uid: 'ead489f6-3a6e-467d-ad3b-82179f32498e',
      resourceVersion: '18243',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:33:11Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRoleBinding',
      name: 'ml-pipeline-persistenceagent',
      uid: '625f0daf-9741-42fd-947d-cd730919187c',
      resourceVersion: '18819',
      createdAt: '2020-10-02T14:34:11Z'
    },
    {
      group: 'networking.istio.io',
      version: 'v1alpha3',
      kind: 'VirtualService',
      namespace: 'kubeflow',
      name: 'grafana-vs',
      uid: '71ccb2d0-7b2c-4df3-b7ad-bce98d8182cd',
      resourceVersion: '21317',
      createdAt: '2020-10-02T14:41:04Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'katib-mysql-secrets',
      uid: '44701404-532c-4f86-b61d-57748e234436',
      resourceVersion: '17989',
      createdAt: '2020-10-02T14:32:55Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'routes.serving.knative.dev',
      uid: 'fcbdff8b-866b-4065-8123-4fd0d791c3f4',
      resourceVersion: '18374',
      createdAt: '2020-10-02T14:33:31Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'RoleBinding',
      namespace: 'kubeflow',
      name: 'ml-pipeline-ui',
      uid: '6df00895-66c3-438e-aa93-b1b0a1a7f894',
      resourceVersion: '18910',
      createdAt: '2020-10-02T14:34:22Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'Role',
      namespace: 'kubeflow',
      name: 'seldon-leader-election-role',
      uid: 'f757e889-11c0-487c-a919-6cad1be45370',
      resourceVersion: '18879',
      createdAt: '2020-10-02T14:34:14Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'argo-server',
      uid: '88f2885c-3d91-4f7c-a57b-3660a32363a8',
      networkingInfo: {
        targetLabels: {
          app: 'argo-server',
          'app.kubernetes.io/component': 'argo',
          'app.kubernetes.io/instance': 'argo-v2.11.2',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'argo',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v2.11.2',
          'kustomize.component': 'argo'
        }
      },
      resourceVersion: '1765722',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'argo-server',
      uid: 'fe3b2067-bda1-483c-a065-f9c4969caca3',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'argo-server',
          uid: '88f2885c-3d91-4f7c-a57b-3660a32363a8'
        }
      ],
      resourceVersion: '1766161',
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'RoleBinding',
      namespace: 'kubeflow',
      name: 'centraldashboard',
      uid: '6fb8e786-9547-4887-b404-ee64a7e3b79b',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'centraldashboard',
          uid: '4f297d44-147f-4f40-a9da-278dbf3ea42e'
        }
      ],
      resourceVersion: '22479',
      createdAt: '2020-10-02T14:34:18Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'knative-serving-podspecable-binding',
      uid: '839edbeb-a5b4-438b-a15c-fc650c661cad',
      resourceVersion: '18666',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'knative-serving',
      name: 'activator-service',
      uid: '9649a5d2-b3f9-489f-b048-0cae19bd3167',
      networkingInfo: {
        targetLabels: {
          app: 'activator',
          'app.kubernetes.io/component': 'knative-serving-install',
          'app.kubernetes.io/instance': 'knative-serving-install-v0.11.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'knative-serving-install',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v0.11.1',
          'kustomize.component': 'knative'
        }
      },
      resourceVersion: '20398',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'knative-serving',
      name: 'activator-service',
      uid: 'db944bd4-54f4-42d9-9c7f-21d632d8f79d',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'knative-serving',
          name: 'activator-service',
          uid: '9649a5d2-b3f9-489f-b048-0cae19bd3167'
        }
      ],
      resourceVersion: '1589537',
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'kfserving-controller-manager-metrics-service',
      uid: '113b3623-886b-4a11-9aff-21deee1db427',
      networkingInfo: {
        targetLabels: {
          'control-plane': 'controller-manager',
          'controller-tools.k8s.io': '1.0',
          'kustomize.component': 'kfserving'
        }
      },
      resourceVersion: '20408',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'kfserving-controller-manager-metrics-service',
      uid: '12be613a-3ab4-42a2-92fa-487373bfe59c',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'kfserving-controller-manager-metrics-service',
          uid: '113b3623-886b-4a11-9aff-21deee1db427'
        }
      ],
      resourceVersion: '20410',
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'services.serving.knative.dev',
      uid: '2e3102d3-bdbd-44a1-8926-4da000874d84',
      resourceVersion: '18390',
      createdAt: '2020-10-02T14:33:33Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'kubeflow-admin',
      uid: '6812d093-e72d-4aa4-bdb9-850914bbb509',
      resourceVersion: '1765554',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'persistent-agent',
      uid: '90682aec-5f5d-456c-9d19-e40217a34857',
      resourceVersion: '1766241',
      createdAt: '2020-10-02T14:41:25Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'ml-pipeline-persistenceagent',
      uid: 'aab19f04-bb6e-4e05-ae99-e4e1ae79d4f6',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'persistent-agent',
          uid: '90682aec-5f5d-456c-9d19-e40217a34857'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645416',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'ml-pipeline-persistenceagent-7785884886',
      uid: '59b0558a-d0b0-4da5-ba13-52240eb84940',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'ml-pipeline-persistenceagent',
          uid: 'aab19f04-bb6e-4e05-ae99-e4e1ae79d4f6'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645415',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'ml-pipeline-persistenceagent-7785884886-rl6wk',
      uid: '270125a3-8385-4e5f-afe1-ffb3919e6e02',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'ml-pipeline-persistenceagent-7785884886',
          uid: '59b0558a-d0b0-4da5-ba13-52240eb84940'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'ml-pipeline-persistenceagent',
          'app.kubernetes.io/component': 'persistent-agent',
          'app.kubernetes.io/instance': 'persistent-agent-0.2.5',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'persistent-agent',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.5',
          'pod-template-hash': '7785884886'
        }
      },
      resourceVersion: '645414',
      images: [
        'gcr.io/ml-pipeline/persistenceagent:0.2.5'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:47Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'tf-job-operator',
      uid: '8fb07bcc-28d9-4888-9015-bbd37797eac8',
      networkingInfo: {
        targetLabels: {
          'app.kubernetes.io/component': 'tfjob',
          'app.kubernetes.io/instance': 'tf-job-operator-v1.0.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'tf-job-operator',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v1.0.0',
          'kustomize.component': 'tf-job-operator',
          name: 'tf-job-operator'
        }
      },
      resourceVersion: '20451',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:15Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'tf-job-operator',
      uid: 'd89a367e-a360-42f0-8a4b-6537d60e79a6',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'tf-job-operator',
          uid: '8fb07bcc-28d9-4888-9015-bbd37797eac8'
        }
      ],
      resourceVersion: '5693461',
      createdAt: '2020-10-02T14:40:15Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'tf-job-operator',
      uid: 'ff0ff348-8a30-49b4-8ccb-f97824608105',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'tf-job-operator',
          uid: '346b82a7-fbe0-4864-9ba6-e17bce696e9a'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '5693451',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'tf-job-operator-7574b968b5',
      uid: '62c476fb-afac-4377-b231-f8a26c246ad6',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'tf-job-operator',
          uid: 'ff0ff348-8a30-49b4-8ccb-f97824608105'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '5693448',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:39Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'tf-job-operator-7574b968b5-vtqsl',
      uid: 'b20aeb52-e97f-4937-9a18-75e84e61267e',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'tf-job-operator-7574b968b5',
          uid: '62c476fb-afac-4377-b231-f8a26c246ad6'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          'app.kubernetes.io/component': 'tfjob',
          'app.kubernetes.io/instance': 'tf-job-operator-v1.0.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'tf-job-operator',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v1.0.0',
          'kustomize.component': 'tf-job-operator',
          name: 'tf-job-operator',
          'pod-template-hash': '7574b968b5'
        }
      },
      resourceVersion: '5693442',
      images: [
        'gcr.io/kubeflow-images-public/tf_operator:v1.0.0-g92389064'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:56Z'
    },
    {
      group: 'networking.istio.io',
      version: 'v1alpha3',
      kind: 'VirtualService',
      namespace: 'kubeflow',
      name: 'google-api-vs',
      uid: 'df323c92-4f08-423e-a824-bd5c7dce5c9f',
      resourceVersion: '21305',
      createdAt: '2020-10-02T14:41:01Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'kubeflow',
      name: 'katib-controller',
      uid: 'd76c22b5-ee73-446e-b0bb-56bd35ef6b3c',
      resourceVersion: '18169',
      createdAt: '2020-10-02T14:33:19Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'katib-controller-token-p9q57',
      uid: 'f00da61e-9066-433b-95a9-c361973cf95a',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'kubeflow',
          name: 'katib-controller',
          uid: 'd76c22b5-ee73-446e-b0bb-56bd35ef6b3c'
        }
      ],
      resourceVersion: '18164',
      createdAt: '2020-10-02T14:33:19Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'kubeflow',
      name: 'ui-parameters-hb792fcf5d',
      uid: '43ae5d92-8846-4303-a557-cd3ab9347a7b',
      resourceVersion: '18080',
      createdAt: '2020-10-02T14:33:09Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'knative-serving',
      name: 'config-istio',
      uid: '90e98f59-d734-47ef-93fe-2d8cafc2160d',
      resourceVersion: '18016',
      createdAt: '2020-10-02T14:33:00Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'kubeflow',
      name: 'metadata-ui-parameters',
      uid: '2a43796d-d5d4-4d93-8dec-9eeb17caffb8',
      resourceVersion: '18079',
      createdAt: '2020-10-02T14:33:09Z'
    },
    {
      group: 'networking.istio.io',
      version: 'v1alpha3',
      kind: 'VirtualService',
      namespace: 'kubeflow',
      name: 'metadata-grpc',
      uid: '69b2ae8d-b6e8-4afc-8b51-ba4bfb685ca8',
      resourceVersion: '21430',
      createdAt: '2020-10-02T14:41:17Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'kubeflow',
      name: 'notebook-controller-service-account',
      uid: '823d0680-81e8-4ad1-b3a9-b0c0782667cd',
      resourceVersion: '18168',
      createdAt: '2020-10-02T14:33:19Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'notebook-controller-service-account-token-2z8gn',
      uid: '27886e11-da00-429a-8e57-db006a85806b',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'kubeflow',
          name: 'notebook-controller-service-account',
          uid: '823d0680-81e8-4ad1-b3a9-b0c0782667cd'
        }
      ],
      resourceVersion: '18165',
      createdAt: '2020-10-02T14:33:19Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRoleBinding',
      name: 'ml-pipeline-viewer-crd-role-binding',
      uid: '2104e601-e530-4181-9f77-cec281f5ecf9',
      resourceVersion: '18795',
      createdAt: '2020-10-02T14:34:07Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'katib-controller',
      uid: '9eb3ec3d-ec82-4a97-80df-dc4b21c10a94',
      networkingInfo: {
        targetLabels: {
          app: 'katib-controller',
          'app.kubernetes.io/component': 'katib',
          'app.kubernetes.io/instance': 'katib-controller-0.8.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'katib-controller',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.8.0'
        }
      },
      resourceVersion: '1428591',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:05Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'katib-controller',
      uid: '156ea0bd-cec0-4040-a0a8-16a06b74b825',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'katib-controller',
          uid: '9eb3ec3d-ec82-4a97-80df-dc4b21c10a94'
        }
      ],
      resourceVersion: '643159',
      createdAt: '2020-10-02T14:40:05Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'katib-mysql',
      uid: '97b5b60d-f454-4ab6-b121-cb2e7fa8b528',
      networkingInfo: {
        targetLabels: {
          app: 'katib',
          'app.kubernetes.io/component': 'katib',
          'app.kubernetes.io/instance': 'katib-controller-0.8.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'katib-controller',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.8.0',
          component: 'mysql'
        }
      },
      resourceVersion: '20462',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:15Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'katib-mysql',
      uid: '7c9806fd-9e5c-4949-b043-94d8428a365d',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'katib-mysql',
          uid: '97b5b60d-f454-4ab6-b121-cb2e7fa8b528'
        }
      ],
      resourceVersion: '645181',
      createdAt: '2020-10-02T14:40:15Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'profiles.kubeflow.org',
      uid: 'd907d673-aa53-4b84-a0a7-eb436a38dd92',
      resourceVersion: '18338',
      createdAt: '2020-10-02T14:33:30Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'scheduledworkflows.kubeflow.org',
      uid: '3095ad0d-8d33-4ba2-9e96-67dd2d579b28',
      resourceVersion: '18373',
      createdAt: '2020-10-02T14:33:30Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'seldon-controller-manager',
      uid: '62f25fca-c2bb-4283-a36f-25e1a58f4571',
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '4658549',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'seldon-controller-manager-76fc795cb4',
      uid: 'e13e0ade-aca4-42a1-ad5a-70c7b67ba950',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'seldon-controller-manager',
          uid: '62f25fca-c2bb-4283-a36f-25e1a58f4571'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '4658546',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'seldon-controller-manager-76fc795cb4-vclkm',
      uid: '0f150574-8d81-4961-ac54-8d6540accfc7',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'seldon-controller-manager-76fc795cb4',
          uid: 'e13e0ade-aca4-42a1-ad5a-70c7b67ba950'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'seldon',
          'app.kubernetes.io/component': 'seldon',
          'app.kubernetes.io/instance': 'seldon-1.2.2',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'seldon-core-operator',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '1.2.2',
          'control-plane': 'seldon-controller-manager',
          'pod-template-hash': '76fc795cb4'
        }
      },
      resourceVersion: '4658544',
      images: [
        'docker.io/seldonio/seldon-core-operator:1.2.2'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:57Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'admission-webhook-cluster-role',
      uid: '414db5fd-96de-420c-ab01-e4b7df6fea5a',
      resourceVersion: '18668',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'notebooks.kubeflow.org',
      uid: 'dca4569e-1a7b-4d28-97ff-66a9b5f73acd',
      resourceVersion: '18304',
      createdAt: '2020-10-02T14:33:30Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'pipeline-runner',
      uid: '8d6d9d23-c688-4001-bae5-dbec20122403',
      resourceVersion: '18579',
      createdAt: '2020-10-02T14:33:42Z'
    },
    {
      group: 'autoscaling',
      version: 'v1',
      kind: 'HorizontalPodAutoscaler',
      namespace: 'knative-serving',
      name: 'activator',
      uid: '0375fb86-3b73-48fe-b700-683e3517193a',
      resourceVersion: '5985872',
      createdAt: '2020-10-02T14:40:51Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'kubeflow',
      name: 'pytorch-operator',
      uid: 'b0b55a91-6ef2-45cc-b541-ed2f5db80d61',
      resourceVersion: '18215',
      createdAt: '2020-10-02T14:33:20Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'pytorch-operator-token-xvp4z',
      uid: '777b2762-09d6-4c0b-81af-73079358e755',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'kubeflow',
          name: 'pytorch-operator',
          uid: 'b0b55a91-6ef2-45cc-b541-ed2f5db80d61'
        }
      ],
      resourceVersion: '18212',
      createdAt: '2020-10-02T14:33:20Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'kubeflow-kfserving-view',
      uid: 'ce0cfb0a-04de-4b98-9665-e057b27730ce',
      resourceVersion: '18691',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      group: 'apiregistration.k8s.io',
      version: 'v1',
      kind: 'APIService',
      name: 'v1beta1.custom.metrics.k8s.io',
      uid: 'fc5c09ce-a6ce-41e4-91fe-d2378918c4bd',
      resourceVersion: '4658640',
      health: {
        status: 'Healthy',
        message: 'Passed: all checks passed'
      },
      createdAt: '2020-10-02T14:40:50Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'knative-serving',
      name: 'config-domain',
      uid: '7af279e5-fb5e-4012-bde1-6f753df0c484',
      resourceVersion: '1428066',
      createdAt: '2020-10-02T14:33:04Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'knative-serving',
      name: 'config-network',
      uid: '4d9ac63b-a74f-4dd1-a63c-7d003bfd3bf2',
      resourceVersion: '18042',
      createdAt: '2020-10-02T14:33:05Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'decoratorcontrollers.metacontroller.k8s.io',
      uid: 'c40e509d-d783-4878-9b09-5904a455f248',
      resourceVersion: '18334',
      createdAt: '2020-10-02T14:33:30Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'metadata-db-secrets',
      uid: 'ea077e31-a072-4c4d-a307-6ac0f6f6477e',
      resourceVersion: '17993',
      createdAt: '2020-10-02T14:32:56Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'mysql',
      uid: '07959525-0107-4ce0-b2b1-2338e7be2bc6',
      resourceVersion: '1766236',
      createdAt: '2020-10-02T14:41:25Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'mysql',
      uid: 'a445edca-d650-4d13-9488-1fc577506b65',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'mysql',
          uid: '07959525-0107-4ce0-b2b1-2338e7be2bc6'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645098',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'mysql-7994454666',
      uid: 'aaeb0e8b-d94f-4ebb-ba60-a732c2f65d41',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'mysql',
          uid: 'a445edca-d650-4d13-9488-1fc577506b65'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645096',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:39Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'mysql-7994454666-x6fdf',
      uid: '431b48a8-4103-48ac-9dbc-cb31dcb8ba77',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'mysql-7994454666',
          uid: 'aaeb0e8b-d94f-4ebb-ba60-a732c2f65d41'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'mysql',
          'app.kubernetes.io/component': 'mysql',
          'app.kubernetes.io/instance': 'mysql-0.2.5',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'mysql',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.5',
          'pod-template-hash': '7994454666'
        }
      },
      resourceVersion: '645095',
      images: [
        'mysql:5.6'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:41Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'ml-pipeline-viewer-kubeflow-pipeline-viewers-view',
      uid: 'ad6c430b-1672-4d07-b788-c6121083ccc8',
      resourceVersion: '18746',
      createdAt: '2020-10-02T14:33:38Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'StatefulSet',
      namespace: 'kubeflow',
      name: 'admission-webhook-bootstrap-stateful-set',
      uid: '26b8cbda-dd00-4f50-8491-f95aee4b514b',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'bootstrap',
          uid: '1cf03ac5-96d8-4787-8059-981efa1edcb1'
        }
      ],
      resourceVersion: '643104',
      health: {
        status: 'Healthy',
        message: 'partitioned roll out complete: 1 new pods have been updated...'
      },
      createdAt: '2020-10-02T14:40:48Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'admission-webhook-bootstrap-stateful-set-0',
      uid: '73851268-5c3f-4190-9ca2-2728b4b6f9a7',
      parentRefs: [
        {
          group: 'apps',
          kind: 'StatefulSet',
          namespace: 'kubeflow',
          name: 'admission-webhook-bootstrap-stateful-set',
          uid: '26b8cbda-dd00-4f50-8491-f95aee4b514b'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          'app.kubernetes.io/component': 'bootstrap',
          'app.kubernetes.io/instance': 'bootstrap-v1.0.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'bootstrap',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v1.0.0',
          'controller-revision-hash': 'admission-webhook-bootstrap-stateful-set-8669cfc578',
          'kustomize.component': 'admission-webhook-bootstrap',
          'statefulset.kubernetes.io/pod-name': 'admission-webhook-bootstrap-stateful-set-0'
        }
      },
      resourceVersion: '643102',
      images: [
        'gcr.io/kubeflow-images-public/ingress-setup:latest'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:56Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ControllerRevision',
      namespace: 'kubeflow',
      name: 'admission-webhook-bootstrap-stateful-set-8669cfc578',
      uid: 'ff531b47-7024-44f6-87e9-ee3e968f0ff9',
      parentRefs: [
        {
          group: 'apps',
          kind: 'StatefulSet',
          namespace: 'kubeflow',
          name: 'admission-webhook-bootstrap-stateful-set',
          uid: '26b8cbda-dd00-4f50-8491-f95aee4b514b'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '21134',
      createdAt: '2020-10-02T14:40:48Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'katib-mysql',
      uid: '530197c3-2542-4e27-9922-76392d80dcc5',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'katib-controller',
          uid: '3e830183-bb37-4b9e-ad33-34b84f02ce2b'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645180',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:42Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'katib-mysql-74747879d7',
      uid: '24645d8e-67ca-4587-aa12-ba0aeab99d4a',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'katib-mysql',
          uid: '530197c3-2542-4e27-9922-76392d80dcc5'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645179',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:42Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'katib-mysql-74747879d7-tqr2s',
      uid: '060571b8-a405-4574-99c9-d0e9f2d1902b',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'katib-mysql-74747879d7',
          uid: '24645d8e-67ca-4587-aa12-ba0aeab99d4a'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'katib',
          'app.kubernetes.io/component': 'katib',
          'app.kubernetes.io/instance': 'katib-controller-0.8.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'katib-controller',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.8.0',
          component: 'mysql',
          'pod-template-hash': '74747879d7'
        }
      },
      resourceVersion: '645178',
      images: [
        'mysql:8'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:52Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'istio-system',
      name: 'istio-multi',
      uid: 'f476d721-5765-498c-9307-9ee28ecd7b77',
      resourceVersion: '18118',
      createdAt: '2020-10-02T13:34:57Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'istio-system',
      name: 'istio-multi-token-bvl22',
      uid: '74ea76fc-9d0a-42b4-9a5f-03e17c9a0a1a',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'istio-system',
          name: 'istio-multi',
          uid: 'f476d721-5765-498c-9307-9ee28ecd7b77'
        }
      ],
      resourceVersion: '2056',
      createdAt: '2020-10-02T13:34:58Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'jupyter-web-app-cluster-role',
      uid: '9253b4d3-6ed2-4d33-a347-6522aafaf276',
      resourceVersion: '18720',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'knative-serving',
      name: 'config-autoscaler',
      uid: '11ed33a9-dc5a-4ff2-b25e-a5be2c945618',
      resourceVersion: '18038',
      createdAt: '2020-10-02T14:33:04Z'
    },
    {
      group: 'caching.internal.knative.dev',
      version: 'v1alpha1',
      kind: 'Image',
      namespace: 'knative-serving',
      name: 'queue-proxy',
      uid: '2ca29a13-77c2-421d-9aa4-238471093862',
      resourceVersion: '21507',
      createdAt: '2020-10-02T14:41:25Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'compositecontrollers.metacontroller.k8s.io',
      uid: '3b73110f-5ab3-468f-b5c1-2a53eb050db3',
      resourceVersion: '18329',
      createdAt: '2020-10-02T14:33:30Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'pipelines-runner',
      uid: '4b23e914-7aa9-4704-908f-9c12f44ae162',
      resourceVersion: '1766244',
      createdAt: '2020-10-02T14:41:25Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'kubeflow',
      uid: '67eb2bec-c8b5-425f-9042-8347aad28960',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'kubeflow',
          uid: '67eb2bec-c8b5-425f-9042-8347aad28960'
        }
      ],
      resourceVersion: '642604',
      createdAt: '2020-10-02T14:41:16Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'kubeflow',
      uid: '67eb2bec-c8b5-425f-9042-8347aad28960',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'kubeflow',
          uid: '67eb2bec-c8b5-425f-9042-8347aad28960'
        }
      ],
      resourceVersion: '642604',
      createdAt: '2020-10-02T14:41:16Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRoleBinding',
      name: 'centraldashboard',
      uid: '1b51180f-1887-4d12-9df3-17dabe0d15d2',
      resourceVersion: '18826',
      createdAt: '2020-10-02T14:34:02Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'tf-job-operator',
      uid: '346b82a7-fbe0-4864-9ba6-e17bce696e9a',
      resourceVersion: '643826',
      createdAt: '2020-10-02T14:41:38Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'tf-job-operator',
      uid: 'ff0ff348-8a30-49b4-8ccb-f97824608105',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'tf-job-operator',
          uid: '346b82a7-fbe0-4864-9ba6-e17bce696e9a'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '5693451',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'tf-job-operator-7574b968b5',
      uid: '62c476fb-afac-4377-b231-f8a26c246ad6',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'tf-job-operator',
          uid: 'ff0ff348-8a30-49b4-8ccb-f97824608105'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '5693448',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:39Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'tf-job-operator-7574b968b5-vtqsl',
      uid: 'b20aeb52-e97f-4937-9a18-75e84e61267e',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'tf-job-operator-7574b968b5',
          uid: '62c476fb-afac-4377-b231-f8a26c246ad6'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          'app.kubernetes.io/component': 'tfjob',
          'app.kubernetes.io/instance': 'tf-job-operator-v1.0.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'tf-job-operator',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v1.0.0',
          'kustomize.component': 'tf-job-operator',
          name: 'tf-job-operator',
          'pod-template-hash': '7574b968b5'
        }
      },
      resourceVersion: '5693442',
      images: [
        'gcr.io/kubeflow-images-public/tf_operator:v1.0.0-g92389064'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:56Z'
    },
    {
      group: 'cert-manager.io',
      version: 'v1',
      kind: 'Issuer',
      namespace: 'kubeflow',
      name: 'seldon-selfsigned-issuer',
      uid: 'bdfe9f0a-0b60-4baf-b713-f50bea3d9649',
      resourceVersion: '21554',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:41:29Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'tf-job-crds',
      uid: '1085ce03-a35d-4839-9b80-2ba082f0e324',
      resourceVersion: '22398',
      createdAt: '2020-10-02T14:41:38Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'metadata-envoy-deployment',
      uid: 'b22810ce-0f15-4fa2-b35b-e05bb2a04af2',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'metadata',
          uid: '6c38734d-08e3-42ff-9cfc-2ae6e29fb34b'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '641654',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'metadata-envoy-deployment-5b9f9466d9',
      uid: 'c0671b3c-6be3-44c1-b10d-65f95e7d6435',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'metadata-envoy-deployment',
          uid: 'b22810ce-0f15-4fa2-b35b-e05bb2a04af2'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '641652',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'metadata-envoy-deployment-5b9f9466d9-ln8wb',
      uid: '1246c288-3dc0-4145-aa79-5522b9c56d7e',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'metadata-envoy-deployment-5b9f9466d9',
          uid: 'c0671b3c-6be3-44c1-b10d-65f95e7d6435'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          'app.kubernetes.io/component': 'metadata',
          'app.kubernetes.io/instance': 'metadata-0.2.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'metadata',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.1',
          component: 'envoy',
          'kustomize.component': 'metadata',
          'pod-template-hash': '5b9f9466d9'
        }
      },
      resourceVersion: '640588',
      images: [
        'gcr.io/ml-pipeline/envoy:metadata-grpc'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:33:37Z'
    },
    {
      group: 'networking.istio.io',
      version: 'v1alpha3',
      kind: 'VirtualService',
      namespace: 'kubeflow',
      name: 'tensorboard',
      uid: 'f772a2ec-f57a-48f4-be78-4f355ea2758b',
      resourceVersion: '21608',
      createdAt: '2020-10-02T14:41:37Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRoleBinding',
      name: 'katib-controller',
      uid: '580f5690-7c9f-4995-8d57-45dcea22b5e1',
      resourceVersion: '18834',
      createdAt: '2020-10-02T14:34:02Z'
    },
    {
      group: 'admissionregistration.k8s.io',
      version: 'v1',
      kind: 'ValidatingWebhookConfiguration',
      name: 'config.webhook.serving.knative.dev',
      uid: 'b9653106-970d-4f29-989f-a3d36799102e',
      resourceVersion: '1766051',
      createdAt: '2020-10-02T14:40:59Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'ml-pipeline-viewer-controller-role',
      uid: 'ffb76941-66e8-40da-98ac-fd53e925937e',
      resourceVersion: '18581',
      createdAt: '2020-10-02T14:33:42Z'
    },
    {
      group: 'admissionregistration.k8s.io',
      version: 'v1',
      kind: 'MutatingWebhookConfiguration',
      name: 'seldon-mutating-webhook-configuration-kubeflow',
      uid: 'b34e6b03-e67a-47ad-9a1b-c8ff9ae4e7e6',
      resourceVersion: '1766271',
      createdAt: '2020-10-02T14:41:28Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'admission-webhook-service',
      uid: 'e8f3e02a-4208-4305-adc9-cd864f804210',
      networkingInfo: {
        targetLabels: {
          app: 'admission-webhook',
          'app.kubernetes.io/component': 'webhook',
          'app.kubernetes.io/instance': 'webhook-v1.0.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'webhook',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v1.0.0',
          'kustomize.component': 'admission-webhook'
        }
      },
      resourceVersion: '20364',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'admission-webhook-service',
      uid: 'e8c77477-6758-4d75-a474-aeaa0702c8f3',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'admission-webhook-service',
          uid: 'e8f3e02a-4208-4305-adc9-cd864f804210'
        }
      ],
      resourceVersion: '1548393',
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'workflows.argoproj.io',
      uid: '8ec072a9-e546-4936-9493-e5638e5d353c',
      resourceVersion: '1765320',
      createdAt: '2020-10-02T14:33:34Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'minio-service',
      uid: '4d1a6bbb-a3df-4c6e-a68f-60bc4dfb704b',
      networkingInfo: {
        targetLabels: {
          app: 'minio',
          'app.kubernetes.io/component': 'minio',
          'app.kubernetes.io/instance': 'minio-0.2.5',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'minio',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.5'
        }
      },
      resourceVersion: '20419',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:11Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'minio-service',
      uid: 'e709ee47-0a99-4170-9dae-d05c9004d150',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'minio-service',
          uid: '4d1a6bbb-a3df-4c6e-a68f-60bc4dfb704b'
        }
      ],
      resourceVersion: '645312',
      createdAt: '2020-10-02T14:40:11Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'kubeflow',
      name: 'notebook-controller-parameters',
      uid: '594618d1-6fcf-4fd2-9c99-5cfbe6585ed1',
      resourceVersion: '18078',
      createdAt: '2020-10-02T14:33:09Z'
    },
    {
      group: 'admissionregistration.k8s.io',
      version: 'v1',
      kind: 'ValidatingWebhookConfiguration',
      name: 'seldon-validating-webhook-configuration-kubeflow',
      uid: 'f080ade1-a772-41d0-ad03-537de37a5a9b',
      resourceVersion: '1766285',
      createdAt: '2020-10-02T14:41:33Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'webhook',
      uid: 'ba129460-f248-4549-af3f-e58534b427d5',
      resourceVersion: '1766313',
      createdAt: '2020-10-02T14:41:40Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRoleBinding',
      name: 'application-controller-cluster-role-binding',
      uid: '0b193ecc-3fc8-404e-ae4c-163f28ee4bc0',
      resourceVersion: '18831',
      createdAt: '2020-10-02T14:34:02Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'kubeflow',
      name: 'meta-controller-service',
      uid: '41449eee-dc90-43af-88db-7c87e8f8233c',
      resourceVersion: '18191',
      createdAt: '2020-10-02T14:33:20Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'meta-controller-service-token-xvqlc',
      uid: '10a8b85f-e36d-403f-9252-39afdceca83c',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'kubeflow',
          name: 'meta-controller-service',
          uid: '41449eee-dc90-43af-88db-7c87e8f8233c'
        }
      ],
      resourceVersion: '18187',
      createdAt: '2020-10-02T14:33:20Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'jupyter-web-app-kubeflow-notebook-ui-admin',
      uid: '063bef52-dfd2-4e27-9fce-89fd2abbfb31',
      resourceVersion: '1765415',
      createdAt: '2020-10-02T14:33:38Z'
    },
    {
      group: 'admissionregistration.k8s.io',
      version: 'v1',
      kind: 'MutatingWebhookConfiguration',
      name: 'webhook.serving.knative.dev',
      uid: '26b5163e-3407-4607-bc6c-73b9efbe48d8',
      resourceVersion: '21640',
      createdAt: '2020-10-02T14:41:41Z'
    },
    {
      group: 'networking.istio.io',
      version: 'v1alpha3',
      kind: 'VirtualService',
      namespace: 'kubeflow',
      name: 'ml-pipeline-tensorboard-ui',
      uid: '7eca7b13-021b-4b00-9cb1-6f4bcee99fb0',
      resourceVersion: '21454',
      createdAt: '2020-10-02T14:41:20Z'
    },
    {
      group: 'networking.istio.io',
      version: 'v1alpha3',
      kind: 'VirtualService',
      namespace: 'kubeflow',
      name: 'ml-pipeline-ui',
      uid: 'a971c130-2889-4b05-b3ef-5a54f10115d1',
      resourceVersion: '21453',
      createdAt: '2020-10-02T14:41:20Z'
    },
    {
      group: 'networking.istio.io',
      version: 'v1alpha3',
      kind: 'VirtualService',
      namespace: 'kubeflow',
      name: 'kfam',
      uid: 'c04d0906-91ec-4395-9084-f3804846d84c',
      resourceVersion: '21381',
      createdAt: '2020-10-02T14:41:12Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'seldon-core-operator',
      uid: '92dd06ac-c426-4844-aaaf-4f20f3608ff2',
      resourceVersion: '1766259',
      createdAt: '2020-10-02T14:41:27Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'workflow-controller-metrics',
      uid: '0cf726f4-e3ad-4534-bab4-2c526c354924',
      networkingInfo: {
        targetLabels: {
          app: 'workflow-controller',
          'app.kubernetes.io/component': 'argo',
          'app.kubernetes.io/instance': 'argo-v2.11.2',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'argo',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v2.11.2',
          'kustomize.component': 'argo'
        }
      },
      resourceVersion: '1765687',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'workflow-controller-metrics',
      uid: '43dc360c-4929-4623-833e-d67d1100e11a',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'workflow-controller-metrics',
          uid: '0cf726f4-e3ad-4534-bab4-2c526c354924'
        }
      ],
      resourceVersion: '1765886',
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      group: 'autoscaling',
      version: 'v1',
      kind: 'HorizontalPodAutoscaler',
      namespace: 'istio-system',
      name: 'cluster-local-gateway',
      uid: '5178d88e-5607-49f0-a579-27b9b42c7400',
      resourceVersion: '5987805',
      createdAt: '2020-10-02T14:40:59Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'kubeflow',
      name: 'spark-operatoroperator-sa',
      uid: '77e4e30a-3fce-42ea-a592-8f30d6e7a467',
      resourceVersion: '18170',
      createdAt: '2020-10-02T14:33:19Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'spark-operatoroperator-sa-token-h5h88',
      uid: '15595d9a-954c-44d4-b6c2-fc1661df50b4',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'kubeflow',
          name: 'spark-operatoroperator-sa',
          uid: '77e4e30a-3fce-42ea-a592-8f30d6e7a467'
        }
      ],
      resourceVersion: '18167',
      createdAt: '2020-10-02T14:33:19Z'
    },
    {
      group: 'batch',
      version: 'v1',
      kind: 'Job',
      namespace: 'kubeflow',
      name: 'spark-operatorcrd-cleanup',
      uid: '281b94b0-0d9c-4acc-bb40-b13084b40500',
      resourceVersion: '22375',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:49Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'metadata',
      uid: '6c38734d-08e3-42ff-9cfc-2ae6e29fb34b',
      resourceVersion: '5988149',
      createdAt: '2020-10-02T14:41:16Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'metadata-grpc-deployment',
      uid: '1a8f512a-76f2-49a3-a49a-77d3b3142d85',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'metadata',
          uid: '6c38734d-08e3-42ff-9cfc-2ae6e29fb34b'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '646648',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:19Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'metadata-grpc-deployment-74f69954dc',
      uid: '8ed288c5-3662-4ba6-8e09-3bc9f0da6033',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'metadata-grpc-deployment',
          uid: '1a8f512a-76f2-49a3-a49a-77d3b3142d85'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '646647',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:19Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'metadata-grpc-deployment-74f69954dc-w9lgv',
      uid: '1b7ec359-a6be-4632-9485-f7528275a5f8',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'metadata-grpc-deployment-74f69954dc',
          uid: '8ed288c5-3662-4ba6-8e09-3bc9f0da6033'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          'app.kubernetes.io/component': 'metadata',
          'app.kubernetes.io/instance': 'metadata-0.2.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'metadata',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.1',
          component: 'grpc-server',
          'kustomize.component': 'metadata',
          'pod-template-hash': '74f69954dc'
        }
      },
      resourceVersion: '646646',
      images: [
        'gcr.io/tfx-oss-public/ml_metadata_store_server:v0.21.1'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:33:37Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'metadata-db',
      uid: '6b4f08b7-3122-4436-8d55-5a4fc74a5d33',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'metadata',
          uid: '6c38734d-08e3-42ff-9cfc-2ae6e29fb34b'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645265',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:42Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'metadata-db-79d6cf9d94',
      uid: 'b767dbf3-b5f8-415b-9584-ae08d493c7d4',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'metadata-db',
          uid: '6b4f08b7-3122-4436-8d55-5a4fc74a5d33'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645263',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:42Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'metadata-db-79d6cf9d94-g9npq',
      uid: '425f1489-d141-4179-98b5-41322fd475e4',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'metadata-db-79d6cf9d94',
          uid: 'b767dbf3-b5f8-415b-9584-ae08d493c7d4'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          'app.kubernetes.io/component': 'metadata',
          'app.kubernetes.io/instance': 'metadata-0.2.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'metadata',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.1',
          component: 'db',
          'kustomize.component': 'metadata',
          'pod-template-hash': '79d6cf9d94'
        }
      },
      resourceVersion: '645262',
      images: [
        'mysql:8.0.3'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:56Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'metadata-deployment',
      uid: '179611a6-0b4f-482e-8eeb-5ebe5038a440',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'metadata',
          uid: '6c38734d-08e3-42ff-9cfc-2ae6e29fb34b'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '641558',
      health: {
        status: 'Progressing',
        message: 'Waiting for rollout to finish: 0 of 1 updated replicas are available...'
      },
      createdAt: '2020-10-02T14:40:43Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'metadata-deployment-5dd4c9d4cf',
      uid: '38ae6485-a556-4d99-b19f-29444ad8f4ed',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'metadata-deployment',
          uid: '179611a6-0b4f-482e-8eeb-5ebe5038a440'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '641550',
      health: {
        status: 'Progressing',
        message: 'Waiting for rollout to finish: 0 out of 1 new replicas are available...'
      },
      createdAt: '2020-10-02T14:40:43Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'metadata-deployment-5dd4c9d4cf-xqsfq',
      uid: '12424e8d-8d12-4fb6-ba57-13cffdd5e77f',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'metadata-deployment-5dd4c9d4cf',
          uid: '38ae6485-a556-4d99-b19f-29444ad8f4ed'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '0/1'
        }
      ],
      networkingInfo: {
        labels: {
          'app.kubernetes.io/component': 'metadata',
          'app.kubernetes.io/instance': 'metadata-0.2.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'metadata',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.1',
          component: 'server',
          'kustomize.component': 'metadata',
          'pod-template-hash': '5dd4c9d4cf'
        }
      },
      resourceVersion: '640392',
      images: [
        'gcr.io/kubeflow-images-public/metadata:v0.1.11'
      ],
      health: {
        status: 'Progressing'
      },
      createdAt: '2020-10-03T23:33:37Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'metadata-envoy-deployment',
      uid: 'b22810ce-0f15-4fa2-b35b-e05bb2a04af2',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'metadata',
          uid: '6c38734d-08e3-42ff-9cfc-2ae6e29fb34b'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '641654',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'metadata-envoy-deployment-5b9f9466d9',
      uid: 'c0671b3c-6be3-44c1-b10d-65f95e7d6435',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'metadata-envoy-deployment',
          uid: 'b22810ce-0f15-4fa2-b35b-e05bb2a04af2'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '641652',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'metadata-envoy-deployment-5b9f9466d9-ln8wb',
      uid: '1246c288-3dc0-4145-aa79-5522b9c56d7e',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'metadata-envoy-deployment-5b9f9466d9',
          uid: 'c0671b3c-6be3-44c1-b10d-65f95e7d6435'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          'app.kubernetes.io/component': 'metadata',
          'app.kubernetes.io/instance': 'metadata-0.2.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'metadata',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.1',
          component: 'envoy',
          'kustomize.component': 'metadata',
          'pod-template-hash': '5b9f9466d9'
        }
      },
      resourceVersion: '640588',
      images: [
        'gcr.io/ml-pipeline/envoy:metadata-grpc'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:33:37Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'metadata-ui',
      uid: '74df07ac-a406-4126-af73-6db912bcd6ea',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'metadata',
          uid: '6c38734d-08e3-42ff-9cfc-2ae6e29fb34b'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '641337',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'metadata-ui-8968fc7d9',
      uid: '0e5b803f-b0f9-4e1b-9b67-fb118694d3f3',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'metadata-ui',
          uid: '74df07ac-a406-4126-af73-6db912bcd6ea'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '641336',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'metadata-ui-8968fc7d9-zkmgq',
      uid: '1873f9a2-6f3a-4dc2-b7a0-ec58ee187d30',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'metadata-ui-8968fc7d9',
          uid: '0e5b803f-b0f9-4e1b-9b67-fb118694d3f3'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'metadata-ui',
          'app.kubernetes.io/component': 'metadata',
          'app.kubernetes.io/instance': 'metadata-0.2.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'metadata',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.1',
          'kustomize.component': 'metadata',
          'pod-template-hash': '8968fc7d9'
        }
      },
      resourceVersion: '640477',
      images: [
        'gcr.io/kubeflow-images-public/metadata-frontend:v0.1.8'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:33:34Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'kubeflow',
      name: 'ml-pipeline-ui',
      uid: '7b28fffa-dc95-4988-a4f8-c1be41af3bb0',
      resourceVersion: '18214',
      createdAt: '2020-10-02T14:33:20Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'ml-pipeline-ui-token-scjqc',
      uid: '95a28e34-b5a0-4728-94bf-2bd5bbcdc39a',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'kubeflow',
          name: 'ml-pipeline-ui',
          uid: '7b28fffa-dc95-4988-a4f8-c1be41af3bb0'
        }
      ],
      resourceVersion: '18211',
      createdAt: '2020-10-02T14:33:20Z'
    },
    {
      group: 'admissionregistration.k8s.io',
      version: 'v1',
      kind: 'MutatingWebhookConfiguration',
      name: 'admission-webhook-mutating-webhook-configuration',
      uid: '36ba7256-4bc8-4ec6-869e-4bf3f58fa5e9',
      resourceVersion: '1766082',
      createdAt: '2020-10-02T14:40:52Z'
    },
    {
      group: 'networking.istio.io',
      version: 'v1alpha3',
      kind: 'VirtualService',
      namespace: 'kubeflow',
      name: 'centraldashboard',
      uid: '0578d0dd-bcd8-4e35-b454-be98037180bd',
      resourceVersion: '21260',
      createdAt: '2020-10-02T14:40:56Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'RoleBinding',
      namespace: 'kubeflow',
      name: 'metadata-ui',
      uid: '7fb88782-ae87-48a2-9751-f74ff952a26f',
      resourceVersion: '18908',
      createdAt: '2020-10-02T14:34:22Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'knative-serving',
      name: 'config-deployment',
      uid: 'd9b4e7ed-19c6-40e3-a0f4-e2dd51888285',
      resourceVersion: '18052',
      createdAt: '2020-10-02T14:33:05Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'pipelines-ui',
      uid: '8dfd933e-7ccb-4954-9796-943a7bca6c74',
      resourceVersion: '1766234',
      createdAt: '2020-10-02T14:41:25Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'ml-pipeline-ui',
      uid: '55307a70-2a88-4af2-a15a-534e37d91a13',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'pipelines-ui',
          uid: '8dfd933e-7ccb-4954-9796-943a7bca6c74'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '642679',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'ml-pipeline-ui-5fbd94b9fb',
      uid: 'e88fd335-20f0-475d-834b-9d5a768cb58a',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'ml-pipeline-ui',
          uid: '55307a70-2a88-4af2-a15a-534e37d91a13'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '642678',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'ml-pipeline-ui-5fbd94b9fb-rblmg',
      uid: '1589bd65-9d84-4fbc-8199-43ff74071c2d',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'ml-pipeline-ui-5fbd94b9fb',
          uid: 'e88fd335-20f0-475d-834b-9d5a768cb58a'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'ml-pipeline-ui',
          'app.kubernetes.io/component': 'pipelines-ui',
          'app.kubernetes.io/instance': 'pipelines-ui-0.2.5',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'pipelines-ui',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.5',
          'pod-template-hash': '5fbd94b9fb'
        }
      },
      resourceVersion: '642677',
      images: [
        'gcr.io/ml-pipeline/frontend:0.2.5'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:57Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'kubeflow-katib-edit',
      uid: '49792838-73fa-4f7b-8aee-82d6b185c5ab',
      resourceVersion: '18719',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'kfserving-controller-manager-service',
      uid: 'edf4753d-daa6-4164-86c0-32da6cdead07',
      networkingInfo: {
        targetLabels: {
          'control-plane': 'kfserving-controller-manager',
          'controller-tools.k8s.io': '1.0',
          'kustomize.component': 'kfserving'
        }
      },
      resourceVersion: '20450',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:15Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'kfserving-controller-manager-service',
      uid: '3db9673f-1eb8-4a18-b6c4-11004ed55b13',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'kfserving-controller-manager-service',
          uid: 'edf4753d-daa6-4164-86c0-32da6cdead07'
        }
      ],
      resourceVersion: '642729',
      createdAt: '2020-10-02T14:40:15Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRoleBinding',
      name: 'kfserving-proxy-rolebinding',
      uid: '4b4bed75-4758-4222-8ff3-e7bff7032da0',
      resourceVersion: '18827',
      createdAt: '2020-10-02T14:34:02Z'
    },
    {
      group: 'admissionregistration.k8s.io',
      version: 'v1',
      kind: 'ValidatingWebhookConfiguration',
      name: 'inferenceservice.serving.kubeflow.org',
      uid: '9b45d385-c3f5-4546-b5c9-b90294245228',
      resourceVersion: '1766101',
      createdAt: '2020-10-02T14:41:05Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'serverlessservices.networking.internal.knative.dev',
      uid: '29bd122f-3efd-4c30-8b93-9d20c2308260',
      resourceVersion: '18386',
      createdAt: '2020-10-02T14:33:33Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'controllerrevisions.metacontroller.k8s.io',
      uid: '45955654-2660-4662-823a-66aee1a4e748',
      resourceVersion: '18308',
      createdAt: '2020-10-02T14:33:30Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'kubeflow',
      name: 'metadata-grpc-configmap',
      uid: '6bd975ac-63ef-4ef2-88ac-3633f1c8de31',
      resourceVersion: '18048',
      createdAt: '2020-10-02T14:33:05Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'kubeflow',
      name: 'pipeline-mysql-parameters',
      uid: '7ee9e6d5-8507-4b7a-a235-390d974904d3',
      resourceVersion: '18069',
      createdAt: '2020-10-02T14:33:08Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'admission-webhook-bootstrap-cluster-role',
      uid: 'f80d12cc-e963-42a4-8c19-1bdcf436bc89',
      resourceVersion: '18559',
      createdAt: '2020-10-02T14:33:38Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'Role',
      namespace: 'kubeflow',
      name: 'centraldashboard',
      uid: 'f5c1927a-d509-4e05-a1b6-e1e893115791',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'centraldashboard',
          uid: '4f297d44-147f-4f40-a9da-278dbf3ea42e'
        }
      ],
      resourceVersion: '22481',
      createdAt: '2020-10-02T14:34:14Z'
    },
    {
      group: 'cert-manager.io',
      version: 'v1',
      kind: 'Certificate',
      namespace: 'kubeflow',
      name: 'seldon-serving-cert',
      uid: '105d0949-b46b-40ab-b43e-66ab9697f5ce',
      resourceVersion: '21665',
      health: {
        status: 'Healthy',
        message: 'Certificate is up to date and has not expired'
      },
      createdAt: '2020-10-02T14:41:30Z'
    },
    {
      group: 'cert-manager.io',
      version: 'v1',
      kind: 'CertificateRequest',
      namespace: 'kubeflow',
      name: 'seldon-serving-cert-vv45j',
      uid: 'c5993f56-e422-46dd-a8db-092aaff8f621',
      parentRefs: [
        {
          group: 'cert-manager.io',
          kind: 'Certificate',
          namespace: 'kubeflow',
          name: 'seldon-serving-cert',
          uid: '105d0949-b46b-40ab-b43e-66ab9697f5ce'
        }
      ],
      resourceVersion: '21650',
      createdAt: '2020-10-02T14:41:41Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'kubeflow',
      name: 'inferenceservice-config',
      uid: '00b67010-0713-47d5-a55b-a8957ff8c70c',
      resourceVersion: '18068',
      createdAt: '2020-10-02T14:33:08Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'knative-serving',
      name: 'webhook-certs',
      uid: '703dddf1-380f-4a3f-b073-a38ce687e3a8',
      resourceVersion: '21309',
      createdAt: '2020-10-02T14:32:55Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'kubeflow-view',
      uid: '2695618f-37f6-43be-b998-cb3d2ff5f67e',
      resourceVersion: '1765550',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'notebook-controller-role',
      uid: '7c7cce93-9e51-4f0f-aaf8-b88c0c151d84',
      resourceVersion: '18663',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'kubeflow',
      name: 'pipeline-runner',
      uid: '50255240-cc1a-4111-9e32-db66ed843050',
      resourceVersion: '18207',
      createdAt: '2020-10-02T14:33:20Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'pipeline-runner-token-6qpsm',
      uid: '28ee453a-6769-4d48-ad88-3870c1de9df4',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'kubeflow',
          name: 'pipeline-runner',
          uid: '50255240-cc1a-4111-9e32-db66ed843050'
        }
      ],
      resourceVersion: '18202',
      createdAt: '2020-10-02T14:33:20Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRoleBinding',
      name: 'pytorch-operator',
      uid: '37c11020-3190-463f-9b73-4dbcd5202ca2',
      resourceVersion: '18832',
      createdAt: '2020-10-02T14:34:12Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'katib-db-manager',
      uid: 'f7bbc2e9-dbf9-4eb8-8d63-f2fd9b317fbc',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'katib-controller',
          uid: '3e830183-bb37-4b9e-ad33-34b84f02ce2b'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645242',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'katib-db-manager-595b5ffd88',
      uid: '1166c8aa-d378-4199-ba6d-b85c2ca9492f',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'katib-db-manager',
          uid: 'f7bbc2e9-dbf9-4eb8-8d63-f2fd9b317fbc'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645240',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'katib-db-manager-595b5ffd88-f9wjr',
      uid: '7fb78886-c26b-4e27-84a9-4381154df197',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'katib-db-manager-595b5ffd88',
          uid: '1166c8aa-d378-4199-ba6d-b85c2ca9492f'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'katib',
          'app.kubernetes.io/component': 'katib',
          'app.kubernetes.io/instance': 'katib-controller-0.8.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'katib-controller',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.8.0',
          component: 'db-manager',
          'pod-template-hash': '595b5ffd88'
        }
      },
      resourceVersion: '645239',
      images: [
        'gcr.io/kubeflow-images-public/katib/v1alpha3/katib-db-manager@sha256:0431ac5b9fd80169c71f7a70cec8607118bbc82988d08d9eef99f8f628afc772'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:43Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'katib-ui',
      uid: 'b0197b81-2971-44ac-914b-b28c32ccd640',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'katib-controller',
          uid: '3e830183-bb37-4b9e-ad33-34b84f02ce2b'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '643736',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:39Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'katib-ui-5d68b6c84b',
      uid: '995b75cb-470d-476c-863e-515efe999441',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'katib-ui',
          uid: 'b0197b81-2971-44ac-914b-b28c32ccd640'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '643735',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:40Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'katib-ui-5d68b6c84b-rjx84',
      uid: 'd282cb48-2702-4dd5-92e6-95b783268c73',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'katib-ui-5d68b6c84b',
          uid: '995b75cb-470d-476c-863e-515efe999441'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'katib',
          'app.kubernetes.io/component': 'katib',
          'app.kubernetes.io/instance': 'katib-controller-0.8.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'katib-controller',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.8.0',
          component: 'ui',
          'pod-template-hash': '5d68b6c84b'
        }
      },
      resourceVersion: '643734',
      images: [
        'gcr.io/kubeflow-images-public/katib/v1alpha3/katib-ui@sha256:783136045e37d4e71b21c1c14ef5739f9d8a997dae40a36d60a936d9545cd871'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:58Z'
    },
    {
      version: 'v1',
      kind: 'Namespace',
      name: 'knative-serving',
      uid: '1793cc97-91e3-485b-8cfe-178c6530383f',
      resourceVersion: '17965',
      createdAt: '2020-10-02T14:32:52Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'knative-serving',
      name: 'config-tracing',
      uid: '4510f54d-f8ea-4040-88ee-d0d0e6084dd0',
      resourceVersion: '18073',
      createdAt: '2020-10-02T14:33:08Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'mlpipeline-minio-artifact',
      uid: '224767e7-ffd3-4868-af0b-0d5e4f0e4be8',
      resourceVersion: '17992',
      createdAt: '2020-10-02T14:32:56Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'ml-pipeline-viewer-kubeflow-pipeline-viewers-admin',
      uid: 'da8b6465-8ec4-4ea0-bd23-717322248dde',
      resourceVersion: '1765542',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'pipelines-viewer',
      uid: '54252c04-3b9c-4666-aacd-2abe5637bb7a',
      resourceVersion: '1766239',
      createdAt: '2020-10-02T14:41:25Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'ml-pipeline-viewer-controller-deployment',
      uid: '691ea4f7-5bfd-465a-bd44-d93a26f46adf',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'pipelines-viewer',
          uid: '54252c04-3b9c-4666-aacd-2abe5637bb7a'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '642335',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:37Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'ml-pipeline-viewer-controller-deployment-69fccfff8c',
      uid: 'f59bf9a5-c016-4614-af10-f0440e22c180',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'ml-pipeline-viewer-controller-deployment',
          uid: '691ea4f7-5bfd-465a-bd44-d93a26f46adf'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '642332',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:37Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'ml-pipeline-viewer-controller-deployment-69fccfff8c-mgt7m',
      uid: '21b1375b-cfe6-40d3-8d71-8eb3835ea340',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'ml-pipeline-viewer-controller-deployment-69fccfff8c',
          uid: 'f59bf9a5-c016-4614-af10-f0440e22c180'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'ml-pipeline-viewer-crd',
          'app.kubernetes.io/component': 'pipelines-viewer',
          'app.kubernetes.io/instance': 'pipelines-viewer-0.2.5',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'pipelines-viewer',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.5',
          'pod-template-hash': '69fccfff8c'
        }
      },
      resourceVersion: '641947',
      images: [
        'gcr.io/ml-pipeline/viewer-crd-controller:0.2.5'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:34Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'kubeflow-scheduledworkflows-admin',
      uid: 'cb837606-5d78-4a38-878b-ab752134563b',
      resourceVersion: '1765535',
      createdAt: '2020-10-02T14:33:36Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'kubeflow-istio-admin',
      uid: '80107726-64fc-4b59-aa3a-249fbc7971c3',
      resourceVersion: '1765543',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'katib-ui',
      uid: '8944f697-2af5-4a5d-86ef-e4ecc4d90503',
      resourceVersion: '18727',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'seldon-manager-role-kubeflow',
      uid: '0911777c-8de3-46d9-9201-5d40d20b28b7',
      resourceVersion: '18718',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'kubeflow',
      name: 'seldon-config',
      uid: '816474ad-49a6-42e1-aea4-f395d3a6b80a',
      resourceVersion: '18057',
      createdAt: '2020-10-02T14:33:05Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'kfserving-proxy-role',
      uid: '32345f8b-bbfa-4b8c-830e-9fa4f1cd5005',
      resourceVersion: '18602',
      createdAt: '2020-10-02T14:33:36Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'pytorch-operator',
      uid: '4bd38132-2ac2-43fc-a435-ef8d645d150d',
      resourceVersion: '18610',
      createdAt: '2020-10-02T14:33:47Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'kubeflow-pytorchjobs-edit',
      uid: '8908f661-080f-48c0-8a2d-6017bdf5092c',
      resourceVersion: '18724',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'pytorch-operator',
      uid: '3f12d82a-1ed7-4e1a-8c8d-ab2e15c63cf2',
      resourceVersion: '645077',
      createdAt: '2020-10-02T14:41:24Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'pytorch-operator',
      uid: 'f2a5b85b-6f64-4b6a-960b-c46dcf8fab30',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'pytorch-operator',
          uid: '3f12d82a-1ed7-4e1a-8c8d-ab2e15c63cf2'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '5693454',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'pytorch-operator-666dd4cd49',
      uid: '75a4f706-44a4-46ac-93ea-2b90e55f0666',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'pytorch-operator',
          uid: 'f2a5b85b-6f64-4b6a-960b-c46dcf8fab30'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '5693453',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'pytorch-operator-666dd4cd49-rk2rh',
      uid: '92f968b4-fd0c-45eb-ac61-78b86e1c41a1',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'pytorch-operator-666dd4cd49',
          uid: '75a4f706-44a4-46ac-93ea-2b90e55f0666'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          'app.kubernetes.io/component': 'pytorch',
          'app.kubernetes.io/instance': 'pytorch-operator-v1.0.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'pytorch-operator',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v1.0.0',
          'kustomize.component': 'pytorch-operator',
          name: 'pytorch-operator',
          'pod-template-hash': '666dd4cd49'
        }
      },
      resourceVersion: '5693452',
      images: [
        'gcr.io/kubeflow-images-public/pytorch-operator:v1.0.0-g047cf0f'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:36:02Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'kubeflow-scheduledworkflows-edit',
      uid: 'b511bc48-7f97-432d-b5b2-c4d1180efc5b',
      resourceVersion: '18603',
      createdAt: '2020-10-02T14:33:36Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'kubeflow-scheduledworkflows-view',
      uid: '07f1155b-f22c-467f-90d4-0170b7a90239',
      resourceVersion: '18617',
      createdAt: '2020-10-02T14:33:36Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'kubeflow',
      name: 'seldon-manager',
      uid: 'a6071fa3-e7a0-4442-ae48-a1c0746b9098',
      resourceVersion: '18221',
      createdAt: '2020-10-02T14:33:21Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'seldon-manager-token-97mpw',
      uid: '9b3c1445-7bdc-40c4-9104-aab0c9e404a1',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'kubeflow',
          name: 'seldon-manager',
          uid: 'a6071fa3-e7a0-4442-ae48-a1c0746b9098'
        }
      ],
      resourceVersion: '18219',
      createdAt: '2020-10-02T14:33:21Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'RoleBinding',
      namespace: 'kubeflow',
      name: 'seldon-leader-election-rolebinding',
      uid: '6ad4a9bd-92f1-470d-9bd4-6b9173a42422',
      resourceVersion: '18907',
      createdAt: '2020-10-02T14:34:18Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'kubeflow-kfserving-admin',
      uid: 'dff7e44a-aebd-49d5-b642-5e7cbd480640',
      resourceVersion: '1765487',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'knative-serving',
      name: 'webhook',
      uid: '553dcd49-1c3c-4323-b5e8-957e0ad2b011',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'knative-serving',
          name: 'knative-serving-install',
          uid: '64044521-8f41-49ba-885c-1e4ce0b8d569'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '635617',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'knative-serving',
      name: 'webhook-576b4f987',
      uid: '8b1952fc-3af5-4951-80b4-2281a478e330',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'knative-serving',
          name: 'webhook',
          uid: '553dcd49-1c3c-4323-b5e8-957e0ad2b011'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '635615',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'knative-serving',
      name: 'webhook-576b4f987-c87q5',
      uid: 'b63eb0b1-f354-4e28-9814-7b05fc9c5677',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'knative-serving',
          name: 'webhook-576b4f987',
          uid: '8b1952fc-3af5-4951-80b4-2281a478e330'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'webhook',
          'app.kubernetes.io/component': 'knative-serving-install',
          'app.kubernetes.io/instance': 'knative-serving-install-v0.11.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'knative-serving-install',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v0.11.1',
          'kustomize.component': 'knative',
          'pod-template-hash': '576b4f987',
          role: 'webhook',
          'serving.knative.dev/release': 'v0.11.2'
        }
      },
      resourceVersion: '635614',
      images: [
        'gcr.io/knative-releases/knative.dev/serving/cmd/webhook@sha256:d07560cd5548640cc79abc819608844527351f10e8b0a847988f9eb602c18972'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:19:18Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRoleBinding',
      name: 'spark-operatorsparkoperator-crb',
      uid: '5f556271-5831-4bc9-bbe1-2c3426531c3c',
      resourceVersion: '18839',
      createdAt: '2020-10-02T14:34:03Z'
    },
    {
      group: 'rbac.istio.io',
      version: 'v1alpha1',
      kind: 'ServiceRoleBinding',
      namespace: 'knative-serving',
      name: 'istio-service-role-binding',
      uid: '06728702-8c7a-49cd-9362-290381799cb5',
      resourceVersion: '21343',
      createdAt: '2020-10-02T14:41:07Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'viewers.kubeflow.org',
      uid: 'befd388d-7df8-40c3-80c5-1561a2f5ab2c',
      resourceVersion: '18417',
      createdAt: '2020-10-02T14:33:34Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'kubeflow-istio-view',
      uid: 'dd904813-3115-4198-91d2-1fedebcbece9',
      resourceVersion: '18749',
      createdAt: '2020-10-02T14:33:38Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'inferenceservices.serving.kubeflow.org',
      uid: '5c1bb26b-2fce-44b3-bdef-592294b4935b',
      resourceVersion: '18375',
      createdAt: '2020-10-02T14:33:30Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'metadata-grpc-deployment',
      uid: '1a8f512a-76f2-49a3-a49a-77d3b3142d85',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'metadata',
          uid: '6c38734d-08e3-42ff-9cfc-2ae6e29fb34b'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '646648',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:19Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'metadata-grpc-deployment-74f69954dc',
      uid: '8ed288c5-3662-4ba6-8e09-3bc9f0da6033',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'metadata-grpc-deployment',
          uid: '1a8f512a-76f2-49a3-a49a-77d3b3142d85'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '646647',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:19Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'metadata-grpc-deployment-74f69954dc-w9lgv',
      uid: '1b7ec359-a6be-4632-9485-f7528275a5f8',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'metadata-grpc-deployment-74f69954dc',
          uid: '8ed288c5-3662-4ba6-8e09-3bc9f0da6033'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          'app.kubernetes.io/component': 'metadata',
          'app.kubernetes.io/instance': 'metadata-0.2.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'metadata',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.1',
          component: 'grpc-server',
          'kustomize.component': 'metadata',
          'pod-template-hash': '74f69954dc'
        }
      },
      resourceVersion: '646646',
      images: [
        'gcr.io/tfx-oss-public/ml_metadata_store_server:v0.21.1'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:33:37Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'minio',
      uid: 'fb146934-e0f4-4a5a-8afc-d16a77c6bebc',
      resourceVersion: '1766202',
      createdAt: '2020-10-02T14:41:18Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'minio',
      uid: 'a5432ea0-0c43-4c32-9b11-53ce4441f59a',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'minio',
          uid: 'fb146934-e0f4-4a5a-8afc-d16a77c6bebc'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645311',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'minio-6b88d6499f',
      uid: '491e0881-0693-4142-8469-db871b5195e8',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'minio',
          uid: 'a5432ea0-0c43-4c32-9b11-53ce4441f59a'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645310',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'minio-6b88d6499f-cl27t',
      uid: '0aaf797a-f749-4b88-a47b-908936853bba',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'minio-6b88d6499f',
          uid: '491e0881-0693-4142-8469-db871b5195e8'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'minio',
          'app.kubernetes.io/component': 'minio',
          'app.kubernetes.io/instance': 'minio-0.2.5',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'minio',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.5',
          'pod-template-hash': '6b88d6499f'
        }
      },
      resourceVersion: '645309',
      images: [
        'minio/minio:RELEASE.2018-02-09T22-40-05Z'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:58Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'kubeflow',
      name: 'argo-server',
      uid: 'e3e71347-baed-4612-b301-1ec45daf7361',
      resourceVersion: '1765243',
      createdAt: '2020-10-02T14:33:15Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'argo-server-token-4fgbc',
      uid: 'c37ede6a-de84-49e3-b1c1-d884ffc18e47',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'kubeflow',
          name: 'argo-server',
          uid: 'e3e71347-baed-4612-b301-1ec45daf7361'
        }
      ],
      resourceVersion: '18122',
      createdAt: '2020-10-02T14:33:15Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'images.caching.internal.knative.dev',
      uid: 'e5fe569e-6a70-4a88-8a9d-8d59b0ea2866',
      resourceVersion: '18345',
      createdAt: '2020-10-02T14:33:30Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRoleBinding',
      name: 'katib-ui',
      uid: '690556ae-8654-4c2f-97e1-d8d54188e4c8',
      resourceVersion: '18828',
      createdAt: '2020-10-02T14:34:02Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'kubeflow',
      name: 'katib-config',
      uid: 'bc966620-b747-464d-8fbd-5894a6dcd3f8',
      resourceVersion: '18023',
      createdAt: '2020-10-02T14:33:02Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'profiles-deployment',
      uid: '47f8ea51-c790-48ce-b400-47d4bdb1c73c',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'profiles',
          uid: '201e1b4c-2af7-4138-b5dc-02fb296536cb'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '643176',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'profiles-deployment-67db6957fc',
      uid: 'f8b8ff8f-8149-4d7f-9394-1d083325a5cf',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'profiles-deployment',
          uid: '47f8ea51-c790-48ce-b400-47d4bdb1c73c'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '643174',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'profiles-deployment-67db6957fc-htr9t',
      uid: 'ce2ba13f-e535-48cb-9941-6d997123ada3',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'profiles-deployment-67db6957fc',
          uid: 'f8b8ff8f-8149-4d7f-9394-1d083325a5cf'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '2/2'
        }
      ],
      networkingInfo: {
        labels: {
          'app.kubernetes.io/component': 'profiles',
          'app.kubernetes.io/instance': 'profiles-v1.0.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'profiles',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v1.0.0',
          'kustomize.component': 'profiles',
          'pod-template-hash': '67db6957fc'
        }
      },
      resourceVersion: '643173',
      images: [
        'auroraprodacr.azurecr.io/kubeflow-dev/profile-controller:v20200619-v0.7.0-rc.5-148-g253890cb-dirty-7346f1',
        'gcr.io/kubeflow-images-public/kfam:v1.0.0-gf3e09203'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:48Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'pytorch-job-crds',
      uid: '88bdca94-fd71-4c95-b3bb-60cdd62d7bf4',
      resourceVersion: '22420',
      createdAt: '2020-10-02T14:41:25Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'kubeflow',
      name: 'parameters',
      uid: '4b40ec05-cb8b-431b-a40b-8195f60e5850',
      resourceVersion: '18081',
      createdAt: '2020-10-02T14:33:09Z'
    },
    {
      group: 'networking.istio.io',
      version: 'v1beta1',
      kind: 'ServiceEntry',
      namespace: 'kubeflow',
      name: 'google-storage-api-entry',
      uid: 'd2b96006-c5d2-4ad0-83e5-a899aba0924f',
      resourceVersion: '21308',
      createdAt: '2020-10-02T14:41:02Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'RoleBinding',
      namespace: 'kubeflow',
      name: 'jupyter-web-app-jupyter-notebook-role-binding',
      uid: '32d9bf84-9a94-45ef-95ad-14d4a04a5453',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'jupyter-web-app',
          uid: '836e36d8-cf05-4c30-9705-aaf0aff02345'
        }
      ],
      resourceVersion: '22373',
      createdAt: '2020-10-02T14:34:20Z'
    },
    {
      version: 'v1',
      kind: 'PersistentVolumeClaim',
      namespace: 'kubeflow',
      name: 'katib-mysql',
      uid: '90b16d44-d8b6-417a-abb5-8bb85d1d4dc2',
      resourceVersion: '18238',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:33:11Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRoleBinding',
      name: 'hpa-controller-custom-metrics',
      uid: '5d2c5091-a434-48b7-8085-00c571248cec',
      resourceVersion: '18840',
      createdAt: '2020-10-02T14:34:03Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'metadata-ui',
      uid: '457a8a46-01c1-4488-a092-d41e52dbea64',
      networkingInfo: {
        targetLabels: {
          app: 'metadata-ui',
          'app.kubernetes.io/component': 'metadata',
          'app.kubernetes.io/instance': 'metadata-0.2.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'metadata',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.1',
          'kustomize.component': 'metadata'
        }
      },
      resourceVersion: '20415',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:11Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'metadata-ui',
      uid: '2757abb3-f251-4a10-ba83-41dcc5929e92',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'metadata-ui',
          uid: '457a8a46-01c1-4488-a092-d41e52dbea64'
        }
      ],
      resourceVersion: '640601',
      createdAt: '2020-10-02T14:40:11Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'notebook-controller',
      uid: '4c1a5510-0da6-4c84-9781-39e24ad73d97',
      resourceVersion: '643816',
      createdAt: '2020-10-02T14:41:25Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'notebook-controller-deployment',
      uid: '747d5b0b-13df-4383-88e8-01d69e65e125',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'notebook-controller',
          uid: '4c1a5510-0da6-4c84-9781-39e24ad73d97'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '642951',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:40Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'notebook-controller-deployment-798559cf4d',
      uid: 'f109eca4-b5f5-4bdd-9905-6b6e882a04e9',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'notebook-controller-deployment',
          uid: '747d5b0b-13df-4383-88e8-01d69e65e125'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '642950',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:40Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'notebook-controller-deployment-798559cf4d-qpvfw',
      uid: 'd61c9903-3bea-422b-bfd6-378e1ae3683e',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'notebook-controller-deployment-798559cf4d',
          uid: 'f109eca4-b5f5-4bdd-9905-6b6e882a04e9'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'notebook-controller',
          'app.kubernetes.io/component': 'notebook-controller',
          'app.kubernetes.io/instance': 'notebook-controller-v1.0.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'notebook-controller',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v1.0.0',
          'kustomize.component': 'notebook-controller',
          'pod-template-hash': '798559cf4d'
        }
      },
      resourceVersion: '642949',
      images: [
        'gcr.io/kubeflow-images-public/notebook-controller:v1.0.0-gcd65ce25'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:57Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'ml-pipeline',
      uid: '4fd2ebc2-e6d5-4b34-9dd3-bc835f54fe73',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'api-service',
          uid: '96f5d9d3-5639-43cf-94ce-b941f777575f'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645457',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'ml-pipeline-698bcdd747',
      uid: 'faa029eb-ab98-4196-88e2-6284f68415be',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'ml-pipeline',
          uid: '4fd2ebc2-e6d5-4b34-9dd3-bc835f54fe73'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645456',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'ml-pipeline-698bcdd747-prcpg',
      uid: '1fca6f72-0110-4934-8b43-a00d935e8251',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'ml-pipeline-698bcdd747',
          uid: 'faa029eb-ab98-4196-88e2-6284f68415be'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'ml-pipeline',
          'app.kubernetes.io/component': 'api-service',
          'app.kubernetes.io/instance': 'api-service-0.2.5',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'api-service',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.5',
          'pod-template-hash': '698bcdd747'
        }
      },
      resourceVersion: '645455',
      images: [
        'gcr.io/ml-pipeline/api-server:0.2.5'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:33:36Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'workflowtemplates.argoproj.io',
      uid: '414e18b0-6192-4c34-ac57-38e1e734de41',
      resourceVersion: '1765315',
      createdAt: '2020-10-02T14:33:29Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'kubeflow',
      name: 'katib-parameters',
      uid: '199e5861-3ef4-428a-a955-4e3044f5558a',
      resourceVersion: '18034',
      createdAt: '2020-10-02T14:33:04Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'katib-crds',
      uid: 'bcaa9e15-1640-4d8b-9fbf-9efad9676a50',
      resourceVersion: '22461',
      createdAt: '2020-10-02T14:41:11Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'configurations.serving.knative.dev',
      uid: 'f3faeb5d-a1d8-488d-a076-4fb84e14963c',
      resourceVersion: '18267',
      createdAt: '2020-10-02T14:33:25Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'knative-serving',
      name: 'controller',
      uid: 'cdd2b38a-3363-44c1-b1b0-9b2240a2f079',
      resourceVersion: '18193',
      createdAt: '2020-10-02T14:33:20Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'knative-serving',
      name: 'controller-token-mv7hr',
      uid: 'b0f2dd0e-5eea-4f52-8cce-bac4d0296acd',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'knative-serving',
          name: 'controller',
          uid: 'cdd2b38a-3363-44c1-b1b0-9b2240a2f079'
        }
      ],
      resourceVersion: '18188',
      createdAt: '2020-10-02T14:33:20Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'Role',
      namespace: 'kubeflow',
      name: 'ml-pipeline-scheduledworkflow',
      uid: '482cc477-6786-4910-a541-811aa311caf3',
      resourceVersion: '18876',
      createdAt: '2020-10-02T14:34:18Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'kubeflow',
      name: 'tf-job-dashboard',
      uid: 'a76513ec-1995-448c-b26c-9faa50d12e61',
      resourceVersion: '18229',
      createdAt: '2020-10-02T14:33:21Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'tf-job-dashboard-token-4qztx',
      uid: 'db426d6e-4f84-47f5-9b04-01bd2de6a86a',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'kubeflow',
          name: 'tf-job-dashboard',
          uid: 'a76513ec-1995-448c-b26c-9faa50d12e61'
        }
      ],
      resourceVersion: '18226',
      createdAt: '2020-10-02T14:33:21Z'
    },
    {
      group: 'networking.istio.io',
      version: 'v1alpha3',
      kind: 'VirtualService',
      namespace: 'kubeflow',
      name: 'argo-server',
      uid: 'a24758a3-b5c9-40f4-97e4-c7bd0e033f4c',
      resourceVersion: '1765985',
      createdAt: '2020-10-02T14:40:54Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'jupyter-web-app-deployment',
      uid: '374126c8-efbe-497d-9f8c-43815227742f',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'jupyter-web-app',
          uid: '836e36d8-cf05-4c30-9705-aaf0aff02345'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '643956',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'jupyter-web-app-deployment-544b7d5684',
      uid: '982c8a2f-e03e-43de-ab10-111103b8f51c',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'jupyter-web-app-deployment',
          uid: '374126c8-efbe-497d-9f8c-43815227742f'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '643955',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'jupyter-web-app-deployment-544b7d5684-skmgz',
      uid: '4cd47d9c-b653-4ddf-94b4-b735bf8e35ce',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'jupyter-web-app-deployment-544b7d5684',
          uid: '982c8a2f-e03e-43de-ab10-111103b8f51c'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'jupyter-web-app',
          'app.kubernetes.io/component': 'jupyter-web-app',
          'app.kubernetes.io/instance': 'jupyter-web-app-v1.0.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'jupyter-web-app',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v1.0.0',
          'kustomize.component': 'jupyter-web-app',
          'pod-template-hash': '544b7d5684'
        }
      },
      resourceVersion: '643953',
      images: [
        'gcr.io/kubeflow-images-public/jupyter-web-app:v1.0.0-g2bd63238'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:36:02Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'tf-job-operator',
      uid: '452dc023-4b30-46d3-8dca-afc95ad58924',
      resourceVersion: '18561',
      createdAt: '2020-10-02T14:33:38Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'RoleBinding',
      namespace: 'knative-serving',
      name: 'custom-metrics-auth-reader',
      uid: '989cc264-4d95-46d9-9a98-33f1acad1c0b',
      resourceVersion: '18911',
      createdAt: '2020-10-02T14:34:18Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'knative-serving',
      name: 'autoscaler',
      uid: 'b9747434-b37b-4521-82b2-e16b0639f57a',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'knative-serving',
          name: 'knative-serving-install',
          uid: '64044521-8f41-49ba-885c-1e4ce0b8d569'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '1548733',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'knative-serving',
      name: 'autoscaler-b5bf74846',
      uid: 'ddb58b88-ab99-41c2-a2f3-1d73cd49d8ca',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'knative-serving',
          name: 'autoscaler',
          uid: 'b9747434-b37b-4521-82b2-e16b0639f57a'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '1548731',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'knative-serving',
      name: 'autoscaler-b5bf74846-njx9h',
      uid: '2ef703fd-6c72-4a45-98ee-362d1058097a',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'knative-serving',
          name: 'autoscaler-b5bf74846',
          uid: 'ddb58b88-ab99-41c2-a2f3-1d73cd49d8ca'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '2/2'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'autoscaler',
          'app.kubernetes.io/component': 'knative-serving-install',
          'app.kubernetes.io/instance': 'knative-serving-install-v0.11.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'knative-serving-install',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v0.11.1',
          'kustomize.component': 'knative',
          'pod-template-hash': 'b5bf74846',
          'security.istio.io/tlsMode': 'istio',
          'service.istio.io/canonical-name': 'knative-serving-install',
          'service.istio.io/canonical-revision': 'v0.11.1',
          'serving.knative.dev/release': 'v0.11.2'
        }
      },
      resourceVersion: '1548730',
      images: [
        'docker.io/istio/proxyv2:1.5.10',
        'gcr.io/knative-releases/knative.dev/serving/cmd/autoscaler@sha256:998a405454832cda18a4bf956d26d610a2df2130a39b834b597a89a3153c8c15'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-05T23:43:48Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'kubeflow-pytorchjobs-admin',
      uid: '514a5e50-5cf2-4506-accc-1cf6bcc15627',
      resourceVersion: '1765419',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'centraldashboard',
      uid: '66c3165a-fd68-4dc1-9631-d0dc219d84f4',
      networkingInfo: {
        targetLabels: {
          app: 'centraldashboard',
          'app.kubernetes.io/component': 'centraldashboard',
          'app.kubernetes.io/instance': 'centraldashboard-v1.0.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'centraldashboard',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v1.0.0',
          'kustomize.component': 'centraldashboard'
        }
      },
      resourceVersion: '20456',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:15Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'centraldashboard',
      uid: '5410d069-fefa-48df-965d-4c8b4c3c6962',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'centraldashboard',
          uid: '66c3165a-fd68-4dc1-9631-d0dc219d84f4'
        }
      ],
      resourceVersion: '642509',
      createdAt: '2020-10-02T14:40:15Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'knative-serving-admin',
      uid: '381d6fcf-6c80-4d0a-9816-d2ed0b11065a',
      resourceVersion: '1765479',
      createdAt: '2020-10-02T14:33:36Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'metadata-ui',
      uid: '74df07ac-a406-4126-af73-6db912bcd6ea',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'metadata',
          uid: '6c38734d-08e3-42ff-9cfc-2ae6e29fb34b'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '641337',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'metadata-ui-8968fc7d9',
      uid: '0e5b803f-b0f9-4e1b-9b67-fb118694d3f3',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'metadata-ui',
          uid: '74df07ac-a406-4126-af73-6db912bcd6ea'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '641336',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'metadata-ui-8968fc7d9-zkmgq',
      uid: '1873f9a2-6f3a-4dc2-b7a0-ec58ee187d30',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'metadata-ui-8968fc7d9',
          uid: '0e5b803f-b0f9-4e1b-9b67-fb118694d3f3'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'metadata-ui',
          'app.kubernetes.io/component': 'metadata',
          'app.kubernetes.io/instance': 'metadata-0.2.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'metadata',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.1',
          'kustomize.component': 'metadata',
          'pod-template-hash': '8968fc7d9'
        }
      },
      resourceVersion: '640477',
      images: [
        'gcr.io/kubeflow-images-public/metadata-frontend:v0.1.8'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:33:34Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'bootstrap',
      uid: '1cf03ac5-96d8-4787-8059-981efa1edcb1',
      resourceVersion: '1765987',
      createdAt: '2020-10-02T14:40:55Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'StatefulSet',
      namespace: 'kubeflow',
      name: 'admission-webhook-bootstrap-stateful-set',
      uid: '26b8cbda-dd00-4f50-8491-f95aee4b514b',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'bootstrap',
          uid: '1cf03ac5-96d8-4787-8059-981efa1edcb1'
        }
      ],
      resourceVersion: '643104',
      health: {
        status: 'Healthy',
        message: 'partitioned roll out complete: 1 new pods have been updated...'
      },
      createdAt: '2020-10-02T14:40:48Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ControllerRevision',
      namespace: 'kubeflow',
      name: 'admission-webhook-bootstrap-stateful-set-8669cfc578',
      uid: 'ff531b47-7024-44f6-87e9-ee3e968f0ff9',
      parentRefs: [
        {
          group: 'apps',
          kind: 'StatefulSet',
          namespace: 'kubeflow',
          name: 'admission-webhook-bootstrap-stateful-set',
          uid: '26b8cbda-dd00-4f50-8491-f95aee4b514b'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '21134',
      createdAt: '2020-10-02T14:40:48Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'admission-webhook-bootstrap-stateful-set-0',
      uid: '73851268-5c3f-4190-9ca2-2728b4b6f9a7',
      parentRefs: [
        {
          group: 'apps',
          kind: 'StatefulSet',
          namespace: 'kubeflow',
          name: 'admission-webhook-bootstrap-stateful-set',
          uid: '26b8cbda-dd00-4f50-8491-f95aee4b514b'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          'app.kubernetes.io/component': 'bootstrap',
          'app.kubernetes.io/instance': 'bootstrap-v1.0.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'bootstrap',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v1.0.0',
          'controller-revision-hash': 'admission-webhook-bootstrap-stateful-set-8669cfc578',
          'kustomize.component': 'admission-webhook-bootstrap',
          'statefulset.kubernetes.io/pod-name': 'admission-webhook-bootstrap-stateful-set-0'
        }
      },
      resourceVersion: '643102',
      images: [
        'gcr.io/kubeflow-images-public/ingress-setup:latest'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:56Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'jupyter-web-app-kubeflow-notebook-ui-view',
      uid: '94471f07-f285-4422-b030-e8de7ceaf6ed',
      resourceVersion: '18657',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'kubeflow',
      name: 'spark-operatorspark-sa',
      uid: '2b6f1d25-9e9f-4558-b45a-c2e24319cc0c',
      resourceVersion: '18216',
      createdAt: '2020-10-02T14:33:20Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'spark-operatorspark-sa-token-z2qm7',
      uid: '2325b093-e2e9-4e81-9415-b306634f9db0',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'kubeflow',
          name: 'spark-operatorspark-sa',
          uid: '2b6f1d25-9e9f-4558-b45a-c2e24319cc0c'
        }
      ],
      resourceVersion: '18213',
      createdAt: '2020-10-02T14:33:20Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'centraldashboard',
      uid: '4f297d44-147f-4f40-a9da-278dbf3ea42e',
      resourceVersion: '5988134',
      createdAt: '2020-10-02T14:40:57Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'Role',
      namespace: 'kubeflow',
      name: 'centraldashboard',
      uid: 'f5c1927a-d509-4e05-a1b6-e1e893115791',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'centraldashboard',
          uid: '4f297d44-147f-4f40-a9da-278dbf3ea42e'
        }
      ],
      resourceVersion: '22481',
      createdAt: '2020-10-02T14:34:14Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'RoleBinding',
      namespace: 'kubeflow',
      name: 'centraldashboard',
      uid: '6fb8e786-9547-4887-b404-ee64a7e3b79b',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'centraldashboard',
          uid: '4f297d44-147f-4f40-a9da-278dbf3ea42e'
        }
      ],
      resourceVersion: '22479',
      createdAt: '2020-10-02T14:34:18Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'centraldashboard',
      uid: '1276a511-dc0d-435d-b712-6a99db800cd0',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'centraldashboard',
          uid: '4f297d44-147f-4f40-a9da-278dbf3ea42e'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '642510',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'centraldashboard-754dc648d8',
      uid: 'd028a239-07d3-4f83-924c-70df38c8237f',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'centraldashboard',
          uid: '1276a511-dc0d-435d-b712-6a99db800cd0'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '642508',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'centraldashboard-754dc648d8-vblks',
      uid: 'bdec4e71-bfbd-489d-8970-f2297ce8543f',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'centraldashboard-754dc648d8',
          uid: 'd028a239-07d3-4f83-924c-70df38c8237f'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'centraldashboard',
          'app.kubernetes.io/component': 'centraldashboard',
          'app.kubernetes.io/instance': 'centraldashboard-v1.0.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'centraldashboard',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v1.0.0',
          'kustomize.component': 'centraldashboard',
          'pod-template-hash': '754dc648d8'
        }
      },
      resourceVersion: '642507',
      images: [
        'auroraprodacr.azurecr.io/gsantomaggio/centraldashboard:node-12'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:46Z'
    },
    {
      group: 'admissionregistration.k8s.io',
      version: 'v1',
      kind: 'MutatingWebhookConfiguration',
      name: 'inferenceservice.serving.kubeflow.org',
      uid: '097e9f5d-ca47-46a2-a26c-768b70d3e65a',
      resourceVersion: '1766098',
      createdAt: '2020-10-02T14:41:04Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'istio-system',
      name: 'cluster-local-gateway',
      uid: '3ccda13e-fb52-48d9-a7a8-9a72df12285d',
      networkingInfo: {
        targetLabels: {
          app: 'cluster-local-gateway',
          istio: 'cluster-local-gateway',
          'kustomize.component': 'cluster-local-gateway'
        }
      },
      resourceVersion: '20445',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:15Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'istio-system',
      name: 'cluster-local-gateway',
      uid: '4b3d3cb7-8ed2-4f7c-8cc1-650a338af9d5',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'istio-system',
          name: 'cluster-local-gateway',
          uid: '3ccda13e-fb52-48d9-a7a8-9a72df12285d'
        }
      ],
      resourceVersion: '1853724',
      createdAt: '2020-10-02T14:40:15Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'kubeflow',
      name: 'katib-ui',
      uid: 'e5646adf-0232-4d01-81b8-8fffdd383e06',
      resourceVersion: '18173',
      createdAt: '2020-10-02T14:33:19Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'katib-ui-token-plpr4',
      uid: 'ba8681e8-7de2-4e45-aea3-27c93d31a6f5',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'kubeflow',
          name: 'katib-ui',
          uid: 'e5646adf-0232-4d01-81b8-8fffdd383e06'
        }
      ],
      resourceVersion: '18171',
      createdAt: '2020-10-02T14:33:19Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'revisions.serving.knative.dev',
      uid: 'd35b7615-fe41-40ba-b1d5-d42f7b8c3cd5',
      resourceVersion: '18339',
      createdAt: '2020-10-02T14:33:30Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'metadata-db',
      uid: '4d3f3f0f-3b23-4ae2-b3c5-d79e79c3edd3',
      networkingInfo: {
        targetLabels: {
          'app.kubernetes.io/component': 'metadata',
          'app.kubernetes.io/instance': 'metadata-0.2.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'metadata',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.1',
          component: 'db',
          'kustomize.component': 'metadata'
        }
      },
      resourceVersion: '20354',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'metadata-db',
      uid: 'dad6520a-605c-47d8-a5f1-b1469d6870bc',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'metadata-db',
          uid: '4d3f3f0f-3b23-4ae2-b3c5-d79e79c3edd3'
        }
      ],
      resourceVersion: '645264',
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'kubeflow-pytorchjobs-view',
      uid: 'f7409a67-cc21-4ff1-9de6-05ad912d9692',
      resourceVersion: '18722',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'kubeflow',
      name: 'workflow-controller-parameters',
      uid: '91c596e7-6893-4232-92b7-6327d2fef691',
      resourceVersion: '1765112',
      createdAt: '2020-10-02T14:33:05Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRoleBinding',
      name: 'cluster-local-gateway-istio-system',
      uid: '934e20cf-ba51-4aab-93f1-b1c54dbc6746',
      resourceVersion: '18830',
      createdAt: '2020-10-02T14:34:02Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'knative-serving',
      name: 'networking-istio',
      uid: '1030095d-b93d-4dfa-993f-a1dd276fe5d3',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'knative-serving',
          name: 'knative-serving-install',
          uid: '64044521-8f41-49ba-885c-1e4ce0b8d569'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '1548387',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:40Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'knative-serving',
      name: 'networking-istio-77c7d9f9c9',
      uid: 'f09352e5-e18e-4a0e-b9b3-650d1308e6c6',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'knative-serving',
          name: 'networking-istio',
          uid: '1030095d-b93d-4dfa-993f-a1dd276fe5d3'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '1548385',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:40Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'knative-serving',
      name: 'networking-istio-77c7d9f9c9-58mbb',
      uid: '13563c7d-5e11-47cd-8373-e5d8ce49dfc5',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'knative-serving',
          name: 'networking-istio-77c7d9f9c9',
          uid: 'f09352e5-e18e-4a0e-b9b3-650d1308e6c6'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'networking-istio',
          'app.kubernetes.io/component': 'knative-serving-install',
          'app.kubernetes.io/instance': 'knative-serving-install-v0.11.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'knative-serving-install',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v0.11.1',
          'kustomize.component': 'knative',
          'pod-template-hash': '77c7d9f9c9',
          'serving.knative.dev/release': 'v0.11.2'
        }
      },
      resourceVersion: '1548384',
      images: [
        'gcr.io/knative-releases/knative.dev/serving/cmd/networking/istio@sha256:61461fa789e19895d7d1e5ab96d8bb52a63788e0607e1bd2948b9570efeb6a8f'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-05T23:43:48Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'kubeflow',
      name: 'trial-template',
      uid: 'a14bcd1b-047d-4c94-a43f-caf24d569915',
      resourceVersion: '18072',
      createdAt: '2020-10-02T14:33:08Z'
    },
    {
      group: 'networking.istio.io',
      version: 'v1alpha3',
      kind: 'VirtualService',
      namespace: 'kubeflow',
      name: 'katib-ui',
      uid: '7630e11f-3fa3-470a-8640-84f87f52ee41',
      resourceVersion: '21382',
      createdAt: '2020-10-02T14:41:12Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'kfserving-crds',
      uid: '4fa713cd-dc1b-43cc-9999-ab48904eb1cb',
      resourceVersion: '1766180',
      createdAt: '2020-10-02T14:41:16Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'knative-serving',
      name: 'controller',
      uid: 'c0225810-5f2c-4420-a0f4-2d6aa55dbe09',
      networkingInfo: {
        targetLabels: {
          app: 'controller',
          'app.kubernetes.io/component': 'knative-serving-install',
          'app.kubernetes.io/instance': 'knative-serving-install-v0.11.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'knative-serving-install',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v0.11.1',
          'kustomize.component': 'knative'
        }
      },
      resourceVersion: '20375',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'knative-serving',
      name: 'controller',
      uid: '11685136-352f-41ae-8552-155a6ca46163',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'knative-serving',
          name: 'controller',
          uid: 'c0225810-5f2c-4420-a0f4-2d6aa55dbe09'
        }
      ],
      resourceVersion: '635610',
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'pipeline-visualization-service',
      uid: '7933787c-85ea-4ae7-9a4e-f4cdd2619cf5',
      resourceVersion: '1766242',
      createdAt: '2020-10-02T14:41:25Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'ml-pipeline-ml-pipeline-visualizationserver',
      uid: 'd47be868-ce2b-44e3-8990-9570daff04d0',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'pipeline-visualization-service',
          uid: '7933787c-85ea-4ae7-9a4e-f4cdd2619cf5'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '643724',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:39Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'ml-pipeline-ml-pipeline-visualizationserver-675656df79',
      uid: 'e2495a85-dfb1-4333-afaa-6d1fcc1d26be',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'ml-pipeline-ml-pipeline-visualizationserver',
          uid: 'd47be868-ce2b-44e3-8990-9570daff04d0'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '643722',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:40Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'ml-pipeline-ml-pipeline-visualizationserver-675656df79-c8m4s',
      uid: '4568e553-6113-475b-a1b2-7096f1f2a33a',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'ml-pipeline-ml-pipeline-visualizationserver-675656df79',
          uid: 'e2495a85-dfb1-4333-afaa-6d1fcc1d26be'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'ml-pipeline-visualizationserver',
          'app.kubernetes.io/component': 'pipeline-visualization-service',
          'app.kubernetes.io/instance': 'pipeline-visualization-service-0.2.5',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'pipeline-visualization-service',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.5',
          'pod-template-hash': '675656df79'
        }
      },
      resourceVersion: '643721',
      images: [
        'gcr.io/ml-pipeline/visualization-server:0.2.5'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:36:01Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'istio-system',
      name: 'cluster-local-gateway-parameters-tbbdb2842d',
      uid: '5d41630a-2358-42c1-befb-c95ed7b0a90e',
      resourceVersion: '18037',
      createdAt: '2020-10-02T14:33:04Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'metadata-db',
      uid: '6b4f08b7-3122-4436-8d55-5a4fc74a5d33',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'metadata',
          uid: '6c38734d-08e3-42ff-9cfc-2ae6e29fb34b'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645265',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:42Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'metadata-db-79d6cf9d94',
      uid: 'b767dbf3-b5f8-415b-9584-ae08d493c7d4',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'metadata-db',
          uid: '6b4f08b7-3122-4436-8d55-5a4fc74a5d33'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645263',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:42Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'metadata-db-79d6cf9d94-g9npq',
      uid: '425f1489-d141-4179-98b5-41322fd475e4',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'metadata-db-79d6cf9d94',
          uid: 'b767dbf3-b5f8-415b-9584-ae08d493c7d4'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          'app.kubernetes.io/component': 'metadata',
          'app.kubernetes.io/instance': 'metadata-0.2.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'metadata',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.1',
          component: 'db',
          'kustomize.component': 'metadata',
          'pod-template-hash': '79d6cf9d94'
        }
      },
      resourceVersion: '645262',
      images: [
        'mysql:8.0.3'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:56Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'kubeflow',
      name: 'jupyter-web-app-parameters',
      uid: 'ea56ad78-bb6e-4bcf-82e0-ef2eb600b5cd',
      resourceVersion: '18017',
      createdAt: '2020-10-02T14:33:00Z'
    },
    {
      group: 'admissionregistration.k8s.io',
      version: 'v1',
      kind: 'ValidatingWebhookConfiguration',
      name: 'validation.webhook.serving.knative.dev',
      uid: 'c4fe132c-4291-420f-8b2e-5a2871132f7f',
      resourceVersion: '21626',
      createdAt: '2020-10-02T14:41:39Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'workfloweventbindings.argoproj.io',
      uid: '4cd3b306-81d3-426e-90ec-44d1598142c0',
      resourceVersion: '1765341',
      createdAt: '2020-10-02T14:33:34Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'kubeflow',
      name: 'admission-webhook-bootstrap-config-map',
      uid: 'f711f14d-3bff-430d-91f8-fbf4e9311d8e',
      resourceVersion: '18018',
      createdAt: '2020-10-02T14:33:00Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'centraldashboard',
      uid: '1276a511-dc0d-435d-b712-6a99db800cd0',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'centraldashboard',
          uid: '4f297d44-147f-4f40-a9da-278dbf3ea42e'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '642510',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'centraldashboard-754dc648d8',
      uid: 'd028a239-07d3-4f83-924c-70df38c8237f',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'centraldashboard',
          uid: '1276a511-dc0d-435d-b712-6a99db800cd0'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '642508',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'centraldashboard-754dc648d8-vblks',
      uid: 'bdec4e71-bfbd-489d-8970-f2297ce8543f',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'centraldashboard-754dc648d8',
          uid: 'd028a239-07d3-4f83-924c-70df38c8237f'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'centraldashboard',
          'app.kubernetes.io/component': 'centraldashboard',
          'app.kubernetes.io/instance': 'centraldashboard-v1.0.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'centraldashboard',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v1.0.0',
          'kustomize.component': 'centraldashboard',
          'pod-template-hash': '754dc648d8'
        }
      },
      resourceVersion: '642507',
      images: [
        'auroraprodacr.azurecr.io/gsantomaggio/centraldashboard:node-12'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:46Z'
    },
    {
      group: 'rbac.istio.io',
      version: 'v1alpha1',
      kind: 'ClusterRbacConfig',
      name: 'default',
      uid: 'f2239712-4dfd-4796-b9c9-62f41a026961',
      resourceVersion: '21294',
      createdAt: '2020-10-02T14:41:00Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'certificates.networking.internal.knative.dev',
      uid: 'd4da967e-3614-458e-a4e8-2df4133d60ed',
      resourceVersion: '18372',
      createdAt: '2020-10-02T14:33:30Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'kubeflow',
      name: 'profiles-controller-service-account',
      uid: '1d24c628-3227-42d1-bfbc-0a6805117c20',
      resourceVersion: '18232',
      createdAt: '2020-10-02T14:33:21Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'profiles-controller-service-account-token-vs4tx',
      uid: '97023311-0e5a-4f28-987e-b1c977b17981',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'kubeflow',
          name: 'profiles-controller-service-account',
          uid: '1d24c628-3227-42d1-bfbc-0a6805117c20'
        }
      ],
      resourceVersion: '18231',
      createdAt: '2020-10-02T14:33:21Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'argo-aggregate-to-admin',
      uid: 'b530c02e-dee8-43dc-89ee-c7506d24a27d',
      resourceVersion: '1765536',
      createdAt: '2020-10-02T14:33:36Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRoleBinding',
      name: 'argo-binding',
      uid: '2514ff5f-a210-4b44-8bbd-358a15491b98',
      resourceVersion: '1765596',
      createdAt: '2020-10-02T14:34:02Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'ml-pipeline-persistenceagent',
      uid: 'e33b5ba8-a37b-4d1e-824e-fca22f7af3cd',
      resourceVersion: '18582',
      createdAt: '2020-10-02T14:33:42Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'ml-pipeline-ui',
      uid: 'c45a6170-334b-4563-ba6d-95bb8c65a30a',
      networkingInfo: {
        targetLabels: {
          app: 'ml-pipeline-ui',
          'app.kubernetes.io/component': 'pipelines-ui',
          'app.kubernetes.io/instance': 'pipelines-ui-0.2.5',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'pipelines-ui',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.5'
        }
      },
      resourceVersion: '20391',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'ml-pipeline-ui',
      uid: '0037159a-7a49-4b93-8f58-5ddd6a2797a4',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'ml-pipeline-ui',
          uid: 'c45a6170-334b-4563-ba6d-95bb8c65a30a'
        }
      ],
      resourceVersion: '642680',
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'ml-pipeline-scheduledworkflow',
      uid: '3373a6ea-4552-4f96-bc2c-1816b2366226',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'scheduledworkflow',
          uid: '28024df8-4685-4c72-adc6-aad3515c66d1'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '643771',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'ml-pipeline-scheduledworkflow-7b4cb5d959',
      uid: 'a0bc035f-27d8-4fc4-95e7-306f40b5260a',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'ml-pipeline-scheduledworkflow',
          uid: '3373a6ea-4552-4f96-bc2c-1816b2366226'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '643770',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'ml-pipeline-scheduledworkflow-7b4cb5d959-9srv5',
      uid: '13b1af1e-32fa-4eb0-bcca-a65e66590ef7',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'ml-pipeline-scheduledworkflow-7b4cb5d959',
          uid: 'a0bc035f-27d8-4fc4-95e7-306f40b5260a'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'ml-pipeline-scheduledworkflow',
          'app.kubernetes.io/component': 'scheduledworkflow',
          'app.kubernetes.io/instance': 'scheduledworkflow-0.2.5',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'scheduledworkflow',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.5',
          'pod-template-hash': '7b4cb5d959'
        }
      },
      resourceVersion: '643769',
      images: [
        'gcr.io/ml-pipeline/scheduledworkflow:0.2.5'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:36:03Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'scheduledsparkapplications.sparkoperator.k8s.io',
      uid: 'd669fb62-ee1e-47f4-a74f-05d6d3116022',
      resourceVersion: '25279',
      createdAt: '2020-10-02T14:51:22Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'kubeflow-tfjobs-edit',
      uid: 'feb09f4e-a6cc-4a96-8dcb-8cec22848dbe',
      resourceVersion: '18601',
      createdAt: '2020-10-02T14:33:36Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'admission-webhook-deployment',
      uid: 'e84595e0-1428-44c1-b2d7-a0706e10ff13',
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '1548391',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:39Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'admission-webhook-deployment-59bc556b94',
      uid: '3afb35bf-3059-47dc-a13e-4f50ff5e3bba',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'admission-webhook-deployment',
          uid: 'e84595e0-1428-44c1-b2d7-a0706e10ff13'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '1548389',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:40Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'admission-webhook-deployment-59bc556b94-vjcgd',
      uid: 'd765c6b6-7efa-42d9-b406-f5d4896e8464',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'admission-webhook-deployment-59bc556b94',
          uid: '3afb35bf-3059-47dc-a13e-4f50ff5e3bba'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'admission-webhook',
          'app.kubernetes.io/component': 'webhook',
          'app.kubernetes.io/instance': 'webhook-v1.0.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'webhook',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v1.0.0',
          'kustomize.component': 'admission-webhook',
          'pod-template-hash': '59bc556b94'
        }
      },
      resourceVersion: '1548388',
      images: [
        'gcr.io/kubeflow-images-public/admission-webhook:v1.0.0-gaf96e4e3'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-05T23:43:48Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'api-service',
      uid: '96f5d9d3-5639-43cf-94ce-b941f777575f',
      resourceVersion: '1765976',
      createdAt: '2020-10-02T14:40:54Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'ml-pipeline',
      uid: '4fd2ebc2-e6d5-4b34-9dd3-bc835f54fe73',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'api-service',
          uid: '96f5d9d3-5639-43cf-94ce-b941f777575f'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645457',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'ml-pipeline-698bcdd747',
      uid: 'faa029eb-ab98-4196-88e2-6284f68415be',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'ml-pipeline',
          uid: '4fd2ebc2-e6d5-4b34-9dd3-bc835f54fe73'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645456',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'ml-pipeline-698bcdd747-prcpg',
      uid: '1fca6f72-0110-4934-8b43-a00d935e8251',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'ml-pipeline-698bcdd747',
          uid: 'faa029eb-ab98-4196-88e2-6284f68415be'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'ml-pipeline',
          'app.kubernetes.io/component': 'api-service',
          'app.kubernetes.io/instance': 'api-service-0.2.5',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'api-service',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.5',
          'pod-template-hash': '698bcdd747'
        }
      },
      resourceVersion: '645455',
      images: [
        'gcr.io/ml-pipeline/api-server:0.2.5'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:33:36Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'knative-serving',
      name: 'knative-serving-install',
      uid: '64044521-8f41-49ba-885c-1e4ce0b8d569',
      resourceVersion: '5988145',
      createdAt: '2020-10-02T14:41:16Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'knative-serving',
      name: 'controller',
      uid: 'c16b6708-cb9c-4bc5-93ff-8e922b830c21',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'knative-serving',
          name: 'knative-serving-install',
          uid: '64044521-8f41-49ba-885c-1e4ce0b8d569'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '635609',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:40Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'knative-serving',
      name: 'controller-6f85bdc877',
      uid: 'e55cd174-f0ed-4c8c-b9bb-086bfd46d395',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'knative-serving',
          name: 'controller',
          uid: 'c16b6708-cb9c-4bc5-93ff-8e922b830c21'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '635607',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:40Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'knative-serving',
      name: 'controller-6f85bdc877-jzcpk',
      uid: 'e82109b1-1ccf-4fe7-8ed7-6b6d5bbe00c4',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'knative-serving',
          name: 'controller-6f85bdc877',
          uid: 'e55cd174-f0ed-4c8c-b9bb-086bfd46d395'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'controller',
          'app.kubernetes.io/component': 'knative-serving-install',
          'app.kubernetes.io/instance': 'knative-serving-install-v0.11.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'knative-serving-install',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v0.11.1',
          'kustomize.component': 'knative',
          'pod-template-hash': '6f85bdc877',
          'serving.knative.dev/release': 'v0.11.2'
        }
      },
      resourceVersion: '635606',
      images: [
        'gcr.io/knative-releases/knative.dev/serving/cmd/controller@sha256:1e77bdab30c8d0f0df299f5fa93d6f99eb63071b9d3329937dff0c6acb99e059'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:19:18Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'knative-serving',
      name: 'networking-istio',
      uid: '1030095d-b93d-4dfa-993f-a1dd276fe5d3',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'knative-serving',
          name: 'knative-serving-install',
          uid: '64044521-8f41-49ba-885c-1e4ce0b8d569'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '1548387',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:40Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'knative-serving',
      name: 'networking-istio-77c7d9f9c9',
      uid: 'f09352e5-e18e-4a0e-b9b3-650d1308e6c6',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'knative-serving',
          name: 'networking-istio',
          uid: '1030095d-b93d-4dfa-993f-a1dd276fe5d3'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '1548385',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:40Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'knative-serving',
      name: 'networking-istio-77c7d9f9c9-58mbb',
      uid: '13563c7d-5e11-47cd-8373-e5d8ce49dfc5',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'knative-serving',
          name: 'networking-istio-77c7d9f9c9',
          uid: 'f09352e5-e18e-4a0e-b9b3-650d1308e6c6'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'networking-istio',
          'app.kubernetes.io/component': 'knative-serving-install',
          'app.kubernetes.io/instance': 'knative-serving-install-v0.11.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'knative-serving-install',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v0.11.1',
          'kustomize.component': 'knative',
          'pod-template-hash': '77c7d9f9c9',
          'serving.knative.dev/release': 'v0.11.2'
        }
      },
      resourceVersion: '1548384',
      images: [
        'gcr.io/knative-releases/knative.dev/serving/cmd/networking/istio@sha256:61461fa789e19895d7d1e5ab96d8bb52a63788e0607e1bd2948b9570efeb6a8f'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-05T23:43:48Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'knative-serving',
      name: 'autoscaler-hpa',
      uid: '9a89c712-20bd-4ca8-8327-f28f297338f2',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'knative-serving',
          name: 'knative-serving-install',
          uid: '64044521-8f41-49ba-885c-1e4ce0b8d569'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '644067',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'knative-serving',
      name: 'autoscaler-hpa-6f568ffc4c',
      uid: '293b349b-e44d-4ef9-93b7-64393b06f729',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'knative-serving',
          name: 'autoscaler-hpa',
          uid: '9a89c712-20bd-4ca8-8327-f28f297338f2'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '644066',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'knative-serving',
      name: 'autoscaler-hpa-6f568ffc4c-8rkbb',
      uid: '3d9e9e45-5b2c-4f80-bd70-0b41879d9963',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'knative-serving',
          name: 'autoscaler-hpa-6f568ffc4c',
          uid: '293b349b-e44d-4ef9-93b7-64393b06f729'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'autoscaler-hpa',
          'app.kubernetes.io/component': 'knative-serving-install',
          'app.kubernetes.io/instance': 'knative-serving-install-v0.11.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'knative-serving-install',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v0.11.1',
          'kustomize.component': 'knative',
          'pod-template-hash': '6f568ffc4c',
          'serving.knative.dev/release': 'v0.11.2'
        }
      },
      resourceVersion: '644065',
      images: [
        'gcr.io/knative-releases/knative.dev/serving/cmd/autoscaler-hpa@sha256:75da5ff75bc1e71799d039846b1bbd632343894c88feaa59914cfeeb1b213c81'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:36:50Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'knative-serving',
      name: 'activator',
      uid: 'f933538f-d9fe-42e6-a41c-18e91ab0474a',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'knative-serving',
          name: 'knative-serving-install',
          uid: '64044521-8f41-49ba-885c-1e4ce0b8d569'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '1549003',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'knative-serving',
      name: 'activator-68b949bd95',
      uid: '659ef5f5-6fe2-49dc-9ac4-331e4971cf85',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'knative-serving',
          name: 'activator',
          uid: 'f933538f-d9fe-42e6-a41c-18e91ab0474a'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '1549000',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:39Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'knative-serving',
      name: 'activator-68b949bd95-tl4n4',
      uid: 'e0712e5e-becd-49bd-af95-3c99509f9dcf',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'knative-serving',
          name: 'activator-68b949bd95',
          uid: '659ef5f5-6fe2-49dc-9ac4-331e4971cf85'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '2/2'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'activator',
          'app.kubernetes.io/component': 'knative-serving-install',
          'app.kubernetes.io/instance': 'knative-serving-install-v0.11.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'knative-serving-install',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v0.11.1',
          'kustomize.component': 'knative',
          'pod-template-hash': '68b949bd95',
          role: 'activator',
          'security.istio.io/tlsMode': 'istio',
          'service.istio.io/canonical-name': 'knative-serving-install',
          'service.istio.io/canonical-revision': 'v0.11.1',
          'serving.knative.dev/release': 'v0.11.2'
        }
      },
      resourceVersion: '1548999',
      images: [
        'docker.io/istio/proxyv2:1.5.10',
        'gcr.io/knative-releases/knative.dev/serving/cmd/activator@sha256:c51023e62e351d5910f92ee941b4929eb82539e62636dd3ccb4a016d73e86b2e'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:36:44Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'knative-serving',
      name: 'webhook',
      uid: '553dcd49-1c3c-4323-b5e8-957e0ad2b011',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'knative-serving',
          name: 'knative-serving-install',
          uid: '64044521-8f41-49ba-885c-1e4ce0b8d569'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '635617',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'knative-serving',
      name: 'webhook-576b4f987',
      uid: '8b1952fc-3af5-4951-80b4-2281a478e330',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'knative-serving',
          name: 'webhook',
          uid: '553dcd49-1c3c-4323-b5e8-957e0ad2b011'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '635615',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'knative-serving',
      name: 'webhook-576b4f987-c87q5',
      uid: 'b63eb0b1-f354-4e28-9814-7b05fc9c5677',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'knative-serving',
          name: 'webhook-576b4f987',
          uid: '8b1952fc-3af5-4951-80b4-2281a478e330'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'webhook',
          'app.kubernetes.io/component': 'knative-serving-install',
          'app.kubernetes.io/instance': 'knative-serving-install-v0.11.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'knative-serving-install',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v0.11.1',
          'kustomize.component': 'knative',
          'pod-template-hash': '576b4f987',
          role: 'webhook',
          'serving.knative.dev/release': 'v0.11.2'
        }
      },
      resourceVersion: '635614',
      images: [
        'gcr.io/knative-releases/knative.dev/serving/cmd/webhook@sha256:d07560cd5548640cc79abc819608844527351f10e8b0a847988f9eb602c18972'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:19:18Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'knative-serving',
      name: 'autoscaler',
      uid: 'b9747434-b37b-4521-82b2-e16b0639f57a',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'knative-serving',
          name: 'knative-serving-install',
          uid: '64044521-8f41-49ba-885c-1e4ce0b8d569'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '1548733',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'knative-serving',
      name: 'autoscaler-b5bf74846',
      uid: 'ddb58b88-ab99-41c2-a2f3-1d73cd49d8ca',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'knative-serving',
          name: 'autoscaler',
          uid: 'b9747434-b37b-4521-82b2-e16b0639f57a'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '1548731',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'knative-serving',
      name: 'autoscaler-b5bf74846-njx9h',
      uid: '2ef703fd-6c72-4a45-98ee-362d1058097a',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'knative-serving',
          name: 'autoscaler-b5bf74846',
          uid: 'ddb58b88-ab99-41c2-a2f3-1d73cd49d8ca'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '2/2'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'autoscaler',
          'app.kubernetes.io/component': 'knative-serving-install',
          'app.kubernetes.io/instance': 'knative-serving-install-v0.11.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'knative-serving-install',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v0.11.1',
          'kustomize.component': 'knative',
          'pod-template-hash': 'b5bf74846',
          'security.istio.io/tlsMode': 'istio',
          'service.istio.io/canonical-name': 'knative-serving-install',
          'service.istio.io/canonical-revision': 'v0.11.1',
          'serving.knative.dev/release': 'v0.11.2'
        }
      },
      resourceVersion: '1548730',
      images: [
        'docker.io/istio/proxyv2:1.5.10',
        'gcr.io/knative-releases/knative.dev/serving/cmd/autoscaler@sha256:998a405454832cda18a4bf956d26d610a2df2130a39b834b597a89a3153c8c15'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-05T23:43:48Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'mysql',
      uid: 'd8d69efe-10e9-4b6c-a998-da996ba64730',
      networkingInfo: {
        targetLabels: {
          app: 'mysql',
          'app.kubernetes.io/component': 'mysql',
          'app.kubernetes.io/instance': 'mysql-0.2.5',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'mysql',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.5'
        }
      },
      resourceVersion: '20384',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'mysql',
      uid: '326fa025-ba09-4cb7-8f73-bd13fedc3348',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'mysql',
          uid: 'd8d69efe-10e9-4b6c-a998-da996ba64730'
        }
      ],
      resourceVersion: '645097',
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'notebook-controller-service',
      uid: '372e05bb-8e14-44d0-b639-acd271292537',
      networkingInfo: {
        targetLabels: {
          app: 'notebook-controller',
          'app.kubernetes.io/component': 'notebook-controller',
          'app.kubernetes.io/instance': 'notebook-controller-v1.0.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'notebook-controller',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v1.0.0',
          'kustomize.component': 'notebook-controller'
        }
      },
      resourceVersion: '20393',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'notebook-controller-service',
      uid: 'ec98d671-5d58-417e-be6f-dffc9c512d62',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'notebook-controller-service',
          uid: '372e05bb-8e14-44d0-b639-acd271292537'
        }
      ],
      resourceVersion: '642952',
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'argo-aggregate-to-edit',
      uid: '0330f7da-5b0d-490e-a398-74d066e4d41e',
      resourceVersion: '1765547',
      createdAt: '2020-10-02T14:33:36Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'katib-controller',
      uid: 'c4da4097-2570-41ac-a550-85d72466d4a9',
      resourceVersion: '18742',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      group: 'policy',
      version: 'v1beta1',
      kind: 'PodDisruptionBudget',
      namespace: 'istio-system',
      name: 'cluster-local-gateway',
      uid: '76a7b247-6a31-45fe-beff-b11fe50d4c45',
      resourceVersion: '1766188',
      createdAt: '2020-10-02T14:32:53Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'manager-role',
      uid: 'a52e63df-4c07-4f37-81cb-09f04b913245',
      resourceVersion: '18670',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'knative-serving',
      name: 'knative-serving-crds',
      uid: '75100a21-9d3c-4a89-84e8-253375e95365',
      resourceVersion: '1766183',
      createdAt: '2020-10-02T14:41:16Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'application-controller-service',
      uid: '12f48b52-eeda-4679-b4aa-15c430922900',
      networkingInfo: {
        targetLabels: {
          'app.kubernetes.io/component': 'kubeflow',
          'app.kubernetes.io/instance': 'kubeflow-v1.0.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'kubeflow',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v1.0.0'
        }
      },
      resourceVersion: '20386',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'application-controller-service',
      uid: '0c9e56d4-cef6-4ed0-838a-ff8906fd3a3c',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'application-controller-service',
          uid: '12f48b52-eeda-4679-b4aa-15c430922900'
        }
      ],
      resourceVersion: '642845',
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'argo',
      uid: '3eb053e7-1db8-4cd2-b840-7ca9b8d8712c',
      resourceVersion: '5986859',
      createdAt: '2020-10-02T14:40:54Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'workflow-controller',
      uid: '4df81191-1777-42ba-a4a6-908bc3d1fa41',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'argo',
          uid: '3eb053e7-1db8-4cd2-b840-7ca9b8d8712c'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '1765971',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-06T11:16:06Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'workflow-controller-754fc55df5',
      uid: '4f8211d3-2639-4a9b-869f-eefcb4668792',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'workflow-controller',
          uid: '4df81191-1777-42ba-a4a6-908bc3d1fa41'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '1765884',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-06T11:16:06Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'workflow-controller-754fc55df5-xqgsg',
      uid: '81bd0323-bdda-4c3a-b9d4-22d86cc41335',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'workflow-controller-754fc55df5',
          uid: '4f8211d3-2639-4a9b-869f-eefcb4668792'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'workflow-controller',
          'app.kubernetes.io/component': 'argo',
          'app.kubernetes.io/instance': 'argo-v2.11.2',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'argo',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v2.11.2',
          'kustomize.component': 'argo',
          'pod-template-hash': '754fc55df5'
        }
      },
      resourceVersion: '1765883',
      images: [
        'argoproj/workflow-controller:v2.11.2'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-06T11:16:06Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'argo-server',
      uid: 'c7d59aa8-7ea2-45bb-a7ff-c112545d38e0',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'argo',
          uid: '3eb053e7-1db8-4cd2-b840-7ca9b8d8712c'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '1766162',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-06T11:16:07Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'argo-server-6d4b4bdfbc',
      uid: '080005aa-37aa-4570-a194-5780028ff60f',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'argo-server',
          uid: 'c7d59aa8-7ea2-45bb-a7ff-c112545d38e0'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '1766159',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-06T11:16:07Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'argo-server-6d4b4bdfbc-pcjp8',
      uid: '09641aad-b583-4565-b536-ae5d2f58e556',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'argo-server-6d4b4bdfbc',
          uid: '080005aa-37aa-4570-a194-5780028ff60f'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'argo-server',
          'app.kubernetes.io/component': 'argo',
          'app.kubernetes.io/instance': 'argo-v2.11.2',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'argo',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v2.11.2',
          'kustomize.component': 'argo',
          'pod-template-hash': '6d4b4bdfbc'
        }
      },
      resourceVersion: '1766157',
      images: [
        'argoproj/argocli:v2.11.2'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-06T11:16:07Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'jupyter-web-app',
      uid: '836e36d8-cf05-4c30-9705-aaf0aff02345',
      resourceVersion: '5988144',
      createdAt: '2020-10-02T14:41:08Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'Role',
      namespace: 'kubeflow',
      name: 'jupyter-web-app-jupyter-notebook-role',
      uid: 'aeba3765-2a19-4dcc-b277-08257991a391',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'jupyter-web-app',
          uid: '836e36d8-cf05-4c30-9705-aaf0aff02345'
        }
      ],
      resourceVersion: '22376',
      createdAt: '2020-10-02T14:34:18Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'RoleBinding',
      namespace: 'kubeflow',
      name: 'jupyter-web-app-jupyter-notebook-role-binding',
      uid: '32d9bf84-9a94-45ef-95ad-14d4a04a5453',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'jupyter-web-app',
          uid: '836e36d8-cf05-4c30-9705-aaf0aff02345'
        }
      ],
      resourceVersion: '22373',
      createdAt: '2020-10-02T14:34:20Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'jupyter-web-app-deployment',
      uid: '374126c8-efbe-497d-9f8c-43815227742f',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'jupyter-web-app',
          uid: '836e36d8-cf05-4c30-9705-aaf0aff02345'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '643956',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'jupyter-web-app-deployment-544b7d5684',
      uid: '982c8a2f-e03e-43de-ab10-111103b8f51c',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'jupyter-web-app-deployment',
          uid: '374126c8-efbe-497d-9f8c-43815227742f'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '643955',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'jupyter-web-app-deployment-544b7d5684-skmgz',
      uid: '4cd47d9c-b653-4ddf-94b4-b735bf8e35ce',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'jupyter-web-app-deployment-544b7d5684',
          uid: '982c8a2f-e03e-43de-ab10-111103b8f51c'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'jupyter-web-app',
          'app.kubernetes.io/component': 'jupyter-web-app',
          'app.kubernetes.io/instance': 'jupyter-web-app-v1.0.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'jupyter-web-app',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v1.0.0',
          'kustomize.component': 'jupyter-web-app',
          'pod-template-hash': '544b7d5684'
        }
      },
      resourceVersion: '643953',
      images: [
        'gcr.io/kubeflow-images-public/jupyter-web-app:v1.0.0-g2bd63238'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:36:02Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'knative-serving',
      name: 'autoscaler-hpa',
      uid: '9a89c712-20bd-4ca8-8327-f28f297338f2',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'knative-serving',
          name: 'knative-serving-install',
          uid: '64044521-8f41-49ba-885c-1e4ce0b8d569'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '644067',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'knative-serving',
      name: 'autoscaler-hpa-6f568ffc4c',
      uid: '293b349b-e44d-4ef9-93b7-64393b06f729',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'knative-serving',
          name: 'autoscaler-hpa',
          uid: '9a89c712-20bd-4ca8-8327-f28f297338f2'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '644066',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'knative-serving',
      name: 'autoscaler-hpa-6f568ffc4c-8rkbb',
      uid: '3d9e9e45-5b2c-4f80-bd70-0b41879d9963',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'knative-serving',
          name: 'autoscaler-hpa-6f568ffc4c',
          uid: '293b349b-e44d-4ef9-93b7-64393b06f729'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'autoscaler-hpa',
          'app.kubernetes.io/component': 'knative-serving-install',
          'app.kubernetes.io/instance': 'knative-serving-install-v0.11.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'knative-serving-install',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v0.11.1',
          'kustomize.component': 'knative',
          'pod-template-hash': '6f568ffc4c',
          'serving.knative.dev/release': 'v0.11.2'
        }
      },
      resourceVersion: '644065',
      images: [
        'gcr.io/knative-releases/knative.dev/serving/cmd/autoscaler-hpa@sha256:75da5ff75bc1e71799d039846b1bbd632343894c88feaa59914cfeeb1b213c81'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:36:50Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'kubeflow-edit',
      uid: '01775b3d-02c2-4e04-a9aa-c8506a6df85b',
      resourceVersion: '1765553',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'kubeflow',
      name: 'profiles-profiles-parameters-gg8k9kkt9g',
      uid: 'e58dddd0-b754-45e4-8dc6-606a23a6f4c5',
      resourceVersion: '18054',
      createdAt: '2020-10-02T14:33:05Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'pytorch-operator',
      uid: 'f2a5b85b-6f64-4b6a-960b-c46dcf8fab30',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'pytorch-operator',
          uid: '3f12d82a-1ed7-4e1a-8c8d-ab2e15c63cf2'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '5693454',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'pytorch-operator-666dd4cd49',
      uid: '75a4f706-44a4-46ac-93ea-2b90e55f0666',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'pytorch-operator',
          uid: 'f2a5b85b-6f64-4b6a-960b-c46dcf8fab30'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '5693453',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'pytorch-operator-666dd4cd49-rk2rh',
      uid: '92f968b4-fd0c-45eb-ac61-78b86e1c41a1',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'pytorch-operator-666dd4cd49',
          uid: '75a4f706-44a4-46ac-93ea-2b90e55f0666'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          'app.kubernetes.io/component': 'pytorch',
          'app.kubernetes.io/instance': 'pytorch-operator-v1.0.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'pytorch-operator',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v1.0.0',
          'kustomize.component': 'pytorch-operator',
          name: 'pytorch-operator',
          'pod-template-hash': '666dd4cd49'
        }
      },
      resourceVersion: '5693452',
      images: [
        'gcr.io/kubeflow-images-public/pytorch-operator:v1.0.0-g047cf0f'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:36:02Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'cronworkflows.argoproj.io',
      uid: '194f95b7-6a51-431c-8ca9-e2b57d8c65da',
      resourceVersion: '1765316',
      createdAt: '2020-10-02T14:33:30Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'istio-system',
      name: 'cluster-local-gateway',
      uid: '396f4779-ede7-4cbf-897f-a92e3f71e952',
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '1766190',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'istio-system',
      name: 'cluster-local-gateway-849558c44b',
      uid: '661e3236-12df-4f49-b8da-eefacbf65454',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'istio-system',
          name: 'cluster-local-gateway',
          uid: '396f4779-ede7-4cbf-897f-a92e3f71e952'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '1766189',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'istio-system',
      name: 'cluster-local-gateway-849558c44b-lhs27',
      uid: '671ce795-b03e-435b-8cc4-a2414729428c',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'istio-system',
          name: 'cluster-local-gateway-849558c44b',
          uid: '661e3236-12df-4f49-b8da-eefacbf65454'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'cluster-local-gateway',
          istio: 'cluster-local-gateway',
          'kustomize.component': 'cluster-local-gateway',
          'pod-template-hash': '849558c44b'
        }
      },
      resourceVersion: '1766046',
      images: [
        'docker.io/istio/proxyv2:1.5.4'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-06T11:16:23Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'istio-system',
      name: 'cluster-local-gateway-849558c44b-lznj7',
      uid: 'b3793785-a073-4069-9828-eb513fcd24cf',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'istio-system',
          name: 'cluster-local-gateway-849558c44b',
          uid: '661e3236-12df-4f49-b8da-eefacbf65454'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'cluster-local-gateway',
          istio: 'cluster-local-gateway',
          'kustomize.component': 'cluster-local-gateway',
          'pod-template-hash': '849558c44b'
        }
      },
      resourceVersion: '1548418',
      images: [
        'docker.io/istio/proxyv2:1.5.4'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-05T23:43:48Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'istio-system',
      name: 'cluster-local-gateway-849558c44b-kb89h',
      uid: 'b096e857-a4f4-4fab-af47-d13fcc684da2',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'istio-system',
          name: 'cluster-local-gateway-849558c44b',
          uid: '661e3236-12df-4f49-b8da-eefacbf65454'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'cluster-local-gateway',
          istio: 'cluster-local-gateway',
          'kustomize.component': 'cluster-local-gateway',
          'pod-template-hash': '849558c44b'
        }
      },
      resourceVersion: '1766060',
      images: [
        'docker.io/istio/proxyv2:1.5.4'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-06T11:16:23Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'istio-system',
      name: 'cluster-local-gateway-849558c44b-kv4w7',
      uid: '768205f9-570b-43b6-9237-4d6672f06172',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'istio-system',
          name: 'cluster-local-gateway-849558c44b',
          uid: '661e3236-12df-4f49-b8da-eefacbf65454'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'cluster-local-gateway',
          istio: 'cluster-local-gateway',
          'kustomize.component': 'cluster-local-gateway',
          'pod-template-hash': '849558c44b'
        }
      },
      resourceVersion: '1766187',
      images: [
        'docker.io/istio/proxyv2:1.5.4'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-06T11:16:38Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'istio-system',
      name: 'cluster-local-gateway-849558c44b-pc6zg',
      uid: '3801b249-7231-46d0-9740-24ffb978dea6',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'istio-system',
          name: 'cluster-local-gateway-849558c44b',
          uid: '661e3236-12df-4f49-b8da-eefacbf65454'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'cluster-local-gateway',
          istio: 'cluster-local-gateway',
          'kustomize.component': 'cluster-local-gateway',
          'pod-template-hash': '849558c44b'
        }
      },
      resourceVersion: '1766068',
      images: [
        'docker.io/istio/proxyv2:1.5.4'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-06T11:16:23Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'suggestions.kubeflow.org',
      uid: '9a787e88-da57-4180-8931-e1f2287689c9',
      resourceVersion: '18387',
      createdAt: '2020-10-02T14:33:33Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'kubeflow',
      name: 'parameters-dgd4h256h5',
      uid: '6be5ad43-d58b-46e4-9f3d-216b51de2579',
      resourceVersion: '18056',
      createdAt: '2020-10-02T14:33:05Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'cluster-local-gateway-istio-system',
      uid: '92ea33c8-af8d-446c-89fc-cb67ad9454db',
      resourceVersion: '18740',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'kubeflow',
      name: 'jupyter-web-app-config',
      uid: 'abb8be53-16dc-4cb1-a20b-a438ace65a7a',
      resourceVersion: '18041',
      createdAt: '2020-10-02T14:33:04Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'kfserving',
      uid: '61310dfd-6ec8-4918-8808-458161f312e3',
      resourceVersion: '1766179',
      createdAt: '2020-10-02T14:41:16Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRoleBinding',
      name: 'knative-serving-controller-admin',
      uid: '85fbfc2e-99f0-47e2-81b6-9a7eec4a9c93',
      resourceVersion: '18829',
      createdAt: '2020-10-02T14:34:02Z'
    },
    {
      version: 'v1',
      kind: 'PersistentVolumeClaim',
      namespace: 'kubeflow',
      name: 'mysql-pv-claim',
      uid: '665208e5-92a0-4a1c-bfc7-c1b8c24abc43',
      resourceVersion: '18142',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:33:11Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'seldondeployments.machinelearning.seldon.io',
      uid: '7d8035f4-4229-445c-9ac5-70bff766a4cb',
      resourceVersion: '18433',
      createdAt: '2020-10-02T14:33:34Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'kubeflow',
      name: 'ml-pipeline',
      uid: '1d49eb54-e7fa-4827-8474-58e6b8c4a727',
      resourceVersion: '18192',
      createdAt: '2020-10-02T14:33:20Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'ml-pipeline-token-t5zrd',
      uid: '49c01db8-61e3-4eb3-80db-6fc527a4e363',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'kubeflow',
          name: 'ml-pipeline',
          uid: '1d49eb54-e7fa-4827-8474-58e6b8c4a727'
        }
      ],
      resourceVersion: '18189',
      createdAt: '2020-10-02T14:33:20Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'kubeflow-katib-admin',
      uid: '432b54c0-8f03-4735-8620-d096cbab68b9',
      resourceVersion: '1765412',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'knative-serving-namespaced-admin',
      uid: 'db365205-d015-49f4-bcc7-5bd56c2063a8',
      resourceVersion: '18615',
      createdAt: '2020-10-02T14:33:36Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'workflow-controller',
      uid: '4df81191-1777-42ba-a4a6-908bc3d1fa41',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'argo',
          uid: '3eb053e7-1db8-4cd2-b840-7ca9b8d8712c'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '1765971',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-06T11:16:06Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'workflow-controller-754fc55df5',
      uid: '4f8211d3-2639-4a9b-869f-eefcb4668792',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'workflow-controller',
          uid: '4df81191-1777-42ba-a4a6-908bc3d1fa41'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '1765884',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-06T11:16:06Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'workflow-controller-754fc55df5-xqgsg',
      uid: '81bd0323-bdda-4c3a-b9d4-22d86cc41335',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'workflow-controller-754fc55df5',
          uid: '4f8211d3-2639-4a9b-869f-eefcb4668792'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'workflow-controller',
          'app.kubernetes.io/component': 'argo',
          'app.kubernetes.io/instance': 'argo-v2.11.2',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'argo',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v2.11.2',
          'kustomize.component': 'argo',
          'pod-template-hash': '754fc55df5'
        }
      },
      resourceVersion: '1765883',
      images: [
        'argoproj/workflow-controller:v2.11.2'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-06T11:16:06Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'experiments.kubeflow.org',
      uid: 'fe23223b-51c2-4476-a1c8-b87f564b6824',
      resourceVersion: '18274',
      createdAt: '2020-10-02T14:33:26Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'trials.kubeflow.org',
      uid: 'abd2ba06-5e58-4621-ae72-e59ac5478637',
      resourceVersion: '18419',
      createdAt: '2020-10-02T14:33:34Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'kfserving-webhook-server-secret',
      uid: '0c7a61e6-f907-4578-8a0f-5cd9cf4c1bfd',
      resourceVersion: '17988',
      createdAt: '2020-10-02T14:32:55Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRoleBinding',
      name: 'custom-metrics:system:auth-delegator',
      uid: 'e42fd4af-b4cc-4036-b674-a1594dbf3097',
      resourceVersion: '18837',
      createdAt: '2020-10-02T14:34:02Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'kubeflow-kubernetes-admin',
      uid: '135be68f-2c6d-4b01-838e-522ac25be277',
      resourceVersion: '18726',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'kubeflow',
      name: 'metadata-ui',
      uid: 'fb73d178-f0e4-49e2-9d3e-72edd7b73fce',
      resourceVersion: '18185',
      createdAt: '2020-10-02T14:33:20Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'metadata-ui-token-7lhp9',
      uid: '46e84f9d-a899-40df-a1b0-750c9408e4a8',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'kubeflow',
          name: 'metadata-ui',
          uid: 'fb73d178-f0e4-49e2-9d3e-72edd7b73fce'
        }
      ],
      resourceVersion: '18184',
      createdAt: '2020-10-02T14:33:20Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'notebook-controller-kubeflow-notebooks-admin',
      uid: '6dcfac3e-c17a-4065-8839-f6d69678f948',
      resourceVersion: '1765475',
      createdAt: '2020-10-02T14:33:38Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRoleBinding',
      name: 'admission-webhook-bootstrap-cluster-role-binding',
      uid: '07f42d24-7ea1-4c19-8283-fc17d7935a9e',
      resourceVersion: '18825',
      createdAt: '2020-10-02T14:34:12Z'
    },
    {
      group: 'networking.istio.io',
      version: 'v1alpha3',
      kind: 'VirtualService',
      namespace: 'kubeflow',
      name: 'jupyter-web-app',
      uid: '68640925-c39c-4600-aed7-c8686cdf6753',
      resourceVersion: '21353',
      createdAt: '2020-10-02T14:41:09Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'centraldashboard',
      uid: '49f3d35d-440a-4001-8d57-fd7d21326aa0',
      resourceVersion: '18725',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      group: 'networking.istio.io',
      version: 'v1beta1',
      kind: 'Gateway',
      namespace: 'knative-serving',
      name: 'cluster-local-gateway',
      uid: '7463ad35-89cd-4a5c-96b1-59cdcd799a34',
      resourceVersion: '21273',
      createdAt: '2020-10-02T14:40:58Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'ml-pipeline-ui',
      uid: '55307a70-2a88-4af2-a15a-534e37d91a13',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'pipelines-ui',
          uid: '8dfd933e-7ccb-4954-9796-943a7bca6c74'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '642679',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'ml-pipeline-ui-5fbd94b9fb',
      uid: 'e88fd335-20f0-475d-834b-9d5a768cb58a',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'ml-pipeline-ui',
          uid: '55307a70-2a88-4af2-a15a-534e37d91a13'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '642678',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'ml-pipeline-ui-5fbd94b9fb-rblmg',
      uid: '1589bd65-9d84-4fbc-8199-43ff74071c2d',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'ml-pipeline-ui-5fbd94b9fb',
          uid: 'e88fd335-20f0-475d-834b-9d5a768cb58a'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'ml-pipeline-ui',
          'app.kubernetes.io/component': 'pipelines-ui',
          'app.kubernetes.io/instance': 'pipelines-ui-0.2.5',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'pipelines-ui',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.5',
          'pod-template-hash': '5fbd94b9fb'
        }
      },
      resourceVersion: '642677',
      images: [
        'gcr.io/ml-pipeline/frontend:0.2.5'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:57Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRoleBinding',
      name: 'ml-pipeline-scheduledworkflow',
      uid: 'd4c771a8-312b-4052-9772-b897cf7bbb78',
      resourceVersion: '18793',
      createdAt: '2020-10-02T14:34:07Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'kubeflow',
      name: 'application-controller-service-account',
      uid: '9dff71f0-d4a1-4450-bfb4-8b4c182b14bf',
      resourceVersion: '18130',
      createdAt: '2020-10-02T14:33:16Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'application-controller-service-account-token-vwqh6',
      uid: '7885a1bd-92cf-486d-88db-6fc2c5078b70',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'kubeflow',
          name: 'application-controller-service-account',
          uid: '9dff71f0-d4a1-4450-bfb4-8b4c182b14bf'
        }
      ],
      resourceVersion: '18128',
      createdAt: '2020-10-02T14:33:16Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'kubeflow',
      name: 'application-controller-parameters',
      uid: 'caf55da6-9de4-42e7-a510-918cce9697b5',
      resourceVersion: '18015',
      createdAt: '2020-10-02T14:33:00Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRoleBinding',
      name: 'profiles-cluster-role-binding',
      uid: '75149078-1a3a-4f8d-8819-e9fbdfa39930',
      resourceVersion: '18853',
      createdAt: '2020-10-02T14:34:03Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'RoleBinding',
      namespace: 'kubeflow',
      name: 'argo-binding',
      uid: '01528961-0a23-42d1-94ea-3fc0f3c2878f',
      resourceVersion: '1765655',
      createdAt: '2020-10-02T14:34:18Z'
    },
    {
      group: 'networking.istio.io',
      version: 'v1alpha3',
      kind: 'VirtualService',
      namespace: 'kubeflow',
      name: 'google-storage-api-vs',
      uid: '813b4246-4f71-406b-96dc-b832995abebd',
      resourceVersion: '21316',
      createdAt: '2020-10-02T14:41:04Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'kubeflow',
      name: 'workflow-controller-configmap',
      uid: '01bee6c0-4784-45a7-9209-8c272e98bea1',
      resourceVersion: '1765134',
      createdAt: '2020-10-02T14:33:08Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'katib-controller',
      uid: 'df4de707-9bea-40f7-8fd5-2d368ddeb2d8',
      resourceVersion: '1427960',
      createdAt: '2020-10-02T14:32:55Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'knative-serving',
      name: 'activator',
      uid: 'f933538f-d9fe-42e6-a41c-18e91ab0474a',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'knative-serving',
          name: 'knative-serving-install',
          uid: '64044521-8f41-49ba-885c-1e4ce0b8d569'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '1549003',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'knative-serving',
      name: 'activator-68b949bd95',
      uid: '659ef5f5-6fe2-49dc-9ac4-331e4971cf85',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'knative-serving',
          name: 'activator',
          uid: 'f933538f-d9fe-42e6-a41c-18e91ab0474a'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '1549000',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:39Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'knative-serving',
      name: 'activator-68b949bd95-tl4n4',
      uid: 'e0712e5e-becd-49bd-af95-3c99509f9dcf',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'knative-serving',
          name: 'activator-68b949bd95',
          uid: '659ef5f5-6fe2-49dc-9ac4-331e4971cf85'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '2/2'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'activator',
          'app.kubernetes.io/component': 'knative-serving-install',
          'app.kubernetes.io/instance': 'knative-serving-install-v0.11.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'knative-serving-install',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v0.11.1',
          'kustomize.component': 'knative',
          'pod-template-hash': '68b949bd95',
          role: 'activator',
          'security.istio.io/tlsMode': 'istio',
          'service.istio.io/canonical-name': 'knative-serving-install',
          'service.istio.io/canonical-revision': 'v0.11.1',
          'serving.knative.dev/release': 'v0.11.2'
        }
      },
      resourceVersion: '1548999',
      images: [
        'docker.io/istio/proxyv2:1.5.10',
        'gcr.io/knative-releases/knative.dev/serving/cmd/activator@sha256:c51023e62e351d5910f92ee941b4929eb82539e62636dd3ccb4a016d73e86b2e'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:36:44Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'profiles-kfam',
      uid: '354c09d2-8d80-469d-9ad2-986ba896c54b',
      networkingInfo: {
        targetLabels: {
          'app.kubernetes.io/component': 'profiles',
          'app.kubernetes.io/instance': 'profiles-v1.0.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'profiles',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v1.0.0',
          'kustomize.component': 'profiles'
        }
      },
      resourceVersion: '20380',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'profiles-kfam',
      uid: 'b833c182-ca21-4391-a56b-2e1391d1f4c8',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'profiles-kfam',
          uid: '354c09d2-8d80-469d-9ad2-986ba896c54b'
        }
      ],
      resourceVersion: '643177',
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'kubeflow-tfjobs-admin',
      uid: '12a6563a-7cb9-4808-a85c-4caddcf7690b',
      resourceVersion: '1765541',
      createdAt: '2020-10-02T14:33:36Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'admission-webhook-kubeflow-poddefaults-view',
      uid: 'eccf81d8-f0b5-43ff-9697-28bab56c6504',
      resourceVersion: '18600',
      createdAt: '2020-10-02T14:33:36Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'applications.app.k8s.io',
      uid: 'b5e973fe-af43-4734-8769-b8e8cd078623',
      resourceVersion: '18371',
      createdAt: '2020-10-02T14:33:30Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'argo-cluster-role',
      uid: '01829b5f-71ee-4720-b12b-4604ce7f6cd1',
      resourceVersion: '1765446',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'knative-serving-addressable-resolver',
      uid: '2c28ba46-17e4-4e59-ac8b-61285a72e97e',
      resourceVersion: '18604',
      createdAt: '2020-10-02T14:33:36Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'kubeflow',
      name: 'admission-webhook-admission-webhook-parameters',
      uid: '5c3758c1-7749-4bed-9f5e-0ec24eab5900',
      resourceVersion: '18044',
      createdAt: '2020-10-02T14:33:05Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'kubeflow-kfserving-edit',
      uid: '3ffe95b4-4511-41d0-b981-2a05e953e454',
      resourceVersion: '18707',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      group: 'cert-manager.io',
      version: 'v1',
      kind: 'Certificate',
      namespace: 'kubeflow',
      name: 'serving-cert',
      uid: '24b8d7d5-1860-4ed0-ad89-57168c763f1c',
      resourceVersion: '21716',
      health: {
        status: 'Healthy',
        message: 'Certificate is up to date and has not expired'
      },
      createdAt: '2020-10-02T14:41:35Z'
    },
    {
      group: 'cert-manager.io',
      version: 'v1',
      kind: 'CertificateRequest',
      namespace: 'kubeflow',
      name: 'serving-cert-sgwk2',
      uid: 'a3e29260-0a26-41b5-a279-ba16b69e1037',
      parentRefs: [
        {
          group: 'cert-manager.io',
          kind: 'Certificate',
          namespace: 'kubeflow',
          name: 'serving-cert',
          uid: '24b8d7d5-1860-4ed0-ad89-57168c763f1c'
        }
      ],
      resourceVersion: '21701',
      createdAt: '2020-10-02T14:41:49Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'tensorboard',
      uid: '0653b1e2-1893-4aeb-a455-fc184afcdf3b',
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '642351',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'tensorboard-6549cd78c9',
      uid: '1a52c8c4-d4c4-4550-80f1-d17229669dda',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'tensorboard',
          uid: '0653b1e2-1893-4aeb-a455-fc184afcdf3b'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '642345',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'tensorboard-6549cd78c9-skcrz',
      uid: '95c44669-0069-4194-8af7-71cad7cb249e',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'tensorboard-6549cd78c9',
          uid: '1a52c8c4-d4c4-4550-80f1-d17229669dda'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'tensorboard',
          'kustomize.component': 'tensorboard',
          'pod-template-hash': '6549cd78c9'
        }
      },
      resourceVersion: '642076',
      images: [
        'tensorflow/tensorflow:1.8.0'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:40Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'kubeflow',
      name: 'argo',
      uid: 'ceb5b213-bc8a-4c6e-8d9b-05c2a31e788b',
      resourceVersion: '1765238',
      createdAt: '2020-10-02T14:33:16Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'argo-token-tzt28',
      uid: 'bdb76b6c-d4a7-4d08-92eb-c0d62df8fc58',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'kubeflow',
          name: 'argo',
          uid: 'ceb5b213-bc8a-4c6e-8d9b-05c2a31e788b'
        }
      ],
      resourceVersion: '18129',
      createdAt: '2020-10-02T14:33:16Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'custom-metrics-server-resources',
      uid: '93a2e916-943c-43c6-9d1a-9f127c68ab7d',
      resourceVersion: '18735',
      createdAt: '2020-10-02T14:33:38Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'kubeflow',
      name: 'admission-webhook-service-account',
      uid: 'fbc08ad0-cb78-4e19-a195-f7270c86a455',
      resourceVersion: '18201',
      createdAt: '2020-10-02T14:33:20Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'admission-webhook-service-account-token-vkdkw',
      uid: 'bd4654ba-abaf-4f61-bd5a-2f406244ebda',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'kubeflow',
          name: 'admission-webhook-service-account',
          uid: 'fbc08ad0-cb78-4e19-a195-f7270c86a455'
        }
      ],
      resourceVersion: '18195',
      createdAt: '2020-10-02T14:33:20Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'profiles',
      uid: '201e1b4c-2af7-4138-b5dc-02fb296536cb',
      resourceVersion: '1766245',
      createdAt: '2020-10-02T14:41:25Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'profiles-deployment',
      uid: '47f8ea51-c790-48ce-b400-47d4bdb1c73c',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'profiles',
          uid: '201e1b4c-2af7-4138-b5dc-02fb296536cb'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '643176',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'profiles-deployment-67db6957fc',
      uid: 'f8b8ff8f-8149-4d7f-9394-1d083325a5cf',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'profiles-deployment',
          uid: '47f8ea51-c790-48ce-b400-47d4bdb1c73c'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '643174',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'profiles-deployment-67db6957fc-htr9t',
      uid: 'ce2ba13f-e535-48cb-9941-6d997123ada3',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'profiles-deployment-67db6957fc',
          uid: 'f8b8ff8f-8149-4d7f-9394-1d083325a5cf'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '2/2'
        }
      ],
      networkingInfo: {
        labels: {
          'app.kubernetes.io/component': 'profiles',
          'app.kubernetes.io/instance': 'profiles-v1.0.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'profiles',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v1.0.0',
          'kustomize.component': 'profiles',
          'pod-template-hash': '67db6957fc'
        }
      },
      resourceVersion: '643173',
      images: [
        'auroraprodacr.azurecr.io/kubeflow-dev/profile-controller:v20200619-v0.7.0-rc.5-148-g253890cb-dirty-7346f1',
        'gcr.io/kubeflow-images-public/kfam:v1.0.0-gf3e09203'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:48Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'kubeflow',
      name: 'ml-pipeline-scheduledworkflow',
      uid: '4b726394-42ae-4dcb-a7b4-2a27558af308',
      resourceVersion: '18203',
      createdAt: '2020-10-02T14:33:20Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'ml-pipeline-scheduledworkflow-token-nq4sq',
      uid: '92ac43a3-b162-4260-b838-69a5f8b3103d',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'kubeflow',
          name: 'ml-pipeline-scheduledworkflow',
          uid: '4b726394-42ae-4dcb-a7b4-2a27558af308'
        }
      ],
      resourceVersion: '18197',
      createdAt: '2020-10-02T14:33:20Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'kubeflow',
      name: 'kfserving-config',
      uid: '6e08e493-f374-43e4-9dcb-bc90a5e987c0',
      resourceVersion: '18036',
      createdAt: '2020-10-02T14:33:04Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'knative-serving',
      name: 'autoscaler',
      uid: '64d92ae4-f0c3-4855-843f-ab33c05c3632',
      networkingInfo: {
        targetLabels: {
          app: 'autoscaler',
          'app.kubernetes.io/component': 'knative-serving-install',
          'app.kubernetes.io/instance': 'knative-serving-install-v0.11.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'knative-serving-install',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v0.11.1',
          'kustomize.component': 'knative'
        }
      },
      resourceVersion: '20353',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'knative-serving',
      name: 'autoscaler',
      uid: 'b142a364-d079-4d83-9ce3-c8e2d0bea5dd',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'knative-serving',
          name: 'autoscaler',
          uid: '64d92ae4-f0c3-4855-843f-ab33c05c3632'
        }
      ],
      resourceVersion: '1589536',
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      group: 'rbac.istio.io',
      version: 'v1alpha1',
      kind: 'ServiceRole',
      namespace: 'knative-serving',
      name: 'istio-service-role',
      uid: 'da30c462-baa4-41c6-8452-b6142973af97',
      resourceVersion: '21335',
      createdAt: '2020-10-02T14:41:06Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'Role',
      namespace: 'kubeflow',
      name: 'ml-pipeline',
      uid: '910be434-1393-4f1a-a395-32cbe0380d81',
      resourceVersion: '18883',
      createdAt: '2020-10-02T14:34:18Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'katib-db-manager',
      uid: 'd43e2d33-a8b4-4232-8d35-2c92d661884f',
      networkingInfo: {
        targetLabels: {
          app: 'katib',
          'app.kubernetes.io/component': 'katib',
          'app.kubernetes.io/instance': 'katib-controller-0.8.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'katib-controller',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.8.0',
          component: 'db-manager'
        }
      },
      resourceVersion: '20412',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:11Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'katib-db-manager',
      uid: '09781e66-d623-4526-961d-aa9e6bd2be3c',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'katib-db-manager',
          uid: 'd43e2d33-a8b4-4232-8d35-2c92d661884f'
        }
      ],
      resourceVersion: '645241',
      createdAt: '2020-10-02T14:40:11Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'kubeflow',
      name: 'centraldashboard',
      uid: '258f7f0f-7f79-482c-94a2-b5b42996c6fa',
      resourceVersion: '18225',
      createdAt: '2020-10-02T14:33:21Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'centraldashboard-token-v7sdv',
      uid: 'b743e404-eae4-4e77-8bf9-ce707c8fbc5d',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'kubeflow',
          name: 'centraldashboard',
          uid: '258f7f0f-7f79-482c-94a2-b5b42996c6fa'
        }
      ],
      resourceVersion: '18223',
      createdAt: '2020-10-02T14:33:21Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'kubeflow',
      name: 'istio-parameters-t6hhgfg9k2',
      uid: '1fdfbd19-47cd-409f-9031-edfd684be1fb',
      resourceVersion: '18071',
      createdAt: '2020-10-02T14:33:08Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'katib-ui',
      uid: 'f84a19bc-f434-41e9-a56b-b8ecedef22fe',
      networkingInfo: {
        targetLabels: {
          app: 'katib',
          'app.kubernetes.io/component': 'katib',
          'app.kubernetes.io/instance': 'katib-controller-0.8.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'katib-controller',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.8.0',
          component: 'ui'
        }
      },
      resourceVersion: '20464',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:15Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'katib-ui',
      uid: '837ed038-4e74-43fd-9898-01424b326dcf',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'katib-ui',
          uid: 'f84a19bc-f434-41e9-a56b-b8ecedef22fe'
        }
      ],
      resourceVersion: '643737',
      createdAt: '2020-10-02T14:40:15Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'podautoscalers.autoscaling.internal.knative.dev',
      uid: 'c2be19eb-6a2f-4ba0-8bf9-c2d833e0990b',
      resourceVersion: '18316',
      createdAt: '2020-10-02T14:33:30Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRoleBinding',
      name: 'meta-controller-cluster-role-binding',
      uid: '8643641b-6bd3-43a4-a49f-1f2179ee04f3',
      resourceVersion: '18854',
      createdAt: '2020-10-02T14:34:03Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'ml-pipeline-ml-pipeline-visualizationserver',
      uid: '6b3ab698-ecc6-41c2-bd07-6d165b0312d0',
      networkingInfo: {
        targetLabels: {
          app: 'ml-pipeline-visualizationserver',
          'app.kubernetes.io/component': 'pipeline-visualization-service',
          'app.kubernetes.io/instance': 'pipeline-visualization-service-0.2.5',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'pipeline-visualization-service',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.5'
        }
      },
      resourceVersion: '20370',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'ml-pipeline-ml-pipeline-visualizationserver',
      uid: '01433904-0465-4ed6-8a2a-034aafdc4f01',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'ml-pipeline-ml-pipeline-visualizationserver',
          uid: '6b3ab698-ecc6-41c2-bd07-6d165b0312d0'
        }
      ],
      resourceVersion: '643725',
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'clusterworkflowtemplates.argoproj.io',
      uid: '5c927bb2-d2ce-4ced-afe0-53e5987199c7',
      resourceVersion: '1765319',
      createdAt: '2020-10-02T14:33:30Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'argo-aggregate-to-view',
      uid: '83b2fe80-35fe-4993-ba55-92a51de55c2e',
      resourceVersion: '1765520',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'spark-operatoroperator-cr',
      uid: 'de5b4824-3896-4b37-b896-518bbe56bb7e',
      resourceVersion: '18723',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'Role',
      namespace: 'kubeflow',
      name: 'argo-role',
      uid: '24931ef7-abec-4fcd-8baa-473d77f021a0',
      resourceVersion: '1765632',
      createdAt: '2020-10-02T14:34:14Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'kubeflow',
      name: 'jupyter-web-app-service-account',
      uid: '065a2ad5-74e2-43ea-abab-cb1fa559e17c',
      resourceVersion: '18204',
      createdAt: '2020-10-02T14:33:20Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'jupyter-web-app-service-account-token-5chsc',
      uid: '3d8a03c2-6234-4d03-9574-a51a9a8b110e',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'kubeflow',
          name: 'jupyter-web-app-service-account',
          uid: '065a2ad5-74e2-43ea-abab-cb1fa559e17c'
        }
      ],
      resourceVersion: '18199',
      createdAt: '2020-10-02T14:33:20Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'metrics.autoscaling.internal.knative.dev',
      uid: 'a797e492-8383-4331-97d2-2aae07ca41d6',
      resourceVersion: '18322',
      createdAt: '2020-10-02T14:33:30Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'knative-serving-core',
      uid: '5d3a355e-f700-4bb5-8a53-3032ff7b0d07',
      resourceVersion: '18607',
      createdAt: '2020-10-02T14:33:36Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'spark-operatorsparkoperator',
      uid: '3a2a731c-887e-47a3-9371-ee8506daae78',
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '642893',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'spark-operatorsparkoperator-7c484c6859',
      uid: '32bf6258-0a2c-4aec-8765-c428a26df233',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'spark-operatorsparkoperator',
          uid: '3a2a731c-887e-47a3-9371-ee8506daae78'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '642892',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'spark-operatorsparkoperator-7c484c6859-njgp5',
      uid: 'f16cf400-947f-4822-b5b2-c6c5877b7667',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'spark-operatorsparkoperator-7c484c6859',
          uid: '32bf6258-0a2c-4aec-8765-c428a26df233'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          'app.kubernetes.io/component': 'spark-operator',
          'app.kubernetes.io/instance': 'spark-operator-v1.0.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'sparkoperator',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v1.0.0',
          'kustomize.component': 'spark-operator',
          'pod-template-hash': '7c484c6859'
        }
      },
      resourceVersion: '642889',
      images: [
        'gcr.io/spark-operator/spark-operator:v1beta2-1.0.0-2.4.4'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:35Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'application-controller-cluster-role',
      uid: '54dcb269-7fb8-42d5-b728-52a655d26703',
      resourceVersion: '18613',
      createdAt: '2020-10-02T14:33:36Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'StatefulSet',
      namespace: 'kubeflow',
      name: 'application-controller-stateful-set',
      uid: '41a1d4ac-2111-4fcb-9bb8-e4d40bda906b',
      resourceVersion: '642843',
      health: {
        status: 'Healthy',
        message: 'partitioned roll out complete: 1 new pods have been updated...'
      },
      createdAt: '2020-10-02T14:40:48Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'application-controller-stateful-set-0',
      uid: '530bbe26-ddc5-47eb-974e-4649baf63847',
      parentRefs: [
        {
          group: 'apps',
          kind: 'StatefulSet',
          namespace: 'kubeflow',
          name: 'application-controller-stateful-set',
          uid: '41a1d4ac-2111-4fcb-9bb8-e4d40bda906b'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'application-controller',
          'app.kubernetes.io/component': 'kubeflow',
          'app.kubernetes.io/instance': 'kubeflow-v1.0.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'kubeflow',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v1.0.0',
          'controller-revision-hash': 'application-controller-stateful-set-77466bd685',
          'statefulset.kubernetes.io/pod-name': 'application-controller-stateful-set-0'
        }
      },
      resourceVersion: '642842',
      images: [
        'gcr.io/kubeflow-images-public/kubernetes-sigs/application:1.0-beta'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:48Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ControllerRevision',
      namespace: 'kubeflow',
      name: 'application-controller-stateful-set-77466bd685',
      uid: '7d10b746-ec95-4e7b-8943-cd669edb6004',
      parentRefs: [
        {
          group: 'apps',
          kind: 'StatefulSet',
          namespace: 'kubeflow',
          name: 'application-controller-stateful-set',
          uid: '41a1d4ac-2111-4fcb-9bb8-e4d40bda906b'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '21135',
      createdAt: '2020-10-02T14:40:48Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'kubeflow-kubernetes-edit',
      uid: '02b7070a-7e61-4de0-b4e0-5cb20ccd07f9',
      resourceVersion: '18686',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'metadata-envoy-service',
      uid: '2f30b20f-1657-4e80-ab86-4b9d5c5e73fa',
      networkingInfo: {
        targetLabels: {
          'app.kubernetes.io/component': 'metadata',
          'app.kubernetes.io/instance': 'metadata-0.2.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'metadata',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.1',
          component: 'envoy',
          'kustomize.component': 'metadata'
        }
      },
      resourceVersion: '20339',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:07Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'metadata-envoy-service',
      uid: '9f8a3054-9270-4927-88de-5e589adde7df',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'metadata-envoy-service',
          uid: '2f30b20f-1657-4e80-ab86-4b9d5c5e73fa'
        }
      ],
      resourceVersion: '640648',
      createdAt: '2020-10-02T14:40:07Z'
    },
    {
      version: 'v1',
      kind: 'PersistentVolumeClaim',
      namespace: 'kubeflow',
      name: 'metadata-mysql',
      uid: '321d0602-8247-4096-be91-f2a11e031322',
      resourceVersion: '18247',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:33:11Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'katib-controller',
      uid: '3e830183-bb37-4b9e-ad33-34b84f02ce2b',
      resourceVersion: '5987504',
      createdAt: '2020-10-02T14:41:11Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'katib-db-manager',
      uid: 'f7bbc2e9-dbf9-4eb8-8d63-f2fd9b317fbc',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'katib-controller',
          uid: '3e830183-bb37-4b9e-ad33-34b84f02ce2b'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645242',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'katib-db-manager-595b5ffd88',
      uid: '1166c8aa-d378-4199-ba6d-b85c2ca9492f',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'katib-db-manager',
          uid: 'f7bbc2e9-dbf9-4eb8-8d63-f2fd9b317fbc'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645240',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'katib-db-manager-595b5ffd88-f9wjr',
      uid: '7fb78886-c26b-4e27-84a9-4381154df197',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'katib-db-manager-595b5ffd88',
          uid: '1166c8aa-d378-4199-ba6d-b85c2ca9492f'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'katib',
          'app.kubernetes.io/component': 'katib',
          'app.kubernetes.io/instance': 'katib-controller-0.8.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'katib-controller',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.8.0',
          component: 'db-manager',
          'pod-template-hash': '595b5ffd88'
        }
      },
      resourceVersion: '645239',
      images: [
        'gcr.io/kubeflow-images-public/katib/v1alpha3/katib-db-manager@sha256:0431ac5b9fd80169c71f7a70cec8607118bbc82988d08d9eef99f8f628afc772'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:43Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'katib-mysql',
      uid: '530197c3-2542-4e27-9922-76392d80dcc5',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'katib-controller',
          uid: '3e830183-bb37-4b9e-ad33-34b84f02ce2b'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645180',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:42Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'katib-mysql-74747879d7',
      uid: '24645d8e-67ca-4587-aa12-ba0aeab99d4a',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'katib-mysql',
          uid: '530197c3-2542-4e27-9922-76392d80dcc5'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645179',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:42Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'katib-mysql-74747879d7-tqr2s',
      uid: '060571b8-a405-4574-99c9-d0e9f2d1902b',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'katib-mysql-74747879d7',
          uid: '24645d8e-67ca-4587-aa12-ba0aeab99d4a'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'katib',
          'app.kubernetes.io/component': 'katib',
          'app.kubernetes.io/instance': 'katib-controller-0.8.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'katib-controller',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.8.0',
          component: 'mysql',
          'pod-template-hash': '74747879d7'
        }
      },
      resourceVersion: '645178',
      images: [
        'mysql:8'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:52Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'katib-ui',
      uid: 'b0197b81-2971-44ac-914b-b28c32ccd640',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'katib-controller',
          uid: '3e830183-bb37-4b9e-ad33-34b84f02ce2b'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '643736',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:39Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'katib-ui-5d68b6c84b',
      uid: '995b75cb-470d-476c-863e-515efe999441',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'katib-ui',
          uid: 'b0197b81-2971-44ac-914b-b28c32ccd640'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '643735',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:40Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'katib-ui-5d68b6c84b-rjx84',
      uid: 'd282cb48-2702-4dd5-92e6-95b783268c73',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'katib-ui-5d68b6c84b',
          uid: '995b75cb-470d-476c-863e-515efe999441'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'katib',
          'app.kubernetes.io/component': 'katib',
          'app.kubernetes.io/instance': 'katib-controller-0.8.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'katib-controller',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.8.0',
          component: 'ui',
          'pod-template-hash': '5d68b6c84b'
        }
      },
      resourceVersion: '643734',
      images: [
        'gcr.io/kubeflow-images-public/katib/v1alpha3/katib-ui@sha256:783136045e37d4e71b21c1c14ef5739f9d8a997dae40a36d60a936d9545cd871'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:58Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'katib-controller',
      uid: 'b80aee85-edb7-4b8c-9c92-09c9e95d7da8',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'katib-controller',
          uid: '3e830183-bb37-4b9e-ad33-34b84f02ce2b'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '643160',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'katib-controller-66888bbf5',
      uid: '8d8593bd-9a61-4a7d-a038-e682874f9eb5',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'katib-controller',
          uid: 'b80aee85-edb7-4b8c-9c92-09c9e95d7da8'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '643158',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:46Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'katib-controller-66888bbf5-2pg7r',
      uid: '52336440-bd87-435f-ab67-f5fdc80d5613',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'katib-controller-66888bbf5',
          uid: '8d8593bd-9a61-4a7d-a038-e682874f9eb5'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'katib-controller',
          'app.kubernetes.io/component': 'katib',
          'app.kubernetes.io/instance': 'katib-controller-0.8.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'katib-controller',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.8.0',
          'pod-template-hash': '66888bbf5'
        }
      },
      resourceVersion: '643157',
      images: [
        'gcr.io/kubeflow-images-public/katib/v1alpha3/katib-controller@sha256:46b6a3d2611d32fa75e072faa9fe6d72a234de0a20cc967f36c3695b7505e990'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:56Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'StatefulSet',
      namespace: 'kubeflow',
      name: 'kfserving-controller-manager',
      uid: '5b032c82-2159-4d45-abfb-3adab7ce8b73',
      resourceVersion: '642727',
      health: {
        status: 'Healthy',
        message: 'partitioned roll out complete: 1 new pods have been updated...'
      },
      createdAt: '2020-10-02T14:40:48Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'kfserving-controller-manager-0',
      uid: '045ddba8-acec-46a4-b056-6e07a1a5675e',
      parentRefs: [
        {
          group: 'apps',
          kind: 'StatefulSet',
          namespace: 'kubeflow',
          name: 'kfserving-controller-manager',
          uid: '5b032c82-2159-4d45-abfb-3adab7ce8b73'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '2/2'
        }
      ],
      networkingInfo: {
        labels: {
          'control-plane': 'kfserving-controller-manager',
          'controller-revision-hash': 'kfserving-controller-manager-68d7b6566d',
          'controller-tools.k8s.io': '1.0',
          'kustomize.component': 'kfserving',
          'statefulset.kubernetes.io/pod-name': 'kfserving-controller-manager-0'
        }
      },
      resourceVersion: '642726',
      images: [
        'gcr.io/kubebuilder/kube-rbac-proxy:v0.4.0',
        'gcr.io/kfserving/kfserving-controller:v0.3.0'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:55Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ControllerRevision',
      namespace: 'kubeflow',
      name: 'kfserving-controller-manager-68d7b6566d',
      uid: '793c35cf-28d0-47fd-b3ef-583e86e9836e',
      parentRefs: [
        {
          group: 'apps',
          kind: 'StatefulSet',
          namespace: 'kubeflow',
          name: 'kfserving-controller-manager',
          uid: '5b032c82-2159-4d45-abfb-3adab7ce8b73'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '21133',
      createdAt: '2020-10-02T14:40:48Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'kubeflow-kubernetes-view',
      uid: '635655cb-39fd-4133-84c6-7cc1e595b90c',
      resourceVersion: '18624',
      createdAt: '2020-10-02T14:33:36Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'Role',
      namespace: 'kubeflow',
      name: 'ml-pipeline-ui',
      uid: 'a34730c1-7dfc-4d01-be15-b8d8f0a0e987',
      resourceVersion: '18881',
      createdAt: '2020-10-02T14:34:18Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'kubeflow',
      name: 'ml-pipeline-viewer-crd-service-account',
      uid: '66ba0897-00a2-46a4-91a7-788eb37025fc',
      resourceVersion: '18205',
      createdAt: '2020-10-02T14:33:20Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'ml-pipeline-viewer-crd-service-account-token-ldlqr',
      uid: 'ad716524-0ea0-4800-b414-f37851edf6b1',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'kubeflow',
          name: 'ml-pipeline-viewer-crd-service-account',
          uid: '66ba0897-00a2-46a4-91a7-788eb37025fc'
        }
      ],
      resourceVersion: '18200',
      createdAt: '2020-10-02T14:33:20Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'kubeflow',
      name: 'tf-job-operator',
      uid: '62431bdd-ba63-4981-bec9-308e9a251216',
      resourceVersion: '18172',
      createdAt: '2020-10-02T14:33:19Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'tf-job-operator-token-mtbrr',
      uid: 'f9157ace-8371-45fb-8b09-e4661fc243a7',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'kubeflow',
          name: 'tf-job-operator',
          uid: '62431bdd-ba63-4981-bec9-308e9a251216'
        }
      ],
      resourceVersion: '18166',
      createdAt: '2020-10-02T14:33:19Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'kubeflow',
      name: 'ml-pipeline-config',
      uid: '10a307e5-c32e-4ec3-8a77-93be75eafa23',
      resourceVersion: '18055',
      createdAt: '2020-10-02T14:33:05Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'kubeflow-katib-view',
      uid: 'e4106c47-7552-4086-9f13-830246cf028b',
      resourceVersion: '18667',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'jupyter-web-app-service',
      uid: 'fe852b1c-5033-4beb-a845-b7ae295df712',
      networkingInfo: {
        targetLabels: {
          app: 'jupyter-web-app',
          'app.kubernetes.io/component': 'jupyter-web-app',
          'app.kubernetes.io/instance': 'jupyter-web-app-v1.0.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'jupyter-web-app',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v1.0.0',
          'kustomize.component': 'jupyter-web-app'
        }
      },
      resourceVersion: '20372',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'jupyter-web-app-service',
      uid: '7682aa0f-5e52-48b2-821a-6041471a1b97',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'jupyter-web-app-service',
          uid: 'fe852b1c-5033-4beb-a845-b7ae295df712'
        }
      ],
      resourceVersion: '643957',
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRoleBinding',
      name: 'manager-rolebinding',
      uid: '3bc9d08e-8a62-4c81-b2b1-96320b0e4742',
      resourceVersion: '18836',
      createdAt: '2020-10-02T14:34:02Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'knative-serving-istio',
      uid: '2dfa9edc-247d-4f12-94a7-6e9d700ad3d2',
      resourceVersion: '18621',
      createdAt: '2020-10-02T14:33:36Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'knative-serving-namespaced-edit',
      uid: '40e0c31f-ce43-45f8-aef3-fdb63b72052b',
      resourceVersion: '18619',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'knative-serving',
      name: 'controller',
      uid: 'c16b6708-cb9c-4bc5-93ff-8e922b830c21',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'knative-serving',
          name: 'knative-serving-install',
          uid: '64044521-8f41-49ba-885c-1e4ce0b8d569'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '635609',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:40Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'knative-serving',
      name: 'controller-6f85bdc877',
      uid: 'e55cd174-f0ed-4c8c-b9bb-086bfd46d395',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'knative-serving',
          name: 'controller',
          uid: 'c16b6708-cb9c-4bc5-93ff-8e922b830c21'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '635607',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:40Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'knative-serving',
      name: 'controller-6f85bdc877-jzcpk',
      uid: 'e82109b1-1ccf-4fe7-8ed7-6b6d5bbe00c4',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'knative-serving',
          name: 'controller-6f85bdc877',
          uid: 'e55cd174-f0ed-4c8c-b9bb-086bfd46d395'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'controller',
          'app.kubernetes.io/component': 'knative-serving-install',
          'app.kubernetes.io/instance': 'knative-serving-install-v0.11.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'knative-serving-install',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v0.11.1',
          'kustomize.component': 'knative',
          'pod-template-hash': '6f85bdc877',
          'serving.knative.dev/release': 'v0.11.2'
        }
      },
      resourceVersion: '635606',
      images: [
        'gcr.io/knative-releases/knative.dev/serving/cmd/controller@sha256:1e77bdab30c8d0f0df299f5fa93d6f99eb63071b9d3329937dff0c6acb99e059'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:19:18Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'ml-pipeline-persistenceagent',
      uid: 'aab19f04-bb6e-4e05-ae99-e4e1ae79d4f6',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'persistent-agent',
          uid: '90682aec-5f5d-456c-9d19-e40217a34857'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645416',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'ml-pipeline-persistenceagent-7785884886',
      uid: '59b0558a-d0b0-4da5-ba13-52240eb84940',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'ml-pipeline-persistenceagent',
          uid: 'aab19f04-bb6e-4e05-ae99-e4e1ae79d4f6'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645415',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'ml-pipeline-persistenceagent-7785884886-rl6wk',
      uid: '270125a3-8385-4e5f-afe1-ffb3919e6e02',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'ml-pipeline-persistenceagent-7785884886',
          uid: '59b0558a-d0b0-4da5-ba13-52240eb84940'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'ml-pipeline-persistenceagent',
          'app.kubernetes.io/component': 'persistent-agent',
          'app.kubernetes.io/instance': 'persistent-agent-0.2.5',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'persistent-agent',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.5',
          'pod-template-hash': '7785884886'
        }
      },
      resourceVersion: '645414',
      images: [
        'gcr.io/ml-pipeline/persistenceagent:0.2.5'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:47Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'ml-pipeline',
      uid: '8eb51c99-1274-4b25-ba76-2a438f970ada',
      networkingInfo: {
        targetLabels: {
          app: 'ml-pipeline',
          'app.kubernetes.io/component': 'api-service',
          'app.kubernetes.io/instance': 'api-service-0.2.5',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'api-service',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.5'
        }
      },
      resourceVersion: '20361',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'ml-pipeline',
      uid: 'f735ffa9-c1dc-47ee-952a-54a81be908b4',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'ml-pipeline',
          uid: '8eb51c99-1274-4b25-ba76-2a438f970ada'
        }
      ],
      resourceVersion: '645458',
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'kubeflow-istio-edit',
      uid: '531998b3-f59c-4482-aa44-7560bc6743db',
      resourceVersion: '18692',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRoleBinding',
      name: 'tf-job-operator',
      uid: '30d1a8c4-deac-4386-8093-aa9b67173eee',
      resourceVersion: '18799',
      createdAt: '2020-10-02T14:34:08Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'pytorch-operator',
      uid: '4855416e-4f29-415b-aa45-3f4ce012ae04',
      networkingInfo: {
        targetLabels: {
          'app.kubernetes.io/component': 'pytorch',
          'app.kubernetes.io/instance': 'pytorch-operator-v1.0.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'pytorch-operator',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v1.0.0',
          'kustomize.component': 'pytorch-operator',
          name: 'pytorch-operator'
        }
      },
      resourceVersion: '20326',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:05Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'pytorch-operator',
      uid: '7cc231cc-31a2-476a-9ae6-8b24fba965f0',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'pytorch-operator',
          uid: '4855416e-4f29-415b-aa45-3f4ce012ae04'
        }
      ],
      resourceVersion: '5693462',
      createdAt: '2020-10-02T14:40:05Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'seldon-manager-sas-role-kubeflow',
      uid: 'def76eb6-ec3c-468c-9eb6-aaadae52a5db',
      resourceVersion: '18721',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'metadata-service',
      uid: '8900ba5f-5f2f-444d-a03a-680100b96b98',
      networkingInfo: {
        targetLabels: {
          'app.kubernetes.io/component': 'metadata',
          'app.kubernetes.io/instance': 'metadata-0.2.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'metadata',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.1',
          component: 'server',
          'kustomize.component': 'metadata'
        }
      },
      resourceVersion: '20458',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:15Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'metadata-service',
      uid: 'd0b41c7e-4fad-4c46-9de8-a4be5a074839',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'metadata-service',
          uid: '8900ba5f-5f2f-444d-a03a-680100b96b98'
        }
      ],
      resourceVersion: '640655',
      createdAt: '2020-10-02T14:40:15Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'notebook-controller-kubeflow-notebooks-view',
      uid: '26d075cb-a903-4431-9d28-b44542de631f',
      resourceVersion: '18685',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'admission-webhook-kubeflow-poddefaults-edit',
      uid: '0dbd95f7-ccf6-4fd4-9686-5a3554697dd6',
      resourceVersion: '1765521',
      createdAt: '2020-10-02T14:33:36Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'katib-controller',
      uid: 'b80aee85-edb7-4b8c-9c92-09c9e95d7da8',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'katib-controller',
          uid: '3e830183-bb37-4b9e-ad33-34b84f02ce2b'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '643160',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:45Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'katib-controller-66888bbf5',
      uid: '8d8593bd-9a61-4a7d-a038-e682874f9eb5',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'katib-controller',
          uid: 'b80aee85-edb7-4b8c-9c92-09c9e95d7da8'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '643158',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:46Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'katib-controller-66888bbf5-2pg7r',
      uid: '52336440-bd87-435f-ab67-f5fdc80d5613',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'katib-controller-66888bbf5',
          uid: '8d8593bd-9a61-4a7d-a038-e682874f9eb5'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'katib-controller',
          'app.kubernetes.io/component': 'katib',
          'app.kubernetes.io/instance': 'katib-controller-0.8.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'katib-controller',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.8.0',
          'pod-template-hash': '66888bbf5'
        }
      },
      resourceVersion: '643157',
      images: [
        'gcr.io/kubeflow-images-public/katib/v1alpha3/katib-controller@sha256:46b6a3d2611d32fa75e072faa9fe6d72a234de0a20cc967f36c3695b7505e990'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:56Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'ingresses.networking.internal.knative.dev',
      uid: '05b9122a-191c-4d47-80e4-04046d21ca9f',
      resourceVersion: '18349',
      createdAt: '2020-10-02T14:33:30Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'jupyter-web-app-kubeflow-notebook-ui-edit',
      uid: 'bb1db7b2-312e-4715-82ca-6500c3291136',
      resourceVersion: '18608',
      createdAt: '2020-10-02T14:33:36Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'kfserving-webhook-server-service',
      uid: 'e8393b37-f519-48f1-bb31-2d8c066e085b',
      networkingInfo: {
        targetLabels: {
          'control-plane': 'kfserving-controller-manager',
          'kustomize.component': 'kfserving'
        }
      },
      resourceVersion: '20429',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:13Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'kfserving-webhook-server-service',
      uid: '8c831284-5b8e-417d-9891-50916c5174e7',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'kfserving-webhook-server-service',
          uid: 'e8393b37-f519-48f1-bb31-2d8c066e085b'
        }
      ],
      resourceVersion: '642728',
      createdAt: '2020-10-02T14:40:13Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'knative-serving',
      name: 'config-gc',
      uid: '9f799021-2966-44d0-bc75-a95580ad6a6d',
      resourceVersion: '18030',
      createdAt: '2020-10-02T14:33:03Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'notebook-controller-deployment',
      uid: '747d5b0b-13df-4383-88e8-01d69e65e125',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'notebook-controller',
          uid: '4c1a5510-0da6-4c84-9781-39e24ad73d97'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '642951',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:40Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'notebook-controller-deployment-798559cf4d',
      uid: 'f109eca4-b5f5-4bdd-9905-6b6e882a04e9',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'notebook-controller-deployment',
          uid: '747d5b0b-13df-4383-88e8-01d69e65e125'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '642950',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:40Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'notebook-controller-deployment-798559cf4d-qpvfw',
      uid: 'd61c9903-3bea-422b-bfd6-378e1ae3683e',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'notebook-controller-deployment-798559cf4d',
          uid: 'f109eca4-b5f5-4bdd-9905-6b6e882a04e9'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'notebook-controller',
          'app.kubernetes.io/component': 'notebook-controller',
          'app.kubernetes.io/instance': 'notebook-controller-v1.0.0',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'notebook-controller',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v1.0.0',
          'kustomize.component': 'notebook-controller',
          'pod-template-hash': '798559cf4d'
        }
      },
      resourceVersion: '642949',
      images: [
        'gcr.io/kubeflow-images-public/notebook-controller:v1.0.0-gcd65ce25'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:57Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'ml-pipeline-tensorboard-ui',
      uid: '94061b90-2e1f-4d65-bc0a-4149ea5c257d',
      networkingInfo: {
        targetLabels: {
          app: 'ml-pipeline-tensorboard-ui',
          'app.kubernetes.io/component': 'pipelines-ui',
          'app.kubernetes.io/instance': 'pipelines-ui-0.2.5',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'pipelines-ui',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.5'
        }
      },
      resourceVersion: '20402',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'ml-pipeline-tensorboard-ui',
      uid: '44edfa9a-f44d-402b-95b6-e9cd083e4385',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'ml-pipeline-tensorboard-ui',
          uid: '94061b90-2e1f-4d65-bc0a-4149ea5c257d'
        }
      ],
      resourceVersion: '20403',
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'ml-pipeline-viewer-kubeflow-pipeline-viewers-edit',
      uid: '1e64e713-7497-4954-8ba6-f226ed12d050',
      resourceVersion: '18747',
      createdAt: '2020-10-02T14:33:38Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'kubeflow',
      name: 'admission-webhook-bootstrap-service-account',
      uid: '0a70d10a-c87e-45fe-af5a-e0149dc8c5af',
      resourceVersion: '18152',
      createdAt: '2020-10-02T14:33:19Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'admission-webhook-bootstrap-service-account-token-rtnh6',
      uid: '2bd1cfce-48f3-4014-a044-0f29de881a0c',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'kubeflow',
          name: 'admission-webhook-bootstrap-service-account',
          uid: '0a70d10a-c87e-45fe-af5a-e0149dc8c5af'
        }
      ],
      resourceVersion: '18151',
      createdAt: '2020-10-02T14:33:19Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'Role',
      namespace: 'kubeflow',
      name: 'jupyter-web-app-jupyter-notebook-role',
      uid: 'aeba3765-2a19-4dcc-b277-08257991a391',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'jupyter-web-app',
          uid: '836e36d8-cf05-4c30-9705-aaf0aff02345'
        }
      ],
      resourceVersion: '22376',
      createdAt: '2020-10-02T14:34:18Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'tfjobs.kubeflow.org',
      uid: '290858cf-6ef7-45de-a930-e406e8aed2dc',
      resourceVersion: '18405',
      createdAt: '2020-10-02T14:33:34Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRoleBinding',
      name: 'seldon-manager-sas-rolebinding-kubeflow',
      uid: '0a13988e-7740-465f-9d8c-01e7b9dc14a4',
      resourceVersion: '18849',
      createdAt: '2020-10-02T14:34:03Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'sparkapplications.sparkoperator.k8s.io',
      uid: '39a6a8f3-89fb-4a9f-822a-f8cce147d262',
      resourceVersion: '25283',
      createdAt: '2020-10-02T14:51:22Z'
    },
    {
      group: 'cert-manager.io',
      version: 'v1',
      kind: 'Issuer',
      namespace: 'kubeflow',
      name: 'selfsigned-issuer',
      uid: 'ead79091-55c5-49ea-b9ba-f40a9bb36003',
      resourceVersion: '21587',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:41:34Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'knative-serving-namespaced-view',
      uid: '59619075-4df5-449a-b17f-7a5a769fbcf4',
      resourceVersion: '18637',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'knative-serving',
      name: 'config-defaults',
      uid: '53b2c01d-12af-4f6d-b876-ad43294e7067',
      resourceVersion: '18039',
      createdAt: '2020-10-02T14:33:04Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'knative-serving',
      name: 'config-logging',
      uid: 'bf90270f-f717-4f8a-89b6-9d77746c3ce1',
      resourceVersion: '18047',
      createdAt: '2020-10-02T14:33:05Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'ml-pipeline-viewer-controller-deployment',
      uid: '691ea4f7-5bfd-465a-bd44-d93a26f46adf',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'pipelines-viewer',
          uid: '54252c04-3b9c-4666-aacd-2abe5637bb7a'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '642335',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:37Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'ml-pipeline-viewer-controller-deployment-69fccfff8c',
      uid: 'f59bf9a5-c016-4614-af10-f0440e22c180',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'ml-pipeline-viewer-controller-deployment',
          uid: '691ea4f7-5bfd-465a-bd44-d93a26f46adf'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '642332',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:37Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'ml-pipeline-viewer-controller-deployment-69fccfff8c-mgt7m',
      uid: '21b1375b-cfe6-40d3-8d71-8eb3835ea340',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'ml-pipeline-viewer-controller-deployment-69fccfff8c',
          uid: 'f59bf9a5-c016-4614-af10-f0440e22c180'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'ml-pipeline-viewer-crd',
          'app.kubernetes.io/component': 'pipelines-viewer',
          'app.kubernetes.io/instance': 'pipelines-viewer-0.2.5',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'pipelines-viewer',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.5',
          'pod-template-hash': '69fccfff8c'
        }
      },
      resourceVersion: '641947',
      images: [
        'gcr.io/ml-pipeline/viewer-crd-controller:0.2.5'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:34Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'admission-webhook-kubeflow-poddefaults-admin',
      uid: '75490d17-7d3e-422b-aafd-32e44315d838',
      resourceVersion: '1765473',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'RoleBinding',
      namespace: 'kubeflow',
      name: 'ml-pipeline',
      uid: '2c6a201e-c6c3-43cd-8ba3-c1ae4029c6a4',
      resourceVersion: '18899',
      createdAt: '2020-10-02T14:34:21Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'istio-system',
      name: 'cluster-local-gateway-service-account',
      uid: '4db6c7cc-ef1a-419e-819b-a380fb0a4470',
      resourceVersion: '18158',
      createdAt: '2020-10-02T14:33:19Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'istio-system',
      name: 'cluster-local-gateway-service-account-token-7f8gc',
      uid: '98129d5b-74e7-4ec6-9977-09efa43febc1',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'istio-system',
          name: 'cluster-local-gateway-service-account',
          uid: '4db6c7cc-ef1a-419e-819b-a380fb0a4470'
        }
      ],
      resourceVersion: '18157',
      createdAt: '2020-10-02T14:33:19Z'
    },
    {
      group: 'networking.istio.io',
      version: 'v1alpha3',
      kind: 'VirtualService',
      namespace: 'kubeflow',
      name: 'metadata-ui',
      uid: '50b2662e-c41e-49aa-bd0e-0f3ab1f23a9e',
      resourceVersion: '21428',
      createdAt: '2020-10-02T14:41:17Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRoleBinding',
      name: 'seldon-manager-rolebinding-kubeflow',
      uid: 'e2d9fe46-90e1-4ac0-9cc1-24c901f66f3a',
      resourceVersion: '18851',
      createdAt: '2020-10-02T14:34:03Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'kubeflow-tfjobs-view',
      uid: '752486c4-8405-43f9-a50a-b64ee4bdf9df',
      resourceVersion: '18631',
      createdAt: '2020-10-02T14:33:36Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRoleBinding',
      name: 'admission-webhook-cluster-role-binding',
      uid: '2cab9467-02b4-4956-ab96-ad3db33cad05',
      resourceVersion: '18823',
      createdAt: '2020-10-02T14:34:02Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'knative-serving',
      name: 'webhook',
      uid: '6e680928-8d71-40a7-bd3d-1909aec1b8ad',
      networkingInfo: {
        targetLabels: {
          'app.kubernetes.io/component': 'knative-serving-install',
          'app.kubernetes.io/instance': 'knative-serving-install-v0.11.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'knative-serving-install',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v0.11.1',
          'kustomize.component': 'knative',
          role: 'webhook'
        }
      },
      resourceVersion: '20363',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'knative-serving',
      name: 'webhook',
      uid: 'cce06458-baa6-4911-96c2-d2904172e671',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'knative-serving',
          name: 'webhook',
          uid: '6e680928-8d71-40a7-bd3d-1909aec1b8ad'
        }
      ],
      resourceVersion: '635616',
      createdAt: '2020-10-02T14:40:10Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'metadata-grpc-service',
      uid: 'aa4c84d1-af94-4192-b6c4-bf133a18df2e',
      networkingInfo: {
        targetLabels: {
          'app.kubernetes.io/component': 'metadata',
          'app.kubernetes.io/instance': 'metadata-0.2.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'metadata',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.1',
          component: 'grpc-server',
          'kustomize.component': 'metadata'
        }
      },
      resourceVersion: '20469',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:15Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'metadata-grpc-service',
      uid: 'f581d616-87b5-4b7b-96ca-58a15ee59c98',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'metadata-grpc-service',
          uid: 'aa4c84d1-af94-4192-b6c4-bf133a18df2e'
        }
      ],
      resourceVersion: '646649',
      createdAt: '2020-10-02T14:40:15Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'argo-server',
      uid: 'c7d59aa8-7ea2-45bb-a7ff-c112545d38e0',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'argo',
          uid: '3eb053e7-1db8-4cd2-b840-7ca9b8d8712c'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '1766162',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-06T11:16:07Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'argo-server-6d4b4bdfbc',
      uid: '080005aa-37aa-4570-a194-5780028ff60f',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'argo-server',
          uid: 'c7d59aa8-7ea2-45bb-a7ff-c112545d38e0'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '1766159',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-06T11:16:07Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'argo-server-6d4b4bdfbc-pcjp8',
      uid: '09641aad-b583-4565-b536-ae5d2f58e556',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'argo-server-6d4b4bdfbc',
          uid: '080005aa-37aa-4570-a194-5780028ff60f'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'argo-server',
          'app.kubernetes.io/component': 'argo',
          'app.kubernetes.io/instance': 'argo-v2.11.2',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'argo',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': 'v2.11.2',
          'kustomize.component': 'argo',
          'pod-template-hash': '6d4b4bdfbc'
        }
      },
      resourceVersion: '1766157',
      images: [
        'argoproj/argocli:v2.11.2'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-06T11:16:07Z'
    },
    {
      group: 'networking.istio.io',
      version: 'v1beta1',
      kind: 'ServiceEntry',
      namespace: 'kubeflow',
      name: 'google-api-entry',
      uid: 'f971daa8-8e70-496b-8ecd-cb5f5cd1def4',
      resourceVersion: '21302',
      createdAt: '2020-10-02T14:41:01Z'
    },
    {
      version: 'v1',
      kind: 'ServiceAccount',
      namespace: 'kubeflow',
      name: 'ml-pipeline-persistenceagent',
      uid: 'b9baf8c0-418c-4326-a397-07e138ff692d',
      resourceVersion: '18190',
      createdAt: '2020-10-02T14:33:20Z'
    },
    {
      version: 'v1',
      kind: 'Secret',
      namespace: 'kubeflow',
      name: 'ml-pipeline-persistenceagent-token-mvbpv',
      uid: '86aaa084-bda9-4b03-bc23-0496a72d3732',
      parentRefs: [
        {
          kind: 'ServiceAccount',
          namespace: 'kubeflow',
          name: 'ml-pipeline-persistenceagent',
          uid: 'b9baf8c0-418c-4326-a397-07e138ff692d'
        }
      ],
      resourceVersion: '18186',
      createdAt: '2020-10-02T14:33:20Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'ml-pipeline-ml-pipeline-visualizationserver',
      uid: 'd47be868-ce2b-44e3-8990-9570daff04d0',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'pipeline-visualization-service',
          uid: '7933787c-85ea-4ae7-9a4e-f4cdd2619cf5'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '643724',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:39Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'ml-pipeline-ml-pipeline-visualizationserver-675656df79',
      uid: 'e2495a85-dfb1-4333-afaa-6d1fcc1d26be',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'ml-pipeline-ml-pipeline-visualizationserver',
          uid: 'd47be868-ce2b-44e3-8990-9570daff04d0'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '643722',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:40Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'ml-pipeline-ml-pipeline-visualizationserver-675656df79-c8m4s',
      uid: '4568e553-6113-475b-a1b2-7096f1f2a33a',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'ml-pipeline-ml-pipeline-visualizationserver-675656df79',
          uid: 'e2495a85-dfb1-4333-afaa-6d1fcc1d26be'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'ml-pipeline-visualizationserver',
          'app.kubernetes.io/component': 'pipeline-visualization-service',
          'app.kubernetes.io/instance': 'pipeline-visualization-service-0.2.5',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'pipeline-visualization-service',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.5',
          'pod-template-hash': '675656df79'
        }
      },
      resourceVersion: '643721',
      images: [
        'gcr.io/ml-pipeline/visualization-server:0.2.5'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:36:01Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'spark-operator',
      uid: '55a1afcb-7d38-4cb1-9a65-c01434b616e8',
      resourceVersion: '22421',
      createdAt: '2020-10-02T14:41:35Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'argo-server-cluster-role',
      uid: '68c296a9-c167-43c9-ba4b-317e396ca795',
      resourceVersion: '1765426',
      createdAt: '2020-10-02T14:33:37Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRoleBinding',
      name: 'argo-server-binding',
      uid: 'd710d104-1fe3-4999-ae77-a4b20c68a6a4',
      resourceVersion: '1765610',
      createdAt: '2020-10-02T14:34:02Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'istio-reader',
      uid: 'd462707e-5d88-4b33-9df4-51e7d4100c2e',
      resourceVersion: '18737',
      createdAt: '2020-10-02T13:34:58Z'
    },
    {
      version: 'v1',
      kind: 'ConfigMap',
      namespace: 'kubeflow',
      name: 'pipeline-minio-parameters',
      uid: '067e679a-b4ce-4c3c-bfb9-cdbd69ca3b0f',
      resourceVersion: '18059',
      createdAt: '2020-10-02T14:33:06Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRole',
      name: 'notebook-controller-kubeflow-notebooks-edit',
      uid: '9598c538-8b9e-436b-a42d-8389b26645b2',
      resourceVersion: '18741',
      createdAt: '2020-10-02T14:33:38Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRoleBinding',
      name: 'notebook-controller-role-binding',
      uid: '15955682-48f0-4d9c-97c6-480c62dc94cc',
      resourceVersion: '18822',
      createdAt: '2020-10-02T14:34:02Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRoleBinding',
      name: 'pipeline-runner',
      uid: 'fde76aa9-6730-4a6a-bd0b-8623be1e9819',
      resourceVersion: '18835',
      createdAt: '2020-10-02T14:34:12Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'pytorchjobs.kubeflow.org',
      uid: '923ba0a5-4a4f-4af0-9966-5625e42f8aa7',
      resourceVersion: '18352',
      createdAt: '2020-10-02T14:33:30Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'seldon-webhook-service',
      uid: '9bcaa3ef-1f31-4a7f-bf43-d163d31a3dba',
      networkingInfo: {
        targetLabels: {
          app: 'seldon',
          'app.kubernetes.io/component': 'seldon',
          'app.kubernetes.io/instance': 'seldon-1.2.2',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'seldon-core-operator',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '1.2.2',
          'control-plane': 'seldon-controller-manager'
        }
      },
      resourceVersion: '20323',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:05Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'seldon-webhook-service',
      uid: '48510538-207f-4722-87ed-3fc9328aa0e1',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'seldon-webhook-service',
          uid: '9bcaa3ef-1f31-4a7f-bf43-d163d31a3dba'
        }
      ],
      resourceVersion: '4658569',
      createdAt: '2020-10-02T14:40:05Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'StatefulSet',
      namespace: 'kubeflow',
      name: 'metacontroller',
      uid: 'a39affcf-aca0-4d09-801d-14f19859bfed',
      resourceVersion: '635619',
      health: {
        status: 'Healthy',
        message: 'partitioned roll out complete: 1 new pods have been updated...'
      },
      createdAt: '2020-10-02T14:40:48Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ControllerRevision',
      namespace: 'kubeflow',
      name: 'metacontroller-6559b8f5c7',
      uid: '8a56cad3-3ba5-4433-a50a-ccb06ed84bb1',
      parentRefs: [
        {
          group: 'apps',
          kind: 'StatefulSet',
          namespace: 'kubeflow',
          name: 'metacontroller',
          uid: 'a39affcf-aca0-4d09-801d-14f19859bfed'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '21136',
      createdAt: '2020-10-02T14:40:48Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'metacontroller-0',
      uid: '34949e47-3fd9-4aeb-860d-765c4a65e482',
      parentRefs: [
        {
          group: 'apps',
          kind: 'StatefulSet',
          namespace: 'kubeflow',
          name: 'metacontroller',
          uid: 'a39affcf-aca0-4d09-801d-14f19859bfed'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'metacontroller',
          'controller-revision-hash': 'metacontroller-6559b8f5c7',
          'kustomize.component': 'metacontroller',
          'statefulset.kubernetes.io/pod-name': 'metacontroller-0'
        }
      },
      resourceVersion: '635618',
      images: [
        'metacontroller/metacontroller:v0.3.0'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:19:18Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'metadata-deployment',
      uid: '179611a6-0b4f-482e-8eeb-5ebe5038a440',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'metadata',
          uid: '6c38734d-08e3-42ff-9cfc-2ae6e29fb34b'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '641558',
      health: {
        status: 'Progressing',
        message: 'Waiting for rollout to finish: 0 of 1 updated replicas are available...'
      },
      createdAt: '2020-10-02T14:40:43Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'metadata-deployment-5dd4c9d4cf',
      uid: '38ae6485-a556-4d99-b19f-29444ad8f4ed',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'metadata-deployment',
          uid: '179611a6-0b4f-482e-8eeb-5ebe5038a440'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '641550',
      health: {
        status: 'Progressing',
        message: 'Waiting for rollout to finish: 0 out of 1 new replicas are available...'
      },
      createdAt: '2020-10-02T14:40:43Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'metadata-deployment-5dd4c9d4cf-xqsfq',
      uid: '12424e8d-8d12-4fb6-ba57-13cffdd5e77f',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'metadata-deployment-5dd4c9d4cf',
          uid: '38ae6485-a556-4d99-b19f-29444ad8f4ed'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '0/1'
        }
      ],
      networkingInfo: {
        labels: {
          'app.kubernetes.io/component': 'metadata',
          'app.kubernetes.io/instance': 'metadata-0.2.1',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'metadata',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.1',
          component: 'server',
          'kustomize.component': 'metadata',
          'pod-template-hash': '5dd4c9d4cf'
        }
      },
      resourceVersion: '640392',
      images: [
        'gcr.io/kubeflow-images-public/metadata:v0.1.11'
      ],
      health: {
        status: 'Progressing'
      },
      createdAt: '2020-10-03T23:33:37Z'
    },
    {
      version: 'v1',
      kind: 'Service',
      namespace: 'kubeflow',
      name: 'tensorboard',
      uid: '74739a35-37e1-46d8-bb1b-03edb1910277',
      networkingInfo: {
        targetLabels: {
          app: 'tensorboard',
          'kustomize.component': 'tensorboard'
        }
      },
      resourceVersion: '20439',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:14Z'
    },
    {
      version: 'v1',
      kind: 'Endpoints',
      namespace: 'kubeflow',
      name: 'tensorboard',
      uid: 'a85e8cb7-2c45-4996-972f-45acf410ae32',
      parentRefs: [
        {
          kind: 'Service',
          namespace: 'kubeflow',
          name: 'tensorboard',
          uid: '74739a35-37e1-46d8-bb1b-03edb1910277'
        }
      ],
      resourceVersion: '642077',
      createdAt: '2020-10-02T14:40:14Z'
    },
    {
      group: 'apiextensions.k8s.io',
      version: 'v1',
      kind: 'CustomResourceDefinition',
      name: 'poddefaults.kubeflow.org',
      uid: '47a5eab3-5f01-4476-bf60-ba287dfd0623',
      resourceVersion: '18335',
      createdAt: '2020-10-02T14:33:30Z'
    },
    {
      group: 'rbac.authorization.k8s.io',
      version: 'v1',
      kind: 'ClusterRoleBinding',
      name: 'jupyter-web-app-cluster-role-binding',
      uid: '9bc96642-655e-4dd7-9176-625fc0c957ae',
      resourceVersion: '18821',
      createdAt: '2020-10-02T14:34:02Z'
    },
    {
      group: 'app.k8s.io',
      version: 'v1beta1',
      kind: 'Application',
      namespace: 'kubeflow',
      name: 'scheduledworkflow',
      uid: '28024df8-4685-4c72-adc6-aad3515c66d1',
      resourceVersion: '1766261',
      createdAt: '2020-10-02T14:41:27Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'ml-pipeline-scheduledworkflow',
      uid: '3373a6ea-4552-4f96-bc2c-1816b2366226',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'scheduledworkflow',
          uid: '28024df8-4685-4c72-adc6-aad3515c66d1'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '643771',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'ml-pipeline-scheduledworkflow-7b4cb5d959',
      uid: 'a0bc035f-27d8-4fc4-95e7-306f40b5260a',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'ml-pipeline-scheduledworkflow',
          uid: '3373a6ea-4552-4f96-bc2c-1816b2366226'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '643770',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'ml-pipeline-scheduledworkflow-7b4cb5d959-9srv5',
      uid: '13b1af1e-32fa-4eb0-bcca-a65e66590ef7',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'ml-pipeline-scheduledworkflow-7b4cb5d959',
          uid: 'a0bc035f-27d8-4fc4-95e7-306f40b5260a'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'ml-pipeline-scheduledworkflow',
          'app.kubernetes.io/component': 'scheduledworkflow',
          'app.kubernetes.io/instance': 'scheduledworkflow-0.2.5',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'scheduledworkflow',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.5',
          'pod-template-hash': '7b4cb5d959'
        }
      },
      resourceVersion: '643769',
      images: [
        'gcr.io/ml-pipeline/scheduledworkflow:0.2.5'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:36:03Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
      namespace: 'kubeflow',
      name: 'mysql',
      uid: 'a445edca-d650-4d13-9488-1fc577506b65',
      parentRefs: [
        {
          group: 'app.k8s.io',
          kind: 'Application',
          namespace: 'kubeflow',
          name: 'mysql',
          uid: '07959525-0107-4ce0-b2b1-2338e7be2bc6'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645098',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:38Z'
    },
    {
      group: 'apps',
      version: 'v1',
      kind: 'ReplicaSet',
      namespace: 'kubeflow',
      name: 'mysql-7994454666',
      uid: 'aaeb0e8b-d94f-4ebb-ba60-a732c2f65d41',
      parentRefs: [
        {
          group: 'apps',
          kind: 'Deployment',
          namespace: 'kubeflow',
          name: 'mysql',
          uid: 'a445edca-d650-4d13-9488-1fc577506b65'
        }
      ],
      info: [
        {
          name: 'Revision',
          value: 'Rev:1'
        }
      ],
      resourceVersion: '645096',
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-02T14:40:39Z'
    },
    {
      version: 'v1',
      kind: 'Pod',
      namespace: 'kubeflow',
      name: 'mysql-7994454666-x6fdf',
      uid: '431b48a8-4103-48ac-9dbc-cb31dcb8ba77',
      parentRefs: [
        {
          group: 'apps',
          kind: 'ReplicaSet',
          namespace: 'kubeflow',
          name: 'mysql-7994454666',
          uid: 'aaeb0e8b-d94f-4ebb-ba60-a732c2f65d41'
        }
      ],
      info: [
        {
          name: 'Status Reason',
          value: 'Running'
        },
        {
          name: 'Containers',
          value: '1/1'
        }
      ],
      networkingInfo: {
        labels: {
          app: 'mysql',
          'app.kubernetes.io/component': 'mysql',
          'app.kubernetes.io/instance': 'mysql-0.2.5',
          'app.kubernetes.io/managed-by': 'kfctl',
          'app.kubernetes.io/name': 'mysql',
          'app.kubernetes.io/part-of': 'kubeflow',
          'app.kubernetes.io/version': '0.2.5',
          'pod-template-hash': '7994454666'
        }
      },
      resourceVersion: '645095',
      images: [
        'mysql:5.6'
      ],
      health: {
        status: 'Healthy'
      },
      createdAt: '2020-10-03T23:35:41Z'
    }
  ]
}