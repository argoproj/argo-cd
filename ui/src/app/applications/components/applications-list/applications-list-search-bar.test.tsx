import {act, fireEvent, render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';
import {KeybindingProvider} from 'argo-ui/v2';
import {createMemoryHistory} from 'history';
import * as React from 'react';

import {Context} from '../../../shared/context';
import {ApplicationsListSearchBar} from './applications-list';

jest.mock('../../../shared/services', () => ({
    services: {viewPreferences: {getPreferences: jest.fn(), updatePreferences: jest.fn()}}
}));
jest.mock('../../../sidebar/sidebar', () => ({useSidebarTarget: () => ({current: null})}));
jest.mock('./applications-filter', () => ({ApplicationsFilter: () => null}));
jest.mock('./applications-status-bar', () => ({AppsStatusBar: () => null}));
jest.mock('./applications-table', () => ({ApplicationsTable: () => null}));
jest.mock('./applications-tiles', () => ({ApplicationTiles: () => null}));
jest.mock('../applications-sync-panel/applications-sync-panel', () => ({ApplicationsSyncPanel: () => null}));
jest.mock('../applications-refresh-panel/applications-refresh-panel', () => ({ApplicationsRefreshPanel: () => null}));
jest.mock('../application-sync-panel/application-sync-panel', () => ({ApplicationSyncPanel: () => null}));

// The search bar's "/" shortcut must turn off while the New App panel is open (?new=...), even though only
// the query string changes and the search bar's own props stay the same.
describe('ApplicationsListSearchBar', () => {
    const renderSearchBar = () => {
        const history = createMemoryHistory({initialEntries: ['/applications']});
        const ctx = {history, navigation: {goto: jest.fn()}} as any;
        render(
            <Context.Provider value={ctx}>
                <KeybindingProvider>
                    <ApplicationsListSearchBar content='' searchRegex={false} ctx={ctx} apps={[]} />
                </KeybindingProvider>
            </Context.Provider>
        );
        return {history, input: screen.getByPlaceholderText('Search applications...')};
    };

    const pressSlash = () => {
        fireEvent.keyDown(document.body, {key: '/', keyCode: 191});
        fireEvent.keyUp(document.body, {key: '/', keyCode: 191});
    };

    it('focuses the search on "/" when no panel is open', () => {
        const {input} = renderSearchBar();
        pressSlash();
        expect(input).toHaveFocus();
    });

    it('ignores "/" once the New App panel opens', () => {
        const {history, input} = renderSearchBar();
        act(() => history.push('/applications?new=%7B%7D'));
        pressSlash();
        expect(input).not.toHaveFocus();
    });
});
