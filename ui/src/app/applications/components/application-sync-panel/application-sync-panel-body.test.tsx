import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {act, fireEvent, render, screen, within} from '@testing-library/react';
import '@testing-library/jest-dom';
import {FormApi} from 'argo-ui';
import {createMemoryHistory} from 'history';
import * as React from 'react';

import {Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationSyncPanelBody} from './application-sync-panel-body';

jest.mock('../../../shared/services', () => ({
    services: {
        applications: {sync: jest.fn()},
        authService: {settings: jest.fn().mockResolvedValue({syncWithReplaceAllowed: false})}
    }
}));

jest.mock('argo-ui', () => ({...jest.requireActual('argo-ui'), Tooltip: ({children}: {children: React.ReactNode}) => children}));

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
    const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
    const ctx = {
        history: createMemoryHistory(),
        popup: {confirm: jest.fn(), prompt: jest.fn()},
        notifications: {show: jest.fn()},
        navigation: {},
        baseHref: '/'
    } as unknown as React.ContextType<typeof Context>;
    const panel = (a: models.Application) => (
        <QueryClientProvider client={client}>
            <Context.Provider value={ctx}>
                <ApplicationSyncPanelBody application={a} selectedResource='all' hide={jest.fn()} getApi={api => (formApi = api)} setPending={jest.fn()} />
            </Context.Provider>
        </QueryClientProvider>
    );
    let view: ReturnType<typeof render>;
    await act(async () => {
        view = render(panel(app));
    });
    return {
        container: view.container,
        submit: () => act(async () => formApi.submitForm(null)),
        update: (a: models.Application) => act(async () => view.rerender(panel(a)))
    };
};

const checkbox = (key: string) => screen.getByLabelText(key) as HTMLInputElement;
const sync = services.applications.sync as jest.Mock;
const syncedResources = () => sync.mock.calls[0][6];

const getKindFilterContainer = (container: HTMLElement) => container.querySelector('.application-sync-panel__kind-filter') as HTMLElement;

const selectKindInComboBox = (container: HTMLElement, optionTitle: string) => {
    const kindFilterEl = getKindFilterContainer(container);
    fireEvent.click(within(kindFilterEl).getByText(optionTitle, {selector: '.select__option span'}));
};

