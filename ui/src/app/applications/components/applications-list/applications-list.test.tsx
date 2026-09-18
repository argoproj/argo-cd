import {act, fireEvent, render, screen} from '@testing-library/react';
import * as React from 'react';

jest.mock('../application-create-panel/application-create-panel', () => ({
    ApplicationCreatePanel: () => null
}));

jest.mock('../application-sync-panel/application-sync-panel', () => ({
    ApplicationSyncPanel: () => null
}));

jest.mock('../applications-sync-panel/applications-sync-panel', () => ({
    ApplicationsSyncPanel: () => null
}));

jest.mock('../applications-refresh-panel/applications-refresh-panel', () => ({
    ApplicationsRefreshPanel: () => null
}));

jest.mock('./applications-summary', () => ({
    ApplicationsSummary: () => null
}));

jest.mock('./applications-table', () => ({
    ApplicationsTable: () => null
}));

jest.mock('./applications-tiles', () => ({
    ApplicationTiles: () => null
}));

jest.mock('./applications-status-bar', () => ({
    AppsStatusBar: () => null
}));

jest.mock('./view-type-switcher', () => ({
    ViewTypeSwitcher: () => null
}));

jest.mock('../../../shared/components', () => ({
    SearchBar: ({
        value,
        onChange,
        placeholder
    }: {
        value: string;
        onChange: (value: string) => void;
        placeholder: string;
    }) => <input value={value} onChange={e => onChange(e.target.value)} placeholder={placeholder} />
}));

jest.mock('../../../shared/context', () => ({
    AuthSettingsCtx: React.createContext({appsInAnyNamespaceEnabled: false})
}));

import {ApplicationsListSearchBar} from './applications-list';

describe('ApplicationsListSearchBar', () => {
    afterEach(() => {
        jest.useRealTimers();
        jest.clearAllMocks();
    });

    test('preserves rapid keystrokes and debounces navigation', () => {
        jest.useFakeTimers();

        const navigate = jest.fn();

        render(
            <ApplicationsListSearchBar
                content=''
                searchRegex={false}
                ctx={{navigation: {goto: navigate}} as any}
                apps={[]}
            />
        );

        const input = screen.getByPlaceholderText('Search applications...');

        act(() => {
            fireEvent.change(input, {target: {value: 't'}});
            fireEvent.change(input, {target: {value: 'te'}});
            fireEvent.change(input, {target: {value: 'tes'}});
            fireEvent.change(input, {target: {value: 'test'}});
        });

        expect(input).toHaveValue('test');
        expect(navigate).not.toHaveBeenCalled();

        act(() => {
            jest.advanceTimersByTime(199);
        });

        expect(navigate).not.toHaveBeenCalled();

        act(() => {
            jest.advanceTimersByTime(1);
        });

        expect(navigate).toHaveBeenCalledTimes(1);
        expect(navigate).toHaveBeenCalledWith('.', {search: 'test'}, {replace: true});
    });

    test('updates the search input when content changes externally', () => {
    const navigate = jest.fn();

    const SearchBarWithKey = ({content}: {content: string}) => (
        <ApplicationsListSearchBar
            key={content}
            content={content}
            searchRegex={false}
            ctx={{navigation: {goto: navigate}} as any}
            apps={[]}
        />
    );

    const {rerender} = render(<SearchBarWithKey content='initial' />);

    expect(screen.getByPlaceholderText('Search applications...')).toHaveValue('initial');

    rerender(<SearchBarWithKey content='updated' />);

    expect(screen.getByPlaceholderText('Search applications...')).toHaveValue('updated');
});
});