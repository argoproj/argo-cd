// import * as React from 'react';
import {Application, OperationPhases} from '../../shared/models';
import {getAppOperationState} from './utils';

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
