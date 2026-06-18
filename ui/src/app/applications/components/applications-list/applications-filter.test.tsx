import { getAutoSyncStatus } from './applications-filter';
import { SyncPolicy } from '../../../shared/models';

const AUTO_SYNC_ENABLED = 'Enabled';
const AUTO_SYNC_DISABLED = 'Disabled';
const AUTO_SYNC_SELECTIVE = 'Selective';

test('automated.enabled is true, return to `Enabled`.', () => {
    const syncPolicy = {
        automated: {
            enabled: true,
            prune: false,
            selfHeal: false
        }
    } as SyncPolicy;

    expect(getAutoSyncStatus(syncPolicy)).toBe(AUTO_SYNC_ENABLED);
});

test('automated.enabled is undefined, return to `Enabled`.', () => {
    const syncPolicy = {
        automated: {}
    } as unknown as SyncPolicy;

    expect(getAutoSyncStatus(syncPolicy)).toBe(AUTO_SYNC_ENABLED);
});

test('automated.enabled is false, return to `Disabled`.', () => {
    const syncPolicy = {
        automated: {
            enabled: false,
            prune: false,
            selfHeal: false
        }
    } as SyncPolicy;

    expect(getAutoSyncStatus(syncPolicy)).toBe(AUTO_SYNC_DISABLED);
});

test('syncPolicy is nil, return to `Disabled`', () => {
    expect(getAutoSyncStatus(undefined)).toBe(AUTO_SYNC_DISABLED);
});

test('automated is nil, return to `Disabled`.', () => {
    const syncPolicy = {} as SyncPolicy;
    expect(getAutoSyncStatus(syncPolicy)).toBe(AUTO_SYNC_DISABLED);
});

test('selective sync is enabled, return to `Selective`.', () => {
    const syncPolicy = {
        automated: {
            enabled: true,
            prune: false,
            selfHeal: false,
            selective: {
                enabled: true
            }
        }
    } as SyncPolicy;

    expect(getAutoSyncStatus(syncPolicy)).toBe(AUTO_SYNC_SELECTIVE);
});

test('selective sync present but disabled, return to `Enabled`.', () => {
    const syncPolicy = {
        automated: {
            enabled: true,
            prune: false,
            selfHeal: false,
            selective: {
                enabled: false
            }
        }
    } as SyncPolicy;

    expect(getAutoSyncStatus(syncPolicy)).toBe(AUTO_SYNC_ENABLED);
});
