import * as React from 'react';
import {SlidingPanel, ErrorNotification, NotificationType} from 'argo-ui';
import {Form, FormApi} from 'react-form';
import {ProgressPopup} from '../../../shared/components';
import {Consumer} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationSelector} from '../../../shared/components';

interface Progress {
    percentage: number;
    title: string;
}

export const ApplicationsRestartPanel = ({show, apps, hide}: {show: boolean; apps: models.Application[]; hide: () => void}) => {
    const [form, setForm] = React.useState<FormApi>(null);
    const [progress, setProgress] = React.useState<Progress>(null);
    const [isRestartPending, setRestartPending] = React.useState(false);

    const getSelectedApps = (params: any) => apps.filter((_, i) => params['app/' + i]);

    return (
        <Consumer>
            {ctx => (
                <SlidingPanel
                    isMiddle={true}
                    isShown={show}
                    onClose={hide}
                    header={
                        <div>
                            <button className='argo-button argo-button--base' disabled={isRestartPending} onClick={() => form.submitForm(null)}>
                                Restart
                            </button>{' '}
                            <button onClick={hide} className='argo-button argo-button--base-o'>
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

                            setProgress({percentage: 0, title: 'Restarting applications'});
                            setRestartPending(true);
                            let i = 0;
                            const restartActions = [];
                            for (const app of selectedApps) {
                                const restartAction = async () => {
                                    try {
                                        const tree = await services.applications.resourceTree(app.metadata.name, app.metadata.namespace);
                                        const relevantResources = (tree?.nodes || []).filter(
                                            n => (n.kind === 'Deployment' || n.kind === 'StatefulSet' || n.kind === 'DaemonSet') && n.group === 'apps'
                                        );
                                        for (const resource of relevantResources) {
                                            await services.applications.runResourceAction(app.metadata.name, app.metadata.namespace, resource, 'restart');
                                        }
                                    } catch (e) {
                                        ctx.notifications.show({
                                            content: <ErrorNotification title={`Unable to restart resources in ${app.metadata.name}`} e={e} />,
                                            type: NotificationType.Error
                                        });
                                    }
                                    i++;
                                    setProgress({
                                        percentage: i / selectedApps.length,
                                        title: `Restarted ${i} of ${selectedApps.length} applications`
                                    });
                                };
                                restartActions.push(restartAction());

                                if (restartActions.length >= 20) {
                                    await Promise.all(restartActions);
                                    restartActions.length = 0;
                                }
                            }
                            await Promise.all(restartActions);
                            setRestartPending(false);
                            hide();
                        }}
                        getApi={setForm}>
                        {formApi => (
                            <React.Fragment>
                                <div className='argo-form-row' style={{marginTop: 0}}>
                                    <h4>Restart app(s)</h4>
                                    {progress !== null && <ProgressPopup onClose={() => setProgress(null)} percentage={progress.percentage} title={progress.title} />}
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
