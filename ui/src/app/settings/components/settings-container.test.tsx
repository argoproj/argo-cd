import * as React from 'react';
import {render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';
import {MemoryRouter, Route} from 'react-router';

import {SettingsContainer} from './settings-container';

jest.mock('argo-ui/v2', () => ({
    KeybindingProvider: (props: {children?: React.ReactNode}) => props.children
}));

jest.mock('../../shared/components', () => ({
    Spinner: () => {
        const ReactLib = require('react');
        return ReactLib.createElement('div', null, 'loading spinner');
    }
}));

jest.mock('./settings-overview/settings-overview', () => {
    const ReactLib = require('react');
    return {SettingsOverview: () => ReactLib.createElement('div', null, 'settings overview panel')};
});

jest.mock('./repos-list/repos-list', () => {
    throw new Error('Loading chunk settings-repos failed');
});

const renderAt = (path: string) =>
    render(
        <MemoryRouter initialEntries={[path]}>
            <Route path='/settings' component={SettingsContainer} />
        </MemoryRouter>
    );

describe('SettingsContainer', () => {
    let consoleError: jest.SpyInstance;

    beforeEach(() => {
        consoleError = jest.spyOn(console, 'error').mockImplementation(() => undefined);
    });

    afterEach(() => {
        consoleError.mockRestore();
    });

    it('renders a lazy settings panel through the Suspense fallback', async () => {
        renderAt('/settings');

        expect(screen.getByText('loading spinner')).toBeInTheDocument();
        expect(await screen.findByText('settings overview panel')).toBeInTheDocument();
        expect(screen.queryByText('loading spinner')).not.toBeInTheDocument();
    });

    it('shows the error boundary message when a panel chunk fails to load', async () => {
        renderAt('/settings/repos');

        expect(await screen.findByText('Failed to load this page. Please reload and try again.')).toBeInTheDocument();
    });
});
