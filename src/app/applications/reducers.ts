import { State } from './state';

export const ACTION_TYPES = {
    APPLICATIONS_LOAD_REQUEST: 'APPLICATIONS_LOAD_REQUEST',
    APPLICATIONS_LOAD_SUCCESS: 'APPLICATIONS_LOAD_SUCCESS',
};

export default function(state: State = { }, action: any): State {
    switch (action.type) {
        case ACTION_TYPES.APPLICATIONS_LOAD_REQUEST:
            return {...state, applications: null };
        case ACTION_TYPES.APPLICATIONS_LOAD_SUCCESS:
            return {...state, applications: action.applications};
    }
    return state;
}
