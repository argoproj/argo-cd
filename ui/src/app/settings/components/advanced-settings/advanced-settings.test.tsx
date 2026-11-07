import * as React from 'react';
import {render, screen, fireEvent} from '@testing-library/react';
import '@testing-library/jest-dom';
import {AppContextReact} from 'argo-ui';
import {MemoryRouter, useHistory} from 'react-router';

import {AuthSettingsCtx, Context} from '../../../shared/context';
import {AuthSettings} from '../../../shared/models';
import {services} from '../../../shared/services';
import {AdvancedSettings} from './advanced-settings';

jest.mock('../../../shared/components/monaco-editor', () => {
    const ReactLib = require('react');
    return {MonacoEditor: () => ReactLib.createElement('div', {'data-testid': 'json-editor'})};
});

const settings: AuthSettings = {
    appLabelKey: 'app.kubernetes.io/instance',
    execEnabled: true
} as AuthSettings;

// The advanced settings page derives the selected tab from the URL and stores it in the query
// string, so the test host has to re-render on history changes the same way the router does.
const Host = () => {
    const history = useHistory();
    const [, reRender] = React.useState(0);
    React.useEffect(() => history.listen(() => reRender(n => n + 1)), [history]);

    const ctx = React.useMemo(
        () => ({
            history,
            baseHref: '/',
            navigation: {
                goto: (path: string, params: {[key: string]: string} = {}) => {
                    const search = Object.entries(params)
                        .filter(([, value]) => value !== undefined)
                        .map(([key, value]) => `${key}=${encodeURIComponent(value)}`)
                        .join('&');
                    history.push({pathname: path.startsWith('.') ? history.location.pathname : path, search: search ? `?${search}` : ''});
                },
                redirect: () => undefined
            } as any,
            popup: {} as any,
            notifications: {} as any
        }),
        [history]
    );

    return (
        <AppContextReact.Provider value={{apis: {notifications: {show: () => undefined}}} as any}>
            <Context.Provider value={ctx}>
                <AuthSettingsCtx.Provider value={settings}>
                    <AdvancedSettings />
                </AuthSettingsCtx.Provider>
            </Context.Provider>
        </AppContextReact.Provider>
    );
};

const renderPage = () =>
    render(
        <MemoryRouter initialEntries={['/settings/advanced?tab=configuration']}>
            <Host />
        </MemoryRouter>
    );

const filterToggle = () => screen.queryByText(/Show All Options|Show Configured Only/);

describe('AdvancedSettings', () => {
    beforeAll(() => {
        services.viewPreferences.init();
        // The login state is rendered in the toolbar; keep it pending so it cannot update the
        // component tree while the test is asserting on the filter button.
        jest.spyOn(services.users, 'get').mockReturnValue(new Promise(() => undefined) as any);
    });

    it('keeps the show all/configured only toggle when switching between the tabs', () => {
        renderPage();
        expect(filterToggle()).toBeInTheDocument();

        fireEvent.click(screen.getByText('JSON'));
        expect(filterToggle()).toBeInTheDocument();

        fireEvent.click(screen.getByText('Configuration'));
        expect(filterToggle()).toBeInTheDocument();

        fireEvent.click(screen.getByText('Show All Options'));
        expect(filterToggle()).toHaveTextContent('Show Configured Only');

        fireEvent.click(screen.getByText('JSON'));
        expect(filterToggle()).toBeInTheDocument();

        fireEvent.click(screen.getByText('Configuration'));
        expect(filterToggle()).toHaveTextContent('Show Configured Only');

        fireEvent.click(screen.getByText('Show Configured Only'));
        expect(filterToggle()).toHaveTextContent('Show All Options');
    });
});
