import * as React from 'react';
import {fireEvent, render, screen} from '@testing-library/react';
import {MemoryRouter} from 'react-router-dom';
import * as models from '../../../shared/models';
import {AuthSettingsCtx, Context} from '../../../shared/context';
import {ApplicationSummary} from './application-summary';

// Both components pull in lodash-es, which Jest cannot parse.
jest.mock('../application-retry-options/application-retry-options', () => ({ApplicationRetryOptions: () => null}));
jest.mock('../application-retry-view/application-retry-view', () => ({ApplicationRetryView: () => null}));

const appWithoutSource = {
    metadata: {name: 'no-source', namespace: 'argocd'},
    spec: {
        project: 'default',
        destination: {name: 'in-cluster', namespace: 'default'}
    },
    status: {
        sync: {status: 'Unknown'},
        health: {status: 'Unknown'},
        summary: {},
        conditions: [{type: 'InvalidSpecError', message: 'spec.source.repoURL and either spec.source.path or spec.source.chart are required'}]
    }
} as unknown as models.Application;

// EditablePanel reads notifications off the app Context.
const ctxValue = {notifications: {show: () => undefined}} as any;

describe('ApplicationSummary', () => {
    it('renders an application that has neither source nor sources', () => {
        render(
            <Context.Provider value={ctxValue}>
                <MemoryRouter>
                    <ApplicationSummary app={appWithoutSource} updateApp={() => Promise.resolve()} />
                </MemoryRouter>
            </Context.Provider>
        );
        expect(screen.getByText('NO-SOURCE')).toBeInTheDocument();
        expect(screen.getByText('REPO URL')).toBeInTheDocument();
    });

    it('enables the source hydrator on an application that has neither source nor sources', () => {
        render(
            <Context.Provider value={ctxValue}>
                <AuthSettingsCtx.Provider value={{hydratorEnabled: true} as models.AuthSettings}>
                    <MemoryRouter>
                        <ApplicationSummary app={appWithoutSource} updateApp={() => Promise.resolve()} />
                    </MemoryRouter>
                </AuthSettingsCtx.Provider>
            </Context.Provider>
        );
        // The second EDIT button belongs to the SOURCE panel.
        fireEvent.click(screen.getAllByText('Edit')[1]);
        const checkbox = document.getElementById('enable-hydrator') as HTMLInputElement;
        fireEvent.click(checkbox);
        expect(checkbox.checked).toBe(true);
    });
});
