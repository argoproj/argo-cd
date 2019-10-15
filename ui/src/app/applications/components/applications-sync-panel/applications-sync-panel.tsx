import {ErrorNotification, NotificationType, SlidingPanel} from 'argo-ui';
import * as React from 'react';
import {Checkbox, Form, FormApi} from 'react-form';
import * as models from '../../../shared/models';
import {Consumer} from '../../../shared/context';
import {services} from '../../../shared/services';
import {ComparisonStatusIcon, HealthStatusIcon} from "../utils";

export const ApplicationsSyncPanel = ({show, apps, hide}: {
    show: boolean;
    apps: models.Application[];
    hide: () => any;
}) => {
    const [form, setForm] = React.useState<FormApi>(null);
    const syncStrategy = {} as models.SyncStrategy;
    return (
        <Consumer>
            {(ctx) => (
                <SlidingPanel isMiddle={true} isShown={show} onClose={() => hide()} header={(
                    <div>
                        <button className='argo-button argo-button--base' onClick={() => form.submitForm(null)}>Sync
                        </button>
                        <button onClick={() => hide()} className='argo-button argo-button--base-o'>Cancel</button>
                    </div>
                )}>
                    <Form
                        onSubmit={async (params: any) => {
                            for (const app of apps) {
                                if (params.applyOnly) {
                                    syncStrategy.apply = {force: params.force};
                                } else {
                                    syncStrategy.hook = {force: params.force};
                                }
                                try {
                                    await services.applications.sync(app.metadata.name, app.spec.source.targetRevision, params.prune, params.dryRun, syncStrategy, null);
                                } catch (e) {
                                    ctx.notifications.show({
                                        content: <ErrorNotification title='Unable to deploy' e={e}/>,
                                        type: NotificationType.Error,
                                    });
                                }
                            }
                            hide();
                        }}
                        getApi={setForm}>
                        {() => (
                            <React.Fragment>
                                <h4>Sync {apps.length} app(s)</h4>
                                <ul>
                                    {apps.map((app) => (<li>
                                        <HealthStatusIcon state={app.status.health}/>
                                        <ComparisonStatusIcon status={app.status.sync.status}/>
                                        {app.metadata.name}
                                    </li>))}
                                </ul>
                                <div className='argo-form-row'>
                                    <label><Checkbox field='prune'/> Prune</label>
                                    <label><Checkbox field='dryRun'/> Dry Run</label>
                                    <label><Checkbox field='applyOnly'/> Apply Only</label>
                                    <label><Checkbox field='force'/> Force</label>
                                </div>
                            </React.Fragment>
                        )}
                    </Form>
                </SlidingPanel>
            )
            }
        </Consumer>
    );
};
