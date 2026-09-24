import * as React from 'react';
import {render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';
import {MemoryRouter, Route} from 'react-router';

import {SettingsContainer} from './settings-container';

jest.mock('argo-ui/v2', () => ({
    KeybindingProvider: (props: {children?: React.ReactNode}) => props.children
}));

jest.mock('./settings-overview/settings-overview', () => {
    const ReactLib = require('react');
    return {SettingsOverview: () => ReactLib.createElement('div', null, 'settings overview panel')};
});

jest.mock('./repos-list/repos-list', () => {
    const ReactLib = require('react');
    return {ReposList: () => ReactLib.createElement('div', null, 'repos list panel')};
});

const renderAt = (path: string) =>
    render(
        <MemoryRouter initialEntries={[path]}>
            <Route path='/settings' component={SettingsContainer} />
        </MemoryRouter>
    );

describe('SettingsContainer', () => {
    it('renders the settings overview panel', () => {
        renderAt('/settings');

        expect(screen.getByText('settings overview panel')).toBeInTheDocument();
    });

    it('renders the repos list panel', () => {
        renderAt('/settings/repos');

        expect(screen.getByText('repos list panel')).toBeInTheDocument();
    });
});
