import {ErrorNotification, FormField, NotificationType, SlidingPanel} from 'argo-ui';
import * as React from 'react';
import {Form, FormApi} from 'react-form';
import {ProgressPopup} from '../../../shared/components';
import {Consumer} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationRetryOptions} from '../application-retry-options/application-retry-options';
import {ApplicationManualSyncFlags, ApplicationSyncOptions, SyncFlags} from '../application-sync-options/application-sync-options';
import {ApplicationSelector} from '../../../shared/components';

interface Progress {
    percentage: number;
    title: string;
}

export const ApplicationsSyncPanel = ({show, apps, hide}: {show: boolean; apps: models.Application[]; hide: () => void}) => {
    const [form, setForm] = React.useState<FormApi>(null);
    const [progress, setProgress] = React.useState<Progress>(null);
    const getSelectedApps = (params: any) => apps.filter((_, i) => params['app/' + i]);
    return (
        <Consumer>
            {ctx => (
                <SlidingPanel
                    isMiddle={true}
                    isShown={show}
                    onClose={() => hide()}
                    header={
                        <div>
                            <button className='argo-button argo-button--base' onClick={() => form.submitForm(null)}>
                                Sync
                            </button>{' '}
                            <button onClick={() => hide()} className='argo-button argo-button--base-o'>
                                Cancel
                            </button>
                        </div>
                    }>
                    <Form
                        defaultValues={{syncFlags: []}}
                        onSubmit={async (params: any) => {
                            const selectedApps = getSelectedApps(params);
                            if (selectedApps.length === 0) {
                                ctx.notifications.show({content: `No apps selected`, type: NotificationType.Error});
                                return;
                            }

                            const syncFlags = {...params.syncFlags} as SyncFlags;

                            const syncStrategy: models.SyncStrategy =
                                syncFlags.ApplyOnly || false ? {apply: {force: syncFlags.Force || false}} : {hook: {force: syncFlags.Force || false}};

                            setProgress({percentage: 0, title: 'Starting...'});
                            let i = 0;
                            for (const app of selectedApps) {
                                await services.applications
                                    .sync(
                                        app.metadata.name,
                                        app.spec.source.targetRevision,
                                        syncFlags.Prune || false,
                                        syncFlags.DryRun || false,
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
                                    });
                                i++;
                                setProgress({
                                    percentage: i / selectedApps.length,
                                    title: `${i} of ${selectedApps.length} apps now syncing`
                                });
                            }
                            setProgress({percentage: 100, title: 'Complete'});
                        }}
                        getApi={setForm}>
                        {formApi => (
                            <React.Fragment>
                                <div className='argo-form-row' style={{marginTop: 0}}>
                                    <h4>Sync app(s)</h4>
                                    {progress !== null && <ProgressPopup onClose={() => setProgress(null)} percentage={progress.percentage} title={progress.title} />}
                                    <div style={{marginBottom: '1em'}}>
                                        <FormField formApi={formApi} field='syncFlags' component={ApplicationManualSyncFlags} />
                                    </div>
                                    <div style={{marginBottom: '1em'}}>
                                        <label>Sync Options</label>
                                        <ApplicationSyncOptions
                                            options={formApi.values.syncOptions}
                                            onChanged={opts => {
                                                formApi.setTouched('syncOptions', true);
                                                formApi.setValue('syncOptions', opts);
                                            }}
                                        />
                                    </div>

                                    <ApplicationRetryOptions formApi={formApi} />

                                    <ApplicationSelector apps={apps} formApi={formApi} />
                                </div>
                            </React.Fragment>
                        )}
                    </Form>
                </SlidingPanel>
            )}
        </Consumer>
    );
};
