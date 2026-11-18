import {ErrorNotification, FormField, NotificationType} from 'argo-ui';
import * as React from 'react';
import {Form, FormApi} from 'argo-ui';
import {ARGO_WARNING_COLOR, ProgressPopup} from '../../../shared/components';
import {Consumer} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationRetryOptions} from '../application-retry-options/application-retry-options';
import {ApplicationManualSyncFlags, ApplicationSyncOptions, FORCE_WARNING} from '../application-sync-options/application-sync-options';
import {ApplicationSelector} from '../../../shared/components';
import {getAppDefaultSource} from '../utils';
import {resolveBulkSyncFlags, SyncFlagValues} from './resolve-bulk-sync-flags';

interface Progress {
    percentage: number;
    title: string;
}

export const ApplicationsSyncPanelBody = ({apps, getApi, setPending}: {apps: models.Application[]; getApi: (api: FormApi) => void; setPending: (pending: boolean) => void}) => {
    const [progress, setProgress] = React.useState<Progress>(null);
    const getSelectedApps = (params: any) => apps.filter((_, i) => params['app/' + i]);
    return (
        <Consumer>
            {ctx => (
                <Form
                    defaultValues={{syncFlags: []}}
                    onSubmit={async (params: any) => {
                        setPending(true);
                        const selectedApps = getSelectedApps(params);
                        if (selectedApps.length === 0) {
                            ctx.notifications.show({content: `No apps selected`, type: NotificationType.Error});
                            setPending(false);
                            return;
                        }

                        const syncFlags = {...params.syncFlags} as SyncFlagValues;
                        const resolvedByApp = selectedApps.map(app => ({
                            app,
                            flags: resolveBulkSyncFlags(syncFlags, app)
                        }));

                        // Confirm force based on per-app resolved values (checkbox or manualDefaults).
                        if (resolvedByApp.some(({flags}) => flags.force)) {
                            const confirmed = await ctx.popup.confirm('Synchronize with force?', () => (
                                <div>
                                    <i className='fa fa-exclamation-triangle' style={{color: ARGO_WARNING_COLOR}} /> {FORCE_WARNING} Are you sure you want to continue?
                                </div>
                            ));
                            if (!confirmed) {
                                setPending(false);
                                return;
                            }
                        }

                        setProgress({percentage: 0, title: 'Starting...'});
                        let i = 0;
                        for (const {app, flags} of resolvedByApp) {
                            // flags.* are already resolved per app (checkbox if toggled, else
                            // that app's manualDefaults, else legacy false). Always send the
                            // resolved booleans so the API does not re-apply defaults.
                            const syncStrategy: models.SyncStrategy = flags.applyOnly ? {apply: {force: flags.force}} : {hook: {force: flags.force}};
                            await services.applications
                                .sync(
                                    app.metadata.name,
                                    app.metadata.namespace,
                                    getAppDefaultSource(app).targetRevision,
                                    flags.prune,
                                    flags.dryRun,
                                    syncStrategy,
                                    null,
                                    params.syncOptions,
                                    params.retryStrategy
                                )
                                .catch(e => {
                                    ctx.notifications.show({
                                        content: <ErrorNotification title={`Unable to sync ${app.metadata.name}`} e={e} />,
                                        type: NotificationType.Error
                                    });
                                })
                                .finally(() => {
                                    setPending(false);
                                });
                            i++;
                            setProgress({
                                percentage: i / selectedApps.length,
                                title: `${i} of ${selectedApps.length} apps now syncing`
                            });
                        }
                        setProgress({percentage: 100, title: 'Complete'});
                    }}
                    getApi={getApi}>
                    {formApi => (
                        <div className='argo-form-row' style={{marginTop: 0}}>
                            <h4>Sync app(s)</h4>
                            {progress !== null && <ProgressPopup onClose={() => setProgress(null)} percentage={progress.percentage} title={progress.title} />}
                            <div style={{marginBottom: '1em'}}>
                                <FormField formApi={formApi} field='syncFlags' component={ApplicationManualSyncFlags} />
                                <div style={{marginTop: '0.5em', fontSize: '12px', color: '#6d7f8b'}}>Untouched options use each app&apos;s manual sync defaults.</div>
                            </div>
                            <div style={{marginBottom: '1em'}}>
                                <label>Sync Options</label>
                                <ApplicationSyncOptions
                                    options={formApi.values.syncOptions}
                                    onChanged={opts => {
                                        formApi.setTouched('syncOptions', true);
                                        formApi.setValue('syncOptions', opts);
                                    }}
                                    id='applications-sync-panel'
                                />
                            </div>

                            <ApplicationRetryOptions id='applications-sync-panel' formApi={formApi} />

                            <ApplicationSelector apps={apps} formApi={formApi} />
                        </div>
                    )}
                </Form>
            )}
        </Consumer>
    );
};
