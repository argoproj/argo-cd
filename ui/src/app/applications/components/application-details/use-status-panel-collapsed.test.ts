import {act, renderHook} from '@testing-library/react';

import {SHORT_WINDOW_MEDIA_QUERY} from '../../../shared/hooks/use-short-window';
import {AppDetailsPreferences, services} from '../../../shared/services';
import {useStatusPanelCollapsed} from './use-status-panel-collapsed';

jest.mock('../../../shared/services', () => ({
    services: {
        viewPreferences: {
            updatePreferences: jest.fn()
        }
    }
}));

interface MockMediaQueryList {
    matches: boolean;
    addEventListener: jest.Mock;
    removeEventListener: jest.Mock;
    notify(matches: boolean): void;
}

const mockMatchMedia = (matches: boolean): MockMediaQueryList => {
    const listeners = new Set<(event: MediaQueryListEvent) => void>();
    const mediaQuery: MockMediaQueryList = {
        matches,
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

const tallPref = {hideStatusPanel: false} as AppDetailsPreferences;
const hiddenPref = {hideStatusPanel: true} as AppDetailsPreferences;

beforeEach(() => {
    jest.clearAllMocks();
});

afterEach(() => {
    jest.restoreAllMocks();
});

describe('useStatusPanelCollapsed', () => {
    it('follows the persisted preference on a tall window', () => {
        mockMatchMedia(false);
        const {result, rerender} = renderHook(({pref}) => useStatusPanelCollapsed(pref), {initialProps: {pref: tallPref}});
        expect(result.current.collapsed).toBe(false);

        act(() => result.current.onToggle());
        expect(services.viewPreferences.updatePreferences).toHaveBeenCalledWith({appDetails: {...tallPref, hideStatusPanel: true}});

        rerender({pref: hiddenPref});
        expect(result.current.collapsed).toBe(true);

        act(() => result.current.onToggle());
        expect(services.viewPreferences.updatePreferences).toHaveBeenCalledWith({appDetails: {...hiddenPref, hideStatusPanel: false}});
    });

    it('auto-collapses on a short window without writing the preference', () => {
        mockMatchMedia(true);
        const {result} = renderHook(() => useStatusPanelCollapsed(tallPref));
        expect(result.current.collapsed).toBe(true);
        expect(services.viewPreferences.updatePreferences).not.toHaveBeenCalled();
    });

    it('lets the user expand a short window without writing the preference', () => {
        mockMatchMedia(true);
        const {result} = renderHook(() => useStatusPanelCollapsed(tallPref));
        expect(result.current.collapsed).toBe(true);

        act(() => result.current.onToggle());
        expect(result.current.collapsed).toBe(false);
        expect(services.viewPreferences.updatePreferences).not.toHaveBeenCalled();
    });

    it('collapses again on a short window as a local override without writing the preference', () => {
        mockMatchMedia(true);
        const {result} = renderHook(() => useStatusPanelCollapsed(tallPref));

        act(() => result.current.onToggle());
        expect(result.current.collapsed).toBe(false);

        act(() => result.current.onToggle());
        expect(result.current.collapsed).toBe(true);
        expect(services.viewPreferences.updatePreferences).not.toHaveBeenCalled();
    });

    it('keeps a persisted hide preference when expanding on a short window', () => {
        mockMatchMedia(true);
        const {result} = renderHook(() => useStatusPanelCollapsed(hiddenPref));
        expect(result.current.collapsed).toBe(true);

        act(() => result.current.onToggle());
        expect(result.current.collapsed).toBe(false);
        expect(services.viewPreferences.updatePreferences).not.toHaveBeenCalled();
    });

    it('clears a short-window expand override when the hide preference becomes true', () => {
        mockMatchMedia(true);
        const {result, rerender} = renderHook(({pref}) => useStatusPanelCollapsed(pref), {initialProps: {pref: tallPref}});

        act(() => result.current.onToggle());
        expect(result.current.collapsed).toBe(false);

        act(() => {
            rerender({pref: hiddenPref});
        });
        expect(result.current.collapsed).toBe(true);

        act(() => result.current.onToggle());
        expect(result.current.collapsed).toBe(false);
        expect(services.viewPreferences.updatePreferences).not.toHaveBeenCalled();
    });

    it('auto-collapses when a tall window becomes short and expands again when it becomes tall', () => {
        const mediaQuery = mockMatchMedia(false);
        const {result} = renderHook(() => useStatusPanelCollapsed(tallPref));
        expect(result.current.collapsed).toBe(false);

        act(() => mediaQuery.notify(true));
        expect(result.current.collapsed).toBe(true);
        expect(services.viewPreferences.updatePreferences).not.toHaveBeenCalled();

        act(() => mediaQuery.notify(false));
        expect(result.current.collapsed).toBe(false);
    });

    it('clears a short-window expand override when the window becomes tall', () => {
        const mediaQuery = mockMatchMedia(true);
        const {result} = renderHook(() => useStatusPanelCollapsed(tallPref));

        act(() => result.current.onToggle());
        expect(result.current.collapsed).toBe(false);

        act(() => mediaQuery.notify(false));
        expect(result.current.collapsed).toBe(false);

        act(() => mediaQuery.notify(true));
        expect(result.current.collapsed).toBe(true);
    });
});
