import * as renderer from 'react-test-renderer';
import * as React from 'react';
import {Application, HealthStatuses, OperationPhases, ResultCodes, SyncStatuses} from '../../shared/models';
import {
    ComparisonStatusIcon,
    getAppOperationState,
    getOperationType,
    HealthStatusIcon,
    OperationPhaseIcon,
    OperationState,
    ResourceResultIcon,
} from './utils';

test('getAppOperationState.DeletionTimestamp', () => {
    const state = getAppOperationState({metadata: {deletionTimestamp: new Date(0)}} as Application);

    expect(state).toStrictEqual({phase: OperationPhases.Running, startedAt: new Date(0)});
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
        status: {operationState: {phase: OperationPhases.Error, startedAt: new Date(0).toISOString()}},
    } as Application);

    expect(state.phase).toBe(OperationPhases.Error);
});

test('getOperationType.Delete', () => {
    const state = getOperationType({metadata: {deletionTimestamp: new Date(0)}} as Application);

    expect(state).toBe('Delete');
});

test('getOperationType.Sync.Operation', () => {
    const state = getOperationType({metadata: {}, operation: {sync: {}}} as Application);

    expect(state).toBe('Sync');
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
    const tree = renderer.create(<OperationState app={{metadata: {}, status: {}} as Application}/>).toJSON();

    expect(tree).toMatchSnapshot();
});

test('OperationState.quiet', () => {
    const tree = renderer.create(<OperationState app={{metadata: {}, status: {operationState: {}}} as Application}
                                                 quiet={true}/>).toJSON();

    expect(tree).toMatchSnapshot();
});

test('OperationState.Default', () => {
    const tree = renderer.create(<OperationState
        app={{metadata: {}, status: {operationState: {}}} as Application}/>).toJSON();

    expect(tree).toMatchSnapshot();
});

test('OperationPhaseIcon.Succeeded', () => {
    const tree = renderer.create(<OperationPhaseIcon
        app={{metadata: {}, status: {operationState: {phase: OperationPhases.Succeeded}}} as Application}/>).toJSON();

    expect(tree).toMatchSnapshot();
});

test('OperationPhaseIcon.Error', () => {
    const tree = renderer.create(<OperationPhaseIcon
        app={{metadata: {}, status: {operationState: {phase: OperationPhases.Error}}} as Application}/>).toJSON();

    expect(tree).toMatchSnapshot();
});

test('OperationPhaseIcon.Failed', () => {
    const tree = renderer.create(<OperationPhaseIcon
        app={{metadata: {}, status: {operationState: {phase: OperationPhases.Failed}}} as Application}/>).toJSON();

    expect(tree).toMatchSnapshot();
});

test('OperationPhaseIcon.Running', () => {
    const tree = renderer.create(<OperationPhaseIcon
        app={{metadata: {}, status: {operationState: {phase: OperationPhases.Running}}} as Application}/>).toJSON();

    expect(tree).toMatchSnapshot();
});

test('ComparisonStatusIcon.Synced', () => {
    const tree = renderer.create(<ComparisonStatusIcon status={SyncStatuses.Synced}/>).toJSON();

    expect(tree).toMatchSnapshot();
});

test('ComparisonStatusIcon.OutOfSync', () => {
    const tree = renderer.create(<ComparisonStatusIcon status={SyncStatuses.OutOfSync}/>).toJSON();

    expect(tree).toMatchSnapshot();
});

test('ComparisonStatusIcon.Unknown', () => {
    const tree = renderer.create(<ComparisonStatusIcon status={SyncStatuses.Unknown}/>).toJSON();

    expect(tree).toMatchSnapshot();
});

test('HealthStatusIcon.Healthy', () => {
    const tree = renderer.create(<HealthStatusIcon state={HealthStatuses.Healthy}/>).toJSON();

    expect(tree).toMatchSnapshot();
});

test('HealthStatusIcon.Suspended', () => {
    const tree = renderer.create(<HealthStatusIcon state={HealthStatuses.Suspended}/>).toJSON();

    expect(tree).toMatchSnapshot();
});

test('ResourceResultIcon.Synced', () => {
    const tree = renderer.create(<ResourceResultIcon
        resource={{status: ResultCodes.Synced, message: 'my-message'}}/>).toJSON();

    expect(tree).toMatchSnapshot();
});

test('ResourceResultIcon.Pruned', () => {
    const tree = renderer.create(<ResourceResultIcon resource={{status: ResultCodes.Pruned}}/>).toJSON();

    expect(tree).toMatchSnapshot();
});

test('ResourceResultIcon.SyncFailed', () => {
    const tree = renderer.create(<ResourceResultIcon resource={{status: ResultCodes.SyncFailed}}/>).toJSON();

    expect(tree).toMatchSnapshot();
});

test('ResourceResultIcon.Hook.Running', () => {
    const tree = renderer.create(<ResourceResultIcon
        resource={{hookType: 'Sync', hookPhase: OperationPhases.Running, message: 'my-message'}}/>).toJSON();

    expect(tree).toMatchSnapshot();
});

test('ResourceResultIcon.Hook.Failed', () => {
    const tree = renderer.create(<ResourceResultIcon
        resource={{hookType: 'Sync', hookPhase: OperationPhases.Failed}}/>).toJSON();

    expect(tree).toMatchSnapshot();
});

test('ResourceResultIcon.Hook.Error', () => {
    const tree = renderer.create(<ResourceResultIcon
        resource={{hookType: 'Sync', hookPhase: OperationPhases.Error}}/>).toJSON();

    expect(tree).toMatchSnapshot();
});

test('ResourceResultIcon.Hook.Succeeded', () => {
    const tree = renderer.create(<ResourceResultIcon
        resource={{hookType: 'Sync', hookPhase: OperationPhases.Succeeded}}/>).toJSON();

    expect(tree).toMatchSnapshot();
});


test('ResourceResultIcon.Hook.Terminating', () => {
    const tree = renderer.create(<ResourceResultIcon
        resource={{hookType: 'Sync', hookPhase: OperationPhases.Terminating}}/>).toJSON();

    expect(tree).toMatchSnapshot();
});
