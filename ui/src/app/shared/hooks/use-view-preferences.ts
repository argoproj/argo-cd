import * as React from 'react';

import {services, ViewPreferences} from '../services';

// View preferences are local state (a BehaviorSubject over localStorage), not server data, so they
// are read synchronously instead of going through react-query.
const subscribe = (onChange: () => void) => {
    const subscription = services.viewPreferences.getPreferences().subscribe(() => onChange());
    return () => subscription.unsubscribe();
};

const getSnapshot = () => services.viewPreferences.getCurrentPreferences();

export function useViewPreferences(): ViewPreferences;
// Re-renders only when the selected value changes. Select a section (pref => pref.appList), not a newly
// built object: a new object on every call never compares equal and loops the render. Paginate and the
// resources table change sortOptions, sortDirections and pageSizes in place, so select their entries, not the maps.
export function useViewPreferences<T>(select: (pref: ViewPreferences) => T): T;
export function useViewPreferences<T>(select?: (pref: ViewPreferences) => T) {
    return React.useSyncExternalStore<ViewPreferences | T>(subscribe, select ? () => select(getSnapshot()) : getSnapshot);
}
