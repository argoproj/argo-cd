import {ErrorNotification, FormField, NotificationType} from 'argo-ui';
import * as React from 'react';
import {FormApi, FormFunctionProps} from 'react-form';
import {ARGO_WARNING_COLOR} from '../../../shared/components';
import {ContextApis} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationRetryOptions} from '../application-retry-options/application-retry-options';
import {ApplicationManualSyncFlags, ApplicationSyncOptions, FORCE_WARNING, SyncFlags} from '../application-sync-options/application-sync-options';
import {ApplicationSelector} from '../../../shared/components';
import {confirmSyncingAppOfApps, getAppDefaultSource} from '../utils';
import {ApplicationsOperationPanel} from '../applications-operation-panel/applications-operation-panel';

interface Progress {
    percentage: number;
    title: string;
}
interface OperationHandlerContext {
    setProgress: (progress: Progress) => void;
    ctx: ContextApis;
}

export const ApplicationsSyncPanel = ({show, apps, hide}: {show: boolean; apps: models.Application[]; hide: () => void}) => {
    const validate = async (formApi: FormApi, ctx: ContextApis) => {
        const formValues = formApi.getFormState().values;
        const replaceChecked = formValues.syncOptions?.includes('Replace=true');
        const selectedApps = apps.filter((_, i) => formValues['app/' + i]);
        const selectedAppOfApps = selectedApps.filter(app => app.isAppOfAppsPattern);

        if (replaceChecked && selectedAppOfApps.length > 0) {
            const confirmed = await confirmSyncingAppOfApps(selectedAppOfApps, ctx, formApi);
            return confirmed;
        }
        return true;
    };

    const handleSubmit = async (selectedApps: models.Application[], params: any, {ctx, setProgress}: OperationHandlerContext) => {
        if (params.syncFlags?.Force) {
            const confirmed = await ctx.popup.confirm('Synchronize with force?', () => (
                <div>
                    <i className='fa fa-exclamation-triangle' style={{color: ARGO_WARNING_COLOR}} /> {FORCE_WARNING} Are you sure you want to continue?
                </div>
            ));
            if (!confirmed) {
                return;
            }
        }

        const syncFlags = {...params.syncFlags} as SyncFlags;
        const syncStrategy: models.SyncStrategy = syncFlags.ApplyOnly || false ? {apply: {force: syncFlags.Force}} : {hook: {force: syncFlags.Force}};

        for (let i = 0; i < selectedApps.length; i++) {
            const app = selectedApps[i];
            try {
                setProgress({
                    percentage: i / selectedApps.length,
                    title: `Syncing ${app.metadata.name} (${i + 1}/${selectedApps.length})`
                });

                await services.applications.sync(
                    app.metadata.name,
                    app.metadata.namespace,
                    getAppDefaultSource(app).targetRevision,
                    syncFlags.Prune || false,
                    syncFlags.DryRun || false,
                    syncStrategy,
                    null,
                    params.syncOptions,
                    params.retryStrategy
                );
            } catch (e) {
                ctx.notifications.show({
                    content: <ErrorNotification title={`Unable to sync ${app.metadata.name}`} e={e} />,
                    type: NotificationType.Error
                });
            }
        }
        setProgress({percentage: 100, title: 'All applications synced'});
    };

    return (
        <ApplicationsOperationPanel show={show} apps={apps} hide={hide} title='Sync app(s)' buttonTitle='Sync' validate={validate} onSubmit={handleSubmit}>
            {(formApi: FormFunctionProps) => (
                <>
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
                            id='applications-sync-panel'
                        />
                    </div>
                    <ApplicationRetryOptions id='applications-sync-panel' formApi={formApi} />
                    <ApplicationSelector apps={apps} formApi={formApi} />
                </>
            )}
        </ApplicationsOperationPanel>
    );
};
