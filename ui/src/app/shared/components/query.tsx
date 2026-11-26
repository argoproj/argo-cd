import * as React from 'react';
import {type Observable} from 'rxjs';

import {useLocationSearch, useObservableQuery} from '../hooks/query';

export interface QueryProps {
    children: (params: URLSearchParams) => React.ReactNode;
}

export interface ObservableQueryProps {
    children: (params: Observable<URLSearchParams>) => React.ReactNode;
}

// Subscribes to history so that query-only navigation (e.g. ?editRole=foo)
// re-renders the children, even when the React Compiler memoizes the parent.
function Query({children}: QueryProps) {
    const search = useLocationSearch();
    const query = React.useMemo(() => new URLSearchParams(search), [search]);

    return <>{children(query)}</>;
}

function ObservableQuery({children}: ObservableQueryProps) {
    const searchParams$ = useObservableQuery();

    return <>{children(searchParams$)}</>;
}

export {ObservableQuery, Query};
