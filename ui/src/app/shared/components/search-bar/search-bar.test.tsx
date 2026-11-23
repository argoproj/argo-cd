import {act, fireEvent, render, screen} from '@testing-library/react';
import {KeybindingProvider} from 'argo-ui/v2';
import * as React from 'react';

import {SearchBar} from './search-bar';

const renderSearchBar = (ui: React.ReactElement) => render(<KeybindingProvider>{ui}</KeybindingProvider>);

describe('SearchBar', () => {
    beforeEach(() => {
        jest.useFakeTimers();
    });

    afterEach(() => {
        jest.useRealTimers();
    });

    test('cancels pending debounce when an external value replaces the edit', () => {
        const onChange = jest.fn();
        const {rerender} = renderSearchBar(<SearchBar value='' onChange={onChange} debounceMs={300} />);

        fireEvent.change(screen.getByPlaceholderText('Search...'), {target: {value: 'stale'}});
        expect(onChange).not.toHaveBeenCalled();
        expect(screen.getByPlaceholderText('Search...')).toHaveValue('stale');

        rerender(
            <KeybindingProvider>
                <SearchBar value='external' onChange={onChange} debounceMs={300} />
            </KeybindingProvider>
        );
        expect(screen.getByPlaceholderText('Search...')).toHaveValue('external');

        act(() => {
            jest.advanceTimersByTime(300);
        });
        expect(onChange).not.toHaveBeenCalled();
    });

    test('still notifies onChange after debounce when the value is not replaced', () => {
        const onChange = jest.fn();
        renderSearchBar(<SearchBar value='' onChange={onChange} debounceMs={300} />);

        fireEvent.change(screen.getByPlaceholderText('Search...'), {target: {value: 'keep'}});
        act(() => {
            jest.advanceTimersByTime(300);
        });
        expect(onChange).toHaveBeenCalledWith('keep');
    });
});
