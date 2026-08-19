/* eslint-env jest */
declare const test: any;
declare const expect: any;
declare const describe: any;
declare const jest: any;

import * as React from 'react';
import {render, screen, fireEvent} from '@testing-library/react';
import {createMemoryHistory} from 'history';
import {Context, AuthSettingsCtx} from '../../../shared/context';
import {AuthSettings} from '../../../shared/models';
import {AdvancedSettings} from './advanced-settings';

jest.mock('../../../shared/components/monaco-editor', () => ({
    MonacoEditor: () => <div data-testid='monaco-editor-mock' />
}));

const mockAuthSettings: AuthSettings = {
    url: 'https://localhost:8080',
    statusBadgeEnabled: true,
    statusBadgeRootUrl: 'https://localhost:8080',
    googleAnalytics: {trackingID: '', anonymizeUsers: false},
    dexConfig: {connectors: []},
    oidcConfig: {name: ''},
    help: {chatUrl: '', chatText: '', binaryUrls: {}},
    userLoginsDisabled: false,
    kustomizeVersions: [],
    uiCssURL: '',
    uiBannerContent: '',
    uiBannerURL: '',
    uiBannerPermanent: false,
    uiBannerPosition: '',
    uiLoginButtonText: '',
    execEnabled: true,
    appsInAnyNamespaceEnabled: false,
    hydratorEnabled: false,
    syncWithReplaceAllowed: false,
    impersonationEnabled: false,
    resourceViewEnabled: false
};

const renderAdvancedSettings = (initialSearch = '?tab=configuration') => {
    const history = createMemoryHistory({initialEntries: [`/settings/advanced${initialSearch}`]});
    const navigation = {
        goto: (path: string, params?: Record<string, any>) => {
            const search = new URLSearchParams();
            if (params) {
                Object.entries(params).forEach(([k, v]) => {
                    if (v !== null && v !== undefined) {
                        search.set(k, String(v));
                    }
                });
            }
            const queryStr = search.toString() ? `?${search.toString()}` : '';
            history.replace(`${path}${queryStr}`);
        }
    };

    const ctxValue: any = {
        history,
        navigation,
        popup: null,
        notifications: null,
        baseHref: '/'
    };

    return render(
        <Context.Provider value={ctxValue}>
            <AuthSettingsCtx.Provider value={mockAuthSettings}>
                <AdvancedSettings />
            </AuthSettingsCtx.Provider>
        </Context.Provider>
    );
};

describe('AdvancedSettings component (#29247)', () => {
    test('action menu toggle is present on configuration tab and toggles filter state', () => {
        renderAdvancedSettings('?tab=configuration');

        // 1. Initial state on configuration tab shows "Show All Options" button
        const toggleButton = screen.getByRole('button', {name: /Show All Options/i});
        expect(toggleButton).toBeInTheDocument();

        // 2. Clicking toggles to "Show Configured Only"
        fireEvent.click(toggleButton);
        expect(screen.getByRole('button', {name: /Show Configured Only/i})).toBeInTheDocument();

        // 3. Clicking again toggles back to "Show All Options"
        fireEvent.click(screen.getByRole('button', {name: /Show Configured Only/i}));
        expect(screen.getByRole('button', {name: /Show All Options/i})).toBeInTheDocument();
    });

    test('action menu toggle remains present on JSON tab', () => {
        renderAdvancedSettings('?tab=json');

        // Verify toggle button is present on JSON tab
        const toggleButton = screen.getByRole('button', {name: /Show All Options/i});
        expect(toggleButton).toBeInTheDocument();
        expect(screen.getByTestId('monaco-editor-mock')).toBeInTheDocument();
    });

    test('exact reproduction sequence for issue #29247: toggle never disappears across tab transitions', () => {
        renderAdvancedSettings('?tab=configuration');

        // 1. Configuration tab: button present
        expect(screen.getByRole('button', {name: /Show All Options/i})).toBeInTheDocument();

        // 2. Click JSON tab
        const jsonTabHeader = screen.getByText('JSON');
        fireEvent.click(jsonTabHeader);
        expect(screen.getByRole('button', {name: /Show All Options/i})).toBeInTheDocument();
        expect(screen.getByTestId('monaco-editor-mock')).toBeInTheDocument();

        // 3. Click Configuration tab
        const configTabHeader = screen.getByText('Configuration');
        fireEvent.click(configTabHeader);
        expect(screen.getByRole('button', {name: /Show All Options/i})).toBeInTheDocument();

        // 4. Click "Show All Options" -> becomes "Show Configured Only"
        fireEvent.click(screen.getByRole('button', {name: /Show All Options/i}));
        expect(screen.getByRole('button', {name: /Show Configured Only/i})).toBeInTheDocument();

        // 5. Click "Show Configured Only" -> becomes "Show All Options"
        fireEvent.click(screen.getByRole('button', {name: /Show Configured Only/i}));
        expect(screen.getByRole('button', {name: /Show All Options/i})).toBeInTheDocument();

        // 6. Click JSON tab
        fireEvent.click(jsonTabHeader);
        // 7. Verify action menu is STILL present on JSON tab!
        expect(screen.getByRole('button', {name: /Show All Options/i})).toBeInTheDocument();

        // 8. Click Configuration tab
        fireEvent.click(configTabHeader);
        // 9. Verify toggle remains present and functional
        expect(screen.getByRole('button', {name: /Show All Options/i})).toBeInTheDocument();
    });
});
