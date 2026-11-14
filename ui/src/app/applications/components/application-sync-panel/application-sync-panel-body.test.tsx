import {FormApi} from 'argo-ui';
import {act, fireEvent, render, screen} from '@testing-library/react';
import {createMemoryHistory} from 'history';
import * as React from 'react';

import {Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationSyncPanelBody} from './application-sync-panel-body';

jest.mock('../../../shared/services', () => ({services: {applications: {sync: jest.fn()}, authService: {settings: jest.fn().mockResolvedValue({syncWithReplaceAllowed: false})}}}));

// argo-ui's Tooltip unmounts its tippy root from a timer, which floods the output with act() warnings.
jest.mock('argo-ui', () => ({...jest.requireActual('argo-ui'), Tooltip: ({children}: {children: React.ReactNode}) => children}));

// lodash-es ships as untransformed ESM under node_modules; stub the single helper application-retry-options uses.
jest.mock('lodash-es', () => ({
    __esModule: true,
    omit: (value: object, keys: string | string[]) => Object.fromEntries(Object.entries(value).filter(([key]) => ![].concat(keys as never).includes(key as never)))
}));

const res = (group: string, kind: string, name: string, status: models.SyncStatusCode) => ({
    group,
    version: 'v1',
    kind,
    namespace: 'ns',
    name,
    status,
    health: {status: 'Healthy'}
});

const appWith = (...resources: ReturnType<typeof res>[]) =>
    ({
        metadata: {name: 'guestbook', namespace: 'argocd'},
        spec: {source: {repoURL: 'https://example.com/repo.git', targetRevision: 'HEAD'}},
        status: {resources}
    }) as unknown as models.Application;

// Sorted by nodeKey (group/kind/namespace/name), which is the order the panel renders and indexes them in:
// 0 /ConfigMap/ns/cm, 1 /Service/ns/api, 2 /Service/ns/web, 3 apps/Deployment/ns/api, 4 apps/Deployment/ns/web
const application = () =>
    appWith(
        res('apps', 'Deployment', 'web', 'OutOfSync'),
        res('', 'Service', 'web', 'Synced'),
        res('apps', 'Deployment', 'api', 'Synced'),
        res('', 'ConfigMap', 'cm', 'OutOfSync'),
        res('', 'Service', 'api', 'OutOfSync')
    );

const renderPanel = async (app = application()) => {
    let formApi: FormApi;
    const ctx = {
        history: createMemoryHistory(),
        popup: {confirm: jest.fn(), prompt: jest.fn()},
        notifications: {show: jest.fn()},
        navigation: {},
        baseHref: '/'
    } as unknown as React.ContextType<typeof Context>;
    const panel = (a: models.Application) => (
        <Context.Provider value={ctx}>
            <ApplicationSyncPanelBody application={a} selectedResource='all' hide={jest.fn()} getApi={api => (formApi = api)} setPending={jest.fn()} />
        </Context.Provider>
    );
    // Awaiting act lets the DataLoader inside the sync options settle, otherwise it logs act() warnings.
    let view: ReturnType<typeof render>;
    await act(async () => {
        view = render(panel(app));
    });
    return {
        submit: () => act(async () => formApi.submitForm(null)),
        update: (a: models.Application) => act(async () => view.rerender(panel(a)))
    };
};

const checkbox = (key: string) => screen.getByLabelText(key) as HTMLInputElement;
const sync = services.applications.sync as jest.Mock;
const syncedResources = () => sync.mock.calls[0][6];

