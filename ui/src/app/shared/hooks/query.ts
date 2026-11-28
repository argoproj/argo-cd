import * as React from 'react';
import {BehaviorSubject} from 'rxjs';
import {map} from 'rxjs/operators';

import {Context} from '../context';

// The current URL query params. Subscribes to history so that query-only
// navigation (e.g. ?editRole=foo) re-renders the caller; reading
// history.location during render alone is not enough once the React Compiler
// memoizes the component.
const useSearchParams = () => {
    const search = useLocationSearch();
    return React.useMemo(() => new URLSearchParams(search), [search]);
};

function useObservableQuery() {
    const context = React.useContext(Context);

    const [search$] = React.useState(() => new BehaviorSubject(context.history.location.search));

    const searchParams$ = React.useMemo(() => search$.pipe(map(searchStr => new URLSearchParams(searchStr))), [search$]);

    React.useEffect(() => {
        const unsubscribe = context.history.listen(location => {
            search$.next(location.search);
        });
        return unsubscribe;
    }, [context.history, search$]);

    React.useEffect(() => {
        return () => {
            search$.complete();
        };
    }, [search$]);

    return searchParams$;
}

// The current URL query string, re-rendering when navigation changes it.
function useLocationSearch(): string {
    const history = React.useContext(Context)?.history;
    return React.useSyncExternalStore(
        React.useCallback(onChange => (history ? history.listen(() => onChange()) : () => {}), [history]),
        () => history?.location.search ?? ''
    );
}

export {useSearchParams, useObservableQuery, useLocationSearch};
