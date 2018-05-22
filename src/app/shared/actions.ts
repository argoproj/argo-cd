import { push } from 'react-router-redux';
import { Dispatch } from 'redux';

import { services } from './services';

export const ACTION_TYPES = {
    LOGIN_SUCCEEDED: 'LOGIN_SUCCEEDED',
    LOGIN_FAILED: 'LOGIN_FAILED',
};

export function login(username: string, password: string, returnURL: string): any {
    return async (dispatch: Dispatch<any>) => {
        try {
            await services.userService.login(username, password);
            dispatch({ type: ACTION_TYPES.LOGIN_SUCCEEDED });
            if (returnURL) {
                const url = new URL(returnURL);
                dispatch(push(url.pathname + url.search));
            } else {
                dispatch(push('/applications'));
            }
        } catch (e) {
            dispatch({ type: ACTION_TYPES.LOGIN_FAILED, message: e.response.body.error });
        }
    };
}

export function logout(): any {
    return async (dispatch: Dispatch<any>) => {
        await services.userService.logout();
        dispatch(push('/login'));
    };
}
