import * as models from '../../../shared/models';
import {isAutomatedSyncEnabled, isRollbackAwareAutoSync} from '../utils';

export interface RollbackConfirmation {
    /** Whether rolling back has to turn automated sync off before it can proceed. */
    needDisableRollback: boolean;
    /** The text shown in the rollback confirmation popup. */
    message: string;
}

/**
 * Describes what rolling back an application is about to do, for the confirmation popup.
 *
 * Rollback and automated sync used to be mutually exclusive, so rolling back had to disable
 * automated sync first. With rollback-aware automated sync the controller records the rolled back
 * revision and skips it instead, so automated sync can stay enabled. An explicit per-application
 * `rollbackAware` wins over `rollbackAwareDefault`, the instance-wide default served by the
 * settings API.
 */
export function getRollbackConfirmation(application: models.Application, appName: string, rollbackAwareDefault?: boolean): RollbackConfirmation {
    const autoSyncEnabled = isAutomatedSyncEnabled(application);
    const rollbackAware = isRollbackAwareAutoSync(application, rollbackAwareDefault);
    const needDisableRollback = autoSyncEnabled && !rollbackAware;

    if (needDisableRollback) {
        return {
            needDisableRollback,
            message: `Auto-Sync needs to be disabled in order for rollback to occur.
Are you sure you want to disable auto-sync and rollback application '${appName}'?`
        };
    }
    if (autoSyncEnabled) {
        return {
            needDisableRollback,
            message: `Auto-Sync stays enabled and will skip the current revision until a new revision is available.
Are you sure you want to rollback application '${appName}'?`
        };
    }
    return {needDisableRollback, message: `Are you sure you want to rollback application '${appName}'?`};
}
