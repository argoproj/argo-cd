import * as React from 'react';
import {cleanup, fireEvent, render, screen, waitFor} from '@testing-library/react';
import {createMemoryHistory} from 'history';
import {NavigationManager} from 'argo-ui';
import {NEVER} from 'rxjs';

import {Context} from '../../../shared/context';
import {services, ViewPreferencesService} from '../../../shared/services';
import {ApplicationsList} from './applications-list';
import {ApplicationSetsList} from './application-sets-list';
import {ResourcesList} from '../../../resources/components/resources-list/resources-list';

jest.mock('argo-ui', () => ({
    ...jest.requireActual('argo-ui'),
    Tooltip: ({children}: {children: React.ReactNode}) => children
}));

jest.mock('../../../shared/components', () => ({
    ...jest.requireActual('../../../shared/components'),
    Page: ({children}: {children: React.ReactNode}) => children,
    FlexTopBar: () => null,
    Paginate: ({data, emptyState}: {data: unknown[]; emptyState: () => React.ReactNode}) => (data.length ? null : emptyState())
}));

jest.mock('lodash-es', () => ({
    __esModule: true,
    cloneDeep: (value: unknown) => (value == null ? value : JSON.parse(JSON.stringify(value)))
}));

const pages = ['applications', 'applicationsets', 'resources'] as const;
const originalPreferences = services.viewPreferences;

function storedPreferences() {
    return JSON.parse(window.localStorage.getItem('view_preferences'));
}

beforeEach(() => {
    window.localStorage.clear();
    services.viewPreferences = new ViewPreferencesService();
    services.viewPreferences.init();
    const sidebar = document.createElement('div');
    sidebar.id = 'sidebar-tools';
    document.body.appendChild(sidebar);
    jest.spyOn(services.clusters, 'list').mockResolvedValue([]);
    jest.spyOn(services.projects, 'list').mockResolvedValue([]);
    jest.spyOn(services.applications, 'watch').mockReturnValue(NEVER);
    jest.spyOn(services.applications, 'list').mockResolvedValue({
        metadata: {resourceVersion: '1'},
        items: [
            {
                metadata: {name: 'example', namespace: 'argocd', labels: {}, annotations: {}},
                spec: {project: 'default', destination: {server: 'https://kubernetes.default.svc', namespace: 'default'}, source: {repoURL: 'https://example.com/repo'}},
                status: {
                    health: {status: 'Degraded'},
                    sync: {status: 'Synced'},
                    resources: [{group: '', kind: 'ConfigMap', name: 'example', namespace: 'default', health: {status: 'Degraded'}, status: 'Synced'}]
                }
            }
        ]
    });
});

afterEach(() => {
    cleanup();
    document.getElementById('sidebar-tools')?.remove();
    jest.restoreAllMocks();
    services.viewPreferences = originalPreferences;
});

function renderList(page: (typeof pages)[number], query: string) {
    const history = createMemoryHistory({initialEntries: ['/' + page + query]});
    const ctx = {history, navigation: new NavigationManager(history)} as React.ContextType<typeof Context>;
    const props = {history, location: history.location, match: {path: '/' + page, url: '/' + page, isExact: true, params: {}}};
    const List = {applications: ApplicationsList, applicationsets: ApplicationSetsList, resources: ResourcesList}[page];
    return render(
        <Context.Provider value={ctx}>
            <List {...props} />
        </Context.Provider>
    );
}

