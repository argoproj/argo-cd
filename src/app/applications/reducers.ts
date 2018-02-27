import { State } from './state';

export const ACTION_TYPES = {
    APPLICATIONS_LOAD_REQUEST: 'APPLICATIONS_LOAD_REQUEST',
    APPLICATIONS_LOAD_SUCCESS: 'APPLICATIONS_LOAD_SUCCESS',
    APPLICATION_LOAD_REQUEST: 'APPLICATION_LOAD_REQUEST',
    APPLICATION_LOAD_SUCCESS: 'APPLICATION_LOAD_SUCCESS',
};

export default function(state: State = { }, action: any): State {
    switch (action.type) {
        case ACTION_TYPES.APPLICATIONS_LOAD_REQUEST:
            return {...state, applications: null };
        case ACTION_TYPES.APPLICATIONS_LOAD_SUCCESS:
            return {...state, applications: action.applications};
        case ACTION_TYPES.APPLICATION_LOAD_REQUEST:
            return {...state, application: null };
        case ACTION_TYPES.APPLICATION_LOAD_SUCCESS:
            return {...state, application: action.application};
    }
    return state;
}
