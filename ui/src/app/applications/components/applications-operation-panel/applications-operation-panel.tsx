import {ErrorNotification, NotificationType, SlidingPanel} from 'argo-ui';
import * as React from 'react';
import {Form, FormApi} from 'react-form';
import {ProgressPopup, Spinner} from '../../../shared/components';
import {Consumer, ContextApis} from '../../../shared/context';
import * as models from '../../../shared/models';

interface Progress {
    percentage: number;
    title: string;
}

interface OperationHandlerContext {
    setProgress: (progress: Progress) => void;
    ctx: ContextApis;
}

export interface OperationPanelProps {
    show: boolean;
    apps: models.Application[];
    hide: () => void;
    title: string;
    buttonTitle: string;
    children?: (formApi: FormApi) => React.ReactNode;
    onSubmit: (selectedApps: models.Application[], params: any, handlerCtx: OperationHandlerContext) => Promise<void>;
    validate?: (formApi: FormApi, ctx: ContextApis) => Promise<boolean>;
}

export const ApplicationsOperationPanel = ({show, apps, hide, title, buttonTitle, children, onSubmit, validate}: OperationPanelProps) => {
    const [form, setForm] = React.useState<FormApi>(null);
    const [progress, setProgress] = React.useState<Progress>(null);
    const [isPending, setPending] = React.useState(false);

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
                            <button
                                className='argo-button argo-button--base'
                                disabled={isPending}
                                onClick={async () => {
                                    if (validate) {
                                        const isValid = await validate(form, ctx);
                                        if (!isValid) {
                                            return;
                                        }
                                    }
                                    form.submitForm(null);
                                }}>
                                <Spinner show={isPending} style={{marginRight: '5px'}} />
                                {buttonTitle}
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

                            try {
                                setProgress({percentage: 0, title: 'Starting...'});
                                await onSubmit(selectedApps, params, {setProgress, ctx});
                            } catch (e) {
                                ctx.notifications.show({
                                    content: <ErrorNotification title='Operation failed' e={e} />,
                                    type: NotificationType.Error
                                });
                            } finally {
                                setPending(false);
                            }
                        }}
                        getApi={setForm}>
                        {formApi => (
                            <div className='argo-form-row' style={{marginTop: 0}}>
                                <h4>{title}</h4>
                                {progress !== null && <ProgressPopup onClose={() => setProgress(null)} percentage={progress.percentage} title={progress.title} />}
                                {children && children(formApi)}
                            </div>
                        )}
                    </Form>
                </SlidingPanel>
            )}
        </Consumer>
    );
};
