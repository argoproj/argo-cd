import { AppState } from 'argo-ui';
import { Dispatch } from 'redux';
import { Observable } from 'rxjs';

import { services } from '../shared/services';
import { ACTION_TYPES } from './reducers';
import { State } from './state';

function ensureUnsubscribed(getState: () => AppState<State>) {
    const state = getState();
    if (state.page.changesSubscription) {
        state.page.changesSubscription.unsubscribe();
    }
}

export function loadAppsList(): any {
    return async (dispatch: Dispatch<any>, getState: () => AppState<State>) => {
        dispatch({ type: ACTION_TYPES.APPLICATIONS_LOAD_REQUEST });

        ensureUnsubscribed(getState);
        const changesSubscription = services.applications.watch().subscribe((applicationChange) => {
            dispatch({ type: ACTION_TYPES.APPLICATIONS_CHANGED, applicationChange });
        });

        const applications = (await services.applications.list());
        dispatch({ type: ACTION_TYPES.APPLICATIONS_LOAD_SUCCESS, applications, changesSubscription});
    };
}

export function loadApplication(name: string): any {
    return async (dispatch: Dispatch<any>, getState: () => AppState<State>) => {
        dispatch({ type: ACTION_TYPES.APPLICATION_LOAD_REQUEST });
        const appUpdates = Observable
            .from([await services.applications.get(name)])
            .merge(services.applications.watch({name}).map((changeEvent) => changeEvent.application));
        const changesSubscription = appUpdates.subscribe((application) => {
            dispatch({ type: ACTION_TYPES.APPLICATION_LOAD_SUCCESS, application, changesSubscription });
        });
    };
}
