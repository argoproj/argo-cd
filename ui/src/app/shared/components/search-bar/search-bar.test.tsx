import * as React from 'react';
import {act, fireEvent, render, screen} from '@testing-library/react';
import {KeybindingProvider} from 'argo-ui/v2';
import {SearchBar} from './search-bar';

const items = ['guestbook', 'guestbook-prod', 'payments'];

const renderBar = (props: Partial<React.ComponentProps<typeof SearchBar>> & {value?: string; onChange?: (v: string) => void} = {}) => {
    const onChange = props.onChange ?? jest.fn();
    const ui = (value: string) => (
        <KeybindingProvider>
            <SearchBar autocomplete={{items, onSelect: jest.fn()}} {...props} value={value} onChange={onChange} />
        </KeybindingProvider>
    );
    const result = render(ui(props.value ?? ''));
    return {onChange, rerenderWith: (value: string) => result.rerender(ui(value)), ...result};
};

const input = () => screen.getByPlaceholderText('Search...') as HTMLInputElement;
const type = (value: string) => fireEvent.change(input(), {target: {value}});

describe('SearchBar', () => {
    beforeEach(() => jest.useFakeTimers());
    afterEach(() => jest.useRealTimers());

    it('updates the input immediately but debounces onChange', () => {
        const {onChange} = renderBar();
        type('g');
        type('gu');
        type('gue');
        expect(input().value).toBe('gue');
        expect(onChange).not.toHaveBeenCalled();

        act(() => {
            jest.advanceTimersByTime(300);
        });
        expect(onChange).toHaveBeenCalledTimes(1);
        expect(onChange).toHaveBeenCalledWith('gue');
    });

    it('applies a cleared value immediately', () => {
        const {onChange} = renderBar({value: 'guest'});
        type('');
        expect(onChange).toHaveBeenCalledWith('');
    });

    it('does not drop keystrokes while the parent value lags behind', () => {
        const {onChange, rerenderWith} = renderBar();
        type('gue');
        act(() => {
            jest.advanceTimersByTime(300);
        });
        expect(onChange).toHaveBeenCalledWith('gue');

        // user keeps typing before the parent echoes 'gue' back
        type('gues');
        rerenderWith('gue');
        expect(input().value).toBe('gues');
    });

    it('adopts external value changes', () => {
        const {rerenderWith} = renderBar({value: 'foo'});
        rerenderWith('bar');
        expect(input().value).toBe('bar');
    });

    it('adopts an external value equal to one it emitted earlier', () => {
        const {rerenderWith} = renderBar();
        type('foo');
        act(() => {
            jest.advanceTimersByTime(300);
        });
        rerenderWith('foo'); // echo of our own value
        rerenderWith(''); // external reset
        expect(input().value).toBe('');
        rerenderWith('foo'); // external restore (e.g. browser back)
        expect(input().value).toBe('foo');
    });

    it('drops a pending keystroke when the value changes externally', () => {
        const {onChange, rerenderWith} = renderBar({value: 'foo'});
        type('foobar');
        rerenderWith('baz'); // e.g. browser back before the debounce fires
        act(() => {
            jest.advanceTimersByTime(300);
        });
        expect(input().value).toBe('baz');
        expect(onChange).not.toHaveBeenCalled();
    });

    it('does not call onChange after unmount', () => {
        const {onChange, unmount} = renderBar();
        type('gue');
        unmount();
        act(() => {
            jest.advanceTimersByTime(300);
        });
        expect(onChange).not.toHaveBeenCalled();
    });
});
