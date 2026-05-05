import * as React from 'react';
import {renderToStaticMarkup} from 'react-dom/server.node';
import {
    Application,
    HealthStatus,
    HealthStatuses,
    OperationPhases,
    ResourceResult,
    ResultCodes,
    State,
    SyncStatuses
} from '../../shared/models';
import * as jsYaml from 'js-yaml';
import {
    appRBACName,
    ComparisonStatusIcon,
    getAppOperationState,
    getOperationType,
    getPodStateReason,
    HealthStatusIcon,
    OperationState,
    ResourceResultIcon
} from './utils';

const zero = new Date(0).toISOString();

function renderMarkup(element: React.ReactElement) {
  return renderToStaticMarkup(element);
}

test('getAppOperationState.DeletionTimestamp', () => {
    const state = getAppOperationState({metadata: {deletionTimestamp: zero}} as Application);

    expect(state).toStrictEqual({phase: OperationPhases.Running, startedAt: zero});
});

test('getAppOperationState.Operation', () => {
    const state = getAppOperationState({metadata: {}, operation: {}} as Application);

    expect(state.phase).toBe(OperationPhases.Running);
    expect(state.startedAt).toBeDefined();
    expect(state.operation).toStrictEqual({sync: {}});
});

test('getAppOperationState.Status', () => {
    const state = getAppOperationState({
        metadata: {},
        status: {operationState: {phase: OperationPhases.Error, startedAt: zero}},
    } as Application);

    expect(state.phase).toBe(OperationPhases.Error);
});

test('getOperationType.Delete', () => {
    const state = getOperationType({metadata: {deletionTimestamp: zero.toString()}} as Application);

    expect(state).toBe('Delete');
});

test('getOperationType.Sync.Operation', () => {
    const state = getOperationType({metadata: {}, operation: {sync: {}}} as Application);

    expect(state).toBe('Sync');
});

test('getOperationType.DeleteAndRecentSync', () => {
    const state = getOperationType({metadata: {deletionTimestamp: '123'}, status: {operationState: {operation: {sync: {}}}}} as Application);

    expect(state).toBe('Delete');
});

test('getOperationType.Sync.Status', () => {
    const state = getOperationType({metadata: {}, status: {operationState: {operation: {sync: {}}}}} as Application);

    expect(state).toBe('Sync');
});

test('getOperationType.Unknown', () => {
    const state = getOperationType({metadata: {}, status: {}} as Application);

    expect(state).toBe('Unknown');
});

test('OperationState.undefined', () => {
  const tree = renderMarkup(<OperationState app={{metadata: {}, status: {}} as Application} />);

    expect(tree).toMatchSnapshot();
});

test('OperationState.quiet', () => {
  const tree = renderMarkup(<OperationState app={{metadata: {}, status: {operationState: {}}} as Application} quiet={true} />);

    expect(tree).toMatchSnapshot();
});

test('OperationState.Unknown', () => {
  const tree = renderMarkup(<OperationState app={{metadata: {}, status: {operationState: {}}} as Application} />);

    expect(tree).toMatchSnapshot();
});

test('OperationState.Deleting', () => {
  const tree = renderMarkup(<OperationState app={{metadata: {deletionTimestamp: zero}, status: {operationState: {}}} as Application} />);

    expect(tree).toMatchSnapshot();
});

test('OperationState.Sync OK', () => {
  const tree = renderMarkup(<OperationState app={{metadata: {}, status: {operationState: {operation: {sync: {}}, phase: OperationPhases.Succeeded}}} as Application} />);

    expect(tree).toMatchSnapshot();
});

test('OperationState.Sync error', () => {
  const tree = renderMarkup(<OperationState app={{metadata: {}, status: {operationState: {operation: {sync: {}}, phase: OperationPhases.Error}}} as Application} />);

    expect(tree).toMatchSnapshot();
});

test('OperationState.Sync failed', () => {
  const tree = renderMarkup(<OperationState app={{metadata: {}, status: {operationState: {operation: {sync: {}}, phase: OperationPhases.Failed}}} as Application} />);

    expect(tree).toMatchSnapshot();
});

