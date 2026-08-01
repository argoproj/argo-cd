jest.mock('lodash-es', () => ({
    cloneDeep: (val: any) => JSON.parse(JSON.stringify(val)),
    debounce: (fn: any) => fn
}));
jest.mock('../../../shared/services', () => ({
    services: {
        applications: {
            list: () => Promise.resolve({
                items: [
                    {
                        metadata: { name: 'app-1', namespace: 'argocd' },
                        spec: {
                            project: 'default',
                            destination: { server: 'https://kubernetes.default.svc', namespace: 'default' },
                            source: { repoURL: 'https://github.com/argoproj/argocd-example-apps', path: 'guestbook', targetRevision: 'HEAD' }
                        },
                        status: { sync: { status: 'OutOfSync' }, health: { status: 'Healthy' } }
                    }
                ],
                metadata: { resourceVersion: '123' }
            }),
            watch: () => {
                const { Subject } = require('rxjs');
                return new Subject();
            },
            getBatchApplicationDiff: () => Promise.resolve([])
        },
        clusters: {
            list: () => Promise.resolve([])
        },
        viewPreferences: {
            getPreferences: () => {
                const { BehaviorSubject } = require('rxjs');
                return new BehaviorSubject({
                    appList: {
                        view: 'summary',
                        syncFilter: [],
                        autoSyncFilter: [],
                        healthFilter: [],
                        namespacesFilter: [],
                        projectsFilter: [],
                        reposFilter: [],
                        syncOptionsFilter: [],
                        clustersFilter: [],
                        labelsFilter: [],
                        targetRevisionFilter: [],
                        annotationsFilter: [],
                        operationFilter: [],
                        showFavorites: false,
                        favoritesAppList: [],
                        search: ''
                    }
                });
            },
            updatePreferences: () => Promise.resolve({})
        }
    },
    AppsListViewKey: {
        List: 'list',
        Summary: 'summary',
        Tiles: 'tiles'
    }
}));

import { render, screen, waitFor } from '@testing-library/react';
import * as React from 'react';
import { MemoryRouter } from 'react-router-dom';
import { AppContextReact } from 'argo-ui';
import { ApplicationsList } from './applications-list';
import { ClusterCtx } from '../../../shared/components';
import { Context } from '../../../shared/context';
import * as models from '../../../shared/models';

jest.mock('react-svg-piechart', () => ({ default: () => null }));
jest.mock('../utils', () => {
    const original = jest.requireActual('../utils');
    return {
        ...original,
        ComparisonStatusIcon: () => null,
        HealthStatusIcon: () => null,
        HydrateOperationPhaseIcon: () => null
    };
});

// Mock GlobalDiffModal to avoid full-tree modal dependencies
jest.mock('../global-diff/GlobalDiffModal', () => ({
    GlobalDiffModal: ({ isShown }: { isShown: boolean }) => (
        isShown ? <div data-testid="global-diff-modal">Global Diff Modal Mock</div> : null
    )
}));

const mockContext = {
    apis: {
        popup: {
            alert: jest.fn(),
            confirm: jest.fn()
        },
        notifications: {
            show: jest.fn()
        },
        navigation: {
            goto: jest.fn()
        }
    },
    history: {
        location: {
            search: ''
        },
        listen: jest.fn(() => jest.fn())
    } as any,
    popup: {} as any,
    navigation: {
        goto: jest.fn(),
        stepBack: jest.fn()
    } as any,
    baseHref: '/',
    notifications: {
        show: jest.fn(),
    } as any,
};

describe('ApplicationsList View Diffs Badge Integration', () => {
    let sidebarDiv: HTMLDivElement;

    beforeEach(() => {
        sidebarDiv = document.createElement('div');
        sidebarDiv.setAttribute('id', 'sidebar-tools');
        document.body.appendChild(sidebarDiv);
    });

    afterEach(() => {
        if (sidebarDiv) {
            sidebarDiv.remove();
        }
    });

    test('renders Toolbar action button and displays OutOfSync apps badge', async () => {
        const historyMock = {
            push: jest.fn(),
            listen: jest.fn(),
            location: { search: '' },
            createHref: jest.fn()
        } as any;

        const matchMock = {
            url: '/applications',
            path: '/applications',
            isExact: true,
            params: {}
        };

        const appContext = {
            apis: mockContext.apis,
            router: {
                history: historyMock,
                route: {
                    location: historyMock.location,
                    match: matchMock
                }
            }
        } as any;

        render(
            <MemoryRouter initialEntries={['/applications']}>
                <ClusterCtx.Provider value={Promise.resolve([])}>
                    <AppContextReact.Provider value={appContext}>
                        <Context.Provider value={mockContext}>
                            <ApplicationsList history={historyMock} location={historyMock.location} match={matchMock} />
                        </Context.Provider>
                    </AppContextReact.Provider>
                </ClusterCtx.Provider>
            </MemoryRouter>
        );

        await waitFor(() => {
            const btn = screen.getByRole('button', { name: /View Diffs/i });
            expect(btn).toBeInTheDocument();
            const badge = btn.querySelector('.badge');
            expect(badge).toBeInTheDocument();
            expect(badge.textContent).toBe('1');
        });
    });
});