describe('ApplicationSyncPanelBody kind filter', () => {
    beforeEach(() => sync.mockReset().mockResolvedValue({}));

    it('syncs the whole application (null resources) when nothing is filtered or unchecked', async () => {
        const {submit} = await renderPanel();
        await submit();
        expect(syncedResources()).toBeNull();
    });

    it('does not render kind filter when only one kind is present', async () => {
        const {container} = await renderPanel(appWith(res('apps', 'Deployment', 'web', 'OutOfSync'), res('apps', 'Deployment', 'api', 'Synced')));
        expect(getKindFilterContainer(container)).toBeNull();
        expect(checkbox('apps/Deployment/ns/api')).toBeInTheDocument();
    });

    it('filters visible resources via the searchable Kind combo box and supports searching options', async () => {
        const {container} = await renderPanel();
        const kindFilterEl = getKindFilterContainer(container);
        expect(kindFilterEl).not.toBeNull();

        const searchInput = within(kindFilterEl).getByPlaceholderText('Search') as HTMLInputElement;
        fireEvent.change(searchInput, {target: {value: 'Dep'}});
        expect(within(kindFilterEl).getByText('Deployment', {selector: '.select__option span'})).toBeInTheDocument();
        expect(within(kindFilterEl).queryByText('Service', {selector: '.select__option span'})).not.toBeInTheDocument();

        selectKindInComboBox(container, 'Deployment');

        expect(screen.queryByLabelText('/Service/ns/web')).not.toBeInTheDocument();
        expect(screen.queryByLabelText('/ConfigMap/ns/cm')).not.toBeInTheDocument();
        expect(checkbox('apps/Deployment/ns/web')).toBeInTheDocument();
        expect(checkbox('apps/Deployment/ns/api')).toBeInTheDocument();
    });

    it('respects out-of-sync status filter when filtering by kind, showing only out-of-sync resources and deselecting other kinds', async () => {
        const {container, submit} = await renderPanel();

        // Layer 1: select 'out of sync'
        fireEvent.click(screen.getByText('out of sync', {selector: 'a'}));

        // Layer 2: filter by Deployment
        selectKindInComboBox(container, 'Deployment');

        // Only the 1 out-of-sync deployment is visible, synced deployment is filtered out
        expect(checkbox('apps/Deployment/ns/web')).toBeInTheDocument();
        expect(checkbox('apps/Deployment/ns/web').checked).toBe(true);
        expect(screen.queryByLabelText('apps/Deployment/ns/api')).not.toBeInTheDocument();
        expect(screen.queryByLabelText('/ConfigMap/ns/cm')).not.toBeInTheDocument();
        expect(screen.queryByLabelText('/Service/ns/api')).not.toBeInTheDocument();

        // Submit to verify only Deployment/web is synced (no ConfigMap or Service)
        await submit();
        expect(syncedResources().map((r: models.ResourceStatus) => `${r.kind}/${r.name}`)).toEqual(['Deployment/web']);
    });

    it('switches between all, out of sync, and none while a kind is filtered', async () => {
        const {container, submit} = await renderPanel();

        // Filter to Deployment first
        selectKindInComboBox(container, 'Deployment');

        // By default 'all' is active: both deployments visible and checked
        expect(checkbox('apps/Deployment/ns/web')).toBeInTheDocument();
        expect(checkbox('apps/Deployment/ns/api')).toBeInTheDocument();
        expect(checkbox('apps/Deployment/ns/web').checked).toBe(true);
        expect(checkbox('apps/Deployment/ns/api').checked).toBe(true);

        // Switch to 'out of sync': only out-of-sync deployment visible
        fireEvent.click(screen.getByText('out of sync', {selector: 'a'}));
        expect(checkbox('apps/Deployment/ns/web')).toBeInTheDocument();
        expect(checkbox('apps/Deployment/ns/web').checked).toBe(true);
        expect(screen.queryByLabelText('apps/Deployment/ns/api')).not.toBeInTheDocument();

        // Switch to 'none': uncheck
        fireEvent.click(screen.getByText('none', {selector: 'a'}));
        expect(checkbox('apps/Deployment/ns/web').checked).toBe(false);

        // Manually check web
        fireEvent.click(checkbox('apps/Deployment/ns/web'));
        await submit();
        expect(syncedResources().map((r: models.ResourceStatus) => `${r.kind}/${r.name}`)).toEqual(['Deployment/web']);
    });

    it('resets filter when selected kind disappears from application resources', async () => {
        const {container, update} = await renderPanel();
        selectKindInComboBox(container, 'Deployment');
        expect(screen.queryByLabelText('/Service/ns/web')).not.toBeInTheDocument();

        await update(appWith(res('', 'Service', 'web', 'Synced'), res('', 'ConfigMap', 'cm', 'OutOfSync')));
        expect(screen.getByLabelText('/Service/ns/web')).toBeInTheDocument();
    });

    it('supports selecting a kind using keyboard navigation and selection', async () => {
        const {container} = await renderPanel();
        // The trigger is a button; the focused search input carries the combobox semantics.
        const trigger = screen.getByRole('button', {name: /filter by kind/i});
        expect(trigger).toHaveAttribute('aria-expanded', 'false');

        // Open dropdown via keyboard
        fireEvent.keyDown(trigger, {key: 'ArrowDown'});
        expect(trigger).toHaveAttribute('aria-expanded', 'true');

        const searchInput = within(getKindFilterContainer(container)).getByRole('combobox', {name: /filter by kind/i});
        // Available options: [0: "All kinds", 1: "ConfigMap", 2: "Deployment", 3: "Service"]
        // The option Enter would select is exposed on the focused input.
        fireEvent.keyDown(searchInput, {key: 'ArrowDown'});
        expect(searchInput).toHaveAttribute('aria-activedescendant', 'application-sync-panel-kind-filter-select-opt-ConfigMap');
        fireEvent.keyDown(searchInput, {key: 'ArrowDown'});
        expect(searchInput).toHaveAttribute('aria-activedescendant', 'application-sync-panel-kind-filter-select-opt-Deployment');
        // Press Enter to select
        fireEvent.keyDown(searchInput, {key: 'Enter'});

        // Filter is now Deployment: Service and ConfigMap should not be in the document
        expect(screen.queryByLabelText('/Service/ns/web')).not.toBeInTheDocument();
        expect(screen.queryByLabelText('/ConfigMap/ns/cm')).not.toBeInTheDocument();
        expect(checkbox('apps/Deployment/ns/web')).toBeInTheDocument();
        expect(checkbox('apps/Deployment/ns/api')).toBeInTheDocument();
    });

    it('deselects other kinds when filtering to a specific kind and syncs only that kind', async () => {
        const {container, submit} = await renderPanel();

        // Start with default panel state, then filter to Service
        selectKindInComboBox(container, 'Service');

        // Only Service rows are visible and selected; Deployment and ConfigMap are excluded
        expect(checkbox('/Service/ns/api')).toBeInTheDocument();
        expect(checkbox('/Service/ns/web')).toBeInTheDocument();
        expect(screen.queryByLabelText('apps/Deployment/ns/web')).not.toBeInTheDocument();
        expect(screen.queryByLabelText('/ConfigMap/ns/cm')).not.toBeInTheDocument();

        await submit();
        expect(syncedResources().map((r: models.ResourceStatus) => `${r.kind}/${r.name}`).sort()).toEqual([
            'Service/api',
            'Service/web'
        ]);
    });
});
