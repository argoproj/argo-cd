import * as React from 'react';

import {Event} from '../../../shared/models';

/**
 * usePolledEvents loads events via a one-shot GET (the events endpoints are not streams) and refreshes
 * them on a fixed interval. The shared result is intended to feed both the EVENTS tab list and its badge.
 *
 * @param load    Loader returning the events. Identity does not need to be stable; it is re-read on each poll.
 * @param enabled When false, no request is made and no interval is scheduled (returns the last state).
 * @param keys    Values that identify the subject (e.g. name/namespace). A change restarts the polling.
 * @param intervalMs Poll interval in milliseconds (default 15s).
 */
export function usePolledEvents(load: () => Promise<Event[]>, enabled: boolean, keys: React.DependencyList, intervalMs = 15000): Event[] | null {
    const [events, setEvents] = React.useState<Event[] | null>(null);
    // Keep the latest loader without making it a dependency, so callers can pass an inline closure.
    const loadRef = React.useRef(load);
    React.useEffect(() => {
        loadRef.current = load;
    });

    React.useEffect(() => {
        if (!enabled) {
            return undefined;
        }
        let cancelled = false;
        const run = () =>
            loadRef
                .current()
                .then(next => !cancelled && setEvents(next))
                .catch(() => !cancelled && setEvents([]));
        run();
        const interval = setInterval(run, intervalMs);
        return () => {
            cancelled = true;
            clearInterval(interval);
        };
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [enabled, intervalMs, ...keys]);

    return events;
}
