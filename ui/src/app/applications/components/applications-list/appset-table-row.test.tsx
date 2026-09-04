import * as React from 'react';
import {render, screen} from '@testing-library/react';
import {AuthSettingsCtx} from '../../../shared/context';
import * as models from '../../../shared/models';
import {AppSetTableRow} from './appset-table-row';

const appSet = {
    metadata: {
        name: 'api',
        namespace: 'team-a',
        creationTimestamp: '2026-01-01T00:00:00Z',
        annotations: {},
        labels: {}
    },
    status: {
        conditions: []
    }
} as models.ApplicationSet;

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
            <AppSetTableRow appSet={appSet} selected={false} pref={pref} ctx={ctx} />
        </AuthSettingsCtx.Provider>
    );

describe('AppSetTableRow', () => {
    it('displays the raw name when apps in any namespace is disabled', () => {
        renderRow(false);

        expect(screen.getAllByText('api').length).toBeGreaterThan(0);
        expect(screen.queryByText('team-a/api')).not.toBeInTheDocument();
    });

    it('displays the namespace-qualified name when apps in any namespace is enabled', () => {
        renderRow(true);

        expect(screen.getAllByText('team-a/api').length).toBeGreaterThan(0);
    });
});
