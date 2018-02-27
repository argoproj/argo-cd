import { AppState } from 'argo-ui';
import { Dispatch } from 'redux';

import { services } from '../shared/services';
import { ACTION_TYPES } from './reducers';
import { State } from './state';

export function loadAppsList(): any {
    return async (dispatch: Dispatch<any>, getState: () => AppState<State>) => {
        dispatch({ type: ACTION_TYPES.APPLICATIONS_LOAD_REQUEST });
        const applications = (await services.applications.list());
        dispatch({ type: ACTION_TYPES.APPLICATIONS_LOAD_SUCCESS, applications});
    };
}

export function loadApplication(name: string): any {
    return async (dispatch: Dispatch<any>, getState: () => AppState<State>) => {
        dispatch({ type: ACTION_TYPES.APPLICATION_LOAD_REQUEST });
        const application = await services.applications.get(name);
        dispatch({ type: ACTION_TYPES.APPLICATION_LOAD_SUCCESS, application });
    };
}
