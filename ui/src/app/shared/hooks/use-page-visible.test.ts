import {act, renderHook} from '@testing-library/react';

import {usePageVisible} from './use-page-visible';

const setHidden = (hidden: boolean) => {
    Object.defineProperty(document, 'hidden', {configurable: true, get: () => hidden});
    document.dispatchEvent(new Event('visibilitychange'));
};

beforeEach(() => jest.useFakeTimers());
afterEach(() => {
    setHidden(false);
    jest.useRealTimers();
});

test('follows the tab once it stops switching', () => {
    const {result} = renderHook(() => usePageVisible());
    expect(result.current).toBe(true);

    act(() => setHidden(true));
    expect(result.current).toBe(true);
    act(() => jest.advanceTimersByTime(500));
    expect(result.current).toBe(false);

    // Switching back and forth within the debounce settles on the last state without a change in between.
    act(() => {
        setHidden(false);
        jest.advanceTimersByTime(300);
        setHidden(true);
        jest.advanceTimersByTime(300);
    });
    expect(result.current).toBe(false);
    act(() => {
        setHidden(false);
        jest.advanceTimersByTime(500);
    });
    expect(result.current).toBe(true);
});
