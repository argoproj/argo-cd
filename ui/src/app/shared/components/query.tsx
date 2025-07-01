import * as React from 'react';
import {BehaviorSubject, type Observable} from 'rxjs';
import {map} from 'rxjs/operators';

import {Context} from '../context';

export interface QueryProps {
    children: (params: URLSearchParams) => React.ReactNode;
}

export interface ObservableQueryProps {
    children: (params: Observable<URLSearchParams>) => React.ReactNode;
}

const useQuery = () => {
    const context = React.useContext(Context);

    return new URLSearchParams(context?.history?.location.search);
};

function useObservableQuery() {
    const context = React.useContext(Context);

    const search$ = React.useMemo(() => new BehaviorSubject(context.history.location.search), []);

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

function Query({children}: QueryProps) {
    const query = useQuery();

    return <>{children(query)}</>;
}

function ObservableQuery({children}: ObservableQueryProps) {
    const searchParams$ = useObservableQuery();

    return <>{children(searchParams$)}</>;
}

export {ObservableQuery, Query, useQuery, useObservableQuery};
