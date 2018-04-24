import { AppState, commonActions, NotificationType } from 'argo-ui';
import { push } from 'react-router-redux';
import { Dispatch } from 'redux';
import { Observable } from 'rxjs';

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

export function syncApplication(name: string, revision: string): any {
    return async (dispatch: Dispatch<any>, getState: () => AppState<State>) => {
        try {
            await services.applications.sync(name, revision);
        } catch (e) {
            dispatch(commonActions.showNotification({
                type: NotificationType.Error,
                content: `Unable to deploy revision: ${e.response && e.response.text || 'Internal error'}`,
            }));
        }
    };
}

export function rollbackApplication(name: string, id: number): any {
    return async (dispatch: Dispatch<any>, getState: () => AppState<State>) => {
        try {
            await services.applications.rollback(name, id);
        } catch (e) {
            dispatch(commonActions.showNotification({
                type: NotificationType.Error,
                content: `Unable to rollback application: ${e.response && e.response.text || 'Internal error'}`,
            }));
        }
    };
}

export function deletePod(appName: string, podName: string): any {
    return async (dispatch: Dispatch<any>, getState: () => AppState<State>) => {
        try {
            await services.applications.deletePod(appName, podName);
        } catch (e) {
            dispatch(commonActions.showNotification({
                type: NotificationType.Error,
                content: `Unable to delete pod: ${e.response && e.response.text || 'Internal error'}`,
            }));
        }
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

export function deleteApplication(appName: string, force: boolean): any {
    return async (dispatch: Dispatch<any>, getState: () => AppState<State>) => {
        try {
            await services.applications.delete(appName, force);
            dispatch(push('/applications'));
        } catch (e) {
            dispatch(commonActions.showNotification({
                type: NotificationType.Error,
                content: `Unable to delete application: ${e.response && e.response.text || 'Internal error'}`,
            }));
        }
    };
}
