import {render, screen} from '@testing-library/react';
import * as React from 'react';

import {ContextApis} from '../../../shared/context';
import * as models from '../../../shared/models';
import {ViewPreferences} from '../../../shared/services';
import {ApplicationTile} from './application-tile';

jest.mock('../../../shared/components', () => ({
    ...jest.requireActual('../../../shared/components'),
    Cluster: () => null
}));
jest.mock('../utils', () => ({
    ...jest.requireActual('../utils'),
    ComparisonStatusIcon: () => null,
    HealthStatusIcon: () => null,
    HydrateOperationPhaseIcon: () => null,
    OperationState: () => null
}));

const baseApp = (description?: string) =>
    ({
        metadata: {name: 'guestbook', namespace: 'argocd', creationTimestamp: '2024-01-01T00:00:00Z', labels: {}, annotations: {}},
        spec: {
            project: 'default',
            description,
            destination: {server: 'https://kubernetes.default.svc', namespace: 'default'},
            source: {repoURL: 'https://github.com/example/guestbook.git', path: 'guestbook', targetRevision: 'HEAD'}
        },
        status: {health: {status: 'Healthy'}, sync: {status: 'Synced'}, summary: {}}
    }) as unknown as models.Application;

const ctx = {baseHref: '/', navigation: {goto: jest.fn()}, notifications: {show: jest.fn()}, popup: {}} as unknown as ContextApis;
const pref = {appList: {favoritesAppList: []}, appDetails: {view: 'tiles'}} as unknown as ViewPreferences;

const renderTile = (app: models.Application) =>
    render(
        <ApplicationTile
            app={app}
            selected={false}
            pref={pref}
            ctx={ctx}
            syncApplication={jest.fn()}
            refreshApplication={jest.fn()}
            deleteApplication={jest.fn()}
        />
    );

test('shows a Description row with the application description when one is set', () => {
    renderTile(baseApp('Guestbook example application'));

    expect(screen.getByText('Description:')).toBeInTheDocument();
    expect(screen.getByText('Guestbook example application')).toBeInTheDocument();
});

test('omits the Description row entirely when no description is set', () => {
    renderTile(baseApp(undefined));

    expect(screen.queryByText('Description:')).not.toBeInTheDocument();
});
