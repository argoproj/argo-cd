import {ErrorNotification, NotificationType, SlidingPanel} from 'argo-ui';
import * as React from 'react';
import {Checkbox, Form, FormApi} from 'react-form';
import {ProgressPopup} from '../../../shared/components';
import {Consumer} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ComparisonStatusIcon, HealthStatusIcon, OperationPhaseIcon} from '../utils';

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
                        onSubmit={async (params: any) => {
                            const selectedApps = getSelectedApps(params);
                            if (selectedApps.length === 0) {
                                ctx.notifications.show({content: `No apps selected`, type: NotificationType.Error});
                                return;
                            }
                            const syncStrategy: models.SyncStrategy = params.applyOnly ? {apply: {force: params.force}} : {hook: {force: params.force}};
                            setProgress({percentage: 0, title: 'Starting...'});
                            let i = 0;
                            for (const app of selectedApps) {
                                await services.applications.sync(app.metadata.name, app.spec.source.targetRevision, params.prune, params.dryRun, syncStrategy, null).catch(e => {
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
                                <div className='argo-form-row'>
                                    <h4>Sync app(s)</h4>
                                    {progress !== null && <ProgressPopup onClose={() => setProgress(null)} percentage={progress.percentage} title={progress.title} />}
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
                                        Apps (<a onClick={() => apps.forEach((_, i) => formApi.setValue('app/' + i, true))}>all</a>/
                                        <a onClick={() => apps.forEach((app, i) => formApi.setValue('app/' + i, app.status.sync.status === models.SyncStatuses.OutOfSync))}>
                                            out of sync
                                        </a>
                                        /<a onClick={() => apps.forEach((_, i) => formApi.setValue('app/' + i, false))}>none</a>
                                        ):
                                    </label>
                                    <div style={{paddingLeft: '1em'}}>
                                        {apps.map((app, i) => (
                                            <label key={app.metadata.name}>
                                                <Checkbox field={`app/${i}`} />
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
