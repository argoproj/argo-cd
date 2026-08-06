import {SlidingPanel} from 'argo-ui';
import * as React from 'react';
import {FormApi} from 'argo-ui';
import {Spinner} from '../../../shared/components';
import {lazyWithBoundary} from '../../../shared/components/lazy-with-boundary';
import {Consumer, ContextApis} from '../../../shared/context';
import * as models from '../../../shared/models';
import {confirmSyncingAppOfApps} from '../utils';

const ApplicationsSyncPanelBody = lazyWithBoundary(
    React.lazy(() => import(/* webpackChunkName: "apps-sync-panel" */ './applications-sync-panel-body').then(m => ({default: m.ApplicationsSyncPanelBody}))),
    'Failed to load sync panel. Please reload and try again.'
);

export const ApplicationsSyncPanel = ({show, apps, hide}: {show: boolean; apps: models.Application[]; hide: () => void}) => {
    const [form, setForm] = React.useState<FormApi>(null);
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
                                qe-id='applications-sync-panel-button-synchronize'
                                className='argo-button argo-button--base'
                                disabled={isPending || !form}
                                onClick={() => syncHandler(form, ctx, apps)}>
                                <Spinner show={isPending} style={{marginRight: '5px'}} />
                                Sync
                            </button>{' '}
                            <button onClick={() => hide()} qe-id='applications-sync-panel-button-cancel' className='argo-button argo-button--base-o'>
                                Cancel
                            </button>
                        </div>
                    }>
                    {show && <ApplicationsSyncPanelBody apps={apps} getApi={setForm} setPending={setPending} />}
                </SlidingPanel>
            )}
        </Consumer>
    );
};
