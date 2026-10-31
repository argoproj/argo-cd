import * as React from 'react';
import {fireEvent, render, screen, waitFor} from '@testing-library/react';
import {Provider} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationsSyncPanel} from './applications-sync-panel';

// lodash-es ships as untransformed ESM under node_modules and is not covered by the jest
// transform config, so map the single helper the retry options use to a CJS-friendly stub.
jest.mock('lodash-es', () => ({
    __esModule: true,
    omit: (value: any, ...keys: string[]) => Object.fromEntries(Object.entries(value || {}).filter(([key]) => !keys.flat().includes(key)))
}));

jest.mock('../../../shared/services', () => ({
    services: {
        applications: {
            sync: jest.fn(() => Promise.resolve(true))
        },
        authService: {
            settings: jest.fn(() => Promise.resolve({}))
        }
    }
}));

const newApp = (name: string) =>
    ({
        metadata: {name, namespace: 'argocd'},
        spec: {
            project: 'default',
            source: {repoURL: 'https://github.com/argoproj/argocd-example-apps', path: name, targetRevision: 'HEAD'},
            destination: {server: 'https://kubernetes.default.svc', namespace: 'default'}
        },
        status: {sync: {status: 'OutOfSync'}, health: {status: 'Healthy'}}
    }) as unknown as models.Application;

const appA = newApp('app-a');
const appB = newApp('app-b');
const appC = newApp('app-c');
const appNew = newApp('app-new');

const ctx = {
    popup: {confirm: jest.fn(), prompt: jest.fn()},
    notifications: {show: jest.fn()},
    navigation: {goto: jest.fn()},
    baseHref: '/',
    history: null
} as any;

const renderPanel = (apps: models.Application[]) => {
    const ui = (list: models.Application[]) => (
        <Provider value={ctx}>
            <ApplicationsSyncPanel show={true} apps={list} hide={jest.fn()} />
        </Provider>
    );
    const result = render(ui(apps));
    return {rerender: (list: models.Application[]) => result.rerender(ui(list))};
};

const checkboxOf = (name: string) => screen.getByText(name, {exact: false}).closest('label').querySelector('input') as HTMLInputElement;

const syncButton = () => screen.getByRole('button', {name: 'Sync'});

describe('ApplicationsSyncPanel', () => {
    beforeEach(() => jest.clearAllMocks());

    it.each([
        ['apps are removed from the list', [appC]],
        ['new apps are prepended to the list', [appNew, appA, appB, appC]]
    ])('syncs the selected app when %s while the panel is open', async (_, updatedApps) => {
        const {rerender} = renderPanel([appA, appB, appC]);
        await waitFor(() => expect(syncButton()).toBeEnabled());

        fireEvent.click(checkboxOf('app-c'));
        rerender(updatedApps);
        expect(checkboxOf('app-c').checked).toBe(true);

        fireEvent.click(syncButton());

        await waitFor(() => expect(services.applications.sync).toHaveBeenCalledTimes(1));
        const [name, namespace] = (services.applications.sync as jest.Mock).mock.calls[0];
        expect([name, namespace]).toEqual(['app-c', 'argocd']);
        expect(ctx.notifications.show).not.toHaveBeenCalled();
    });
});
