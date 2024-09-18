import {ErrorNotification, NotificationType, SlidingPanel} from 'argo-ui';
import * as React from 'react';
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

const RefreshTypes = ['normal', 'hard'];

export const ApplicationsRefreshPanel = ({show, apps, hide}: {show: boolean; apps: models.Application[]; hide: () => void}) => {
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
                                Refresh
                            </button>{' '}
                            <button onClick={() => hide()} className='argo-button argo-button--base-o'>
                                Cancel
                            </button>
                        </div>
                    }>
                    <Form
                        defaultValues={{refreshType: 'normal'}}
                        onSubmit={async (params: any) => {
                            const selectedApps = getSelectedApps(params);
                            if (selectedApps.length === 0) {
                                ctx.notifications.show({content: `No apps selected`, type: NotificationType.Error});
                                return;
                            }

                            setProgress({percentage: 0, title: 'Refreshing applications'});
                            let i = 0;
                            const refreshActions = [];
                            for (const app of selectedApps) {
                                const refreshAction = async () => {
                                    await services.applications.get(app.metadata.name, app.metadata.namespace, params.refreshType).catch(e => {
                                        ctx.notifications.show({
                                            content: <ErrorNotification title={`Unable to refresh ${app.metadata.name}`} e={e} />,
                                            type: NotificationType.Error
                                        });
                                    });
                                    i++;
                                    setProgress({
                                        percentage: i / selectedApps.length,
                                        title: `Refreshed ${i} of ${selectedApps.length} applications`
                                    });
                                };
                                refreshActions.push(refreshAction());

                                if (refreshActions.length >= 20) {
                                    await Promise.all(refreshActions);
                                    refreshActions.length = 0;
                                }
                            }
                            await Promise.all(refreshActions);
                            setProgress({percentage: 100, title: 'Complete'});
                        }}
                        getApi={setForm}>
                        {formApi => (
                            <React.Fragment>
                                <div className='argo-form-row' style={{marginTop: 0}}>
                                    <h4>Refresh app(s)</h4>
                                    {progress !== null && <ProgressPopup onClose={() => setProgress(null)} percentage={progress.percentage} title={progress.title} />}
                                    <div style={{marginBottom: '1em'}}>
                                        <label>Refresh Type</label>
                                        <div className='row application-sync-options'>
                                            {RefreshTypes.map(refreshType => (
                                                <label key={refreshType} style={{paddingRight: '1.5em', marginTop: '0.4em'}}>
                                                    <input
                                                        type='radio'
                                                        value={refreshType}
                                                        checked={formApi.values.refreshType === refreshType}
                                                        onChange={() => formApi.setValue('refreshType', refreshType)}
                                                        style={{marginRight: '5px', transform: 'translateY(2px)'}}
                                                    />
                                                    {refreshType}
                                                </label>
                                            ))}
                                        </div>
                                    </div>
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
