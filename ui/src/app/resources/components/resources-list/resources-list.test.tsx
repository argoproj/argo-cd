import * as React from 'react';
import {render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';
import {KeybindingProvider} from 'argo-ui/v2';

import {ContextApis} from '../../../shared/context';
import {HealthStatusBarPreferences, ResourcesListPreferences} from '../../../shared/services';
import {ResourcesToolbar} from './resources-list';

// The toolbar does not use these panels; mocking them keeps their heavy import chains out of the test.
jest.mock('./resources-details-panel', () => ({ResourcesDetailsPanel: (): null => null}));
jest.mock('./resources-table', () => ({ResourcesTable: (): null => null}));

jest.mock('../../../shared/services', () => ({
    ...jest.requireActual('../../../shared/services'),
    services: {viewPreferences: {updatePreferences: jest.fn()}}
}));

const renderToolbar = (search: string, searchRegex: boolean) => {
    const pref = {search, searchRegex, page: 0} as ResourcesListPreferences & {page: number; search: string};
    const ctx = {navigation: {goto: jest.fn()}} as unknown as ContextApis;
    const healthBarPrefs = {showHealthStatusBar: true} as HealthStatusBarPreferences;
    return render(
        <KeybindingProvider>
            <ResourcesToolbar pref={pref} ctx={ctx} healthBarPrefs={healthBarPrefs} />
        </KeybindingProvider>
    );
};

describe('ResourcesToolbar search bar', () => {
    it('shows the plain-text placeholder when regex search is disabled', () => {
        renderToolbar('', false);

        const input = screen.getByPlaceholderText('Search resources...');
        expect(input.closest('.search-bar__input')).not.toHaveClass('search-bar__input--regex');
    });

    it('shows the regex placeholder and regex styling when regex search is enabled', () => {
        renderToolbar('', true);

        expect(screen.queryByPlaceholderText('Search resources...')).not.toBeInTheDocument();
        const input = screen.getByPlaceholderText('Regex search (e.g. ^foo-.*-prod$)');
        expect(input.closest('.search-bar__input')).toHaveClass('search-bar__input--regex');
    });
});
