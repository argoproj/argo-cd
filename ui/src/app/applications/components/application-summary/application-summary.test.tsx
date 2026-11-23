import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen, waitFor, within} from '@testing-library/react';
import * as React from 'react';
import {MemoryRouter} from 'react-router-dom';

import {Provider} from '../../../shared/context';
import * as models from '../../../shared/models';
import {ApplicationSummary} from './application-summary';

// lodash-es ships as untransformed ESM under node_modules; the retry options that import it are not exercised here.
jest.mock('lodash-es', () => ({}));
// DeepLinks default-imports React, which the jest transform does not support.
jest.mock('../../../shared/components/deep-links', () => ({DeepLinks: (): null => null}));

jest.mock('../../../shared/services', () => ({
    services: {
        applications: {getLinks: () => Promise.resolve({items: []})},
        authService: {settings: () => Promise.resolve({})},
        repos: {revisions: () => Promise.resolve({branches: [], tags: []})}
    }
}));

const drySource = {repoURL: 'https://github.com/example/dry.git', targetRevision: 'HEAD', path: 'guestbook'};

const hydratorApp = (sourceHydrator: models.SourceHydrator) =>
    ({
        metadata: {name: 'my-app', namespace: 'argocd'},
        spec: {project: 'default', destination: {server: 'https://kubernetes.default.svc', namespace: 'default'}, sourceHydrator},
        status: {summary: {}, sync: {status: 'Synced'}, health: {status: 'Healthy'}}
    }) as models.Application;

const renderSummary = (app: models.Application) => {
    const updateApp = jest.fn<Promise<void>, [models.Application, {validate?: boolean}]>(() => Promise.resolve());
    render(
        <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
            <Provider value={{baseHref: '/'} as any}>
                <MemoryRouter>
                    <ApplicationSummary app={app} updateApp={updateApp} />
                </MemoryRouter>
            </Provider>
        </QueryClientProvider>
    );
    return updateApp;
};

const sourcePanel = () => screen.getByText('SOURCE').closest('.editable-panel') as HTMLElement;
// The row of the SOURCE panel whose title cell starts with the given title.
const sourceRow = (title: string) => Array.from(sourcePanel().querySelectorAll('.white-box__details-row')).find(row => row.firstElementChild.textContent.startsWith(title));

describe('ApplicationSummary source hydrator sync source', () => {
    test('shows the tag of an OCI sync source and no hydrate-to branch', () => {
        renderSummary(hydratorApp({drySource, syncSource: {repoURL: 'oci://registry.example.com/hydrated', targetRevision: 'dev', path: '.'}}));

        expect(sourceRow('TAG').lastElementChild.textContent).toBe('dev');
        expect(sourceRow('TARGET BRANCH')).toBeUndefined();
        expect(sourceRow('Hydrate To')).toBeUndefined();
    });

    test('follows the sync repo while editing and saves only the fields that apply to it', async () => {
        const updateApp = renderSummary(hydratorApp({drySource, syncSource: {targetBranch: 'env/dev', path: 'guestbook'}, hydrateTo: {targetBranch: 'env/dev-next'}}));
        expect(sourceRow('TARGET BRANCH')).toBeDefined();
        expect(sourceRow('Hydrate To')).toBeDefined();

        fireEvent.click(within(sourcePanel()).getByText('Edit'));
        fireEvent.change(sourceRow('REPO URL (OPTIONAL)').querySelector('input'), {target: {value: 'oci://registry.example.com/hydrated'}});
        await waitFor(() => expect(sourceRow('TAG')).toBeDefined());
        expect(sourceRow('TARGET BRANCH')).toBeUndefined();
        expect(sourceRow('Hydrate To')).toBeUndefined();

        fireEvent.change(sourceRow('TAG').querySelector('input'), {target: {value: 'dev'}});
        fireEvent.click(within(sourcePanel()).getByText('Save'));
        await waitFor(() => expect(updateApp).toHaveBeenCalled());
        expect(updateApp.mock.lastCall[0].spec.sourceHydrator).toEqual({
            drySource,
            syncSource: {repoURL: 'oci://registry.example.com/hydrated', targetRevision: 'dev', path: 'guestbook'}
        });

        // Back in view mode the saved application, which is still Git here, decides again.
        await waitFor(() => expect(sourceRow('TARGET BRANCH')).toBeDefined());
        expect(sourceRow('Hydrate To')).toBeDefined();
    });
});
