import {act, renderHook} from '@testing-library/react';

import {Event} from '../../../shared/models';
import {usePolledEvents} from './use-polled-events';

const event = (type = 'Normal'): Event => ({type, count: 1} as Event);

// Flush pending microtasks (promise callbacks) inside act so state updates are applied.
const flushPromises = () => act(async () => undefined);

afterEach(() => {
    jest.useRealTimers();
    jest.clearAllMocks();
});

describe('usePolledEvents', () => {
    it('does not fetch or schedule polling while disabled', async () => {
        const load = jest.fn().mockResolvedValue([event()]);

        const {result} = renderHook(() => usePolledEvents(load, false, ['app', 'ns']));
        await flushPromises();

        expect(load).not.toHaveBeenCalled();
        expect(result.current).toBeNull();
    });

    it('fetches once on mount when enabled and returns the events', async () => {
        const events = [event('Warning')];
        const load = jest.fn().mockResolvedValue(events);

        const {result} = renderHook(() => usePolledEvents(load, true, ['app', 'ns']));
        await flushPromises();

        expect(load).toHaveBeenCalledTimes(1);
        expect(result.current).toEqual(events);
    });

    it('sets an empty array when the loader rejects', async () => {
        const load = jest.fn().mockRejectedValue(new Error('boom'));

        const {result} = renderHook(() => usePolledEvents(load, true, ['app', 'ns']));
        await flushPromises();

        expect(result.current).toEqual([]);
    });

    it('re-fetches on the configured interval', async () => {
        jest.useFakeTimers();
        const load = jest.fn().mockResolvedValue([event()]);

        renderHook(() => usePolledEvents(load, true, ['app', 'ns'], 15000));
        await act(async () => undefined); // initial fetch
        expect(load).toHaveBeenCalledTimes(1);

        await act(async () => {
            jest.advanceTimersByTime(15000);
        });
        expect(load).toHaveBeenCalledTimes(2);

        await act(async () => {
            jest.advanceTimersByTime(15000);
        });
        expect(load).toHaveBeenCalledTimes(3);
    });

    it('stops polling after unmount', async () => {
        jest.useFakeTimers();
        const load = jest.fn().mockResolvedValue([event()]);

        const {unmount} = renderHook(() => usePolledEvents(load, true, ['app', 'ns'], 15000));
        await act(async () => undefined);
        expect(load).toHaveBeenCalledTimes(1);

        unmount();
        await act(async () => {
            jest.advanceTimersByTime(60000);
        });
        expect(load).toHaveBeenCalledTimes(1);
    });

    it('re-fetches when the keys change', async () => {
        const load = jest.fn().mockResolvedValue([event()]);

        const {rerender} = renderHook(({keys}) => usePolledEvents(load, true, keys), {
            initialProps: {keys: ['app', 'ns']}
        });
        await flushPromises();
        expect(load).toHaveBeenCalledTimes(1);

        rerender({keys: ['app', 'other-ns']});
        await flushPromises();
        expect(load).toHaveBeenCalledTimes(2);
    });

    it('does not re-fetch when only the loader identity changes (uses latest loader on next poll)', async () => {
        jest.useFakeTimers();
        const first = jest.fn().mockResolvedValue([event('Warning')]);
        const second = jest.fn().mockResolvedValue([event('Normal')]);

        const {rerender} = renderHook(({load}) => usePolledEvents(load, true, ['app', 'ns'], 15000), {
            initialProps: {load: first}
        });
        await act(async () => undefined);
        expect(first).toHaveBeenCalledTimes(1);

        // Swapping the loader closure with the same keys must not trigger a new fetch immediately.
        rerender({load: second});
        await act(async () => undefined);
        expect(second).not.toHaveBeenCalled();

        // ...but the next scheduled poll uses the latest loader.
        await act(async () => {
            jest.advanceTimersByTime(15000);
        });
        expect(second).toHaveBeenCalledTimes(1);
        expect(first).toHaveBeenCalledTimes(1);
    });
});
