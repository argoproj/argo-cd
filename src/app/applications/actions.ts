import { AppState, commonActions, NotificationType } from 'argo-ui';
import { push } from 'react-router-redux';
import { Dispatch } from 'redux';

import * as models from '../shared/models';
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

export function createApplication(appName: string, source: models.ApplicationSource, destination?: models.ApplicationDestination): any {
    return async (dispatch: Dispatch<any>, getState: () => AppState<State>) => {
        try {
            await services.applications.create(appName, source, destination);
            dispatch(push('/applications'));
        } catch (e) {
            dispatch(commonActions.showNotification({
                type: NotificationType.Error,
                content: `Unable to create application: ${e.response && e.response.text || 'Internal error'}`,
            }));
        }
    };
}
