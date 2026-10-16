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
 * Prune stays always-explicit so an unchecked box can disable pruning even when
 * manualDefaults.prune defaults to true.
 */
export function resolveBulkSyncFlags(syncFlags: SyncFlagValues, app: models.Application): ResolvedSyncFlags {
    const md = app.spec.syncPolicy?.manualDefaults;
    const flagOrDefault = (flag: keyof SyncFlagValues, fromManualDefaults: boolean | undefined): boolean => {
        if (Object.prototype.hasOwnProperty.call(syncFlags, flag)) {
            return !!syncFlags[flag];
        }
        return !!fromManualDefaults;
    };
    return {
        prune: !!syncFlags.Prune,
        dryRun: flagOrDefault('DryRun', md?.dryRun),
        applyOnly: flagOrDefault('ApplyOnly', md?.applyOnly),
        force: flagOrDefault('Force', md?.force)
    };
}