test.each(pages)('%s production filter handler preserves saved health on URL-filter mount and saves user edits', async page => {
    services.viewPreferences.updateAppListPreferences({healthFilter: ['Healthy']});
    services.viewPreferences.updateResourcesListPreferences({healthFilter: ['Healthy']});
    const update = page === 'resources' ? 'updateResourcesListPreferences' : 'updateAppListPreferences';
    const handler = jest.spyOn(services.viewPreferences, update);
    renderList(page, '?health=Degraded');
    await waitFor(() => expect(handler).toHaveBeenCalled());
    const key = page === 'resources' ? 'resourcesList' : 'appList';
    expect(storedPreferences()[key].healthFilter).toEqual(['Healthy']);
    const health = (await screen.findByTitle('HEALTH STATUS')).closest('.filter');
    const degraded = Array.from(health.querySelectorAll('.filter__item')).find(row => row.textContent.includes('Degraded'));
    fireEvent.click(degraded.querySelector('.checkbox'));
    await waitFor(() => expect(storedPreferences()[key].healthFilter).toEqual([]));
});

test.each([
    ['applications', '10,2'],
    ['resources', '10,2'],
    ['applications', '10,2,10'],
    ['resources', '10,2,10']
] as const)('%s mount preserves saved projects when numeric URL filters normalize (%s)', async (page, projects) => {
    services.viewPreferences.updateAppListPreferences({projectsFilter: ['saved-project']});
    services.viewPreferences.updateResourcesListPreferences({projectsFilter: ['saved-project']});
    const key = page === 'resources' ? 'resourcesList' : 'appList';
    const update = page === 'resources' ? 'updateResourcesListPreferences' : 'updateAppListPreferences';
    const handler = jest.spyOn(services.viewPreferences, update);
    renderList(page, '?proj=' + projects);
    await waitFor(() => expect(handler.mock.calls.some(([change]) => change.projectsFilter?.join(',') === '2,10')).toBe(true));
    expect(storedPreferences()[key].projectsFilter).toEqual(['saved-project']);
});

describe.each(pages)('%s clear all persists explicit empty filters', page => {
    test.each([
        ['empty state', ''],
        ['empty state', 'Degraded'],
        ['sidebar', ''],
        ['sidebar', 'Degraded']
    ])('%s with health URL override %s', async (entry, health) => {
        services.viewPreferences.updateAppListPreferences({
            healthFilter: ['Healthy'],
            labelsFilter: ['team=saved'],
            annotationsFilter: ['saved=true'],
            projectsFilter: ['saved-project'],
            view: 'tiles'
        });
        services.viewPreferences.updateResourcesListPreferences({healthFilter: ['Healthy'], projectsFilter: ['saved-project'], view: 'summary'});
        services.viewPreferences.updatePreferences({appListSearch: 'saved-app-search', appSetListSearch: 'saved-set-search', resourcesListSearch: 'saved-resource-search'});
        const before = storedPreferences();
        const key = page === 'resources' ? 'resourcesList' : 'appList';
        renderList(page, '?health=' + health + '&labels=team%3Durl&search=review-no-match&view=list&page=2');
        await screen.findByTitle('HEALTH STATUS');
        await waitFor(() => expect(screen.getByText(entry === 'sidebar' ? 'CLEAR ALL' : 'clear filters')).toBeTruthy());
        expect(storedPreferences()).toEqual(before);
        fireEvent.click(screen.getByText(entry === 'sidebar' ? 'CLEAR ALL' : 'clear filters'));
        const expected = JSON.parse(JSON.stringify(before));
        const fields =
            page === 'resources'
                ? ['clustersFilter', 'healthFilter', 'namespacesFilter', 'projectsFilter', 'syncFilter', 'apiGroupFilter', 'kindFilter']
                : ['healthFilter', 'labelsFilter', 'annotationsFilter'];
        if (page === 'applications') {
            fields.push(
                'clustersFilter',
                'namespacesFilter',
                'reposFilter',
                'targetRevisionFilter',
                'projectsFilter',
                'syncFilter',
                'autoSyncFilter',
                'operationFilter',
                'hydrationFilter'
            );
        }
        for (const field of fields) {
            expected[key][field] = [];
        }
        if (page !== 'resources') {
            expected[key].showFavorites = false;
        }
        await waitFor(() => expect(storedPreferences()).toEqual(expected));
    });
});
