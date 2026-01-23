import { Application } from '../../../shared/models';

function isAutoSyncEnabled(app: Application): boolean {
  return !!(app.spec.syncPolicy?.automated && app.spec.syncPolicy.automated.enabled !== false);
}

test('automated.enabled is true, return to `true`.', () => {
  const enabledApp = {
    spec: {
      syncPolicy: {
        automated: { enabled: true }
      }
    }
  } as Application;

  expect(isAutoSyncEnabled(enabledApp)).toBe(true);
});

test('automated.enabled is undefined, return to `true`.', () => {
  const enabledApp = {
    spec: {
      syncPolicy: { automated: {} }
    }
  } as Application;

  expect(isAutoSyncEnabled(enabledApp)).toBe(true);
});

test('automated.enabled is false, return to `false`.', () => {
  const disabledApp = {
    spec: {
      syncPolicy: { automated: { enabled: false } }
    }
  } as Application;

  expect(isAutoSyncEnabled(disabledApp)).toBe(false);
});

test('syncPolicy is nil, return to `false`', () => {
  const noSyncPolicyApp = { spec: {} } as Application;
  expect(isAutoSyncEnabled(noSyncPolicyApp)).toBe(false);
});

test('automated is nil, return to `false`.', () => {
  const noAutomatedApp = { spec: { syncPolicy: {} } } as Application;
  expect(isAutoSyncEnabled(noAutomatedApp)).toBe(false);
});
