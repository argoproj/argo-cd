import {ErrorNotification, FormField, NotificationType, SlidingPanel} from 'argo-ui';
import * as React from 'react';
import {Checkbox, Form, FormApi, Text} from 'react-form';

import {Spinner} from '../../../shared/components';
import {Consumer} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationManualSyncFlags, ApplicationSyncOptions, SyncFlags} from '../application-sync-options';
import {ComparisonStatusIcon, nodeKey} from '../utils';

export const ApplicationSyncPanel = ({application, selectedResource, hide}: {application: models.Application; selectedResource: string; hide: () => any}) => {
    const [form, setForm] = React.useState<FormApi>(null);
    const isVisible = !!(selectedResource && application);
    const appResources = ((application && selectedResource && application.status && application.status.resources) || [])
        .sort((first, second) => nodeKey(first).localeCompare(nodeKey(second)))
        .filter(item => !item.hook);
    const syncResIndex = appResources.findIndex(item => nodeKey(item) === selectedResource);
    const syncStrategy = {} as models.SyncStrategy;
    const [isPending, setPending] = React.useState(false);

    return (
        <Consumer>
            {ctx => (
                <SlidingPanel
                    isMiddle={true}
                    isShown={isVisible}
                    onClose={() => hide()}
                    header={
                        <div>
                            <button
                                qe-id='application-sync-panel-button-synchronize'
                                className='argo-button argo-button--base'
                                disabled={isPending}
                                onClick={() => form.submitForm(null)}>
                                <Spinner show={isPending} style={{marginRight: '5px'}} />
                                Synchronize
                            </button>{' '}
                            <button onClick={() => hide()} className='argo-button argo-button--base-o'>
                                Cancel
                            </button>
                        </div>
                    }>
                    {isVisible && (
                        <Form
                            defaultValues={{
                                revision: application.spec.source.targetRevision || 'HEAD',
                                resources: appResources.map((_, i) => i === syncResIndex || syncResIndex === -1),
                                syncOptions: application.spec.syncPolicy ? application.spec.syncPolicy.syncOptions : ''
                            }}
                            validateError={values => ({
                                resources: values.resources.every((item: boolean) => !item) && 'Select at least one resource'
                            })}
                            onSubmit={async (params: any) => {
                                setPending(true);
                                let resources = appResources.filter((_, i) => params.resources[i]);
                                if (resources.length === appResources.length) {
                                    resources = null;
                                }

                                const syncFlags = {...params.syncFlags} as SyncFlags;

                                if (syncFlags.ApplyOnly) {
                                    syncStrategy.apply = {force: syncFlags.Force || false};
                                } else {
                                    syncStrategy.hook = {force: syncFlags.Force || false};
                                }

                                try {
                                    await services.applications.sync(
                                        application.metadata.name,
                                        params.revision,
                                        syncFlags.Prune || false,
                                        syncFlags.DryRun || false,
                                        syncStrategy,
                                        resources,
                                        params.syncOptions
                                    );
                                    hide();
                                } catch (e) {
                                    ctx.notifications.show({
                                        content: <ErrorNotification title='Unable to deploy revision' e={e} />,
                                        type: NotificationType.Error
                                    });
                                } finally {
                                    setPending(false);
                                }
                            }}
                            getApi={setForm}>
                            {formApi => (
                                <form role='form' className='width-control' onSubmit={formApi.submitForm}>
                                    <h6>
                                        Synchronizing application manifests from <a href={application.spec.source.repoURL}>{application.spec.source.repoURL}</a>
                                    </h6>
                                    <div className='argo-form-row'>
                                        <FormField formApi={formApi} label='Revision' field='revision' component={Text} />
                                    </div>

                                    <div className='argo-form-row'>
                                        <div style={{marginBottom: '1em'}}>
                                            <label>Sync Options</label>
                                            <ApplicationSyncOptions
                                                options={formApi.values.syncOptions}
                                                onChanged={opts => {
                                                    formApi.setTouched('syncOptions', true);
                                                    formApi.setValue('syncOptions', opts);
                                                }}
                                            />
                                            <FormField formApi={formApi} field='syncFlags' component={ApplicationManualSyncFlags} />
                                        </div>
                                        <label>Synchronize resources:</label>
                                        <div style={{float: 'right'}}>
                                            <a onClick={() => formApi.setValue('resources', formApi.values.resources.map(() => true))}>all</a> /{' '}
                                            <a
                                                onClick={() =>
                                                    formApi.setValue(
                                                        'resources',
                                                        application.status.resources.map((resource: models.ResourceStatus) => resource.status === models.SyncStatuses.OutOfSync)
                                                    )
                                                }>
                                                out of sync
                                            </a>{' '}
                                            / <a onClick={() => formApi.setValue('resources', formApi.values.resources.map(() => false))}>none</a>
                                        </div>
                                        {!formApi.values.resources.every((item: boolean) => item) && (
                                            <div className='application-details__warning'>WARNING: partial synchronization is not recorded in history</div>
                                        )}
                                        <div style={{paddingLeft: '1em'}}>
                                            {application.status.resources
                                                .filter(item => !item.hook)
                                                .map((item, i) => {
                                                    const resKey = nodeKey(item);
                                                    return (
                                                        <div key={resKey}>
                                                            <Checkbox id={resKey} field={`resources[${i}]`} />{' '}
                                                            <label htmlFor={resKey}>
                                                                {resKey} <ComparisonStatusIcon status={item.status} resource={item} />
                                                            </label>
                                                        </div>
                                                    );
                                                })}
                                            {formApi.errors.resources && <div className='argo-form-row__error-msg'>{formApi.errors.resources}</div>}
                                        </div>
                                    </div>
                                </form>
                            )}
                        </Form>
                    )}
                </SlidingPanel>
            )}
        </Consumer>
    );
};
