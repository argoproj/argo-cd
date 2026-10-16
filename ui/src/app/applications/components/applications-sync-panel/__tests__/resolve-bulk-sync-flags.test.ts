import {resolveBulkSyncFlags} from '../resolve-bulk-sync-flags';
import * as models from '../../../../shared/models';

function appWithManualDefaults(manualDefaults?: models.ManualDefaults): models.Application {
    return {
        metadata: {name: 'demo', namespace: 'argocd'},
        spec: {
            syncPolicy: manualDefaults ? {manualDefaults} : undefined
        }
    } as models.Application;
}

describe('resolveBulkSyncFlags', () => {
    test('uses checkbox values when the user toggled flags', () => {
        const syncFlags = {DryRun: false, Force: true, ApplyOnly: true};
        const app = appWithManualDefaults({dryRun: true, force: false, applyOnly: false});

        expect(resolveBulkSyncFlags(syncFlags, app)).toEqual({
            prune: false,
            dryRun: false,
            applyOnly: true,
            force: true
        });
    });

    test('falls back to app manualDefaults when flags are untouched', () => {
        const syncFlags = {};
        const app = appWithManualDefaults({dryRun: true, force: true, applyOnly: true, prune: true});

        expect(resolveBulkSyncFlags(syncFlags, app)).toEqual({
            prune: false, // prune stays always-explicit; unchecked means false
            dryRun: true,
            applyOnly: true,
            force: true
        });
    });

    test('uses legacy defaults when untouched and no manualDefaults', () => {
        const syncFlags = {};
        const app = appWithManualDefaults();

        expect(resolveBulkSyncFlags(syncFlags, app)).toEqual({
            prune: false,
            dryRun: false,
            applyOnly: false,
            force: false
        });
    });

    test('honors explicit prune checkbox', () => {
        const syncFlags = {Prune: true};
        const app = appWithManualDefaults({prune: false});

        expect(resolveBulkSyncFlags(syncFlags, app).prune).toBe(true);
    });
});
