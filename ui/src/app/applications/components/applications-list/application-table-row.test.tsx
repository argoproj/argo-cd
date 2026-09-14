import * as React from 'react';
import {render, screen} from '@testing-library/react';
import {AuthSettingsCtx} from '../../../shared/context';
import * as models from '../../../shared/models';
import {ApplicationTableRow} from './application-table-row';

const app = {
    metadata: {
        name: 'guestbook',
        namespace: 'team-a',
        creationTimestamp: '2026-01-01T00:00:00Z',
        annotations: {},
        labels: {}
    },
    spec: {
        project: 'default',
        source: {
            repoURL: 'https://github.com/example/repo',
            path: '.',
            targetRevision: 'HEAD'
        },
        destination: {
            server: 'https://kubernetes.default.svc',
            namespace: 'default'
        }
    },
    status: {
        health: {status: 'Healthy'},
        sync: {status: 'Synced'},
        summary: {}
    }
} as models.Application;

const pref = {
    appList: {
        favoritesAppList: []
    }
} as any;

const ctx = {
    baseHref: '/',
    navigation: {
        goto: jest.fn()
    },
    notifications: {
        show: jest.fn()
    }
} as any;

const renderRow = (appsInAnyNamespaceEnabled: boolean) =>
    render(
        <AuthSettingsCtx.Provider value={{appsInAnyNamespaceEnabled} as models.AuthSettings}>
            <ApplicationTableRow
                app={app}
                selected={false}
                pref={pref}
                ctx={ctx}
                syncApplication={jest.fn()}
                refreshApplication={jest.fn()}
                deleteApplication={jest.fn()}
            />
        </AuthSettingsCtx.Provider>
    );

describe('ApplicationTableRow', () => {
    it('displays the raw name when apps in any namespace is disabled', () => {
        renderRow(false);

        expect(screen.getAllByText('guestbook').length).toBeGreaterThan(0);
        expect(screen.queryByText('team-a/guestbook')).not.toBeInTheDocument();
    });

    it('displays the namespace-qualified name when apps in any namespace is enabled', () => {
        renderRow(true);

        expect(screen.getAllByText('team-a/guestbook').length).toBeGreaterThan(0);
    });
});
