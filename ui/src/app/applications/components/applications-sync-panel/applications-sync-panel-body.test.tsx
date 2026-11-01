import * as React from 'react';
import {act, render} from '@testing-library/react';
import {FormApi} from 'argo-ui';
import * as models from '../../../shared/models';
import {Context} from '../../../shared/context';
import {services} from '../../../shared/services';
import {ApplicationsSyncPanelBody} from './applications-sync-panel-body';

// The retry options pull in lodash-es, which Jest cannot parse.
jest.mock('../application-retry-options/application-retry-options', () => ({ApplicationRetryOptions: () => null}));
// The app selector is irrelevant here; apps are selected through the form values.
jest.mock('../../../shared/components', () => ({
    ...jest.requireActual('../../../shared/components'),
    ApplicationSelector: () => null
}));
jest.mock('../../../shared/services', () => ({
    services: {
        applications: {sync: jest.fn(() => Promise.resolve(true))},
        authService: {settings: () => Promise.resolve({})}
    }
}));

const app = (name: string, source?: models.ApplicationSource) =>
    ({
        metadata: {name, namespace: 'argocd'},
        spec: {project: 'default', source, destination: {name: 'in-cluster', namespace: 'default'}}
    }) as unknown as models.Application;

const showNotification = jest.fn();
const ctxValue = {notifications: {show: showNotification}, popup: {confirm: () => Promise.resolve(true)}} as any;

describe('ApplicationsSyncPanelBody', () => {
    it('skips an application that has neither source nor sources and keeps syncing the remaining apps', async () => {
        const apps = [app('no-source'), app('with-source', {repoURL: 'https://github.com/org/repo.git', path: '.', targetRevision: 'main'})];
        let formApi: FormApi;
        const setPending = jest.fn();
        render(
            <Context.Provider value={ctxValue}>
                <ApplicationsSyncPanelBody apps={apps} getApi={api => (formApi = api)} setPending={setPending} />
            </Context.Provider>
        );

        await act(async () => {
            formApi.setValue('app/0', true);
            formApi.setValue('app/1', true);
        });
        await act(async () => {
            formApi.submitForm(null);
        });

        const sync = services.applications.sync as jest.Mock;
        expect(sync.mock.calls.map(call => [call[0], call[2]])).toEqual([['with-source', 'main']]);
        expect(showNotification).toHaveBeenCalledWith(expect.objectContaining({content: 'Unable to sync no-source: the application has no source configured'}));
        // The form stays pending until every request has settled.
        expect(setPending.mock.calls.map(call => call[0])).toEqual([true, false]);
    });
});
