import {act, renderHook} from '@testing-library/react';

import {services} from '../services';
import {useViewPreferences} from './use-view-preferences';

jest.mock('../services', () => {
    const actual = jest.requireActual('../services/view-preferences-service');
    return {...actual, services: {viewPreferences: new actual.ViewPreferencesService()}};
});

beforeAll(() => services.viewPreferences.init());

// The service keeps its state across tests; start each from the same theme and sidebar.
beforeEach(() => services.viewPreferences.updatePreferences({theme: 'light', hideSidebar: false}));

const renderCounting = <T>(read: () => T) => {
    let renders = 0;
    const hook = renderHook(() => {
        renders++;
        return read();
    });
    return {result: hook.result, renders: () => renders};
};

test('returns all preferences and re-renders on any change', () => {
    const {result, renders} = renderCounting(() => useViewPreferences());

    act(() => services.viewPreferences.updatePreferences({theme: 'dark'}));

    expect(result.current.theme).toBe('dark');
    expect(renders()).toBe(2);
});

test('with a selector, re-renders only when the selected section changes', () => {
    const {result, renders} = renderCounting(() => useViewPreferences(pref => pref.appList));

    act(() => services.viewPreferences.updatePreferences({theme: 'dark'}));
    expect(renders()).toBe(1);

    act(() => services.viewPreferences.updatePreferences({appList: {...result.current, search: 'guestbook'}}));
    expect(result.current.search).toBe('guestbook');
    expect(renders()).toBe(2);
});

test('with a selector, does not re-render when the whole preferences are passed back with another change', () => {
    const {renders} = renderCounting(() => useViewPreferences(pref => pref.appList));

    // As the sidebar toggle does: {...pref, hideSidebar}.
    act(() => services.viewPreferences.updatePreferences({...services.viewPreferences.getCurrentPreferences(), hideSidebar: true}));

    expect(renders()).toBe(1);
});
