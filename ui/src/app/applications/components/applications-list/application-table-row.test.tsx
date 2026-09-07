import {render} from '@testing-library/react';
import * as React from 'react';

import {ContextApis} from '../../../shared/context';
import * as models from '../../../shared/models';
import {ViewPreferences} from '../../../shared/services';
import {ApplicationTableRow} from './application-table-row';

jest.mock('../../../shared/components', () => ({
    ...jest.requireActual('../../../shared/components'),
    Cluster: () => null
}));
// argo-ui's Tooltip only mounts `content` into the DOM on hover; render it inline so the
// name tooltip's description text (see the assertions below) is queryable without simulating one.
jest.mock('argo-ui', () => {
    const actual = jest.requireActual('argo-ui');
    const react = require('react');
    return {
        ...actual,
        Tooltip: ({children, content}: {children: React.ReactNode; content: React.ReactNode}) => react.createElement(react.Fragment, null, children, content)
    };
});
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

const renderRow = (app: models.Application) =>
    render(<ApplicationTableRow app={app} selected={false} pref={pref} ctx={ctx} syncApplication={jest.fn()} refreshApplication={jest.fn()} deleteApplication={jest.fn()} />);

test('includes the application description in the name tooltip when one is set', () => {
    const {container} = renderRow(baseApp('Guestbook example application'));

    // The description sits as a bare text node next to <Moment> inside the Tooltip's `content`
    // (no wrapping element of its own), so assert on the rendered text rather than getByText.
    expect(container.textContent).toContain('Guestbook example application');
});

test('renders no description text when none is set', () => {
    const {container} = renderRow(baseApp(undefined));

    expect(container.textContent).not.toContain('Guestbook example application');
});
