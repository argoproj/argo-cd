import * as React from 'react';
import {type Observable} from 'rxjs';

import {useQuery, useObservableQuery} from '../hooks/query';

export interface QueryProps {
    children: (params: URLSearchParams) => React.ReactNode;
}

export interface ObservableQueryProps {
    children: (params: Observable<URLSearchParams>) => React.ReactNode;
}

function Query({children}: QueryProps) {
    const query = useQuery();

    return <>{children(query)}</>;
}

function ObservableQuery({children}: ObservableQueryProps) {
    const searchParams$ = useObservableQuery();

    return <>{children(searchParams$)}</>;
}

export {ObservableQuery, Query};
