import {ErrorNotification, NotificationType} from 'argo-ui';
import * as React from 'react';
import {ContextApis} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationSelector} from '../../../shared/components';
import {ApplicationsOperationPanel} from '../applications-operation-panel/applications-operation-panel';
import {FormFunctionProps} from 'react-form';

interface Progress {
    percentage: number;
    title: string;
}
interface OperationHandlerContext {
    setProgress: (progress: Progress) => void;
    ctx: ContextApis;
}

export const ApplicationsTerminateSyncPanel = ({show, apps, hide}: {show: boolean; apps: models.Application[]; hide: () => void}) => {
    const handleSubmit = async (selectedApps: models.Application[], _: any, {ctx, setProgress}: OperationHandlerContext) => {
        for (let i = 0; i < selectedApps.length; i++) {
            const app = selectedApps[i];
            try {
                setProgress({
                    percentage: i / selectedApps.length,
                    title: `Terminating sync for ${app.metadata.name} (${i + 1}/${selectedApps.length})`
                });

                await services.applications.terminateOperation(app.metadata.name, app.metadata.namespace);
            } catch (e) {
                ctx.notifications.show({
                    content: <ErrorNotification title={`Unable to terminate sync ${app.metadata.name}`} e={e} />,
                    type: NotificationType.Error
                });
            }
        }
        setProgress({percentage: 100, title: 'All operations terminated'});
    };

    const syncingApps = apps.filter(app => app.status.operationState?.phase === 'Running' && app.status.operationState.operation.sync !== undefined);

    return (
        <ApplicationsOperationPanel show={show} apps={apps} hide={hide} title='Terminate Sync app(s)' buttonTitle='Terminate' onSubmit={handleSubmit}>
            {(formApi: FormFunctionProps) => <ApplicationSelector apps={syncingApps} formApi={formApi} filterOptions={{showOutOfSync: false, showAll: true, showNone: true}} />}
        </ApplicationsOperationPanel>
    );
};
