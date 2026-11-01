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
        const syncFlags = {Prune: false, DryRun: false, Force: true, ApplyOnly: true};
        const app = appWithManualDefaults({prune: true, dryRun: true, force: false, applyOnly: false});

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
            prune: true,
            dryRun: true,
            applyOnly: true,
            force: true
        });
    });

    test('manualDefaults with prune omitted defaults prune to true when untouched', () => {
        const syncFlags = {};
        const app = appWithManualDefaults({force: true});

        expect(resolveBulkSyncFlags(syncFlags, app)).toEqual({
            prune: true,
            dryRun: false,
            applyOnly: false,
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

    test('honors explicit prune checkbox over manualDefaults', () => {
        const syncFlags = {Prune: true};
        const app = appWithManualDefaults({prune: false});

        expect(resolveBulkSyncFlags(syncFlags, app).prune).toBe(true);
    });

    test('explicit prune false overrides manualDefaults.prune true', () => {
        const syncFlags = {Prune: false};
        const app = appWithManualDefaults({prune: true});

        expect(resolveBulkSyncFlags(syncFlags, app).prune).toBe(false);
    });
});
