import * as React from 'react';
import {fireEvent, render, screen} from '@testing-library/react';
import * as models from '../../../shared/models';
import {Context} from '../../../shared/context';
import {ApplicationSyncPanelBody} from './application-sync-panel-body';

// The retry options pull in lodash-es, which Jest cannot parse.
jest.mock('../application-retry-options/application-retry-options', () => ({ApplicationRetryOptions: () => null}));

const appWithoutSource = {
    metadata: {name: 'no-source', namespace: 'argocd'},
    spec: {project: 'default', destination: {name: 'in-cluster', namespace: 'default'}},
    status: {sync: {status: 'Unknown'}, health: {status: 'Unknown'}, summary: {}}
} as unknown as models.Application;

const ctxValue = {history: {location: {search: ''}}, notifications: {show: () => undefined}} as any;

describe('ApplicationSyncPanelBody', () => {
    it('renders for an application that has neither source nor sources', () => {
        render(
            <Context.Provider value={ctxValue}>
                <ApplicationSyncPanelBody application={appWithoutSource} selectedResource={null} hide={() => undefined} getApi={() => undefined} setPending={() => undefined} />
            </Context.Provider>
        );
        expect(screen.getByText(/Synchronizing application manifests from/)).toBeInTheDocument();
    });

    it('selects out-of-sync resources for an application without status.resources', () => {
        render(
            <Context.Provider value={ctxValue}>
                <ApplicationSyncPanelBody application={appWithoutSource} selectedResource={null} hide={() => undefined} getApi={() => undefined} setPending={() => undefined} />
            </Context.Provider>
        );
        expect(() => fireEvent.click(screen.getByText('out of sync'))).not.toThrow();
    });
});
