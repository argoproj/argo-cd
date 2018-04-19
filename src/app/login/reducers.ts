import { State } from './state';

import { ACTION_TYPES } from '../shared/actions';

export default function(state: State = { }, action: any): State {
    switch (action.type) {
        case ACTION_TYPES.LOGIN_SUCCEEDED:
            return {...state, loginError: null };
        case ACTION_TYPES.LOGIN_FAILED:
            return {...state, loginError: action.message };
    }
    return state;
}
