import {SlidingPanel} from 'argo-ui';
import * as React from 'react';
import {FormApi} from 'argo-ui';

import {Spinner} from '../../../shared/components';
import {lazyWithBoundary} from '../../../shared/components/lazy-with-boundary';
import * as models from '../../../shared/models';

import './application-sync-panel.scss';

const ApplicationSyncPanelBody = lazyWithBoundary(
    React.lazy(() => import(/* webpackChunkName: "app-sync-panel" */ './application-sync-panel-body').then(m => ({default: m.ApplicationSyncPanelBody}))),
    'Failed to load sync panel. Please reload and try again.'
);

export const ApplicationSyncPanel = ({application, selectedResource, hide}: {application: models.Application; selectedResource: string; hide: () => any}) => {
    const [form, setForm] = React.useState<FormApi>(null);
    const isVisible = !!(selectedResource && application);
    const [isPending, setPending] = React.useState(false);

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
            {isVisible && <ApplicationSyncPanelBody application={application} selectedResource={selectedResource} hide={hide} getApi={setForm} setPending={setPending} />}
        </SlidingPanel>
    );
};