describe('ApplicationSyncPanelBody kind filter', () => {
    beforeEach(() => sync.mockReset().mockResolvedValue({}));

    test('syncs the whole application (null resources) when nothing is filtered or unchecked', async () => {
        const {submit} = await renderPanel();
        await submit();
        expect(syncedResources()).toBeNull();
    });

    test('shows one chip per kind with counts and hides rows of other kinds', async () => {
        await renderPanel();
        expect(screen.getByRole('button', {name: 'Service (2)'})).toBeInTheDocument();
        fireEvent.click(screen.getByRole('button', {name: 'Deployment (2)'}));

        expect(screen.getByRole('button', {name: 'Deployment (2)', pressed: true})).toBeInTheDocument();
        expect(screen.queryByLabelText('/Service/ns/web')).not.toBeInTheDocument();
        expect(screen.queryByLabelText('/ConfigMap/ns/cm')).not.toBeInTheDocument();
        expect(checkbox('apps/Deployment/ns/web')).toBeInTheDocument();
    });

    test('status links act on visible rows only and the right original row is toggled and synced', async () => {
        const {submit} = await renderPanel();
        fireEvent.click(screen.getByRole('button', {name: 'Deployment (2)'}));

        fireEvent.click(screen.getByText('none', {selector: 'a'}));
        expect(checkbox('apps/Deployment/ns/web').checked).toBe(false);
        expect(checkbox('apps/Deployment/ns/api').checked).toBe(false);

        fireEvent.click(checkbox('apps/Deployment/ns/api'));
        expect(checkbox('apps/Deployment/ns/api').checked).toBe(true);
        expect(checkbox('apps/Deployment/ns/web').checked).toBe(false);

        // 3 hidden rows (ConfigMap + 2 Services) are still checked from the default "all" and must be reported.
        expect(screen.getByText(/3 selected resources hidden by the filter will also sync/)).toBeInTheDocument();
        fireEvent.click(screen.getByRole('button', {name: 'deselect them'}));
        expect(screen.queryByText(/hidden by the filter will also sync/)).not.toBeInTheDocument();

        await submit();
        expect(syncedResources().map((r: models.ResourceStatus) => `${r.kind}/${r.name}`)).toEqual(['Deployment/api']);
    });

    test('out of sync selects only visible out of sync rows and keeps hidden rows selected', async () => {
        const {submit} = await renderPanel();
        fireEvent.click(screen.getByRole('button', {name: 'Service (2)'}));
        fireEvent.click(screen.getByText('out of sync', {selector: 'a'}));

        expect(checkbox('/Service/ns/api').checked).toBe(true);
        expect(checkbox('/Service/ns/web').checked).toBe(false);

        await submit();
        const synced = syncedResources().map((r: models.ResourceStatus) => `${r.kind}/${r.name}`);
        expect(synced).toEqual(['ConfigMap/cm', 'Service/api', 'Deployment/api', 'Deployment/web']);
    });

    test('clearing the filter shows every row again', async () => {
        await renderPanel();
        fireEvent.click(screen.getByRole('button', {name: 'Deployment (2)'}));
        fireEvent.click(screen.getByRole('button', {name: 'clear filter'}));
        expect(screen.getByLabelText('/Service/ns/web')).toBeInTheDocument();
    });

    test('shows no kind chips when the application has a single kind', async () => {
        await renderPanel(appWith(res('apps', 'Deployment', 'web', 'OutOfSync'), res('apps', 'Deployment', 'api', 'Synced')));
        expect(screen.queryByRole('group', {name: 'Filter resources by kind'})).not.toBeInTheDocument();
        expect(checkbox('apps/Deployment/ns/api')).toBeInTheDocument();
    });

    test('ignores a selected kind that disappears from the application', async () => {
        const {update} = await renderPanel();
        fireEvent.click(screen.getByRole('button', {name: 'Deployment (2)'}));
        expect(screen.queryByLabelText('/Service/ns/web')).not.toBeInTheDocument();

        await update(appWith(res('', 'Service', 'web', 'Synced'), res('', 'ConfigMap', 'cm', 'OutOfSync')));
        expect(screen.getByLabelText('/Service/ns/web')).toBeInTheDocument();
    });

    test('none, then a kind, then out of sync selects only the out-of-sync rows of that kind (documented flow)', async () => {
        const {submit} = await renderPanel();
        fireEvent.click(screen.getByText('none', {selector: 'a'}));
        fireEvent.click(screen.getByRole('button', {name: 'Deployment (2)'}));
        fireEvent.click(screen.getByText('out of sync', {selector: 'a'}));

        expect(screen.queryByText(/hidden by the filter will also sync/)).not.toBeInTheDocument();
        await submit();
        expect(syncedResources().map((r: models.ResourceStatus) => `${r.kind}/${r.name}`)).toEqual(['Deployment/web']);
    });
});
