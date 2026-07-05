import { Application } from '../../../shared/models';

function showTerminateButton(app: Application): boolean {
  const operationState = app.status && app.status.operationState;
  return !!(operationState && !(operationState.finishedAt && operationState.phase !== 'Running') && operationState.phase !== 'Terminating');
}

test('operation is running, return `true`.', () => {
  const runningApp = {
    status: {
      operationState: { phase: 'Running', startedAt: '2021-01-01T00:00:00Z' }
    }
  } as Application;

  expect(showTerminateButton(runningApp)).toBe(true);
});

test('operation finished successfully, return `false`.', () => {
  const succeededApp = {
    status: {
      operationState: { phase: 'Succeeded', startedAt: '2021-01-01T00:00:00Z', finishedAt: '2021-01-01T00:01:00Z' }
    }
  } as Application;

  expect(showTerminateButton(succeededApp)).toBe(false);
});

test('operation finished with failure, return `false`.', () => {
  const failedApp = {
    status: {
      operationState: { phase: 'Failed', startedAt: '2021-01-01T00:00:00Z', finishedAt: '2021-01-01T00:01:00Z' }
    }
  } as Application;

  expect(showTerminateButton(failedApp)).toBe(false);
});

test('operation is already terminating, return `false`.', () => {
  const terminatingApp = {
    status: {
      operationState: { phase: 'Terminating', startedAt: '2021-01-01T00:00:00Z' }
    }
  } as Application;

  expect(showTerminateButton(terminatingApp)).toBe(false);
});

test('no operation state, return `false`.', () => {
  const noOperationApp = { status: {} } as Application;

  expect(showTerminateButton(noOperationApp)).toBe(false);
});
