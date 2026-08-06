import {SlidingPanel} from 'argo-ui';
import * as React from 'react';
import {FormApi} from 'argo-ui';
import {Spinner} from '../../../shared/components';
import {ErrorBoundary} from '../../../shared/components/error-boundary/error-boundary';
import * as models from '../../../shared/models';

const ApplicationsRefreshPanelBody = React.lazy(() =>
    import(/* webpackChunkName: "apps-refresh-panel" */ './applications-refresh-panel-body').then(m => ({default: m.ApplicationsRefreshPanelBody}))
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
            {show && (
                <ErrorBoundary message='Failed to load refresh panel. Please reload and try again.'>
                    <React.Suspense fallback={<Spinner show={true} />}>
                        <ApplicationsRefreshPanelBody show={show} apps={apps} getApi={setForm} />
                    </React.Suspense>
                </ErrorBoundary>
            )}
        </SlidingPanel>
    );
};
