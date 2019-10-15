import {ErrorNotification, NotificationType, SlidingPanel} from 'argo-ui';
import * as React from 'react';
import {Checkbox, Form, FormApi} from 'react-form';
import * as models from '../../../shared/models';
import {Consumer} from '../../../shared/context';
import {services} from '../../../shared/services';

export const ApplicationsSyncPanel = ({apps, hide}: {
    apps: models.Application[];
    hide: () => any;
}) => {
    const [form, setForm] = React.useState<FormApi>(null);
    const syncStrategy = {} as models.SyncStrategy;
    return (
        <Consumer>
            {(ctx) => (
                <SlidingPanel isMiddle={true} isShown={true} onClose={() => hide()} header={(
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
                            <div className='argo-form-row'>
                                <h6>Synchronizing apps</h6>
                                <span>
                                        <label>
                                            <Checkbox field='prune'/>
                                            Prune
                                        </label>
                                    </span>
                                <span>
                                        <Checkbox id='dry-run-checkbox' field='dryRun'/>
                                        <label htmlFor='dry-run-checkbox'>Dry Run</label>
                                    </span>
                                <span>
                                        <Checkbox id='apply-only-checkbox' field='applyOnly'/>
                                        <label htmlFor='apply-only-checkbox'>Apply Only</label>
                                     </span>
                                <span>
                                        <Checkbox id='force-checkbox' field='force'/>
                                        <label htmlFor='force-checkbox'>Force</label>
                                     </span>
                            </div>
                        )}
                    </Form>
                </SlidingPanel>
            )
            }
        </Consumer>
    );
};
