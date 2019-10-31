import {ErrorNotification, NotificationType, SlidingPanel} from 'argo-ui';
import * as React from 'react';
import {Checkbox, Form, FormApi} from 'react-form';
import {Consumer} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ComparisonStatusIcon, HealthStatusIcon, OperationPhaseIcon} from '../utils';

export const ApplicationsSyncPanel = ({show, apps, hide}: {show: boolean; apps: models.Application[]; hide: () => void}) => {
    const [form, setForm] = React.useState<FormApi>(null);
    const getSelectedApps = (params: any) => apps.filter(app => params['app/' + app.metadata.name]);
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
                        onSubmit={async (params: any) => {
                            const selectedApps = getSelectedApps(params);
                            if (selectedApps.length === 0) {
                                ctx.notifications.show({content: `No apps selected`, type: NotificationType.Error});
                                return;
                            }
                            ctx.notifications.show({
                                content: `Syncing ${selectedApps.length} app(s)`,
                                type: NotificationType.Success
                            });
                            const syncStrategy = (params.applyOnly ? {apply: {force: params.force}} : {hook: {force: params.force}}) as models.SyncStrategy;
                            for (const app of selectedApps) {
                                await services.applications.sync(app.metadata.name, app.spec.source.targetRevision, params.prune, params.dryRun, syncStrategy, null).catch(e => {
                                    ctx.notifications.show({
                                        content: <ErrorNotification title={`Unable to sync ${app.metadata.name}`} e={e} />,
                                        type: NotificationType.Error
                                    });
                                });
                            }
                            hide();
                        }}
                        getApi={setForm}>
                        {formApi => (
                            <React.Fragment>
                                <div className='argo-form-row'>
                                    <h4>Sync app(s)</h4>
                                    <label>Options:</label>
                                    <div style={{paddingLeft: '1em'}}>
                                        <label>
                                            <Checkbox field='prune' /> Prune
                                        </label>
                                        &nbsp;
                                        <label>
                                            <Checkbox field='dryRun' /> Dry Run
                                        </label>
                                        &nbsp;
                                        <label>
                                            <Checkbox field='applyOnly' /> Apply Only
                                        </label>
                                        &nbsp;
                                        <label>
                                            <Checkbox field='force' /> Force
                                        </label>
                                    </div>
                                    <label>
                                        Apps (<a onClick={() => apps.forEach(app => formApi.setValue('app/' + app.metadata.name, true))}>all</a>/
                                        <a
                                            onClick={() =>
                                                apps.forEach(app => formApi.setValue('app/' + app.metadata.name, app.status.sync.status === models.SyncStatuses.OutOfSync))
                                            }>
                                            out of sync
                                        </a>
                                        /<a onClick={() => apps.forEach(app => formApi.setValue('app/' + app.metadata.name, false))}>none</a>
                                        ):
                                    </label>
                                    <div style={{paddingLeft: '1em'}}>
                                        {apps.map(app => (
                                            <label key={app.metadata.name}>
                                                <Checkbox field={`app/${app.metadata.name}`} />
                                                &nbsp;
                                                {app.metadata.name}
                                                &nbsp;
                                                <ComparisonStatusIcon status={app.status.sync.status} />
                                                &nbsp;
                                                <HealthStatusIcon state={app.status.health} />
                                                &nbsp;
                                                <OperationPhaseIcon app={app} />
                                                <br />
                                            </label>
                                        ))}
                                    </div>
                                </div>
                            </React.Fragment>
                        )}
                    </Form>
                </SlidingPanel>
            )}
        </Consumer>
    );
};
