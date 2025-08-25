import * as React from 'react';
import {BehaviorSubject, type Observable} from 'rxjs';
import {map} from 'rxjs/operators';

import {Context} from '../context';

function Query({children}: {children: (params: URLSearchParams) => React.ReactNode}) {
    const context = React.useContext(Context);

    return <>{children(new URLSearchParams(context?.history?.location.search))}</>;
}

export interface ObservableQueryProps {
    children: (params: Observable<URLSearchParams>) => React.ReactNode;
}

function ObservableQuery({children}: ObservableQueryProps) {
    const context = React.useContext(Context);

    const search = React.useMemo(() => {
        return new BehaviorSubject(context.history.location.search);
    }, []);

    const searchParams$ = React.useMemo(() => {
        return search.pipe(map(searchStr => new URLSearchParams(searchStr)));
    }, [search]);

    const handleLocationChange = React.useCallback(
        (location: any) => {
            search.next(location.search);
        },
        [search]
    );

    React.useEffect(() => {
        const unsubscribe = context.history.listen(handleLocationChange);
        return unsubscribe;
    }, [context.history, handleLocationChange]);

    return <>{children(searchParams$)}</>;
}

export {ObservableQuery, Query};
