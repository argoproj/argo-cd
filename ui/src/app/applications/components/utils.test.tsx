import * as React from 'react';
import * as renderer from 'react-test-renderer';
import {Application, HealthStatus, HealthStatuses, OperationPhases, ResourceResult, ResultCodes, SyncStatuses} from '../../shared/models';
import {ComparisonStatusIcon, getAppOperationState, getExternalUrls, getOperationType, HealthStatusIcon, OperationState, ResourceResultIcon} from './utils';

const zero = new Date(0).toISOString();

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
        status: {operationState: {phase: OperationPhases.Error, startedAt: zero}}
    } as Application);

    expect(state.phase).toBe(OperationPhases.Error);
});

test('getExternalUrls One URL from annotation, Empty External URL array', () => {
    const links = getExternalUrls(
        {
            'link.argocd.argoproj.io/external-link' : 'https://github.com/argoproj/argo-cd'
        }, []
    );
    expect(links.length).toBe(1);
    expect(links[0]).toBe('https://github.com/argoproj/argo-cd');
});

test('getExternalUrls One URL from annotation, null URL array', () => {
    const links = getExternalUrls(
        {
            'link.argocd.argoproj.io/external-link' : 'https://github.com/argoproj/argo-cd'
        }, null
    );
    expect(links.length).toBe(1);
    expect(links[0]).toBe('https://github.com/argoproj/argo-cd');
});

test('getExternalUrls One URL from annotation, One External URL array', () => {
    const links = getExternalUrls(
        {
            'link.argocd.argoproj.io/external-link' : 'https://github.com/argoproj/argo-cd'
        }, ['http://ingress-url:1234']
    );

    expect(links.length).toBe(2);
    expect(links[0]).toBe('http://ingress-url:1234');
    expect(links[1]).toBe('https://github.com/argoproj/argo-cd');
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
    const tree = renderer.create(<OperationState app={{metadata: {}, status: {}} as Application} />).toJSON();

    expect(tree).toMatchSnapshot();
});

test('OperationState.quiet', () => {
    const tree = renderer.create(<OperationState app={{metadata: {}, status: {operationState: {}}} as Application} quiet={true} />).toJSON();

    expect(tree).toMatchSnapshot();
});

test('OperationState.Unknown', () => {
    const tree = renderer.create(<OperationState app={{metadata: {}, status: {operationState: {}}} as Application} />).toJSON();

    expect(tree).toMatchSnapshot();
});

test('OperationState.Deleting', () => {
    const tree = renderer.create(<OperationState app={{metadata: {deletionTimestamp: zero}, status: {operationState: {}}} as Application} />).toJSON();

    expect(tree).toMatchSnapshot();
});

test('OperationState.Sync OK', () => {
    const tree = renderer
        .create(<OperationState app={{metadata: {}, status: {operationState: {operation: {sync: {}}, phase: OperationPhases.Succeeded}}} as Application} />)
        .toJSON();

    expect(tree).toMatchSnapshot();
});

test('OperationState.Sync error', () => {
    const tree = renderer.create(<OperationState app={{metadata: {}, status: {operationState: {operation: {sync: {}}, phase: OperationPhases.Error}}} as Application} />).toJSON();

    expect(tree).toMatchSnapshot();
});

test('OperationState.Sync failed', () => {
    const tree = renderer.create(<OperationState app={{metadata: {}, status: {operationState: {operation: {sync: {}}, phase: OperationPhases.Failed}}} as Application} />).toJSON();

    expect(tree).toMatchSnapshot();
});

test('OperationState.Syncing', () => {
    const tree = renderer
        .create(<OperationState app={{metadata: {}, status: {operationState: {operation: {sync: {}}, phase: OperationPhases.Running}}} as Application} />)
        .toJSON();

    expect(tree).toMatchSnapshot();
});

test('ComparisonStatusIcon.Synced', () => {
    const tree = renderer.create(<ComparisonStatusIcon status={SyncStatuses.Synced} />).toJSON();

    expect(tree).toMatchSnapshot();
});

