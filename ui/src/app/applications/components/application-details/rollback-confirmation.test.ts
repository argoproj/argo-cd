import * as models from '../../../shared/models';
import {getRollbackConfirmation} from './rollback-confirmation';

const APP_NAME = 'test-app';

const appWithSyncPolicy = (syncPolicy?: Partial<models.SyncPolicy>) =>
    ({
        metadata: {name: APP_NAME, namespace: 'argocd'},
        spec: {
            project: 'default',
            source: {repoURL: 'https://github.com/org/repo.git', path: '.', targetRevision: 'HEAD'},
            destination: {server: 'https://kubernetes.default.svc', namespace: 'default'},
            syncPolicy
        },
        status: {}
    }) as unknown as models.Application;

const appWith = (automated?: Partial<models.Automated>) => appWithSyncPolicy({automated: {prune: false, selfHeal: false, enabled: true, ...automated}});

const PLAIN = `Are you sure you want to rollback application '${APP_NAME}'?`;
const DISABLE = `Auto-Sync needs to be disabled in order for rollback to occur.
Are you sure you want to disable auto-sync and rollback application '${APP_NAME}'?`;
const STAYS_ENABLED = `Auto-Sync stays enabled and will skip the current revision until a new revision is available.
Are you sure you want to rollback application '${APP_NAME}'?`;

describe('getRollbackConfirmation', () => {
    describe('when automated sync is not in play', () => {
        it('just asks for confirmation when the application has no sync policy', () => {
            expect(getRollbackConfirmation(appWithSyncPolicy(), APP_NAME)).toEqual({needDisableRollback: false, message: PLAIN});
        });

        it('just asks for confirmation when the sync policy carries no automated block', () => {
            expect(getRollbackConfirmation(appWithSyncPolicy({syncOptions: []}), APP_NAME)).toEqual({needDisableRollback: false, message: PLAIN});
        });

        it('just asks for confirmation when automated sync is explicitly disabled', () => {
            expect(getRollbackConfirmation(appWith({enabled: false}), APP_NAME)).toEqual({needDisableRollback: false, message: PLAIN});
        });

        // Nothing has to be disabled, so the instance-wide default cannot change the outcome.
        it('ignores the instance-wide default when automated sync is disabled', () => {
            expect(getRollbackConfirmation(appWith({enabled: false}), APP_NAME, true)).toEqual({needDisableRollback: false, message: PLAIN});
        });
    });

    describe('when automated sync is enabled and rollback awareness is off', () => {
        it('offers to disable automated sync when nothing opts in', () => {
            expect(getRollbackConfirmation(appWith(), APP_NAME, false)).toEqual({needDisableRollback: true, message: DISABLE});
        });

        // An undefined default means the settings API did not report the field at all.
        it('treats an unknown instance-wide default as off', () => {
            expect(getRollbackConfirmation(appWith(), APP_NAME)).toEqual({needDisableRollback: true, message: DISABLE});
        });

        it('applies an explicit per-application opt out against an opted-in instance', () => {
            expect(getRollbackConfirmation(appWith({rollbackAware: false}), APP_NAME, true)).toEqual({needDisableRollback: true, message: DISABLE});
        });
    });

    describe('when automated sync is enabled and rollback awareness is on', () => {
        it('keeps automated sync enabled when the application opts in', () => {
            expect(getRollbackConfirmation(appWith({rollbackAware: true}), APP_NAME, false)).toEqual({needDisableRollback: false, message: STAYS_ENABLED});
        });

        it('keeps automated sync enabled when the instance opts in', () => {
            expect(getRollbackConfirmation(appWith(), APP_NAME, true)).toEqual({needDisableRollback: false, message: STAYS_ENABLED});
        });

        it('applies an explicit per-application opt in against an opted-out instance', () => {
            expect(getRollbackConfirmation(appWith({rollbackAware: true}), APP_NAME, false)).toEqual({needDisableRollback: false, message: STAYS_ENABLED});
        });

        // `automated.enabled` defaults to true, so an absent value still counts as enabled.
        it('treats an absent enabled flag as automated sync being on', () => {
            expect(getRollbackConfirmation(appWith({enabled: undefined, rollbackAware: true}), APP_NAME)).toEqual({needDisableRollback: false, message: STAYS_ENABLED});
        });
    });

    it('names the application being rolled back', () => {
        expect(getRollbackConfirmation(appWithSyncPolicy(), 'other-app').message).toContain(`'other-app'`);
    });
});
