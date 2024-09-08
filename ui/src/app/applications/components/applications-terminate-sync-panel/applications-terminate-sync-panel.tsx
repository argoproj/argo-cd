import {ErrorNotification, NotificationType, SlidingPanel} from 'argo-ui';
import * as React from 'react';
import {Form, FormApi} from 'react-form';
import {ProgressPopup, Spinner} from '../../../shared/components';
import {Consumer, ContextApis} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationSelector} from '../../../shared/components';
import {confirmSyncingAppOfApps} from '../utils';

interface Progress {
    percentage: number;
    title: string;
}

export const ApplicationsTerminateSyncPanel = ({show, apps, hide}: {show: boolean; apps: models.Application[]; hide: () => void}) => {
    const [form, setForm] = React.useState<FormApi>(null);
    const [progress, setProgress] = React.useState<Progress>(null);
    const getSelectedApps = (params: any) => apps.filter((_, i) => params['app/' + i]);
    const [isPending, setPending] = React.useState(false);
    const syncHandler = (currentForm: FormApi, ctx: ContextApis, applications: models.Application[]) => {
        const formValues = currentForm.getFormState().values;
        const replaceChecked = formValues.syncOptions?.includes('Replace=true');
        const selectedApps = [];
        const selectedAppOfApps: models.Application[] = [];
        let containAppOfApps = false;

        for (const key in formValues) {
            if (key.startsWith('app/') && formValues[key]) {
                selectedApps.push(applications[parseInt(key.slice(key.lastIndexOf('/') + 1), 10)]);
            }
        }

        selectedApps.forEach(app => {
            if (app.isAppOfAppsPattern) {
                containAppOfApps = true;
                selectedAppOfApps.push(app);
            }
        });

        if (replaceChecked && containAppOfApps) {
            confirmSyncingAppOfApps(selectedAppOfApps, ctx, currentForm).then(confirmed => {
                setPending(confirmed ? true : false);
            });
        } else {
            currentForm.submitForm(null);
        }
    };

    apps.forEach(item => console.log(item.status.operationState.phase));
    return (
        <Consumer>
            {ctx => (
                <SlidingPanel
                    isMiddle={true}
                    isShown={show}
                    onClose={() => hide()}
                    header={
                        <div>
                            <button className='argo-button argo-button--base' disabled={isPending} onClick={() => syncHandler(form, ctx, apps)}>
                                <Spinner show={isPending} style={{marginRight: '5px'}} />
                                Terminate
                            </button>{' '}
                            <button onClick={() => hide()} className='argo-button argo-button--base-o'>
                                Cancel
                            </button>
                        </div>
                    }>
                    <Form
                        defaultValues={{syncFlags: []}}
                        onSubmit={async (params: any) => {
                            setPending(true);
                            const selectedApps = getSelectedApps(params);

                            if (selectedApps.length === 0) {
                                ctx.notifications.show({content: `No apps selected`, type: NotificationType.Error});
                                setPending(false);
                                return;
                            }

                            setProgress({percentage: 0, title: 'Starting...'});
                            let i = 0;
                            for (const app of selectedApps) {
                                await services.applications
                                    .terminateOperation(app.metadata.name, app.metadata.namespace)
                                    .catch(e => {
                                        ctx.notifications.show({
                                            content: <ErrorNotification title={`Unable to terminate sync ${app.metadata.name}`} e={e} />,
                                            type: NotificationType.Error
                                        });
                                    })
                                    .finally(() => {
                                        setPending(false);
                                    });
                                i++;
                                setProgress({
                                    percentage: i / selectedApps.length,
                                    title: `${i} of ${selectedApps.length} apps now terminating sync`
                                });
                            }
                            setProgress({percentage: 100, title: 'Complete'});
                        }}
                        getApi={setForm}>
                        {formApi => (
                            <React.Fragment>
                                <div className='argo-form-row' style={{marginTop: 0}}>
                                    <h4>Terminate Sync app(s)</h4>
                                    {progress !== null && <ProgressPopup onClose={() => setProgress(null)} percentage={progress.percentage} title={progress.title} />}

                                    <ApplicationSelector terminate apps={apps} formApi={formApi} />
                                </div>
                            </React.Fragment>
                        )}
                    </Form>
                </SlidingPanel>
            )}
        </Consumer>
    );
};
