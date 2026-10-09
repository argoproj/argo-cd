import {SlidingPanel} from 'argo-ui';
import * as React from 'react';
import {FormApi} from 'argo-ui';
import * as models from '../../../shared/models';
import {ApplicationsRefreshPanelBody} from './applications-refresh-panel-body';

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
