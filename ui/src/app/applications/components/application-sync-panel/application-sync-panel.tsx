import {ErrorNotification, FormField, NotificationType, SlidingPanel, Tooltip} from 'argo-ui';
import * as React from 'react';
import {Form, FormApi, Text} from 'react-form';

import {ARGO_WARNING_COLOR, CheckboxField, Spinner} from '../../../shared/components';
import {Consumer} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationRetryOptions} from '../application-retry-options/application-retry-options';
import {
    ApplicationManualSyncFlags,
    ApplicationSyncOptions,
    FORCE_WARNING,
    SyncFlags,
    REPLACE_WARNING,
    PRUNE_ALL_WARNING
} from '../application-sync-options/application-sync-options';
import {ComparisonStatusIcon, getAppDefaultSource, nodeKey} from '../utils';

import './application-sync-panel.scss';

export const ApplicationSyncPanel = ({application, selectedResource, hide}: {application: models.Application; selectedResource: string; hide: () => any}) => {
    const [form, setForm] = React.useState<FormApi>(null);
    const isVisible = !!(selectedResource && application);
    const appResources = ((application && selectedResource && application.status && application.status.resources) || [])
        .sort((first, second) => nodeKey(first).localeCompare(nodeKey(second)))
        .filter(item => !item.hook);
    const syncResIndex = appResources.findIndex(item => nodeKey(item) === selectedResource);
    const syncStrategy = {} as models.SyncStrategy;
    const [isPending, setPending] = React.useState(false);
    const source = getAppDefaultSource(application);

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
                                revision: new URLSearchParams(ctx.history.location.search).get('revision') || source.targetRevision || 'HEAD',
                                resources: appResources.map((_, i) => i === syncResIndex || syncResIndex === -1),
                                syncOptions: application.spec.syncPolicy ? application.spec.syncPolicy.syncOptions : []
                            }}
                            validateError={values => ({
                                resources: values.resources.every((item: boolean) => !item) && 'Select at least one resource'
                            })}
                            onSubmit={async (params: any) => {
                                setPending(true);
                                let selectedResources = appResources.filter((_, i) => params.resources[i]);
                                const allResourcesAreSelected = selectedResources.length === appResources.length;
                                const syncFlags = {...params.syncFlags} as SyncFlags;

                                const allRequirePruning = !selectedResources.some(resource => !resource?.requiresPruning);
                                if (syncFlags.Prune && allRequirePruning && allResourcesAreSelected) {
                                    const confirmed = await ctx.popup.confirm('Prune all resources?', () => (
                                        <div>
                                            <i className='fa fa-exclamation-triangle' style={{color: ARGO_WARNING_COLOR}} />
                                            {PRUNE_ALL_WARNING} Are you sure you want to continue?
                                        </div>
                                    ));
                                    if (!confirmed) {
                                        setPending(false);
                                        return;
                                    }
                                }
                                if (allResourcesAreSelected) {
                                    selectedResources = null;
                                }
                                const replace = params.syncOptions?.findIndex((opt: string) => opt === 'Replace=true') > -1;
                                if (replace) {
                                    const confirmed = await ctx.popup.confirm('Synchronize using replace?', () => (
                                        <div>
                                            <i className='fa fa-exclamation-triangle' style={{color: ARGO_WARNING_COLOR}} /> {REPLACE_WARNING} Are you sure you want to continue?
                                        </div>
                                    ));
                                    if (!confirmed) {
                                        setPending(false);
                                        return;
                                    }
                                }

                                const force = syncFlags.Force || false;

                                if (syncFlags.ApplyOnly) {
                                    syncStrategy.apply = {force};
                                } else {
                                    syncStrategy.hook = {force};
                                }
                                if (force) {
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

                                try {
                                    await services.applications.sync(
                                        application.metadata.name,
                                        application.metadata.namespace,
                                        params.revision,
                                        syncFlags.Prune || false,
                                        syncFlags.DryRun || false,
                                        syncStrategy,
                                        selectedResources,
                                        params.syncOptions,
                                        params.retryStrategy
                                    );
                                    hide();
                                } catch (e) {
                                    ctx.notifications.show({
                                        content: <ErrorNotification title='Unable to sync' e={e} />,
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
                                        Synchronizing application manifests from <a href={source.repoURL}>{source.repoURL}</a>
                                    </h6>
                                    <div className='argo-form-row'>
                                        <FormField formApi={formApi} label='Revision' field='revision' component={Text} />
                                    </div>

                                    <div className='argo-form-row'>
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
                                                id='application-sync-panel'
                                            />
                                        </div>

                                        <ApplicationRetryOptions
                                            id='application-sync-panel'
                                            formApi={formApi}
                                            initValues={application.spec.syncPolicy ? application.spec.syncPolicy.retry : null}
                                        />

                                        <label>Synchronize resources:</label>
                                        <div style={{float: 'right'}}>
                                            <a
                                                onClick={() =>
                                                    formApi.setValue(
                                                        'resources',
                                                        formApi.values.resources.map(() => true)
                                                    )
                                                }>
                                                all
                                            </a>{' '}
                                            /{' '}
                                            <a
                                                onClick={() =>
                                                    formApi.setValue(
                                                        'resources',
                                                        application.status.resources
                                                            .filter(item => !item.hook)
                                                            .map((resource: models.ResourceStatus) => resource.status === models.SyncStatuses.OutOfSync)
                                                    )
                                                }>
                                                out of sync
                                            </a>{' '}
                                            /{' '}
                                            <a
                                                onClick={() =>
                                                    formApi.setValue(
                                                        'resources',
                                                        formApi.values.resources.map(() => false)
                                                    )
                                                }>
                                                none
                                            </a>
                                        </div>
                                        <div className='application-details__warning'>
                                            {!formApi.values.resources.every((item: boolean) => item) && <div>WARNING: partial synchronization is not recorded in history</div>}
                                        </div>
                                        <div>
                                            {application.status.resources
                                                .filter(item => !item.hook)
                                                .map((item, i) => {
                                                    const resKey = nodeKey(item);
                                                    const contentStart = resKey.substr(0, Math.floor(resKey.length / 2));
                                                    let contentEnd = resKey.substr(-Math.floor(resKey.length / 2));
                                                    // We want the ellipsis to be in the middle of our text, so we use RTL layout to put it there.
                                                    // Unfortunately, strong LTR characters get jumbled around, so make sure that the last character isn't strong.
                                                    const firstLetter = /[a-z]/i.exec(contentEnd);
                                                    if (firstLetter) {
                                                        contentEnd = contentEnd.slice(firstLetter.index);
                                                    }
                                                    const isLongLabel = resKey.length > 68;
                                                    return (
                                                        <div key={resKey} className='application-sync-panel__resource'>
                                                            <CheckboxField id={resKey} field={`resources[${i}]`} />
                                                            <Tooltip content={<div style={{wordBreak: 'break-all'}}>{resKey}</div>}>
                                                                <div className='container'>
                                                                    {isLongLabel ? (
                                                                        <label htmlFor={resKey} content-start={contentStart} content-end={contentEnd} />
                                                                    ) : (
                                                                        <label htmlFor={resKey}>{resKey}</label>
                                                                    )}
                                                                </div>
                                                            </Tooltip>
                                                            <ComparisonStatusIcon status={item.status} resource={item} />
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
