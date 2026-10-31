import * as models from '../../../shared/models';

export interface SyncFlagValues {
    Prune?: boolean;
    DryRun?: boolean;
    ApplyOnly?: boolean;
    Force?: boolean;
}

export interface ResolvedSyncFlags {
    prune: boolean;
    dryRun: boolean;
    applyOnly: boolean;
    force: boolean;
}

/**
 * Resolve bulk-sync flags for one app.
 * Checkbox values win only when the user has toggled that flag; otherwise fall back to
 * that app's syncPolicy.manualDefaults, then to legacy defaults (false).
 * When manualDefaults is set and prune is omitted, prune defaults to true (same as the API
 * and single-app sync panel).
 */
export function resolveBulkSyncFlags(syncFlags: SyncFlagValues, app: models.Application): ResolvedSyncFlags {
    const md = app.spec.syncPolicy?.manualDefaults;
    const flagOrDefault = (flag: keyof SyncFlagValues, fromManualDefaults: boolean | undefined): boolean => {
        if (Object.prototype.hasOwnProperty.call(syncFlags, flag)) {
            return !!syncFlags[flag];
        }
        return !!fromManualDefaults;
    };
    // Match SyncPolicy.GetPrune(): when manualDefaults exists and prune is unset, default true.
    const pruneDefault = md ? (md.prune ?? true) : undefined;
    return {
        prune: flagOrDefault('Prune', pruneDefault),
        dryRun: flagOrDefault('DryRun', md?.dryRun),
        applyOnly: flagOrDefault('ApplyOnly', md?.applyOnly),
        force: flagOrDefault('Force', md?.force)
    };
}
