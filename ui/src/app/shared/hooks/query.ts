import * as React from 'react';
import {BehaviorSubject} from 'rxjs';
import {map} from 'rxjs/operators';

import {Context} from '../context';

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

export {useQuery, useObservableQuery};
