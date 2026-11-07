import * as React from 'react';

import {services, ViewPreferences} from '../services';

// View preferences are local state (a BehaviorSubject over localStorage), not server data, so they
// are read synchronously instead of going through react-query.
const subscribe = (onChange: () => void) => {
    const subscription = services.viewPreferences.getPreferences().subscribe(() => onChange());
    return () => subscription.unsubscribe();
};

const getSnapshot = () => services.viewPreferences.getCurrentPreferences();

export function useViewPreferences(): ViewPreferences {
    return React.useSyncExternalStore(subscribe, getSnapshot);
}