test('OperationState.Syncing', () => {
  const tree = renderMarkup(<OperationState app={{metadata: {}, status: {operationState: {operation: {sync: {}}, phase: OperationPhases.Running}}} as Application} />);

    expect(tree).toMatchSnapshot();
});

test('ComparisonStatusIcon.Synced', () => {
  const tree = renderMarkup(<ComparisonStatusIcon status={SyncStatuses.Synced} />);

    expect(tree).toMatchSnapshot();
});

test('ComparisonStatusIcon.OutOfSync', () => {
  const tree = renderMarkup(<ComparisonStatusIcon status={SyncStatuses.OutOfSync} />);

    expect(tree).toMatchSnapshot();
});

test('ComparisonStatusIcon.Unknown', () => {
  const tree = renderMarkup(<ComparisonStatusIcon status={SyncStatuses.Unknown} />);

    expect(tree).toMatchSnapshot();
});

test('HealthStatusIcon.Unknown', () => {
  const tree = renderMarkup(<HealthStatusIcon state={{status: HealthStatuses.Unknown} as HealthStatus} />);

    expect(tree).toMatchSnapshot();
});
test('HealthStatusIcon.Progressing', () => {
  const tree = renderMarkup(<HealthStatusIcon state={{status: HealthStatuses.Progressing} as HealthStatus} />);

    expect(tree).toMatchSnapshot();
});

test('HealthStatusIcon.Suspended', () => {
  const tree = renderMarkup(<HealthStatusIcon state={{status: HealthStatuses.Suspended} as HealthStatus} />);

    expect(tree).toMatchSnapshot();
});

test('HealthStatusIcon.Healthy', () => {
  const tree = renderMarkup(<HealthStatusIcon state={{status: HealthStatuses.Healthy} as HealthStatus} />);

    expect(tree).toMatchSnapshot();
});

test('HealthStatusIcon.Degraded', () => {
  const tree = renderMarkup(<HealthStatusIcon state={{status: HealthStatuses.Degraded} as HealthStatus} />);

    expect(tree).toMatchSnapshot();
});
test('HealthStatusIcon.Missing', () => {
  const tree = renderMarkup(<HealthStatusIcon state={{status: HealthStatuses.Missing} as HealthStatus} />);

    expect(tree).toMatchSnapshot();
});

test('ResourceResultIcon.Synced', () => {
  const tree = renderMarkup(<ResourceResultIcon resource={{status: ResultCodes.Synced, message: 'my-message'} as ResourceResult} />);

    expect(tree).toMatchSnapshot();
});

test('ResourceResultIcon.Pruned', () => {
  const tree = renderMarkup(<ResourceResultIcon resource={{status: ResultCodes.Pruned} as ResourceResult} />);

    expect(tree).toMatchSnapshot();
});

test('ResourceResultIcon.SyncFailed', () => {
  const tree = renderMarkup(<ResourceResultIcon resource={{status: ResultCodes.SyncFailed} as ResourceResult} />);

    expect(tree).toMatchSnapshot();
});

test('ResourceResultIcon.Hook.Running', () => {
  const tree = renderMarkup(
    <ResourceResultIcon
      resource={
        {
          hookType: 'Sync',
          hookPhase: OperationPhases.Running,
          message: 'my-message',
        } as ResourceResult
      }
    />,
  );

    expect(tree).toMatchSnapshot();
});

test('ResourceResultIcon.Hook.Failed', () => {
  const tree = renderMarkup(<ResourceResultIcon resource={{hookType: 'Sync', hookPhase: OperationPhases.Failed} as ResourceResult} />);

    expect(tree).toMatchSnapshot();
});

test('ResourceResultIcon.Hook.Error', () => {
  const tree = renderMarkup(<ResourceResultIcon resource={{hookType: 'Sync', hookPhase: OperationPhases.Error} as ResourceResult} />);

    expect(tree).toMatchSnapshot();
});

test('ResourceResultIcon.Hook.Succeeded', () => {
  const tree = renderMarkup(<ResourceResultIcon resource={{hookType: 'Sync', hookPhase: OperationPhases.Succeeded} as ResourceResult} />);

    expect(tree).toMatchSnapshot();
});