test('ComparisonStatusIcon.OutOfSync', () => {
    const tree = renderer.create(<ComparisonStatusIcon status={SyncStatuses.OutOfSync} />).toJSON();

    expect(tree).toMatchSnapshot();
});

test('ComparisonStatusIcon.Unknown', () => {
    const tree = renderer.create(<ComparisonStatusIcon status={SyncStatuses.Unknown} />).toJSON();

    expect(tree).toMatchSnapshot();
});

test('HealthStatusIcon.Unknown', () => {
    const tree = renderer.create(<HealthStatusIcon state={{status: HealthStatuses.Unknown} as HealthStatus} />).toJSON();

    expect(tree).toMatchSnapshot();
});
test('HealthStatusIcon.Progressing', () => {
    const tree = renderer.create(<HealthStatusIcon state={{status: HealthStatuses.Progressing} as HealthStatus} />).toJSON();

    expect(tree).toMatchSnapshot();
});

test('HealthStatusIcon.Suspended', () => {
    const tree = renderer.create(<HealthStatusIcon state={{status: HealthStatuses.Suspended} as HealthStatus} />).toJSON();

    expect(tree).toMatchSnapshot();
});

test('HealthStatusIcon.Healthy', () => {
    const tree = renderer.create(<HealthStatusIcon state={{status: HealthStatuses.Healthy} as HealthStatus} />).toJSON();

    expect(tree).toMatchSnapshot();
});

test('HealthStatusIcon.Degraded', () => {
    const tree = renderer.create(<HealthStatusIcon state={{status: HealthStatuses.Degraded} as HealthStatus} />).toJSON();

    expect(tree).toMatchSnapshot();
});
test('HealthStatusIcon.Missing', () => {
    const tree = renderer.create(<HealthStatusIcon state={{status: HealthStatuses.Missing} as HealthStatus} />).toJSON();

    expect(tree).toMatchSnapshot();
});

test('ResourceResultIcon.Synced', () => {
    const tree = renderer.create(<ResourceResultIcon resource={{status: ResultCodes.Synced, message: 'my-message'} as ResourceResult} />).toJSON();

    expect(tree).toMatchSnapshot();
});

test('ResourceResultIcon.Pruned', () => {
    const tree = renderer.create(<ResourceResultIcon resource={{status: ResultCodes.Pruned} as ResourceResult} />).toJSON();

    expect(tree).toMatchSnapshot();
});

test('ResourceResultIcon.SyncFailed', () => {
    const tree = renderer.create(<ResourceResultIcon resource={{status: ResultCodes.SyncFailed} as ResourceResult} />).toJSON();

    expect(tree).toMatchSnapshot();
});

test('ResourceResultIcon.Hook.Running', () => {
    const tree = renderer
        .create(
            <ResourceResultIcon
                resource={
                    {
                        hookType: 'Sync',
                        hookPhase: OperationPhases.Running,
                        message: 'my-message'
                    } as ResourceResult
                }
            />
        )
        .toJSON();

    expect(tree).toMatchSnapshot();
});

test('ResourceResultIcon.Hook.Failed', () => {
    const tree = renderer.create(<ResourceResultIcon resource={{hookType: 'Sync', hookPhase: OperationPhases.Failed} as ResourceResult} />).toJSON();

    expect(tree).toMatchSnapshot();
});

test('ResourceResultIcon.Hook.Error', () => {
    const tree = renderer.create(<ResourceResultIcon resource={{hookType: 'Sync', hookPhase: OperationPhases.Error} as ResourceResult} />).toJSON();

    expect(tree).toMatchSnapshot();
});

test('ResourceResultIcon.Hook.Succeeded', () => {
    const tree = renderer.create(<ResourceResultIcon resource={{hookType: 'Sync', hookPhase: OperationPhases.Succeeded} as ResourceResult} />).toJSON();

    expect(tree).toMatchSnapshot();
});

test('ResourceResultIcon.Hook.Terminating', () => {
    const tree = renderer.create(<ResourceResultIcon resource={{hookType: 'Sync', hookPhase: OperationPhases.Terminating} as ResourceResult} />).toJSON();

    expect(tree).toMatchSnapshot();
});
