import {act, cleanup, render} from '@testing-library/react';
import {AppContextReact} from 'argo-ui';
import {KeybindingProvider} from 'argo-ui/v2';
import * as React from 'react';
import {of} from 'rxjs';
import {Provider} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationTiles} from './applications-tiles';

// Regression coverage for the two flows reported in #30257: switching "Items per page" from "all" back to
// a finite page, and tearing the tiles view down when navigating into an application. Both shrink the list
// of mounted tiles, which is where the reported "NotFoundError: Failed to execute 'removeChild' on 'Node'"
// used to replace the page with the error screen.

const makeApp = (i: number): models.Application =>
    ({
        kind: 'Application',
        metadata: {
            name: `app-${i}`,
            namespace: 'argocd',
            uid: `uid-${i}`,
            creationTimestamp: '2020-01-01T00:00:00Z',
            labels: {team: `team-${i}`},
            annotations: {}
        },
        spec: {
            project: 'default',
            destination: {server: 'https://kubernetes.default.svc', namespace: 'default'},
            source: {repoURL: `https://github.com/org/repo-${i}.git`, path: `apps/app-${i}`, targetRevision: 'HEAD'}
        },
        status: {health: {status: 'Healthy'}, sync: {status: 'Synced'}, summary: {externalURLs: []}}
    }) as models.Application;

const applications = Array.from({length: 27}, (_, i) => makeApp(i));

const ctx = {
    navigation: {goto: jest.fn()},
    popup: {confirm: jest.fn()},
    notifications: {show: jest.fn()},
    baseHref: '/'
} as any;

const renderTiles = (apps: models.Application[]) => (
    <AppContextReact.Provider
        value={{apis: {popup: ctx.popup, notifications: ctx.notifications, navigation: ctx.navigation}, history: {} as any, router: {} as any}}>
        <Provider value={ctx}>
            <KeybindingProvider>
                <ApplicationTiles
                    applications={apps}
                    syncApplication={jest.fn()}
                    refreshApplication={jest.fn()}
                    deleteApplication={jest.fn()}
                    useVirtualScrolling={false}
                    statusBarVisible={true}
                />
            </KeybindingProvider>
        </Provider>
    </AppContextReact.Provider>
);

const tiles = (container: HTMLElement) => container.querySelectorAll('.applications-list__entry');

beforeEach(() => {
    jest.spyOn(services.viewPreferences, 'getPreferences').mockReturnValue(
        of({
            appList: {view: 'tiles'},
            appDetails: {view: 'tree'},
            pageSizes: {'applications-list': -1}
        }) as any
    );
});

afterEach(() => {
    cleanup();
    jest.restoreAllMocks();
});

test('renders every application while the page size is "all"', () => {
    const {container} = render(renderTiles(applications));

    expect(tiles(container)).toHaveLength(applications.length);
});

test('switching from "all" back to 10 items per page only unmounts the extra tiles', () => {
    const {container, rerender} = render(renderTiles(applications));
    expect(tiles(container)).toHaveLength(applications.length);

    // Same component instance, smaller slice: this is what the paginator hands over after the change.
    act(() => {
        rerender(renderTiles(applications.slice(0, 10)));
    });

    expect(tiles(container)).toHaveLength(10);
    expect(tiles(container)[0].textContent).toContain('app-0');
    expect(Array.from(tiles(container)).some(tile => tile.textContent.includes('app-26'))).toBe(false);
});

test('tearing the tiles view down does not throw', () => {
    const {unmount} = render(renderTiles(applications));

    act(() => {
        expect(() => unmount()).not.toThrow();
    });
});