test('ResourceResultIcon.Hook.Terminating', () => {
  const tree = renderMarkup(<ResourceResultIcon resource={{hookType: 'Sync', hookPhase: OperationPhases.Terminating} as ResourceResult} />);

    expect(tree).toMatchSnapshot();
});

// These tests are equivalent to those in controller/cache/info_test.go. If you change a test here, update the corresponding test there.
describe('getPodStateReason', () => {
    it('TestGetPodInfo', () => {
      const podYaml = `
  apiVersion: v1
  kind: Pod
  metadata:
    name: helm-guestbook-pod
    namespace: default
    ownerReferences:
    - apiVersion: extensions/v1beta1
      kind: ReplicaSet
      name: helm-guestbook-rs
    resourceVersion: "123"
    labels:
      app: guestbook
  spec:
    nodeName: minikube
    containers:
    - image: bar
      resources:
        requests:
          memory: 128Mi
      `

        const pod = jsYaml.load(podYaml);

        const {reason} = getPodStateReason(pod as State);

        expect(reason).toBe('Unknown');
    });

    it('TestGetPodWithInitialContainerInfo', () => {
        const podYaml = `
  apiVersion: "v1"
  kind: "Pod"
  metadata: 
    labels: 
      app: "app-with-initial-container"
    name: "app-with-initial-container-5f46976fdb-vd6rv"
    namespace: "default"
    ownerReferences: 
    - apiVersion: "apps/v1"
      kind: "ReplicaSet"
      name: "app-with-initial-container-5f46976fdb"
  spec: 
    containers: 
    - image: "alpine:latest"
      imagePullPolicy: "Always"
      name: "app-with-initial-container"
    initContainers: 
    - image: "alpine:latest"
      imagePullPolicy: "Always"
      name: "app-with-initial-container-logshipper"
    nodeName: "minikube"
  status: 
    containerStatuses: 
    - image: "alpine:latest"
      name: "app-with-initial-container"
      ready: true
      restartCount: 0
      started: true
      state: 
        running: 
          startedAt: "2024-10-08T08:44:25Z"
    initContainerStatuses: 
    - image: "alpine:latest"
      name: "app-with-initial-container-logshipper"
      ready: true
      restartCount: 0
      started: false
      state: 
        terminated: 
          exitCode: 0
          reason: "Completed"
    phase: "Running"
`;
        const pod = jsYaml.load(podYaml);

        const {reason} = getPodStateReason(pod as State);
            expect(reason).toBe('Running');
    });

    it('TestGetPodWithInitialContainerInfoWithResources', () => {
      const podYaml = `
        apiVersion: "v1"
        kind: "Pod"
        metadata:
            labels:
                app: "app-with-initial-container"
            name: "app-with-initial-container-5f46976fdb-vd6rv"
            namespace: "default"
            ownerReferences:
            - apiVersion: "apps/v1"
              kind: "ReplicaSet"
              name: "app-with-initial-container-5f46976fdb"
        spec:
            containers:
            - image: "alpine:latest"
              imagePullPolicy: "Always"
              name: "app-with-initial-container"
              resources:
                requests:
                  cpu: "100m"
                  memory: "128Mi"
                limits:
                  cpu: "500m"
                  memory: "512Mi"
            initContainers:
            - image: "alpine:latest"
              imagePullPolicy: "Always"
              name: "app-with-initial-container-logshipper"
              resources:
                requests:
                  cpu: "50m"
                  memory: "64Mi"
                limits:
                  cpu: "250m"
                  memory: "256Mi"
            nodeName: "minikube"
        status:
            containerStatuses:
            - image: "alpine:latest"
              name: "app-with-initial-container"
              ready: true
              restartCount: 0
              started: true
              state:
                running:
                  startedAt: "2024-10-08T08:44:25Z"
            initContainerStatuses:
            - image: "alpine:latest"
              name: "app-with-initial-container-logshipper"
              ready: true
              restartCount: 0
              started: false
              state:
                terminated:
                  exitCode: 0
                  reason: "Completed"
            phase: "Running"
`;

        const pod = jsYaml.load(podYaml);

        const {reason} = getPodStateReason(pod as State);
            expect(reason).toBe('Running');
    });

    it('TestGetPodInfoWithSidecar', () => {
        const podYaml = `
  apiVersion: v1
  kind: Pod
  metadata:
    labels:
      app: app-with-sidecar
    name: app-with-sidecar-6664cc788c-lqlrp
    namespace: default
    ownerReferences:
      - apiVersion: apps/v1
        kind: ReplicaSet
        name: app-with-sidecar-6664cc788c
  spec:
    containers:
    - image: 'docker.m.daocloud.io/library/alpine:latest'
      imagePullPolicy: Always
      name: app-with-sidecar
    initContainers:
    - image: 'docker.m.daocloud.io/library/alpine:latest'
      imagePullPolicy: Always
      name: logshipper
      restartPolicy: Always
    nodeName: minikube
  status:
    containerStatuses:
    - image: 'docker.m.daocloud.io/library/alpine:latest'
      name: app-with-sidecar
      ready: true
      restartCount: 0
      started: true
      state:
        running:
          startedAt: '2024-10-08T08:39:43Z'
    initContainerStatuses:
    - image: 'docker.m.daocloud.io/library/alpine:latest'
      name: logshipper
      ready: true
      restartCount: 0
      started: true
      state:
        running:
          startedAt: '2024-10-08T08:39:40Z'
    phase: Running
`;
        const pod = jsYaml.load(podYaml);

        const {reason} = getPodStateReason(pod as State);
            expect(reason).toBe('Running');
    });

    it('TestGetPodInfoWithInitialContainer', () => {
        const podYaml = `
  apiVersion: v1
  kind: Pod
  metadata:
    generateName: myapp-long-exist-56b7d8794d-
    labels:
      app: myapp-long-exist
    name: myapp-long-exist-56b7d8794d-pbgrd
    namespace: linghao
    ownerReferences:
      - apiVersion: apps/v1
        kind: ReplicaSet
        name: myapp-long-exist-56b7d8794d
  spec:
    containers:
      - image: alpine:latest
        imagePullPolicy: Always
        name: myapp-long-exist
    initContainers:
      - image: alpine:latest
        imagePullPolicy: Always
        name: myapp-long-exist-logshipper
    nodeName: minikube
  status:
    containerStatuses:
      - image: alpine:latest
        name: myapp-long-exist
        ready: false
        restartCount: 0
        started: false
        state:
          waiting:
            reason: PodInitializing
    initContainerStatuses:
      - image: alpine:latest
        name: myapp-long-exist-logshipper
        ready: false
        restartCount: 0
        started: true
        state:
          running:
            startedAt: '2024-10-09T08:03:45Z'
    phase: Pending
    startTime: '2024-10-09T08:02:39Z'
`;
        const pod = jsYaml.load(podYaml);

        const {reason} = getPodStateReason(pod as State);
            expect(reason).toBe('Init:0/1');
    });

    it('TestGetPodInfoWithRestartableInitContainer', () => {
        const podYaml = `
  apiVersion: v1
  kind: Pod
  metadata:
    name: test1
  spec:
    initContainers:
      - name: restartable-init-1
        restartPolicy: Always
      - name: restartable-init-2
        restartPolicy: Always
    containers:
      - name: container
    nodeName: minikube
  status:
    phase: Pending
    initContainerStatuses:
      - name: restartable-init-1
        ready: false
        restartCount: 3
        state:
          running: {}
        started: false
        lastTerminationState:
          terminated:
            finishedAt: "2023-10-01T00:00:00Z" # Replace with actual time
      - name: restartable-init-2
        ready: false
        state:
          waiting: {}
        started: false
    containerStatuses:
      - ready: false
        restartCount: 0
        state:
          waiting: {}
    conditions:
      - type: PodInitialized
        status: "False"
`;
        const pod = jsYaml.load(podYaml);

        const {reason} = getPodStateReason(pod as State);
            expect(reason).toBe('Init:0/2');
    });

    it('TestGetPodInfoWithPartiallyStartedInitContainers', () => {
        const podYaml = `
  apiVersion: v1
  kind: Pod
  metadata:
    name: test1
  spec:
    initContainers:
      - name: restartable-init-1
        restartPolicy: Always
      - name: restartable-init-2
        restartPolicy: Always
    containers:
      - name: container
    nodeName: minikube
  status:
    phase: Pending
    initContainerStatuses:
      - name: restartable-init-1
        ready: false
        restartCount: 3
        state:
          running: {}
        started: true
        lastTerminationState:
          terminated:
            finishedAt: "2023-10-01T00:00:00Z" # Replace with actual time
      - name: restartable-init-2
        ready: false
        state:
          running: {}
        started: false
    containerStatuses:
      - ready: false
        restartCount: 0
        state:
          waiting: {}
    conditions:
      - type: PodInitialized
        status: "False"
`;
        const pod = jsYaml.load(podYaml);

        const {reason} = getPodStateReason(pod as State);
        expect(reason).toBe('Init:1/2');
    });

    it('TestGetPodInfoWithStartedInitContainers', () => {
        const podYaml = `
  apiVersion: v1
  kind: Pod
  metadata:
    name: test2
  spec:
    initContainers:
      - name: restartable-init-1
        restartPolicy: Always
      - name: restartable-init-2
        restartPolicy: Always
    containers:
      - name: container
    nodeName: minikube
  status:
    phase: Running
    initContainerStatuses:
      - name: restartable-init-1
        ready: false
        restartCount: 3
        state:
          running: {}
        started: true
        lastTerminationState:
          terminated:
            finishedAt: "2023-10-01T00:00:00Z" # Replace with actual time
      - name: restartable-init-2
        ready: false
        state:
          running: {}
        started: true
    containerStatuses:
      - ready: true
        restartCount: 4
        state:
          running: {}
        lastTerminationState:
          terminated:
            finishedAt: "2023-10-01T00:00:00Z" # Replace with actual time
    conditions:
      - type: PodInitialized
        status: "True"
`;
        const pod = jsYaml.load(podYaml);

        const {reason} = getPodStateReason(pod as State);
            expect(reason).toBe('Running');
    });

    it('TestGetPodInfoWithNormalInitContainer', () => {
        const podYaml = `
  apiVersion: v1
  kind: Pod
  metadata:
    name: test7
  spec:
    initContainers:
      - name: init-container
    containers:
      - name: main-container
    nodeName: minikube
  status:
    phase: podPhase
    initContainerStatuses:
      - ready: false
        restartCount: 3
        state:
          running: {}
        lastTerminationState:
          terminated:
            finishedAt: "2023-10-01T00:00:00Z" # Replace with the actual time
    containerStatuses:
      - ready: false
        restartCount: 0
        state:
          waiting: {}
`;
        const pod = jsYaml.load(podYaml);

        const {reason} = getPodStateReason(pod as State);
            expect(reason).toBe('Init:0/1');
    });

    it('TestPodConditionSucceeded', () => {
        const podYaml = `
apiVersion: v1
kind: Pod
metadata:
  name: test8
spec:
  nodeName: minikube
  containers:
    - name: container
status:
  phase: Succeeded
  containerStatuses:
    - ready: false
      restartCount: 0
      state:
        terminated:
          reason: Completed
          exitCode: 0
`;
        const pod = jsYaml.load(podYaml);

        const {reason} = getPodStateReason(pod as State);

            expect(reason).toBe('Completed');
    });

    it('TestPodConditionSucceededWithResources', () => {
        const podYaml = `
  apiVersion: v1
  kind: Pod
  metadata:
    name: test8
  spec:
    nodeName: minikube
    containers:
      - name: container
        resources:
          requests:
            cpu: "50m"
            memory: "64Mi"
          limits:
            cpu: "250m"
            memory: "256Mi"
  status:
    phase: Succeeded
    containerStatuses:
      - ready: false
        restartCount: 0
        state:
          terminated:
            reason: Completed
            exitCode: 0
`;
        const pod = jsYaml.load(podYaml);

        const {reason} = getPodStateReason(pod as State);

            expect(reason).toBe('Completed');
    });

    it('TestPodConditionFailed', () => {
        const podYaml = `
  apiVersion: v1
  kind: Pod
  metadata:
    name: test9
  spec:
    nodeName: minikube
    containers:
      - name: container
  status:
    phase: Failed
    containerStatuses:
      - ready: false
        restartCount: 0
        state:
          terminated:
            reason: Error
            exitCode: 1
`;
        const pod = jsYaml.load(podYaml);

        const {reason} = getPodStateReason(pod as State);

            expect(reason).toBe('Error');
    });

    it('TestPodConditionFailedWithResources', () => {
      const podYaml = `
apiVersion: v1
kind: Pod
metadata:
  name: test9
spec:
  nodeName: minikube
  containers:
    - name: container
      resources:
        requests:
          cpu: "50m"
          memory: "64Mi"
        limits:
          cpu: "250m"
          memory: "256Mi"
status:
  phase: Failed
  containerStatuses:
    - ready: false
      restartCount: 0
      state:
        terminated:
          reason: Error
          exitCode: 1
`;
      const pod = jsYaml.load(podYaml);

      const {reason} = getPodStateReason(pod as State);

          expect(reason).toBe('Error');
  });

    it('TestPodConditionSucceededWithDeletion', () => {
        const podYaml = `
  apiVersion: v1
  kind: Pod
  metadata:
    name: test10
    deletionTimestamp: "2023-10-01T00:00:00Z"
  spec:
    nodeName: minikube
    containers:
      - name: container
  status:
    phase: Succeeded
    containerStatuses:
      - ready: false
        restartCount: 0
        state:
          terminated:
            reason: Completed
            exitCode: 0
`;
        const pod = jsYaml.load(podYaml);

        const {reason} = getPodStateReason(pod as State);

            expect(reason).toBe('Completed');
    });

    it('TestPodConditionRunningWithDeletion', () => {
        const podYaml = `
  apiVersion: v1
  kind: Pod
  metadata:
    name: test11
    deletionTimestamp: "2023-10-01T00:00:00Z"
  spec:
    nodeName: minikube
    containers:
      - name: container
  status:
    phase: Running
    containerStatuses:
      - ready: false
        restartCount: 0
        state:
          running: {}
`;
        const pod = jsYaml.load(podYaml);

        const {reason} = getPodStateReason(pod as State);

            expect(reason).toBe('Terminating');
    });

    it('TestPodConditionPendingWithDeletion', () => {
        const podYaml = `
  apiVersion: v1
  kind: Pod
  metadata:
    name: test12
    deletionTimestamp: "2023-10-01T00:00:00Z"
  spec:
    nodeName: minikube
    containers:
      - name: container
  status:
    phase: Pending
        `
        const pod = jsYaml.load(podYaml);

        const {reason} = getPodStateReason(pod as State);

            expect(reason).toBe('Terminating');
    });

    it('TestPodScheduledWithSchedulingGated', () => {
        const podYaml = `
  apiVersion: v1
  kind: Pod
  metadata:
    name: test13
  spec:
    nodeName: minikube
    containers:
      - name: container1
      - name: container2
  status:
    phase: podPhase
    conditions:
      - type: PodScheduled
        status: "False"
        reason: SchedulingGated
          `

        const pod = jsYaml.load(podYaml);

        const {reason} = getPodStateReason(pod as State);

        expect(reason).toBe('SchedulingGated');
    });
});

describe('appRBACName', () => {
    it('returns project/namespace/name when namespace is defined', () => {
        const app = {
            metadata: {
                name: 'my-app',
                namespace: 'my-namespace'
            },
            spec: {
                project: 'my-project'
            }
        } as Application;

        const result = appRBACName(app);

        expect(result).toBe('my-project/my-namespace/my-app');
    });

    it('returns project/name when namespace is undefined', () => {
        const app = {
            metadata: {
                name: 'my-app'
            },
            spec: {
                project: 'my-project'
            }
        } as Application;

        const result = appRBACName(app);

        expect(result).toBe('my-project/my-app');
    });

    it('handles empty namespace string as undefined', () => {
        const app = {
            metadata: {
                name: 'test-app',
                namespace: ''
            },
            spec: {
                project: 'test-project'
            }
        } as Application;

        // Note: The function uses a falsy check on namespace, so empty string is treated the same as undefined
        const result = appRBACName(app);

        expect(result).toBe('test-project/test-app');
    });
});