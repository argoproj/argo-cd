import { push } from 'react-router-redux';
import { Dispatch } from 'redux';

import { services } from './services';

export function logout(): any {
    return async (dispatch: Dispatch<any>) => {
        await services.userService.logout();
        dispatch(push('/login'));
    };
}
