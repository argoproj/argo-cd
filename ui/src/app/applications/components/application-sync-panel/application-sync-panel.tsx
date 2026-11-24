import {ErrorNotification, NotificationType, SlidingPanel} from 'argo-ui';
import * as React from 'react';
import {FormApi} from 'argo-ui';

import {Spinner} from '../../../shared/components';
import {Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationSyncPanelBody} from './application-sync-panel-body';

import './application-sync-panel.scss';

function parseSelectedChildApp(selectedResource: string, application: models.Application): {name: string; namespace: string} | null {
    if (!selectedResource || selectedResource === 'all') {
        return null;
    }
    const parts = selectedResource.split('/');
    if (parts.length === 4 && parts[0] === 'argoproj.io' && parts[1] === 'Application') {
        const namespace = parts[2];
        const name = parts[3];
        if (name !== application.metadata.name || namespace !== application.metadata.namespace) {
            return {name, namespace};
        }
    }
    return null;
}

export const ApplicationSyncPanel = ({application, selectedResource, hide}: {application: models.Application; selectedResource: string; hide: () => any}) => {
    const ctx = React.useContext(Context);
    const [form, setForm] = React.useState<FormApi>(null);
    const isVisible = !!(selectedResource && application);
    const [childApp, setChildApp] = React.useState<models.Application | null>(null);
    const [failedAppRef, setFailedAppRef] = React.useState<{name: string; namespace: string} | null>(null);
    const childAppRef = parseSelectedChildApp(selectedResource, application);
    const childAppRefName = childAppRef?.name;
    const childAppRefNamespace = childAppRef?.namespace;

    React.useEffect(() => {
        if (!childAppRefName || !childAppRefNamespace) {
            return;
        }
        services.applications
            .get(childAppRefName, childAppRefNamespace, 'application')
            .then(app => setChildApp(app as models.Application))
            .catch(e => {
                setFailedAppRef({name: childAppRefName, namespace: childAppRefNamespace});
                ctx.notifications.show({
                    content: <ErrorNotification title={`Unable to load child application '${childAppRefName}'`} e={e} />,
                    type: NotificationType.Error
                });
            });
    }, [childAppRefName, childAppRefNamespace, ctx.notifications]);

    const [isPending, setPending] = React.useState(false);
    const targetApp = childAppRef && childApp && childApp.metadata.name === childAppRef.name && childApp.metadata.namespace === childAppRef.namespace ? childApp : application;
    const canSync = !(failedAppRef && failedAppRef.name === childAppRefName && failedAppRef.namespace === childAppRefNamespace);

    return (
        <SlidingPanel
            isMiddle={true}
            isShown={isVisible}
            onClose={() => hide()}
            header={
                <div>
                    {canSync && (
                        <>
                            <button
                                qe-id='application-sync-panel-button-synchronize'
                                className='argo-button argo-button--base'
                                disabled={isPending || !form}
                                onClick={() => form.submitForm(null)}>
                                <Spinner show={isPending} style={{marginRight: '5px'}} />
                                Synchronize
                            </button>{' '}
                        </>
                    )}
                    <button onClick={() => hide()} qe-id='application-sync-panel-button-cancel' className='argo-button argo-button--base-o'>
                        Cancel
                    </button>
                </div>
            }>
            {isVisible && canSync && (
                <ApplicationSyncPanelBody
                    key={`${targetApp.metadata.namespace}/${targetApp.metadata.name}`}
                    application={targetApp}
                    selectedResource={selectedResource}
                    hide={hide}
                    getApi={setForm}
                    setPending={setPending}
                />
            )}
            {isVisible && !canSync && (
                <div className='white-box'>
                    <p>Unable to load application '{childAppRefName}'. You may not have permission to view it, or it no longer exists.</p>
                </div>
            )}
        </SlidingPanel>
    );
};
