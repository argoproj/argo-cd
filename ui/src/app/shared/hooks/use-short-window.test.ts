import {act, renderHook} from '@testing-library/react';

import {SHORT_WINDOW_MEDIA_QUERY, useShortWindow} from './use-short-window';

interface MockMediaQueryList {
    matches: boolean;
    media: string;
    addEventListener: jest.Mock;
    removeEventListener: jest.Mock;
    notify(matches: boolean): void;
}

const mockMatchMedia = (matches: boolean): MockMediaQueryList => {
    const listeners = new Set<(event: MediaQueryListEvent) => void>();
    const mediaQuery: MockMediaQueryList = {
        matches,
        media: SHORT_WINDOW_MEDIA_QUERY,
        addEventListener: jest.fn((_event: string, listener: (event: MediaQueryListEvent) => void) => {
            listeners.add(listener);
        }),
        removeEventListener: jest.fn((_event: string, listener: (event: MediaQueryListEvent) => void) => {
            listeners.delete(listener);
        }),
        notify(next: boolean) {
            mediaQuery.matches = next;
            const event = {matches: next} as MediaQueryListEvent;
            listeners.forEach(listener => listener(event));
        }
    };
    window.matchMedia = jest.fn().mockImplementation((query: string) => {
        expect(query).toBe(SHORT_WINDOW_MEDIA_QUERY);
        return mediaQuery;
    });
    return mediaQuery;
};

afterEach(() => {
    jest.restoreAllMocks();
});

test('reports a short window from the current media query', () => {
    mockMatchMedia(true);
    const {result} = renderHook(() => useShortWindow());
    expect(result.current).toBe(true);
});

test('reports a tall window from the current media query', () => {
    mockMatchMedia(false);
    const {result} = renderHook(() => useShortWindow());
    expect(result.current).toBe(false);
});

test('updates when the viewport height crosses the short-window threshold', () => {
    const mediaQuery = mockMatchMedia(false);
    const {result} = renderHook(() => useShortWindow());
    expect(result.current).toBe(false);

    act(() => mediaQuery.notify(true));
    expect(result.current).toBe(true);

    act(() => mediaQuery.notify(false));
    expect(result.current).toBe(false);
});

test('removes the media query listener on unmount', () => {
    const mediaQuery = mockMatchMedia(false);
    const {unmount} = renderHook(() => useShortWindow());
    unmount();
    expect(mediaQuery.removeEventListener).toHaveBeenCalledWith('change', expect.any(Function));
});
