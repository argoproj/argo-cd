import {SlidingPanel} from 'argo-ui';
import * as React from 'react';
import {FormApi} from 'argo-ui';

import {Spinner} from '../../../shared/components';
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
    const [form, setForm] = React.useState<FormApi>(null);
    const isVisible = !!(selectedResource && application);
    const [childApp, setChildApp] = React.useState<{key: string; app: models.Application} | null>(null);
    const childAppRef = parseSelectedChildApp(selectedResource, application);
    const childAppName = childAppRef?.name;
    const childAppNamespace = childAppRef?.namespace;

    React.useEffect(() => {
        if (!childAppName || !childAppNamespace) {
            return undefined;
        }
        let cancelled = false;
        services.applications.get(childAppName, childAppNamespace, 'application').then(app => {
            if (!cancelled) {
                setChildApp({key: selectedResource, app: app as models.Application});
            }
        });
        return () => {
            cancelled = true;
        };
    }, [selectedResource, childAppName, childAppNamespace]);

    const [isPending, setPending] = React.useState(false);
    const targetApp = childAppRef && childApp && childApp.key === selectedResource ? childApp.app : application;

    return (
        <SlidingPanel
            isMiddle={true}
            isShown={isVisible}
            onClose={() => hide()}
            header={
                <div>
                    <button
                        qe-id='application-sync-panel-button-synchronize'
                        className='argo-button argo-button--base'
                        disabled={isPending || !form}
                        onClick={() => form.submitForm(null)}>
                        <Spinner show={isPending} style={{marginRight: '5px'}} />
                        Synchronize
                    </button>{' '}
                    <button onClick={() => hide()} qe-id='application-sync-panel-button-cancel' className='argo-button argo-button--base-o'>
                        Cancel
                    </button>
                </div>
            }>
            {isVisible && <ApplicationSyncPanelBody application={targetApp} selectedResource={selectedResource} hide={hide} getApi={setForm} setPending={setPending} />}
        </SlidingPanel>
    );
};
