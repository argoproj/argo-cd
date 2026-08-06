import {SlidingPanel} from 'argo-ui';
import * as React from 'react';
import {FormApi} from 'argo-ui';
import {lazyWithBoundary} from '../../../shared/components/lazy-with-boundary';
import * as models from '../../../shared/models';

const ApplicationsRefreshPanelBody = lazyWithBoundary(
    React.lazy(() => import(/* webpackChunkName: "apps-refresh-panel" */ './applications-refresh-panel-body').then(m => ({default: m.ApplicationsRefreshPanelBody}))),
    'Failed to load refresh panel. Please reload and try again.'
);

export const ApplicationsRefreshPanel = ({show, apps, hide}: {show: boolean; apps: models.Application[]; hide: () => void}) => {
    const [form, setForm] = React.useState<FormApi>(null);

    return (
        <SlidingPanel
            isMiddle={true}
            isShown={show}
            onClose={() => hide()}
            header={
                <div>
                    <button className='argo-button argo-button--base' disabled={!form} onClick={() => form.submitForm(null)}>
                        Refresh
                    </button>{' '}
                    <button onClick={() => hide()} className='argo-button argo-button--base-o'>
                        Cancel
                    </button>
                </div>
            }>
            {show && <ApplicationsRefreshPanelBody apps={apps} getApi={setForm} />}
        </SlidingPanel>
    );
};
