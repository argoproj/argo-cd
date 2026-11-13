import {act, fireEvent, render, screen} from '@testing-library/react';
import * as React from 'react';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {MemoryRouter} from 'react-router-dom';

import {Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import {ApplicationSummary} from './application-summary';

jest.mock('lodash-es', () => jest.requireActual('lodash'));
// deep-links.tsx uses a default React import that this jest config does not support.
jest.mock('../../../shared/components/deep-links', () => ({DeepLinks: () => null}));
jest.mock('../../../shared/services', () => ({
    services: {
        applications: {getLinks: jest.fn().mockResolvedValue({items: []}), updateSpec: jest.fn()},
        projects: {list: jest.fn().mockResolvedValue([])}
    }
}));

describe('ApplicationSummary', () => {
    beforeEach(() => {
        Object.assign(navigator, {clipboard: {writeText: jest.fn().mockResolvedValue(undefined)}});
    });

    test('displays the name upper-cased but copies it with its original casing', async () => {
        const app = {
            metadata: {name: 'Guestbook-App', namespace: 'argocd'},
            spec: {project: 'default', source: {repoURL: 'https://example.com/repo.git', path: '.', targetRevision: 'HEAD'}, destination: {server: 'https://k8s', namespace: 'default'}},
            status: {sync: {status: 'Synced'}, health: {status: 'Healthy'}, summary: {}}
        } as unknown as models.Application;

        const ctx = {popup: {}, notifications: {show: jest.fn()}, navigation: {}, baseHref: '/', history: {}} as any;
        render(
            <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
                <MemoryRouter>
                    <Context.Provider value={ctx}>
                        <ApplicationSummary app={app} updateApp={jest.fn()} />
                    </Context.Provider>
                </MemoryRouter>
            </QueryClientProvider>
        );

        expect(screen.getByText('GUESTBOOK-APP')).toBeTruthy();
        await act(async () => {
            fireEvent.click(screen.getByLabelText('Copy application name'));
        });
        expect(navigator.clipboard.writeText).toHaveBeenCalledWith('Guestbook-App');
    });
});
